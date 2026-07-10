#!/usr/bin/env bash
# 实验 2：redo log 兜底「持久性(D)」——崩溃恢复（WAL）
#
# 三幕，核心是看懂 innodb_flush_log_at_trx_commit（=1/=0/=2）在「进程崩」下的差别：
#   幕一 =1（默认，安全）：提交后 kill -9 主库，重启，数据还在 → redo 已 fsync，重放回来。
#   幕二 =0（危险）：提交后立刻崩，最近提交的数据丢 → redo 停在进程内存(log buffer)，随进程死。
#   幕三 =2（对照）：提交后立刻崩，数据却「不丢」→ redo 提交时已 write() 进 OS page cache，
#                    docker kill 只杀 mysqld 进程、不清宿主机内核缓存，所以扛得住「进程崩」。
#
# ⚠️ 关键前提：docker kill = 给容器主进程(mysqld)发 SIGKILL = 模拟【进程崩溃】，
#    宿主机内核和它的 page cache【毫发无损】。所以本实验【无法模拟掉电/OS 崩】，
#    只能区分「=0 扛不住进程崩」vs「=2 扛得住进程崩」。=2 在真实掉电下仍会丢 ≤1s，
#    那个场景本环境复现不了，只能靠语义推演。
#
# 【为什么原来的幕二会「翻车」（数据没丢）？】
#    =0 下 redo 由后台 master 线程每 innodb_flush_log_at_timeout 秒（默认 1s）刷一次盘。
#    脚本从「提交」到「kill」要经过 docker exec / mysql client 启动等开销，很容易 >1s，
#    后台线程已经抢先把 redo 刷盘 → 重启重放 → 数据还在，看不到丢失。
#    修法：把 innodb_flush_log_at_timeout 调到 30s，让后台在我们这几秒窗口内【绝不刷盘】，
#    再快速 kill，=0 丢数据几乎必现。这不是作弊，是把竞态窗口人为放大到可控。
set -euo pipefail
cd "$(dirname "$0")"
M="docker exec -i mysql-primary mysql -uroot -proot lab"

crash_and_recover () {
  echo "  → kill -9 主库 mysqld（docker kill=SIGKILL，模拟【进程崩】；宿主机内核/page cache 不受影响）"
  docker kill --signal=KILL mysql-primary >/dev/null
  sleep 2
  echo "  → docker start：复用同一份 /var/lib/mysql，触发 redo 崩溃恢复"
  docker start mysql-primary >/dev/null
  until docker exec mysql-primary mysqladmin ping -uroot -proot --silent 2>/dev/null; do sleep 1; done
  echo "  → 崩溃恢复日志："
  docker logs mysql-primary 2>&1 | grep -iE 'crash recovery|Recovering|Apply batch|rollback|Starting to' | tail -5 || true
}

########################################################################
echo "###################### 幕一：innodb_flush_log_at_trx_commit = 1（默认，D 成立）"
$M -e "SET GLOBAL innodb_flush_log_at_trx_commit=1;"
$M <<'SQL'
DROP TABLE IF EXISTS durable; CREATE TABLE durable(id INT PRIMARY KEY, note VARCHAR(50)) ENGINE=InnoDB;
INSERT INTO durable VALUES (1,'committed-then-crash'); COMMIT;
SQL
echo "  提交完成，立即制造崩溃..."
crash_and_recover
echo "  恢复后查数据："
$M -e "SELECT * FROM durable;"
echo "  >>> 期望：那一行【还在】。提交时 redo 已 fsync 到盘，脏页没落盘也能重放回来。"

########################################################################
echo
echo "###################### 幕二：innodb_flush_log_at_trx_commit = 0（提交啥都不做，进程崩即丢）"
# 先在【安全的 =1】下把表建好并落盘，保证「表在、只是行丢」，现象更干净。
$M -e "SET GLOBAL innodb_flush_log_at_trx_commit=1;"
$M <<'SQL'
DROP TABLE IF EXISTS lossy; CREATE TABLE lossy(id INT PRIMARY KEY, note VARCHAR(50)) ENGINE=InnoDB;
SQL
# 切到危险模式，并把后台刷盘周期拉到 30s —— 关键修正：放大丢失窗口，压掉竞态。
$M -e "SET GLOBAL innodb_flush_log_at_trx_commit=0; SET GLOBAL innodb_flush_log_at_timeout=30;"
echo "  已设 trx_commit=0 + flush_log_at_timeout=30s（后台 30s 内绝不刷盘）"
echo "  连续提交 a/b/c 后【立刻】崩溃（redo 只在进程内存 log buffer 里，还没被后台刷走）"
$M <<'SQL'
INSERT INTO lossy VALUES (1,'a'); COMMIT;
INSERT INTO lossy VALUES (2,'b'); COMMIT;
INSERT INTO lossy VALUES (3,'c'); COMMIT;
SQL
crash_and_recover
echo "  恢复后查数据："
$M -e "SELECT * FROM lossy;" || echo "  （表若也不在，说明连 DDL 的 redo 都没刷）"
echo "  >>> 期望：a/b/c【大概率全丢】（0 行）。=0 提交时啥都不做，redo 随 mysqld 进程一起没了。"

########################################################################
echo
echo "###################### 幕三：innodb_flush_log_at_trx_commit = 2（对照：进程崩【不丢】）"
# 同样先在 =1 下把表建好落盘。
$M -e "SET GLOBAL innodb_flush_log_at_trx_commit=1; SET GLOBAL innodb_flush_log_at_timeout=1;"
$M <<'SQL'
DROP TABLE IF EXISTS survive2; CREATE TABLE survive2(id INT PRIMARY KEY, note VARCHAR(50)) ENGINE=InnoDB;
SQL
# 切到 =2，并同样拉长后台刷盘周期到 30s：证明就算后台没 fsync，进程崩也不丢。
$M -e "SET GLOBAL innodb_flush_log_at_trx_commit=2; SET GLOBAL innodb_flush_log_at_timeout=30;"
echo "  已设 trx_commit=2 + flush_log_at_timeout=30s（提交只 write() 到 OS cache，不 fsync；后台 30s 才 fsync）"
echo "  连续提交 x/y/z 后【立刻】崩溃（此时盘上还没 fsync，但数据已在宿主机内核 page cache）"
$M <<'SQL'
INSERT INTO survive2 VALUES (1,'x'); COMMIT;
INSERT INTO survive2 VALUES (2,'y'); COMMIT;
INSERT INTO survive2 VALUES (3,'z'); COMMIT;
SQL
crash_and_recover
echo "  恢复后查数据："
$M -e "SELECT * FROM survive2;"
echo "  >>> 期望：x/y/z【还在】！=2 提交时已 write() 进 OS page cache，docker kill 只杀进程、不清内核缓存。"
echo "  >>> 点睛：=2 扛得住【进程崩】，但扛不住【掉电/OS 崩】——后者本环境无法复现，只能推演。"
echo "  >>> 幕二(=0 全丢) vs 幕三(=2 不丢) 的差别，恰恰只体现在「进程崩」这种崩法上。"

########################################################################
echo
echo "  收尾：把参数调回安全值（trx_commit=1，timeout=1）"
$M -e "SET GLOBAL innodb_flush_log_at_trx_commit=1; SET GLOBAL innodb_flush_log_at_timeout=1;"
