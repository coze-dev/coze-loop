# Python FaaS 增强版 Dockerfile
# 基于 Debian 构建，包含 Deno 和 Python 环境

FROM debian:bookworm-slim

# 设置环境变量
ENV DENO_INSTALL=/usr/local
ENV PATH=$DENO_INSTALL/bin:$PATH
ENV DENO_DIR=/tmp/.deno
ENV DENO_NO_UPDATE_CHECK=1
ENV PYTHONUNBUFFERED=1
ENV PYTHONDONTWRITEBYTECODE=1

# 安装系统依赖
RUN apt-get update && apt-get install -y \
    curl \
    unzip \
    ca-certificates \
    python3 \
    python3-pip \
    python3-venv \
    && rm -rf /var/lib/apt/lists/*

# 安装 Deno - 使用备用方法
RUN curl -fsSL https://github.com/denoland/deno/releases/download/v1.45.5/deno-x86_64-unknown-linux-gnu.zip -o deno.zip && \
    unzip deno.zip && \
    mv deno $DENO_INSTALL/bin/ && \
    chmod +x $DENO_INSTALL/bin/deno && \
    rm deno.zip

# 安装常用的 Python 包（可选）
RUN python3 -m pip install --no-cache-dir \
    numpy \
    pandas \
    matplotlib \
    requests \
    --break-system-packages

# 创建工作目录
WORKDIR /app

# 创建非 root 用户
RUN groupadd -r faas && useradd -r -g faas faas && \
    mkdir -p /tmp/faas-workspace && \
    chown -R faas:faas /app /tmp/faas-workspace

# 切换到非 root 用户
USER faas

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
    CMD deno eval "try { const resp = await fetch('http://localhost:8000/health'); if (resp.ok) { Deno.exit(0); } else { Deno.exit(1); } } catch (e) { Deno.exit(1); }"

# 暴露端口
EXPOSE 8000

# 默认命令
CMD ["deno", "run", "--allow-net=0.0.0.0:8000", "--allow-env", "--allow-read=/app", "--allow-write=/tmp", "--allow-run=python3", "/app/bootstrap/enhanced_python_faas_server.ts"]