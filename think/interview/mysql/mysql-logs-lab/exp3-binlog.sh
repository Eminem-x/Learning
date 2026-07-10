#!/usr/bin/env bash
# 实验 3：binlog 兜底「复制 / PITR」——它是 Server 层的逻辑日志，不属 ACID
#   1) 建主从复制，写主库，看从库自动跟上（binlog → relay log → 重放）
#   2) 用 mysqlbinlog 把 binlog 解码成人能读的事件，看清它记的是「逻辑变更」
#   3) SET sql_log_bin=0 写一条，证明「不进 binlog 的写，从库永远收不到」
set -euo pipefail
cd "$(dirname "$0")"
MP="docker exec -i mysql-primary mysql -uroot -proot"
MR="docker exec -i mysql-replica  mysql -uroot -proot"

echo "== 1. 主库建复制账号 =="
$MP <<'SQL'
CREATE USER IF NOT EXISTS 'repl'@'%' IDENTIFIED WITH mysql_native_password BY 'repl';
GRANT REPLICATION SLAVE ON *.* TO 'repl'@'%';
SQL

echo "== 2. 从库指向主库并启动复制（GTID 自动定位）=="
$MR <<'SQL'
STOP REPLICA;
CHANGE REPLICATION SOURCE TO
  SOURCE_HOST='mysql-primary', SOURCE_PORT=3306,
  SOURCE_USER='repl', SOURCE_PASSWORD='repl',
  SOURCE_AUTO_POSITION=1;
START REPLICA;
SQL
sleep 3
echo "  复制线程状态（两个 Yes 才算通）："
$MR -e "SHOW REPLICA STATUS\G" | grep -E 'Replica_IO_Running|Replica_SQL_Running|Last_Error' | head -3

echo
echo "== 3. 在主库写数据 =="
$MP lab <<'SQL'
DROP TABLE IF EXISTS repl_demo; CREATE TABLE repl_demo(id INT PRIMARY KEY, msg VARCHAR(50)) ENGINE=InnoDB;
INSERT INTO repl_demo VALUES (1,'from-primary-via-binlog');
SQL
sleep 2
echo "  从库查（应能看到，且从库是 read_only，数据只能从复制流来）："
$MR -e "SELECT * FROM lab.repl_demo;"

echo
echo "== 4. 把 binlog 解码成人类可读事件（看它记的是逻辑行变更）=="
BINFILE=$($MP -N -e "SHOW BINARY LOGS;" | tail -1 | awk '{print $1}')
echo "  当前 binlog 文件：$BINFILE"
docker exec -i mysql-primary mysqlbinlog --base64-output=DECODE-ROWS -vv "/var/lib/mysql/$BINFILE" \
  | grep -iE 'INSERT INTO|Table_map|### ' | tail -15 || true

echo
echo "== 5. sql_log_bin=0：绕过 binlog 写一条，证明从库收不到 =="
$MP lab -e "SET sql_log_bin=0; INSERT INTO repl_demo VALUES (99,'NOT-logged'); SET sql_log_bin=1;"
sleep 2
echo "  主库有 99："; $MP -e "SELECT * FROM lab.repl_demo WHERE id=99;"
echo "  从库没有 99（binlog 是复制的唯一信息源）："; $MR -e "SELECT * FROM lab.repl_demo WHERE id=99;" || echo "  （空）"
echo
echo ">>> 记忆锚：redo 是引擎内部物理日志，救的是本机崩溃；binlog 是 Server 层逻辑日志，喂的是从库和 PITR。两者层级不同、用途不同。"
