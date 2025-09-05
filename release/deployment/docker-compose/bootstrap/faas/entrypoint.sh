#!/bin/sh

set -e

echo "=========================================="
echo "Starting coze-loop-faas container..."
echo "=========================================="

# 环境变量检查
echo "Environment variables:"
echo "  DENO_DIR: ${DENO_DIR}"
echo "  FAAS_WORKSPACE: ${FAAS_WORKSPACE}"
echo "  FAAS_PORT: ${FAAS_PORT}"
echo "  FAAS_TIMEOUT: ${FAAS_TIMEOUT}"

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
    echo "Warning: Python3 is not available, Python code execution will fail"
fi

# 检查启动脚本是否存在
if [ ! -f "/app/bootstrap/faas-server.ts" ]; then
    echo "Error: faas-server.ts not found!"
    exit 1
fi

echo "Environment check completed successfully!"
echo "Starting FaaS server..."

# 启动 FaaS 服务器
exec deno run --allow-all /app/bootstrap/faas-server.ts