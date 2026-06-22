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
2. 单列导入到已有行（不覆盖其他列）—— 依赖数据模块的部分列更新原子能力（负责人：骆弘珊）

故本 design 在评测域内做"扩展 + 校验 + 透传"，跨域能力通过 `impact-analysis.md` 的 must_ask 显式上抛。

## Goals / Non-Goals

**Goals:**

- 让评测集成为 Trajectory 的一等公民：字段类型、JSON Schema 校验、默认列、导入路径（trace_id / 文件）、单列更新
- 让评估器具备开箱即用的轨迹质量度量：tool / planning / context_memory 三个内置黑盒评估器
- 让实验配置/运行/报告对 Trajectory 透明：字段映射校验、时间轴可视化、Row 级智能解读
- 在 IDL 上做最小破坏性扩展：能复用 `content_type / schema_key / text_schema` 与 `EvaluatorInputData / TrajectoryAnalysisResult` 等现有结构就不引入新 struct

**Non-Goals:**

- Phase 2 能力（第三方 RPC/ByteFaas 评估器；连续值/类别/自由文本输出；报告→评测集导入）—— 仅在 proposal What Changes 中声明，不写 spec
- 不修改 trace 平台、不直接读 trace 存储 —— 通过 observability 模块的接口拉解析
- 不动 `EvaluatorResult.score: double` —— 保持兼容；Phase 2 再扩展输出形态
- 不引入新的数据集存储引擎；容量上调由数据团队评估

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

### D5：单列导入引入新 RPC `BatchUpsertEvaluationSetItemColumns`

**决策**：新增 RPC，参数 `[{item_id | match_key, field_data[]}]`；语义上是"部分列 upsert"。

**理由**：现有 `BatchUpdateEvaluationSetItems` 是整行替换，硬塞会破坏老调用者。`BatchUpsert` 后缀语义清晰，且与 import_mode 审计字段呼应。

### D6：Row 级智能解读新增 RPC，复用 InsightAnalysisRecord 数据结构

**决策**：新增 `InsightAnalysisExperimentRow` RPC，参数 `expt_id + item_id + turn_id`，返回 `insight_analysis_record_id`；`ExptInsightAnalysisRecord` 增加 `scope` 字段标识 Experiment 级 vs Row 级。

**理由**：列表 / 详情 / Feedback / Comment 等附属 RPC 全部沿用，无需再引入一套并行结构。`scope` 字段是低破坏性扩展。

### D7：1MB / 5MB 容量上调，跨域协调

**决策**：本 spec 仅声明上限目标值，实际生效依赖数据模块基础能力 + 存储侧扩容评估。`impact-analysis.md` 列为 must_ask 必须问。

**理由**：避免评测域单方面写死数字而存储侧未跟进，造成线上故障。

## Risks / Trade-offs

| 风险 | 影响 | 缓解 |
|------|------|------|
| Trajectory schema 长期演进与版本兼容 | 老评测集字段值在新 schema 下校验失败 | schema 校验按字段 `text_schema` 版本号路由，老数据沿用旧 validator；lazy 迁移 |
| 单字段 1MB 致接口序列化压力 | 报告侧 P99 延迟上升 | 报告默认返回 summary + lazy field 拉取（`GetEvaluationSetItemField`） |
| 内置评估器质量难以横向公平 | 用户产生"内置 < 自写"印象 | EvaluatorInfo 中明示 benchmark 与适用场景；UI 提示 debug 入口 |
| 跨域依赖（数据模块容量 / observability trace 解析）阻塞 GA | 季度交付风险 | 立项时 must_ask 上抛；本 change 先交付评测域内能力，跨域字段降级（capacity 临时维持现状 + 缺少 trace 入口的兼容） |
| Phase 2 输出形态扩充对 EvaluatorResult 的破坏 | 现有报告依赖 score:double | 提前在 design 中冻结 D7 决策：Phase 1 不动 score；Phase 2 通过新增字段 + 兼容层处理 |
| 老前端不识别 ContentType.Trajectory | 字段渲染异常 | 前端版本灰度 + ContentType unknown 时降级为 raw JSON 文本 |

---

## Cross-Module Scope Update（2026-06-22）

### 背景

上一轮 must_ask 答复后，本 change 后端范围由 **evaluation 单模块** 扩展为 **evaluation + data + observability 三模块**：

- **MA-1 / MA-2 落地 data 模块**：单字段 100KB→1MB、单行 1MB→5MB 容量上调；单列导入到已有行（部分列更新原子能力）。
- **MA-3 落地 observability 模块**：trace_id → Trajectory 解析对外契约。

为保留每个模块的边界清晰、并支持各自独立 archive，按模块拆分 capability spec。

### 三模块职责分工

| 模块 | 角色 | 本 change 增量 |
|------|------|----------------|
| **evaluation** | 业务编排：评测集 / 评估器 / 实验三元组 + Trajectory 业务语义 | Trajectory ContentType、JSON Schema validator、`BatchUpsertEvaluationSetItemColumns`（业务接口）、内置评估器、`InsightAnalysisExperimentRow`、字段映射校验 |
| **data** | Dataset 底座：存储、容量、schema、原子 IO | 单字段 / 单行容量上调 + 部分列更新原子能力 + IDL 暴露 `BatchPatchDatasetItems` |
| **observability** | Trace 平台：span 摄取、检索、解析 | 把已有 `ListTrajectory(trace_ids → Trajectory)` 固化为跨域契约，补 warning / 鉴权 / 限流语义；保留 `UpsertTrajectoryConfig` 用作过滤规则配置 |

### 跨模块调用链

```
[evaluation.eval_set]
   └─ 按 trace_id 导入 (ParseImportSourceFile 扩展 source_type=Trace)
        └─ port: TraceTrajectoryParserPort
             └─ infra adapter → RPC: observability.ListTrajectory(trace_ids)

[evaluation.eval_set]
   └─ 业务接口 BatchUpsertEvaluationSetItemColumns
        └─ port: DatasetItemColumnPatchPort
             └─ infra adapter → RPC: data.BatchPatchDatasetItems

[evaluation.evaluator / experiment]
   └─ Trajectory 字段透传到 EvaluatorInputData / 报告
        └─（不跨模块；evaluation 内部完成）
```

> Port/Adapter：所有跨模块调用 SHALL 经过 evaluation Domain 的 port，由 infra 层选择 in-process（同进程不同模块）或 RPC（跨服务）。这保留了后续物理拆分的灵活性。

### 关键事实校正（基于 IDL 扫描）

- **`coze-loop/idl/thrift/coze/loop/trajectory.thrift` 已存在**，定义 `Trajectory / RootStep / AgentStep / Step / ModelInfo / BasicInfo / MetricsInfo / Error`，已被 observability 引用。这是 MA-4 Trajectory JSON Schema 的事实基础，**无需在评测域另起一套结构**。
- **`coze.loop.observability.trace.thrift::TraceService::ListTrajectory`** 已存在，签名 `(workspace_id, trace_ids[1..10], platform_type, start_time)` → `list<trajectory.Trajectory>`。MA-3 不需要新增 RPC，转为"契约固化 + 补缺失语义"。
- **`UpsertTrajectoryConfig / GetTrajectoryConfig`** 已存在，用于配置 trace → trajectory 解析的 filter 规则（按 FilterFields），可被 evaluation 端复用。

### IDL 归属决策

| 接口 | 归属 IDL 文件 | 理由 |
|------|--------------|------|
| 单字段 / 单行容量上调 | `coze/loop/data/domain/dataset.thrift` (`DatasetSpec` 默认值) | 容量是 Dataset 底座属性，evaluation 通过 spec 透传 |
| `BatchPatchDatasetItems` (底座原子) | `coze/loop/data/coze.loop.data.dataset.thrift` | 数据集底座 RPC |
| `BatchUpsertEvaluationSetItemColumns` (业务) | `coze/loop/evaluation/coze.loop.evaluation.eval_set.thrift` | 业务侧封装，调用 data 底座 |
| `ListTrajectory` (现存) | `coze/loop/observability/coze.loop.observability.trace.thrift` | 已存在，不动 IDL |
| `Trajectory` 数据结构 | `coze/loop/trajectory.thrift` (共享) | 已存在；evaluation `EvaluatorInputData` 在使用时通过 ContentType + text_schema 引用 |

### Trajectory JSON Schema 最小可工作定义（MA-4）

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

### Decisions 追加

#### D8：跨模块边界采用 Port/Adapter

evaluation Domain 引入 `TraceTrajectoryParserPort`、`DatasetItemColumnPatchPort` 两个 port；adapter 由 infra 层在编译期选择 in-process 调用 vs RPC 调用。保留物理拆分弹性，避免 evaluation 直接 import data/observability 内部包。

#### D9：容量上调与 MQ / HTTP 网关耦合

容量上调的"目标值生效"以下游中间件能够承载为前置条件。Phase 1 内 evaluation 侧使用过限走 oversize-payload 旁路（外存 + 引用）而非阻断业务请求。MQ 端见 `wiki/dev-guides/middleware/cozeloop-mq-guide.md`。

#### D10：observability `ListTrajectory` 不动 IDL，做契约固化

`ListTrajectory` 已存在；为本 change 增量定义"跨域契约"语义：
- 单 trace_id 失败不影响整体（warning 通道）
- workspace 鉴权强制
- 按 workspace 限流
- 字段稳定性承诺（不删 / 增字段 optional）

这些语义通过 spec + observability 仓 release-notes 落地，不改 IDL 结构。

#### D11：单列导入接口分层

- data 模块新增 `BatchPatchDatasetItems`（底座 RPC，按 item_id 部分列原子更新）
- evaluation 模块沿用 `BatchUpsertEvaluationSetItemColumns`（业务接口，包含评测集语义校验 + import audit）
- 评测域接口转发到底座，**不**直接走 `UpdateDatasetItem`（避免整行替换语义）

## Open Questions（待 PM / 平台方确认，不阻塞实施）

| # | 议题 | 默认假设 | 影响 | 触发动作 |
|---|------|----------|------|----------|
| OQ-1 (来自 MA-5) | 三个内置评估器（tool / planning / context_memory）是否复用现有 evaluator-judge 的 LLM 通道与计费？是否需要新 channel / 预算？ | **复用现有 evaluator-judge LLM 通道与计费，不另开预算** | 中：影响成本预估与上线灰度策略 | 待 PM 确认；若不通过，需在 commercial 内接入新 LLM channel 并补预算 |
| OQ-2 (来自 MA-6) | 行级 Insight SLA 形态：行级走异步（提交→轮询/事件通知），全报告 Insight 同步限时返回？ | **是**。`InsightAnalysisExperimentRow` 返回 `record_id` + `Pending` 状态，前端轮询 `GetExptInsightAnalysisRecord`；全报告 `InsightAnalysisExperiment` 维持现有同步语义 | 中：影响实验报告 Trajectory tab 的 UX 与限流策略 | 待 PM 确认；若需同步限时，需评估行级解读 P95 在 LLM 侧是否可压到 < 10s |
| OQ-3 (Trajectory JSON Schema) | PRD 中 Trajectory 数据结构标注 TBD。Phase 1 已基于 `coze/loop/trajectory.thrift` 冻结最小可工作定义；后续 PRD 冻结后字段是否需要回填？ | 通过 `text_schema.version` 路由的方式演进，不阻塞 Phase 1 | 低 | PRD 冻结后增量改 validator，老数据沿用旧 schema 版本 |
| OQ-4 (data 底座接口名) | `BatchPatchDatasetItems` 命名 / 是否在 `UpdateDatasetItem` 上加 `partial` 旗标 | 新增 RPC，命名 `BatchPatchDatasetItems`（详见 D11） | 低 | 与 data 模块 owner 对齐，最终以 IDL PR 评审为准 |
| OQ-5 (容量 5MB 对 MQ 影响) | 评测运行触发 MQ 投递 Trajectory 时是否会触达 Broker 限额 | 用 oversize-payload 旁路 | 中 | 与 MQ owner 对齐；评估 RocketMQ Broker 配置 |

