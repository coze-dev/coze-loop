// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	evaluatordo "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestConvertEvaluatorVersionPO2DO(t *testing.T) {
	tests := []struct {
		name    string
		po      *model.EvaluatorVersion
		want    *evaluatordo.Evaluator
		wantErr bool
	}{
		{
			name: "nil input should return nil",
			po:   nil,
			want: nil,
		},
		{
			name: "EvaluatorTypePrompt with complete data",
			po: &model.EvaluatorVersion{
				ID:            123,
				SpaceID:       456,
				EvaluatorType: ptr.Of(int32(1)), // EvaluatorTypePrompt
				EvaluatorID:   789,
				Version:       "v1.0.0",
				Description:   ptr.Of("Test description"),
				Metainfo: ptr.Of([]byte(`{
					"prompt_source_type": 1,
					"prompt_template_key": "test_template",
					"message_list": [
						{
							"role": 1,
							"content": {
								"content_type": "Text",
								"text": "Hello, this is a test message"
							}
						}
					],
					"model_config": {
						"model_id": 12345,
						"model_name": "test-model",
						"temperature": 0.7,
						"max_tokens": 1000
					},
					"tools": [
						{
							"type": 1,
							"function": {
								"name": "test_function",
								"description": "A test function",
								"parameters": "{\"type\": \"object\", \"properties\": {\"param1\": {\"type\": \"string\"}}}"
							}
						}
					]
				}`)),
				InputSchema: ptr.Of([]byte(`[
					{
						"key": "input_param",
						"support_content_types": ["Text"],
						"json_schema": "{\"type\": \"string\", \"description\": \"Input parameter\"}"
					}
				]`)),
				ReceiveChatHistory: ptr.Of(true),
				CreatedBy:          "user123",
				UpdatedBy:          "user456",
				CreatedAt:          time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
				UpdatedAt:          time.Date(2025, 1, 2, 12, 0, 0, 0, time.UTC),
				DeletedAt: gorm.DeletedAt{
					Time:  time.Date(2025, 1, 3, 12, 0, 0, 0, time.UTC),
					Valid: true,
				},
			},
			want: &evaluatordo.Evaluator{
				EvaluatorType: evaluatordo.EvaluatorTypePrompt,
				PromptEvaluatorVersion: &evaluatordo.PromptEvaluatorVersion{
					PromptSourceType:  evaluatordo.PromptSourceTypeBuiltinTemplate,
					PromptTemplateKey: "test_template",
					MessageList: []*evaluatordo.Message{
						{
							Role: evaluatordo.RoleSystem,
							Content: &evaluatordo.Content{
								ContentType: ptr.Of(evaluatordo.ContentTypeText),
								Text:        ptr.Of("Hello, this is a test message"),
							},
						},
					},
					ModelConfig: &evaluatordo.ModelConfig{
						ModelID:     12345,
						ModelName:   "test-model",
						Temperature: ptr.Of(float64(0.7)),
						MaxTokens:   ptr.Of(int32(1000)),
					},
					Tools: []*evaluatordo.Tool{
						{
							Type: evaluatordo.ToolTypeFunction,
							Function: &evaluatordo.Function{
								Name:        "test_function",
								Description: "A test function",
								Parameters:  "{\"type\": \"object\", \"properties\": {\"param1\": {\"type\": \"string\"}}}",
							},
						},
					},
					InputSchemas: []*evaluatordo.ArgsSchema{
						{
							Key: ptr.Of("input_param"),
							SupportContentTypes: []evaluatordo.ContentType{
								evaluatordo.ContentTypeText,
							},
							JsonSchema: ptr.Of("{\"type\": \"string\", \"description\": \"Input parameter\"}"),
						},
					},
				},
			},
		},
		{
			name: "EvaluatorTypePrompt with minimal data",
			po: &model.EvaluatorVersion{
				ID:            123,
				SpaceID:       456,
				EvaluatorType: ptr.Of(int32(1)), // EvaluatorTypePrompt
				EvaluatorID:   789,
				Version:       "v1.0.0",
				CreatedBy:     "user123",
				UpdatedBy:     "user456",
				CreatedAt:     time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
				UpdatedAt:     time.Date(2025, 1, 2, 12, 0, 0, 0, time.UTC),
			},
			want: &evaluatordo.Evaluator{
				EvaluatorType:          evaluatordo.EvaluatorTypePrompt,
				PromptEvaluatorVersion: &evaluatordo.PromptEvaluatorVersion{},
			},
		},
		{
			name: "EvaluatorTypePrompt with invalid JSON in Metainfo",
			po: &model.EvaluatorVersion{
				ID:            123,
				SpaceID:       456,
				EvaluatorType: ptr.Of(int32(1)), // EvaluatorTypePrompt
				EvaluatorID:   789,
				Version:       "v1.0.0",
				Metainfo:      ptr.Of([]byte("invalid json")),
				CreatedBy:     "user123",
				UpdatedBy:     "user456",
				CreatedAt:     time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
				UpdatedAt:     time.Date(2025, 1, 2, 12, 0, 0, 0, time.UTC),
			},
			wantErr: true,
		},
		{
			name: "EvaluatorTypePrompt with invalid JSON in InputSchema",
			po: &model.EvaluatorVersion{
				ID:            123,
				SpaceID:       456,
				EvaluatorType: ptr.Of(int32(1)), // EvaluatorTypePrompt
				EvaluatorID:   789,
				Version:       "v1.0.0",
				Metainfo: ptr.Of([]byte(`{
					"prompt_source_type": 1,
					"prompt_template_key": "test_template"
				}`)),
				InputSchema: ptr.Of([]byte("invalid json")),
				CreatedBy:   "user123",
				UpdatedBy:   "user456",
				CreatedAt:   time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC),
				UpdatedAt:   time.Date(2025, 1, 2, 12, 0, 0, 0, time.UTC),
			},
			want: &evaluatordo.Evaluator{
				EvaluatorType: evaluatordo.EvaluatorTypePrompt,
				PromptEvaluatorVersion: &evaluatordo.PromptEvaluatorVersion{
					PromptSourceType:  evaluatordo.PromptSourceTypeBuiltinTemplate,
					PromptTemplateKey: "test_template",
				},
			},
		},
		{
			name: "EvaluatorTypePrompt with complex business scenario",
			po: &model.EvaluatorVersion{
				ID:            0,
				SpaceID:       7462935257420120114,
				EvaluatorType: ptr.Of(int32(1)), // EvaluatorTypePrompt
				EvaluatorID:   0,
				Version:       "0.0.1",
				Description:   ptr.Of(""),
				Metainfo: ptr.Of([]byte(`{
					"id": 0,
					"space_id": 7462935257420120114,
					"evaluator_type": 1,
					"evaluator_id": 0,
					"description": "",
					"version": "0.0.1",
					"input_schemas": [
						{
							"key": "user_input",
							"support_content_types": ["Text"],
							"json_schema": "{\"type\": \"string\"}"
						},
						{
							"key": "agent_output",
							"support_content_types": ["Text"],
							"json_schema": "{\"type\": \"string\"}"
						}
					],
					"prompt_source_type": 1,
					"prompt_template_key": "builtin_template_task_completion_rate",
					"message_list": [
						{
							"role": 1,
							"content": {
								"content_type": "Text",
								"format": 1,
								"text": "你是一位Agent任务评估助手，你的任务是评估一个 Agent 中是否成功、完整地实现了用户的目标。\n\n        <输入> \n        [用户输入]：{{user_input}}\n        [Agent 响应]:{{agent_output}} \n        </输入>\n\n        <评分标准>\n        请根据任务完成程度给出一个得分：\n        - 1.0：完全完成任务，表述清晰且完整。\n        - 0.5：基本完成任务，但内容不够清楚。\n        - 0.0：Agent没有完成任务。即使解释合理，但实质上未完成用户任务也得 0 分。\n        </评分标准>\n\n        <思考指导>\n        首先，请通过查看输入的上下文理解用户的真实意图。如果输入中没有明确表达意图，请尝试从上下文或消息内容中合理推断。一旦你理解了目标，请开始判断 Agent 最终响应是否成功完成了目标。然后依照评分标准，按照完成任务的程度给出最终得分。\n        </思考指导>\n        \n     "
							}
						}
					],
					"model_config": {
						"model_id": 1749615085,
						"model_name": "豆包·1.6·深度思考",
						"max_tokens": 4096,
						"temperature": 0.1,
						"top_p": 0.7,
						"tool_choice": ""
					},
					"tools": [],
					"receive_chat_history": null,
					"base_info": {
						"created_by": {
							"user_id": "4281933497631152"
						},
						"updated_by": {
							"user_id": "4281933497631152"
						},
						"created_at": 1755675640201,
						"updated_at": 1755675640201
					},
					"parse_type": "",
					"prompt_suffix": ""
				}`)),
				InputSchema: ptr.Of([]byte(`[
					{
						"key": "user_input",
						"support_content_types": ["Text"],
						"json_schema": "{\"type\": \"string\"}"
					},
					{
						"key": "agent_output",
						"support_content_types": ["Text"],
						"json_schema": "{\"type\": \"string\"}"
					}
				]`)),
				ReceiveChatHistory: nil,
				CreatedBy:          "4281933497631152",
				UpdatedBy:          "4281933497631152",
				CreatedAt:          time.UnixMilli(1755675640201),
				UpdatedAt:          time.UnixMilli(1755675640201),
			},
			want: &evaluatordo.Evaluator{
				EvaluatorType: evaluatordo.EvaluatorTypePrompt,
				PromptEvaluatorVersion: &evaluatordo.PromptEvaluatorVersion{
					PromptSourceType:  evaluatordo.PromptSourceTypeBuiltinTemplate,
					PromptTemplateKey: "builtin_template_task_completion_rate",
					MessageList: []*evaluatordo.Message{
						{
							Role: evaluatordo.RoleSystem,
							Content: &evaluatordo.Content{
								ContentType: ptr.Of(evaluatordo.ContentTypeText),
								Format:      ptr.Of(evaluatordo.PlainText),
								Text:        ptr.Of("你是一位Agent任务评估助手，你的任务是评估一个 Agent 中是否成功、完整地实现了用户的目标。\n\n        <输入> \n        [用户输入]：{{user_input}}\n        [Agent 响应]:{{agent_output}} \n        </输入>\n\n        <评分标准>\n        请根据任务完成程度给出一个得分：\n        - 1.0：完全完成任务，表述清晰且完整。\n        - 0.5：基本完成任务，但内容不够清楚。\n        - 0.0：Agent没有完成任务。即使解释合理，但实质上未完成用户任务也得 0 分。\n        </评分标准>\n\n        <思考指导>\n        首先，请通过查看输入的上下文理解用户的真实意图。如果输入中没有明确表达意图，请尝试从上下文或消息内容中合理推断。一旦你理解了目标，请开始判断 Agent 最终响应是否成功完成了目标。然后依照评分标准，按照完成任务的程度给出最终得分。\n        </思考指导>\n        \n     "),
							},
						},
					},
					ModelConfig: &evaluatordo.ModelConfig{
						ModelID:     1749615085,
						ModelName:   "豆包·1.6·深度思考",
						MaxTokens:   ptr.Of(int32(4096)),
						Temperature: ptr.Of(float64(0.1)),
						TopP:        ptr.Of(float64(0.7)),
					},
					Tools: []*evaluatordo.Tool{},
					InputSchemas: []*evaluatordo.ArgsSchema{
						{
							Key: ptr.Of("user_input"),
							SupportContentTypes: []evaluatordo.ContentType{
								evaluatordo.ContentTypeText,
							},
							JsonSchema: ptr.Of("{\"type\": \"string\"}"),
						},
						{
							Key: ptr.Of("agent_output"),
							SupportContentTypes: []evaluatordo.ContentType{
								evaluatordo.ContentTypeText,
							},
							JsonSchema: ptr.Of("{\"type\": \"string\"}"),
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertEvaluatorVersionPO2DO(tt.po)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tt.po == nil {
				assert.Nil(t, got)
				return
			}

			// 验证基本字段
			assert.Equal(t, tt.want.EvaluatorType, got.EvaluatorType)

			// 验证 PromptEvaluatorVersion 相关字段
			if tt.want.PromptEvaluatorVersion != nil {
				require.NotNil(t, got.PromptEvaluatorVersion)

				// 验证 Metainfo 解析的字段
				if tt.po.Metainfo != nil {
					// 验证 PromptSourceType
					if tt.want.PromptEvaluatorVersion.PromptSourceType != 0 {
						assert.Equal(t, tt.want.PromptEvaluatorVersion.PromptSourceType, got.PromptEvaluatorVersion.PromptSourceType)
					}

					// 验证 PromptTemplateKey
					if tt.want.PromptEvaluatorVersion.PromptTemplateKey != "" {
						assert.Equal(t, tt.want.PromptEvaluatorVersion.PromptTemplateKey, got.PromptEvaluatorVersion.PromptTemplateKey)
					}

					// 验证 MessageList
					if tt.want.PromptEvaluatorVersion.MessageList != nil {
						assert.Equal(t, len(tt.want.PromptEvaluatorVersion.MessageList), len(got.PromptEvaluatorVersion.MessageList))
						for i, wantMsg := range tt.want.PromptEvaluatorVersion.MessageList {
							if i < len(got.PromptEvaluatorVersion.MessageList) {
								gotMsg := got.PromptEvaluatorVersion.MessageList[i]
								assert.Equal(t, wantMsg.Role, gotMsg.Role)
								if wantMsg.Content != nil && gotMsg.Content != nil {
									assert.Equal(t, wantMsg.Content.ContentType, gotMsg.Content.ContentType)
									if wantMsg.Content.Text != nil && gotMsg.Content.Text != nil {
										assert.Equal(t, *wantMsg.Content.Text, *gotMsg.Content.Text)
									}
								}
							}
						}
					}

					// 验证 ModelConfig
					if tt.want.PromptEvaluatorVersion.ModelConfig != nil {
						require.NotNil(t, got.PromptEvaluatorVersion.ModelConfig)
						assert.Equal(t, tt.want.PromptEvaluatorVersion.ModelConfig.ModelID, got.PromptEvaluatorVersion.ModelConfig.ModelID)
						assert.Equal(t, tt.want.PromptEvaluatorVersion.ModelConfig.ModelName, got.PromptEvaluatorVersion.ModelConfig.ModelName)
						if tt.want.PromptEvaluatorVersion.ModelConfig.Temperature != nil {
							assert.Equal(t, *tt.want.PromptEvaluatorVersion.ModelConfig.Temperature, *got.PromptEvaluatorVersion.ModelConfig.Temperature)
						}
						if tt.want.PromptEvaluatorVersion.ModelConfig.MaxTokens != nil {
							assert.Equal(t, *tt.want.PromptEvaluatorVersion.ModelConfig.MaxTokens, *got.PromptEvaluatorVersion.ModelConfig.MaxTokens)
						}
					}

					// 验证 Tools
					if tt.want.PromptEvaluatorVersion.Tools != nil {
						assert.Equal(t, len(tt.want.PromptEvaluatorVersion.Tools), len(got.PromptEvaluatorVersion.Tools))
						for i, wantTool := range tt.want.PromptEvaluatorVersion.Tools {
							if i < len(got.PromptEvaluatorVersion.Tools) {
								gotTool := got.PromptEvaluatorVersion.Tools[i]
								assert.Equal(t, wantTool.Type, gotTool.Type)
								if wantTool.Function != nil && gotTool.Function != nil {
									assert.Equal(t, wantTool.Function.Name, gotTool.Function.Name)
									assert.Equal(t, wantTool.Function.Description, gotTool.Function.Description)
									assert.Equal(t, wantTool.Function.Parameters, gotTool.Function.Parameters)
								}
							}
						}
					}
				}

				// 验证 InputSchema 解析的字段
				if tt.po.InputSchema != nil && tt.want.PromptEvaluatorVersion.InputSchemas != nil {
					assert.Equal(t, len(tt.want.PromptEvaluatorVersion.InputSchemas), len(got.PromptEvaluatorVersion.InputSchemas))
					for i, wantSchema := range tt.want.PromptEvaluatorVersion.InputSchemas {
						if i < len(got.PromptEvaluatorVersion.InputSchemas) {
							gotSchema := got.PromptEvaluatorVersion.InputSchemas[i]
							if wantSchema.Key != nil && gotSchema.Key != nil {
								assert.Equal(t, *wantSchema.Key, *gotSchema.Key)
							}
							if wantSchema.JsonSchema != nil && gotSchema.JsonSchema != nil {
								assert.Equal(t, *wantSchema.JsonSchema, *gotSchema.JsonSchema)
							}
							if wantSchema.SupportContentTypes != nil {
								assert.Equal(t, len(wantSchema.SupportContentTypes), len(gotSchema.SupportContentTypes))
								for j, wantType := range wantSchema.SupportContentTypes {
									if j < len(gotSchema.SupportContentTypes) {
										assert.Equal(t, wantType, gotSchema.SupportContentTypes[j])
									}
								}
							}
						}
					}
				}
			}

			// 验证基础信息字段
			require.NotNil(t, got.GetEvaluatorVersion())
			assert.Equal(t, tt.po.ID, got.GetEvaluatorVersion().GetID())
			assert.Equal(t, tt.po.Version, got.GetEvaluatorVersion().GetVersion())
			assert.Equal(t, tt.po.SpaceID, got.GetEvaluatorVersion().GetSpaceID())
			assert.Equal(t, tt.po.EvaluatorID, got.GetEvaluatorVersion().GetEvaluatorID())

			if tt.po.Description != nil {
				assert.Equal(t, *tt.po.Description, got.GetEvaluatorVersion().GetDescription())
			}

			// 验证 BaseInfo
			baseInfo := got.GetEvaluatorVersion().GetBaseInfo()
			require.NotNil(t, baseInfo)
			assert.Equal(t, tt.po.CreatedBy, *baseInfo.CreatedBy.UserID)
			assert.Equal(t, tt.po.UpdatedBy, *baseInfo.UpdatedBy.UserID)
			assert.Equal(t, tt.po.CreatedAt.UnixMilli(), *baseInfo.CreatedAt)
			assert.Equal(t, tt.po.UpdatedAt.UnixMilli(), *baseInfo.UpdatedAt)

			if tt.po.DeletedAt.Valid {
				assert.Equal(t, tt.po.DeletedAt.Time.UnixMilli(), *baseInfo.DeletedAt)
			}
		})
	}
}
