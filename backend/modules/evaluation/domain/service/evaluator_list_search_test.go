// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// TestBuildListEvaluatorRequest_TransfersSearchDescription 锁定 service 层透传契约:
// buildListEvaluatorRequest 必须将 entity.ListEvaluatorRequest.SearchDescription
// 原样透传到 repo.ListEvaluatorRequest.SearchDescription（空值也原样透传空值）。
func TestBuildListEvaluatorRequest_TransfersSearchDescription(t *testing.T) {
	tests := []struct {
		name              string
		searchDescription string
		want              string
	}{
		{name: "non-empty transfers through", searchDescription: "foo", want: "foo"},
		{name: "empty transfers through as empty", searchDescription: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &entity.ListEvaluatorRequest{
				SpaceID:           1,
				SearchDescription: tt.searchDescription,
			}
			got, err := buildListEvaluatorRequest(context.Background(), req)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got.SearchDescription)
		})
	}
}
