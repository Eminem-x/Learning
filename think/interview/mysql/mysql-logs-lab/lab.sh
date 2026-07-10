#!/usr/bin/env bash
# 实验台总控。用法：
#   ./lab.sh up        起主从两个 MySQL 8 实例（首次拉镜像稍慢）
#   ./lab.sh down      拆掉（含数据）
#   ./lab.sh p "SQL"   在主库执行一条 SQL
#   ./lab.sh r "SQL"   在从库执行一条 SQL
#   ./lab.sh psh       进主库交互式 shell（演 MVCC 时开两个终端各一个）
#   ./lab.sh status    看主库 InnoDB 状态里关键几行（LSN / History list）
#
#   ./lab.sh p "SHOW VARIABLES LIKE 'transaction_isolation';"
set -euo pipefail
cd "$(dirname "$0")"

M="docker exec -i mysql-primary mysql -uroot -proot"
R="docker exec -i mysql-replica mysql -uroot -proot"

case "${1:-}" in
up)
  docker compose up -d
  echo "等主库就绪..."
  until docker exec mysql-primary mysqladmin ping -uroot -proot --silent 2>/dev/null; do sleep 2; done
  echo "主库 OK（host:3316），从库 host:3317。下一步跑 ./exp3-binlog.sh 建复制，或直接跑 exp1/exp2。"
  ;;
down) docker compose down -v ;;
p) echo "${2:?需要 SQL}" | $M lab ;;
r) echo "${2:?需要 SQL}" | $R ;;
psh) docker exec -it mysql-primary mysql -uroot -proot lab ;;
rsh) docker exec -it mysql-replica mysql -uroot -proot ;;
status)
  docker exec -i mysql-primary mysql -uroot -proot -e "SHOW ENGINE INNODB STATUS\G" |
    grep -Ei 'Log sequence number|Log flushed up to|Last checkpoint|History list length'
  ;;
*) grep '^#' "$0" | sed 's/^# \{0,1\}//' ;;
esac
