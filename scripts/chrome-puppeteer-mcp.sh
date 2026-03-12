#!/bin/bash

set -e

# 这个脚本使用 Puppeteer MCP Server 替代 chrome-devtools-mcp
# Puppeteer 是更成熟的浏览器自动化工具,有官方 MCP 支持

# 检测操作系统并找到 Chrome 可执行文件
find_chrome_binary() {
  if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    # Linux 系统
    if command -v google-chrome &> /dev/null; then
      echo "google-chrome"
    elif command -v google-chrome-stable &> /dev/null; then
      echo "google-chrome-stable"
    elif command -v chromium &> /dev/null; then
      echo "chromium"
    elif command -v chromium-browser &> /dev/null; then
      echo "chromium-browser"
    else
      echo "Error: Chrome/Chromium not found on Linux" >&2
      return 1
    fi
  elif [[ "$OSTYPE" == "darwin"* ]]; then
    # macOS 系统
    if [ -f "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" ]; then
      echo "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
    else
      echo "Error: Chrome not found on macOS" >&2
      return 1
    fi
  else
    echo "Error: Unsupported operating system: $OSTYPE" >&2
    return 1
  fi
}

# 找到 Chrome 可执行文件
CHROME_BIN=$(find_chrome_binary)
if [ $? -ne 0 ]; then
  exit 1
fi

# 设置 Puppeteer 环境变量,使其使用系统 Chrome
export PUPPETEER_SKIP_CHROMIUM_DOWNLOAD=true
export PUPPETEER_EXECUTABLE_PATH="$CHROME_BIN"

# 使用 @modelcontextprotocol/server-puppeteer
# 这是官方的 Puppeteer MCP 服务器实现
exec npx -y @modelcontextprotocol/server-puppeteer
