// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	domainopenapi "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain_openapi/prompt"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestOpenAPIThinkingOptionDTO2DO_DefaultBranch(t *testing.T) {
	t.Parallel()
	unknown := domainopenapi.ThinkingOption("unknown")
	result := OpenAPIThinkingOptionDTO2DO(&unknown)
	assert.Nil(t, result)
}

func TestOpenAPIReasoningEffortDTO2DO_DefaultBranch(t *testing.T) {
	t.Parallel()
	unknown := domainopenapi.ReasoningEffort("unknown")
	result := OpenAPIReasoningEffortDTO2DO(&unknown)
	assert.Nil(t, result)
}

func TestOpenAPIThinkingOptionDO2DTO_DefaultBranch(t *testing.T) {
	t.Parallel()
	unknown := entity.ThinkingOption("unknown")
	result := OpenAPIThinkingOptionDO2DTO(&unknown)
	assert.Nil(t, result)
}

func TestOpenAPIReasoningEffortDO2DTO_DefaultBranch(t *testing.T) {
	t.Parallel()
	unknown := entity.ReasoningEffort("unknown")
	result := OpenAPIReasoningEffortDO2DTO(&unknown)
	assert.Nil(t, result)
}

func TestOpenAPIContentPartDO2DTO_VideoURLEmptyString(t *testing.T) {
	t.Parallel()
	do := &entity.ContentPart{
		Type: entity.ContentTypeVideoURL,
		VideoURL: &entity.VideoURL{
			URL: "",
		},
	}
	result := OpenAPIContentPartDO2DTO(do)
	assert.NotNil(t, result)
	assert.Nil(t, result.VideoURL)
}

func TestOpenAPIContentPartDO2DTO_MediaConfigNil(t *testing.T) {
	t.Parallel()
	do := &entity.ContentPart{
		Type:        entity.ContentTypeText,
		Text:        ptr.Of("hello"),
		MediaConfig: nil,
	}
	result := OpenAPIContentPartDO2DTO(do)
	assert.NotNil(t, result)
	assert.Nil(t, result.Config)
}

func TestOpenAPIContentPartDTO2DO_ImageURLNil(t *testing.T) {
	t.Parallel()
	dto := &domainopenapi.ContentPart{
		Type:     ptr.Of(domainopenapi.ContentTypeText),
		Text:     ptr.Of("hello"),
		ImageURL: nil,
	}
	result := OpenAPIContentPartDTO2DO(dto)
	assert.NotNil(t, result)
	assert.Nil(t, result.ImageURL)
}

func TestOpenAPIContentPartDTO2DO_VideoURLNil(t *testing.T) {
	t.Parallel()
	dto := &domainopenapi.ContentPart{
		Type:     ptr.Of(domainopenapi.ContentTypeText),
		Text:     ptr.Of("hello"),
		VideoURL: nil,
	}
	result := OpenAPIContentPartDTO2DO(dto)
	assert.NotNil(t, result)
	assert.Nil(t, result.VideoURL)
}

func TestOpenAPIBatchVariableDefDO2DTO_EmptySlice(t *testing.T) {
	t.Parallel()
	dos := make([]*entity.VariableDef, 0)
	result := OpenAPIBatchVariableDefDO2DTO(dos)
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestOpenAPIPromptBasicDO2DTO_LatestCommittedAtNotNil(t *testing.T) {
	t.Parallel()
	now := time.Now()
	do := &entity.Prompt{
		ID:        1,
		SpaceID:   2,
		PromptKey: "key",
		PromptBasic: &entity.PromptBasic{
			DisplayName:       "test",
			Description:       "desc",
			LatestVersion:     "v1",
			CreatedBy:         "user1",
			UpdatedBy:         "user2",
			CreatedAt:         now,
			UpdatedAt:         now,
			LatestCommittedAt: &now,
		},
	}
	result := OpenAPIPromptBasicDO2DTO(do)
	assert.NotNil(t, result)
	assert.NotNil(t, result.LatestCommittedAt)
	assert.Equal(t, now.UnixMilli(), *result.LatestCommittedAt)
}

func TestOpenAPIBatchToolDTO2DO_NilSlice(t *testing.T) {
	t.Parallel()
	result := OpenAPIBatchToolDTO2DO(nil)
	assert.Nil(t, result)
}

func TestOpenAPIBatchParamConfigValueDTO2DO_NilSlice(t *testing.T) {
	t.Parallel()
	result := OpenAPIBatchParamConfigValueDTO2DO(nil)
	assert.Nil(t, result)
}

func TestOpenAPIContentTypeDO2DTO_Base64Data(t *testing.T) {
	t.Parallel()
	result := OpenAPIContentTypeDO2DTO(entity.ContentTypeBase64Data)
	assert.Equal(t, domainopenapi.ContentTypeBase64Data, result)
}

func TestOpenAPIContentTypeDTO2DO_Base64Data(t *testing.T) {
	t.Parallel()
	result := OpenAPIContentTypeDTO2DO(domainopenapi.ContentTypeBase64Data)
	assert.Equal(t, entity.ContentTypeBase64Data, result)
}

func TestOpenAPIThinkingConfigDTO2DO(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		dto  *domainopenapi.ThinkingConfig
		want *entity.ThinkingConfig
	}{
		{
			name: "nil input",
			dto:  nil,
			want: nil,
		},
		{
			name: "all fields set",
			dto: &domainopenapi.ThinkingConfig{
				BudgetTokens:    ptr.Of(int64(512)),
				ThinkingOption:  ptr.Of(domainopenapi.ThinkingOptionAuto),
				ReasoningEffort: ptr.Of(domainopenapi.ReasoningEffortMedium),
			},
			want: &entity.ThinkingConfig{
				BudgetTokens:    ptr.Of(int64(512)),
				ThinkingOption:  ptr.Of(entity.ThinkingOptionAuto),
				ReasoningEffort: ptr.Of(entity.ReasoningEffortMedium),
			},
		},
		{
			name: "only budget tokens",
			dto: &domainopenapi.ThinkingConfig{
				BudgetTokens: ptr.Of(int64(1024)),
			},
			want: &entity.ThinkingConfig{
				BudgetTokens: ptr.Of(int64(1024)),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, OpenAPIThinkingConfigDTO2DO(tt.dto))
		})
	}
}

func TestOpenAPIThinkingConfigDO2DTO(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		do   *entity.ThinkingConfig
		want *domainopenapi.ThinkingConfig
	}{
		{
			name: "nil input",
			do:   nil,
			want: nil,
		},
		{
			name: "all fields set",
			do: &entity.ThinkingConfig{
				BudgetTokens:    ptr.Of(int64(256)),
				ThinkingOption:  ptr.Of(entity.ThinkingOptionDisabled),
				ReasoningEffort: ptr.Of(entity.ReasoningEffortMinimal),
			},
			want: &domainopenapi.ThinkingConfig{
				BudgetTokens:    ptr.Of(int64(256)),
				ThinkingOption:  ptr.Of(domainopenapi.ThinkingOptionDisabled),
				ReasoningEffort: ptr.Of(domainopenapi.ReasoningEffortMinimal),
			},
		},
		{
			name: "only budget tokens",
			do: &entity.ThinkingConfig{
				BudgetTokens: ptr.Of(int64(64)),
			},
			want: &domainopenapi.ThinkingConfig{
				BudgetTokens: ptr.Of(int64(64)),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, OpenAPIThinkingConfigDO2DTO(tt.do))
		})
	}
}

func TestOpenAPIThinkingOptionDTO2DO_AllBranches(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		dto  *domainopenapi.ThinkingOption
		want *entity.ThinkingOption
	}{
		{
			name: "nil input",
			dto:  nil,
			want: nil,
		},
		{
			name: "disabled",
			dto:  ptr.Of(domainopenapi.ThinkingOptionDisabled),
			want: ptr.Of(entity.ThinkingOptionDisabled),
		},
		{
			name: "enabled",
			dto:  ptr.Of(domainopenapi.ThinkingOptionEnabled),
			want: ptr.Of(entity.ThinkingOptionEnabled),
		},
		{
			name: "auto",
			dto:  ptr.Of(domainopenapi.ThinkingOptionAuto),
			want: ptr.Of(entity.ThinkingOptionAuto),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, OpenAPIThinkingOptionDTO2DO(tt.dto))
		})
	}
}

func TestOpenAPIReasoningEffortDTO2DO_AllBranches(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		dto  *domainopenapi.ReasoningEffort
		want *entity.ReasoningEffort
	}{
		{
			name: "nil input",
			dto:  nil,
			want: nil,
		},
		{
			name: "minimal",
			dto:  ptr.Of(domainopenapi.ReasoningEffortMinimal),
			want: ptr.Of(entity.ReasoningEffortMinimal),
		},
		{
			name: "low",
			dto:  ptr.Of(domainopenapi.ReasoningEffortLow),
			want: ptr.Of(entity.ReasoningEffortLow),
		},
		{
			name: "medium",
			dto:  ptr.Of(domainopenapi.ReasoningEffortMedium),
			want: ptr.Of(entity.ReasoningEffortMedium),
		},
		{
			name: "high",
			dto:  ptr.Of(domainopenapi.ReasoningEffortHigh),
			want: ptr.Of(entity.ReasoningEffortHigh),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, OpenAPIReasoningEffortDTO2DO(tt.dto))
		})
	}
}

func TestOpenAPIThinkingOptionDO2DTO_AllBranches(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		do   *entity.ThinkingOption
		want *domainopenapi.ThinkingOption
	}{
		{
			name: "nil input",
			do:   nil,
			want: nil,
		},
		{
			name: "disabled",
			do:   ptr.Of(entity.ThinkingOptionDisabled),
			want: ptr.Of(domainopenapi.ThinkingOptionDisabled),
		},
		{
			name: "enabled",
			do:   ptr.Of(entity.ThinkingOptionEnabled),
			want: ptr.Of(domainopenapi.ThinkingOptionEnabled),
		},
		{
			name: "auto",
			do:   ptr.Of(entity.ThinkingOptionAuto),
			want: ptr.Of(domainopenapi.ThinkingOptionAuto),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, OpenAPIThinkingOptionDO2DTO(tt.do))
		})
	}
}

func TestOpenAPIReasoningEffortDO2DTO_AllBranches(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		do   *entity.ReasoningEffort
		want *domainopenapi.ReasoningEffort
	}{
		{
			name: "nil input",
			do:   nil,
			want: nil,
		},
		{
			name: "minimal",
			do:   ptr.Of(entity.ReasoningEffortMinimal),
			want: ptr.Of(domainopenapi.ReasoningEffortMinimal),
		},
		{
			name: "low",
			do:   ptr.Of(entity.ReasoningEffortLow),
			want: ptr.Of(domainopenapi.ReasoningEffortLow),
		},
		{
			name: "medium",
			do:   ptr.Of(entity.ReasoningEffortMedium),
			want: ptr.Of(domainopenapi.ReasoningEffortMedium),
		},
		{
			name: "high",
			do:   ptr.Of(entity.ReasoningEffortHigh),
			want: ptr.Of(domainopenapi.ReasoningEffortHigh),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, OpenAPIReasoningEffortDO2DTO(tt.do))
		})
	}
}

func TestOpenAPIBatchParamConfigValueDTO2DO_AllCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		dtos []*domainopenapi.ParamConfigValue
		want []*entity.ParamConfigValue
	}{
		{
			name: "nil input",
			dtos: nil,
			want: nil,
		},
		{
			name: "normal values",
			dtos: []*domainopenapi.ParamConfigValue{
				{
					Name:  ptr.Of("top_p"),
					Label: ptr.Of("Top P"),
					Value: &domainopenapi.ParamOption{
						Value: ptr.Of("0.9"),
						Label: ptr.Of("0.9"),
					},
				},
				{
					Name:  ptr.Of("top_k"),
					Label: ptr.Of("Top K"),
					Value: &domainopenapi.ParamOption{
						Value: ptr.Of("50"),
						Label: ptr.Of("50"),
					},
				},
			},
			want: []*entity.ParamConfigValue{
				{
					Name:  "top_p",
					Label: "Top P",
					Value: &entity.ParamOption{
						Value: "0.9",
						Label: "0.9",
					},
				},
				{
					Name:  "top_k",
					Label: "Top K",
					Value: &entity.ParamOption{
						Value: "50",
						Label: "50",
					},
				},
			},
		},
		{
			name: "with nil elements",
			dtos: []*domainopenapi.ParamConfigValue{
				nil,
				{
					Name:  ptr.Of("param1"),
					Label: ptr.Of("Param 1"),
				},
				nil,
			},
			want: []*entity.ParamConfigValue{
				{
					Name:  "param1",
					Label: "Param 1",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, OpenAPIBatchParamConfigValueDTO2DO(tt.dtos))
		})
	}
}

func TestOpenAPIParamConfigValueDTO2DO(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		dto  *domainopenapi.ParamConfigValue
		want *entity.ParamConfigValue
	}{
		{
			name: "nil input",
			dto:  nil,
			want: nil,
		},
		{
			name: "with value",
			dto: &domainopenapi.ParamConfigValue{
				Name:  ptr.Of("temperature"),
				Label: ptr.Of("Temperature"),
				Value: &domainopenapi.ParamOption{
					Value: ptr.Of("0.7"),
					Label: ptr.Of("0.7"),
				},
			},
			want: &entity.ParamConfigValue{
				Name:  "temperature",
				Label: "Temperature",
				Value: &entity.ParamOption{
					Value: "0.7",
					Label: "0.7",
				},
			},
		},
		{
			name: "without value",
			dto: &domainopenapi.ParamConfigValue{
				Name:  ptr.Of("mode"),
				Label: ptr.Of("Mode"),
				Value: nil,
			},
			want: &entity.ParamConfigValue{
				Name:  "mode",
				Label: "Mode",
				Value: nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, OpenAPIParamConfigValueDTO2DO(tt.dto))
		})
	}
}

func TestOpenAPIParamOptionDTO2DO(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		dto  *domainopenapi.ParamOption
		want *entity.ParamOption
	}{
		{
			name: "nil input",
			dto:  nil,
			want: nil,
		},
		{
			name: "normal option",
			dto: &domainopenapi.ParamOption{
				Value: ptr.Of("0.95"),
				Label: ptr.Of("0.95"),
			},
			want: &entity.ParamOption{
				Value: "0.95",
				Label: "0.95",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, OpenAPIParamOptionDTO2DO(tt.dto))
		})
	}
}

func TestOpenAPIToolChoiceSpecificationDTO2DO(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		dto  *domainopenapi.ToolChoiceSpecification
		want *entity.ToolChoiceSpecification
	}{
		{
			name: "nil input",
			dto:  nil,
			want: nil,
		},
		{
			name: "function type specification",
			dto: &domainopenapi.ToolChoiceSpecification{
				Type: ptr.Of(domainopenapi.ToolTypeFunction),
				Name: ptr.Of("get_weather"),
			},
			want: &entity.ToolChoiceSpecification{
				Type: entity.ToolTypeFunction,
				Name: "get_weather",
			},
		},
		{
			name: "google search type specification",
			dto: &domainopenapi.ToolChoiceSpecification{
				Type: ptr.Of(domainopenapi.ToolTypeGoogleSearch),
				Name: ptr.Of("search"),
			},
			want: &entity.ToolChoiceSpecification{
				Type: entity.ToolTypeGoogleSearch,
				Name: "search",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, OpenAPIToolChoiceSpecificationDTO2DO(tt.dto))
		})
	}
}

func TestOpenAPIContentPartDTO2DO_AllBranches(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		dto  *domainopenapi.ContentPart
		want *entity.ContentPart
	}{
		{
			name: "nil input",
			dto:  nil,
			want: nil,
		},
		{
			name: "image url with value",
			dto: &domainopenapi.ContentPart{
				Type:     ptr.Of(domainopenapi.ContentTypeImageURL),
				ImageURL: ptr.Of("https://example.com/img.png"),
			},
			want: &entity.ContentPart{
				Type: entity.ContentTypeImageURL,
				ImageURL: &entity.ImageURL{
					URL: "https://example.com/img.png",
				},
			},
		},
		{
			name: "image url empty string",
			dto: &domainopenapi.ContentPart{
				Type:     ptr.Of(domainopenapi.ContentTypeImageURL),
				ImageURL: ptr.Of(""),
			},
			want: &entity.ContentPart{
				Type: entity.ContentTypeImageURL,
			},
		},
		{
			name: "video url with value",
			dto: &domainopenapi.ContentPart{
				Type:     ptr.Of(domainopenapi.ContentTypeVideoURL),
				VideoURL: ptr.Of("https://example.com/video.mp4"),
			},
			want: &entity.ContentPart{
				Type: entity.ContentTypeVideoURL,
				VideoURL: &entity.VideoURL{
					URL: "https://example.com/video.mp4",
				},
			},
		},
		{
			name: "video url empty string",
			dto: &domainopenapi.ContentPart{
				Type:     ptr.Of(domainopenapi.ContentTypeVideoURL),
				VideoURL: ptr.Of(""),
			},
			want: &entity.ContentPart{
				Type: entity.ContentTypeVideoURL,
			},
		},
		{
			name: "base64 data",
			dto: &domainopenapi.ContentPart{
				Type:       ptr.Of(domainopenapi.ContentTypeBase64Data),
				Base64Data: ptr.Of("data:image/png;base64,abc123"),
			},
			want: &entity.ContentPart{
				Type:       entity.ContentTypeBase64Data,
				Base64Data: ptr.Of("data:image/png;base64,abc123"),
			},
		},
		{
			name: "with config fps",
			dto: &domainopenapi.ContentPart{
				Type:     ptr.Of(domainopenapi.ContentTypeVideoURL),
				VideoURL: ptr.Of("https://example.com/video.mp4"),
				Config: &domainopenapi.MediaConfig{
					Fps: ptr.Of(3.0),
				},
			},
			want: &entity.ContentPart{
				Type: entity.ContentTypeVideoURL,
				VideoURL: &entity.VideoURL{
					URL: "https://example.com/video.mp4",
				},
				MediaConfig: &entity.MediaConfig{
					Fps: ptr.Of(3.0),
				},
			},
		},
		{
			name: "config with nil fps",
			dto: &domainopenapi.ContentPart{
				Type:     ptr.Of(domainopenapi.ContentTypeVideoURL),
				VideoURL: ptr.Of("https://example.com/video.mp4"),
				Config: &domainopenapi.MediaConfig{
					Fps: nil,
				},
			},
			want: &entity.ContentPart{
				Type: entity.ContentTypeVideoURL,
				VideoURL: &entity.VideoURL{
					URL: "https://example.com/video.mp4",
				},
			},
		},
		{
			name: "nil config",
			dto: &domainopenapi.ContentPart{
				Type:     ptr.Of(domainopenapi.ContentTypeVideoURL),
				VideoURL: ptr.Of("https://example.com/video.mp4"),
				Config:   nil,
			},
			want: &entity.ContentPart{
				Type: entity.ContentTypeVideoURL,
				VideoURL: &entity.VideoURL{
					URL: "https://example.com/video.mp4",
				},
			},
		},
		{
			name: "with signature",
			dto: &domainopenapi.ContentPart{
				Type:      ptr.Of(domainopenapi.ContentTypeText),
				Text:      ptr.Of("hello"),
				Signature: ptr.Of("sig123"),
			},
			want: &entity.ContentPart{
				Type:      entity.ContentTypeText,
				Text:      ptr.Of("hello"),
				Signature: ptr.Of("sig123"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, OpenAPIContentPartDTO2DO(tt.dto))
		})
	}
}

func TestOpenAPIModelConfigDTO2DO_NilInput(t *testing.T) {
	t.Parallel()
	assert.Nil(t, OpenAPIModelConfigDTO2DO(nil))
}

func TestOpenAPIContentTypeDO2DTO_Base64DataBranch(t *testing.T) {
	t.Parallel()
	assert.Equal(t, domainopenapi.ContentTypeBase64Data, OpenAPIContentTypeDO2DTO(entity.ContentTypeBase64Data))
}

func TestOpenAPIContentTypeDTO2DO_AllBranches(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		dto  domainopenapi.ContentType
		want entity.ContentType
	}{
		{
			name: "text",
			dto:  domainopenapi.ContentTypeText,
			want: entity.ContentTypeText,
		},
		{
			name: "image url",
			dto:  domainopenapi.ContentTypeImageURL,
			want: entity.ContentTypeImageURL,
		},
		{
			name: "video url",
			dto:  domainopenapi.ContentTypeVideoURL,
			want: entity.ContentTypeVideoURL,
		},
		{
			name: "base64 data",
			dto:  domainopenapi.ContentTypeBase64Data,
			want: entity.ContentTypeBase64Data,
		},
		{
			name: "multi part variable",
			dto:  domainopenapi.ContentTypeMultiPartVariable,
			want: entity.ContentTypeMultiPartVariable,
		},
		{
			name: "unknown defaults to text",
			dto:  domainopenapi.ContentType("unknown"),
			want: entity.ContentTypeText,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, OpenAPIContentTypeDTO2DO(tt.dto))
		})
	}
}

func TestOpenAPIContentPartDO2DTO_Base64DataWithFps(t *testing.T) {
	t.Parallel()
	do := &entity.ContentPart{
		Type:       entity.ContentTypeBase64Data,
		Base64Data: ptr.Of("data:video/mp4;base64,QUJD"),
		MediaConfig: &entity.MediaConfig{
			Fps: ptr.Of(1.5),
		},
	}
	result := OpenAPIContentPartDO2DTO(do)
	assert.NotNil(t, result)
	assert.Equal(t, ptr.Of(domainopenapi.ContentTypeBase64Data), result.Type)
	assert.Equal(t, ptr.Of("data:video/mp4;base64,QUJD"), result.Base64Data)
	assert.NotNil(t, result.Config)
	assert.Equal(t, ptr.Of(1.5), result.Config.Fps)
}

func TestOpenAPIContentPartDO2DTO_Signature(t *testing.T) {
	t.Parallel()
	do := &entity.ContentPart{
		Type:      entity.ContentTypeText,
		Text:      ptr.Of("thought"),
		Signature: ptr.Of("sig_abc"),
	}
	result := OpenAPIContentPartDO2DTO(do)
	assert.NotNil(t, result)
	assert.Equal(t, ptr.Of("sig_abc"), result.Signature)
}

func TestOpenAPIToolChoiceTypeDTO2DO(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		dto  domainopenapi.ToolChoiceType
		want entity.ToolChoiceType
	}{
		{
			name: "auto",
			dto:  domainopenapi.ToolChoiceTypeAuto,
			want: entity.ToolChoiceTypeAuto,
		},
		{
			name: "none",
			dto:  domainopenapi.ToolChoiceTypeNone,
			want: entity.ToolChoiceTypeNone,
		},
		{
			name: "specific",
			dto:  domainopenapi.ToolChoiceTypeSpecific,
			want: entity.ToolChoiceTypeSpecific,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, OpenAPIToolChoiceTypeDTO2DO(tt.dto))
		})
	}
}
