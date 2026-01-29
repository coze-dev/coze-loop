#!/bin/bash

exec 2>&1
set -e

log() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*"
}

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

start_time=$(date +%s)
print_banner "Starting..."
log "Init start, waiting for RMQ broker..."

MQADMIN_CMD="${ROCKETMQ_HOME}/bin/mqadmin"
MQNAMESRV_ADDR=coze-loop-rmq-namesrv:9876

declare -A topics
{
  while IFS='=' read -r topic consumers || [[ -n "${topic}" ]]; do
    [[ -z "${topic}" || "${topic:0:1}" == "#" ]] && continue
    topics["${topic}"]="${consumers}"
  done
} < /coze-loop-rmq-init/bootstrap/init-subscription/subscriptions.cfg

for i in $(seq 1 60); do
  if "${ROCKETMQ_HOME}/bin/mqadmin" \
      clusterList \
      -n "${MQNAMESRV_ADDR}" \
      2>/dev/null \
      | grep -q DefaultCluster; then
    break
  else
    sleep 1
  fi
  if [ "$i" -eq 60 ]; then
    log "[ERROR] RMQ broker not available after 60 time."
    exit 1
  fi
done

log "Broker ready (waited ${i}s), creating topics..."
i=1
for topic in "${!topics[@]}"; do
  ii=$i
  (
    log "+ Check if topic#$ii('$topic') exists..."
    if ! "${MQADMIN_CMD}" topicList -n "${MQNAMESRV_ADDR}" | grep -q "^$topic$"; then
      log "[+] Topic#$ii('$topic') not exists, now creating..."
      "${MQADMIN_CMD}" updateTopic -n "${MQNAMESRV_ADDR}" -c DefaultCluster -t "$topic" -r 8 -w 8
    else
      log "[-] Topic#$ii('$topic') already exists."
    fi

    IFS=',' read -ra consumer_groups <<< "${topics[$topic]}"
    j=1
    for group in "${consumer_groups[@]}"; do
      log "++ Check if consumer#$ii-$j('$group') exists..."
      if ! "${MQADMIN_CMD}" consumerProgress -n "${MQNAMESRV_ADDR}" | grep -q "^$group$"; then
        log "[++] Consumer#$ii-$j('$group') not exists, now creating..."
        "${MQADMIN_CMD}" updateSubGroup -n "${MQNAMESRV_ADDR}" -c DefaultCluster -g "$group"

        retry_topic="%RETRY%$group"
        log "[+++] Consumer#$ii-$j('$group')'s related retry topic('$retry_topic') is creating..."
        "${MQADMIN_CMD}" updateTopic -n "${MQNAMESRV_ADDR}" -c DefaultCluster -t "$retry_topic" -r 8 -w 8
      else
        log "[--] Consumer#$ii-$j('$group')' already exists."
      fi
      j=$((j + 1))
    done

    log "+ Topic#$ii('$topic') is ready! (with it's consumers and retry topics)"
  ) &
  i=$((i + 1))
done

wait

end_time=$(date +%s)
elapsed=$((end_time - start_time))
log "Total init duration: ${elapsed}s"
print_banner "Completed!"