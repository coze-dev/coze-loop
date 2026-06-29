# 后端 DDD 模块与 API 参考

> 本文档汇总后端 6 个 DDD 业务模块的职责、领域实体和 API 路由入口，供快速查阅。
> 详细模块导航见 [`../../backend/AGENTS.md`](../../backend/AGENTS.md)。

## 业务模块一览

| 模块 | 目录 | 职责 |
|------|------|------|
| **data** | `backend/modules/data/` | 数据管理 — 数据集 CRUD、数据导入导出 |
| **evaluation** | `backend/modules/evaluation/` | 评测 — 评测集、评估器、实验管理与执行 |
| **foundation** | `backend/modules/foundation/` | 基础能力 — 用户、权限、通用配置 |
| **llm** | `backend/modules/llm/` | LLM 集成 — 模型调用、Eino 框架对接 |
| **observability** | `backend/modules/observability/` | 可观测性 — Trace 采集、查询、分析 |
| **prompt** | `backend/modules/prompt/` | Prompt 管理 — Prompt CRUD、版本管理、Playground |

## DDD 分层结构（每个模块内部统一）

```
modules/<domain>/
├── application/        # 应用服务（用例编排、Wire DI）
│   ├── wire.go         # 依赖注入定义
│   ├── wire_gen.go     # 依赖注入生成（勿手动修改）
│   └── convertor/      # DTO <-> Entity 转换
├── domain/             # 领域模型
│   ├── entity/         # 领域实体定义
│   ├── repo/           # 仓储接口定义
│   ├── service/        # 领域服务
│   ├── events/         # 领域事件（如有）
│   └── component/      # 领域组件（如有）
├── infra/              # 基础设施实现
│   ├── repo/           # 仓储接口实现（MySQL/ClickHouse）
│   ├── mq/             # 消息队列
│   ├── rpc/            # RPC 调用
│   ├── storage/        # 文件存储
│   └── metrics/        # 指标采集
├── pkg/                # 模块内工具
│   ├── errno/          # 错误码定义
│   └── utils/          # 辅助函数
└── consts/             # 模块常量
```

**依赖方向**: `api/handler/ → application/ → domain/ ← infra/`

## HTTP API 入口

- 路由注册: `backend/api/router.go`
- Handler 按领域分组: `backend/api/handler/coze/`
- 生成路由（勿手动修改）: `backend/api/router_gen.go`

## 共享基础设施 (`backend/infra/`)

| 目录 | 职责 |
|------|------|
| `db/` | MySQL 连接与 GORM 配置 |
| `ck/` | ClickHouse 连接（分析查询） |
| `redis/` | Redis 客户端（缓存/分布式锁） |
| `mq/` | RocketMQ 消息队列 |
| `http/` | HTTP 客户端 |
| `middleware/` | Hertz 中间件（认证、日志、限流等） |
| `i18n/` | 国际化 |
| `metrics/` | 指标采集 |
| `lock/` | 分布式锁 |
| `idgen/` | ID 生成器 |
| `limiter/` | 限流器 |
| `fileserver/` | 文件存储抽象 |

## Thrift IDL 目录 (`idl/thrift/`)

```
idl/thrift/
├── base.thrift                # 公共基础类型
├── extra.thrift               # 扩展类型
├── trajectory.thrift          # 轨迹类型
└── coze/loop/
    ├── apis/                  # API 定义
    ├── data/                  # 数据模块 IDL
    ├── evaluation/            # 评测模块 IDL
    ├── foundation/            # 基础能力 IDL
    ├── llm/                   # LLM 模块 IDL
    ├── observability/         # 可观测性 IDL
    └── prompt/                # Prompt 模块 IDL
```

## 代码生成脚本 (`backend/script/`)

| 脚本 | 用途 | 输入 |
|------|------|------|
| `cloudwego/kitex_tool.sh` | Kitex/Thrift 代码生成 | `idl/thrift/` |
| `cloudwego/hertz_tool.sh` | Hertz HTTP 代码生成 | IDL |
| `cloudwego/code_gen.sh` | 通用代码生成 | IDL |
| `gorm_gen/` | GORM 模型代码生成 | 数据库 schema |
| `errorx/` | 错误码代码生成 | 错误码定义 |
