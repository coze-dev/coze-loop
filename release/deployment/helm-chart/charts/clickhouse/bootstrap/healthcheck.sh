#!/bin/sh

set -e

clickhouse-client \
  -u "${COZE_LOOP_CLICKHOUSE_USER}" \
  --password="${COZE_LOOP_CLICKHOUSE_PASSWORD}" \
  --query "SELECT 1" \
  > /dev/null 2>&1