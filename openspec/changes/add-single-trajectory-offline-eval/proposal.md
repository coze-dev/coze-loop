# Proposal: 单轨迹离线评测能力 (Single-Trajectory Offline Evaluation)

> **Change name**: `add-single-trajectory-offline-eval`
> **Platform**: Fornax & CozeLoop（评测域）
> **季度**: 2025Q4（10月）
> **Branch**: `feat/single-trajectory-offline-eval`（fork from `eval/third_party_evaluator`）
> **PSM**: `stone.cozeloop.evaluation`，BAM service-id `26411`

## Why

随着 AI Agent 应用普及，开发者关注的「应用质量」从单次回复正确性扩展到完整 Agent 工作轨迹（Trajectory）的质量。CozeLoop / Fornax 评测平台现有评测集（Dataset）、评估器（Evaluator）、实验（Experiment）能力仅支持字符串 / 多模态 / 多轮对话等数据类型，无法承载 Agent 轨迹（含工具调用、规划链、上下文记忆等结构化步骤），导致：

- 用户无法把线上观测到的 Trace 直接导入评测集做离线回归
- 缺少针对 Agent 轨迹的内置评估器（如工具调用正确性、规划合理性、上下文记忆），用户需要自行写 Prompt Evaluator
- 实验报告无法可视化展示完整 Agent 工作过程，问题定位依然依赖 Trace 平台跳转
- Phase 2 远期能力（第三方 RPC/ByteFaas 评估器、连续值/类别/自由文本输出、报告→数据集导入）也需要轨迹数据基础

为 2025Q4 内场 GA 提供端到端的「轨迹评测」闭环，让 Agent 开发者一站式完成 Trace → Dataset → 评估 → 实验 → 解读 全链路。

## What Changes

### Phase 1（本次 Change 范围）

1. **评测集 — Trajectory 数据类型**
   - 新增 `Trajectory` 字段类型 + JSON Schema 预定义结构（steps[] 含 role/type/content/tool_calls/timestamp 等），评测集创建后默认带 `trajectory` 列
   - 导入方式：(a) 按 trace_id 解析导入（消费 observability 现有 `ListTrajectory` RPC 契约，as-is）(b) 文件导入（JSON/CSV，schema 校验）
   - 单列导入到已有行：通过 evaluation 自身的 `BatchUpsertEvaluationSetItemColumns` RPC 实现（不依赖新增 data RPC）
   - **OUT-OF-SCOPE**：单字段 100KB → 1MB、单行 1MB → 5MB 容量提升需 data 服务侧改动，本 change 不交付，须由 data 服务 owner 独立协调（详见 design.md / tasks.md）
2. **评估器 — 内置轨迹评估器**
   - Phase 1 优先级：`tool`（工具调用正确性）> `planning`（规划合理性）> `context_memory`（上下文记忆）
   - 复用现有 `Evaluator.builtin = true` + `EvaluatorBoxType.Black` 模型；新增 `ContentType.Trajectory` 入参标识
3. **实验 — 字段映射 / 轨迹可视化 / 智能解读**
   - 实验配置时 `Trajectory` 字段需校验数据集列存在性 + 类型一致性
   - 报告中以时间轴 / step-by-step 形式展示轨迹
   - 新增 Row 级智能解读 RPC；保留并兼容现有 Experiment 级 `InsightAnalysisExperiment`

### Phase 2（仅在 proposal 中声明，不在本次 spec 中展开）

- 第三方服务评估器（RPC 类型 / ByteFaas HTTP 类型）— Phase 2 占位
- 评估器输出形态扩充：连续值 / 类别 / 自由文本（现 EvaluatorResult 仅 double）— Phase 2 占位
- 实验报告导入评测集 — Phase 2 占位

## Capabilities

### New Capabilities (evaluation only)

- `evaluation/dataset-trajectory-type`: 评测集新增 Trajectory 字段类型与 JSON Schema 校验；默认 `trajectory` 列；ContentType 扩展或 schema_key 复用
- `evaluation/dataset-trajectory-import`: 按 trace_id 解析导入 / 文件导入（JSON/CSV）+ Trajectory schema 校验
- `evaluation/dataset-single-column-import`: 单列追加 / 部分列更新到已有行（不覆盖其它列）
- `evaluation/builtin-trajectory-evaluator`: 三个内置黑盒轨迹评估器（tool / planning / context_memory）
- `evaluation/experiment-trajectory-mapping`: 实验配置字段映射对 Trajectory 类型的存在性 + 一致性校验
- `evaluation/experiment-trajectory-report`: 报告中 Trajectory 时间轴可视化 + Row 级智能解读 RPC

> **2026-06-22 范围收窄**：原计划的 data / observability 模块 capability（`dataset-capacity-upgrade` / `dataset-single-column-import`（data 侧） / `trace-to-trajectory-parser`）从本 change 移除。共享 IDL 类型 `SourceType` + `DatasetIOTrace` 保留（evaluation 依赖），但 data / observability 服务不做实现改动。

### Modified Capabilities

> 本次为评测域首次正式启用 OpenSpec，仓库尚无 `docs/xdev/openspec/specs/` 主 spec。所有 spec 均以 ADDED 形式声明，作为后续 archive 时 sync 入主 spec 的基线；对底层 IDL 的修改在 `impact-analysis.md` 中单独列出。

## Impact

### Code / IDL

> **范围**：本 change 仅修改 evaluation 模块 IDL 与服务实现。共享 IDL 类型 `SourceType`（来自 data 域）与 `DatasetIOTrace`（来自 data 域）保持现状，evaluation 引用消费；data / observability 服务实现不在本 change 内改动。

- `coze-loop/idl/thrift/coze/loop/evaluation/domain/common.thrift`
  - 新增 `ContentType.Trajectory` 枚举值（与现有 `ArgSchemaTextType.Trajectory` / `ArgSchemaKey_Trajectory` 协同）
- `coze-loop/idl/thrift/coze/loop/evaluation/domain/eval_set.thrift`
  - `FieldSchema` 借助 `content_type=Trajectory` + `schema_key="trajectory"` + `text_schema`（存 JSON Schema）承载轨迹列
- `coze-loop/idl/thrift/coze/loop/evaluation/coze.loop.evaluation.eval_set.thrift`
  - 扩展 `ParseImportSourceFile` 支持 trace_id 来源（消费 observability 现有 `ListTrajectory` 契约）
  - 新增 `BatchUpsertEvaluationSetItemColumns` RPC，由 evaluation 自身实现单列部分更新（不依赖新增 data RPC）
- `coze-loop/idl/thrift/coze/loop/evaluation/domain/evaluator.thrift`
  - `EvaluatorInputData` 增加对 `Trajectory` ContentType 的承载（复用现有 multi-part 模型）
  - 新增三个内置评估器 seed 数据（builtin=true, box_type=Black, tag=Trajectory）
- `coze-loop/idl/thrift/coze/loop/evaluation/domain/expt.thrift`
  - `TargetFieldMapping` / `EvaluatorFieldMapping` 校验路径补充对 Trajectory 类型的检查
  - 报告侧 `ExperimentTurnPayload.trajectory_analysis_result` 已存在，沿用
- `coze-loop/idl/thrift/coze/loop/evaluation/coze.loop.evaluation.expt.thrift`
  - 新增 Row 级解读 RPC（沿用 `ExptInsightAnalysisRecord` 通用结构）

### Downstream

- **前端 (`repos/coze-loop-frontend`)**: 数据集列编辑器（Trajectory 列预览）/ 实验配置字段映射 UI / 实验报告 Trajectory 时间轴组件 / 智能解读面板
- **商业版 (`repos/cozeloop-commercial`)**: 内场上线，沿用 commercial DI；初期 builtin 评估器实现挂在 commercial 域以利快速迭代
- **观测模块（observability）**: evaluation 消费**现有** `ListTrajectory` RPC（as-is，无契约改动）；observability 服务实现不在本 change 范围

### Migration

- 老评测集自动迁移：通过列扩展默认携带 `trajectory` 列（lazy 初始化，对老数据无影响）
- 旧实验报告不受影响，不展示 Trajectory tab

### Out-of-Scope (deferred)

- **数据集容量上调（1MB 字段 / 5MB 行）**：原 PRD 提到的容量提升落在 data 服务，本 change 不交付，须由 data 服务 owner 独立协调与排期。
- **trace_id 解析后端能力**：observability 现有 `ListTrajectory` RPC 已够用，本 change 不要求 observability 侧新增 RPC / 改动 warning 通道 / 改动鉴权语义。

### Risks

- 数据容量未上调时，Trajectory 字段较大可能触达现有 size 上限 → 用户层裁剪 / oversize 旁路降级
- Phase 1 不实现 Phase 2 的输出形态扩充，需保持 `EvaluatorResult.score` 兼容
