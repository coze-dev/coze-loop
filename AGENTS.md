# AGENTS.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

> 开始任务前先通读本文件；涉及架构决策时读 `ARCHITECTURE.md`；进入具体模块时读模块代码。

## 项目概要

Coze Loop 是一个开源的 LLM 评测与可观测性平台，提供 Prompt 开发调试、评测（评测集 + 评估器 + 实验）、可观测性（Trace/Span）、模型集成（OpenAI / Volcengine Ark）等能力。采用 Go 后端（DDD 6 模块，Hertz + Kitex）+ TypeScript/React 前端（Rush.js 59 包）的多语言单体仓库架构，Thrift IDL 共享契约。

## 文档体系

| 层级 | 文件 | 职责 |
|------|------|------|
| L0 入口 | `AGENTS.md`（本文件） | 项目全貌、核心约束、导航 |
| L1 架构 | `ARCHITECTURE.md` | 系统架构、代码地图、依赖关系、不变量 |
| L1 专题 | `docs/AGENTS.md` | 中层索引，串联 reference/ 和 guidance/ |
| L3 参考 | `docs/reference/backend-modules-api.md` | 后端 6 模块职责、分层、API 路由 |
| L3 参考 | `docs/reference/frontend-packages.md` | 前端 59 包分层结构 |
| L3 指南 | `docs/guidance/deployment-guide.md` | Docker Compose / Helm 部署 |
| L3 指南 | `docs/guidance/idl-codegen-guide.md` | IDL 变更后代码生成流程 |

## 导航表

| 我想… | 去哪里 |
|-------|--------|
| 了解仓库整体架构和模块关系 | `ARCHITECTURE.md` |
| 查找 docs 下的专题文档 | `docs/AGENTS.md` |
| 了解后端 6 个 DDD 模块的职责和 API | `docs/reference/backend-modules-api.md` |
| 了解前端 59 包的分层结构 | `docs/reference/frontend-packages.md` |
| 部署 Coze Loop（Docker / Helm） | `docs/guidance/deployment-guide.md` |
| 修改 IDL 后生成代码 | `docs/guidance/idl-codegen-guide.md` |
| 修改数据库表结构 | 本文件「开发流程」→ Step 3-4 |
| 添加新的 API 接口 | 本文件「开发流程」→ Step 1-2 → Step 6 |
| 运行本地开发环境 | `make compose-up-dev` |
| 了解错误码定义规范 | 本文件「开发流程」→ Step 7 |
| 执行 lint 检查 | `cd backend && golangci-lint run --config ../.github/.golangci.yaml --fix` |

## 仓库结构

```
coze-loop/
├── idl/thrift/coze/loop/       # Thrift IDL（前后端共享契约，同步源）
│   ├── <module>/domain/        # 开源领域实体
│   ├── <module>/domain_openapi/# OpenAPI 领域实体
│   └── <module>/coze.loop.*.thrift  # 服务接口
├── backend/
│   ├── cmd/                    # 服务入口（main.go HTTP, consumer.go MQ）
│   ├── api/                    # Hertz 网关层（生成 + handler）
│   ├── modules/                # 6 个 DDD 业务模块
│   │   ├── evaluation/         # 评测（实验 / 评估器 / 评测集）
│   │   ├── observability/      # 可观测性（Trace / Span）
│   │   ├── prompt/             # Prompt 管理
│   │   ├── data/               # 数据管理（Dataset）
│   │   ├── llm/                # LLM 模型集成
│   │   └── foundation/         # 基础（用户 / 空间 / API Key）
│   ├── infra/                  # 共享基础设施（DB, Redis, ClickHouse, MQ, middleware）
│   ├── pkg/                    # 共享工具库
│   ├── kitex_gen/              # Kitex 生成代码 ⚠️ 禁止手动修改
│   ├── loop_gen/               # Loop 生成代码 ⚠️ 禁止手动修改
│   └── script/                 # 代码生成脚本
│       ├── cloudwego/code_gen.sh   # IDL → Go
│       ├── gorm_gen/generate.go    # SQL → GORM Model
│       └── errorx/code_gen.py      # 错误码生成
├── frontend/                   # Rush.js 前端 monorepo（59 包）
│   ├── apps/cozeloop/          # 主 SPA（React 18, Rsbuild, react-router, zustand）
│   ├── packages/               # loop-pages, loop-modules, loop-components, loop-base
│   ├── config/                 # eslint-config, ts-config, vitest-config 等
│   └── infra/                  # IDL 转 TypeScript 工具等
├── release/deployment/
│   ├── docker-compose/         # Docker 部署（conf/ + bootstrap/）
│   └── helm-chart/             # Kubernetes 部署
└── common/                     # git-hooks（Rush 管理）
```

## 核心约束

### 1. DDD 分层纪律

每个 `backend/modules/<domain>/` 遵循三层架构，依赖方向单向：

```
api/ → application/ → domain/ ← infra/
```

- **domain/** 定义接口，**infra/** 实现接口，domain **绝不**引用 infra
- Application 层负责 DTO↔DO 转换、用例编排
- Repository 的事务只在 infra 层启动

### 2. IDL 优先

`idl/thrift/` 是接口契约的同步源。商业化 IDL 仓库的 `open/` 目录从此处同步。修改 IDL 后必须运行代码生成。

### 3. 生成代码不可手动修改

- `backend/kitex_gen/` — Kitex 生成
- `backend/loop_gen/` — Loop 本地调用生成
- `backend/api/router_gen.go` — Hertz 路由生成
- `**/wire_gen.go` — Wire DI 生成
- `**/gorm_gen/**` — GORM 模型生成

### 4. 向前兼容

已有企业用户二开部署。IDL API / Domain 核心接口 / 配置文件 / 存储 Schema 均要求向前兼容。新增字段必须 optional，数据库只允许加列。

### 5. SQL 双路径同步

修改数据库表时，以下两个目录必须保持一致：
- `release/deployment/docker-compose/bootstrap/mysql-init/`（Docker）
- `release/deployment/helm-chart/charts/app/bootstrap/init/mysql/init-sql/`（Helm）

修改表还需要在 `patch-sql/` 写 ALTER 语句。CI 会通过 `mysql-schema-check` 工作流校验两侧一致性。

### 6. 前端依赖分层

前端 6 层依赖结构（Level-1 到 Level-6），高层级只能依赖低层级，禁止反向依赖。商业版与开源版差异通过 Adapter 模式解耦（`adapter-interfaces/` → `*-adapter/` → `components-with-adapter/`）。

## 后端开发流程

```
Step 1: IDL 定义 → Step 2: 代码生成 → Step 3: MySQL Schema → Step 4: GORM Gen
→ Step 5: 配置文件 → Step 6: 业务代码 → Step 7: 错误码 → Step 8: 国际化
→ Step 9: UT → Step 10: golint
```

详细步骤见 `docs/guidance/idl-codegen-guide.md`（Step 1-2）和 `docs/guidance/deployment-guide.md`（部署配置）。

**Wire 依赖注入**：修改 `wire.go` 后运行 `wire generate` 重新生成 `wire_gen.go`。禁止手动编辑 `wire_gen.go`。

**代码复用约束**：新增功能前必须先搜索 codebase 是否已有相同/类似实现，有则复用或扩展，不得新建重复文件。

## 常用命令

### 后端（Go）

| 操作 | 命令 |
|------|------|
| Go 编译 | `cd backend && go build ./...` |
| 全量测试 | `cd backend && go test -gcflags="all=-N -l" ./...` |
| 单测（指定包） | `cd backend && go test -gcflags="all=-N -l" ./modules/<domain>/...` |
| 单测（指定函数） | `cd backend && go test -gcflags="all=-N -l" -run TestFuncName ./modules/<domain>/...` |
| CI 级测试（含 race） | `cd backend && go test -gcflags="all=-N -l" -race -v -coverprofile=coverage.out -coverpkg=./... ./...` |
| golint 修复 | `cd backend && golangci-lint run --config ../.github/.golangci.yaml --fix` |
| IDL 代码生成 | `bash backend/script/cloudwego/code_gen.sh` |
| GORM 代码生成 | `cd backend && go run script/gorm_gen/generate.go` |
| 错误码生成 | `backend/script/errorx/code_gen.py <biz>` |

### 前端（Rush.js + pnpm）

| 操作 | 命令 |
|------|------|
| 安装依赖 | `node common/scripts/install-run-rush.js install` |
| 全量构建 | `node common/scripts/install-run-rush.js rebuild --verbose` |
| 开发服务器 | `cd frontend/apps/cozeloop && npm run dev` |
| Lint | `cd frontend/apps/cozeloop && npm run lint` |
| 测试 | `cd frontend/apps/cozeloop && npm run test` |
| TypeScript 检查 | `cd frontend/apps/cozeloop && npm run build:ts` |

### 部署

| 操作 | 命令 |
|------|------|
| 本地 dev 部署 | `make compose-up-dev` |
| 本地 debug 部署（含 Delve） | `make compose-up-debug` |
| 停止 dev | `make compose-down-dev` |
| 停止并清理 volumes | `make compose-down-v-dev` |
| Helm 部署 | `make helm-up` |
| 查看 Pod 状态 | `make helm-pod` |

### PR 标题格式

PR 标题必须符合 `[<type>][<scope>] <description>` 格式，CI 会校验。

- type: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `chore`, `build`, `ci`
- scope: `all`, `idl`, `frontend`, `backend`, `infra`, `workflow`, `prompt`, `evaluation`, `trace`, `model`, `tag`, `dataset`, `foundation`

示例：`[feat][evaluation] add offline metric support`
