#!/bin/bash

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
    echo "[ERROR] RMQ broker not available after 60 time."
    exit 1
  fi
done

i=1
for topic in "${!topics[@]}"; do
  ii=$i
  (
    echo "+ Check if topic#$ii('$topic') exists..."
    if ! "${MQADMIN_CMD}" topicList -n "${MQNAMESRV_ADDR}" | grep -q "^$topic$"; then
      echo "[+] Topic#$ii('$topic') not exists, now creating..."
      "${MQADMIN_CMD}" updateTopic -n "${MQNAMESRV_ADDR}" -c DefaultCluster -t "$topic" -r 8 -w 8
    else
      echo "[-] Topic#$ii('$topic') already exists."
    fi

    IFS=',' read -ra consumer_groups <<< "${topics[$topic]}"
    j=1
    for group in "${consumer_groups[@]}"; do
      echo "++ Check if consumer#$ii-$j('$group') exists..."
      if ! "${MQADMIN_CMD}" consumerProgress -n "${MQNAMESRV_ADDR}" | grep -q "^$group$"; then
        echo "[++] Consumer#$ii-$j('$group') not exists, now creating..."
        "${MQADMIN_CMD}" updateSubGroup -n "${MQNAMESRV_ADDR}" -c DefaultCluster -g "$group"

        retry_topic="%RETRY%$group"
        echo "[+++] Consumer#$ii-$j('$group')'s related retry topic('$retry_topic') is creating..."
        "${MQADMIN_CMD}" updateTopic -n "${MQNAMESRV_ADDR}" -c DefaultCluster -t "$retry_topic" -r 8 -w 8
      else
        echo "[--] Consumer#$ii-$j('$group')' already exists."
      fi
      j=$((j + 1))
    done

    echo "+ Topic#$ii('$topic') is ready! (with it's consumers and retry topics)"
  ) &
  i=$((i + 1))
done

wait

print_banner "Completed!"
