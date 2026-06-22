> **2026-06-22 范围收窄**：data + observability 服务实现移出本 change（用户澄清）。共享 IDL 类型 `SourceType` + `DatasetIOTrace` 保留。容量上调（1MB 字段 / 5MB 行）**OUT-OF-SCOPE**，须由 data 服务 owner 独立排期协调。原 §9 / §10 / §12 跨模块任务已删除；§11 仅保留 evaluation 侧消费 observability 现有 `ListTrajectory` 的部分；evaluation 自实现单列导入（不依赖新增 data RPC）。

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
- [ ] 2.3 评测集：Trace import adapter（调用 observability **现有** `ListTrajectory` RPC，as-is，转换为 Trajectory JSON）
- [ ] 2.4 评测集：文件导入（JSON/CSV）的 Trajectory 列推断 + 校验路径
- [ ] 2.5 评测集：`BatchUpsertEvaluationSetItemColumns` 实现 —— **evaluation 自实现**（先 Get 整行 → 合并 patch 列 → Put 整行，加乐观锁；不依赖新增 data RPC）
- [ ] 2.6 评估器：三个内置评估器元数据 seeding（开源版仅声明元数据；实际执行实现挂 commercial）
- [ ] 2.7 实验：`TargetFieldMapping` / `EvaluatorFieldMapping` 校验链路补充 Trajectory ContentType 一致性
- [ ] 2.8 实验：ExperimentRunner 把 Trajectory 字段透传到 EvaluatorInputData
- [ ] 2.9 实验：`InsightAnalysisExperimentRow` 实现 + `ExptInsightAnalysisRecord.scope` 持久化

## 3. Commercial 层（cozeloop-commercial）

- [ ] 3.1 三个内置评估器的执行实现（builtin_trajectory_tool / planning / context_memory）；选择 Prompt 或 Code 承载形态
- [ ] 3.2 evaluator info 元数据（benchmark / vendor / user_manual_url）填充
- [ ] 3.3 Wire DI 注册 + commercial 评估器路由
- [ ] 3.4 Row 级智能解读的 LLM prompt + 模型路由（沿用现有 Insight 服务）

## 4. 跨域协调（OUT-OF-SCOPE，仅记录）

> 以下项 **不在本 change 内交付**，仅记录给 data / observability 服务 owner 后续独立排期：

- [ ] 4.1 (OUT-OF-SCOPE) 数据服务：单字段 1MB / 单行 5MB 容量上调 — 由 data 服务 owner 独立排期；evaluation 在容量未上调前用户层裁剪 / MQ oversize 旁路降级
- [ ] 4.2 (OUT-OF-SCOPE) 数据服务：部分列更新原子能力 — 本 change evaluation 自实现部分列 upsert，不依赖该能力
- [ ] 4.3 (NO ACTION) 观测服务 `ListTrajectory`：消费**现有** RPC 契约 as-is，不要求 observability 侧新增 warning / 鉴权 / 限流改动

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

## 9. evaluation 侧消费 observability 现有 `ListTrajectory` 的集成（新增，2026-06-22 收窄后）

> 范围：evaluation 单模块；不要求 data / observability 服务侧做任何改动。

### 9.1 Domain port（evaluation 自定义）

- [ ] 9.1.1 Domain port：`TraceTrajectoryParserPort.ParseByTraceIDs(ctx, workspace_id, trace_ids, filter, start_time)` — 抽象消费 observability `ListTrajectory` 的边界

### 9.2 Infra adapter（evaluation 自维护）

- [ ] 9.2.1 `infra/observability_client.go`：实现 `TraceTrajectoryParserPort`，调用 **现有** observability `ListTrajectory` RPC（as-is）

### 9.3 业务串联

- [ ] 9.3.1 `ParseImportSourceFile` 扩展 source_type=Trace：通过 port 调用 observability，转换 `Trajectory` 为评测集 `FieldData`（schema_key="trajectory"）
- [ ] 9.3.2 失败语义映射：observability `ListTrajectory` 单 trace_id 失败 / response 中空 Trajectory → evaluation 层映射为 `ItemErrorType.GetTraceFailed`
- [ ] 9.3.3 容量降级：单 Trajectory 字段超过 data 现有上限（100KB）时，evaluation 层返回 `FIELD_SIZE_EXCEEDED` 引导用户裁剪；MQ 投递大 Trajectory 走 oversize-payload 旁路（见 `wiki/dev-guides/middleware/cozeloop-mq-guide.md`）

---

## 10. evaluation 侧单列导入 E2E 测试（合并到 §5 测试矩阵）

> data 服务不动，evaluation 自实现单列 upsert（先 Get→合并→Put + 乐观锁），相关测试已并入 §5.3 / §5.5；不另开测试章节。

---

## 11. OUT-OF-SCOPE 跟踪事项（仅记录，不本 change 推进）

- [ ] 11.1 data 服务：单字段 100KB → 1MB、单行 1MB → 5MB 容量上调 — 由 data 服务 owner 独立排期协调；evaluation 在上调生效前保持现有上限。
- [ ] 11.2 data 服务：部分列更新原子能力（`BatchPatchDatasetItems` 或等价能力）— 本 change evaluation 自实现 fallback，未来若 data 上线该能力，evaluation 可切换 adapter。
- [ ] 11.3 observability 服务：`ListTrajectory` warning 通道 / 鉴权 / 限流增强 — 不在本 change 范围；evaluation 消费现有契约 as-is。
