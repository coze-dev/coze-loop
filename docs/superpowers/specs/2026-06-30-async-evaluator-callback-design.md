# 异步评估器执行完成回调（Callback URL）— 设计文档

- 日期：2026-06-30
- 分支：feat/asyncevaluatorcall
- 范围：仅后端（evaluation 模块）。无 DB / Clickhouse / 配置变更。

## 1. 背景与问题

`AsyncRunEvaluatorOApi`（`POST /v1/loop/evaluation/evaluators_versions/:evaluator_version_id/async_run`）
当前只支持「提交后轮询」：调用方拿到 `invoke_id` 后，需反复轮询
`BatchGetEvaluatorRecordsOApi` 直到记录状态离开 `AsyncInvoking`。

本设计新增**主动回调**能力：调用方在提交时携带 `callback_url`，评估器执行完成后，
服务端主动以带 HMAC 签名的 POST 通知该 URL，免去轮询。

既有可复用基础设施：

- `EvalAsyncCtx`（`domain/entity/expt_run.go:577`）：异步上下文，JSON 序列化后存入 Redis
  （key `evaluator:{recordID}`，TTL 12h），由 `ReportEvaluatorInvokeResult` 回调读取。
- 实验 webhook 的签名/POST 逻辑（`domain/service/webhook_dispatcher.go`）：
  HMAC-SHA256 签名 + `X-CozeLoop-Timestamp/Nonce/Signature` header。当前为包内未导出函数，
  与 Experiment 领域耦合。
- `IWebhookSecretProvider`（`domain/service/webhook_dispatcher.go:33`）：签名密钥提供者，
  开源默认 `NoopWebhookSecretProvider` 返回空 secret；企业版可经 Wire 绑定真实实现。
- `infra/backoff.RetryThreeSeconds(ctx, fn)`：3 秒窗口指数退避重试（项目既有，多处使用）。

## 2. 目标与非目标

### 目标
- `AsyncRunEvaluatorOApiRequest` 新增可选 `callback_url` 字段（纯新增，向前兼容）。
- 评估器异步执行完成时，主动以带 HMAC 签名的 POST 通知 `callback_url`。
- 投递采用「同步 POST + `backoff.RetryThreeSeconds` 重试」；失败仅记日志，不进 MQ。
- 复用既有签名方案与 `IWebhookSecretProvider`，使已对接 CozeLoop webhook 的调用方可复用验签逻辑。

### 非目标
- 不引入 MQ 异步重试链路（区别于实验 webhook 的 1/5/30min 重试）。
- 不改动实验 webhook（`WebhookDispatcher` / `WebhookRetryConsumer`）的行为。
- 不改 `ReportEvaluatorInvokeResult` 的对外契约；回调失败不影响该接口返回成功。
- 无 DB / Clickhouse / 配置变更。

## 3. 设计决策（已与用户确认）

| 决策点 | 结论 |
| --- | --- |
| 投递可靠性 | 同步 POST + `backoff.RetryThreeSeconds`；失败仅记日志，不进 MQ |
| Payload 与签名 | 全量结果 + HMAC-SHA256 签名（复用 `IWebhookSecretProvider`） |
| 签名/POST 代码复用 | 新建轻量 `EvaluatorCallbackDispatcher`，复用从 `webhook_dispatcher.go` 抽出的签名 helper |
| URL 传入位置 | `AsyncRunEvaluatorOApiRequest` 新增顶级字段 `callback_url` |

## 4. 详细设计

### 4.1 IDL — `idl/thrift/coze/loop/evaluation/coze.loop.evaluation.openapi.thrift`

在 `AsyncRunEvaluatorOApiRequest`（第 889 行）新增字段 5（字段号当前空闲）：

```thrift
struct AsyncRunEvaluatorOApiRequest {
    1: optional i64 evaluator_version_id (api.path = "evaluator_version_id", api.js_conv = "true", go.tag = 'json:"evaluator_version_id"')
    2: optional i64 workspace_id (api.body = "workspace_id", api.js_conv = "true", go.tag = 'json:"workspace_id"')
    3: optional evaluator.EvaluatorInputData input_data (api.body = "input_data")
    4: optional evaluator.EvaluatorRunConfig evaluator_run_conf (api.body = "evaluator_run_conf")
    5: optional string callback_url (api.body = "callback_url")  // 执行完成后回调通知的 URL，为空则不回调

    100: optional map<string, string> ext (api.body = "ext")

    254: optional extra.Extra extra (agw.source = "not_body_struct")
    255: optional base.Base Base
}
```

向前兼容性：纯新增 optional 字段，不修改任何既有字段编号或方法签名。

### 4.2 代码生成

修改 IDL 后，使用项目既有 codegen 工具链重新生成（参考 `docs/guidance/idl-codegen-guide.md`）：

```bash
cd backend
bash script/cloudwego/kitex_tool.sh
bash script/cloudwego/hertz_tool.sh
bash script/cloudwego/code_gen.sh
```

生成产物会更新 `AsyncRunEvaluatorOApiRequest` DTO（新增 `CallbackURL *string` 字段及
`GetCallbackURL()` getter）。本字段仅作用于请求 DTO，handler / router / local service
转发逻辑无需改变签名。

> 注意：直接运行 `kitex_tool.sh` 可能触发 CI 的 auto-commit 检查并污染仓库本地 git config；
> 优先使用 `code_gen.sh`，或在执行前 `export NO_PUSH_REMOTE=true`，事后核对
> `git config --local user.name/user.email` 干净。

### 4.3 实体 — `domain/entity/expt_run.go`

`EvalAsyncCtx` 新增字段：

```go
type EvalAsyncCtx struct {
	Event                   *ExptItemEvalEvent
	RecordID                int64
	AsyncUnixMS             int64
	Session                 *Session
	Callee                  string
	EvaluatorVersionID      int64
	EnableExtractTrajectory *bool
	CallbackURL             string `json:"callback_url,omitempty"` // 异步执行完成后回调通知的 URL
}
```

向前兼容性：`EvalAsyncCtx` 经 `json.Marshal` 存入 Redis（`redis/convert/item_turn_eval_async.go`）。
旧记录无该字段 → 反序列化为 `""`；新记录携带 URL。无需迁移，TTL 12h 自然过期。

### 4.4 签名 helper 复用 — `domain/service/webhook_dispatcher.go`

将签名/POST 所需的纯函数导出（或抽到同包 `webhook_sign.go`），供新 dispatcher 复用，
不改变实验 webhook 行为：

- `ComputeHMACSHA256(secret, message string) string`（原 `computeHMACSHA256`）
- `GenerateNonce() string`（原 `generateNonce`）

`WebhookDispatcher` 内部调用点同步改为引用导出名。`doPost` 不复用（各 dispatcher 自带
`http.Client` 与 header 设置，逻辑短小，复制以保持解耦）。

### 4.5 新建轻量 dispatcher — `domain/service/evaluator_callback_dispatcher.go`

```go
// IEvaluatorCallbackDispatcher 评估器异步执行完成回调分发器
type IEvaluatorCallbackDispatcher interface {
	Dispatch(ctx context.Context, spaceID int64, callbackURL string, payload *EvaluatorCallbackPayload) error
}

// EvaluatorCallbackPayload 回调 POST body
type EvaluatorCallbackPayload struct {
	DeliveryID         string `json:"delivery_id"`
	InvokeID           int64  `json:"invoke_id"`
	WorkspaceID        int64  `json:"workspace_id"`
	EvaluatorVersionID int64  `json:"evaluator_version_id"`
	Status             string `json:"status"` // success | fail
	Output             any    `json:"output,omitempty"`
	TimeConsumingMS    int64  `json:"time_consuming_ms"`
}

type EvaluatorCallbackDispatcher struct {
	httpClient     *http.Client
	secretProvider IWebhookSecretProvider
}

func NewEvaluatorCallbackDispatcher(secretProvider IWebhookSecretProvider) *EvaluatorCallbackDispatcher {
	return &EvaluatorCallbackDispatcher{
		httpClient:     &http.Client{Timeout: 5 * time.Second},
		secretProvider: secretProvider,
	}
}
```

`Dispatch` 逻辑：

1. `callbackURL == ""` → 直接返回 nil（跳过）。
2. `json.Marshal(payload)` → body。marshal 失败记日志并返回 nil（不阻塞主流程）。
3. `secret, _ = secretProvider.GetSecret(ctx, spaceID)`（Noop 返回空）。
4. `backoff.RetryThreeSeconds(ctx, func() error { 每次重试重新生成 timestamp/nonce/signature 并 doPost })`。
   - 每次重试重算签名（与 `WebhookRetryConsumer` 行为一致，timestamp/nonce 随时间变化）。
   - `signature = ComputeHMACSHA256(secret, timestamp + "\n" + nonce + "\n")`。
   - `doPost` 设置 header：`Content-Type: application/json`、`X-CozeLoop-Timestamp`、
     `X-CozeLoop-Nonce`、`X-CozeLoop-Signature`；非 2xx 视为失败触发重试。
5. 3 秒窗口耗尽仍失败 → `logs.CtxError` 记录，返回 nil（**不向上抛错**，回调失败不影响调用方接口）。

`DeliveryID` 由 dispatcher 生成（`GenerateNonce()`），用于调用方去重/追踪。

### 4.6 提交时写入 URL — `application/eval_openapi_app.go` `AsyncRunEvaluatorOApi`

在构造 `EvalAsyncCtx` 时填入 URL：

```go
if err = e.asyncRepo.SetEvalAsyncCtx(ctx, asyncCtxKey, &entity.EvalAsyncCtx{
	RecordID:           record.ID,
	AsyncUnixMS:        startTime.UnixMilli(),
	Session:            &entity.Session{UserID: usersession.UserIDInCtxOrEmpty(ctx)},
	EvaluatorVersionID: req.GetEvaluatorVersionID(),
	CallbackURL:        req.GetCallbackURL(),
}); err != nil { ... }
```

### 4.7 回调触发 — `application/eval_openapi_app.go` `ReportEvaluatorInvokeResult_`

在记录结果更新成功之后、既有实验事件发布逻辑之后，新增回调投递：

```go
// 既有：actx.Event != nil 时发布实验事件（不动）

// 新增：独立调用携带 callback_url 时，主动回调通知
if actx.CallbackURL != "" {
	payload := &entity.EvaluatorCallbackPayload{
		InvokeID:           req.GetInvokeID(),
		WorkspaceID:        req.GetWorkspaceID(),
		EvaluatorVersionID: actx.EvaluatorVersionID,
		Status:             callbackStatusString(req.GetStatus()), // success | fail
		Output:             req.GetOutput(),
		TimeConsumingMS:    time.Now().UnixMilli() - actx.AsyncUnixMS,
	}
	if derr := e.callbackDispatcher.Dispatch(ctx, req.GetWorkspaceID(), actx.CallbackURL, payload); derr != nil {
		logs.CtxError(ctx, "[ReportEvaluatorInvokeResult] callback dispatch fail, invoke_id: %v, url: %v, err: %v",
			req.GetInvokeID(), actx.CallbackURL, derr)
		// 不返回错误：回调失败不影响运行时回报接口成功
	}
}
```

`callbackStatusString` 将 `EvaluatorRunStatus` 映射为 `"success"` / `"fail"`。
`req.GetOutput()` 为 DTO 输出，直接作为 `Output any` 序列化。

> `EvaluatorCallbackPayload` 在 dispatcher 接口定义于 `domain/service`，
> payload 结构体放在 `domain/entity`（被 application 与 service 共同引用），避免循环依赖。
> 4.5 中示意的结构体最终落在 `domain/entity`，dispatcher 接口引用之。

### 4.8 依赖注入 — `application/wire.go` 及相关 provider set

`EvalOpenAPIApplication` 新增成员 `callbackDispatcher service.IEvaluatorCallbackDispatcher`，
经构造函数注入。Wire provider set 增加：

- `NewEvaluatorCallbackDispatcher`（绑定 `IEvaluatorCallbackDispatcher`）。
- 复用既有 `IWebhookSecretProvider` 绑定（`NoopWebhookSecretProvider`，`domain/service/wire.go:48`）。

确认 `EvalOpenAPIApplication` 构造函数与其 Wire provider 同步更新；运行 `wire` 重新生成
`wire_gen.go`（勿手改）。

### 4.9 调用流程时序

```
调用方 → AsyncRunEvaluatorOApi(callback_url)
          ├─ 鉴权 / 转换
          ├─ AsyncRunEvaluator → 落库 record(status=AsyncInvoking) → 分发 Agent 运行时
          ├─ SetEvalAsyncCtx("evaluator:{id}", {…, CallbackURL: url})
          └─ 返回 {invoke_id, record(AsyncInvoking)}

Agent 运行时执行完成 → ReportEvaluatorInvokeResult(invoke_id, output, status)
          ├─ GetEvalAsyncCtx → 拿到 CallbackURL
          ├─ ReportEvaluatorInvokeResult → 更新记录(status=Success/Fail)
          ├─ actx.Event != nil → 发布实验事件（既有，不动）
          ├─ actx.CallbackURL != "" → EvaluatorCallbackDispatcher.Dispatch
          │     └─ backoff.RetryThreeSeconds(POST + HMAC 签名)
          │         成功 → 完成；失败 → 仅 logs.CtxError，不进 MQ
          └─ 返回成功（回调失败不影响）

调用方收到带签名 POST → 验签 → 处理结果（无需轮询）
```

## 5. 错误处理

| 场景 | 行为 |
| --- | --- |
| 提交时 `callback_url` 为空 | 正常提交，`EvalAsyncCtx.CallbackURL = ""`，完成时不回调 |
| `SetEvalAsyncCtx` 失败 | 记日志并返回错误（与既有一致） |
| 回调阶段 `CallbackURL == ""` | dispatcher 直接返回 nil（跳过） |
| payload marshal 失败 | 记日志，返回 nil，不阻塞主流程 |
| POST 3s 内重试仍失败 | 记日志，返回 nil；`ReportEvaluatorInvokeResult` 仍返回成功 |
| `IWebhookSecretProvider` 为 Noop | secret 为空，签名照常计算（开源默认行为，与实验 webhook 一致） |

## 6. 测试计划（TDD）

### 6.1 `EvaluatorCallbackDispatcher` 单测（`httptest.Server`）
1. `callbackURL == ""` → 跳过，server 不被调用。
2. 成功路径：server 收到 POST，校验 body 字段 + 三个签名 header 存在且非空。
3. 签名正确性：用固定 secret 的 fake `IWebhookSecretProvider`，服务端按
   `HMAC_SHA256(secret, ts+"\n"+nonce+"\n")` 复算并比对 `X-CozeLoop-Signature`。
4. server 返回 500 → 触发 `RetryThreeSeconds` 重试（多次命中），最终返回 nil（不抛错）。
5. payload marshal 失败（构造不可序列化 Output）→ 返回 nil，server 不被调用。

### 6.2 `AsyncRunEvaluatorOApi` 单测（扩展既有用例）
- 成功路径断言 `SetEvalAsyncCtx` 收到的 `EvalAsyncCtx.CallbackURL == req.callback_url`
  （DoAndReturn 中断言）。
- `callback_url` 为空时 `EvalAsyncCtx.CallbackURL == ""`。

### 6.3 `ReportEvaluatorInvokeResult_` 单测（mock dispatcher）
- `actx.CallbackURL != ""` → mock dispatcher `Dispatch` 被调用一次，入参 url/status/invoke_id 正确。
- `actx.CallbackURL == ""` → `Dispatch` 不被调用。
- `Dispatch` 返回 error → 接口仍返回成功（断言无错误返回）。

### 6.4 编译与回归
- `go vet ./...`（遵循 CLAUDE.md，不用 `go build`）。
- 运行 evaluation 模块既有测试，确认关联单测通过。

## 7. 影响文件清单

- `idl/thrift/coze/loop/evaluation/coze.loop.evaluation.openapi.thrift`（`AsyncRunEvaluatorOApiRequest` 新增字段 5）
- 生成代码：`backend/kitex_gen/...`、`backend/loop_gen/...`、handler / router（DTO 新增 getter）
- `backend/modules/evaluation/domain/entity/expt_run.go`（`EvalAsyncCtx.CallbackURL`）
- `backend/modules/evaluation/domain/entity/`（新增 `EvaluatorCallbackPayload` 结构体）
- `backend/modules/evaluation/domain/service/webhook_dispatcher.go`（导出签名 helper）
- `backend/modules/evaluation/domain/service/evaluator_callback_dispatcher.go`（新建）
- `backend/modules/evaluation/domain/service/evaluator_callback_dispatcher_test.go`（新建）
- `backend/modules/evaluation/application/eval_openapi_app.go`（提交写 URL + 回调触发）
- `backend/modules/evaluation/application/eval_openapi_app_test.go`（扩展用例）
- `backend/modules/evaluation/application/wire.go` 及 `wire_gen.go`（注入 dispatcher）
- 相关 mock 文件（`IEvaluatorCallbackDispatcher` 的 gomock）
- 无 DB / Clickhouse / 配置改动。
