# 我的 ETL 链路深挖：从 crawl 到 StarRocks/ES 的全链路讲法

> 来源：个人思考 + 工作实践梳理
> 收录时间：2026-07-23 CST
>
> 配套：[架构评审与风险清单](./09-etl-architecture-review-20260723.md)｜[Spark](./01-Spark.md)｜[存储层](./02-存储层-S3-Hive-文件格式.md)｜[OLAP 服务层](./04-OLAP服务层-ES-StarRocks-ClickHouse.md)｜[Airflow](./06-Airflow调度与MongoDB.md)｜[Harness 工程化](../../harness-engineering-for-etl-20260403.md)
>
> **定位**：`00`~`07` 是通用组件原理，这一篇是**我这条真实链路的形态**——面试里「介绍下你负责的项目」「你们数据链路长什么样」的标准答案来源。
> 通用八股不重复写，只写**我这条链路特有的选择、代价、事故与追问**。

---

## 0. 30 秒电梯版（复习先过这段，卡壳再往下挖）

> 一条覆盖 15 个市场的 T+1 离线数仓。**采集侧**独立跑爬虫，清洗后把 JSON 落 S3；**存储**用 S3 + Hive 外表（Metastore 元数据在 PG），Parquet + date 分区；**调度**用 Airflow（CeleryExecutor，1 master + 3 worker），YAML 描述 DAG，单 DAG 300~400 个 task，UTC 0 点起跑；**计算**是 Spark on K8s **client 模式**——driver 跑在 Airflow worker 容器里，executor 是 K8s pod；按 ODS→DWD→DWS→ADS 分层，中间结果全部回落 S3，靠**分区 OVERWRITE 保证幂等**；**出口**双写互补：搜索类进 ES（Spark 直写 + alias），关联查询类进 StarRocks（Stream Load，主键模型，日期分区、主键分桶）；**治理**上每天清 S3 与 StarRocks 冗余（SR 留 2 天），ES 按索引生命周期删除（留 4 天）。
>
> **一句话讲清取舍**：这套架构在「接入成本」和「隔离性」之间选了前者——client 模式让 Spark 日志直接进 Airflow task log、接入几乎零成本，代价是 driver 和调度器共用一个容器的内存池，出过 OOM；而**极短的保留期（2/4 天）**换来了存储成本，代价是**恢复窗口很窄**。

---

## 1. 全链路架构图

```
┌─ 采集域（独立，非 Airflow 管辖）────────────────────────────────────┐
│                                                                    │
│   crawl 爬虫 ──► 采集侧小 ETL（清洗/补字段）──┬──► S3  (原始 JSON) │
│   「能爬多少是多少」，无产出量 SLA           ├──► PostgreSQL (dim)│
│                                              └──► MongoDB  (待确认)│
└────────────────────────────────────────────────────────────────────┘
             │                       │
             │  ⚠️ 无就绪信号        │  JDBC 直读（gmv / sold_count 等核心指标）
             ▼                       ▼
┌─ 计算域（Airflow 调度）─────────────────────────────────────────────┐
│                                                                    │
│  Airflow 集群：1 master + 3 worker，CeleryExecutor                  │
│  DAG 由 YAML 描述（DAG 内 task 依赖），单 DAG 300~400 task          │
│  UTC 00:00 起跑（北京时间 08:00），各市场结束时间不同               │
│                                                                    │
│    ┌──────────────────────────────────────────────────┐            │
│    │  每个 task = spark-submit (client mode)           │            │
│    │  ┌────────────────────────────┐                   │            │
│    │  │ Airflow worker 容器         │                   │            │
│    │  │  ├─ Celery worker 进程      │                   │            │
│    │  │  ├─ Driver #1 (JVM) ────────┼──► executor pod   │            │
│    │  │  ├─ Driver #2 (JVM) ────────┼──► executor pod   │  K8s       │
│    │  │  └─ Driver #N (JVM) ────────┼──► executor pod   │            │
│    │  └────────────────────────────┘  ⚠️ 共享内存池     │            │
│    └──────────────────────────────────────────────────┘            │
│                                                                    │
│   分层：ODS ──► DWD ──► DWS ──► ADS                                 │
│   中间结果全部落回 S3（Parquet / date 分区 / Hive 外表）            │
│   幂等：INSERT OVERWRITE 覆盖当天分区                               │
│   失败：retry × 3 → bot 通知 → 人工处理                             │
└────────────────────────────────────────────────────────────────────┘
             │                              │
   Stream Load│                              │Spark 直写（es-hadoop）
             ▼                              ▼
     ┌───────────────┐              ┌────────────────┐
     │  StarRocks    │              │  Elasticsearch │
     │  主键模型      │              │  托管云服务     │
     │  日期分区      │              │  alias + 索引   │
     │  主键分桶      │              │  别名切换       │
     │  → 关联查询    │              │  → 搜索         │
     │  保留 2 天     │              │  保留 4 天      │
     └───────────────┘              └────────────────┘
             ▲                              ▲
             └──────── 每日清理任务 ─────────┘
      S3：代码管理为主（lifecycle 只兜碎片和特定目录）
```

**两条数据流的分工**：
- **主流（S3 → Spark → S3 → sink）**：批处理主干，量大、可重跑。
- **旁路（PG → JDBC → Spark）**：核心业务指标（gmv、sold_count）从 PG 直取，结果落 S3 汇入主流。

---

## 2. 逐环节拆解：现状 / 为什么 / 怎么讲 / 三层追问

### 环节 1：采集侧 crawl —— 一个「弱契约」的上游

| 维度 | 现状 |
|------|------|
| 产出 | 爬取原始 JSON，采集侧自带小 ETL 做清洗，最终落 **JSON**（非 Parquet）到 S3 |
| 调度 | 采集侧**自己维护的异步调度**，不在 Airflow 里 |
| 契约 | 「能爬多少是多少」，**无产出完整性 SLA，无就绪信号** |
| 去向 | S3 / PostgreSQL(dim) / MongoDB |

**为什么 ODS 之前那层留 JSON 而不是直接 Parquet？**
因为「会有一些 ad hoc 处理」——JSON 是 schema-free 的，字段结构不稳定的爬取数据先原样落盘，能保证**上游 schema 变了不丢数据**；转 Parquet 是在进 ODS/DWD 时做的。这是典型的 **schema-on-read**：把 schema 的解释权推迟到读取时，代价是**读取更慢、无列裁剪**。

> 📌 **面试怎么讲**：「我们采集层刻意保留原始 JSON，不在最上游就定 schema。因为爬取源的字段随时会变，如果最上游就强制 schema，源站改一个字段整条链路当天就断了。落原始 JSON 保证了『先存下来再说』，转 Parquet 放到 ODS 入仓时做——用一次多余的转换成本，换整条链路对上游变更的容忍度。」

**三层追问**：
1. **JSON 落 S3 有什么坏处？** → 无列裁剪（读一列要读整行）、无谓词下推、压缩率差（文本 + 重复 key）、无 schema 演进元数据。所以只在最上游用，进 ODS 立刻转 Parquet。
2. **为什么不用 Avro（同样 schema 灵活但是二进制）？** → Avro 需要 schema registry 配套，且不能人眼直读；JSON 的 ad hoc 价值在于「出问题时能直接 `aws s3 cp` 下来 grep 一下」——运维可读性也是一种成本。
3. **上游没有就绪信号，你们怎么知道今天数据齐了？** → **老实答：目前没有机制**，UTC 0 点到点就跑，读到多少算多少。这是我认为当前链路**最大的正确性风险**，改进方案见评审篇 P0-1（crawl 写完打 `_SUCCESS` + `S3KeySensor` + 行数环比告警）。

> ⚠️ **待确认**：MongoDB 在这条链路里承担什么角色（原始文档留档？供业务侧点查？还是采集任务状态存储）——这块我没有信息，面试前你需要自己确认，因为「你们为什么用 MongoDB」是必然会被追问的。

---

### 环节 2：存储层 —— S3 + Hive 外表，元数据在 PG

| 维度 | 现状 |
|------|------|
| 表形态 | **Hive 外表（EXTERNAL TABLE）**，没有用 Iceberg / Hudi / Delta |
| 元数据 | Hive Metastore，后端库是 **PostgreSQL** |
| 格式 | **Parquet** |
| 分区 | **date 分区** |

**为什么是外表而不是内表？**
外表的 `DROP TABLE` 只删元数据不删数据——在「表定义频繁变更、数据是共享资产」的场景下这是安全兜底。同时**存算分离**：S3 上的同一份数据可以被 Spark、Trino、其他团队同时读，Hive 表只是「贴在数据上的一张 schema 标签」。

**为什么只有 date 一级分区，不加 country 二级分区？**
> 这是个值得想清楚的点。你们是 15 个市场共用表结构、按国家前缀区分**库/表名**（见 `market_mapping.yaml`：`{country}_pre_nebula_entity`），也就是说**国家维度体现在表名层，而不是分区层**。
> - **好处**：单个国家的任务天然只扫自己的表，物理隔离，一个市场的数据错乱不会污染别的市场；权限也好切。
> - **代价**：跨国家聚合要 `UNION ALL` 15 张表；DDL 变更要改 15 遍（所以才有 `check_market_consistency.py` 这个 validator）。

> 📌 **面试怎么讲**：「我们的多租户（15 个市场）是在**表名层**做隔离，不是在分区层。这样每个市场的作业物理隔离、故障不互相污染，代价是 schema 变更要横向覆盖 15 份——我们为此写了一个 validator 脚本做覆盖检查，纳入 CI。」

**三层追问**：
1. **Metastore 为什么要有？没有它 Spark 不能读 S3 吗？** → 能读（`spark.read.parquet(path)`），但你会丢掉三样东西：**分区裁剪的元数据**（不然要 list 整个前缀去发现分区）、**统一 schema**（每个人自己猜列名类型）、**跨引擎共享**（Trino 也能看到同一张表）。Metastore 的本质是「把路径约定升级成可查询的目录服务」。
2. **Metastore 存在 PG 里，PG 挂了会怎样？** → **整个数仓的表全部不可见**，所有 Spark 作业在解析阶段就失败。Metastore 是这条链路上一个**隐藏的单点**。追问会继续问：有没有主从？备份频率？—— 这个你需要确认。⚠️ **待确认：Metastore 用的 PG 和存 dim 的业务 PG 是不是同一个实例？** 如果是，业务侧的大查询会拖慢所有作业的元数据解析，这是评审篇的 P1 项。
3. **为什么不上 Iceberg？** → 诚实答「历史原因，没上」，但要能说清**上了能解决什么**：① Iceberg 用元数据文件指针替代目录 rename，**从根上消除 S3 rename/delete 风暴**（正好是你们被 COS 限流打过的那个问题）；② 支持行级 update/delete，维度合并不用整分区重写；③ 时间旅行 + 快照回滚，直接解决「误删要回补」的痛点；④ schema 演进不用重写数据。代价是引入新组件、Spark 版本要求、需要额外的 compaction/expire 维护作业。

---

### 环节 3：维度层 —— PG 直读 + 快照 union 合并

| 维度 | 现状 |
|------|------|
| 读取方式 | Spark **JDBC 直读 PG**，取 gmv、sold_count 等核心指标，结果落 S3 |
| 维度更新 | 每天拉最新，**与前一天快照表 union 合并更新** |

**「union 合并」本质是什么？**
这是 **SCD Type 1（覆盖式缓慢变化维）** 的一种实现：

```sql
-- 概念上等价于
INSERT OVERWRITE TABLE dim_xxx PARTITION (dt = '${today}')
SELECT * FROM (
  SELECT *, ROW_NUMBER() OVER (PARTITION BY biz_key ORDER BY src_priority) rn
  FROM (
    SELECT *, 1 AS src_priority FROM today_incremental      -- 今天拉到的最新
    UNION ALL
    SELECT *, 2 AS src_priority FROM dim_xxx WHERE dt = '${yesterday}'  -- 昨天的快照
  ) t
) WHERE rn = 1;
```

**关键性质：这是一条「链」。** 今天的快照 = f(今天增量, 昨天快照)，昨天的快照 = f(昨天增量, 前天快照)……

> 📌 **面试怎么讲（这是个能讲出深度的点）**：「我们的维度是**每日全量快照 + 增量合并**，不是拉链表。好处是实现简单、下游 join 直接按 dt 取当天快照，不用处理 `between start_date and end_date` 的区间逻辑。
> 但它有个**必须知道的性质：快照之间是链式依赖的**。如果某一天的合并出错或漏跑，错误不会停在那一天——它会被第二天当作输入继承下去，一路传播。所以回补不能只补出错那天，必须**从出错日开始逐天串行重跑**，成本是 O(n) 而不是 O(1)。这也反过来约束了 S3 的保留期：**快照的保留天数必须大于『发现问题的最长延迟』**，否则想回补时源头已经被清理任务删了。」

**三层追问**：
1. **快照 vs 拉链表怎么选？**

| | 每日全量快照（你们） | 拉链表（SCD2） |
|---|---|---|
| 存储 | O(天数 × 全量)，冗余大 | O(变更量)，省 |
| 查「当前值」 | 简单：取最新 dt 分区 | 需 `where end_date='9999-12-31'` |
| 查「历史某时点」 | 直接取那天分区，**最简单** | 需区间判断 |
| 回补 | **链式，O(n) 串行** | 可单点修复 |
| 实现复杂度 | 低 | 高（要处理开链/闭链） |

选快照的理由通常是「存储便宜过人力」——但你们保留期只有 2~4 天，快照的存储优势并没有兑现，反而承担了链式回补的成本。**这是评审篇 P1-4 的来源**。

2. **JDBC 直读 PG，Spark 是怎么读的？会不会打爆业务库？** → 关键在于 `spark.read.jdbc` **默认是单分区单连接**——一个 executor 用一个 JDBC 连接把结果集全拉过来，既慢又是单点。要并行必须显式给 `partitionColumn` / `lowerBound` / `upperBound` / `numPartitions`，Spark 才会把它拆成 N 个带 `WHERE col BETWEEN ? AND ?` 的并发查询。
   **但并行度一高就是另一个问题**：300~400 个 task 里若有多个都在读 PG，并发连接数会直接压在业务库上。追问会到：有没有读从库？连接数上限？有没有 `fetchsize` 调优（默认 10 行一批，读大表极慢）？
3. **更好的做法是什么？** → 「**一次 dump，多次消费**」：每天由一个专门任务把 PG 快照卸载到 S3（或走 CDC/逻辑复制），下游所有作业读 S3 而不是读 PG。这样对业务库的压力从 O(任务数) 降到 O(1)，且 dump 出来的快照天然可重放——重跑不用再打一次库。

---

### 环节 4：调度层 —— Airflow + CeleryExecutor（这块你说不熟，重点补）

| 维度 | 现状 |
|------|------|
| Executor | **CeleryExecutor** |
| 拓扑 | 1 master + 3 worker，worker 对应多个 pod |
| DAG | **YAML 描述**，单 DAG **300~400 个 task**，DAG 内表达依赖 |
| 时间 | UTC 00:00 起跑（北京时间 08:00），各市场结束时间不同 |
| 失败 | retry × 3 → bot 通知 → 人工处理 |

#### 4.1 CeleryExecutor 到底是什么（补课）

Celery 是 Python 生态的**分布式任务队列**，标准的生产者–消费者模型。Airflow 用它把「决定跑什么」和「实际跑」拆开：

```
┌────────────┐   ①解析 DAG、判断依赖满足    ┌──────────────┐
│ Scheduler  │──────────────────────────────►│ Metadata DB  │
│ (master)   │   ②把 TaskInstance 标记为      │  (PG/MySQL)  │
└─────┬──────┘      queued 并投递             └──────▲───────┘
      │ ③ 投递消息                                   │
      ▼                                              │ ⑥ 回写状态
┌────────────┐                                       │
│  Broker    │  (Redis / RabbitMQ)                    │
│  消息队列   │                                       │
└─────┬──────┘                                       │
      │ ④ worker 主动拉取                            │
      ▼                                              │
┌──────────────────────────────────────┐             │
│ Celery Worker × 3                     │─────────────┘
│  worker_concurrency 个并发槽          │
│  每个槽 fork 一个进程跑一个 task      │  ⑤ 执行
└──────────────────────────────────────┘
```

**必须记住的三件事**：
1. **Worker 是常驻进程，不是每个 task 起一个**。`worker_concurrency`（默认 16）决定单个 worker 同时能跑几个 task，每个占一个「槽」。
2. **Worker 是拉（pull）不是推**。Scheduler 只往 broker 投消息，worker 空闲了自己去拿——所以扩容就是加 worker，天然负载均衡。
3. **Broker 只传「要跑什么」，状态全在 Metadata DB**。Broker 挂了任务丢，但状态不丢；Metadata DB 挂了整个 Airflow 瘫痪。

#### 4.2 CeleryExecutor vs KubernetesExecutor —— 你这套 OOM 的根因就在这

| | CeleryExecutor（你们） | KubernetesExecutor |
|---|---|---|
| Task 载体 | worker 常驻进程里的一个**槽** | **每个 task 一个独立 pod** |
| 启动开销 | 几乎为零 | 有 pod 调度开销（秒级） |
| 资源隔离 | ❌ **同 worker 内所有 task 共享容器内存** | ✅ 每 task 独立 requests/limits |
| 弹性 | worker 数固定，要手动扩 | 随 task 数自动伸缩 |
| 爆炸半径 | **一个 task 撑爆内存 → 整个 worker 容器 OOMKilled → 同容器所有 task 一起死** | 单 pod 死，互不影响 |

> 🔥 **这就是你「一个 worker 跑多个 driver → OOM」的完整解释**：
> client 模式下每个 task 的 Spark driver 是一个**跑在 worker 容器内的 JVM**。worker 容器的内存 limit 是**静态的**，而容器里同时住着 N 个 driver JVM（N = 并发槽数）。任何一个 driver 的堆 + 堆外一涨，整个容器 RSS 触碰 limit，就被内核 **OOMKilled（退出码 137）**——**死的不是那一个 task，是整个 worker 上正在跑的所有 task**。
>
> 你们的解法（降低 worker 并发数 / 加机器内存）本质是**给静态内存池留余量**——治标。治本是把 driver 从 worker 容器里搬出去（cluster 模式）或让每个 task 有自己的 limit（KubernetesExecutor）。

#### 4.3 单 DAG 300~400 task + YAML 生成

**为什么放一个 DAG 里而不是拆多个？**
DAG 内的依赖是 Airflow 原生的、**同步且可靠**的；跨 DAG 依赖要靠 `ExternalTaskSensor`，它是**轮询语义**，两个 DAG 的 schedule 对不齐就会挂死或误判，而且 sensor 本身还占一个并发槽（除非用 `reschedule` 模式或 deferrable operator）。**单 DAG 内表达依赖是正确的选择**。

> ⚠️ **YAML 生成 DAG 的隐藏成本（高频追问）**：Airflow 的 DAG 文件会被 **DagFileProcessor 反复解析**（`min_file_process_interval` 默认 30 秒一轮）。如果 YAML 的读取、解析、展开写在模块**顶层代码**里，那就是**每 30 秒把 15 个市场 × 400 个 task 的 YAML 全解析一遍**——scheduler CPU 直接被拖垮，表现为「task 明明依赖满足了却迟迟不调度」。
> **正确做法**：解析结果缓存、YAML 尽量小、避免在顶层做 IO/网络调用；关注 `dag_file_processor_timeout` 和 Airflow UI 里的 **DAG parse time** 指标。

**三层追问**：
1. **`execution_date` 是什么？** → 见 [06 篇](./06-Airflow调度与MongoDB.md)，这是必考题：**数据的逻辑日期，不是运行时刻**。你们 UTC 0 点跑的这一批，`execution_date`（Airflow 2.2+ 叫 `logical_date` / `data_interval_start`）是**前一天**。所有 `INSERT OVERWRITE PARTITION (dt = ...)` 都必须用它而不是 `current_date()`——用 `current_date()` 的作业**永远无法正确回补历史**。
2. **retry 3 次为什么是 3 次？重试真的安全吗？** → 重试安全的前提是**幂等**。你们的 Spark 侧靠分区 OVERWRITE 幂等（见环节 5），但要注意：**重试对下游 sink 是否也幂等**（见环节 6、7），以及**部分成功**的情况——一个 task 写了 S3、正要写 SR 时挂了，重试会重写 S3（幂等，OK）再重写 SR（要看 SR 侧是否幂等）。
3. **告警只有 bot 通知，够吗？** → 不够。缺的是 **SLA 告警**——「retry 3 次全失败」是**故障告警**，但「任务没失败，只是跑到中午还没出数」不会触发任何告警。Airflow 有 `sla` 参数 / SLA miss callback 可以覆盖这个盲区。评审篇 P2 项。

---

### 环节 5：计算层 —— Spark on K8s client 模式（最容易被挖的一环）

| 维度 | 现状 |
|------|------|
| 部署 | Spark on K8s，**client 模式** |
| Driver | 跑在 **Airflow worker 容器内**（历史包袱，接手即如此） |
| Executor | K8s pod |
| 资源 | executor 资源在 **YAML 中静态配置**（非 dynamicAllocation） |
| 幂等 | **INSERT OVERWRITE 覆盖当天分区** |

#### 5.1 client vs cluster 模式，讲全这张表

```
client 模式（你们）                      cluster 模式
┌─────────────────┐                     ┌─────────────────┐
│ Airflow worker  │                     │ Airflow worker  │
│  ┌───────────┐  │                     │  spark-submit   │ ← 提交完可退出
│  │  Driver   │  │ ← 常驻，作业期间     │       │         │
│  └─────┬─────┘  │    不能死           └───────┼─────────┘
└────────┼────────┘                             │ 创建
         │ 调度 executor                        ▼
         ▼                              ┌───────────────┐
   ┌──────────┐                         │  Driver Pod   │ ← K8s 调度
   │executor  │ pods                    └───────┬───────┘
   └──────────┘                                 ▼
                                          ┌──────────┐
                                          │executor  │ pods
                                          └──────────┘
```

| 维度 | client | cluster |
|---|---|---|
| Driver 位置 | 提交端进程内 | 集群内（K8s 上是 driver pod） |
| **日志** | **直接进 Airflow task log** ✅ | 要 `kubectl logs` / 日志采集 ❌ |
| **接入成本** | 一个 Operator 即可，**近乎为零** ✅ | 需 SparkKubernetesOperator + Sensor 跟踪 pod 状态 |
| 提交端 | **必须存活整个作业周期** ❌ | 提交完即可退出 ✅ |
| 资源隔离 | **与调度器共享容器内存** ❌ | driver pod 有独立 limit ✅ |
| 爆炸半径 | **拖垮整个 worker** ❌ | 只死自己 ✅ |
| 网络要求 | executor 要能回连 driver（提交端需可路由） | 集群内互通即可 |
| 适用 | 交互式（spark-shell / notebook）、调试 | **生产批处理** |

> 📌 **面试怎么讲（重点：讲取舍，不要变成骂前人）**：
> 「我们用的是 client 模式，driver 跑在 Airflow worker 容器里。这个选择我接手时就存在，从**当时的角度看是有合理性的**：client 模式下 Spark driver 的所有输出直接进 Airflow 的 task log，排查问题在 Airflow UI 上一站式完成，接入成本几乎为零——不需要额外的 SparkKubernetesOperator、不需要处理 pod 生命周期和 RBAC、不需要单独搭日志采集。对一个要快速铺 15 个市场的团队，这是很实际的选择。
> 但它的代价我踩过：**driver 和调度器共享一个容器的内存池，且没有 per-task 的 limit**。一个 driver 的堆外内存涨上去，整个 worker 容器被 OOMKilled，死的是这台 worker 上**所有正在跑的 task**——爆炸半径远大于问题本身。我们当时的处理是降低单 worker 并发数、上调机器内存，但那是给静态内存池留余量，**治标**。
> 如果重做，我会把 driver 也放进 K8s（cluster 模式 + SparkKubernetesOperator，或者换 KubernetesExecutor 让每个 task 独立 pod），换来两件事：**每个作业有自己的资源上限**，以及**故障隔离**。代价是日志要单独采集、Operator 层要处理 pod 状态跟踪——这部分工作量是可预期的，比长期背着一个爆炸半径不可控的风险划算。」

**三层追问**：
1. **driver 具体在干什么，凭什么会 OOM？** → driver 不只是「提交个作业」，它负责：**解析 SQL 生成执行计划**、**维护 DAGScheduler/TaskScheduler**、**保存所有 task 的元数据与状态**、**broadcast 变量的构造**、**collect()/take() 的结果汇聚**。OOM 的典型来源：① `collect()` 把大结果拉回 driver；② broadcast 一张不小的表（广播变量先在 driver 组装）；③ **分区数极多**（几万个 task 的元数据本身就占内存）；④ 你们已有笔记里那个案例——**堆外内存**涨导致容器 RSS 超限，加堆内反而更糟（`memoryOverhead` factor 0.3）。
2. **既然 executor 在 K8s，为什么不开 dynamicAllocation？** → K8s 上开动态分配需要 **shuffle 数据的去处**：要么 Spark 3.x 的 `shuffleTracking.enabled`（保留还持有 shuffle 数据的 executor，回收效率受限），要么外部 shuffle service / 远程 shuffle service（K8s 上部署复杂）。你们静态配置的合理性在于：**批处理作业的数据量每天基本恒定**，静态配置可预测、不会在 K8s 上抢占不到资源导致长尾。追问再深会问「那资源利用率怎么办」——答：批作业按国家错峰，且瓶颈在 S3 IO 不在 CPU（见 01 篇）。
3. **`INSERT OVERWRITE` 的幂等有个大坑，你知道吗？** → **必考**。Spark 的 `spark.sql.sources.partitionOverwriteMode` 有两个值：
   - `static`（**默认**）：按 `INSERT OVERWRITE` 语句里**静态指定**的分区做覆盖；如果用的是动态分区写入，**会先清空整张表的目标路径**——这就是「重跑一天，结果把历史全删了」这类事故的来源。
   - `dynamic`：**只覆盖本次实际写出数据的那些分区**，其余分区不动。
   写 T+1 分区任务**必须**确认这个配置是 `dynamic`（或者语句里静态写死 `PARTITION (dt='${logical_date}')`）。这是「幂等」这两个字底下真正的实现细节，面试官一问就见功底。

---

### 环节 6：Sink 到 StarRocks —— Stream Load + 主键模型

| 维度 | 现状 |
|------|------|
| 导入 | **Stream Load** |
| 表模型 | **主键模型（Primary Key）** |
| 分区 | 日期 |
| 分桶 | **主键** |
| 保留 | **2 天**（关联数据） |
| 定位 | 存关联数据（多表 join 场景） |

#### 6.1 Stream Load 的关键机制：Label 去重

Stream Load 是**同步 HTTP PUT** 导入：客户端发起，FE 转发到 BE，BE 直接把数据写成 tablet，**同步返回结果**。它有一个对幂等至关重要的机制——**Label**：

- 每次导入可以带一个用户指定的 `label`；
- StarRocks 保证**同一个 label 只会成功导入一次**，重复提交直接返回 `LABEL_ALREADY_EXISTS`；
- Label 有保留期（`label_keep_max_second`，默认 3 天），过期后 label 可复用。

> 📌 **这是你回答「重跑会不会重复」的核心武器**：
> 「我们 sink 到 StarRocks 用的是 Stream Load + 主键模型，幂等有**两道保险**：
> 第一道是**主键模型本身**——按主键 upsert，同一批数据导两次结果一致，不会像明细模型那样追加出重复行。
> 第二道是 **Stream Load 的 label 机制**——如果导入 label 用确定性的规则生成（比如 `表名_分区日期_批次号`），重复提交会被 StarRocks 直接拒绝，从导入层面就杜绝了重复。
> 但这里有个**主键模型解决不了的问题**：如果上游今天的数据行**变少了**（某些 key 不该存在了），主键 upsert 只会更新存在的行，**旧的多余行不会消失**。要处理这种情况必须显式 `DELETE` 或者按分区 `TRUNCATE` 后重导——单靠 upsert 是『只增不减』的。」

#### 6.2 分桶键 = 主键，是好是坏？

| | 评价 |
|---|---|
| ✅ 数据分布 | 主键唯一性最高，hash 后分布最均匀，**不会数据倾斜** |
| ✅ 主键点查 | 按完整主键查询能**直接裁剪到单个分桶**，极快 |
| ⚠️ 其他查询 | 按**非主键列**过滤时无法分桶裁剪，要扫全部分桶 |
| ⚠️ 主键较长时 | 主键模型的主键索引**常驻内存**（可选持久化索引），主键列多/长 → 内存占用大 |

追问会问「**为什么选主键模型而不是明细/聚合模型**」：
- **明细（Duplicate）**：只追加，写最快，但**没有去重语义**——重跑就是重复数据。
- **聚合（Aggregate）**：按 key 预聚合，适合指标汇总，但原始明细拿不回来。
- **更新（Unique）**：也能 upsert，但是 merge-on-read（查询时才合并版本），**查询变慢**。
- **主键（Primary Key）**：delete-and-insert，**写时就完成合并**，查询不用付 merge 代价 → **适合频繁更新 + 高频查询**的场景。
代价是主键索引占内存。你们是 T+1 覆盖式更新 + 对外服务查询，选主键模型是对的。

**三层追问**：
1. **Stream Load 和 Broker Load 怎么选？** → Stream Load 是**同步、客户端推**，适合单次不太大（官方建议 < 10GB）、需要立刻知道结果的场景，天然适合 Spark 每个分区推一批。Broker Load 是**异步、StarRocks 主动拉**（从 HDFS/S3），适合超大批量，但要轮询任务状态。你们是 Spark 侧主动推 → Stream Load 更顺。
2. **保留 2 天怎么删？** → 必须是 **DROP PARTITION**（元数据操作，秒级）而不是 `DELETE FROM ... WHERE dt < x`（要生成 delete 标记、后续 compaction 才真正回收空间，既慢又放大写）。这就是为什么**按日期分区**是保留策略的前提。
3. **只留 2 天，业务查更早的数据怎么办？** → 说明 SR 在这里的定位是**服务层的热数据缓存**，不是历史存档——历史在 S3。这是合理设计，但要能说清「查 3 天前的数据走什么路径」（Trino 查 S3？还是根本不支持？）。⚠️ 这个你需要确认。

---

### 环节 7：Sink 到 ES —— Spark 直写 + alias

| 维度 | 现状 |
|------|------|
| 写入 | **Spark 直写**（es-hadoop connector） |
| 索引 | **alias + 索引别名** |
| 服务 | **托管云 Elasticsearch**（此前是 OpenSearch） |
| 保留 | **4 天**，直接删除 |
| 定位 | 存搜索数据 |

#### 7.1 alias 模式的价值：原子切换 + 零停机重建

```
写入阶段：              切换阶段（原子）：        清理阶段：
                        POST /_aliases
index_20260723  ←写      { actions: [              index_20260719 → DELETE
index_20260722  ←读        {remove: {index_20260722, alias: idx}},
   ↑ alias:idx            {add:    {index_20260723, alias: idx}}
                        ]}
                        ↑ 这一步是原子的，查询侧无感知
```

> 📌 **面试怎么讲**：「ES 侧我们用 alias 做索引切换。每天写一个新索引，写完校验通过后，用 `_aliases` 的 remove+add **原子操作**把别名指过去——查询侧读的一直是别名，全程零停机、无中间态。这也让**回滚变成一个动作**：把别名指回昨天的索引就行。
> 它同时解决了 ES 一个硬约束：**已有字段的 mapping 不能修改**。要改字段类型只能重建索引，有 alias 就可以后台重建完再切，业务无感。」

#### 7.2 ⚠️ Spark 直写 ES 的幂等陷阱（必须确认）

`es-hadoop` 写 ES 时，行为完全取决于 **`es.mapping.id` 有没有配**：

| 配置 | 写入语义 | 重跑结果 |
|---|---|---|
| **不配 `es.mapping.id`** | ES 自动生成 `_id`（随机） | ❌ **同一条数据插入两份，静默重复** |
| **配了 `es.mapping.id`** | 用业务字段做 `_id`，index 操作即 upsert | ✅ 幂等 |
| **每天写新索引 + alias 切换** | 整索引替换 | ✅ 幂等（重跑就是重写整个新索引） |

你说有 alias + 索引别名，**如果是「每天新索引 + 切别名」的模式，幂等是成立的**。但如果是「固定索引 + 每天追加写」，就必须确认 `es.mapping.id` 配了——没配的话，**retry 3 次会让数据翻倍，而且不报错**。
⚠️ **这个你必须去代码里确认一次**，是评审篇的 P1 项。

**三层追问**：
1. **ILM 还是 ISM？** → **ILM 是 Elasticsearch 的，ISM 是 OpenSearch 的**（AWS fork 后自己实现的 Index State Management）。你的经历刚好横跨两个：之前 OpenSearch 用 ISM，现在托管 ES 用 ILM。**这个细节讲出来很加分**——它说明你不是背概念，是真的迁移过。两者思路一致（按索引年龄/大小做阶段流转），但 API 和策略语法不同，迁移时策略要重写。
2. **ILM 的四个阶段是什么？你们只用了删除？** → hot（写入+查询）→ warm（只读，可 shrink/forcemerge/降副本）→ cold（可冻结、搬到廉价存储）→ delete。你们保留 4 天直接删，**没走 warm/cold**——对 4 天生命周期这是合理的（4 天内数据都是热的，分层没有收益）。能说清「为什么不分层」比硬套四阶段更显功底。
3. **为什么 ES 和 StarRocks 要双写，不能只用一个？** → 因为**两者的索引结构是为不同查询设计的**：ES 是**倒排索引**，为「给定词找文档」优化，全文检索/模糊匹配/相关性打分是它的主场，但多表 join 是弱项（ES 几乎不支持 join）。StarRocks 是**列存 + 向量化 MPP**，为「大规模扫描 + 聚合 + join」优化，但没有倒排、做不了真正的全文检索。你们「ES 存搜索数据、SR 存关联数据」是**按查询模式而非按数据来源分流**，这是正确的服务层设计。代价是同一份上游要写两处，一致性靠上游同源保证（都从 S3 的同一份 DWS/ADS 出）。

---

### 环节 8：生命周期治理 —— 用代码而不是用 lifecycle rule

| 对象 | 策略 |
|------|------|
| S3 | **代码管理为主**，lifecycle rule 只兜「碎片和特定目录」 |
| StarRocks | 保留 **2 天** |
| ES | 保留 **4 天**，直接删除 |

**为什么不全用 S3 Lifecycle Rule？** 你给的两个理由都很硬：
1. **S3 lifecycle 规则有数量上限**（每个 bucket 上限 1000 条规则），而你们是 15 个市场 × 多张表 × 不同保留期，规则数会爆。
2. **不同表生命周期不一样**，用代码表达比维护上千条前缀规则更可控、可 review、可测试。

lifecycle rule 保留给两类：**清理不完整的分片上传（AbortIncompleteMultipartUpload）**——这是必开的，否则失败的 multipart 会永久占空间且在控制台不可见；以及规则简单的固定目录。

> 📌 **面试怎么讲**：「S3 的生命周期我们没有全交给 lifecycle rule，主要在调度里用代码管。原因有两个：一是 lifecycle 规则**每个 bucket 有数量上限**，我们 15 个市场乘上各自不同保留期的表，规则数扛不住；二是保留策略本身是**业务逻辑**——哪张表留几天跟下游依赖有关，写在代码里能 review、能测、能跟表定义放在一起演进，而散落在 bucket 配置里是不可见的。
> 但有一类我们坚持用 lifecycle：**AbortIncompleteMultipartUpload**。Spark 写 S3 失败留下的分片上传碎片在控制台看不见，也不计入对象列表，只有账单会涨——这个只能靠 lifecycle 兜。」

**三层追问**：
1. **清理任务本身出过事吗？** → 你说有过（所以才要 ILM 管），能通过 Airflow 重跑 to_es 任务恢复，但很慢。这是**很好的自曝式回答**——面试官爱听真实教训。要点：① 清理是**不可逆操作**，必须有 dry-run 和白名单；② 恢复能力受限于**上游保留期**——ES 删了能靠 S3 重灌，但如果 S3 那份也过期了就彻底没了。
2. **「恢复窗口」这个概念你能定义吗？** → **可恢复的前提是「源头还在」**。你们的链条是：ES(4天) ← S3(N天) ← crawl(不可重爬或重爬代价高)。所以真正的恢复窗口 = **S3 对应表的保留天数**，而不是 ES 的 4 天。如果 S3 上某表只留 3 天，那 ES 里第 4 天的数据一旦删错就**永久丢失**。⚠️ 这是评审篇 P0-2，你需要实际核对一遍每张表的 S3 保留期 ≥ 下游保留期 + 发现问题所需时间。
3. **有没有想过用 Iceberg 的 snapshot 做时间旅行代替这套清理？** → 可以顺势聊回环节 2 的追问 3，把「误删要重跑很慢」这个痛点和「Iceberg 快照回滚是秒级元数据操作」对上——展示你不只是知道 Iceberg 这个词，而是知道它解决你的哪个具体问题。

---

## 3. 三个能拿出去讲的 STAR 故事

> 面试里「讲一个你解决的难题」，你的素材远超一般候选人。下面按 STAR 搭好骨架，**方括号里的数字你需要自己填**——填不上的就别说具体数，说量级。

### 故事 A：两次跨云厂商迁移（AWS → 腾讯云，火山 → 腾讯云）★ 首推

这是**最能打的故事**，因为它同时展示了：跨系统的全局理解、数据一致性把控、风险控制能力，而且很少有候选人做过。

| | 内容 |
|---|---|
| **S**ituation | 公司出于 [成本 / 合规 / 战略] 考虑，要把整条离线数据链路从 AWS（以及后来的火山）迁到腾讯云。链路覆盖 15 个市场，每天 300~400 个 Spark 任务，涉及对象存储、Hive Metastore、Airflow 集群、StarRocks、ES 全套组件。我从**接手维护做到从 0 搭建**，包含数据迁移。 |
| **T**ask | 目标：**在不中断每日 T+1 产出的前提下**完成迁移，且迁移后数据与原环境**可校验一致**。 |
| **A**ction | ① **组件重建**：在腾讯云侧从 0 搭 Airflow 集群 + Spark on K8s + StarRocks + ES；<br>② **存储层适配**：S3 → COS 不只是换个 endpoint——**对象存储的请求限流阈值、一致性语义、连接器实现（s3a vs cosn）都不同**，我们就在这一步踩了 COS 限流的坑（见故事 B）；<br>③ **Metastore 迁移**：Hive 表的 `LOCATION` 存的是**绝对 URI**，迁移必须批量改写 Metastore 里的 `DBS.DB_LOCATION_URI` 和 `SDS.LOCATION`，否则表迁过去了指向的还是旧云；<br>④ **数据迁移**：[全量搬历史分区 + 增量双跑] 的方式，[用 xx 工具]；<br>⑤ **一致性校验**：按表按分区比对 **行数 + 关键指标聚合值**，差异超阈值不切；<br>⑥ **切流与回滚**：[灰度顺序 / 回滚预案]。 |
| **R**esult | 两次迁移均完成，[停机时间 / 数据差异率 / 成本变化]。更重要的收获是：**这条链路上每一处对云厂商的隐式依赖，我都被迫显式地过了一遍**——包括那些平时看不见的（Metastore 里的绝对路径、对象存储的限流阈值、ES 的 ILM/ISM 差异）。 |

> **面试官必追的三个问题，先准备好**：
> 1. 「怎么保证迁移前后数据一致？」→ 讲**分层校验**：行数（快、粗）→ 关键指标聚合（中）→ 抽样逐行比对（慢、准）。以及「**为什么不能只比行数**」——行数一样字段可能错位（你们 MLSQL 字段顺序那个坑正好是例子）。
> 2. 「迁移过程中每天的数据还要出，怎么办？」→ 讲**双跑期**：两边同时跑，产出比对，比对通过再切消费方。成本翻倍但风险可控。
> 3. 「有没有回滚过？」→ 有就讲，没有就讲**回滚预案是什么、判断标准是什么**。

### 故事 B：Spark 频繁删除 S3 根目录触发 COS 限流 ★ 技术深度最强

| | 内容 |
|---|---|
| **S** | 迁到腾讯云 COS 后，Spark 作业开始大面积失败/极慢，报限流类错误。 |
| **T** | 定位为什么在 AWS 上跑得好好的作业，换到 COS 就被限流。 |
| **A** | 根因链条（**这段是这个故事的价值所在，要能一口气讲完**）：<br>① **对象存储没有目录，也没有原子 rename**。Hadoop 的 `FileOutputCommitter` 却是按「文件系统」设计的——task 先写到 `_temporary/`，commit 时 **rename** 到最终路径。在对象存储上，rename = **copy + delete**，是 O(文件数) 次 API 调用，不是一次元数据操作。<br>② `INSERT OVERWRITE` 分区还要先**删除目标路径下所有对象**——又是一批 DELETE 请求。<br>③ 对象存储的**请求速率限制是按前缀（prefix）算的**（S3 是每前缀 3500 写/5500 读 per second 量级，COS 有类似 QPS 上限）。所有分区挤在同一个前缀下 → **热点前缀**，瞬时请求打满，返回限流。<br>④ 和已有笔记里那个「13 个 print 导致 shuffle 放大 13 倍触发 SlowDown」是**同一个病的两种触发路径**：一个是文件数被放大，一个是删除请求被放大，终点都是**打爆单前缀的请求配额**。<br>处理方向：降低输出文件数（`shuffle.partitions` 调低）、减少不必要的重复删除、[你们实际用的手段]。 |
| **R** | 作业恢复，并且沉淀出一条认知：**「在对象存储上跑 Hadoop 生态，rename 和 delete 是要按 API 调用次数计费和限流的，不是免费的元数据操作」**——这条认知直接指向后续该上 Iceberg 这类用元数据指针替代目录 rename 的表格式。 |

> **这个故事的杀伤力在于**：它能一路讲到「为什么湖仓表格式（Iceberg/Hudi/Delta）会出现」——从踩坑推导出技术演进的动因，这比背 Iceberg 的 feature list 高一个层次。

### 故事 C：ES 榜单场景 shard 过多导致读写变慢 ★ 展示容量规划能力

| | 内容 |
|---|---|
| **S** | 榜单类数据存在 ES，随着索引数增长，集群读写整体变慢。 |
| **T** | 定位并治理。 |
| **A** | 根因：**shard 数量爆炸**。每个 shard 是一个**独立的 Lucene 索引**，有自己的段文件、内存结构（term dictionary/FST 等）。shard 太多会从三个方向同时压垮集群：<br>① **master 侧**：cluster state 膨胀，元数据变更（建索引/分配分片）变慢；<br>② **数据节点侧**：每个 shard 都有固定的 heap 开销，shard 一多 heap 就被元数据吃掉，GC 压力上升；<br>③ **查询侧**：一次查询要 **fan-out 到所有相关 shard 再 merge**，总延迟由**最慢的那个 shard** 决定（tail latency 放大），而且大量小 shard 让每次查询都在付调度开销、却没有并行收益。<br>榜单场景特别容易中招：**按天建索引 × 多市场 × 每个索引默认多个 shard**，几周就能堆出上万 shard。<br>治理方向：按**实际数据量**定 shard 数（业界经验：单 shard 10~50GB；每 GB heap 不超过 ~20 个 shard），小索引直接 1 shard；用 rollover 按大小滚动而非按天硬切；历史索引 shrink/forcemerge；用 alias 屏蔽物理索引变化让业务无感。 |
| **R** | [读写延迟改善数据]。沉淀认知：**ES 的 shard 数不是「越多越并行」，它是有固定成本的资源**，容量规划要按数据量倒推，不能按时间粒度拍脑袋。 |

### 备用素材：VM 读放大与内核参数调优

> ⚠️ **这个故事我不替你写，因为细节我不确定**。你提到「VM 读放大调整 Linux 内核配置」，可能是几个不同的东西，讲之前请先确认是哪个：
> - **预读（readahead）过大**：顺序读优化的预读设置，在**随机读为主**的负载（ES/StarRocks 的查询）上会造成严重读放大——每次读 4KB 实际从磁盘拉 128KB+。调 `blockdev --setra` 或 `/sys/block/<dev>/queue/read_ahead_kb`。
> - **`vm.swappiness`**：非 0 时 JVM 堆被换出到磁盘，表现为 GC 极慢、查询抖动。ES/JVM 服务通常要设 0 或 1（或直接关 swap）。
> - **`vm.max_map_count`**：ES 用 mmap 读段文件，默认 65530 太小，shard/段一多就报错。ES 官方明确要求调到 262144。
> - **`vm.dirty_ratio` / `dirty_background_ratio`**：脏页刷盘策略，影响写入毛刺。
>
> 确认是哪个之后，这个故事的价值很高——**它证明你排障能穿透到操作系统层**，而不是停在应用层调参。按同样的 STAR 骨架补上即可。

---

## 4. 全链路追问速查表（面试前 10 分钟扫这张）

| 环节 | 高频追问 | 一句话答案 |
|---|---|---|
| crawl | 数据齐了才跑吗？ | **没有就绪信号，到点就跑**——已知风险，改进是 `_SUCCESS` + Sensor + 行数环比 |
| crawl | 为什么留 JSON？ | schema-on-read，容忍上游字段变更，代价是无列裁剪 |
| 存储 | 为什么外表？ | 存算分离 + DROP 不删数据 + 跨引擎共享 |
| 存储 | Metastore 挂了呢？ | **全数仓表不可见，隐藏单点**——要确认主从与备份 |
| 存储 | 为什么不上 Iceberg？ | 历史原因；上了能治 rename 风暴、支持行级更新、快照回滚 |
| 维度 | 快照 vs 拉链？ | 快照实现简单，但**链式依赖，回补 O(n) 串行** |
| 维度 | JDBC 读 PG 的问题？ | 默认单分区单连接；并行又压业务库；应改为一次 dump 多次消费 |
| 调度 | Celery 是什么？ | 分布式任务队列，scheduler 投 broker，worker 拉；**worker 常驻，槽位共享内存** |
| 调度 | 为什么 OOM 会连坐？ | CeleryExecutor **无 per-task 内存 limit**，容器 OOMKilled 杀掉全部 task |
| 调度 | 400 task 单 DAG 有问题吗？ | 依赖表达对；风险在 **YAML 顶层解析拖垮 scheduler**（默认 30s 一轮） |
| 调度 | `execution_date` 是什么？ | **数据逻辑日期，不是运行时刻**；不用它写分区就无法回补 |
| 计算 | client vs cluster？ | client 换来零成本日志与接入，代价是**共享内存池 + 爆炸半径** |
| 计算 | driver 为什么 OOM？ | collect / broadcast 组装 / 海量 task 元数据 / **堆外内存**（overhead 0.3） |
| 计算 | 幂等怎么实现？ | 分区 OVERWRITE；**必须确认 `partitionOverwriteMode=dynamic`**，否则清空整表 |
| SR | 重跑会重复吗？ | 主键模型 upsert + **Stream Load label 去重**，双保险；但**删除的行不会消失** |
| SR | 为什么主键模型？ | 写时合并，查询不付 merge 代价；代价是主键索引占内存 |
| SR | 保留 2 天怎么删？ | **DROP PARTITION**（元数据操作），不是 DELETE |
| ES | 重跑会重复吗？ | 看 **`es.mapping.id` 有没有配** / 是否每天新索引切 alias——⚠️ 需确认 |
| ES | alias 干嘛用？ | **原子切换**、零停机重建、回滚一步到位；绕开 mapping 不可改的约束 |
| ES | ILM 还是 ISM？ | **ILM 是 ES 的，ISM 是 OpenSearch 的**——你两个都用过，这是加分细节 |
| ES/SR | 为什么双写？ | 倒排索引 vs 列存 MPP，**按查询模式分流**，不是按数据来源 |
| 治理 | 为什么不用 lifecycle？ | 规则数有上限 + 保留策略是业务逻辑要能 review；但**碎片清理必须用 lifecycle** |
| 治理 | 误删能恢复吗？ | 能重跑，但**恢复窗口 = 上游 S3 保留期**，不是 ES 的 4 天 |

---

## 5. 自测题（盖住下面的答案，先自己讲一遍）

**基础层**
1. 画出这条链路的全景图，说出每一层用了什么、为什么。
2. `execution_date` 和实际运行时刻的关系？为什么必须用它写分区？
3. Hive 外表和内表的区别？Metastore 在这条链路里是什么角色？

**原理层**
4. CeleryExecutor 的四个组件是什么？消息流向如何？worker 是推还是拉？
5. 为什么 client 模式下一个 task OOM 会杀掉整个 worker 上的所有 task？
6. `INSERT OVERWRITE` 的 static / dynamic 两种 partitionOverwriteMode 差别是什么？用错会怎样？
7. Stream Load 的 label 机制如何保证幂等？主键模型的 upsert **解决不了**什么问题？
8. ES shard 过多为什么会同时拖慢读和写？从 master / 数据节点 / 查询三个角度说。
9. 对象存储上 `INSERT OVERWRITE` 为什么会打爆请求配额？rename 在 S3 上是什么操作？

**设计层**
10. 你们的维度用每日快照 union 合并，这和拉链表相比有什么代价？为什么这个代价和「S3 保留期」耦合在一起？
11. 为什么 ES 和 StarRocks 要双写？如果只能留一个，你怎么取舍？
12. 这条链路里哪几个地方是单点？分别挂了会怎样？
13. 如果给你一个季度重构这条链路，你先改哪三件事？为什么是这三件？（答案见[评审篇](./09-etl-architecture-review-20260723.md)）

<details>
<summary>答案要点（点开对照）</summary>

1. 见第 1 节架构图；每层的「为什么」见第 2 节各环节。
2. `execution_date` 是**数据的逻辑日期**，T+1 场景下 = 昨天；实际运行时刻是今天。必须用它是因为**回补（backfill）时运行时刻是今天但要写的是历史分区**，用 `current_date()` 会把历史数据写到今天的分区里。
3. 外表 DROP 只删元数据；Metastore = 贴在 S3 数据上的 schema 目录服务，提供分区裁剪元数据 + 跨引擎共享，是**隐藏单点**。
4. Scheduler / Metadata DB / Broker(Redis,RabbitMQ) / Worker。Scheduler 判断依赖 → 投 broker → **worker 主动拉** → 执行 → 回写 Metadata DB。
5. worker 容器内存 limit 是**静态、共享**的，`worker_concurrency` 个 task（含各自的 driver JVM）共用它；任一 driver 涨内存导致容器 RSS 超 limit → **内核 OOMKill 整个容器（137）**，同容器所有 task 陪葬。CeleryExecutor 没有 per-task limit。
6. `static`（默认）在动态分区写入时会**清空整个目标表路径**；`dynamic` 只覆盖本次写出的分区。用错 = 重跑一天把历史全删。
7. 同一 label 只能成功导入一次，重复提交返回 `LABEL_ALREADY_EXISTS`（label 有保留期）。**解决不了「上游行变少」**——upsert 只增不改删，多余旧行需 DELETE 或 TRUNCATE 分区。
8. master：cluster state 膨胀，元数据操作变慢；数据节点：每 shard 固定 heap 开销，GC 压力上升；查询：fan-out 到所有 shard 再 merge，**延迟由最慢 shard 决定**，小 shard 只付调度成本没有并行收益。
9. 对象存储无原子 rename，`FileOutputCommitter` 的 `_temporary` → 最终路径的 rename 变成 **copy+delete**，O(文件数) 次 API；OVERWRITE 还要先批量 DELETE。请求速率**按前缀限流**，所有分区同前缀 → 热点 → 限流。
10. 快照是**链式依赖**：错一天会沿链传播，回补必须从错误日起**逐天串行**重跑，O(n)。这要求 S3 快照保留天数 > 发现问题的最长延迟，否则源头已被清理任务删掉，无法回补。
11. 倒排索引（找词）vs 列存向量化 MPP（扫描聚合 join），**按查询模式分流**。只留一个 = 放弃其中一类查询能力，或者用性能差几个量级的方式凑合。
12. Hive Metastore 的 PG、Airflow 的 Metadata DB、单 DAG（一个 DAG 挂全市场受影响）、crawl 上游（无 SLA）、client 模式下的 Airflow worker（driver 载体）。
13. 见评审篇 P0 三项。

</details>

---

## 6. 毕业标准

- [ ] 能在白板上**默画出第 1 节的架构图**，并对每一层说出「用了什么 + 为什么这么选 + 代价是什么」，不看笔记讲满 5 分钟。
- [ ] 能完整讲清 **client 模式 OOM 连坐**的因果链（CeleryExecutor 共享内存池 → driver 在容器内 → 静态 limit → OOMKill 整容器），并给出两条治本方案。
- [ ] 能讲清 **`partitionOverwriteMode`** 的坑和 **Stream Load label** 的幂等机制，说出主键模型 upsert 解决不了什么。
- [ ] **三个 STAR 故事**（跨云迁移 / COS 限流 / ES shard 治理）每个能讲 3 分钟，且能接住至少两轮追问。
- [ ] 能回答「你觉得现在这套架构有什么问题」——先讲取舍、再讲风险、最后给路线图（见评审篇）。
- [ ] **补齐三个待确认项**：MongoDB 的角色、Metastore 的 PG 是否与业务 PG 同实例、ES 写入是否配了 `es.mapping.id`。

---

## 附录：自我思考

### 「接手就是这样」不是免死金牌，但也不是原罪

client 模式这件事，我最初的反应是「历史包袱」。但写这篇的时候意识到，**当年选它是有道理的**：零接入成本、日志一站式。真正的问题不是「当初选错了」，而是**选择背后的假设（作业规模小、并发低）已经不成立了**，而架构没有跟着变。

面试里讲这段，讲成「前人挖的坑」是减分的，讲成「**这个选择在当时的约束下是合理的，但随着规模增长它的代价开始超过收益，我识别到了这个拐点并知道怎么改**」才是加分的。这也是四年经验该有的判断力——不是能挑毛病，是能判断**什么时候该还技术债**。

### 这条链路真正的软肋不是性能，是「可恢复性」

梳理下来我最意外的发现：这条链路在性能上的坑（S3 限流、shard 过多）都是**看得见、会报错、能定位**的；但真正危险的是三个**静默失效**的地方——crawl 数据不全不会报错、ES 无 `_id` 重复写不会报错、快照链某天出错会一路传播下去也不会报错。

**报错的问题是最好的问题。** 沉默的错误才要命，因为它把「发现问题的时间」拉长到几天甚至几周，而我们的保留期只有 2~4 天——**等发现的时候，能用来修复的源头数据已经被清理任务删掉了**。

性能问题会让你加班一晚上；可恢复性问题会让你永久丢数据。这个优先级我以前是排反的。

### 「用代码管生命周期」是个被低估的正确决策

lifecycle rule 看起来更「云原生」，但保留策略本质上是**业务决策**——哪张表留几天取决于下游谁在用、出问题多久能发现。把它写在代码里，它就能被 review、被测试、跟表定义一起演进；散在 bucket 配置里，它就是一堆没人敢动的黑盒。

这和 harness 那篇的思路是同一个：**把隐性知识外化到可 review 的地方**。区别只是那篇外化的是「我脑子里的上下文」，这里外化的是「保留多久的业务判断」。

### 下一步

1. 先把三个 ⚠️ 待确认项查清楚——尤其是 `es.mapping.id`，这个如果没配，是**正在发生的数据正确性问题**，不只是面试话术。
2. 核对每张表的 S3 保留期 ≥ 下游保留期 + 发现问题所需时间，把「恢复窗口」这个概念显式化。
3. VM 读放大那个故事补齐细节，它是唯一能证明「排障能下沉到内核层」的素材，值得花时间还原。
