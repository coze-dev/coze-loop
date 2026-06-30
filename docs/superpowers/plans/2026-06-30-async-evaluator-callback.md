# 异步评估器执行完成回调（Callback URL）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 让 `AsyncRunEvaluatorOApi` 支持调用方传入 `callback_url`，评估器异步执行完成后主动以带 HMAC 签名的 POST 回调通知该 URL。

**Architecture:** 调用方提交时携带 `callback_url`，随 `EvalAsyncCtx` 落入 Redis；Agent 运行时通过既有 `ReportEvaluatorInvokeResult` 回报结果时，服务端读取 URL，经新建的轻量 `EvaluatorCallbackDispatcher` 用 `backoff.RetryThreeSeconds` 同步 POST（复用实验 webhook 的 HMAC 签名 helper），失败仅记日志、不进 MQ、不影响回报接口返回成功。

**Tech Stack:** Go、CloudWeGo Thrift/Kitex/Hertz codegen、gomock、`infra/backoff`、`net/http`、HMAC-SHA256。

**约束（来自 CLAUDE.md）：** 不在本地运行项目；编译检查用 `go vet` 不用 `go build`；IDL 向前兼容；后端遵循 DDD；改动后确认关联单测通过；无 DB/Clickhouse/配置变更，故 release 下 docker/k8s 无需改动。

---

## 文件结构

| 文件 | 职责 | 动作 |
| --- | --- | --- |
| `idl/thrift/coze/loop/evaluation/coze.loop.evaluation.openapi.thrift` | `AsyncRunEvaluatorOApiRequest` 新增字段 5 `callback_url` | 修改 |
| `backend/kitex_gen/...`、`backend/loop_gen/...`、handler/router | 生成 DTO `CallbackURL` + `GetCallbackURL()` | 生成 |
| `backend/modules/evaluation/domain/entity/expt_run.go` | `EvalAsyncCtx.CallbackURL` 字段 | 修改 |
| `backend/modules/evaluation/domain/entity/evaluator_callback.go` | `EvaluatorCallbackPayload` 结构体 | 创建 |
| `backend/modules/evaluation/domain/service/webhook_dispatcher.go` | 导出签名 helper（`ComputeHMACSHA256` / `GenerateNonce`） | 修改 |
| `backend/modules/evaluation/domain/service/evaluator_callback_dispatcher.go` | `IEvaluatorCallbackDispatcher` + 实现 | 创建 |
| `backend/modules/evaluation/domain/service/evaluator_callback_dispatcher_test.go` | dispatcher 单测 | 创建 |
| `backend/modules/evaluation/domain/service/mocks/evaluator_callback_dispatcher_mock.go` | gomock | 生成 |
| `backend/modules/evaluation/application/eval_openapi_app.go` | 提交写 URL + 回报触发回调 | 修改 |
| `backend/modules/evaluation/application/eval_openapi_app_test.go` | 扩展用例 | 修改 |
| `backend/modules/evaluation/application/wire.go` + `wire_gen.go` | 注入 dispatcher | 修改/生成 |

---

## Task 1: IDL 新增 callback_url 字段

**Files:**
- Modify: `idl/thrift/coze/loop/evaluation/coze.loop.evaluation.openapi.thrift:889-899`

- [ ] **Step 1: 在 `AsyncRunEvaluatorOApiRequest` 新增字段 5**

将该结构体（第 889 行起）修改为（仅新增 `5: ... callback_url` 一行，其余不动）：

```thrift
struct AsyncRunEvaluatorOApiRequest {
    1: optional i64 evaluator_version_id (api.path = "evaluator_version_id", api.js_conv = "true", go.tag = 'json:"evaluator_version_id"')
    2: optional i64 workspace_id (api.body = "workspace_id", api.js_conv = "true", go.tag = 'json:"workspace_id"')
    3: optional evaluator.EvaluatorInputData input_data (api.body = "input_data")
    4: optional evaluator.EvaluatorRunConfig evaluator_run_conf (api.body = "evaluator_run_conf")
    5: optional string callback_url (api.body = "callback_url")

    100: optional map<string, string> ext (api.body = "ext")

    254: optional extra.Extra extra (agw.source = "not_body_struct")
    255: optional base.Base Base
}
```

- [ ] **Step 2: 校验 IDL 语法（diff 检查）**

Run: `cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop && git diff idl/thrift/coze/loop/evaluation/coze.loop.evaluation.openapi.thrift`
Expected: 仅新增一行 `5: optional string callback_url (api.body = "callback_url")`，字段号 5 此前空闲，无其它改动。

- [ ] **Step 3: Commit**

```bash
cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop
git add idl/thrift/coze/loop/evaluation/coze.loop.evaluation.openapi.thrift
git commit -m "[idl][evaluation] add callback_url to AsyncRunEvaluatorOApiRequest"
```

---

## Task 2: 重新生成代码

**Files:**
- Generate: `backend/kitex_gen/coze/loop/evaluation/openapi/...`、`backend/loop_gen/...`、handler、router

- [ ] **Step 1: 运行 codegen 工具链**

```bash
cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop/backend
export NO_PUSH_REMOTE=true
bash script/cloudwego/kitex_tool.sh
bash script/cloudwego/hertz_tool.sh
bash script/cloudwego/code_gen.sh
```

注意：codegen 可能触发 CI auto-commit 污染本地 git config。执行后务必核对：

```bash
git config --local --get user.name; git config --local --get user.email
```
若被改成空值，执行 `git config --local --unset user.name; git config --local --unset user.email` 清理，确保有效身份为 `liushengyang <liushengyang@bytedance.com>`。

- [ ] **Step 2: 确认生成了 getter**

Run: `cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop/backend && grep -n "func (p \*AsyncRunEvaluatorOApiRequest) GetCallbackURL" kitex_gen/coze/loop/evaluation/openapi/coze.loop.evaluation.openapi.go`
Expected: 命中一行 `func (p *AsyncRunEvaluatorOApiRequest) GetCallbackURL() (v string)`。

- [ ] **Step 3: 编译检查**

Run: `cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop/backend && go vet ./kitex_gen/coze/loop/evaluation/openapi/... ./api/... 2>&1 | grep -v "JobID repeats json tag" | head`
Expected: 无新增报错（已知 `JobID repeats json tag "workspace_id"` 为基线既有警告，与本次无关，忽略）。

- [ ] **Step 4: Commit**

```bash
cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop
git add backend/kitex_gen backend/loop_gen backend/api
git commit -m "[backend][evaluation] regenerate code for callback_url"
```

---

## Task 3: EvalAsyncCtx 新增 CallbackURL 字段

**Files:**
- Modify: `backend/modules/evaluation/domain/entity/expt_run.go:577-585`
- Test: `backend/modules/evaluation/infra/repo/experiment/redis/convert/item_turn_eval_async_test.go`（若存在则扩展；否则新建）

- [ ] **Step 1: 写失败测试（验证 CallbackURL 经 JSON round-trip 保留）**

新建或追加到 `backend/modules/evaluation/infra/repo/experiment/redis/convert/item_turn_eval_async_test.go`：

```go
// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestExptItemTurnEvalAsyncCtx_CallbackURLRoundTrip(t *testing.T) {
	c := NewExptItemTurnEvalAsyncCtx()
	in := &entity.EvalAsyncCtx{
		RecordID:           123,
		EvaluatorVersionID: 456,
		CallbackURL:        "https://example.com/hook",
	}
	b, err := c.FromDO(in)
	assert.NoError(t, err)

	out, err := c.ToDO(b)
	assert.NoError(t, err)
	assert.Equal(t, "https://example.com/hook", out.CallbackURL)
	assert.Equal(t, int64(123), out.RecordID)
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop/backend && go test ./modules/evaluation/infra/repo/experiment/redis/convert/ -run TestExptItemTurnEvalAsyncCtx_CallbackURLRoundTrip -v`
Expected: FAIL 或编译错误 `out.CallbackURL undefined`（字段尚不存在）。

- [ ] **Step 3: 在 `EvalAsyncCtx` 新增字段**

修改 `backend/modules/evaluation/domain/entity/expt_run.go` 第 577 行起的结构体，新增最后一行：

```go
type EvalAsyncCtx struct {
	Event                   *ExptItemEvalEvent
	RecordID                int64
	AsyncUnixMS             int64 // async call time with unix ms ts
	Session                 *Session
	Callee                  string
	EvaluatorVersionID      int64 // evaluator version id, used for evaluator async scenario
	EnableExtractTrajectory *bool
	CallbackURL             string `json:"callback_url,omitempty"` // 异步执行完成后回调通知的 URL，为空则不回调
}
```

- [ ] **Step 4: 运行测试，确认通过**

Run: `cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop/backend && go test ./modules/evaluation/infra/repo/experiment/redis/convert/ -run TestExptItemTurnEvalAsyncCtx_CallbackURLRoundTrip -v`
Expected: PASS。

- [ ] **Step 5: Commit**

```bash
cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop
git add backend/modules/evaluation/domain/entity/expt_run.go backend/modules/evaluation/infra/repo/experiment/redis/convert/item_turn_eval_async_test.go
git commit -m "[backend][evaluation] add CallbackURL to EvalAsyncCtx"
```

---

## Task 4: 导出签名 helper

**Files:**
- Modify: `backend/modules/evaluation/domain/service/webhook_dispatcher.go:108-114,170-180`

- [ ] **Step 1: 重命名两个未导出 helper 为导出名**

在 `webhook_dispatcher.go` 第 170 行起，将函数改名：

```go
// ComputeHMACSHA256 计算 HMAC-SHA256 签名（hex 编码），供 webhook 与 evaluator 回调复用
func ComputeHMACSHA256(secret, message string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

// GenerateNonce 生成 16 字节随机 nonce（hex 编码）
func GenerateNonce() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
```

- [ ] **Step 2: 更新 `Dispatch` 内部调用点**

在 `webhook_dispatcher.go` `Dispatch` 方法签名段（约第 110-113 行），将：

```go
	nonce := generateNonce()
	signMessage := timestamp + "\n" + nonce + "\n"
	signature := computeHMACSHA256(secret, signMessage)
```

改为：

```go
	nonce := GenerateNonce()
	signMessage := timestamp + "\n" + nonce + "\n"
	signature := ComputeHMACSHA256(secret, signMessage)
```

- [ ] **Step 3: 编译检查（确认无遗漏旧名引用）**

Run: `cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop/backend && grep -rn "computeHMACSHA256\|generateNonce" modules/evaluation/domain/service/ ; go vet ./modules/evaluation/domain/service/ 2>&1 | head`
Expected: grep 无 `computeHMACSHA256` / `generateNonce`（已全部改为导出名；注意 `webhook_retry.go` 中的 `computeRetryHMACSHA256`/`generateRetryNonce` 是独立函数，不在此范围）；`go vet` 无报错。

- [ ] **Step 4: 运行既有 webhook 单测，确认无回归**

Run: `cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop/backend && go test ./modules/evaluation/domain/service/ -run "Webhook" -v 2>&1 | tail -20`
Expected: 既有 webhook 测试全部 PASS。

- [ ] **Step 5: Commit**

```bash
cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop
git add backend/modules/evaluation/domain/service/webhook_dispatcher.go
git commit -m "[backend][evaluation] export webhook signing helpers for reuse"
```

---

## Task 5: EvaluatorCallbackPayload 实体

**Files:**
- Create: `backend/modules/evaluation/domain/entity/evaluator_callback.go`

- [ ] **Step 1: 新建 payload 结构体**

创建 `backend/modules/evaluation/domain/entity/evaluator_callback.go`：

```go
// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// EvaluatorCallbackPayload 异步评估器执行完成回调的 POST body
type EvaluatorCallbackPayload struct {
	DeliveryID         string `json:"delivery_id"`
	InvokeID           int64  `json:"invoke_id"`
	WorkspaceID        int64  `json:"workspace_id"`
	EvaluatorVersionID int64  `json:"evaluator_version_id"`
	Status             string `json:"status"` // success | fail
	Output             any    `json:"output,omitempty"`
	TimeConsumingMS    int64  `json:"time_consuming_ms"`
}
```

- [ ] **Step 2: 编译检查**

Run: `cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop/backend && go vet ./modules/evaluation/domain/entity/ 2>&1 | head`
Expected: 无报错。

- [ ] **Step 3: Commit**

```bash
cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop
git add backend/modules/evaluation/domain/entity/evaluator_callback.go
git commit -m "[backend][evaluation] add EvaluatorCallbackPayload entity"
```

---

## Task 6: EvaluatorCallbackDispatcher（接口 + 实现 + 单测）

**Files:**
- Create: `backend/modules/evaluation/domain/service/evaluator_callback_dispatcher.go`
- Test: `backend/modules/evaluation/domain/service/evaluator_callback_dispatcher_test.go`

- [ ] **Step 1: 写失败测试（httptest.Server 验证投递、签名、跳过、重试）**

创建 `backend/modules/evaluation/domain/service/evaluator_callback_dispatcher_test.go`：

```go
// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// fixedSecretProvider 返回固定 secret，用于验签
type fixedSecretProvider struct{ secret string }

func (p *fixedSecretProvider) GetSecret(ctx context.Context, spaceID int64) (string, error) {
	return p.secret, nil
}

func TestEvaluatorCallbackDispatcher_EmptyURL_Skips(t *testing.T) {
	var called int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&called, 1)
	}))
	defer srv.Close()

	d := NewEvaluatorCallbackDispatcher(&fixedSecretProvider{secret: "s"})
	err := d.Dispatch(context.Background(), 1, "", &entity.EvaluatorCallbackPayload{InvokeID: 1})
	assert.NoError(t, err)
	assert.Equal(t, int32(0), atomic.LoadInt32(&called))
}

func TestEvaluatorCallbackDispatcher_Success_PostsSignedPayload(t *testing.T) {
	var gotBody []byte
	var gotSig, gotTs, gotNonce string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		gotSig = r.Header.Get("X-CozeLoop-Signature")
		gotTs = r.Header.Get("X-CozeLoop-Timestamp")
		gotNonce = r.Header.Get("X-CozeLoop-Nonce")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	secret := "test-secret"
	d := NewEvaluatorCallbackDispatcher(&fixedSecretProvider{secret: secret})
	payload := &entity.EvaluatorCallbackPayload{
		InvokeID:           100,
		WorkspaceID:        200,
		EvaluatorVersionID: 300,
		Status:             "success",
		TimeConsumingMS:    42,
	}
	err := d.Dispatch(context.Background(), 200, srv.URL, payload)
	assert.NoError(t, err)

	var decoded entity.EvaluatorCallbackPayload
	assert.NoError(t, json.Unmarshal(gotBody, &decoded))
	assert.Equal(t, int64(100), decoded.InvokeID)
	assert.Equal(t, "success", decoded.Status)
	assert.NotEmpty(t, decoded.DeliveryID)
	// 验签：服务端用相同 secret 复算
	assert.Equal(t, ComputeHMACSHA256(secret, gotTs+"\n"+gotNonce+"\n"), gotSig)
}

func TestEvaluatorCallbackDispatcher_Non2xx_RetriesThenReturnsNil(t *testing.T) {
	var called int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&called, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	d := NewEvaluatorCallbackDispatcher(&fixedSecretProvider{secret: "s"})
	err := d.Dispatch(context.Background(), 1, srv.URL, &entity.EvaluatorCallbackPayload{InvokeID: 1})
	// 3s 窗口耗尽仍失败 → 不抛错
	assert.NoError(t, err)
	// 至少调用了一次（退避会重试多次）
	assert.GreaterOrEqual(t, atomic.LoadInt32(&called), int32(1))
}
```

- [ ] **Step 2: 运行测试，确认失败**

Run: `cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop/backend && go test ./modules/evaluation/domain/service/ -run "TestEvaluatorCallbackDispatcher" -v 2>&1 | head`
Expected: 编译失败 `NewEvaluatorCallbackDispatcher undefined`。

- [ ] **Step 3: 实现 dispatcher**

创建 `backend/modules/evaluation/domain/service/evaluator_callback_dispatcher.go`：

```go
// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/backoff"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

//go:generate mockgen -destination mocks/evaluator_callback_dispatcher_mock.go -package mocks . IEvaluatorCallbackDispatcher

// IEvaluatorCallbackDispatcher 评估器异步执行完成回调分发器
type IEvaluatorCallbackDispatcher interface {
	Dispatch(ctx context.Context, spaceID int64, callbackURL string, payload *entity.EvaluatorCallbackPayload) error
}

// EvaluatorCallbackDispatcher 同步 POST + backoff 重试投递；失败仅记日志，不进 MQ
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

func (d *EvaluatorCallbackDispatcher) Dispatch(ctx context.Context, spaceID int64, callbackURL string, payload *entity.EvaluatorCallbackPayload) error {
	if callbackURL == "" {
		return nil
	}
	if payload.DeliveryID == "" {
		payload.DeliveryID = GenerateNonce()
	}

	body, err := json.Marshal(payload)
	if err != nil {
		logs.CtxError(ctx, "[EvaluatorCallbackDispatcher] marshal payload fail, invoke_id: %v, err: %v", payload.InvokeID, err)
		return nil // 不阻塞主流程
	}

	var secret string
	if d.secretProvider != nil {
		secret, _ = d.secretProvider.GetSecret(ctx, spaceID)
	}

	// 同步 POST + 3s 窗口退避重试；每次重试重算签名
	if rerr := backoff.RetryThreeSeconds(ctx, func() error {
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		nonce := GenerateNonce()
		signature := ComputeHMACSHA256(secret, timestamp+"\n"+nonce+"\n")
		return d.doPost(ctx, callbackURL, body, timestamp, nonce, signature)
	}); rerr != nil {
		logs.CtxError(ctx, "[EvaluatorCallbackDispatcher] post fail after retry, invoke_id: %v, url: %v, err: %v", payload.InvokeID, callbackURL, rerr)
	}
	return nil
}

func (d *EvaluatorCallbackDispatcher) doPost(ctx context.Context, url string, body []byte, timestamp, nonce, signature string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CozeLoop-Timestamp", timestamp)
	req.Header.Set("X-CozeLoop-Nonce", nonce)
	req.Header.Set("X-CozeLoop-Signature", signature)

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()        //nolint:errcheck
	io.Copy(io.Discard, resp.Body) //nolint:errcheck

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("evaluator callback returned non-2xx status: %d", resp.StatusCode)
	}
	return nil
}
```

- [ ] **Step 4: 运行测试，确认通过**

Run: `cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop/backend && go test ./modules/evaluation/domain/service/ -run "TestEvaluatorCallbackDispatcher" -v 2>&1 | tail -20`
Expected: 三个用例全部 PASS。

- [ ] **Step 5: 生成 mock**

```bash
cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop/backend
go generate ./modules/evaluation/domain/service/...
```
若 `go generate` 因其它 `//go:generate` 报错，可单独生成本接口：
```bash
cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service
mockgen -destination mocks/evaluator_callback_dispatcher_mock.go -package mocks . IEvaluatorCallbackDispatcher
```

Run: `cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop/backend && ls modules/evaluation/domain/service/mocks/evaluator_callback_dispatcher_mock.go`
Expected: 文件存在。

- [ ] **Step 6: 编译检查**

Run: `cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop/backend && go vet ./modules/evaluation/domain/service/...`
Expected: 无报错。

- [ ] **Step 7: Commit**

```bash
cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop
git add backend/modules/evaluation/domain/service/evaluator_callback_dispatcher.go backend/modules/evaluation/domain/service/evaluator_callback_dispatcher_test.go backend/modules/evaluation/domain/service/mocks/evaluator_callback_dispatcher_mock.go
git commit -m "[backend][evaluation] add EvaluatorCallbackDispatcher"
```

---

## Task 7: 应用层接入（注入 + 提交写 URL + 回报触发回调）

**Files:**
- Modify: `backend/modules/evaluation/application/eval_openapi_app.go`（struct 第 49 行、构造函数第 71 行、`AsyncRunEvaluatorOApi` 约第 2050 行、`ReportEvaluatorInvokeResult_` 约第 2571 行）

- [ ] **Step 1: struct 新增成员**

在 `eval_openapi_app.go` 第 49 行起的 `EvalOpenAPIApplication` struct 末尾（`fileProvider` 之后）新增：

```go
	fileProvider           rpc.IFileProvider
	callbackDispatcher     service.IEvaluatorCallbackDispatcher
}
```

- [ ] **Step 2: 构造函数新增参数与赋值**

在 `NewEvalOpenAPIApplication`（第 71 行）参数列表末尾 `fileProvider rpc.IFileProvider,` 之后新增参数，并在返回的结构体字面量末尾新增赋值：

参数列表新增：
```go
	fileProvider rpc.IFileProvider,
	callbackDispatcher service.IEvaluatorCallbackDispatcher,
) IEvalOpenAPIApplication {
```

返回字面量新增（`fileProvider: fileProvider,` 之后）：
```go
		fileProvider:                fileProvider,
		callbackDispatcher:          callbackDispatcher,
	}
```

- [ ] **Step 3: 提交时写入 CallbackURL**

在 `AsyncRunEvaluatorOApi` 内构造 `EvalAsyncCtx` 的 `SetEvalAsyncCtx` 调用处（当前字段为 `RecordID/AsyncUnixMS/Session/EvaluatorVersionID`），新增 `CallbackURL` 字段：

```go
	if err = e.asyncRepo.SetEvalAsyncCtx(ctx, asyncCtxKey, &entity.EvalAsyncCtx{
		RecordID:           record.ID,
		AsyncUnixMS:        startTime.UnixMilli(),
		Session:            &entity.Session{UserID: usersession.UserIDInCtxOrEmpty(ctx)},
		EvaluatorVersionID: req.GetEvaluatorVersionID(),
		CallbackURL:        req.GetCallbackURL(),
	}); err != nil {
		logs.CtxError(ctx, "[AsyncRunEvaluatorOApi] SetEvalAsyncCtx fail, invokeID: %d, err: %v", record.ID, err)
		return nil, err
	}
```

- [ ] **Step 4: 新增回调状态映射 helper**

在 `eval_openapi_app.go` 文件末尾新增（与 `req.GetStatus()` 的 `spi.InvokeEvaluatorRunStatus` 类型对齐）：

```go
func evaluatorCallbackStatusString(status spi.InvokeEvaluatorRunStatus) string {
	if status == spi.InvokeEvaluatorRunStatus_SUCCESS {
		return "success"
	}
	return "fail"
}
```

确认文件 import 已含 `spi "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"`（`ReportEvaluatorInvokeResult_` 已使用 `req.GetStatus()`/`req.GetOutput()`，通常已导入；若未导入则补上）。

- [ ] **Step 5: 回报时触发回调**

在 `ReportEvaluatorInvokeResult_`（第 2571 行）中，既有的 `if actx.Event != nil { ... PublishExptRecordEvalEvent ... }` 块**之后**、`return &openapi.ReportEvaluatorInvokeResultResponse{...}` **之前**，新增：

```go
	if actx.CallbackURL != "" {
		payload := &entity.EvaluatorCallbackPayload{
			InvokeID:           req.GetInvokeID(),
			WorkspaceID:        req.GetWorkspaceID(),
			EvaluatorVersionID: actx.EvaluatorVersionID,
			Status:             evaluatorCallbackStatusString(req.GetStatus()),
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

- [ ] **Step 6: 编译检查**

Run: `cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop/backend && go vet ./modules/evaluation/application/ 2>&1 | grep -v "wire_gen" | head`
Expected: 仅可能出现 `wire_gen.go` 调用 `NewEvalOpenAPIApplication` 参数不匹配的错误（Task 8 处理）；本文件本身无语法错误。

- [ ] **Step 7: Commit**

```bash
cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop
git add backend/modules/evaluation/application/eval_openapi_app.go
git commit -m "[backend][evaluation] wire callback dispatch into async evaluator flow"
```

---

## Task 8: Wire 注入

**Files:**
- Modify: `backend/modules/evaluation/application/wire.go:120-135`
- Generate: `backend/modules/evaluation/application/wire_gen.go`

- [ ] **Step 1: 在 provider set 增加 dispatcher provider**

在 `wire.go` 中（`NewEvalOpenAPIApplication` 所在的 provider set，第 129 行附近），新增 `service.NewEvaluatorCallbackDispatcher` 及其接口绑定。在该 set 内合适位置加入：

```go
		service.NewEvaluatorCallbackDispatcher,
		wire.Bind(new(service.IEvaluatorCallbackDispatcher), new(*service.EvaluatorCallbackDispatcher)),
```

确认 `service.IWebhookSecretProvider` 的绑定（`NoopWebhookSecretProvider`）在依赖图内可达——它已存在于 `service` 包的 provider set（`domain/service/wire.go:48`）。若 application 的 wire set 未引入该 service set，则需引入；以 `wire` 命令的报错为准。

- [ ] **Step 2: 重新生成 wire_gen.go**

```bash
cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop/backend/modules/evaluation/application
wire
```
若 `wire` 报 `IWebhookSecretProvider` 无 provider，在本 set 增加 `service.NewNoopWebhookSecretProvider` + `wire.Bind(new(service.IWebhookSecretProvider), new(*service.NoopWebhookSecretProvider))`，再重跑 `wire`。

Run: `cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop/backend && grep -n "NewEvaluatorCallbackDispatcher" modules/evaluation/application/wire_gen.go`
Expected: `wire_gen.go` 中出现对 `NewEvaluatorCallbackDispatcher` 的调用，且 `NewEvalOpenAPIApplication(...)` 调用新增了 dispatcher 实参。

- [ ] **Step 3: 编译检查**

Run: `cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop/backend && go vet ./modules/evaluation/application/ 2>&1 | head`
Expected: 无报错。

- [ ] **Step 4: Commit**

```bash
cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop
git add backend/modules/evaluation/application/wire.go backend/modules/evaluation/application/wire_gen.go
git commit -m "[backend][evaluation] inject EvaluatorCallbackDispatcher via wire"
```

---

## Task 9: 应用层单测扩展

**Files:**
- Modify: `backend/modules/evaluation/application/eval_openapi_app_test.go`

- [ ] **Step 1: 在 `AsyncRunEvaluatorOApi` 成功用例断言 CallbackURL 落 ctx**

在 `TestEvalOpenAPIApplication_AsyncRunEvaluatorOApi` 的成功用例中，请求带 `CallbackURL: gptr.Of("https://example.com/hook")`，并将 `SetEvalAsyncCtx` 的 mock 改为 `DoAndReturn` 断言：

```go
asyncRepo.EXPECT().SetEvalAsyncCtx(gomock.Any(), "evaluator:4004", gomock.Any()).
	DoAndReturn(func(_ context.Context, _ string, actx *entity.EvalAsyncCtx) error {
		assert.Equal(t, "https://example.com/hook", actx.CallbackURL)
		return nil
	})
```
（沿用既有成功用例中已有的 invokeID/evaluatorVersionID 常量；请求结构体 `&openapi.AsyncRunEvaluatorOApiRequest{...}` 增加 `CallbackURL: gptr.Of("https://example.com/hook")`。）

- [ ] **Step 2: 为 `ReportEvaluatorInvokeResult_` 新增回调用例**

新增测试函数 `TestEvalOpenAPIApplication_ReportEvaluatorInvokeResult_Callback`，构造三种场景。使用 `servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"` 中的 `IEvaluatorCallbackDispatcher` mock，并把它装入 `&EvalOpenAPIApplication{ callbackDispatcher: mockDispatcher, ... }`：

```go
func TestEvalOpenAPIApplication_ReportEvaluatorInvokeResult_Callback(t *testing.T) {
	t.Run("has callback url -> dispatch called once", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		auth := rpcmocks.NewMockIAuthProvider(ctrl)
		asyncRepo := repomocks.NewMockIEvalAsyncRepo(ctrl)
		evaluatorSvc := servicemocks.NewMockEvaluatorService(ctrl)
		dispatcher := servicemocks.NewMockIEvaluatorCallbackDispatcher(ctrl)

		auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
		asyncRepo.EXPECT().GetEvalAsyncCtx(gomock.Any(), "evaluator:100").
			Return(&entity.EvalAsyncCtx{RecordID: 100, EvaluatorVersionID: 7, CallbackURL: "https://cb"}, nil)
		evaluatorSvc.EXPECT().ReportEvaluatorInvokeResult(gomock.Any(), gomock.Any()).Return(nil)
		dispatcher.EXPECT().Dispatch(gomock.Any(), int64(1), "https://cb", gomock.Any()).
			DoAndReturn(func(_ context.Context, _ int64, url string, p *entity.EvaluatorCallbackPayload) error {
				assert.Equal(t, int64(100), p.InvokeID)
				assert.Equal(t, "success", p.Status)
				return nil
			})

		app := &EvalOpenAPIApplication{auth: auth, asyncRepo: asyncRepo, evaluatorService: evaluatorSvc, callbackDispatcher: dispatcher}
		_, err := app.ReportEvaluatorInvokeResult_(context.Background(), &openapi.ReportEvaluatorInvokeResultRequest{
			WorkspaceID: gptr.Of(int64(1)),
			InvokeID:    gptr.Of(int64(100)),
			Status:      spi.InvokeEvaluatorRunStatus_SUCCESS.Ptr(),
		})
		assert.NoError(t, err)
	})

	t.Run("no callback url -> dispatch not called", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		auth := rpcmocks.NewMockIAuthProvider(ctrl)
		asyncRepo := repomocks.NewMockIEvalAsyncRepo(ctrl)
		evaluatorSvc := servicemocks.NewMockEvaluatorService(ctrl)
		dispatcher := servicemocks.NewMockIEvaluatorCallbackDispatcher(ctrl)

		auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
		asyncRepo.EXPECT().GetEvalAsyncCtx(gomock.Any(), "evaluator:100").
			Return(&entity.EvalAsyncCtx{RecordID: 100, CallbackURL: ""}, nil)
		evaluatorSvc.EXPECT().ReportEvaluatorInvokeResult(gomock.Any(), gomock.Any()).Return(nil)
		// dispatcher.Dispatch 不设期望 -> 被调用即 fail

		app := &EvalOpenAPIApplication{auth: auth, asyncRepo: asyncRepo, evaluatorService: evaluatorSvc, callbackDispatcher: dispatcher}
		_, err := app.ReportEvaluatorInvokeResult_(context.Background(), &openapi.ReportEvaluatorInvokeResultRequest{
			WorkspaceID: gptr.Of(int64(1)),
			InvokeID:    gptr.Of(int64(100)),
			Status:      spi.InvokeEvaluatorRunStatus_SUCCESS.Ptr(),
		})
		assert.NoError(t, err)
	})

	t.Run("dispatch error -> report still succeeds", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		auth := rpcmocks.NewMockIAuthProvider(ctrl)
		asyncRepo := repomocks.NewMockIEvalAsyncRepo(ctrl)
		evaluatorSvc := servicemocks.NewMockEvaluatorService(ctrl)
		dispatcher := servicemocks.NewMockIEvaluatorCallbackDispatcher(ctrl)

		auth.EXPECT().Authorization(gomock.Any(), gomock.Any()).Return(nil)
		asyncRepo.EXPECT().GetEvalAsyncCtx(gomock.Any(), "evaluator:100").
			Return(&entity.EvalAsyncCtx{RecordID: 100, CallbackURL: "https://cb"}, nil)
		evaluatorSvc.EXPECT().ReportEvaluatorInvokeResult(gomock.Any(), gomock.Any()).Return(nil)
		dispatcher.EXPECT().Dispatch(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(assert.AnError)

		app := &EvalOpenAPIApplication{auth: auth, asyncRepo: asyncRepo, evaluatorService: evaluatorSvc, callbackDispatcher: dispatcher}
		_, err := app.ReportEvaluatorInvokeResult_(context.Background(), &openapi.ReportEvaluatorInvokeResultRequest{
			WorkspaceID: gptr.Of(int64(1)),
			InvokeID:    gptr.Of(int64(100)),
			Status:      spi.InvokeEvaluatorRunStatus_SUCCESS.Ptr(),
		})
		assert.NoError(t, err)
	})
}
```

> 说明：实际 import 别名（`rpcmocks` / `repomocks` / `servicemocks` / `spi` / `gptr` / `openapi` / `entity`）以文件顶部既有 import 为准，按既有用例的写法对齐。若 `spi.InvokeEvaluatorRunStatus_SUCCESS.Ptr()` 不存在，改用 `gptr.Of(spi.InvokeEvaluatorRunStatus_SUCCESS)`。

- [ ] **Step 3: 运行 application 测试，确认通过**

Run: `cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop/backend && go test ./modules/evaluation/application/ -run "AsyncRunEvaluatorOApi|ReportEvaluatorInvokeResult" -v 2>&1 | tail -30`
Expected: 既有 `AsyncRunEvaluatorOApi` 用例 + 新增 `ReportEvaluatorInvokeResult_Callback` 三个子用例全部 PASS。

- [ ] **Step 4: Commit**

```bash
cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop
git add backend/modules/evaluation/application/eval_openapi_app_test.go
git commit -m "[backend][evaluation] test callback url propagation and dispatch"
```

---

## Task 10: 最终验证

**Files:** 无（仅校验）

- [ ] **Step 1: 全量编译检查（go vet）**

Run: `cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop/backend && go vet ./modules/evaluation/... 2>&1 | grep -v "JobID repeats json tag" | head`
Expected: 无新增报错（已知 `JobID repeats json tag "workspace_id"` 基线警告除外）。

- [ ] **Step 2: 运行 evaluation 模块关联单测**

Run: `cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop/backend && go test ./modules/evaluation/application/ ./modules/evaluation/domain/service/ ./modules/evaluation/domain/entity/ ./modules/evaluation/infra/repo/experiment/redis/convert/ 2>&1 | tail -20`
Expected: 全部 `ok`。

- [ ] **Step 3: 确认 git 身份未被 codegen 污染**

Run: `cd /Users/bytedance/go/src/github.com/coze-dev/coze-loop && git config --get user.name && git config --get user.email`
Expected: `liushengyang` / `liushengyang@bytedance.com`（若 local 被置空，按 Task 2 Step 1 清理）。

---

## 自检清单（写计划后）

- **Spec 覆盖**：§4.1 IDL→Task1；§4.2 codegen→Task2；§4.3 实体→Task3；§4.4 helper→Task4；§4.5 dispatcher→Task6（payload 实体→Task5）；§4.6 提交写 URL→Task7 Step3；§4.7 回报触发→Task7 Step5；§4.8 Wire→Task8；§6 测试→Task3/6/9；§6.4 验证→Task10。全覆盖。
- **类型一致性**：`IEvaluatorCallbackDispatcher.Dispatch(ctx, spaceID int64, callbackURL string, payload *entity.EvaluatorCallbackPayload)` 在 Task6 定义、Task7 调用、Task9 mock 三处签名一致；`EvaluatorCallbackPayload` 字段在 Task5 定义、Task6/7/9 引用一致；状态字符串统一为 `"success"`/`"fail"`。
- **无占位符**：每个代码步骤均含完整代码与确切命令。
