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
  if curl \
      -sf "http://coze-loop-minio:9000/minio/health/live" \
      > /dev/null; then
    break
  else
    sleep 1
  fi
  if [ "$i" -eq 60 ]; then
    echo "[ERROR] MinIO server or bucket('${COZE_LOOP_OSS_BUCKET}') not available after 60 time."
    exit 1
  fi
done

export MC_HOST_myminio="http://${COZE_LOOP_OSS_USER}:${COZE_LOOP_OSS_PASSWORD}@coze-loop-minio:9000"

echo "+ check bucket($COZE_LOOP_OSS_BUCKET) exists..."
if mc ls myminio/"${COZE_LOOP_OSS_BUCKET}" >/dev/null 2>&1; then
  echo "+ bucket already exists: ${COZE_LOOP_OSS_BUCKET}"
else
  echo "+ bucket not found. Creating: ${COZE_LOOP_OSS_BUCKET}"
  mc mb --quiet myminio/"${COZE_LOOP_OSS_BUCKET}"
fi

print_banner "Completed!"