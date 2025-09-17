// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package llmfactory

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/entity"
	llm_errorx "github.com/coze-dev/coze-loop/backend/modules/llm/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/unittest"
)

func TestFactoryImpl_CreateLLM(t *testing.T) {
	paramCfg := &entity.ParamConfig{ParamSchemas: []*entity.ParamSchema{
		{
			Name:         "max_tokens",
			Type:         entity.ParamTypeInt,
			Min:          "0",
			Max:          "4096",
			DefaultValue: "1024",
		},
		{
			Name:         "temperature",
			Type:         entity.ParamTypeFloat,
			Min:          "0",
			Max:          "1",
			DefaultValue: "0.7",
		},
		{
			Name:         "top_p",
			Type:         entity.ParamTypeFloat,
			Min:          "0",
			Max:          "1",
			DefaultValue: "0.7",
		},
		{
			Name:         "top_k",
			Type:         entity.ParamTypeInt,
			Min:          "0",
			Max:          "50",
			DefaultValue: "10",
		},
		{
			Name:         "stop",
			Type:         entity.ParamTypeString,
			DefaultValue: "[\"test\"]",
		},
		{
			Name:         "frequency_penalty",
			Type:         entity.ParamTypeFloat,
			Min:          "-1",
			Max:          "1",
			DefaultValue: "0.6",
		},
		{
			Name:         "presence_penalty",
			Type:         entity.ParamTypeFloat,
			Min:          "-1",
			Max:          "1",
			DefaultValue: "0.5",
		},
	}}
	type args struct {
		ctx   context.Context
		model *entity.Model
	}
	tests := []struct {
		name       string
		args       args
		wantNotNil bool
		wantErr    error
	}{
		{
			name: "eino_ark",
			args: args{
				ctx: context.Background(),
				model: &entity.Model{
					Frame:    entity.FrameDefault,
					Protocol: entity.ProtocolArk,
					ProtocolConfig: &entity.ProtocolConfig{
						APIKey:    "your-api-key",
						Model:     "your-model",
						TimeoutMs: nil,
						ProtocolConfigArk: &entity.ProtocolConfigArk{
							Region:    "cn-beijing",
							AccessKey: "your-access-key",
							SecretKey: "your-secret-key",
						},
					},
					ParamConfig: paramCfg,
				},
			},
			wantNotNil: true,
			wantErr:    nil,
		},
		{
			name: "eino_openai",
			args: args{
				ctx: context.Background(),
				model: &entity.Model{
					Frame:    entity.FrameDefault,
					Protocol: entity.ProtocolOpenAI,
					ProtocolConfig: &entity.ProtocolConfig{
						APIKey:    "your-api-key",
						Model:     "your-model",
						TimeoutMs: nil,
						ProtocolConfigOpenAI: &entity.ProtocolConfigOpenAI{
							ByAzure:            true,
							ResponseFormatType: "json_schema",
							ResponseFormatJsonSchema: `{
    "name": "Categorization",
    "schema": {
        "type": "object",
        "properties": {
            "results": {
                "type": "array",
                "description": "List of two-level product categorization results",
                "items": {
                    "type": "object",
                    "properties": {
                        "product_id": {
                            "type": "string"
                        },
                        "primary_category": {
                            "type": "string"
                        },
                        "secondary_category": {
                            "type": "string"
                        }
                    },
                    "required": ["product_id", "primary_category", "secondary_category"],
                    "additionalProperties": false
                }
            }
        },
        "required": ["results"],
        "additionalProperties": false
    },
    "strict": true
}`,
						},
					},
					ParamConfig: paramCfg,
				},
			},
			wantNotNil: true,
			wantErr:    nil,
		},
		{
			name: "eino_deepseek",
			args: args{
				ctx: context.Background(),
				model: &entity.Model{
					Frame:    entity.FrameDefault,
					Protocol: entity.ProtocolDeepseek,
					ProtocolConfig: &entity.ProtocolConfig{
						APIKey:                 "your-api-key",
						Model:                  "your-model",
						TimeoutMs:              nil,
						ProtocolConfigDeepSeek: &entity.ProtocolConfigDeepSeek{ResponseFormatType: ""},
					},
					ParamConfig: paramCfg,
				},
			},
			wantNotNil: true,
			wantErr:    nil,
		},
		{
			name: "eino_claude",
			args: args{
				ctx: context.Background(),
				model: &entity.Model{
					Frame:    entity.FrameDefault,
					Protocol: entity.ProtocolClaude,
					ProtocolConfig: &entity.ProtocolConfig{
						APIKey:    "your-api-key",
						Model:     "your-model",
						TimeoutMs: nil,
						ProtocolConfigClaude: &entity.ProtocolConfigClaude{
							ByBedrock: false,
						},
					},
					ParamConfig: paramCfg,
				},
			},
			wantNotNil: true,
			wantErr:    nil,
		},
		{
			name: "eino_ollama",
			args: args{
				ctx: context.Background(),
				model: &entity.Model{
					Frame:    entity.FrameDefault,
					Protocol: entity.ProtocolOllama,
					ProtocolConfig: &entity.ProtocolConfig{
						BaseURL:              "your-base-url",
						APIKey:               "your-api-key",
						Model:                "your-model",
						TimeoutMs:            nil,
						ProtocolConfigOllama: &entity.ProtocolConfigOllama{},
					},
					ParamConfig: paramCfg,
				},
			},
			wantNotNil: true,
			wantErr:    nil,
		},
		{
			name: "eino_gemini",
			args: args{
				ctx: context.Background(),
				model: &entity.Model{
					Frame:    entity.FrameDefault,
					Protocol: entity.ProtocolGemini,
					ProtocolConfig: &entity.ProtocolConfig{
						APIKey:               "your-api-key",
						Model:                "your-model",
						TimeoutMs:            nil,
						ProtocolConfigGemini: &entity.ProtocolConfigGemini{},
					},
					ParamConfig: paramCfg,
				},
			},
			wantNotNil: true,
			wantErr:    nil,
		},
		{
			name: "eino_qwen",
			args: args{
				ctx: context.Background(),
				model: &entity.Model{
					Frame:    entity.FrameDefault,
					Protocol: entity.ProtocolQwen,
					ProtocolConfig: &entity.ProtocolConfig{
						APIKey:             "your-api-key",
						Model:              "your-model",
						TimeoutMs:          nil,
						ProtocolConfigQwen: &entity.ProtocolConfigQwen{},
					},
					ParamConfig: paramCfg,
				},
			},
			wantNotNil: true,
			wantErr:    nil,
		},
		{
			name: "eino_qianfan",
			args: args{
				ctx: context.Background(),
				model: &entity.Model{
					Frame:    entity.FrameDefault,
					Protocol: entity.ProtocolQianfan,
					ProtocolConfig: &entity.ProtocolConfig{
						APIKey:                "your-api-key",
						Model:                 "your-model",
						TimeoutMs:             nil,
						ProtocolConfigQianfan: &entity.ProtocolConfigQianfan{},
					},
					ParamConfig: paramCfg,
				},
			},
			wantNotNil: true,
			wantErr:    nil,
		},
		{
			name: "eino_arkbot",
			args: args{
				ctx: context.Background(),
				model: &entity.Model{
					Frame:    entity.FrameDefault,
					Protocol: entity.ProtocolArkBot,
					ProtocolConfig: &entity.ProtocolConfig{
						APIKey:               "your-api-key",
						Model:                "your-model",
						TimeoutMs:            nil,
						ProtocolConfigArkBot: &entity.ProtocolConfigArkBot{},
					},
					ParamConfig: paramCfg,
				},
			},
			wantNotNil: true,
			wantErr:    nil,
		},
		{
			name: "failed",
			args: args{
				ctx: context.Background(),
				model: &entity.Model{
					Frame: "",
				},
			},
			wantNotNil: false,
			wantErr:    errorx.NewByCode(llm_errorx.ModelInvalidCode),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &FactoryImpl{}
			got, err := f.CreateLLM(tt.args.ctx, tt.args.model)
			unittest.AssertErrorEqual(t, tt.wantErr, err)
			assert.Equal(t, tt.wantNotNil, got != nil)
		})
	}
}
