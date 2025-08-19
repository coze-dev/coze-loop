#!/bin/sh

exec 2>&1
set -e

print_banner() {
  msg="$1"
  side=30
  content=" $msg "
  content_len=${#content}
  line_len=$((side * 2 + content_len))

  line=$(printf '*%.0s' $(seq 1 "$line_len"))
  side_eq=$(printf '*%.0s' $(seq 1 "$side"))

  printf "%s\n%s%s%s\n%s\n" "$line" "$side_eq" "$content" "$side_eq" "$line"
}

print_banner "Clickhouse Init Starting..."

for i in $(seq 1 60); do
  if clickhouse-client \
      --host=coze-loop-clickhouse \
      -u "${COZE_LOOP_CLICKHOUSE_USER}" \
      --password="${COZE_LOOP_CLICKHOUSE_PASSWORD}" \
      --query "SELECT 1" \
      2>/dev/null \
      | grep -q 1; then
    break
  else
    sleep 1
  fi
  if [ "$i" -eq 60 ]; then
    echo "[ERROR] Clickhouse server or database('${COZE_LOOP_CLICKHOUSE_DATABASE}') not available after 60 time."
    exit 1
  fi
done

clickhouse-client \
  --host=coze-loop-clickhouse \
  -u "${COZE_LOOP_CLICKHOUSE_USER}" \
  --password="${COZE_LOOP_CLICKHOUSE_PASSWORD}" \
  --query "CREATE DATABASE IF NOT EXISTS \`${COZE_LOOP_CLICKHOUSE_DATABASE}\`;"

i=1
# shellcheck disable=SC2010
for file in $(ls /coze-loop-clickhouse-init/bootstrap/init-sql | grep '\.sql$'); do
  echo "+ init #$i: < $file"
  clickhouse-client \
    --host=coze-loop-clickhouse \
    -u "${COZE_LOOP_CLICKHOUSE_USER}" \
    --password="${COZE_LOOP_CLICKHOUSE_PASSWORD}" \
    --database="${COZE_LOOP_CLICKHOUSE_DATABASE}" \
    < "/coze-loop-clickhouse-init/bootstrap/init-sql/${file}"
  i=$((i + 1))
done

print_banner "Clickhouse Init Completed!"