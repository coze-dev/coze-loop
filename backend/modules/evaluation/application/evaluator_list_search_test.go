// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	evaluatorservice "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/evaluator"
)

// TestBuildSrvListEvaluatorRequest_TransfersSearchDescription 锁定 application 层透传契约:
// buildSrvListEvaluatorRequest 必须把 ListEvaluatorsRequest.SearchDescription（optional thrift *string）
// 经 GetSearchDescription() 透传到 entity.ListEvaluatorRequest.SearchDescription;
// 未设置(nil)时 GetSearchDescription() 返回空串, 透传结果也应为空。
func TestBuildSrvListEvaluatorRequest_TransfersSearchDescription(t *testing.T) {
	tests := []struct {
		name              string
		searchDescription *string
		want              string
	}{
		{name: "set transfers through", searchDescription: gptr.Of("bar"), want: "bar"},
		{name: "nil transfers through as empty", searchDescription: nil, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &evaluatorservice.ListEvaluatorsRequest{
				WorkspaceID:       1,
				SearchDescription: tt.searchDescription,
			}
			got := buildSrvListEvaluatorRequest(req)
			assert.Equal(t, tt.want, got.SearchDescription)
		})
	}
}
