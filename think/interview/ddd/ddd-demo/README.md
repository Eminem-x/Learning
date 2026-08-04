# DDD Demo（Go）：订单 BC + 库存 BC

> 来源：个人思考 + 学习（配套笔记：[DDD 领域驱动设计（白板级）](../ddd-domain-driven-design-20260723.md)）
> 收录时间：2026-07-23 CST
>
> 一个**零依赖、可直接运行**的 DDD 最小完整实现。不用 Docker、不连数据库、不引任何第三方库，
> `go run` 就能看到聚合、领域事件、Outbox、防腐层、Saga 补偿、乐观锁、CQRS 全部跑起来。

---

## 30 秒：这个 demo 想让你「看见」什么

概念文章讲 DDD 容易变成名词表演。这个 demo 的目标只有一个：

> **让每一个 DDD 概念都对应到一段能跑、能改、能跑挂的代码。**

它实现了两个限界上下文（订单 + 库存）之间一次完整的业务往返：

```
下单 → 跨 BC 预留库存 → 订单落库 + 事件写 outbox（同一事务）
     → relay 投递事件 → 库存 BC 消费 → 最终一致
     ↑ 失败则 Saga 补偿          ↑ 重复投递被幂等吸收
```

**为什么必须有两个 BC？** 因为 DDD 最重要的那几个洞察（聚合边界、事务边界、最终一致、防腐层）
在单个上下文里根本显现不出来。一个 BC 的 demo 只能演示「分层」，而分层是 DDD 里最不重要的部分。

---

## 怎么跑

```bash
cd ddd-demo

go run ./cmd/demo    # 跑完整业务流程，8 个场景带解说输出
go test ./...        # 跑领域层单测（注意速度：毫秒级，因为不依赖任何基础设施）
go vet ./...
```

---

## 概念 → 代码 对照表（核心，先看这张）

| DDD 概念 | 在哪 | 一句话看点 |
|---------|------|-----------|
| **值对象 Value Object** | `order/domain/money.go` | 字段不导出 + 只给方法 ⇒ 「金额非负、币种一致、以分为单位」全系统只写一遍，无法绕过 |
| **实体 Entity** | `order/domain/order.go` `OrderItem` | 有业务身份但依附聚合根；没有自己的 Repository |
| **聚合根 Aggregate Root** | `order/domain/order.go` `Order` | 字段全不导出，外部只能通过 `Pay/Ship/Cancel` 改状态 |
| **不变性 Invariant** | `Order.AddItem` / `Stock.Reserve` | `total == Σ 明细` 与 `可用量 >= 0`，任何路径都成立 |
| **只用 ID 引用其它聚合** | `Order.buyerID string` | 不是 `*User`。防止对象图爆炸 + 防止一个事务改两个聚合 |
| **工厂 Factory** | `domain.NewOrder` | 创建即合法，不存在「半成品订单」 |
| **领域事件 Domain Event** | `order/domain/events.go` | 过去式命名；在聚合内累积，由应用层取走 |
| **资源库 Repository（端口）** | `order/domain/repository.go` | **接口定义在领域层**，实现在 infra —— 依赖倒置的落点 |
| **应用服务 Application Service** | `order/app/commands.go` | 数一数：加载 → 调行为 → 保存 → 发事件，**没有一个 if 是业务规则** |
| **事务边界** | `order/app/commands.go` + `platform/tx` | 事务在应用层划定，领域层完全不知道「事务」存在 |
| **一个事务只改一个聚合** | `inventory/app/service.go` `Reserve` | N 个 SKU = N 个聚合 = N 个事务 + 失败补偿，而不是一个大事务 |
| **Saga 补偿** | `order/app/place_order.go` ③ + `inventory/app/service.go` | 订单落库失败 ⇒ 把已预留的库存退回去 |
| **防腐层 ACL** | `order/infra/inventory_acl.go` | 订单 BC 唯一能看见库存 BC 的地方；翻译**类型**也翻译**错误语义** |
| **集成事件 / 对外契约** | `platform/eventbus/bus.go` `Envelope` | 跨 BC 传的是扁平 JSON，绝不是领域对象 |
| **Outbox（双写原子性）** | `platform/outbox/outbox.go` | 业务变更与「事件待发」进同一个本地事务 |
| **至少一次 + 幂等消费** | `Stock.reservations` map | 幂等不是外挂去重表，而是**模型自带的业务语义** |
| **乐观锁** | `order/infra/memory_repo.go` `Save` | 版本比对在**提交时**执行，冲突则整个事务丢弃 |
| **CQRS 读侧** | `order/infra/query.go` | 直接读存储拼 DTO，不加载聚合、不开事务 |
| **六边形 / 依赖倒置** | 整个目录结构 | `domain` 包只 import 标准库；`infra` 单向依赖 `domain` |
| **组合根** | `cmd/demo/main.go` | 全程序唯一知道「谁实现谁」的地方 |
| **反模式对照** | `internal/anemic/order_service.go` | 同一段业务的贫血/事务脚本写法，标了 6 个问题 |

---

## 目录结构

```
ddd-demo/
├── cmd/demo/main.go                    组合根 + 8 个演示场景
├── internal/
│   ├── anemic/order_service.go         ❌ 反例：贫血模型 + 事务脚本（对照阅读）
│   │
│   ├── order/                          【限界上下文 1：订单】
│   │   ├── domain/                     领域层 —— 只 import 标准库
│   │   │   ├── money.go                  值对象
│   │   │   ├── order.go                  聚合根 + 聚合内实体 + 持久化快照
│   │   │   ├── events.go                 领域事件
│   │   │   ├── repository.go             Repository 端口
│   │   │   └── order_test.go             ★ 零依赖单测，毫秒级
│   │   ├── app/                        应用层 —— 只做编排
│   │   │   ├── ports.go                  出站端口（库存/事件/ID/时钟）
│   │   │   ├── place_order.go            下单：跨 BC + 本地事务 + 补偿
│   │   │   ├── commands.go               支付 / 取消 / 发货
│   │   │   └── query.go                  CQRS 读侧端口与 DTO
│   │   └── infra/                      基础设施层 —— 适配器
│   │       ├── memory_repo.go            Repository 实现（乐观锁）
│   │       ├── event_publisher.go        领域事件 → 集成事件 → outbox
│   │       ├── inventory_acl.go          ★ 防腐层
│   │       ├── query.go                  读侧实现
│   │       └── support.go                ID 生成器 / 时钟
│   │
│   ├── inventory/                      【限界上下文 2：库存】
│   │   ├── domain/stock.go               聚合根 Stock：不超卖 + 幂等
│   │   ├── domain/stock_test.go          ★ 零依赖单测
│   │   ├── app/service.go                预留 / 释放（一个 SKU 一个事务 + 补偿）
│   │   └── infra/
│   │       ├── memory_repo.go            Repository 实现
│   │       └── subscriber.go             订阅 order.cancelled → 释放预留
│   │
│   └── platform/                       跨 BC 的**技术**设施（不含业务）
│       ├── tx/memory.go                  事务管理器 + 工作单元（两阶段提交玩具版）
│       ├── eventbus/bus.go               进程内事件总线（真实项目=Kafka）
│       └── outbox/outbox.go              本地消息表 + relay
```

**依赖方向自查**（DDD 项目最该守的一条线）：

```bash
# 三条命令都应该**没有任何输出**（只查 import 路径，注释里提到不算）

# 1. 领域层不依赖基础设施 / 平台设施
grep -rn "dddshop/internal/order/infra\|dddshop/internal/platform" internal/order/domain/

# 2. 订单 BC 的领域层和应用层完全不知道库存 BC 的存在
grep -rn "dddshop/internal/inventory" internal/order/domain/ internal/order/app/

# 3. 领域层只 import 标准库（下面这条会列出所有非标准库依赖）
go list -deps ./internal/order/domain | grep dddshop | grep -v "order/domain"
```

第二条尤其关键：**订单 BC 的领域层和应用层从头到尾不知道「库存 BC」的存在**，
它只知道自己定义的 `InventoryPort`。库存换成第三方 WMS、换成 gRPC 调用，只需要重写 ACL。

---

## 8 个演示场景

| 步骤 | 演示什么 | 关键输出 |
|:--:|---------|---------|
| 1 | 初始化库存 | — |
| 2 | 下单成功：跨 BC 预留 + 本地事务落库 + 事件入 outbox | 库存 5→3，outbox 多 1 条 |
| 3 | **Saga 补偿**：多 SKU 预留中途失败 | 壳先预留成功 → iPhone 不足 → 壳被补偿释放回 10 |
| 4 | **状态机由聚合守住** | 已发货订单取消被拒；规则不在任何 Service 里 |
| 5 | **领域事件 + Outbox + 至少一次 + 幂等** | 每条事件故意投两次：库存幂等跳过，通知服务却发了两条短信（反面教材） |
| 6 | **乐观锁** + 事务原子性 | A 成功 B 冲突；B 的 Save 和 outbox 写入一起被丢弃 |
| 7 | **CQRS 读侧** | 列表查询不加载聚合、不开事务 |
| 8 | 最终一致收敛 | 库存最终回到正确值 |

---

## 建议阅读顺序

1. **`internal/anemic/order_service.go`** —— 先看「我们平时是怎么写的」，里面标了 6 个问题。
2. **`internal/order/domain/order.go`** —— 再看聚合根。重点看：字段为什么不导出、`Cancel` 为什么幂等、`Snapshot/Rehydrate` 为什么必须存在。
3. **`internal/order/domain/order_test.go`** —— 看这个测试文件的 import 只有标准库。**这是判断真假 DDD 最直接的标尺**。
4. **`internal/order/app/commands.go`** —— 看应用层有多薄。
5. **`internal/order/infra/inventory_acl.go`** —— 防腐层，类型翻译 + 错误语义翻译。
6. **`internal/platform/outbox/outbox.go`** —— 双写原子性的标准解法。
7. **`internal/inventory/domain/stock.go`** —— 「不超卖」和「幂等」如何被表达成模型自身的语义。
8. 最后回到 **`cmd/demo/main.go`** 看整体是怎么装配起来的。

---

## 几个「为什么这么写」的自问自答

**Q：为什么两个 BC 各有一个 `tx.Memory`？**
因为它们代表两个独立的数据库。这在代码层面强制了 DDD 的硬约束：**跨 BC 不存在共享事务**。
如果它们共用一个事务管理器，你会不知不觉写出「一个事务同时改订单和库存」的代码 —— 那就退化成单体大泥球了。

**Q：为什么下单时先调库存、再开本地事务，而不是包在一个事务里？**
因为库存调用在真实系统里是一次网络请求。把网络调用放进数据库事务里，等于把 DB 连接的持有时间
和一个不可控的外部服务绑定 —— 对方抖一下，你的连接池就打满了。所以顺序必须是：
**先做不可回滚的外部调用 → 再做本地事务 → 本地事务失败则补偿外部调用**。

**Q：为什么 `Order` 的字段全不导出，还要额外写 `Snapshot`/`Rehydrate`？**
不导出是为了让规则守得住（外部无法 `o.status = 3`）。代价是 ORM 的反射进不来，
所以要显式写快照转换。这是 Go 做 DDD 必须付的税 —— 换来的是**领域模型与表结构彻底解耦**。

**Q：为什么 `Cancel(reason, now)` 要把时间当参数传？**
`time.Now()` 散落在领域层会让单测无法断言。把不确定性（时间、随机数、ID）挡在领域层之外，
是「领域层可测试」的前提。

**Q：`Stock` 里那个 `reservations map[string]int` 为什么重要？**
它把「幂等」从技术补丁变成了业务语义：「同一个订单只能占一次坑」本身就是一条业务规则。
于是 MQ 重复投递、用户重复点击、超时重试全部天然安全 —— 不需要额外的去重表。

**Q：为什么内存版事务要拆成 check / apply 两段？**
为了让「要么全写、要么一个都不写」真的成立：提交时先跑完所有 check，全通过才跑 apply。
这是 2PC 的玩具版，也让步骤 6 能演示「乐观锁失败时，outbox 写入一起被丢弃」。

---

## 动手改造练习（真正吃透靠这个）

按难度排序，每一条都能暴露一个理解盲点：

1. **加一条业务规则**：金额 > 10000 的订单不允许直接取消，必须先走审批。
   → 想清楚这条规则该放哪：`Order.Cancel` 里？还是应用层？（答案：放聚合内，因为它是业务不变性）
2. **加一个领域事件** `OrderShipped`，让某个新的下游订阅它。
   → 验证：订单 BC 的代码需要改几行？（答案：领域层加事件，应用层加一行 `AppendAll`，下游零侵入）
3. **让下单支持优惠券**：新增 `Discount` 值对象，总额 = 明细和 - 优惠。
   → 注意不变性怎么变，以及 `Snapshot` 要不要跟着改。
4. **把内存 Repository 换成 SQLite/MySQL**。
   → 验证 DDD 的核心承诺：`domain` 和 `app` 包能不能做到一行不改？
5. **给通知服务加幂等**（步骤 5 里它发了两条短信）。
   → 体会「每个消费者都要自己扛幂等」这句话的重量。
6. **故意制造一个大聚合**：把 `Stock` 改成「一个仓库一个聚合，内含所有 SKU」，
   然后并发下单 100 次。→ 观察乐观锁冲突率怎么爆炸，亲手验证「聚合要小」这条原则。
7. **让 relay 在投递时随机失败**，观察未投递事件如何被下一轮重试捞起来。

---

## 与笔记的交叉链接

- 概念全景、战略设计、面试追问链条、自测题 → [DDD 领域驱动设计（白板级）](../ddd-domain-driven-design-20260723.md)
- Outbox 依赖的「本地事务 + binlog/CDC」 → 能力地图第 4 格，[MySQL 三大日志实验台](../../mysql/mysql-logs-lab/README.md)
- 乐观锁 vs 悲观锁、MVCC → [MVCC × 隔离级别 × 幻读](../../mysql/mysql-mvcc-isolation-phantom-read-20260710.md)
- 分布式事务 / Saga / TCC → 能力地图第 8 格（未启）

---

## 附录：自我思考（这个 demo 刻意没做的事）

写这个 demo 时做了一些**有意的简化**，每一处简化本身也是个知识点，列出来免得以后误读：

1. **没有真实数据库**。用 map + 内存事务，是为了让「跑起来」的门槛为零。
   代价：`tx.Memory` 用一把全局锁模拟串行化隔离，看不到真实的 MVCC / 间隙锁行为。
   要补这块深度，去 MySQL 那条线，别在这个 demo 里找。
2. **没有 HTTP/gRPC 接口层**。用户接口层在 DDD 里是最薄、最没信息量的一层（协议转换 + DTO 映射），
   加上它只会稀释重点。真实项目里 `api/http.go` 应该只有「解析请求 → 组 Command → 调 Handler → 序列化响应」。
3. **ACL 是进程内直接调用**，真实场景是 gRPC/HTTP。但**位置和职责完全一样** —— 换的只是传输方式，
   这恰恰说明「BC 是逻辑边界，微服务是部署边界」：单体内部就可以先把边界立好，将来物理拆分只换适配器。
4. **没有事件溯源（ES）**。ES 成本极高（版本演进、快照、调试困难），绝大多数系统不需要。
   知道它是什么、为什么一般不用，比硬上一个更重要。
5. **没有领域服务（Domain Service）的例子**。因为这个业务里所有规则都能自然地落在某个聚合上 ——
   这其实是好事。领域服务是「实在放不进任何实体时」的兜底，不是必须凑齐的元素。
   什么时候会需要？比如「跨两个账户的转账」「需要商品+会员+活动三方数据的定价」。
6. **补偿失败没有兜底**。代码里 `logf` 了一句「真实系统应告警 + 定时对账」——
   这是最容易在面试被追问、也最容易在生产上出事的地方：**补偿本身也会失败**，
   所以最终一致系统必须有第三道防线：对账 + 人工干预入口。

最后一条也是最想提醒自己的：**这个 demo 的价值不在「代码写得多漂亮」，而在它能被改坏。**
上面那 7 条改造练习里，第 6 条（故意做一个大聚合然后看冲突率爆炸）最值得动手 ——
DDD 的原则光背是没用的，亲手把它违反一次、看到系统怎么塌，才会真的记住。
