#!/bin/sh

set -e

if clickhouse-client \
    -u "${COZE_LOOP_CLICKHOUSE_USER}" \
    --password="${COZE_LOOP_CLICKHOUSE_PASSWORD}" \
    --query "SELECT 1" \
    2>/dev/null \
    | grep -q 1; then
  exit 0
else
  exit 1
fi