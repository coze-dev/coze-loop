// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestPromptDO2BasicPO(t *testing.T) {
	tests := []struct {
		name     string
		do       *entity.Prompt
		expected *model.PromptBasic
	}{
		{
			name:     "nil input",
			do:       nil,
			expected: nil,
		},
		{
			name: "nil PromptBasic",
			do: &entity.Prompt{
				ID:        1,
				SpaceID:   100,
				PromptKey: "test_key",
			},
			expected: nil,
		},
		{
			name: "complete prompt",
			do: &entity.Prompt{
				ID:        1,
				SpaceID:   100,
				PromptKey: "test_key",
				PromptBasic: &entity.PromptBasic{
					DisplayName:   "test_name",
					Description:   "test_description",
					CreatedBy:     "test_creator",
					UpdatedBy:     "test_updater",
					LatestVersion: "1.0.0",
					CreatedAt:     time.Unix(1000, 0),
					UpdatedAt:     time.Unix(2000, 0),
				},
			},
			expected: &model.PromptBasic{
				ID:            1,
				SpaceID:       100,
				PromptKey:     "test_key",
				Name:          "test_name",
				Description:   "test_description",
				CreatedBy:     "test_creator",
				UpdatedBy:     "test_updater",
				LatestVersion: "1.0.0",
				CreatedAt:     time.Unix(1000, 0),
				UpdatedAt:     time.Unix(2000, 0),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PromptDO2BasicPO(tt.do)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestPromptDO2CommitPO(t *testing.T) {
	tests := []struct {
		name     string
		do       *entity.Prompt
		expected *model.PromptCommit
	}{
		{
			name:     "nil input",
			do:       nil,
			expected: nil,
		},
		{
			name: "empty prompt",
			do: &entity.Prompt{
				ID:        1,
				SpaceID:   100,
				PromptKey: "test_key",
			},
			expected: &model.PromptCommit{
				SpaceID:   100,
				PromptID:  1,
				PromptKey: "test_key",
			},
		},
		{
			name: "complete prompt with commit info",
			do: &entity.Prompt{
				ID:        1,
				SpaceID:   100,
				PromptKey: "test_key",
				PromptCommit: &entity.PromptCommit{
					CommitInfo: &entity.CommitInfo{
						Version:     "1.0.0",
						BaseVersion: "0.9.0",
						Description: "test commit",
						CommittedBy: "test_user",
						CommittedAt: time.Unix(1000, 0),
					},
					PromptDetail: &entity.PromptDetail{
						ModelConfig: &entity.ModelConfig{
							ModelID: 111,
						},
						Tools: []*entity.Tool{
							{
								Type: entity.ToolTypeFunction,
								Function: &entity.Function{
									Name:        "get_weather",
									Description: "tool for get weather",
									Parameters:  "test_tool_schema",
								},
							},
						},
						ToolCallConfig: &entity.ToolCallConfig{
							ToolChoice: entity.ToolChoiceTypeAuto,
						},
						PromptTemplate: &entity.PromptTemplate{
							TemplateType: entity.TemplateTypeNormal,
							Messages: []*entity.Message{
								{
									Role:    entity.RoleSystem,
									Content: ptr.Of("test content"),
								},
							},
							VariableDefs: []*entity.VariableDef{
								{
									Key:  "test_key",
									Desc: "test_key",
									Type: entity.VariableTypeString,
								},
							},
						},
					},
				},
			},
			expected: &model.PromptCommit{
				SpaceID:        100,
				PromptID:       1,
				PromptKey:      "test_key",
				Version:        "1.0.0",
				BaseVersion:    "0.9.0",
				Description:    ptr.Of("test commit"),
				CommittedBy:    "test_user",
				CreatedAt:      time.Unix(1000, 0),
				UpdatedAt:      time.Unix(1000, 0),
				ModelConfig:    ptr.Of(`{"model_id":111}`),
				Tools:          ptr.Of(`[{"type":"function","function":{"name":"get_weather","description":"tool for get weather","parameters":"test_tool_schema"}}]`),
				ToolCallConfig: ptr.Of(`{"tool_choice":"auto"}`),
				TemplateType:   ptr.Of("normal"),
				Messages:       ptr.Of(`[{"role":"system","content":"test content"}]`),
				VariableDefs:   ptr.Of(`[{"key":"test_key","desc":"test_key","type":"string"}]`),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PromptDO2CommitPO(tt.do)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestBatchBasicPO2PromptDO(t *testing.T) {
	tests := []struct {
		name     string
		pos      []*model.PromptBasic
		expected []*entity.Prompt
	}{
		{
			name:     "nil input",
			pos:      nil,
			expected: nil,
		},
		{
			name:     "empty slice",
			pos:      []*model.PromptBasic{},
			expected: nil,
		},
		{
			name: "slice with nil element",
			pos: []*model.PromptBasic{
				nil,
				{
					ID:            1,
					SpaceID:       100,
					PromptKey:     "test_key",
					Name:          "test_name",
					Description:   "test_description",
					CreatedBy:     "test_creator",
					UpdatedBy:     "test_updater",
					LatestVersion: "1.0.0",
					CreatedAt:     time.Unix(1000, 0),
					UpdatedAt:     time.Unix(2000, 0),
				},
			},
			expected: []*entity.Prompt{
				{
					ID:        1,
					SpaceID:   100,
					PromptKey: "test_key",
					PromptBasic: &entity.PromptBasic{
						DisplayName:   "test_name",
						Description:   "test_description",
						CreatedBy:     "test_creator",
						UpdatedBy:     "test_updater",
						LatestVersion: "1.0.0",
						CreatedAt:     time.Unix(1000, 0),
						UpdatedAt:     time.Unix(2000, 0),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BatchBasicPO2PromptDO(tt.pos)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestPromptPO2DO(t *testing.T) {
	tests := []struct {
		name     string
		basicPO  *model.PromptBasic
		commitPO *model.PromptCommit
		draftPO  *model.PromptUserDraft
		expected *entity.Prompt
	}{
		{
			name:     "nil basicPO",
			basicPO:  nil,
			commitPO: nil,
			draftPO:  nil,
			expected: nil,
		},
		{
			name: "complete prompt",
			basicPO: &model.PromptBasic{
				ID:            1,
				SpaceID:       100,
				PromptKey:     "test_key",
				Name:          "test_name",
				Description:   "test_description",
				CreatedBy:     "test_creator",
				UpdatedBy:     "test_updater",
				LatestVersion: "1.0.0",
				CreatedAt:     time.Unix(1000, 0),
				UpdatedAt:     time.Unix(2000, 0),
			},
			commitPO: &model.PromptCommit{
				Version:     "1.0.0",
				BaseVersion: "0.9.0",
				Description: ptr.Of("test commit"),
				CommittedBy: "test_user",
				CreatedAt:   time.Unix(1000, 0),
			},
			draftPO: &model.PromptUserDraft{
				UserID:      "test_user",
				BaseVersion: "1.0.0",
			},
			expected: &entity.Prompt{
				ID:        1,
				SpaceID:   100,
				PromptKey: "test_key",
				PromptBasic: &entity.PromptBasic{
					DisplayName:   "test_name",
					Description:   "test_description",
					CreatedBy:     "test_creator",
					UpdatedBy:     "test_updater",
					LatestVersion: "1.0.0",
					CreatedAt:     time.Unix(1000, 0),
					UpdatedAt:     time.Unix(2000, 0),
				},
				PromptCommit: &entity.PromptCommit{
					PromptDetail: &entity.PromptDetail{
						PromptTemplate: &entity.PromptTemplate{},
					},
					CommitInfo: &entity.CommitInfo{
						Version:     "1.0.0",
						BaseVersion: "0.9.0",
						Description: "test commit",
						CommittedBy: "test_user",
						CommittedAt: time.Unix(1000, 0),
					},
				},
				PromptDraft: &entity.PromptDraft{
					PromptDetail: &entity.PromptDetail{
						PromptTemplate: &entity.PromptTemplate{},
					},
					DraftInfo: &entity.DraftInfo{
						UserID:      "test_user",
						BaseVersion: "1.0.0",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PromptPO2DO(tt.basicPO, tt.commitPO, tt.draftPO)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestBasicPO2DO(t *testing.T) {
	tests := []struct {
		name     string
		po       *model.PromptBasic
		expected *entity.PromptBasic
	}{
		{
			name:     "nil input",
			po:       nil,
			expected: nil,
		},
		{
			name: "complete basic",
			po: &model.PromptBasic{
				Name:          "test_name",
				Description:   "test_description",
				CreatedBy:     "test_creator",
				UpdatedBy:     "test_updater",
				LatestVersion: "1.0.0",
				CreatedAt:     time.Unix(1000, 0),
				UpdatedAt:     time.Unix(2000, 0),
			},
			expected: &entity.PromptBasic{
				DisplayName:   "test_name",
				Description:   "test_description",
				CreatedBy:     "test_creator",
				UpdatedBy:     "test_updater",
				LatestVersion: "1.0.0",
				CreatedAt:     time.Unix(1000, 0),
				UpdatedAt:     time.Unix(2000, 0),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BasicPO2DO(tt.po)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestBatchGetCommitInfoDOFromCommitDO(t *testing.T) {
	tests := []struct {
		name     string
		do       []*entity.PromptCommit
		expected []*entity.CommitInfo
	}{
		{
			name:     "nil input",
			do:       nil,
			expected: nil,
		},
		{
			name:     "empty slice",
			do:       []*entity.PromptCommit{},
			expected: nil,
		},
		{
			name: "slice with nil elements",
			do: []*entity.PromptCommit{
				nil,
				{
					CommitInfo: &entity.CommitInfo{
						Version:     "1.0.0",
						BaseVersion: "0.9.0",
						Description: "test commit",
						CommittedBy: "test_user",
						CommittedAt: time.Unix(1000, 0),
					},
				},
				{
					CommitInfo: nil,
				},
			},
			expected: []*entity.CommitInfo{
				{
					Version:     "1.0.0",
					BaseVersion: "0.9.0",
					Description: "test commit",
					CommittedBy: "test_user",
					CommittedAt: time.Unix(1000, 0),
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BatchGetCommitInfoDOFromCommitDO(tt.do)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestBatchCommitPO2DO(t *testing.T) {
	tests := []struct {
		name     string
		pos      []*model.PromptCommit
		expected []*entity.PromptCommit
	}{
		{
			name:     "nil input",
			pos:      nil,
			expected: nil,
		},
		{
			name:     "empty slice",
			pos:      []*model.PromptCommit{},
			expected: nil,
		},
		{
			name: "complete commits",
			pos: []*model.PromptCommit{
				{
					Version:     "1.0.0",
					BaseVersion: "0.9.0",
					Description: ptr.Of("test commit"),
					CommittedBy: "test_user",
					CreatedAt:   time.Unix(1000, 0),
				},
			},
			expected: []*entity.PromptCommit{
				{
					PromptDetail: &entity.PromptDetail{
						PromptTemplate: &entity.PromptTemplate{},
					},
					CommitInfo: &entity.CommitInfo{
						Version:     "1.0.0",
						BaseVersion: "0.9.0",
						Description: "test commit",
						CommittedBy: "test_user",
						CommittedAt: time.Unix(1000, 0),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BatchCommitPO2DO(tt.pos)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestCommitPO2DO(t *testing.T) {
	tests := []struct {
		name     string
		po       *model.PromptCommit
		expected *entity.PromptCommit
	}{
		{
			name:     "nil input",
			po:       nil,
			expected: nil,
		},
		{
			name: "complete commit",
			po: &model.PromptCommit{
				Version:        "1.0.0",
				BaseVersion:    "0.9.0",
				Description:    ptr.Of("test commit"),
				CommittedBy:    "test_user",
				CreatedAt:      time.Unix(1000, 0),
				ModelConfig:    ptr.Of(`{"model_id":111}`),
				Tools:          ptr.Of(`[{"type":"function","function":{"name":"get_weather","description":"tool for get weather","parameters":"test_tool_schema"}}]`),
				ToolCallConfig: ptr.Of(`{"tool_choice":"auto"}`),
				TemplateType:   ptr.Of("normal"),
				Messages:       ptr.Of(`[{"role":"system","content":"test content"}]`),
				VariableDefs:   ptr.Of(`[{"key":"test_key","desc":"test_key","type":"string"}]`),
			},
			expected: &entity.PromptCommit{
				CommitInfo: &entity.CommitInfo{
					Version:     "1.0.0",
					BaseVersion: "0.9.0",
					Description: "test commit",
					CommittedBy: "test_user",
					CommittedAt: time.Unix(1000, 0),
				},
				PromptDetail: &entity.PromptDetail{
					ModelConfig: &entity.ModelConfig{
						ModelID: 111,
					},
					Tools: []*entity.Tool{
						{
							Type: entity.ToolTypeFunction,
							Function: &entity.Function{
								Name:        "get_weather",
								Description: "tool for get weather",
								Parameters:  "test_tool_schema",
							},
						},
					},
					ToolCallConfig: &entity.ToolCallConfig{
						ToolChoice: entity.ToolChoiceTypeAuto,
					},
					PromptTemplate: &entity.PromptTemplate{
						TemplateType: entity.TemplateTypeNormal,
						Messages: []*entity.Message{
							{
								Role:    entity.RoleSystem,
								Content: ptr.Of("test content"),
							},
						},
						VariableDefs: []*entity.VariableDef{
							{
								Key:  "test_key",
								Desc: "test_key",
								Type: entity.VariableTypeString,
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CommitPO2DO(tt.po)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestPromptDO2DraftPO(t *testing.T) {
	tests := []struct {
		name     string
		do       *entity.Prompt
		expected *model.PromptUserDraft
	}{
		{
			name:     "nil input",
			do:       nil,
			expected: nil,
		},
		{
			name: "empty prompt",
			do: &entity.Prompt{
				ID:        1,
				SpaceID:   100,
				PromptKey: "test_key",
			},
			expected: &model.PromptUserDraft{
				SpaceID:  100,
				PromptID: 1,
			},
		},
		{
			name: "complete prompt with draft",
			do: &entity.Prompt{
				ID:        1,
				SpaceID:   100,
				PromptKey: "test_key",
				PromptDraft: &entity.PromptDraft{
					DraftInfo: &entity.DraftInfo{
						UserID:      "test_user",
						BaseVersion: "1.0.0",
						IsModified:  true,
					},
					PromptDetail: &entity.PromptDetail{
						ModelConfig: &entity.ModelConfig{
							ModelID: 111,
						},
						Tools: []*entity.Tool{
							{
								Type: entity.ToolTypeFunction,
								Function: &entity.Function{
									Name:        "get_weather",
									Description: "tool for get weather",
									Parameters:  "test_tool_schema",
								},
							},
						},
						ToolCallConfig: &entity.ToolCallConfig{
							ToolChoice: entity.ToolChoiceTypeAuto,
						},
						PromptTemplate: &entity.PromptTemplate{
							TemplateType: entity.TemplateTypeNormal,
							Messages: []*entity.Message{
								{
									Role:    entity.RoleSystem,
									Content: ptr.Of("test content"),
								},
							},
							VariableDefs: []*entity.VariableDef{
								{
									Key:  "test_key",
									Desc: "test_key",
									Type: entity.VariableTypeString,
								},
							},
						},
					},
				},
			},
			expected: &model.PromptUserDraft{
				SpaceID:        100,
				PromptID:       1,
				UserID:         "test_user",
				BaseVersion:    "1.0.0",
				IsDraftEdited:  1,
				ModelConfig:    ptr.Of(`{"model_id":111}`),
				Tools:          ptr.Of(`[{"type":"function","function":{"name":"get_weather","description":"tool for get weather","parameters":"test_tool_schema"}}]`),
				ToolCallConfig: ptr.Of(`{"tool_choice":"auto"}`),
				TemplateType:   ptr.Of("normal"),
				Messages:       ptr.Of(`[{"role":"system","content":"test content"}]`),
				VariableDefs:   ptr.Of(`[{"key":"test_key","desc":"test_key","type":"string"}]`),
			},
		},
		{
			name: "prompt with nil draft info",
			do: &entity.Prompt{
				ID:        1,
				SpaceID:   100,
				PromptKey: "test_key",
				PromptDraft: &entity.PromptDraft{
					PromptDetail: &entity.PromptDetail{
						ModelConfig: &entity.ModelConfig{
							ModelID: 111,
						},
					},
				},
			},
			expected: &model.PromptUserDraft{
				SpaceID:     100,
				PromptID:    1,
				ModelConfig: ptr.Of(`{"model_id":111}`),
			},
		},
		{
			name: "prompt with nil prompt detail",
			do: &entity.Prompt{
				ID:        1,
				SpaceID:   100,
				PromptKey: "test_key",
				PromptDraft: &entity.PromptDraft{
					DraftInfo: &entity.DraftInfo{
						UserID:      "test_user",
						BaseVersion: "1.0.0",
						IsModified:  true,
					},
				},
			},
			expected: &model.PromptUserDraft{
				SpaceID:       100,
				PromptID:      1,
				UserID:        "test_user",
				BaseVersion:   "1.0.0",
				IsDraftEdited: 1,
			},
		},
		{
			name: "prompt with nil prompt template",
			do: &entity.Prompt{
				ID:        1,
				SpaceID:   100,
				PromptKey: "test_key",
				PromptDraft: &entity.PromptDraft{
					DraftInfo: &entity.DraftInfo{
						UserID:      "test_user",
						BaseVersion: "1.0.0",
						IsModified:  true,
					},
					PromptDetail: &entity.PromptDetail{
						ModelConfig: &entity.ModelConfig{
							ModelID: 111,
						},
					},
				},
			},
			expected: &model.PromptUserDraft{
				SpaceID:       100,
				PromptID:      1,
				UserID:        "test_user",
				BaseVersion:   "1.0.0",
				IsDraftEdited: 1,
				ModelConfig:   ptr.Of(`{"model_id":111}`),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PromptDO2DraftPO(tt.do)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestPromptTemplateMetadataRoundTrip(t *testing.T) {
	t.Parallel()

	commitMetadata := map[string]string{"commit": "meta"}
	draftMetadata := map[string]string{"draft": "meta"}
	prompt := &entity.Prompt{
		ID:        1,
		SpaceID:   2,
		PromptKey: "test_key",
		PromptCommit: &entity.PromptCommit{
			PromptDetail: &entity.PromptDetail{
				PromptTemplate: &entity.PromptTemplate{
					Metadata: commitMetadata,
				},
			},
		},
		PromptDraft: &entity.PromptDraft{
			DraftInfo: &entity.DraftInfo{UserID: "user"},
			PromptDetail: &entity.PromptDetail{
				PromptTemplate: &entity.PromptTemplate{
					Metadata: draftMetadata,
				},
			},
		},
	}

	commitPO := PromptDO2CommitPO(prompt)
	if assert.NotNil(t, commitPO.Metadata) {
		commitDO := CommitPO2DO(&model.PromptCommit{Metadata: commitPO.Metadata})
		assert.NotNil(t, commitDO)
		assert.NotNil(t, commitDO.PromptDetail)
		assert.NotNil(t, commitDO.PromptDetail.PromptTemplate)
		assert.Equal(t, commitMetadata, commitDO.PromptDetail.PromptTemplate.Metadata)
	}

	draftPO := PromptDO2DraftPO(prompt)
	if assert.NotNil(t, draftPO.Metadata) {
		draftModel := &model.PromptUserDraft{
			UserID:   "user",
			Metadata: draftPO.Metadata,
		}
		draftDO := DraftPO2DO(draftModel)
		assert.NotNil(t, draftDO)
		assert.NotNil(t, draftDO.PromptDetail)
		assert.NotNil(t, draftDO.PromptDetail.PromptTemplate)
		assert.Equal(t, draftMetadata, draftDO.PromptDetail.PromptTemplate.Metadata)
	}
}
