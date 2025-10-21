# Python FaaS Pyodide 版 Dockerfile
# 基于 Deno 官方镜像，使用 Pyodide WebAssembly Python

FROM denoland/deno:1.45.5

# 设置环境变量
ENV DENO_DIR=/tmp/faas-workspace/.deno
ENV DENO_NO_UPDATE_CHECK=1
ENV FAAS_WORKSPACE=/tmp/faas-workspace
ENV FAAS_PORT=8000
ENV FAAS_TIMEOUT=30000
ENV FAAS_LANGUAGE=python
ENV PYODIDE_VERSION=0.26.2

# 池化配置
ENV FAAS_POOL_MIN_SIZE=2
ENV FAAS_POOL_MAX_SIZE=8
ENV FAAS_POOL_IDLE_TIMEOUT=300000
ENV FAAS_MAX_EXECUTION_TIME=30000
ENV FAAS_PRELOAD_TIMEOUT=60000

# 安装系统依赖
USER root
RUN apt-get update && apt-get install -y \
    curl \
    unzip \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# 创建工作目录
WORKDIR /app

# 复制 Deno 配置文件和脚本
COPY deno.json ./
COPY entrypoint.sh ./
COPY healthcheck.sh ./
COPY pyodide_faas_server.ts ./
COPY pyodide_pool_manager.ts ./

# 预先 vendor 远程依赖并打包到镜像
RUN mkdir -p /tmp/faas-workspace/vendor && \
    deno vendor jsr:@eyurtsev/pyodide-sandbox@0.0.3 --output=/tmp/faas-workspace/vendor && \
    echo '{"imports":{"https://jsr.io/":"./jsr.io/"},"scopes":{"./jsr.io/":{"jsr:@eyurtsev/pyodide-sandbox@0.0.3":"./jsr.io/@eyurtsev/pyodide-sandbox/0.0.3/main.ts","jsr:@std/path@^1.0.8":"./jsr.io/@std/path/1.1.2/mod.ts","jsr:/@std/cli@^1.0.16/parse-args":"./jsr.io/@std/cli/1.0.23/parse_args.ts","jsr:@std/internal@^1.0.10/os":"./jsr.io/@std/internal/1.0.12/os.ts"}}}' > /tmp/faas-workspace/vendor/import_map.json && \
    # 复制 vendor 文件到 /app 作为备份（用于 emptyDir 挂载恢复）
    mkdir -p /app/vendor && \
    cp -r /tmp/faas-workspace/vendor/* /app/vendor/

# 确保脚本有可执行权限
RUN chmod +x entrypoint.sh healthcheck.sh

# 创建非 root 用户
RUN groupadd -r faas && useradd -r -g faas faas && \
    mkdir -p /tmp/faas-workspace && \
    chown -R faas:faas /app /tmp/faas-workspace

# 切换到非 root 用户
USER faas

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD deno eval "try { const resp = await fetch('http://localhost:8000/health'); if (resp.ok) { Deno.exit(0); } else { Deno.exit(1); } } catch (e) { Deno.exit(1); }"


