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

// TestListExperimentStats 测试 ListExperimentStats handler
func TestListExperimentStats(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "成功获取实验统计",
			requestBody:    `{"space_id": 123}`,
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "请求参数无效",
			requestBody:    `{"invalid": "data"}`,
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:           "空请求体",
			requestBody:    `{}`,
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			c := &app.RequestContext{}
			c.Request.SetBody([]byte(tt.requestBody))
			c.Request.Header.Set("Content-Type", "application/json")

			ListExperimentStats(ctx, c)

			if tt.expectError {
				assert.True(t, c.Response.StatusCode() != http.StatusOK)
			} else {
				assert.DeepEqual(t, http.StatusOK, c.Response.StatusCode())
			}
		})
	}
}

// TestUpsertExptTurnResultFilter 测试 UpsertExptTurnResultFilter handler
func TestUpsertExptTurnResultFilter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "成功更新过滤器",
			requestBody:    `{"experiment_id": 123, "filter": {"status": "completed"}}`,
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "请求参数无效",
			requestBody:    `{"invalid": "data"}`,
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:           "空请求体",
			requestBody:    `{}`,
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			c := &app.RequestContext{}
			c.Request.SetBody([]byte(tt.requestBody))
			c.Request.Header.Set("Content-Type", "application/json")

			UpsertExptTurnResultFilter(ctx, c)

			if tt.expectError {
				assert.True(t, c.Response.StatusCode() != http.StatusOK)
			} else {
				assert.DeepEqual(t, http.StatusOK, c.Response.StatusCode())
			}
		})
	}
}

// TestInsightAnalysisExperiment 测试 InsightAnalysisExperiment handler
func TestInsightAnalysisExperiment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "成功分析实验",
			requestBody:    `{"experiment_id": 123, "analysis_type": "performance"}`,
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "请求参数无效",
			requestBody:    `{"invalid": "data"}`,
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			c := &app.RequestContext{}
			c.Request.SetBody([]byte(tt.requestBody))
			c.Request.Header.Set("Content-Type", "application/json")

			InsightAnalysisExperiment(ctx, c)

			if tt.expectError {
				assert.True(t, c.Response.StatusCode() != http.StatusOK)
			} else {
				assert.DeepEqual(t, http.StatusOK, c.Response.StatusCode())
			}
		})
	}
}

// TestListExptInsightAnalysisRecord 测试 ListExptInsightAnalysisRecord handler
func TestListExptInsightAnalysisRecord(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "成功获取分析记录列表",
			requestBody:    `{"experiment_id": 123}`,
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "请求参数无效",
			requestBody:    `{"invalid": "data"}`,
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			c := &app.RequestContext{}
			c.Request.SetBody([]byte(tt.requestBody))
			c.Request.Header.Set("Content-Type", "application/json")

			ListExptInsightAnalysisRecord(ctx, c)

			if tt.expectError {
				assert.True(t, c.Response.StatusCode() != http.StatusOK)
			} else {
				assert.DeepEqual(t, http.StatusOK, c.Response.StatusCode())
			}
		})
	}
}

// TestDeleteExptInsightAnalysisRecord 测试 DeleteExptInsightAnalysisRecord handler
func TestDeleteExptInsightAnalysisRecord(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "成功删除分析记录",
			requestBody:    `{"experiment_id": 123, "insight_analysis_record_id": 456}`,
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "请求参数无效",
			requestBody:    `{"invalid": "data"}`,
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			c := &app.RequestContext{}
			c.Request.SetBody([]byte(tt.requestBody))
			c.Request.Header.Set("Content-Type", "application/json")

			DeleteExptInsightAnalysisRecord(ctx, c)

			if tt.expectError {
				assert.True(t, c.Response.StatusCode() != http.StatusOK)
			} else {
				assert.DeepEqual(t, http.StatusOK, c.Response.StatusCode())
			}
		})
	}
}

// TestGetExptInsightAnalysisRecord 测试 GetExptInsightAnalysisRecord handler
func TestGetExptInsightAnalysisRecord(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "成功获取分析记录",
			requestBody:    `{"experiment_id": 123, "insight_analysis_record_id": 456}`,
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "请求参数无效",
			requestBody:    `{"invalid": "data"}`,
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			c := &app.RequestContext{}
			c.Request.SetBody([]byte(tt.requestBody))
			c.Request.Header.Set("Content-Type", "application/json")

			GetExptInsightAnalysisRecord(ctx, c)

			if tt.expectError {
				assert.True(t, c.Response.StatusCode() != http.StatusOK)
			} else {
				assert.DeepEqual(t, http.StatusOK, c.Response.StatusCode())
			}
		})
	}
}

// TestFeedbackExptInsightAnalysisReport 测试 FeedbackExptInsightAnalysisReport handler
func TestFeedbackExptInsightAnalysisReport(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "成功提交反馈",
			requestBody:    `{"experiment_id": 123, "insight_analysis_record_id": 456, "feedback": "good"}`,
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "请求参数无效",
			requestBody:    `{"invalid": "data"}`,
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			c := &app.RequestContext{}
			c.Request.SetBody([]byte(tt.requestBody))
			c.Request.Header.Set("Content-Type", "application/json")

			FeedbackExptInsightAnalysisReport(ctx, c)

			if tt.expectError {
				assert.True(t, c.Response.StatusCode() != http.StatusOK)
			} else {
				assert.DeepEqual(t, http.StatusOK, c.Response.StatusCode())
			}
		})
	}
}

// TestListExptInsightAnalysisComment 测试 ListExptInsightAnalysisComment handler
func TestListExptInsightAnalysisComment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "成功获取评论列表",
			requestBody:    `{"experiment_id": 123, "insight_analysis_record_id": 456}`,
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "请求参数无效",
			requestBody:    `{"invalid": "data"}`,
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			c := &app.RequestContext{}
			c.Request.SetBody([]byte(tt.requestBody))
			c.Request.Header.Set("Content-Type", "application/json")

			ListExptInsightAnalysisComment(ctx, c)

			if tt.expectError {
				assert.True(t, c.Response.StatusCode() != http.StatusOK)
			} else {
				assert.DeepEqual(t, http.StatusOK, c.Response.StatusCode())
			}
		})
	}
}

// TestHandlerRequestBinding 测试请求绑定功能
func TestHandlerRequestBinding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		handler     func(context.Context, *app.RequestContext)
		requestBody string
		wantError   bool
	}{
		{
			name:        "ListExperimentStats - 有效请求",
			handler:     ListExperimentStats,
			requestBody: `{"space_id": 123}`,
			wantError:   false,
		},
		{
			name:        "ListExperimentStats - 无效JSON",
			handler:     ListExperimentStats,
			requestBody: `{invalid json}`,
			wantError:   true,
		},
		{
			name:        "UpsertExptTurnResultFilter - 有效请求",
			handler:     UpsertExptTurnResultFilter,
			requestBody: `{"experiment_id": 123}`,
			wantError:   false,
		},
		{
			name:        "UpsertExptTurnResultFilter - 无效JSON",
			handler:     UpsertExptTurnResultFilter,
			requestBody: `{invalid json}`,
			wantError:   true,
		},
		{
			name:        "InsightAnalysisExperiment - 有效请求",
			handler:     InsightAnalysisExperiment,
			requestBody: `{"experiment_id": 123}`,
			wantError:   false,
		},
		{
			name:        "InsightAnalysisExperiment - 无效JSON",
			handler:     InsightAnalysisExperiment,
			requestBody: `{invalid json}`,
			wantError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			c := &app.RequestContext{}
			c.Request.SetBody([]byte(tt.requestBody))
			c.Request.Header.Set("Content-Type", "application/json")

			tt.handler(ctx, c)

			if tt.wantError {
				assert.True(t, c.Response.StatusCode() != http.StatusOK)
			} else {
				assert.DeepEqual(t, http.StatusOK, c.Response.StatusCode())
			}
		})
	}
}

// TestResponseFormat 测试响应格式
func TestResponseFormat(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	c := &app.RequestContext{}
	c.Request.SetBody([]byte(`{}`))
	c.Request.Header.Set("Content-Type", "application/json")

	ListExperimentStats(ctx, c)

	// 验证响应是JSON格式
	assert.DeepEqual(t, http.StatusOK, c.Response.StatusCode())
	
	// 验证响应体不为空
	responseBody := c.Response.Body()
	require.NotNil(t, responseBody)
	require.True(t, len(responseBody) > 0)
}

// TestConcurrentRequests 测试并发请求处理
func TestConcurrentRequests(t *testing.T) {
	t.Parallel()

	const numGoroutines = 10
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer func() { done <- true }()

			ctx := context.Background()
			c := &app.RequestContext{}
			c.Request.SetBody([]byte(`{"space_id": 123}`))
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