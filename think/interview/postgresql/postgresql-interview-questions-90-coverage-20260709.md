# PostgreSQL 面试题库（90% 覆盖率 · 白板级）

> 来源：个人思考 + 学习（对标《四年后端 + 轻量大数据能力地图》第 4 格：关系型数据库 / 第 7 格：存储引擎原理）
> 收录时间：2026-07-09 CST
>
> 说明：本篇是**综合入口 + 抽考清单**，覆盖 PostgreSQL 面试约 90% 的高频考点。体例对齐 [mysql 面试题库(广度)](../mysql/mysql-interview-questions-90-coverage-20260704.md) 与 [redis 面试题库(广度)](../redis/redis-interview-questions-90-coverage-20260709.md)：每个考点给「一句话答案 + 追问链条」，深挖到白板级。**贯穿全篇的主线是「和 MySQL 有什么不一样」**——面试官问 PG 十有八九在追这个。版本以 **PostgreSQL 16** 为基线，涉及旧版差异会标注。

---

## 30 秒电梯版（盖住全文也要能背出主线）

PG 面试就九条主线，几乎每条都能和 MySQL 对照：

1. **进程模型**：PG 是**多进程**（每连接一个 backend 进程，不是线程），靠共享内存 + 一堆后台进程（checkpointer/bgwriter/walwriter/autovacuum/wal sender）。连接贵 → 生产必配 **PgBouncer 连接池**。
2. **存储结构**：**堆表（heap）+ 独立索引**，PG **没有聚簇索引**——所有索引叶子都存 **TID（行物理位置）**，本质都是「二级索引」，不存在 MySQL「二级索引回聚簇索引」那一跳。大字段走 **TOAST** 单独存。
3. **MVCC（最核心差异）**：PG 的多版本**直接堆在表里**——一次 update = 插一个新行版本 + 把旧版本标记过期，每行带 `xmin/xmax`。不像 InnoDB 用 **undo 版本链**。代价：产生**死元组**，表会**膨胀（bloat）**。
4. **VACUUM**：因为死元组留在堆里，必须靠 **VACUUM/autovacuum** 回收空间、更新可见性位图、**冻结（freeze）**旧事务 ID。不 vacuum 会膨胀，极端会 **XID 回卷（wraparound）宕库**。这是 PG 特有的运维命门。
5. **事务隔离**：PG 默认 **Read Committed**（不是 MySQL 的 RR！）；PG 的 RR 实为**快照隔离(SI)**，天然无幻读；**Serializable 用 SSI（可串行化快照隔离）**——检测读写依赖环、按需回滚，是 PG 独门亮点。
6. **索引家族**：不止 B-tree——**GIN**（倒排，JSONB/数组/全文）、**GiST**（几何/范围/近邻）、**BRIN**（超大有序表、块级摘要、极省空间）、**Hash**、**SP-GiST**。还有**部分索引、表达式索引、覆盖索引(INCLUDE)**。
7. **WAL**：预写日志，PG 崩溃恢复 + 流复制的地基。`checkpoint` 刷脏页、`full_page_writes` 防页断裂。对应 MySQL 的 redo，但 PG 只有这一套物理日志 + 逻辑解码，**没有 binlog/redo 分家、也没有两阶段提交那套**。
8. **复制**：**流复制（物理，传 WAL）** 做高可用 standby；**逻辑复制（发布/订阅，解码 WAL 成行变更）** 做异构/部分同步。同步/异步可选，**replication slot** 防 WAL 被过早回收。
9. **优化**：`EXPLAIN (ANALYZE, BUFFERS)`；扫描方式（Seq/Index/Index-Only/**Bitmap**）、join 方式（Nested Loop/Hash/Merge）；代价模型 + 统计信息（`ANALYZE`）；连接池、分区表、`HOT` update 减少索引写。

卡在哪一条，就往对应章节回挖。

---

## 一、架构与进程模型（考频 ★★★★，PG 特色开场）

### Q1. PostgreSQL 的进程模型是怎样的？和 MySQL 有什么区别？

**一句话**：PG 是**多进程架构**——每个客户端连接由一个**独立的 backend 进程**服务（`fork` 出来的）；MySQL 是**多线程**（每连接一个线程）。

**追问链**：
- **入口 postmaster**：主守护进程 `postgres`（postmaster），监听端口，每来一个连接就 `fork` 一个子进程专门服务它，连接断开进程退出。
- **共享内存**：所有进程共享一块 **shared memory**，核心是 **shared_buffers**（数据页缓存，类比 InnoDB buffer pool）、WAL buffers、锁表、事务状态。
- **后台进程一堆**：`checkpointer`（做检查点刷脏页）、`background writer`（后台预刷脏页）、`walwriter`（刷 WAL）、`autovacuum launcher/worker`（自动清理死元组）、`wal sender/receiver`（复制）、`archiver`（归档 WAL）、`stats collector/logical replication worker` 等。
- **多进程的代价**：进程比线程重，**每个连接占独立内存（work_mem 等按连接算）+ fork 开销**，连接数一多内存和上下文切换就爆 → **PG 连接数不能像 MySQL 那样随便开几千**，通常几百就要上连接池。
- **多进程的好处**：隔离性强，一个 backend 崩溃不容易带垮整库；实现简单、稳定。

### Q2. 为什么 PostgreSQL 必须用连接池（PgBouncer）？

- **根因**：一连接一进程，每个 backend 的**建立/销毁成本高**（fork + 初始化 catalog cache），且**每连接常驻内存**。高并发短连接场景下，几千连接 = 几千进程，内存和调度直接压垮。
- **PgBouncer**：轻量连接池中间件，在应用和 PG 之间维护一个**小而稳定的物理连接池**，把成百上千的客户端连接**复用**到几十个真实后端连接上。
- **三种池化模式**：`session`（连接绑定整个会话，最安全）、`transaction`（事务级复用，最常用，一个事务结束就还池）、`statement`（语句级，最激进，不支持多语句事务）。用 `transaction` 模式要注意**别用会话级特性**（prepared statement、临时表、`SET`）。
- 对比 MySQL：MySQL 线程模型 + 自带 thread cache，连接开销小得多，通常不强依赖外部连接池（但高并发也用 ProxySQL 等）。

---

## 二、存储结构：堆表、TID、TOAST（考频 ★★★★★，和 MySQL 分水岭）

### Q3. PostgreSQL 有聚簇索引吗？表在磁盘上怎么组织？

**核心结论：PG 没有聚簇索引（clustered index），表是「堆（heap）」组织，所有索引都是独立的、指向堆的「二级索引」。**

- **堆表（heap）**：行（tuple）**无序**地存在数据文件的页（page，默认 8KB）里，插入哪就在哪，物理上不按主键排序。
- **索引全都独立**：主键索引也只是一棵单独的 B-tree，**叶子存的是 TID（行的物理地址）**，不存整行。查任何索引都要拿 TID 再去堆里取行。
- **对比 MySQL InnoDB**：InnoDB 是**索引组织表**——主键聚簇索引的叶子直接存整行，二级索引叶子存主键、要**回表**。PG 里根本没有「表即主键 B+ 树」这回事，也就**没有回表的概念**（但有「访问堆」这一步，见 Index-Only Scan Q19）。
- **`CLUSTER` 命令**：PG 有个 `CLUSTER table USING index`，能**一次性**把堆按某索引物理重排——但它是**一次性操作**，之后的增删改不再维持顺序，和 InnoDB 那种「持续维护的聚簇索引」完全不是一回事。

### Q4. TID / CTID 是什么？

- **TID（Tuple ID）= (页号, 页内偏移)**，行在堆里的物理位置。系统列 **`ctid`** 就是它，`SELECT ctid, * FROM t` 能看到。
- **索引叶子存的就是 TID**：查索引 → 得到 TID → 按 (页号, 偏移) 直接定位堆里的行。
- **TID 会变**：`UPDATE` 在 PG 里是「插新版本」，新版本 ctid 变了（除非 HOT，见 Q15）；`VACUUM FULL`/`CLUSTER` 也会挪行。所以 ctid **不能当稳定主键用**。

### Q5. TOAST 是什么？大字段怎么存？

**TOAST（The Oversized-Attribute Storage Technique，超大属性存储）**：PG 的页固定 8KB，一行必须放进一页（不能跨页），那超长的 text/jsonb/bytea 怎么办？→ TOAST。

- **机制**：当一行超过约 **2KB（TOAST_TUPLE_THRESHOLD）** 时，把大字段**压缩**，压缩后还大就**切片**存到该表专属的 **TOAST 附表**（`pg_toast.*`）里，主表行里只留一个指针。
- **策略（每列可设）**：`PLAIN`（不压不外置）、`EXTENDED`（先压再外置，默认）、`EXTERNAL`（只外置不压，利于子串快速读取）、`MAIN`（尽量压缩但留主表）。
- **意义**：大字段对主表的行宽几乎无影响 → 不读大字段的查询很快（不用把大 blob 拖进内存）。类比但不同于 MySQL 的行外存储（`DYNAMIC` 行格式下 InnoDB 也会把长 VARCHAR/BLOB 存到 overflow 页）。

### Q6. 页（page）内部结构？FILLFACTOR 有什么用？

- **页结构**：页头 → **行指针数组（line pointer / ItemId）** → 空闲空间 → 从页尾往前堆的**实际行数据**。索引里的 TID 偏移指向的是「行指针」，行指针再指向实际数据 → 这层间接让行在页内移动（如 HOT）时不必改索引。
- **FILLFACTOR**：建表/建索引时预留的**页填充比例**（表默认 100，索引 B-tree 默认 90）。调低（如 90）→ 每页留空 → **给 HOT update 和就地更新留位置**，减少行迁移和页分裂。更新频繁的表调低 fillfactor 是常见优化。

---

## 三、MVCC：PostgreSQL 的心脏（考频 ★★★★★，最高频追问）

### Q7. PostgreSQL 的 MVCC 怎么实现？和 MySQL 有什么本质区别？

**一句话**：PG 把**多个行版本直接存在堆表里**，每行带隐藏系统列 **`xmin`（插入它的事务 ID）** 和 **`xmax`（删除/更新它的事务 ID）**，靠事务快照判断哪个版本可见。MySQL InnoDB 是把旧版本放进 **undo log 版本链**，行里只留最新版。

**这是 PG 面试的最大分水岭，展开讲**：
- **INSERT**：写一行，`xmin = 当前事务ID`，`xmax = 0`。
- **DELETE**：不真删，只把该行 `xmax = 当前事务ID`（标记「被这个事务删了」）。
- **UPDATE**：= DELETE 旧版本 + INSERT 新版本。旧行 `xmax` 设为当前事务，新行 `xmin` 设为当前事务，两者用 ctid 串起来。**所以 PG 的 update 天生会产生一个死元组**。
- **可见性**：一个事务读一行时，看它的 `xmin/xmax` 相对自己**快照**是否可见（插入它的事务已提交、删除它的事务未提交/未开始 → 可见）。
- **和 MySQL 的深层差异**：
  - InnoDB 的旧版本在 undo 里、会被 purge 线程清掉，**主表始终紧凑**；PG 的旧版本**就躺在主表里**，要靠 VACUUM 清 → **PG 有膨胀问题，InnoDB 没有**（但 InnoDB 的长事务会撑大 undo/history list）。
  - InnoDB 回滚极快（丢弃 undo）；PG 回滚也快（新版本直接失效，不用改回），但**留下的死元组要 vacuum**。
  - PG 读旧版本不用「顺着链找」（版本就在堆里各处），InnoDB 要顺 undo 链重建。

### Q8. xmin/xmax 之外，事务快照怎么判断可见性？

- **事务快照（snapshot）**由三部分定义：`xmin`（当前最小活跃事务）、`xmax`（下一个待分配事务号），以及 `xip_list`（快照时刻仍活跃的事务列表）。
- **可见性规则**（简化）：一个行版本对我可见 ⟺ **插入它的事务在我快照之前已提交** 且 **删除它的事务尚未提交（或就是我看不到的）**。
- **CLOG（commit log / `pg_xact`）**：记录每个事务 ID 的状态（进行中/已提交/已中止）。判断 xmin/xmax 对应事务提没提交，就查 CLOG。为加速，行上有 **hint bits**（提交状态缓存位），第一次判定后回写，省得反复查 CLOG。
- **RC vs RR 的快照时机**（和 MySQL 一致的直觉）：**Read Committed 每条语句取一个新快照**（所以能看到别的事务新提交的数据）；**Repeatable Read 事务开始时取一次快照、整个事务用到底**（所以可重复读）。

### Q9. 为什么说 PG 的 update 代价可能比 MySQL 大？

- PG 的 `UPDATE` = 写新版本 + 旧版本变死元组，**且所有指向旧行的索引都可能要加新索引项**（除非 HOT，见 Q15）。改一个非索引列，最坏也要动所有索引。
- InnoDB 更新是**就地更新聚簇索引的行**（变长导致页满才分裂），二级索引只在被改列涉及时才动。
- **缓解**：① `HOT`（Heap-Only Tuple）——更新的列不涉及任何索引，且同页有空间 → 新版本留同页、**不动索引**（靠调低 fillfactor 制造空间）；② 及时 autovacuum 回收死元组。

---

## 四、VACUUM 与事务 ID 回卷（考频 ★★★★★，PG 特有命门）

### Q10. VACUUM 是干什么的？为什么 PG 非它不可？

**根因**：MVCC 把死元组（被删/被旧 update 淘汰的行版本）留在堆里，不清理就**膨胀（bloat）**——表和索引越来越大、缓存命中率下降、扫描变慢。VACUUM 就是清道夫。

**VACUUM 做四件事**：
1. **回收死元组空间**：把死元组占的空间标记为可复用（**注意：普通 VACUUM 不把空间还给操作系统**，只是页内可重用；要还给 OS 得 `VACUUM FULL`）。
2. **更新可见性位图（Visibility Map, VM）**：标记「整页都是所有事务可见的活元组」的页 → 这是 **Index-Only Scan** 能免访问堆的前提，也是下次 vacuum 能跳过该页的依据。
3. **更新 Free Space Map（FSM）**：记录每页剩余空间，供插入时找位置。
4. **冻结（FREEZE）旧事务 ID**：防 XID 回卷（见 Q12）。

### Q11. VACUUM 和 VACUUM FULL 区别？autovacuum 是什么？

| | `VACUUM`（普通） | `VACUUM FULL` |
|--|-----------------|---------------|
| **做法** | 就地标记死元组空间可复用 | **重写整张表**到新文件，物理紧凑 |
| **空间** | 不还给 OS（表文件不缩小） | 还给 OS（表真正变小） |
| **锁** | 弱锁，**不阻塞读写** | **AccessExclusiveLock，全程锁表** |
| **场景** | 日常、autovacuum 自动做 | 膨胀严重时手动救火（业务低峰） |

- **autovacuum**：后台进程自动触发 VACUUM 和 `ANALYZE`（收集统计信息）。触发阈值：死元组数超过 `autovacuum_vacuum_threshold + autovacuum_vacuum_scale_factor × 表行数`（默认 scale factor 0.2，即死元组超 20% 就触发）。
- **大表调优**：默认 20% 对大表太迟钝（1 亿行要死 2000 万才动）→ 常把大表的 `scale_factor` 调小、`threshold` 调大，或调高 `autovacuum_max_workers` / `autovacuum_vacuum_cost_limit` 让它更勤更快。
- **`VACUUM FULL` 的替代**：生产更爱用 `pg_repack` 扩展，**在线重组表、不长时间锁表**。

### Q12. 什么是事务 ID 回卷（XID Wraparound）？为什么可能让整个库停摆？

**这是 PG 最凶险、也最爱考的坑。**

- **XID 是 32 位**，约 **21 亿（2³²）** 个就用完，用完要**回卷**（绕回 0 重新用）。
- **可见性靠比较 XID 大小**判断「谁在谁之前」。一旦回卷，一个很老的事务 XID 会显得「比现在还新」→ 本该可见的旧数据**突然变得不可见** → 数据「消失」。灾难。
- **解法——冻结（FREEZE）**：VACUUM 会把足够老的行版本的 xmin 标记为一个特殊的「**FrozenXID / 冻结标志**」，表示「无限旧、对所有事务永远可见」，从而把它移出 XID 比较的赛道。
- **防线告警**：当某个库距离回卷只剩约 **1000 万** XID 时，PG 强制发警告；再近就**拒绝新事务、只读**，逼你 vacuum。到这一步库就「停摆」了。
- **运维要点**：监控 `datfrozenxid` / `age(relfrozenxid)`，保证 autovacuum 能跟上写入速度、别被长事务或失败的 autovacuum 拖住。**长事务是头号敌人**——它按住 xmin 让 vacuum 不能回收、也不能推进冻结。

### Q13. 为什么长事务对 PostgreSQL 特别有害？

- **按住可见性水位**：一个开着不提交的老事务，其快照 `xmin` 很低，VACUUM **不敢清理任何比它新的死元组**（怕这老事务还要读）→ 死元组堆积、表狂膨胀。
- **拖慢冻结**：vacuum 推不动 `relfrozenxid` → 逼近 XID 回卷。
- **占锁**：长事务可能长期持锁，阻塞 DDL、其他事务。
- **排查**：`pg_stat_activity` 看 `state='idle in transaction'` 且 `xact_start` 很老的连接——「事务开着但啥也不干」是经典元凶（应用忘了 commit/rollback）。设 `idle_in_transaction_session_timeout` 自动踢掉。

---

## 五、索引家族（考频 ★★★★★，PG 强项）

### Q14. PostgreSQL 有哪些索引类型？各自适用场景？

PG 索引类型远比 MySQL 丰富，**面试问「你只知道 B-tree 吗」时要能报菜名**：

| 类型 | 结构 | 适用场景 |
|------|------|---------|
| **B-tree** | B+ 树（默认） | 等值 `=`、范围 `< > between`、排序、最左前缀。绝大多数场景 |
| **Hash** | 哈希表 | 只支持等值 `=`；PG 10 起写 WAL、可复制（之前不推荐）。用途窄，B-tree 基本能替 |
| **GIN** | 倒排索引（Generalized Inverted Index） | **JSONB、数组、全文检索（tsvector）**——「一个字段里含多个值，要查包含关系」。`@>`、`?`、全文 `@@` |
| **GiST** | 通用平衡树框架 | **几何 / 空间（PostGIS）、范围类型、最近邻(KNN)、模糊匹配**。可扩展的「有损」索引 |
| **SP-GiST** | 空间分区树（四叉树/前缀树） | 非平衡数据：IP 前缀、电话、文本前缀 |
| **BRIN** | 块范围索引（Block Range Index） | **超大且物理有序**的表（如按时间追加的日志/时序）。只存每段块的 min/max 摘要，**索引极小**（GB 级表索引几 MB），代价是精度粗 |

- **GIN vs GiST 口诀**：GIN **查得快、建/更新慢、占空间大**（适合读多写少、静态数据）；GiST **更新快、查稍慢**（适合动态数据、空间/KNN）。
- **BRIN 的杀手锏**：对「插入顺序≈物理顺序」的大表（时序、日志），BRIN 用几 MB 就能索引上百 GB，秒杀 B-tree 的体积。前提是**数据物理有序**，否则每个块 min/max 都横跨全域、过滤不掉东西。

### Q15. HOT update 是什么？为什么能省索引写？

**HOT（Heap-Only Tuple，仅堆元组更新）**——PG 为缓解「update 要动所有索引」而生的优化：

- **触发条件**：① 本次 UPDATE **不修改任何被索引的列**；② 新版本能塞进**同一个页**（有空闲空间，靠调低 fillfactor 制造）。
- **效果**：新版本直接放同页，**旧版本的行指针指向新版本形成 HOT 链**，索引项**完全不动**（索引仍指向旧行指针，顺链找到最新版本）。→ 省掉所有索引的写入和后续膨胀。
- **和 fillfactor 的配合**：更新频繁的表把 fillfactor 调到 80~90，页内留空间给 HOT，是标准调优。
- 类比：这解决的正是 Q9 说的「PG update 比 MySQL 贵」的问题。

### Q16. PG 的特色索引特性：部分索引、表达式索引、覆盖索引？

- **部分索引（Partial Index）**：`CREATE INDEX ... WHERE condition`，**只索引满足条件的行**。经典：只给 `WHERE status='active'` 建索引（99% 是已完成订单、只查活跃的）→ 索引小、维护轻。MySQL 没有这个（只能索引全表）。
- **表达式索引（Expression Index）**：`CREATE INDEX ON t (lower(email))`，索引一个**函数/表达式的结果**。这样 `WHERE lower(email)=...` 能走索引——对照 MySQL「函数包裹列导致索引失效」，PG 可以直接给表达式建索引绕过。（MySQL 8.0 也有函数索引了，但 PG 更早更通用。）
- **覆盖索引 `INCLUDE`（PG 11+）**：`CREATE INDEX ON t (a) INCLUDE (b, c)`——`a` 参与索引排序，`b/c` 只**附带存在叶子**用于覆盖查询，不参与排序/不占键空间。配合 Index-Only Scan 免访问堆。
- **并发建索引 `CREATE INDEX CONCURRENTLY`**：不锁表建索引（慢一点、扫两遍），生产给大表加索引的标配。对照 MySQL 8.0 的 Online DDL。

---

## 六、查询执行与优化（考频 ★★★★★）

### Q17. EXPLAIN 怎么看？和 EXPLAIN ANALYZE 区别？

- **`EXPLAIN`**：只出**优化器估算**的执行计划（估算行数、代价），**不真跑**。
- **`EXPLAIN ANALYZE`**：**真正执行** SQL，给出**实际**行数、时间、循环次数。诊断慢查询用它。⚠️ 会真跑，`INSERT/UPDATE/DELETE` 要包在事务里 `ROLLBACK`。
- **常用增强**：`EXPLAIN (ANALYZE, BUFFERS)`——加上**缓冲区命中/读盘统计**（`shared hit` vs `read`），判断是不是没命中缓存在读盘。
- **关键看点**：① **估算行数 vs 实际行数差多大**（差几个数量级 → 统计信息过期，该 `ANALYZE`）；② 扫描方式；③ join 方式；④ 有没有 `Rows Removed by Filter`（扫太多滤太多）。

### Q18. PG 有哪几种扫描方式？Bitmap Scan 是什么？

| 扫描方式 | 含义 |
|---------|------|
| **Seq Scan** | 全表顺序扫。小表或要读大部分行时反而最优（顺序 IO） |
| **Index Scan** | 走索引拿 TID，逐个**随机访问堆**取行。命中行少时优 |
| **Index Only Scan** | 查询列全在索引里 + 页在可见性位图中全可见 → **完全不访问堆**（见 Q19） |
| **Bitmap Heap Scan** | 先扫索引把命中的 TID **收集成一张按页排序的位图**，再**按物理页顺序**批量访问堆 → 把随机 IO 变半顺序 IO |

- **Bitmap Scan 的意义（PG 特色，面试常追）**：当索引命中行数**中等**（比 Index Scan 的逐行随机划算、又比 Seq Scan 少）时，PG 先用索引建 TID 位图、按页号排序，再顺序读堆页 → **避免同一页重复读、把随机 IO 拉成顺序**。还能对**多个索引的位图做 AND/OR 合并**（`BitmapAnd`/`BitmapOr`），一条查询用上多个索引——这是 InnoDB 做不到的（InnoDB 一般一表一次只用一个索引 + index merge 较弱）。

### Q19. Index-Only Scan 为什么还需要「可见性位图」？

- 索引里只有键值和 TID，**没有 MVCC 可见性信息**（xmin/xmax 在堆里）。所以就算查询列全在索引里，PG 默认仍要访问堆确认「这个版本对我可见吗」。
- **可见性位图（VM）** 由 VACUUM 维护，标记「某页所有行对所有事务都可见」。若索引项对应的页在 VM 里是「全可见」→ **可跳过访问堆**，真正 Index-Only。
- **推论**：一张**很久没 vacuum** 的表，VM 过期 → Index-Only Scan 退化成频繁访问堆，变慢。所以「覆盖索引没生效/时快时慢」常和 vacuum 有关。

### Q20. PG 的 join 有哪几种？优化器怎么选？

| Join 算法 | 适用 |
|-----------|------|
| **Nested Loop** | 一侧行很少（或有索引可探测）。外表每行去内表查一次 |
| **Hash Join** | 大表 join、等值连接。把小表建哈希表，大表探测。要 `work_mem` 够 |
| **Merge Join** | 两侧都**有序**（有索引或排序），归并。适合大表 + 有序 |

- **代价模型（cost-based）**：优化器用统计信息估各方案代价，选最小的。PG 优化器很强（比早期 MySQL 强），支持复杂子查询提升、CTE 内联（12+）。
- **统计信息**：`ANALYZE` 收集列的直方图、distinct 值、相关性存进 `pg_statistic`。统计过期 → 估算失准 → 选错计划。这是慢查询头号原因之一。PG 14+ 还有**扩展统计（`CREATE STATISTICS`）** 处理多列相关性。
- **调优旋钮**：`work_mem`（每个排序/哈希节点可用内存，太小会落盘 → `external merge Disk`）、`random_page_cost`（SSD 上调低到 1.1 让优化器更爱走索引）、`effective_cache_size`。

### Q21. `work_mem` 设太小/太大会怎样？

- **太小**：排序、哈希、去重超过 `work_mem` 就**落盘**（`temp files`），EXPLAIN 里出现 `Sort Method: external merge Disk` → 慢。
- **太大**：`work_mem` 是**每连接、每个计划节点**都能各用一份的！一个复杂查询可能有多个排序/哈希节点 × 高并发连接 → 内存瞬间放大到 `work_mem × 节点数 × 连接数`，可能 OOM。
- **正解**：全局设保守值，对个别重查询会话级 `SET work_mem` 临时调大。

---

## 七、事务与隔离级别（考频 ★★★★★，PG 独门 SSI）

### Q22. PG 的默认隔离级别是什么？和 MySQL 有何不同？

- **PG 默认 Read Committed；MySQL(InnoDB) 默认 Repeatable Read。** 这是必答的差异点。
- **PG 只有三个「真」级别**：`Read Committed`、`Repeatable Read`、`Serializable`（写 `Read Uncommitted` 也会被当作 Read Committed —— PG **不存在脏读**，因为 MVCC 从不读未提交版本）。

| 级别 | 脏读 | 不可重复读 | 幻读 | 序列化异常 |
|------|:--:|:--:|:--:|:--:|
| Read Committed（默认） | 不会 | 会 | 会 | 会 |
| Repeatable Read | 不会 | 不会 | **不会**（快照隔离） | 会 |
| Serializable | 不会 | 不会 | 不会 | **不会**（SSI） |

### Q23. PG 的 Repeatable Read 为什么天然不会幻读？（对比 MySQL）

- **PG 的 RR = 快照隔离（Snapshot Isolation, SI）**：事务开始时拿一个快照，整个事务只看这个快照 → 无论别人插入多少行，你**都看不到** → 天然无幻读，**不需要 MySQL 那种间隙锁（gap lock）**。
- **对比 MySQL**：InnoDB 的 RR 靠 **MVCC（快照读）+ Next-Key Lock（当前读的间隙锁）** 组合防幻读，`SELECT ... FOR UPDATE` 会锁间隙。PG 的 RR **不加间隙锁**，靠纯快照。
- **代价——写偏斜（write skew）**：SI 挡不住「序列化异常」。经典例子：两个事务各自读到「至少还有一个医生值班」，各自把自己设为休息，都提交 → 结果没人值班。两个事务读的行没冲突、写的行不同，SI 检测不到 → 这就是 `Serializable` 要解决的。
- **PG RR 的另一个行为**：并发 UPDATE 同一行时，PG RR 会直接**报序列化失败错误**（`could not serialize access`）让你重试，而不是像 RC 那样等锁后基于最新值重算。应用要准备**重试逻辑**。

### Q24. Serializable Snapshot Isolation（SSI）是什么？PG 怎么做到可串行化又不重锁？

**SSI 是 PG 的招牌功能（9.1+），面试答出来是加分项。**

- **传统 Serializable**：靠两阶段封锁（2PL）+ 谓词锁，重锁、并发差。
- **PG 的 SSI**：**乐观**——事务照常用快照隔离跑（不加读锁），但**监控事务间的「读写依赖（rw-dependency）」**。当检测到可能形成**危险的依赖环**（会导致不可串行化的结构，即「dangerous structure」——一个事务的读被另一个事务的写覆盖，且构成环）时，**主动 abort 其中一个事务**，报 `could not serialize access due to read/write dependencies`。
- **关键点**：
  - 不加读锁，**读不阻塞读、读不阻塞写** → 并发接近 SI，却保证真正的可串行化。
  - 用 **SIREAD 锁（谓词锁）** 记录「谁读过什么范围」，但这是**追踪用的轻量标记，不阻塞别人**，只用于检测冲突。
  - 应用**必须处理序列化失败并重试**——这是使用 SSI 的前提。
- **对比**：MySQL 的 Serializable 是给所有普通 `SELECT` 加共享锁（LiS/2PL），并发差得多。PG 的 SSI 是学术界（Cahill 的论文）落地的先进方案。

### Q25. PG 的行锁存在哪？为什么说「PG 行锁不占内存」？

- **PG 的行锁信息存在**元组自己身上（`xmax` + infomask 标志位），**不在内存里维护锁表** → **锁多少行都不额外吃内存**。
- 对比 InnoDB：行锁在内存的锁结构里维护。
- **推论**：PG 里 `UPDATE` 一百万行不会有「锁内存爆掉/锁升级」问题（PG 也**没有锁升级**概念）；但代价是**判断行是否被锁要访问那一行**。
- **表级锁**有 8 种模式（`AccessShareLock` ~ `AccessExclusiveLock`），DDL 用强锁。查 `pg_locks` 看锁等待。
- **咨询锁（Advisory Lock）**：`pg_advisory_lock(key)`——应用层自定义的、和具体数据无关的锁，用来做分布式互斥/串行化任务，PG 特色。

---

## 八、WAL、崩溃恢复与检查点（考频 ★★★★）

### Q26. WAL 是什么？和 MySQL 的 redo/binlog 怎么对应？

- **WAL（Write-Ahead Log，预写日志）**：任何数据页改动**先写 WAL 再改内存页**，崩溃后靠重放 WAL 恢复。这是 **WAL 原则**（对照 [PG WAL 笔记](../../infra/postgresql-wal-replication-and-amplification.md)）。
- **和 MySQL 对应**：
  - PG 的 **WAL ≈ InnoDB 的 redo log**（物理/物理逻辑日志、崩溃恢复、循环+归档）。
  - 但 PG **没有独立的 binlog**！MySQL 有「redo（引擎层）+ binlog（server 层）」两套，还得靠**两阶段提交**保证一致。PG 只有 WAL 一套，做复制靠**流复制传 WAL**（物理）或**逻辑解码 WAL**（逻辑复制），**不存在两日志一致性问题**。
  - PG 也没有 MySQL 那种独立的 **undo 表空间**——旧版本在堆里（见 MVCC Q7），所以 PG 没有「undo」这个独立组件，回滚靠标记版本失效。
- **好处/代价**：PG 一套日志更简洁；代价是逻辑复制要额外解码。

### Q27. 检查点（Checkpoint）是什么？full_page_writes 为什么存在？

- **Checkpoint**：周期性把 shared_buffers 里的**脏页刷到数据文件**，并在 WAL 里打一个点 → 崩溃恢复只需从最近 checkpoint 之后的 WAL 开始重放（否则要从头放）。由 `checkpointer` 进程做。触发：`checkpoint_timeout`（时间）或 `max_wal_size`（WAL 量）。
- **checkpoint 风暴**：一次刷太多脏页会 IO 尖峰 → `checkpoint_completion_target`（默认 0.9）把刷盘**摊平**到整个周期，避免尖峰。
- **`full_page_writes`（页断裂防护）**：OS 页 4KB、PG 页 8KB，掉电时可能只写了半个 PG 页（**torn page，页断裂**）。开启 `full_page_writes` 后，**每个 checkpoint 之后某页第一次被改时，把整页完整写进 WAL** → 恢复时用整页覆盖，修复断裂页。代价是 WAL 变大（写放大）——这条直接接 [PG WAL 与放大笔记](../../infra/postgresql-wal-replication-and-amplification.md)。

---

## 九、复制与高可用（考频 ★★★★★）

### Q28. PG 的物理复制（流复制）原理？

- **流复制（Streaming Replication）**：主库把 **WAL 记录**实时流式发给备库（standby），备库**持续重放 WAL** 保持同步。备库是主库的**物理字节级副本**（连块号都一样）。
- **角色进程**：主库 `wal sender` ⟷ 备库 `wal receiver` + `startup`（重放）进程。
- **热备（Hot Standby）**：备库在重放 WAL 的同时**可对外提供只读查询** → 读写分离。
- **对比 MySQL**：MySQL 主从传 **binlog（逻辑）**，从库用 SQL thread 回放；PG 传 **WAL（物理）**，物理级重放。物理复制**主备结构必须完全一致**（同版本、同架构），不能像逻辑复制那样只同步部分表。

### Q29. 同步复制 vs 异步复制？replication slot 是什么？

- **异步复制（默认）**：主库写完 WAL 本地落盘就返回客户端，不等备库 → **快，但主库宕机可能丢最后一点未传的 WAL**（类比 MySQL 异步复制、Redis 异步复制丢数据）。
- **同步复制**：`synchronous_commit = on` + 配 `synchronous_standby_names`，主库提交要**等至少一个备库确认收到 WAL** 才返回 → 不丢数据，但延迟高、备库挂了会**卡住主库提交**（要配多个备库或 quorum）。
  - `synchronous_commit` 还有细粒度：`remote_write`（备库收到）、`on`（备库落盘）、`remote_apply`（备库重放完，读一致但最慢）。
- **Replication Slot（复制槽）**：主库为每个备库/逻辑订阅维护一个「进度标记」，**保证备库还没消费的 WAL 不会被主库提前删掉/回收**。
  - **双刃剑**：备库长时间掉线，槽会**一直拦着 WAL 不让删 → 主库磁盘被 WAL 撑爆**。要监控 `pg_replication_slots` 的滞后，设 `max_slot_wal_keep_size` 兜底。

### Q30. 物理复制 vs 逻辑复制？各自场景？

| | 物理复制（流复制） | 逻辑复制（Logical Replication，10+） |
|--|-------------------|-------------------------------------|
| **传什么** | WAL 物理字节 | 解码 WAL 成**行级变更**（INSERT/UPDATE/DELETE） |
| **粒度** | 整个实例，全库全表 | **按表 / 发布(publication)** 选择性同步 |
| **版本/架构** | 必须完全一致 | 可**跨大版本、跨架构**（升级利器） |
| **备库可写** | 否（只读） | **是**（订阅端是独立可写库） |
| **典型场景** | 高可用 standby、读扩展、灾备 | 大版本升级、异构同步、数据分发、只同步部分表、CDC |

- **逻辑复制机制**：`CREATE PUBLICATION` on 主库 + `CREATE SUBSCRIPTION` on 从库；底层靠**逻辑解码（logical decoding）**把 WAL 翻译成变更流。表**必须有主键或 replica identity** 才能复制 UPDATE/DELETE。
- **类比**：逻辑复制类似 MySQL 的 **row-based binlog 复制**（都是行级变更）；物理复制没有 MySQL 直接对应物（更底层）。CDC 工具（Debezium）就是接 PG 的逻辑解码。

### Q31. PG 怎么做自动故障转移（failover）？

- **PG 核心不自带自动 failover**（不像 Redis Sentinel 内建）→ 靠外部工具编排：**Patroni**（最主流，配 etcd/Consul/ZooKeeper 做分布式选主）、repmgr、pg_auto_failover。
- **流程**：检测主库宕 → 从备库中选一个（复制最全的）→ `pg_promote()` 提升为主 → 其余备库 repoint 到新主 → 更新 VIP/服务发现指向新主。
- **脑裂防护**：靠 DCS（etcd 等）的分布式锁保证只有一个主，配 fencing（STONITH）。对照 Redis 脑裂（见 [redis 面试题库 Q41](../redis/redis-interview-questions-90-coverage-20260709.md)）——同样是「异步复制 + 分区」导致的经典问题。

---

## 十、数据类型、分区与生态（考频 ★★★，PG 加分项）

### Q32. JSONB 和 JSON 的区别？为什么面试爱问？

- **`json`**：原样存文本，保留空格/键序/重复键，每次查要重新解析 → 存得快、查得慢。
- **`jsonb`（推荐）**：存**解析后的二进制**，去重键、不保序 → 存稍慢，但**查询快、可建 GIN 索引、支持丰富操作符**（`@>` 包含、`->`/`->>` 取值、`?` 键存在）。
- **意义**：PG 用 jsonb 在关系库里干了半个文档数据库的活——**结构化字段用列、半结构化塞 jsonb**，还能给 jsonb 建 GIN 索引高效查询。这是「要不要上 MongoDB」时 PG 的有力回应。对照 [redis/NoSQL 选型](../redis/redis-interview-questions-90-coverage-20260709.md)。

### Q33. PG 的声明式分区表？和分库分表什么关系？

- **声明式分区（Declarative Partitioning，10+）**：`PARTITION BY RANGE/LIST/HASH`，一张逻辑父表 + 多个物理子分区。查询带分区键时**分区裁剪（partition pruning）**只扫相关分区。
- **典型**：按时间 RANGE 分区（每月一个分区），冷数据整分区 `DETACH`/`DROP`（比 `DELETE` 快无膨胀）。
- **和 MySQL 分区/分库分表的关系**：都是**单库内**的分区（垂直/水平切子表），**不解决单机容量/写扩展**——那要靠**分库分表 / Citus 扩展（把 PG 变成分布式）**。分区表 ≠ 分布式。

### Q34. PG 的扩展（Extension）生态举几个？

PG 的 `CREATE EXTENSION` 机制是它相对 MySQL 的大优势——把数据库当平台：
- **PostGIS**：地理空间之王（GIS 事实标准）。
- **pg_stat_statements**：SQL 级性能统计（慢查询分析必装）。
- **pg_trgm**：三元组模糊匹配，给 `LIKE '%x%'` 加 GIN/GiST 索引。
- **Citus**：把 PG 变成分布式（分片 + 并行），做 HTAP/大数据量。
- **TimescaleDB**：时序数据库扩展。
- **pg_repack / pgvector（向量检索，AI 火了之后爆红）** 等。

### Q35. 序列（Sequence）、SERIAL、IDENTITY？和 MySQL 自增区别？

- **PG 自增靠序列（sequence）对象**：`SERIAL`（老写法）/ `GENERATED ... AS IDENTITY`（SQL 标准，PG 10+ 推荐）底层都是一个独立的 sequence。
- **和 MySQL `AUTO_INCREMENT` 的差异**：
  - PG 序列是**独立对象**，可被多表共享、可 `nextval()` 手动取值、可设 cache/步长。
  - **序列不回滚**：事务回滚后已取的序列值**不归还**（会留空洞）→ 保证并发下唯一但不连续。MySQL 自增也类似（回滚不还）。
  - PG 序列**独立于表**，`TRUNCATE ... RESTART IDENTITY` 才重置。

---

## 十一、PG vs MySQL 综合对比（考频 ★★★★★，压轴题）

### Q36. PostgreSQL 和 MySQL 怎么选？核心差异总结？

**这是压轴题，把全篇串成一张表**：

| 维度 | PostgreSQL | MySQL(InnoDB) |
|------|-----------|---------------|
| **进程模型** | 多进程（连接贵，需连接池） | 多线程（连接轻） |
| **表组织** | 堆表 + 独立索引，**无聚簇索引** | 索引组织表，主键聚簇、二级索引回表 |
| **MVCC** | 多版本**存堆表**，xmin/xmax，产生死元组要 **VACUUM** | 旧版本存 **undo 版本链**，主表紧凑，purge 清理 |
| **默认隔离级别** | Read Committed | Repeatable Read |
| **可串行化** | **SSI**（乐观、并发好） | 2PL 加锁（并发差） |
| **索引类型** | B-tree/**GIN/GiST/BRIN**/Hash/SP-GiST，部分/表达式索引 | 主要 B+ 树、全文、空间（较弱） |
| **日志** | 单套 **WAL**（+ 逻辑解码） | **redo + binlog**，两阶段提交 |
| **复制** | 物理流复制 + 逻辑复制 | binlog 逻辑复制 |
| **SQL/功能** | 更强：CTE、窗口、SSI、丰富类型(JSONB/数组/范围)、扩展生态(PostGIS/Citus) | 简单够用，生态偏运维/中间件成熟 |
| **性能取向（各自快在哪）** | **复杂查询/分析、并行扫描、多索引合并、特殊索引加速** 更快；写密集/主键点查偏弱 | **简单 OLTP、主键点查（聚簇一次到行）、写/更新密集、海量连接** 更快 |
| **典型口碑** | 功能强、标准严、复杂查询/GIS/分析强 | 简单快、读多写少互联网场景、生态成熟运维熟 |

> ⚠️ **「PG 比 MySQL 快」是伪命题，别踩坑。** 没有谁「普遍更快」，只有「某类负载下更快」，而两者的架构恰好各自优化了不同场景：
> - **InnoDB 更快**：主键点查（**索引组织表**，一次索引遍历直接拿整行；PG 要「索引→TID→访问堆」两步）；写/更新密集（**就地更新**，PG 的 update 造死元组 + 非 HOT 更新所有索引 + 靠 VACUUM 兜底，见 Q9）；海量连接（线程轻，PG 一连接一进程）。
> - **PG 更强**：复杂查询（优化器更成熟）、分析型大扫描（并行查询 9.6+ 成熟得早）、一条查询用多索引（**Bitmap Scan** 位图 AND/OR，见 Q18）、能索引 MySQL 索引不了的东西（GIN/GiST/BRIN，见 Q14/Q38）、强一致高并发（SSI，见 Q24）。
> - **面试话术**：「PG 不是普遍更快——**简单 OLTP、主键点查、写密集 InnoDB 常更快；复杂查询、分析、特殊索引、强一致 PG 更强**。选型看负载，不看‘谁快’。」比盲目说「PG 快」高一个段位。

**选型口诀**：
- **选 PG**：复杂查询/分析、GIS、需要 JSONB/数组/丰富类型、要强一致可串行化、要扩展生态。
- **选 MySQL**：互联网 CRUD、读多写少、团队熟、中间件（分库分表/ProxySQL）生态成熟、简单可控。

### Q37. 一句话回答「PG 最大的运维坑是什么」？

**VACUUM / 表膨胀 / XID 回卷** —— 这是 PG 的 MVCC 把旧版本留堆里换来的代价，也是它区别于 MySQL 的运维核心。答不出 vacuum，就说明没真正用过 PG。（回挖第四章）

### Q38. 为什么 PG 能支持那么多索引类型（GIN/GiST/BRIN…），MySQL 却不能？

**核心：PG 把索引做成了「实现一个抽象接口就能挂载的插件」。** 两个根因——一个架构、一个血统。

**根因 1（架构）：堆表 + 索引解耦 + 抽象的「访问方法」接口。**
- **堆表让索引彻底解耦**：PG **没有聚簇索引**（见 Q3），所有索引一视同仁——**任何索引的职责都只是「把某种 key 映射到一批 TID」**，不负责存行、不和表存储绑定。对比 InnoDB：**表本身就是主键 B+ 树**，存储引擎和 B+ 树深度耦合，想换索引结构等于重写引擎。
- **可插拔的索引访问方法（Access Method，`pg_am`）**：PG 定义了一套抽象接口（插入、扫描、代价估算……），**任何索引类型只要实现这套接口就能挂进来**——B-tree/GIN/GiST/BRIN/SP-GiST 都是它的实现。尤其 **GiST 是「通用框架」**：你只要提供几个算子函数（`consistent`/`union`/`penalty`/`picksplit`），就能造出一整个新索引类型 → PostGIS 的空间索引、pg_trgm 的模糊匹配都这么加的。
- **可扩展的类型 + 操作符 + 操作符类**：新数据类型能声明「我用哪个索引、支持哪些操作符」（`operator class`），索引与类型解耦到底。

**根因 2（血统/哲学）：它生来就是「可扩展研究数据库」。** PG 前身是 Berkeley 的 **POSTGRES（Stonebraker）**，一个**以「可扩展 / 对象关系」为立项目标**的研究项目——用户自定义类型、操作符、访问方法从第一天就是设计目标，**扩展性刻在 DNA 里**。MySQL 出身是「快而简单的 Web 数据库」，存储引擎单体、索引类型写死（B+ 树 + FULLTEXT/SPATIAL 特例），用户无法插新索引类型。

> **一句话**：PG 索引多，是因为「堆表让索引和存储解耦」+「生来为扩展而设计」→ 索引成了可插拔插件。这也直接决定了下一题。

### Q39. PG 凭什么能变身时序库（TimescaleDB）、向量库（pgvector）？

**它们不是「PG 内置了 TSDB/向量」，而是 Q38 那套可扩展机制再往上一层——`CREATE EXTENSION` 的产物。** 扩展机制能加载动态库，往库里注入**新类型 + 新操作符 + 新索引访问方法 + 后台进程 + planner 钩子**。

- **pgvector（向量检索，RAG/AI 爆红）** —— 恰好用满三个可扩展点：
  1. **新类型** `vector`（可扩展类型系统）；
  2. **新操作符** `<->`(L2)/`<=>`(余弦)/`<#>`(内积距离)（可扩展操作符）；
  3. **新索引 AM** —— **IVFFlat / HNSW** 近似最近邻（ANN）索引，**正是「可插拔索引访问方法」的兑现**：有人把 HNSW 算法实现成一个 PG 索引类型，它只需「向量→TID」，不碰堆存储。
- **TimescaleDB（时序）**：**Hypertable** 按时间自动分区成 chunk（复用 PG 分区 + 后台 worker + planner 钩子）+ 原生压缩、连续聚合、保留策略；时序数据物理有序 → **BRIN 索引**天生契合（几 MB 索引上百 GB，见 Q14）——又绕回 Q38 的索引优势。
- **诚实的 nuance（面试加分）**：pgvector 在亿级向量/极致 QPS 下打不过专用向量库（Milvus/Pinecone），TimescaleDB 在极端写入量下也不如专用 TSDB。PG 的价值主张不是「每项都最强」，而是 **「一个库搞定 80% 场景，省掉引入+运维+数据同步一套新系统的成本」**——业务数据、向量、时序放同一个事务里，这才是杀手锏。

> **因果链一句话收尾**：`POSTGRES 为扩展而生 + 堆表让索引解耦 → 抽象索引访问方法(pg_am) → 索引可插拔(GIN/GiST/BRIN) → CREATE EXTENSION 注入新类型/操作符/索引AM → pgvector/TimescaleDB/PostGIS`。所以 PG 相对 MySQL 的真优势**不是「更快」，而是「更能长」**——一个 PG 当半个专用数据库用。

---

## 附录：自我思考

### 这篇的定位与局限

- 广度优先的抽考清单，目标「面试前快速捞回 PG 90% 考点」，体例对齐 [mysql](../mysql/mysql-interview-questions-90-coverage-20260704.md) / [redis](../redis/redis-interview-questions-90-coverage-20260709.md) 两篇。全篇刻意**以「和 MySQL 的差异」为轴**，因为面试官问 PG 基本都在这条线上追。
- 以下几处还停在「结论层」，符合 depth contract 应单独成篇 + 动手：
  - **MVCC + VACUUM（Q7/Q10/Q12）** 最该起 Docker 实测：造死元组 → 看 `pg_stat_user_tables` 的 `n_dead_tup` 膨胀 → 手动 vacuum → 用 `pgstattuple` 看膨胀率；再造长事务卡住 vacuum，观察 `age(relfrozenxid)` 逼近回卷。这条直接接隔壁 [PG WAL 笔记](../../infra/postgresql-wal-replication-and-amplification.md)，能织成「PG 存储引擎」完整一条线。
  - **索引家族（Q14）** 值得每种建一遍看 EXPLAIN：GIN 索引 JSONB `@>`、BRIN 索引时序大表看体积对比、部分索引/表达式索引实测走没走。
  - **SSI（Q24）** 可以用两个并发会话复现 write skew：RR 下两事务都提交造成「无人值班」，切 Serializable 看其中一个被 abort。把「序列化异常」从背变成看到。
  - **Bitmap Scan（Q18）** 造中等选择性查询，看优化器从 Index Scan → Bitmap Heap Scan → Seq Scan 的切换阈值。

### 与能力地图的挂钩

- 本篇覆盖第 4 格「关系型数据库」的 PG 侧 + 第 7 格「存储引擎原理」（堆表 MVCC/WAL/放大接 ../infra/）。和 MySQL 篇是**对照双子篇**：同一格、两种实现哲学（索引组织表 vs 堆表、undo 链 vs 堆内多版本、redo+binlog vs 单 WAL、2PL 可串行化 vs SSI）。
- **交叉链接**：B+ 树/树结构接 [四棵树对比](../mysql/tree-structures-bplus-btree-rbtree-20260708.md)；WAL/放大接 [PG WAL 笔记](../../infra/postgresql-wal-replication-and-amplification.md)；复制脑裂/异步丢数据接 [redis 面试题库 Q41](../redis/redis-interview-questions-90-coverage-20260709.md) 和 [mysql 主从复制](../mysql/mysql-replication-master-slave-standby-consistency-20260709.md)；JSONB vs 文档库选型接 NoSQL（格 5）；Citus/分库分表接分布式（格 8）。
- **暂缺**：Citus 分布式细节、pgvector 的 IVFFlat/HNSW 建索引实操、PG 性能压测实操——按需再补（可扩展机制的「为什么」已在 Q38/Q39 讲透）。

### 复习调度

按 CLAUDE.md 第五节：本篇**「进行中」**（还没动手验证，不能算毕业）。
- **+1 天（07-10）**：盖答案自测 30 秒电梯版 + Q7/Q10/Q12/Q22/Q24/Q36/Q38/Q39 八道核心题。
- **动手项**：起 `docker run postgres:16`，跑通 Q10~Q13 的死元组膨胀 + 长事务卡 vacuum 复现，和 Q24 的 write skew 复现，补进正文才升级为「毕业」。

### 自测题（盖住上面的答案，先自己讲一遍）

1. PG 是多进程还是多线程？为什么必须配连接池？
2. PG 有聚簇索引吗？表怎么组织？索引叶子存什么？和 InnoDB 回表有何区别？
3. PG 的 MVCC 和 MySQL 本质区别是什么？xmin/xmax 怎么工作？为什么 PG 的 update 会产生死元组？
4. VACUUM 做哪四件事？VACUUM 和 VACUUM FULL 区别？autovacuum 何时触发？
5. XID 回卷是什么？为什么会让库停摆？靠什么防？长事务为什么是头号敌人？
6. PG 索引类型报菜名：GIN/GiST/BRIN 各适合什么？HOT update 怎么省索引写？
7. Bitmap Heap Scan 是什么、解决什么问题？Index-Only Scan 为什么还依赖可见性位图？
8. PG 默认隔离级别是什么（和 MySQL 差在哪）？RR 为什么天然无幻读、不用间隙锁？代价是什么？
9. SSI 怎么做到可串行化又不重锁？和 MySQL 的 Serializable 差在哪？
10. PG 的 WAL 对应 MySQL 的什么？为什么 PG 没有 binlog、没有两阶段提交那套？full_page_writes 防什么？
11. 物理复制 vs 逻辑复制各是什么、什么场景？replication slot 的双刃剑是什么？
12. PG vs MySQL 核心差异用一张表答出来；各自什么场景选哪个？「PG 比 MySQL 快」为什么是伪命题？各自快在哪？
13. PG 最大的运维坑是什么？
14. 为什么 PG 能支持那么多索引类型、MySQL 不能？（堆表解耦 + 可插拔访问方法 + 血统）
15. PG 凭什么能变身 tsdb（TimescaleDB）/向量库（pgvector）？扩展机制注入了哪三样东西？

答不出的，回对应 Q 挖。
