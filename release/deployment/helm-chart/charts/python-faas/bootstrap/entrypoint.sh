#!/bin/sh

exec 2>&1
set -e

print_banner() {
  msg="$1"
  side=30
  content=" $msg "
  content_len=${#content}
  line_len=$((side * 2 + content_len))

  line=$(printf '*%.0s' $(seq 1 "$line_len"))
  side_eq=$(printf '*%.0s' $(seq 1 "$side"))

  printf "%s\n%s%s%s\n%s\n" "$line" "$side_eq" "$content" "$side_eq" "$line"
}

print_banner "Starting Pooled Pyodide Python FaaS..."

echo "🔧 验证Deno和Pyodide环境..."
# 验证Deno安装
if command -v deno >/dev/null 2>&1; then
    echo "✅ Deno 已安装: $(deno --version)"
else
    echo "❌ Deno 未安装"
    exit 1
fi

# 验证Pyodide可用性
# 移除启动期的Pyodide网络依赖验证，避免无外网环境失败

# 确保工作空间目录存在
mkdir -p "${FAAS_WORKSPACE:-/tmp/faas-workspace}"

# 验证 vendor 文件是否正确
if [ -f "${FAAS_WORKSPACE:-/tmp/faas-workspace}/vendor/import_map.json" ]; then
    echo "🔍 验证 vendor 文件..."
    if grep -q "pyodide-sandbox" "${FAAS_WORKSPACE:-/tmp/faas-workspace}/vendor/import_map.json"; then
        echo "✅ Vendor 文件包含 pyodide-sandbox 映射"
        # 测试离线执行
        echo "🧪 测试离线执行..."
        deno run -A \
            --import-map="${FAAS_WORKSPACE:-/tmp/faas-workspace}/vendor/import_map.json" \
            "${FAAS_WORKSPACE:-/tmp/faas-workspace}/vendor/jsr.io/@eyurtsev/pyodide-sandbox/0.0.3/main.ts" -c "print('Vendor test successful!')" && \
        echo "✅ 离线执行测试成功" || \
        echo "❌ 离线执行测试失败"
    else
        echo "❌ Vendor 文件不包含 pyodide-sandbox 映射"
    fi
else
    echo "❌ 未找到 vendor 文件"
fi

# 后台健康检查循环
(
  while true; do
    if sh /coze-loop/bootstrap/healthcheck.sh; then
      print_banner "Pyodide Python FaaS Ready!"
      break
    else
      sleep 1
    fi
  done
)&

# 使用池化 Pyodide Python FaaS 服务器
echo "🚀 启动池化 Pyodide Python FaaS 服务器..."
echo "🏊 进程池配置:"
echo "  - 最小进程数: ${FAAS_POOL_MIN_SIZE:-2}"
echo "  - 最大进程数: ${FAAS_POOL_MAX_SIZE:-8}"
echo "  - 空闲超时: ${FAAS_POOL_IDLE_TIMEOUT:-300000}ms"
echo "  - 执行超时: ${FAAS_MAX_EXECUTION_TIME:-30000}ms"

exec deno run \
  --no-lock \
  --allow-net=0.0.0.0:8000 \
  --allow-env \
  --allow-read=/app,/tmp \
  --allow-write=/tmp \
  --allow-run=deno \
  /coze-loop/bootstrap/pyodide_faas_server.ts
