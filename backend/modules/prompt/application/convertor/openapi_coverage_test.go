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
