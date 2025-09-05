#!/bin/sh

set -e

echo "=========================================="
echo "Starting coze-loop-faas-unified container..."
echo "=========================================="

# 环境变量检查
echo "Environment variables:"
echo "  DENO_DIR: ${DENO_DIR}"
echo "  FAAS_WORKSPACE: ${FAAS_WORKSPACE}"
echo "  FAAS_PORT: ${FAAS_PORT}"
echo "  FAAS_TIMEOUT: ${FAAS_TIMEOUT}"
echo "  FAAS_MODE: ${FAAS_MODE:-enhanced}"

# 模式特定配置
if [ "${FAAS_MODE:-enhanced}" = "enhanced" ]; then
    echo "Enhanced mode configuration:"
    echo "  FAAS_POOL_SIZE: ${FAAS_POOL_SIZE:-10}"
    echo "  FAAS_MAX_INSTANCES: ${FAAS_MAX_INSTANCES:-50}"
    echo "  FAAS_WORKER_COUNT: ${FAAS_WORKER_COUNT:-10}"
    echo "  FAAS_ENABLE_POOL: ${FAAS_ENABLE_POOL:-true}"
    echo "  FAAS_ENABLE_SCHEDULER: ${FAAS_ENABLE_SCHEDULER:-true}"
    echo "  FAAS_ENABLE_METRICS: ${FAAS_ENABLE_METRICS:-true}"
else
    echo "Basic mode: Simple code execution without pool management"
fi

# 创建必要的目录
echo "Creating necessary directories..."
mkdir -p "${FAAS_WORKSPACE}"
mkdir -p "${DENO_DIR}"

# 设置目录权限
chmod 755 "${FAAS_WORKSPACE}"
chmod 755 "${DENO_DIR}"

# Deno 环境验证
echo "Verifying Deno environment..."
deno --version

# 检查 Python 是否可用（用于 Python 代码执行）
if command -v python3 >/dev/null 2>&1; then
    echo "Python3 is available: $(python3 --version)"
else
    echo "Warning: Python3 is not available, Python code execution will use Pyodide only"
fi

# 检查启动脚本是否存在
if [ ! -f "/app/bootstrap/faas-unified-server.ts" ]; then
    echo "Error: faas-unified-server.ts not found!"
    exit 1
fi

# 预热Deno缓存
echo "Pre-warming Deno cache..."
deno cache --reload /app/bootstrap/faas-unified-server.ts || echo "Cache warming failed, continuing..."

echo "Environment check completed successfully!"

# 根据模式显示启动信息
if [ "${FAAS_MODE:-enhanced}" = "enhanced" ]; then
    echo "Starting Unified FaaS server in Enhanced mode with sandbox pool and task scheduler..."
else
    echo "Starting Unified FaaS server in Basic mode..."
fi

# 启动统一 FaaS 服务器
exec deno run --allow-all /app/bootstrap/faas-unified-server.ts