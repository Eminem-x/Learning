# MVCC × 隔离级别 × 幻读：从原理到亲手复现

> 来源：个人思考 + 学习（对话式推演 + Docker 实验复现）
> 收录时间：2026-07-10 01:20 CST
>
> 能力地图定位：格 4「关系型数据库」——事务·隔离级别·MVCC·锁。
> 关联笔记：[三大日志×ACID 动手实验台](./mysql-logs-lab/README.md)、[mysql 面试题库(广度)](./mysql-interview-questions-90-coverage-20260704.md)、[postgresql 面试题库(广度)](../postgresql/postgresql-interview-questions-90-coverage-20260709.md)（堆表 MVCC 对照）

---

## 30 秒电梯版（盖住全文也能讲清主线）

- **MVCC** = 多版本并发控制：写不覆盖旧值，保留多版本；读时按「可见性规则」挑一个历史版本读 → **读不加锁，读写不互堵**。用空间换并发。
- **InnoDB 实现三件套**：行内隐藏字段（`DB_TRX_ID` + `DB_ROLL_PTR`）+ undo log 串成**版本链** + **ReadView** 判可见性。
- **四种隔离级别**从松到严：读未提交 < 读已提交 < 可重复读 < 串行化，每升一级多堵一类异常（脏读→不可重复读→幻读）。
- **RC vs RR 的唯一区别**：ReadView「每次快照读新建」(RC) vs「事务内第一次建、之后复用」(RR)。
- **RR 防幻读靠双防线**：快照读靠 **MVCC**、当前读靠 **间隙锁 / Next-Key Lock**。
- **缝在哪**：先快照读、再对幻影行做当前读（UPDATE），幻读会复现——所以 InnoDB RR 也**不能 100%** 消除幻读，只有 SERIALIZABLE 彻底干净。

---

## 一、MVCC 是什么，解决什么问题

不用 MVCC 的朴素做法：读写都加锁 → **读写互斥**，一个大查询堵死所有写，并发极差。

MVCC 的核心洞察：**读的是「过去某个时刻的快照」，快照是不变的历史，所以不需要锁。** 写只管往前追加新版本，读各自去翻自己该看的那一版。

| | 传统锁 | MVCC |
|---|---|---|
| 读-读 | 不冲突 | 不冲突 |
| 读-写 | **互斥（堵）** | **不堵**（读旧版本） |
| 写-写 | 互斥 | 互斥（仍需锁） |

一句话：**用空间换并发——多存几份版本，换来「读不堵写、写不堵读」。**

---

## 二、InnoDB 怎么实现（面试重点）

### 1. 每行的隐藏字段
- `DB_TRX_ID`：最后一次修改这行的**事务 ID**
- `DB_ROLL_PTR`：**回滚指针**，指向 undo log 里这行的**上一个版本**
- `DB_ROW_ID`：隐藏主键（无显式主键时用）

### 2. undo log 串成版本链
每次更新，旧值写进 undo log，`DB_ROLL_PTR` 把它们串成**从新到旧的链表**：

```
最新版本(trx 50) ──roll_ptr──▶ 旧版本(trx 30) ──roll_ptr──▶ 更旧(trx 10)
```

### 3. ReadView 决定「我该看哪一版」
快照读时生成 ReadView，核心字段：
- `m_ids`：生成 ReadView 那一刻**还活跃（未提交）的事务 ID 列表**
- `min_trx_id`：m_ids 里最小的
- `max_trx_id`：下一个要分配的事务 ID
- `creator_trx_id`：当前事务自己的 ID

**可见性判断**（拿版本链上每个版本的 `DB_TRX_ID` 去比）：
- `trx_id < min_trx_id` → 快照前就提交了，**可见**
- `trx_id ≥ max_trx_id` → 快照后才开始的，**不可见**，顺 roll_ptr 找更旧版本
- `min ≤ trx_id < max`：
  - 在 `m_ids` 里 → 当时未提交，**不可见**，找更旧的
  - 不在 `m_ids` 里 → 当时已提交，**可见**

**沿版本链从新往旧走，找到第一个「对我可见」的版本就读它。**

---

## 三、四种隔离级别与三类异常

### 严格度顺序（从松到严 = 并发从高到低）

```
读未提交 < 读已提交 < 可重复读 < 串行化
```

### 逐级消除三类读异常

| 隔离级别 | 脏读 | 不可重复读 | 幻读 |
|---|:---:|:---:|:---:|
| 读未提交 RU | ❌会发生 | ❌ | ❌ |
| 读已提交 RC | ✅挡住 | ❌ | ❌ |
| 可重复读 RR | ✅ | ✅挡住 | ❌标准 / InnoDB 基本挡住 |
| 串行化 | ✅ | ✅ | ✅挡住 |

### 三类异常辨析

| | 读到的是 | 变化的是 | 罪魁操作 | 数据是否真实 |
|---|---|---|---|---|
| **脏读** | 未提交的数据 | — | 别的事务**未提交** | ❌ 可能被回滚 |
| **不可重复读** | 已提交的数据 | 某**一行的值** | 别人 **UPDATE** 提交 | ✅ 真实 |
| **幻读** | 已提交的数据 | 结果集的**行数** | 别人 **INSERT/DELETE** 提交 | ✅ 真实 |

- 脏读 = 读到**假数据**（未提交）
- 不可重复读 = 同一行**值变了**（UPDATE）→ 行锁可防
- 幻读 = 同一范围**行数变了**（INSERT/DELETE）→ 行锁无效，需**间隙锁**锁住「行与行之间的间隙」

> 为什么防幻读比防不可重复读「贵」：不可重复读锁住已存在的行即可；幻读要锁的是「原本不存在的新行」——只能锁范围/间隙。

### RC vs RR 的底层唯一区别

| 隔离级别 | ReadView 生成时机 | 效果 |
|---|---|---|
| **RC** | **每次快照读都新建** | 每次看到别人最新提交 → 有不可重复读 |
| **RR** | **事务内第一次快照读时建，之后复用** | 整个事务同一快照 → 可重复读 |

### 各库默认级别

| 数据库 | 默认 |
|---|---|
| MySQL (InnoDB) | 可重复读 RR |
| PostgreSQL / Oracle / SQL Server | 读已提交 RC |

---

## 四、RR 防幻读的「双防线」

InnoDB 的 RR 防幻读靠**两条腿**，对应两种读：

| 读类型 | 例子 | 靠什么防幻读 |
|---|---|---|
| **快照读** | 普通 `SELECT` | **MVCC / ReadView**（读历史快照，新 INSERT 进不来） |
| **当前读** | `UPDATE` / `DELETE` / `SELECT ... FOR UPDATE` / `LOCK IN SHARE MODE` | **间隙锁 / Next-Key Lock**（锁住范围，别人 INSERT 被阻塞） |

**缝隙**：先快照读、再对「快照里看不见的幻影行」做当前读 UPDATE，会把它拉进自己的快照 → 幻读复现（见下节实验二）。所以严格说 InnoDB RR 也不能 100% 消除幻读。

---

## 五、动手验证（Docker 复现，两个实验一正一反）

### 实验一（正）：快照读下 MVCC 挡住幻读 → 还是 2

```
Session B                          Session A
BEGIN;
SELECT count(*) → 2   ← 第一次快照读，建 ReadView（RR 复用它）
                                   INSERT 一行; COMMIT;   (trx_id 落在不可见区间)
SELECT count(*) → 还是 2  ✅ 新行对 B 永远不可见
```

结论：InnoDB RR 的**快照读**用 MVCC 挡住了幻读——所以看到 2，不是 3。这一步验证了「RR 无法避免幻读」是**标准 SQL 定义**，InnoDB 比标准更强。

### 实验二（反）：加一步当前读，幻读现形 → 变 3

```
Session B                          Session A
BEGIN;
SELECT → 2                          INSERT (id=3); COMMIT;
SELECT → 还是 2   (快照读，MVCC 挡住)

UPDATE t SET xxx=xxx WHERE id=3;   ← 当前读！读最新版本，
                                     把 id=3 这行的 DB_TRX_ID 改成 B 自己的事务号
SELECT → 3  ⚠️ 幻影出现
```

**机制钉死**：UPDATE 那行的瞬间，把幻影行的 `DB_TRX_ID` 改成了自己的事务号 → 这行从此对当前事务可见 → 后续快照读就能看到它。**幻读的本质是「自己的写，让别人的幻影行入了自己的快照」。**

> 两个实验都已本人在 Docker 环境复现（对上号：正=2、反=3）。

---

## 六、易被追穿的补充细节

### 1. ReadView 建在「第一次快照读」，不是 BEGIN
```sql
BEGIN;                    -- 此刻还没有快照！
-- 别的事务在这里 INSERT+COMMIT → 会被算进快照
SELECT ...;               -- ← 快照从这一刻才冻结
```
想让快照严格从事务开始那刻起：
```sql
START TRANSACTION WITH CONSISTENT SNAPSHOT;   -- 立即建 ReadView
```
**这是复现实验最易踩的时序坑**：先 BEGIN 没 SELECT，对方 INSERT 提交，你第一次 SELECT 就把它算进快照了，现象和预期对不上。

### 2. RC 下默认没有间隙锁
RC 只加记录锁、**基本不加间隙锁** → RC **压根不防幻读**（连当前读也不防）。这是 RC 并发更高但更弱的原因。
> 实践取舍：很多互联网公司（阿里系）线上默认用 **RC** 而非 InnoDB 默认的 RR——没间隙锁 → **死锁更少、锁范围更小、并发更好**，幻读在业务层兜。高频追问「你们为什么改默认级别」。

### 3. SERIALIZABLE = 所有读都加锁的当前读
SERIALIZABLE 给普通 SELECT 也自动加锁（`LOCK IN SHARE MODE`），**取消 MVCC 快照读**，所有读都成当前读 → 读写完全串行、幻读彻底消失，代价是并发几乎归零。

### 4. 半一致读（Semi-consistent read）
RC 下 `UPDATE ... WHERE` 扫描时，对**不满足条件**的行「读最新已提交版本，不匹配就放行不加锁」，减少 UPDATE 锁冲突。偏门但答上是加分项。

---

## 七、查看与修改隔离级别（速查）

```sql
-- MySQL 8.0+
SELECT @@transaction_isolation;          -- 会话级（不带 global 是会话）
SELECT @@global.transaction_isolation;   -- 全局级
-- MySQL 5.7-：@@tx_isolation

-- 合法级别名只有四个，两词间空格，无逗号无引号：
SET SESSION TRANSACTION ISOLATION LEVEL READ UNCOMMITTED;
SET SESSION TRANSACTION ISOLATION LEVEL READ COMMITTED;
SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ;
SET SESSION TRANSACTION ISOLATION LEVEL SERIALIZABLE;
-- 坑：READ 只跟 UNCOMMITTED/COMMITTED；REPEATABLE 只跟 READ。
--     "REPEATABLE COMMITTED" 是非法拼接 → ERROR 1064。
-- SET GLOBAL 只对之后的新连接生效，当前连接要用 SET SESSION。
```

```sql
-- PostgreSQL
SHOW default_transaction_isolation;   -- 新事务默认
SHOW transaction_isolation;           -- 当前事务实际（事务块内才有意义）
```

---

## 八、白板自测题（盖住答案自己讲一遍）

1. MVCC 为什么能「读不加锁」？牺牲了什么换并发？
2. InnoDB 实现 MVCC 靠哪三样？版本链怎么串？
3. ReadView 可见性算法：拿 `trx_id` 和哪几个边界比？各自结论？
4. 四种隔离级别严格度顺序？每级多堵哪类异常？
5. 脏读 / 不可重复读 / 幻读，变化的分别是什么、罪魁分别是哪个操作？
6. RC 和 RR 在 MVCC 层面唯一的区别是什么？
7. InnoDB RR 防幻读的两条防线分别对应什么读、靠什么机制？
8. 为什么实验一看到 2 不是 3？加一步 UPDATE 后为什么变 3？机制是什么？
9. ReadView 到底哪一刻建？怎样让它从事务开始那刻建？
10. 为什么有的公司线上把默认级别从 RR 改成 RC？
11. SERIALIZABLE 是怎么实现「无幻读」的？

---

## 附录：自我思考

- **这次学习的价值不在结论，在「一正一反两个实验对上号」。** 「RR 能不能防幻读」这种问题，只背结论必被追穿——因为标准定义、InnoDB 快照读、InnoDB 当前读三个语境下答案不同。亲手复现「快照读=2、当前读后=3」，才真正把「快照读/当前读双防线」这个知识结构焊死。
- **踩坑记录**：一开始困惑「INSERT+COMMIT 后为什么还是 2，不该是 3 吗」——这正是把「书上说 RR 不防幻读」错误外推到 InnoDB 快照读的典型误区。修正点：那句话是标准 SQL 定义，InnoDB 比标准强；InnoDB 的幻读只在「当前读」的缝里出现。
- **方法论审视**：这块知识（MVCC + 隔离级别 + 三类异常 + 锁）是强耦合的一张网，散着记必然混。以后复习先过 30 秒电梯版，卡在哪一环就回挖对应小节。
- **延伸钩子**：锁的具体类型（记录锁 / 间隙锁 / 临键锁 / 插入意向锁）还没系统展开，是这条线的自然下一站；PG 的堆表 MVCC（死元组 + VACUUM + XID 回卷）与 InnoDB undo log 方案的对比，接 postgresql 笔记那条线。

---

## 毕业标准

- [ ] 能白板画出「版本链 + ReadView 可见性算法」，并口算一个 `trx_id` 该不该可见。
- [ ] 能不看笔记复现实验二（当前读触发幻读），并解释 `DB_TRX_ID` 被改写的机制。
- [ ] 能对「RR 到底防不防幻读」分「标准 / 快照读 / 当前读」三个语境分别作答并举反例。
- [ ] 能说清 RC vs RR 的实现差异（ReadView 时机）与 RC 无间隙锁的工程取舍。

**复习调度**：+1 天（07-11）/ +7 天（07-17）/ +30 天（08-09）各回看自测题一次。
