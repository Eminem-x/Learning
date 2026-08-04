# DDD 领域驱动设计（白板级）

> 来源：个人思考 + 学习（对标《四年后端 + 轻量大数据能力地图》第 9 格：微服务与架构 — 分层 / DDD）
> 主要参考：Eric Evans《Domain-Driven Design》(2003)、Vaughn Vernon《Implementing DDD》(2013)、Martin Fowler [AnemicDomainModel](https://martinfowler.com/bliki/AnemicDomainModel.html)、Alistair Cockburn [Hexagonal Architecture](https://alistair.cockburn.us/hexagonal-architecture/)
> 收录时间：2026-07-23 CST
>
> 说明：DDD 是**方法论**不是框架，面试考的不是背名词，而是「你能不能用它把一个真实业务切开、说清边界和一致性」。本篇按 **战略设计 → 战术设计 → 架构落地 → Go 代码 → 反模式 → 追问链条** 组织，代码用 Go。

---

## 30 秒电梯版（盖住全文也要能讲出主线）

1. **DDD 解决的是业务复杂度，不是技术复杂度。** 手段只有一个核心：**让代码结构长得像业务结构**，消除「业务语言 → 技术语言」的翻译损耗。
2. **统一语言（Ubiquitous Language）是地基。** 领域专家、产品、代码用同一套词；这套词的**有效边界**就是**限界上下文（Bounded Context, BC）**。同一个「商品」在商品域/库存域/订单域是三个不同的模型 —— 这不是重复，这是正确。
3. **战略设计**（划边界）：领域 → 子域（**核心域 / 支撑域 / 通用域**，决定投多少人力）→ 限界上下文 → **上下文映射**（ACL 防腐层、OHS 开放主机服务、Customer-Supplier…）。**微服务拆分应该先做 BC 划分**，BC 是逻辑边界，微服务是部署边界。
4. **战术设计**（填内容）：**实体**（有 ID、可变）、**值对象**（无 ID、不可变、值相等）、**聚合**（一致性边界 + 事务边界，聚合根是唯一入口）、**领域服务**（跨实体的无状态领域逻辑）、**资源库 Repository**（面向聚合根的集合抽象，接口在领域层）、**领域事件**（过去式命名，聚合间/BC 间解耦）。
5. **聚合是全篇最重要的概念。** 四原则：① 边界内保护不变性 ② 尽量设计小 ③ 聚合之间**只用 ID 引用** ④ **一个事务只改一个聚合**，跨聚合走最终一致（领域事件 / Saga）。
6. **架构上是依赖倒置**：领域层不依赖任何外部技术，基础设施层实现领域层定义的接口 → 四层架构 / **六边形（端口-适配器）** / 洋葱 / 整洁架构，本质同构：**依赖只能由外向内指向领域**。
7. **贫血 vs 充血**：把业务规则从 Service 挪回领域对象，Service 只做编排（事务、权限、发事件）。但贫血在纯 CRUD 场景**不是错**，DDD 有成本，只在**核心域**用。
8. **查询别走聚合。** 复杂列表/报表用 **CQRS** 分离读模型，直接 SQL 拼 DTO —— 这是 DDD 能落地的关键松绑。
9. **落地最大的坑**：只改了目录结构，Service 里还是事务脚本；聚合切太大导致并发冲突；Repository 退化成 DAO。

---

## 一、DDD 到底解决什么问题（先说清动机，别背名词）

### 1.1 软件复杂度的两类来源

| 类型 | 例子 | 应对手段 |
|------|------|---------|
| **技术复杂度** | 高并发、分布式、存储选型、性能 | 缓存、分库分表、MQ、限流熔断（能力地图 5/6/8/10 格） |
| **业务复杂度** | 优惠券叠加规则、结算规则、多状态流转、行业术语 | **DDD** |

四年经验的人踩过的典型场景：一个 `OrderService.java/go` 涨到 3000 行，`updateOrder` 里塞了 12 个 `if status ==`，改一处崩三处，新人看不懂业务只能问老人。这不是技术问题 —— 是**业务知识没有在代码里被结构化表达**。

### 1.2 DDD 的核心主张

> **模型即代码，代码即模型。** 领域模型不是画在 Wiki 上的图，而是**运行中的代码本身**。

推论有三条，面试可以直接讲：

1. 如果业务规则只存在于产品经理脑子里和 `if` 语句里，模型就是**失效**的。
2. 模型必须能被领域专家看懂 —— 所以要有**统一语言**。
3. 模型不可能全局统一 —— 所以要有**限界上下文**（DDD 相对早期 OOA/OOD 最大的突破就在这里）。

### 1.3 与传统三层架构的对比

```
传统三层（事务脚本 Transaction Script）        DDD 四层
┌──────────────┐                              ┌──────────────┐
│ Controller   │                              │ 用户接口层    │  HTTP/gRPC/MQ消费/CLI
├──────────────┤                              ├──────────────┤
│ Service      │ ← 业务规则全在这，越涨越大     │ 应用层        │  编排：事务/权限/发事件（薄）
├──────────────┤                              ├──────────────┤
│ DAO / Model  │ ← Model 是纯数据袋（贫血）    │ 领域层        │  业务规则在这（厚，核心）
└──────────────┘                              ├──────────────┤
       ↓ 依赖数据库                            │ 基础设施层    │  DB/Redis/MQ/外部API
                                              └──────────────┘
                                                  ↑ 依赖倒置：实现领域层接口
```

关键差异不是「多了一层」，而是**两点**：

- **业务规则的归属**：从 Service 移到领域对象内部（充血模型）。
- **依赖方向**：领域层**不依赖**基础设施层，反过来由基础设施层实现领域层定义的接口。这样领域层可以**脱离数据库单测**。

---

## 二、战略设计：怎么划边界（面试的高分区）

> 战术设计（实体、值对象）大家都会背；**战略设计才是区分「读过书」和「做过事」的地方**。

### 2.1 领域 / 子域 / 核心域

**领域（Domain）** = 业务的整体问题空间，如「电商」。拆成**子域（Subdomain）**：

| 子域类型 | 定义 | 电商例子 | 投入策略 |
|---------|------|---------|---------|
| **核心域 Core** | 差异化竞争力所在，公司靠它赚钱 | 交易/定价/推荐（看公司定位） | **最强的人 + DDD 深耕 + 自研** |
| **支撑域 Supporting** | 业务特有但不构成竞争力 | 售后工单、商家后台 | 一般团队，简单分层即可 |
| **通用域 Generic** | 行业通用，别人做得更好 | 登录认证、短信通知、对象存储 | **买 / 开源 / 外包**，别自研 |

**面试用法**：被问「DDD 成本这么高，怎么落地？」标准答案就是 —— **不全量落地，只在核心域落地**。支撑域用简单 CRUD，通用域直接买。这句话能立刻把你和「背概念的人」区分开。

### 2.2 统一语言（Ubiquitous Language）

规则很简单：**代码里的类名/方法名 = 领域专家嘴里说的词**。

反例（真实项目里到处都是）：

```go
// ❌ 技术黑话，领域专家看不懂，三个月后自己也看不懂
func (s *OrderService) UpdateStatus(id int64, st int, extInfo map[string]interface{}) error
```

正例：

```go
// ✅ 业务动作即方法名，产品经理能直接读
func (o *Order) Cancel(reason CancelReason) error
func (o *Order) Pay(p Payment) error
func (o *Order) Ship(t TrackingNo) error
```

**追问「统一语言怎么建立？」** → 事件风暴（Event Storming）：把开发、产品、领域专家关一屋，用便利贴按时间轴贴出「发生了什么事件（橙色）→ 谁的命令触发（蓝色）→ 落在哪个聚合（黄色）」，事件聚集的天然裂缝就是**限界上下文的候选边界**。

### 2.3 限界上下文（Bounded Context）—— 全篇第二重要的概念

> **定义：一个模型（和它的统一语言）保持内部一致的显式边界。**

核心洞察：**同一个业务名词，在不同上下文里就是不同的模型，不该强行统一。**

以「商品」为例：

| 上下文 | 「商品」是什么 | 关键字段 |
|--------|--------------|---------|
| **商品中心** | SPU / SKU、类目、属性、图文详情 | 品牌、规格、上下架状态 |
| **库存** | 一个可扣减的计数器 | skuID、可用量、锁定量、仓库 |
| **订单** | 下单那一刻的**快照** | skuID、名称快照、成交价（**不能跟着商品中心改价**） |
| **搜索** | 一份倒排索引文档 | 分词字段、权重、销量 |

> 新人最常犯的错：造一张「大而全的商品表」，40 个字段，所有域共用 —— 这就是 **大泥球（Big Ball of Mud）** 的起点。任何一个域改字段，全公司回归。

**BC 和微服务的关系（高频追问）**：

- BC 是**逻辑/语义边界**，微服务是**物理/部署边界**。
- 一个 BC 可以拆成多个微服务（按性能/团队），但**一个微服务不应横跨多个 BC**（否则又变大泥球）。
- **拆微服务的正确顺序**：先用事件风暴划 BC → 验证边界（看是不是每次改需求都要同时改两个 BC，是就说明边界划错了）→ 再考虑物理拆分。**边界没想清就先拆服务 = 把单体的混乱升级成分布式的混乱**（还额外送你网络超时、分布式事务、链路追踪）。

### 2.4 上下文映射（Context Map）—— BC 之间怎么协作

九种关系，面试记住加粗的四种就够用：

| 关系 | 含义 | 什么时候用 |
|------|------|-----------|
| **防腐层 ACL (Anticorruption Layer)** | 我方建一层翻译，把外部模型转成我方模型 | **最常用**：对接遗留系统、第三方（支付宝/微信）、上游模型很烂 |
| **开放主机服务 OHS (Open Host Service)** | 我作为上游，提供一套稳定的公共协议给所有下游 | 我是被很多方调用的基础服务 |
| **发布语言 PL (Published Language)** | 双方约定一套公共交换格式（如统一的事件 schema） | 常和 OHS 搭配 |
| **客户方-供应方 Customer-Supplier** | 上下游有明确协作关系，下游需求能影响上游排期 | 同公司内的两个团队 |
| 共享内核 Shared Kernel | 共用一小块模型代码 | 慎用，耦合两个团队 |
| 遵奉者 Conformist | 完全顺从上游模型，不做翻译 | 上游强势且模型还行，省成本 |
| 合作关系 Partnership | 两个 BC 同生共死，一起发布 | 强耦合，尽量避免 |
| 另谋他路 Separate Ways | 干脆不集成，各做各的 | 集成成本 > 收益 |
| 大泥球 Big Ball of Mud | 无边界的历史包袱 | 识别出来，然后用 ACL 隔离它 |

**ACL 是最该会写的一个**（Go 示例见 §5.5）。一句话价值：**把外部的烂模型/破坏性变更挡在自己领域之外**。对接过第三方支付回调的人对这个应该有痛感 —— 回调字段是 `trade_status="TRADE_SUCCESS"` 这种字符串，绝不能让它渗进领域层的 `Order`。

---

## 三、战术设计：模型怎么建（写代码的部分）

### 3.1 实体 Entity vs 值对象 Value Object

| | 实体 Entity | 值对象 Value Object |
|--|------------|-------------------|
| **标识** | 有唯一 ID | 无 ID |
| **相等性** | ID 相同即同一个 | **所有属性相同才相等** |
| **可变性** | 可变（有生命周期） | **不可变**（改就是造个新的） |
| **例子** | 订单、用户、账户 | 金额 Money、地址 Address、时间段 DateRange、坐标 |

**判定口诀**：「换一个但值一样，业务上还是同一个吗？」
- 两张 100 元钞票 → 业务上等价 → **值对象**。
- 两个叫张三的用户 → 不是同一人 → **实体**。

**为什么要优先用值对象？**（追问必出）
1. **不可变 ⇒ 天然线程安全、可自由共享、无副作用**。
2. **把散落的校验收拢**：`Money` 内部保证币种一致、金额非负、用 `int64` 分为单位避免浮点误差 —— 而不是在 20 个 Service 里各写一遍 `if amount < 0`。
3. **易测试**：纯函数，不需要 mock。
4. 消灭「基本类型偏执（Primitive Obsession）」：`func Transfer(from, to string, amount float64)` 四个参数全是裸类型，传反了编译器不报错；换成 `AccountID` / `Money` 就编译期挡住了。

**值对象在数据库怎么存？**（高频追问）
- **内嵌列（推荐）**：`Address{Province, City, Detail}` → 订单表加三列 `province/city/detail`。
- **JSON 列**：结构复杂、不需要按字段查时。
- **单独表**：只有当它是集合（如一个订单多个标签）时。
- 注意：值对象**不需要自己的 ID**，如果你给它加了主键，先反思是不是它其实是实体。

### 3.2 聚合 Aggregate —— 全篇最重要

> **定义：一组必须作为整体保持一致的对象，外部只能通过唯一入口「聚合根」访问。**

两个身份，一定要背下来：

- **一致性边界**：边界内的**不变性（invariant）**必须在每次操作后立即成立。
- **事务边界**：**一个数据库事务只应修改一个聚合实例**。

**Vernon 的四条设计原则**（经典面试点）：

| # | 原则 | 说明 / 违反后果 |
|:-:|------|----------------|
| 1 | **在边界内保护不变性** | 划边界的唯一依据是「哪些数据必须强一致」。例：订单总额 = ∑ 明细金额，必须强一致 → 明细属于订单聚合内。 |
| 2 | **聚合设计得尽量小** | 大聚合 = 加载慢 + 锁范围大 + 乐观锁冲突率高。典型灾难：把「用户」聚合做成含订单列表 → 用户改个昵称要加载 5000 个订单。 |
| 3 | **聚合之间只用 ID 引用** | `Order.buyerID string` 而不是 `Order.buyer *User`。对象引用会诱导你在一个事务里连改两个聚合，还会让加载失控（对象图爆炸）。 |
| 4 | **边界外用最终一致** | 跨聚合的一致性通过**领域事件 + 异步处理**（或 Saga）达成，不用分布式事务。 |

**经典白板题：订单和库存要不要放一个聚合？**

答：**不要**。理由分三层讲：
1. 它们的不变性不同（订单管自身状态机与金额，库存管「可用量 ≥ 0」），不需要强一致地放一起；
2. 放一起会导致聚合巨大、并发扣减时锁竞争严重（爆款 SKU 直接把订单写入拖垮）；
3. 正确做法：订单聚合 + 库存聚合，下单时应用层编排 —— **要么同步调库存扣减接口（库存聚合内部用 DB 原子扣减 `UPDATE ... SET qty=qty-1 WHERE qty>=1` 或乐观锁保证不超卖），要么走「预扣 + 最终一致 + 超时释放」**；跨服务时用 TCC / Saga / 本地消息表（→ 能力地图第 8 格，可与 `../` 分布式事务笔记交叉）。

**追问「一个事务只改一个聚合，那我批量取消 100 个订单怎么办？」**
→ 拆成 100 个事务（或分批），失败的重试；如果业务要求「全成功或全失败」，那说明这是一个 Saga，用编排 + 补偿，而不是把 100 个订单塞进一个聚合。

**追问「怎么判断我的聚合切大了？」** 三个信号：
- 加载一个聚合要 JOIN 五张表 / 取上千行；
- 乐观锁冲突（`version` 冲突重试）频繁；
- 聚合内有大量字段跟当前操作的不变性完全无关。

### 3.3 领域服务 Domain Service vs 应用服务 Application Service

**新人最容易混的一对**，面试常考。

| | 领域服务 | 应用服务 |
|--|---------|---------|
| **在哪一层** | 领域层 | 应用层 |
| **装什么** | **不属于任何单个实体/值对象的业务规则** | **编排**：开事务、查权限、调 Repository、调领域对象、发事件 |
| **能不能有业务规则** | 能（它就是业务规则） | **不能**（有了就是又退化成事务脚本） |
| **有状态吗** | 无状态 | 无状态 |
| **例子** | 转账（跨两个账户聚合）、定价策略（要商品+会员+活动多方数据）、订单折扣计算 | `CancelOrderAppService.Handle(cmd)` |

判定：**「这段逻辑是业务概念，还是流程胶水？」** 领域专家能听懂 → 领域服务；只有程序员关心（事务、日志、DTO 转换）→ 应用服务。

### 3.4 资源库 Repository —— 不是 DAO

| | Repository | DAO |
|--|-----------|-----|
| **面向** | **聚合根**（领域概念） | 表（存储概念） |
| **心智模型** | 一个**内存集合**：`Add / Remove / FindByID` | SQL 执行器：`insert/update/selectByXxx` |
| **接口定义在** | **领域层** | 数据访问层 |
| **实现在** | 基础设施层（依赖倒置） | 数据访问层 |
| **粒度** | 整个聚合一起存取 | 单表、单字段可任意更新 |

**关键约束：一个聚合一个 Repository**，不要给聚合内部的实体（如 `OrderItem`）单独建 Repository —— 那会绕过聚合根、破坏不变性。

**追问「Repository 里能不能写 `FindByStatusAndTimeRange` 这种查询？」**
→ 能少写就少写。**Repository 是给「写模型」用的**（加载聚合 → 执行行为 → 保存）。列表/报表/多条件筛选是**读场景**，应该走 **CQRS 的查询侧**：直接写 SQL 返回 DTO，不经过聚合。硬把复杂查询塞进 Repository，最后一定退化成 DAO（这几乎是 DDD 落地失败的头号症状）。

### 3.5 领域事件 Domain Event

**定义**：领域中已经发生的、领域专家关心的事实。**过去式命名**：`OrderPlaced`、`OrderCancelled`、`PaymentSucceeded`。

用途：
1. **聚合间最终一致**：订单取消 → 发 `OrderCancelled` → 库存聚合监听并回滚预扣。
2. **跨 BC 集成**：订单 BC → MQ → 积分 BC / 通知 BC。**这解耦了上游对下游的感知**（下游加一个「取消送优惠券」的需求，订单代码一行不用改）。
3. **审计 / 溯源**：事件序列天然是业务流水。

**领域事件 vs 集成事件（追问点）**：

| | 领域事件 Domain Event | 集成事件 Integration Event |
|--|---------------------|--------------------------|
| 作用域 | BC 内部 | 跨 BC / 跨服务 |
| 载体 | 进程内事件总线 | MQ（Kafka/RocketMQ） |
| 内容 | 可以携带领域对象 | 只能是**扁平的、版本化的 DTO**（对外契约，不能暴露内部模型） |

**落地必答：「更新 DB」和「发 MQ 消息」怎么保证原子？**
→ 双写不可能原子。标准解法是 **本地消息表 / Outbox 模式**：

```
BEGIN;
  UPDATE orders SET status='CANCELLED', version=version+1 WHERE id=? AND version=?;
  INSERT INTO outbox_events(aggregate_id, type, payload, status) VALUES(...);
COMMIT;
-- 独立的 relay 进程/CDC(Debezium 读 binlog) 轮询 outbox → 投递 MQ → 标记已发
```

这样把「分布式事务」降级成「本地事务 + 至少一次投递」，代价是**消费端必须幂等**（去重表 / 唯一键 / 状态机幂等）。这条链接到能力地图第 8 格，也直接呼应 MySQL binlog 那条线。

### 3.6 工厂 Factory 与模块 Module

- **Factory**：聚合创建逻辑复杂（要校验一堆前置条件、组装多个对象）时，把创建从构造函数里抽出来。Go 里通常就是一个 `NewOrder(...) (*Order, error)` 包级函数，够用。
- **Module**：Go 里就是 **package**。一个 BC 一个顶层包，用 `internal/` 强制边界（跨 BC 只能通过导出的应用服务或事件访问）——这一点 Go 比 Java 做得更干净。

---

## 四、架构落地：分层、六边形、CQRS

### 4.1 四层架构与依赖方向

```
       ┌───────────────────────────────────────────┐
       │ 用户接口层 Interfaces                      │  HTTP handler / gRPC server / MQ consumer / CLI
       │  职责：协议转换、参数校验、DTO ⇄ Command    │
       └────────────────┬──────────────────────────┘
                        │ 依赖
       ┌────────────────▼──────────────────────────┐
       │ 应用层 Application                         │  AppService / Command Handler
       │  职责：事务边界、权限、编排、发布事件（薄）  │
       └────────────────┬──────────────────────────┘
                        │ 依赖
       ┌────────────────▼──────────────────────────┐
       │ 领域层 Domain（核心，零外部依赖）           │  Entity / VO / Aggregate / DomainService
       │  ★ 定义 Repository 等接口（端口 Port）      │  ★ 领域事件
       └────────────────▲──────────────────────────┘
                        │ 实现（依赖倒置 DIP）
       ┌────────────────┴──────────────────────────┐
       │ 基础设施层 Infrastructure                  │  MySQL/Redis/Kafka/第三方 SDK/ACL
       └───────────────────────────────────────────┘
```

**一句话考点**：传统三层里基础设施在**最底层被依赖**；DDD 四层里基础设施在**最外层做实现**。这个反转就是**依赖倒置原则（DIP）**，也是「领域层能脱库单测」的根因。

### 4.2 六边形架构（端口-适配器）

四层图画成六边形就是 Cockburn 的模型：

```
   驱动侧（主动调我）                      被驱动侧（我调它）
   Driving Adapters                        Driven Adapters
   ┌──────────┐                            ┌──────────────┐
   │ HTTP     │──┐                    ┌───▶│ MySQL Repo   │
   │ gRPC     │──┼──▶ ┌───────────┐ ──┤    │ Redis Cache  │
   │ MQ 消费  │──┤    │  应用+领域 │   ├───▶│ Kafka Producer│
   │ 定时任务 │──┘    │  (六边形)  │   └───▶│ 支付网关 ACL  │
   └──────────┘       └───────────┘        └──────────────┘
                    ↑ 入站端口(接口)  ↑ 出站端口(接口，定义在领域/应用层)
```

**价值一句话**：**业务逻辑对「谁调用我」和「我用什么存」都无感知** —— 从 HTTP 换成 gRPC、从 MySQL 换成 PG，领域层零改动；测试时全部端口用内存实现，跑得比启动 Docker 快 100 倍。

洋葱架构（Onion）、整洁架构（Clean Architecture）和六边形本质同构，都是同一条规则：**依赖只能由外向内**。面试被问区别，答「细节命名不同，核心规则一致」即可，别硬背层名。

### 4.3 CQRS（Command Query Responsibility Segregation）

**动机**：写模型（聚合）为「保护不变性」而设计，读模型为「查询效率」而设计，二者诉求天然冲突。

```
 命令 Command ──▶ AppService ──▶ 聚合(领域规则) ──▶ Repository ──▶ 写库
                                       │
                                       └─ 领域事件 ─▶（可选）同步到读库/ES/缓存
 查询 Query   ──▶ QueryService ──▶ 直接 SQL / ES ──▶ DTO（不经过聚合）
```

三个层级，按需选：
1. **同库不同模型（推荐起步）**：同一个 MySQL，写走聚合，读直接 SQL 拼 DTO。**零额外成本，收益最大**。
2. **读写分离库**：读走从库 / ES（你熟悉的场景：订单写 MySQL、查询走 ES —— 这就是 CQRS）。
3. **事件溯源 Event Sourcing**：状态不落库，只存事件流，靠回放推导。**成本极高**（版本演进、快照、调试困难），面试知道概念、能说出「一般不用」和为什么，比硬吹更加分。

> **ES ≠ CQRS**。ES 是「怎么存」，CQRS 是「读写分离建模」。两者常搭配但互不依赖。

---

## 五、Go 落地：目录结构 + 一个订单聚合的完整代码

> Go 做 DDD 有两个先天优势：**接口隐式实现**（依赖倒置几乎零仪式感）、**组合优于继承**（天然贴合值对象/聚合）。
> 一个先天劣势：**没有 JPA 那种脏检查**，`Repository.Save` 得自己处理「哪些字段变了」。
>
> 📦 **本节的代码有一份可运行的完整实现**：[ddd-demo/](./ddd-demo/README.md) —— 两个限界上下文（订单 + 库存），
> 零依赖 `go run ./cmd/demo` 即可看到聚合、领域事件、Outbox、防腐层、Saga 补偿、乐观锁、CQRS 全部跑起来，
> 另附一份**贫血模型反例**供对照阅读。下面的片段都是从那里摘的。

### 5.1 目录结构（单体内多 BC）

```
internal/
├── order/                     # 一个限界上下文 = 一个顶层包
│   ├── domain/                # 领域层：零外部依赖（不 import gorm/gin/redis）
│   │   ├── order.go           #   聚合根 Order + 实体 OrderItem
│   │   ├── money.go           #   值对象
│   │   ├── event.go           #   领域事件
│   │   ├── repository.go      #   ★ Repository 接口（端口）
│   │   └── pricing.go         #   领域服务
│   ├── app/                   # 应用层：命令处理、事务编排
│   │   ├── cancel_order.go
│   │   └── query.go           #   ★ CQRS 读侧，可直接依赖 sqlx
│   ├── infra/                 # 基础设施层：适配器
│   │   ├── mysql_repo.go      #   ★ 实现 domain.Repository
│   │   ├── outbox.go
│   │   └── payment_acl.go     #   防腐层：第三方支付
│   └── api/                   # 用户接口层
│       └── http.go
├── inventory/                 # 另一个 BC，同构
└── pkg/                       # 跨 BC 的纯技术工具（不放业务）
```

**边界怎么强制？** 靠 `internal/` + code review + `go vet`/自定义 lint 规则（禁止 `order/domain` import 任何 `infra`/第三方 ORM）。

### 5.2 值对象：Money

```go
package domain

import "errors"

var ErrCurrencyMismatch = errors.New("币种不一致")

// Money 值对象：不可变、无 ID、值相等。
// 字段全部不导出 —— 外部无法绕过构造函数造出非法值。
type Money struct {
	amount   int64  // 以「分」为单位，杜绝浮点误差
	currency string
}

func NewMoney(amount int64, currency string) (Money, error) {
	if amount < 0 {
		return Money{}, errors.New("金额不能为负")
	}
	if currency == "" {
		return Money{}, errors.New("币种不能为空")
	}
	return Money{amount: amount, currency: currency}, nil
}

// Add 返回新值，不修改自身 —— 不可变性的体现
func (m Money) Add(o Money) (Money, error) {
	if m.currency != o.currency {
		return Money{}, ErrCurrencyMismatch
	}
	return Money{amount: m.amount + o.amount, currency: m.currency}, nil
}

func (m Money) MulInt(n int) Money { return Money{m.amount * int64(n), m.currency} }

// Equals 值相等：所有属性相同即相等（对比实体的 ID 相等）
func (m Money) Equals(o Money) bool {
	return m.amount == o.amount && m.currency == o.currency
}

func (m Money) Amount() int64    { return m.amount }
func (m Money) Currency() string { return m.currency }
```

### 5.3 聚合根：Order

```go
package domain

import (
	"errors"
	"time"
)

type OrderID string
type OrderStatus uint8

const (
	StatusCreated OrderStatus = iota + 1
	StatusPaid
	StatusShipped
	StatusCancelled
)

var (
	ErrInvalidStatus = errors.New("当前状态不允许该操作")
	ErrEmptyOrder    = errors.New("订单不能没有商品")
)

// OrderItem：聚合内实体（有 ID，但生命周期依附于 Order，外部不可直接访问）
type OrderItem struct {
	skuID string
	name  string // 下单时的商品名快照 —— 商品中心改名不影响历史订单
	qty   int
	price Money // 成交价快照
}

func (i OrderItem) Subtotal() Money { return i.price.MulInt(i.qty) }

// Order：聚合根，唯一入口
type Order struct {
	id      OrderID
	buyerID string // ★ 只用 ID 引用其它聚合（原则 3），不是 *User
	items   []OrderItem
	status  OrderStatus
	total   Money
	version int           // 乐观锁：并发修改保护
	events  []DomainEvent // 待发布的领域事件（在聚合内累积，由应用层统一发出）
}

// NewOrder 工厂：创建时就保证不变性成立
func NewOrder(id OrderID, buyerID string, items []OrderItem) (*Order, error) {
	if len(items) == 0 {
		return nil, ErrEmptyOrder
	}
	total, err := sum(items)
	if err != nil {
		return nil, err
	}
	o := &Order{id: id, buyerID: buyerID, items: items,
		status: StatusCreated, total: total, version: 0}
	o.raise(OrderPlaced{OrderID: id, BuyerID: buyerID, Total: total, At: time.Now()})
	return o, nil
}

// ---------- 业务行为：规则封在聚合内部，外部改不了状态 ----------

func (o *Order) Pay(paidAt time.Time) error {
	if o.status != StatusCreated {
		return ErrInvalidStatus // 不变性：只有待支付订单能支付
	}
	o.status = StatusPaid
	o.raise(OrderPaid{OrderID: o.id, Amount: o.total, At: paidAt})
	return nil
}

func (o *Order) Cancel(reason string) error {
	if o.status == StatusCancelled {
		return nil // 幂等：重复取消不报错（对接 MQ 至少一次投递的必要设计）
	}
	if o.status == StatusShipped {
		return ErrInvalidStatus // 不变性：已发货不可取消
	}
	o.status = StatusCancelled
	o.raise(OrderCancelled{OrderID: o.id, Reason: reason, At: time.Now()})
	return nil
}

// AddItem 演示「不变性在边界内立即成立」：改明细必须同步重算总额
func (o *Order) AddItem(it OrderItem) error {
	if o.status != StatusCreated {
		return ErrInvalidStatus
	}
	newItems := append(append([]OrderItem{}, o.items...), it)
	total, err := sum(newItems)
	if err != nil {
		return err
	}
	o.items, o.total = newItems, total // 要么都改，要么都不改
	return nil
}

func (o *Order) raise(e DomainEvent) { o.events = append(o.events, e) }

// PullEvents 取出并清空事件，由应用层在事务内写 outbox
func (o *Order) PullEvents() []DomainEvent {
	es := o.events
	o.events = nil
	return es
}

func (o *Order) ID() OrderID       { return o.id }
func (o *Order) Status() OrderStatus { return o.status }
func (o *Order) Total() Money      { return o.total }

func sum(items []OrderItem) (Money, error) {
	if len(items) == 0 {
		return Money{}, ErrEmptyOrder
	}
	t := items[0].Subtotal()
	for _, it := range items[1:] {
		var err error
		if t, err = t.Add(it.Subtotal()); err != nil {
			return Money{}, err
		}
	}
	return t, nil
}
```

**对比一下贫血写法，差异一眼可见**：

```go
// ❌ 贫血：Order 是数据袋，状态机规则散落在 N 个 Service 里，谁都能改坏
type Order struct {
	ID     string
	Status int  // 任何人 o.Status = 3 就绕过了所有校验
	Total  float64
}
func (s *OrderService) Cancel(id string) error {
	o, _ := s.dao.Get(id)
	if o.Status == 3 { return errors.New("已发货") } // 这条规则在别的 Service 里可能被漏掉
	o.Status = 4
	return s.dao.Update(o)
}
```

### 5.4 Repository 端口 + 应用服务（事务边界）

```go
// ---------- domain/repository.go：接口定义在领域层 ----------
package domain

import "context"

var ErrConcurrentModification = errors.New("并发修改冲突")

type Repository interface {
	FindByID(ctx context.Context, id OrderID) (*Order, error)
	Save(ctx context.Context, o *Order) error // 整个聚合一起存，内部带 version 乐观锁
}
```

```go
// ---------- app/cancel_order.go：应用层只做编排 ----------
package app

type CancelOrderCmd struct {
	OrderID   domain.OrderID
	Reason    string
	OperatorID string
}

type CancelOrderHandler struct {
	tx     TxManager        // 出站端口：事务
	repo   domain.Repository // 出站端口：持久化
	outbox OutboxWriter      // 出站端口：事件外发
}

func (h *CancelOrderHandler) Handle(ctx context.Context, cmd CancelOrderCmd) error {
	// 事务边界在应用层 —— 领域层不该知道「事务」这个技术概念
	return h.tx.Do(ctx, func(ctx context.Context) error {
		o, err := h.repo.FindByID(ctx, cmd.OrderID)
		if err != nil {
			return err
		}
		if err := o.Cancel(cmd.Reason); err != nil { // ★ 唯一一行业务逻辑，且在领域层
			return err
		}
		if err := h.repo.Save(ctx, o); err != nil { // 乐观锁失败 → 整个事务回滚，上层重试
			return err
		}
		// ★ 同事务写 outbox：保证「状态变更」与「事件投递意图」原子
		return h.outbox.AppendAll(ctx, o.PullEvents())
	})
}
```

注意这个 Handler 的形状：**没有一个 `if` 是业务规则**，全是流程胶水。这就是「应用层要薄」的具象含义 —— 面试时把这段讲出来，比背十条定义有用。

### 5.5 防腐层 ACL：把第三方模型挡在外面

```go
// ---------- infra/payment_acl.go ----------
package infra

// 第三方支付回调的原始结构 —— 丑陋但不是我能控制的
type alipayCallback struct {
	OutTradeNo  string `json:"out_trade_no"`
	TradeStatus string `json:"trade_status"` // "TRADE_SUCCESS" / "TRADE_CLOSED" / ...
	TotalAmount string `json:"total_amount"` // ★ 字符串的金额
	GmtPayment  string `json:"gmt_payment"`  // "2026-07-23 15:04:05"
}

// PaymentACL：翻译层。领域层只认 domain.PaymentResult，永远看不到 alipay 的存在。
// 第三方改字段/加状态 → 只改这里，领域层零改动。
type PaymentACL struct{}

func (a PaymentACL) Translate(raw []byte) (domain.PaymentResult, error) {
	var cb alipayCallback
	if err := json.Unmarshal(raw, &cb); err != nil {
		return domain.PaymentResult{}, err
	}
	yuan, err := strconv.ParseFloat(cb.TotalAmount, 64)
	if err != nil {
		return domain.PaymentResult{}, err
	}
	amount, err := domain.NewMoney(int64(math.Round(yuan*100)), "CNY")
	if err != nil {
		return domain.PaymentResult{}, err
	}
	paidAt, _ := time.ParseInLocation("2006-01-02 15:04:05", cb.GmtPayment, time.Local)

	return domain.PaymentResult{
		OrderID:  domain.OrderID(cb.OutTradeNo),
		Amount:   amount,
		Succeeded: cb.TradeStatus == "TRADE_SUCCESS", // ★ 外部枚举 → 领域语义
		PaidAt:   paidAt,
	}, nil
}
```

### 5.6 Go 特有的落地坑

| 坑 | 说明 | 解法 |
|----|------|------|
| **领域对象字段不导出，ORM 存不了** | Go 反射填不了私有字段 | Repository 里手写 `domain ⇄ PO(持久化对象)` 转换；或在 infra 定义同包 PO 结构体 |
| **Save 全量更新 vs 增量** | 没有脏检查 | 小聚合直接全量 UPDATE + version 乐观锁；明细用「先删后插」或对比 diff |
| **值对象没有值语义的深拷贝** | 含 slice/map 的 VO 仍可被外部改 | VO 内避免暴露 slice；必须暴露时返回副本 |
| **循环依赖** | domain 想用 infra 的工具 | 说明抽象错了：把接口定义在 domain，实现放 infra |
| **`time.Now()` 散落在领域层** | 单测不可控 | 时间作为参数传入（见 `Pay(paidAt)`），或注入 `Clock` 接口 |

---

## 六、反模式清单（面试问「你踩过什么坑」时直接用）

| # | 反模式 | 症状 | 纠正 |
|:-:|--------|------|------|
| 1 | **DDD-Lite / 目录 DDD** | 分了四层包，`app` 里还是 3000 行事务脚本 | 判定标准只有一条：**业务规则在不在领域对象里** |
| 2 | **巨型聚合** | 一个聚合 JOIN 六张表；乐观锁频繁冲突 | 按不变性重切，跨聚合用 ID 引用 + 事件 |
| 3 | **Repository 退化成 DAO** | 出现 `FindByAAndBOrderByCLimit10` | 查询走 CQRS 读侧，Repository 只服务写模型 |
| 4 | **领域层被框架污染** | domain 结构体挂满 `gorm:"column:xx"`、`json:"xx"` | 严格做要拆 PO/DTO；小团队可容忍 ORM tag，但**绝不能出现 gorm/gin 的 import** |
| 5 | **全量上 DDD** | 字典表、配置表也建聚合、Repository、事件 | 只在**核心域**用；支撑域 CRUD 就够 |
| 6 | **先拆微服务再想边界** | 一个需求要改 5 个服务、联调两周 | 先在单体内划清 BC（模块化单体），跑稳了再物理拆 |
| 7 | **领域事件当同步函数用** | `raise` 之后立刻同步调用下游，失败就整体回滚 | 那就直接调用，别叫事件；真要解耦就异步 + Outbox + 幂等 |
| 8 | **贫血 VO** | `type Money struct{ Amount float64; Currency string }`，字段全导出、无行为 | 收敛构造函数、字段不导出、行为内聚 |
| 9 | **统一语言只在文档里** | 代码里全是 `info`/`data`/`ext`/`util` | 命名评审是最便宜的 DDD 实践 |

**「什么时候不该用 DDD」（很加分的一问）**：

- 纯 CRUD 后台、报表系统 —— 业务规则约等于零，DDD 只增加样板代码。
- **数据管道 / ETL / 数仓**（贴你的工作面）：本质是「数据变换 + 编排」，领域模型极薄，复杂度在**调度、幂等、回溯、数据倾斜**上。这类系统该投资的是**分层建模（ODS/DWD/DWS/ADS）、血缘、可观测性**，而不是聚合根。硬套 DDD 是拿错药方。
- 团队没有稳定的领域专家可对话 —— 统一语言建不起来，DDD 缺地基。
- 生命周期 < 半年的项目 —— 收益期没到就下线了。

---

## 七、面试追问链条（三层深度演练）

**链条 1｜边界**
> Q: 微服务怎么拆？
> A: 先做战略设计划 BC（事件风暴），BC 是逻辑边界，微服务是部署边界。
> → 追问：BC 怎么划？
> A: 看统一语言在哪里发生歧义（同一个词含义变了就该切）；看业务能力闭环；看团队边界（康威定律）。
> → 追问：怎么验证划对了？
> A: 看**变更内聚性** —— 一个典型需求是否只改一个 BC。若长期需要跨 BC 同步改，说明边界切错，应合并或重划。

**链条 2｜一致性**
> Q: 聚合边界怎么定？
> A: 按不变性（必须强一致的规则）定，并遵守「一个事务一个聚合」。
> → 追问：那订单-库存跨聚合怎么保证不超卖？
> A: 库存聚合内部保证 `可用量 ≥ 0`（DB 原子扣减或乐观锁），订单侧最终一致；失败走补偿/超时释放。
> → 追问：消息丢了怎么办？
> A: Outbox + 至少一次投递 + 消费端幂等（唯一键/去重表/状态机），再加对账兜底。
> → 追问：为什么不用 2PC？
> A: 2PC 同步阻塞、协调者单点、锁持有时间跨网络，在高并发下吞吐塌陷；互联网系统普遍选**最终一致 + 补偿**。

**链条 3｜落地**
> Q: 你们项目怎么落地 DDD 的？
> A: 只在核心域落，模块化单体起步，四层 + 依赖倒置，读写分离建模。
> → 追问：领域层怎么保证不被污染？
> A: 用 `internal/` + lint 规则禁止 domain 包 import 任何基础设施/框架；CI 里跑依赖检查。
> → 追问：怎么证明有收益？
> A: 领域层可以脱库单测（覆盖率与执行速度是硬指标）；核心规则改动的影响面收敛到单个包；新人上手能通过读 `order.go` 理解业务状态机。

---

## 八、自测题（盖住答案自己讲一遍）

<details>
<summary><b>1. 一句话说清 DDD 解决什么问题，以及它的核心手段。</b></summary>

解决**业务复杂度**（不是技术复杂度）。核心手段：用**统一语言**让代码结构映射业务结构，用**限界上下文**承认「模型不可能全局统一」，把业务规则从 Service 收回到**领域对象**里。
</details>

<details>
<summary><b>2. 实体和值对象怎么区分？为什么建议优先用值对象？</b></summary>

区分靠**标识**：实体有 ID、按 ID 判等、可变、有生命周期；值对象无 ID、按全部属性判等、不可变。
优先 VO 的四个理由：不可变 ⇒ 线程安全可共享；把校验收拢在一处；纯函数易测；消灭基本类型偏执（编译期挡住传参错误）。
</details>

<details>
<summary><b>3. 说出聚合的四条设计原则，并解释第 4 条为什么重要。</b></summary>

① 边界内保护不变性 ② 尽量小 ③ 聚合间只用 ID 引用 ④ 一个事务只改一个聚合，边界外最终一致。
第 4 条决定了系统的**可伸缩性**：它把「跨聚合一致性」从分布式事务降级为异步最终一致，避免长事务和跨服务锁；同时天然为后续拆微服务留好了缝。
</details>

<details>
<summary><b>4. 领域服务和应用服务的区别？各举一例。</b></summary>

领域服务在领域层，装**不属于任何单个实体的业务规则**（跨账户转账、多因子定价），领域专家能听懂。
应用服务在应用层，只做**编排**：开事务、鉴权、调 Repository、调领域对象、发事件，**不含业务规则**（`CancelOrderHandler.Handle`）。
</details>

<details>
<summary><b>5. Repository 和 DAO 有什么本质区别？Repository 里该不该放复杂查询？</b></summary>

Repository 面向**聚合根**、心智模型是内存集合、接口在领域层、整聚合存取；DAO 面向**表**、任意字段增删改查。
复杂查询不该放：那是读场景，走 **CQRS 查询侧**直接 SQL 出 DTO。硬塞会让 Repository 退化成 DAO —— DDD 落地失败的头号症状。
</details>

<details>
<summary><b>6. 领域事件和集成事件的区别？「改库 + 发 MQ」怎么保证原子？</b></summary>

领域事件在 BC 内、走进程内总线、可携带领域对象；集成事件跨 BC、走 MQ、必须是扁平版本化 DTO（对外契约）。
原子性：**双写做不到原子** → **Outbox/本地消息表**：同一本地事务里写业务表 + outbox 表，独立 relay（轮询或 CDC 读 binlog）投递 MQ，消费端**幂等**兜底。
</details>

<details>
<summary><b>7. 画出 DDD 四层架构并说明依赖方向，跟传统三层的关键差异在哪？</b></summary>

用户接口 → 应用 → 领域 ← 基础设施（**基础设施实现领域定义的接口**）。
关键差异两点：① 业务规则从 Service 移入领域对象；② 依赖倒置 —— 基础设施从「被依赖的底层」变成「最外层的实现"。收益是领域层可脱库单测、可替换存储与协议。
</details>

<details>
<summary><b>8. 订单和库存该不该放一个聚合？不放的话超卖怎么解决？</b></summary>

不该。不变性不同、聚合会过大、热点 SKU 锁竞争。
超卖由**库存聚合内部**保证：`UPDATE stock SET qty=qty-? WHERE sku=? AND qty>=?` 的原子扣减（或乐观锁 version）。跨服务时用预扣 + TCC/Saga + 超时释放 + 对账。
</details>

<details>
<summary><b>9. 什么场景不该用 DDD？举三个。</b></summary>

纯 CRUD/报表系统（规则约等于零）；**ETL/数据管道**（复杂度在调度、幂等、回溯、倾斜，不在领域模型）；无稳定领域专家或生命周期极短的项目。另外支撑域/通用域也不该上，只在**核心域**投入。
</details>

<details>
<summary><b>10. 「我们做了 DDD」但其实没做，怎么一眼识破？</b></summary>

看**业务规则在哪**。如果 `domain` 包里的结构体只有字段没有行为、状态靠外部赋值、所有 `if` 都在 AppService 里，那就是**目录 DDD**：换了包名的事务脚本。第二个信号：Repository 里出现一堆 `FindByXxxAndYyy`。
</details>

---

## 九、毕业标准

不是「读过」，是能做到这五条：

1. **能白板画出**四层/六边形架构图，并说清依赖倒置发生在哪条边、带来什么收益。
2. **能对一个陌生业务现场做战略设计**：给你「外卖平台」，10 分钟内划出 4~6 个 BC，指出哪个是核心域，标出至少两处需要 ACL 的地方。
3. **能徒手写出一个聚合根**（Go），包含：值对象、不变性校验、状态机行为方法、乐观锁字段、领域事件累积；并能指出贫血写法差在哪。
4. **能完整答出跨聚合一致性链条**：为什么不用 2PC → Outbox 怎么做 → 消费幂等怎么做 → 对账兜底。
5. **能说清成本与边界**：什么时候不该用 DDD，以及「模块化单体 → 微服务」的正确顺序。

**动手作业（三角咬合的「动手」环节）**：
参照 [ddd-demo/](./ddd-demo/README.md) 跑通两个 BC 的完整往返，然后做 README 里的**改造练习**——
尤其是第 6 条：**故意把 `Stock` 改成「一个仓库一个聚合」，并发下单 100 次，看乐观锁冲突率怎么爆炸**。
DDD 的原则光背没用，亲手违反一次、看到系统怎么塌，才算真记住。
硬性要求：改完之后 **`domain` 包的单测仍然不启动任何 Docker、不 import 任何第三方库**。

---

## 附录：自我思考

**1. DDD 最容易被误读成「一套目录结构」。**
我以前对 DDD 的印象就是「多分几层包」，这恰恰是最没价值的部分。写这篇最大的收获是意识到：**分层是结果不是手段**。真正的判定标准只有一句 —— **业务规则住在哪里**。住在 Service 里就是事务脚本，包名叫 domain 也没用。

**2. 战略设计比战术设计重要，但面试和文章都反着来。**
实体/值对象/聚合这些名词好记好考，所以到处在讲；但真实项目里造成最大痛苦的是**边界划错**：一个需求横跨三个服务、联调两周、上线回滚。聚合切错还能重构，BC 切错基本要推倒重来。以后看到「我们要拆微服务」，第一反应应该是「BC 划了吗」而不是「用什么注册中心」。

**3. 「一个事务只改一个聚合」这条约束，其实是分布式系统的入口。**
这条原则表面在讲代码设计，实际是在逼你面对最终一致性。它和我在 MySQL 那条线学的 binlog、和第 8 格的分布式事务、和 Kafka 的幂等消费是**同一个问题的不同切面**。Outbox 模式恰好把这三条线缝在了一起 —— 这种交叉点应该重点记，因为面试官最爱在交叉点上追问。

**4. 诚实地讲，DDD 对我当前的工作面（后端 + 轻量大数据）是「半个刚需」。**
后端交易类业务是刚需；但 ETL / 数仓那半边，DDD 收益很低 —— 那边的复杂度在调度依赖、幂等重跑、数据倾斜、血缘上，领域模型薄得几乎不存在。写这篇时想清楚这一点反而是有价值的：**知道一个方法论的适用边界，比会背它的名词更像四年经验**。面试被问到「你们数据平台怎么用 DDD 的」，正确答案是解释为什么不用，而不是硬编一个聚合根。

**5. 待补的钩子**：
- 事件风暴（Event Storming）没实操过，只是纸上谈兵 —— 找个真实业务（比如自己写的项目）走一遍流程。
- CQRS 的读模型同步延迟问题（写完立刻读读不到）在实践中怎么处理，需要单独挖一篇。
- 与能力地图第 8 格「分布式事务 / Saga」的交叉部分，等那一格开工时回来补链接。
