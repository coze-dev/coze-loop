# 异步评估器 OpenAPI 调用 — 设计文档

- 日期：2026-06-29
- 分支：feat/asyncevaluatorcall
- 范围：仅后端（evaluation 模块）。不涉及 DB / Clickhouse / 配置变更。

## 1. 背景与问题

当前 `EvaluationOpenAPIService` 仅提供 **同步** 评估器调用 `RunEvaluatorOApi`
（`POST /v1/loop/evaluation/evaluators_versions/:evaluator_version_id/run`，IDL 第 1235 行）。
调用方必须同步等待评估器执行完成。

异步评估器能力在内部已经存在，但**尚未通过 OpenAPI 暴露提交入口**：

- 领域服务 `EvaluatorService.AsyncRunEvaluator`（`evaluator_impl.go:914`）：生成 `invokeID`，
  分发到 `evaluatorSourceService.AsyncRun`，并以 `Status = EvaluatorRunStatusAsyncInvoking` 落库一条
  `EvaluatorRecord`（record ID = invokeID），随即返回。
- 内部 RPC 应用 `EvaluatorHandlerImpl.AsyncRunEvaluator`（`evaluator_app.go:2091`）：鉴权 → 调用领域服务 →
  通过 `evalAsyncRepo.SetEvalAsyncCtx(ctx, "evaluator:{recordID}", &EvalAsyncCtx{...})` 写入异步上下文 →
  返回 `InvokeID`。
- 结果回调 OpenAPI 端点 `ReportEvaluatorInvokeResult`
  （`POST /v1/loop/evaluation/evaluators/result`，`eval_openapi_app.go:2484`）**已存在**：
  读取 `EvalAsyncCtx`，调用 `EvaluatorService.ReportEvaluatorInvokeResult` 更新记录结果，
  并在 `actx.Event != nil` 时发布实验事件以恢复等待中的实验。

因此本设计只需新增一个**薄的 OpenAPI 提交端点**，复用上述全部既有基础设施。

## 2. 目标与非目标

### 目标
- 新增 OpenAPI 端点，允许外部调用方**异步提交**单个评估器版本的执行，立即返回 `invoke_id`。
- 复用既有异步领域服务、异步上下文存储（Redis）、记录状态机与既有回调端点。
- IDL 改动保持向前兼容（纯新增）。

### 非目标
- 不修改 `AsyncRunEvaluator` 的评估器类型限制：保持 **仅支持 Agent 类型**
  （`evaluator_impl.go:923`）。prompt / code 类型沿用既有 “does not support async run” 错误。
- 不新增结果检索端点：结果通过既有 `BatchGetEvaluatorRecordsOApi` 轮询获取。
- 不改动 `ReportEvaluatorInvokeResult` 回调端点。
- 无 DB / Clickhouse / 配置变更，故 release 下 docker/k8s 的表结构与配置均不涉及。

## 3. 设计决策（已与用户确认）

| 决策点 | 结论 |
| --- | --- |
| 评估器类型限制 | 保持 Agent-only，限制由领域层继承 |
| 端点形态 | 镜像同步端点；URI 为 `.../async_run` |
| 响应内容 | 同时返回 `invoke_id` 与 `AsyncInvoking` 记录 |
| 结果检索 | 调用方轮询既有 `BatchGetEvaluatorRecordsOApi`；并在文档中澄清 `ReportEvaluatorInvokeResult` 是运行时→服务端的入站回调，而非面向调用方的检索接口 |

## 4. 详细设计

### 4.1 IDL — `idl/thrift/coze/loop/evaluation/coze.loop.evaluation.openapi.thrift`

在同步结构（第 887 行后）新增请求/响应结构，字段与 `RunEvaluatorOApiRequest` 完全镜像：

```thrift
// 3.10.2 异步执行评估器
struct AsyncRunEvaluatorOApiRequest {
    1: optional i64 evaluator_version_id (api.path = "evaluator_version_id", api.js_conv = "true", go.tag = 'json:"evaluator_version_id"')
    2: optional i64 workspace_id (api.body = "workspace_id", api.js_conv = "true", go.tag = 'json:"workspace_id"')
    3: optional evaluator.EvaluatorInputData input_data (api.body = "input_data")
    4: optional evaluator.EvaluatorRunConfig evaluator_run_conf (api.body = "evaluator_run_conf")

    100: optional map<string, string> ext (api.body = "ext")

    254: optional extra.Extra extra (agw.source = "not_body_struct")
    255: optional base.Base Base
}

struct AsyncRunEvaluatorOApiResponse {
    1: optional i32 code
    2: optional string msg
    3: optional AsyncRunEvaluatorOpenAPIData data

    255: base.BaseResp BaseResp
}

struct AsyncRunEvaluatorOpenAPIData {
    1: optional i64 invoke_id (api.body = "invoke_id", api.js_conv = "true", go.tag = 'json:"invoke_id"')
    2: optional evaluator.EvaluatorRecord record (api.body = "record") // status = AsyncInvoking
}
```

在 `EvaluationOpenAPIService`（同步方法第 1235 行后）新增方法：

```thrift
// 异步执行评估器
AsyncRunEvaluatorOApiResponse AsyncRunEvaluatorOApi(1: AsyncRunEvaluatorOApiRequest req)
    (api.category = "openapi", api.post = "/v1/loop/evaluation/evaluators_versions/:evaluator_version_id/async_run")
```

向前兼容性：纯新增结构与方法，不修改任何既有字段编号或方法签名。

### 4.2 代码生成

使用项目既有 codegen 工具链，从 IDL 重新生成：

- `backend/kitex_gen/coze/loop/evaluation/openapi`（请求/响应 DTO 与 Args/Result）
- `backend/loop_gen/coze/loop/evaluation/loopenapi/local_evaluationopenapiservice.go`
  （新增 `AsyncRunEvaluatorOApi` 本地服务转发方法，委托 `l.impl.AsyncRunEvaluatorOApi`）
- `backend/api/handler/coze/loop/apis/eval_open_apiservice.go`
  （新增 `AsyncRunEvaluatorOApi` Hertz handler，`invokeAndRender` 包装）
- `backend/api/router/coze/loop/apis/coze.loop.apis.go`
  （在 `_evaluator_version_id0` 路由组下新增 `POST /async_run` 与对应中间件条目）

### 4.3 应用层 — `backend/modules/evaluation/application/eval_openapi_app.go`

新增方法 `func (e *EvalOpenAPIApplication) AsyncRunEvaluatorOApi(ctx, req *openapi.AsyncRunEvaluatorOApiRequest) (*openapi.AsyncRunEvaluatorOApiResponse, error)`，
逻辑镜像 `RunEvaluatorOApi`（`eval_openapi_app.go:1911`），仅在执行步骤改走异步路径：

1. `req == nil` 校验，返回 `CommonInvalidParamCode`。
2. `defer e.metric.EmitOpenAPIMetric(...)` 上报指标（与同步端点一致）。
3. 加载评估器版本：`e.evaluatorService.GetEvaluatorVersion(ctx, nil, versionID, false, false)`；
   为空返回 `ResourceNotFoundCode`。
4. 鉴权：
   - builtin：允许跨 workspace，不额外鉴权（与同步一致）。
   - 非 builtin：校验 `SpaceID == WorkspaceID`，再 `AuthorizationWithoutSPI`（Action=Read，
     EntityType=Evaluator，带 OwnerID / ResourceSpaceID），与同步端点完全一致。
5. DTO→DO 转换：`OpenAPIEvaluatorInputDataDTO2DO` / `OpenAPIEvaluatorRunConfigDTO2DO`；
   若 runConf 含 `EvaluatorRuntimeParam.JSONValue`，注入
   `inputData.Ext[consts.FieldAdapterBuiltinFieldNameRuntimeParam]`（与同步一致）。
6. 记录提交起始时间 `startTime := time.Now()`（用于异步耗时计算）。
7. 调用异步领域服务：
   ```go
   resp, err := e.evaluatorService.AsyncRunEvaluator(ctx, &entity.AsyncRunEvaluatorRequest{
       SpaceID:            req.GetWorkspaceID(),
       Name:               evaluator.Name,
       EvaluatorVersionID: req.GetEvaluatorVersionID(),
       InputData:          inputData,
       EvaluatorRunConf:   runConf,
       Ext:                req.Ext,
       // ExperimentID / ExperimentRunID / ItemID / TurnID 留零值——独立调用
   })
   ```
   返回的 `resp.ID` 即 `invokeID`（= record ID）。非 Agent 类型在此返回既有限制错误。
8. 写入异步上下文（`Event = nil`，因独立调用无实验需恢复）：
   ```go
   if err := e.asyncRepo.SetEvalAsyncCtx(ctx, fmt.Sprintf("evaluator:%d", resp.ID), &entity.EvalAsyncCtx{
       RecordID:           resp.ID,
       AsyncUnixMS:        startTime.UnixMilli(),
       Session:            &entity.Session{UserID: session.UserIDInCtxOrEmpty(ctx)},
       EvaluatorVersionID: req.GetEvaluatorVersionID(),
   }); err != nil { return nil, err }
   ```
9. 读取 `AsyncInvoking` 记录并返回：使用既有
   `evaluator_convertor.OpenAPIEvaluatorRecordDO2DTO`。`AsyncRunEvaluator` 已返回该
   `*entity.EvaluatorRecord`，直接转换即可，无需二次查询。返回
   `invoke_id = resp.ID` 与该记录。

> 说明：`EvalOpenAPIApplication` 已持有 `asyncRepo`（`eval_openapi_app.go:50`）与
> `evaluatorService`，构造函数无需改动。

### 4.4 结果检索（仅文档，无新增代码）

- 调用方使用既有 `BatchGetEvaluatorRecordsOApi`（`POST .../evaluator_records/batch_get`），
  以 `invoke_id`（= record_id）轮询，直至记录状态离开 `AsyncInvoking`（变为 `Success` / `Fail`）。
- 既有 `ReportEvaluatorInvokeResult`（`POST .../evaluators/result`）是
  **Agent 运行时 → 服务端** 的入站结果回调端点，由运行时在执行完成时调用，
  **不面向 OpenAPI 调用方**。本设计不改动它。

### 4.5 调用流程时序

```
调用方 → AsyncRunEvaluatorOApi
            ├─ 鉴权 / 转换
            ├─ AsyncRunEvaluator → 落库 record(status=AsyncInvoking) → 分发 Agent 运行时
            ├─ SetEvalAsyncCtx("evaluator:{id}", Event=nil)
            └─ 返回 {invoke_id, record(AsyncInvoking)}

Agent 运行时执行完成 → ReportEvaluatorInvokeResult（既有端点）
            ├─ GetEvalAsyncCtx
            ├─ ReportEvaluatorInvokeResult → UpdateEvaluatorRecordResult(status=Success/Fail)
            └─ actx.Event == nil → 不发布实验事件

调用方轮询 BatchGetEvaluatorRecordsOApi(invoke_id) → 获取最终结果
```

## 5. 错误处理

| 场景 | 行为 |
| --- | --- |
| `req == nil` | `CommonInvalidParamCode` |
| 评估器版本不存在 | `ResourceNotFoundCode` |
| 非 builtin 且 workspace 不匹配 | `ResourceNotFoundCode`（避免越权探测） |
| 鉴权失败 | `AuthorizationWithoutSPI` 返回的错误 |
| 非 Agent 类型 | 领域层 `InvalidEvaluatorTypeCode`（"does not support async run"）透传 |
| `SetEvalAsyncCtx` 失败 | 记录日志并返回错误（与内部 `AsyncRunEvaluator` 行为一致） |

## 6. 测试计划（TDD）

为 `AsyncRunEvaluatorOApi` 在 `eval_openapi_app` 测试文件新增单测，mock
`evaluatorService` / `asyncRepo` / `auth` / `metric`：

1. 成功路径：返回 `invoke_id` 与 `AsyncInvoking` 记录，且 `SetEvalAsyncCtx` 被以
   `evaluator:{id}`、`Event=nil` 调用。
2. `req == nil` → `CommonInvalidParamCode`。
3. 版本不存在 → `ResourceNotFoundCode`。
4. 非 builtin 且 workspace 不匹配 → `ResourceNotFoundCode`。
5. builtin 路径：跳过非 builtin 鉴权分支。
6. 非 Agent 类型：领域层错误透传。
7. `SetEvalAsyncCtx` 失败：返回错误。

编译检查使用 `go vet`（遵循 CLAUDE.md，不使用 `go build`），并运行 evaluation 模块既有测试命令确认关联单测通过。

## 7. 影响文件清单

- `idl/thrift/coze/loop/evaluation/coze.loop.evaluation.openapi.thrift`（新增结构 + 方法）
- 生成代码：`backend/kitex_gen/...`、`backend/loop_gen/.../local_evaluationopenapiservice.go`、
  `backend/api/handler/coze/loop/apis/eval_open_apiservice.go`、
  `backend/api/router/coze/loop/apis/coze.loop.apis.go`
- `backend/modules/evaluation/application/eval_openapi_app.go`（新增方法）
- `backend/modules/evaluation/application/eval_openapi_app_test.go`（或对应测试文件，新增单测）
- 无 DB / Clickhouse / 配置改动。
