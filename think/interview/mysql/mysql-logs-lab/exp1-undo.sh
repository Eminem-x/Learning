#!/usr/bin/env bash
# 实验 1：undo log 同时兜底「原子性(A)」和「隔离性(I / MVCC)」
# 这一部分自动演「原子性」；MVCC 需要两个会话，脚本会打印手动步骤。
set -euo pipefail
cd "$(dirname "$0")"
M="docker exec -i mysql-primary mysql -uroot -proot lab"

echo "== 准备账户表 =="
$M <<'SQL'
DROP TABLE IF EXISTS account;
CREATE TABLE account (id INT PRIMARY KEY, balance INT) ENGINE=InnoDB;
INSERT INTO account VALUES (1, 100);
SQL

echo
echo "== A 原子性：改了但 ROLLBACK，undo 把 before-image 反向重放回来 =="
$M <<'SQL'
BEGIN;
UPDATE account SET balance = 999 WHERE id = 1;   -- 事务内已改
SELECT balance AS '事务内看到的(改后)' FROM account WHERE id = 1;
ROLLBACK;                                          -- 靠 undo 撤销
SELECT balance AS 'ROLLBACK之后(回到100)' FROM account WHERE id = 1;
SQL

echo
echo "== 看 History list length（未被 purge 的 undo 版本数）=="
docker exec -i mysql-primary mysql -uroot -proot -e "SHOW ENGINE INNODB STATUS\G" | grep -i 'History list length'

cat <<'TIP'

== I 隔离性(MVCC)：需要两个终端手动演，才能看清 undo 版本链 ==
终端 A：  ./lab.sh psh
    SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ;
    BEGIN;
    SELECT balance FROM account WHERE id=1;   -- 建立 Read View，读到 100

终端 B：  ./lab.sh psh
    UPDATE account SET balance=500 WHERE id=1;   -- 自动提交，真实值变 500

回到终端 A（同一事务里再读一次）：
    SELECT balance FROM account WHERE id=1;   -- 仍是 100 ！
    -- 值已经被 B 改成 500，A 却还看到 100，
    -- 就是因为 A 顺着行上的 DB_ROLL_PTR 回滚指针，
    -- 跳进 undo log 版本链读到了「对我可见的旧版本」——这就是 MVCC。
    COMMIT;
    SELECT balance FROM account WHERE id=1;   -- 现在才看到 500

结论：undo 不只是「回滚用」，它还是 MVCC 快照读的数据来源。面试必答点。

== 进阶：同一个实验，再演「长事务撑大 undo」==
目的：看见「A 的旧快照没提交 → 历史版本无法 purge → History list length 上涨」。

终端 A：  ./lab.sh psh
    SET SESSION TRANSACTION ISOLATION LEVEL REPEATABLE READ;
    BEGIN;
    SELECT balance FROM account WHERE id=1;   -- 建立 Read View，之后【别提交】

终端 B：  ./lab.sh psh   （连续改多次，每次自动提交，制造多个历史版本）
    UPDATE account SET balance=500 WHERE id=1;
    UPDATE account SET balance=600 WHERE id=1;
    UPDATE account SET balance=700 WHERE id=1;

另开一个终端看未被 purge 的 undo 版本数（多跑几次看趋势）：
    docker exec -i mysql-primary mysql -uroot -proot \
      -e "SHOW ENGINE INNODB STATUS\G" | grep -i 'History list length'
    -- 会看到它涨了：A 的旧快照还在用，这些 100→500→600 的历史版本
    -- 还不能被 purge 线程回收，只能堆在 undo 里。

回到终端 A：
    COMMIT;                                   -- 旧快照释放，purge 才能回收
    -- 再看 History list length，会随 purge 推进回落。

结论：长事务不提交 → undo 版本链无法回收 → History list length 涨 → undo 表空间膨胀。
这就是面试「长事务有什么危害」的底层答案。
TIP
