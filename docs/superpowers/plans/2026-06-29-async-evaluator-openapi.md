# 异步评估器 OpenAPI 调用 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增 OpenAPI 端点 `AsyncRunEvaluatorOApi`，允许外部调用方异步提交评估器版本执行并立即返回 `invoke_id` 与 `AsyncInvoking` 记录。

**Architecture:** 复用已有的异步评估器基础设施——领域服务 `EvaluatorService.AsyncRunEvaluator`、Redis 异步上下文 `IEvalAsyncRepo.SetEvalAsyncCtx`、记录状态 `EvaluatorRunStatusAsyncInvoking`，以及已存在的入站回调端点 `ReportEvaluatorInvokeResult`。本计划在 IDL 新增结构与方法（纯新增，向前兼容），重新生成代码，并在 `EvalOpenAPIApplication` 新增一个薄的应用层方法。结果检索复用既有 `BatchGetEvaluatorRecordsOApi` 轮询。

**Tech Stack:** Go、CloudWeGo Thrift IDL（Kitex/Hertz codegen，脚本位于 `backend/script/cloudwego/`）、gomock 单元测试、`go vet` 编译检查（遵循 CLAUDE.md，禁用 `go build`）。

**设计文档：** `docs/superpowers/specs/2026-06-29-async-evaluator-openapi-design.md`

---

## 背景：关键既有代码（实现者必读）

实现者对本代码库零上下文，以下是必须了解的既有事实：

- **同步端点（镜像对象）**：`backend/modules/evaluation/application/eval_openapi_app.go:1911` 的
  `func (e *EvalOpenAPIApplication) RunEvaluatorOApi(ctx, req *openapi.RunEvaluatorOApiRequest)`。
  新方法的鉴权/转换逻辑与它完全一致，只把执行步骤从 `RunEvaluator` 换成异步路径。
- **内部异步提交（逻辑模板）**：`backend/modules/evaluation/application/evaluator_app.go:2091` 的
  `EvaluatorHandlerImpl.AsyncRunEvaluator`。它示范了「调用 `AsyncRunEvaluator` → `SetEvalAsyncCtx`」的写法。
- **领域服务签名**：`backend/modules/evaluation/domain/service/evaluator.go`：
  `AsyncRunEvaluator(ctx, *entity.AsyncRunEvaluatorRequest) (*entity.EvaluatorRecord, error)`（第 39 行）。
  实现位于 `evaluator_impl.go:914`，返回 **完整的** `*entity.EvaluatorRecord`（`ID = invokeID`，
  `Status = EvaluatorRunStatusAsyncInvoking`）。内部已做 Agent 类型校验（非 Agent 返回
  `InvalidEvaluatorTypeCode "async run only supports Agent evaluator type"`）与 workspace 校验。
- **请求实体**：`backend/modules/evaluation/domain/entity/param.go:247` 的 `AsyncRunEvaluatorRequest`
  字段：`SpaceID, Name, EvaluatorVersionID, InputData, ExperimentID, ExperimentRunID, ItemID, TurnID,
  Ext, EvaluatorRunConf`。独立调用时实验相关字段留零值。
- **异步上下文实体**：`EvalAsyncCtx`（`expt_run.go:577`）字段：`Event, RecordID, AsyncUnixMS, Session,
  Callee, EvaluatorVersionID, EnableExtractTrajectory`。独立调用 `Event = nil`（回调端点
  `eval_openapi_app.go:2523` 已 `if actx.Event != nil` 守卫）。
- **异步上下文 key 格式**：`fmt.Sprintf("evaluator:%d", recordID)`（见 evaluator_app.go:2115）。
- **应用结构体已具备依赖**：`EvalOpenAPIApplication`（`eval_openapi_app.go:48`）已含字段
  `asyncRepo repo.IEvalAsyncRepo`（第 50 行）、`evaluatorService`、`auth`、`metric`。**构造函数无需改动。**
- **转换器**：`backend/modules/evaluation/application/convertor/evaluator/openapi.go`：
  `OpenAPIEvaluatorInputDataDTO2DO`(375)、`OpenAPIEvaluatorRunConfigDTO2DO`(387)、
  `OpenAPIEvaluatorRecordDO2DTO`(255)。
- **常量**：`consts.FieldAdapterBuiltinFieldNameRuntimeParam`、`consts.Read`。
- **session**：`session.UserIDInCtxOrEmpty(ctx)`。

## 文件结构

| 文件 | 责任 | 操作 |
| --- | --- | --- |
| `idl/thrift/coze/loop/evaluation/coze.loop.evaluation.openapi.thrift` | 契约：新增请求/响应结构 + service 方法 | 修改 |
| `backend/kitex_gen/...`, `backend/loop_gen/...`, `backend/api/handler/...`, `backend/api/router_gen.go`/`coze.loop.apis.go` | 生成代码 | 由脚本生成（勿手改） |
| `backend/modules/evaluation/application/eval_openapi_app.go` | 应用层 `AsyncRunEvaluatorOApi` 方法 | 修改 |
| `backend/modules/evaluation/application/eval_openapi_app_test.go` | 单元测试 | 修改 |

> 注意（CLAUDE.md）：本变更**无 DB/Clickhouse/配置改动**，故 release 下 docker/k8s 的表结构与配置均不涉及。

---

## Task 1: IDL 新增异步评估器结构与方法

**Files:**
- Modify: `idl/thrift/coze/loop/evaluation/coze.loop.evaluation.openapi.thrift`（结构插入到第 887 行 `RunEvaluatorOpenAPIData` 之后；service 方法插入到第 1235 行 `RunEvaluatorOApi` 之后）

- [ ] **Step 1: 新增请求/响应/数据结构**

在第 887 行（`struct RunEvaluatorOpenAPIData { ... }` 闭合大括号）之后插入：

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

- [ ] **Step 2: 在 service 中新增方法**

在 `EvaluationOpenAPIService` 内，第 1235 行 `RunEvaluatorOApi(...)` 那一行之后插入：

```thrift
    // 异步执行评估器
    AsyncRunEvaluatorOApiResponse AsyncRunEvaluatorOApi(1: AsyncRunEvaluatorOApiRequest req) (api.category = "openapi", api.post = "/v1/loop/evaluation/evaluators_versions/:evaluator_version_id/async_run")
```

- [ ] **Step 3: 校验 thrift 语法正确（结构闭合、字段编号唯一、无重复 URI）**

人工检查：三个新结构大括号闭合；`AsyncRunEvaluatorOApiRequest` 字段编号 1/2/3/4/100/254/255 与同步版一致；新 URI `.../async_run` 与既有 `.../run` 不冲突。

- [ ] **Step 4: Commit**

```bash
git add idl/thrift/coze/loop/evaluation/coze.loop.evaluation.openapi.thrift
git commit -m "[feat][evaluation] add AsyncRunEvaluatorOApi IDL definition"
```

---

## Task 2: 重新生成后端代码

**Files:**
- Generate（勿手改）: `backend/kitex_gen/...`、`backend/loop_gen/coze/loop/evaluation/loopenapi/local_evaluationopenapiservice.go`、`backend/api/handler/coze/loop/apis/eval_open_apiservice.go`、`backend/api/router/coze/loop/apis/coze.loop.apis.go`（及 `backend/api/router_gen.go` 若涉及）

- [ ] **Step 1: 运行 Kitex 代码生成**

Run:
```bash
cd backend && bash script/cloudwego/kitex_tool.sh
```
Expected: 成功无报错；`backend/kitex_gen/coze/loop/evaluation/openapi` 下出现 `AsyncRunEvaluatorOApiRequest`/`AsyncRunEvaluatorOApiResponse`/`AsyncRunEvaluatorOpenAPIData` 类型。

- [ ] **Step 2: 运行 Hertz 代码生成**

Run:
```bash
cd backend && bash script/cloudwego/hertz_tool.sh
```
Expected: 成功无报错；`backend/api/handler/coze/loop/apis/eval_open_apiservice.go` 新增 `func AsyncRunEvaluatorOApi(ctx, c *app.RequestContext)`；路由文件 `backend/api/router/coze/loop/apis/coze.loop.apis.go` 在 `_evaluator_version_id0` 组下新增 `POST /async_run` 条目。

- [ ] **Step 3: 运行通用代码生成（loop_gen 本地服务包装）**

Run:
```bash
cd backend && bash script/cloudwego/code_gen.sh
```
Expected: 成功无报错；`backend/loop_gen/coze/loop/evaluation/loopenapi/local_evaluationopenapiservice.go` 新增 `func (l *LocalEvaluationOpenAPIService) AsyncRunEvaluatorOApi(...)`，其内部委托 `l.impl.AsyncRunEvaluatorOApi(ctx, arg.Req)`。

- [ ] **Step 4: 验证生成产物存在**

Run:
```bash
cd backend && grep -rl "AsyncRunEvaluatorOApi" kitex_gen loop_gen api/handler api/router | sort -u
```
Expected: 至少列出 kitex_gen、loop_gen、api/handler、api/router 四处文件。

- [ ] **Step 5: 编译检查（此时 impl 尚未实现，期望接口不匹配报错）**

Run:
```bash
cd backend && go vet ./loop_gen/... ./api/... 2>&1 | head -30
```
Expected: 可能因 `EvalOpenAPIApplication` 尚未实现 `AsyncRunEvaluatorOApi` 而报「does not implement」错误——这是预期的，将在 Task 4 解决。若仅此类错误则继续。

- [ ] **Step 6: Commit 生成代码**

```bash
cd .. && git add backend/kitex_gen backend/loop_gen backend/api
git commit -m "[feat][evaluation] regenerate code for AsyncRunEvaluatorOApi"
```

---

## Task 3: 应用层方法——先写失败测试（TDD）

**Files:**
- Test: `backend/modules/evaluation/application/eval_openapi_app_test.go`（在文件末尾 `TestEvalOpenAPIApplication_RunEvaluatorOApi` 之后追加）

实现者注意：本测试模仿既有 `TestEvalOpenAPIApplication_RunEvaluatorOApi`（同文件第 3385 行）。
mock 包别名沿用该文件既有 import：`rpcmocks`（auth）、`servicemocks`（EvaluatorService）、`repomocks`（IEvalAsyncRepo），
`fakeOpenAPIMetric`（同文件第 50 行已定义）。断言风格沿用同文件。

- [ ] **Step 1: 追加失败测试**

在 `eval_openapi_app_test.go` 末尾追加：

```go
func TestEvalOpenAPIApplication_AsyncRunEvaluatorOApi(t *testing.T) {
	t.Parallel()

	workspaceID := int64(1001)
	evaluatorVersionID := int64(3003)
	invokeID := int64(4004)

	tests := []struct {
		name    string
		req     *openapi.AsyncRunEvaluatorOApiRequest
		setup   func(auth *rpcmocks.MockIAuthProvider, evaluatorSvc *servicemocks.MockEvaluatorService, asyncRepo *repomocks.MockIEvalAsyncRepo)
		wantErr int32
	}{
		{
			name:    "nil request",
			req:     nil,
			setup:   func(_ *rpcmocks.MockIAuthProvider, _ *servicemocks.MockEvaluatorService, _ *repomocks.MockIEvalAsyncRepo) {},
			wantErr: errno.CommonInvalidParamCode,
		},
		{
			name: "evaluator version not found",
			req: &openapi.AsyncRunEvaluatorOApiRequest{
				WorkspaceID:        gptr.Of(workspaceID),
				EvaluatorVersionID: gptr.Of(evaluatorVersionID),
			},
			setup: func(_ *rpcmocks.MockIAuthProvider, evaluatorSvc *servicemocks.MockEvaluatorService, _ *repomocks.MockIEvalAsyncRepo) {
				evaluatorSvc.EXPECT().GetEvaluatorVersion(gomock.Any(), gomock.Any(), evaluatorVersionID, false, false).Return(nil, nil)
			},
			wantErr: errno.ResourceNotFoundCode,
		},
		{
			name: "evaluator version not found in workspace",
			req: &openapi.AsyncRunEvaluatorOApiRequest{
				WorkspaceID:        gptr.Of(workspaceID),
				EvaluatorVersionID: gptr.Of(evaluatorVersionID),
			},
			setup: func(_ *rpcmocks.MockIAuthProvider, evaluatorSvc *servicemocks.MockEvaluatorService, _ *repomocks.MockIEvalAsyncRepo) {
				evaluator := &entity.Evaluator{ID: evaluatorVersionID, SpaceID: workspaceID + 1, Builtin: false}
				evaluatorSvc.EXPECT().GetEvaluatorVersion(gomock.Any(), gomock.Any(), evaluatorVersionID, false, false).Return(evaluator, nil)
			},
			wantErr: errno.ResourceNotFoundCode,
		},
		{
			name: "auth failed",
			req: &openapi.AsyncRunEvaluatorOApiRequest{
				WorkspaceID:        gptr.Of(workspaceID),
				EvaluatorVersionID: gptr.Of(evaluatorVersionID),
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evaluatorSvc *servicemocks.MockEvaluatorService, _ *repomocks.MockIEvalAsyncRepo) {
				evaluator := &entity.Evaluator{
					ID: evaluatorVersionID, SpaceID: workspaceID,
					BaseInfo: &entity.BaseInfo{CreatedBy: &entity.UserInfo{UserID: gptr.Of("owner")}},
				}
				evaluatorSvc.EXPECT().GetEvaluatorVersion(gomock.Any(), gomock.Any(), evaluatorVersionID, false, false).Return(evaluator, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(errorx.NewByCode(errno.CommonNoPermissionCode))
			},
			wantErr: errno.CommonNoPermissionCode,
		},
		{
			name: "async run failed (e.g. non-agent type)",
			req: &openapi.AsyncRunEvaluatorOApiRequest{
				WorkspaceID:        gptr.Of(workspaceID),
				EvaluatorVersionID: gptr.Of(evaluatorVersionID),
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evaluatorSvc *servicemocks.MockEvaluatorService, _ *repomocks.MockIEvalAsyncRepo) {
				evaluator := &entity.Evaluator{
					ID: evaluatorVersionID, SpaceID: workspaceID,
					BaseInfo: &entity.BaseInfo{CreatedBy: &entity.UserInfo{UserID: gptr.Of("owner")}},
				}
				evaluatorSvc.EXPECT().GetEvaluatorVersion(gomock.Any(), gomock.Any(), evaluatorVersionID, false, false).Return(evaluator, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
				evaluatorSvc.EXPECT().AsyncRunEvaluator(gomock.Any(), gomock.Any()).Return(nil, errors.New("async run failed"))
			},
			wantErr: -1,
		},
		{
			name: "set async ctx failed",
			req: &openapi.AsyncRunEvaluatorOApiRequest{
				WorkspaceID:        gptr.Of(workspaceID),
				EvaluatorVersionID: gptr.Of(evaluatorVersionID),
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evaluatorSvc *servicemocks.MockEvaluatorService, asyncRepo *repomocks.MockIEvalAsyncRepo) {
				evaluator := &entity.Evaluator{
					ID: evaluatorVersionID, SpaceID: workspaceID,
					BaseInfo: &entity.BaseInfo{CreatedBy: &entity.UserInfo{UserID: gptr.Of("owner")}},
				}
				record := &entity.EvaluatorRecord{ID: invokeID, Status: entity.EvaluatorRunStatusAsyncInvoking}
				evaluatorSvc.EXPECT().GetEvaluatorVersion(gomock.Any(), gomock.Any(), evaluatorVersionID, false, false).Return(evaluator, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
				evaluatorSvc.EXPECT().AsyncRunEvaluator(gomock.Any(), gomock.Any()).Return(record, nil)
				asyncRepo.EXPECT().SetEvalAsyncCtx(gomock.Any(), gomock.Any(), gomock.Any()).Return(errors.New("redis error"))
			},
			wantErr: -1,
		},
		{
			name: "success",
			req: &openapi.AsyncRunEvaluatorOApiRequest{
				WorkspaceID:        gptr.Of(workspaceID),
				EvaluatorVersionID: gptr.Of(evaluatorVersionID),
			},
			setup: func(auth *rpcmocks.MockIAuthProvider, evaluatorSvc *servicemocks.MockEvaluatorService, asyncRepo *repomocks.MockIEvalAsyncRepo) {
				evaluator := &entity.Evaluator{
					ID: evaluatorVersionID, SpaceID: workspaceID, Name: "agent-eval",
					BaseInfo: &entity.BaseInfo{CreatedBy: &entity.UserInfo{UserID: gptr.Of("owner")}},
				}
				record := &entity.EvaluatorRecord{ID: invokeID, Status: entity.EvaluatorRunStatusAsyncInvoking}
				evaluatorSvc.EXPECT().GetEvaluatorVersion(gomock.Any(), gomock.Any(), evaluatorVersionID, false, false).Return(evaluator, nil)
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(nil)
				evaluatorSvc.EXPECT().AsyncRunEvaluator(gomock.Any(), gomock.Any()).Return(record, nil)
				asyncRepo.EXPECT().SetEvalAsyncCtx(gomock.Any(), "evaluator:4004", gomock.Any()).Return(nil)
			},
		},
		{
			name: "builtin success",
			req: &openapi.AsyncRunEvaluatorOApiRequest{
				WorkspaceID:        gptr.Of(workspaceID),
				EvaluatorVersionID: gptr.Of(evaluatorVersionID),
			},
			setup: func(_ *rpcmocks.MockIAuthProvider, evaluatorSvc *servicemocks.MockEvaluatorService, asyncRepo *repomocks.MockIEvalAsyncRepo) {
				evaluator := &entity.Evaluator{ID: evaluatorVersionID, SpaceID: workspaceID + 999, Builtin: true, Name: "builtin-agent"}
				record := &entity.EvaluatorRecord{ID: invokeID, Status: entity.EvaluatorRunStatusAsyncInvoking}
				evaluatorSvc.EXPECT().GetEvaluatorVersion(gomock.Any(), gomock.Any(), evaluatorVersionID, false, false).Return(evaluator, nil)
				evaluatorSvc.EXPECT().AsyncRunEvaluator(gomock.Any(), gomock.Any()).Return(record, nil)
				asyncRepo.EXPECT().SetEvalAsyncCtx(gomock.Any(), "evaluator:4004", gomock.Any()).Return(nil)
			},
		},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			auth := rpcmocks.NewMockIAuthProvider(ctrl)
			evaluatorSvc := servicemocks.NewMockEvaluatorService(ctrl)
			asyncRepo := repomocks.NewMockIEvalAsyncRepo(ctrl)
			metric := &fakeOpenAPIMetric{}

			app := &EvalOpenAPIApplication{
				auth:             auth,
				evaluatorService: evaluatorSvc,
				asyncRepo:        asyncRepo,
				metric:           metric,
			}

			if tc.name == "nil request" {
				auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Times(0)
				evaluatorSvc.EXPECT().GetEvaluatorVersion(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
				evaluatorSvc.EXPECT().AsyncRunEvaluator(gomock.Any(), gomock.Any()).Times(0)
			} else {
				tc.setup(auth, evaluatorSvc, asyncRepo)
			}

			resp, err := app.AsyncRunEvaluatorOApi(context.Background(), tc.req)

			if tc.wantErr != 0 {
				assert.Error(t, err)
				if tc.wantErr > 0 {
					statusErr, ok := errorx.FromStatusError(err)
					assert.True(t, ok)
					assert.Equal(t, tc.wantErr, statusErr.Code())
				}
				assert.Nil(t, resp)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, resp)
				assert.Equal(t, invokeID, resp.GetData().GetInvokeID())
				assert.NotNil(t, resp.GetData().GetRecord())
			}

			if tc.req != nil {
				assert.True(t, metric.called)
				assert.Equal(t, tc.req.GetWorkspaceID(), metric.spaceID)
				assert.Equal(t, tc.req.GetEvaluatorVersionID(), metric.evaluationSetID)
			}
		})
	}
}
```

- [ ] **Step 2: 运行测试，确认编译失败（方法尚未实现）**

Run:
```bash
cd backend && go test ./modules/evaluation/application/ -run TestEvalOpenAPIApplication_AsyncRunEvaluatorOApi 2>&1 | head -20
```
Expected: 编译失败，提示 `app.AsyncRunEvaluatorOApi undefined`（方法尚未实现）。这是预期的红灯。

- [ ] **Step 3: Commit 失败测试**

```bash
cd .. && git add backend/modules/evaluation/application/eval_openapi_app_test.go
git commit -m "[test][evaluation] add failing test for AsyncRunEvaluatorOApi"
```

---

## Task 4: 实现应用层 `AsyncRunEvaluatorOApi`

**Files:**
- Modify: `backend/modules/evaluation/application/eval_openapi_app.go`（在 `RunEvaluatorOApi` 方法即第 1980 行之后插入新方法）

实现者注意：所需 import（`fmt`、`time`、`strconv`、`gptr`、`session`、`consts`、`entity`、`errorx`、`errno`、`rpc`、`evaluator_convertor`、`openapi`、`kitexutil`）在 `eval_openapi_app.go` 中均已存在（`RunEvaluatorOApi` 与 `ReportEvaluatorInvokeResult_` 已使用）。无需新增 import。

- [ ] **Step 1: 实现方法**

在 `eval_openapi_app.go` 第 1980 行 `RunEvaluatorOApi` 方法闭合大括号之后插入：

```go
func (e *EvalOpenAPIApplication) AsyncRunEvaluatorOApi(ctx context.Context, req *openapi.AsyncRunEvaluatorOApiRequest) (r *openapi.AsyncRunEvaluatorOApiResponse, err error) {
	if req == nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("req is nil"))
	}
	startTime := time.Now()
	defer func() {
		e.metric.EmitOpenAPIMetric(ctx, req.GetWorkspaceID(), req.GetEvaluatorVersionID(), kitexutil.GetTOMethod(ctx), startTime.UnixMilli(), err)
	}()

	// 校验评估器版本是否存在且有权限
	// 预置评估器（Builtin）允许跨 workspace 执行：查询时不传 spaceID
	evaluator, err := e.evaluatorService.GetEvaluatorVersion(ctx, nil, req.GetEvaluatorVersionID(), false, false)
	if err != nil {
		return nil, err
	}
	if evaluator == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("evaluator version not found"))
	}

	if !evaluator.Builtin {
		if evaluator.SpaceID != req.GetWorkspaceID() {
			return nil, errorx.NewByCode(errno.ResourceNotFoundCode, errorx.WithExtraMsg("evaluator version not found"))
		}

		var ownerID *string
		if evaluator.BaseInfo != nil && evaluator.BaseInfo.CreatedBy != nil {
			ownerID = evaluator.BaseInfo.CreatedBy.UserID
		}
		err = e.auth.AuthorizationWithoutSPI(ctx, &rpc.AuthorizationWithoutSPIParam{
			ObjectID:        strconv.FormatInt(evaluator.ID, 10),
			SpaceID:         req.GetWorkspaceID(),
			ActionObjects:   []*rpc.ActionObject{{Action: gptr.Of(consts.Read), EntityType: gptr.Of(rpc.AuthEntityType_Evaluator)}},
			OwnerID:         ownerID,
			ResourceSpaceID: evaluator.SpaceID,
		})
		if err != nil {
			return nil, err
		}
	}

	inputData := evaluator_convertor.OpenAPIEvaluatorInputDataDTO2DO(req.InputData)
	runConf := evaluator_convertor.OpenAPIEvaluatorRunConfigDTO2DO(req.EvaluatorRunConf)
	// 与同步 RunEvaluatorOApi 一致：将 evaluator_runtime_param 注入到 InputData.Ext，供下游执行时使用
	if runConf != nil && runConf.EvaluatorRuntimeParam != nil && runConf.EvaluatorRuntimeParam.JSONValue != nil && len(*runConf.EvaluatorRuntimeParam.JSONValue) > 0 {
		if inputData == nil {
			inputData = &entity.EvaluatorInputData{}
		}
		if inputData.Ext == nil {
			inputData.Ext = make(map[string]string)
		}
		inputData.Ext[consts.FieldAdapterBuiltinFieldNameRuntimeParam] = *runConf.EvaluatorRuntimeParam.JSONValue
	}

	// 异步提交：评估器类型限制（仅 Agent）由领域层 AsyncRunEvaluator 继承
	record, err := e.evaluatorService.AsyncRunEvaluator(ctx, &entity.AsyncRunEvaluatorRequest{
		SpaceID:            req.GetWorkspaceID(),
		Name:               evaluator.Name,
		EvaluatorVersionID: req.GetEvaluatorVersionID(),
		InputData:          inputData,
		EvaluatorRunConf:   runConf,
		Ext:                req.Ext,
	})
	if err != nil {
		return nil, err
	}

	// 写入异步上下文，供 ReportEvaluatorInvokeResult 回调读取。独立调用无实验需恢复，Event 留空。
	asyncCtxKey := fmt.Sprintf("evaluator:%d", record.ID)
	if err = e.asyncRepo.SetEvalAsyncCtx(ctx, asyncCtxKey, &entity.EvalAsyncCtx{
		RecordID:           record.ID,
		AsyncUnixMS:        startTime.UnixMilli(),
		Session:            &entity.Session{UserID: session.UserIDInCtxOrEmpty(ctx)},
		EvaluatorVersionID: req.GetEvaluatorVersionID(),
	}); err != nil {
		logs.CtxError(ctx, "[AsyncRunEvaluatorOApi] SetEvalAsyncCtx fail, invokeID: %d, err: %v", record.ID, err)
		return nil, err
	}

	return &openapi.AsyncRunEvaluatorOApiResponse{
		Data: &openapi.AsyncRunEvaluatorOpenAPIData{
			InvokeID: gptr.Of(record.ID),
			Record:   evaluator_convertor.OpenAPIEvaluatorRecordDO2DTO(record),
		},
	}, nil
}
```

> 实现者校验点：
> - `logs` 包在 `eval_openapi_app.go` 已 import（`ReportEvaluatorInvokeResult_` 使用 `logs.CtxInfo`）。
> - `EmitOpenAPIMetric` 第 5 参为 `startTime int64`（毫秒），故传 `startTime.UnixMilli()`（同步版用 `time.Now().UnixNano()/int64(time.Millisecond)`，等价毫秒）。若编译期发现签名为纳秒，改用 `startTime.UnixNano()/int64(time.Millisecond)` 保持与同步一致。
> - 生成的 getter 命名以 kitex 产物为准：若 `resp.GetData().GetInvokeID()` / `GetRecord()` 不存在，按生成代码实际 getter 调整测试断言（Task 3 Step 1）。

- [ ] **Step 2: 运行新测试，确认通过**

Run:
```bash
cd backend && go test ./modules/evaluation/application/ -run TestEvalOpenAPIApplication_AsyncRunEvaluatorOApi -v 2>&1 | tail -30
```
Expected: 全部子用例 PASS（nil request / not found / not found in workspace / auth failed / async run failed / set async ctx failed / success / builtin success）。

- [ ] **Step 3: 编译检查整个模块与生成代码（go vet，遵循 CLAUDE.md）**

Run:
```bash
cd backend && go vet ./modules/evaluation/... ./loop_gen/... ./api/... 2>&1 | head -30
```
Expected: 无错误输出（之前 Task 2 Step 5 的「does not implement」错误此时消失）。

- [ ] **Step 4: 运行 application 包既有全部测试，确认无回归**

Run:
```bash
cd backend && go test ./modules/evaluation/application/ 2>&1 | tail -20
```
Expected: `ok` / 全部 PASS。

- [ ] **Step 5: Commit 实现**

```bash
cd .. && git add backend/modules/evaluation/application/eval_openapi_app.go
git commit -m "[feat][evaluation] implement AsyncRunEvaluatorOApi async OpenAPI endpoint"
```

---

## Task 5: 收尾验证

- [ ] **Step 1: 全量编译检查（go vet）**

Run:
```bash
cd backend && go vet ./... 2>&1 | head -40
```
Expected: 无错误。若有与本次无关的既有告警，确认非本次引入即可。

- [ ] **Step 2: 确认无遗漏的生成产物未提交**

Run:
```bash
git status --short
```
Expected: 工作区干净（所有 idl / 生成代码 / 应用代码 / 测试均已提交）。

- [ ] **Step 3: 人工对照设计文档勾验**

逐条核对 `docs/superpowers/specs/2026-06-29-async-evaluator-openapi-design.md` 第 7 节「影响文件清单」：IDL、生成代码、应用方法、单测均已落实；无 DB/Clickhouse/配置改动。

---

## Self-Review 记录

- **Spec 覆盖**：IDL（设计 4.1 → Task 1）、代码生成（4.2 → Task 2）、应用层方法含 Event=nil 与 Agent-only 继承（4.3 → Task 3/4）、错误处理矩阵（第 5 节 → Task 3 各子用例）、结果检索为既有端点无需改码（4.4 → 无任务，已在背景说明）。全部覆盖。
- **Placeholder 扫描**：无 TBD/TODO；所有代码步骤含完整代码与确切命令、预期输出。
- **类型一致性**：`AsyncRunEvaluatorOApiRequest/Response/OpenAPIData`、`entity.AsyncRunEvaluatorRequest`（字段名核对自 param.go:247）、`entity.EvalAsyncCtx`（字段名核对自 expt_run.go:577）、`EvaluatorRunStatusAsyncInvoking`、key 格式 `evaluator:%d`、转换器函数名均与既有代码一致。
- **风险标注**：`EmitOpenAPIMetric` 时间单位、生成 getter 命名两处已在 Task 4 Step 1 备注兜底方案。
