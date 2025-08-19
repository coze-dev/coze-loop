#!/bin/sh
set -e

if redis-cli -a "${COZE_LOOP_REDIS_PASSWORD}" --no-auth-warning ping | grep -q PONG; then
  exit 0
else
  exit 1
fi