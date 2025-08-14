#!/bin/sh

set -e

if "${ROCKETMQ_HOME}/bin/mqadmin" \
    topicList \
    -n localhost:9876 \
    > /dev/null 2>&1; then
  exit 0
else
  exit 1
fi