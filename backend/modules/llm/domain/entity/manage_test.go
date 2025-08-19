// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestModel_Available(t *testing.T) {
	model := &Model{
		ScenarioConfigs: map[Scenario]*ScenarioConfig{
			ScenarioDefault: {},
			ScenarioEvaluator: {
				Scenario:    ScenarioEvaluator,
				Quota:       nil,
				Unavailable: true,
			},
		},
	}
	type fields struct {
		Model *Model
	}
	type args struct {
		scenario *Scenario
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "no scenario",
			fields: fields{
				Model: model,
			},
			args: args{scenario: nil},
			want: true,
		},
		{
			name: "no scenario config",
			fields: fields{
				Model: model,
			},
			args: args{scenario: ptr.Of(ScenarioPromptDebug)},
			want: true,
		},
		{
			name: "not available scenario",
			fields: fields{
				Model: model,
			},
			args: args{scenario: ptr.Of(ScenarioEvaluator)},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.fields.Model.Available(tt.args.scenario))
		})
	}
}

func TestModel_GetModel(t *testing.T) {
	model := &Model{
		ScenarioConfigs: map[Scenario]*ScenarioConfig{
			ScenarioDefault: {
				Scenario: ScenarioDefault,
				Quota: &Quota{
					Qpm: 10,
					Tpm: 1000,
				},
			},
			ScenarioEvaluator: {
				Scenario:    ScenarioEvaluator,
				Quota:       nil,
				Unavailable: true,
			},
		},
	}
	type fields struct {
		model *Model
	}
	type args struct {
		scenario *Scenario
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   *ScenarioConfig
	}{
		{
			name:   "scenario config nil",
			fields: fields{model: &Model{}},
			args:   args{scenario: nil},
			want:   nil,
		},
		{
			name:   "scenario nil",
			fields: fields{model: model},
			args:   args{scenario: nil},
			want:   model.ScenarioConfigs[ScenarioDefault],
		},
		{
			name:   "scenario evaluator",
			fields: fields{model: model},
			args:   args{scenario: ptr.Of(ScenarioEvaluator)},
			want:   model.ScenarioConfigs[ScenarioEvaluator],
		},
		{
			name:   "scenario prompt debug",
			fields: fields{model: model},
			args:   args{scenario: ptr.Of(ScenarioPromptDebug)},
			want:   model.ScenarioConfigs[ScenarioDefault],
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wantCfg := tt.fields.model.GetScenarioConfig(tt.args.scenario)
			assert.Equal(t, tt.want == nil, wantCfg == nil)
			if tt.want == nil {
				return
			}
			assert.Equal(t, tt.want.Unavailable, wantCfg.Unavailable)
		})
	}
}

func TestModel_Valid(t *testing.T) {
	type fields struct {
		model *Model
	}
	tests := []struct {
		name    string
		fields  fields
		wantErr bool
	}{
		{
			name: "model is nil",
			fields: fields{
				model: nil,
			},
			wantErr: true,
		},
		{
			name: "model id is 0",
			fields: fields{
				model: &Model{ID: 0},
			},
			wantErr: true,
		},
		{
			name: "model name is empty",
			fields: fields{
				model: &Model{ID: 1, Name: ""},
			},
			wantErr: true,
		},
		{
			name: "model ability is invalid",
			fields: fields{
				model: &Model{
					ID: 1, Name: "name",
					Ability: &Ability{
						MultiModal:        true,
						AbilityMultiModal: nil,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "model ability is invalid",
			fields: fields{
				model: &Model{
					ID: 1, Name: "name",
					Ability: &Ability{
						MultiModal: true,
						AbilityMultiModal: &AbilityMultiModal{
							Image:        true,
							AbilityImage: nil,
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "model ability is nil",
			fields: fields{
				model: &Model{
					ID: 1, Name: "name",
					Ability:        nil,
					Protocol:       ProtocolArk,
					ProtocolConfig: &ProtocolConfig{},
				},
			},
			wantErr: false,
		},
		{
			name: "model protocol is invalid",
			fields: fields{
				model: &Model{
					ID: 1, Name: "name",
					Protocol:       ProtocolArk,
					ProtocolConfig: nil,
				},
			},
			wantErr: true,
		},
		{
			name: "model protocol is invalid",
			fields: fields{
				model: &Model{
					ID: 1, Name: "name",
					Protocol:       "",
					ProtocolConfig: &ProtocolConfig{},
				},
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantErr, tt.fields.model.Valid() != nil)
		})
	}
}

func TestGetModel(t *testing.T) {
	type fields struct {
		model *Model
	}
	tests := []struct {
		name   string
		fields fields
		want   string
	}{
		{
			name: "model is nil",
			fields: fields{
				model: nil,
			},
			want: "",
		},
		{
			name: "model pt is nil",
			fields: fields{
				model: &Model{ID: 1},
			},
			want: "",
		},
		{
			name: "model is valid",
			fields: fields{
				model: &Model{
					ID: 1,
					ProtocolConfig: &ProtocolConfig{
						Model: "your model",
					},
				},
			},
			want: "your model",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.fields.model.GetModel())
		})
	}
}

func TestSupportImageURL(t *testing.T) {
	type fields struct {
		model *Model
	}
	tests := []struct {
		name         string
		fields       fields
		wantSupport  bool
		wantImageCnt int64
	}{
		{
			name: "model is nil",
			fields: fields{
				model: nil,
			},
			wantSupport:  false,
			wantImageCnt: 0,
		},
		{
			name: "model is valid",
			fields: fields{
				model: &Model{Ability: &Ability{
					MultiModal: true,
					AbilityMultiModal: &AbilityMultiModal{
						Image: true,
						AbilityImage: &AbilityImage{
							URLEnabled:    true,
							MaxImageCount: 1,
						},
					},
				}},
			},
			wantSupport:  true,
			wantImageCnt: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualSupport, actualCnt := tt.fields.model.SupportImageURL()
			assert.Equal(t, tt.wantSupport, actualSupport)
			assert.Equal(t, tt.wantImageCnt, actualCnt)
		})
	}
}

func TestParamConfig_GetCommonParamDefaultVal(t *testing.T) {
	type fields struct {
		ParamSchemas []*ParamSchema
	}
	tests := []struct {
		name   string
		fields fields
		want   CommonParam
	}{
		{
			name: "test get common param default val",
			fields: fields{
				ParamSchemas: []*ParamSchema{
					{
						Name:         "max_tokens",
						Label:        "max_tokens",
						Desc:         "max_tokens",
						Type:         ParamTypeInt,
						Min:          "1",
						Max:          "1000",
						DefaultValue: "100",
						Options:      nil,
					},
					{
						Name:         "temperature",
						Label:        "temperature",
						Desc:         "temperature",
						Type:         ParamTypeFloat,
						Min:          "0",
						Max:          "1",
						DefaultValue: "0.7",
						Options:      nil,
					},
					{
						Name:         "top_p",
						Label:        "top_p",
						Desc:         "top_p",
						Type:         ParamTypeFloat,
						Min:          "0",
						Max:          "1",
						DefaultValue: "0.7",
						Options:      nil,
					},
					{
						Name:         "top_k",
						Label:        "top_k",
						Desc:         "top_k",
						Type:         ParamTypeInt,
						Min:          "0",
						Max:          "100",
						DefaultValue: "0",
						Options:      nil,
					},
					{
						Name:         "frequency_penalty",
						Label:        "frequency_penalty",
						Desc:         "frequency_penalty",
						Type:         ParamTypeFloat,
						Min:          "0",
						Max:          "1",
						DefaultValue: "0",
						Options:      nil,
					},
					{
						Name:         "presence_penalty",
						Label:        "presence_penalty",
						Desc:         "presence_penalty",
						Type:         ParamTypeFloat,
						Min:          "0",
						Max:          "1",
						DefaultValue: "0",
						Options:      nil,
					},
				},
			},
			want: CommonParam{
				MaxTokens:        ptr.Of(100),
				Temperature:      ptr.Of(float32(0.7)),
				TopP:             ptr.Of(float32(0.7)),
				TopK:             ptr.Of(0),
				FrequencyPenalty: ptr.Of(float32(0)),
				PresencePenalty:  ptr.Of(float32(0)),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &ParamConfig{
				ParamSchemas: tt.fields.ParamSchemas,
			}
			assert.Equalf(t, tt.want, p.GetCommonParamDefaultVal(), "GetCommonParamDefaultVal()")
		})
	}
}
