# 存储层：S3 + Hive + 文件格式 + 数据湖

> 读者定位：维护 ETL 多年，对 S3 / Hive 停留在"会用、了解"层面，准备面试想把这块讲透。
> 本文不追求深挖源码，目标是把你已经"知道名字"的东西，补齐"是什么 / 为什么 / 怎么答"。

一句话主线：**现代数据湖 = 一堆文件躺在对象存储(S3)上 + 一层元数据(Hive Metastore)把它们伪装成"表" + 一种聪明的文件格式(Parquet)让扫描更快。** 全文都在拆解这句话。

---

## 0. 全景图：一次查询到底碰了什么

```
     SELECT ... FROM orders WHERE date='2026-07-20' AND country='US'
                              │
                              ▼
   ┌──────────────────────────────────────────────┐
   │  查询引擎 (Spark / Trino / Presto)             │  ← 计算层（本文不展开）
   └──────────────────────────────────────────────┘
        │  ①问："orders 这张表的文件在哪？schema 是啥？"
        ▼
   ┌──────────────────────────────────────────────┐
   │  Hive Metastore（元数据库，通常一个 MySQL）      │  ← 表→路径、schema、分区映射
   │  orders → s3://bucket/warehouse/orders/        │
   │  分区 date=2026-07-20/country=US → 子目录       │
   └──────────────────────────────────────────────┘
        │  ②拿到路径，只去扫需要的那个分区目录
        ▼
   ┌──────────────────────────────────────────────┐
   │  S3（对象存储）                                 │  ← 真正存字节的地方
   │  .../date=2026-07-20/country=US/part-0001.parquet│
   │                                    part-0002.parquet│
   └──────────────────────────────────────────────┘
```

记住这三层的分工，后面每一节都是在讲其中一环。

---

## 1. 对象存储 S3

### 是什么

先把"三种存储"分清楚，这是面试最容易被追问的地基概念：

| 类型 | 代表 | 访问方式 | 类比 | 典型场景 |
|------|------|----------|------|----------|
| **块存储 (Block)** | EBS、云硬盘 | 读写裸块，上面盖文件系统 | 一块生硬盘 | 数据库、虚拟机系统盘 |
| **文件存储 (File)** | NFS、HDFS、本地盘 | 目录树 + POSIX 语义（open/seek/rename） | 一棵文件夹树 | 传统应用共享目录 |
| **对象存储 (Object)** | **S3**、GCS、OSS、TOS | HTTP API：PUT/GET/DELETE 一个 key | 一个巨型 key-value 仓库 | 数据湖、备份、静态资源 |

**对象存储的本质：一张扁平的 key-value 表。**
- `bucket`（桶）= 命名空间/顶层容器
- `key` = 对象的完整名字，是一整根字符串，**没有真正的"目录"**
- `value` = 对象内容（字节）+ 元数据（大小、etag、content-type…）

关键认知：`s3://my-bucket/warehouse/orders/date=2026-07-20/part-001.parquet` 里的斜杠 `/` **只是 key 里的普通字符**，S3 并不存在 `warehouse/` 这个文件夹实体。所谓"目录"是控制台/客户端按前缀（prefix）帮你模拟出来的视觉效果。这一点是理解后面所有 S3 痛点的根。

### 为什么成为数据湖地基

1. **容量近乎无限、按量付费**：不用预先规划磁盘，PB 级随便存，比自建 HDFS 集群便宜太多。
2. **存算分离**：数据躺 S3，计算集群（Spark/Trino）随用随起、用完销毁。集群挂了数据还在；要扩计算不用动存储。这是云原生数仓（Snowflake、Databricks、EMR）的根本架构。
3. **11 个 9 的持久性、跨 AZ 冗余**：运维不用操心副本、坏盘。
4. **开放**：任何引擎都能读同一份 S3 文件，不被某个数据库锁死。

### 最终一致性（Eventual Consistency）

历史上 S3 是"最终一致"：刚 PUT 一个对象，紧接着 LIST 可能看不到它，需要一小段时间才"传播"到位。这在大数据里坑很深——一个 Spark 任务写完文件，下游立刻 list 却读不全。
> ⚠️ 面试可加分的时间点：**AWS S3 从 2020 年 12 月起已提供强读写一致性（strong read-after-write consistency）**，单对象的 PUT-then-GET/LIST 现在是强一致。但要注意：
> - 这只解决"单个对象可见性"，**不解决"目录级操作不是原子"**（见下）。
> - 很多兼容 S3 协议的自建/第三方对象存储（MinIO、Ceph、部分厂商 OSS/TOS）一致性保证不一定和 AWS 对齐，跨云别默认。

### S3 上做大数据的三大痛点（高频考点）

因为 S3 是"扁平 key-value + HTTP"，而不是文件系统，很多在本地盘/HDFS 上天经地义的操作在 S3 上都很别扭：

1. **LIST 慢且贵**
   "列出某目录下所有文件"在 S3 上其实是 **按前缀分页扫描整个 key 空间**，一次最多返回 1000 个 key，目录里文件多就得翻很多页。分区多 + 小文件多时，光是"发现要读哪些文件"就能耗掉大量时间和请求数。→ 直接引出小文件问题（第 8 节）。

2. **rename 不是原子操作，而且很贵**
   文件系统里 `mv a b` 是改个指针，瞬间完成且原子。S3 **没有 rename**，所谓改名 = `COPY`（服务端复制整份字节）+ `DELETE` 原对象，两步、非原子、按数据量收费。
   为什么这要命：Spark/Hive 写数据的经典套路是"先写到临时目录，成功后 rename 到最终目录"来保证原子提交。在 S3 上这个 rename 退化成海量 copy+delete，慢且中途失败会留下半成品。→ 这正是 Iceberg/Hudi/Delta（第 7 节）要解决的问题之一。

3. **请求限流 SlowDown（429/503）**
   S3 对**同一前缀**的请求速率有上限，超了会返回 `503 SlowDown` 让你退避重试。大 job 的 shuffle 会瞬间产生成千上万个小文件的读写/删除请求，很容易撞限流，然后 retry 累积、雪崩。

   > **本项目真实的坑**（`docs/product-channel-attribution-30d-runbook.md`）：跑一张 28M 行的大表 + 3 个 LEFT JOIN 时，不加防护 conf 会触发 `MultiObjectDeleteSupport: SlowDown` 累积 → stage retry 累积 → driver shutdown hook 抛 `S3ShuffleManager.NoSuchMethodError` → **job abort 不写表**。
   >
   > 项目里每条 spark-submit 都必须叠加这几个 conf 来缓解：
   > ```bash
   > --conf spark.sql.shuffle.partitions=100              # 降 shuffle 文件数，减少 S3 请求量
   > --conf spark.hadoop.fs.s3a.multiobjectdelete.enable=false  # 禁用 S3 批量删，避开 SlowDown
   > --conf spark.sql.autoBroadcastJoinThreshold=104857600      # 小表 broadcast，省一次 shuffle
   > --conf spark.shuffle.io.retryWait=10s                # retry 间隔加长
   > ```
   > 记这个例子面试很加分：它把"S3 限流"从抽象名词落到了一个具体的生产事故 + 缓解手段。

**一句话总结痛点根因**：S3 假装成文件系统，但它骨子里是 key-value + HTTP，凡是文件系统里"改目录、原子重命名、快速列目录"的操作，到 S3 上都变成慢/贵/非原子的组合动作。

---

## 2. Hive 到底是什么（澄清高频误区）

### 误区

很多人一听"Hive"就想到"用 SQL 写 MapReduce 的那个慢东西"。**这是十年前的 Hive。** 现在的人说"我们数据存在 Hive 表里""查一下 Hive"，**几乎都不是在说 Hive 的执行引擎**，而是在说两件事：

> **现代语境的"Hive" ≈ Hive Metastore（元数据服务） + Hive 表格式（目录约定）**
> 至于查询用什么引擎跑——Spark、Trino、Presto、Flink 都行，Hive 自己的 MapReduce 引擎早就没人用了。

### Hive Metastore 存了什么

Metastore 通常就是一个 **MySQL/PostgreSQL 数据库**（存元数据，不存业务数据），记录：

- **表 → 存储路径**：`orders` 这张表的数据在 `s3://bucket/warehouse/orders/`
- **schema**：有哪些列、每列什么类型（`user_id string, amount double, …`）
- **分区映射**：`date=2026-07-20/country=US` 这个分区对应哪个 S3 子路径
- **文件格式 / SERDE**：这些文件是 Parquet 还是 ORC 还是 CSV，怎么反序列化
- 表属性、统计信息（行数、大小，供优化器用）等

### Hive 表的本质

> **Hive 表 = 给 S3（或 HDFS）上一堆文件，套一层"这是一张有 schema 的表"的抽象。**

文件还是那些文件，Hive 只是在 Metastore 里登记了"这堆文件叫 orders 表，长这样，按 date/country 分区"。所以：

- 你可以直接往 S3 对应目录 `put` 一个格式正确的 parquet 文件，再 `MSCK REPAIR TABLE` / `ADD PARTITION`，Hive 就"认"这份新数据了——数据和元数据是解耦的。
- 反过来，绕过 Hive 直接删/改 S3 文件，Metastore 不知道，就会元数据和实际数据打架（这也是第 7 节表格式要治的病）。

**本项目里真实的 Hive DDL**（`CLAUDE.md` 规范 + `scripts/ddl/`）就是这个抽象的落地——它只是在声明"表名、有哪些列、按什么分区、文件用 Parquet 的哪套 SERDE 解析"：

```sql
CREATE TABLE `nebula_entity.table_name` (
    `column_name` string,
    `another_col` int,
    `arr_col`     array<string>)
PARTITIONED BY (
    `date` string,
    `country` string)
ROW FORMAT SERDE
  'org.apache.hadoop.hive.ql.io.parquet.serde.ParquetHiveSerDe'
STORED AS INPUTFORMAT
  'org.apache.hadoop.hive.ql.io.parquet.MapredParquetInputFormat'
OUTPUTFORMAT
  'org.apache.hadoop.hive.ql.io.parquet.MapredParquetOutputFormat';
```
> 注意 `PARTITIONED BY (date, country)` 和那串 `ParquetHiveSerDe`：前者定义了 S3 上的目录结构（第 3 节），后者告诉引擎"这些文件按 Parquet 解析"（第 4 节）。一张 DDL 就把本文两个核心概念全串起来了。

### Hive 是数据库吗？（经典问题）

**不是。** Hive 本身不存业务数据、不做事务型增删改查。它是：
- 一层**元数据 + SQL 抽象**，让你能用 SQL 去查躺在 S3/HDFS 上的文件。
- 唯一"像数据库"的部分是 Metastore 那个 MySQL，但它存的是"表的说明书"，不是你的订单数据。

---

## 3. 分区（Partition）与分区裁剪

### 是什么

分区 = **按某个列的值，把表的数据物理拆到不同子目录**。最常见按 `date`（再叠 `country`）：

```
s3://bucket/warehouse/orders/
  ├── date=2026-07-19/country=US/part-000.parquet
  ├── date=2026-07-19/country=GB/part-000.parquet
  ├── date=2026-07-20/country=US/part-000.parquet   ← 目录名 = 分区键值
  └── date=2026-07-20/country=GB/part-000.parquet
```

注意：分区键 `date` / `country` 的值**写在目录名里**，通常不重复存进文件本身（省空间）。

### 为什么能加速（分区裁剪 Partition Pruning）

查询 `WHERE date='2026-07-20' AND country='US'` 时，引擎先问 Metastore：这个条件命中哪些分区目录？→ 只有 `date=2026-07-20/country=US/` 一个。于是 **只 LIST + 读这一个目录的文件，其它分区连碰都不碰**。

一张存了 3 年数据的表，查一天 = 只扫 1/1000 的数据。这就是分区裁剪，是数据湖查询提速最基础、最有效的手段。

> 呼应第 1 节：分区裁剪之所以关键，正因为 S3 上"少 LIST 一个目录"既省时间又省请求数（防 SlowDown）。

### 工程规范（本项目 CLAUDE.md 明确要求）

- **分区键必须放在 SELECT 字段列表的最后**，且 `save ... partitionBy` 的顺序要和 Hive DDL 一致。
- Spark 的 `insertInto` **按列位置映射而非列名**，分区列必须在 DataFrame 最末尾——顺序错了数据会静默错位。

### 分区设计的坑

1. **分区过多（over-partitioning）**：按 `user_id`、按小时+国家+品类多级分区 → 分区数爆炸到几十万。Metastore 要管理海量分区元数据、查询规划变慢、每个分区目录里文件又小又碎。
2. **小文件**：分区切得太细，每个分区就那么点数据，必然产生大量小文件（第 8 节详述）。
3. **数据倾斜**：某个分区（比如某爆款日/大国）数据量远超其它，处理时长木桶效应。

**经验法则**：分区列选**低基数 + 高频过滤**的列（date、country 是黄金组合）；别拿高基数列（user_id、订单号）当分区键。

---

## 4. 文件格式：行存 vs 列存

### 是什么

同样一张表的数据，落到磁盘上可以有完全不同的物理排布：

**行存（Row-based）**：一行的所有字段挨着存。
```
CSV / JSON / Avro
[u1,100,US][u2,200,GB][u3,150,US]...   ← 一行一行连续
```

**列存（Columnar）**：同一列的所有值挨着存。
```
Parquet / ORC
user: [u1,u2,u3...]  amount: [100,200,150...]  country: [US,GB,US...]
```

| 格式 | 存储 | 特点 | 适合 |
|------|------|------|------|
| CSV | 行存 | 无 schema、无类型、无压缩，人可读 | 临时交换、小数据 |
| JSON | 行存 | 半结构化、自带字段名（冗余大） | 日志、嵌套数据、接口交换 |
| **Avro** | **行存** | 自带 schema、二进制紧凑、支持 schema 演进 | **流式写入 / Kafka / 整行读写** |
| **Parquet** | **列存** | 列裁剪、谓词下推、高压缩、schema 内嵌 | **分析型查询（OLAP）标配** |
| ORC | 列存 | 同 Parquet，Hive 生态出身 | Hive 老栈常见 |

### 为什么 Parquet 是分析场景标配

分析查询的典型特征是"**宽表几百列，但每次只 SELECT 几列，扫很多行做聚合**"。Parquet 为此量身定制了四大杀器：

1. **列裁剪（Column Pruning）**：`SELECT amount FROM orders` 只读 `amount` 那一列的存储块，其它列的字节根本不从磁盘/S3 读取。300 列的表只查 3 列 = 少读 99% 的数据。行存做不到（得整行读进来再丢字段）。
2. **谓词下推（Predicate Pushdown）**：Parquet 把数据分成一个个 **row group / page**，每块头部记录了这块内 min/max 等统计。`WHERE amount > 1000` 时，某块 max=500 就整块直接跳过，不解压不读。
3. **高压缩比**：见第 5 节——同列同类型数据挨着放，压缩率碾压行存。
4. **schema 内嵌**：文件自己带列名和类型，自描述、支持 schema 演进（加列不用重写旧文件）。

### Avro 补一句

Avro 是**行存**里的"正经二进制格式"：自带 schema、紧凑、schema 演进友好。它的定位和 Parquet 互补——**写入侧 / 流式**用 Avro（一次写一整行、要频繁追加，行存友好），**分析侧 / 落盘长期查询**用 Parquet（列存扫得快）。很多管道是 "Kafka(Avro) → 落 S3 时转成 Parquet"。

---

## 5. 列存为什么快（把第 4 节的"为什么"讲透）

三个独立的加速来源，面试能分点说清就很扎实：

1. **只读需要的列（I/O 直接减少）**
   列存把每列单独成块。查 3 列就只从 S3 拉这 3 列的字节。**数据湖的瓶颈大多是 I/O（尤其从 S3 读），少读字节 = 直接少花钱少花时间。** 这是最大头的收益。

2. **同列同类型 → 压缩率极高**
   `country` 列全是 `US/GB/…` 少数几个值，`date` 列全是相邻日期。相同/相邻类型的值挨在一起，**字典编码、RLE（游程编码）、delta 编码、位打包**都能发挥到极致，再叠加 Snappy/ZSTD 通用压缩。行存里一行 `[u1,100,US,2026-07-20]` 四种类型混着，压缩器无从下手。压得越小 → 从 S3 读得越少 → 又快又省钱（双重收益）。

3. **向量化执行（Vectorization）**
   一列同类型的值连续存放，CPU 可以一次性把一批值加载进来，用 **SIMD / 批处理**成列地算（一次算 1024 个 amount 的求和），而不是一行一行地解释执行。缓存命中率、CPU 流水线利用率都高得多。

> 一句话记忆：**列存快 = 少读(列裁剪) + 读得少(高压缩) + 算得快(向量化)。**

---

## 6. Schema-on-read vs Schema-on-write

这对概念是"数据湖 vs 传统数仓"哲学差异的核心。

| | **Schema-on-write（写时模式）** | **Schema-on-read（读时模式）** |
|---|---|---|
| 代表 | 传统数据仓库（Oracle/MySQL/Teradata） | 数据湖（S3 + Hive/Spark） |
| 何时校验 schema | **写入前**：不符合表结构直接拒绝 | **读取时**：先原样存文件，查询时才套 schema 解析 |
| 数据形态 | 入库即规整、强类型 | 先落原始文件（甚至 JSON/日志），后定义结构 |
| 优点 | 数据质量有保证、查询快 | 灵活、先存后想、同一份数据可用不同 schema 解读 |
| 缺点 | 僵硬，加字段/改结构成本高，非结构化难塞 | 脏数据混进来、读时才暴露问题、需要治理 |

- **Schema-on-write**：像进仓库前先质检打包，不合规不让进。查得快但不灵活。
- **Schema-on-read**：像先把货全堆进仓库，要用时再按需分拣。灵活但可能堆进垃圾。

**为什么数据湖选 schema-on-read**：数据源杂（日志、埋点、第三方 API、半结构化 JSON），先低成本原样存下来（S3 便宜），schema 后面按分析需求再定义。呼应第 2 节——Hive 表就是"给已经躺在 S3 的文件套一层 schema"，这层 schema 是**读的时候**才生效的，改 schema 只改 Metastore，不重写数据。

---

## 7. 数据湖 / 数据仓库 / 湖仓一体

### 三个概念区分

- **数据仓库（Data Warehouse）**：结构化、schema-on-write、强治理、为 BI/报表服务。规整但贵、灵活性差、装不下非结构化数据。
- **数据湖（Data Lake）**：S3 上原始文件的大集合，schema-on-read，什么都能存、便宜、灵活。但**容易变"数据沼泽"**——没事务、没质量保证、文件被乱改乱删、元数据和实际数据打架。
- **湖仓一体（Lakehouse）**：想同时要湖的便宜灵活 + 仓的可靠治理。做法是在数据湖的文件之上，**加一层"表格式（Table Format）"**，把数据库级别的能力带回到 S3 文件上。

### 表格式：Iceberg / Hudi / Delta Lake（点到为止）

它们**不是新的文件格式**（底下还是 Parquet/ORC），而是**在文件之上再加一层元数据/清单（manifest）**，用来记录"这张表当前由哪些文件精确构成、每次改动生成一个新版本"。解决的核心痛点：

1. **ACID 事务**：并发读写不打架，写到一半失败不会让下游读到半成品。—— 直接治了第 1 节"S3 rename 非原子、提交不安全"的病。
2. **时间旅行（Time Travel）**：每次写生成一个快照版本，可以查"昨天 3 点这张表长什么样"、可回滚。
3. **不用直接操作文件**：增删改（甚至行级 UPDATE/DELETE/MERGE）通过表格式的元数据层完成，不再靠"手动删 S3 文件 + MSCK 修分区"这种脆弱操作，元数据和数据永远一致。
4. **schema/分区演进**：改 schema、甚至改分区策略，不用重写全部历史数据。
5. **顺带缓解小文件**：内置 compaction、避免每次都全目录 LIST（靠 manifest 直接知道有哪些文件，绕开 S3 慢 LIST）。

> 面试话术：**"传统 Hive 表 = 目录约定 + Metastore，缺事务、改数据靠直接动文件、大量 LIST 很慢；Iceberg/Hudi/Delta 这类表格式在文件之上加了一层带版本的清单元数据，把 ACID、时间旅行、schema 演进带到了数据湖上，这就是湖仓一体（Lakehouse）的技术底座。"** 知道这段就够了，不用深挖三者差异。

---

## 8. 小文件问题（大数据经典痛点）

### 是什么 / 为什么是痛点

"小文件问题" = 一张表由**海量的小文件**（几 KB ~ 几十 MB，远小于理想的 128MB~1GB）组成。危害：

1. **元数据压力**：文件越多，Metastore / NameNode 要管的条目越多；查询规划要枚举的文件越多。
2. **S3 请求爆炸（呼应第 1 节）**：读一张表要先 LIST 出所有文件、再对每个文件发 GET。10 万个小文件 = 10 万次请求起步，又慢又贵又容易撞 **SlowDown 限流**。
3. **调度开销**：Spark 常常一个文件起一个 task，10 万小文件 = 10 万个 task，任务调度/启动的开销可能比真正算数据还久。
4. **压缩/向量化打折**：文件太小，Parquet 的 row group 装不满，列存压缩和向量化的优势发挥不出来。

### 怎么产生的

- **过度分区**：分区切太细（第 3 节），每个分区就一点点数据 → 天然一堆小文件。
- **流式/微批写入**：每几分钟/每条消息落一个文件（Kafka→S3、CDC、实时管道），一天下来几万个小文件。
- **高并发写 + shuffle**：Spark `shuffle.partitions` 太大（默认 200），每个 reduce task 各写一个文件，分区一多，文件数 = 分区数 × task 数，指数级膨胀。
  > 本项目就靠 `--conf spark.sql.shuffle.partitions=100`（默认 200）压低 shuffle 产生的文件数，正是从源头控制小文件+S3 请求数。
- **频繁小批 INSERT / UPDATE**：每次操作都追加新文件。

### 怎么治理

1. **合并 / Compaction**：定期把小文件合并成大文件（`INSERT OVERWRITE` 重写分区、Spark `repartition`/`coalesce` 后重写、或表格式内置的 compaction 作业）。这是最主线的手段。
2. **写入侧控制**：写之前 `repartition(N)` / `coalesce(N)` 控制输出文件数；调低 `shuffle.partitions`；用 Spark 3+ 的 **AQE（Adaptive Query Execution）自动合并小分区**。
3. **合理分区粒度**：别过度分区；低基数列做分区键；数据量小的表干脆不分区。
4. **用带 compaction 能力的表格式**：Iceberg/Hudi/Delta 提供自动/手动 compaction 和小文件合并（第 7 节）。

> 一句话：**小文件问题的根，是"文件多"在 S3 上会被"LIST 慢 + 每文件一次请求 + 每文件一个 task"三重放大；治理核心就是想办法让文件变少变大（合并）+ 从源头少产生（控分区、控 shuffle）。**

---

## 9. 面试高频问题 + 答题要点

**Q1. S3 拿来做数据湖有什么坑？**
> 根因：S3 是扁平 key-value + HTTP，不是文件系统。① LIST 慢且按前缀分页，文件一多发现开销就大；② 没有原子 rename，改名 = COPY+DELETE，慢/贵/非原子，导致 Spark/Hive 的"临时目录 rename 提交"很脆；③ 同前缀请求限流 SlowDown(503)，大 job shuffle 海量请求容易撞限流后 retry 雪崩。历史上还有最终一致性，AWS 2020 年底已改强一致，但"目录操作非原子"依旧。这些正是 Iceberg/Hudi/Delta 要解决的。

**Q2. Hive 是数据库吗？现在说的 Hive 到底指什么？**
> 不是数据库。它不存业务数据、不做事务 CRUD。现代语境的"Hive"≈ **Hive Metastore（一个存元数据的 MySQL：表→S3 路径、schema、分区映射）+ Hive 表格式（目录约定）**，实际查询用 Spark/Trino 跑，Hive 自己的 MapReduce 引擎早废弃。Hive 表 = 给 S3 上一堆文件套一层"有 schema 的表"的抽象。

**Q3. 分区裁剪的原理？为什么能加速？**
> 表按 date/country 等列拆成不同子目录，分区键值写在目录名里。查询带 `WHERE date=... AND country=...` 时，引擎先问 Metastore 命中哪些分区目录，只 LIST+读那几个目录，其余分区完全不碰。3 年的表查一天 ≈ 只扫 1/1000 数据；在 S3 上还额外省了 LIST 请求（防限流）。关键：分区键选低基数+高频过滤列，别用高基数列否则分区爆炸。

**Q4. 为什么用 Parquet 而不是 CSV/JSON？**
> Parquet 是列存，专为分析查询设计：① 列裁剪——只读 SELECT 的那几列，300 列查 3 列少读 99%；② 谓词下推——row group 头存 min/max，不满足条件整块跳过；③ 高压缩——同列同类型挨着存，字典/RLE/delta 编码压缩率碾压行存；④ schema 内嵌，自描述、支持演进。CSV/JSON 是行存、无类型、无压缩、要整行读，分析场景又慢又费。

**Q5. 列存为什么比行存快？**
> 三个来源：① 少读列（列裁剪，直接减 I/O，这是最大头）；② 同列同类型压缩率极高，从 S3 读更少字节（又快又省钱）；③ 向量化——同类型值连续存放，CPU 用 SIMD 批量成列计算，缓存和流水线利用率高。总结：少读 + 读得少 + 算得快。

**Q6. schema-on-read 和 schema-on-write 的区别？**
> schema-on-write（传统数仓）：写入前校验 schema，不合规拒绝，数据规整查得快但僵硬。schema-on-read（数据湖）：先原样落文件，读取时才套 schema 解析，灵活、先存后定义，代价是脏数据可能混入、读时才暴露。数据湖选后者是因为数据源杂、先低成本存 S3、schema 按需再定义；改 schema 只改 Metastore 不重写数据。

**Q7. 小文件问题为什么严重？怎么解决？**
> 严重是因为在 S3 上被三重放大：LIST 慢、每文件一次 GET 请求（易撞 SlowDown）、Spark 每文件一个 task（调度开销可能超过算数据）；还让 Parquet 的 row group 装不满、压缩/向量化打折。产生：过度分区、流式微批写、shuffle.partitions 太大、频繁小 INSERT。治理：① compaction 合并成大文件（主线）；② 写入侧 repartition/coalesce、调低 shuffle.partitions、开 AQE 自动合并；③ 合理分区粒度别过细；④ 用带 compaction 的表格式。

**Q8. Iceberg/Hudi/Delta 这类"表格式"解决了什么？和文件格式什么关系？**
> 它们**不是文件格式**（底层还是 Parquet/ORC），而是在文件之上加一层带版本的清单元数据，把数据库能力带回数据湖：ACID 事务（治 S3 提交非原子）、时间旅行与回滚、行级 UPDATE/DELETE/MERGE、schema/分区演进、内置 compaction、靠 manifest 绕开 S3 慢 LIST。这就是"湖仓一体（Lakehouse）"的技术底座——湖的便宜灵活 + 仓的可靠治理。

**Q9.（进阶）存算分离是什么？为什么云上数仓都这么搞？**
> 数据常驻 S3，计算集群（Spark/Trino/EMR）随用随起、用完销毁。好处：存储和计算各自独立扩缩容、集群挂了数据还在、同一份 S3 数据多引擎共享、按量付费成本低。代价是计算要跨网络从 S3 读数据（I/O 是瓶颈），所以列存+分区裁剪+压缩这些"少读数据"的手段格外关键。

**Q10.（进阶）为什么 S3 的 rename 慢会影响 Spark 写数据？**
> Spark/Hive 默认用 FileOutputCommitter：先写到临时目录，job 成功后 rename 到最终目录来实现原子提交。文件系统里 rename 是改指针、瞬间且原子；S3 没有 rename，退化成逐个文件 COPY 整份字节 + DELETE，大量文件时又慢又非原子，中途失败留半成品。缓解：用 S3A 优化的 committer（如 magic/directory committer）或直接上 Iceberg/Delta 这类用元数据提交、不依赖 rename 的表格式。

---

## 10. 速查表

### 三种存储对比
| | 块存储 | 文件存储 | 对象存储(S3) |
|---|---|---|---|
| 单位 | 块 | 文件/目录 | 对象(key-value) |
| 接口 | 裸块 | POSIX | HTTP API |
| 目录 | — | 真目录树 | 假目录(前缀模拟) |
| 原子 rename | — | 有 | **无** |
| 场景 | DB/系统盘 | 共享目录/HDFS | **数据湖** |

### 文件格式对比
| 格式 | 行/列 | schema | 压缩 | 定位 |
|---|---|---|---|---|
| CSV | 行 | 无 | 无 | 临时交换 |
| JSON | 行 | 半结构 | 无 | 日志/嵌套 |
| Avro | **行** | 内嵌 | 有 | **流式写入/Kafka** |
| Parquet | **列** | 内嵌 | 高 | **分析查询标配** |
| ORC | 列 | 内嵌 | 高 | Hive 老栈 |

### 核心概念一句话
| 概念 | 一句话 |
|---|---|
| S3 | 扁平 key-value 对象存储，数据湖的地基，但 LIST慢/rename非原子/会限流 |
| Hive | Metastore(元数据) + 表格式，给 S3 文件套"表"的抽象，不是数据库 |
| Metastore | 存"表→S3路径、schema、分区映射"的 MySQL |
| 分区裁剪 | 按目录分区，查询只扫命中分区，少读数据 |
| Parquet 快 | 列裁剪 + 谓词下推 + 高压缩 + schema内嵌 |
| 列存快 | 少读列 + 高压缩 + 向量化 |
| schema-on-read | 先存文件、读时才套 schema（数据湖） |
| Lakehouse | 数据湖 + 表格式(Iceberg/Hudi/Delta)，带来 ACID/时间旅行 |
| 小文件问题 | 文件多被 S3(LIST/请求)+调度三重放大，靠 compaction 合并治理 |

### 三层分工记忆
> **Spark/Trino（算）→ 问 Hive Metastore（表在哪、schema、分区）→ 去 S3（读 Parquet 文件）。**
> 所有优化都是在减少最后一步"从 S3 读的字节数和请求数"：分区裁剪少读分区、列裁剪少读列、压缩少读字节、合并小文件少发请求。

---

## 附：本项目里能对上号的真实例子（面试可当谈资）

- **Hive DDL 模板**（`CLAUDE.md` §4）：`PARTITIONED BY (date, country)` + 完整 `ParquetHiveSerDe` SERDE——就是"给 S3 文件套 Parquet 表抽象 + 分区目录"的落地写法。
- **分区键放最后**（`CLAUDE.md`）：Spark `insertInto` 按列位置而非列名映射，分区列必须在 DataFrame 末尾，顺序错会静默错位。
- **S3 SlowDown 真实事故**（`docs/product-channel-attribution-30d-runbook.md`）：28M 行大表 + 3 LEFT JOIN，不加防护 conf → `MultiObjectDeleteSupport: SlowDown` 累积 → stage retry → job abort 不写表；靠 `shuffle.partitions=100` + `multiobjectdelete.enable=false` 等 conf 缓解。这是"S3 限流"最鲜活的注脚。
- **DDL 与逻辑同步落地**（`CLAUDE.md` ddl-mirror 规则）：任何表结构变更必须在 `scripts/ddl/` 补对应 `.sql`——本质是给"schema-on-read 的 schema 演进"留可追溯的时间线，防止 Metastore 认知和实际文件列错位（典型坑：save OVERWRITE 把 N+1 列写进 N 列 metadata，下游第 N+1 列被截断退化成 NULL）。

