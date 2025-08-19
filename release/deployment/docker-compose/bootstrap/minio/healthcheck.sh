#!/bin/sh

set -e

if curl \
    -sf "http://localhost:9000/minio/health/live" \
    > /dev/null; then
  exit 0
else
  exit 1
fi