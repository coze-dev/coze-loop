#!/bin/sh

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

print_banner "Redis Init Starting..."

for i in $(seq 1 60); do
  if redis-cli \
      -h coze-loop-redis \
      -a "${COZE_LOOP_REDIS_PASSWORD}" \
      --no-auth-warning ping \
      2>/dev/null \
      | grep -q PONG; then
    echo "[INFO] Redis is ready"
    print_banner "Redis Init Completed!"
    exit 0
  else
    echo "[INFO] [$i/60] Waiting for Redis..."
    sleep 1
  fi
done

echo "[ERROR] Redis did not become ready after 60 attempts."
exit 1