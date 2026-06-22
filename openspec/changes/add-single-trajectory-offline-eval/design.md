## Context

CozeLoop 评测平台采用 DDD 分层（L0-L8）与 Thrift IDL 单源（`coze-loop/idl/`）。当前评测域已具备评测集 / 评估器 / 实验三元组的完整 CRUD 与运行能力，但不支持 Agent 轨迹（Trajectory）这一类结构化数据。

调研 IDL 后发现 Trajectory 相关基础设施已部分铺垫：

- `evaluation/domain/common.thrift` 中已有 `ArgSchemaTextType.Trajectory = 1` 与常量 `ArgSchemaKey_Trajectory = "trajectory"`，但 `ContentType` 枚举尚无 Trajectory 值
- `evaluation/domain/eval_set.thrift` 中 `FieldSchema` 已携带 `content_type / schema_key / text_schema` 三件套，足以承载 Trajectory 列；`BizCategory.FromOnlineTrace = "from_online_trace"` 已存在
- `evaluation/domain/evaluator.thrift` 中 `Evaluator.builtin / box_type=Black / EvaluatorInfo` 已为内置黑盒评估器准备好骨架；`DebugBuiltinEvaluator` RPC 已经存在
- `evaluation/domain/expt.thrift` 中 `ColumnEvalTargetName_Trajectory = "trajectory"` 常量 + `TrajectoryAnalysisResult` 结构 + `ExperimentTurnPayload.trajectory_analysis_result` 字段均已存在，说明 Trajectory 在实验报告层是有先验占位的
- `coze.loop.evaluation.expt.thrift` 中智能解读 RPC 体系完备但只有 Expt 级（`InsightAnalysisExperiment`），没有 Row 级 RPC

PRD 显式声明的两项能力涉及评测域之外：

1. 单字段容量 100KB → 1MB、单行 1MB → 5MB —— 落在 `data/domain/dataset.thrift` `DatasetSpec`
2. 单列导入到已有行（不覆盖其他列）—— 原 PRD 描述依赖数据模块的部分列更新原子能力

**2026-06-22 范围收窄**：经用户澄清，data + observability 服务实现不在本 change 范围。具体处置：
- 容量上调（1)：**OUT-OF-SCOPE**，须由 data 服务 owner 独立排期；evaluation 侧在容量未上调前用户层裁剪 / oversize 旁路降级。
- 单列导入（2)：evaluation 自实现 `BatchUpsertEvaluationSetItemColumns`，不依赖新增 data RPC。
- trace_id 解析（MA-3）：**消费 observability 现有 `ListTrajectory` RPC 契约 as-is**，不要求 observability 侧新增 warning 通道或改动语义。
- 共享 IDL 类型 `SourceType` + `DatasetIOTrace` 保留（evaluation 依赖），data IDL 文件结构不动。

## Goals / Non-Goals

**Goals:**

- 让评测集成为 Trajectory 的一等公民：字段类型、JSON Schema 校验、默认列、导入路径（trace_id / 文件）、单列更新
- 让评估器具备开箱即用的轨迹质量度量：tool / planning / context_memory 三个内置黑盒评估器
- 让实验配置/运行/报告对 Trajectory 透明：字段映射校验、时间轴可视化、Row 级智能解读
- 在 IDL 上做最小破坏性扩展：能复用 `content_type / schema_key / text_schema` 与 `EvaluatorInputData / TrajectoryAnalysisResult` 等现有结构就不引入新 struct

**Non-Goals:**

- Phase 2 能力（第三方 RPC/ByteFaas 评估器；连续值/类别/自由文本输出；报告→评测集导入）—— 仅在 proposal What Changes 中声明，不写 spec
- 不修改 trace 平台、不直接读 trace 存储 —— 通过 observability 模块**现有** `ListTrajectory` RPC 拉解析（as-is）
- 不动 `EvaluatorResult.score: double` —— 保持兼容；Phase 2 再扩展输出形态
- **不在本 change 中交付 data 服务容量上调（1MB / 5MB）**：由 data 服务 owner 独立协调
- **不在本 change 中改动 observability 服务实现**：仅消费现有 `ListTrajectory` 契约
- 共享 IDL 类型 `SourceType`（data 域）+ `DatasetIOTrace`（data 域）保留并被 evaluation 引用，但 data IDL / 服务实现不在本 change 改动

## Decisions

### D1：Trajectory 通过 ContentType 扩展承载

**决策**：在 `common.thrift` 的 `ContentType` 枚举新增 `Trajectory` 值，并约定 `schema_key = ArgSchemaKey_Trajectory ("trajectory")`、`text_schema` 存 JSON Schema 字符串。

**理由**：现有 `ContentType` 是字段类型的主开关；`FieldSchema.content_type` 已被字段映射 / 实验运行链路广泛使用。如果只用 `schema_key` 不引入 ContentType，下游所有按 ContentType 分支的代码（评估器入参装配、报告渲染、字段映射校验）都要二次判断 schema_key，破坏抽象。

**备选**：仅复用 `ArgSchemaTextType.Trajectory + schema_key`，不引入 ContentType。已否决 —— 见上。

### D2：Trajectory JSON Schema 在 spec 中冻结骨架，实现期可继续细化

**决策**：spec 中固化 steps[] 的核心字段集合（id / role / type / content / tool_calls / timestamp），其余 metadata / token_usage 作为 optional 扩展点。PRD 中 Trajectory 数据结构标注为"TBD"，故对 schema 字段集留扩展位。

**理由**：保护对外契约稳定，又为产品后续微调提供窗口。Schema 校验函数注册为可热更换的 validator（具体落在 domain/service 层）。

### D3：内置评估器借用 commercial 域实现

**决策**：tool / planning / context_memory 三个内置评估器的 prompt / code 实现挂在 `cozeloop-commercial`，open-core 只声明评估器元数据（name / box_type / tags / builtin=true / info）。

**理由**：内场上线，需要快速迭代且涉及内部 LLM 接入；commercial 已有 Wire DI 与字节内部基础设施挂载。后续需开源时再迁移到 open-core。

### D4：导入路径复用 `CreateEvaluationSetWithImport` + `ParseImportSourceFile`

**决策**：新增 trace_id 来源由 `source_type` 扩展承载（不新增 RPC）；trace → trajectory 的转换函数注册为 import adapter。文件导入沿用现有 JSON/CSV 通路，新增 Trajectory schema 推断逻辑。

**理由**：保持 API surface 稳定，减少前端改动。

### D5：单列导入引入新 RPC `BatchUpsertEvaluationSetItemColumns`（evaluation 自实现）

**决策**：在 evaluation 服务中新增 RPC，参数 `[{item_id | match_key, field_data[]}]`；语义上是"部分列 upsert"。**evaluation 自身实现部分列更新逻辑，不调用 data 新增 RPC**（原计划的 `data.BatchPatchDatasetItems` 不在本 change 范围）。

**理由**：现有 `BatchUpdateEvaluationSetItems` 是整行替换，硬塞会破坏老调用者。`BatchUpsert` 后缀语义清晰，且与 import_mode 审计字段呼应。由于 data 服务不参与本 change，evaluation 内部用"先 Get 整行 → 合并 patch 列 → Put 整行"路径实现（已知风险：并发写覆盖，在 evaluation 层加乐观锁缓解）。

### D6：Row 级智能解读新增 RPC，复用 InsightAnalysisRecord 数据结构

**决策**：新增 `InsightAnalysisExperimentRow` RPC，参数 `expt_id + item_id + turn_id`，返回 `insight_analysis_record_id`；`ExptInsightAnalysisRecord` 增加 `scope` 字段标识 Experiment 级 vs Row 级。

**理由**：列表 / 详情 / Feedback / Comment 等附属 RPC 全部沿用，无需再引入一套并行结构。`scope` 字段是低破坏性扩展。

### D7：1MB / 5MB 容量上调 — **OUT-OF-SCOPE，延后到 data 服务独立排期**

**决策（2026-06-22 修订）**：本 change **不交付** 1MB / 5MB 容量上调。data 服务实现不在本 change 范围；evaluation 在容量未上调前依赖 data 现有上限（100KB 字段 / 1MB 行）。

**降级方案**：
- 大 Trajectory 字段触达上限时，evaluation 层返回 `FIELD_SIZE_EXCEEDED` 引导用户裁剪
- MQ 投递大 Trajectory 走 oversize-payload 旁路（外存指针，见 `wiki/dev-guides/middleware/cozeloop-mq-guide.md`）

**后续**：由 data 服务 owner 独立排期容量上调；完成后 evaluation 透传新上限值即可（IDL 默认值在 data 域调整，evaluation 无需改动）。

## Risks / Trade-offs

| 风险 | 影响 | 缓解 |
|------|------|------|
| Trajectory schema 长期演进与版本兼容 | 老评测集字段值在新 schema 下校验失败 | schema 校验按字段 `text_schema` 版本号路由，老数据沿用旧 validator；lazy 迁移 |
| 单字段 1MB 致接口序列化压力 | 报告侧 P99 延迟上升 | 报告默认返回 summary + lazy field 拉取（`GetEvaluationSetItemField`） |
| 内置评估器质量难以横向公平 | 用户产生"内置 < 自写"印象 | EvaluatorInfo 中明示 benchmark 与适用场景；UI 提示 debug 入口 |
| data 容量未上调致大 Trajectory 触达 1MB 行上限 | 用户层导入失败 | 用户层裁剪 + MQ oversize 旁路；提示用户等待 data 服务上调（OUT-OF-SCOPE） |
| Phase 2 输出形态扩充对 EvaluatorResult 的破坏 | 现有报告依赖 score:double | 提前在 design 中冻结 D7 决策：Phase 1 不动 score；Phase 2 通过新增字段 + 兼容层处理 |
| 老前端不识别 ContentType.Trajectory | 字段渲染异常 | 前端版本灰度 + ContentType unknown 时降级为 raw JSON 文本 |

---

## Scope Narrowing（2026-06-22）

经用户澄清，本 change 范围收窄为 **evaluation 单模块**。data + observability 服务实现不在本 change 内交付。

### 范围处置

| 原跨模块项 | 当前处置 |
|------------|----------|
| MA-1 容量上调（1MB 字段 / 5MB 行） | **OUT-OF-SCOPE**：由 data 服务 owner 独立排期；evaluation 在容量未上调前用户层裁剪 / oversize 旁路 |
| MA-2 单列导入 | evaluation 内自实现 `BatchUpsertEvaluationSetItemColumns`（先 Get 整行→合并→Put 整行 + 乐观锁），不依赖新增 data RPC |
| MA-3 trace_id 解析 | 消费 observability **现有** `ListTrajectory` RPC（as-is），不要求 observability 侧新增 warning 通道 / 改动鉴权 / 改动限流语义 |
| 共享 IDL `SourceType` + `DatasetIOTrace` | 保留，被 evaluation 引用（无 IDL 改动） |

### 跨服务调用链（仅消费方视角）

```
[evaluation.eval_set]
   └─ 按 trace_id 导入 (ParseImportSourceFile 扩展 source_type=Trace)
        └─ RPC: observability.ListTrajectory(trace_ids)   ← 消费现有契约，as-is

[evaluation.eval_set]
   └─ 业务接口 BatchUpsertEvaluationSetItemColumns        ← evaluation 自实现
        └─ 内部走 data 现有 `UpdateDatasetItem` 整行替换 + 乐观锁

[evaluation.evaluator / experiment]
   └─ Trajectory 字段透传到 EvaluatorInputData / 报告
```

### 关键事实校正（基于 IDL 扫描）

- **`coze-loop/idl/thrift/coze/loop/trajectory.thrift` 已存在**，定义 `Trajectory / RootStep / AgentStep / Step / ModelInfo / BasicInfo / MetricsInfo / Error`，已被 observability 引用。这是 Trajectory JSON Schema 的事实基础，evaluation 复用此结构而不另起。
- **`coze.loop.observability.trace.thrift::TraceService::ListTrajectory`** 已存在，签名 `(workspace_id, trace_ids[1..10], platform_type, start_time)` → `list<trajectory.Trajectory>`。evaluation **消费此现有契约 as-is**。
- **`UpsertTrajectoryConfig / GetTrajectoryConfig`** 已存在；如评测端需配置 trace → trajectory 解析的 filter，可复用现有接口。

### Trajectory JSON Schema 最小可工作定义（evaluation validator 用）

基于 `coze/loop/trajectory.thrift` 中的 Trajectory 结构以及 PRD 标注的 TBD 字段集，Phase 1 冻结以下骨架：

```jsonc
{
  "$schema": "https://json-schema.org/draft-07/schema#",
  "type": "object",
  "required": ["id"],
  "properties": {
    "id": { "type": "string" },                                 // trace_id
    "root_step": {
      "type": "object",
      "properties": {
        "id": { "type": "string" },                              // span_id
        "name": { "type": "string" },
        "input": { "type": "string" },
        "output": { "type": "string" },
        "metadata": { "type": "object", "additionalProperties": { "type": "string" } },
        "basic_info": { "$ref": "#/definitions/BasicInfo" },
        "metrics_info": { "$ref": "#/definitions/MetricsInfo" }
      }
    },
    "agent_steps": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "id": { "type": "string" },
          "parent_id": { "type": "string" },
          "name": { "type": "string" },
          "input": { "type": "string" },
          "output": { "type": "string" },
          "steps": {
            "type": "array",
            "items": { "$ref": "#/definitions/Step" }
          },
          "metadata": { "type": "object" },
          "basic_info": { "$ref": "#/definitions/BasicInfo" },
          "metrics_info": { "$ref": "#/definitions/MetricsInfo" }
        }
      }
    }
  },
  "definitions": {
    "Step": {
      "type": "object",
      "properties": {
        "id": { "type": "string" },
        "parent_id": { "type": "string" },
        "type": { "enum": ["agent", "model", "tool"] },
        "name": { "type": "string" },
        "input": { "type": "string" },
        "output": { "type": "string" },
        "model_info": { "$ref": "#/definitions/ModelInfo" },
        "basic_info": { "$ref": "#/definitions/BasicInfo" }
      }
    },
    "BasicInfo": {
      "type": "object",
      "properties": {
        "started_at": { "type": "string" },
        "duration": { "type": "string" },
        "error": { "type": "object", "properties": { "code": { "type": "integer" }, "msg": { "type": "string" } } }
      }
    },
    "ModelInfo": { "type": "object" },
    "MetricsInfo": { "type": "object" }
  }
}
```

> **演进策略**：text_schema 内嵌 `version` 字段；evaluation 侧 validator 按 schema version 路由。PRD 冻结后增量。

## Open Questions（待 PM / 平台方确认，不阻塞实施）

| # | 议题 | 默认假设 | 影响 | 触发动作 |
|---|------|----------|------|----------|
| OQ-1 (来自 MA-5) | 三个内置评估器（tool / planning / context_memory）是否复用现有 evaluator-judge 的 LLM 通道与计费？是否需要新 channel / 预算？ | **复用现有 evaluator-judge LLM 通道与计费，不另开预算** | 中：影响成本预估与上线灰度策略 | 待 PM 确认；若不通过，需在 commercial 内接入新 LLM channel 并补预算 |
| OQ-2 (来自 MA-6) | 行级 Insight SLA 形态：行级走异步（提交→轮询/事件通知），全报告 Insight 同步限时返回？ | **是**。`InsightAnalysisExperimentRow` 返回 `record_id` + `Pending` 状态，前端轮询 `GetExptInsightAnalysisRecord`；全报告 `InsightAnalysisExperiment` 维持现有同步语义 | 中：影响实验报告 Trajectory tab 的 UX 与限流策略 | 待 PM 确认；若需同步限时，需评估行级解读 P95 在 LLM 侧是否可压到 < 10s |
| OQ-3 (Trajectory JSON Schema) | PRD 中 Trajectory 数据结构标注 TBD。Phase 1 已基于 `coze/loop/trajectory.thrift` 冻结最小可工作定义；后续 PRD 冻结后字段是否需要回填？ | 通过 `text_schema.version` 路由的方式演进，不阻塞 Phase 1 | 低 | PRD 冻结后增量改 validator，老数据沿用旧 schema 版本 |
| OQ-5 (容量 5MB 对 MQ 影响) | 评测运行触发 MQ 投递 Trajectory 时是否会触达 Broker 限额 | 用 oversize-payload 旁路；容量上调本身 OUT-OF-SCOPE，跟踪到 data 服务排期 | 中 | 与 MQ owner 对齐；评估 RocketMQ Broker 配置 |

> OQ-4（原 data RPC 命名议题）已删除：data 服务不在本 change 范围。

