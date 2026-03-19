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
