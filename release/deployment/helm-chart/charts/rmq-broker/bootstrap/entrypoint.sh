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

for i in $(seq 1 60); do
  if "${ROCKETMQ_HOME}/bin/mqadmin" \
      topicList \
      -n coze-loop-rmq-namesrv:9876 \
      > /dev/null 2>&1; then
    break
  else
    sleep 1
  fi
  if [ "$i" -eq 60 ]; then
    echo "[ERROR] RMQ namesrv not available after 60 time."
    exit 1
  fi
done

mkdir -p /store/logs

(
  while true; do
    if sh /coze-loop-rmq-broker/bootstrap/healthcheck.sh; then
      print_banner "Completed!"
      break
    else
      sleep 1
    fi
  done
)&

exec "${ROCKETMQ_HOME}"/bin/mqbroker -n coze-loop-rmq-namesrv:9876