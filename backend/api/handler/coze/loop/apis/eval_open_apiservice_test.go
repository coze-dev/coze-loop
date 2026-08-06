// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package apis

import (
	"context"
	"net/http"
	"testing"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/test/assert"
)

// TestReportEvalTargetStepMetric 覆盖 ReportEvalTargetStepMetric handler 的绑定与响应分支。
// 该 handler 仅做 BindAndValidate + 返回空 200 响应，所有字段 optional，因此合法 body 恒 200。
func TestReportEvalTargetStepMetric(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		requestBody    string
		expectedStatus int
	}{
		{
			name:           "empty body -> 200",
			requestBody:    `{}`,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "full valid body -> 200",
			requestBody:    `{"workspace_id":123,"invoke_id":456,"step_name":"llm","duration_ms":100,"success":true,"error_code":0,"error_message":""}`,
			expectedStatus: http.StatusOK,
		},
		// 说明：req 所有字段均为 optional，hertz BindAndValidate 对本请求体不产生解码错误，
		// 故 handler 内 `err != nil -> 400` 为防御性死分支，无法经 HTTP body 触发（与本包
		// 既有 handler 测试对 invalid json 恒 200 的观察一致）。
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			c := &app.RequestContext{}
			c.Request.SetBody([]byte(tt.requestBody))
			c.Request.Header.Set("Content-Type", "application/json")

			ReportEvalTargetStepMetric(ctx, c)

			assert.DeepEqual(t, tt.expectedStatus, c.Response.StatusCode())
		})
	}
}
