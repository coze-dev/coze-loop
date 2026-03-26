# Tool 实体 CRUD + 版本管理后端实现

本 ExecPlan 是一份活文档。Progress、Surprises & Discoveries、Decision Log 和 Outcomes & Retrospective 章节必须随工作推进持续更新。

本文档遵循 ExecPlan 规范维护（路径：`.claude/skills/devclaw-exec-plan/SKILL.md`）。

**创建时代码基线：**
- 分支：`20260326195427-pe-tools-management`
- Commit SHA：`cf28187f1e049e53aed6e9ea1e6cfa1e56486b2a`


## Purpose / Big Picture

本次改动为 Prompt Studio 的"函数"（Tool）管理功能提供完整的后端 API 支持。完成后，前端可以通过 REST API 实现：

1. **创建 Tool**（`POST /api/prompt/v1/tools`）——新建一个公共函数，同时可选保存初始草稿
2. **获取 Tool 详情**（`GET /api/prompt/v1/tools/:tool_id`）——查看 Tool 基本信息，可选带草稿或指定版本内容
3. **列表查询**（`POST /api/prompt/v1/tools/list`）——分页、搜索、排序、筛选
4. **保存草稿**（`POST /api/prompt/v1/tools/:tool_id/drafts/save`）——保存 JSON Schema 编辑内容
5. **提交版本**（`POST /api/prompt/v1/tools/:tool_id/drafts/commit`）——将草稿提交为正式版本
6. **版本列表**（`POST /api/prompt/v1/tools/:tool_id/commits/list`）——浏览历史版本
7. **批量获取**（`POST /api/prompt/v1/tools/mget`）——按 ID+版本批量查询


## Progress

- [x] (2026-03-26 20:00:31+08:00) ExecPlan 创建完成
- [x] (2026-03-26 20:14:26+08:00) Milestone 1: IDL 代码生成 + SQL 文件 + GORM 模型生成
- [x] (2026-03-26 20:16:30+08:00) Milestone 2: Domain 层实现（entity、repo interface、service interface）
- [x] (2026-03-26 20:20:00+08:00) Milestone 3: Infra 层实现（DAO、convertor、repo impl）
- [x] (2026-03-26 20:24:00+08:00) Milestone 4: Application 层实现（tool_manage application、convertor、wire）
- [x] (2026-03-26 20:28:00+08:00) Milestone 5: API Handler 层接入 + 路由注册
- [x] (2026-03-26 20:33:00+08:00) Milestone 6: 编译验证 + 单元测试
- [x] (2026-03-26 20:34:00+08:00) Milestone 7: 文档更新


## Surprises & Discoveries

- 观察：`entity.Tool` 与 `prompt_detail.go` 中已有的 `Tool`（Prompt 的 tool_call 配置结构体）命名冲突
  证据：编译报错 `Tool redeclared in this block`
  处理：所有 Tool 实体改用 `CommonTool` 前缀

- 观察：observability ingestion IDL 已提交但未纳入聚合 thrift 文件，导致 `loop_gen` 引用不存在的 `kitex_gen` 包
  证据：`go mod tidy` 报 `does not contain package github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/ingestion`
  处理：将 `coze.loop.observability.ingestion.thrift` 纳入 `coze.loop.observability.thrift` 聚合文件

- 观察：prompt 模块大量 mock 文件使用旧的 `github.com/golang/mock/gomock` 路径，与测试代码使用的 `go.uber.org/mock/gomock` 不兼容
  证据：infra/repo 测试编译失败
  处理：统一所有 mock 文件为 `go.uber.org/mock/gomock`


## Decision Log

- 决策：Tool 实体放在 `modules/prompt` 模块内，而非独立模块
  理由：需求明确指出本次实现在 prompt 模块内，且 IDL 已在 `prompt/` 路径下定义。Tool 与 Prompt 属于同一业务域（Prompt Studio 下的"组件"）。
  日期/作者：2026-03-26 / Agent

- 决策：Tool 的草稿模型使用 `tool_commit` 表存储（version 为 `$PublicDraft`），而非独立的 `tool_user_draft` 表
  理由：与 Prompt 的 per-user draft 不同，Tool 的草稿是公共草稿（PublicDraft），所有人共享同一份草稿，因此复用 `tool_commit` 表、以特殊版本号 `$PublicDraft` 标记即可，无需单独建表。IDL 和 DB schema 也印证了这一点——不存在 `tool_user_draft` 表。
  日期/作者：2026-03-26 / Agent

- 决策：ToolManageService 作为独立的 KiteX Service 接入，不复用 PromptManageService
  理由：IDL 中已将 `ToolManageService` 定义为独立 service，且在 `coze.loop.apis.thrift` 中通过 `extends` 暴露。PromptHandler 需要扩展以嵌入 ToolManageService。
  日期/作者：2026-03-26 / Agent

- 决策：BatchGetTools 接口不鉴权
  理由：参考 BatchGetPrompt 的实现（`manage.go:359` 注释"内部接口不鉴权"），BatchGetTools 同样是供内部服务调用的批量接口。
  日期/作者：2026-03-26 / Agent

- 决策：`tool_basic` 表使用 `deleted_at bigint` 软删除；`tool_commit` 表无软删除
  理由：与 `prompt_basic`/`prompt_commit` 的模式完全一致。basic 表需要软删除来保留历史记录，commit 表作为不可变版本记录无需软删除。
  日期/作者：2026-03-26 / Agent


## Outcomes & Retrospective

### 成果
- 完成 Tool 实体的完整 CRUD + 版本管理后端实现，共 7 个 API 端点
- 严格遵循 DDD 三层架构和 constitution 规范
- 全项目编译零错误，prompt 模块全部测试通过（含新增 14 个单元测试用例）
- 代码生成工具链（IDL/SQL/GORM/Wire）全部正常运行

### 经验教训
1. Domain 实体命名需注意与现有类型的冲突——`Tool` 在 `prompt_detail.go` 中已被 Prompt 的 tool_call 配置使用，因此改为 `CommonTool` 前缀
2. GORM gen 生成的 query 类型在 `WithContext` 后会变成 `*xxxDo` 类型（丢失字段访问），需要通过 `query.Use(session)` 重新获取 Query 对象来访问字段
3. Mock 文件的 gomock import 路径需要统一（`go.uber.org/mock/gomock` vs `github.com/golang/mock/gomock`）
4. `field.Expr` 和 `field.OrderExpr` 是不同类型，排序需要用 `OrderExpr`
5. 预先存在的 observability ingestion IDL 问题需要顺带修复（空 service 未纳入聚合文件导致编译失败）


## Context and Orientation

### 仓库结构

Coze Loop 是一个前后端一体的仓库，后端使用 Go 语言，基于 CloudWeGo 的 Hertz（HTTP）和 KiteX（RPC）框架。后端代码位于 `backend/` 目录下，按业务域划分为多个模块（`modules/prompt`、`modules/evaluation` 等），每个模块严格遵循 DDD 三层架构：

- **Application 层**（`application/`）：DTO↔DO 转换、用例编排、Wire 依赖注入
- **Domain 层**（`domain/`）：实体定义、仓储接口、领域服务
- **Infrastructure 层**（`infra/`）：仓储实现（MySQL DAO、Redis Cache）、RPC 适配器

### 关键约束（来自 `constitution/constitution.md`）

1. 层依赖方向：Application → Domain ← Infrastructure，不可逆
2. Domain 层禁止引用 DTO/PO 结构
3. DAO 接口必须预留 `opts ...db.Option` 参数
4. 事务只能在 Repository 层启动
5. IDL 变更后必须用 `upgrade-idl` skill 生成代码
6. SQL 变更后必须用 `upgrade-sql` skill 生成 GORM 模型
7. Wire 变更后必须用 `upgrade-wire` skill 生成注入代码
8. 生成文件（`kitex_gen/`、`loop_gen/`、`wire_gen.go`、`gorm_gen/`）禁止手动修改

### 现有参考模式

Tool 实体的实现与 Prompt 实体高度同构。以下是关键参考文件和它们对应的 Tool 实现映射：

| Prompt 参考文件 | Tool 对标文件（待创建） |
|---|---|
| `domain/entity/prompt.go` | `domain/entity/tool.go` |
| `domain/repo/manage.go` | `domain/repo/tool.go` |
| `domain/service/service.go` (部分方法) | `domain/service/tool.go` |
| `infra/repo/manage.go` | `infra/repo/tool.go` |
| `infra/repo/mysql/prompt_basic.go` | `infra/repo/mysql/tool_basic.go` |
| `infra/repo/mysql/prompt_commit.go` | `infra/repo/mysql/tool_commit.go` |
| `infra/repo/mysql/convertor/manage.go` | `infra/repo/mysql/convertor/tool.go` |
| `application/manage.go` | `application/tool_manage.go` |
| `application/convertor/prompt.go` | `application/convertor/tool.go` |
| `application/wire.go` | `application/wire.go`（扩展） |

### IDL 定义

Tool 的 IDL 已提交到仓库：

- 领域模型定义：`idl/thrift/coze/loop/prompt/domain/tool.thrift`
- 服务接口定义：`idl/thrift/coze/loop/prompt/coze.loop.prompt.tool_manage.thrift`
- 聚合入口：`idl/thrift/coze/loop/apis/coze.loop.apis.thrift`（已添加 extends）

ToolManageService 包含 7 个 RPC 方法：

1. `CreateTool` — POST `/api/prompt/v1/tools`
2. `GetToolDetail` — GET `/api/prompt/v1/tools/:tool_id`
3. `ListTool` — POST `/api/prompt/v1/tools/list`
4. `SaveToolDetail` — POST `/api/prompt/v1/tools/:tool_id/drafts/save`
5. `CommitToolDraft` — POST `/api/prompt/v1/tools/:tool_id/drafts/commit`
6. `ListToolCommit` — POST `/api/prompt/v1/tools/:tool_id/commits/list`
7. `BatchGetTools` — POST `/api/prompt/v1/tools/mget`

### DB Schema

两张表，`tool_basic`（带软删除）和 `tool_commit`（无软删除）。草稿存储在 `tool_commit` 中，version 字段为 `$PublicDraft`。

### Domain Model

用户在需求中已给出完整的 Domain 实体定义（`Tool`、`ToolBasic`、`ToolCommit`、`ToolDetail`、`CommitInfo`），以及 `PublicDraftVersion` 常量和 `IsPublicDraft()` 方法。


## Plan of Work

### Milestone 1: IDL 代码生成 + SQL 文件 + GORM 模型生成

**目标**：让 KiteX/Hertz 框架代码和 GORM 模型就绪，后续里程碑可以直接引用生成的类型。

**工作内容**：

1. **运行 `upgrade-idl` skill** 生成 KiteX RPC 和 Hertz 路由代码。这会在以下目录产出：
   - `backend/kitex_gen/coze/loop/prompt/tool_manage/` — ToolManageService 的 KiteX Client/Server 代码
   - `backend/kitex_gen/coze/loop/prompt/domain/tool/` — Tool 领域结构体
   - `backend/loop_gen/coze/loop/prompt/lotool_manage/` — Local Call 适配器
   - `backend/api/handler/coze/loop/apis/tool_manage_service.go` — Hertz Handler 桩代码
   - `backend/api/router/coze/loop/apis/coze.loop.apis.go` — 路由注册（更新）

2. **创建 SQL 初始化文件**：
   - `release/deployment/docker-compose/bootstrap/mysql-init/init-sql/tool_basic.sql`
   - `release/deployment/docker-compose/bootstrap/mysql-init/init-sql/tool_commit.sql`
   - 同步到 `release/deployment/helm-chart/charts/app/bootstrap/init/mysql/init-sql/` 目录

3. **更新 GORM gen 配置**（`backend/script/gorm_gen/generate.go`）：
   - 在 `generateForPrompt` 函数中，将 `tool_basic` 添加到带软删除的表列表，将 `tool_commit` 添加到无软删除的表列表

4. **运行 `upgrade-sql` skill** 生成 GORM 模型到 `modules/prompt/infra/repo/mysql/gorm_gen/model/` 和 `query/` 目录

**验收**：
- `go build ./backend/...` 零错误
- `backend/kitex_gen/coze/loop/prompt/tool_manage/` 目录存在且包含生成代码
- `backend/modules/prompt/infra/repo/mysql/gorm_gen/model/tool_basic.gen.go` 和 `tool_commit.gen.go` 存在

### Milestone 2: Domain 层实现

**目标**：定义 Tool 的领域模型、仓储接口和领域服务接口。

**工作内容**：

1. **创建实体文件** `backend/modules/prompt/domain/entity/tool.go`：
   - 按用户提供的 Domain 定义写入 `Tool`、`ToolBasic`、`ToolCommit`、`ToolDetail`、`CommitInfo` 结构体
   - 包含 `PublicDraftVersion` 常量和 `IsPublicDraft()` 方法

2. **创建仓储接口文件** `backend/modules/prompt/domain/repo/tool.go`：
   - `IToolRepo` 接口，包含以下方法：
     - `CreateTool(ctx, toolDO) (toolID int64, err error)` — 创建 tool_basic + 可选的初始草稿
     - `GetTool(ctx, param GetToolParam) (*entity.Tool, error)` — 获取 tool 详情
     - `MGetTool(ctx, queries []MGetToolQuery) (map[MGetToolQuery]*entity.Tool, error)` — 批量获取（供 BatchGetTools 使用）
     - `ListTool(ctx, param ListToolParam) (*ListToolResult, error)` — 列表查询
     - `SaveDraft(ctx, toolDO *entity.Tool) error` — 保存/更新草稿（upsert tool_commit where version=$PublicDraft）
     - `CommitDraft(ctx, param CommitToolDraftParam) error` — 提交草稿为正式版本
     - `ListCommitInfo(ctx, param ListToolCommitParam) (*ListToolCommitResult, error)` — 版本列表
   - 对应的参数和结果结构体

3. **创建领域服务接口** `backend/modules/prompt/domain/service/tool.go`：
   - `IToolService` 接口，封装跨 Repo 的业务逻辑：
     - `CreateTool(ctx, toolDO) (toolID int64, err error)` — 生成 ID + 调用 repo
     - `GetTool(ctx, param) (*entity.Tool, error)` — 直接代理 repo
     - `SaveDraft(ctx, toolDO) error` — 直接代理 repo
   - `ToolServiceImpl` 结构体和构造函数 `NewToolService`

**验收**：
- `go build ./backend/modules/prompt/...` 零错误
- entity、repo interface、service interface 文件存在且语法正确

### Milestone 3: Infra 层实现

**目标**：实现数据访问层，包括 DAO、PO↔DO 转换器和 Repository 实现。

**工作内容**：

1. **创建 DAO 接口和实现**：

   `backend/modules/prompt/infra/repo/mysql/tool_basic.go`：
   - `IToolBasicDAO` 接口：`Create`、`Get`、`MGet`、`List`（带分页、搜索、排序）、`Update`、`Delete`
   - `ToolBasicDAOImpl` 实现，使用 GORM gen 生成的 query builder
   - 参考 `prompt_basic.go` 的模式（WriteTracker、分页、keyword 搜索）

   `backend/modules/prompt/infra/repo/mysql/tool_commit.go`：
   - `IToolCommitDAO` 接口：`Create`、`Get`、`MGet`（按 tool_id+version 批量查询）、`Upsert`（用于草稿 upsert）、`Delete`（用于删除草稿）、`List`（cursor 分页）
   - `ToolCommitDAOImpl` 实现
   - 参考 `prompt_commit.go` 的模式

2. **创建 PO↔DO 转换器** `backend/modules/prompt/infra/repo/mysql/convertor/tool.go`：
   - `ToolPO2DO(basicPO, commitPO) *entity.Tool` — 从 PO 组装 DO
   - `ToolDO2BasicPO(toolDO) *model.ToolBasic` — DO→BasicPO
   - `ToolDO2CommitPO(toolDO, toolID, spaceID) *model.ToolCommit` — DO→CommitPO
   - `BatchBasicPO2ToolDO(basicPOs) []*entity.Tool`
   - `CommitPO2DO(commitPO) *entity.ToolCommit`
   - `BatchCommitInfoDOFromCommitPO(commitPOs) []*entity.CommitInfo`

3. **创建 Repository 实现** `backend/modules/prompt/infra/repo/tool.go`：
   - `ToolRepoImpl` 结构体，组合 `db.Provider`、`idgen.IIDGenerator`、`IToolBasicDAO`、`IToolCommitDAO`
   - 构造函数 `NewToolRepo`
   - 实现 `IToolRepo` 接口所有方法：
     - `CreateTool`：事务中创建 basic + 可选草稿（如果有 draft_detail）
     - `GetTool`：查询 basic，可选查询 commit（指定版本或草稿）
     - `MGetTool`：批量查询 basic + commit，组装返回
     - `ListTool`：调用 basicDAO.List 获取分页数据
     - `SaveDraft`：Upsert tool_commit where version=$PublicDraft
     - `CommitDraft`：事务中——创建新 commit 记录、删除草稿（如存在）、更新 basic 的 latest_committed_version
     - `ListCommitInfo`：cursor 分页查询 commit 记录（排除 $PublicDraft）

**验收**：
- `go build ./backend/modules/prompt/...` 零错误
- 所有 DAO、convertor、repo 文件存在

### Milestone 4: Application 层实现

**目标**：实现 Application 层，完成 DTO↔DO 转换、用例编排和 Wire 注入。

**工作内容**：

1. **创建 DTO↔DO 转换器** `backend/modules/prompt/application/convertor/tool.go`：
   - `ToolDTO2DO(dto *tool.Tool) *entity.Tool` — IDL Tool → Domain Tool
   - `ToolDO2DTO(do *entity.Tool) *tool.Tool` — Domain Tool → IDL Tool
   - `ToolBasicDTO2DO` / `ToolBasicDO2DTO`
   - `ToolCommitDTO2DO` / `ToolCommitDO2DTO`
   - `ToolDetailDTO2DO` / `ToolDetailDO2DTO`
   - `CommitInfoDTO2DO` / `CommitInfoDO2DTO`
   - `BatchToolDO2DTO`
   - 时间戳转换：`time.Time` ↔ `int64`（UnixMilli）

2. **创建 Application 实现** `backend/modules/prompt/application/tool_manage.go`：
   - `ToolManageApplicationImpl` 结构体，依赖 `IToolRepo`、`IToolService`、`IAuthProvider`、`IUserProvider`
   - 构造函数 `NewToolManageApplication`，返回 `tool_manage.ToolManageService` 接口
   - 实现 7 个方法：

     **CreateTool**：
     1. 从 session 获取 userID
     2. 调用 authRPCProvider.CheckSpacePermission（ActionLoopPromptEdit 或合适的 action）
     3. 构建 toolDO（设置 SpaceID、Name、Description、CreatedBy、UpdatedBy）
     4. 如果有 draft_detail，设置 ToolCommit（PublicDraftVersion）
     5. 调用 toolService.CreateTool
     6. 返回 tool_id

     **GetToolDetail**：
     1. 从 session 获取 userID
     2. 鉴权
     3. 构建 GetToolParam（with_commit、commit_version、with_draft）
     4. 调用 repo.GetTool
     5. 转换并返回

     **ListTool**：
     1. 鉴权
     2. 构建 ListToolParam（keyword、created_bys、committed_only、分页、排序）
     3. 调用 repo.ListTool
     4. 收集 userID 集合，调用 userRPCProvider.MGetUserInfo
     5. 转换并返回（带 users 列表）

     **SaveToolDetail**：
     1. 鉴权
     2. 构建 toolDO（ID、SpaceID、ToolCommit 含 ToolDetail + CommitInfo with PublicDraftVersion）
     3. 调用 repo.SaveDraft
     4. 返回空响应

     **CommitToolDraft**：
     1. 鉴权
     2. 构建 CommitToolDraftParam（toolID、version、description、base_version、committed_by）
     3. 调用 repo.CommitDraft
     4. 返回空响应

     **ListToolCommit**：
     1. 鉴权
     2. 构建 ListToolCommitParam（toolID、page_size、page_token、asc、with_commit_detail）
     3. 调用 repo.ListCommitInfo
     4. 收集 committedBy 的 userID 集合，查用户信息
     5. 转换并返回（带 users）

     **BatchGetTools**：
     1. 不鉴权（内部接口）
     2. 构建 queries（MGetToolQuery 列表）
     3. 调用 repo.MGetTool
     4. 组装结果列表
     5. 返回

3. **更新 Wire 配置** `backend/modules/prompt/application/wire.go`：
   - 新增 `toolDomainSet` Wire Set，包含：
     - `service.NewToolService`
     - `repo.NewToolRepo`（infra repo）
     - `mysql.NewToolBasicDAO`
     - `mysql.NewToolCommitDAO`
   - 新增 `toolManageSet`，包含 `NewToolManageApplication` + `toolDomainSet` + 共享依赖（auth、user RPC）
   - 新增 `InitToolManageApplication` 函数

4. **运行 `upgrade-wire` skill** 生成 `wire_gen.go`

**验收**：
- `go build ./backend/modules/prompt/...` 零错误
- Application 层所有方法实现完成

### Milestone 5: API Handler 层接入 + 路由注册

**目标**：将 ToolManageService 接入 HTTP 路由，使 API 可访问。

**工作内容**：

1. **检查并确认** `upgrade-idl` 在 Milestone 1 中已生成了 `tool_manage_service.go` handler 桩文件和路由注册代码。如果没有，需要手动创建。

2. **扩展 PromptHandler**（`backend/api/handler/coze/loop/apis/handler.go`）：
   - 在 `PromptHandler` 结构体中嵌入 `tool_manage.ToolManageService`
   - 更新 `NewPromptHandler` 构造函数，接收 `toolManageApp tool_manage.ToolManageService` 参数
   - 调用 `bindLocalCallClient` 绑定 ToolManageService 的 local client

3. **更新 Wire 配置**（`backend/api/handler/coze/loop/apis/wire.go`）：
   - `promptSet` 中添加 `promptapp.InitToolManageApplication`
   - `InitPromptHandler` 函数签名无需改变（Wire 自动推导）

4. **运行 `upgrade-wire` skill** 生成 `wire_gen.go`

**验收**：
- `go build ./backend/...` 零错误（全项目编译通过）
- Tool API 路由在编译后可达

### Milestone 6: 编译验证 + 单元测试

**目标**：确保整体编译通过，并为核心逻辑编写单元测试。

**工作内容**：

1. **全项目编译验证**：`go build ./backend/...`

2. **单元测试编写**：
   - `application/tool_manage_test.go`：使用 mockgen 生成的 mock 测试 7 个 Application 方法
   - `infra/repo/tool_test.go`：测试 Repository 层的关键逻辑（CreateTool、CommitDraft 的事务逻辑）
   - `domain/service/tool_test.go`：测试 ToolService 的业务逻辑
   - `application/convertor/tool_test.go`：测试 DTO↔DO 转换的正确性

3. **Mock 生成**：
   - 在 `domain/repo/tool.go` 添加 `//go:generate mockgen` 指令
   - 在 `domain/service/tool.go` 添加 `//go:generate mockgen` 指令
   - 在 DAO 接口文件添加 mockgen 指令
   - 执行 `go generate ./backend/modules/prompt/...`

4. **Lint 检查**：`golangci-lint run ./backend/...`

**验收**：
- `go build ./backend/...` 零错误
- `go test ./backend/modules/prompt/... -count=1` 全部 PASS，0 failures，0 skipped
- `golangci-lint run ./backend/modules/prompt/...` 零 warning

### Milestone 7: 文档更新

**目标**：确保仓库文档与代码变更保持同步。

**工作内容**：

1. 评估并更新 `constitution/prompt-domain-specific-constitution.md`：
   - 添加 Tool 相关命名约定说明（与 Prompt Commit 类似，Tool Commit 的命名约定）

2. 如果仓库存在模块级 AGENTS.md（prompt 模块下），更新知识导航

**验收**：
- 文档更新后与代码实际行为一致
- 新增 Tool 相关命名约定在 constitution 中可查


## Concrete Steps

### Milestone 1

    # Step 1.1: 运行 upgrade-idl skill 生成 KiteX + Hertz 代码
    # （通过 skill 执行，会自动处理 kitex gen 和 hertz gen）

    # Step 1.2: 创建 SQL 文件
    # 在 release/deployment/docker-compose/bootstrap/mysql-init/init-sql/ 创建：
    #   tool_basic.sql
    #   tool_commit.sql
    # 同步到 release/deployment/helm-chart/charts/app/bootstrap/init/mysql/init-sql/

    # Step 1.3: 更新 gorm_gen/generate.go
    # 在 generateForPrompt() 中添加 tool_basic（带软删除）和 tool_commit（无软删除）

    # Step 1.4: 运行 upgrade-sql skill 生成 GORM 模型

    # Step 1.5: 验证编译
    cd /Users/bytedance/workspace/src/github/coze-dev/coze-loop && go build ./backend/...

### Milestone 2

    # Step 2.1: 创建 domain/entity/tool.go
    # Step 2.2: 创建 domain/repo/tool.go（IToolRepo 接口 + 参数/结果结构体）
    # Step 2.3: 创建 domain/service/tool.go（IToolService + ToolServiceImpl）
    # Step 2.4: 验证编译
    cd /Users/bytedance/workspace/src/github/coze-dev/coze-loop && go build ./backend/modules/prompt/...

### Milestone 3

    # Step 3.1: 创建 infra/repo/mysql/tool_basic.go（IToolBasicDAO + impl）
    # Step 3.2: 创建 infra/repo/mysql/tool_commit.go（IToolCommitDAO + impl）
    # Step 3.3: 创建 infra/repo/mysql/convertor/tool.go（PO↔DO 转换器）
    # Step 3.4: 创建 infra/repo/tool.go（ToolRepoImpl，实现 IToolRepo）
    # Step 3.5: 验证编译

### Milestone 4

    # Step 4.1: 创建 application/convertor/tool.go（DTO↔DO）
    # Step 4.2: 创建 application/tool_manage.go（ToolManageApplicationImpl）
    # Step 4.3: 更新 application/wire.go（添加 Tool 相关 Wire Set + Init 函数）
    # Step 4.4: 运行 upgrade-wire skill
    # Step 4.5: 验证编译

### Milestone 5

    # Step 5.1: 确认 handler 桩文件已生成
    # Step 5.2: 更新 handler.go（扩展 PromptHandler）
    # Step 5.3: 更新 apis/wire.go（promptSet 添加 InitToolManageApplication）
    # Step 5.4: 运行 upgrade-wire skill
    # Step 5.5: 验证全项目编译
    cd /Users/bytedance/workspace/src/github/coze-dev/coze-loop && go build ./backend/...

### Milestone 6

    # Step 6.1: 添加 mockgen 指令
    # Step 6.2: 生成 mock 文件
    cd /Users/bytedance/workspace/src/github/coze-dev/coze-loop && go generate ./backend/modules/prompt/...
    # Step 6.3: 编写单元测试
    # Step 6.4: 运行测试
    cd /Users/bytedance/workspace/src/github/coze-dev/coze-loop && go test ./backend/modules/prompt/... -count=1 -v
    # Step 6.5: Lint 检查
    cd /Users/bytedance/workspace/src/github/coze-dev/coze-loop && golangci-lint run ./backend/modules/prompt/...


## Validation and Acceptance

1. **编译通过**：`go build ./backend/...` 零错误
2. **单元测试通过**：`go test ./backend/modules/prompt/... -count=1` 全部 PASS，0 failures，0 skipped
3. **Lint 通过**：`golangci-lint run ./backend/modules/prompt/...` 零 warning
4. **API 路由注册正确**：编译后检查生成的路由文件包含所有 7 个 Tool API 端点
5. **IDL 一致性**：生成的 KiteX 代码与 IDL 定义一致，ToolManageService 包含 7 个方法


## Documentation Update

需要更新的文档：
- `constitution/prompt-domain-specific-constitution.md`：添加 Tool Commit 的命名约定说明，与 Prompt Commit 命名约定保持一致风格

如果 prompt 模块下存在独立的模块级文档（AGENTS.md 或 README），也需要同步更新。经检查，当前仓库根目录无 AGENTS.md，因此不需要更新知识导航表。


## Idempotence and Recovery

- **IDL 代码生成**（upgrade-idl）：可安全重复执行，覆盖已有生成文件
- **SQL 文件创建**：幂等——文件内容相同时覆盖无副作用
- **GORM 模型生成**（upgrade-sql）：可安全重复执行
- **Wire 代码生成**（upgrade-wire）：可安全重复执行
- **手工代码文件**：如果中途失败，可从当前 Milestone 重新开始。Domain、Infra、Application 层之间有编译依赖，但每层内部的文件创建是独立的
- **回滚路径**：所有变更在本地分支 `20260326195427-pe-tools-management` 上，可通过 `git checkout main` 安全回退


## Artifacts and Notes

（待实施过程中填写关键输出）


## Interfaces and Dependencies

### 外部依赖（已存在于仓库）

| 依赖 | 用途 | 包路径 |
|---|---|---|
| CloudWeGo Hertz | HTTP 框架 | `github.com/cloudwego/hertz` |
| CloudWeGo KiteX | RPC 框架 | `github.com/cloudwego/kitex` |
| Google Wire | 依赖注入 | `github.com/google/wire` |
| GORM | ORM 框架 | `gorm.io/gorm` |
| GORM Gen | 代码生成 | `gorm.io/gen` |
| GoMock | Mock 生成 | `go.uber.org/mock/gomock` |

### 内部依赖（本模块新增接口签名）

    // domain/repo/tool.go
    type IToolRepo interface {
        CreateTool(ctx context.Context, toolDO *entity.Tool) (toolID int64, err error)
        GetTool(ctx context.Context, param GetToolParam) (*entity.Tool, error)
        MGetTool(ctx context.Context, queries []MGetToolQuery) (map[MGetToolQuery]*entity.Tool, error)
        ListTool(ctx context.Context, param ListToolParam) (*ListToolResult, error)
        SaveDraft(ctx context.Context, toolDO *entity.Tool) error
        CommitDraft(ctx context.Context, param CommitToolDraftParam) error
        ListCommitInfo(ctx context.Context, param ListToolCommitParam) (*ListToolCommitResult, error)
    }

    // domain/service/tool.go
    type IToolService interface {
        CreateTool(ctx context.Context, toolDO *entity.Tool) (toolID int64, err error)
        GetTool(ctx context.Context, param GetToolParam) (*entity.Tool, error)
        SaveDraft(ctx context.Context, toolDO *entity.Tool) error
    }

    // infra/repo/mysql/tool_basic.go
    type IToolBasicDAO interface {
        Create(ctx context.Context, basicPO *model.ToolBasic, opts ...db.Option) error
        Get(ctx context.Context, toolID int64, opts ...db.Option) (*model.ToolBasic, error)
        MGet(ctx context.Context, toolIDs []int64, opts ...db.Option) (map[int64]*model.ToolBasic, error)
        List(ctx context.Context, param ListToolBasicDAOParam) ([]*model.ToolBasic, *int64, error)
        Update(ctx context.Context, toolID int64, updateFields map[string]interface{}, opts ...db.Option) error
        Delete(ctx context.Context, toolID int64, opts ...db.Option) error
    }

    // infra/repo/mysql/tool_commit.go
    type IToolCommitDAO interface {
        Create(ctx context.Context, commitPO *model.ToolCommit, createdAt time.Time, opts ...db.Option) error
        Get(ctx context.Context, toolID int64, version string, opts ...db.Option) (*model.ToolCommit, error)
        MGet(ctx context.Context, pairs []ToolIDVersionPair, opts ...db.Option) (map[ToolIDVersionPair]*model.ToolCommit, error)
        Upsert(ctx context.Context, commitPO *model.ToolCommit, opts ...db.Option) error
        Delete(ctx context.Context, toolID int64, version string, opts ...db.Option) error
        List(ctx context.Context, param ListToolCommitDAOParam) ([]*model.ToolCommit, error)
    }
