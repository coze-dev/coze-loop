#!/bin/sh

set -e

if wget \
    -qO- http://localhost:8888/ping \
    2>/dev/null \
    | grep -q pong; then
  exit 0
else
  exit 1
fi