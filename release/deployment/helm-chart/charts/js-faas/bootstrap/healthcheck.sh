#!/bin/sh

set -e

# 使用Deno检查JavaScript FaaS的健康状态
if deno eval "try { const resp = await fetch('http://localhost:8000/health'); if (resp.ok) { const data = await resp.json(); console.log('Health:', data.status); Deno.exit(0); } else { Deno.exit(1); } } catch (e) { console.error(e); Deno.exit(1); }" 2>/dev/null; then
  exit 0
else
  exit 1
fi