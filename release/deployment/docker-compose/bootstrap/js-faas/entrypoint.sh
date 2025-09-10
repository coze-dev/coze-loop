#!/bin/bash

# JavaScript FaaS服务启动脚本

set -e

echo "启动JavaScript FaaS服务..."

# 确保工作空间目录存在
mkdir -p "${FAAS_WORKSPACE:-/tmp/faas-workspace}"

# 启动JavaScript FaaS服务器
exec deno run --allow-all /app/bootstrap/js_faas_server.ts