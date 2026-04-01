// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/tool"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity/toolmgmt"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestToolMgmtDO2DTO(t *testing.T) {
	now := time.Now()
	nowMilli := now.UnixMilli()

	tests := []struct {
		name     string
		input    *toolmgmt.Tool
		expected *tool.Tool
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:  "empty struct",
			input: &toolmgmt.Tool{},
			expected: &tool.Tool{
				ID:          ptr.Of(int64(0)),
				WorkspaceID: ptr.Of(int64(0)),
			},
		},
		{
			name: "complete data",
			input: &toolmgmt.Tool{
				ID:      123,
				SpaceID: 456,
				ToolBasic: &toolmgmt.ToolBasic{
					Name:                   "test_tool",
					Description:            "test description",
					LatestCommittedVersion: "1.0.0",
					CreatedBy:              "user1",
					UpdatedBy:              "user2",
					CreatedAt:              time.UnixMilli(nowMilli),
					UpdatedAt:              time.UnixMilli(nowMilli),
				},
				ToolCommit: &toolmgmt.ToolCommit{
					CommitInfo: &toolmgmt.CommitInfo{
						Version:     "1.0.0",
						BaseVersion: "0.9.0",
						Description: "initial commit",
						CommittedBy: "user1",
						CommittedAt: time.UnixMilli(nowMilli),
					},
					ToolDetail: &toolmgmt.ToolDetail{
						Content: "tool content",
					},
				},
			},
			expected: &tool.Tool{
				ID:          ptr.Of(int64(123)),
				WorkspaceID: ptr.Of(int64(456)),
				ToolBasic: &tool.ToolBasic{
					Name:                   ptr.Of("test_tool"),
					Description:            ptr.Of("test description"),
					LatestCommittedVersion: ptr.Of("1.0.0"),
					CreatedBy:              ptr.Of("user1"),
					UpdatedBy:              ptr.Of("user2"),
					CreatedAt:              ptr.Of(nowMilli),
					UpdatedAt:              ptr.Of(nowMilli),
				},
				ToolCommit: &tool.ToolCommit{
					CommitInfo: &tool.CommitInfo{
						Version:     ptr.Of("1.0.0"),
						BaseVersion: ptr.Of("0.9.0"),
						Description: ptr.Of("initial commit"),
						CommittedBy: ptr.Of("user1"),
						CommittedAt: ptr.Of(nowMilli),
					},
					Detail: &tool.ToolDetail{
						Content: ptr.Of("tool content"),
					},
				},
			},
		},
		{
			name: "with only basic",
			input: &toolmgmt.Tool{
				ID:      789,
				SpaceID: 321,
				ToolBasic: &toolmgmt.ToolBasic{
					Name:      "basic_only",
					CreatedAt: time.UnixMilli(nowMilli),
					UpdatedAt: time.UnixMilli(nowMilli),
				},
			},
			expected: &tool.Tool{
				ID:          ptr.Of(int64(789)),
				WorkspaceID: ptr.Of(int64(321)),
				ToolBasic: &tool.ToolBasic{
					Name:                   ptr.Of("basic_only"),
					Description:            ptr.Of(""),
					LatestCommittedVersion: ptr.Of(""),
					CreatedBy:              ptr.Of(""),
					UpdatedBy:              ptr.Of(""),
					CreatedAt:              ptr.Of(nowMilli),
					UpdatedAt:              ptr.Of(nowMilli),
				},
			},
		},
		{
			name: "with only commit",
			input: &toolmgmt.Tool{
				ID:      100,
				SpaceID: 200,
				ToolCommit: &toolmgmt.ToolCommit{
					CommitInfo: &toolmgmt.CommitInfo{
						Version:     "2.0.0",
						CommittedAt: time.UnixMilli(nowMilli),
					},
				},
			},
			expected: &tool.Tool{
				ID:          ptr.Of(int64(100)),
				WorkspaceID: ptr.Of(int64(200)),
				ToolCommit: &tool.ToolCommit{
					CommitInfo: &tool.CommitInfo{
						Version:     ptr.Of("2.0.0"),
						BaseVersion: ptr.Of(""),
						Description: ptr.Of(""),
						CommittedBy: ptr.Of(""),
						CommittedAt: ptr.Of(nowMilli),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, ToolMgmtDO2DTO(tt.input))
		})
	}
}

func TestToolMgmtBasicDO2DTO(t *testing.T) {
	now := time.Now()
	nowMilli := now.UnixMilli()

	tests := []struct {
		name     string
		input    *toolmgmt.ToolBasic
		expected *tool.ToolBasic
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:  "empty struct",
			input: &toolmgmt.ToolBasic{},
			expected: &tool.ToolBasic{
				Name:                   ptr.Of(""),
				Description:            ptr.Of(""),
				LatestCommittedVersion: ptr.Of(""),
				CreatedBy:              ptr.Of(""),
				UpdatedBy:              ptr.Of(""),
				CreatedAt:              ptr.Of(time.Time{}.UnixMilli()),
				UpdatedAt:              ptr.Of(time.Time{}.UnixMilli()),
			},
		},
		{
			name: "complete data",
			input: &toolmgmt.ToolBasic{
				Name:                   "my_tool",
				Description:            "A great tool",
				LatestCommittedVersion: "3.1.0",
				CreatedBy:              "creator",
				UpdatedBy:              "updater",
				CreatedAt:              time.UnixMilli(nowMilli),
				UpdatedAt:              time.UnixMilli(nowMilli),
			},
			expected: &tool.ToolBasic{
				Name:                   ptr.Of("my_tool"),
				Description:            ptr.Of("A great tool"),
				LatestCommittedVersion: ptr.Of("3.1.0"),
				CreatedBy:              ptr.Of("creator"),
				UpdatedBy:              ptr.Of("updater"),
				CreatedAt:              ptr.Of(nowMilli),
				UpdatedAt:              ptr.Of(nowMilli),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, ToolMgmtBasicDO2DTO(tt.input))
		})
	}
}

func TestToolMgmtCommitDO2DTO(t *testing.T) {
	now := time.Now()
	nowMilli := now.UnixMilli()

	tests := []struct {
		name     string
		input    *toolmgmt.ToolCommit
		expected *tool.ToolCommit
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty struct",
			input:    &toolmgmt.ToolCommit{},
			expected: &tool.ToolCommit{},
		},
		{
			name: "with commit info only",
			input: &toolmgmt.ToolCommit{
				CommitInfo: &toolmgmt.CommitInfo{
					Version:     "1.0.0",
					BaseVersion: "0.9.0",
					Description: "release",
					CommittedBy: "admin",
					CommittedAt: time.UnixMilli(nowMilli),
				},
			},
			expected: &tool.ToolCommit{
				CommitInfo: &tool.CommitInfo{
					Version:     ptr.Of("1.0.0"),
					BaseVersion: ptr.Of("0.9.0"),
					Description: ptr.Of("release"),
					CommittedBy: ptr.Of("admin"),
					CommittedAt: ptr.Of(nowMilli),
				},
			},
		},
		{
			name: "with detail only",
			input: &toolmgmt.ToolCommit{
				ToolDetail: &toolmgmt.ToolDetail{
					Content: "some content",
				},
			},
			expected: &tool.ToolCommit{
				Detail: &tool.ToolDetail{
					Content: ptr.Of("some content"),
				},
			},
		},
		{
			name: "complete data",
			input: &toolmgmt.ToolCommit{
				CommitInfo: &toolmgmt.CommitInfo{
					Version:     "2.0.0",
					BaseVersion: "1.0.0",
					Description: "major update",
					CommittedBy: "dev",
					CommittedAt: time.UnixMilli(nowMilli),
				},
				ToolDetail: &toolmgmt.ToolDetail{
					Content: "updated content",
				},
			},
			expected: &tool.ToolCommit{
				CommitInfo: &tool.CommitInfo{
					Version:     ptr.Of("2.0.0"),
					BaseVersion: ptr.Of("1.0.0"),
					Description: ptr.Of("major update"),
					CommittedBy: ptr.Of("dev"),
					CommittedAt: ptr.Of(nowMilli),
				},
				Detail: &tool.ToolDetail{
					Content: ptr.Of("updated content"),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, ToolMgmtCommitDO2DTO(tt.input))
		})
	}
}

func TestToolMgmtCommitInfoDO2DTO(t *testing.T) {
	now := time.Now()
	nowMilli := now.UnixMilli()

	tests := []struct {
		name     string
		input    *toolmgmt.CommitInfo
		expected *tool.CommitInfo
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:  "empty struct",
			input: &toolmgmt.CommitInfo{},
			expected: &tool.CommitInfo{
				Version:     ptr.Of(""),
				BaseVersion: ptr.Of(""),
				Description: ptr.Of(""),
				CommittedBy: ptr.Of(""),
				CommittedAt: ptr.Of(time.Time{}.UnixMilli()),
			},
		},
		{
			name: "complete data",
			input: &toolmgmt.CommitInfo{
				Version:     "v1.2.3",
				BaseVersion: "v1.2.2",
				Description: "bugfix release",
				CommittedBy: "maintainer",
				CommittedAt: time.UnixMilli(nowMilli),
			},
			expected: &tool.CommitInfo{
				Version:     ptr.Of("v1.2.3"),
				BaseVersion: ptr.Of("v1.2.2"),
				Description: ptr.Of("bugfix release"),
				CommittedBy: ptr.Of("maintainer"),
				CommittedAt: ptr.Of(nowMilli),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, ToolMgmtCommitInfoDO2DTO(tt.input))
		})
	}
}

func TestToolMgmtDetailDO2DTO(t *testing.T) {
	tests := []struct {
		name     string
		input    *toolmgmt.ToolDetail
		expected *tool.ToolDetail
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:  "empty struct",
			input: &toolmgmt.ToolDetail{},
			expected: &tool.ToolDetail{
				Content: ptr.Of(""),
			},
		},
		{
			name: "complete data",
			input: &toolmgmt.ToolDetail{
				Content: "function definition content",
			},
			expected: &tool.ToolDetail{
				Content: ptr.Of("function definition content"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, ToolMgmtDetailDO2DTO(tt.input))
		})
	}
}

func TestBatchToolMgmtDO2DTO(t *testing.T) {
	now := time.Now()
	nowMilli := now.UnixMilli()

	tests := []struct {
		name     string
		input    []*toolmgmt.Tool
		expected []*tool.Tool
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name:     "empty slice",
			input:    []*toolmgmt.Tool{},
			expected: nil,
		},
		{
			name: "single element",
			input: []*toolmgmt.Tool{
				{
					ID:      1,
					SpaceID: 10,
				},
			},
			expected: []*tool.Tool{
				{
					ID:          ptr.Of(int64(1)),
					WorkspaceID: ptr.Of(int64(10)),
				},
			},
		},
		{
			name: "multiple elements",
			input: []*toolmgmt.Tool{
				{
					ID:      1,
					SpaceID: 10,
					ToolBasic: &toolmgmt.ToolBasic{
						Name:      "tool1",
						CreatedAt: time.UnixMilli(nowMilli),
						UpdatedAt: time.UnixMilli(nowMilli),
					},
				},
				{
					ID:      2,
					SpaceID: 20,
					ToolBasic: &toolmgmt.ToolBasic{
						Name:      "tool2",
						CreatedAt: time.UnixMilli(nowMilli),
						UpdatedAt: time.UnixMilli(nowMilli),
					},
				},
			},
			expected: []*tool.Tool{
				{
					ID:          ptr.Of(int64(1)),
					WorkspaceID: ptr.Of(int64(10)),
					ToolBasic: &tool.ToolBasic{
						Name:                   ptr.Of("tool1"),
						Description:            ptr.Of(""),
						LatestCommittedVersion: ptr.Of(""),
						CreatedBy:              ptr.Of(""),
						UpdatedBy:              ptr.Of(""),
						CreatedAt:              ptr.Of(nowMilli),
						UpdatedAt:              ptr.Of(nowMilli),
					},
				},
				{
					ID:          ptr.Of(int64(2)),
					WorkspaceID: ptr.Of(int64(20)),
					ToolBasic: &tool.ToolBasic{
						Name:                   ptr.Of("tool2"),
						Description:            ptr.Of(""),
						LatestCommittedVersion: ptr.Of(""),
						CreatedBy:              ptr.Of(""),
						UpdatedBy:              ptr.Of(""),
						CreatedAt:              ptr.Of(nowMilli),
						UpdatedAt:              ptr.Of(nowMilli),
					},
				},
			},
		},
		{
			name: "contains nil elements",
			input: []*toolmgmt.Tool{
				{
					ID:      1,
					SpaceID: 10,
				},
				nil,
				{
					ID:      3,
					SpaceID: 30,
				},
			},
			expected: []*tool.Tool{
				{
					ID:          ptr.Of(int64(1)),
					WorkspaceID: ptr.Of(int64(10)),
				},
				{
					ID:          ptr.Of(int64(3)),
					WorkspaceID: ptr.Of(int64(30)),
				},
			},
		},
		{
			name:     "all nil elements",
			input:    []*toolmgmt.Tool{nil, nil, nil},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.expected, BatchToolMgmtDO2DTO(tt.input))
		})
	}
}
