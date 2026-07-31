# 分布式事务编程范式

> 状态：草案
> 日期：2026-07-31
> 适用范围：任何需要跨系统保持一致性的 Go 服务
> 配套文档：[三方支付对接后端链路规范](./third-party-payment-integration.md) §2 讲**有哪些模式**及选型；本文讲这些模式**落到代码里长什么样**

---

## 0. 这份文档的角度

2PC、TCC、Saga、Outbox 这些名字讲的是"策略"。但真正写代码时，你面对的问题是具体的：

- 这个状态该存在哪、由谁改？
- 重启之后怎么知道该从哪继续？
- 补偿失败了怎么办？
- 并发跑两个实例会不会重复执行？

这份文档回答的是这些。看完应该能直接动手写，而不是还要再翻一遍论文。

**核心命题先放在最前面：**

> **最实用的分布式事务范式没有响亮的名字——把每一个跨系统操作建模成一个「有状态机的持久化实体」，再写一个幂等的推进器去驱动它。**

Saga、TCC、Outbox 都是这个基本形态的特化。理解了它，那些名字就只是变体。

本项目其实已经在用了：`async_tasks` 表（`SCHEDULED → RUNNING → FINISHED / FAILED / TERMINATED` + 每分钟重试 job）、`withdrawal_order` 表（`PENDING → COMPLETED / CANCELLED / FAILED`）都是这个形态。只是推进器有 bug，而最需要它的 `order` 表反而没有状态机。

---

## 1. 四个共享构件

不管你用哪种模式，代码里一定有这四样。**缺任何一个，这个"事务"都是假的。**

### 1.1 幂等键 + 数据库唯一约束

幂等**必须落在数据库约束上**，不能只靠代码里的 if 判断——判断和写入之间永远有窗口。

```go
// ✅ 靠唯一约束
func (r *Repo) ClaimEvent(ctx context.Context, eventID string) (bool, error) {
    err := r.db.WithContext(ctx).Create(&InboxEvent{EventID: eventID}).Error
    if errors.Is(err, gorm.ErrDuplicatedKey) {
        return false, nil          // 已处理过
    }
    return err == nil, err
}

// ❌ 靠先查后写
exists, _ := r.Exists(ctx, eventID)   // ← 这里到下一行之间，别人可能也查到了 false
if !exists { process() }
```

幂等键取值：入站用渠道给的 `event_id` / `transaction_id`；自己发起的用业务单号；实在没有就用「业务类型 + 关联单号」的组合唯一索引。

### 1.2 显式状态机 + CAS 迁移

**迁移表要是数据，不是散落各处的 if。**

```go
type Status int

var allowedTransitions = map[Status][]Status{
    Created:    {Processing, Failed},
    Processing: {Succeeded, Failed},
    Succeeded:  {},                    // 终态，不可迁出
    Failed:     {},
}

func CanTransit(from, to Status) bool {
    for _, s := range allowedTransitions[from] {
        if s == to { return true }
    }
    return false
}
```

迁移动作用 CAS，靠 `RowsAffected` 判断"是不是**我**改成功的"：

```go
// 返回 false 有两种含义：别人先改了（重复），或这条迁移非法（乱序）
// 两种都应该"什么都不做并返回成功"，而不是报错重试
func (r *Repo) Transit(ctx context.Context, id string, from []Status, to Status) (bool, error) {
    res := r.db.WithContext(ctx).
        Model(&Order{}).
        Where("id = ? AND status IN ?", id, from).
        Update("status", to)
    if res.Error != nil {
        return false, res.Error
    }
    return res.RowsAffected == 1, nil
}
```

⚠️ **`RowsAffected == 0` 的语义依赖 READ COMMITTED 隔离级别。** 提到 REPEATABLE READ 后 PostgreSQL 会抛序列化错误而不是静默返回 0 行，那时必须捕获重试。详见规范文档 §1.4。

### 1.3 补偿动作必须可重入

补偿一定会被重复调用，所以它自己也要幂等。而且必须能处理**"要补偿的东西根本不存在"**——因为正向操作可能压根没执行成功。

```go
func (s *Service) Compensate(ctx context.Context, gtxID string) error {
    return s.tx.Do(ctx, func(db *DB) error {
        br, err := db.GetBranchForUpdate(ctx, gtxID)
        if err != nil { return err }

        if br == nil {
            // 空补偿：正向操作没执行过。插一条终态占位记录，
            // 让晚到的正向操作能发现"已被取消"
            return db.InsertBranch(ctx, gtxID, StatusCompensated)
        }
        if br.Status == StatusCompensated {
            return nil                 // 幂等：已补偿过
        }
        return s.doCompensate(ctx, db, br)
    })
}
```

### 1.4 后台推进器

这是**最容易被省掉、也最不能省**的一个。

同步链路只负责"发起并记录"，真正保证系统收敛到终态的是后台那个循环。没有它，任何一次网络抖动都会留下一条永远卡住的记录。

---

## 2. 形态一：状态机 + Reconciler

**默认选择。** 适合"推进到终态"型的流程——你不关心走了几步，只关心最终是否达成。

思想来自 Kubernetes 的 reconcile loop：**推进器不记忆"上次做到哪"，只看当前状态决定下一步**。所以它天然可重入、可重启、可并发。

### 2.1 骨架

```go
func reconcileOnce(ctx context.Context) {
    // 只捞未达终态的；必须有 limit，多实例要用 SKIP LOCKED
    orders, err := repo.ListPendingForUpdate(ctx, 200)
    if err != nil {
        metrics.ReconcileError.Inc()
        return
    }

    for _, o := range orders {
        next, action := decide(o)      // 纯函数，见下
        if action == nil {
            continue                    // 无需推进
        }

        if err := action(ctx, o); err != nil {
            repo.ScheduleRetry(ctx, o.ID, backoff(o.Attempts))
            continue
        }

        // CAS：并发的另一个实例可能已经推进过了
        ok, err := repo.Transit(ctx, o.ID, []Status{o.Status}, next)
        if err != nil {
            continue                    // 下一轮重来，动作是幂等的所以安全
        }
        if ok {
            publishOutbox(ctx, o.ID, next)
        }
    }
}
```

### 2.2 关键：decide 必须是纯函数

```go
// 给定状态返回下一步，不做 IO、不改状态
func decide(o Order) (Status, func(context.Context, Order) error) {
    switch {
    case o.Status == Created && o.ExpireAt < now():
        return Closed, nil                       // 纯状态迁移，无外部动作
    case o.Status == Processing:
        return Succeeded, queryChannelAndVerify  // 需要外部动作
    default:
        return o.Status, nil
    }
}
```

纯函数带来三个好处：可以**穷举单元测试**（把状态空间全跑一遍）、可以安全地重复调用、出问题时可以脱离环境复现。

### 2.3 多实例并发

```sql
SELECT * FROM orders
 WHERE status IN ('created','processing')
   AND next_retry_at <= now()
 ORDER BY id
 LIMIT 200
 FOR UPDATE SKIP LOCKED;      -- ← 关键：多实例各取一批，互不阻塞
```

`SKIP LOCKED` 比分布式锁更好：不需要额外组件，且多实例能**并行**处理而不是排队。

### 2.4 退避

```go
func backoff(attempts int) time.Time {
    // 指数退避 + 抖动，避免下游恢复瞬间被同时打爆
    d := time.Duration(math.Pow(2, float64(attempts))) * time.Second
    if d > 30*time.Minute { d = 30 * time.Minute }
    jitter := time.Duration(rand.Int63n(int64(d / 4)))
    return time.Now().Add(d + jitter)
}
```

固定间隔重试是常见错误——下游宕机时你会把它捶得更惨，自己的日志也被刷爆。

---

## 3. 形态二：编排式 Saga

适合**步骤多、顺序固定、每步都能语义补偿**的流程。

与形态一的区别：Saga 关心"走到第几步"，所以**游标必须持久化**，否则进程重启就不知道该补偿什么。

```go
type Step struct {
    Name       string
    Do         func(ctx context.Context, s *SagaState) error
    Compensate func(ctx context.Context, s *SagaState) error
}

func Run(ctx context.Context, saga *SagaState, steps []Step) error {
    for i := saga.Cursor; i < len(steps); i++ {   // 从持久化游标继续
        if err := steps[i].Do(ctx, saga); err != nil {
            saga.Phase = PhaseCompensating
            repo.Save(ctx, saga)
            return compensate(ctx, saga, steps[:i])
        }
        saga.Cursor = i + 1
        if err := repo.Save(ctx, saga); err != nil {   // 每步都落库
            return err
        }
    }
    saga.Phase = PhaseDone
    return repo.Save(ctx, saga)
}

func compensate(ctx context.Context, saga *SagaState, done []Step) error {
    for i := len(done) - 1; i >= 0; i-- {          // 反向
        if err := done[i].Compensate(ctx, saga); err != nil {
            // ⚠️ 补偿失败不能吞：标记后交给 reconciler 重试
            saga.FailedAt = done[i].Name
            repo.Save(ctx, saga)
            return err
        }
        saga.Cursor = i
        repo.Save(ctx, saga)
    }
    return nil
}
```

### 三个实践要点

**不可补偿的动作放最后。** 发邮件、发优惠券、真实转账这类"泼出去收不回"的操作，必须放在整个 Saga 的最后一步——前面所有可补偿步骤都成功之后再执行。这一步叫 **pivot transaction**。

**补偿失败必须重试，不能吞。** 上面代码里如果写成 `_ = done[i].Compensate(...)`，就是留了一个永久不一致的口子。

**Saga 有中间态可见性问题。** 每一步都是真实提交的，所以别人能读到"钱扣了但货没锁"的状态。不能接受就用 TCC。

---

## 4. 形态三：TCC

适合**不能容忍中间态被读到**的场景（超卖、超付）。代价是强侵入——每个参与方要实现三个方法。

```go
type Participant interface {
    Try(ctx context.Context, gtxID string, req Req) error   // 预留资源
    Confirm(ctx context.Context, gtxID string) error        // 转成实际扣减
    Cancel(ctx context.Context, gtxID string) error         // 释放预留
}
```

### 三个必踩的坑，全靠一张事务控制表

| 坑 | 场景 | 处理 |
|---|---|---|
| **空回滚** | Try 因网络失败没执行，Cancel 却来了 | Cancel 发现无对应 Try → 插终态占位记录并返回成功 |
| **悬挂** | Cancel 先到、Try 后到（Try 超时重发） | Try 发现已有 CANCELLED 记录 → 拒绝预留 |
| **幂等** | Confirm / Cancel 都会被重试 | 查分支状态，已处理直接返回 |

```go
func (p *wallet) Try(ctx context.Context, gtxID string, req Req) error {
    return p.tx.Do(ctx, func(db *DB) error {
        br, err := db.GetBranchForUpdate(ctx, gtxID)
        if err != nil { return err }

        if br != nil {
            if br.Status == StatusCancelled {
                return ErrHanging          // 防悬挂：已被取消，不能再预留
            }
            return nil                     // 幂等：已 Try 过
        }
        if err := db.InsertBranch(ctx, gtxID, StatusTried); err != nil {
            return err
        }
        // 预留：可用余额 → 冻结余额
        return db.FreezeBalance(ctx, req.Owner, req.Amount)
    })
}
```

> **本项目的提现流程已经是 TCC 形态**：`WITHDRAWAL_APPLY`（可用 −、提现中 +）是 Try，`WITHDRAWAL_COMPLETED`（提现中 −）是 Confirm，`WITHDRAWAL_CANCELLED` / `FAILED`（提现中 −、可用 +）是 Cancel，`withdrawing_balance` 就是预留资源的载体。
>
> 缺的正是这张控制表带来的空回滚和悬挂防护——`handle.go:142` 的 APPLY 不查重 `related_order_id`，同一笔可以申请出多个 PENDING 单。

---

## 5. 形态四：Outbox

完整机制见规范文档 §3。这里只强调代码形态上最容易写错的一点：

```go
// 事务内只做本地写，绝不碰网络
err := tx.Do(ctx, func(db *DB) error {
    ok, err := db.Transit(ctx, orderID, []Status{Pending}, Paid)
    if err != nil { return err }
    if !ok {
        return nil          // ← CAS 失败 = 已处理过，不写 outbox
    }
    return db.InsertOutbox(ctx, Event{ID: eventID, Payload: snapshot})
})
// 投递在事务外，由独立 relay 完成
```

**幂等判断和 outbox 写入必须在同一个事务里。** 分开写的话，重复回调会产生重复事件——CAS 挡住了状态更新，却挡不住事件发布。

---

## 6. 形态五：最大努力通知

参与者不在你控制之下时（第三方支付、商户系统），这是**唯一可选项**。

代码形态就是 Outbox 的 relay + 一个查询接口：

```go
// 推送侧：退避重试，耗尽进死信并告警
func deliverOnce(ctx context.Context, ev OutboxEvent) {
    err := httpPost(ctx, ev.Endpoint, sign(ev))     // 带 HMAC 签名
    switch {
    case err == nil:
        repo.MarkSent(ctx, ev.ID)
    case ev.Attempts >= maxAttempts:
        repo.MarkDead(ctx, ev.ID)
        alert.DeadLetter(ctx, ev)                    // ← 必须告警
    default:
        repo.ScheduleRetry(ctx, ev.ID, backoff(ev.Attempts))
    }
}
```

**必须同时提供一个查询接口**作为逃生口。你要向对方承认"我可能通知不到你"，那就得让对方能主动来问。

---

## 7. 选型决策树

按顺序问四个问题：

```
1. 参与者在你控制之下吗？
   否 → 最大努力通知 + 对账（没得选，硬约束）
   是 ↓

2. 只涉及"本地库 + 消息队列"吗？
   是 → Outbox 就够，别上 Saga 框架
   否 ↓

3. 能容忍中间态被读到吗？
   否（超卖、超付）→ TCC
   是 ↓

4. 是固定序列，还是"推进到终态"？
   固定序列 → 编排式 Saga
   推进到终态 → 状态机 + Reconciler
```

**一条实践建议**：大多数团队不需要 Saga 框架或 Seata。「**状态机 + CAS + Reconciler + Outbox**」这个组合能覆盖九成场景，而且全是自己能读懂、能调试的代码，没有黑盒。等真的出现"五个服务、十二个步骤、必须严格补偿"的流程，再考虑上框架。

---

## 8. 反模式清单

按出现频率排序，全都在真实代码里见过。

| 反模式 | 为什么错 |
|---|---|
| **用 `go func()` 代替 Outbox** | 跨事务、错误无人接收、进程重启即丢失，还常误用会被取消的请求 context |
| **用行锁当幂等** | 行锁只在事务期间存在，挡不住"先后到达"的重复 |
| **在事务里包裹 HTTP 调用** | 事务时长被外部服务绑定，连接池瞬间打满 |
| **无条件 UPDATE 状态** | 乱序消息会让状态回退，已成功的订单被改回待支付 |
| **状态映射的 default 返回中间态** | 未识别的外部状态被当成"待处理"，同样导致回退。应该报错 + 告警 |
| **早退语句只跳出闭包** | `TransactionDo(func(){ ... return nil })` 里的 return 不会终止外层函数，幂等守卫形同虚设 |
| **幂等标记写在 defer 里** | 处理失败时也会标记为已处理，把重试机制废掉，失败即永久丢失 |
| **固定间隔重试** | 下游宕机时把它捶得更惨；应该指数退避 + 抖动 |
| **死信无告警** | 躺在表里没人看的死信等于丢了 |
| **扫表无 limit / 无索引** | 量一上来单轮跑不完，调度堆积 |
| **消息分区键用 event_id** | 同一实体的事件散到不同分区，完全失去顺序保证 |
| **补偿失败被吞掉** | `_ = compensate()` 留下永久不一致的口子 |
| **没有对账** | 前面所有机制都是"尽力而为"，对账是唯一能发现漏了什么的手段 |

---

## 9. 怎么测

这类代码最难的是"故障路径根本跑不到"，所以测试要专门制造故障。

**decide 纯函数穷举。** 状态空间通常只有几个值，直接遍历所有 `(状态, 外部返回)` 组合，断言迁移合法：

```go
func TestDecideNeverRegresses(t *testing.T) {
    for _, from := range allStatuses {
        next, _ := decide(Order{Status: from})
        if next != from && !CanTransit(from, next) {
            t.Fatalf("非法迁移 %v -> %v", from, next)
        }
    }
}
```

**重复投递测试。** 同一条消息连续投三次，断言副作用只发生一次（余额只变一次、事件只发一次）。

**乱序投递测试。** 把 `succeeded` 和 `created` 两条事件反序投递，断言最终状态是 `succeeded`。

**故障注入。** 在「CAS 成功」和「写 outbox」之间、在「投递成功」和「标记 SENT」之间人为 panic，重启后断言系统能收敛。这两个点正是 at-least-once 的窗口所在。

---

## 10. 需要哪些指标

没有这几个指标，你不知道这套机制有没有在工作：

| 指标 | 为什么 |
|---|---|
| **各状态的记录数** | 非终态记录持续增长 = 推进器没在工作 |
| **非终态记录的年龄分布**（P50 / P99 / max） | 最老的那条卡了多久，是最直接的健康信号 |
| **推进器每轮耗时与处理条数** | 单轮耗时接近调度间隔 = 快要堆积了 |
| **重试次数分布** | 重试集中在某个动作 = 那个下游有问题 |
| **死信数量** | 必须配告警，不能只记录 |
| **对账差异数（按四类分）** | 渠道有本地无 / 本地有渠道无 / 金额不符 / 状态不符 |

本项目目前**一个都没有**——OTel 只接了 log exporter，没有 metrics。

---

## 11. 本项目落地建议

现状对照：

| 模块 | 当前形态 | 缺什么 |
|---|---|---|
| `order` | ❌ 无状态机，10 处裸赋值，`UpdateOrder` 无条件更新 | 迁移表 + CAS。补上之后三个轮询 task 立刻变成合格的 reconciler |
| `async_tasks` | ✅ 已是状态机 + 推进器形态 | 修幂等早退（`handler/base.go:58`）、修错误吞掉（`callback_handler.go:21`）、退避改指数、死信加告警 |
| `withdrawal_order` | ✅ 已是 TCC 形态 | 事务控制表（防空回滚 / 悬挂） |
| `wallet_account` | ✅ 悲观锁 + 版本号 CAS，写法正确 | 修 `wallet_account_dao.go:78` 缺主键条件；加 `amount > 0` 校验 |
| `event_producer` | ⚠️ Outbox 骨架齐全但三处断电 | 挪进业务事务、加 relay job、删掉裸 goroutine |
| 对外通知 | ❌ 无签名无重试 | HMAC + event_id + 退避 + 死信告警 |
| 对账 | ❌ 完全没有 | 先做钱包内部对账（余额 vs 流水），再做渠道对账 |

**建议的第一步是给 `order` 表补状态机和 CAS。** 收益最大：它同时解决重复回调、状态回退、以及"轮询与回调并发覆盖"三个问题，而且改动集中——一张迁移表 + 一个 `Transit` 方法 + 把 10 处裸赋值替换掉。`async_tasks` 那套推进器骨架已经在了，修掉两个 bug 就能用。
