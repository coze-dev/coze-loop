## 1. IDL 变更（coze-loop/idl/thrift/coze/loop/evaluation/）

- [ ] 1.1 `domain/common.thrift`: 新增 `ContentType.Trajectory` 枚举值；与已有 `ArgSchemaTextType.Trajectory` / `ArgSchemaKey_Trajectory` 协同
- [ ] 1.2 `domain/eval_set.thrift`: `FieldSchema` 沿用 `content_type + schema_key + text_schema` 承载 Trajectory；新增 `BizCategory.FromOnlineTrace` 已存在，复用
- [ ] 1.3 `domain/evaluator.thrift`: `EvaluatorInputData` 增加 Trajectory ContentType 承载；新增三个内置评估器 seed（builtin=true, box_type=Black, tag=Trajectory）
- [ ] 1.4 `domain/expt.thrift`: 在 `ExptInsightAnalysisRecord` 新增 `scope` 字段（Experiment / Row）；`TargetFieldMapping` / `EvaluatorFieldMapping` 注释补充 Trajectory 校验语义
- [ ] 1.5 `coze.loop.evaluation.eval_set.thrift`:
  - 扩展 `ParseImportSourceFile` 支持 trace_id 来源（在 source_type 上扩展）
  - 新增 `BatchUpsertEvaluationSetItemColumns` RPC
- [ ] 1.6 `coze.loop.evaluation.expt.thrift`: 新增 `InsightAnalysisExperimentRow` RPC（参数 expt_id + item_id + turn_id）
- [ ] 1.7 IDL 同步：CI sync 触发 `cozeloop-idl-commercial/open/`；`saas/` 部分手动 sync

## 2. Domain / Service 层（coze-loop/backend）

- [ ] 2.1 评测集：Trajectory FieldSchema 默认列 lazy 注入逻辑
- [ ] 2.2 评测集：Trajectory JSON Schema validator 与版本路由器（domain/dataset/validator）
- [ ] 2.3 评测集：Trace import adapter（调用 observability 模块 trace 解析接口，转换为 Trajectory JSON）— **依赖 observability 接口契约**
- [ ] 2.4 评测集：文件导入（JSON/CSV）的 Trajectory 列推断 + 校验路径
- [ ] 2.5 评测集：`BatchUpsertEvaluationSetItemColumns` 实现 —— **依赖数据模块部分列更新原子能力**
- [ ] 2.6 评估器：三个内置评估器元数据 seeding（开源版仅声明元数据；实际执行实现挂 commercial）
- [ ] 2.7 实验：`TargetFieldMapping` / `EvaluatorFieldMapping` 校验链路补充 Trajectory ContentType 一致性
- [ ] 2.8 实验：ExperimentRunner 把 Trajectory 字段透传到 EvaluatorInputData
- [ ] 2.9 实验：`InsightAnalysisExperimentRow` 实现 + `ExptInsightAnalysisRecord.scope` 持久化

## 3. Commercial 层（cozeloop-commercial）

- [ ] 3.1 三个内置评估器的执行实现（builtin_trajectory_tool / planning / context_memory）；选择 Prompt 或 Code 承载形态
- [ ] 3.2 evaluator info 元数据（benchmark / vendor / user_manual_url）填充
- [ ] 3.3 Wire DI 注册 + commercial 评估器路由
- [ ] 3.4 Row 级智能解读的 LLM prompt + 模型路由（沿用现有 Insight 服务）

## 4. 跨域协调（must_ask）

- [ ] 4.1 数据模块：单字段 1MB / 单行 5MB 容量上调 — 负责人骆弘珊，需排期 & 评估存储影响
- [ ] 4.2 数据模块：部分列更新原子能力补齐 — 同上
- [ ] 4.3 观测模块（observability）：`trace_id → trajectory` 解析接口契约 — 待对齐接口名 / 鉴权 / 失败语义

## 5. 单元测试 / 集成测试

- [ ] 5.1 Trajectory schema validator 单测（合法 / 缺字段 / 超 size）
- [ ] 5.2 ParseImportSourceFile 的 Trajectory 列推断测试
- [ ] 5.3 BatchUpsertEvaluationSetItemColumns 部分列更新行为测试
- [ ] 5.4 三个内置评估器的 DebugBuiltinEvaluator 黄金用例（每个评估器 ≥ 3 case）
- [ ] 5.5 TargetFieldMapping / EvaluatorFieldMapping Trajectory 一致性校验测试
- [ ] 5.6 InsightAnalysisExperimentRow 端到端测试（Row scope 标识 + Status 流转）

## 6. 前端联调（coze-loop-frontend，本 change 不直接产出代码）

- [ ] 6.1 列编辑器支持 Trajectory ContentType（预览 + 时间轴）
- [ ] 6.2 实验配置字段映射 UI 显示 Trajectory 字段
- [ ] 6.3 实验报告 Trajectory 时间轴 + step-by-step 视图
- [ ] 6.4 行级智能解读面板（区分 scope）

## 7. 上线 / 运维

- [ ] 7.1 灰度策略：内场 workspace 白名单
- [ ] 7.2 核心指标埋点（导入成功率 / 评估器调用 P95 / 报告渲染成功率 / 解读任务成功率）
- [ ] 7.3 Runbook：Trajectory 导入失败 / 内置评估器调用失败 / 行级解读异常的处理流程

## 8. SDD 归档

- [ ] 8.1 PR 合入后，将本 change 通过 `/openspec archive` 归档，merge delta 到主 spec
- [ ] 8.2 同步 wiki/entities / wiki/processes 中相关条目（评测集 / 评估器 / 实验三元组）

---

## 9. data 模块任务（新增，2026-06-22 扩展）

> **依赖序**：IDL 扩展 → kitex_gen → Domain → Infra → Application → Wire。**单字段 / 单行容量上调与单列导入解耦**，可并行。

### 9.1 IDL 扩展（`repos/coze-loop/idl/thrift/coze/loop/data/`）

- [ ] 9.1.1 `domain/dataset.thrift`: `DatasetSpec.max_item_size` 默认值由 1MB 调整为 5MB（IDL 字段结构不变；调整生效在 Domain 默认填充逻辑）
- [ ] 9.1.2 `domain/dataset.thrift`: `ItemErrorDetail` 注释补充 `scope=field|row` 区分（仅文档；IDL 不破坏）
- [ ] 9.1.3 `coze.loop.data.dataset.thrift`: 新增 `BatchPatchDatasetItems` RPC（部分列更新底座原子能力）。请求 `{dataset_id, items[].{item_id|item_key, data[FieldData]}}`；响应 `{patched_count, errors[ItemErrorGroup]}`
- [ ] 9.1.4 IDL 同步：CI sync 到 `cozeloop-idl-commercial/open/data/`

### 9.2 Domain / Infra / Service / Application 层（`repos/coze-loop/backend/modules/data/`）

- [ ] 9.2.1 Domain：`DatasetSpec` 默认值常量统一为 5MB；字段级隐含 1MB 上限以校验函数承载
- [ ] 9.2.2 Domain：行级容量校验函数 `ValidateItemSize` 区分 `scope=field|row`，错误细化
- [ ] 9.2.3 Domain：部分列 patch 原子操作（事务边界 + 乐观锁 / 行锁 + 未指定列保留）
- [ ] 9.2.4 Infra：存储侧（RDS / Abase）检查 column 长度限制；如有 mediumtext / longtext 升级在此处落地
- [ ] 9.2.5 Service：`BatchPatchDatasetItems` 实现，复用 `ValidateDatasetItems` 单列子集校验
- [ ] 9.2.6 Application：RPC handler 注册；按 `dataset_io_jobs` 记录 `job_type = "column_patch"`
- [ ] 9.2.7 Wire DI：注入 patch service 到 dataset module 出口

### 9.3 上游耦合校准（依赖 [9.1]）

- [ ] 9.3.1 HTTP 网关 / Thrift payload 上限校验：BatchCreate / BatchPatch 单次 payload 上限 ≥ 5MB × 批量条数；超限明确返回 `REQUEST_BODY_TOO_LARGE`
- [ ] 9.3.2 MQ oversize-payload 旁路：评测运行投递大 Trajectory 时启用外存指针；见 `wiki/dev-guides/middleware/cozeloop-mq-guide.md`

---

## 10. observability 模块任务（新增）

> 已有 `ListTrajectory` RPC，本 change 不动 IDL，只补语义 + 契约固化。

### 10.1 IDL（`repos/coze-loop/idl/thrift/coze/loop/observability/`）

- [ ] 10.1.1 `coze.loop.observability.trace.thrift`: `ListTrajectory` 不改字段；release-notes 中标注"跨域稳定契约"
- [ ] 10.1.2 评估补 `ListTrajectoryResponse` 的 warning 通道：复用 `BaseResp` 扩展字段或新增 optional `warnings: list<TraceParseWarning>`（决定权在 observability owner）

### 10.2 Domain / Service / Application

- [ ] 10.2.1 Service：`ListTrajectory` 实现强化——单 trace_id 失败时位置对齐返回空 Trajectory + warning，整体 200
- [ ] 10.2.2 Service：workspace 鉴权强制（trace 跨 workspace 拒绝）
- [ ] 10.2.3 Service：trace 部分 span 缺失 / broken 时 step.BasicInfo.error 填充
- [ ] 10.2.4 Service：按 workspace 维度限流（与现有 RateLimit middleware 集成），超限返回 `RATE_LIMITED`
- [ ] 10.2.5 Application：把 `UpsertTrajectoryConfig` / `GetTrajectoryConfig` / `ListTrajectory` 三件套作为跨域 SDK 暴露（go module 内的 public client）

---

## 11. evaluation 模块对跨域调用的集成（新增）

### 11.1 跨域 port

- [ ] 11.1.1 Domain port：`TraceTrajectoryParserPort.ParseByTraceIDs(ctx, workspace_id, trace_ids, filter, start_time)`
- [ ] 11.1.2 Domain port：`DatasetItemColumnPatchPort.BatchPatch(ctx, dataset_id, items[])`

### 11.2 Infra adapter

- [ ] 11.2.1 `infra/observability_client.go`：实现 `TraceTrajectoryParserPort`，调用 observability `ListTrajectory`
- [ ] 11.2.2 `infra/data_client.go`：实现 `DatasetItemColumnPatchPort`，调用 data `BatchPatchDatasetItems`

### 11.3 业务串联

- [ ] 11.3.1 `ParseImportSourceFile` 扩展 source_type=Trace：通过 port 调用 observability，转换 `Trajectory` 为评测集 `FieldData`（schema_key="trajectory"）
- [ ] 11.3.2 `BatchUpsertEvaluationSetItemColumns`：业务侧校验（schema validation + import audit）后通过 port 调用 data 底座 patch
- [ ] 11.3.3 失败语义映射：observability warning → `ItemErrorType.GetTraceFailed`（新增子型）；data Mismatch → 原样透传

---

## 12. 跨模块测试矩阵（新增）

- [ ] 12.1 data: `BatchPatchDatasetItems` E2E（成功 / 列不存在 / 类型不匹配 / 并发冲突 / 行级超限）
- [ ] 12.2 data: 单字段 1MB / 单行 5MB 边界用例（恰好 / 超出）
- [ ] 12.3 observability: `ListTrajectory` E2E（trace 存在 / 不存在 / 部分 span broken / workspace 鉴权失败 / 超过 batch 10 上限）
- [ ] 12.4 evaluation × observability：trace_id 导入端到端（mock observability 返回 Trajectory → 评测集列写入）
- [ ] 12.5 evaluation × data：`BatchUpsertEvaluationSetItemColumns` 业务接口 E2E（含底座调用 mock）

---

## 13. 跨模块依赖与并行策略

| 任务组 | 依赖 | 并行性 |
|--------|------|--------|
| §9.1 IDL data | 无 | 与 §10.1 并行 |
| §9.2 data Domain/Service | §9.1 | data 内部串行 |
| §10.1 observability IDL | 无 | 与 §9 并行 |
| §10.2 observability service | §10.1 | observability 内部串行 |
| §11 evaluation 跨域集成 | §9.1 + §10.1 完成 kitex_gen 后即可 | evaluation 内部任务可并行 |
| §12 测试矩阵 | §9 + §10 + §11 完成 | 测试阶段并行 |

> evaluation 已有 6 个 capability 的实现（任务 §1-§8）与新增的 §9 / §10 解耦，可与跨模块 IDL 改动并行推进，调用边界先 mock。
