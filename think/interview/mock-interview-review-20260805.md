# 模拟面试复盘：一场综合压力面的完整拆解 + 盲区补课 + 话术重写

> 来源：个人思考 + Claude 模拟面试实录（综合面 / 纯压力面 / 全程不点评）
> 收录时间：2026-08-05 CST
>
> **场次设定**：对标一线后端综合面，覆盖 数据服务 / 存储选型 / 支付一致性 / 消息与任务框架 / Go 运行时。
> **结论**：本场不通过。加权 ≈ **4.0 / 10**。字节口径：**2-1 基本盘，2-2 悬，3-1 够不着**（历史职级锚 1-2）。
> **定位**：这不是知识点笔记，是**一次真实的能力体检报告**。第 3 节是补课主体，第 4 节是话术重写——**后者才是 30 天内 ROI 最高的部分**。
>
> 关联：[Kafka 面试题库](./kafka/kafka-interview-questions-90-coverage-20260723.md)｜[Go 内功答案](./go/go-internals-interview-answers-20260609.md)｜[ES shard 容量 runbook](./es/es-shard-capacity-runbook.md)｜[PostgreSQL 题库](./postgresql/postgresql-interview-questions-90-coverage-20260709.md)｜[支付渠道接入](./pay/payment-channel-integration.md)｜[分布式事务范式](./pay/distributed-transaction-patterns.md)｜[ETL 链路深挖](./bigdata/08-etl-pipeline-deep-dive-20260723.md)

---

## 0. 30 秒电梯版（复习先过这段，卡壳再往下挖）

> 这场面试暴露的**不是知识点缺失，是三个模式级问题**：
>
> 1. **归因不闭环**——三次关键决策（Python→Go、ES→StarRocks、SR catalog→dump+stream load）都只讲了「换了就好了」，讲不出被替换方案**卡在哪条物理约束上**。这是 P6/2-2 与 3-1 之间最主要的分界线。
> 2. **简历数字扛不住反推**——「QPS 25→200 且 P99 5s→1s」用 Little's Law 一算就要求解释，解释不了。面试官不是在质疑造假，是在测**你是否理解自己做过的事**。
> 3. **声明的可靠性 > 实际实现的可靠性**——简历写「保证消息不丢失」，但 `acks` 没配过、offset 自动提交。这句话遇到懂 Kafka 的面试官就是必翻的坑。
>
> 而**具体盲区只有五块**，且都是一个月能补完的：Kafka 可靠性三段论、Go 运行时四件套、ES 的 query-then-fetch 与深分页放大、PG 当队列表的 HOT/膨胀、支付验签与对账。
>
> **一句话总结**：交付量已经在 3-1 的量级上，缺的**纯粹是解释能力**——不需要再攒经验，只需要把已经做过的事重新解释一遍。

---

## 1. 成绩单

| # | 方向 | 分数 | 一句话 |
|:-:|------|:--:|------|
| 1 | 数据集成服务 QPS 25→200 归因 | **3/10** | 全程在讲「我换了 Go」；最后自己承认「Python 也能做到」，等于亲手推翻了简历上的归因 |
| 2 | ES → StarRocks 榜单迁移 | **4/10** | 数据报得出（6k shard/node、300 shard/天、3TB），**机制层全空**；根因说成「shard 多」，实为索引粒度设计；回滚预案没有 |
| 3 | 支付网关与钱包 | **6/10** | 全场最好。唯一索引兜幂等 ✓、以渠道流水号为准 ✓、int64 存分 ✓。验签/防重放 ✗、渠道对账 ✗、多币种精度 ✗ |
| 4 | 自研异步任务框架 | **5/10** | 架构直觉不错（本地消息表 ✓、CAS ✓、`SKIP LOCKED` ✓、批量 200 限流 ✓）。但「保证消息不丢失」被当场证伪 |
| 5 | Go 内功 | **2/10** | 主力语言，五个追问答出零个 |

**答对的（别忘了这些是加分项，讲的时候要主动说出来）**

- 支付幂等落到**数据库唯一索引**，不是靠查询判断——这是及格线以上的做法
- 幂等键**以渠道流水号为准**，不是自己生成的 ID——很多人在这里翻车
- 金额用 **int64 存分**——没用 float
- 本地消息表 + 定时扫表重发——双写问题的标准解
- `FOR UPDATE ... SKIP LOCKED` + `Limit(200)` 批量抢占 + CAS 状态流转——PG 队列表的正解
- 不知道就说不知道——面试里这是**加分**，别改

---

## 2. 三个模式级问题（比具体知识点严重得多）

### 2.1 归因不闭环 —— 解决问题的方式是「换一个」，不是「搞明白」

三次决策，三次说不出原方案为什么不行：

| 决策 | 我当时的理由 | 面试官听到的 |
|---|---|---|
| Python → Go 重构 | 「优化不动了」 | 你没定位到瓶颈 |
| ES → StarRocks | 「shard 太多」 | 根因说错了（是索引粒度） |
| SR catalog 读 Hive → dump + stream load | 「数据丢失严重」 | catalog 是只读外表，**它不产生丢失，只会读不到**；根因没查就换方案 |

> **为什么这一条最致命**：3-1 / P7 的核心能力不是「你搞定了多少事」，而是**「面对几个方案，你凭什么选这个」**。面试官从归因质量里预测的是：**下次遇到不能换的问题（线上核心链路、合同锁死的云厂商），你会不会卡住。**

#### ✅ 归因四段式模板（以后所有「为什么换 X」都套这个）

```
① 现象  ：什么指标、什么数值、什么时候开始的
② 证据  ：我用什么工具、看到什么、排除了哪些可能
③ 根因  ：卡在哪条物理约束上（资源上限 / 算法复杂度 / 协议语义 / 架构耦合）
④ 方案与代价：常规解为什么不够 → 我选的方案 → 我为此付出了什么
```

**关键在 ④ 的后半句。** 只讲收益不讲代价 = 没做过取舍；能主动说出代价 = 有判断力。这是最便宜的加分动作。

---

### 2.2 简历数字扛不住反推

**面试官的武器是 Little's Law**：

```
系统内在飞的请求数 L = 吞吐 λ × 平均驻留时间 W
```

我写的「QPS 25、P99 5s」 → `L ≈ 25 × 5 = 125` 个并发在飞。
追问立刻来了：**一台 2C4G 的 Python 机器，怎么同时扛住 125 个在飞的请求？**

- 若是**同步 WSGI**（gunicorn 多 worker/线程）：2C4G 开不出 125 个 worker，数字不成立
- 若是 **asyncio**：请求主要在等下游 IO、不吃 CPU，**25 QPS 反而低得不正常**，说明瓶颈另有其人

**更要命的是第二个反推**：吞吐涨 8 倍的同时延迟降 5 倍。按排队论，负载升高时延迟通常**恶化**。这两个数能同时成立，只有两种可能：① 不是同一条件下测的（不同压测并发数/数据集/接口）；② 优化的是**串行结构**而非资源效率。**必须主动说清是哪一种**，否则面试官会认为所有数字都要打折。

> **行动**：简历上每一组数字，写之前先自己反推一遍——**「这个数除以那个数，等于什么？合理吗？」**

---

### 2.3 声明的可靠性 > 实际实现的可靠性

简历原文：*「保证消息不丢失、可重试、可追溯」*

实际实现：
- `acks` 没配过 → 客户端默认值（watermill 底下是 sarama，**默认 `acks=1`**）
- offset **自动提交** → 进程被 kill 直接丢一整批
- 幂等**下推给 6 个业务方**，框架层无强制

**真实的可靠性来源是 PostgreSQL 任务表兜底**（落库 → 扫表 → 超时重投 → CAS），Kafka 那条路是裸的。

> 这不是造假，是**用词不准**。改成准确的说法反而更专业——见第 4 节。

---

## 3. 盲区补课（本篇主体）

### 3.1 Kafka 可靠性三段论 ★★★★★（我答 0 分，且简历必被问）

**记忆锚**：消息可能丢在三个地方，**Producer、Broker、Consumer，一个都不能少**。

#### ① 生产端

| 参数 | 取值 | 语义 |
|---|---|---|
| `acks=0` | 发出去就算成功 | 最快，网络抖动就丢 |
| `acks=1` | **Leader 写入本地日志**即返回 | Leader 落盘后、Follower 还没同步就宕机 → **丢** |
| `acks=all`(-1) | **ISR 中所有副本**都确认 | 配合 `min.insync.replicas` 才真正不丢 |

- `enable.idempotence=true`：Producer 带 **PID + 单调递增序列号**，Broker 端按 `(PID, 分区, seq)` 去重 → 解决**重试导致的重复**（注意：只在单会话、单分区内保证）
- `retries` + `max.in.flight.requests.per.connection <= 5`：开幂等时 Kafka 会强制这个上限，否则重试会乱序
- **各客户端默认值不同，这是必答的细节**：

| 客户端 | 默认 `acks` | 默认幂等 |
|---|---|---|
| Java client（3.0+） | `all` | **开启** |
| **sarama（watermill 底座）** | **`WaitForLocal` = acks=1** | **关闭** |
| franz-go / kgo | `all` | 开启 |

> **我的服务用的是 watermill → sarama → 默认 acks=1、无幂等。**「保证不丢」在生产端第一步就不成立。

#### ② Broker 端

```
acks=all  且  min.insync.replicas=1   →  等价退化成 acks=1（ISR 里只剩 leader 也算"全部确认"）
acks=all  且  min.insync.replicas=2  且  RF=3  →  真·不丢
unclean.leader.election.enable=false  →  禁止落后副本当 leader（否则截断丢数据）
```

**不丢数据三件套**：`acks=all` + `min.insync.replicas=2`（RF=3）+ `unclean.leader.election=false`。三个缺一个都不成立。

#### ③ 消费端（我踩的就是这个）

`enable.auto.commit=true`（**默认值**）时，offset 由**后台按 `auto.commit.interval.ms`（默认 5s）在 `poll()` 调用时提交**——**和你业务处理到第几条毫无关系**。

```
poll() 拉回 500 条
  → 后台定时器把 offset 提交了（现在 broker 认为 500 条都消费完了）
  → 业务代码才处理到第 100 条
  → 进程 OOM Kill
  → 重启后从已提交的 offset 开始，剩下 400 条 永久丢失
```

**正解**：

```go
// 关闭自动提交，处理完再提交 → at-least-once
consumer.Config.Consumer.Offsets.AutoCommit.Enable = false

for msg := range claim.Messages() {
    if err := handle(msg); err != nil {
        return err          // 不提交，下次重投
    }
    session.MarkMessage(msg, "")  // 处理成功后才标记
}
```

**三种投递语义与代价**：

| 语义 | 怎么做 | 代价 |
|---|---|---|
| at-most-once | 先提交 offset 再处理 | **会丢** |
| **at-least-once** | 先处理再提交 | **会重** → 必须配业务幂等 |
| exactly-once | 幂等 Producer + 事务 + `isolation.level=read_committed` | 只在 **Kafka→Kafka 闭环**内成立；端到端仍靠下游幂等 |

> **面试口径**：「我们选的是 **at-least-once + 消费端幂等**，因为 EOS 只在 Kafka 闭环内成立，我们的下游是 PG 和第三方接口，兜不住。」

延伸细节走 [Kafka 面试题库](./kafka/kafka-interview-questions-90-coverage-20260723.md)。

---

### 3.2 Go 运行时四件套 ★★★★★（主力语言零答，硬伤）

#### ① `context` 的取消是怎么传播的

```go
type cancelCtx struct {
    Context
    mu       sync.Mutex
    done     atomic.Value          // chan struct{}，懒创建
    children map[canceler]struct{} // 子 context 集合
    err      error
}
```

**核心机制一句话**：`ctx.Done()` 返回一个 `chan struct{}`，**cancel 时不是往里写值，是 `close(ch)`**。
`close` 一个 channel 会让**所有**正在 `select` 它的 goroutine 同时被唤醒——**这是 Go 里天然的一对多广播原语**。

传播路径：cancelCtx 持有 `children`，cancel 时**沿树向下递归** cancel 所有子 context；同时把自己从父节点的 children 里摘掉（避免泄漏）。

```go
// fan-out 6 个数据源，整体 1s 超时的标准写法
func (s *Svc) Query(ctx context.Context) (*Result, error) {
    ctx, cancel := context.WithTimeout(ctx, time.Second)
    defer cancel()                       // 必须 defer，否则 timer 泄漏

    g, ctx := errgroup.WithContext(ctx)  // 任一子任务 err → 整组 ctx 被 cancel
    results := make([]Item, 6)
    for i, src := range s.sources {
        i, src := i, src
        g.Go(func() error {
            r, err := src.Query(ctx)     // ctx 一路传到 driver
            if err != nil {
                return nil               // 单源失败降级：不污染整组
            }
            results[i] = r
            return nil
        })
    }
    _ = g.Wait()
    return merge(results), nil
}
```

> ⚠️ **我当时的答法「单源失败不影响整体」是对的降级策略**，但它回避了问题本身。面试官问的是**机制**：取消信号怎么到达那个正阻塞在 socket 上的 goroutine。答案是 `close(done channel)` + `select`。

**再往下一层（DB driver 层，这一层能答出来是明显加分）**：

`database/sql` 在执行时会起一个 watcher goroutine 监听 `ctx.Done()`，触发后：

- **PG 系（pq / pgx）**：**另开一条连接**向服务端发 `CancelRequest`（PG 协议里专门的取消请求），服务端 kill 掉那条正在跑的语句
- **MySQL（go-sql-driver）**：把这条连接**标记为坏并直接关闭**

**共同点：取消不是免费的**——要么多一次网络往返，要么损失一条池化连接。所以**超时风暴会顺带打爆连接池**，这是线上真实的次生灾害。

#### ② goroutine 泄漏：无缓冲 channel 的经典写法错误

```go
// ❌ 泄漏
func bad(ctx context.Context) {
    ch := make(chan Item)              // 无缓冲
    for i := 0; i < 6; i++ {
        go func() { ch <- query() }()  // 主协程走了以后，这里永久阻塞
    }
    select {
    case r := <-ch:                    // 只收 1 个
        use(r)
    case <-ctx.Done():                 // 超时先返回 → 另外 5 个 goroutine 永远卡在 ch<-
        return
    }
}
```

无缓冲 channel 的发送**必须有接收方就位**才能完成。主协程返回后没人收 → 5 个 goroutine + 它们持有的连接/内存**永久泄漏**。

**两种修法**：

```go
ch := make(chan Item, 6)   // 修法一：缓冲区容量 = 发送方个数，发送永不阻塞

// 修法二：发送侧也 select ctx
go func() {
    select {
    case ch <- query():
    case <-ctx.Done():
    }
}()
```

**线上 10 分钟定位泄漏**：

```bash
curl 'http://ip:port/debug/pprof/goroutine?debug=2' > g.txt   # 直接看到每个 goroutine 阻塞在哪一行
go tool pprof -http=:8080 http://ip:port/debug/pprof/goroutine # 火焰图看聚集点
```
配合监控里的 `runtime.NumGoroutine()` 曲线——**只涨不跌就是泄漏**。

#### ③ 大 map 常驻堆 对 GC 的影响（我的「本地缓存配置」正踩这个）

Go 用**三色标记 + 混合写屏障**，两次 STW（mark setup / mark termination）都极短（微秒~毫秒级）。**真正的代价不在 STW，在 mark 阶段**：

- GC 每轮要**扫描所有可达对象里的指针**。几十万 key 的 `map[string]*Config`，**每个 key（string 含数据指针）+ 每个 value（指针）+ Config 内部的指针**，全都要扫
- 扫描量大 → mark 阶段 CPU 占用高 → **mutator assist** 被迫帮忙做标记 → **业务 goroutine 的延迟被拖高**（这才是线上体感变慢的原因）
- 我遇到的 **OOM**，本质是 `GOGC=100` 默认下堆目标 = 存活量 × 2，常驻大缓存把存活量抬高，堆目标翻倍撞上容器 limit

**四种优化方向**：

| 手段 | 原理 |
|---|---|
| `map[string]Config`（值类型，且 Config 内无指针） | **key/value 都不含指针的 map，bucket 分配在 noscan span，GC 直接跳过整个 map** |
| `GOMEMLIMIT`（1.19+） | 设软内存上限，逼 GC 提前触发，比扩机器省钱 |
| 分片（sharded map） | 降单个对象大小，减少单次扫描的停顿集中度 |
| 堆外缓存（freecache / bigcache） | 数据存 `[]byte` 大块，**对 GC 完全不可见** |

> **我当时的处理是「扩机器 + 减少无用 key」——方向没错（减小存活量），但说不出为什么有效。** 现在的答法：「常驻大 map 抬高了 GC 存活量基线，`GOGC=100` 下堆目标随之翻倍撞了容器 limit；减 key 是降存活量，正确的下一步是 `GOMEMLIMIT` + 去指针化。」

#### ④ `sync.Map` —— 「读多写少用 sync.Map」是流传最广也最容易翻车的经验

`sync.Map` 内部是 **read（原子只读）+ dirty（带锁可写）双 map**：

- 读命中 read → **完全无锁**（原子加载）
- 读 miss → 加锁查 dirty，`misses++`；**当 `misses >= len(dirty)` 时把 dirty 整体提升为 read**
- 写**新 key** → 必须加锁写 dirty；dirty 为 nil 时要**把整个 read 复制一份**

**所以它的真实适用场景是**：

✅ **key 集合基本稳定**的读多写少（append-only 缓存、只增不删）
✅ 多核 goroutine 读写**不相交的 key 集**（分片场景）
❌ **key 频繁新增/删除** → dirty 反复重建 + 整表复制 → **比 `RWMutex + map` 更慢**
❌ 需要遍历、需要精确 len

**我这个「配置缓存」的最优解其实都不是这两个，是 `atomic.Pointer` 整体替换**：

```go
type Cache struct{ m atomic.Pointer[map[string]*Config] }

func (c *Cache) Get(k string) *Config { return (*c.m.Load())[k] }   // 读：完全无锁，无原子竞争

func (c *Cache) Reload(fresh map[string]*Config) { c.m.Store(&fresh) } // 写：整张换掉
```

**依据**：配置是**启动加载 + 低频全量刷新**，不是逐 key 更新。这种「读极多、写极少且是全量替换」的形态，**换整张 map 的指针**比任何逐 key 的并发容器都快。

> **面试话术**：「我会先问是逐 key 更新还是全量刷新。全量刷新用 `atomic.Pointer` 换整张 map，读路径零锁；逐 key 更新且 key 集稳定才考虑 `sync.Map`；key 增删频繁我反而用 `RWMutex + map`，因为 `sync.Map` 的 dirty 提升成本会吃掉收益。」

延伸走 [Go 内功答案](./go/go-internals-interview-answers-20260609.md)。

---

### 3.3 Elasticsearch：shard 数如何**物理地**拖死集群 ★★★★

**我的现场数据**：6k+ shard/node × 16 node ≈ **21 万 shard**，总数据 **3TB**，每天新增 ~300 shard。
→ **平均单 shard ≈ 15~30MB**。官方建议 **10~50GB**。**小了三个数量级。**

#### ① 根因不是「shard 多」，是「索引粒度设计 + 无生命周期管理」

```
按「国家 × 榜单类型 × 日」建索引 + 默认 shard 数 + 带副本 + 从不删除
   = 每天 300 个碎片 shard，两年 21 万
```
**「shard 多」是症状，「粒度设计」才是根因。** 这个区别在面试里直接决定评分档位。

#### ② 开销为什么随 shard 数**超线性**增长

ES 查询是 **query-then-fetch 两阶段**：

```
Coordinating Node
   │ ① query 阶段：fan-out 到【每一个】shard
   ├──► shard 1 : 独立 Lucene index，跑一遍查询，返回局部 TopN 的 (docId, score)
   ├──► shard 2 : ...
   └──► shard N : ...
   │ ② 归并：协调节点排序取全局 TopN
   │ ③ fetch 阶段：按 docId 回到对应 shard 取 _source
```

三处随 shard 数增长：

| 位置 | 机制 |
|---|---|
| **search 线程池** | 大小 = `int(核数 × 3 / 2) + 1`，16 核 → **25 个线程**，queue 1000。6k shard 的 fan-out 远超线程数 → **大量排队甚至 `rejected`** |
| **每 shard 常驻 heap** | 每个 shard 是独立 Lucene index，有自己的 segment 元数据、terms dict FST。**「内存 90%」要看 JVM heap，不是机器内存**——ES 机器 page cache 吃满物理内存是正常态 |
| **cluster state** | 元数据随 shard 数膨胀，master 更新变慢、GC 压力大，集群操作全面变慢 |

#### ③ 深分页放大公式（榜单场景的真凶，必背）

```
翻到第 from 页、每页 size 条时：
   每个 shard 都要返回  from + size  条
   协调节点需要暂存    shard 数 × (from + size)  条
```

**6k shard 翻到第 100 页（from=1000, size=10）→ 协调节点要收 600 万条。** 这才是「查询体感 3s+」的物理解释。
（正解是 `search_after` + PIT，代价是不能跳页。）

#### ④ 常规解法：改索引模板**不需要**改后端

我当时的理由是「改模板要改 ETL 和后端逻辑」——**后端那半是错的**：

- **ES alias 支持 filter**（filtered alias）：合并索引后，读侧照样查 alias，**由 alias 定义带上粒度过滤条件**，业务代码一行不动
- **rollover + ILM**：按大小/时间自动滚动新索引，从根上杜绝碎片
- **`_shrink`**：本体是**同节点硬链接**，秒级~分钟级。真正慢的是**前置条件——要求源索引所有 primary shard 先 relocate 到同一节点**，那才是网络搬数据。我实测「一个索引几分钟」，**说明它不慢，可以后台滚着跑**

> **迁 StarRocks 这个决策本身没错**（榜单确实不需要倒排/分词，列存更省钱），**错的是给出的理由**。正确的理由是「查询形态与存储引擎不匹配 + 成本」，不是「shard 太多所以换库」。

延伸走 [es-shard-capacity-runbook](./es/es-shard-capacity-runbook.md)。

---

### 3.4 PostgreSQL 当队列表用：我天天在踩但答不出的坑 ★★★★

#### ① `FOR UPDATE ... SKIP LOCKED` 的锁只活到**事务结束**

我的代码（GORM gen）：

```go
d.getQuery().AsyncTask.WithContext(ctx).
    Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
    Where(Status.In("SCHEDULED", "RUNNING")).
    Where(UpdatedAt.Lt(expectedAt)).
    Limit(200).Find()
```

**这段本身是对的**（我确认了 `getQuery()` 传的是 tx）。但必须知道**为什么必须在事务里**：

```
如果不在显式事务里 → GORM 走隐式事务 → SELECT 执行完立即 COMMIT → 行锁当场释放
→ 下一个 worker 几毫秒后扫过来，这 200 条一条都不会被 skip → SKIP LOCKED 形同虚设
```

**正确姿势**：`BEGIN` → `SELECT ... FOR UPDATE SKIP LOCKED` → `UPDATE 置为 RUNNING` → `COMMIT`，**抢占和状态置位在同一个事务里**，锁的持有期覆盖整个抢占动作。

#### ② PG 特有病：高频状态更新 → dead tuple → 表膨胀

PG 的 MVCC 是**堆表多版本**：`UPDATE` 不是原地改，是**写一个新版本 + 把旧版本标记为死元组**。

```
pending → running → success   一行任务至少 2 次 UPDATE = 2 个 dead tuple
1000w 任务 = 2000w 死元组，autovacuum 跟不上 → 表和索引双双膨胀 → 扫表越来越慢
```

#### ③ HOT update：能救，但有前置条件

**HOT（Heap-Only Tuple）**：新版本留在**同一个数据页**，**不需要更新任何索引**，旧版本可被 page pruning **就地回收**。

> **成立条件（面试必答）**：
> 1. **被更新的列不属于任何索引的索引列**
> 2. **同一页内有足够空闲空间**（取决于 `fillfactor`，堆表默认 100）

**我的表命中不了 HOT**：因为要扫表必然在 `status` 上建索引，而每次更新的就是 `status` → **索引列被更新 → HOT 失效 → 每次都要动索引 + 留死元组**。

**四个可落地的动作**：

```sql
-- 1. 留出页内空间给 HOT（新页生效，老数据需 VACUUM FULL / 重建）
ALTER TABLE async_task SET (fillfactor = 80);

-- 2. 针对这张表调激进的 autovacuum（默认 0.2 = 涨 20% 才触发，队列表太迟）
ALTER TABLE async_task SET (
  autovacuum_vacuum_scale_factor = 0.02,
  autovacuum_vacuum_cost_limit   = 2000
);

-- 3. 扫表 SQL 的复合索引（顺序：等值列在前，范围列在后）
CREATE INDEX CONCURRENTLY idx_task_scan ON async_task (status, updated_at);
--    更省的做法：部分索引，只索引"还没终态"的行，索引体积小一个量级
CREATE INDEX CONCURRENTLY idx_task_pending ON async_task (updated_at)
  WHERE status IN ('SCHEDULED','RUNNING');

-- 4. 观测
SELECT relname, n_live_tup, n_dead_tup, last_autovacuum
FROM pg_stat_user_tables WHERE relname = 'async_task';
```

#### ④ 清理：**按时间分区 + DROP 分区**，不要 DELETE

`DELETE` 只产生更多死元组，越删越膨胀。正解是**声明式分区（按天/按周）+ 直接 `DROP TABLE` 老分区**——`DROP` 是 O(1) 的元数据操作，不产生垃圾。

#### ⑤ 顺带修掉：超时重投的「两份同时在跑」

我承认没考虑。标准解是 **租约 + 心跳续期（lease + heartbeat）**：

```
worker 抢到任务 → 写 lease_until = now() + 30s
执行中每 10s 续期一次 lease_until
扫描器只捞 lease_until < now() 的任务（而不是"running 超过 N 分钟"）
worker 提交结果时用 CAS：UPDATE ... WHERE id=? AND owner_id=? AND lease_until > now()
                        → 影响行数 = 0 说明租约已被抢走，本次结果丢弃
```
**关键是最后那步 CAS**：它保证即使真的跑了两份，**也只有持有有效租约的那份能写回结果**。

延伸走 [PostgreSQL 题库](./postgresql/postgresql-interview-questions-90-coverage-20260709.md)。

---

### 3.5 StarRocks：我答了但没答透的几处 ★★★

| 我的答案 | 正确/补充 |
|---|---|
| FE 选主「Raft 我记得是」 | ❌ **StarRocks / Doris 的 FE 元数据复制用的是 BDBJE**（Berkeley DB Java Edition），类 Paxos 的复制协议，不是 Raft。FE 要奇数台是为了多数派选主 |
| 表模型「Unique Key」→「组合 Primary Key」 | 这是**两个不同的表模型**，别混：**Unique Key = merge-on-read**（查询时做版本合并，读放大）；**Primary Key = delete + insert，读时无合并**，覆盖写 + 高频查询的榜单场景应选 **Primary Key**（代价是主键索引常驻内存） |
| 副本 2 份（省磁盘） | ⚠️ StarRocks 写入要**多数派确认**：2 副本时 quorum = 2，**挂任意一台 BE，它上面的 tablet 直接不可写**。偶数副本在可用性上**比 3 副本更差**，省的磁盘换的是可用性——这个取舍要**主动说出来**，说不出就是没想过 |
| 分桶数 = BE × 2 = 6 | 3TB / 2 年 ≈ 4GB/天，按天分区 ÷ 6 桶 ≈ **单 tablet 700MB**，官方建议 **1~10GB**。**分桶数应该按「数据量 / 目标 tablet 大小」倒推，而不是按节点数拍**——否则就是在 SR 上重演 ES 的碎片化 |

**Stream Load 的幂等**（当时没答）：Stream Load 支持 **`label`**，同一个 label 在保留期内只会成功导入一次，重复提交返回「label already exists」。**这是批失败重试时保证不重复写入的官方机制**——3TB 切批导入必须用它。

---

### 3.6 支付：欠的三块 ★★★★

#### ① 回调验签与防重放（我完全没答）

**回调是不可信输入，永远不要相信报文里的金额和状态。**

| 渠道 | 验签机制 | 防重放 |
|---|---|---|
| **微信支付 APIv3** | 用**微信支付平台证书**验 `Wechatpay-Signature`（SHA256-RSA）；报文体用 APIv3 密钥做 **AES-256-GCM** 解密 | `Wechatpay-Timestamp` 与本地时差 **> 5 分钟拒绝** + `Wechatpay-Nonce` 去重 |
| **支付宝** | **RSA2**（SHA256WithRSA），用支付宝公钥验签；参数按字典序拼接 | `notify_id` 去重 + 可回调 `notify_verify` 二次确认 |
| **Stripe** | `Stripe-Signature` 头，用 endpoint signing secret 做 **HMAC-SHA256** | 签名串里含 `t=` 时间戳，SDK 默认 **tolerance 5 分钟** |

**三层防线（这是标准答案的结构）**：

```
第 1 层：验签 —— 证明报文来自渠道，没被篡改
第 2 层：时间戳 + nonce —— 挡住原样重放（超出时间窗直接拒）
第 3 层：幂等表唯一索引 —— 即使前两层被绕过，也只加一次钱  ← 我做到了这层
        UNIQUE KEY (channel, channel_trade_no)
第 4 层：金额与状态【以主动查询渠道为准】，不信回调报文里的值
```

> **我的答案只有第 3 层。** 第 3 层是最后的物理防线（做对了很关键），但**只有第 3 层意味着任何人都能拿旧报文打你的接口**——虽然不重复加钱，但会触发状态机流转、日志污染、甚至把已退款订单打回成功。

#### ② 幂等：check-then-act vs 唯一索引

```go
// ❌ TOCTOU：两条相同回调并发进来，都查到"未处理"
if exists, _ := dao.Exists(ctx, tradeNo); exists { return }
dao.Insert(ctx, record)

// ✅ 让数据库的唯一索引来裁决
err := dao.Insert(ctx, record)
if errors.Is(err, gorm.ErrDuplicatedKey) {   // PG: 23505 unique_violation
    return nil                               // 已处理过，幂等返回
}
```
**「查询判断是否已处理」不是幂等，是「大概率不重复」。** 幂等必须落在**数据库的物理约束**上。（我实际有唯一索引，✅；但讲的时候先讲了查询判断，**讲错了顺序，等于把加分项讲成了减分项**。）

#### ③ 渠道对账（日终）

```
拉渠道对账单 → 与本地流水按 (渠道, 渠道流水号) 全量比对 → 分四类处理：

① 渠道有、我方无（漏单）  ：钱真到了但系统没记 → 补单 + 告警
② 我方有、渠道无（挂单）  ：系统记了但渠道没有 → 大概率我方状态置错/渠道未成功 → 冲正
③ 金额不一致              ：【以渠道为准】，差异挂"待处理差异账"，人工介入
④ 状态不一致              ：主动调渠道查询接口二次确认，以查询结果为准
```
术语上，实收 > 应收叫**长款**，实收 < 应收叫**短款**。**核心原则：对账以渠道为准，差异必须落表可追溯，绝不静默修正。**

#### ④ 多币种精度（我承认没考虑）

`int64` 存「分」是对的，但 **×100 这个常数在跨境场景是错的**：

| 币种 | 最小单位指数 | ×100 会怎样 |
|---|---|---|
| USD / CNY / EUR | 2（分） | 正确 |
| **JPY / KRW**（zero-decimal） | **0**（没有"分"） | **Stripe 对 JPY 传的就是整数日元，×100 会多算 100 倍** |
| **KWD / BHD / TND** | **3** | ×100 会少算 10 倍 |

**正解**：存 `(amount_minor int64, currency char(3))`，**指数从币种表查（ISO 4217 exponent），不要硬编码 100**。
换汇除不尽的差额：**约定舍入方向（一般对平台有利方向舍入）+ 差额单独记「汇兑损益」科目**，绝不能让它凭空消失——**「那几厘钱去哪了」是财务合规问题，不是技术问题**，答不出会被认为没做过真支付。

延伸走 [支付渠道接入](./pay/payment-channel-integration.md)。

---

## 4. 话术优化：逐条 before / after ★★★★★（30 天内 ROI 最高）

> **同样的经历，讲出来差一个职级。** 下面每一条 after 都只用了我**真实做过的事**，没有夸大。

### 4.1 数据集成服务重构

> ❌ **Before**：「老服务是 Python 写的，压测只有 25 QPS，用 py-spy 看了火焰图，优化不动了，就用 Go 重构了，QPS 到 200+。」
>
> ✅ **After**：
> 「先说结论：**这次收益的主要来源是查询编排从串行改并行，不是换语言**——语言换 Go 是顺带做的工程收益（团队主力语言统一、类型安全、并发原语更好用）。
> 定位过程是：老服务是 Python，压测 25 QPS、P99 5s。按 Little's Law 反推，系统内在飞的请求约 125 个，而机器只有 2C4G——说明请求**绝大部分时间不在 CPU 上，而是串行等下游 IO**。py-spy 火焰图也印证了这一点：CPU 占用不高，时间花在等待上。
> 所以真正的瓶颈是**编排结构**：一个请求要查 N 个数据源，原来是串行的，总耗时是 N 段 IO 之和。改成并发 fan-out 之后，总耗时收敛到最慢的那一段；再把配置从远程读改成进程内缓存，砍掉每请求固定的几次远程调用。
> **代价我也说一下**：并发 fan-out 会把下游的瞬时并发放大 N 倍，所以我加了并发上限控制；本地缓存配置带来了配置下发的一致性窗口，靠 [具体机制] 收敛。
> 数字口径：200+ QPS 和 P99 < 1s 是重构后同一套压测环境下测的，跟 25 QPS 那次的并发梯度不完全一致，严格说不能直接除。」

**为什么这个版本能拿分**：① 主动纠正了归因（不邀语言之功）；② 用了定量推理；③ 主动讲代价；④ **主动交代数字口径**——这一条尤其加分，面试官会认为你诚实且严谨。

### 4.2 ES → StarRocks

> ❌ **Before**：「ES 集群 CPU 90%、内存 90%，查询 3s+，排查发现 shard 数量过多，就把榜单迁到 StarRocks 了。」
>
> ✅ **After**：
> 「现象是 16 台 16C64G 的 ES 集群查询体感 3s+。**先纠正一个我们当时的观测误区：机器内存 90% 不是病征**——ES 会用 page cache 吃满物理内存，该看的是 JVM heap。
> 真正的问题在 shard：单节点 6000+ shard（默认上限是 1000，被前人一路调上来的），全集群约 21 万个 shard，而总数据只有 3TB——**平均单 shard 30MB，比官方建议的 10~50GB 小三个数量级**。
> **根因不是「shard 多」，是索引粒度设计**：按「国家 × 榜单类型 × 日」建索引、默认分片数、带副本、且**没有 ILM 生命周期管理**，两年只进不出。
> 它怎么拖垮查询的：ES 是 query-then-fetch，query 阶段要 fan-out 到**每一个** shard，而 16 核的 search 线程池只有 25 个线程；再叠加榜单的深分页——**协调节点要暂存 `shard 数 × (from+size)` 条**，这是超线性放大。
> **常规解**（合并索引模板 + rollover/ILM + filtered alias 让读侧无感 + 后台滚 shrink）**是能解决的，我们评估过**。最终选择迁 StarRocks，理由不是 ES 修不好，而是**查询形态和存储引擎不匹配**：榜单是排序聚合查询，根本用不到倒排和分词，列存 + 向量化在这个形态上是数量级优势，而且能顺带砍机器。
> 结果：集群 CPU 90%→15%，查询 < 1s，机器 16→8 台，月成本净省约 3 万。
> **代价**：换存储引擎的一致性验证成本远高于 ES 内部重建，我们的对账只做到了 count + 抽样比对，**没做全量校验，也没准备回滚预案——这是当时留下的风险**。」

**为什么这个版本能拿分**：① 主动纠错（内存指标）；② 根因说到了设计层；③ 讲清了机制；④ **主动说「常规解能解决，但我选了另一条路，理由是 X」——这就是技术判断力的展示方式**；⑤ 主动暴露风险。**主动说出自己方案的不足，是资深候选人最强的信号之一。**

### 4.3 异步任务框架

> ❌ **Before**：「自研异步任务框架，保证消息不丢失、可重试、可追溯。」
>
> ✅ **After**：
> 「准确地说，它提供的是 **at-least-once 语义 + 全链路可追溯**，不是 exactly-once。
> 可靠性的落点在**本地消息表**，不在 Kafka：业务事务里先落一条任务记录，再投递消息。所以即使 Kafka 那一跳完全失败，定时扫描器也会把它捞回来重投——**Kafka 在这里是「快通道」，PG 任务表才是「真相源」**。
> 抢占用的是 PG 的 `FOR UPDATE ... SKIP LOCKED` + `LIMIT 200` 批量拉取，多 worker 可以真并发抢任务而不互相阻塞；状态流转用 CAS（`WHERE status = 'pending'` 看影响行数），保证同一任务只被推进一次。
> 延时任务走 asynq + Redis ZSET（秒级精度，业务够用），Redis 开了 RDB + AOF，同时任务本身在 PG 有落表兜底。
> **我知道的不足有三处**：① Kafka 侧目前是 sarama 默认配置（acks=1、offset 自动提交），可靠性实际由任务表兜着，**这是我接下来要收敛的**；② 超时重投目前没有租约机制，理论上存在同一任务两份并发执行，靠业务幂等兜；③ 框架层没有强制幂等，是下推给接入方的。」

**为什么这个版本能拿分**：把「保证不丢」这个会被证伪的表述，换成了**准确的语义术语 + 清晰的可靠性来源 + 主动列出三处不足**。**面试官对「自己知道自己的系统哪里不行」的候选人评价极高**——因为这说明你真的理解它。

### 4.4 通用：被问到「你不知道的东西」

> ❌ **Before**：「ack 设的多少是指什么？」（暴露没配过，且没有补救）
>
> ✅ **After**：「`acks` 这个参数我确实没有主动配过，用的是客户端默认值。不过我现在意识到 watermill 底下是 sarama，**sarama 默认是 acks=1**，也就是 leader 落盘就返回——leader 宕机且 follower 未同步时会丢。我们能扛住是因为有本地消息表兜底，但这是**用架构补掉了配置的缺失**，不是配置本身对。」

**模板**：`承认 → 补上我现在知道的 → 说清它对我系统的实际影响`。**这个结构能把一个「不会」变成半个「会」。**

---

## 5. 简历必改（面试官就是照着这几句挖的）

| 现在的写法 | 问题 | 建议改成 |
|---|---|---|
| 单机 QPS 从 25 提升至 200+，P99 5s 降至 1s 内 | 两数放一起可被 Little's Law 反推；归因给了"重构" | 拆开写贡献项：「**查询编排串行改并发 + 配置本地化**，P99 5s → 1s」；QPS 若保留，注明同口径压测 |
| 经排查定位因为 **shard 数量过多** | 根因说错（是索引粒度设计） | 「**索引按国家×榜单×日切分且无 ILM，两年累积 21 万个 ~30MB 碎片 shard，深分页下协调节点归并放大严重**」 |
| 保证**消息不丢失**、可重试、可追溯 | 当场可被证伪 | 「基于**本地消息表 + 状态机 CAS + SKIP LOCKED 抢占 + 超时重投**，实现 **at-least-once 语义**与全链路可追溯」 |
| 期间**无数据丢失** | 追问校验方式时只有 count + 抽样 | 补校验口径：「按天×国家分组 count/sum 双向比对」 |
| 设计钱包系统，支持充值、对账、转账、提现 | "对账"会被追问长短款 | 要么补上「**渠道日终对账 + 差异挂账**」，要么在准备时把 3.6 ③ 背下来 |

---

## 6. 自测题（盖住答案，先自己讲一遍）

### 毕业标准
> 不是「看过」，而是：**下面 15 题能在白板上讲清、并扛住一层追问**。任一题卡壳 → 回对应章节重挖。

**Kafka**
1. `acks` 三种取值各是什么语义？`acks=all` 在什么配置下会退化成 `acks=1`？
2. 「不丢数据三件套」是哪三个参数？
3. `enable.auto.commit=true` 时 offset 在**什么时机**被提交？为什么它和「不丢消息」冲突？画出丢消息的时序。
4. sarama 的默认 `acks` 是多少？Java client 3.0+ 呢？
5. 为什么说 EOS 只在「Kafka → Kafka」闭环内成立？

**Go**
6. `ctx.Done()` 返回什么？cancel 时对它做了什么操作？为什么这个操作天然是「一对多广播」？
7. DB driver 收到 ctx 取消后做了什么？PG 和 MySQL 的做法有什么不同？取消的**代价**是什么？
8. 主协程超时返回后，还在往无缓冲 channel 写的 6 个 goroutine 会怎样？两种修法各是什么？
9. 几十万 key 的 `map[string]*Config` 常驻堆，对 GC 的影响链条是什么？什么条件下 GC 会**完全跳过**一个 map？
10. `sync.Map` 的 read/dirty 是怎么工作的？什么场景下它**比 `RWMutex+map` 慢**？配置缓存的最优解是什么？

**ES / PG / SR / 支付**
11. 写出 ES 深分页时协调节点的内存放大公式。为什么 search 线程池是瓶颈？（给出线程数公式）
12. HOT update 的两个前置条件是什么？为什么我的任务表命中不了？四个补救动作？
13. `FOR UPDATE SKIP LOCKED` 的锁活多久？不在显式事务里会发生什么？
14. 支付回调的四层防线是什么？只有第 3 层（唯一索引）会有什么后果？
15. JPY 的最小单位指数是多少？`int64 × 100` 存金额在跨境场景会出什么事故？

<details>
<summary>答案索引（先自己讲完再展开）</summary>

1→§3.1① ｜ 2→§3.1② ｜ 3→§3.1③ ｜ 4→§3.1① 表 ｜ 5→§3.1③ 表
6→§3.2① ｜ 7→§3.2① 末 ｜ 8→§3.2② ｜ 9→§3.2③ ｜ 10→§3.2④
11→§3.3②③ ｜ 12→§3.4③ ｜ 13→§3.4① ｜ 14→§3.6① ｜ 15→§3.6④

</details>

---

## 7. 30 天补课路线（按 ROI 排序）

| 优先级 | 内容 | 工时 | 为什么排这个位置 |
|:--:|---|:--:|---|
| **P0** | **重写第 4 节的四段话术，录音自己讲一遍** | 1 天 | **故事我已经有了，缺的只是机制层的话术。全场性价比最高——不需要新知识** |
| **P0** | Kafka 可靠性三段（§3.1） | 1 天 | 被打得最狠、简历必被问、且一天就能补完 |
| **P1** | Go 运行时四件套（§3.2） | 3 天 | **主力语言答不出是硬伤，其他题答得再好也救不回来** |
| **P1** | 简历改写（§5） | 0.5 天 | 改完就少挨三个追问 |
| **P2** | PG MVCC / HOT / 膨胀 / autovacuum（§3.4） | 2 天 | 任务表正在踩，讲出来是加分项；顺带把租约机制补上（这是能落地的真改进） |
| **P2** | ES 机制层（§3.3） | 1 天 | 配合 §4.2 话术一起过 |
| **P3** | 支付验签 / 对账 / 多币种（§3.6） | 1 天 | 简历强项，补完可以变成"主动展开"的题 |
| **P3** | StarRocks 表模型 / 副本 / tablet（§3.5） | 0.5 天 | 补齐即可，考频低于上面几项 |

**复习节奏**（按 CLAUDE.md 第五节）：毕业后 **+1 天 / +7 天 / +30 天** 各回看一次第 6 节自测题。

**验收标准**：
> 找人（或让 AI）**照着我的简历重新压一遍**，第 6 节 15 题全过 + 第 4 节四段话术能脱稿讲完 = 毕业。
> 目标定级：**字节 2-2 稳，3-1 值得赌一把**。

---

## 附录：自我思考

### 一、最扎心的一条：我的强项恰恰是我讲不出来的

评估里那句「**你输的全部是讲不出底层**」是准确的。这场面试里我答对的每一条——唯一索引兜幂等、以渠道流水号为准、int64 存分、本地消息表、`SKIP LOCKED`、CAS 状态机——**都是真做过的、且都是对的选择**。但我讲的时候，把它们讲成了"我们就是这么做的"，而不是"我们为什么必须这么做"。

**同一件事，两种讲法，差一个职级。** 这个认知比这次的分数重要得多。

### 二、「换一个」这个习惯是怎么养成的

复盘三次决策，我发现共同点：**我都是在时间压力下做的选择，而"换一个"是当时最快能验证的路径。** 创业公司里这个决策模式很可能是对的——ROI 就是最高的。

但它有个隐性代价：**每次"换一个"成功，我就少一次"挖到底"的机会。** 四年下来，交付量上去了，但**能讲清的底层机制没有同步增长**。这才是"会用和会讲之间隔着一整层"的真实成因——不是我懒，是我的工作模式在系统性地奖励"换一个"。

**所以补课不能只补知识点，得改一个习惯**：以后每次决策，哪怕最终还是"换一个"，也要**先写下被替换方案卡在哪条物理约束上**。写不出来，就说明这次换得没道理，只是运气好。

### 三、关于诚实

全场我说了好几次「不知道」「确实没考虑到」「坦白说」。评估里说这是加分——我信这个判断，但也要警惕：**诚实只能救"不会"，救不了"讲不清自己做过的事"**。前者面试官会觉得你可信，后者他会觉得你没想过。

这次有一处是我自己把加分项讲成了减分项：**支付幂等，我其实有唯一索引，但先说了"查询判断是否已处理"**。顺序讲反，面试官第一反应是"这人不懂 TOCTOU"。**讲技术方案时，要先讲物理约束，再讲应用层逻辑**——因为面试官在找的是"你知不知道最终由谁裁决"。

### 四、职级这件事

从字节 1-2 走的，这个锚是客观存在的，但它反映的是"我还没来得及被评"（正职满打满算一年，没赶上完整晋升周期），不是能力天花板。

真正能打破锚定的只有一样：**责任量级 + 讲得清**。前者我已经有了——创业公司后端负责人、25w 用户、0→1 建支付/权限/任务框架、独立扛 20PB 跨云迁移，这个责任面在大厂内部是 2-2 甚至 3-1 才碰得到的活。

**所以我的天花板不在职级历史上，在于我今天答不出 `acks` 和 `context` 怎么传播。前者改不了，后者一个月就能改。**
