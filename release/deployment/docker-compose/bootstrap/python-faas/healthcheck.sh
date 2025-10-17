#!/bin/sh

set -e

echo "🔍 检查Pyodide Python FaaS健康状态..."

# 验证Deno环境
if ! command -v deno >/dev/null 2>&1; then
    echo "❌ Deno 不可用"
    exit 1
fi

# 验证Pyodide可用性
echo "🧪 检查Pyodide可用性..."
if deno run -A jsr:@eyurtsev/pyodide-sandbox -c "print('healthcheck')" >/dev/null 2>&1; then
    echo "✅ Pyodide 可用"
else
    echo "⚠️  Pyodide 可能需要网络连接"
fi

# 使用Deno检查Python FaaS的健康状态
if deno eval "try { const resp = await fetch('http://localhost:8000/health'); if (resp.ok) { const data = await resp.json(); if (data.status === 'healthy') { console.log('✅ Health: OK'); Deno.exit(0); } else { console.log('⚠️  Health: Degraded'); Deno.exit(1); } } else { console.log('❌ Health: HTTP Error'); Deno.exit(1); } } catch (e) { console.error('❌ Health check failed:', e); Deno.exit(1); }" 2>/dev/null; then
  echo "✅ 健康检查通过"
  exit 0
else
  echo "❌ 健康检查失败"
  exit 1
fi