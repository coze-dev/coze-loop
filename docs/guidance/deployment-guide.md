# 部署与环境配置指南

> 本文档汇总 Docker Compose 和 Helm Chart 两种部署方式的配置细节和常用操作。
> 快速开始见 [`../../README.md`](../../README.md)。

## Docker Compose 部署

### 目录结构

```
release/deployment/docker-compose/
├── .env                          # 环境变量（镜像版本、端口、密码等）
├── docker-compose.yml            # 基础服务定义
├── docker-compose-dev.yml        # 开发模式覆盖
├── docker-compose-debug.yml      # 调试模式覆盖（含 Delve 远程调试）
├── conf/
│   └── model_config.yaml         # LLM 模型配置
└── bootstrap/                    # 初始化脚本
```

### 关键环境变量 (`.env`)

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `COZE_LOOP_APP_IMAGE_TAG` | `1.5.1` | 应用镜像版本 |
| `COZE_LOOP_APP_OPENAPI_PORT` | `8888` | OpenAPI 端口 |
| `COZE_LOOP_APP_DEBUG_PORT` | `40000` | 远程调试端口 |
| `COZE_LOOP_REDIS_PORT` | `6379` | Redis 端口 |
| `COZE_LOOP_REDIS_PASSWORD` | `cozeloop-redis` | Redis 密码 |

### 常用 Makefile 命令

| 命令 | 说明 |
|------|------|
| `make compose-up` | 启动基础服务 |
| `make compose-down` | 停止基础服务 |
| `make compose-down-v` | 停止并删除 volumes |
| `make compose-up-dev` | 启动开发模式（含构建） |
| `make compose-down-dev` | 停止开发模式 |
| `make compose-up-debug` | 启动调试模式（含 Delve） |
| `make compose-down-debug` | 停止调试模式 |
| `make compose-restart-<svc>` | 重启指定基础服务 |
| `make compose-restart-dev-<svc>` | 重启指定开发服务 |

### 访问地址

- 应用: `http://localhost:8082`

## Helm Chart 部署 (Kubernetes)

### 目录结构

```
release/deployment/helm-chart/umbrella/
├── Chart.yaml
├── values.yaml                   # 默认配置
├── templates/
│   └── ingress.yaml              # Ingress 配置（需按集群调整）
├── conf/
│   └── model_config.yaml         # LLM 模型配置
└── examples/
    └── minikube/                 # Minikube 示例 values
```

### 常用 Makefile 命令

| 命令 | 说明 |
|------|------|
| `make helm-up` | 部署/升级 Helm Release |
| `make helm-pod` | 查看 Pod 状态 |
| `make helm-svc` | 查看 Service 列表 |
| `make helm-ingress` | 查看 Ingress 配置 |
| `make helm-logf-app` | 查看应用日志 |
| `make helm-logf-nginx` | 查看 Nginx 日志 |
| `make helm-ctx` | 列出 kube contexts |
| `make helm-ctx-<context>` | 切换 kube context |
| `make helm-ns` | 列出 namespaces |
| `make helm-chart-deps` | 构建 chart 依赖 |
| `make helm-up-exp-minikube-<vals>` | 使用 Minikube 示例 values 部署 |

## 镜像构建

| 命令 | 说明 |
|------|------|
| `make image-<version>` | 构建并推送应用镜像（多架构） |
| `make image-python-faas-bpush-<version>` | 构建并推送 Python FaaS 镜像 |
| `make image--login` | 登录镜像仓库 |

### 镜像信息

- Registry: `docker.io`
- Repository: `cozedev`
- 应用镜像: `cozedev/coze-loop`
- Python FaaS 镜像: `cozedev/coze-loop-python-faas`

## 模型配置

编辑 `conf/model_config.yaml`（Docker Compose 或 Helm Chart 目录下均有）:

- `api_key`: LLM 服务的 API Key
- `model`: 模型 Endpoint ID

支持的模型服务:
- Volcengine Ark（国内）
- BytePlus ModelArk（海外）
- OpenAI 兼容接口
