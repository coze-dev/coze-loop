# IDL 变更与代码生成流程

> 本文档描述修改 Thrift IDL 后，前后端代码如何同步更新。

## IDL 目录结构

```
idl/thrift/
├── base.thrift                    # 公共基础类型
├── extra.thrift                   # 扩展类型
├── trajectory.thrift              # 轨迹类型
└── coze/loop/
    ├── apis/                      # API 接口定义
    ├── data/                      # 数据模块类型
    ├── evaluation/                # 评测模块类型
    ├── foundation/                # 基础能力类型
    ├── llm/                       # LLM 模块类型
    ├── observability/             # 可观测性类型
    └── prompt/                    # Prompt 模块类型
```

## 变更流程

### 1. 修改 IDL 文件

在 `idl/thrift/` 下修改对应的 `.thrift` 文件。

### 2. 后端代码生成

```bash
cd backend

# Kitex (RPC) 代码生成
bash script/cloudwego/kitex_tool.sh

# Hertz (HTTP) 代码生成
bash script/cloudwego/hertz_tool.sh

# 通用代码生成
bash script/cloudwego/code_gen.sh
```

生成产物:
- `backend/kitex_gen/` — Kitex/Thrift 生成的 Go 代码（勿手动修改）
- `backend/loop_gen/` — 其他生成代码（勿手动修改）
- `backend/api/router_gen.go` — 路由生成代码（勿手动修改）

### 3. 前端类型生成

前端通过 `frontend/infra/idl/` 工具将 Thrift IDL 转换为 TypeScript 类型定义。

生成产物进入 `frontend/packages/loop-base/api-schema/`。

### 4. Wire 依赖注入（如需）

如果 IDL 变更导致新增了服务接口，需更新对应模块的 Wire 定义:

```bash
cd backend/modules/<domain>/application
wire
```

生成产物: `wire_gen.go`（勿手动修改）

### 5. GORM 模型生成（如需）

如果涉及数据库 schema 变更:

```bash
cd backend
# 使用 gorm_gen 脚本
```

### 6. 错误码生成（如需）

```bash
cd backend
# 使用 errorx 脚本
```

## CI 检查

- IDL 变更会触发 `.github/workflows/idl.yaml` CI 工作流
- 数据库 schema 变更触发 `.github/workflows/mysql-schema-check.yaml`

## 注意事项

- 所有 `*_gen.go`、`kitex_gen/`、`loop_gen/` 文件均为自动生成，禁止手动修改
- IDL 变更后必须同时更新前后端生成代码，保持契约一致
- 生成结果需要提交到仓库（不在 .gitignore 中）
