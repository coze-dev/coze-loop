#!/bin/bash

set -e

# =============================================================================
# Chrome DevTools MCP 启动脚本
#
# 支持两种模式：
# 1. 池模式（推荐）：如果设置了 CHROME_DEBUG_PORT 环境变量，直接连接到已运行的 Chrome
# 2. 独立模式：启动新的 Chrome 实例
# =============================================================================

# 调试输出：显示当前环境变量
echo "[chrome-devtools-mcp.sh] Starting..." >&2
echo "[chrome-devtools-mcp.sh] CHROME_DEBUG_PORT=${CHROME_DEBUG_PORT:-<not set>}" >&2

# 如果设置了 CHROME_DEBUG_PORT 环境变量，直接连接到已运行的 Chrome
if [ -n "$CHROME_DEBUG_PORT" ]; then
  echo "使用池模式：连接到已运行的 Chrome (端口: $CHROME_DEBUG_PORT)" >&2

  # 等待 Chrome 就绪
  max_attempts=60
  attempt=0
  while [ $attempt -lt $max_attempts ]; do
    if curl -s "http://127.0.0.1:$CHROME_DEBUG_PORT/json/version" > /dev/null 2>&1; then
      break
    fi
    sleep 0.5
    attempt=$((attempt + 1))
  done

  if [ $attempt -eq $max_attempts ]; then
    echo "Error: Chrome not available on port $CHROME_DEBUG_PORT" >&2
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

  # 为每个实例创建独立的 npm 缓存目录，避免并发时的缓存竞争
  NPX_CACHE_DIR="/tmp/npm-cache-mcp-$$"
  mkdir -p "$NPX_CACHE_DIR"

  # 直接连接到已运行的 Chrome 实例
  # 注意：不使用 exec，以便 trap 能正常工作清理缓存目录
  npx --cache "$NPX_CACHE_DIR" -y chrome-devtools-mcp@latest --browserUrl "http://127.0.0.1:$CHROME_DEBUG_PORT"
  exit_code=$?
  rm -rf "$NPX_CACHE_DIR" 2>/dev/null || true
  exit $exit_code
fi

# =============================================================================
# 独立模式：启动新的 Chrome 实例
# =============================================================================

# 检测操作系统并找到 Chrome 可执行文件
find_chrome_binary() {
  if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    # Linux 系统 - 固定使用 Chrome 143 版本（支持现代 ES2022+ 语法）
    CHROME_143="$HOME/.cache/puppeteer/chrome/linux-143.0.7499.42/chrome-linux64/chrome"
    if [ -x "$CHROME_143" ]; then
      echo "$CHROME_143"
      return 0
    else
      echo "Error: Chrome 143 not found at $CHROME_143" >&2
      echo "Please run: npx puppeteer browsers install chrome@143.0.7499.42" >&2
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

# 生成字符串的 MD5 哈希值（跨平台）
generate_md5() {
  local input="$1"
  if command -v md5sum &> /dev/null; then
    # Linux
    echo -n "$input" | md5sum | cut -d' ' -f1
  elif command -v md5 &> /dev/null; then
    # macOS
    echo -n "$input" | md5
  else
    # 如果都不可用，使用项目路径的简单哈希（作为后备）
    echo -n "$input" | cksum | cut -d' ' -f1
  fi
}

# 在给定端口范围内找到一个可用端口
find_available_port() {
  local start_port=${CHROME_PORT_START:-9222}
  local end_port=${CHROME_PORT_END:-9350}

  for port in $(seq $start_port $end_port); do
    # 使用 ss 命令（Linux）或 lsof 命令（macOS）检查端口
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
      # 如果两个命令都不可用，使用 netstat 作为后备
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

# 为每个实例创建独立的 npm 缓存目录，避免并发时的缓存竞争
NPX_CACHE_DIR="/tmp/npm-cache-mcp-$$-$PORT"
mkdir -p "$NPX_CACHE_DIR"

# 确保脚本退出时清理 Chrome 进程和 npm 缓存目录
cleanup() {
  kill $CHROME_PID 2>/dev/null || true
  rm -rf "$NPX_CACHE_DIR" 2>/dev/null || true
}
trap cleanup EXIT

# 等待 Chrome 启动完成
if ! wait_for_chrome $PORT; then
  exit 1
fi

# 确保 npx 能找到全局安装的包和依赖
# 添加常见的 npm 全局 bin 目录到 PATH
export PATH="$HOME/.npm-global/bin:$PATH"
export PATH="$HOME/.npm/bin:$PATH"
# 尝试添加 npm 的全局 node_modules/.bin 目录
if command -v npm &> /dev/null; then
  NPM_GLOBAL_ROOT=$(npm root -g 2>/dev/null || echo "")
  if [ -n "$NPM_GLOBAL_ROOT" ] && [ -d "$NPM_GLOBAL_ROOT/.bin" ]; then
    export PATH="$NPM_GLOBAL_ROOT/.bin:$PATH"
  fi
fi

# 启动 chrome-devtools-mcp 连接到该 Chrome 实例
# 注意：不使用 exec，以便 trap 能正常工作清理资源
npx --cache "$NPX_CACHE_DIR" -y chrome-devtools-mcp@latest --browserUrl "http://127.0.0.1:$PORT"
