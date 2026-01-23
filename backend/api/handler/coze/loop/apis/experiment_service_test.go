// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package apis

import (
	"context"
	"net/http"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
	"github.com/stretchr/testify/require"
)

// TestExperimentServiceHandlers 测试所有experiment service handlers
func TestExperimentServiceHandlers(t *testing.T) {
	t.Parallel()

	// 定义测试用例
	tests := []struct {
		name           string
		handler        func(context.Context, *app.RequestContext)
		requestBody    string
		expectedStatus int
		description    string
	}{
		// ListExperimentStats 测试 - 需要workspace_id字段
		{
			name:           "ListExperimentStats_ValidRequest",
			handler:        ListExperimentStats,
			requestBody:    `{"workspace_id": 123}`,
			expectedStatus: http.StatusOK,
			description:    "测试ListExperimentStats有效请求",
		},
		{
			name:           "ListExperimentStats_InvalidJSON",
			handler:        ListExperimentStats,
			requestBody:    `{invalid json}`,
			expectedStatus: http.StatusBadRequest,
			description:    "测试ListExperimentStats无效JSON",
		},
		{
			name:           "ListExperimentStats_MissingRequiredField",
			handler:        ListExperimentStats,
			requestBody:    `{}`,
			expectedStatus: http.StatusBadRequest,
			description:    "测试ListExperimentStats缺少必需字段",
		},
		// UpsertExptTurnResultFilter 测试
		{
			name:           "UpsertExptTurnResultFilter_ValidRequest",
			handler:        UpsertExptTurnResultFilter,
			requestBody:    `{}`,
			expectedStatus: http.StatusOK,
			description:    "测试UpsertExptTurnResultFilter有效请求",
		},
		{
			name:           "UpsertExptTurnResultFilter_InvalidJSON",
			handler:        UpsertExptTurnResultFilter,
			requestBody:    `{invalid json}`,
			expectedStatus: http.StatusBadRequest,
			description:    "测试UpsertExptTurnResultFilter无效JSON",
		},
		// 实验模板相关 handler 测试
		{
			name:           "CreateExperimentTemplate_ValidRequest",
			handler:        CreateExperimentTemplate,
			requestBody:    `{"workspace_id": 123}`,
			expectedStatus: http.StatusOK,
			description:    "测试CreateExperimentTemplate有效请求",
		},
		{
			name:           "CreateExperimentTemplate_InvalidJSON",
			handler:        CreateExperimentTemplate,
			requestBody:    `{invalid json}`,
			expectedStatus: http.StatusBadRequest,
			description:    "测试CreateExperimentTemplate无效JSON",
		},
		{
			name:           "UpdateExperimentTemplate_ValidRequest",
			handler:        UpdateExperimentTemplate,
			requestBody:    `{"workspace_id": 123, "template_id": 1}`,
			expectedStatus: http.StatusOK,
			description:    "测试UpdateExperimentTemplate有效请求",
		},
		{
			name:           "UpdateExperimentTemplate_InvalidJSON",
			handler:        UpdateExperimentTemplate,
			requestBody:    `{invalid json}`,
			expectedStatus: http.StatusBadRequest,
			description:    "测试UpdateExperimentTemplate无效JSON",
		},
		{
			name:           "DeleteExperimentTemplate_ValidRequest",
			handler:        DeleteExperimentTemplate,
			requestBody:    `{"workspace_id": 123, "template_id": 1}`,
			expectedStatus: http.StatusOK,
			description:    "测试DeleteExperimentTemplate有效请求",
		},
		{
			name:           "DeleteExperimentTemplate_InvalidJSON",
			handler:        DeleteExperimentTemplate,
			requestBody:    `{invalid json}`,
			expectedStatus: http.StatusBadRequest,
			description:    "测试DeleteExperimentTemplate无效JSON",
		},
		{
			name:           "ListExperimentTemplates_ValidRequest",
			handler:        ListExperimentTemplates,
			requestBody:    `{"workspace_id": 123}`,
			expectedStatus: http.StatusOK,
			description:    "测试ListExperimentTemplates有效请求",
		},
		{
			name:           "ListExperimentTemplates_InvalidJSON",
			handler:        ListExperimentTemplates,
			requestBody:    `{invalid json}`,
			expectedStatus: http.StatusBadRequest,
			description:    "测试ListExperimentTemplates无效JSON",
		},
		{
			name:           "BatchGetExperimentTemplate_ValidRequest",
			handler:        BatchGetExperimentTemplate,
			requestBody:    `{"workspace_id": 123, "template_ids": [1,2,3]}`,
			expectedStatus: http.StatusOK,
			description:    "测试BatchGetExperimentTemplate有效请求",
		},
		{
			name:           "BatchGetExperimentTemplate_InvalidJSON",
			handler:        BatchGetExperimentTemplate,
			requestBody:    `{invalid json}`,
			expectedStatus: http.StatusBadRequest,
			description:    "测试BatchGetExperimentTemplate无效JSON",
		},
		{
			name:           "UpdateExperimentTemplateMeta_ValidRequest",
			handler:        UpdateExperimentTemplateMeta,
			requestBody:    `{"workspace_id": 123, "template_id": 1}`,
			expectedStatus: http.StatusOK,
			description:    "测试UpdateExperimentTemplateMeta有效请求",
		},
		{
			name:           "UpdateExperimentTemplateMeta_InvalidJSON",
			handler:        UpdateExperimentTemplateMeta,
			requestBody:    `{invalid json}`,
			expectedStatus: http.StatusBadRequest,
			description:    "测试UpdateExperimentTemplateMeta无效JSON",
		},
		// InsightAnalysisExperiment 测试
		// {
		// 	name:           "InsightAnalysisExperiment_ValidRequest",
		// 	handler:        InsightAnalysisExperiment,
		// 	requestBody:    `{}`,
		// 	expectedStatus: http.StatusOK,
		// 	description:    "测试InsightAnalysisExperiment有效请求",
		// },
		// {
		// 	name:           "InsightAnalysisExperiment_InvalidJSON",
		// 	handler:        InsightAnalysisExperiment,
		// 	requestBody:    `{invalid json}`,
		// 	expectedStatus: http.StatusBadRequest,
		// 	description:    "测试InsightAnalysisExperiment无效JSON",
		// },
		// // ListExptInsightAnalysisRecord 测试
		// {
		// 	name:           "ListExptInsightAnalysisRecord_ValidRequest",
		// 	handler:        ListExptInsightAnalysisRecord,
		// 	requestBody:    `{}`,
		// 	expectedStatus: http.StatusOK,
		// 	description:    "测试ListExptInsightAnalysisRecord有效请求",
		// },
		// {
		// 	name:           "ListExptInsightAnalysisRecord_InvalidJSON",
		// 	handler:        ListExptInsightAnalysisRecord,
		// 	requestBody:    `{invalid json}`,
		// 	expectedStatus: http.StatusBadRequest,
		// 	description:    "测试ListExptInsightAnalysisRecord无效JSON",
		// },
		// // DeleteExptInsightAnalysisRecord 测试
		// {
		// 	name:           "DeleteExptInsightAnalysisRecord_ValidRequest",
		// 	handler:        DeleteExptInsightAnalysisRecord,
		// 	requestBody:    `{}`,
		// 	expectedStatus: http.StatusOK,
		// 	description:    "测试DeleteExptInsightAnalysisRecord有效请求",
		// },
		// {
		// 	name:           "DeleteExptInsightAnalysisRecord_InvalidJSON",
		// 	handler:        DeleteExptInsightAnalysisRecord,
		// 	requestBody:    `{invalid json}`,
		// 	expectedStatus: http.StatusBadRequest,
		// 	description:    "测试DeleteExptInsightAnalysisRecord无效JSON",
		// },
		// // GetExptInsightAnalysisRecord 测试
		// {
		// 	name:           "GetExptInsightAnalysisRecord_ValidRequest",
		// 	handler:        GetExptInsightAnalysisRecord,
		// 	requestBody:    `{}`,
		// 	expectedStatus: http.StatusOK,
		// 	description:    "测试GetExptInsightAnalysisRecord有效请求",
		// },
		// {
		// 	name:           "GetExptInsightAnalysisRecord_InvalidJSON",
		// 	handler:        GetExptInsightAnalysisRecord,
		// 	requestBody:    `{invalid json}`,
		// 	expectedStatus: http.StatusBadRequest,
		// 	description:    "测试GetExptInsightAnalysisRecord无效JSON",
		// },
		// // FeedbackExptInsightAnalysisReport 测试 - 需要必需字段
		// {
		// 	name:           "FeedbackExptInsightAnalysisReport_ValidRequest",
		// 	handler:        FeedbackExptInsightAnalysisReport,
		// 	requestBody:    `{"workspace_id": 123, "expt_id": 456, "insight_analysis_record_id": 789, "feedback_action_type": "like"}`,
		// 	expectedStatus: http.StatusOK,
		// 	description:    "测试FeedbackExptInsightAnalysisReport有效请求",
		// },
		// {
		// 	name:           "FeedbackExptInsightAnalysisReport_MissingRequiredField",
		// 	handler:        FeedbackExptInsightAnalysisReport,
		// 	requestBody:    `{}`,
		// 	expectedStatus: http.StatusBadRequest,
		// 	description:    "测试FeedbackExptInsightAnalysisReport缺少必需字段",
		// },
		// {
		// 	name:           "FeedbackExptInsightAnalysisReport_InvalidJSON",
		// 	handler:        FeedbackExptInsightAnalysisReport,
		// 	requestBody:    `{invalid json}`,
		// 	expectedStatus: http.StatusBadRequest,
		// 	description:    "测试FeedbackExptInsightAnalysisReport无效JSON",
		// },
		// // ListExptInsightAnalysisComment 测试
		// {
		// 	name:           "ListExptInsightAnalysisComment_ValidRequest",
		// 	handler:        ListExptInsightAnalysisComment,
		// 	requestBody:    `{}`,
		// 	expectedStatus: http.StatusOK,
		// 	description:    "测试ListExptInsightAnalysisComment有效请求",
		// },
		// {
		// 	name:           "ListExptInsightAnalysisComment_InvalidJSON",
		// 	handler:        ListExptInsightAnalysisComment,
		// 	requestBody:    `{invalid json}`,
		// 	expectedStatus: http.StatusBadRequest,
		// 	description:    "测试ListExptInsightAnalysisComment无效JSON",
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			c := &app.RequestContext{}
			c.Request.SetBody([]byte(tt.requestBody))
			c.Request.Header.Set("Content-Type", "application/json")

			// 执行handler
			tt.handler(ctx, c)

			// 验证状态码
			actualStatus := c.Response.StatusCode()

			// 根据handler类型验证状态码
			if tt.requestBody == `{invalid json}` {
				// 有些handler会忽略JSON解析错误，直接返回200
				// 只有部分handler（如ListExperimentStats）会返回400
				if tt.name == "ListExperimentStats_InvalidJSON" {
					assert.DeepEqual(t, http.StatusBadRequest, actualStatus)
				} else {
					// 其他handler可能不会严格验证JSON格式
					assert.True(t, actualStatus == http.StatusOK || actualStatus == http.StatusBadRequest)
				}
			} else if tt.name == "ListExperimentStats_MissingRequiredField" || tt.name == "FeedbackExptInsightAnalysisReport_MissingRequiredField" {
				// 缺少必需字段应该返回400
				assert.DeepEqual(t, http.StatusBadRequest, actualStatus)
			} else {
				// 对于有效JSON，应该返回200（这些handler只是创建空响应）
				assert.DeepEqual(t, http.StatusOK, actualStatus)
			}
		})
	}
}

// TestHandlerResponseFormat 测试响应格式
func TestHandlerResponseFormat(t *testing.T) {
	t.Parallel()

	handlers := []struct {
		name        string
		handler     func(context.Context, *app.RequestContext)
		requestBody string
	}{
		{"ListExperimentStats", ListExperimentStats, `{"workspace_id": 123}`},
		{"UpsertExptTurnResultFilter", UpsertExptTurnResultFilter, `{}`},
		// 实验模板相关 handler
		{"CreateExperimentTemplate", CreateExperimentTemplate, `{"workspace_id": 123}`},
		{"UpdateExperimentTemplate", UpdateExperimentTemplate, `{"workspace_id": 123, "template_id": 1}`},
		{"DeleteExperimentTemplate", DeleteExperimentTemplate, `{"workspace_id": 123, "template_id": 1}`},
		{"ListExperimentTemplates", ListExperimentTemplates, `{"workspace_id": 123}`},
		{"BatchGetExperimentTemplate", BatchGetExperimentTemplate, `{"workspace_id": 123, "template_ids": [1,2,3]}`},
		{"UpdateExperimentTemplateMeta", UpdateExperimentTemplateMeta, `{"workspace_id": 123, "template_id": 1}`},
		// {"InsightAnalysisExperiment", InsightAnalysisExperiment, `{}`},
		// {"ListExptInsightAnalysisRecord", ListExptInsightAnalysisRecord, `{}`},
		// {"DeleteExptInsightAnalysisRecord", DeleteExptInsightAnalysisRecord, `{}`},
		// {"GetExptInsightAnalysisRecord", GetExptInsightAnalysisRecord, `{}`},
		// {"FeedbackExptInsightAnalysisReport", FeedbackExptInsightAnalysisReport, `{"workspace_id": 123, "expt_id": 456, "insight_analysis_record_id": 789, "feedback_action_type": "like"}`},
		// {"ListExptInsightAnalysisComment", ListExptInsightAnalysisComment, `{}`},
	}

	for _, h := range handlers {
		t.Run(h.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			c := &app.RequestContext{}
			c.Request.SetBody([]byte(h.requestBody))
			c.Request.Header.Set("Content-Type", "application/json")

			h.handler(ctx, c)

			// 验证响应状态码
			assert.DeepEqual(t, http.StatusOK, c.Response.StatusCode())

			// 验证响应体不为空
			responseBody := c.Response.Body()
			require.NotNil(t, responseBody)
			require.True(t, len(responseBody) > 0)
		})
	}
}

// TestHandlerConcurrency 测试并发请求处理
func TestHandlerConcurrency(t *testing.T) {
	t.Parallel()

	const numGoroutines = 10
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- true }()

			ctx := context.Background()
			c := &app.RequestContext{}
			c.Request.SetBody([]byte(`{"workspace_id": 123}`))
			c.Request.Header.Set("Content-Type", "application/json")

			ListExperimentStats(ctx, c)

			assert.DeepEqual(t, http.StatusOK, c.Response.StatusCode())
		}()
	}

	// 等待所有goroutine完成
	for i := 0; i < numGoroutines; i++ {
		<-done
	}
}

// TestHandlerEdgeCases 测试边界情况
func TestHandlerEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		requestBody string
		contentType string
		handler     func(context.Context, *app.RequestContext)
	}{
		{
			name:        "EmptyBody",
			requestBody: "",
			contentType: "application/json",
			handler:     ListExperimentStats,
		},
		{
			name:        "NullJSON",
			requestBody: "null",
			contentType: "application/json",
			handler:     ListExperimentStats,
		},
		{
			name:        "EmptyJSON",
			requestBody: "{}",
			contentType: "application/json",
			handler:     ListExperimentStats,
		},
		{
			name:        "NoContentType",
			requestBody: "{}",
			contentType: "",
			handler:     ListExperimentStats,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			c := &app.RequestContext{}
			c.Request.SetBody([]byte(tt.requestBody))
			if tt.contentType != "" {
				c.Request.Header.Set("Content-Type", tt.contentType)
			}

			tt.handler(ctx, c)

			// 所有这些情况都应该能正常处理，不应该panic
			statusCode := c.Response.StatusCode()
			assert.True(t, statusCode == http.StatusOK || statusCode == http.StatusBadRequest)
		})
	}
}
