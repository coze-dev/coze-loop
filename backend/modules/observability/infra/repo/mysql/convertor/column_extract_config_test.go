// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/gorm_gen/model"
	"github.com/stretchr/testify/assert"
)

func TestColumnExtractConfigPO2DO(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	configJSON := `[{"Column":"input","JSONPath":"$.messages[0].content"},{"Column":"output","JSONPath":"$.data"}]`

	tests := []struct {
		name string
		po   *model.ObservabilityColumnExtractConfig
		want *entity.ColumnExtractConfig
	}{
		{
			name: "nil input",
			po:   nil,
			want: nil,
		},
		{
			name: "full conversion",
			po: &model.ObservabilityColumnExtractConfig{
				ID:           1,
				WorkspaceID:  100,
				PlatformType: "coze_loop",
				SpanListType: "llm_span",
				AgentName:    "test-agent",
				Config:       &configJSON,
				CreatedAt:    now,
				CreatedBy:    "user-1",
				UpdatedAt:    now,
				UpdatedBy:    "user-2",
			},
			want: &entity.ColumnExtractConfig{
				ID:           1,
				WorkspaceID:  100,
				PlatformType: "coze_loop",
				SpanListType: "llm_span",
				AgentName:    "test-agent",
				Columns: []entity.ColumnExtractRule{
					{Column: "input", JSONPath: "$.messages[0].content"},
					{Column: "output", JSONPath: "$.data"},
				},
				CreatedAt: now,
				CreatedBy: "user-1",
				UpdatedAt: now,
				UpdatedBy: "user-2",
			},
		},
		{
			name: "nil config",
			po: &model.ObservabilityColumnExtractConfig{
				ID:           2,
				WorkspaceID:  200,
				PlatformType: "coze_loop",
				SpanListType: "all_span",
				Config:       nil,
				CreatedAt:    now,
				CreatedBy:    "user-1",
				UpdatedAt:    now,
				UpdatedBy:    "user-1",
			},
			want: &entity.ColumnExtractConfig{
				ID:           2,
				WorkspaceID:  200,
				PlatformType: "coze_loop",
				SpanListType: "all_span",
				CreatedAt:    now,
				CreatedBy:    "user-1",
				UpdatedAt:    now,
				UpdatedBy:    "user-1",
			},
		},
		{
			name: "empty config string",
			po: func() *model.ObservabilityColumnExtractConfig {
				empty := ""
				return &model.ObservabilityColumnExtractConfig{
					ID:           3,
					WorkspaceID:  300,
					PlatformType: "coze_loop",
					SpanListType: "root_span",
					Config:       &empty,
					CreatedAt:    now,
					CreatedBy:    "user-1",
					UpdatedAt:    now,
					UpdatedBy:    "user-1",
				}
			}(),
			want: &entity.ColumnExtractConfig{
				ID:           3,
				WorkspaceID:  300,
				PlatformType: "coze_loop",
				SpanListType: "root_span",
				CreatedAt:    now,
				CreatedBy:    "user-1",
				UpdatedAt:    now,
				UpdatedBy:    "user-1",
			},
		},
		{
			name: "invalid JSON config",
			po: func() *model.ObservabilityColumnExtractConfig {
				invalid := "not-json"
				return &model.ObservabilityColumnExtractConfig{
					ID:           4,
					WorkspaceID:  400,
					PlatformType: "coze_loop",
					SpanListType: "all_span",
					Config:       &invalid,
					CreatedAt:    now,
					CreatedBy:    "user-1",
					UpdatedAt:    now,
					UpdatedBy:    "user-1",
				}
			}(),
			want: &entity.ColumnExtractConfig{
				ID:           4,
				WorkspaceID:  400,
				PlatformType: "coze_loop",
				SpanListType: "all_span",
				CreatedAt:    now,
				CreatedBy:    "user-1",
				UpdatedAt:    now,
				UpdatedBy:    "user-1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ColumnExtractConfigPO2DO(tt.po)
			assert.Equal(t, tt.want, got)
		})
	}
}
