#!/bin/bash

set -e

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

# 生成字符串的 MD5 哈希值(跨平台)
generate_md5() {
  local input="$1"
  if command -v md5sum &> /dev/null; then
    # Linux
    echo -n "$input" | md5sum | cut -d' ' -f1
  elif command -v md5 &> /dev/null; then
    # macOS
    echo -n "$input" | md5
  else
    # 如果都不可用,使用项目路径的简单哈希(作为后备)
    echo -n "$input" | cksum | cut -d' ' -f1
  fi
}

# 在给定端口范围内找到一个可用端口
find_available_port() {
  local start_port=${CHROME_PORT_START:-9222}
  local end_port=${CHROME_PORT_END:-9350}

  for port in $(seq $start_port $end_port); do
    # 使用 ss 命令(Linux)或 lsof 命令(macOS)检查端口
    if command -v ss &> /dev/null; then
      if ! ss -tuln | grep -q ":$port "; then
        echo $port
        return 0
      fi
    elif command -v lsof &> /dev/null; then
      if ! lsof -i :$port > /dev/null 2>&1; then
        echo $port
        return 0
      fi
    else
      # 如果两个命令都不可用,使用 netstat 作为后备
      if ! netstat -tuln 2>/dev/null | grep -q ":$port "; then
        echo $port
        return 0
      fi
    fi
  done
  echo "Error: No available port in range $start_port-$end_port" >&2
  return 1
}

# 等待 Chrome 在指定端口启动完成
wait_for_chrome() {
  local port=$1
  local max_attempts=30
  local attempt=0

  while [ $attempt -lt $max_attempts ]; do
    if curl -s "http://localhost:$port/json/version" > /dev/null 2>&1; then
      return 0
    fi
    sleep 0.5
    attempt=$((attempt + 1))
  done

  echo "Error: Chrome failed to start on port $port" >&2
  return 1
}

# 找到 Chrome 可执行文件
CHROME_BIN=$(find_chrome_binary)
if [ $? -ne 0 ]; then
  exit 1
fi

# 找到可用端口
PORT=$(find_available_port)
if [ $? -ne 0 ]; then
  exit 1
fi

# 获取当前项目的绝对路径并生成哈希值
PROJECT_PATH=$(cd "$(dirname "$0")/.." && pwd)
PROJECT_HASH=$(generate_md5 "$PROJECT_PATH")

# 为每个项目创建独立的用户数据目录
USER_DATA_DIR="/tmp/chrome-mcp-project-${PROJECT_HASH}-$PORT"

# 设置中文 locale 环境变量
export LANG="zh_CN.UTF-8"
export LC_ALL="zh_CN.UTF-8"

# 启动 Chrome 实例
$CHROME_BIN \
  --headless=new \
  --remote-debugging-port=$PORT \
  --user-data-dir="$USER_DATA_DIR" \
  --no-first-run \
  --no-default-browser-check \
  --disable-background-networking \
  --disable-sync \
  --no-sandbox \
  --disable-dev-shm-usage \
  --disable-gpu \
  --lang=zh-CN \
  --font-render-hinting=none \
  --force-device-scale-factor=1 \
  > /dev/null 2>&1 &

CHROME_PID=$!

# 确保脚本退出时清理 Chrome 进程
cleanup() {
  kill $CHROME_PID 2>/dev/null || true
}
trap cleanup EXIT

# 等待 Chrome 启动完成
if ! wait_for_chrome $PORT; then
  exit 1
fi

# 尝试多个可能的包名
# 1. 尝试 @modelcontextprotocol/server-puppeteer (官方 MCP 包)
# 2. 回退到原始的 chrome-devtools-mcp
PACKAGES=(
  "@modelcontextprotocol/server-puppeteer"
  "chrome-devtools-mcp"
)

for package in "${PACKAGES[@]}"; do
  echo "尝试使用包: $package" >&2
  if npx -y "$package@latest" --browserUrl "http://127.0.0.1:$PORT" 2>&1; then
    exit 0
  fi
done

echo "Error: 所有包都无法启动" >&2
exit 1
