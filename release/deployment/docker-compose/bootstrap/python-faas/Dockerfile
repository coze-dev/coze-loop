# Python FaaS Pyodide 版 Dockerfile
# 基于 Deno 官方镜像，使用 Pyodide WebAssembly Python

FROM denoland/deno:1.45.5

# 设置环境变量
ENV DENO_DIR=/tmp/.deno
ENV DENO_NO_UPDATE_CHECK=1
ENV FAAS_WORKSPACE=/tmp/faas-workspace
ENV FAAS_PORT=8000
ENV FAAS_TIMEOUT=30000
ENV FAAS_LANGUAGE=python
ENV PYODIDE_VERSION=0.26.2

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
    CMD deno run -A jsr:@eyurtsev/pyodide-sandbox -c "print('healthcheck')" || deno eval "try { const resp = await fetch('http://localhost:8000/health'); if (resp.ok) { Deno.exit(0); } else { Deno.exit(1); } } catch (e) { Deno.exit(1); }"

# 暴露端口
EXPOSE 8000

# 默认命令 - 使用 entrypoint 脚本
CMD ["/app/entrypoint.sh"]