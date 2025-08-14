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

print_banner "Starting..."

(
  while true; do
    if sh /coze-loop-redis/bootstrap/healthcheck.sh; then
      print_banner "Completed!"
      break
    else
      sleep 1
    fi
  done
)&

exec redis-server \
      --requirepass "${COZE_LOOP_REDIS_PASSWORD}"