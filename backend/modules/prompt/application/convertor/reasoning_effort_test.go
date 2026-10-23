// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	domainopenapi "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain_openapi/prompt"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestReasoningEffort_OpenAPIExactRoundTrip(t *testing.T) {
	assert.Nil(t, OpenAPIReasoningEffortDTO2DO(nil))
	assert.Nil(t, OpenAPIReasoningEffortDO2DTO(nil))
	for _, value := range []string{"", "xhigh", "XHigh", "tier_v2", "none", "1", "true", "minimal", "low", "medium", "high"} {
		t.Run(value, func(t *testing.T) {
			dto := &domainopenapi.ThinkingConfig{ThinkingOption: ptr.Of(domainopenapi.ThinkingOptionEnabled), ReasoningEffort: ptr.Of(domainopenapi.ReasoningEffort(value))}
			do := OpenAPIThinkingConfigDTO2DO(dto)
			require.NotNil(t, do.ReasoningEffort)
			assert.Equal(t, entity.ReasoningEffort(value), *do.ReasoningEffort)
			got := OpenAPIThinkingConfigDO2DTO(do)
			require.NotNil(t, got.ReasoningEffort)
			assert.Equal(t, domainopenapi.ReasoningEffort(value), *got.ReasoningEffort)
		})
	}
}
