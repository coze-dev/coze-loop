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

print_banner "RMQ Init Starting..."

MQADMIN_CMD="${ROCKETMQ_HOME}/bin/mqadmin"
MQNAMESRV_ADDR=coze-loop-rmq-namesrv:9876
MQADMIN_MAX_ATTEMPTS=3

retry_mqadmin() {
  local attempt
  for ((attempt = 1; attempt <= MQADMIN_MAX_ATTEMPTS; attempt++)); do
    if "${MQADMIN_CMD}" "$@"; then
      return 0
    fi
    echo "[!] 'mqadmin $1' failed (attempt ${attempt}/${MQADMIN_MAX_ATTEMPTS})" >&2
    sleep 2
  done
  return 1
}

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
pids=()
for topic in "${!topics[@]}"; do
  ii=$i
  (
    echo "+ Check if topic#$ii('$topic') exists..."
    if ! "${MQADMIN_CMD}" topicList -n "${MQNAMESRV_ADDR}" | grep -q "^$topic$"; then
      echo "[+] Topic#$ii('$topic') not exists, now creating..."
      retry_mqadmin updateTopic -n "${MQNAMESRV_ADDR}" -c DefaultCluster -t "$topic" -r 8 -w 8
    else
      echo "[-] Topic#$ii('$topic') already exists."
    fi

    IFS=',' read -ra consumer_groups <<< "${topics[$topic]}"
    j=1
    for group in "${consumer_groups[@]}"; do
      echo "++ Check if consumer#$ii-$j('$group') exists..."
      if ! "${MQADMIN_CMD}" consumerProgress -n "${MQNAMESRV_ADDR}" | grep -q "^$group$"; then
        echo "[++] Consumer#$ii-$j('$group') not exists, now creating..."
        retry_mqadmin updateSubGroup -n "${MQNAMESRV_ADDR}" -c DefaultCluster -g "$group"

        retry_topic="%RETRY%$group"
        echo "[+++] Consumer#$ii-$j('$group')'s related retry topic('$retry_topic') is creating..."
        retry_mqadmin updateTopic -n "${MQNAMESRV_ADDR}" -c DefaultCluster -t "$retry_topic" -r 8 -w 8
      else
        echo "[--] Consumer#$ii-$j('$group')' already exists."
      fi
      j=$((j + 1))
    done

    echo "+ Topic#$ii('$topic') is ready! (with it's consumers and retry topics)"
  ) &
  pids+=("$!")
  i=$((i + 1))
done

init_failed=0
for pid in "${pids[@]}"; do
  if ! wait "${pid}"; then
    init_failed=1
  fi
done

if [ "${init_failed}" -ne 0 ]; then
  echo "[ERROR] RMQ init failed: at least one topic or consumer group could not be created."
  exit 1
fi

registered_topics="$(retry_mqadmin topicList -n "${MQNAMESRV_ADDR}")"
missing_topics=()
for topic in "${!topics[@]}"; do
  if ! grep -q "^${topic}$" <<< "${registered_topics}"; then
    missing_topics+=("${topic}")
  fi
done

if [ "${#missing_topics[@]}" -ne 0 ]; then
  echo "[ERROR] RMQ init failed: topics missing from namesrv route table: ${missing_topics[*]}"
  exit 1
fi

print_banner "RMQ Init Completed!"