# PostgreSQL WAL、复制机制与存储放大

> 来源：个人思考（由一次 DTS 数据迁移引出的专题梳理）
> 收录时间：2026-06-05 16:20 CST
>
> 主线：WAL 是什么 → wal_level 三级 → 物理复制 vs 逻辑复制 → 复制槽的坑 → 读/写/空间放大（RUM 猜想）

---

## 0. 这条线是怎么串起来的

做 DTS 数据迁移时，发现 `wal_level` 需要从 `replica` 改成 `logical`。顺着这个点往下挖，可以串起一整条 PostgreSQL 复制与存储原理的认知链：

```
WAL（预写日志，一切的基础）
  └─ wal_level 决定 WAL 记多少信息
       ├─ replica → 物理复制（主从/主备/读副本/PITR）
       └─ logical → 逻辑复制（CDC/DTS/选表同步/跨版本）
            └─ 靠 replication slot 保证 WAL 不被回收
                 └─ slot 不清理 → WAL 堆积 → 磁盘撑爆
            └─ logical 多记信息 → 写放大 ↑
                 └─ 引出 读/写/空间 三种放大与 RUM 权衡
```

---

## 1. WAL 是什么

**WAL = Write-Ahead Log（预写日志）。**

核心原则：**任何对数据页的修改，必须先把变更写进 WAL 并落盘，才能修改实际的数据文件。**

为什么这样设计：

- **崩溃恢复（持久性 D）**：宕机后重放 WAL，把"已提交但还没刷到数据文件"的事务恢复出来。
- **性能**：WAL 是顺序写，远快于随机写数据页。事务提交只要保证 WAL 落盘即可，脏页延迟批量刷（checkpoint）。
- **复制的基础**：WAL 本身就是一份"变更流水账"，传给备库重放即实现物理复制。

结构：WAL 由一段段 segment 文件组成（默认 16MB），每条记录用 **LSN（Log Sequence Number）** 作为全局位置标识。

---

## 2. wal_level 的三个级别

`wal_level` 决定**往 WAL 里记录多少信息**。级别越高记得越详细，写放大越大。三者是**逐级包含**关系：`logical ⊃ replica ⊃ minimal`。

| 级别 | 记录内容 | 用途 |
|------|---------|------|
| `minimal` | 只记崩溃恢复必需的信息 | 单机，无复制、无归档 |
| `replica` | 在 minimal 基础上，支持**物理复制**和 WAL 归档 | 主备流复制、PITR（默认值）|
| `logical` | 在 replica 基础上，额外记录**逻辑解码**所需信息 | 逻辑复制、CDC、DTS 迁移 |

> ⚠️ `wal_level` **不是热生效参数，修改后必须重启实例**。云数据库改这个参数会触发重启，安排在低峰期。

---

## 3. 两种复制：物理 vs 逻辑（最关键的概念区分）

PostgreSQL 里"复制"有两个完全不同的体系，极易混淆：

| | 物理复制 (Physical) | 逻辑复制 (Logical) |
|---|---|---|
| 别名 | 流复制 / streaming replication | logical replication |
| 复制单位 | 整个实例（字节级照搬数据页） | 表级（可选库、选表） |
| wal_level | **replica 就够** | 必须 **logical** |
| 典型场景 | 主从、主备、读写分离、灾备、PITR | CDC、异构迁移(DTS)、选择性同步、跨大版本升级 |
| 备库能否写 | 只读 | 目标端是独立可写的库 |
| 版本要求 | 主备版本需一致 | 可跨大版本 |

### 本质区别：WAL 能不能被解码成行级变更

- **物理复制**：备库是主库的**字节级克隆**。WAL 重放就是照搬数据页的物理变更——不需要知道"users 表 id=5 的 name 改成了张三"，只需要知道"12345 号 8KB 数据页第几字节变成了什么"。→ `replica` 记的物理信息刚好够。

- **逻辑复制**：DTS 做异构迁移（PG→PG 不同版本、PG→MySQL、PG→数仓），目标端物理存储结构完全不同，照搬数据页没意义。必须把 WAL **逻辑解码（logical decoding）** 成可理解的行级 SQL 变更：
  ```
  INSERT INTO users (id, name) VALUES (5, '张三')
  UPDATE orders SET status='paid' WHERE id=100
  ```
  要解码出这些，WAL 必须额外记录：涉及哪张表、哪些列、以及 UPDATE/DELETE 时如何定位到具体行。→ 才要 `logical`。

### 常见误区纠正

- ❌ "主从库应该用 logical" → ✅ **主从/主备/读副本都是物理复制，`replica` 就够**。用 logical 不是"更好"，而是浪费——多记的逻辑解码信息用不上，白增写放大。
- 主库和备库是**同一组**：主从 = 一个主库 + 一个或多个备库，整套用 `replica`。
- 只有要做**逻辑层面同步**（DTS/CDC、原生 `CREATE PUBLICATION/SUBSCRIPTION`、Debezium/Kafka Connect、选表同步、跨大版本升级）才需要 `logical`。

```
wal_level = minimal   →  单机，无备库
wal_level = replica   →  物理复制（主从/主备/读副本/PITR）   ← 高可用默认
wal_level = logical   →  逻辑复制（CDC/DTS/选表/跨版本）      ← 迁移时临时升到这
                          （向下兼容，也能同时跑物理复制）
```

---

## 4. 逻辑复制的配套要点（DTS 迁移实操）

光改 `wal_level` 不够，逻辑复制 / DTS 通常还要处理：

### 4.1 怎么改参数（wal_level / slot 数 / sender 数）

三个相关参数，**都需要重启实例才生效**（不能 `reload`）：

| 参数 | 作用 | 生效方式 |
|------|------|---------|
| `wal_level` | 设为 `logical` 才能逻辑解码 | **restart** |
| `max_replication_slots` | 复制槽数量上限 | **restart** |
| `max_wal_senders` | WAL 发送进程数上限，要 ≥ 槽数否则发不出 | **restart** |

**推荐用 `ALTER SYSTEM`**（写入数据目录的 `postgresql.auto.conf`，干净可追溯）：

```sql
-- 查看当前值
SHOW wal_level;             -- 默认 replica
SHOW max_replication_slots; -- 默认 10
SHOW max_wal_senders;       -- 默认 10

-- 修改
ALTER SYSTEM SET wal_level = 'logical';
ALTER SYSTEM SET max_replication_slots = 30;
ALTER SYSTEM SET max_wal_senders = 30;
```

#### slot / sender 数量怎么定——别拍脑袋写 15

通用逻辑复制经验是「一个订阅占 1 个 slot」，但 **DTS 整库迁移的并发模型不同**：迁移工具对**每个待迁移 DB** 通常建 **2 个复制连接**（全量 + 增量 / 并行槽）。所以工具的校验公式是：

```
max_wal_senders ≥ 已使用数 + 待迁移 DB 数 × 2
max_replication_slots 同理（每个 DB 也要占槽）
```

实例：已用 0、迁移 11 个 DB → 最小值 `0 + 11×2 = 22`。设 15 会被校验拦下（报「源实例参数不符合要求」）。按工具「尽量多预留」的提示，取 **30**（22 + 余量），避免后续加库又要重启。

> 🔑 教训：slot/sender **不是固定值**，要按「已用 + 库数 × 2」算再留余量。低估会卡在 DTS 预检环节。

#### Docker 部署：restart 还是 recreate？

**只要配置落在 volume 持久化的数据目录，`docker restart` 就够，不用 recreate、不丢数据。** 判断依据是「配置改在哪」：

| 改配置的方式 | 配置位置 | 操作 |
|-------------|---------|------|
| `ALTER SYSTEM SET ...` | 数据目录 `postgresql.auto.conf`（在 volume 内） | `docker restart` ✅ |
| 进容器改 `postgresql.conf` | 数据目录（在 volume 内） | `docker restart` ✅ |
| 改宿主机挂载的 conf 文件 | 宿主机文件（已挂载） | `docker restart` ✅ |
| 改 compose 的 `command:` / `-c` 启动参数 | 容器定义本身 | `docker compose up -d`（recreate，volume 保留） |
| 改 `environment:`（`POSTGRES_*`） | 仅首次初始化生效 | 改了无效，需重建数据 ⚠️ |

> 核心：配置在 **volume / 宿主机挂载文件** → 进程重启即重新加载，`restart` 即可；只有改 **容器定义（command/env/ports）** 才需要 recreate。

实操（`ALTER SYSTEM` + restart 路径）：

```bash
docker exec -it <容器名> psql -U postgres -c "ALTER SYSTEM SET wal_level = 'logical';"
docker exec -it <容器名> psql -U postgres -c "ALTER SYSTEM SET max_replication_slots = 30;"
docker exec -it <容器名> psql -U postgres -c "ALTER SYSTEM SET max_wal_senders = 30;"
docker restart <容器名>
docker exec -it <容器名> psql -U postgres -c "SHOW wal_level;"  -- 验证应为 logical
```

> ☁️ 云数据库（RDS / 阿里云等）通常禁用 `ALTER SYSTEM`，改在控制台参数组里设 `wal_level` / `max_replication_slots` / `max_wal_senders`，再重启实例。

### 4.2 REPLICA IDENTITY

没有主键的表，UPDATE/DELETE 默认无法逻辑解码出"改的是哪一行"（只记了新值，没记定位旧行的键）。要设置：

```sql
ALTER TABLE 表名 REPLICA IDENTITY FULL;  -- 把整行旧值都记进 WAL
```

有主键的表默认用主键定位，无需处理。

### 4.3 复制槽（replication slot）——最危险的坑

**作用**：给下游消费者记一个书签——"我已消费到 LSN xxx"。PG 看到书签，就知道这个 LSN 之前的 WAL 还有人没消费，**不能回收**，于是保留。

正常情况：DTS 不断消费、推进书签，旧 WAL 正常清理。

**出事场景：消费停了但书签还在**
- DTS 任务暂停 / 挂了 / 被强删，但 slot 没删
- 书签永远停在旧 LSN
- 之后产生的所有 WAL 都被判定"还没人消费"，**全部保留**
- 主库继续写 → WAL 越堆越多 → **磁盘被撑爆 → 源库宕机**

生产里"磁盘莫名其妙满了"很多就是死掉的 slot 导致的。

**要不要删 slot，取决于场景：**

| 场景 | 处理 |
|------|------|
| 迁移**彻底完成**，DTS 不再需要 | **要删**，释放 WAL |
| DTS **临时暂停**，稍后还要继续 | **别删**，删了书签 = 丢断点，恢复时可能要重新全量 |
| DTS 任务**已废弃/删除**但 slot 残留 | **要删**，这就是会堆积的死槽 |

关键：**slot 是用来"续传"的，只要还打算让这个任务继续增量同步，就不能删。删 slot = 放弃断点。**

> 正常在 DTS 控制台**结束/释放**任务，DTS 通常会自己回头清理源库 slot，不用手动操作。需要手动上源库删的是**异常情况**（任务被强删、报错卡死、排查磁盘堆积）。

**排查与删除：**

```sql
SELECT
  slot_name,
  active,                          -- f(false) = 当前无消费者连着，大概率死槽
  restart_lsn,                     -- 书签卡在哪
  pg_size_pretty(
    pg_wal_lsn_diff(pg_current_wal_lsn(), restart_lsn)
  ) AS retained_wal                -- 这个槽导致保留了多少 WAL（堆积量）
FROM pg_replication_slots;

-- 确认是死槽（active=false 且 retained_wal 大 且 对应任务已废弃）后再删：
SELECT pg_drop_replication_slot('槽名');
```

> ⚠️ 删之前务必确认对应 DTS 任务确实废弃。删活槽 / 删临时暂停任务的槽 → 下游同步中断、断点丢失。

### 4.4 其他

- **权限**：执行迁移账号需 `REPLICATION` 权限（云上是对应高权限账号）。
- **写放大**：`logical` 比 `replica` 写更多 WAL，写密集主库有额外开销，迁移期留意 IO。
- **迁移后是否降回 replica**：若库只做主备高可用、不再需要逻辑复制，可降回 `replica` 省写放大。但降级同样要重启，且要先确认没有 slot 还在用。很多团队图省事一直留 logical，影响通常不大，看对写性能的敏感度。

---

## 5. 存储放大（RUM 猜想）

**放大（amplification）= 逻辑上想做 1 单位操作，系统底层实际付出 N 倍代价，N 即放大倍数。**

三种放大常一起讨论，业界称 **RUM 猜想**：Read（读）、Update（写）、Memory（空间）三者不可兼得，优化一个往往恶化另一个。

### 5.1 写放大 (Write Amplification)

**定义**：逻辑写入量 vs 物理实际写入量的比值。改一行 100 字节，底层可能实际写几 KB~几十 KB。

**PostgreSQL 里的来源：**
1. **WAL 写入**：改数据前先写 WAL，同一份变更天然写两遍（WAL + 数据文件）。`logical` 比 `replica` 记更多 → WAL 写放大更大。
2. **Full Page Writes（整页写）**：默认开启。一个数据页在 checkpoint 后**第一次被修改**时，把**整个 8KB 页**完整写进 WAL（防页面撕裂）。改 100 字节却记 8KB —— PG 写放大的大头。
3. **HOT 更新 vs 索引更新**：MVCC 下"更新 = 插新版本行 + 标记旧行死亡"。若更新列被索引覆盖，每个索引都要新增指向新版本的条目。改一行 → 一份数据 + N 份索引。
4. TOAST、checkpoint 刷脏页等也贡献额外写。

**LSM-Tree 系统更严重（RocksDB / Cassandra / MyRocks）：**
- 数据先写内存 memtable，刷成磁盘 SSTable
- 后台 **Compaction** 反复把小文件合并成大文件，同一份数据生命周期里被反复读出、合并、重写
- 写放大可达 **10~30 倍**

**危害：**

| 危害 | 说明 |
|------|------|
| 磁盘 IO 带宽被吃掉 | 实际写入 N 倍，提前到 IO 瓶颈，吞吐上不去 |
| SSD 寿命缩短 | 闪存有擦写次数上限（P/E cycle），写放大 10 倍 ≈ 盘提前 10 倍报废 |
| 延迟抖动 | 后台 compaction/checkpoint 抢 IO，前台 p99 飙高 |
| WAL/磁盘堆积 | 写越多 WAL segment 越多，配合死 slot 更易撑爆磁盘 |

> SSD 硬件层还有一层写放大（擦除以块为单位、写入以页为单位）。软件写放大 × 硬件写放大，叠加。

### 5.2 读放大 (Read Amplification)

**定义**：逻辑读 1 行，底层实际读了多少数据 / 访问多少次磁盘。

**来源：**
- **B-Tree（PostgreSQL / InnoDB）读放大相对小**：走索引一次读 树高(2~4) 个页即可定位。但也有来源：读一行要读整个 8KB 页；MVCC 顺版本链找可见版本；表膨胀时扫过一堆 dead tuple。
- **LSM-Tree 读放大严重**：一个 key 可能散在 memtable + 多层 SSTable，要逐层找，靠 **Bloom Filter**（快速判断"这文件里肯定没这 key"）压低读放大。

**危害：** 查询延迟高（多次磁盘 IO）；缓存效率低（读进无用数据挤占 buffer pool）；CPU 开销（解压、比较、过滤无效版本）。

### 5.3 空间放大 (Space Amplification)

**定义**：实际占用磁盘 vs 真实有效数据的比值。

- PG 里就是**表膨胀（bloat）**：MVCC 留下的 dead tuple 没被 `VACUUM` 回收，逻辑 1GB 的表物理占 3GB。
- LSM 里是同一 key 的多个旧版本散在不同层，还没被 compaction 清理。

危害：浪费磁盘、降低缓存命中、加重读放大。

### 5.4 三者的根本矛盾（RUM 猜想）

**读放大、写放大、空间放大，通常不能同时最优，优化一个往往牺牲另一个。** 这是存储引擎设计与选型的本质。

| 架构 | 写放大 | 读放大 | 空间放大 | 取向 |
|------|--------|--------|---------|------|
| **B-Tree**（PostgreSQL/InnoDB） | 中 | **低** | 中 | 读友好，适合 OLTP 读多 |
| **LSM-Tree**（RocksDB/Cassandra） | **高** | 高（靠 Bloom Filter 救） | 中 | 写友好，适合写密集 |

- 写多读少（日志、时序、IoT）→ 选 LSM，容忍写/读放大换高写吞吐
- 读多写少（典型业务库）→ B-Tree 系（PostgreSQL）更合适

---

## 6. 回到 DTS 迁移：这些原理如何叠加成风险

迁移链路上的风险其实是几条线叠加：

1. **`wal_level` replica → logical**：每次写记更多逻辑解码信息，WAL 写放大增加。
2. **逻辑解码本身**：DTS 读 WAL 解码成行变更，额外的读 + CPU。
3. **死掉的 replication slot**：WAL 不回收，写放大产生的量全堆在磁盘上，放大"撑爆磁盘"风险。

所以迁移期间要盯紧源库的 **磁盘使用率** 和 **WAL 生成速率**——`logical 多记信息（写放大↑）` + `slot 不清理（WAL 不回收）` 是两个叠加的磁盘风险因素。

---

## 附录：自我思考

- **一条主线串多个概念**：这次梳理的价值不在单个知识点，而在"WAL → wal_level → 复制类型 → slot → 放大"能用一条因果链串起来。底层知识最怕碎片化，能串成链才算真懂。
- **概念区分优先于细节**：物理复制 vs 逻辑复制是整条线里最容易错的地方（我一开始就把主从该用什么级别搞反了）。先把"两种复制是两套独立体系"这个框架立住，下面的 slot、放大才不会乱。
- **RUM 是通用透镜**：读/写/空间放大不只属于 PostgreSQL，它是看待**任何**存储引擎（数据库、KV、文件系统、SSD 固件）的通用视角。以后选型、调参、排查性能问题，都可以先问"这是在 RUM 三角里做什么权衡"。
- **可延伸的坑**：本篇没深入但值得后续单独挖的——VACUUM 与表膨胀治理、checkpoint 调优、Full Page Writes 与 `wal_compression`、LSM 的 compaction 策略（leveled vs tiered）、Bloom Filter 原理。
