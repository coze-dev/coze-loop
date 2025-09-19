#!/bin/bash

# Python FaaS服务启动脚本

set -e

echo "启动Python FaaS服务..."

# 确保工作空间目录存在
mkdir -p "${FAAS_WORKSPACE:-/tmp/faas-workspace}"

# 启动Python FaaS服务器
exec python3 /app/bootstrap/python_faas_server.py