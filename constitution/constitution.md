# Coze Loop 后端开发宪章

## 核心原则

### I. 领域驱动设计(DDD)架构 (NON-NEGOTIABLE)

目录结构标准
```
backend/modules/{module}/
├── application/    # Application 应用层: DTO<->DO 转换, 应用服务, Wire 配置
├── domain/         # Domain 领域层: 实体, 领域服务, 仓储接口定义
└── infra/          # Infrastructure 基础设施层: 仓储实现, RPC, 指标收集
```
严格遵循 DDD 三层架构,明确层次边界与职责:
- **层次依赖**: Application → Domain ← Infrastructure,依赖方向单向不可逆
- **接口优先**: Domain 层定义接口,Infrastructure 层实现,禁止反向依赖
- **数据隔离**: Domain 层禁止引用 DTO/PO 结构,仅使用 DO(领域对象)
- **数据流转规则**:
  - DTO ↔ DO 转换必须在 Application 层完成
  - DO ↔ PO 转换必须在 Infrastructure 层的 Repository 实现中完成
  - DAO 层只能使用 PO,严禁使用 DO
- **包边界**: 使用依赖注入和接口优先设计避免循环导入
- **关于 domain/component/ 的用法说明**:
  - 出 repo 外，其他的组件的接口都定义在这里，其实现类都在 infra 目录下
  - 比如 rpc, 沙箱 等组件都定义在这里，其实现类都在 infra 目录下

### II. 单一模块原则 (NON-NEGOTIABLE)

每个开发阶段(Tasks里的Phase)必须专注于单一模块(`backend/modules/{module}`),严格控制变更范围:
- **单一焦点**: 一个Phase只能涉及一个模块,不得跨模块修改
- **明确边界**: 开始前必须明确定义模块范围
- **零跨模块变更**: 未经明确批准,严禁修改其他模块的 IDL、数据库、配置或代码
- **变更评估**: 必须在开发前识别所需变更类型(IDL/数据库/配置/业务逻辑)
- **跨模块的功能开发最佳实践**: 
  - 例如，对于需求功能 "将[观测模块]的Trace数据导入到[评测模块]的评测集中"
    - 该功能涉及到两个模块的交互，技术实现上涉及到 `modules/observability` 对 `modules/evaluation` 的RPC调用
    - 则两模块的开发必须拆分到两个不同的Phase中
    - 且，必须用RPC调用，不能直接在 `modules/observability` 中 import `modules/evaluation` 的代码
      - 开发团队需要协调这两个模块的开发进度，确保在一个Phase内完成
        - RPC接口实现&调用的规范如下：
          - RPC接口定义以及RPC框架位于 `backend/kite_gen/` 目录下，实现则是在 `backend/modules/{module}/application/` 目录下
          - 一个模块A对另一个模块B的RPC调用，必须遵循DDD原则，A要对B的原始RPC接口进行一层封装，封装的接口定义在 `modules/{module}/domain/component/rpc` 目录下
            - 封装接口的实现在 `modules/{module}/infra/rpc` 目录下
            - B的原始RPC的DTO不能直接被引入到A的 `domain` 层，需要封装一下才行，两者之间的转换在 `infra/rpc/convertor/` 里
      - 最终，B的RPC的Local KiteX RPC Client要通过依赖注入的方式注入给A，参考 @backend/api/api.go 里的代码
```go
	promptHandler, err := apis.InitPromptHandler(ctx, idgen, db, cmdable, meter, configFactory, limiterFactory, benefitSvc,
        loruntime.NewLocalLLMRuntimeService(llmHandler.LLMRuntimeService),
        loauth.NewLocalAuthService(foundationHandler.AuthService),
        lofile.NewLocalFileService(foundationHandler.FileService),
        louser.NewLocalUserService(foundationHandler.UserService),
        auditClient,
    )
```

### III. 向前兼容性原则 (NON-NEGOTIABLE)

所有变更必须保持向前兼容,扩展而非替换:
- **方法演进**: 扩展现有方法,不创建新方法(除非语义根本不兼容)
- **可选参数**: 新增参数必须使用指针类型或零值,保证可选性
- **默认行为**: 省略新参数时,行为必须与之前版本完全一致
- **数据库变更**: 仅允许添加列,严禁修改或删除历史字段
- **软删除模式**: 逻辑删除必须使用软删除模式,保留历史数据

### IV. 扩展性法则 (NON-NEGOTIABLE)

#### 方法扩展法则：params 结构体传参，不要字段打散传参

- 仓储方法扩展
**场景**: 向现有 ListPrompt 方法添加基于创建者的过滤
**之前** (❌ **禁止**):
```go
// 错误:创建新方法而不是扩展现有方法
func (r *repository) ListPromptByCreator(ctx context.Context, spaceID int64, creator string) ([]Prompt, error)

// 错误:创建多个特定过滤器的方法
func (r *repository) ListPromptByCreatorAndSpace(ctx context.Context, spaceID int64, creator string) ([]Prompt, error)
```
**之后** (✅ **必需**):
```go
// 正确:使用可选参数扩展现有 ListPrompt 方法
func (r *repository) ListPrompt(ctx context.Context, params ListPromptParams) ([]Prompt, error)

// 带有可选过滤字段的 ListPromptParams 结构体
type ListPromptParams struct {
    SpaceID  *int64  `json:"space_id,omitempty"`
    Creator  *string `json:"creator,omitempty"`
    // 可以添加额外的过滤器而不破坏现有代码
}
```

- 服务接口扩展
**场景**: 向服务层添加新的过滤能力
**之前** (❌ **禁止**):
```go
// 错误:为每个新过滤器创建新服务方法
func (s *service) ListPromptByCreator(ctx context.Context, req ListPromptByCreatorRequest) (*ListPromptResponse, error)
```
**之后** (✅ **必需**):
```go
// 正确:扩展现有服务方法
func (s *service) ListPrompt(ctx context.Context, req ListPromptRequest) (*ListPromptResponse, error)

// 使用新的可选字段扩展请求结构体
type ListPromptRequest struct {
    SpaceID  *int64  `json:"space_id,omitempty"`
    Creator  *string `json:"creator,omitempty"`
}
```


#### API 端点扩展法则

**场景**: 向 REST 端点添加查询参数
**之前** (❌ **禁止**):
```go
// 错误:为每个过滤器组合创建新的 API 端点
GET /api/v1/prompts/by-creator/{creator}
GET /api/v1/prompts/by-creator/{creator}/space/{spaceId}
```
**之后** (✅ **必需**):
```go
// 正确:使用查询参数扩展现有端点
GET /api/v1/prompts?space_id=123&creator=alice
```

### V. 模型定义规范

#### domain entity model定义规范

- 位置：`backend/modules/{module}/domain/entity/`
- 定义规范：
  1. 主键ID都是int64
  2. Key这些都是string

#### sql 表定义规范

> 参考如下

```sql
CREATE TABLE IF NOT EXISTS `space`
(
    `id`          bigint(20) unsigned NOT NULL COMMENT 'Primary Key ID, Space ID',
    `owner_id`    bigint(20) unsigned NOT NULL DEFAULT 0 COMMENT 'Owner ID',
    `name`        varchar(200)        NOT NULL DEFAULT '' COMMENT 'Space Name',
    `description` varchar(2000)       NOT NULL DEFAULT '' COMMENT 'Space Description',
    `space_type`  tinyint(4)          NOT NULL DEFAULT '0' COMMENT 'Space Type, 1: Personal, 2: Team',
    `icon_uri`    varchar(200)        NOT NULL DEFAULT '' COMMENT 'Icon URI',
    `created_by`  bigint(20) unsigned NOT NULL DEFAULT 0 COMMENT 'Creator ID',
    `deleted_at`  bigint              NOT NULL DEFAULT '0' COMMENT '删除时间',
    `created_at`  datetime            NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at`  datetime            NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`id`),
    KEY `idx_owner_id` (`owner_id`),
    KEY `idx_creator_id` (`created_by`)
) ENGINE = InnoDB
  DEFAULT CHARSET = utf8mb4
  COLLATE = utf8mb4_unicode_ci COMMENT = 'Space Table';
```

变更规则：
- **表命名**: 如果实体B是附在实体A上的，则关系表命名为 "实体A_实体B_ref"
  - 举个例子，在 abc 上打 label，那关系表命名为 "abc_label_ref"
  - 另外，实体表名一般带有domain的前缀，比如 prompt_abc, prompt_label，那关系表只需要出现一次前缀即可 "prompt_abc_label_ref"
- **必选字段**: 实体表必须有space_id,created_at,created_by,updated_at,updated_by,deleted_at这5个，关系表则不需要有_by字段

### VI. 事务管理规范
明确事务边界与使用规则:
- **层次限制**: 事务只能在 Infrastructure 层的 Repository 里启动,严禁在 Application 层 和 Infrastructure 层的 DAO 里开启事务
- **DAO 设计**: 所有 DAO 接口必须预留 `opts ...db.Option` 变长参数用于事务传递
- **事务传递**: Repository 通过 `db.WithTransaction(tx)` 将事务传递给 DAO 层
- **原子性**: 多表操作必须在单一事务中执行,保证数据一致性
- **会话复用**: DAO 层必须使用 `query.Use(d.db.NewSession(ctx, opts...))` 复用事务
- **Repository 样例展示**: 
```go
// Repository 层
func (r *repository) CreateWithTransaction(ctx context.Context, entity Entity) error {
    return r.db.Transaction(ctx, func(tx *gorm.DB) error {
        opt := db.WithTransaction(tx)

        // 跨多个操作使用事务
        if err := r.dao.Create(ctx, entity, opt); err != nil {
            return err
        }

        if err := r.relatedDao.Create(ctx, related, opt); err != nil {
            return err
        }

        return nil
    })
}
```

### VII. 代码生成工具链 & 规范

规范化的代码生成流程:
- **IDL 生成**: 变更 IDL(Thrift 文件) 后，**必须使用** upgrade-idl skills
  - 完成后，如果有IDL新增接口则 **apis接口也需要同步新增**: 
    - 位置：`backend/api/handler/coze/loop/apis/` 
    - **必须用** invokeAndRender 调用 KiteX 生成的 RPC Client 接口，参考如下
```go
// UpdateEvaluatorRecord .
// @router /api/evaluationv3/evaluator_records/:evaluator_record_id [PATCH]
func UpdateEvaluatorRecord(ctx context.Context, c *app.RequestContext) {
	invokeAndRender(ctx, c, localEvaluatorSvc.UpdateEvaluatorRecord)
}
```
- **数据库生成**: 变更 Schema(SQL 文件) 后，**必须使用** upgrade-sql skills
- **依赖注入**: 变更 wire.go 后，**必须使用** upgrade-wire skills
    - **尤其注意**: wire.go中不得定义 NewXxx 这样的函数，NewXxx 根据 DDD 法则，只能定义在各自组件impl的文件中
- **错误码变更**: 变更 errorx的yaml后，**必须使用** upgrade-bizcode skills
- **配置变更**: 变更 conf的yaml后，**必须使用** upgrade-config skills
- **只读文件**: kitex_gen/、loop_gen/、wire_gen.go 等生成文件严禁手动修改


### VIII. domain 特定 constitution

1. Prompt domain specific constitution 详见 [prompt-domain-specific-constitution.md](./prompt-domain-specific-constitution.md)
2. Evaluation domain specific constitution 详见 [evaluation-domain-specific-constitution.md](./evaluation-domain-specific-constitution.md)

## 开发流程约束

### 五阶段开发流程
严格按照以下顺序执行开发任务:
1. **需求分析**: 模块识别 + 变更评估
2. **IDL 定义**: Thrift 文件编写 + 代码生成
3. **数据库变更**: 多环境 SQL 同步 + GORM 代码生成
4. **配置更新**: 多环境配置同步
5. **核心开发**: DDD 架构实现 + 错误处理 + 事务管理

### 代码质量门禁
所有 PR 必须通过以下检查:
- **静态检查**: golangci-lint 代码质量检查
- **格式化**: go fmt 统一代码格式
- **安全扫描**: 凭证处理静态安全分析
- **方法演进**: 自动检查新方法创建合规性
