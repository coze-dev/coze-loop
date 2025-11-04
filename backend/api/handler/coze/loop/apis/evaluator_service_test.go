// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package apis

import (
	"context"
	"net/http"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/kitex/client/callopt"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/apis/evaluatorservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/evaluator"
)

// mockEvaluatorClient 实现 evaluatorservice.Client 的必要方法，返回空响应以通过渲染
type mockEvaluatorClient struct{}

func (m *mockEvaluatorClient) ListEvaluators(ctx context.Context, req *evaluator.ListEvaluatorsRequest, callOptions ...callopt.Option) (r *evaluator.ListEvaluatorsResponse, err error) {
	return &evaluator.ListEvaluatorsResponse{}, nil
}

func (m *mockEvaluatorClient) BatchGetEvaluators(ctx context.Context, req *evaluator.BatchGetEvaluatorsRequest, callOptions ...callopt.Option) (r *evaluator.BatchGetEvaluatorsResponse, err error) {
	return &evaluator.BatchGetEvaluatorsResponse{}, nil
}

func (m *mockEvaluatorClient) GetEvaluator(ctx context.Context, req *evaluator.GetEvaluatorRequest, callOptions ...callopt.Option) (r *evaluator.GetEvaluatorResponse, err error) {
	return &evaluator.GetEvaluatorResponse{}, nil
}

func (m *mockEvaluatorClient) CreateEvaluator(ctx context.Context, req *evaluator.CreateEvaluatorRequest, callOptions ...callopt.Option) (r *evaluator.CreateEvaluatorResponse, err error) {
	return &evaluator.CreateEvaluatorResponse{}, nil
}

func (m *mockEvaluatorClient) UpdateEvaluator(ctx context.Context, req *evaluator.UpdateEvaluatorRequest, callOptions ...callopt.Option) (r *evaluator.UpdateEvaluatorResponse, err error) {
	return &evaluator.UpdateEvaluatorResponse{}, nil
}

func (m *mockEvaluatorClient) UpdateEvaluatorDraft(ctx context.Context, req *evaluator.UpdateEvaluatorDraftRequest, callOptions ...callopt.Option) (r *evaluator.UpdateEvaluatorDraftResponse, err error) {
	return &evaluator.UpdateEvaluatorDraftResponse{}, nil
}

func (m *mockEvaluatorClient) DeleteEvaluator(ctx context.Context, req *evaluator.DeleteEvaluatorRequest, callOptions ...callopt.Option) (r *evaluator.DeleteEvaluatorResponse, err error) {
	return &evaluator.DeleteEvaluatorResponse{}, nil
}

func (m *mockEvaluatorClient) CheckEvaluatorName(ctx context.Context, req *evaluator.CheckEvaluatorNameRequest, callOptions ...callopt.Option) (r *evaluator.CheckEvaluatorNameResponse, err error) {
	return &evaluator.CheckEvaluatorNameResponse{}, nil
}

func (m *mockEvaluatorClient) ListEvaluatorVersions(ctx context.Context, req *evaluator.ListEvaluatorVersionsRequest, callOptions ...callopt.Option) (r *evaluator.ListEvaluatorVersionsResponse, err error) {
	return &evaluator.ListEvaluatorVersionsResponse{}, nil
}

func (m *mockEvaluatorClient) GetEvaluatorVersion(ctx context.Context, req *evaluator.GetEvaluatorVersionRequest, callOptions ...callopt.Option) (r *evaluator.GetEvaluatorVersionResponse, err error) {
	return &evaluator.GetEvaluatorVersionResponse{}, nil
}

func (m *mockEvaluatorClient) BatchGetEvaluatorVersions(ctx context.Context, req *evaluator.BatchGetEvaluatorVersionsRequest, callOptions ...callopt.Option) (r *evaluator.BatchGetEvaluatorVersionsResponse, err error) {
	return &evaluator.BatchGetEvaluatorVersionsResponse{}, nil
}

func (m *mockEvaluatorClient) SubmitEvaluatorVersion(ctx context.Context, req *evaluator.SubmitEvaluatorVersionRequest, callOptions ...callopt.Option) (r *evaluator.SubmitEvaluatorVersionResponse, err error) {
	return &evaluator.SubmitEvaluatorVersionResponse{}, nil
}

func (m *mockEvaluatorClient) ListTemplates(ctx context.Context, req *evaluator.ListTemplatesRequest, callOptions ...callopt.Option) (r *evaluator.ListTemplatesResponse, err error) {
	return &evaluator.ListTemplatesResponse{}, nil
}

func (m *mockEvaluatorClient) GetTemplateInfo(ctx context.Context, req *evaluator.GetTemplateInfoRequest, callOptions ...callopt.Option) (r *evaluator.GetTemplateInfoResponse, err error) {
	return &evaluator.GetTemplateInfoResponse{}, nil
}

func (m *mockEvaluatorClient) GetDefaultPromptEvaluatorTools(ctx context.Context, req *evaluator.GetDefaultPromptEvaluatorToolsRequest, callOptions ...callopt.Option) (r *evaluator.GetDefaultPromptEvaluatorToolsResponse, err error) {
	return &evaluator.GetDefaultPromptEvaluatorToolsResponse{}, nil
}

func (m *mockEvaluatorClient) RunEvaluator(ctx context.Context, req *evaluator.RunEvaluatorRequest, callOptions ...callopt.Option) (r *evaluator.RunEvaluatorResponse, err error) {
	return &evaluator.RunEvaluatorResponse{}, nil
}

func (m *mockEvaluatorClient) DebugEvaluator(ctx context.Context, req *evaluator.DebugEvaluatorRequest, callOptions ...callopt.Option) (r *evaluator.DebugEvaluatorResponse, err error) {
	return &evaluator.DebugEvaluatorResponse{}, nil
}

func (m *mockEvaluatorClient) BatchDebugEvaluator(ctx context.Context, req *evaluator.BatchDebugEvaluatorRequest, callOptions ...callopt.Option) (r *evaluator.BatchDebugEvaluatorResponse, err error) {
	return &evaluator.BatchDebugEvaluatorResponse{}, nil
}

func (m *mockEvaluatorClient) UpdateEvaluatorRecord(ctx context.Context, req *evaluator.UpdateEvaluatorRecordRequest, callOptions ...callopt.Option) (r *evaluator.UpdateEvaluatorRecordResponse, err error) {
	return &evaluator.UpdateEvaluatorRecordResponse{}, nil
}

func (m *mockEvaluatorClient) GetEvaluatorRecord(ctx context.Context, req *evaluator.GetEvaluatorRecordRequest, callOptions ...callopt.Option) (r *evaluator.GetEvaluatorRecordResponse, err error) {
	return &evaluator.GetEvaluatorRecordResponse{}, nil
}

func (m *mockEvaluatorClient) BatchGetEvaluatorRecords(ctx context.Context, req *evaluator.BatchGetEvaluatorRecordsRequest, callOptions ...callopt.Option) (r *evaluator.BatchGetEvaluatorRecordsResponse, err error) {
	return &evaluator.BatchGetEvaluatorRecordsResponse{}, nil
}

func (m *mockEvaluatorClient) ValidateEvaluator(ctx context.Context, req *evaluator.ValidateEvaluatorRequest, callOptions ...callopt.Option) (r *evaluator.ValidateEvaluatorResponse, err error) {
	return &evaluator.ValidateEvaluatorResponse{}, nil
}

func (m *mockEvaluatorClient) ListTemplatesV2(ctx context.Context, req *evaluator.ListTemplatesV2Request, callOptions ...callopt.Option) (r *evaluator.ListTemplatesV2Response, err error) {
	return &evaluator.ListTemplatesV2Response{}, nil
}

func (m *mockEvaluatorClient) CreateEvaluatorTemplate(ctx context.Context, req *evaluator.CreateEvaluatorTemplateRequest, callOptions ...callopt.Option) (r *evaluator.CreateEvaluatorTemplateResponse, err error) {
	return &evaluator.CreateEvaluatorTemplateResponse{}, nil
}

func (m *mockEvaluatorClient) UpdateEvaluatorTemplate(ctx context.Context, req *evaluator.UpdateEvaluatorTemplateRequest, callOptions ...callopt.Option) (r *evaluator.UpdateEvaluatorTemplateResponse, err error) {
	return &evaluator.UpdateEvaluatorTemplateResponse{}, nil
}

func (m *mockEvaluatorClient) DeleteEvaluatorTemplate(ctx context.Context, req *evaluator.DeleteEvaluatorTemplateRequest, callOptions ...callopt.Option) (r *evaluator.DeleteEvaluatorTemplateResponse, err error) {
	return &evaluator.DeleteEvaluatorTemplateResponse{}, nil
}

func (m *mockEvaluatorClient) DebugBuiltinEvaluator(ctx context.Context, req *evaluator.DebugBuiltinEvaluatorRequest, callOptions ...callopt.Option) (r *evaluator.DebugBuiltinEvaluatorResponse, err error) {
	return &evaluator.DebugBuiltinEvaluatorResponse{}, nil
}

func (m *mockEvaluatorClient) UpdateBuiltinEvaluatorTags(ctx context.Context, req *evaluator.UpdateBuiltinEvaluatorTagsRequest, callOptions ...callopt.Option) (r *evaluator.UpdateBuiltinEvaluatorTagsResponse, err error) {
	return &evaluator.UpdateBuiltinEvaluatorTagsResponse{}, nil
}

func (m *mockEvaluatorClient) ListEvaluatorTags(ctx context.Context, req *evaluator.ListEvaluatorTagsRequest, callOptions ...callopt.Option) (r *evaluator.ListEvaluatorTagsResponse, err error) {
	return &evaluator.ListEvaluatorTagsResponse{}, nil
}

// 确保 mock 实现了接口（编译期检查）
var _ evaluatorservice.Client = (*mockEvaluatorClient)(nil)

// helper：构造简易的 RequestContext，设置 JSON 空对象避免绑定失败
func newJSONCtx() *app.RequestContext {
	c := &app.RequestContext{}
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.SetRequestURI("/")
	c.Request.SetMethod("POST")
	c.Request.SetBody([]byte("{}"))
	return c
}

func newJSONCtxWithBody(body string) *app.RequestContext {
	c := &app.RequestContext{}
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.SetRequestURI("/")
	c.Request.SetMethod("POST")
	c.Request.SetBody([]byte(body))
	return c
}

func TestEvaluatorService_Handlers_Smoke(t *testing.T) {
	// 替换并恢复全局 client
	old := localEvaluatorSvc
	localEvaluatorSvc = &mockEvaluatorClient{}
	t.Cleanup(func() { localEvaluatorSvc = old })

	ctx := context.Background()

	// 选取常用的几个接口做冒烟验证（其余接口同理，均走 invokeAndRender 路径）
	cases := []struct {
		name string
		fn   func(context.Context, *app.RequestContext)
	}{
		{name: "CreateEvaluator", fn: CreateEvaluator},
		{name: "ListEvaluators", fn: ListEvaluators},
		{name: "UpdateEvaluator", fn: UpdateEvaluator},
		{name: "UpdateEvaluatorDraft", fn: UpdateEvaluatorDraft},
		{name: "RunEvaluator", fn: RunEvaluator},
		{name: "DebugEvaluator", fn: DebugEvaluator},
	}

	for _, cs := range cases {
		t.Run(cs.name, func(t *testing.T) {
			var c *app.RequestContext
			switch cs.name {
			case "DebugBuiltinEvaluator":
				// 需要必填: evaluator_id, workspace_id, input_data（提供最小可解析结构）
				c = newJSONCtxWithBody(`{
                    "evaluator_id": 1,
                    "workspace_id": 1,
                    "input_data": {"input_fields": {}}
                }`)
			case "CreateEvaluatorTemplate":
				// 需要必填: evaluator_template
				c = newJSONCtxWithBody(`{
                    "evaluator_template": {}
                }`)
			default:
				c = newJSONCtx()
			}
			cs.fn(ctx, c)
			assert.Equal(t, http.StatusOK, c.Response.StatusCode())
		})
	}
}

func TestEvaluatorService_Handlers(t *testing.T) {
	// 替换并恢复全局 client
	old := localEvaluatorSvc
	localEvaluatorSvc = &mockEvaluatorClient{}
	t.Cleanup(func() { localEvaluatorSvc = old })

	ctx := context.Background()

	cases := []struct {
		name string
		fn   func(context.Context, *app.RequestContext)
	}{
		{name: "ValidateEvaluator", fn: ValidateEvaluator},
		{name: "BatchDebugEvaluator", fn: BatchDebugEvaluator},
		{name: "ListTemplatesV2", fn: ListTemplatesV2},
		{name: "GetTemplateV2", fn: GetTemplateV2},
		{name: "DebugBuiltinEvaluator", fn: DebugBuiltinEvaluator},
		{name: "CreateEvaluatorTemplate", fn: CreateEvaluatorTemplate},
		{name: "UpdateEvaluatorTemplate", fn: UpdateEvaluatorTemplate},
		{name: "DeleteEvaluatorTemplate", fn: DeleteEvaluatorTemplate},
		{name: "ListEvaluatorTags", fn: ListEvaluatorTags},
		{name: "UpdateBuiltinEvaluatorTags", fn: UpdateBuiltinEvaluatorTags},
	}

	for _, cs := range cases {
		t.Run(cs.name, func(t *testing.T) {
			c := newJSONCtx()
			cs.fn(ctx, c)
			assert.Equal(t, http.StatusOK, c.Response.StatusCode())
		})
	}
}
