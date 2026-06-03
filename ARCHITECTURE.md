# ARCHITECTURE.md — Coze Loop

> Coze Loop 是一个开源的 LLM 评测与可观测性平台，采用 Go 后端 + TypeScript/React 前端的多语言单体仓库架构。

## 全景视图

```
┌─────────────────────────────────────────────────────────────────┐
│                        coze-loop 仓库                           │
│                                                                 │
│  ┌──────────────┐   ┌───────────────┐   ┌────────────────────┐  │
│  │   frontend/   │   │    backend/    │   │     release/       │  │
│  │  Rush.js SPA  │──▶│  Go DDD 服务   │   │  Docker / Helm    │  │
│  │  59 packages  │   │  6 业务模块    │   │  部署配置          │  │
│  └──────┬───────┘   └───────┬───────┘   └────────────────────┘  │
│         │                   │                                    │
│         └───────┬───────────┘                                    │
│                 ▼                                                │
│          ┌────────────┐                                          │
│          │ idl/thrift/ │  Thrift IDL（前后端共享契约）            │
│          └────────────┘                                          │
│                                                                 │
│  ┌──────────────┐   ┌───────────────┐   ┌────────────────────┐  │
│  │   common/     │   │  .github/      │   │    Makefile        │  │
│  │  git-hooks    │   │  workflows/    │   │  image / deploy   │  │
│  └──────────────┘   └───────────────┘   └────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

## 代码地图

### Backend (`backend/`)

Go 服务，模块路径 `github.com/coze-dev/coze-loop/backend`。

| 目录 | 职责 | 架构不变量 |
|------|------|-----------|
| `cmd/` | 服务入口（main.go HTTP, consumer.go MQ） | 只做启动编排，不含业务逻辑 |
| `api/` | HTTP 路由 + handler（Hertz 框架） | handler 只做参数校验和转发，业务逻辑在 application 层 |
| `modules/` | 6 个 DDD 业务模块 | 模块间不直接互调 |
| `infra/` | 共享基础设施（DB, Redis, ClickHouse, MQ, HTTP, middleware） | 被 modules 引用，不引用 modules |
| `pkg/` | 共享工具库（errors, JSON, logging, context cache） | 纯工具，无业务依赖 |
| `kitex_gen/` | Kitex/Thrift 生成代码 | **自动生成，禁止手动修改** |
| `loop_gen/` | 其他生成代码 | **自动生成，禁止手动修改** |
| `script/` | 代码生成脚本（cloudwego, gorm_gen, errorx） | 生成结果提交到仓库 |

#### Backend DDD 分层

每个 `modules/<domain>/` 内部遵循严格的 DDD 分层:

```
modules/<domain>/
├── application/     # 应用服务（用例编排、Wire DI）
│   ├── wire.go      # DI 定义
│   └── wire_gen.go  # DI 生成代码
├── domain/          # 领域模型（entity, repo 接口, service）
│   ├── entity/
│   ├── repo/        # 仓储接口定义
│   └── service/
├── infra/           # 基础设施实现（repo 实现, RPC, MQ, storage）
│   ├── repo/        # 仓储接口实现
│   ├── mq/  rpc/  storage/
│   └── ...
├── pkg/             # 模块内工具（errno, utils）
└── consts/          # 模块常量
```

**依赖方向**: `api/ → application/ → domain/ ← infra/`。domain 层定义接口，infra 层实现。domain **绝不**引用 infra。

#### Backend 技术栈

- **HTTP**: Hertz (CloudWeGo)
- **RPC**: Kitex (CloudWeGo) + Thrift
- **ORM**: GORM（MySQL, ClickHouse）
- **DI**: Wire (Google)
- **LLM**: Eino
- **存储**: MySQL（主存储）、ClickHouse（分析查询）、Redis（缓存/锁）、RocketMQ（异步消息）

### Frontend (`frontend/`)

Rush.js 5.172.1 管理的 TypeScript/React 单体仓库，pnpm 10.27.0，59 个包。

| 层级 | 目录 | 包数 | 职责 |
|------|------|------|------|
| Level-6 | `apps/cozeloop/` | 1 | 主 SPA（React 18, Rsbuild, react-router, zustand） |
| Level-5 | `packages/loop-pages/` | 5 | 页面模块（auth, evaluate, observation, prompt, tag） |
| Level-4 | `packages/loop-modules/` | 1 | 高阶业务模块（evaluate） |
| Level-3 | `packages/loop-components/` | 13 | UI 组件包 + adapter 模式 |
| Level-2 | `packages/loop-base/` | 20 | 基础库（account, api-schema, hooks, components, env, i18n, stores, route...） |
| Level-1 | `config/` + `infra/` | 10+ | 工具链配置 + ESLint 插件 + IDL 转换 |

**依赖不变量**: 高层级只能依赖低层级，禁止反向依赖。

#### Adapter 模式

用于解耦商业版与开源版差异:

```
adapter-interfaces/   # 接口定义（Level-3）
    ↑
*-adapter/            # 接口实现（Level-3，按 evaluate/observation 等分）
    ↑
components-with-adapter/  # 消费者（Level-3）
```

新增商业版/开源版差异必须走 adapter 接口，不可在组件中硬编码条件分支。

### IDL (`idl/thrift/`)

Thrift IDL 定义，是前后端的共享契约。修改 IDL 后需分别运行:
- 后端: `backend/script/cloudwego/` 重新生成 Go 代码
- 前端: `infra/idl/` 工具将 Thrift 转换为 TypeScript 类型

### Release (`release/`)

| 目录 | 内容 |
|------|------|
| `release/image/` | Dockerfile（主服务, debug, python-faas） |
| `release/deployment/docker-compose/` | Docker Compose 本地部署 |
| `release/deployment/helm-chart/` | Helm Chart (Kubernetes 部署) |

Makefile 提供 `image-*`、`compose-*`、`helm-*` 等快捷目标。

## 横切关注点

### 代码生成

| 生成物 | 触发方式 | 输入 |
|--------|---------|------|
| `kitex_gen/` | `backend/script/cloudwego/` | `idl/thrift/` |
| `loop_gen/` | 相关脚本 | IDL / schema |
| `wire_gen.go` | 各 module `application/` 目录下执行 `wire` | `wire.go` |
| `router_gen.go` | Hertz 代码生成 | 路由定义 |

### CI/CD

`.github/workflows/` 共 10 个工作流:

| 工作流 | 触发 | 作用 |
|--------|------|------|
| `backend-ci.yaml` | PR (backend/**) | Go test + lint + Codecov |
| `frontend-ci.yaml` | PR (frontend/**) | Rush build + Vitest + lint |
| `frontend-tsc-ci.yaml` | PR (frontend/**) | TypeScript 类型检查 |
| `idl.yaml` | PR (idl/**) | IDL 变更检查 |
| `mysql-schema-check.yaml` | PR | 数据库 schema 变更检查 |
| `semantic-pull-request.yaml` | PR | PR 标题规范检查 |
| `license-check.yaml` | PR | License 合规检查 |

### Git Hooks (`common/git-hooks/`)

由 Rush 管理，包含 pre-commit, commit-msg, pre-push, post-checkout, post-commit, post-merge。详见 `CONTRIBUTING.md`。
