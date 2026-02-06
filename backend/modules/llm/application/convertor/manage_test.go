// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/domain/manage"
	manage2 "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/manage"
	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/entity"
)

func TestModelDO2DTO(t *testing.T) {
	model := &entity.Model{
		ID:          1,
		WorkspaceID: 2,
		Name:        "model1",
		Desc:        "desc1",
		Protocol:    "ark",
		ProtocolConfig: &entity.ProtocolConfig{
			BaseURL: "http://test.com",
		},
		Status:      entity.ModelStatusEnabled,
		PresetModel: true,
		CreatedAt:   123456,
	}

	t.Run("no mask", func(t *testing.T) {
		got := ModelDO2DTO(model, false)
		assert.NotNil(t, got)
		assert.Equal(t, model.ID, *got.ModelID)
		assert.NotNil(t, got.ProtocolConfig)
		assert.Equal(t, model.ProtocolConfig.BaseURL, *got.ProtocolConfig.BaseURL)
		assert.Equal(t, int64(123456), *got.CreatedAt)
	})

	t.Run("with mask", func(t *testing.T) {
		got := ModelDO2DTO(model, true)
		assert.NotNil(t, got)
		assert.Nil(t, got.ProtocolConfig)
	})

	t.Run("nil input", func(t *testing.T) {
		got := ModelDO2DTO(nil, false)
		assert.Nil(t, got)
	})
}

func TestModelsDO2DTO(t *testing.T) {
	models := []*entity.Model{
		{ID: 1},
		{ID: 2},
	}
	got := ModelsDO2DTO(models, false)
	assert.Len(t, got, 2)
	assert.Equal(t, int64(1), *got[0].ModelID)
	assert.Equal(t, int64(2), *got[1].ModelID)
}

func TestSeriesDO2DTO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		got := SeriesDO2DTO(nil)
		assert.Nil(t, got)
	})

	t.Run("valid input", func(t *testing.T) {
		v := &entity.Series{
			Name:   "series1",
			Icon:   "icon1",
			Family: entity.FamilySeed,
		}
		got := SeriesDO2DTO(v)
		assert.Equal(t, v.Name, *got.Name)
		assert.Equal(t, v.Icon, *got.Icon)
		assert.Equal(t, manage.FamilySeed, *got.Family)
	})
}

func TestFamilyDO2DTO(t *testing.T) {
	tests := []struct {
		name string
		from entity.Family
		want manage.Family
	}{
		{"seed", entity.FamilySeed, manage.FamilySeed},
		{"deepseek", entity.FamilyDeepSeek, manage.FamilyDeepseek},
		{"glm", entity.FamilyGLM, manage.FamilyGlm},
		{"kimi", entity.FamilyKimi, manage.FamilyKimi},
		{"doubao", entity.FamilyDoubao, manage.FamilyDoubao},
		{"undefined", "other", manage.FamilyUndefined},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, FamilyDO2DTO(tt.from))
		})
	}
}

func TestFamilyDTO2DO(t *testing.T) {
	tests := []struct {
		name string
		from manage.Family
		want entity.Family
	}{
		{"seed", manage.FamilySeed, entity.FamilySeed},
		{"deepseek", manage.FamilyDeepseek, entity.FamilyDeepSeek},
		{"glm", manage.FamilyGlm, entity.FamilyGLM},
		{"kimi", manage.FamilyKimi, entity.FamilyKimi},
		{"doubao", manage.FamilyDoubao, entity.FamilyDoubao},
		{"undefined", manage.FamilyUndefined, entity.FamilyUndefined},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, FamilyDTO2DO(tt.from))
		})
	}
}

func TestVisibilityDO2DTO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, VisibilityDO2DTO(nil))
	})
	t.Run("valid input", func(t *testing.T) {
		v := &entity.Visibility{
			Mode:     entity.VisibleModelAll,
			SpaceIDs: []int64{1, 2},
		}
		got := VisibilityDO2DTO(v)
		assert.Equal(t, manage.VisibleModeAll, *got.Mode)
		assert.Equal(t, v.SpaceIDs, got.SpaceIDs)
	})
}

func TestModelStatusConvert(t *testing.T) {
	assert.Equal(t, manage.ModelStatusAvailable, ModelStatusDO2DTO(entity.ModelStatusEnabled))
	assert.Equal(t, entity.ModelStatusEnabled, ModelStatusDTO2DO(manage.ModelStatusAvailable))
}

func TestAbilityDO2DTO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, AbilityDO2DTO(nil))
	})
	t.Run("valid input", func(t *testing.T) {
		a := &entity.Ability{
			MaxContextTokens: gptr.Of(int64(100)),
			FunctionCall:     true,
		}
		got := AbilityDO2DTO(a)
		assert.Equal(t, a.MaxContextTokens, got.MaxContextTokens)
		assert.True(t, *got.FunctionCall)
	})
}

func TestProtocolConfigArkDO2DTO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, ProtocolConfigArkDO2DTO(nil))
	})
	t.Run("valid input", func(t *testing.T) {
		p := &entity.ProtocolConfigArk{
			Region:    "cn-beijing",
			AccessKey: "ak",
		}
		got := ProtocolConfigArkDO2DTO(p)
		assert.Equal(t, p.Region, *got.Region)
		assert.Equal(t, p.AccessKey, *got.AccessKey)
	})
}

func TestListModelsFilterDTO2DO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, ListModelsFilterDTO2DO(nil))
	})
	t.Run("valid input", func(t *testing.T) {
		f := &manage2.Filter{
			NameLike: gptr.Of("test"),
			Families: []manage.Family{manage.FamilySeed},
		}
		got := ListModelsFilterDTO2DO(f)
		assert.Equal(t, f.NameLike, got.NameLike)
		assert.Equal(t, entity.FamilySeed, got.Families[0])
	})
}

func TestAbilityImageDO2DTO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, AbilityImageDO2DTO(nil))
	})
	t.Run("valid input", func(t *testing.T) {
		a := &entity.AbilityImage{
			URLEnabled:    true,
			BinaryEnabled: false,
			MaxImageSize:  1024,
			MaxImageCount: 5,
		}
		got := AbilityImageDO2DTO(a)
		assert.True(t, *got.URLEnabled)
		assert.False(t, *got.BinaryEnabled)
		assert.Equal(t, a.MaxImageSize, *got.MaxImageSize)
	})
}

func TestProtocolConfigDO2DTO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, ProtocolConfigDO2DTO(nil))
	})
	t.Run("full input", func(t *testing.T) {
		p := &entity.ProtocolConfig{
			BaseURL: "http://test.com",
			APIKey:  "key",
			Model:   "model",
			ProtocolConfigArk: &entity.ProtocolConfigArk{
				Region: "region",
			},
			ProtocolConfigOpenAI: &entity.ProtocolConfigOpenAI{
				ByAzure: true,
			},
			ProtocolConfigClaude: &entity.ProtocolConfigClaude{
				ByBedrock: true,
			},
			ProtocolConfigDeepSeek: &entity.ProtocolConfigDeepSeek{
				ResponseFormatType: "json",
			},
			ProtocolConfigGemini: &entity.ProtocolConfigGemini{
				EnableCodeExecution: true,
				SafetySettings: []entity.ProtocolConfigGeminiSafetySetting{
					{Category: 1, Threshold: 2},
				},
			},
			ProtocolConfigQwen: &entity.ProtocolConfigQwen{
				ResponseFormatType: gptr.Of("json"),
			},
			ProtocolConfigQianfan: &entity.ProtocolConfigQianfan{
				LLMRetryCount:         gptr.Of(3),
				LLMRetryTimeout:       gptr.Of(float32(1.5)),
				LLMRetryBackoffFactor: gptr.Of(float32(2.0)),
			},
			ProtocolConfigOllama: &entity.ProtocolConfigOllama{
				Format: gptr.Of("json"),
			},
			ProtocolConfigArkBot: &entity.ProtocolConfigArkBot{
				Region: "region",
			},
		}
		got := ProtocolConfigDO2DTO(p)
		assert.NotNil(t, got)
		assert.Equal(t, p.BaseURL, *got.BaseURL)
		assert.True(t, *got.ProtocolConfigOpenai.ByAzure)
		assert.Len(t, got.ProtocolConfigGemini.SafetySettings, 1)
		assert.Equal(t, int32(1), *got.ProtocolConfigGemini.SafetySettings[0].Category)
	})
}

func TestVisibleModelDO2DTO(t *testing.T) {
	assert.Equal(t, manage.VisibleModeAll, VisibleModelDO2DTO(entity.VisibleModelAll))
	assert.Equal(t, manage.VisibleModeSpecified, VisibleModelDO2DTO(entity.VisibleModelSpecified))
	assert.Equal(t, manage.VisibleModeDefault, VisibleModelDO2DTO(entity.VisibleModelDefault))
	assert.Equal(t, manage.VisibleModeUndefined, VisibleModelDO2DTO(entity.VisibleModeUndefined))
}

func TestModelStatusDO2DTO(t *testing.T) {
	assert.Equal(t, manage.ModelStatusAvailable, ModelStatusDO2DTO(entity.ModelStatusEnabled))
	assert.Equal(t, manage.ModelStatusUnavailable, ModelStatusDO2DTO(entity.ModelStatusDisabled))
	assert.Equal(t, manage.ModelStatusUndefined, ModelStatusDO2DTO(entity.ModelStatusUndefined))
}

func TestModelStatusDTO2DO(t *testing.T) {
	assert.Equal(t, entity.ModelStatusEnabled, ModelStatusDTO2DO(manage.ModelStatusAvailable))
	assert.Equal(t, entity.ModelStatusDisabled, ModelStatusDTO2DO(manage.ModelStatusUnavailable))
	assert.Equal(t, entity.ModelStatusUndefined, ModelStatusDTO2DO(manage.ModelStatusUndefined))
}

func TestAbilityMultiModalDO2DTO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, AbilityMultiModalDO2DTO(nil))
	})
	t.Run("valid input", func(t *testing.T) {
		a := &entity.AbilityMultiModal{
			Image: true,
			AbilityImage: &entity.AbilityImage{
				URLEnabled: true,
			},
		}
		got := AbilityMultiModalDO2DTO(a)
		assert.True(t, *got.Image)
		assert.True(t, *got.AbilityImage.URLEnabled)
	})
}

func TestScenarioConfigMapDO2DTO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, ScenarioConfigMapDO2DTO(nil))
	})
	t.Run("valid input", func(t *testing.T) {
		m := map[entity.Scenario]*entity.ScenarioConfig{
			entity.ScenarioDefault: {
				Scenario: entity.ScenarioDefault,
				Quota: &entity.Quota{
					Qpm: 10,
				},
			},
		}
		got := ScenarioConfigMapDO2DTO(m)
		assert.Len(t, got, 1)
	})
}

func TestParamConfigDO2DTO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, ParamConfigDO2DTO(nil))
	})
	t.Run("valid input", func(t *testing.T) {
		p := &entity.ParamConfig{
			ParamSchemas: []*entity.ParamSchema{
				{
					Name: "param1",
					Options: []*entity.ParamOption{
						{Value: "v1"},
					},
					Reaction: &entity.Reaction{
						Dependency: "dep1",
					},
				},
			},
		}
		got := ParamConfigDO2DTO(p)
		assert.Len(t, got.ParamSchemas, 1)
		assert.Equal(t, "param1", *got.ParamSchemas[0].Name)
		assert.Len(t, got.ParamSchemas[0].Options, 1)
		assert.Equal(t, "dep1", *got.ParamSchemas[0].Reaction.Dependency)
	})
}

func TestAbilityEnumDTO2DO(t *testing.T) {
	assert.Equal(t, entity.AbilityEnumFunctionCall, AbilityEnumDTO2DO(manage.AbilityFunctionCall))
	assert.Equal(t, entity.AbilityEnumMultiModal, AbilityEnumDTO2DO(manage.AbilityMultiModal_))
	assert.Equal(t, entity.AbilityEnumJsonMode, AbilityEnumDTO2DO(manage.AbilityJSONMode))
	assert.Equal(t, entity.AbilityEnumUndefined, AbilityEnumDTO2DO(manage.AbilityUndefined))
}
