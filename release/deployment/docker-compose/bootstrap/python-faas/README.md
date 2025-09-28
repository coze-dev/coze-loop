# Python FaaS 服务

## 概述

基于 Deno + Python 的安全代码执行服务，提供真实的 Python 代码执行能力。

## 核心文件

- `enhanced_python_faas_server.ts` - 真实执行器实现
- `Dockerfile` - Docker 镜像构建文件  
- `README.md` - 本文档

## 部署配置

服务通过 Docker Compose 部署，配置位于 `docker-compose.yml` 中的 `coze-loop-python-faas` 服务。

### 关键配置
- 基础镜像：`denoland/deno:1.45.5`
- 端口：8000 (内部) -> 8890 (外部，可配置)
- 安全特性：容器隔离、权限控制、代码安全检查
- 资源限制：4GB 内存、2 CPU 核心（可配置）

## 使用方法

### 启动服务
```bash
docker-compose up -d coze-loop-python-faas
```

### 健康检查
```bash
curl http://localhost:8890/health
```

### 执行 Python 代码
```bash
curl -X POST http://localhost:8890/run_code \
  -H "Content-Type: application/json" \
  -d '{"language":"python","code":"print(\"Hello, World!\")"}'
```

## 安全特性

- **Deno 权限控制**：严格的权限模型
- **代码安全检查**：检测和阻止危险代码模式  
- **模块导入控制**：黑名单策略，允许 `sys` 和 `ast` 模块，阻止其他危险模块
- **危险函数检测**：阻止 `exec`、`eval`、`open` 等危险函数
- **容器隔离**：Docker 容器级别的安全隔离