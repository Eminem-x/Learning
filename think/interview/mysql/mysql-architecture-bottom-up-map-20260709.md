# MySQL 自底向上认知地图(总索引 / 复习总纲)

> 来源:个人思考 + 学习
> 收录时间:2026-07-09 CST
>
> 能力地图定位:格 4(关系型数据库)的**顶层入口**——把散篇串成一张分层图
> 定位:这是 MySQL 各专题笔记的**总索引**。每次复习**先过这张图定位"我在哪一层、这层挂了哪几篇"**,卡壳再钻具体篇。
> 交叉链接:[四棵树对比](./tree-structures-bplus-btree-rbtree-20260708.md)、[联合索引·最左前缀·ICP](./mysql-icp-composite-index-leftmost-prefix-20260709.md)、[三大日志×ACID 实验台](./mysql-logs-lab/README.md)、[mysql 面试题库(广度)](./mysql-interview-questions-90-coverage-20260704.md)

---

## 30 秒电梯版(盖住全文也要能讲清主线)

一句话主线:**MySQL 是一摞有依赖的层,不是一堆并列的名词;认知要"自底向上"搭,而且分清"包含"和"并列"。**

一句话把八层串起来:
> 数据按「表空间→页」摆在磁盘,页组织成 **B+树**当**索引**;读写先进 **Buffer Pool** 内存页;改动靠 **undo** 存旧值、**redo** 保证不丢、**锁 + MVCC** 管并发;提交时 redo 和 Server 层的 **binlog** 用 **2PC** 对齐、binlog 喂给从库;这一切之上,**优化器**决定怎么查、你用 **EXPLAIN** 验证。

三个最容易搞错的层级关系(审计红线):
1. **InnoDB 是"盒子"不是"一层"**:index / 锁 / MVCC / undo / redo 全在盒子内,只有 binlog 在盒子外(Server 层)。
2. **undo 垫在 MVCC 下面**:别把"三大 log"捆成一块——undo 是 MVCC 和回滚的地基,redo 管持久性,binlog 在 Server 层,三者是散开的。
3. **锁 和 MVCC 是并列的两套并发控制**:锁管写写冲突/当前读,MVCC 管读写不阻塞/快照读,不是上下层。

---

## 一、完整分层骨架(自底向上)

```
第7层  查询优化 & EXPLAIN     执行计划、索引选择、type/key_len/Extra、调优
第6层  Server 层             连接器→分析器→优化器→执行器
         │                   binlog(逻辑/跨引擎)→ 主从复制、PITR
         │                   redo ↔ binlog 用 2PC 对齐
━━━━━━━━━━━━━━ InnoDB 盒子边界 ━━━━━━━━━━━━━━
第5层  事务与并发(盒子内)    redo → WAL / 持久性(D)
         │                   ┌ 锁(行/间隙/next-key)→ 写写冲突、当前读
         │                   └ MVCC(Read View)──┐  读写不阻塞、快照读
         │                   undo ──────────────┘← MVCC 和回滚(A)的地基
         │                   隔离级别 RU/RC/RR/S = 上面几样的组合目的
第4层  Buffer Pool(内存侧)  缓存页、脏页、LRU、change buffer、doublewrite
━━━━━━━━━━━━━━ 磁盘 ↔ 内存 的桥 ━━━━━━━━━━━━━━
第3层  索引                  聚簇索引(=数据)、二级、覆盖、最左前缀、回表
第2层  数据结构 B+树         页作为节点,串成矮胖的树
第1层  物理组织             表空间→段→区→页(16KB)→行
第0层  OS / 磁盘            4KB 页、顺序/随机 IO
```

**两条对称轴**(记这两条,整张图就立起来了):
- **磁盘侧 vs 内存侧**:第 1~3 层(表空间/B+树/索引)是数据在**磁盘**怎么摆;第 4 层 Buffer Pool 是数据在**内存**怎么缓存。WAL、脏页、`trx_commit` 全建立在"先改内存页、后台刷盘"这条桥上。
- **盒子内 vs 盒子外**:第 5 层全在 InnoDB 内(只救本机);第 6 层 binlog 在 Server 层(跨引擎、喂从库)。2PC 就是缝合这条边界的针。

---

## 二、每层挂哪几篇笔记(复习调度器)

> 复习时先看这张表定位,再进对应笔记。空的格子 = 还没补的坑(见第四节)。

| 层 | 主题 | 已沉淀笔记 | 状态 |
|:--:|---|---|:--:|
| 1 | 表空间→段→区→页→行 物理组织 | [四棵树 §8.A.1 / §8.D](./tree-structures-bplus-btree-rbtree-20260708.md)(页、数据页=叶子页) | 部分 |
| 2 | **B+树**(为何数据库用 B+、页扇出、3 层容量) | [四棵树对比](./tree-structures-bplus-btree-rbtree-20260708.md) | ✅ |
| 3 | **索引**(聚簇/二级/覆盖/最左前缀/回表/ICP/索引失效) | [ICP·最左前缀](./mysql-icp-composite-index-leftmost-prefix-20260709.md) + [四棵树 §8.A](./tree-structures-bplus-btree-rbtree-20260708.md) + [题库](./mysql-interview-questions-90-coverage-20260704.md) | ✅ |
| 4 | **Buffer Pool**(脏页/LRU/change buffer/doublewrite) | — | ⬜ 未补 |
| 5 | **事务与并发**(undo/redo、MVCC、锁、隔离级别、2PC) | [三大日志×ACID 实验台](./mysql-logs-lab/README.md)(定位+手感);锁/Read View 深挖待补 | 部分 |
| 6 | **Server 层 + binlog + 复制**(连接器→优化器→执行器、主从、PITR) | [主从复制·一致性光谱](./mysql-replication-master-slave-standby-consistency-20260709.md)(主从/主备/读写分离辨析、异步/半同步/MGR)+ [实验台 exp3/§四](./mysql-logs-lab/README.md)(binlog+2PC) | 部分 |
| 7 | **查询优化 & EXPLAIN**(执行计划、type/key_len/Extra、调优) | [ICP §四](./mysql-icp-composite-index-leftmost-prefix-20260709.md)(key_len/Extra 三态)+ [题库](./mysql-interview-questions-90-coverage-20260704.md) | 部分 |

---

## 三、三个最容易搞错的层级关系(审计要点)

平铺背名词时最常犯的三个"维度错位",逐个掰清:

### ① InnoDB 是"盒子",不是和 index/锁/MVCC 并列的"一层"

```
❌ 隐含误解:innodb、index、锁、mvcc、log 平铺成几层
✅ 实际:index / 锁 / MVCC / undo / redo 全在 InnoDB【盒子内】
        只有 binlog 在盒子【外】(Server 层,跨引擎)
```
所以谈"存储引擎"时,它是第 4~5 层的**容器边界**,不是某一横层。换个引擎(MyISAM)→ 盒子里的 MVCC/redo/undo/行锁全没了,但 binlog、Server 层照旧——这就是"盒子内 vs 盒子外"的意义。

### ② undo 垫在 MVCC 下面——别把"三大 log"捆成一块

三大 log 的层级其实是**散开**的,捆一块恰恰割裂了最重要的依赖:

| log | 兜什么 | 该排哪 |
|---|---|---|
| **undo** | 回滚(A)+ **MVCC 版本链** | MVCC 的**地基**,排在 MVCC 之下 |
| **redo** | WAL / 持久性(D) | 事务提交层,和 MVCC 并列偏上 |
| **binlog** | 复制 / PITR(**不属 ACID**) | InnoDB 盒子外,最上面 |

> 记忆锚点:**"三大 log" 是个考试话术,不是一个架构层。** MVCC 快照读要"沿 undo 版本链找旧版本"——undo 不先于 MVCC,MVCC 无从谈起。

### ③ 锁 和 MVCC 是并列的两套并发控制,不是上下层

| 手段 | 解决 | 场景 |
|---|---|---|
| **锁**(行/间隙/next-key) | **写写**冲突、当前读 | `UPDATE/DELETE/SELECT...FOR UPDATE` |
| **MVCC**(undo + Read View) | **读写**不阻塞、快照读 | 普通 `SELECT` |

RR 下防幻读 = **MVCC(快照读)+ next-key 锁(当前读)** 两手都要,正是二者"并列分工"的体现。

---

## 四、缺口与未启格子(诚实标注,别以为已闭环)

这张图目前**骨架完整、承重墙有两处还没砌**。按面试性价比,缺口排序:

| 缺口 | 属层 | 追问频率 | 优先级 |
|---|:--:|:--:|:--:|
| **锁**(行锁/间隙锁/next-key、加锁规则、死锁排查) | 5 | 🔥🔥 | **高**(格 4 明确要求,当前空白) |
| **MVCC 的 Read View 可见性算法**(m_ids/min/max_trx_id/creator 判断) | 5 | 🔥🔥 | **高**(实验台只到"沿版本链",没展开规则) |
| **Buffer Pool**(脏页/LRU 冷热分离/change buffer/doublewrite) | 4 | 🔥 | 中(WAL 的前提) |
| **redo 内部**(LSN/checkpoint/log block/组提交) | 5 | 🔥 | 中 |
| **Server 层执行链路**(连接器→分析器→优化器→执行器全流程) | 6 | 🔥 | 中 |
| ~~**主从复制细节**(异步/半同步、延迟)~~ ✅ 已补 [主从复制·一致性光谱](./mysql-replication-master-slave-standby-consistency-20260709.md);剩 binlog 组提交、MGR 深挖 | 6 | 🔥 | 中 |

> 下一步建议:优先补**锁**和 **MVCC Read View**——这两块是格 4 追问命中率最高、当前最空的,补完这张图才算"能扛三层追问"。

---

## 五、自测题(盖住答案,先自己讲一遍)

<details>
<summary>1. 从磁盘到客户端,MySQL 有哪几层?一句话串起来。</summary>

物理组织(表空间→页)→ B+树 → 索引 → Buffer Pool(内存) → 事务并发(undo/redo/锁/MVCC) → Server 层(binlog/优化器/执行器) → EXPLAIN 调优。串法见 30 秒电梯版那句。
</details>

<details>
<summary>2. 为什么说 InnoDB 是"盒子"而不是"一层"?换成 MyISAM 会怎样?</summary>

index/锁/MVCC/undo/redo 都在 InnoDB 内部,binlog 在 Server 层(盒子外)。换 MyISAM:盒子内的 MVCC/redo/undo/行锁全没了(表锁、无事务),但 binlog、Server 层照旧。
</details>

<details>
<summary>3. 为什么不能把"三大 log"当成一个架构层?undo 该排哪?</summary>

三者层级散开:undo 是 MVCC 和回滚的地基(排 MVCC 之下)、redo 管持久性(事务提交层)、binlog 在 Server 层(不属 ACID)。捆一块会割裂 undo→MVCC 依赖。
</details>

<details>
<summary>4. 锁和 MVCC 是上下层关系吗?RR 防幻读用哪个?</summary>

并列的两套并发控制:锁管写写冲突/当前读,MVCC 管读写不阻塞/快照读。RR 防幻读两手都用:快照读靠 MVCC,当前读靠 next-key 锁。
</details>

<details>
<summary>5. "磁盘侧 vs 内存侧"这条对称轴指什么?WAL 建立在它哪一侧?</summary>

磁盘侧=表空间/B+树/索引(数据怎么摆);内存侧=Buffer Pool(数据怎么缓存)。WAL/脏页/trx_commit 建立在"先改内存页、后台刷盘"这条桥上,没有 Buffer Pool 讲不通。
</details>

**毕业标准**:能白板默画这八层 + 两条对称轴(磁盘/内存、盒子内/外);能不看表说出三处层级关系(InnoDB 是盒子、undo 垫 MVCC、锁与 MVCC 并列);能一句话自底向上把八层串成一条 UPDATE 的旅程。

---

## 附录:自我思考

- **这篇是"地图"不是"景点"**:它的价值不是讲透某一层(那是各分篇的事),而是**给每篇笔记一个坐标**,让复习有个"总台"先定位再钻。CLAUDE.md §五要的"沉淀索引 + 复习调度器",顶层入口就是这张图。
- **审计的产物**:这张图是一次自我审计的结晶——原来脑中是"b+树/innodb/index/锁/mvcc/三大log/查询优化/explain"的**平铺链**,审计后发现三处维度错位(盒子/地基/并列)。**平铺 → 分层带依赖,是从"会背"到"会讲"的关键一跃。**
- **待补钩子**:第四节六个缺口,尤其**锁**和 **MVCC Read View** 两篇独立笔记——补完回来把第二节表格的 ⬜ 填绿、把本篇从"骨架完整"升级成"承重墙齐全"。

**下次复习**:2026-07-10(+1 天)→ 07-16(+7)→ 08-09(+30)。每次复习**从本篇进**,再散到各分篇。
