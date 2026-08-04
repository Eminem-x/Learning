# Kafka 面试题库（90% 覆盖率 · 白板级）

> 来源：个人思考 + 学习（对标《四年后端 + 轻量大数据能力地图》第 6 格：消息队列）
> 收录时间：2026-07-23 CST
>
> 说明：本篇是**综合入口 + 抽考清单**，覆盖 Kafka 面试约 90% 的高频考点。体例对齐 [mysql 面试题库(广度)](../mysql/mysql-interview-questions-90-coverage-20260704.md) 与 [redis 面试题库(广度)](../redis/redis-interview-questions-90-coverage-20260709.md)：每个考点给「一句话答案 + 追问链条」，深挖到白板级；能单独成篇的专题（ISR/HW 推演、事务 2PC、rebalance 实验）在此挂钩，后续再拆。版本以 **Kafka 3.x（KRaft）** 为基线，涉及 ZooKeeper 时代与 0.11/2.x 差异会标注。
>
> **面试站位提醒**：这一格补的不是「Kafka 使用手册」，而是**消息中间件的通用原理**——分区与并行、副本与一致性、投递语义、消费者组与再均衡、积压与回溯。Kafka 只是最好的入口（因为它把这些机制都做在明面上）。

---

## 30 秒电梯版（盖住全文也要能背出主线）

Kafka 面试就十条主线：

1. **模型**：Topic 逻辑分类 → **Partition 是并行与顺序的最小单位**（分区内有序、分区间无序）→ Partition 有多副本（1 Leader + N Follower）。**读写只走 Leader**，Follower 只负责拉取同步（不分担读，2.4+ 才有受限的 follower fetch）。
2. **为什么快**：**顺序追加写**（append-only log）+ **页缓存 PageCache**（不走 JVM 堆，避 GC）+ **零拷贝 sendfile**（消费端）+ **批量 & 压缩**（RecordBatch）+ **分区并行**。五条要能各自物理解释。
3. **存储**：分区 = 一串 **Segment 文件**（`.log` + 稀疏 `.index` + `.timeindex`），文件名是 baseOffset；查一条消息 = **二分找段 → 二分找稀疏索引 → 顺序扫**。清理策略两种：`delete`（按时间/大小删）和 `compact`（按 key 只留最新值）。
4. **生产者**：`acks=0/1/all` 决定持久性；`enable.idempotence`（PID + 单调序列号）解决**重试导致的重复**；`transactional.id` + 事务协调器 2PC 解决**跨分区原子写**；`batch.size`/`linger.ms` 是吞吐与延迟的旋钮。
5. **副本一致性**：**ISR**（与 Leader 同步的副本集合，靠 `replica.lag.time.max.ms` 判定）+ **HW/LEO**（消费者只能读到 HW）+ **Leader Epoch**（0.11+，修掉纯 HW 方案的截断丢数据）。`acks=all` + `min.insync.replicas=2` + `unclean.leader.election=false` 是不丢数据三件套。
6. **消费者组**：组内**一个分区只能被一个消费者消费**（所以消费者数 > 分区数会空转）；offset 存在内部 topic `__consumer_offsets`（compact，50 分区）；**Rebalance** 由 Group Coordinator 主持（JoinGroup/SyncGroup），Eager 会 STW，**CooperativeSticky（2.4+）** 增量再均衡。
7. **投递语义**：至多一次 / **至少一次（默认）** / 精确一次。EOS = **幂等生产者 + 事务 + 消费端 `isolation.level=read_committed`**，且只在「Kafka → Kafka」闭环内成立；**端到端 EOS 靠下游幂等/事务，别指望 Kafka 单方面兜底**。
8. **消息丢失三处**：Producer（`acks` 太松 / 不重试）、Broker（副本不足 + unclean 选举 / 全 ISR 同时宕机而未落盘）、Consumer（**先提交 offset 后处理**）。重复消费同理三处，最终都靠**业务幂等**兜底。
9. **积压与回溯**：Lag = LEO − committed offset；扩消费能力的上限是**分区数**；回溯靠 `seek` / `kafka-consumer-groups --reset-offsets`（按 offset/时间戳）——这是 Kafka 相对传统 MQ 最大的优势：**消息不因消费而删除**。
10. **元数据面**：老版本靠 **ZooKeeper**（Controller 选举、元数据存储），3.3+ 生产可用的 **KRaft** 用 Raft 自管元数据（`__cluster_metadata`），**4.0 彻底移除 ZK**，好处是元数据一致性更强、Controller failover 从分钟级降到秒级、分区规模上限大幅提升。

卡在哪一条，就往对应章节回挖。

---

## 一、架构与核心概念（考频 ★★★★★）

### Q1. 画一下 Kafka 的整体架构，说清每个角色

```
Producer ──┐                        ┌── Consumer(组 A) ─┐
           │   ┌──────────────┐     │                   │ 同组内
Producer ──┼──▶│  Kafka 集群  │────▶├── Consumer(组 A) ─┘ 分区互斥
           │   │ Broker0/1/2  │     │
Producer ──┘   └──────────────┘     └── Consumer(组 B) 独立一份全量
                      ▲
                      │ 元数据/Controller
              ZooKeeper（≤2.x） 或 KRaft Quorum（3.x+）
```

| 角色 | 是什么 | 追问点 |
|------|--------|--------|
| **Broker** | 一个 Kafka 服务进程/节点，存储分区数据 | 一个 Broker 可放多个分区副本；`broker.id` 唯一 |
| **Topic** | 消息的**逻辑分类**，纯逻辑概念，无物理实体 | Topic 本身不存数据，数据在分区里 |
| **Partition** | Topic 的**物理分片**，一个有序、不可变的追加日志 | **并行度与顺序性的最小单位**，是理解 Kafka 的钥匙 |
| **Replica** | 分区的副本，`replication.factor` 决定份数 | 1 个 Leader + N 个 Follower；**只有 Leader 对外读写** |
| **Offset** | 消息在**某个分区内**的唯一递增序号 | 只在分区内唯一，**不是全局唯一**；由消费者维护消费位点 |
| **Producer** | 生产者，决定消息写哪个分区 | 分区策略见 Q12 |
| **Consumer / Group** | 消费者 / 消费者组 | 组内分区互斥、组间互不影响（天然支持广播 + 单播）|
| **Controller** | 集群中的「大脑」Broker，负责分区 Leader 选举、副本状态管理 | ZK 时代靠抢 ZK 临时节点选出；KRaft 时代是 Raft Quorum 的 Leader |
| **Coordinator** | **Group Coordinator**（管消费者组/offset）+ **Transaction Coordinator**（管事务） | 都是普通 Broker 承担的角色，按内部 topic 的分区 Leader 定位 |

### Q2. 为什么要分区（Partition）？分区带来了什么、牺牲了什么？

**得到的**：
1. **水平扩展**：Topic 数据分散到多台 Broker，突破单机磁盘/带宽上限。
2. **并行消费**：一个分区一个消费者，分区数 = 消费并行度上限。
3. **负载均衡**：不同分区的 Leader 分散到不同 Broker，读写压力打散。

**牺牲的**：
1. **全局有序性没了**——只保证**分区内有序**（见 Q26）。
2. **分区数不是越多越好**：① 每个分区对应至少 3 个文件句柄（.log/.index/.timeindex）× 副本数，**文件句柄与内存开销线性增长**；② Producer 端每个分区一个 batch 缓冲区，**内存放大**；③ 端到端延迟上升（每个分区都要完成复制）；④ ZK 时代 Controller failover 要逐分区选举，**分区越多故障恢复越慢**（KRaft 大幅缓解）。
3. **分区只能增不能减**，且增分区会**破坏按 key 的顺序保证**（见 Q27）。

> **白板结论**：分区数按「目标吞吐 / 单分区吞吐」估算，同时不小于「峰值消费者数」。经验值单集群万级分区量级（ZK 时代）；KRaft 后可到百万级。

### Q3. ZooKeeper 在 Kafka 里干什么？KRaft 是什么、为什么要换掉 ZK？

**ZK 时代（≤2.x）ZK 存的**：Broker 注册与存活（临时节点）、Topic/分区/副本元数据、Controller 选举（抢 `/controller` 临时节点）、ACL 与配置。
> 注意：**消费位点 offset 从 0.9 起就搬进了 Kafka 内部 topic `__consumer_offsets`**，不再放 ZK（老版本放 ZK 时写 ZK 压力大）。这是高频陷阱题。

**ZK 方案的痛点**：
- **元数据传播是「Controller 逐个 RPC 推送」**，分区多时慢，Controller 切换要重新从 ZK 拉全量元数据（分钟级）。
- 两套一致性系统（ZK 的 ZAB + Kafka 自己的复制）**心智与运维成本翻倍**。
- 元数据规模受 ZK znode 限制，分区数上不去。

**KRaft（KIP-500，2.8 预览 / 3.3 生产可用 / 4.0 移除 ZK）**：
- 若干 Broker 担任 **Controller 角色**，组成 **Raft Quorum**，元数据本身存进一个内部 topic **`__cluster_metadata`**（用 Kafka 自己的日志复制机制存元数据，自举）。
- 其余 Broker 作为 **observer 异步拉取**元数据日志并回放 → 元数据传播变成「拉日志」，天然增量。
- **收益**：Controller failover **秒级**（新 Controller 已有全量元数据在内存）、分区规模上限提升到百万级、少运维一套 ZK。

### Q4. Kafka 为什么快？（必背，五条要能各自物理解释）

| 机制 | 物理解释 |
|------|---------|
| **顺序追加写（append-only）** | 消息只追加到分区末尾，机械盘顺序写可达 600MB/s，比随机写快几个数量级；SSD 上也避免了写放大。**这是 Kafka 敢用磁盘当主存储的根因** |
| **页缓存 PageCache** | 写入只写到 OS 页缓存就返回（不主动 fsync），读也大概率命中页缓存。**不用 JVM 堆缓存**：省去对象头开销、避免大堆 GC 停顿，且**进程重启后页缓存仍在**（缓存不随进程消亡） |
| **零拷贝 sendfile** | 消费者读取时用 `FileChannel.transferTo` → 内核 `sendfile`，数据**不经过用户态**，从页缓存直送网卡（见 Q9） |
| **批量 + 压缩** | Producer 攒 `batch.size`/`linger.ms` 成一个 **RecordBatch** 再发；压缩以**批**为单位（同一批消息相似度高，压缩比远好于单条），且 **Broker 原样存储压缩批**，消费端才解压 → 网络与磁盘双双省 |
| **分区并行 + 紧凑二进制协议** | 分区把单 Topic 的吞吐横向摊开；自定义二进制协议 + 变长编码（varint）+ 相对 offset/时间戳，**协议开销极小** |

**反向追问：什么时候零拷贝会失效？** → 当 Broker 需要「碰」消息内容时：① Broker 与 Client 的**消息格式版本不一致**需要向下转换（`down-conversion`，老客户端连新 Broker 是典型坑，CPU 飙升）；② Broker 端做了**解压重压缩**（如 `compression.type` 与 Producer 不一致）；③ 开启 **SSL/TLS**（加密必须在用户态做，sendfile 用不上）。这三条是「为什么我开了 TLS 后吞吐掉一半」的标准答案。

---

## 二、存储引擎与日志（考频 ★★★★★）

### Q5. 一个分区在磁盘上长什么样？消息怎么定位？

分区目录 `topic-0/` 下是一串 **Segment（日志段）**，文件名是该段第一条消息的 **baseOffset**（20 位补零）：

```
topic-0/
├── 00000000000000000000.log          # 消息本体（RecordBatch 顺序追加）
├── 00000000000000000000.index        # 偏移量稀疏索引：相对offset → 物理位置
├── 00000000000000000000.timeindex    # 时间戳稀疏索引：时间戳 → 相对offset
├── 00000000000000368769.log          # 下一个段，baseOffset=368769
├── ...
├── leader-epoch-checkpoint           # Leader Epoch 记录（见 Q22）
└── partition.metadata
```

**为什么切段（`log.segment.bytes` 默认 1GB）**：单文件太大不利于**删除过期数据**（删段 = 删文件，O(1)）和索引查找。只有最后一个段是「活跃段」可写，其余只读。

**查 offset = 368800 的消息（三步二分 + 一次顺扫）**：
1. **二分找段**：Broker 内存里用 `ConcurrentSkipListMap<baseOffset, Segment>` 存段列表，跳表二分定位到 `368769.log` 这一段。
2. **二分找索引**：在 `.index` 里二分查找 ≤ (368800−368769) 的最大相对 offset 项，拿到一个物理起始位置。
3. **顺序扫描**：从该物理位置在 `.log` 里往后顺序扫，直到命中目标 offset。

**为什么是稀疏索引**：`log.index.interval.bytes` 默认每写 **4KB** 才建一条索引项。稠密索引内存放不下（索引文件是 **mmap** 内存映射的），稀疏索引用「少量内存 + 一小段顺序扫」换取索引常驻内存。**这是「空间换时间」的反向取舍——用一点时间换空间**。

> **索引项为什么用「相对 offset」（4 字节）而非绝对 offset（8 字节）**：段内 offset 差值不超过 int 范围，省一半索引空间。同理时间戳索引项是 8B 时间戳 + 4B 相对 offset = 12B。

### Q6. `.index` 和 `.timeindex` 分别解决什么？

- **`.index`（偏移量索引）**：`相对offset → .log 物理字节位置`，8 字节/项。服务「从 offset X 开始消费」。
- **`.timeindex`（时间戳索引）**：`时间戳 → 相对offset`，12 字节/项。服务 **`offsetsForTimes()`**——「我要消费昨天 10 点之后的数据」。查时间戳先在 `.timeindex` 二分找到 offset，再走 `.index` 那套流程。
- **实战价值**：**按时间回溯**是 ETL/数仓补数的命根子。`kafka-consumer-groups --reset-offsets --to-datetime` 走的就是这条链路。

### Q7. 日志清理策略有哪两种？compact（日志压缩）怎么工作？

`log.cleanup.policy`：

| 策略 | 规则 | 场景 |
|------|------|------|
| **delete（默认）** | 按 **时间**（`retention.ms` 默认 7 天）或 **大小**（`retention.bytes`）删除**整个过期 Segment** | 常规流式数据 |
| **compact（日志压缩）** | 按 **key** 保留**每个 key 的最新 value**，旧值被清理 | 需要「状态快照」语义：`__consumer_offsets`、CDC/binlog 表镜像、KTable |
| `delete,compact` | 两者叠加 | 既压缩又限总量 |

**compact 细节（易被追问）**：
- 由后台 **Log Cleaner** 线程做：扫描「脏段」构建 `key → 最新 offset` 的 OffsetMap，然后重写段，丢掉旧版本。
- **活跃段永远不参与 compact**（正在写）。
- **墓碑消息（tombstone）**：value = `null` 表示删除该 key。tombstone 本身也要保留一段时间（`delete.retention.ms` 默认 24h），确保所有消费者都能看到「这个 key 被删了」，之后才彻底清除。
- **compact 后 offset 不连续**（中间的旧版本被删了），消费者必须能容忍 offset 跳跃——这是正常的，别当 bug。

### Q8. Kafka 写入会立刻刷盘（fsync）吗？不刷盘为什么还敢说「持久化」？

**默认不主动 fsync。** `log.flush.interval.messages` / `.ms` 默认几乎无穷大，即**把刷盘完全交给 OS 页缓存回写**。

**为什么敢**：Kafka 的持久性**不建立在「单机落盘」上，而建立在「多副本冗余」上**——`acks=all` 意味着数据已在多台机器的页缓存里。单机断电丢的是「这台机器未落盘的部分」，其他副本还有。

**代价与边界**：如果**整个 ISR 的所有机器同时断电**（同机架/同电源），页缓存里的数据就真丢了。所以生产上要**跨机架/跨 AZ 部署副本**（`broker.rack` + 机架感知副本分配）。

> **对比追问**：RocketMQ 提供同步刷盘（`SYNC_FLUSH`）选项，Kafka 有意不推荐主动 fsync——**用「副本数」而不是「fsync」换持久性，是吞吐上的关键选择**。这条和 [PG WAL](../../infra/postgresql-wal-replication-and-amplification.md) 的「WAL 必须 fsync」形成鲜明对照：数据库单机就是真相源，MQ 靠集群冗余。

### Q9. 零拷贝到底省了哪几次拷贝？（接能力地图格 2/10）

**传统读文件发网络（4 次拷贝 + 4 次上下文切换）**：
```
磁盘 →[DMA]→ 内核页缓存 →[CPU]→ 用户缓冲区 →[CPU]→ Socket缓冲区 →[DMA]→ 网卡
```

**sendfile 零拷贝（2 次拷贝 + 2 次上下文切换）**：
```
磁盘 →[DMA]→ 内核页缓存 →[DMA gather]→ 网卡
                      （只把「位置+长度」描述符给 socket，数据不再复制）
```
- 省掉的是**两次 CPU 拷贝**和**用户态往返**——数据全程不进 JVM 堆，**不产生垃圾、不触发 GC**。
- Java 侧入口：`FileChannel.transferTo()`。
- **另一处零拷贝**：索引文件用 **mmap**（`MappedByteBuffer`）内存映射，读写索引不走 read/write 系统调用。

**辨析（必答）**：**Producer 写入路径没有零拷贝**——数据从网卡进来必须经过内核到用户态（要解析协议、校验 CRC、赋 offset），再写文件。零拷贝只发生在**消费读取路径**。

---

## 三、生产者（考频 ★★★★★）

### Q10. Producer 发送一条消息的完整链路？

```
send() → 拦截器 Interceptor → 序列化 Serializer → 分区器 Partitioner
      → RecordAccumulator（按 <topic,partition> 攒 batch，双端队列）
      → Sender 线程按 Broker 聚合成 Request → NetworkClient(NIO) → Broker
      → 响应回调 Callback / Future
```
**关键点**：`send()` 是**异步**的，只是把消息扔进 **RecordAccumulator** 缓冲区就返回；真正发送由独立的 **Sender 线程**做。所以「发送成功」必须看 Callback/Future，不能看 `send()` 返回。

### Q11. `acks` 三个取值分别什么语义？各自丢数据的场景？

| acks | 语义 | 丢数据场景 | 吞吐 |
|:----:|------|-----------|:----:|
| **0** | 发出去就算成功，不等任何响应 | 网络丢包、Broker 挂了，全丢 | 最高 |
| **1** | **Leader 写入本地日志**（页缓存）即返回 | Leader 写完立即宕机、Follower 还没拉到 → 丢 | 中 |
| **all / -1** | **ISR 中所有副本**都同步成功才返回 | 配合 `min.insync.replicas≥2` 基本不丢 | 最低 |

**陷阱**：`acks=all` **单独用是不够的**！如果 ISR 因故只剩 Leader 一个，`acks=all` 就退化成 `acks=1`。必须配 **`min.insync.replicas=2`**（Broker/Topic 级配置）——ISR 数量不足时 Producer 直接收到 `NotEnoughReplicasException`，**宁可写失败也不假装成功**。

> **不丢数据三件套**：`acks=all` + `min.insync.replicas=2`（且 `replication.factor=3`）+ `unclean.leader.election.enable=false`，再加 Producer 侧 `retries=Integer.MAX_VALUE`。

### Q12. Producer 怎么决定消息写哪个分区？

1. **指定了 partition** → 直接用。
2. **指定了 key** → `murmur2(key) % numPartitions`（**默认哈希，保证同 key 同分区** → 这是实现「按业务键有序」的基础）。
3. **无 key** →
   - **≤2.3**：Round-Robin 轮询（每条消息换一个分区 → batch 被打散，攒批效率低）。
   - **≥2.4 默认 Sticky Partitioner（KIP-480）**：**黏住一个分区攒满一个 batch 再换下一个**。整体分布仍均匀，但**批次更满 → 吞吐更高、延迟更低**。这是个很好的「批处理直觉」考点。
4. **自定义 Partitioner**：实现接口即可（如按业务字段路由、热点打散）。

### Q13. 幂等生产者（`enable.idempotence`）原理？它能保证什么、不能保证什么？

**要解决的问题**：网络抖动导致「Broker 已写入但 ACK 丢失」→ Producer 重试 → **同一条消息写了两份**。

**原理（PID + Sequence Number）**：
- Producer 初始化时向 Broker 申请一个 **PID（Producer ID）**。
- Producer 对**每个 `<PID, Topic-Partition>`** 维护一个**单调递增的序列号 seq**，随消息批一起发送。
- Broker 端为每个 `<PID, Partition>` 记录 **最后成功写入的 seq**（`lastSeq`）：
  - 收到 `seq == lastSeq + 1` → 正常接受。
  - 收到 `seq ≤ lastSeq` → **判定为重复，直接丢弃并返回成功**（去重）。
  - 收到 `seq > lastSeq + 1` → 中间有丢失，抛 `OutOfOrderSequenceException`（保证不出现空洞）。

**能保证**：单个 Producer **会话内**、**单个分区上**的 Exactly-Once 写入 + **有序**。

**不能保证**（必答的三条边界）：
1. **跨会话不保证**——Producer 重启后 PID 变了，Broker 认不出老消息（跨会话要靠**事务**的 `transactional.id`）。
2. **跨分区不保证**——PID 的 seq 是按分区独立维护的（跨分区原子性要靠**事务**）。
3. **应用层重复不保证**——业务代码自己调了两次 `send()`，Kafka 无从分辨。

> **3.0+ 默认 `enable.idempotence=true`**（同时 `acks` 自动变 all、`retries` 变 MAX）。旧版本要手动开。

### Q14. `max.in.flight.requests.per.connection` 和消息乱序是怎么回事？

- 该参数 = **一个连接上允许有多少个未收到响应的请求**，默认 5。
- **不开幂等时**：若 `>1` 且 `retries>0`，第 1 批失败重试、第 2 批已成功 → **重试的批次后写入 → 分区内乱序**。要严格有序必须设 `=1`（吞吐大跌）。
- **开了幂等时**：Broker 端靠 seq 号**给最多 5 个 in-flight 批次重新排序**（缓存乱序到达的批次），所以 **`≤5` 即可同时拿到「有序 + 高吞吐」**。>5 则幂等的有序保证失效（Producer 会直接报配置错误）。

> 白板一句话：**「幂等 + max.in.flight≤5」是既要顺序又要吞吐的正确姿势。**

### Q15. Kafka 事务是什么？2PC 怎么走？和「分布式事务」是一回事吗？

**解决的问题**：**跨分区/跨 Topic 的原子写**，以及典型的 **consume-process-produce（读-处理-写）** 场景里，把「写下游消息」和「提交上游 offset」变成一个原子操作。

**核心组件**：`transactional.id`（用户指定，跨会话稳定）、**Transaction Coordinator**（Broker 角色）、内部 topic **`__transaction_state`**。

**流程（典型 2PC）**：
1. `initTransactions()`：按 `transactional.id` 找到 Coordinator，拿到 PID 和**递增的 epoch**——**epoch 用于「隔离僵尸实例」（zombie fencing）**：老实例带旧 epoch 再来写会被拒。
2. `beginTransaction()` → 发送消息到多个分区（Coordinator 记录本事务涉及的分区列表到 `__transaction_state`）。
3. `sendOffsetsToTransaction()`：把消费位点也作为事务的一部分写入 `__consumer_offsets`。
4. `commitTransaction()`：
   - **Prepare**：Coordinator 写 `PREPARE_COMMIT` 到 `__transaction_state`（**这一刻事务结果已确定，不可回退**）。
   - **Commit**：Coordinator 向每个涉及的分区写一条 **控制消息 Control Batch（COMMIT marker）**。
   - 写 `COMPLETE_COMMIT`，事务结束。

**消费端怎么配合**：`isolation.level=read_committed` 时，消费者**只能读到 LSO（Last Stable Offset）之前**的数据——LSO 是「第一条尚未完成的事务消息」的位置。未提交事务的消息虽然物理上已在日志里，但会被消费者**过滤掉**（abort 的消息靠 `.txnindex` 中止索引跳过）。默认 `read_uncommitted` 则全读。

**和数据库分布式事务的区别（关键辨析）**：Kafka 事务**只在 Kafka 内部闭环**——它不能把「写 MySQL」和「写 Kafka」放进一个事务。跨异构系统的原子性还得靠 **本地消息表 / 事务消息（RocketMQ 半消息）/ Outbox 模式 + CDC**。

**代价**：事务有额外的 Coordinator 往返和控制消息，吞吐会掉（经验值 3%~20%，取决于事务大小）；事务开太小会放大开销。

---

## 四、副本、ISR 与高可用（考频 ★★★★★）

### Q16. AR / ISR / OSR 是什么？副本什么时候被踢出 ISR？

- **AR（Assigned Replicas）**：分区的**全部**副本。
- **ISR（In-Sync Replicas）**：**与 Leader 保持同步**的副本集合（含 Leader 自己）。
- **OSR（Out-of-Sync Replicas）**：落后的副本。`AR = ISR + OSR`。

**判定标准（0.9 之后只剩时间维度）**：`replica.lag.time.max.ms`（默认 **30s**）——**Follower 超过 30s 没有追上 Leader 的 LEO**，就被移出 ISR；追上后再加回来。

> **为什么 0.9 删掉了 `replica.lag.max.messages`（条数阈值）？** 因为条数阈值在**流量突增**时会误判：Producer 一瞬间打进 10 万条，所有 Follower 瞬时落后超过阈值 → **全部被踢出 ISR，然后追上又加回来 → ISR 反复震荡**。而时间维度对突发流量天然免疫。这是个很漂亮的「阈值该用绝对量还是时间」的设计题。

### Q17. LEO 和 HW 是什么？消费者为什么只能读到 HW？

- **LEO（Log End Offset）**：每个副本**下一条待写入**消息的 offset（= 当前最后一条 + 1）。
- **HW（High Watermark，高水位）**：**ISR 中所有副本 LEO 的最小值**。
- **规则：消费者只能消费到 HW − 1**，HW 之后的消息对消费者不可见。

**为什么**：HW 之前的消息意味着「**已被 ISR 中所有副本复制**」，即使 Leader 立刻宕机、任何一个 ISR 副本升主，这些消息都还在 → **保证消费者读到的数据不会因为主从切换而「消失」**（避免「读到了又没了」的幻读）。

**白板推演（务必能画）**：
```
Leader  : [0][1][2][3][4]   LEO=5
FollowA : [0][1][2][3]      LEO=4
FollowB : [0][1][2]         LEO=3
                            HW = min(5,4,3) = 3  → 消费者只能读 0,1,2
```

**HW 更新的两轮 RPC（延迟的来源）**：Follower fetch 时带上自己的 LEO → Leader 更新 HW → **下一次** fetch 响应才把新 HW 带给 Follower。所以 **Follower 的 HW 天然比 Leader 慢一轮**——这正是下面 Leader Epoch 要修的坑。

### Q18. 只用 HW 会出什么问题？Leader Epoch 怎么修的？（区分度题）

**问题一：数据丢失。** Follower 的 HW 更新滞后一轮，若 Follower B 刚重启（按自己的旧 HW 截断日志），此时 Leader A 宕机、B 升主 → B 已把那条消息截掉了 → **已经被 `acks=all` 确认过的消息丢了**。

**问题二：数据不一致。** 两个副本先后宕机重启，各自按本地 HW 截断，可能出现「**同一 offset 上是不同消息**」的分叉。

**Leader Epoch（0.11+ 引入）**：
- 每个副本维护 **`leader-epoch-checkpoint`** 文件，记录 `<epoch, startOffset>` 序列。**每次 Leader 变更 epoch +1**，并记下该 epoch 起始 offset。
- Follower 重启后**不再盲目按 HW 截断**，而是先向 Leader 发 `OffsetsForLeaderEpochRequest`，问「我这个 epoch 的结束位置是多少」。
- Leader 回复该 epoch 的结束 offset，Follower **按这个权威答案截断**（往往根本不需要截断）。

> 一句话：**用「谁在什么任期写了从哪开始的日志」这条权威链，替代了「靠本地 HW 猜」**。这个思路和 Raft 的 `(term, index)` 一致性检查是同源的——见能力地图格 8。

### Q19. Leader 挂了怎么选新 Leader？unclean 选举是什么？

**默认（clean election）**：Controller 从 **ISR 列表中按顺序**选第一个存活副本当新 Leader（不是投票，是 Controller 指定；ISR 里的副本本来就有全部已提交数据，无需比较日志）。

**ISR 空了怎么办**（所有同步副本都挂了）：
- `unclean.leader.election.enable=false`（**默认**）：**分区不可用**，等 ISR 中的副本恢复。选择了 **CP**（一致性优先）。
- `=true`：允许从 **OSR（落后副本）** 里选 Leader → **分区恢复可用，但落后的那段数据永久丢失**，且消费者可能看到 offset「倒退」。选择了 **AP**（可用性优先）。

> **白板结论**：这是 Kafka 里最赤裸的 **CAP 取舍旋钮**。金融/计费类必须 `false`；日志/埋点类可以 `true` 换可用性。

### Q20. 为什么 Follower 不能分担读？（对比 MySQL 读写分离）

**默认所有读写只走 Leader**，原因：
1. **一致性简单**：单一写入点 + 单一读取点 = 天然的「读己之写」和单调读，不会读到副本的滞后数据（MySQL 主从读写分离的经典难题在 Kafka 这里不存在）。
2. **Kafka 的扩展不靠副本，靠分区**——要更高吞吐就加分区，副本纯粹是冗余，职责清晰。

**例外（2.4+，KIP-392）**：支持 **Follower Fetching**——消费者可以从**同机架的 Follower** 读，目的**不是分担负载，而是省跨 AZ 流量费/降延迟**。且仍受 HW 约束，一致性不破。这是「知道边界」的加分点。

### Q21. Kafka 是 CP 还是 AP？

**「可调（tunable），默认偏 CP，但不是严格的 CP。」**

- `acks=all` + `min.insync.replicas=2` + `unclean=false` → **偏 CP**：ISR 不足时拒绝写入（牺牲 A 保 C）。
- `acks=1` + `unclean=true` → **偏 AP**：永远可写、可选主，容忍丢数据。
- 严格说 Kafka 的复制是**主从同步复制（ISR）而非多数派共识**：`replication.factor=3` 时只要 ISR 里的都确认就行，不要求「多数派」。**但 KRaft 的元数据层是真正的 Raft 多数派**——数据面和元数据面用了两套不同的一致性模型，这是个很好的追问点。

---

## 五、消费者与消费者组（考频 ★★★★★）

### Q22. 消费者组的分配规则？消费者比分区多会怎样？

**铁律：同一个消费者组内，一个分区只能被一个消费者消费**（但一个消费者可以消费多个分区）。

- **消费者数 < 分区数** → 有消费者分到多个分区（正常）。
- **消费者数 = 分区数** → 一一对应（**最优**）。
- **消费者数 > 分区数** → **多出来的消费者空转**，完全拿不到数据（**扩容失效的经典事故**：加机器不加分区 = 白加）。
- **不同组之间互不影响** → 每组都能拿到全量数据（天然支持「一份数据多方消费」，这是 Kafka 相对传统队列的核心优势）。

### Q23. 分区分配策略有哪几种？各自什么问题？

`partition.assignment.strategy`：

| 策略 | 规则 | 问题 |
|------|------|------|
| **Range（默认之一）** | **按 Topic 逐个**分配：分区数 / 消费者数，除不尽时**靠前的消费者多拿一个** | **多 Topic 时倾斜累积**——每个 Topic 都是前面的多拿，C0 会比 C2 多很多 |
| **RoundRobin** | 把**所有 Topic 的所有分区**排序后轮询分配 | 整体更均衡；但组内消费者**订阅不一致**时会分错 |
| **Sticky（0.11+）** | ①尽量均衡 ②**再均衡时尽量保留原有分配** | 减少分区迁移带来的状态丢失/重复消费 |
| **CooperativeSticky（2.4+，推荐）** | Sticky + **增量协作式再均衡** | 见 Q25 |

### Q24. Rebalance（再均衡）什么时候触发？流程是什么？

**触发条件（四条）**：
1. **组成员变化**：新消费者加入 / 消费者主动退出 / **消费者被判定崩溃**（心跳超时或 poll 超时）。
2. **订阅的 Topic 数量变化**（正则订阅匹配到新 Topic）。
3. **订阅 Topic 的分区数变化**（扩分区）。
4. Coordinator 所在 Broker 变更。

**流程（两阶段）**：
1. **FindCoordinator**：`hash(groupId) % 50` 定位 `__consumer_offsets` 的某个分区，**该分区的 Leader 所在 Broker** 就是这个组的 Group Coordinator。
2. **JoinGroup**：所有成员向 Coordinator 发 JoinGroup，Coordinator 选**第一个加入的成员为 Consumer Leader**，把成员列表和订阅信息发给它。
3. **SyncGroup**：**Consumer Leader 在客户端本地执行分配算法**（所以分配策略是客户端配置的！），把分配方案发给 Coordinator，Coordinator 再分发给每个成员。

> **设计追问：为什么分配逻辑放客户端而不是 Broker？** 让用户能自定义分配策略、Broker 保持简单无状态（Kafka 一贯的「**智能客户端 + 笨 Broker**」哲学，和 RabbitMQ 的「Broker 做路由/过滤」正相反）。
> **反转**：新版 **KIP-848（3.7 预览 / 4.0 GA）** 把分配搬回了 **Broker 端**，因为客户端分配导致协议复杂、故障排查困难、rebalance 慢。答到这里是加分项。

### Q25. Rebalance 的危害？怎么减少？Cooperative 好在哪？

**危害**：
- **Eager 协议（默认历史行为）**：再均衡开始时**所有消费者先放弃全部分区（Stop The World）**，等新方案下发才继续 → **整组消费停顿**，大组可能停顿数秒到数十秒。
- 分区易主 → 从上次提交的 offset 重新消费 → **重复消费**；有本地状态的（如窗口聚合）状态全丢。

**减少手段（面试要能报菜名）**：
1. **合理设置超时**：`session.timeout.ms`（默认 45s，3.0+）> 3 × `heartbeat.interval.ms`（默认 3s）；**`max.poll.interval.ms`（默认 5 分钟）** 是「两次 poll 之间的最大间隔」——**消费逻辑太慢导致超时是最常见的 rebalance 元凶**，配合调小 `max.poll.records`。
   > 关键辨析：**心跳线程是独立的**（0.10.1+），所以心跳正常 ≠ 消费正常；`max.poll.interval.ms` 才是防「假活着」的那道闸。
2. **静态成员 Static Membership（2.3+，KIP-345）**：给消费者配 **`group.instance.id`**，滚动重启/短暂掉线时 Coordinator **在 session 超时内不触发 rebalance**，认得出「还是原来那个实例」。**运维滚动发布必开**。
3. **CooperativeSticky（增量协作式再均衡，KIP-429）**：不再 STW，分**两轮**——第一轮只**回收需要转移的那部分分区**（其余分区**继续正常消费**），第二轮把回收的分区分配出去。**从「全停」变成「只动该动的」**。

### Q26. offset 存在哪？自动提交和手动提交怎么选？

- **存储**：内部 topic **`__consumer_offsets`**，默认 **50 个分区**、`cleanup.policy=compact`（只留每个 `<group, topic, partition>` 的最新位点）。0.9 之前存 ZK（写压力大，已废弃）。
- **自动提交** `enable.auto.commit=true`（默认，间隔 `auto.commit.interval.ms=5s`）：在 `poll()` 里顺带提交**上一批**的位点。**问题**：① 消息处理到一半崩溃 → 已提交的部分**漏消费**（若提交早于处理）；② 或提交前崩溃 → **重复消费**。**自动提交无法与业务处理对齐，生产不推荐**。
- **手动提交**：
  - `commitSync()`：同步，会重试，阻塞；
  - `commitAsync()`：异步，不重试（重试会造成位点倒退），吞吐好。
  - **最佳实践**：循环里用 `commitAsync()`，`finally` 里用 `commitSync()` 兜底。
- **`auto.offset.reset`**：没有已提交位点（新组）或位点越界时的行为——`latest`（默认，从最新开始，**新消费组会漏掉历史数据**，经典踩坑）、`earliest`（从头）、`none`（抛异常）。

### Q27. Kafka 怎么保证消息顺序？全局有序怎么办？

**保证的只有分区内有序**（append-only log，offset 单调递增）。

**做「按业务键有序」**（99% 的真实需求）：**用业务 key 发送**（如订单 ID）→ 同 key 落同分区 → 同一订单的事件严格有序。同时要保证：
- Producer 侧：开幂等 + `max.in.flight ≤ 5`（否则重试乱序，见 Q14）。
- Consumer 侧：**同一分区的消息不要丢到多线程池里并发处理**（否则消费端把顺序又打乱了）——要么单线程消费，要么按 key 再哈希到固定的处理线程。

**全局有序**：只能 **单分区**（`partitions=1`）→ **吞吐退化成单机、无法并行**，基本只在「量极小且必须全序」的场景用。**面试标准答法：反问业务是否真的需要全局有序，绝大多数只需要「按 key 有序」。**

**顺序性的三个隐藏杀手**：① **扩分区**——扩容后 `hash(key) % N` 变了，同一 key 的新消息会落到新分区，与老分区的旧消息**并发被消费 → 顺序断裂**（所以扩分区要么停写、要么接受这个窗口）；② **重试乱序**；③ **消费端并发**。

### Q28. 消费者是推（push）还是拉（pull）？为什么？

**Consumer 是 pull（主动拉取）。**

- **pull 的好处**：① **消费速率由消费者自己控制**，天然背压（push 模式下 Broker 推太快会压垮慢消费者）；② 便于**批量拉取**；③ 消费者可以**任意回溯**（seek 到任意 offset）。
- **pull 的代价**：没数据时空轮询浪费 CPU → Kafka 用 **long polling** 解决：`fetch.min.bytes`（攒够多少字节才返回）+ `fetch.max.wait.ms`（最多等多久），**Broker 端把请求挂起（Purgatory 延时队列）直到有数据或超时**。
- 注：**Broker 之间**的副本同步也是 pull（Follower 主动 fetch Leader），机制统一。

---

## 六、可靠性：不丢、不重、语义（考频 ★★★★★）

### Q29. 消息在哪几个环节会丢？分别怎么防？（最高频综合题）

| 环节 | 丢失原因 | 防护 |
|------|---------|------|
| **Producer** | `acks=0/1`；`retries=0` 遇到瞬时异常直接丢；**异步 send 未处理 Callback 异常**；缓冲区满时 `block.on.buffer.full` 处理不当 | `acks=all` + `retries=MAX` + `enable.idempotence=true` + **必须处理 Callback**；关键链路用 `send().get()` 同步确认 |
| **Broker** | ① ISR 只剩 Leader 时 `acks=all` 退化；② `unclean.leader.election=true` 选了落后副本；③ 所有 ISR 同时断电、页缓存未落盘 | `min.insync.replicas=2` + `replication.factor=3` + `unclean=false` + **跨机架/AZ 部署副本**（`broker.rack`） |
| **Consumer** | **先提交 offset 后处理业务**，处理失败消息就跳过了；自动提交在处理中途提交 | 关自动提交，**处理成功后再手动提交**（这样至少是 at-least-once，重复由业务幂等兜） |

> **白板收尾**：Kafka 只能做到 at-least-once 的可靠投递，**「不丢」靠配置，「不重」靠业务幂等**——这是所有 MQ 的共同真相。

### Q30. 三种投递语义怎么实现？Exactly-Once 到底是不是真的？

| 语义 | 实现 | 代价 |
|------|------|------|
| **At Most Once（至多一次）** | `acks=0` / 先提交 offset 再处理 | 可能丢，不会重 |
| **At Least Once（至少一次，默认）** | `acks=all` + 重试 + **先处理后提交** | 不会丢，**可能重** |
| **Exactly Once（精确一次）** | **幂等 Producer + 事务 + `read_committed`** | 吞吐下降、复杂度上升 |

**EOS 的真实边界（这是最容易吹过头的地方，必须答清）**：
- Kafka 的 EOS 成立于 **「Kafka → 处理 → Kafka」的闭环**（即 Kafka Streams / consume-process-produce 模式），靠事务把「写下游」和「提交上游 offset」原子化。
- **一旦下游是外部系统（MySQL/ES/HDFS），Kafka 管不了**——必须靠：① 下游**幂等写**（唯一键 upsert）；② 或 **offset 与业务数据存在同一事务里**（如把 offset 写进 MySQL 同一事务）；③ 或 Flink 那种 **两阶段提交 Sink（TwoPhaseCommitSinkFunction）+ checkpoint 对齐**。
- **一句话**：**端到端 EOS = Kafka 事务 + 下游幂等/事务，缺一不可。**

> **挂到我的 ETL 场景**：Spark/Flink 消费 Kafka 写 ES/Hive 时，我们实际用的是「**至少一次 + 下游按主键幂等覆盖**」，而不是 Kafka 事务——因为 Sink 不在 Kafka 里。这条能和 [ETL 链路深挖](../bigdata/08-etl-pipeline-deep-dive-20260723.md) 的幂等三处实现直接对上。

### Q31. 重复消费怎么产生？怎么解决？

**产生**：① Rebalance 时分区易主，从上次提交的位点重放；② 提交 offset 前崩溃；③ Producer 重试（未开幂等）；④ `max.poll.interval.ms` 超时被踢出组，处理完的这批位点提交失败。

**解决（只能靠消费端幂等）**：
1. **业务唯一键 + 数据库唯一索引/upsert**（最常用，最可靠）。
2. **去重表 / Redis SETNX 存 messageId**（注意 TTL 和存储成本）。
3. **状态机幂等**：只允许 `待支付 → 已支付` 这种单向流转，重复请求天然被拒。
4. **利用 `<topic, partition, offset>` 天然唯一**做去重键（比业务 ID 更省心）。

---

## 七、积压、回溯与运维（考频 ★★★★）

### Q32. 消费积压（Lag）怎么定义、怎么排查、怎么处理？

**Lag = 分区的 HW（或 LEO）− 消费组已提交 offset**。查看：`kafka-consumer-groups.sh --describe --group X`，或走 JMX / Burrow / Prometheus exporter。

**排查顺序（面试要有章法）**：
1. **是消费慢还是生产暴增？** 看生产速率曲线 vs 消费速率曲线。
2. **消费者是不是挂了 / 一直在 rebalance？** 看组状态是否 `Stable`。
3. **是不是某几个分区倾斜？** 看单分区 lag——若集中在个别分区，是 **key 分布倾斜**（热点 key）。
4. **消费逻辑瓶颈在哪？** 下游 DB 慢查询、外部 HTTP 调用、GC。

**处理**：
- **临时**：加消费者实例（**上限是分区数！**）；调大 `max.poll.records` / `fetch.min.bytes` 提高批量；消费逻辑改**批量写下游**。
- **中期**：**扩分区**（同时扩消费者）；消费端**先落库/转存再异步处理**（把慢逻辑摘出消费主循环）。
- **应急**：新建一个分区数更多的 Topic，用一个「搬运消费者」把积压数据转过去多并发消费；或**直接跳过**（`reset-offsets --to-latest`，业务能接受丢的场景，如埋点）。

### Q33. 怎么重新消费历史数据（回溯）？

**这是 Kafka 相对 RabbitMQ/传统 MQ 的杀手锏——消息不因被消费而删除，留存期内可任意重放。**

- **命令行**（组必须处于**非活跃**状态）：
  ```bash
  kafka-consumer-groups.sh --bootstrap-server ... --group g1 --topic t1 \
      --reset-offsets --to-datetime 2026-07-20T00:00:00.000 --execute
  # 其他：--to-earliest / --to-latest / --to-offset N / --shift-by -1000 / --by-duration PT1H
  ```
- **代码**：`consumer.seek(partition, offset)` 或 `offsetsForTimes(时间戳)` 后 seek。
- **换个组名**：新 group + `auto.offset.reset=earliest` → 从头消费一遍（**最简单的重放法**，也是数仓补数常用手段）。

### Q34. 分区数怎么定？为什么只能加不能减？

**怎么定**：
1. 压测单分区的生产吞吐 `Tp` 和消费吞吐 `Tc`，目标吞吐 `T` → `分区数 ≥ max(T/Tp, T/Tc)`。
2. **不小于峰值消费者实例数**（否则消费者空转）。
3. 留 2~3 倍余量（因为**扩分区破坏 key 顺序**，宁可一次到位）。

**为什么不能减分区**：删掉的分区里的数据怎么办？① 直接丢 → 违背持久性承诺；② 合并到其他分区 → **offset 会冲突、顺序被破坏、消费位点全乱**。Kafka 选择了**不支持**，把复杂性推给用户（要缩就新建 Topic 迁移）。**这是「简单性优先」的典型设计取舍**，是个很好的开放题。

### Q35. 怎么做延迟消息 / 死信队列？（Kafka 的短板）

**Kafka 原生没有延迟消息和死信队列**（RabbitMQ 有 TTL+DLX，RocketMQ 有 18 级延迟 + 原生 DLQ）。

**延迟消息的常见做法**：
1. **多级延迟 Topic**：建 `delay-5s`、`delay-1m`、`delay-10m` 等固定档位 Topic，由一个中转服务消费、判断是否到期，未到期就**暂停消费该分区（`pause()`）并 sleep**，到期后转发到业务 Topic。（**注意不能只 sleep 不 pause，否则触发 `max.poll.interval.ms` 被踢出组**——这是踩过的坑级细节。）
2. **外部时间轮 / 定时器**：消息先落库（或 Redis ZSet，score=到期时间），到期再投递到 Kafka。
3. **需要任意精度延迟就别用 Kafka**——选 RocketMQ/RabbitMQ 或 Redis ZSet（见 [redis 面试题库 Q33](../redis/redis-interview-questions-90-coverage-20260709.md)）。

**死信队列**：自己建一个 `xxx-dlq` Topic，消费失败重试 N 次后投递进去 + 告警 + 人工/定时补偿。**Kafka Connect 和 Spring Kafka 都内置了 DLQ 支持**（`errors.deadletterqueue.topic.name` / `DeadLetterPublishingRecoverer`）。

### Q36. 常见线上故障与排查（接 oncall 那条线）

| 现象 | 常见根因 | 排查/处理 |
|------|---------|----------|
| **消费组反复 rebalance** | 消费逻辑超过 `max.poll.interval.ms`；GC 停顿；网络抖动 | 调小 `max.poll.records`、把慢逻辑异步化、开 Static Membership |
| **Lag 只在个别分区涨** | key 倾斜（热点用户/租户） | 换分区策略、key 加盐打散、单独隔离热点 |
| **磁盘写满** | retention 配置过长 / 突发流量 / **`__consumer_offsets` 异常膨胀** | 临时调小 `retention.ms`、扩盘、检查 Log Cleaner 是否卡死 |
| **ISR 频繁收缩扩张** | Follower 所在 Broker IO/网络瓶颈、GC、`num.replica.fetchers` 不足 | 看 `UnderReplicatedPartitions` 指标，扩 fetcher 线程、查磁盘 |
| **CPU 突然飙高** | **消息格式向下转换（down-conversion）**——老客户端连新 Broker，零拷贝失效 | 升级客户端；`log.message.format.version` 对齐 |
| **Producer 报 `TimeoutException: Expiring N records`** | Broker 压力大 / 网络 / `buffer.memory` 满 | 看 `request.timeout.ms`、`delivery.timeout.ms`，扩容或降速 |

**必看指标**：`UnderReplicatedPartitions`（>0 就是有副本掉队，**最重要的健康指标**）、`OfflinePartitionsCount`（>0 = 有分区无 Leader，**P0**）、`ActiveControllerCount`（全集群应恒等于 1，不是 1 就是脑裂或无主）、`RequestHandlerAvgIdlePercent`（<30% 说明处理线程吃紧）、消费组 Lag。

---

## 八、选型与横向对比（考频 ★★★★）

### Q37. Kafka vs RabbitMQ vs RocketMQ vs Pulsar 怎么选？

| 维度 | **Kafka** | **RabbitMQ** | **RocketMQ** | **Pulsar** |
|------|-----------|--------------|--------------|------------|
| **定位** | 分布式**日志流平台** | 传统**消息代理**（AMQP） | 阿里系业务 MQ | 云原生**存算分离**流平台 |
| **吞吐** | **极高**（百万级/s） | 中（万级） | 高（十万级） | 高 |
| **延迟** | 毫秒级（批量攒批有权衡） | **微秒~毫秒**（最低） | 毫秒 | 毫秒 |
| **消息模型** | 分区日志，**消费不删除** | 队列，**消费即删** | 分区日志 | 分层：Topic→分片→**BookKeeper** |
| **路由能力** | 弱（只有 Topic+分区） | **强**（Exchange：direct/topic/fanout/headers） | 中（Tag/SQL92 过滤） | 中 |
| **延迟消息** | ❌ 需自建 | ✅ TTL+DLX（插件） | ✅ **原生 18 级/任意精度** | ✅ 原生 |
| **事务消息** | 内部原子写（闭环） | ❌ | ✅ **半消息 + 回查**（业务侧分布式事务） | ✅ |
| **顺序** | 分区有序 | 单队列有序 | **分区有序 + 全局有序（单队列）** | 分区有序 |
| **回溯** | ✅ **任意 offset/时间** | ❌ | ✅ | ✅ |
| **运维** | 中（KRaft 后简化） | 简单 | 中 | **复杂**（ZK+BookKeeper+Broker 三件套）|

**选型口诀**：
- **大数据/日志/流处理/需要回溯与多方消费** → **Kafka**（我的 ETL 场景就是这个）。
- **业务解耦、复杂路由、低延迟、消息量不大** → **RabbitMQ**。
- **电商交易、需要分布式事务消息/任意延迟/顺序消息** → **RocketMQ**。
- **多租户、存算分离、需要弹性扩缩容（存储与计算独立扩）** → **Pulsar**。

### Q38. Kafka vs 数据库的 WAL / LSM，有什么共通之处？（跨格串联，加分题）

- **共同点**：都是 **append-only 顺序写 + 后台整理**。Kafka 的 Segment ≈ LSM 的 SSTable，Kafka 的 **log compaction** ≈ LSM 的 **compaction**（都是「保留每个 key 的最新版本」），Kafka 的稀疏索引 ≈ SSTable 的 sparse index + block。
- **关键差异**：数据库 WAL 是**为了崩溃恢复**，必须 fsync；Kafka 的日志**本身就是数据**，靠副本而非 fsync 保证持久性（见 Q8）。
- **Kafka 是「把 WAL 变成产品」的典范**——Jay Kreps 的《The Log》一文就是这个立论：**日志是分布式系统的统一抽象**（数据库复制、状态机复制、流处理、CDC 都是同一个东西）。
- 接 [四棵树对比](../mysql/tree-structures-bplus-btree-rbtree-20260708.md) 与 `../infra/` 的 PG WAL 那条线，能力地图格 7。

### Q39. Kafka 和大数据栈怎么配合？（挂到我熟悉的 ETL 场景）

- **Kafka → Flink/Spark Streaming**：`FlinkKafkaConsumer`/Structured Streaming 直接管理 offset，**offset 存进 checkpoint 而不是 `__consumer_offsets`**（所以 Flink 作业的 lag 要从 checkpoint 或 metrics 看，`kafka-consumer-groups` 可能看不到——**踩坑点**）。
- **Exactly-Once**：Flink 靠 **Chandy-Lamport 分布式快照（barrier 对齐）+ 两阶段提交 Sink**；Spark Structured Streaming 靠 **WAL + 幂等 Sink**。都不是 Kafka 单方面提供的。
- **Kafka Connect**：无代码的 Source/Sink（MySQL CDC → Kafka → S3/ES/HDFS），比自己写消费者省事；配合 **Debezium** 做 CDC 是 Outbox 模式的标配。
- **数仓分层**：Kafka 常作为 **ODS 层的实时入口**（埋点/CDC → Kafka → 落 ODS），批流一体（Lambda/Kappa 架构）的核心枢纽。
- 详见 [大数据复习包](../bigdata/README.md) 与 [ETL 链路深挖](../bigdata/08-etl-pipeline-deep-dive-20260723.md)。

---

## 九、深水区补充（区分度题，考频 ★★★）

### Q40. Kafka 的网络模型是什么？（Reactor + 线程池，接格 2/10）

**Kafka Broker 用的是经典的 Reactor 主从多线程模型**：

```
1 个 Acceptor 线程（监听端口、接受连接）
  → N 个 Processor 线程（num.network.threads，默认3，负责读写网络、封解包）
    → RequestChannel 请求队列
      → M 个 KafkaRequestHandler 线程（num.io.threads，默认8，真正处理请求、读写日志）
        → 响应队列 → Processor 写回
```
- **`num.network.threads`** 吃网络，**`num.io.threads`** 吃磁盘/CPU；`RequestHandlerAvgIdlePercent` 低就该调后者。
- **Purgatory（炼狱）**：延时请求的暂存处，用**分层时间轮（Hierarchical Timing Wheel）** 实现——`acks=all` 的生产请求（等 Follower 同步）和 long-polling 的拉取请求都挂在这里。**时间轮 O(1) 插入/删除**，比 `DelayQueue`（O(logN)）更适合海量延时任务。这个点答出来很加分（也直接对应「延迟队列怎么实现」这类通用题）。

### Q41. Kafka 怎么做多租户/资源隔离？（Quota 限流）

- **Quota（配额）**：按 **client-id / user / (user, client-id)** 限制 **生产速率、消费速率、请求处理时间占比（request percentage）**。
- **实现方式**：**不是拒绝请求，而是「延迟响应」**——Broker 计算出应该 throttle 多少毫秒，把响应挂到 Purgatory 里延迟返回，客户端自然被拖慢。**优雅之处：不丢请求、不报错，靠背压自然降速。**
- 这是「限流不一定要拒绝，也可以排队」的好例子，能接能力地图格 8（限流熔断降级）。

### Q42. 副本是怎么分配到 Broker 上的？机架感知是什么？

- **默认分配算法**：从随机一个 Broker 开始轮询放 Leader，Follower 依次错开一个偏移量放置 → 保证 **Leader 均匀分布** 且 **同分区的副本不在同一 Broker**。
- **机架感知（`broker.rack`）**：分配时尽量把同一分区的副本**打散到不同机架/AZ**，避免整机架断电导致分区不可用。**这是 Q8「不 fsync 也敢说持久」的前提条件**。
- **重分配工具**：`kafka-reassign-partitions.sh`（扩容加机器后要手动做数据搬迁，Kafka **不会自动 rebalance 数据**——这是运维必知，也是和自动分片系统的重要区别）。

### Q43. 消息格式演进（为什么会有 down-conversion 这个坑）

- **v0/v1（旧）**：每条消息独立带 offset、CRC、时间戳，**重复字段多**。
- **v2（0.11+，RecordBatch）**：一个批次共享 baseOffset、baseTimestamp、producerId、epoch，单条记录只存**增量（varint 编码）**；批次级 CRC。**为幂等/事务提供了承载字段**，同时体积大幅缩小。
- **坑**：老客户端（<0.11）请求数据时，Broker 必须把 v2 **转回 v0/v1**（down-conversion）→ **数据要读进堆内存转换 → 零拷贝失效 + GC 压力 + CPU 飙升**。监控指标 `FetchMessageConversionsPerSec`。**升级 Broker 前先升客户端**是铁律。

### Q44. 版本关键特性速记（时效性）

- **0.9**：offset 迁入 `__consumer_offsets`、新 Consumer API、Quota、ACL。
- **0.10**：时间戳索引、Kafka Streams、心跳线程独立。
- **0.11**：**幂等 Producer + 事务（EOS）**、**消息格式 v2**、**Leader Epoch**。（**最重要的一个版本**）
- **2.4**：**CooperativeSticky 增量再均衡**、Follower Fetching（KIP-392）、Sticky Partitioner。
- **2.8 / 3.3**：**KRaft** 预览 / 生产可用。
- **3.0**：默认 `enable.idempotence=true`、`acks=all`。
- **4.0**：**彻底移除 ZooKeeper**、**KIP-848 新消费者组协议（Broker 端分配）GA**、丢弃老客户端协议。

> 面试答到「**0.11 带来幂等+事务+Leader Epoch，2.4 带来协作式再均衡，3.x 转向 KRaft，4.0 移除 ZK 并换新 rebalance 协议**」就足够了。

---

## 附录：自我思考

### 这篇的定位与局限

这是**广度优先的抽考清单**，目标「面试前一晚快速捞回 90% 考点」。按 CLAUDE.md 第三节的 depth contract，以下几处还停在**结论层**，应单独成篇深挖（动手验证才算毕业）：

- **ISR / HW / Leader Epoch（Q17~Q19）**：最该动手的一块。起 3 broker 的 docker-compose，用 `kafka-dump-log.sh` 看 `leader-epoch-checkpoint`，手动 kill leader，观察 HW 推进与截断行为，把「HW 滞后一轮导致丢数据」**真复现出来**。这是全篇区分度最高的题。
- **事务 2PC（Q15）**：用 `kafka-console-consumer --isolation-level read_committed` 对比读到的内容，`kafka-dump-log` 看 **控制消息（COMMIT marker）** 真实长什么样，验证 LSO 的推进。
- **Rebalance 三种协议（Q24/Q25）**：Go 写一个多消费者的最小实现（`segmentio/kafka-go` 或 `confluent-kafka-go`），故意让消费逻辑 sleep 超过 `max.poll.interval.ms`，观察 Eager vs CooperativeSticky 的停顿差异，再开 `group.instance.id` 对比滚动重启。
- **存储与索引（Q5/Q6）**：`kafka-dump-log.sh --files xxx.index --index-sanity-check` 直接把稀疏索引打出来看，验证「4KB 一条索引项」和三步查找。这个成本最低、收益最直观，**应该第一个做**。
- **零拷贝失效（Q4/Q43）**：开 TLS 前后压测吞吐对比，把「加密杀死零拷贝」量化出来。

### 与能力地图的挂钩

- 本篇覆盖第 **6 格「消息队列」**，基本打满：分区副本 ISR / 零拷贝+页缓存 / ack 语义 / 消费者组 rebalance / 事务与延迟死信 / 积压回溯 / 选型对比。
- **交叉链接**：
  - 顺序写 + Segment + compaction 接 **格 7 存储引擎**（LSM vs B+Tree，见 [四棵树对比](../mysql/tree-structures-bplus-btree-rbtree-20260708.md) 与 `../infra/` PG WAL）——Q38 已埋线。
  - Leader Epoch ↔ Raft 的 `(term, index)`、KRaft ↔ Raft、unclean 选举 ↔ CAP 取舍，接 **格 8 分布式系统**（未启，这条线最该接着补）。
  - 零拷贝 / 页缓存 / Reactor / epoll / 时间轮，接 **格 2 操作系统** 和 **格 10 高并发**。
  - Quota 的「延迟响应式限流」接 **格 8 限流熔断**。
  - Flink/Spark 消费 offset 管理与端到端 EOS，接 **格 13 大数据** 和 [ETL 链路深挖](../bigdata/08-etl-pipeline-deep-dive-20260723.md)。
- **暂缺**：Kafka Streams（做流处理才需要）、Connect 深入、Cruise Control 等运维生态、Pulsar 细节。按需再补。

### 一个诚实的自评

写完这篇最大的感受是：**我在 ETL 场景里用 Kafka，用到的其实只有「生产 → 消费 → 看 lag」这一层。** 真正的深水区——HW 滞后导致的截断丢数据、事务的 LSO、down-conversion 杀死零拷贝、rebalance 的三代协议演进——**这些都是「用了四年也不会自动知道」的东西**，恰好就是面试官往下追三层会撞上的地方。这印证了 CLAUDE.md 里那条铁律：**「平时在用」≠「可以跳过」**。

另一个反思：Kafka 的很多设计都是**同一个哲学的不同投影**——「**把复杂性推给客户端，Broker 保持简单**」（分配算法在客户端、消费位点自己管、不做消息过滤、不自动 rebalance 数据、分区只增不减）。理解了这条主线，很多「为什么 Kafka 不支持 X」的问题都能自己推出答案，而不用背。**但 KIP-848 把分配搬回 Broker，说明这条哲学也在被修正**——工程上没有永恒正确的取舍，只有当时约束下的最优解。

### 复习调度

按 CLAUDE.md 第五节：本篇**「进行中」**（尚未动手验证，不能算毕业）。
- **+1 天（07-24）**：盖答案自测 30 秒电梯版 + Q4/Q13/Q17/Q24/Q29/Q30 六道核心题。
- **动手项（升级为「毕业」的前置）**：① `kafka-dump-log` 看稀疏索引（Q5）；② 3 broker 复现 HW 截断（Q18）；③ Go 写消费者复现 rebalance 停顿（Q25）。做完补进正文。

### 自测题（盖住上面的答案，先自己讲一遍）

1. Kafka 为什么快？五条机制各自的物理解释是什么？**什么情况下零拷贝会失效**？
2. 一个分区在磁盘上由哪些文件组成？查 offset=X 的消息走哪三步？为什么用稀疏索引而不是稠密索引？
3. `log.cleanup.policy` 两种策略分别做什么？compact 的墓碑消息为什么不能立刻删？
4. Kafka 默认不 fsync，凭什么敢说持久化？这个承诺在什么情况下会破？前置条件是什么？
5. `acks=all` 单独用为什么不够？不丢数据三件套是哪三个配置？
6. 幂等生产者的 PID + seq 怎么去重？它**不能**保证哪三件事？
7. 开了幂等后为什么 `max.in.flight` 可以到 5 还保证有序？不开幂等时呢？
8. Kafka 事务的 2PC 走哪几步？`transactional.id` 的 epoch 防的是什么？消费端 `read_committed` 靠什么过滤未提交消息？
9. ISR 的判定标准是什么？为什么 0.9 删掉了「落后条数」这个阈值？
10. 画出 LEO / HW 的关系图。消费者为什么只能读到 HW？HW 更新为什么要两轮 RPC？
11. 只用 HW 会出现哪两个问题？Leader Epoch 怎么修的？它和 Raft 的 `(term,index)` 像在哪？
12. `unclean.leader.election` 开与不开，分别选择了 CAP 里的什么？
13. 为什么 Follower 默认不分担读？2.4 的 Follower Fetching 是为了什么（不是为了什么）？
14. Rebalance 的四个触发条件？JoinGroup/SyncGroup 两阶段各做什么？**分配算法在客户端还是 Broker**，为什么，后来又为什么变了？
15. `session.timeout.ms` 和 `max.poll.interval.ms` 分别防什么？为什么心跳正常不代表消费正常？
16. 减少 rebalance 的三个手段？CooperativeSticky 比 Eager 好在哪？
17. Kafka 怎么保证顺序？破坏顺序的三个隐藏杀手是什么？全局有序的代价是什么？
18. 消息在哪三个环节会丢？各自的防护配置？「不丢」和「不重」分别靠什么保证？
19. Exactly-Once 的真实边界在哪？端到端 EOS 需要哪两个部分配合？
20. 消费积压怎么排查（四步）？扩消费者的上限是什么？应急手段有哪些？
21. 分区数怎么估算？为什么只能增不能减？增分区会破坏什么？
22. Kafka 没有原生延迟消息，怎么实现？中转服务为什么必须 `pause()` 而不能只 sleep？
23. Kafka / RabbitMQ / RocketMQ / Pulsar 各自的定位与选型场景？
24. Kafka 的日志和数据库 WAL / LSM 的共通与差异是什么？
25. Purgatory 用什么数据结构实现延时？为什么不用 DelayQueue？
26. Quota 限流为什么是「延迟响应」而不是「拒绝请求」？
27. down-conversion 是什么？为什么它会让 CPU 飙升？怎么避免？
28. ZK 时代 ZK 存了什么、**没存**什么？KRaft 解决了哪三个痛点？

答不出的，回对应 Q 挖。
