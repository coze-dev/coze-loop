// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/prompt"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestSecurityLevelDTO2DO_AllCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		dto  prompt.SecurityLevel
		want entity.SecurityLevel
	}{
		{"L1", prompt.SecurityLevelL1, entity.SecurityLevelL1},
		{"L2", prompt.SecurityLevelL2, entity.SecurityLevelL2},
		{"L3", prompt.SecurityLevelL3, entity.SecurityLevelL3},
		{"L4", prompt.SecurityLevelL4, entity.SecurityLevelL4},
		{"default", "unknown", entity.SecurityLevelL3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, SecurityLevelDTO2DO(tt.dto))
		})
	}
}

func TestSecurityLevelDO2DTO_AllCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		do   entity.SecurityLevel
		want *prompt.SecurityLevel
	}{
		{"L1", entity.SecurityLevelL1, ptr.Of(prompt.SecurityLevelL1)},
		{"L2", entity.SecurityLevelL2, ptr.Of(prompt.SecurityLevelL2)},
		{"L3", entity.SecurityLevelL3, ptr.Of(prompt.SecurityLevelL3)},
		{"L4", entity.SecurityLevelL4, ptr.Of(prompt.SecurityLevelL4)},
		{"default", entity.SecurityLevel("unknown"), ptr.Of(prompt.SecurityLevel("L3"))},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, SecurityLevelDO2DTO(tt.do))
		})
	}
}

func TestVariableTypeDTO2DO_Default(t *testing.T) {
	t.Parallel()
	assert.Equal(t, entity.VariableTypeString, VariableTypeDTO2DO("unknown_type"))
}

func TestToolChoiceTypeDTO2DO_Default(t *testing.T) {
	t.Parallel()
	assert.Equal(t, entity.ToolChoiceTypeAuto, ToolChoiceTypeDTO2DO("unknown"))
}

func TestPromptTypeDTO2DO_Default(t *testing.T) {
	t.Parallel()
	assert.Equal(t, entity.PromptTypeNormal, PromptTypeDTO2DO("unknown"))
}

func TestPromptTypeDO2DTO_Default(t *testing.T) {
	t.Parallel()
	assert.Equal(t, prompt.PromptTypeNormal, PromptTypeDO2DTO("unknown"))
}

func TestTemplateTypeDTO2DO_Default(t *testing.T) {
	t.Parallel()
	assert.Equal(t, entity.TemplateTypeNormal, TemplateTypeDTO2DO("unknown"))
}

func TestBatchPromptDTO2DO_AllNil(t *testing.T) {
	t.Parallel()
	result := BatchPromptDTO2DO([]*prompt.Prompt{nil, nil, nil})
	assert.Nil(t, result)
}

func TestBatchPromptDO2DTO_EmptyAndAllNil(t *testing.T) {
	t.Parallel()
	// empty slice
	assert.Nil(t, BatchPromptDO2DTO([]*entity.Prompt{}))
	// all nil elements
	assert.Nil(t, BatchPromptDO2DTO([]*entity.Prompt{nil, nil}))
}

func TestBatchCommitInfoDO2DTO_EmptyAndAllNil(t *testing.T) {
	t.Parallel()
	// empty slice
	assert.Nil(t, BatchCommitInfoDO2DTO([]*entity.CommitInfo{}))
	// all nil elements
	assert.Nil(t, BatchCommitInfoDO2DTO([]*entity.CommitInfo{nil, nil}))
}

func TestPromptBasicDO2DTO_LatestCommittedAtNotNil(t *testing.T) {
	t.Parallel()
	now := time.Now()
	do := &entity.PromptBasic{
		PromptType:        entity.PromptTypeNormal,
		SecurityLevel:     entity.SecurityLevelL3,
		DisplayName:       "test",
		LatestCommittedAt: &now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	dto := PromptBasicDO2DTO(do)
	assert.NotNil(t, dto)
	assert.NotNil(t, dto.LatestCommittedAt)
	assert.Equal(t, now.UnixMilli(), *dto.LatestCommittedAt)
}

func TestMcpConfigDTO2DO_NonNil(t *testing.T) {
	t.Parallel()
	dto := &prompt.McpConfig{
		IsMcpCallAutoRetry: ptr.Of(true),
		McpServers: []*prompt.McpServerCombine{
			{
				McpServerID:   ptr.Of(int64(10)),
				AccessPointID: ptr.Of(int64(20)),
			},
		},
	}
	result := McpConfigDTO2DO(dto)
	assert.NotNil(t, result)
	assert.Equal(t, ptr.Of(true), result.IsMcpCallAutoRetry)
	assert.Len(t, result.McpServers, 1)
	assert.Equal(t, ptr.Of(int64(10)), result.McpServers[0].McpServerID)
}

func TestMcpServerCombineDTO2DO_NonNil(t *testing.T) {
	t.Parallel()
	dto := &prompt.McpServerCombine{
		McpServerID:    ptr.Of(int64(1)),
		AccessPointID:  ptr.Of(int64(2)),
		DisabledTools:  []string{"a"},
		EnabledTools:   []string{"b"},
		IsEnabledTools: ptr.Of(true),
	}
	result := McpServerCombineDTO2DO(dto)
	assert.NotNil(t, result)
	assert.Equal(t, ptr.Of(int64(1)), result.McpServerID)
	assert.Equal(t, ptr.Of(int64(2)), result.AccessPointID)
	assert.Equal(t, []string{"a"}, result.DisabledTools)
	assert.Equal(t, []string{"b"}, result.EnabledTools)
	assert.Equal(t, ptr.Of(true), result.IsEnabledTools)
}

func TestParamConfigValueDTO2DO_NonNil(t *testing.T) {
	t.Parallel()
	dto := &prompt.ParamConfigValue{
		Name:  ptr.Of("name"),
		Label: ptr.Of("label"),
		Value: &prompt.ParamOption{
			Value: ptr.Of("v"),
			Label: ptr.Of("l"),
		},
	}
	result := ParamConfigValueDTO2DO(dto)
	assert.NotNil(t, result)
	assert.Equal(t, "name", result.Name)
	assert.Equal(t, "label", result.Label)
	assert.NotNil(t, result.Value)
	assert.Equal(t, "v", result.Value.Value)
}

func TestParamOptionDTO2DO_NonNil(t *testing.T) {
	t.Parallel()
	dto := &prompt.ParamOption{
		Value: ptr.Of("val"),
		Label: ptr.Of("lab"),
	}
	result := ParamOptionDTO2DO(dto)
	assert.NotNil(t, result)
	assert.Equal(t, "val", result.Value)
	assert.Equal(t, "lab", result.Label)
}

func TestThinkingOptionDTO2DO_NonNil(t *testing.T) {
	t.Parallel()
	opt := prompt.ThinkingOptionEnabled
	result := ThinkingOptionDTO2DO(&opt)
	assert.NotNil(t, result)
	assert.Equal(t, entity.ThinkingOptionEnabled, *result)
}

func TestReasoningEffortDTO2DO_NonNil(t *testing.T) {
	t.Parallel()
	eff := prompt.ReasoningEffortHigh
	result := ReasoningEffortDTO2DO(&eff)
	assert.NotNil(t, result)
	assert.Equal(t, entity.ReasoningEffortHigh, *result)
}

func TestThinkingOptionDO2DTO_NonNil(t *testing.T) {
	t.Parallel()
	opt := entity.ThinkingOptionAuto
	result := ThinkingOptionDO2DTO(&opt)
	assert.NotNil(t, result)
	assert.Equal(t, prompt.ThinkingOptionAuto, *result)
}

func TestReasoningEffortDO2DTO_NonNil(t *testing.T) {
	t.Parallel()
	eff := entity.ReasoningEffortMedium
	result := ReasoningEffortDO2DTO(&eff)
	assert.NotNil(t, result)
	assert.Equal(t, prompt.ReasoningEffortMedium, *result)
}

func TestScenarioDTO2DO_Default(t *testing.T) {
	t.Parallel()
	assert.Equal(t, entity.ScenarioDefault, ScenarioDTO2DO("unknown"))
}

func TestContentTypeDO2DTO_VideoURLAndDefault(t *testing.T) {
	t.Parallel()
	// VideoURL case returns "video_url"
	assert.Equal(t, prompt.ContentType("video_url"), ContentTypeDO2DTO(entity.ContentTypeVideoURL))
	// default case
	assert.Equal(t, prompt.ContentTypeText, ContentTypeDO2DTO(entity.ContentType("unknown")))
}

func TestRoleDO2DTO_PlaceholderAndDefault(t *testing.T) {
	t.Parallel()
	assert.Equal(t, prompt.RolePlaceholder, RoleDO2DTO(entity.RolePlaceholder))
	assert.Equal(t, prompt.RoleUser, RoleDO2DTO(entity.Role("unknown")))
}

func TestTokenUsageDO2DTO(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		do   *entity.TokenUsage
		want *prompt.TokenUsage
	}{
		{
			name: "nil",
			do:   nil,
			want: nil,
		},
		{
			name: "normal",
			do: &entity.TokenUsage{
				InputTokens:  100,
				OutputTokens: 200,
			},
			want: &prompt.TokenUsage{
				InputTokens:  ptr.Of(int64(100)),
				OutputTokens: ptr.Of(int64(200)),
			},
		},
		{
			name: "zero values",
			do: &entity.TokenUsage{
				InputTokens:  0,
				OutputTokens: 0,
			},
			want: &prompt.TokenUsage{
				InputTokens:  ptr.Of(int64(0)),
				OutputTokens: ptr.Of(int64(0)),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, TokenUsageDO2DTO(tt.do))
		})
	}
}

func TestBatchDebugToolCallDO2DTO(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		dos  []*entity.DebugToolCall
		want []*prompt.DebugToolCall
	}{
		{
			name: "nil",
			dos:  nil,
			want: nil,
		},
		{
			name: "with nil elements",
			dos:  []*entity.DebugToolCall{nil, nil},
			want: []*prompt.DebugToolCall{},
		},
		{
			name: "normal",
			dos: []*entity.DebugToolCall{
				{
					ToolCall: entity.ToolCall{
						Index: 0,
						ID:    "tc-1",
						Type:  entity.ToolTypeFunction,
						FunctionCall: &entity.FunctionCall{
							Name:      "fn1",
							Arguments: ptr.Of(`{"a":1}`),
						},
					},
					MockResponse:  "mock-resp",
					DebugTraceKey: "trace-key",
				},
			},
			want: []*prompt.DebugToolCall{
				{
					ToolCall: &prompt.ToolCall{
						Index: ptr.Of(int64(0)),
						ID:    ptr.Of("tc-1"),
						Type:  ptr.Of(prompt.ToolTypeFunction),
						FunctionCall: &prompt.FunctionCall{
							Name:      ptr.Of("fn1"),
							Arguments: ptr.Of(`{"a":1}`),
						},
					},
					MockResponse:  ptr.Of("mock-resp"),
					DebugTraceKey: ptr.Of("trace-key"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, BatchDebugToolCallDO2DTO(tt.dos))
		})
	}
}

func TestDebugToolCallDO2DTO(t *testing.T) {
	t.Parallel()
	assert.Nil(t, DebugToolCallDO2DTO(nil))
	result := DebugToolCallDO2DTO(&entity.DebugToolCall{
		ToolCall: entity.ToolCall{
			Index: 1,
			ID:    "id-1",
			Type:  entity.ToolTypeFunction,
		},
		MockResponse:  "resp",
		DebugTraceKey: "key",
	})
	assert.NotNil(t, result)
	assert.Equal(t, ptr.Of("resp"), result.MockResponse)
	assert.Equal(t, ptr.Of("key"), result.DebugTraceKey)
}

func TestBatchVariableValDTO2DO(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		dtos []*prompt.VariableVal
		want []*entity.VariableVal
	}{
		{
			name: "nil",
			dtos: nil,
			want: nil,
		},
		{
			name: "with nil elements",
			dtos: []*prompt.VariableVal{nil},
			want: []*entity.VariableVal{},
		},
		{
			name: "normal",
			dtos: []*prompt.VariableVal{
				{
					Key:   ptr.Of("var1"),
					Value: ptr.Of("value1"),
				},
			},
			want: []*entity.VariableVal{
				{
					Key:   "var1",
					Value: ptr.Of("value1"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, BatchVariableValDTO2DO(tt.dtos))
		})
	}
}

func TestVariableValDTO2DO(t *testing.T) {
	t.Parallel()
	assert.Nil(t, VariableValDTO2DO(nil))
	result := VariableValDTO2DO(&prompt.VariableVal{
		Key:   ptr.Of("k1"),
		Value: ptr.Of("v1"),
		PlaceholderMessages: []*prompt.Message{
			{
				Role:    ptr.Of(prompt.RoleUser),
				Content: ptr.Of("hello"),
			},
		},
		MultiPartValues: []*prompt.ContentPart{
			{
				Type: ptr.Of(prompt.ContentTypeText),
				Text: ptr.Of("text-val"),
			},
		},
	})
	assert.NotNil(t, result)
	assert.Equal(t, "k1", result.Key)
	assert.Equal(t, ptr.Of("v1"), result.Value)
	assert.Len(t, result.PlaceholderMessages, 1)
	assert.Len(t, result.MultiPartValues, 1)
}

func TestBatchVariableValDO2DTO(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		dos  []*entity.VariableVal
		want []*prompt.VariableVal
	}{
		{
			name: "nil",
			dos:  nil,
			want: nil,
		},
		{
			name: "with nil elements",
			dos:  []*entity.VariableVal{nil},
			want: []*prompt.VariableVal{},
		},
		{
			name: "normal",
			dos: []*entity.VariableVal{
				{
					Key:   "k1",
					Value: ptr.Of("v1"),
				},
			},
			want: []*prompt.VariableVal{
				{
					Key:   ptr.Of("k1"),
					Value: ptr.Of("v1"),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, BatchVariableValDO2DTO(tt.dos))
		})
	}
}

func TestVariableValDO2DTO(t *testing.T) {
	t.Parallel()
	assert.Nil(t, VariableValDO2DTO(nil))
	result := VariableValDO2DTO(&entity.VariableVal{
		Key:   "k1",
		Value: ptr.Of("v1"),
		PlaceholderMessages: []*entity.Message{
			{
				Role:    entity.RoleUser,
				Content: ptr.Of("hello"),
			},
		},
		MultiPartValues: []*entity.ContentPart{
			{
				Type: entity.ContentTypeText,
				Text: ptr.Of("text-val"),
			},
		},
	})
	assert.NotNil(t, result)
	assert.Equal(t, ptr.Of("k1"), result.Key)
	assert.Equal(t, ptr.Of("v1"), result.Value)
	assert.Len(t, result.PlaceholderMessages, 1)
	assert.Len(t, result.MultiPartValues, 1)
}

func TestBatchPromptCommitDO2DTO(t *testing.T) {
	t.Parallel()
	now := time.Now()
	tests := []struct {
		name string
		dos  []*entity.PromptCommit
		want []*prompt.PromptCommit
	}{
		{
			name: "nil",
			dos:  nil,
			want: nil,
		},
		{
			name: "empty",
			dos:  []*entity.PromptCommit{},
			want: nil,
		},
		{
			name: "with nil elements",
			dos:  []*entity.PromptCommit{nil},
			want: []*prompt.PromptCommit{},
		},
		{
			name: "normal",
			dos: []*entity.PromptCommit{
				{
					CommitInfo: &entity.CommitInfo{
						Version:     "1.0",
						BaseVersion: "0.9",
						Description: "desc",
						CommittedBy: "user1",
						CommittedAt: now,
					},
				},
			},
			want: []*prompt.PromptCommit{
				{
					CommitInfo: &prompt.CommitInfo{
						Version:     ptr.Of("1.0"),
						BaseVersion: ptr.Of("0.9"),
						Description: ptr.Of("desc"),
						CommittedBy: ptr.Of("user1"),
						CommittedAt: ptr.Of(now.UnixMilli()),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, BatchPromptCommitDO2DTO(tt.dos))
		})
	}
}

func TestPromptCommitDO2DTO(t *testing.T) {
	t.Parallel()
	assert.Nil(t, PromptCommitDO2DTO(nil))
	now := time.Now()
	result := PromptCommitDO2DTO(&entity.PromptCommit{
		CommitInfo: &entity.CommitInfo{
			Version:     "2.0",
			CommittedAt: now,
		},
		PromptDetail: &entity.PromptDetail{
			ExtInfos: map[string]string{"k": "v"},
		},
	})
	assert.NotNil(t, result)
	assert.Equal(t, ptr.Of("2.0"), result.CommitInfo.Version)
	assert.NotNil(t, result.Detail)
	assert.Equal(t, map[string]string{"k": "v"}, result.Detail.ExtInfos)
}

func TestCommitInfoDO2DTO(t *testing.T) {
	t.Parallel()
	assert.Nil(t, CommitInfoDO2DTO(nil))
	now := time.Now()
	result := CommitInfoDO2DTO(&entity.CommitInfo{
		Version:     "1.0",
		BaseVersion: "0.9",
		Description: "init",
		CommittedBy: "user",
		CommittedAt: now,
	})
	assert.NotNil(t, result)
	assert.Equal(t, ptr.Of("1.0"), result.Version)
	assert.Equal(t, ptr.Of("0.9"), result.BaseVersion)
	assert.Equal(t, ptr.Of("init"), result.Description)
	assert.Equal(t, ptr.Of("user"), result.CommittedBy)
	assert.Equal(t, ptr.Of(now.UnixMilli()), result.CommittedAt)
}

func TestPromptDetailDO2DTO_WithMcpAndExtInfos(t *testing.T) {
	t.Parallel()
	assert.Nil(t, PromptDetailDO2DTO(nil))
	result := PromptDetailDO2DTO(&entity.PromptDetail{
		McpConfig: &entity.McpConfig{
			IsMcpCallAutoRetry: ptr.Of(true),
			McpServers: []*entity.McpServerCombine{
				{
					McpServerID:   ptr.Of(int64(1)),
					AccessPointID: ptr.Of(int64(2)),
				},
			},
		},
		ExtInfos: map[string]string{"ext_key": "ext_val"},
	})
	assert.NotNil(t, result)
	assert.NotNil(t, result.McpConfig)
	assert.Equal(t, ptr.Of(true), result.McpConfig.IsMcpCallAutoRetry)
	assert.Len(t, result.McpConfig.McpServers, 1)
	assert.Equal(t, map[string]string{"ext_key": "ext_val"}, result.ExtInfos)
}

func TestMcpConfigDO2DTO(t *testing.T) {
	t.Parallel()
	assert.Nil(t, McpConfigDO2DTO(nil))
	result := McpConfigDO2DTO(&entity.McpConfig{
		IsMcpCallAutoRetry: ptr.Of(false),
		McpServers: []*entity.McpServerCombine{
			{
				McpServerID:    ptr.Of(int64(10)),
				AccessPointID:  ptr.Of(int64(20)),
				DisabledTools:  []string{"d1"},
				EnabledTools:   []string{"e1"},
				IsEnabledTools: ptr.Of(true),
			},
		},
	})
	assert.NotNil(t, result)
	assert.Equal(t, ptr.Of(false), result.IsMcpCallAutoRetry)
	assert.Len(t, result.McpServers, 1)
	assert.Equal(t, ptr.Of(int64(10)), result.McpServers[0].McpServerID)
}

func TestBatchMcpServerCombineDO2DTO(t *testing.T) {
	t.Parallel()
	assert.Nil(t, BatchMcpServerCombineDO2DTO(nil))
	result := BatchMcpServerCombineDO2DTO([]*entity.McpServerCombine{nil, {McpServerID: ptr.Of(int64(5))}})
	assert.Len(t, result, 1)
	assert.Equal(t, ptr.Of(int64(5)), result[0].McpServerID)
}

func TestMcpServerCombineDO2DTO(t *testing.T) {
	t.Parallel()
	assert.Nil(t, McpServerCombineDO2DTO(nil))
	result := McpServerCombineDO2DTO(&entity.McpServerCombine{
		McpServerID:    ptr.Of(int64(1)),
		AccessPointID:  ptr.Of(int64(2)),
		DisabledTools:  []string{"a"},
		EnabledTools:   []string{"b"},
		IsEnabledTools: ptr.Of(true),
	})
	assert.NotNil(t, result)
	assert.Equal(t, ptr.Of(int64(1)), result.McpServerID)
	assert.Equal(t, ptr.Of(int64(2)), result.AccessPointID)
	assert.Equal(t, []string{"a"}, result.DisabledTools)
	assert.Equal(t, []string{"b"}, result.EnabledTools)
	assert.Equal(t, ptr.Of(true), result.IsEnabledTools)
}

func TestToolChoiceTypeDTO2DO_NoneAndSpecific(t *testing.T) {
	t.Parallel()
	assert.Equal(t, entity.ToolChoiceTypeNone, ToolChoiceTypeDTO2DO(prompt.ToolChoiceTypeNone))
	assert.Equal(t, entity.ToolChoiceTypeSpecific, ToolChoiceTypeDTO2DO(prompt.ToolChoiceTypeSpecific))
}

func TestScenarioDTO2DO_EvalTarget(t *testing.T) {
	t.Parallel()
	assert.Equal(t, entity.ScenarioEvalTarget, ScenarioDTO2DO(prompt.ScenarioEvalTarget))
}

func TestBatchMcpServerCombineDTO2DO_WithNilElements(t *testing.T) {
	t.Parallel()
	assert.Nil(t, BatchMcpServerCombineDTO2DO(nil))
	result := BatchMcpServerCombineDTO2DO([]*prompt.McpServerCombine{nil, {McpServerID: ptr.Of(int64(3))}})
	assert.Len(t, result, 1)
	assert.Equal(t, ptr.Of(int64(3)), result[0].McpServerID)
}

func TestPromptDetailDTO2DO_WithMcpAndExtInfos(t *testing.T) {
	t.Parallel()
	assert.Nil(t, PromptDetailDTO2DO(nil))
	result := PromptDetailDTO2DO(&prompt.PromptDetail{
		McpConfig: &prompt.McpConfig{
			IsMcpCallAutoRetry: ptr.Of(true),
			McpServers: []*prompt.McpServerCombine{
				{McpServerID: ptr.Of(int64(99))},
			},
		},
		ExtInfos: map[string]string{"info_key": "info_val"},
	})
	assert.NotNil(t, result)
	assert.NotNil(t, result.McpConfig)
	assert.Equal(t, ptr.Of(true), result.McpConfig.IsMcpCallAutoRetry)
	assert.Len(t, result.McpConfig.McpServers, 1)
	assert.Equal(t, map[string]string{"info_key": "info_val"}, result.ExtInfos)
}

func TestBatchMessageDO2DTO_WithNilElements(t *testing.T) {
	t.Parallel()
	assert.Nil(t, BatchMessageDO2DTO(nil))
	assert.Nil(t, BatchMessageDO2DTO([]*entity.Message{}))
	result := BatchMessageDO2DTO([]*entity.Message{nil, {Role: entity.RoleUser, Content: ptr.Of("hi")}})
	assert.Len(t, result, 1)
	assert.Equal(t, ptr.Of(prompt.RoleUser), result[0].Role)
}

func TestBatchToolDO2DTO_WithNilElements(t *testing.T) {
	t.Parallel()
	assert.Nil(t, BatchToolDO2DTO(nil))
	assert.Nil(t, BatchToolDO2DTO([]*entity.Tool{}))
	result := BatchToolDO2DTO([]*entity.Tool{nil, {Type: entity.ToolTypeFunction}})
	assert.Len(t, result, 1)
}

func TestBatchToolCallDO2DTO_WithNilElements(t *testing.T) {
	t.Parallel()
	assert.Nil(t, BatchToolCallDO2DTO(nil))
	result := BatchToolCallDO2DTO([]*entity.ToolCall{nil, {ID: "tc1", Type: entity.ToolTypeFunction}})
	assert.Len(t, result, 1)
	assert.Equal(t, ptr.Of("tc1"), result[0].ID)
}

func TestBatchContentPartDO2DTO_WithNilElements(t *testing.T) {
	t.Parallel()
	assert.Nil(t, BatchContentPartDO2DTO(nil))
	result := BatchContentPartDO2DTO([]*entity.ContentPart{nil, {Type: entity.ContentTypeText, Text: ptr.Of("t")}})
	assert.Len(t, result, 1)
}

func TestBatchVariableDefDO2DTO_WithNilElements(t *testing.T) {
	t.Parallel()
	assert.Nil(t, BatchVariableDefDO2DTO(nil))
	assert.Nil(t, BatchVariableDefDO2DTO([]*entity.VariableDef{}))
	result := BatchVariableDefDO2DTO([]*entity.VariableDef{nil, {Key: "k", Type: entity.VariableTypeString}})
	assert.Len(t, result, 1)
	assert.Equal(t, ptr.Of("k"), result[0].Key)
}
