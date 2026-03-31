// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity/toolmgmt"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestToolPO2DO(t *testing.T) {
	tests := []struct {
		name     string
		basicPO  *model.ToolBasic
		commitPO *model.ToolCommit
		expected *toolmgmt.Tool
	}{
		{
			name:     "nil basicPO",
			basicPO:  nil,
			commitPO: nil,
			expected: nil,
		},
		{
			name: "nil commitPO",
			basicPO: &model.ToolBasic{
				ID:                     1,
				SpaceID:                100,
				Name:                   "test_tool",
				Description:            "test_description",
				LatestCommittedVersion: "1.0.0",
				CreatedBy:              "test_creator",
				UpdatedBy:              "test_updater",
				CreatedAt:              time.Unix(1000, 0),
				UpdatedAt:              time.Unix(2000, 0),
			},
			commitPO: nil,
			expected: &toolmgmt.Tool{
				ID:      1,
				SpaceID: 100,
				ToolBasic: &toolmgmt.ToolBasic{
					Name:                   "test_tool",
					Description:            "test_description",
					LatestCommittedVersion: "1.0.0",
					CreatedBy:              "test_creator",
					UpdatedBy:              "test_updater",
					CreatedAt:              time.Unix(1000, 0),
					UpdatedAt:              time.Unix(2000, 0),
				},
			},
		},
		{
			name: "complete data",
			basicPO: &model.ToolBasic{
				ID:                     1,
				SpaceID:                100,
				Name:                   "test_tool",
				Description:            "test_description",
				LatestCommittedVersion: "1.0.0",
				CreatedBy:              "test_creator",
				UpdatedBy:              "test_updater",
				CreatedAt:              time.Unix(1000, 0),
				UpdatedAt:              time.Unix(2000, 0),
			},
			commitPO: &model.ToolCommit{
				ID:          10,
				SpaceID:     100,
				ToolID:      1,
				Content:     ptr.Of("test_content"),
				Version:     "1.0.0",
				BaseVersion: "0.9.0",
				CommittedBy: "test_user",
				Description: ptr.Of("test commit"),
				CreatedAt:   time.Unix(3000, 0),
				UpdatedAt:   time.Unix(4000, 0),
			},
			expected: &toolmgmt.Tool{
				ID:      1,
				SpaceID: 100,
				ToolBasic: &toolmgmt.ToolBasic{
					Name:                   "test_tool",
					Description:            "test_description",
					LatestCommittedVersion: "1.0.0",
					CreatedBy:              "test_creator",
					UpdatedBy:              "test_updater",
					CreatedAt:              time.Unix(1000, 0),
					UpdatedAt:              time.Unix(2000, 0),
				},
				ToolCommit: &toolmgmt.ToolCommit{
					CommitInfo: &toolmgmt.CommitInfo{
						Version:     "1.0.0",
						BaseVersion: "0.9.0",
						Description: "test commit",
						CommittedBy: "test_user",
						CommittedAt: time.Unix(3000, 0),
					},
					ToolDetail: &toolmgmt.ToolDetail{
						Content: "test_content",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToolPO2DO(tt.basicPO, tt.commitPO)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestToolCommitPO2DO(t *testing.T) {
	tests := []struct {
		name     string
		commitPO *model.ToolCommit
		expected *toolmgmt.ToolCommit
	}{
		{
			name:     "nil input",
			commitPO: nil,
			expected: nil,
		},
		{
			name: "complete commit",
			commitPO: &model.ToolCommit{
				ID:          10,
				SpaceID:     100,
				ToolID:      1,
				Content:     ptr.Of("test_content"),
				Version:     "1.0.0",
				BaseVersion: "0.9.0",
				CommittedBy: "test_user",
				Description: ptr.Of("test commit"),
				CreatedAt:   time.Unix(1000, 0),
				UpdatedAt:   time.Unix(2000, 0),
			},
			expected: &toolmgmt.ToolCommit{
				CommitInfo: &toolmgmt.CommitInfo{
					Version:     "1.0.0",
					BaseVersion: "0.9.0",
					Description: "test commit",
					CommittedBy: "test_user",
					CommittedAt: time.Unix(1000, 0),
				},
				ToolDetail: &toolmgmt.ToolDetail{
					Content: "test_content",
				},
			},
		},
		{
			name: "nil content and description",
			commitPO: &model.ToolCommit{
				ID:          10,
				SpaceID:     100,
				ToolID:      1,
				Content:     nil,
				Version:     "2.0.0",
				BaseVersion: "1.0.0",
				CommittedBy: "test_user",
				Description: nil,
				CreatedAt:   time.Unix(3000, 0),
				UpdatedAt:   time.Unix(4000, 0),
			},
			expected: &toolmgmt.ToolCommit{
				CommitInfo: &toolmgmt.CommitInfo{
					Version:     "2.0.0",
					BaseVersion: "1.0.0",
					Description: "",
					CommittedBy: "test_user",
					CommittedAt: time.Unix(3000, 0),
				},
				ToolDetail: &toolmgmt.ToolDetail{
					Content: "",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToolCommitPO2DO(tt.commitPO)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestToolDO2BasicPO(t *testing.T) {
	tests := []struct {
		name     string
		do       *toolmgmt.Tool
		expected *model.ToolBasic
	}{
		{
			name:     "nil input",
			do:       nil,
			expected: nil,
		},
		{
			name: "nil ToolBasic",
			do: &toolmgmt.Tool{
				ID:      1,
				SpaceID: 100,
			},
			expected: nil,
		},
		{
			name: "complete data",
			do: &toolmgmt.Tool{
				ID:      1,
				SpaceID: 100,
				ToolBasic: &toolmgmt.ToolBasic{
					Name:                   "test_tool",
					Description:            "test_description",
					LatestCommittedVersion: "1.0.0",
					CreatedBy:              "test_creator",
					UpdatedBy:              "test_updater",
					CreatedAt:              time.Unix(1000, 0),
					UpdatedAt:              time.Unix(2000, 0),
				},
			},
			expected: &model.ToolBasic{
				ID:                     1,
				SpaceID:                100,
				Name:                   "test_tool",
				Description:            "test_description",
				LatestCommittedVersion: "1.0.0",
				CreatedBy:              "test_creator",
				UpdatedBy:              "test_updater",
				CreatedAt:              time.Unix(1000, 0),
				UpdatedAt:              time.Unix(2000, 0),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ToolDO2BasicPO(tt.do)
			assert.Equal(t, tt.expected, got)
		})
	}
}
