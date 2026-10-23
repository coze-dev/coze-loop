// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	webprompt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/prompt"
	domainopenapi "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain_openapi/prompt"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestReasoningEffort_OpenAPIExactRoundTrip(t *testing.T) {
	assert.Nil(t, OpenAPIReasoningEffortDTO2DO(nil))
	assert.Nil(t, OpenAPIReasoningEffortDO2DTO(nil))
	for _, value := range []string{"", "xhigh", "XHigh", "tier_v2", "none", "1", "true", "minimal", "low", "medium", "high"} {
		t.Run(value, func(t *testing.T) {
			dto := &domainopenapi.ThinkingConfig{ThinkingOption: ptr.Of(domainopenapi.ThinkingOptionEnabled), ReasoningEffortValue: ptr.Of(value)}
			do := OpenAPIThinkingConfigDTO2DO(dto)
			require.NotNil(t, do.ReasoningEffortValue)
			assert.Equal(t, value, *do.ReasoningEffortValue)
			assert.Nil(t, do.ReasoningEffort)
			got := OpenAPIThinkingConfigDO2DTO(do)
			require.NotNil(t, got.ReasoningEffortValue)
			assert.Equal(t, value, *got.ReasoningEffortValue)
			assert.Nil(t, got.ReasoningEffort)
			*got.ReasoningEffortValue = "changed"
			assert.Equal(t, value, *dto.ReasoningEffortValue)
			assert.Equal(t, value, *do.ReasoningEffortValue)
		})
	}
}

func TestWebThinkingValueRoundTripPreservesPresence(t *testing.T) {
	for _, value := range []*string{nil, ptr.Of(""), ptr.Of("high"), ptr.Of("Tier_V2")} {
		dto := &webprompt.ThinkingConfig{ReasoningEffortValue: value}
		do := ThinkingConfigDTO2DO(dto)
		require.Equal(t, value, do.ReasoningEffortValue)
		require.Nil(t, do.ReasoningEffort)
		got := ThinkingConfigDO2DTO(do)
		require.Equal(t, value, got.ReasoningEffortValue)
		if value != nil {
			original := *value
			*got.ReasoningEffortValue = "changed"
			require.Equal(t, original, *dto.ReasoningEffortValue)
			require.Equal(t, original, *do.ReasoningEffortValue)
		}
	}
}

func TestLegacyThinkingRoundTripDoesNotInjectValue(t *testing.T) {
	for _, value := range []entity.ReasoningEffort{entity.ReasoningEffortMinimal, entity.ReasoningEffortLow, entity.ReasoningEffortMedium, entity.ReasoningEffortHigh} {
		legacy := &entity.ThinkingConfig{ReasoningEffort: ptr.Of(value)}
		web := ThinkingConfigDO2DTO(legacy)
		require.Nil(t, web.ReasoningEffortValue)
		require.Equal(t, string(value), string(*web.ReasoningEffort))
		require.Nil(t, ThinkingConfigDTO2DO(web).ReasoningEffortValue)
		api := OpenAPIThinkingConfigDO2DTO(legacy)
		require.Nil(t, api.ReasoningEffortValue)
		require.Equal(t, string(value), string(*api.ReasoningEffort))
		require.Nil(t, OpenAPIThinkingConfigDTO2DO(api).ReasoningEffortValue)
	}
}
