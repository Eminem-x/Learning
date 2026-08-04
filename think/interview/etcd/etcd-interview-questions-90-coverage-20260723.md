# etcd 面试题库（90% 覆盖率 · 白板级）

> 来源：个人思考 + 学习（对标《四年后端 + 轻量大数据能力地图》第 8 格：分布式系统 / 共识；兼跨第 7 格存储引擎、第 9 格微服务基础设施）
> 收录时间：2026-07-23 CST
>
> 说明：本篇是**综合入口 + 抽考清单**，覆盖 etcd 面试约 90% 的高频考点。体例对齐 [mysql 面试题库(广度)](../mysql/mysql-interview-questions-90-coverage-20260704.md) 与 [redis 面试题库(广度)](../redis/redis-interview-questions-90-coverage-20260709.md)：每个考点给「一句话答案 + 追问链条」，追到白板级。版本以 **etcd v3.5 / v3.6** 为基线，涉及 3.4 及更早的差异会标注。
>
> ⚠️ 面试里 etcd 很少单独考「怎么用」，**它是「Raft + MVCC + Watch + 租约」这四块通用原理的最佳载体**——面试官问 etcd，八成是在问共识、线性一致性、分布式锁、K8s 底层。答的时候要有意识地把话往原理上引。

---

## 30 秒电梯版（盖住全文也要能背出主线）

etcd 面试就八条主线：

1. **定位**：**CP 型**、强一致的**分布式元数据存储**（不是缓存、不是通用数据库）。数据量小（GB 级）、读多写少、一致性优先。K8s 的唯一数据源就是它。
2. **共识 = Raft**：Leader 选举（任期 term + 随机选举超时）、日志复制（AppendEntries + 多数派 commit）、安全性（选举限制保证已提交日志不丢）。**写请求必须过 Raft，多数派落盘才返回**。
3. **存储 = MVCC + bbolt**：全局单调递增的 **revision** 是版本主线；内存里 **treeIndex（B 树）** 存 `user key → revision 列表`，磁盘上 **bbolt（B+ 树）** 存 `revision → KeyValue`。写是**追加新 revision**，不覆盖旧值 → 才能支持「读历史版本 / Watch 历史事件」。
4. **持久化两层**：**WAL**（Raft 日志，写前顺序追加 + fsync，决定写延迟）+ **snapshot**（状态机快照，用来截断 WAL）。这和 MySQL redo log、PG WAL 是同一套思想。
5. **线性一致读 = ReadIndex**：读不写日志，但 Leader 要先用心跳向多数派**确认自己还是 Leader**，拿到当前 commitIndex，等本地 applyIndex 追上再读。想快可以退化成 `serializable`（本地读、可能读到旧值）。
6. **Watch**：基于 revision 的**流式**事件订阅，从任意历史 revision 开始，有序、不丢、事务原子；唯一会丢的场景是**请求的 revision 已被 compact**（`ErrCompacted`）。这是 K8s list-watch 的地基。
7. **Lease 租约**：带 TTL 的对象，key 挂在 lease 上，lease 过期 → 挂它的 key 全删。**租约 + Watch + Txn = 分布式锁 / 服务注册 / 选主**的全部原材料。
8. **运维红线**：奇数节点（3/5）；**磁盘 fsync 延迟就是 etcd 的命**（慢盘 → 心跳超时 → Leader 反复切换）；db 有 **quota（默认 2GB）**，超了整个集群**只读**；要定期 **compact（清历史 revision）+ defrag（还空间给 OS）**。

卡在哪一条，就往对应章节回挖。

---

## 一、定位与整体架构（考频 ★★★★★）

### Q1. etcd 是什么？解决什么问题？什么场景该用、什么场景不该用？

**一句话**：etcd 是一个基于 **Raft** 的、**强一致（CP）**、支持 **Watch** 和 **Lease** 的**分布式键值存储**，定位是**集群的元数据 / 配置 / 协调中心**（"the single source of truth"）。

**典型场景（都是「小数据 + 强一致 + 要通知」）**：

| 场景 | 用到的能力 |
|------|-----------|
| **服务注册发现** | Lease（心跳续约）+ Watch（实例上下线通知）+ 前缀查询 |
| **配置中心** | KV + Watch（配置热更新） |
| **分布式锁 / 选主** | Txn（CAS）+ Lease（自动释放）+ Watch（等锁） |
| **K8s 全部对象存储** | MVCC + Watch（list-watch）+ 线性一致读 |
| **分布式协调**（任务分配、Leader 唯一性、幂等标记） | Txn + Lease |

**不该用**：
- ❌ **当缓存 / 高 QPS 读写库**：每次写都要过 Raft + 多数派 fsync，写 QPS 只有**万级甚至千级**，比 Redis 差两个数量级。
- ❌ **存大数据 / 大 value**：默认 db quota 2GB（推荐上限 8GB），单 value 建议 < 1.5MB（`--max-request-bytes` 默认 1.5MB）。
- ❌ **当消息队列**：Watch 是「状态变更通知」不是「消息投递」，没有消费位点管理、没有堆积能力（历史事件会被 compact 掉）。

> **面试口径**：「etcd 是 **CP** 的元数据存储，用一致性换吞吐；Redis 是 **AP** 的高性能缓存，用一致性换吞吐的反面。选型第一问是**这份数据丢一条能不能忍**——不能忍就 etcd。」

### Q2. etcd 的整体架构 / 一次写请求的完整链路？（必背，白板题）

分四层，从上到下：

```
Client (gRPC)
   │
┌──┴──────────────────────────────────────────────┐
│ ① API 层：gRPC Server (KV / Watch / Lease / Auth │
│    / Cluster / Maintenance) + gRPC Gateway(HTTP) │
├─────────────────────────────────────────────────┤
│ ② Server 层：EtcdServer                          │
│    - 权限校验、配额检查(quota)                    │
│    - 提交 proposal 给 Raft                        │
│    - apply 已提交的日志到状态机                    │
├─────────────────────────────────────────────────┤
│ ③ Raft 层：raft 库(纯状态机, 无 IO) + raftNode     │
│    - 选举、日志复制、成员变更                      │
│    - 输出 Ready{要持久化的entries, 要发的msg,      │
│      已commit的entries}                          │
├─────────────────────────────────────────────────┤
│ ④ 存储层                                          │
│    - WAL（Raft 日志，顺序追加 + fsync）            │
│    - Snapshot（状态机快照，截断 WAL）              │
│    - MVCC: treeIndex(内存B树) + backend bbolt(B+树)│
└─────────────────────────────────────────────────┘
```

**一次 `Put` 的 8 步（面试高频，能背下来直接加分）**：

1. **client → 任意节点**：gRPC `Put`。若打到 Follower，Follower 把请求**转发给 Leader**（`raftHttp` 转发 proposal）。
2. **Leader 做前置检查**：鉴权、`quota` 检查（db 是否超限、是否有 NOSPACE alarm）。
3. **Leader 发起 Raft proposal**：把这条写操作序列化成一条 **Raft 日志 entry**，append 到自己的 raft log。
4. **并行两件事**：① 本地把 entry 写 **WAL 并 fsync**；② 通过 `AppendEntries` **广播给 Follower**。
5. **Follower** 收到后同样「写 WAL + fsync」，成功则回 ACK。
6. **多数派（quorum）ACK** → Leader 推进 **commitIndex**，这条日志变成 **committed**（此刻数据已经「不会丢」了，但还没生效）。
7. **apply 到状态机**：Leader 把 committed entry 应用到 **MVCC 层**——分配新 revision、写 treeIndex（内存）、写 bbolt（先进内存 buffer，**默认 100ms 批量 commit 一次**）。同时通过 `AppendEntries` 的下一轮把 commitIndex 带给 Follower，Follower 也 apply。
8. **返回客户端成功**。

> **两个关键分界，追问必问**：
> - **committed ≠ applied**：多数派落 WAL 就 committed（持久性已保证），apply 是之后的事。所以「写成功后立刻在同节点读」需要 **ReadIndex 等 applied 追上**（见 Q17）。
> - **写延迟 = WAL fsync + 一轮 RTT**。所以 **etcd 的性能天花板是磁盘 fsync 和网络 RTT**，不是 CPU。这是所有 etcd 调优的出发点。

### Q3. etcd v2 和 v3 有什么区别？（老题但常问）

| 维度 | v2 | v3 |
|------|----|----|
| **数据模型** | 目录树（类似 ZK znode），内存中 | **扁平 KV**，靠 **range（前缀）** 模拟目录 |
| **协议** | HTTP/1.1 + JSON | **gRPC + Protobuf**（同时提供 gRPC-Gateway 兼容 HTTP） |
| **持久化** | 只有 WAL + snapshot，全量数据在内存 | **bbolt 落盘** + MVCC，内存只放索引 |
| **版本** | 每个 key 一个 index，无历史 | **全局 revision + MVCC**，可读历史版本 |
| **Watch** | HTTP long polling，**一次性**，易丢事件 | **gRPC stream**，可从历史 revision 开始，**不丢事件** |
| **TTL** | 每个 key 独立 TTL，靠客户端不断刷新 | **Lease**：多 key 共享一个租约，`KeepAlive` 流式续约 |
| **事务** | 只有 CAS/CAD 单 key | **Txn**：多 key 的 `If-Then-Else` 迷你事务 |
| **连接** | 每个 watch 一条 HTTP 连接 | **一条 gRPC 连接多路复用**多个 watch |

> **一句话**：v3 是为「**K8s 这种要 list-watch 海量对象**」重写的——批量、多路复用、历史版本、租约共享，全是为了降低 watch 的连接和内存开销。v2 API 在 3.5 已废弃、3.6 移除。

---

## 二、Raft 共识（考频 ★★★★★，重中之重）

### Q4. 一句话讲清 Raft？它把共识拆成哪三块？

Raft = **"可理解版的 Paxos"**，核心是**强 Leader**：所有写都从 Leader 走，日志只从 Leader 流向 Follower（**append-only，单向**）。拆成三个子问题：

1. **Leader 选举（Leader Election）**：谁当老大。
2. **日志复制（Log Replication）**：老大怎么把日志同步给大家。
3. **安全性（Safety）**：怎么保证「已提交的日志永不丢失、各节点状态机执行序列完全一致」。

**三种角色**：`Follower`（被动）→ `Candidate`（拉票中）→ `Leader`（干活）。
**核心变量**：`term`（任期，逻辑时钟，单调递增）、`commitIndex`（已提交位置）、`applyIndex`（已应用位置）。

> **术语对齐**：`term` 相当于「第几届」，每次选举 +1。**任何 RPC 都带 term，看到更大的 term 就无条件退回 Follower 并更新自己的 term**——这一条是 Raft 的「宪法」，绝大多数脑裂问题靠它自动化解。

### Q5. Leader 选举的完整过程？为什么选举超时要随机？

**过程**：
1. Follower 在 `election timeout`（etcd 默认 1000ms）内没收到 Leader 心跳 → 认为 Leader 挂了。
2. 自增 `term`，转为 **Candidate**，**先投自己一票**，并行向所有节点发 `RequestVote`。
3. 其他节点的投票规则（**每个 term 只能投一票**，先到先得）：
   - 请求的 `term` ≥ 自己的 term；
   - 本 term 还没投过票；
   - **候选人的日志至少和自己一样新**（见 Q7 的选举限制）。
4. 拿到**多数派选票** → 成为 Leader，立即广播心跳（空 `AppendEntries`）压制其他候选人。
5. 若超时仍未选出（选票被瓜分 split vote）→ term 再 +1，重新选举。

**为什么随机化选举超时？**
如果所有节点超时时间一样，Leader 一挂它们会**同时**变 Candidate、同时投自己 → 谁也拿不到多数派 → **反复瓜分选票，活锁**。随机化（etcd 取 `[electionTimeout, 2*electionTimeout)`）让某个节点大概率**先**超时、抢跑成功，**一轮就选出**。这是 Raft 里非常典型的「用随机性打破对称性」。

### Q6. 日志复制怎么做？commit 的条件是什么？

- Leader 收到写请求 → append 到本地 log（此时 **uncommitted**）→ 并行发 `AppendEntries` 给所有 Follower。
- `AppendEntries` 携带 **`prevLogIndex` / `prevLogTerm`**：Follower 检查自己在 `prevLogIndex` 处的 term 是否匹配，**不匹配就拒绝** → Leader 把 `nextIndex` 往回退，重试，直到找到双方**最后一致的位置**，然后用 Leader 的日志**覆盖**后面的部分。这叫 **日志一致性检查（Log Matching Property）**。
- **commit 条件**：该 entry 被**复制到多数派**，且**该 entry 属于当前 term**（关键！见下）。commitIndex 推进后，Leader 通过后续 RPC 告知 Follower。
- **apply**：commitIndex > applyIndex 的部分依次应用到状态机（etcd 里 = 写 MVCC）。

> **追问：为什么「只能提交当前 term 的日志」？（Raft 论文 Figure 8，高阶题）**
> 如果允许 Leader 通过「已复制到多数派」直接提交**前任 term** 的日志，可能出现：某条旧日志复制到多数派后被认为已提交，但后来一个日志更新的节点当选，把它覆盖掉 → **已提交的日志被回滚**，违反安全性。
> Raft 的解法：新 Leader 上任先追加一条**本 term 的空日志（no-op entry）**，这条一旦提交，它之前的所有日志**间接提交**（Log Matching 保证）。etcd 的 raft 库正是这么干的——**这也是为什么 Leader 切换后有一小段时间读不到最新数据**：要等 no-op 提交并 apply。

### Q7. Raft 靠什么保证「已提交的日志永不丢」？（安全性五条）

面试答**选举限制 + 状态机安全**两条即可，能背五条更好：

1. **Election Safety**：一个 term 内最多一个 Leader（多数派投票 + 每 term 一票）。
2. **Leader Append-Only**：Leader 从不删改自己的日志，只追加。
3. **Log Matching**：若两个日志在某 index 的 term 相同，则该 index 之前的所有日志**完全相同**。
4. **Leader Completeness（选举限制，核心）**：只有**日志足够新**的候选人才能当选。比较规则：先比**最后一条日志的 term**，term 大的更新；term 相同比 **index**，index 大的更新。→ 保证任何已提交的日志一定存在于新 Leader 上。
5. **State Machine Safety**：所有节点在同一 index 应用的是同一条日志 → 状态机确定性一致。

> **白板问法**：「已经返回客户端成功的写，Leader 立刻宕机，会丢吗？」
> 答：**不会**。返回成功意味着已被多数派持久化（WAL fsync）。新 Leader 必须拥有全部已提交日志（选举限制），所以这条日志一定在新 Leader 上，会被继续提交并 apply。
> 反过来，**没返回成功的写可能丢，也可能"幽灵般"成功**——客户端超时不代表没写进去，所以**业务侧必须做幂等**。

### Q8. Raft 和 Paxos、ZAB 有什么区别？为什么大家都选 Raft？

| | Paxos（Multi-Paxos） | **Raft** | ZAB（ZooKeeper） |
|--|---------------------|----------|------------------|
| **可理解性** | 极难，论文到工程有巨大 gap | **易懂**（设计目标就是可理解） | 中等 |
| **Leader** | 可无 Leader / 弱 Leader，日志可乱序空洞 | **强 Leader**，日志连续无空洞、单向流动 | 强 Leader |
| **日志** | 允许空洞，需额外填补 | **不允许空洞** | 不允许空洞 |
| **成员变更** | 需自行设计 | 内建（单节点变更 / joint consensus） | 需重启或 3.5+ 动态重配置 |
| **恢复方式** | 复杂 | 靠日志一致性检查回退 | 崩溃恢复阶段同步（TRUNC/DIFF/SNAP） |

**Raft 赢在工程化**：强 Leader + 无空洞日志把状态空间压得很小，实现和调试都简单；etcd/TiKV/CockroachDB/Consul 全用它。**本质上 Raft ≈ 加了约束的 Multi-Paxos**，一致性能力等价，可理解性和可实现性大胜。

### Q9. PreVote 是什么？解决什么问题？

**问题**：一个被网络分区隔离的 Follower，收不到心跳 → 不断超时、不断 `term++` 拉票（虽然永远选不上）。等网络恢复，它带着一个**远高于集群的 term** 回来 → 按「见更大 term 就退位」的规则，**正常工作的 Leader 被迫下台**，集群无谓地重新选举一次（可能反复）。这叫 **term 膨胀 / 网络分区干扰**。

**PreVote（预投票）**：正式选举前先发一轮 **不自增 term** 的试探性拉票——"如果我现在选，你们会投我吗？"。只有拿到多数派的「会」，才真正 `term++` 发起选举。被隔离的节点永远拿不到多数派响应 → **term 永远不涨** → 回归时不会打扰集群。

> **版本**：etcd v3.4 引入 `--pre-vote`（默认关闭），**v3.5 起默认开启**。配套的还有 **CheckQuorum**：Leader 若在一个选举周期内联系不上多数派，主动退位，避免「僵尸 Leader」继续对外声称自己是主。

### Q10. Raft 怎么做成员变更？为什么不能直接改配置？

**危险在哪**：如果所有节点「同时瞬间」从旧配置 `Cold` 切到新配置 `Cnew` 是做不到的（切换有先后），中间可能出现**两个不相交的多数派** → **同一 term 选出两个 Leader → 脑裂**。经典例子：3 节点扩到 5 节点，切换过程中 {A,B}（旧配置多数派）和 {C,D,E}（新配置多数派）可以各自选出 Leader。

**两种解法**：
1. **单节点变更（one-at-a-time，etcd 默认）**：一次只加/减**一个**节点。数学上可证明：旧多数派和新多数派**必然有交集** → 不可能同时选出两个 Leader。简单，是 etcd 实际使用的方式。扩容 3→5 要做两次（3→4→5）。
2. **联合共识（Joint Consensus, `ConfChangeV2`）**：引入过渡配置 `Cold,new`，该阶段的任何决议需要**同时**在新旧两个配置里各拿到多数派 → 天然不可能脑裂。支持一次性替换多个节点。etcd 的 raft 库支持，主要给 TiKV 这类需要批量调度的场景用。

> **实操坑**：加节点要**先 `member add`（此时集群进入「4 节点、quorum=3」的脆弱状态）再启动新节点**。如果原来是 3 节点且已挂 1 台，此时再 `member add` 会让 quorum 从 2 涨到 3 → **集群直接不可用**。所以「先修复再扩容」。这就是 **Learner** 的由来。

### Q11. Learner（非投票成员）是什么？解决什么问题？

**Learner（v3.4+）** = 只接收日志、**不参与投票、不计入 quorum** 的成员。

解决三个痛点：
- **新节点追数据期间拖垮集群**：新加的节点要从头同步几个 GB，期间它响应慢，若算进 quorum 会拉高写延迟甚至导致选举抖动。Learner 不计入 quorum，追数据不影响集群。
- **误操作把集群拖进不可用**：如上一题的「加节点导致 quorum 上升」。Learner 不改变 quorum。
- **异地只读副本**：跨机房放 Learner 做灾备/只读，不拖累主集群的写延迟（不参与投票 → 不需要等它 ACK）。

追平后用 `etcdctl member promote` 提升为正式成员。

### Q12. Raft 日志会无限增长吗？snapshot 怎么做？

不会——靠 **snapshot（快照）截断日志**：

- 状态机数据到达一定量后（etcd `--snapshot-count`，**v3.4+ 默认 100000 条 entry**，3.4 之前是 10000），把当前状态机做一份快照，然后**丢弃快照点之前的 Raft 日志**（WAL 文件也可以回收）。
- **落后太多的 Follower**：如果 Follower 需要的 entry 已被 Leader 截断，Leader 就发 **`InstallSnapshot`** RPC，直接传整个快照，Follower 加载后从快照点继续跟。
- **etcd 的特殊之处**：因为数据本来就落在 **bbolt** 里（不是纯内存状态机），所谓 snapshot 主要是**记录一个 apply 位点 + 保留 bbolt 数据**，加上 `--max-wals`/`--max-snapshots`（默认各 5）控制文件个数。

> **注意区分三个"快照"**（超易混，面试栽跟头的点）：
> - **Raft snapshot**：状态机快照，用于**截断 Raft 日志 / 追平落后节点**。
> - **`etcdctl snapshot save`**：**备份**用的 db 文件拷贝（本质是 bbolt 文件的一致性副本）。v3.6 起该子命令移到 **`etcdutl`**。
> - **MVCC compaction**：清理**历史 revision**，和上面两个都不是一回事（见 Q16）。

---

## 三、存储引擎：MVCC + bbolt + WAL（考频 ★★★★★）

### Q13. etcd 的 MVCC 是怎么实现的？revision 是什么？

**核心：revision 是全局单调递增的逻辑版本号，是整个 etcd 的时间轴。**

- **revision 是二元组 `(main, sub)`**：`main` 每个**事务** +1，`sub` 是事务内第 N 个操作。所以一个 Txn 里的多个写共享同一个 main revision → **天然的原子可见性**。
- **写不覆盖，只追加**：`Put k=v2` 不会覆盖 `k=v1`，而是在新 revision 下写一条新记录。旧版本仍在磁盘上，直到被 compact。

**每个 KeyValue 上的四个版本字段（必背）**：

| 字段 | 含义 | 用途 |
|------|------|------|
| **`create_revision`** | 该 key **本轮生命周期**首次创建时的 revision | **分布式锁排队**（谁的 create_revision 最小谁先来）、判断 key 是否新建 |
| **`mod_revision`** | 该 key **最后一次修改**时的 revision | **CAS 乐观锁**的比较基准 |
| **`version`** | 该 key 本轮生命周期内被修改的**次数**（创建时 =1，删除后重建归 1） | 判断「有没有被改过」 |
| **`revision`（全局）** | 集群当前的全局版本 | Watch 起始点、一致性读的水位线 |

**两个索引层（白板必画）**：

```
用户 key "foo"
      │
      ▼  ① treeIndex（内存中的 B 树，Google btree）
   keyIndex{ key: "foo",
             generations: [ {rev: 5,8,12(tombstone)},   ← 第一轮生命周期
                            {rev: 20, 23}  ]}           ← 删除后重建，新 generation
      │  查到「foo 在 rev≤N 时的最新版本是 rev=8」
      ▼  ② backend bbolt（磁盘 B+ 树）
   key = "8_0"(revision 编码)  →  value = pb.KeyValue{key:"foo", value:"...", ...}
```

- **读当前值**：treeIndex 查 `foo` 的最大 revision → 拿这个 revision 去 bbolt 取 value。**一次内存查 + 一次磁盘（page cache）查**。
- **读历史值**：`get foo --rev=9` → treeIndex 里找 ≤9 的最大 revision（=8）→ 取值。
- **删除**：写一个 **tombstone（墓碑）** revision，并在 treeIndex 里结束当前 generation。所以删除也是一次写、也占空间。

> **对照记忆**：这套 `内存索引 → 版本号 → 磁盘版本链` 的结构，和 MySQL InnoDB 的 `聚簇索引 + undo 版本链 + ReadView` 是同构的，区别是 **InnoDB 的旧版本在 undo log 里靠 purge 清理，etcd 的旧版本在主存储里靠 compact 清理**。参见 [MVCC×隔离级别×幻读](../mysql/mysql-mvcc-isolation-phantom-read-20260710.md)。

### Q14. 底层为什么用 bbolt（B+ 树）而不是 LSM-Tree？

**bbolt** 是 BoltDB 的 etcd 维护分支：单文件、mmap、**B+ 树**、写时复制（COW）、单写多读、支持 ACID 事务。

**为什么选它（对标能力地图第 7 格 RUM 权衡）**：

| 维度 | B+ 树（bbolt） | LSM-Tree（RocksDB） |
|------|---------------|---------------------|
| **读放大** | **低**，一次树查找到底 | 高，可能查多层 SST + bloom |
| **写放大** | 高（页级 COW，改 1 字节写整页） | **低**（顺序追加），但 compaction 有后台放大 |
| **范围扫描** | **好**（叶子有序、天然聚簇） | 好但需归并多层 |
| **实现复杂度** | **简单**、无后台线程、行为可预测 | 复杂，compaction 抖动难控 |

**etcd 的选择理由**：它是**读多写少、数据量小（GB 级）、要求延迟稳定可预测**的元数据库。B+ 树的写放大在 GB 级数据下完全可以接受，而**「没有后台 compaction 抖动」对一个要保证心跳及时的共识系统极其重要**——LSM 的 compaction stall 可能直接导致 etcd 心跳超时、触发选举。

> **追问「那 TiKV 为什么用 RocksDB？」**：TiKV 是**通用分布式数据库**，TB~PB 级数据、写密集，写放大是主要成本 → LSM 更合适。**同样是 Raft，存储引擎选型跟着数据量和读写比走**，这就是 RUM 权衡的现场教学。

### Q15. WAL 在 etcd 里的角色？和 MySQL redo log / PG WAL 一样吗？

**思想完全一样：先写日志再改数据，用顺序 IO 换随机 IO，用日志保证崩溃可恢复。** 但对象不同：

| | MySQL redo log | PG WAL | **etcd WAL** |
|--|---------------|--------|-------------|
| **记录什么** | 物理页变更 | 物理/逻辑混合 | **Raft 日志 entry**（客户端请求本身）+ HardState(term/vote/commit) |
| **谁读它** | 崩溃恢复 | 恢复 + 流复制 | **崩溃恢复 + 复制给 Follower** |
| **fsync 时机** | `innodb_flush_log_at_trx_commit` | `synchronous_commit` | **每次 append 必 fsync**（Raft 安全性硬要求，不可关） |

**关键差异**：etcd 的 WAL **不能不 fsync**——Raft 的正确性建立在「回 ACK 就意味着已持久化」之上。如果 Follower 回 ACK 但数据只在 page cache，机器断电后这条「已提交」的日志就消失了 → **违反 Leader Completeness → 已提交数据丢失**。

> 所以 **etcd 的写延迟下限 = 一次磁盘 fsync + 一轮网络 RTT**。监控上就是 `etcd_disk_wal_fsync_duration_seconds`（**P99 应 < 10ms**，超过 25ms 就该报警了）。这也是官方**强烈要求 SSD**、且反对把 etcd 和其他 IO 密集服务混布的根本原因。

### Q16. compact 和 defrag 有什么区别？为什么必须做？

这是 etcd 运维**最高频事故源**，两个词经常被搞混：

| | **compact（压缩）** | **defrag（碎片整理）** |
|--|---------------------|----------------------|
| **干什么** | 删除 revision ≤ N 的**历史版本**（保留每个 key 在 N 时的最新值） | 把 bbolt 文件里的**空闲页**整理掉，**把空间还给文件系统** |
| **作用对象** | 逻辑数据（MVCC 版本链） | 物理文件 |
| **效果** | db 里的**逻辑数据变少**，但**文件大小不变**（空间进入 bbolt 的 freelist 复用） | **文件真正变小** |
| **代价** | 相对轻量 | **阻塞该节点读写**（重建整个文件），GB 级要数秒~数十秒 |
| **怎么做** | `etcdctl compact <rev>`；或自动：`--auto-compaction-mode=periodic --auto-compaction-retention=1h` | `etcdctl defrag`（**必须逐节点滚动执行，不能同时**） |

**为什么必须做**：不 compact → 历史 revision 无限堆积 → db 撑爆 quota → **NOSPACE alarm → 整个集群拒绝写入，只能读**（这是 etcd 最经典的线上事故）。
只 compact 不 defrag → 文件大小不降（虽然空间可复用），`etcd_mvcc_db_total_size_in_bytes`（物理）和 `..._in_use_bytes`（逻辑）差距越拉越大。

**NOSPACE 事故的标准处置流程（背下来能直接答故障题）**：
```bash
etcdctl endpoint status -w table          # 看 DB SIZE / RAFT INDEX / errors
etcdctl alarm list                        # 确认 NOSPACE
rev=$(etcdctl endpoint status -w json | jq '.[0].Status.header.revision')
etcdctl compact $rev                      # 1. 压掉历史版本
etcdctl defrag --cluster                  # 2. 逐节点整理碎片（生产建议一台台来）
etcdctl alarm disarm                      # 3. 解除告警，恢复写入
# 事后：开自动压缩 + 加 db size 监控告警，别再等它炸
```

> **K8s 场景注意**：kube-apiserver **自己会定期 compact**（`--etcd-compaction-interval`，默认 5min），此时 etcd 侧不要再开一个冲突的自动压缩策略；但 **defrag 谁都不会自动做，得运维自己排期**。

---

## 四、读一致性：线性一致读怎么做到（考频 ★★★★★）

### Q17. etcd 的读为什么也要走一遍"共识"？ReadIndex 是什么？

**朴素错误**：「读嘛，Leader 本地内存里有数据，直接返回不就行了？」——**不行，有两个坑**：

1. **Leader 可能已经不是 Leader 了**（网络分区，新 Leader 已在另一边产生并写入了新数据），它本地读到的是**过期数据** → 违反线性一致性（stale read）。
2. **Follower 的数据必然落后**（apply 有延迟）。

**ReadIndex 算法（etcd 默认的线性一致读）四步**：

1. Leader 记录当前的 **`commitIndex`** 作为本次读的 **readIndex**。
2. Leader **向所有 Follower 发一轮心跳**，收到**多数派**响应 → 证明「**此刻我仍是合法 Leader**」（因为若已有新 Leader，它必然拿到了多数派，两个多数派必相交，交集节点会用更大的 term 打脸我）。
3. Leader **等待自己的 `applyIndex ≥ readIndex`**（保证读的时候，readIndex 之前的写都已生效）。
4. **从本地状态机读**并返回。

**关键收益**：读**不写 Raft 日志、不落盘**，只多了**一轮心跳 RTT**。相比「把读也当成一条日志走一遍 Raft」（Log Read），省掉了磁盘 fsync，吞吐高一个量级。

**Follower 也能提供线性一致读**：Follower 向 Leader **索要 readIndex**（一次 RPC），Leader 走完上面 1~2 步把 readIndex 回给它，Follower 等自己 applyIndex 追上再本地读。→ **读可以水平扩展到所有节点，同时保持线性一致**，这是 etcd 相比 Redis 主从的一个亮点。

### Q18. Lease Read（租约读）是什么？和 ReadIndex 比？

**Lease Read**：Leader 在收到多数派心跳响应后，认为在接下来的 `election timeout` 时间窗口内**不可能有新 Leader 产生**（因为别人至少要等一个 election timeout 才会发起选举），于是**这段时间内的读直接本地返回，连心跳都省了**。

- **优点**：省掉一轮 RTT，读延迟最低。
- **风险**：**正确性依赖时钟**——如果机器时钟漂移 / 进程被 STW 暂停（GC）/ VM 被挂起，Leader 可能在"自以为还持有租约"时其实已被取代 → **可能读到旧值**。
- **etcd 的取舍**：默认用 **ReadIndex**（安全优先），Lease Read 在 raft 库层面可配（`ReadOnlyLeaseBased`），生产不建议开。TiKV 则默认开 Lease Read 并配合严格的时钟校验。

### Q19. `serializable` 读是什么？什么时候该用？

etcd 的读有两档（客户端 `clientv3.WithSerializable()`）：

| | **Linearizable（默认）** | **Serializable** |
|--|-------------------------|------------------|
| **路径** | 走 ReadIndex，要一轮心跳 + 等 apply | **直接读本节点本地数据** |
| **一致性** | 线性一致：读一定能看到此前所有已完成的写 | **可能读到旧值**（该节点 apply 落后） |
| **延迟/吞吐** | 高延迟、受 Leader 影响 | **极低延迟，可打任意节点，完美水平扩展** |
| **适用** | 分布式锁、选主、CAS 前置读、K8s 关键路径 | 监控大盘、配置全量拉取、能容忍秒级陈旧的场景 |

> **踩坑警告**：分布式锁、选主、"先读再 CAS"这类逻辑**绝不能用 serializable 读**——读到旧的 `mod_revision` 会让 CAS 逻辑失效或产生双主。
> 另外 K8s 里 `list` 请求带 `resourceVersion=0` 时走 apiserver 的 **watch cache**（本质就是可能陈旧的读），这也是很多"刚创建的 Pod list 不到"的怪现象根源。

### Q20. etcd 的一致性模型到底是什么级别？

- **写 + 默认读：线性一致（Linearizability）**——最强的单对象一致性，等价于「所有操作在某个时刻瞬间原子发生，且符合真实时间顺序」。
- **Txn：可串行化的迷你事务**——单次 Txn 内的所有操作原子生效（共享同一 main revision），但**不支持交互式事务**（不能"读了再想想再写"，只能一次性提交 If/Then/Else）。
- **Watch：有序 + 可靠 + 原子**（见 Q22）。
- **CAP 定位**：**CP**。网络分区时，少数派分区**拒绝服务**（无法达成 quorum），牺牲可用性保一致性。

> **对比 Redis**：Redis 主从是**异步复制**，主挂了会丢已 ACK 的写（AP）；etcd 是**多数派同步复制**，ACK 即持久（CP）。所以「Redis 分布式锁在主从切换时会双持有，etcd 锁不会」——这是 Redlock 争议的正解，也是面试的经典对比题。参见 [redis 面试题库 Q41](../redis/redis-interview-questions-90-coverage-20260709.md)。

---

## 五、Watch 机制（考频 ★★★★★）

### Q21. Watch 是怎么实现的？为什么能从历史 revision 开始？

**前提**：因为有 **MVCC**，历史事件本身就以 revision 的形式存在磁盘上，所以 Watch 可以「回放历史」。

**服务端结构（`watchableStore`）**：

```
watchableStore
 ├── synced   watcherGroup   ← 已追上当前 revision 的 watcher，新事件直接推
 ├── unsynced watcherGroup   ← 起始 revision 落后于当前的 watcher，需要"补历史"
 └── victims                 ← 客户端消费太慢、channel 阻塞的 watcher，稍后重试
```

**流程**：
1. 客户端 `Watch(key, WithRev(N))`。
   - `N == 0`（默认）：从**当前 revision** 开始，只收未来事件 → 直接进 **synced**。
   - `N > 0`：进 **unsynced**。后台 `syncWatchersLoop`（每 100ms）从 bbolt **批量读取 revision ≥ N 的所有事件**，推给它；追平后移入 **synced**。
2. 有新写入 apply 时，MVCC 层生成 `Event{type: PUT/DELETE, kv, prev_kv}`，**遍历 synced 组做区间匹配**（watcherGroup 内部用**区间树 interval tree** 加速前缀/范围匹配），把事件推给命中的 watcher。
3. 事件通过**同一条 gRPC stream** 回传，多个 watch 靠 `watch_id` 多路复用**共享一条 TCP 连接**。

**性能设计要点**：
- **一条连接多 watch**：K8s 一个 apiserver 可能有上万个 watcher，v2 的「一 watch 一连接」直接爆炸，v3 靠 gRPC stream 多路复用解决。
- **事件批量合并**：短时间内的多个事件打包在一个 `WatchResponse` 里发。
- **慢消费者隔离**：推不动的 watcher 进 `victims`，不阻塞其他 watcher（但会持续占内存 → 慢客户端是 etcd 内存暴涨的常见原因）。

### Q22. Watch 有哪些保证？什么情况下会丢事件？

**四条保证（官方明确承诺，背下来）**：
1. **有序（Ordered）**：事件按 revision 递增顺序推送，绝不乱序。
2. **可靠（Reliable）**：一个连续的事件序列不会中间少一个（要么全给，要么报错）。
3. **原子（Atomic）**：**同一个事务产生的所有事件**一定在**同一个 WatchResponse** 里给出，不会被拆开 → 客户端看到的永远是事务级一致的状态。
4. **去重（Unique）**：同一个 revision 的同一事件不会重复推送。

**唯一会"丢"的场景：`ErrCompacted`。**
- 客户端从 revision N 开始 watch，但 N 已被 **compact** 掉 → 服务端**没有那段历史了**，直接返回 `ErrCompacted` 并**关闭这个 watch**。
- **不是静默丢事件，而是明确报错** —— 这一点很重要：客户端收到 `ErrCompacted` 必须**重新 List 一次拿全量 + 拿到最新 revision，再从该 revision 重新 Watch**。

> **这就是 K8s 里 "too old resource version" 报错的真身**：informer 的 relist。K8s 的 **list-watch** 范式本质就是：
> ```
> List(拿全量 + 拿到 revision R) → Watch(from R+1) → 增量更新本地 cache
>                                     ↓ ErrCompacted / 断连
>                                   重新 List，回到起点
> ```
> **另一个坑**：Watch **不保证收到每一次中间状态**——如果一个 key 在你 watch 之前被改了 5 次，你从当前 revision 开始只会看到最后一次之后的变化。Watch 传递的是**状态变更的事实**，业务逻辑应当**面向最终状态设计（幂等、level-triggered）**，而不是依赖收到每一个中间事件（edge-triggered）。K8s controller 的 reconcile 循环就是这个哲学。

---

## 六、Lease 租约（考频 ★★★★）

### Q23. Lease 是什么？和 Redis 的 key TTL 有什么区别？

**Lease = 一个带 TTL 的"生命令牌"对象，key 可以挂在 lease 上；lease 一过期，挂在它上面的所有 key 被批量删除。**

```go
lease, _ := cli.Grant(ctx, 10)                            // 申请 10s 租约
cli.Put(ctx, "/svc/order/10.0.0.1:8080", "meta", clientv3.WithLease(lease.ID))
ch, _ := cli.KeepAlive(ctx, lease.ID)                     // 流式自动续约（每 TTL/3 一次）
```

**和 Redis TTL 的四点区别**：

| | Redis `EXPIRE` | **etcd Lease** |
|--|---------------|----------------|
| **粒度** | 一个 key 一个 TTL | **一个 lease 挂 N 个 key**（服务实例的多个注册信息同生共死） |
| **续期** | 客户端反复 `EXPIRE`（一次一个 key） | **一条 `KeepAlive` gRPC 流**，续一次全部生效 → 海量 key 时开销天差地别 |
| **过期删除** | 惰性 + 定期抽样，**不精确、不通知** | Leader 上的 **lessor 用最小堆**，到期发起一次**Raft 提议**删除 → 删除是**集群一致事件，会触发 Watch** |
| **一致性** | 主从异步，切换后 TTL 可能重置/丢失 | 走 Raft，**所有节点对"lease 何时过期"达成共识** |

**关键点：lease 到期的删除也是一次 Raft 写。** 所以它是**强一致的、可被 Watch 到的**——这正是服务发现能"实例挂了立刻通知"的基础，而 Redis 的过期删除没有这个保证。

### Q24. Lease 的 TTL 在 Leader 切换后会被重置吗？（checkpoint）

**老版本会——这是个经典坑。** lease 的**剩余 TTL 只在 Leader 内存里**维护，Leader 切换后新 Leader 按**完整 TTL** 重新计时。若 TTL 是 300s，Leader 频繁切换会让这个 lease **永远不过期** → 挂掉的服务实例的注册信息**一直不被清理**（幽灵实例）。

**Lease Checkpoint 机制**：Leader 定期（默认 5min）把每个 lease 的**剩余 TTL 通过 Raft 同步给 Follower**，切换后新 Leader 从 checkpoint 恢复剩余时间，而不是从头计时。
- v3.4：`--experimental-enable-lease-checkpoint`（默认关）
- v3.5+：逐步转正、默认开启

> **实操建议**：TTL 别设太长（服务发现一般 10~30s），既降低幽灵实例窗口，也削弱这个坑的影响。

---

## 七、事务 Txn 与 CAS（考频 ★★★★）

### Q25. etcd 的事务长什么样？和数据库事务的区别？

etcd Txn 是一次性提交的 **`If-Then-Else` 迷你事务**，**原子执行、无交互**：

```go
// CAS：只有当 key 还是我上次读到的版本时才更新（乐观锁）
resp, err := cli.Txn(ctx).
    If(clientv3.Compare(clientv3.ModRevision("/cfg/rate"), "=", oldModRev)).
    Then(clientv3.OpPut("/cfg/rate", "200")).
    Else(clientv3.OpGet("/cfg/rate")).          // 失败时把最新值带回来，供重试
    Commit()

if !resp.Succeeded {  /* 冲突了，用 Else 拿到的最新值重试 */ }
```

**Compare 的四个维度**：`Value` / `Version` / `CreateRevision` / `ModRevision`（还可比 `Lease`）。

**和 MySQL 事务的关键区别**：

| | MySQL 事务 | etcd Txn |
|--|-----------|----------|
| **交互性** | 可 `BEGIN` → 读 → 思考 → 写 → `COMMIT` | ❌ **一次性提交**，不能中途读了再决定 |
| **隔离级别** | RR/RC 等多档，靠 MVCC + 锁 | 只有一档：**整个 Txn 原子生效**（共享一个 main revision） |
| **并发控制** | 悲观锁为主（行锁/间隙锁） | **纯乐观**（比 revision，冲突就重试） |
| **规模** | 可涉及大量行、长事务 | 操作数受 `--max-txn-ops`（默认 128）限制 |

> **为什么设计成非交互式？** 交互式事务需要**持有锁跨越多次网络往返**，在一个要保证毫秒级心跳的共识系统里是灾难（长事务 = 长时间占用 apply 线程 = 心跳延迟 = 选举）。etcd 用「**一次性提交 + 客户端重试循环**」换取服务端的绝对简单和可预测。clientv3 的 **STM（Software Transactional Memory）** 包就是在客户端封装了这个「读 → 算 → CAS → 冲突则重试」的循环，让你写起来像有交互式事务。

### Q26. 怎么用 etcd 实现一个安全的计数器 / 幂等标记？

```go
// 幂等：只在 key 不存在时创建（CreateRevision == 0 表示"不存在"）
resp, _ := cli.Txn(ctx).
    If(clientv3.Compare(clientv3.CreateRevision("/job/20260723/lock"), "=", 0)).
    Then(clientv3.OpPut("/job/20260723/lock", hostname, clientv3.WithLease(leaseID))).
    Commit()
if resp.Succeeded { /* 我抢到了，我来跑这个任务 */ }
```

`CreateRevision == 0` 是 etcd 里表示「**这个 key 不存在**」的惯用法，**分布式锁、选主、幂等去重全靠它**。

> 这一招正好接上 [ETL 链路深挖](../bigdata/08-etl-pipeline-deep-dive-20260723.md) 里的**幂等**话题：ETL 任务的「同一批次只跑一次」，用 etcd 的 `CreateRevision==0` + Lease 做分布式任务抢占，比用 MySQL 唯一键更适合多机调度场景（因为宕机后 lease 自动释放，不会留下死锁记录）。

---

## 八、分布式锁与选主（考频 ★★★★★）

### Q27. 用 etcd 实现分布式锁，完整方案是什么？

**朴素版（有惊群问题）**：所有人抢同一个 key，抢不到就 watch 它，删了再抢 → key 一释放，**几百个客户端同时被唤醒同时抢**（thundering herd），只有一个成功，其余白跑，还给 etcd 制造流量尖峰。

**etcd 官方 `concurrency.Mutex` 的正解（公平队列锁）**：

1. **申请 Lease**（TTL，比如 10s），起 `KeepAlive` 自动续约。
2. 在锁前缀下**创建自己的排队节点**：`Put("/lock/mykey/<leaseID>", "", WithLease(leaseID))`，记下自己的 **`CreateRevision`**。
3. **`Get` 该前缀下 `CreateRevision` 最小的 key**：
   - 是我 → **我拿到锁**。
   - 不是我 → 找到**恰好排在我前面**那个 key（CreateRevision 比我小的里面最大的那个），**只 watch 它的 DELETE 事件**。
4. 前一个被删（正常释放 / 持有者宕机导致 lease 过期）→ 我被唤醒 → 回到第 3 步确认。
5. **释放**：`Delete` 自己的 key + `Revoke` lease。

**关键设计点（追问必答）**：
- **只 watch 前驱，不 watch 整个前缀** → **彻底消除惊群**，一次释放只唤醒一个客户端。
- **靠 `CreateRevision` 排序** → 天然 **FIFO 公平锁**，不会饿死。
- **Lease 兜底** → 持有者进程崩溃/断网，lease 到期自动删 key，锁自动释放，**不会死锁**。
- **KeepAlive 续约** → 只要进程活着，锁不会被误释放（对应 Redisson 的看门狗）。

```go
s, _ := concurrency.NewSession(cli, concurrency.WithTTL(10))  // 内含 lease + keepalive
defer s.Close()
m := concurrency.NewMutex(s, "/lock/etl-job/")
if err := m.Lock(ctx); err != nil { return err }              // 阻塞直到拿到锁
defer m.Unlock(ctx)
// ... 临界区 ...
```

### Q28. etcd 锁 vs Redis 锁，到底该选哪个？（超高频对比题）

| 维度 | **Redis 锁**（`SET NX PX` + Lua） | **etcd 锁**（Txn + Lease + Watch） |
|------|--------------------------------|----------------------------------|
| **一致性模型** | **AP**，主从异步复制 | **CP**，多数派同步 |
| **主从切换** | ⚠️ **可能双持有**：锁写入主库、未同步就切换 → 新主没这把锁 → 第二个人也能拿到 | ✅ **不会**：写入即多数派持久化，新 Leader 必然有这把锁 |
| **等待方式** | 自旋轮询（浪费 CPU + 打 Redis） | **Watch 前驱，事件驱动**，零轮询 |
| **公平性** | ❌ 无序，可能饿死 | ✅ **FIFO 公平**（按 CreateRevision） |
| **自动续期** | 需客户端实现（Redisson 看门狗） | ✅ **Lease KeepAlive 原生支持** |
| **性能** | **极高**（10w+ QPS） | 一般（**千~万级**，每次加锁 = 一次 Raft 写） |
| **Redlock** | 多节点投票，Martin Kleppmann 撰文质疑其安全性（时钟假设、GC 暂停） | 不需要，共识层已经解决 |

**选型口径**：
- **要正确性**（选主、金融扣款、"绝对不能两个人同时执行"）→ **etcd / ZooKeeper**。
- **要性能、且锁失效只是"多做一次无害的工作"**（防缓存击穿、限流、去重）→ **Redis**。
- **务实的说法**：「**Redis 锁是性能锁，etcd 锁是正确性锁**。真正要命的场景，除了分布式锁还要在业务层做幂等/唯一约束兜底——因为**任何分布式锁都无法防住『持有锁的进程 GC 停顿超时后又醒过来继续写』**（fencing token 问题）。」

### Q29. 选主（Leader Election）怎么做？和分布式锁什么关系？

**本质是同一套机制**，`concurrency.Election` 就是 Mutex 的变体：

```go
s, _ := concurrency.NewSession(cli, concurrency.WithTTL(15))
e := concurrency.NewElection(s, "/election/etl-scheduler")
if err := e.Campaign(ctx, myID); err != nil { return err }  // 阻塞直到当选
// —— 我是 Leader，开始干活（跑定时调度）——
// 其他实例：ch := e.Observe(ctx)  → 持续观察当前 Leader 是谁
```

- **`Campaign`**：创建带 lease 的排队 key，**CreateRevision 最小者当选**，其余 watch 前驱等着接班。
- **`Observe`**：非参选者可以持续观察当前 Leader 是谁（用于把流量路由到 Leader）。
- **`Resign`**：主动让位。

**⚠️ 面试重点：即使有 etcd，你的应用也可能"双主"。**
`Campaign` 返回后，你的进程可能因为 **GC 长暂停 / 机器时钟 / 网络分区** 而在 lease 已过期后**仍然自以为是 Leader** 继续写数据，而此时新 Leader 已经产生。
**正解**：① 持有 Leader 期间**持续检查 session 是否还活着**（`<-s.Done()` 立刻停止工作）；② 关键写操作带 **fencing token**（用 lease ID 或 revision 作为单调递增令牌，下游拒绝旧 token 的写）。
**"分布式锁不能保证互斥，只能降低冲突概率；真正的互斥要靠下游的单调令牌校验"** —— 这句话能答出来，这道题就满分了。

---

## 九、集群部署与运维（考频 ★★★★）

### Q30. 集群该部署几个节点？为什么是奇数？

**容错公式**：`n` 个节点，quorum = `n/2 + 1`，可容忍 **`(n-1)/2`** 个节点故障。

| 节点数 | quorum | 可容忍故障 | 评价 |
|:--:|:--:|:--:|------|
| 1 | 1 | 0 | 测试用 |
| 2 | 2 | **0** | ❌ **比单节点还差**（任一挂了就全挂，且多一倍故障面） |
| **3** | 2 | **1** | ✅ **最常用** |
| 4 | 3 | 1 | ❌ 和 3 一样的容错，写更慢、故障面更大 |
| **5** | 3 | **2** | ✅ 要求更高可用时用（K8s 大集群标配） |
| 7+ | 4+ | 3+ | ⚠️ 写延迟明显上升，收益递减，官方不推荐 |

**为什么奇数**：**偶数节点不增加容错能力，只增加故障概率和写延迟**。4 节点和 3 节点都只能挂 1 台，但 4 节点有更多机器可能坏、每次写要等更多 ACK。

**为什么不无脑加节点**：
- **写性能随节点数下降**：quorum 变大 → 要等更多节点 fsync + 网络回执 → 尾延迟被最慢的那台拖累。
- **读性能可以随节点数上升**（Follower 也能做线性一致读 / serializable 读），所以「读扩展加节点，写扩展加不了——只能加机器性能」。

> **跨机房部署**：三机房各 1 节点（3AZ）是最佳实践，能扛整个机房挂掉。但**跨地域（跨城）部署要极其慎重**——每次写都要等跨城 RTT（几十 ms），写延迟直接爆炸。跨地域用 **Learner 做只读副本**，不参与投票。

### Q31. etcd 的性能瓶颈在哪？怎么调优？

**瓶颈只有两个：磁盘 fsync 延迟 和 网络 RTT。**（CPU/内存基本不是瓶颈，除非 watcher 太多或数据量大）

**调优清单**：

| 方向 | 做法 |
|------|------|
| **磁盘（第一优先）** | 必须 **SSD/NVMe**；**etcd 独占磁盘**，绝不和 apiserver/日志/其他 IO 密集服务共盘；Linux 上给 etcd 进程调高 IO 优先级（`ionice -c2 -n0`） |
| **心跳/选举参数** | `--heartbeat-interval`（默认 100ms）应 ≈ 节点间 **RTT**；`--election-timeout`（默认 1000ms）应 **≥ 10 × heartbeat**，且 ≤ 50000ms。跨机房 RTT 大就要一起调大，否则**假选举**不断 |
| **快照/日志** | `--snapshot-count`（默认 100000）；太小 → 频繁快照；太大 → WAL 占空间、恢复慢 |
| **批量提交** | `--backend-batch-interval`（默认 100ms）/ `--backend-batch-limit`：攒批写 bbolt，牺牲一点可见延迟换吞吐 |
| **配额** | `--quota-backend-bytes`（默认 2GB，**推荐上限 8GB**）；配上自动压缩 `--auto-compaction-mode=periodic --auto-compaction-retention=1h` |
| **客户端侧** | 用 **Range 批量读**代替循环单 key 读；**只读走 serializable** 分摊到 Follower；watch 复用同一个 client（共享 gRPC 连接） |
| **限流** | `--max-request-bytes`（默认 1.5MB）、`--max-concurrent-streams`；防止大 value 打爆内存 |

### Q32. etcd 必须盯的核心监控指标有哪些？

| 指标 | 含义 | 红线 |
|------|------|------|
| `etcd_disk_wal_fsync_duration_seconds` | **WAL fsync 延迟**（最重要） | P99 < 10ms，>25ms 报警 |
| `etcd_disk_backend_commit_duration_seconds` | bbolt 提交延迟 | P99 < 25ms |
| `etcd_server_leader_changes_seen_total` | **Leader 切换次数** | 稳态应几乎不变，频繁涨 = 磁盘慢/网络抖 |
| `etcd_server_has_leader` | 是否有 Leader | **必须 = 1**，为 0 即集群不可写 |
| `etcd_network_peer_round_trip_time_seconds` | 节点间 RTT | 决定 heartbeat 参数 |
| `etcd_mvcc_db_total_size_in_bytes` / `..._in_use_bytes` | db 物理 / 逻辑大小 | 物理逼近 quota 要报警；两者差距大 = 该 defrag |
| `etcd_server_proposals_failed_total` | 提议失败数 | 持续增长 = 集群不健康 |
| `etcd_server_proposals_pending` | 待处理提议堆积 | 堆积 = apply 跟不上 |
| `etcd_debugging_mvcc_watcher_total` | watcher 数量 | 暴涨 = 客户端泄漏 watch |

**日志里的经典报警语**（见到就知道是慢盘/网络问题）：
- `failed to send out heartbeat on time` → **apply 或磁盘阻塞了主循环**，心跳发不出去 → 快要选举了。
- `apply entries took too long` → 状态机 apply 慢（大事务 / 慢盘）。
- `slow fdatasync` / `took too long to execute` → 磁盘不行。
- `mvcc: database space exceeded` → NOSPACE，见 Q16。

### Q33. 备份与恢复怎么做？

```bash
# 备份（对任一节点执行，产出一致性快照）
etcdctl snapshot save /backup/etcd-$(date +%F).db

# 查看快照信息（v3.6 起用 etcdutl）
etcdutl snapshot status /backup/etcd-xxx.db -w table

# 恢复：在每个节点上分别 restore，产出新的数据目录，然后重启集群
etcdutl snapshot restore /backup/etcd-xxx.db \
  --name m1 --initial-cluster m1=http://10.0.0.1:2380,m2=...,m3=... \
  --initial-advertise-peer-urls http://10.0.0.1:2380 \
  --data-dir /var/lib/etcd-new
```

**必须知道的三个点**：
1. **restore 会创建一个全新集群**（新的 cluster ID、新的 member ID），**不能**把恢复出来的节点加进老集群 —— 会因 cluster ID 不匹配被拒绝。
2. **恢复后 revision 会回退**到备份时刻。K8s 场景下这意味着**所有 informer 的 resourceVersion 都超前于 etcd** → apiserver 必须重启，controller 会 relist。这是 K8s etcd 恢复后必须重启控制面组件的原因。
3. **备份的是快照那一刻的数据**，中间的写全部丢失 → 高价值集群要 **定时快照 + WAL 归档**，或直接用**跨机房 Learner** 做热备。

---

## 十、K8s 场景：etcd 在生产里怎么被用（考频 ★★★★）

### Q34. K8s 是怎么用 etcd 的？

- **只有 kube-apiserver 直连 etcd**，其他所有组件（scheduler / controller-manager / kubelet）都通过 apiserver 访问 —— etcd 被严格收敛在一个入口后面（这是**架构上的收口**，也是为什么 etcd 不需要复杂的多租户权限）。
- **数据布局**：所有对象扁平存在 `/registry/<resource>/<namespace>/<name>`，如 `/registry/pods/default/nginx-abc`。value 是对象的序列化（protobuf 或 JSON）。
- **`resourceVersion` 就是 etcd 的 revision**：K8s 对象上那个字符串型 `resourceVersion`，本质是 etcd 的 `mod_revision`（或 list 时的全局 revision）。所以它**全局单调、不可跨资源比较大小、不能当版本号做算术**。
- **list-watch**：见 Q22。`ErrCompacted` → `"too old resource version"` → informer relist。
- **乐观并发**：`kubectl apply` 的冲突检测（`Operation cannot be fulfilled ... object has been modified`）就是 apiserver 拿 `resourceVersion` 做了一次 etcd 的 **`ModRevision` CAS**。
- **Lease 的用武之地**：node 心跳（`coordination.k8s.io/Lease` 对象，每个节点每 10s 更新一次，取代了老版的频繁更新 Node status）、controller-manager/scheduler 的**选主**。

### Q35. 大规模 K8s 集群里 etcd 的典型优化？

1. **Events 拆到独立 etcd 集群**：`kube-apiserver --etcd-servers-overrides=/events#https://etcd-events:2379`。Event 写入量极大、价值低、TTL 短，会疯狂消耗主 etcd 的写能力和 db 空间 —— **拆分是最有效的单项优化**。
2. **调 compaction 间隔**：`--etcd-compaction-interval`（默认 5min）。
3. **apiserver watch cache**：`--watch-cache-sizes`，让大量 `list` 打在 apiserver 内存缓存而不是 etcd。
4. **限制大对象**：ConfigMap/Secret 有 1MB 上限（受 etcd `max-request-bytes` 制约）；别把大文件塞进去。
5. **控制 etcd 数据总量**：目标 **< 8GB**；定期 defrag（滚动，一次一台）。
6. **5 节点 + 独占 NVMe + 3AZ**，控制面和 etcd 分离部署。

> **典型事故串讲（能讲出来说明真懂）**：
> 某个 controller 有 bug，疯狂 update 同一批对象 → **etcd revision 暴涨 + 历史版本堆积** → db 逼近 2GB quota → 触发 **NOSPACE alarm → 整个集群 API 变只读** → 所有 Deployment 无法更新、Pod 无法调度。
> 处置：`alarm list` 确认 → `compact` + `defrag` + `alarm disarm` 恢复 → 再回头查是哪个 controller 在写风暴（看 `etcd_debugging_mvcc_put_total`、apiserver 审计日志）。
> 预防：db size 监控 + 自动压缩 + 对 controller 的写做限流。

---

## 十一、选型对比与其余高频（考频 ★★★）

### Q36. etcd vs ZooKeeper vs Consul vs Nacos？

| 维度 | **etcd** | **ZooKeeper** | **Consul** | **Nacos** |
|------|---------|--------------|-----------|----------|
| **共识算法** | Raft | ZAB | Raft | Raft（CP 模式）/ Distro（AP 模式） |
| **语言** | Go | Java | Go | Java |
| **数据模型** | 扁平 KV + 前缀范围 | znode 树 | KV + 服务目录 | 配置 + 服务 |
| **接口** | gRPC / HTTP | 自定义 TCP（Jute） | HTTP / DNS | HTTP |
| **Watch** | **流式、可回放历史、不丢** | 早期**一次性**（用完要重注册，易丢事件），3.6+ 支持持久 watch | HTTP long polling（blocking query） | 长轮询 |
| **临时节点** | Lease（多 key 共享） | Ephemeral znode（绑 session） | Session + TTL check | 临时实例 |
| **健康检查** | 靠 Lease 续约 | 靠 session 心跳 | ✅ **内建多种健康检查**（HTTP/TCP/脚本） | ✅ 内建 |
| **一致性** | CP | CP | CP（可配 stale 读） | **CP/AP 可切换** |
| **生态位** | **K8s 事实标准**、云原生基础设施 | Hadoop/Kafka/HBase 生态（**大数据系标配**） | 服务网格、多数据中心 | Spring Cloud Alibaba 生态 |

**面试口径**：
- 「**云原生 / K8s 相关 → etcd**（生态绑定，Go 系，运维简单）」
- 「**JVM 大数据生态 → ZooKeeper**（Kafka 老版本的 controller 选举与元数据、HBase、Hive HA 都靠它；不过 **Kafka 已用 KRaft 去 ZK 化**，这是个可以主动提的加分点）」
- 「**要开箱即用的服务发现 + 健康检查 + 多 DC → Consul**」
- 「**Java 微服务、要 AP 模式抗大规模注册 → Nacos**」

### Q37. Kafka 为什么要去掉 ZooKeeper（KRaft）？（延伸加分题）

- **痛点**：ZK 是外部依赖（多一套集群要运维）；controller 从 ZK 加载全量元数据在分区数极多时**恢复极慢**（分钟级）；元数据变更走 ZK watch，扩展性受限；两套一致性系统（ZK + Kafka 自身副本）语义割裂。
- **KRaft**：Kafka 内建 Raft，把元数据本身存成一个 **`__cluster_metadata` 内部日志 topic**，controller 就是这个日志的 Leader。元数据变更 = 日志追加，broker 通过**消费日志**同步元数据 → 恢复变成"读日志"，秒级；分区数上限提升一个数量级。
- **这题的价值**：它是「**共识组件从外置走向内置**」这一趋势的最佳案例，能顺带把 Raft 讲一遍。

### Q38. etcd 的鉴权和安全怎么做？

- **传输安全**：客户端-服务端（2379）和节点间（2380）都应开 **mTLS 双向认证**（`--cert-file/--key-file/--trusted-ca-file/--client-cert-auth/--peer-*`）。K8s 里 etcd 证书泄露 ≈ **整个集群沦陷**（所有 Secret 明文可读）。
- **认证**：`user`/`role` + RBAC，role 按 **key 前缀或范围**授权读写。开启 `--auth-token=jwt` 时 token 可跨节点验证。
- **静态加密**：etcd 本身**不加密数据**。K8s 的 Secret 加密靠 apiserver 侧的 **EncryptionConfiguration**（写入 etcd 前先加密），而不是 etcd 提供 —— 这是常考的认知点：**「Secret 默认只是 base64，不是加密」，加密要在 apiserver 配置。**

### Q39. Go 客户端 `clientv3` 的工程注意事项？

```go
cli, err := clientv3.New(clientv3.Config{
    Endpoints:            []string{"h1:2379", "h2:2379", "h3:2379"},
    DialTimeout:          5 * time.Second,
    DialKeepAliveTime:    30 * time.Second,   // 保活探测，防中间设备掐连接
    DialKeepAliveTimeout: 10 * time.Second,
    AutoSyncInterval:     5 * time.Minute,    // 自动同步集群成员变化
})
defer cli.Close()   // ⚠️ client 是长连接，全局单例复用，不要每次请求都 New
```

- **client 必须复用**：内部是 gRPC 长连接 + 连接池，频繁创建会打爆 etcd 的连接数和 watcher 数。
- **每个操作都要带 `context.WithTimeout`**：否则 etcd 抖动时会**无限阻塞**你的业务 goroutine（这是 Go 服务里最常见的 etcd 事故形态）。
- **区分错误类型**：`rpc.ErrCompacted`（要 relist）、`ErrLeaseNotFound`（session 已失效，要重新选主/加锁）、`context.DeadlineExceeded`（**不代表写失败，可能已成功** → 业务要幂等）。
- **Watch 要处理 `resp.Canceled` 和 `resp.CompactRevision`**，并实现**自动重连 + 重新 List** 的循环，别写"一次 for-range 完事"的玩具代码。
- **别把 etcd 当高频读接口**：本地缓存 + Watch 增量更新（就是 K8s informer 的做法），而不是每次请求都去 Get。

---

## 附录：自我思考

### 这篇的定位与局限

- 这是**广度优先的抽考清单**，目标「面试前一晚快速捞回 90% 考点」，体例对齐 [mysql 面试题库](../mysql/mysql-interview-questions-90-coverage-20260704.md) 和 [redis 面试题库](../redis/redis-interview-questions-90-coverage-20260709.md)。
- **以下几处还停在「结论层」，按 depth contract 应该单独成篇 + 动手验证**：
  - **Raft 手写最小实现**（Q4~Q7）：用 Go 写选举 + 日志复制的骨架（或直接跑 MIT 6.824 Lab 2），把「随机超时打破对称」「日志回退对齐」「Figure 8 的提交限制」从背变成调试过。这是本篇最该补的一块。
  - **MVCC 层动手**（Q13）：起 3 节点 etcd，`put` 同一个 key 多次，用 `get --rev=N` 逐版本读回来，再 `compact` 掉，观察 `ErrCompacted`；用 `bbolt` 命令行工具直接扒开 db 文件看 `key` bucket 里的 revision 编码。对照 [PG WAL 那篇](../infra/postgresql-wal-replication-and-amplification.md) 一起看放大问题。
  - **NOSPACE 事故复现**（Q16/Q35）：`--quota-backend-bytes=16777216` 起个小配额集群，写爆它，走一遍 `alarm list → compact → defrag → disarm` 全流程。**这是能直接讲进面试的动手素材**。
  - **分布式锁的失效实验**（Q27~Q29）：写一个 Go 程序，持有锁后 `SIGSTOP` 模拟 GC 停顿超过 TTL，观察第二个客户端拿到锁，两边同时写 → 亲手复现「分布式锁不保证互斥」，并加 fencing token 修好它。
  - **慢盘复现**（Q31/Q32）：用 `cgroup blkio` 或 `dm-delay` 限制 etcd 的磁盘，观察 `wal_fsync_duration` 上涨 → `failed to send out heartbeat on time` → Leader 反复切换的完整因果链。

### 与能力地图的挂钩

- 本篇主打第 **8 格「分布式系统」**：CAP 定位、Raft 共识、脑裂、分布式锁、线性一致性 —— 这一格原本是「未启」，etcd 是打开它最好的入口，因为**它把共识从论文变成了可以 docker run 的东西**。
- **跨格串联**：
  - 格 7 存储引擎：bbolt B+ 树 vs LSM（Q14）、WAL（Q15）、MVCC（Q13）、compact 对应 purge/vacuum —— 和 [四棵树对比](../mysql/tree-structures-bplus-btree-rbtree-20260708.md)、[MySQL MVCC](../mysql/mysql-mvcc-isolation-phantom-read-20260710.md)、PG WAL 那篇是同一条线的不同侧面。
  - 格 5 缓存：etcd 锁 vs Redis 锁（Q28）、CP vs AP（Q20），直接补上了 [redis 面试题库 Q41](../redis/redis-interview-questions-90-coverage-20260709.md) 留的钩子。
  - 格 6 消息队列：Kafka KRaft 去 ZK（Q37），是 Kafka 那篇开工时的引子。
  - 格 9/12 微服务与运维：服务注册发现、配置中心、K8s 控制面（Q34/Q35）。
  - 格 4 数据库：Txn 的乐观并发 vs MySQL 的悲观锁（Q25）。
- **暂缺**：ZooKeeper 的 ZAB 细节（本篇只做了对比，若目标岗位是大数据/JVM 系需单独补）；Raft 的正式性证明；etcd v3.6+ 的 downgrade / memory 优化等新特性。

### 复习调度

按 CLAUDE.md 第五节：本篇状态 **「进行中」**（纯纸面，还没动手验证，不能算毕业）。
- **+1 天（07-24）**：盖答案自测 30 秒电梯版 + Q2（写链路 8 步）/ Q5 / Q7 / Q13 / Q17 / Q27 六道核心题。
- **动手项（做完才升级为「毕业」）**：① `docker-compose` 起 3 节点 etcd，跑通 Q16 的 NOSPACE 全流程；② 写 Go 版分布式锁并复现 Q29 的双持有；③ Raft 选举最小实现。

### 自测题（盖住上面的答案，先自己讲一遍）

**Raft 与一致性**
1. 一次 `Put` 从客户端到返回，经过哪 8 步？在哪一步数据算"不会丢了"？committed 和 applied 差在哪？
2. Leader 选举为什么要随机化超时？不随机会怎样？
3. Raft 靠哪一条规则保证「已提交的日志不会被新 Leader 覆盖」？完整比较规则是什么？
4. 为什么 Leader 不能直接提交前任 term 的日志？etcd 用什么手段绕过（no-op entry 的作用）？
5. PreVote 解决什么问题？CheckQuorum 又解决什么？
6. 成员变更为什么危险？单节点变更为什么安全？Learner 解决哪三个痛点？
7. 线性一致读的 ReadIndex 四步是什么？为什么"发一轮心跳"就能证明自己还是 Leader？
8. Lease Read 比 ReadIndex 快在哪、危险在哪？serializable 读什么时候绝对不能用？

**存储与 MVCC**
9. revision / create_revision / mod_revision / version 四者分别是什么、各自用在什么场景？
10. 白板画出 treeIndex + bbolt 两层结构，讲清「读当前值」和「读历史版本」各走几步。
11. 为什么 etcd 选 B+ 树（bbolt）而不是 LSM？TiKV 为什么反过来选 RocksDB？
12. etcd 的 WAL 和 MySQL redo log 有什么异同？为什么 etcd 的 WAL 不能不 fsync？
13. compact 和 defrag 分别干什么？只做前者会怎样？NOSPACE 告警的处置四步是什么？

**Watch / Lease / 锁**
14. Watch 的 synced / unsynced 两个组各是什么？从历史 revision watch 是怎么补数据的？
15. Watch 的四条保证是什么？唯一会丢事件的场景是什么？K8s 的 "too old resource version" 是怎么来的？
16. Lease 和 Redis 的 key TTL 有哪四点区别？为什么 lease 过期删除能被 Watch 到？
17. lease checkpoint 解决什么问题？不开会出现什么现象？
18. etcd 分布式锁的五步流程？「只 watch 前驱」防的是什么？靠什么实现 FIFO 公平？
19. etcd 锁和 Redis 锁怎么选？为什么说「任何分布式锁都不能保证绝对互斥」？fencing token 怎么补这个洞？

**运维与场景**
20. 为什么要奇数节点？4 节点为什么比 3 节点差？加节点能提升写性能吗？
21. etcd 的性能瓶颈是什么？heartbeat 和 election-timeout 该怎么配、为什么是 10 倍关系？
22. 哪三个监控指标最该盯？看到 `failed to send out heartbeat on time` 说明什么？
23. snapshot restore 有哪三个必须知道的坑？K8s 恢复 etcd 后为什么要重启 apiserver？
24. K8s 的 resourceVersion 是什么？大集群里 etcd 的典型优化有哪几条？为什么 events 要拆出去？
25. etcd / ZooKeeper / Consul / Nacos 各自的生态位？Kafka 为什么要去 ZK 化？

答不出的，回对应 Q 挖。
