# 基于Deno官方镜像
FROM denoland/deno:1.45.5

# 安装Python3和必要的工具
RUN apt-get update && \
    apt-get install -y python3 python3-pip curl && \
    rm -rf /var/lib/apt/lists/*

# 创建工作目录
WORKDIR /app

# 复制启动脚本和服务器代码
COPY . /app/bootstrap/

# 设置权限
RUN chmod +x /app/bootstrap/entrypoint.sh

# 预热Deno缓存
RUN deno cache --reload /app/bootstrap/faas-enhanced-server.ts || echo "Cache warming failed, continuing..."

# 暴露端口
EXPOSE 8000

# 设置入口点
ENTRYPOINT ["/app/bootstrap/entrypoint.sh"]