#!/bin/bash

set -e

# 共享模式的 chrome-devtools-mcp 脚本
# 连接到预先启动的 Chrome 实例，而不是启动新实例

# 默认端口，可通过环境变量覆盖
CHROME_PORT=${CHROME_DEBUG_PORT:-9222}

# 检查 Chrome 是否已在指定端口运行
check_chrome_running() {
  local port=$1
  if curl -s "http://127.0.0.1:$port/json/version" > /dev/null 2>&1; then
    return 0
  fi
  return 1
}

# 等待 Chrome 就绪
wait_for_chrome() {
  local port=$1
  local max_attempts=60
  local attempt=0

  while [ $attempt -lt $max_attempts ]; do
    if check_chrome_running $port; then
      return 0
    fi
    sleep 0.5
    attempt=$((attempt + 1))
  done

  echo "Error: Chrome not available on port $port after ${max_attempts} attempts" >&2
  return 1
}

# 确保 Chrome 在指定端口可用
if ! wait_for_chrome $CHROME_PORT; then
  echo "Error: Chrome is not running on port $CHROME_PORT" >&2
  echo "Please start Chrome with: google-chrome --headless=new --remote-debugging-port=$CHROME_PORT" >&2
  exit 1
fi

# 确保 npx 能找到全局安装的包
export PATH="$HOME/.npm-global/bin:$PATH"
export PATH="$HOME/.npm/bin:$PATH"
if command -v npm &> /dev/null; then
  NPM_GLOBAL_ROOT=$(npm root -g 2>/dev/null || echo "")
  if [ -n "$NPM_GLOBAL_ROOT" ] && [ -d "$NPM_GLOBAL_ROOT/.bin" ]; then
    export PATH="$NPM_GLOBAL_ROOT/.bin:$PATH"
  fi
fi

# 启动 chrome-devtools-mcp 连接到已运行的 Chrome 实例
exec npx -y chrome-devtools-mcp@latest --browserUrl "http://127.0.0.1:$CHROME_PORT"
