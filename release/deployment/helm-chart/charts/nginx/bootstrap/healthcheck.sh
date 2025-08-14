#!/bin/sh

set -e

if curl \
    -s http://localhost:80 \
    2>/dev/null \
    | grep -Eq cozeloop; then
  exit 0
else
  exit 1
fi