# MySQL 三大日志 × ACID 动手实验台（redo / undo / binlog）

> 来源：个人思考 + 学习（配套可运行 Docker 实验）
> 收录时间：2026-07-08 CST
> 对标能力格：#4 关系型数据库 —— 事务 / MVCC / 三大日志
> 配套代码：本目录 `docker-compose.yml` + `exp1~3` + `lab.sh`

---

## 30 秒电梯版（盖住全文也要能背出来）

- **ACID 是目的，三大日志是手段，别横向背、要竖着挂。**
- **undo log**：存「改之前的旧版本(before image)」。兜 **A 原子性**（回滚=反向重放 undo）+ **I 隔离性**（MVCC 快照读顺着 undo 版本链找旧版本）。
- **redo log**：InnoDB 引擎层、**物理**日志、**循环写**。兜 **D 持久性**（WAL：先写 redo 再改页，崩了重放已提交事务）。参数 `innodb_flush_log_at_trx_commit` 决定它多勤快刷盘。
- **binlog**：Server 层、**逻辑**日志、**追加写**。**不属 ACID**，管**主从复制 + 时间点恢复(PITR)**。参数 `sync_binlog` 决定刷盘频率。
- **redo 与 binlog 用两阶段提交(2PC)绑定**：redo(prepare) → 写 binlog → redo(commit)。保证「引擎里提交了」和「从库能收到」两件事崩溃时也一致。**「双一」= `innodb_flush_log_at_trx_commit=1` + `sync_binlog=1`。**

---

## 一、为什么你背不下来（先修好心智模型）

平铺记「redo 是重做、undo 是回滚、binlog 是归档」——三个孤立名词，考场必忘。
真正的结构是**两层**：

```
        目的：ACID
   ┌──────┼──────┬──────────┐
   A      I      D          C（结果，非单一日志）
   │      │      │
 undo   undo   redo         ← 手段：日志
                │
              binlog（另一维度：跨引擎复制/PITR，不在 ACID 里）
                │
        redo ↔ binlog 靠 2PC 对齐
```

| ACID | 谁兜底 | 物理直觉 | 对应实验 |
|:--|:--|:--|:--|
| A 原子性 | undo | 回滚 = 拿 before-image 反向重放 | exp1 幕一 |
| I 隔离性(MVCC) | undo | 快照读沿 undo 版本链找可见旧版本 | exp1 幕二 |
| D 持久性 | redo | WAL，崩溃后重放已提交事务 | exp2 |
| C 一致性 | 以上全部 + 约束 | 是结果，不是某个 log 单独负责 | — |
| 复制/PITR（非 ACID） | binlog | 逻辑变更流喂给从库 / 恢复到某时刻 | exp3 |

### redo vs binlog 一张对比表（高频追问）

| 维度 | redo log | binlog |
|:--|:--|:--|
| 层级 | InnoDB **引擎层** | MySQL **Server 层** |
| 内容 | **物理**：某页某偏移改成啥 | **逻辑**：ROW 模式记行前后镜像 / STMT 记语句 |
| 写法 | **循环写**，写满覆盖旧的 | **追加写**，写满换下一个文件，永久保留 |
| 作用 | 崩溃恢复（**只对本机 InnoDB**） | 复制、PITR（**跨引擎、跨主机**） |
| 有无 | InnoDB 独有 | 所有引擎、Server 级都有 |

### 一-A、WAL：redo 背后的上位原理（redo = InnoDB 版的 WAL）

> exp2「崩了还能恢复」的底层就是 **WAL（Write-Ahead Logging，预写日志）**。它不是 InnoDB 专有名词，是数据库通用手艺；redo log 只是 InnoDB 对它的实现。

**一句话原理：改数据页之前，先把「要怎么改」写进日志并落盘；数据页本身可以晚点再刷。核心四字——日志先行。**

**它破的是什么矛盾**：既要「提交后断电不丢」（要刷盘），又要「快」（数据页随机分散、逐次刷盘慢到没法用）。WAL 的破局：

```
朴素做法：改内存页 → 立刻把数据页【随机写】盘        （慢，随机 IO 满盘乱跳）
WAL 做法：改内存页 → 只把 redo【顺序追加】写盘 → 数据页留内存慢慢批量刷（快，顺序 IO）
```

**为什么就快了**：日志是「顺序追加」（永远写在文件末尾），数据页是「随机分散」。WAL 用 **1 次便宜的顺序日志写**，换掉 **N 次昂贵的随机数据页写**。所以 `innodb_flush_log_at_trx_commit=1` 提交时 **fsync 的是 redo，不是数据页**——这正是 exp2 幕一「脏页没落盘也不怕」的原因。

**崩了怎么恢复**：提交时数据页可能还在内存（脏页），断电就没了；但 **redo 已落盘** → 重启扫 redo，把「已提交但数据页没落盘」的改动**重新做一遍（redo=重做）** → 数据回来。WAL 的持久性保证浓缩成一句：**只要日志落了盘，数据就等于持久了——数据页还在内存飘也能靠日志重建。**

**两条铁律（WAL 协议本质）**：
1. 改数据页之前，对应 redo 必须**先**落盘（Write-**Ahead** 字面意思）。
2. 事务提交时，它的 redo 必须落盘——否则提交不算数。
   → `trx_commit=0`（exp2 幕二）正是**违反第 2 条**：提交时 redo 没落盘 → 崩了无日志可重放 → 丢数据。**exp2 本质就是一个「WAL 保证开/关」的对比实验。**

**配套概念**：脏页（改过没刷盘的页，WAL 允许它大量堆内存慢慢刷）；检查点 checkpoint（把某位置前的脏页刷盘，之后那段 redo 就可被循环覆盖 / 恢复时不用看，所以 redo 能循环写而不无限涨）。

**不是 InnoDB 专利**：InnoDB→redo log；**PostgreSQL 直接就叫 WAL**（见 `../../infra/postgresql-wal-replication-and-amplification.md`）；SQLite 有 WAL 模式；etcd/ZooKeeper 的共识日志同理。原理都一样：日志先行、顺序写、崩溃重放。

---

## 二、跑起来（5 分钟）

```bash
open -a Docker                 # 先确保 Docker 在跑
cd mysql-logs-lab
chmod +x *.sh
./lab.sh up                    # 起主(3316)从(3317)两个 MySQL 8

./exp1-undo.sh                 # A + I：看 undo 兜原子性和 MVCC
./exp2-redo-crash.sh           # D：崩溃恢复；并亲手看 trx_commit=0 丢数据
./exp3-binlog.sh               # 复制/PITR：写主库看从库跟上，解码 binlog

./lab.sh status                # 随时看 LSN / History list
./lab.sh down                  # 收工（连数据一起删）
```

> `lab.sh psh` 开交互 shell；演 MVCC(exp1 幕二)要开**两个终端**各一个会话。

---

## 三、实验各自证明了什么

- **exp1（undo）**：`UPDATE` 后 `ROLLBACK`，值回到原样 —— A。两个会话在 RR 下，B 改了并提交，A 在同一事务里**仍读到旧值** —— 这只能靠 undo 版本链，就是 MVCC 的 I。记牢「undo 不止回滚，还供 MVCC」。exp1 脚本尾部还带一段**长事务撑大 undo** 的进阶演示，见下面「三-A」。
- **exp2（redo）**：三幕看透 `innodb_flush_log_at_trx_commit`。幕一 `=1` 提交后 `kill -9` 重启数据仍在（提交时 redo 已 fsync，WAL 重放）；幕二 `=0` 提交后立刻崩 —— 数据**丢了**；幕三 `=2` 同样立刻崩 —— 数据却**不丢**。你会**亲眼**看到这个参数是「持久性 ↔ 性能」的旋钮，以及 `=0` 与 `=2` 的差别恰恰只在「进程崩」这种崩法上体现。详见下面「三-B：exp2 翻车复盘」。
- **exp3（binlog）**：主库写、从库自动跟上（binlog→relay log→重放）；`mysqlbinlog` 把日志解码成逻辑行事件；`SET sql_log_bin=0` 写的行从库**永远收不到** —— 证明 binlog 是复制的唯一信息源，且和 redo 完全是两回事。

> 复制的**用途辨析（主从/主备/读写分离）+ 一致性光谱（异步/半同步/MGR）+ "收到≠重放"陷阱**,已单独成篇:[主从复制·一致性光谱](../mysql-replication-master-slave-standby-consistency-20260709.md)。本 lab 给"binlog 是复制信息源"的手感,那篇讲"这套复制怎么被用、一致性怎么权衡"。

---

## 三-B、exp2 翻车复盘：为什么 `=0` 一开始「没丢数据」

> 真实踩坑记录。第一次跑 exp2 幕二（`trx_commit=0`），期望丢数据，结果 `a/b/c` 三行**全在**。不是脚本写错，是这个实验**天生带时序竞态**。复习价值极高，专门记下来。

### `innodb_flush_log_at_trx_commit` 三值语义（务必分「进程崩 / 掉电崩」两种崩法）

| 值 | commit 那一刻做什么 | 后台每秒 | 扛得住 `kill -9`（**进程崩**）？ | 扛得住掉电 / OS 崩？ |
|:-:|---|---|:-:|:-:|
| **1** | write + **fsync** | — | ✅ | ✅ |
| **2** | 只 write 到 **OS page cache** | fsync | ✅（数据在内核，进程死不丢） | ❌ 丢 ≤1s |
| **0** | **啥都不做**，redo 停在 InnoDB **log buffer（进程内存）** | write+fsync | ❌ 丢 ≤1s | ❌ 丢 ≤1s |

### 翻车根因：后台线程「抢先」刷了 redo

`=0` 下 redo 由后台 master 线程**每 `innodb_flush_log_at_timeout` 秒（默认 1s）**刷一次盘。脚本从「提交」到「`docker kill`」要经过 docker exec、mysql client 启动等开销，**很容易 >1s** → 后台已抢先把 redo 刷盘 → 重启重放 → 数据还在，看不到丢失。`=0` 保证的是「**不承诺**持久」，不是「**保证**丢失」，那 <1s 的窗口没踩中就不丢。

### 修法：把丢失窗口人为放大到可控

```sql
SET GLOBAL innodb_flush_log_at_timeout = 30;   -- 后台 30s 才刷，我们几秒内 kill 必落窗口内
```

拉长后台刷盘周期到 30s，`=0` 丢数据几乎必现。不是作弊，是把竞态窗口放大到可观测。（新版脚本幕二已内置。）

### 关键澄清：`docker kill` 只能模拟「进程崩」，不能模拟「掉电」

`docker kill` = 给容器主进程 mysqld 发 SIGKILL，**宿主机内核和它的 page cache 毫发无损**。所以：
- 它能区分 **`=0` 扛不住进程崩**（redo 在进程内存，随进程死）vs **`=2` 扛得住进程崩**（redo 已在内核 page cache）——这正是新增**幕三**的意义。
- 它**无法**复现掉电 / OS 崩，那种场景下 `=2` 也会丢 ≤1s。本 lab 对这一层只能语义推演，别把结论说过头。

**一句话收束**：exp2 幕二(`=0` 全丢) vs 幕三(`=2` 不丢) 的差别，**只体现在「进程崩」这种崩法**上；`=1` 才是两种崩法都扛得住的唯一选项。

---

## 三-A、从 exp1 延伸：MVCC 的三个配角 —— Read View / DB_ROLL_PTR / purge，与长事务危害

exp1 幕二证明了「A 重复读到旧值」，但要讲透**为什么能读到、旧值堆到什么时候**，得把三个隐藏角色摆出来。这也是面试从「MVCC 是什么」追到「长事务为什么危险」的完整链路。

### 1. MVCC 快照读的完整机制（把幕二讲到底）

InnoDB 每行带三个隐藏字段，其中两个是 MVCC 的钥匙：

| 隐藏字段 | 作用 |
|:--|:--|
| `DB_TRX_ID` | 最后修改这行的事务 ID |
| `DB_ROLL_PTR` | **回滚指针**，指向该行在 undo log 里的上一个版本，把「当前行」和「历史版本」串成版本链 |

B 执行 `UPDATE balance=500` 时，不是简单覆盖：① 把旧值 100 的 before-image 写进 undo；② 数据页改成 500；③ 该行 `DB_ROLL_PTR` 指向刚写的 undo 记录；④ `DB_TRX_ID` 改成 B。于是内存里形成一条链：

```
数据页当前行                       undo log 历史版本
┌──────────────────┐            ┌──────────────────┐
│ balance = 500    │            │ balance = 100    │
│ DB_TRX_ID = B    │ ─ROLL_PTR▶ │ DB_TRX_ID = 初始 │ ─▶ NULL
└──────────────────┘            └──────────────────┘
```

A 第二次读时的判定：读到当前行 500 → 拿 `DB_TRX_ID=B` 问自己的 **Read View**「B 对我可见吗」→ 不可见（B 在 A 的快照建立后才提交）→ **顺 `DB_ROLL_PTR` 跳进 undo 读上一版 100** → 可见 → 返回 100。**「顺指针往回跳」就是在读 undo log**，没有 undo 这条链就断了，可重复读无从谈起。

> Read View（一致性视图）= A 首次快照读时生成的一张「谁可见」的判定表；`DB_ROLL_PTR` = 串起版本链的指针；undo log = 历史版本的物理仓库。**可重复读（现象）= MVCC 快照读（机制）= Read View 判定 + undo 提供旧版本（实现）。**

### 2. purge：旧版本的垃圾回收

undo 版本不会永远留着。**purge 是 InnoDB 的后台线程**，专门做两件收尾工作：

1. **物理删除被「删除标记」的记录**。`DELETE` 并不立刻从数据页物理删行，只是打一个 delete-mark。为什么不立刻删？因为可能还有老事务的 Read View 要看到这行的旧样子（MVCC）。等到没有任何活跃事务再需要这个版本，purge 才把它物理清掉。
2. **回收不再需要的 undo log**。这正是与本实验直接相关的部分：一条 undo 旧版本（如 balance=100）能被回收的前提是——**再没有任何 Read View 可能读到它**。

判定靠一条 **purge view**：某个版本**再没有任何活跃事务的 Read View 可能读到它**时，才允许回收。把它和已经熟的两个词摆一起就清楚了：

| 动作 | 谁做 | 时机 | 本质 |
|:--|:--|:--|:--|
| **rollback** | 事务自己 | 主动撤销时 | 拿 undo 反向重放 before-image |
| **MVCC 读** | 查询 | 快照读时 | 顺 DB_ROLL_PTR 沿 undo 链找可见旧版 |
| **purge** | 后台线程 | 旧版本**再没人要**时 | GC：物理删 delete-mark 记录 + 回收 undo |

只要有一个老事务还持有旧 Read View，purge 就不敢往前推——万一它还要读呢。于是 undo 版本只能堆着，`History list length` 一路涨。这就是下面长事务危害的机制入口。

### 3. 长事务的危害（exp1 进阶演示证明的东西）

exp1 脚本尾部让 A 建 Read View 后**不提交**，B 连改 500→600→700，观察 `History list length`（未被 purge 的 undo 版本数）上涨；A `COMMIT` 后随 purge 推进回落。这条现象背后是一串真实的生产事故链：

| 危害 | 机制 |
|:--|:--|
| **undo 表空间膨胀** | 老事务的 Read View 卡住 purge → 历史版本无法回收 → `ibdata`/undo tablespace 撑大，磁盘吃紧 |
| **查询变慢** | 版本链越拉越长，MVCC 读要顺着链回溯更多版本才找到可见版，单次读变重 |
| **锁长期占用** | 长事务持有的行锁/间隙锁迟迟不释放 → 别的事务等锁、超时、甚至死锁概率上升 |
| **主从延迟** | 大事务在从库要等整个事务重放完才可见，放大复制延迟 |
| **回滚/崩溃恢复慢** | 事务越大，`ROLLBACK` 或崩溃后要重放/撤销的量越大 |

排查手段：`SHOW ENGINE INNODB STATUS` 看 `History list length`（持续高不落 = 有长事务卡 purge）；`information_schema.INNODB_TRX` 按 `trx_started` 找跑得最久的事务。治理：控制事务粒度、别在事务里夹 RPC/慢逻辑、大批量操作分批提交、监控长事务并告警。

> 一句话面试收尾：**长事务的根因危害，全都能收敛到「它的旧 Read View 卡住了 purge」这一句上。** undo 撑大、读变慢、锁不放，是同一个原因的三张脸。

---

## 四、实验 4（思想实验）：为什么要两阶段提交？— 把三大日志串成一个故事

redo 归 InnoDB，binlog 归 Server，是**两份独立的日志**。一个事务提交要同时落这两份，若不用 2PC，崩溃点不同就会不一致：

- 只写了 redo 没写 binlog → 本机恢复后有这条数据，但**从库永远缺这条** → 主从不一致。
- 只写了 binlog 没写 redo → 从库有、主库崩溃恢复后没有 → 同样不一致。

InnoDB 的解法 —— **内部 XA / 两阶段提交**：

```
1. redo 写入并置为 prepare 状态（记下 XID）
2. 写 binlog 并刷盘   ← 这一步是「提交点」
3. redo 置为 commit 状态
```

崩溃恢复时的仲裁规则：
- redo 是 commit → 直接生效。
- redo 是 prepare：**去 binlog 里找这个 XID**——
  - binlog 有 → 说明第 2 步已过，**提交**（补上 commit）。
  - binlog 无 → 说明崩在第 2 步前，**回滚**。

于是「引擎里生效」与「binlog 里有(从库能拿到)」永远同进同退。
**双一配置** `innodb_flush_log_at_trx_commit=1` + `sync_binlog=1` 才能让每一步都真落盘，2PC 的保证才不打折。

> 想真跑 2PC 崩溃点，可在 exp2 的 kill 前后配合 `SHOW BINLOG EVENTS` 观察，属于加餐。

---

## 五、一条 UPDATE 的完整生命周期（把所有日志走一遍——面试终极题）

`UPDATE account SET balance=balance-100 WHERE id=1;`

1. 找到 id=1 的页（不在 buffer pool 就从磁盘读入）。
2. **记 undo**：把旧值 balance=100 写进 undo log（回滚 & MVCC 用）。
3. 改内存页：balance=0（此页变「脏页」，还没落盘）。
4. **记 redo**（物理：某页某处改成 0），置 **prepare**。
5. Server 层**记 binlog**（逻辑：这行的变更），刷盘。
6. **redo 置 commit**，事务返回成功。
7. 稍后后台线程慢慢把脏页刷回磁盘（与提交解耦，正是 WAL 的意义）。

背这七步 = 一次性把 undo/redo/binlog/2PC/WAL/脏页 全串起来。

---

## 六、自测题（盖住答案先自己讲）

1. undo log 除了回滚，还支撑哪个 ACID 特性？具体怎么支撑？
2. redo 和 binlog 有什么区别？至少说 4 个维度。为什么有了 binlog 还需要 redo？
3. `innodb_flush_log_at_trx_commit` 的 0/1/2 分别什么语义？**分「进程崩」和「掉电崩」**各丢多少数据？为什么 `=2` 扛得住进程崩却扛不住掉电？（exp2 三幕全验证过，`docker kill` 只能模拟进程崩）
4. 为什么 redo 和 binlog 要两阶段提交？只用一个日志、或不用 2PC 会怎样？举出两种崩溃点。
5. 崩溃恢复时，一个处于 prepare 状态的 redo 记录，MySQL 凭什么决定提交还是回滚？
6. 完整讲一条 UPDATE 从执行到提交，undo/redo/binlog 各在哪一步、脏页何时落盘？
7. 「双一」指什么？关掉后分别拿什么换什么？
8. binlog 的 ROW / STATEMENT / MIXED 格式区别？为什么复制推荐 ROW？
9. MVCC 快照读怎么找到「对我可见的旧版本」？说出 Read View、DB_ROLL_PTR、undo 三者各自的角色。
10. purge 线程是干什么的？一个长事务不提交，为什么会让 undo 膨胀、`History list length` 涨？由此说出长事务至少 3 个危害。

<details>
<summary>答案要点（先自答再展开）</summary>

1. 隔离性 / MVCC：快照读沿 undo 版本链回溯到当前事务 Read View 可见的旧版本。
2. 见「redo vs binlog 对比表」。redo 是物理、循环写、引擎层、只救本机崩溃且会被覆盖；binlog 逻辑、追加、Server 层、永久，管复制/PITR，救不了引擎页级崩溃恢复 → 两者不可互相替代。
3. `1`=每次提交 write+fsync（默认，最多丢 0）；`0`=只写 log buffer，靠后台每秒刷（崩溃约丢 1s）；`2`=每次 write 到 OS page cache 但不 fsync（MySQL 崩不丢，OS/主机崩约丢 1s）。
4. 保证 redo(引擎生效) 与 binlog(从库可见) 一致。崩溃点：只 redo 无 binlog→从库缺数据；只 binlog 无 redo→主库缺数据。
5. 去 binlog 按 XID 查：有则提交，无则回滚。
6. 见第五节七步。
7. `innodb_flush_log_at_trx_commit=1` + `sync_binlog=1`；关掉用持久性换吞吐。
8. STATEMENT 记 SQL（省空间但有不确定函数/主从不一致风险）；ROW 记行前后镜像（准确、可 PITR，稍大）；MIXED 自动择优。推荐 ROW 因复制确定性最强。
9. 读当前行 → 拿其 DB_TRX_ID 问 Read View 是否可见 → 不可见就顺 DB_ROLL_PTR 跳进 undo 读上一版，循环到可见为止。Read View 定「可见性规则」，DB_ROLL_PTR 是串起版本链的指针，undo 是旧版本的物理仓库。
10. purge 是后台 GC：物理删除 delete-mark 记录 + 回收再没人要的 undo；判定靠 purge view。长事务持有的旧 Read View 让 purge 不敢往前推 → 历史版本无法回收 → undo 膨胀、History list length 涨。危害：undo 表空间膨胀、版本链变长拖慢查询、行锁长期不释放引发等锁/死锁（还有主从延迟、回滚/恢复慢）。
</details>

---

## 七、毕业标准

- [ ] 能在白板画出「ACID(目的) ↔ 三大日志(手段)」两层映射，不看表。
- [ ] 三个实验都亲手跑过，尤其 exp2 **亲眼见过 trx_commit=0 丢数据、=2 不丢**，并能解释这只证明「进程崩」层面的差别（见三-B）。
- [ ] 能脱稿讲第五节的 UPDATE 七步 + 第四节的 2PC 恢复仲裁。
- [ ] 自测题 1/2/4/5 能扛住连追三层。

> 复习节奏：毕业后 +1 / +7 / +30 天回看自测题；卡住就回来重跑对应实验。

---

## 八、参考资源

主要挖深读物：**《MySQL 是怎样运行的：从根儿上理解 MySQL》**（作者 小孩子4919，原掘金小册，后出版成纸质书）。
对应本 lab 的是全书**第四部分：事务与锁**（redo/undo、MVCC、锁）——跑完实验建立手感后，读这部分补「为什么这么设计」。

| 渠道 | 链接 | 说明 |
|:--|:--|:--|
| 掘金小册（原版·最权威） | <https://juejin.cn/>（站内搜「MySQL 是怎样运行的」） | 付费在线读，持续勘误 |
| 纸质 / 电子书 | [豆瓣](https://book.douban.com/subject/35231266/) · [京东](https://item.jd.com/13009316.html) | 人民邮电出版社，内容同小册 |
| 免费公开笔记 | <https://relph1119.github.io/mysql-learning-notes/> | 读者按书章节整理的开源笔记，网页直读，够啃 |

进阶「拆开引擎看」工具：
- **Jeremy Cole `innodb_ruby`** <https://github.com/jeremycole/innodb_ruby> —— 直接 dump InnoDB 页 / undo 段的物理结构，配 exp1 看 MVCC 版本链最直观。

> ⚠️ 搜索结果里那些「PDF 下载」站（hdbeta 之类）是盗版，勘误跟不上，别用。
