# ES/OpenSearch Shard 容量 Runbook

> 背景：2026-07-21 `product_snapshot_to_es` 写入报错 —— 集群 shard 数打满
> `[7200]/[7200] maximum normal shards open`，rollover 新索引需 8 shard 被拒。
> 本文记录 shard 与 node 的关系、上限计算、过多的危害、以及止血 + 治本方案。

---

## 1. Shard 与 Node 的关系

- **Shard = 数据分布/并行的最小单位**，一个 shard（含其 segment）只能被**一个 data node** 承载，不能跨机拆分。
- **primary shard 数 ≤ 有效 data node 数** 才能让所有节点都参与写入/查询；primary 比 node 少则部分 node 对该索引闲置。
- **replica 与 primary 不同 node**（高可用），所以需要 `data node 数 ≥ (1 + replica)`，否则 replica 一直 unassigned（集群 yellow）。
- **总 shard 数 = primary × (1 + replica)**。

## 2. Shard 上限怎么算

集群级硬上限（最常触发）：

```
集群 open shard 上限 = cluster.max_shards_per_node × data node 数
```

- 只算 **data node**，master-only / coordinating / ingest-only 节点**不计入**。
- 只统计 **normal open shard**；`close` 的索引不占额度，frozen 层走单独的 `cluster.max_shards_per_node.frozen`。

### 本集群（2026-07-21 事故时）

| 项 | 值 |
|----|----|
| `cluster.max_shards_per_node` | 1200 |
| data node 数 | 6（已确认全是 data node） |
| **集群 shard 上限** | **1200 × 6 = 7200** |
| 事故时水位 | 7200 / 7200（100% 打满） |

> `cluster.routing.use_adaptive_replica_selection: false` 与 shard 上限无关，是关掉自适应副本选择、查询路由退回轮询，另一码事。

## 3. 常用查询命令

```bash
# 确认 data node 数（node.role 含 d 才是 data node）
GET _cat/nodes?v&h=name,node.role,master

# 当前 open shard 总数（÷ 上限 = 水位）
GET _cluster/health?filter_path=active_shards,unassigned_shards

# 每个 node 扛多少 shard（是否顶格/均衡）
GET _cat/allocation?v

# 按索引看 shard 数 + 创建时间（找日期滚动堆积的老索引）
GET _cat/indices?v&h=index,pri,rep,docs.count,store.size,creation.date.string&s=creation.date

# 当前 shard 上限设置
GET _cluster/settings?include_defaults=true&flat_settings=true&filter_path=**.max_shards_per_node

# CPU 到底花在哪（GC/merge 空耗 vs 真查询）
GET _nodes/hot_threads?threads=10
```

## 4. Shard 过多的危害（按触发顺序）

| 瓶颈 | 顺序 | 机制 |
|------|------|------|
| **Heap / GC** | 最先 | 每个 shard 常驻占 heap（Lucene segment 元数据、field data、terms 索引），几千 shard = 几 GB 常驻 → 频繁 GC，极端 OOM |
| **Master / cluster state** | 其次 | 每个 shard 的路由/mapping/settings 都在 cluster state 里，由 master 维护并广播；shard 越多 state 越大 → 建/删索引卡、master 慢甚至脑裂风险 |
| **CPU** | 通常伴随 GC | 见下 |
| **磁盘 IO** | merge/refresh 密集时 | 每 shard 独立 segment merge，shard 多则 merge 线程争抢、IO 打满 |

### CPU 会不会被拖垮？会，但多是"空耗型"CPU

Shard 过多推高 CPU 的路径：
1. **GC 烧 CPU** —— shard 多 → heap 常驻多 → GC 频繁，GC 本身占满 CPU 核，且不干正事。
2. **Segment merge** —— 每 shard 独立 merge，CPU + IO 双密集，后台线程抢核。
3. **查询 scatter-gather 协调** —— 查询 fan-out 到 N 个 shard 各起 search task，shard 越多则调度 + reduce 合并（尤其聚合查询）CPU 越高。
4. **小 shard 固定成本** —— 每 shard 查询有与数据量无关的固定开销（初始化 search context 等），碎片越多固定成本总和越大。

> **关键区分**：shard 多导致的 CPU 高，大部分是 **GC + merge + scatter 协调**这类**非业务 CPU**，会出现"CPU 很忙但 QPS 上不去"的反直觉现象。诊断时用 `_nodes/hot_threads` 确认：大量 GC/merge 线程 → 是碎片 shard 空耗，**加核没用，得减 shard**；search/bulk 线程才是真业务负载。
>
> 对照项目记忆 `compute_seller_dashboard_monthly_features_v1`（IO-bound、cpu/run≈7%，加 CPU 无效）—— 同理，别单看 CPU 高就加核。

## 5. 合理设置经验值（业界通用）

| 维度 | 推荐值 | 说明 |
|------|--------|------|
| 单 shard 大小 | **10–50 GB** | 日志/时序可到 30–50GB；检索类 20–30GB |
| 每 GB heap 承载 shard | **≤ 20 个** | 官方硬经验。31GB heap node → ≤ ~600 shard |
| 单 node shard 总数 | 通常几百，别到 1000+ | 本集群 `max_shards_per_node=1200` 偏激进 |
| 单索引 primary 数 | `数据量 ÷ 目标 shard 大小` 向上取整 | 小索引就 1 个 primary，别习惯性给 8 |
| replica | 生产 ≥ 1 | 需 data node ≥ (1 + replica) |

**本集群健康水位估算**：6 data node，假设每 node heap 31GB → 健康上限约 `31 × 20 × 6 ≈ 3700` shard。真实水位应落在 **3000–4000**，而非逼近 7200。7200 是激进上限，长期隐患已在积累。

## 6. 事故处置：止血 + 治本

### 止血（二选一，建议先 A 争取时间再做 B）

**A. 临时抬上限（最快恢复写入，治标）**
```bash
PUT _cluster/settings
{ "persistent": { "cluster.max_shards_per_node": 1400 } }   # 1400×6=8400 留缓冲
```

**B. 释放 shard（治本，删/关老索引）**
```bash
POST pre-xxx_<老日期>-v2/_close        # 关闭：保留数据、不占 open 额度、可随时 open
DELETE pre-xxx_<确认无用日期>-v2        # 删除：释放最彻底（删前确认无查询在读）
```

### 治本（否则明天照样爆）

**杠杆 1 — 降单索引 primary 数（收益最大，一步砍到 1/8）**

事故索引 `number_of_shards=8`。先自查单日数据量：
```bash
GET _cat/indices/pre-xxx_<最新日期>*?v&h=index,pri,store.size
GET pre-xxx_<最新日期>-v2/_settings?filter_path=**.number_of_shards
```
- 单日 < 30GB → **1 个 primary 足够**；单日几 GB 却分 8 shard = 每 shard 几百 MB，纯浪费。
- 把 index template 的 `number_of_shards` 降到 1–2，**下次滚动的新日期索引生效**（符合项目"不 reindex、新滚动索引承载新配置"的惯例，见 CLAUDE.md §5）。

**杠杆 2 — 配 ISM/OpenSearch（ILM/ES）到期自动删老索引**

日滚动索引必须有生命周期策略，不能靠手删。保留天数估算：
```
可保留天数 ≈ 目标水位(如 3600) ÷ 单日新增 shard 数
```
- 降到 1 primary + 1 replica = 2 shard/天 → 3600 / 2 = 1800 天都放得下。
- 维持 8 primary + 1 replica = 16 shard/天 → 3600 / 16 ≈ 225 天就满。

> **优先拧杠杆 1**（降 primary），一步把 shard 增速砍到 1/8，再叠加杠杆 2 的到期删除。

## 7. 一句话总结

Shard 过多的本质危害是 **cluster state 膨胀拖垮 master + heap 常驻 GC + 查询 scatter 放大 + merge/GC 空耗 CPU**；健康标准是 **单 shard 10–50GB、每 GB heap ≤20 shard、单 node 几百个**。本集群 7200 上限偏激进，治本是**把日索引 primary 从 8 降到 1–2 + ISM 到期删老索引**，让真实水位回落到 3000–4000。
