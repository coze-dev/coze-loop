#!/bin/sh

set -e

curl \
  -f "http://localhost:9000/minio/health/live" \
  > /dev/null 2>&1