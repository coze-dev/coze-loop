// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	druntime "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/domain/runtime"
	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/entity"
)

func TestModelAndTools2OptionDOs(t *testing.T) {
	t.Run("full input", func(t *testing.T) {
		modelCfg := &druntime.ModelConfig{
			Temperature:      gptr.Of(float64(0.7)),
			MaxTokens:        gptr.Of(int64(100)),
			TopP:             gptr.Of(float64(0.9)),
			Stop:             []string{"stop1"},
			ToolChoice:       gptr.Of(druntime.ToolChoiceAuto),
			ResponseFormat:   &druntime.ResponseFormat{Type: gptr.Of(druntime.ResponseFormatJSONObject)},
			TopK:             gptr.Of(int32(10)),
			PresencePenalty:  gptr.Of(float64(0.5)),
			FrequencyPenalty: gptr.Of(float64(0.6)),
		}
		tools := []*druntime.Tool{
			{
				Name: gptr.Of("tool1"),
			},
		}
		parameters := map[string]string{"key1": "value1"}
		paramValues := map[string]*entity.ParamValue{"pv1": {Value: "v1"}}

		got := ModelAndTools2OptionDOs(modelCfg, tools, parameters, paramValues)
		assert.NotEmpty(t, got)
	})

	t.Run("nil input", func(t *testing.T) {
		got := ModelAndTools2OptionDOs(nil, nil, nil, nil)
		assert.Empty(t, got)
	})
}

func TestToolsDTO2DO(t *testing.T) {
	ts := []*druntime.Tool{
		{
			Name: gptr.Of("tool1"),
		},
	}
	got := ToolsDTO2DO(ts)
	assert.Len(t, got, 1)
	assert.Equal(t, "tool1", got[0].Name)
}

func TestResponseFormatDTO2DO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, ResponseFormatDTO2DO(nil))
	})
	t.Run("valid input", func(t *testing.T) {
		r := &druntime.ResponseFormat{
			Type: gptr.Of(druntime.ResponseFormatJSONObject),
		}
		got := ResponseFormatDTO2DO(r)
		assert.Equal(t, entity.ResponseFormatType(r.GetType()), got.Type)
	})
}

func TestToolDTO2DO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, ToolDTO2DO(nil))
	})
	t.Run("valid input", func(t *testing.T) {
		t1 := &druntime.Tool{
			Name: gptr.Of("tool1"),
			Desc: gptr.Of("desc1"),
		}
		got := ToolDTO2DO(t1)
		assert.Equal(t, *t1.Name, got.Name)
		assert.Equal(t, *t1.Desc, got.Desc)
	})
}

func TestToolChoiceDTO2DO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, ToolChoiceDTO2DO(nil))
	})
	t.Run("valid input", func(t *testing.T) {
		tc := druntime.ToolChoiceAuto
		got := ToolChoiceDTO2DO(&tc)
		assert.Equal(t, entity.ToolChoice(tc), *got)
	})
}
