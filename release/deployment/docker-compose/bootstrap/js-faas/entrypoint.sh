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

print_banner "Starting JavaScript FaaS..."

# 确保工作空间目录存在
mkdir -p "${FAAS_WORKSPACE:-/tmp/faas-workspace}"

# 后台健康检查循环
(
  while true; do
    if sh /coze-loop-js-faas/bootstrap/healthcheck.sh; then
      print_banner "JavaScript FaaS Completed!"
      break
    else
      sleep 1
    fi
  done
)&

# 启动JavaScript FaaS服务器
exec deno run --allow-net=0.0.0.0:8000 --allow-env --allow-read=/coze-loop-js-faas/bootstrap,/tmp --allow-write=/tmp --allow-run /coze-loop-js-faas/bootstrap/js_faas_server.ts
