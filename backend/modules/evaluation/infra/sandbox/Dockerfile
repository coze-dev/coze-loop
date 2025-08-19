# 多阶段构建 Dockerfile for 代码评估器沙箱环境

# 第一阶段：构建阶段
FROM golang:1.23-alpine AS builder

# 安装必要的系统依赖
RUN apk add --no-cache git ca-certificates tzdata

# 设置工作目录
WORKDIR /app

# 复制 go mod 文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
WORKDIR /app/modules/evaluation/sandbox
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o sandbox ./cmd/demo

# 第二阶段：运行阶段
FROM debian:bullseye-slim

# 安装运行时依赖
RUN apt-get update && apt-get install -y \
    ca-certificates \
    curl \
    unzip \
    && rm -rf /var/lib/apt/lists/*

# 安装 Deno (用于JavaScript和Python执行)
RUN curl -fsSL https://deno.land/x/install/install.sh | sh
ENV PATH="/root/.deno/bin:${PATH}"

# 验证Deno安装
RUN deno --version

# 创建非root用户
RUN useradd -r -s /bin/false sandbox

# 设置工作目录
WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/modules/evaluation/sandbox/sandbox .

# 复制Deno运行时文件
COPY modules/evaluation/sandbox/infra/pyodide/pyodide_runner.ts ./

# 创建临时目录
RUN mkdir -p /tmp/sandbox && chown sandbox:sandbox /tmp/sandbox

# 设置权限
RUN chown -R sandbox:sandbox /app
RUN chmod +x /app/sandbox

# 切换到非root用户
USER sandbox

# 暴露端口
EXPOSE 8080

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD curl -f http://localhost:8080/api/v1/sandbox/health || exit 1

# 启动应用
CMD ["./sandbox"]