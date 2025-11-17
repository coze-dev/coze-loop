#!/bin/sh

set -e

echo "🔍 检查Pyodide Python FaaS健康状态..."

# 验证Deno环境
if ! command -v deno >/dev/null 2>&1; then
    echo "❌ Deno 不可用"
    exit 1
fi

# 移除对 jsr 的远程依赖探测，避免离线/只读环境失败

# 使用Deno检查Python FaaS的健康状态
if deno eval "try { const resp = await fetch('http://localhost:8000/health'); if (resp.ok) { const data = await resp.json(); if (data.status === 'healthy') { console.log('✅ Health: OK'); Deno.exit(0); } else { console.log('⚠️  Health: Degraded'); Deno.exit(1); } } else { console.log('❌ Health: HTTP Error'); Deno.exit(1); } } catch (e) { console.error('❌ Health check failed:', e); Deno.exit(1); }" 2>/dev/null; then
  echo "✅ 健康检查通过"
  exit 0
else
  echo "❌ 健康检查失败"
  exit 1
fi
