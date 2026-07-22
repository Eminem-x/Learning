# Spark 深度（面试重点）

> 读者定位：维护 ETL 多年、日常通过 MLSQL 封装跑批、对 Spark「会用不会讲」的工程师。
> 目标：把原理讲透、把调优讲实，面试能答、实战能救火。
> 全文穿插本项目（`analytic-framework-workflows` / `offline_analytic_framework`）真实踩坑案例，
> 案例都有具体数字，面试时讲这些比背八股更能打动面试官。

---

## 0. 怎么用这份文档

每一节结构固定：**概念 → 为什么 → 面试怎么答**。
先通读一遍建立骨架，面试前重点看第 12 节「高频问答」和第 13 节「调优速查表」。

一句话记住 Spark 的三条主线，后面所有内容都挂在这三条上：

1. **Lazy（惰性）** —— transformation 不执行，action 才触发；一个 action 会从头重算整条血缘。
2. **Shuffle（洗牌）** —— 宽依赖触发跨节点数据重分布，是最贵的操作，也是绝大多数性能事故的根因。
3. **Memory（内存）** —— 内存计算是快的来源，也是 OOM / spill / 容器被杀的来源。

---

## 1. Spark 是什么 / 解决什么

### 概念
Spark 是一个**分布式内存计算引擎**。把一份大到单机装不下的数据切成很多分片（partition），
分发到集群里多台机器的进程（executor）上**并行计算**，中间结果尽量留在**内存**里而不是每步都落磁盘。

它不只是「一个更快的 MapReduce」，而是一整套统一栈：Spark SQL（结构化批处理）、
Structured Streaming（流）、MLlib（机器学习）、GraphX（图）共用同一个执行引擎。
我们项目里用的其实是 **Spark SQL**，只不过被 MLSQL 这层 DSL 封装了。

### 为什么比 MapReduce 快
面试最爱问这个。核心就三点，别只背「基于内存」这一句：

| 维度 | MapReduce | Spark |
|------|-----------|-------|
| 中间结果 | 每个 Map/Reduce 阶段结果**落 HDFS 磁盘**，下一步再读回来 | 中间结果尽量留**内存**，多个算子串成一条流水线 |
| 计算模型 | 只有 Map 和 Reduce 两个算子，复杂逻辑要串多个 MR job，job 间反复读写磁盘 | DAG（有向无环图）任意组合算子，一个 job 内多个 stage 流水线执行 |
| 任务调度 | 每个 task 起一个 JVM 进程，启动开销大 | executor 是常驻进程，task 是线程，复用 JVM，调度轻量 |
| 优化 | 无查询优化，全靠人写 | Catalyst 优化器 + Tungsten 代码生成，自动优化执行计划 |

一句话版本：**MapReduce 把每一步中间结果都落盘，多步作业磁盘 IO 反复往返；
Spark 用 DAG 把多步算子串成内存流水线、中间不落盘，加上常驻进程复用和查询优化，所以快一个数量级。**

⚠️ 一个诚实的补充（说出来显得你真懂）：Spark 快的前提是**中间结果放得进内存**。
一旦发生大量 **shuffle 落盘**或 **spill 溢写**，Spark 也会退化到跟 MR 一样疯狂读写磁盘。
本项目就有真实案例：一个小表 JOIN 百万行大表走了 sort-merge join，把大表 shuffle 落盘，
撑爆节点本地盘（ephemeral-storage 只有 ~5.8GB），executor 被 Evicted，task 重试 6.5 小时后 stage abort。
所以「Spark 一定比 MR 快」是错的，**用错了照样慢到死**。

### 面试怎么答
先给「内存 + DAG + 常驻进程 + Catalyst」四点结构，再补一句「但前提是数据放得进内存，
一旦大量 shuffle/spill 落盘优势会被抵消」，立刻显得比只会背书的人深。

---

## 2. 核心架构：Driver / Executor / Cluster Manager

### 概念
一个 Spark 应用运行时有三类角色：

- **Driver（驱动进程）**：应用的大脑。运行 `main`，构建 DAG，把 job 拆成 stage、stage 拆成 task，
  调度 task 到 executor，收集结果，维护 `SparkContext`。**Driver 挂了整个应用就挂。**
- **Executor（执行进程）**：干活的工人。每个是一个 JVM 进程，跑在集群 worker 节点上，
  内部用多个 core（线程）并行执行 task，缓存数据（cache 存这里），做 shuffle 读写。
- **Cluster Manager（集群管理器）**：管资源分配。可以是 YARN、Kubernetes、Standalone、Mesos。
  Driver 向它申请 executor，它负责在节点上拉起 executor 进程。
  **本项目跑在 Kubernetes 上**，每个 executor 是一个 K8s pod。

### 一个 Job 怎么被拆开
这是面试必考的「层级链」，一定要能一口气讲完：

```
Application（一个 spark-submit / 一次 MLSQL 运行）
  └─ Job（每个 action 触发一个 job：save / count / collect / print …）
       └─ Stage（以 shuffle 为边界切分；宽依赖处断开）
            └─ Task（stage 内每个 partition 一个 task，是最小执行单元）
```

- **一个 action = 一个 Job**。我们 MLSQL 里每个 `save` / `print` / `count` 都是一个 job。
- **Job 按 shuffle 切成 Stage**：遇到需要跨节点重分布数据的地方（宽依赖），就断成新的 stage。
  一个 job 有 N 个 shuffle，就大约有 N+1 个 stage。
- **Stage 按 partition 切成 Task**：一个 stage 里，数据有多少个 partition 就有多少个 task，
  它们是**同一段逻辑在不同数据分片上的并行副本**。task 是调度和执行的最小单位。

### DAG（有向无环图）
Driver 把你写的所有 transformation 组织成一张 DAG：节点是 RDD/DataFrame，边是算子依赖。
DAGScheduler 沿着这张图，从最终 action 往回推，在 shuffle 边界处切 stage，
把每个 stage 提交给 TaskScheduler 分发到 executor。DAG 是「逻辑计划」，stage/task 是「物理执行」。

### 宽依赖 vs 窄依赖（stage 边界的决定因素）
这是**理解 stage 划分的钥匙**，面试高频：

- **窄依赖（Narrow）**：父 partition 与子 partition 是**一对一或多对一**，
  子 partition 只依赖**确定的少数几个**父 partition。例：`map` / `filter` / `union` / `mapPartitions`。
  特点：**不需要 shuffle**，可以在同一个 executor 上流水线连续执行，同属一个 stage。
- **宽依赖（Wide / Shuffle Dependency）**：父 partition 会被**打散分发到多个**子 partition，
  子 partition 依赖**所有**父 partition。例：`groupByKey` / `reduceByKey` / `join`（非广播）/ `distinct` / `repartition`。
  特点：**必须 shuffle**（跨节点重分布数据），是 stage 的**切分点**。

记忆法：**窄依赖 = 数据不跨节点搬运（同 stage 流水线）；宽依赖 = 数据要跨节点洗牌（切 stage）。**
为什么在这切 stage？因为 shuffle 需要「上游全部写完 → 下游才能读」，是一个**全局同步屏障（barrier）**，
天然把执行切成前后两段。

### 面试怎么答
画出 `Application → Job → Stage → Task` 四层，说明「action 触发 job、shuffle 切 stage、partition 切 task」，
再点明「宽依赖是 stage 边界，因为 shuffle 是同步屏障」。能主动补一句「本项目每个 executor 是一个 K8s pod」加分。

---

## 3. RDD / DataFrame / Dataset

### 概念与演进
- **RDD（Resilient Distributed Dataset，弹性分布式数据集）**：Spark 最底层抽象。
  一个不可变、分区的分布式对象集合，带血缘（lineage）可容错重算。
  API 是函数式的（`map` / `filter` / `reduce`），你操作的是 JVM 对象，**引擎看不懂你在算什么**，无法优化。
- **DataFrame**：带 **schema（列名 + 类型）** 的分布式表，等价于关系表。
  你用 SQL 或 DSL（`select` / `groupBy` / `join`）表达**意图**，引擎知道列结构，
  可以走 **Catalyst 优化器**。底层数据用 **Tungsten** 的紧凑二进制格式存储，绕过 JVM 对象开销。
- **Dataset**：DataFrame 的类型安全版（Scala/Java 才有），编译期检查类型。
  在 Scala 里 `DataFrame = Dataset[Row]`。**Python/SQL 场景没有 Dataset**，所以对我们不重要。

演进关系：**RDD（2011，灵活但慢且不优化）→ DataFrame（2015，结构化 + 自动优化）→ Dataset（类型安全）**。

### 为什么现在用 DataFrame
1. **Catalyst 优化器**：把 SQL/DSL 解析成逻辑计划，做谓词下推（filter 尽早执行）、
   列裁剪（只读用到的列）、常量折叠、join 重排序等，最后生成优化的物理计划。RDD 完全没有这些。
2. **Tungsten 引擎**：数据用堆外二进制格式紧凑存储，减少 GC 压力；
   whole-stage code generation（整段代码生成）把一个 stage 的多个算子编译成一个函数，减少虚函数调用和临时对象。
3. **跨语言零损耗**：Python DataFrame 操作最终转成 JVM 执行计划，
   不像 RDD 那样每行数据在 Python 和 JVM 间序列化往返（PySpark UDF 才有这个损耗）。

我们项目里 MLSQL 的 `select ... to xxx`、`load hive` 最终都是 DataFrame/Spark SQL，
所以**天然吃到 Catalyst 优化**——这也是为什么写 SQL 比手写 RDD 更值得。

### 面试怎么答
「RDD 是底层、灵活但引擎看不懂无法优化；DataFrame 带 schema，能走 Catalyst 优化器和 Tungsten 引擎，
所以现在优先用 DataFrame/SQL。Dataset 是 Scala 的类型安全版，Python 用不到。」
Catalyst = 逻辑优化，Tungsten = 物理执行优化（内存布局 + 代码生成），点到即可。

---

## 4. Lazy Evaluation（惰性求值）—— 本节最重要

### 概念
Spark 的算子分两类：

- **Transformation（转换）**：定义「怎么算」，**不执行**。返回一个新的 DataFrame/RDD。
  例：`select` / `filter` / `join` / `groupBy` / `map` / MLSQL 的 `select ... to xxx`。
- **Action（行动）**：**触发实际执行**，让 Driver 提交 job、executor 真正跑数据。
  例：`save` / `count` / `collect` / `show` / `write` / MLSQL 的 `save` / `print` / `count`。

你写一串 transformation，Spark 只是在**攒 DAG**，一行数据都没动；
直到遇到 action，才「从数据源头开始，沿着 DAG 把这条链上所有 transformation 跑一遍」。

### Lineage（血缘）
每个 DataFrame 都记着「我是从哪些父 DataFrame、经过哪些算子得来的」，这条依赖链就是 **lineage**。
它有两个作用：
1. **容错**：某个 partition 丢了（executor 挂了），Spark 顺着 lineage **重算**这个 partition，不用整体重跑。
2. **触发计算**：action 沿着 lineage 从头执行。

### 为什么多个 action 会导致上游被重算 N 次 —— 面试和实战都爱考

**关键陷阱**：每个 action 都会**从血缘的源头**重新执行整条链。
如果一个 lazy view（比如一个重 JOIN 的中间结果）被 **N 个 action** 依赖，
而你**没有 cache/物化**它，那么这个中间结果会被**完整重算 N 次**——
包括它上游所有的 JOIN、GROUP BY、shuffle。

因为 lazy view（MLSQL 的 `select ... to xxx`）**只是内存里的一个逻辑视图，不是物理表**，
它不会自动保存结果。第二个 action 用到它时，Spark 不知道「刚才算过了」，只能顺着 lineage 再算一遍。

#### 本项目经典事故：13 个 print 把 base join 跑了 13 次

这是本项目 CLAUDE.md 记录的教科书级案例，面试讲这个非常出彩：

> 一个诊断 / 测试 MLSQL 脚本里有 **13 个 `print`**（都是 action），输出不同的聚合结果，
> 但它们**全部引用同一个 base CTE**（一个带多路 LEFT JOIN + 30 天 sum 的重 lazy view）。
> 由于没有 cache、也没有物化，这个 base join 被**从头重算了 13 次**：
>
> ```
> base join 跑 1 次  →  被 13 个 action 共享  →  实际跑 13 次
> shuffle 量级直接放大 13×
>   → S3 批量删除限流（MultiObjectDeleteSupport: SlowDown）
>   → stage retry 不断累积
>   → 最终 driver shutdown hook 触发 S3ShuffleManager.NoSuchMethodError
>   → job abort，表没写成
> ```

这个案例把「lazy 重算」的危害讲到了极致：不是慢一点，而是**直接把作业跑死、数据写不出来**。

同类还有 `compute_seller_dashboard_monthly_features_v1`：实测 `cpu/run = 7%`（严重 IO-bound），
耗时大头就是 `shop_monthly_metrics`(被下游用 4 次)、`shop_monthly_metrics_saved`(5 次)、
`final_shop_dashboad_top10_features`(2 次) 这些**重 lazy view 被多个 action 各自从头重算的 S3 shuffle 读**，
整个作业跑了 19.3 小时。

### 面试怎么答
「Transformation 惰性只攒 DAG，action 才触发，且每个 action 从血缘源头重算整条链。
所以一个重 JOIN 的中间结果被 N 个 action 依赖时，会被完整重算 N 次，
包括所有 shuffle——不 cache 的话 shuffle 量放大 N 倍。
我们项目就出过 13 个 print 把 base join 跑 13 次、shuffle 放大 13 倍触发 S3 限流把作业跑死的事故，
解法是 cache 或物化中间结果断开血缘。」

---

## 5. Shuffle 深讲 —— 最贵的操作

### 概念
**Shuffle = 在集群里把数据按 key 重新分区、跨节点重新分发。**
本质是「上游每个 task 按目标分区把自己的数据分好、写到本地（shuffle write），
下游每个 task 再从所有上游 task 那里把属于自己分区的数据拉过来（shuffle read）」。
这是一个 **M×N 的全连接数据搬运**（M 个 map task，N 个 reduce task）。

### 什么操作触发 shuffle
凡是「需要把相同 key 的数据聚到一起」的操作，都要 shuffle：

- **JOIN**（非广播）—— 两表按 join key 重分区到同一节点才能匹配
- **groupBy / reduceByKey / aggregate** —— 相同 key 聚到一起才能聚合
- **distinct** —— 全局去重，相同值要聚到一起
- **repartition / coalesce(增大分区)** —— 显式重分区
- **窗口函数 `over(partition by ...)`、`order by`（全局排序）**

### 为什么是最贵的操作
1. **磁盘 IO**：shuffle write 要把中间数据**落本地磁盘**（不是纯内存）。
2. **网络 IO**：shuffle read 要跨节点通过网络拉数据，M×N 条连接。
3. **序列化开销**：数据落盘和过网络都要序列化 / 反序列化。
4. **是同步屏障**：下游必须等上游**全部** map task 写完才能开始读，无法流水线，且是 stage 边界。
5. **小文件爆炸**：M 个 map × N 个 reduce = **M×N 个 shuffle 文件块**，
   分区数一大，文件数量指数级增长，对底层存储（尤其对象存储 S3）压力巨大。

### Shuffle 文件与 S3 限流的关系 —— 本项目血泪史
本项目 shuffle 数据**存在 S3**（用 `S3ShuffleManager`，shuffle 从 S3 流式读写），
这放大了 shuffle 的代价，踩过多个坑：

- **SlowDown 限流**：S3 对同一前缀的请求速率有限。shuffle 文件数一多（尤其批量删除时），
  就触发 `MultiObjectDeleteSupport: SlowDown` 限流。第 4 节那个 13 个 print 的案例就是这么死的。
- **病态重试放大成数小时死锁**：`compute_seller_dashboard_daily_features_v1` 反复 80 分钟 hang。
  根因是全局 base conf 设了病态的 S3 重试：
  `s3a.connection.timeout=500000`(500s) × `s3a.retry.limit=50` × `s3a.attempts.maximum=50`。
  **一次 S3 SlowDown 限流会被这套重试放大成数小时的单 task hang**。
  卡住的 task 数据极小（≤4.5MB，正常 1-2 秒），证明不是倾斜/算力问题，纯粹是 S3 卡死。
  而集群只有 `2 exec × 2 core = 4 slot`，几个 hang task 就锁死整个集群。

  两个关键教训：
  1. **override 要同时压 `retry.limit` 和 `attempts.maximum`**，否则 base 的 50 会赢。
     但压重试只是把「hang」变成「fail-fast」，持续 SlowDown 还是会 fail——**治本要从源头降低 shuffle 写并发**。
  2. **「改了 YAML 但没重新部署」**：git 里 YAML 早已是新值（2g/8g/40），运行时却还在用旧值（1g/7g/50）。
     判断配置是否真的生效，要 grep event log 的 Environment 里 `spark.executor.instances` / `s3a.retry.limit`。

### 怎么减少 shuffle（面试要能给方案）
1. **broadcast join** 替代 shuffle join（小表广播，见第 6 节）
2. **调 `spark.sql.shuffle.partitions`** 到合理值，控制 shuffle 文件数（见第 7 节）
3. **物化中间结果断血缘**，避免重复 shuffle（见第 4、10 节）
4. **能过滤/裁剪就先做**（谓词下推、列裁剪），减少参与 shuffle 的数据量
5. **`reduceByKey` 优于 `groupByKey`**（前者 map 端预聚合，shuffle 数据量小）

### 面试怎么答
「Shuffle 是按 key 跨节点重分布数据，JOIN/groupBy/distinct/repartition 都会触发。
它贵在磁盘落盘 + 网络传输 + 序列化 + 同步屏障，还会产生 M×N 个 shuffle 文件。
我们项目 shuffle 存 S3，文件一多就触发 SlowDown 限流；
还遇到过病态的 s3a 重试配置（500s×50×50）把一次限流放大成几小时死锁的事故。
优化就是 broadcast join、调 shuffle partitions、物化中间结果、尽早过滤。」

---

## 6. Broadcast Join（广播 Join）

### 概念
普通 JOIN（sort-merge join）需要把两张表都按 join key **shuffle** 到同一节点，很贵。
如果其中**一张表足够小**，Spark 可以把**小表整个广播到每个 executor**（每个 executor 存一份完整小表），
然后大表的每个 partition 在本地就能完成 JOIN——**完全不需要 shuffle 大表**。这就是 broadcast join（也叫 map-side join）。

### 什么时候会自动 broadcast
由 **`spark.sql.autoBroadcastJoinThreshold`** 控制，**默认 10MB**。
当 Spark **估算**参与 join 的一张表的大小 **< 这个阈值**时，自动走 broadcast join。
本项目 CLAUDE.md 建议这个值可设到 `104857600`（100MB），让 100MB 内的小表自动广播。

也可以用 **hint 强制广播**：`/*+ BROADCAST(小表别名) */`，跳过大小估算直接广播。

### 什么时候不会 / 不该 broadcast
- 表**估算**大小超过阈值（注意：Spark 估的是**未压缩**大小，Parquet 压缩表实际可能小很多，
  容易估过头而不广播——这时用 hint 强制）。
- **无法准确估算大小**（比如上游是复杂子查询/刚 shuffle 完的中间结果），Spark 保守不广播。
- 表**太大**：广播意味着每个 executor 都存一份，广播大表会**撑爆 executor 内存 / Driver 内存**
  （广播数据要先 collect 到 Driver 再分发），甚至 OOM。所以广播只适合小表。

### 本项目案例：用 broadcast 消除 shuffle 落盘
SEA 的 `product_signal_enrichment` 节点：一个小 `pool_features` 表 JOIN 百万行 `search_index` 大表，
走了 **sort-merge join**，把大表 shuffle 落盘，**撑爆节点 ephemeral-storage（仅 ~5.8GB）**，
executor 被 Evicted，task 重试 6.5 小时后 stage abort。
**真正的修法不是加大磁盘配额，而是 `/*+ BROADCAST(pf) */` 广播小表，从根上消除大表 shuffle 落盘。**
这个案例说明：广播的价值不只是「快」，而是**避免大表落盘/网络传输本身**。

### 面试怎么答
「小表广播到每个 executor，大表本地 join，避免 shuffle。
由 autoBroadcastJoinThreshold 控制（默认 10MB），也可 `/*+ BROADCAST(t) */` 强制。
坑在于 Spark 按未压缩大小估算容易估过头，Parquet 小表可能不自动广播，需要 hint。
广播大表会 OOM，只适合小表。我们项目用 BROADCAST hint 把一个小表 join 百万行大表的
sort-merge 改成广播，消除了撑爆本地盘的 shuffle 落盘。」

---

## 7. Partition 与并行度

### 概念
- **Partition（分区）**：数据的物理切片，是并行的最小数据单位。**一个 partition 对应一个 task。**
- **并行度**：同一时刻能并发跑多少 task，上限 = **总 core 数 = executor 数 × 每 executor 的 core 数**。

三个关键 partition 数：
1. **读入分区数**：由数据源决定（HDFS/Parquet block 数、`spark.sql.files.maxPartitionBytes` 默认 128MB）。
2. **`spark.sql.shuffle.partitions`**：**shuffle 之后的分区数，默认 200**。
   这是最常调的参数——JOIN/groupBy 之后的 partition 数就是它。
3. **写出分区数**：影响输出文件数。

### partition 数量对性能的影响（太多/太少各自的问题）

**分区太少**（比如默认 200 但你只有 4 个 core，或数据大但分区数小）：
- 并行度不足，core 闲置（4 个 core 却分 200 个 task 也不是坏事，但分 2 个 task 就浪费）。
- 单个 partition 数据太大，装不进内存 → **spill 溢写**甚至 **OOM**。
- 拖慢：本项目 `compute_seller_dashboard_monthly_features_v1` 有效并行度恰好 = `instances(2)×cores(2)=4`，
  IO-bound 场景下想提速就得加 `executor.instances` 提高并行。

**分区太多**（比如 200 个 partition 但数据只有几 MB）：
- 每个 task 处理的数据极少，**调度开销 > 计算开销**，大量时间花在起停 task 上。
- **shuffle 文件爆炸**：M×N 个文件，对 S3 直接触发 SlowDown 限流（见第 5 节）。
  本项目 CLAUDE.md 明确建议：小集群场景 `spark.sql.shuffle.partitions=100`（甚至更低）
  来**降低 shuffle 文件数、防 S3 限流**——注意这跟「默认 200」是反着调的，因为我们瓶颈在 S3 IO 不在 CPU。
- 小文件问题：写出时产生大量小文件，拖累下游读取和元数据管理。

**经验法则**：分区数大致设成**总 core 数的 2~4 倍**，让每个 core 能领到多个 task 做负载均衡，
同时每个 partition 大小控制在 **128MB~200MB** 量级。但**具体要结合瓶颈**：
CPU-bound 就往并行度靠拢，S3-IO-bound（我们的常态）就适当**压低分区数**防限流。

### 面试怎么答
「一个 partition 一个 task，并行度上限是总 core 数。
最常调的是 shuffle.partitions（默认 200）。太少会并行度不足、单分区过大导致 spill/OOM；
太多会调度开销大、shuffle 小文件爆炸。经验是设成 core 数 2-4 倍、每分区 128-200MB。
但要看瓶颈——我们项目 shuffle 存 S3，反而要把 shuffle.partitions 压到 100 来防 S3 SlowDown 限流。」

---

## 8. 数据倾斜（Data Skew）

### 概念
**数据倾斜 = shuffle 后某几个 key 的数据量远超其他 key，导致对应的 task 特别慢/OOM，
其他 task 早跑完在等它。** 典型表现：一个 stage 里 199 个 task 几秒跑完，剩 1 个跑几十分钟（长尾）。
成因：某些 key 天然是热点（NULL、默认值、爆款商品 id、大卖家 id）。

### 如何定位
- **Spark UI**：看某个 stage 的 task 时长分布 / shuffle read size 分布，
  某几个 task 的 **Duration / Shuffle Read** 是中位数的几十上百倍 → 倾斜。
- **看 key 分布**：`SELECT key, count(*) FROM t GROUP BY key ORDER BY 2 DESC LIMIT 20`，看头部 key 占比。
- **区分倾斜 vs S3 卡死**：本项目有个重要教训——不是所有「长尾 task」都是倾斜。
  `compute_seller_dashboard_daily_features_v1` 的 hang task **数据只有 ≤4.5MB**（正常 1-2 秒），
  却卡了 80 分钟，这**不是倾斜是 S3 重试卡死**。**判断依据：看卡住 task 的 shuffle read size——
  数据大才是倾斜，数据小却慢就是 IO/网络/限流问题。** 别把 S3 卡死误诊成倾斜去加盐。

### 常见解法
1. **广播 join 消除 shuffle**：如果倾斜发生在 join 且有一边是小表，直接 broadcast，没 shuffle 就没倾斜（首选）。
2. **加盐（salting）+ 两阶段聚合**：给热点 key 加随机前缀打散。
   - 第一阶段：`key` 加随机后缀 `key_0..key_n`，`GROUP BY 加盐key` 局部聚合（热点被拆到多个 task）。
   - 第二阶段：去掉盐，`GROUP BY 原key` 做全局聚合。
   这样把一个巨大 task 拆成 n 个小 task。
3. **join 加盐**：倾斜表的热点 key 加盐拆分，另一张表对应 key 膨胀 n 倍（每个盐值一份），再 join。
4. **单独处理热点 key**：把热点 key 过滤出来单独 broadcast join，其余走正常 join，最后 union。
5. **AQE 自动 Skew Join 优化**（Spark 3.0+）：开启 `spark.sql.adaptive.enabled` +
   `spark.sql.adaptive.skewJoin.enabled`，AQE 在运行时**自动检测倾斜分区并拆分**，很多场景不用手动加盐了。
6. **过滤脏 key**：如果倾斜来自 NULL / 无意义默认值，且业务上可丢弃，直接过滤掉。

### 面试怎么答
「倾斜是 shuffle 后个别 key 数据量爆炸，导致长尾 task。
定位看 Spark UI 的 task 时长/shuffle read 分布。
解法优先 broadcast 消除 shuffle，其次加盐 + 两阶段聚合打散热点 key，
Spark 3.0 后可以开 AQE 的 skewJoin 自动拆分倾斜分区。
补一个坑：不是所有长尾都是倾斜——要看卡住 task 的数据量，数据小却慢可能是 IO/限流卡死，别误诊。」

---

## 9. 内存模型

### Executor 内存构成
一个 executor JVM 的堆内内存（`spark.executor.memory`）在 Spark 里被划成几块（统一内存管理）：

```
Executor 堆内内存 = Reserved + (Execution + Storage) + User
```

- **Reserved（预留）**：固定 300MB，Spark 内部对象用，不可动。
- **Unified 区（Execution + Storage）**：默认占（堆 - 300MB）的 60%（`spark.memory.fraction=0.6`）。
  - **Execution（执行内存）**：shuffle、join、sort、aggregate 的中间数据。
  - **Storage（存储内存）**：cache/persist 的数据、broadcast 变量。
  - 两者**动态借用**：Execution 可以抢 Storage 的（cache 数据会被挤掉），Storage 不能抢正在用的 Execution。
- **User（用户内存）**：默认 40%。用户数据结构、UDF 里的对象、RDD 转换里 new 的东西。

### 堆内 vs 堆外
- **堆内（on-heap）**：JVM 堆，受 GC 管理，就是上面 `executor.memory` 那块。
- **堆外（off-heap）**：JVM 堆外的原生内存。包括 `spark.memory.offHeap`（Tungsten 可用堆外），
  以及**读 S3 的 native 缓冲、Parquet 解码、shuffle netty 网络缓冲**等 native 开销。
  这块**不受 executor.memory 管**，属于容器的 **memoryOverhead**。

### memoryOverhead（本项目重点坑）
在 K8s/YARN 上，容器实际申请的内存 = **堆内 + 堆外 overhead**。
本项目 `offline_analytic_framework` 全局设了 **`spark.kubernetes.memoryOverheadFactor: "0.3"`**（不是 Spark 默认 0.1）：

```
pod 内存申请 = spark.executor.memory + max(0.3 × executor.memory, 384MB)
```

拐点在 heap = 1.25g（384 = 0.3×1.25g）：
- heap ≤ 1.25g → overhead 走 384MB 地板（`1g → 1.38GiB`、`512m → 0.88GiB`）
- heap > 1.25g → overhead = 0.3×heap（`6g → 7.8GiB`，堆外税明显）

### OOM 是 Driver 挂还是 Executor 挂？——两种 OOM 要分清

**这是面试进阶题，也是本项目最反直觉的坑。**

- **JVM 堆 OOM（`java.lang.OutOfMemoryError`）**：堆内不够。
  - Executor 堆 OOM：单 partition 太大、大 join/聚合、cache 太多。→ 加 `executor.memory` 或加分区数。
  - **Driver OOM**：`collect()` 把大结果拉回 Driver、广播了大表、太多 task 元数据。→ 加 `driver.memory` 或别 collect 大数据。
- **容器被 K8s 杀（OOMKilled，退出码 137）**：**堆外**内存撑爆容器 limit。
  - 常见于**读 S3 的 native 缓冲 / Parquet 解码 / shuffle netty**这些堆外开销。
  - **反直觉的正确修法**：加 **`spark.executor.memoryOverhead`**（或调大 factor），**不是加 `executor.memory`**！

  本项目实测：一个 PySpark 读 S3 的 MLSQL，**512m 能跑完，768m/700m 反而 OOMKilled**。
  为什么加内存反而挂？因为**单独加 `executor.memory` 只喂堆内**，堆外没变还是不够；
  更糟的是堆变大后 **GC 变懒、堆 RSS 涨**，容器总内存更容易触顶被 137 杀。
  **137 是堆外问题，要修 overhead 不是修 heap。**

  另一条经验：**大 job 降内存收益更大**——overhead 是比例（0.3），
  `6g → 4g` 时 pod 总申请从 7.8g 降到 5.2g，省的是堆内+堆外一起省。

### Spill（溢写）
当 execution 内存不够放下 shuffle/sort/aggregate 的中间数据时，Spark 把一部分**溢写到磁盘**，
腾出内存继续算，最后再合并。spill 不会让作业失败，但**极慢**（退化成磁盘 IO）。
Spark UI 里看到 **Spill (Memory) / Spill (Disk)** 非 0 就是发生了溢写，是加内存或加分区数的信号。
反过来，`compute_seller_dashboard_monthly_features_v1` 实测 **spill = 0**，
所以它慢**不是内存问题**，加内存没用——是 S3 shuffle IO-bound，别乱开药。

### 面试怎么答
「Executor 堆分 Reserved(300MB) + Unified(Execution/Storage 动态借用) + User。
容器还要额外算堆外 memoryOverhead，我们项目 factor 设 0.3。
两种 OOM 要分清：JVM 堆 OOM 加 heap；退出码 137 是容器堆外被杀，要加 memoryOverhead 不是加 heap——
我们实测读 S3 的 job 512m 能跑、768m 反而 137，因为加堆内不解决堆外还让 GC 变懒 RSS 涨。
spill 是内存不够溢写到磁盘，UI 里 spill 非 0 就该加内存或加分区。」

---

## 10. 缓存（cache / persist）

### 概念
`cache()` / `persist()` 把一个 DataFrame 的计算结果**存起来**（默认内存），
之后的 action 直接读缓存，**不再沿血缘重算上游**。
- `cache()` = `persist(MEMORY_AND_DISK)`（DataFrame 默认级别；RDD 默认 `MEMORY_ONLY`）。
- `persist(level)` 可指定存储级别，`cache()` 是它的无参快捷方式。

**cache/persist 本身也是 lazy 的**——调用后不立即缓存，要等**第一个 action 触发**时才真正写入缓存，
之后的 action 才受益。

### 存储级别（StorageLevel）
| 级别 | 说明 |
|------|------|
| `MEMORY_ONLY` | 只存内存，放不下的 partition 不缓存、下次重算 |
| `MEMORY_AND_DISK` | 内存放不下的溢写磁盘（DataFrame 默认，最常用） |
| `MEMORY_ONLY_SER` | 内存中序列化存储，省空间但费 CPU |
| `DISK_ONLY` | 只存磁盘 |
| `*_2` | 加 `_2` 后缀 = 存两副本，容错 |

### 什么时候该 cache（判断标准）
本项目 CLAUDE.md 给了明确判据，直接背：

- **一个 lazy view 被 ≥2 个 action 依赖**（含下游 select 链上的 action）→ **必须 cache/物化**。
- **上游链路有 JOIN / GROUP BY / shuffle 等重操作** → **应该 cache/物化**。
- **单次性查询 / 只被一个 save 消费** → **不需要** cache。

一句话：**「重算代价高」×「被复用多次」= 该 cache。** 只满足一个不值得。

### ⚠️ 本项目特例：MLSQL interpreter 不支持 cache 语句
这是本项目一个**非常具体、面试可当亮点讲**的坑：

> 本项目的 MLSQL interpreter **实测不支持 `cache` 语句**：
> - `cache xxx;` → ANTLR 报 `missing TABLE at 'xxx'`
> - `cache TABLE xxx;` → 过了 ANTLR 但 interpreter 抛 `ValueError: Invalid mlsql_statement context`（grammar 收但没实现 handler）
>
> 所以在本项目里，「该 cache」的场景**只能用物化中间结果来实现**。

### 替代方案：物化中间结果到临时表（断开 lineage，最可靠）
既然不能 cache，本项目首选**物理物化**——把重 lazy view 落成 Hive 物理表再读回来，
**物理落盘 = 彻底断开血缘**，比 cache 还可靠（cache 内存不够会被挤掉重算，物化不会）：

```sql
-- 1. 原 lazy view 改名加 _raw 后缀
select ... to product_base_raw;

-- 2. save 强制写到 hive 物理表（saveAsTable 自动建表，无分区）
save product_base_raw overwrite
to hive."nebula_entity.tmp_product_base_<date>"
options format="parquet", method="saveAsTable";

-- 3. load 回来，用回原名（下游 print/save 引用不变）
load hive
select * from nebula_entity.tmp_product_base_<date>
to product_base;

-- 跑完手动清理：DROP TABLE hive.nebula_entity.tmp_product_base_<date>;
```

代价：每个物化点多 1 次 disk write + 1 次 read，但换来 N 个下游 action **不再重算 join**。
第 4 节那个 13 个 print 的事故，正解就是把 base CTE 物化掉，让 join 只跑 1 次。

次选方案：**减少 action 数**——把多个 print 的聚合用 `UNION ALL` 合并成一个 print，
N 次 driver collect 变 1 次。

### 用完记得 unpersist
cache 占着 Storage 内存，不用了要 `unpersist()` 释放，否则挤占后续计算的 Execution 内存。

### 面试怎么答
「cache/persist 把结果存下来，后续 action 不再沿血缘重算。
判据是『被 ≥2 个 action 依赖』且『上游有重 JOIN/shuffle』两个条件同时满足才值得 cache。
cache 默认 MEMORY_AND_DISK，也是 lazy 的、第一个 action 才生效。
一个真实亮点：我们项目的 MLSQL interpreter 不支持 cache 语句，
所以用『物化中间结果到临时 Hive 表』来断血缘——物理落盘比 cache 更可靠，
因为 cache 内存不够会被挤掉重算，物化不会。」

---

## 11. AQE（自适应查询执行）—— 现代 Spark 调优利器

### 概念
**AQE（Adaptive Query Execution，Spark 3.0+）**：让 Spark 在**运行时**根据**真实的中间数据统计**
动态调整执行计划，而不是只靠编译期的估算。开关 `spark.sql.adaptive.enabled`（Spark 3.2+ 默认开）。

三大能力：
1. **动态合并 shuffle 分区**（`coalescePartitions`）：shuffle 后如果很多分区很小，
   AQE 自动把它们合并成合适数量，**缓解「shuffle.partitions 设太大导致小文件多」的问题**。
2. **动态切换 join 策略**：运行时发现某表实际很小（估算失败但真实小），
   自动把 sort-merge join **改成 broadcast join**。
3. **动态优化倾斜 join**（`skewJoin`）：运行时检测到倾斜分区，自动拆分成多个小分区并行处理（见第 8 节）。

### 面试怎么答
「AQE 是运行时根据真实统计调计划：自动合并小 shuffle 分区、自动切 broadcast、自动拆倾斜分区。
它让很多手动调优（精调 shuffle.partitions、手动加盐）变得没那么必要。Spark 3.2 起默认开启。」

---

## 12. 面试高频问题 + 答题要点

**Q1. Spark 为什么比 MapReduce 快？**
中间结果留内存不落盘（MR 每步落 HDFS）；DAG 把多算子串成内存流水线（MR 只有 Map/Reduce 两算子、多 job 反复读写盘）；
executor 常驻进程复用 JVM（MR 每 task 起新 JVM）；Catalyst+Tungsten 自动优化。
补一句：前提是数据放得进内存，大量 shuffle/spill 落盘时优势会被抵消。

**Q2. 宽依赖和窄依赖的区别？为什么重要？**
窄依赖：父子 partition 一对一/多对一，不 shuffle，同 stage 流水线（map/filter）。
宽依赖：父 partition 打散到多个子 partition，必须 shuffle（groupBy/join/distinct/repartition）。
重要是因为**宽依赖是 stage 的切分边界**——shuffle 是同步屏障，把执行切成前后两段。
容错上窄依赖只需重算少数父 partition，宽依赖要重算整个上游 stage。

**Q3. transformation 和 action 的区别？惰性求值是什么？**
transformation 惰性、只攒 DAG 不执行（select/filter/join）；action 触发实际计算（save/count/collect）。
惰性的好处是 Spark 能看到完整 DAG 后统一优化（谓词下推、算子合并）。

**Q4. 一个 job 里有两个（多个）action 会发生什么？**（本项目核心考点）
每个 action 触发一个独立 job，且**从血缘源头重算整条链**。
如果多个 action 共享一个未 cache 的重 lazy view，这个 view 会被**重算 N 次**，shuffle 也放大 N 倍。
我们项目出过 13 个 print 把 base join 跑 13 次、shuffle 放大 13 倍、触发 S3 SlowDown 把作业跑死的事故。
解法：cache 或物化中间结果。

**Q5. 什么是 shuffle？为什么它最贵？**
按 key 跨节点重分布数据，M 个 map task × N 个 reduce task 全连接搬运。
贵在：磁盘落盘 + 网络传输 + 序列化 + 同步屏障（无法流水线）+ M×N 个 shuffle 文件。

**Q6. shuffle 怎么优化？**
broadcast join 消除 shuffle；调 shuffle.partitions 控制分区/文件数；物化中间结果避免重复 shuffle；
尽早 filter/列裁剪减少数据量；reduceByKey 优于 groupByKey（map 端预聚合）；开 AQE 合并小分区。

**Q7. 什么时候用 broadcast join？什么时候不该用？**
一张表小于 autoBroadcastJoinThreshold（默认 10MB）时自动广播，或用 `/*+ BROADCAST(t) */` 强制。
小表广播到每个 executor，大表本地 join，无需 shuffle。
不该用：表太大会撑爆 executor/Driver 内存 OOM。
坑：Spark 按未压缩大小估算，Parquet 小表可能不自动广播，要 hint。

**Q8. 数据倾斜怎么定位和处理？**
定位：Spark UI 看 task 时长/shuffle read 分布，个别 task 远超中位数就是倾斜；或 group by key count 看热点。
处理：优先 broadcast 消 shuffle；加盐 + 两阶段聚合打散热点 key；开 AQE skewJoin 自动拆分；过滤脏 key（NULL 等）。
坑：数据小却慢不是倾斜是 IO/限流卡死，看卡住 task 的 shuffle read size 区分。

**Q9. cache 和 persist 的区别？什么时候该 cache？**
`cache()` = `persist(MEMORY_AND_DISK)`，是 persist 的无参快捷方式；persist 可指定存储级别。
该 cache 的判据：被 ≥2 个 action 依赖 且 上游有重 JOIN/shuffle，两个条件同时满足。
只被一个 action 用、或上游很轻，不该 cache。cache 本身也 lazy，第一个 action 才生效。

**Q10. OOM 是 Driver 挂还是 Executor 挂？怎么修？**
看哪种 OOM。JVM 堆 OOM：Executor 堆不够（大 join/单分区过大）加 executor.memory 或加分区；
Driver OOM（collect 大数据/广播大表）加 driver.memory 或别 collect。
退出码 137（OOMKilled）是**容器堆外**被杀，要加 **memoryOverhead** 不是加 heap——
我们实测读 S3 的 job 512m 能跑、768m 反而 137，因为加堆内不解决堆外还让 GC 变懒 RSS 涨。

**Q11. RDD、DataFrame、Dataset 的区别？为什么用 DataFrame？**
RDD 底层灵活但引擎看不懂无法优化；DataFrame 带 schema，走 Catalyst 优化器 + Tungsten 引擎，自动优化；
Dataset 是 Scala 的类型安全 DataFrame（Python 没有）。现在优先 DataFrame/SQL。

**Q12. spark.sql.shuffle.partitions 是什么？调大调小的影响？**
shuffle 之后的分区数，默认 200。太小：并行度不足、单分区过大 spill/OOM。
太大：调度开销大、shuffle 小文件爆炸（对 S3 触发限流）。经验设 core 数 2-4 倍。
但要看瓶颈：我们项目 shuffle 存 S3，反而把它压到 100 防 SlowDown。

**Q13. Spark 内存模型？execution 和 storage 内存的关系？**
堆 = Reserved(300MB) + Unified(Execution+Storage，默认 60%) + User(40%)。
Execution（shuffle/sort/join）和 Storage（cache/broadcast）动态借用：
Execution 可抢 Storage（把 cache 挤掉），Storage 不能抢正在用的 Execution。

**Q14. spill 是什么？看到 spill 该怎么办？**
execution 内存放不下中间数据时溢写磁盘，不失败但极慢。
UI 里 Spill(Disk) 非 0 就是信号，加内存或加分区数。
反过来 spill=0 说明不是内存瓶颈，加内存无用（我们的 monthly 作业就是 spill=0 的 S3 IO-bound）。

**Q15. AQE 是什么？解决了哪些问题？**
运行时按真实统计调计划：动态合并小 shuffle 分区、动态切 broadcast join、动态拆倾斜分区。
Spark 3.2+ 默认开，让很多手动调优（精调 partitions、手动加盐）变得没那么必要。

---

## 13. 调优速查表

| 症状 / 场景 | 根因 | 手段 | 本项目备注 |
|------------|------|------|-----------|
| 一个 view 被多 action 复用、跑很久 | lazy 血缘重算 N 次 | cache / **物化到临时 Hive 表断血缘** | MLSQL 不支持 cache，只能物化；13 个 print → base join 跑 13 次事故 |
| 大表 join 慢 / shuffle 落盘撑爆盘 | sort-merge join 全 shuffle | `/*+ BROADCAST(小表) */` + 提高 autoBroadcastJoinThreshold(→100MB) | SEA 小表 join 百万行大表撑爆 5.8GB 本地盘，广播根治 |
| shuffle 小文件多 / S3 SlowDown 限流 | shuffle.partitions 过大 | 调低 `spark.sql.shuffle.partitions`（→100） | 我们瓶颈在 S3 IO，反着调（默认 200 太大） |
| 个别 task 长尾（数据大） | 数据倾斜 | 加盐+两阶段聚合 / broadcast / AQE skewJoin | 先确认是数据大——数据小却慢是 IO 卡死不是倾斜 |
| 退出码 137 / OOMKilled | 容器堆外内存爆（读 S3 native 缓冲） | 加 `spark.executor.memoryOverhead`（**不是** executor.memory） | factor=0.3；512m 能跑 768m 反而挂 |
| Executor 堆 OOM | 单分区过大 / 大聚合 / cache 太多 | 加 executor.memory 或加分区数 | — |
| Driver OOM | collect 大数据 / 广播大表 | 加 driver.memory / 别 collect / 别广播大表 | — |
| spill(Disk) 非 0、慢 | execution 内存不足溢写 | 加内存 / 加分区数 | spill=0 就别加内存，可能是 IO-bound |
| 并行度不足、core 闲置 | 分区数 < core 数 | 加分区 / 加 executor.instances | monthly 作业有效并行度仅 4，IO-bound 加 instances |
| S3 一次限流放大成数小时 hang | 病态 s3a 重试 (500s×50×50) | 同时压 `s3a.retry.limit` + `s3a.attempts.maximum`；源头降写并发 | 压重试只是 fail-fast，治本降 shuffle 并发 |
| 改了配置没生效 | YAML 改了没重新部署 | grep event log Environment 确认运行时值 | 判断 `executor.instances`/`s3a.retry.limit` 实际值 |
| 想少写调优代码 | — | 开 AQE（合并分区/切 broadcast/拆倾斜） | Spark 3.2+ 默认开 |

### 一句话总收
> **性能问题先分类瓶颈**：CPU-bound（看 cpu/run 高）→ 加并行度/优化算法；
> **内存 OOM** → 分清堆内(加 heap)还是堆外(加 overhead，退出码 137)；
> **IO-bound**（cpu/run 低、spill=0、shuffle 读多）→ **别加 CPU/内存**，
> 优先物化重复消费的 lazy view 断血缘、降 shuffle 并发防限流。
> 我们项目最贵的教训都是「凭 task 慢就误判 CPU/内存瓶颈乱加资源」，
> **一定先看 Spark UI 的 cpu/run、spill、shuffle read size 定位真瓶颈，再开药。**
