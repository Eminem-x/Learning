# Trino 查询引擎

> 面向对象：维护 ETL 多年、用 Trino 查过 Hive 但只停留在"会用"的工程师，为面试做知识补强。
> 定位：把你脑子里零散的"Trino 查 Hive 表"经验，串成一条能在面试里讲清楚的主线。

---

## 0. 一句话总纲

Trino 是一个**存算分离的 MPP（大规模并行处理）交互式 SQL 查询引擎**，自己不存数据，靠 Connector 连各种数据源，全内存流式执行，主打**秒级 ad-hoc 查询**，不是用来跑重型 ETL 的。

记住这一句，后面所有章节都是它的展开。

---

## 1. Trino 是什么

**是什么**
- MPP 交互式 SQL 查询引擎。前身叫 **PrestoSQL**（Facebook 2012 年开源 Presto，2019 年核心团队出走后把 PrestoSQL 分支改名 Trino，另一支叫 PrestoDB 由 Meta/Linux 基金会维护）。面试里说"Trino 就是原来的 PrestoSQL"即可。
- 定位是**交互式 / ad-hoc 查询**：分析师敲一条 SQL，期望**秒级到分钟级**返回，而不是提交一个跑几小时的批处理作业。

**为什么这样设计**
- 数据湖里数据已经躺在 HDFS / S3 上（Hive 表、Parquet/ORC 文件），需要一个"能直接对着这些文件跑标准 SQL、而且够快"的引擎。老 Hive on MapReduce 太慢（每步落盘），Trino 用全内存 pipeline 把交互式查询的延迟拉到可接受范围。

**例子（贴合本项目）**
- 你在 DBeaver 里连上 Trino，直接
  ```sql
  SELECT country, COUNT(*)
  FROM nebula_entity.some_table
  WHERE date = '20260720'
  GROUP BY country;
  ```
  几秒出结果，用来核对 mlsql 跑出来的数据对不对——这就是典型的 Trino 用法：**验证 / 探查，不生产**。

**边界（面试爱问）**
- Trino **不是数据库**：它没有自己的存储、没有事务（对 Hive 这类源只做读，写也是有限支持）。
- Trino **不适合跑重型 ETL**：没有 checkpoint、失败整条 query 重来、超大 shuffle 会把内存打爆。重型 ETL 交给 Spark。

---

## 2. 核心特点：存算分离 + 联邦查询（最大卖点，重点）

这一节是 Trino 区别于"传统数据库"和"Spark"的根本，面试务必讲透。

### 2.1 存算分离（Storage-Compute Separation）

**是什么**
- Trino 集群里**没有一个字节的业务数据**。计算层（Coordinator + Worker）和存储层（HDFS / S3 / MySQL / ES …）完全解耦。
- 查询时才通过 Connector 去数据源**读**数据，读进内存算，算完把内存释放。

**为什么好**
- 存储和计算可以**各自独立扩缩容**：数据涨了扩存储，查询并发高了加 Worker，互不绑定。
- 同一份数据（比如 S3 上的 Parquet）可以被 Trino、Spark、Hive 同时读，不用搬来搬去、不用为查询单独复制一份。

### 2.2 联邦查询（Federated Query）—— 王牌能力

**是什么**
- Trino 能同时挂载多个数据源（每个源配一个 catalog），**在一条 SQL 里跨源 JOIN**。

**为什么是杀手锏**
- 传统做法：MySQL 的维表要和 Hive 的事实表关联，得先把 MySQL 数据同步进 Hive，再 JOIN。Trino 直接一条 SQL 搞定，省掉同步链路。

**例子**
```sql
-- hive catalog 里的事实表  JOIN  mysql catalog 里的维表
SELECT f.order_id, f.amount, d.seller_name
FROM   hive.nebula_entity.fact_orders  f
JOIN   mysql.crm.dim_seller            d
  ON   f.seller_id = d.id
WHERE  f.date = '20260720';
```
- `hive.` 和 `mysql.` 是两个不同的 catalog，Trino 各自下推过滤、各自读回数据，在计算层做 JOIN。
- 常见挂载源：**Hive、MySQL/PostgreSQL、Elasticsearch/OpenSearch、Kafka、MongoDB、Redis、Iceberg、Delta Lake** 等，只要有对应 Connector。

> 面试一句话：**"Trino 最大的卖点是存算分离带来的联邦查询——不搬数据，一套标准 SQL 查/JOIN 异构数据源。"**

---

## 3. 架构：Coordinator + Worker，全内存 pipeline

**是什么**（两类角色）

| 角色 | 职责 |
|------|------|
| **Coordinator**（协调节点，通常 1 个） | 接收 SQL → 解析（parse）→ 语义分析 → 生成逻辑/物理执行计划 → 切分成 Stage/Task → 调度分发给 Worker → 汇总结果返回客户端。**自己不处理数据。** |
| **Worker**（工作节点，N 个） | 真正干活：从数据源读 split、执行算子（scan/filter/join/agg）、和其他 Worker 交换（exchange）中间数据、把结果给 Coordinator。 |

**执行模型（面试高频）**
- **MPP + 全内存 pipeline**：一条 query 被拆成多个 Stage，Stage 内再拆成并行的 Task/Driver，分散到多个 Worker 同时跑。
- **数据在算子之间是"流式"传递（streaming pipeline）**，中间结果**尽量不落盘**，一批数据（page）算完立刻推给下游算子，而不是像 MapReduce 那样"这一步全部写完盘、下一步再读"。
- 这正是 Trino 快的根源：**没有反复的磁盘 I/O 和 JVM 启动开销**。

**代价**
- 全内存 = **吃内存**，而且中间结果不落盘（早期版本完全不 spill；新版对部分算子支持有限 spill，但默认路径仍以内存为主）。内存不够 → **query 直接失败**，不像 Spark 能优雅 spill 到磁盘继续跑。

**例子（本项目连接方式）**
- 本项目 Trino 入口（查 Hive 全部 `nebula_entity.*` / `temp.*` 表）：
  ```
  JDBC URL: jdbc:trino://10.108.100.42:28080/hive?user=presto_admin
  host:     10.108.100.42
  port:     28080          # 这是 Coordinator 的 HTTP 端口
  user:     presto_admin
  catalog:  hive
  schema:   nebula_entity  (或 temp)
  ```
  你 JDBC 连的 `10.108.100.42:28080` 就是 **Coordinator**；GUI（DBeaver / DataGrip）贴这个 URL 即可，Python `trino` 库 / `trino` CLI 同理。
  ```python
  from trino import dbapi
  cur = dbapi.connect(host="10.108.100.42", port=28080, user="presto_admin",
                      catalog="hive", schema="nebula_entity").cursor()
  cur.execute("SELECT ...")
  ```

---

## 4. Trino vs Spark（面试最高频对比）

**核心一句**：Trino 是"**交互式查询引擎**"，Spark 是"**通用分布式计算/批处理框架**"。它们经常被拿来比，是因为都能跑分布式 SQL，但定位根本不同。

| 维度 | Trino | Spark(SQL) |
|------|-------|-----------|
| 定位 | 交互式 ad-hoc 查询 | 批处理 / 通用计算（ETL、ML、流） |
| 执行模型 | 全内存 pipeline，流式，中间**不落盘** | Stage 切分，shuffle **可落盘** |
| 容错 | **无中间容错**，某 Task 挂→**整条 query 失败重来** | **可 checkpoint / retry stage**，容错强 |
| 延迟 | 秒级~分钟级，**快** | 有作业启动/调度开销，偏高延迟 |
| 内存不足 | query **直接 OOM 失败** | **spill 到磁盘**继续跑（慢但能出结果） |
| 适合 | 交互查询、报表探查、联邦查询、数据核对 | 超大规模 ETL、复杂 pipeline、机器学习 |
| 不适合 | 跑几小时的重型 ETL、超大 shuffle | 要求秒级返回的即席查询 |

**什么场景用哪个（答题要点）**
- **要快、要交互、数据量中等、跨源关联** → **Trino**。例：分析师看板背后的即席查询、跨 Hive+MySQL 的联合查询、核对 ETL 产出。
- **要稳、数据量巨大、逻辑复杂、能容忍分钟级以上延迟、要容错** → **Spark**。例：本项目的 mlsql 批处理作业（几十个节点、30 天 sum、多路 LEFT JOIN），必须 Spark 跑，失败还能 retry stage。
- 本项目的真实分工：**Spark 跑 mlsql 生产表 → Trino 查这些表做核对/探查**。这就是二者协作的标准姿势。

---

## 5. Trino vs Hive(on MapReduce)：为什么 Trino 快

**是什么**
- 老 Hive 默认用 **MapReduce** 执行引擎（现在也可切 Tez/Spark），一条 SQL 被翻译成一串 MR 作业。

**为什么 Trino 快（核心差异）**

| 点 | Hive on MR | Trino |
|----|-----------|-------|
| 中间结果 | 每个 MR 阶段结束都**写 HDFS**，下一阶段再读 | 算子间**内存流式传递**，尽量不落盘 |
| 任务启动 | 每个 MR 任务要**启 JVM**，开销大 | Worker 常驻，**无反复启动** |
| 执行方式 | 一步步串行 MR，磁盘 I/O 密集 | 全内存 pipeline，并行流式 |

- 一句话：**Trino 快在"全内存 pipeline，省掉了 MapReduce 反复落盘和 JVM 启动的开销"。** 代价是牺牲了 MR 那种"每步落盘"带来的天然容错。

**例子**
- 同样一条 `GROUP BY` 聚合，Hive on MR 可能几分钟（起 MR、写中间盘），Trino 几秒返回。这也是当年 Presto 出现的初衷——给交互式分析一个不用等 MR 的选项。

---

## 6. Connector 机制

**一句话**
- Trino 靠 **Connector（连接器）抽象**适配不同数据源，让**同一套标准 SQL 查不同源**。

**是什么**
- 每种数据源对应一个 Connector（Hive Connector、MySQL Connector、Elasticsearch Connector …）。Connector 负责三件事：
  1. **元数据**：告诉 Trino 这个源里有哪些 schema/表/列、数据类型是什么；
  2. **数据位置（split）**：把要读的数据切成一份份 split 分给 Worker 并行读；
  3. **数据读取 + 下推**：实际从源拉数据，并尽量把过滤/裁剪下推给源。
- 一个 Connector 配置一次成为一个 **catalog**（如 `hive`、`mysql`），SQL 里用 `catalog.schema.table` 三段式定位。

**为什么好**
- 上层 SQL 语法统一，换数据源只是换 catalog 前缀，业务方不用学各家私有查询语言（不用为 ES 写 DSL、为 Mongo 写聚合管道）。

**例子**
- 本项目里 `hive` 就是一个 catalog（Hive Connector），`nebula_entity` 是它下面的 schema。`hive.nebula_entity.xxx` 即定位到具体表。

---

## 7. 谓词下推 / 分区裁剪

这是 Trino "读得少所以快" 的关键优化，面试常和"为什么快"连在一起问。

### 7.1 谓词下推（Predicate Pushdown）

**是什么**
- 把 SQL 里的 `WHERE` 过滤条件**尽量推到数据源那一层去执行**，让源只返回符合条件的数据，而不是把全表拉回 Trino 再过滤。

**为什么**
- 减少**网络传输量**和 Trino 内存里要处理的数据量。对 MySQL 这类源，下推后变成源库自己带 `WHERE` 的查询；对 Parquet/ORC，利用文件内 min/max 统计跳过整个 row group。

### 7.2 分区裁剪（Partition Pruning）

**是什么**
- Hive 表按 `date` / `country` 等分区列物理分目录存储。查询带分区过滤时，Trino 只扫描命中的分区目录，**跳过其他分区**。

**例子（本项目）**
```sql
SELECT * FROM hive.nebula_entity.some_table
WHERE date = '20260720' AND country = 'ID';
```
- Trino 只读 `date=20260720/country=ID/` 那个目录下的文件，其他分区完全不碰。**写查询时永远带上分区过滤**，否则全表扫描（full scan）会又慢又可能 OOM——这是 ETL 同学最该养成的习惯。

> 面试一句话："谓词下推让源少返回数据，分区裁剪让 Trino 少扫文件，两者叠加就是 Trino '少读即快' 的核心。"

---

## 8. 使用注意（踩坑经验）

### 8.1 内存不够，query 会失败（不是变慢）
- Trino 全内存执行，**没有 Spark 那种可靠的 spill 兜底**。大 `JOIN` / 大 `GROUP BY` / 大 `ORDER BY` 一旦超出内存限制（`query.max-memory` 等），直接报错 `Query exceeded ... memory limit`，**整条 query 失败**。
- 应对：加分区过滤缩小数据、避免对超大表做无脑全表 `ORDER BY`、大聚合考虑改用 Spark 跑。**"要跑大数据用 Spark，要快查探查用 Trino"** 就是这条的根源。

### 8.2 和 Spark SQL 的方言差异（本项目真实坑）
Trino 用的是 **ANSI SQL** 风格，和 Spark SQL 有细节差异。你在 mlsql（Spark）里写惯的函数，搬到 Trino 要改：

| 场景 | Spark SQL 写法 | Trino 写法 |
|------|---------------|-----------|
| 判断 NaN | `isnan(col)` | **`is_nan(col)`** |
| 近似分位数（多分位） | `APPROX_PERCENTILE(col, ARRAY(0.5, 0.9))` | `APPROX_PERCENTILE(col, ARRAY[0.5, 0.9])` |

- 关键差异：**数组字面量 Spark 用 `ARRAY(...)`（圆括号），Trino 用 `ARRAY[...]`（方括号）**。
- 建议：把在 Spark mlsql 里验证过的诊断 SQL 拿到 Trino 跑时，重点检查 **函数名（下划线风格）** 和 **数组/字面量语法**。别指望原样复制粘贴能跑通。

### 8.3 其他常见注意
- **表名三段式**：`catalog.schema.table`，跨 catalog 才是联邦查询；JDBC URL 里指定了默认 catalog/schema 时可省略前缀。
- **它是查询引擎不是存储**：不要指望在 Trino 里做复杂事务写入；对 Hive 表主要是读、探查、核对。
- **别裸跑全表**：没有分区过滤的 `SELECT *` 大表，等于全量扫描，慢且容易打爆内存。

---

## 9. 面试高频问题 + 答题要点

**Q1. Trino 是什么？和 Presto 什么关系？**
> MPP 交互式 SQL 查询引擎，存算分离，靠 Connector 查多种数据源，主打秒级 ad-hoc 查询。前身是 PrestoSQL，2019 年核心团队把 PrestoSQL 分支改名 Trino（另一支 PrestoDB 归 Meta/Linux 基金会）。

**Q2. Trino 和 Spark 有什么区别？什么时候用哪个？**
> Trino = 交互式查询引擎，全内存流式、快、但吃内存、失败整条 query 重来、无 spill；Spark = 通用批处理框架，shuffle 可落盘、容错强、适合重型 ETL。**要秒级交互 / 跨源查询 / 数据核对用 Trino；要跑超大规模 ETL、复杂 pipeline、能容忍延迟、要容错用 Spark。** 常见协作：Spark 生产表，Trino 查表探查。

**Q3. 什么是联邦查询（Federated Query）？**
> Trino 存算分离、本身不存数据，可同时挂多个数据源（每源一个 catalog），**在一条 SQL 里跨源 JOIN**（如 Hive 事实表 JOIN MySQL 维表），不用先把数据同步到一起。这是 Trino 最大卖点。

**Q4. Trino 为什么快？**
> ①全内存 pipeline，算子间流式传递、中间结果不落盘；②Worker 常驻、无 MapReduce 的 JVM 反复启动开销；③谓词下推 + 分区裁剪，让它"少读数据"。对比 Hive on MR 每步落盘 + 每步起 JVM，差距明显。

**Q5. Trino 的架构？Coordinator 和 Worker 各干什么？**
> Coordinator：解析 SQL、生成执行计划、切 Stage/Task、调度、汇总结果，自己不碰数据。Worker：读 split、执行算子、和其他 Worker 做 exchange 交换中间数据。MPP 并行 + 全内存 pipeline。

**Q6. Trino 适合什么、不适合什么？**
> 适合：交互式 ad-hoc 查询、报表探查、联邦跨源查询、ETL 产出核对。不适合：跑几小时的重型 ETL、超大 shuffle/超大聚合排序（会 OOM）、需要强容错的作业。

**Q7. Trino 内存不够会怎样？和 Spark 有什么不同？**
> Trino 全内存、无可靠 spill，超出内存限制直接报错、**整条 query 失败**；Spark 能 spill 到磁盘继续跑（慢但能出结果）。所以大数据量场景优先 Spark。

**Q8. Connector（连接器）机制是什么？**
> Trino 用 Connector 抽象适配不同数据源，负责提供元数据、切 split、读数据并做下推。配置一次成为一个 catalog，用 `catalog.schema.table` 三段式定位，让同一套标准 SQL 能查 Hive/MySQL/ES/Kafka/Mongo 等不同源。

**Q9. 谓词下推和分区裁剪是什么？**
> 谓词下推：把 `WHERE` 过滤推到数据源执行，让源只返回符合条件的数据，减少网络传输和内存处理量。分区裁剪：Hive 表按 date/country 分区，带分区过滤时只扫命中分区目录、跳过其余。两者是"少读即快"的核心，写查询务必带分区过滤。

**Q10. Trino 比老 Hive(MapReduce) 快在哪？代价是什么？**
> 快在全内存 pipeline（不落盘）+ 常驻进程（不反复起 JVM），省掉 MR 的磁盘 I/O 和启动开销。代价是牺牲了 MR "每步落盘"带来的天然容错——Trino 中间不落盘，失败整条 query 重来。

**Q11.（进阶）Trino 是数据库吗？**
> 不是。它没有自己的存储、不做事务，是一个"计算/查询层"，数据始终在外部源里，查询时才读进来算，算完释放。

---

## 10. 速查表

**核心记忆点**
- 定位：**MPP 交互式 SQL 查询引擎**（前身 PrestoSQL），秒级 ad-hoc，不跑重 ETL。
- 卖点：**存算分离 + 联邦查询**（不搬数据，一套 SQL 跨源 JOIN）。
- 架构：**Coordinator**（解析/规划/调度）+ **Worker**（执行），全内存 pipeline、流式、不落盘。
- 快因：全内存 + 常驻进程 + 谓词下推 + 分区裁剪。
- 软肋：**内存不够 query 直接失败**（无可靠 spill）、失败整条重来、不适合超大 ETL。
- 机制：**Connector** 抽象数据源 → catalog → `catalog.schema.table`。

**本项目连接速查**
```
JDBC:    jdbc:trino://10.108.100.42:28080/hive?user=presto_admin
host:    10.108.100.42      port: 28080(Coordinator)
user:    presto_admin       catalog: hive     schema: nebula_entity / temp
工具:    DBeaver / DataGrip 贴 JDBC；Python trino 库；trino CLI
```

**Spark SQL → Trino 方言差异**
```
isnan(col)                          →  is_nan(col)
APPROX_PERCENTILE(c, ARRAY(0.5))    →  APPROX_PERCENTILE(c, ARRAY[0.5])   -- 圆括号→方括号
查 Hive 分区表务必带 WHERE date=... AND country=...  -- 分区裁剪，避免全表扫描
```

**Trino vs Spark 对比表**

| 维度 | Trino | Spark |
|------|-------|-------|
| 定位 | 交互式查询引擎 | 通用批处理/计算框架 |
| 执行 | 全内存 pipeline，流式不落盘 | Stage 切分，shuffle 可落盘 |
| 延迟 | 秒级~分钟级（快） | 偏高（有启动/调度开销） |
| 容错 | 无中间容错，整条 query 重来 | checkpoint/retry stage，强 |
| 内存不足 | 直接 OOM 失败 | spill 到磁盘继续跑 |
| 适合 | ad-hoc 查询、联邦查询、核对 | 超大 ETL、复杂 pipeline、ML |
| 项目角色 | 查 mlsql 产出的 Hive 表做核对 | 跑 mlsql 生产表 |

**Trino vs Hive(MR) 对比表**

| 维度 | Hive on MR | Trino |
|------|-----------|-------|
| 中间结果 | 每阶段写 HDFS | 内存流式，不落盘 |
| 进程 | 每任务起 JVM | Worker 常驻 |
| 速度 | 慢（分钟级+） | 快（秒级） |
| 容错 | 天然容错（落盘） | 无（失败重来） |
