#!/bin/sh

set -e

if "${ROCKETMQ_HOME}/bin/mqadmin" \
    clusterList \
    -n coze-loop-rmq-namesrv:9876 \
    2>/dev/null \
    | grep -q DefaultCluster; then
  exit 0
else
  exit 1
fi