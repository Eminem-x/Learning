# Airflow 调度 与 MongoDB 文档库

> 面试加分向学习笔记。假设你维护 ETL 多年、用过 Airflow、见过 MongoDB，本文在你已知边界上稍作拓展，帮你把"用过"变成"讲得清"。

---

## 第一部分：Airflow —— 工作流调度（重点）

### 1. Airflow 是什么 / 解决什么问题

Airflow 是一个**工作流编排调度平台（workflow orchestration）**：你用 **Python 代码**把一堆任务的依赖关系、执行顺序、定时规则写出来，它负责按时把这些任务调起来、管好先后顺序、失败重试、并把整个过程可视化。

它解决的核心痛点，正是 ETL 团队最熟悉的那些：

- **谁先谁后**：抽数 → 清洗 → 聚合 → 导出，下游必须等上游成功。没有编排就靠 `sleep` + 一堆 crontab 靠猜时间，脆弱且不可维护。
- **定时跑**：每天凌晨 2 点跑昨天的数据。
- **失败重试 / 告警**：某个任务挂了自动重试 N 次，超过就告警，而不是第二天早上才发现表是空的。
- **回补历史（backfill）**：新加了一个指标，要把过去 30 天重算一遍。
- **可观测**：每个任务什么时候跑的、跑了多久、成功还是失败、日志在哪，一个 Web 界面看全。

**一句话记忆**：Airflow = "用代码定义的、带依赖关系的、可重跑可观测的 crontab 升级版"。它**只负责编排调度，不负责算数据**——真正的计算在 Spark / SQL 引擎 / 外部服务里跑，Airflow 只是那个"发号施令并盯着结果"的指挥。

> 关键设计哲学：**Configuration as Code**。DAG 是 Python 文件，能进 Git、能 code review、能复用函数、能动态生成。这是它相对老式调度工具（图形拖拽、XML 配置）最大的优势。

---

### 2. 核心概念：DAG（有向无环图）

**DAG = Directed Acyclic Graph = 有向无环图**。一个 DAG 就是**一个完整的工作流**。

- **节点（node）** = task（一个任务，比如"抽订单表"）
- **边（edge）** = 依赖关系（"抽完订单才能清洗订单"），有方向
- **Acyclic（无环）** = 不允许出现环

```python
extract >> transform >> load   # extract 完 → transform → load
```

**为什么必须无环？**

因为**环意味着"A 依赖 B，B 又依赖 A"这种死锁**：调度器要跑 A 得先等 B，跑 B 又得先等 A，永远无法确定一个合法的启动顺序（拓扑排序不存在）。DAG 的无环约束保证了**任意依赖图都能拓扑排序出一个确定的执行序列**，调度器才知道"先跑谁、后跑谁、谁可以并行"。

> 记忆点：Directed（有方向，表达上下游）+ Acyclic（无环，保证可排序）+ Graph（不是简单的线性链，可以有分叉汇合）。

---

### 3. Operator / Task / Task Instance —— 三个容易混的概念

这三个是"模板 → 实例化 → 某次运行"的三层关系，面试常追问区别：

| 概念 | 是什么 | 类比 |
|------|--------|------|
| **Operator** | 任务的**模板/类**，定义"这类任务怎么干" | 类（Class） |
| **Task** | DAG 里的一个**具体节点**，是 Operator 的一次实例化 | 对象（Object） |
| **Task Instance (TI)** | 某个 Task 在**某个 execution_date 的一次具体运行** | 对象的一次方法调用/一次执行记录 |

**Operator** 是 Airflow 提供的一堆任务模板，常见的：

- `BashOperator` —— 跑一条 shell 命令
- `PythonOperator` —— 跑一个 Python 函数
- `SparkSubmitOperator` —— 提交 Spark 任务
- `SQLExecuteQueryOperator` / 各数据库 Operator —— 执行 SQL
- `KubernetesPodOperator` —— 起一个 K8s Pod 跑任务
- `EmptyOperator`（旧名 `DummyOperator`）—— 空节点，常用作汇合点/占位

**Task** 是你在 DAG 里写 `t1 = BashOperator(task_id="extract", ...)` 得到的那个节点。

**Task Instance** 是"extract 这个 task 在 2026-07-20 这一天的那次运行"。同一个 task，2026-07-20 和 2026-07-21 是两个不同的 Task Instance，各有各的状态（success/failed/running/up_for_retry...）、各有各的日志。**你在 UI 里点格子看到的每一个方块，就是一个 Task Instance。**

---

### 4. 调度语义 —— execution_date 是重灾区，重点看

#### schedule_interval（调度周期）

定义 DAG 多久跑一次，支持 cron 表达式或预设：

```python
schedule="0 2 * * *"   # 每天凌晨 2 点   （新版参数名 schedule，老版 schedule_interval）
schedule="@daily"      # 等价于 0 0 * * *
schedule="@hourly"     # 每小时
schedule=None          # 不定时，只能手动/被外部触发
```

#### execution_date / logical_date —— 最容易懵的概念

**这是 Airflow 面试的送命题，也是加分题。**

`execution_date`（新版叫 `logical_date`）**不是任务实际运行的时刻**，而是**这批数据所属的"逻辑日期 / 数据窗口的开始时间"**。

关键理解：**Airflow 是在一个调度周期结束后，才去跑这个周期的数据。**

用最经典的 **T+1** 场景说清楚：

- 一个 `@daily` 的 DAG，处理"某一天的全量数据"。
- 7 月 21 日凌晨 2 点这次运行，处理的是 **7 月 20 日**的数据。
- 所以这次运行的 `execution_date` = **2026-07-20**（数据的逻辑日期），而**真实墙上时钟运行时间**约是 2026-07-21 02:00。

为什么这么设计？因为**"20 号这一天的数据"必须等 20 号过完（21 号凌晨）才完整**，才能开始处理。所以 execution_date 天然落后于真实运行时间一个周期。

**这带来一个巨大的好处**：你的 SQL 里永远用 `execution_date`（模板变量 `{{ ds }}` = 逻辑日期字符串 `2026-07-20`）来圈定数据分区，而**不用 `now()` / 当前系统时间**。这样：

- 今天正常跑，处理的是昨天的分区 ✅
- 三天后回补历史，`execution_date` 还是那天，处理的还是那天的分区 ✅ —— 结果完全一致

> **一句话答面试**："execution_date 代表这批数据的逻辑日期，不是实际运行时刻。T+1 场景下，凌晨跑的任务它的 execution_date 是昨天，因为处理的是昨天已经完整的那批数据。ETL 里所有取数、写分区都要用它而不是 now()。"

常用模板变量：`{{ ds }}` = `2026-07-20`，`{{ ds_nodash }}` = `20260720`，`{{ data_interval_start }}` / `{{ data_interval_end }}` = 新版更精确的数据窗口起止。

#### catchup（追赶）

```python
catchup=True   # 默认。若 start_date 是 30 天前，第一次上线会把这 30 天缺的周期全部补跑一遍
catchup=False  # 只从"现在"开始跑，不补历史
```

**新上线的 DAG 强烈建议先设 `catchup=False`**，否则一上线就炸出几十上百个历史 Task Instance 把集群打满——这是新手最常踩的坑之一。

#### backfill（回补历史）

**backfill = 主动把过去某个时间段的 DAG 重新跑一遍**。典型场景：新加了指标、修了 bug、上游历史数据修正。

```bash
airflow dags backfill -s 2026-06-01 -e 2026-06-30 my_dag
```

它会为 6 月每一天生成 Task Instance，按各自的 execution_date 逐日重算。**backfill 能正确工作的前提是任务幂等**（见下节）——否则回补会污染数据。

> catchup vs backfill：catchup 是"上线时自动补齐 start_date 到现在的空档"（被动、自动）；backfill 是"你手动命令重跑一段历史"（主动、手动）。两者机制相同，触发方式不同。

---

### 5. 依赖与触发

#### 上下游依赖（`>>` 语法）

```python
extract >> transform >> load          # 链式
extract >> [clean_a, clean_b] >> merge  # 分叉再汇合
load << transform                     # 反向写法，等价于 transform >> load
```

`a >> b` 读作 "a 完成后触发 b" / "b 依赖 a"。

#### Sensor（传感器）—— 等待外部条件就绪

**Sensor 是一类特殊的 Operator，作用是"卡住等待某个外部条件满足才放行下游"**。ETL 里极其常用，因为你的任务经常依赖别人产出的数据。

常见 Sensor：

- `FileSensor` / `S3KeySensor` —— 等某个文件 / S3 对象出现（上游落数据了吗？）
- `ExternalTaskSensor` —— 等另一个 DAG 的某个 task 成功（跨 DAG 依赖）
- `SqlSensor` —— 轮询直到某条 SQL 返回满足条件的结果（分区写好了吗？行数达标了吗？）

两种工作模式（面试可提，显专业）：

- **poke 模式**（默认）：占着一个 worker 槽位反复轮询，简单但耗资源。
- **reschedule 模式**：每次检查完就释放 worker，睡一会儿再被重新调起，**长时间等待务必用它**，否则一堆 Sensor 挂着会把 worker 占满。

> 一句话答："Sensor 就是一个会阻塞等待的任务，等外部条件（文件到了、上游分区写好了、另一个 DAG 跑完了）满足才让下游开始。长等待要用 reschedule 模式省 worker。"

#### trigger rule（触发规则）

默认一个 task 要等**所有**上游都成功才运行（`all_success`）。trigger rule 可以改这个逻辑：

- `all_success`（默认）：全部上游成功
- `all_done`：全部上游跑完（不管成功失败）—— 常用于"无论前面成没成，最后都要做清理/通知"
- `one_success`：任一上游成功即可
- `none_failed`：没有失败的（成功或跳过都行）

典型用途：一个"发告警"或"清理临时表"的收尾 task 设 `all_done`，保证无论前面成败它都执行。

---

### 6. 幂等性（Idempotency）—— ETL 调度的灵魂

**幂等 = 同一个任务，用同样的输入（同一个 execution_date）跑一次和跑 N 次，结果完全一样。**

**为什么 Airflow 任务必须幂等？**

因为在调度系统里，**同一个 Task Instance 被跑多次是常态，不是异常**：

- 任务失败**自动重试**（retries=3）
- 上线时 **catchup** 补历史
- 修 bug / 加指标后 **backfill** 回补
- 手动 clear 某个格子重跑

如果任务不幂等（比如"往表里 `INSERT` 追加数据"），那么每重跑一次就多插一份，回补 30 天可能把数据翻好几倍——**静默的数据污染，最难查**。

**怎么做到幂等？核心一条：按 execution_date 分区，覆写而非追加。**

```sql
-- ❌ 非幂等：追加，重跑就重复
INSERT INTO orders_daily SELECT ... FROM source WHERE dt = '{{ ds }}';

-- ✅ 幂等：先删本分区再写，或直接 OVERWRITE 本分区
INSERT OVERWRITE TABLE orders_daily PARTITION (dt = '{{ ds }}')
SELECT ... FROM source WHERE dt = '{{ ds }}';
```

要点：

1. **写入用 `execution_date` 圈定的分区**（`{{ ds }}`），一次运行只负责一个分区。
2. **用 OVERWRITE / delete-then-insert 覆盖该分区**，而不是 append。这样重跑该分区只是"用相同结果覆盖相同结果"，无副作用。
3. **取数也用 `execution_date` 而非 `now()`**，保证任何时候重跑都读同一批源数据。

> 一句话答："幂等就是同一 execution_date 重跑结果不变。做法是每次运行只处理并 OVERWRITE 自己那个日期分区，取数写数都用 execution_date 不用 now()。因为 Airflow 里重试、catchup、backfill 都会重跑任务，不幂等就会数据翻倍。"

---

### 7. 架构简述（点到为止，面试能画个框就够）

```
        ┌─────────────┐        ┌──────────────┐
        │  Webserver  │        │  Scheduler   │
        │  (UI/可观测) │        │ (解析DAG/派活)│
        └──────┬──────┘        └──────┬───────┘
               │                      │
        ┌──────┴──────────────────────┴───────┐
        │        Metadata DB (元数据库)          │
        │   DAG/TI 状态、调度记录、连接、变量       │
        │        (PostgreSQL / MySQL)          │
        └──────┬───────────────────────────────┘
               │
        ┌──────┴──────┐        ┌──────────────┐
        │  Executor   │───────▶│   Worker(s)  │
        │ (决定在哪跑) │        │  (真正执行任务) │
        └─────────────┘        └──────────────┘
```

- **Scheduler（调度器）**：Airflow 的大脑。持续解析 DAG 文件、计算哪些 Task Instance 到点该跑、依赖是否满足，然后把它们派发出去。
- **Executor（执行器）**：决定任务"在哪、怎么"执行的策略层。常见：`LocalExecutor`（本机多进程）、`CeleryExecutor`（分布式 worker 队列）、`KubernetesExecutor`（每个任务起一个 Pod）。**注意 Executor 是配置项，不是独立进程；Celery/K8s 模式下真正干活的是 Worker。**
- **Worker（工作节点）**：真正执行任务代码的进程（Celery/K8s 模式下才有独立 Worker）。
- **Metadata DB（元数据库）**：Airflow 的"记忆"。所有 DAG/TI 的状态、调度历史、Connection、Variable、XCom 都存这，通常是 PostgreSQL/MySQL。**它是单点核心，挂了整个 Airflow 就瞎了。**
- **Webserver**：那个 Web UI，看 DAG 图、任务状态、日志、手动触发/clear。

> 记忆链条：Scheduler 读 DAG 决定派谁 → 通过 Executor → Worker 执行 → 状态全写进 Metadata DB → Webserver 从 DB 读出来给你看。

---

### 8. 常见坑（面试问"踩过什么坑"可用）

1. **execution_date 理解错**：以为它是"任务运行时刻"，在 SQL 里对不齐一天，取数取错分区。牢记它是"数据逻辑日期"，T+1 下是昨天。
2. **任务不幂等，回补出错**：用 `INSERT` 追加而非 `OVERWRITE` 分区，backfill/重试后数据翻倍。前面第 6 节的核心。
3. **新 DAG catchup 忘了关**：`catchup=True`（默认）+ start_date 设得很早，一上线炸出上百个历史 TI 把集群打爆。上线先 `catchup=False`。
4. **task 之间传大数据**：XCom 是通过 Metadata DB 传递的，**只适合传小元数据（路径、几个参数、行数），绝不能传 DataFrame / 大 JSON**——会把元数据库撑爆、拖垮 Scheduler。正确姿势：**上游把数据写到 S3/HDFS，通过 XCom 只传"路径"，下游按路径去读。传路径不传数据。**
5. **Sensor 用 poke 长等**：一堆 Sensor 用默认 poke 模式挂着长时间等待，把 worker 槽位占满导致其他任务无 worker 可用。长等待改 reschedule。
6. **DAG 文件里放重逻辑 / 顶层写慢代码**：Scheduler 会**反复解析每个 DAG 文件**（默认几十秒一轮），如果文件顶层有连数据库、跑查询之类的慢操作，会拖垮整个 Scheduler。DAG 文件应只做"定义结构"，重活放进 task 的执行体里。

---

## 第二部分：MongoDB —— 文档数据库（简版）

### 1. MongoDB 是什么

**MongoDB 是一个 NoSQL 文档型数据库**，存储单位是**文档（document）**，本质是 **JSON**（底层用二进制的 **BSON** 存，支持更多类型如 Date、二进制、ObjectId）。

最大特点是 **schema 灵活（schema-less / schema-flexible）**：同一个集合里的文档字段可以不完全一样，加字段不用 `ALTER TABLE`，直接写进去就行。

```json
{
  "_id": ObjectId("..."),
  "user": "alice",
  "order_items": [
    { "sku": "A123", "qty": 2, "price": 9.9 },
    { "sku": "B456", "qty": 1 }
  ],
  "address": { "city": "Shenzhen", "zip": "518000" },
  "tags": ["vip", "new"]
}
```

注意上面：**数组、嵌套对象直接塞在一个文档里**——这在关系型里得拆成好几张表 JOIN，在 MongoDB 里一个文档搞定。这就是"文档模型"的核心卖点。

---

### 2. 在数据体系里的定位

**MongoDB 偏 OLTP / 在线业务存储，不是分析引擎。**

- 它擅长：**高并发的按主键/索引读写单个或少量文档**（在线业务：用户资料、商品目录、订单、日志、CMS 内容、IoT 上报……），schema 多变、结构嵌套的半结构化业务数据。
- 它不擅长：**大规模扫描聚合分析**。你不会拿 MongoDB 当数仓去跑"全表几亿行做多维分析"。那是 **OLAP** 引擎（Hive/Spark/StarRocks/ClickHouse/Presto）的活。

**和 OLAP 那套的区别**：

| 维度 | MongoDB (OLTP 向) | OLAP 引擎 (数仓/分析) |
|------|-------------------|----------------------|
| 典型操作 | 点查、单文档增删改 | 大范围扫描、聚合、JOIN |
| 数据量级 | 单次读写小 | 单次扫描海量 |
| 存储 | 行/文档式，为在线读写优化 | 列式，为分析扫描优化 |
| 角色 | 业务系统的在线库 | 分析/报表的离线库 |

> 一句话定位：MongoDB 是**业务系统那一侧的在线数据库**（跟 MySQL 同一层，只是 schema 更灵活），在大数据链路里它通常是**数据的来源（source）**，不是分析计算的地方。

---

### 3. 核心概念（对比关系型记）

| MongoDB | 关系型 (MySQL) | 说明 |
|---------|---------------|------|
| **database** | database | 一样，库 |
| **collection**（集合） | table（表） | 文档的容器，但**不强制 schema** |
| **document**（文档） | row（行） | 一条记录，但可嵌套、字段可不同 |
| **field**（字段） | column（列） | 文档里的键值对 |
| `_id` | 主键 | 每个文档必有，默认 ObjectId，自动生成且唯一 |
| 嵌套/数组 | 需拆表 + 外键 + JOIN | Mongo 直接内嵌在一个文档里 |

核心差异就一句：**关系型是"表 + 行 + 强 schema + 靠 JOIN 关联"，MongoDB 是"集合 + 文档 + 灵活 schema + 靠内嵌减少 JOIN"。**

---

### 4. 什么时候用 / 什么时候不用 MongoDB

**适合用（✅）：**

- **schema 多变、需要快速迭代**：产品早期字段天天改，用 Mongo 不用频繁 migration。
- **数据天然是嵌套/文档结构**：一个订单带一堆明细、一份商品带多规格属性、一份 CMS 内容带任意扩展字段——一个文档装下，读写整取整存，不用 JOIN。
- **半结构化 / 字段不固定**：不同来源的日志、事件、爬取数据，字段各不相同。
- **高并发点读写**：按 `_id` 或索引查单个文档，性能好、水平扩展（分片 sharding）成熟。

**不适合用（❌）：**

- **复杂多表 JOIN 的分析**：MongoDB 的关联能力弱（`$lookup` 有但不是它强项），需要频繁多表关联就用关系型/数仓。
- **强事务 / 强一致的多表操作**：虽然 4.0+ 支持多文档事务，但如果你的业务核心就是复杂事务（转账、库存扣减对账），传统 RDBMS 往往更稳更合适。
- **大规模离线分析 / 报表**：那是 OLAP 引擎的活，别拿 Mongo 硬跑。

> 决策口诀：**"结构灵活、读写整个文档、少 JOIN" 用 Mongo；"强事务、多表关联、大规模分析" 别用 Mongo。**

---

### 5. 在 ETL 里的角色

MongoDB 在大数据链路里最常见的角色是 **source（数据源）**：

```
MongoDB (业务在线库)
    │  抽取 (CDC / 定时全量或增量导出)
    ▼
数据湖 / 数仓 (S3 + Hive / Iceberg …)
    │  Spark / SQL 清洗聚合
    ▼
分析 / 报表 / OLAP
```

- 业务系统把半结构化数据写在 MongoDB 里（在线读写）。
- ETL 把这些数据**抽出来**（定时全量/增量导出，或用 CDC 工具如 Debezium 捕获变更）灌进数据湖。
- 进湖后往往要**把嵌套文档"拍平"（flatten）**成结构化的行列，才能给下游 Spark/SQL 做分析——因为文档里的数组/嵌套对象，在列式分析引擎里需要展开或用 `struct`/`array` 类型承载。

> 一句话答："MongoDB 在 ETL 里通常是 source，把在线业务数据抽进数据湖。抽的时候要处理它的嵌套结构——把文档拍平成结构化行列，下游数仓才好分析。"

---

## 面试高频问题 + 答题要点

**Q1. Airflow 的 DAG 是什么？为什么必须无环？**
DAG 是有向无环图，一个 DAG = 一个工作流，节点是 task、边是依赖关系。必须无环是因为环会导致依赖死锁（A 等 B、B 等 A），无环才能拓扑排序出确定的执行顺序，调度器才知道先跑谁。

**Q2. execution_date（logical_date）是什么意思？**
它是**这批数据的逻辑日期**，不是任务实际运行的墙上时钟时间。Airflow 在一个调度周期结束后才跑这个周期的数据，所以 T+1 场景下，凌晨跑的任务 execution_date 是昨天（因为处理的是昨天已完整的数据）。ETL 里取数写分区都用它（`{{ ds }}`）而不是 `now()`，这样正常跑和回补历史结果一致。

**Q3. 为什么 ETL 任务必须幂等？怎么做到？**
因为 Airflow 里重试、catchup、backfill 都会让同一任务重跑多次，不幂等（如 INSERT 追加）会导致数据翻倍污染。做法：每次运行只处理并 **OVERWRITE 自己那个 execution_date 分区**，取数写数都用 execution_date 不用 now()，这样重跑就是相同结果覆盖相同结果，无副作用。

**Q4. 什么是 backfill？和 catchup 什么区别？**
backfill 是主动命令 Airflow 把过去某段时间的 DAG 重新跑一遍（新加指标/修 bug 后回补历史），按每天各自的 execution_date 逐日重算，前提是任务幂等。catchup 是 DAG 上线时**自动**补齐从 start_date 到现在的空档周期（被动、默认开启）。两者机制相同，一个手动一个自动。新 DAG 上线常先关 catchup 防止炸集群。

**Q5. Sensor 是干嘛的？poke 和 reschedule 有啥区别？**
Sensor 是一类会阻塞等待的特殊 Operator，等外部条件满足才放行下游（文件到了 FileSensor、上游分区写好了 SqlSensor、另一个 DAG 跑完了 ExternalTaskSensor）。poke 模式占着 worker 反复轮询、简单但费资源；reschedule 模式每次检查完释放 worker、睡一会再被调起，长时间等待必须用它，否则 worker 会被占满。

**Q6. Operator、Task、Task Instance 有什么区别？**
Operator 是任务模板/类（BashOperator/PythonOperator/SparkSubmitOperator），Task 是 DAG 里的一个节点（Operator 的实例化），Task Instance 是某个 Task 在某个 execution_date 的一次具体运行（有自己的状态和日志）。类比：类 → 对象 → 一次调用。

**Q7. Airflow 的整体架构？**
Scheduler（大脑，解析 DAG、算依赖、派任务）→ Executor（决定在哪跑的策略：Local/Celery/Kubernetes）→ Worker（真正执行）→ 状态全写进 Metadata DB（PostgreSQL/MySQL，Airflow 的记忆）→ Webserver 从 DB 读出来做可视化。Metadata DB 是单点核心。

**Q8. Airflow 里任务之间怎么传数据？大数据能传吗？**
小元数据用 XCom 传（路径、参数、行数），但 XCom 走 Metadata DB，**不能传大数据**（DataFrame/大 JSON 会撑爆元数据库拖垮 Scheduler）。正确做法：上游把数据落到 S3/HDFS，XCom 只传路径，下游按路径读。**传路径不传数据。**

**Q9. trigger rule 是什么？举个用途。**
控制一个 task 在上游什么状态下触发。默认 `all_success`（上游全成功）。常改成 `all_done`（上游全跑完不管成败）——典型用于"无论前面成没成，最后都要发告警/清理临时表"的收尾任务。

**Q10. MongoDB 和 MySQL 的区别？**
MySQL 是关系型，表+行+强 schema，靠 JOIN 关联多表，强事务；MongoDB 是文档型 NoSQL，集合+文档（JSON/BSON），schema 灵活可随时加字段，靠内嵌数组/对象减少 JOIN。MySQL 适合强事务、多表关联；MongoDB 适合 schema 多变、嵌套结构、快速迭代的业务数据。

**Q11. 什么场景用 MongoDB，什么场景不用？**
用：schema 多变要快速迭代、数据天然嵌套（订单带明细、商品带多规格）、半结构化/字段不固定、高并发点读写。不用：复杂多表 JOIN 分析、强事务多表操作、大规模离线分析报表（那是 OLAP 引擎的活）。

**Q12. MongoDB 在 ETL 数据链路里是什么角色？**
通常作为 **source**：业务系统在线写 Mongo，ETL 定时全量/增量或用 CDC 把数据抽进数据湖。抽取时要把嵌套文档"拍平"成结构化行列，下游数仓才好分析。它不是分析引擎，是数据来源。

---

## 结尾速查表

### Airflow 概念速查

| 概念 | 一句话 |
|------|--------|
| DAG | 有向无环图 = 一个工作流；无环保证可拓扑排序 |
| Operator | 任务模板/类（Bash/Python/SparkSubmit…） |
| Task | DAG 里的一个节点（Operator 实例化） |
| Task Instance | 某 task 在某 execution_date 的一次运行 |
| schedule / schedule_interval | 调度周期，cron 或 @daily/@hourly |
| **execution_date / logical_date** | **数据的逻辑日期，非运行时刻；T+1 是昨天** |
| `{{ ds }}` | execution_date 字符串 `2026-07-20` |
| catchup | 上线自动补 start_date→今天 的空档（默认 True，新 DAG 常关） |
| backfill | 手动命令回补一段历史，逐日按 execution_date 重跑 |
| Sensor | 阻塞等外部条件；长等用 reschedule 模式 |
| trigger rule | 上游何种状态触发下游；all_success(默认)/all_done/one_success |
| 幂等 | 同 execution_date 重跑结果不变；OVERWRITE 分区，取数用 ds 不用 now() |
| XCom | 传小元数据（路径），**不传大数据** |

### Airflow 架构速查

| 组件 | 职责 |
|------|------|
| Scheduler | 解析 DAG、算依赖、派任务（大脑） |
| Executor | 决定在哪跑：Local / Celery / Kubernetes |
| Worker | 真正执行任务 |
| Metadata DB | 存所有状态/调度记录（PG/MySQL，单点核心） |
| Webserver | Web UI，可视化与手动操作 |

### MongoDB 速查

| 维度 | MongoDB |
|------|---------|
| 类型 | NoSQL 文档数据库，存 JSON/BSON |
| 概念 | database / **collection** / **document** / field（对 MySQL：db/table/row/column） |
| 定位 | OLTP 向在线业务库，schema 灵活；**不是分析引擎** |
| 适合 | schema 多变、嵌套结构、快速迭代、高并发点读写 |
| 不适合 | 复杂多表 JOIN、强事务、大规模离线分析 |
| ETL 角色 | 通常作 source，抽入数据湖时把嵌套文档拍平 |
| vs MySQL | 灵活 schema + 内嵌减 JOIN vs 强 schema + JOIN + 强事务 |
