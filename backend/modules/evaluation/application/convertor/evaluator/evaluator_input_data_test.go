// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	commondto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	evaluatorentity "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestConvertEvaluatorInputDataDTO2DO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *evaluatordto.EvaluatorInputData
		expected *evaluatorentity.EvaluatorInputData
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "empty input data",
			input: &evaluatordto.EvaluatorInputData{
				HistoryMessages:            []*commondto.Message{},
				InputFields:                map[string]*commondto.Content{},
				EvaluateDatasetFields:      map[string]*commondto.Content{},
				EvaluateTargetOutputFields: map[string]*commondto.Content{},
				Ext:                        map[string]string{},
			},
			expected: &evaluatorentity.EvaluatorInputData{
				HistoryMessages:            []*evaluatorentity.Message{},
				InputFields:                map[string]*evaluatorentity.Content{},
				EvaluateDatasetFields:      map[string]*evaluatorentity.Content{},
				EvaluateTargetOutputFields: map[string]*evaluatorentity.Content{},
				Ext:                        map[string]string{},
			},
		},
		{
			name: "complete input data with history messages",
			input: &evaluatordto.EvaluatorInputData{
				HistoryMessages: []*commondto.Message{
					{
						Role: gptr.Of(commondto.Role(1)),
						Content: &commondto.Content{
							ContentType: gptr.Of("text"),
							Text:        gptr.Of("Hello"),
						},
						Ext: map[string]string{"key1": "value1"},
					},
					{
						Role: gptr.Of(commondto.Role(2)),
						Content: &commondto.Content{
							ContentType: gptr.Of("text"),
							Text:        gptr.Of("Hi there"),
						},
					},
				},
				InputFields: map[string]*commondto.Content{
					"input1": {
						ContentType: gptr.Of("text"),
						Text:        gptr.Of("Input text"),
					},
					"input2": {
						ContentType: gptr.Of("image"),
						Image: &commondto.Image{
							Name: gptr.Of("test.jpg"),
							URL:  gptr.Of("https://example.com/test.jpg"),
						},
					},
				},
				EvaluateDatasetFields: map[string]*commondto.Content{
					"dataset1": {
						ContentType: gptr.Of("text"),
						Text:        gptr.Of("Dataset text"),
					},
				},
				EvaluateTargetOutputFields: map[string]*commondto.Content{
					"output1": {
						ContentType: gptr.Of("text"),
						Text:        gptr.Of("Expected output"),
					},
				},
				Ext: map[string]string{
					"ext_key": "ext_value",
				},
			},
			expected: &evaluatorentity.EvaluatorInputData{
				HistoryMessages: []*evaluatorentity.Message{
					{
						Role: evaluatorentity.Role(1),
						Content: &evaluatorentity.Content{
							ContentType: gptr.Of(evaluatorentity.ContentType("text")),
							Text:        gptr.Of("Hello"),
						},
						Ext: map[string]string{"key1": "value1"},
					},
					{
						Role: evaluatorentity.Role(2),
						Content: &evaluatorentity.Content{
							ContentType: gptr.Of(evaluatorentity.ContentType("text")),
							Text:        gptr.Of("Hi there"),
						},
					},
				},
				InputFields: map[string]*evaluatorentity.Content{
					"input1": {
						ContentType: gptr.Of(evaluatorentity.ContentType("text")),
						Text:        gptr.Of("Input text"),
					},
					"input2": {
						ContentType: gptr.Of(evaluatorentity.ContentType("image")),
						Image: &evaluatorentity.Image{
							Name: gptr.Of("test.jpg"),
							URL:  gptr.Of("https://example.com/test.jpg"),
						},
					},
				},
				EvaluateDatasetFields: map[string]*evaluatorentity.Content{
					"dataset1": {
						ContentType: gptr.Of(evaluatorentity.ContentType("text")),
						Text:        gptr.Of("Dataset text"),
					},
				},
				EvaluateTargetOutputFields: map[string]*evaluatorentity.Content{
					"output1": {
						ContentType: gptr.Of(evaluatorentity.ContentType("text")),
						Text:        gptr.Of("Expected output"),
					},
				},
				Ext: map[string]string{
					"ext_key": "ext_value",
				},
			},
		},
		{
			name: "input data with multipart content",
			input: &evaluatordto.EvaluatorInputData{
				InputFields: map[string]*commondto.Content{
					"multipart": {
						ContentType: gptr.Of("multipart"),
						MultiPart: []*commondto.Content{
							{
								ContentType: gptr.Of("text"),
								Text:        gptr.Of("Part 1"),
							},
							{
								ContentType: gptr.Of("text"),
								Text:        gptr.Of("Part 2"),
							},
						},
					},
				},
			},
			expected: &evaluatorentity.EvaluatorInputData{
				HistoryMessages:            []*evaluatorentity.Message{},
				EvaluateDatasetFields:      map[string]*evaluatorentity.Content{},
				EvaluateTargetOutputFields: map[string]*evaluatorentity.Content{},
				InputFields: map[string]*evaluatorentity.Content{
					"multipart": {
						ContentType: gptr.Of(evaluatorentity.ContentType("multipart")),
						MultiPart: []*evaluatorentity.Content{
							{
								ContentType: gptr.Of(evaluatorentity.ContentType("text")),
								Text:        gptr.Of("Part 1"),
							},
							{
								ContentType: gptr.Of(evaluatorentity.ContentType("text")),
								Text:        gptr.Of("Part 2"),
							},
						},
					},
				},
			},
		},
		{
			name: "input data with audio content",
			input: &evaluatordto.EvaluatorInputData{
				InputFields: map[string]*commondto.Content{
					"audio": {
						ContentType: gptr.Of("audio"),
						Audio: &commondto.Audio{
							Format: gptr.Of("mp3"),
							URL:    gptr.Of("https://example.com/audio.mp3"),
						},
					},
				},
			},
			expected: &evaluatorentity.EvaluatorInputData{
				HistoryMessages:            []*evaluatorentity.Message{},
				EvaluateDatasetFields:      map[string]*evaluatorentity.Content{},
				EvaluateTargetOutputFields: map[string]*evaluatorentity.Content{},
				InputFields: map[string]*evaluatorentity.Content{
					"audio": {
						ContentType: gptr.Of(evaluatorentity.ContentType("audio")),
						Audio: &evaluatorentity.Audio{
							Format: gptr.Of("mp3"),
							URL:    gptr.Of("https://example.com/audio.mp3"),
						},
					},
				},
			},
		},
		{
			name: "input data with nil fields",
			input: &evaluatordto.EvaluatorInputData{
				HistoryMessages:            nil,
				InputFields:                nil,
				EvaluateDatasetFields:      nil,
				EvaluateTargetOutputFields: nil,
				Ext:                        nil,
			},
			expected: &evaluatorentity.EvaluatorInputData{
				HistoryMessages:            []*evaluatorentity.Message{},
				InputFields:                map[string]*evaluatorentity.Content{},
				EvaluateDatasetFields:      map[string]*evaluatorentity.Content{},
				EvaluateTargetOutputFields: map[string]*evaluatorentity.Content{},
				Ext:                        nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertEvaluatorInputDataDTO2DO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertEvaluatorInputDataDO2DTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *evaluatorentity.EvaluatorInputData
		expected *evaluatordto.EvaluatorInputData
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "empty input data",
			input: &evaluatorentity.EvaluatorInputData{
				HistoryMessages:            []*evaluatorentity.Message{},
				InputFields:                map[string]*evaluatorentity.Content{},
				EvaluateDatasetFields:      map[string]*evaluatorentity.Content{},
				EvaluateTargetOutputFields: map[string]*evaluatorentity.Content{},
				Ext:                        map[string]string{},
			},
			expected: &evaluatordto.EvaluatorInputData{
				HistoryMessages:            []*commondto.Message{},
				InputFields:                map[string]*commondto.Content{},
				EvaluateDatasetFields:      map[string]*commondto.Content{},
				EvaluateTargetOutputFields: map[string]*commondto.Content{},
				Ext:                        map[string]string{},
			},
		},
		{
			name: "complete input data with history messages",
			input: &evaluatorentity.EvaluatorInputData{
				HistoryMessages: []*evaluatorentity.Message{
					{
						Role: evaluatorentity.Role(1),
						Content: &evaluatorentity.Content{
							ContentType: gptr.Of(evaluatorentity.ContentType("text")),
							Text:        gptr.Of("Hello"),
						},
						Ext: map[string]string{"key1": "value1"},
					},
					{
						Role: evaluatorentity.Role(2),
						Content: &evaluatorentity.Content{
							ContentType: gptr.Of(evaluatorentity.ContentType("text")),
							Text:        gptr.Of("Hi there"),
						},
					},
				},
				InputFields: map[string]*evaluatorentity.Content{
					"input1": {
						ContentType: gptr.Of(evaluatorentity.ContentType("text")),
						Text:        gptr.Of("Input text"),
					},
					"input2": {
						ContentType: gptr.Of(evaluatorentity.ContentType("image")),
						Image: &evaluatorentity.Image{
							Name: gptr.Of("test.jpg"),
							URL:  gptr.Of("https://example.com/test.jpg"),
						},
					},
				},
				EvaluateDatasetFields: map[string]*evaluatorentity.Content{
					"dataset1": {
						ContentType: gptr.Of(evaluatorentity.ContentType("text")),
						Text:        gptr.Of("Dataset text"),
					},
				},
				EvaluateTargetOutputFields: map[string]*evaluatorentity.Content{
					"output1": {
						ContentType: gptr.Of(evaluatorentity.ContentType("text")),
						Text:        gptr.Of("Expected output"),
					},
				},
				Ext: map[string]string{
					"ext_key": "ext_value",
				},
			},
			expected: &evaluatordto.EvaluatorInputData{
				HistoryMessages: []*commondto.Message{
					{
						Role: gptr.Of(commondto.Role(1)),
						Content: &commondto.Content{
							ContentType: gptr.Of("text"),
							Text:        gptr.Of("Hello"),
						},
						Ext: map[string]string{"key1": "value1"},
					},
					{
						Role: gptr.Of(commondto.Role(2)),
						Content: &commondto.Content{
							ContentType: gptr.Of("text"),
							Text:        gptr.Of("Hi there"),
						},
					},
				},
				InputFields: map[string]*commondto.Content{
					"input1": {
						ContentType: gptr.Of("text"),
						Text:        gptr.Of("Input text"),
					},
					"input2": {
						ContentType: gptr.Of("image"),
						Image: &commondto.Image{
							Name: gptr.Of("test.jpg"),
							URL:  gptr.Of("https://example.com/test.jpg"),
						},
					},
				},
				EvaluateDatasetFields: map[string]*commondto.Content{
					"dataset1": {
						ContentType: gptr.Of("text"),
						Text:        gptr.Of("Dataset text"),
					},
				},
				EvaluateTargetOutputFields: map[string]*commondto.Content{
					"output1": {
						ContentType: gptr.Of("text"),
						Text:        gptr.Of("Expected output"),
					},
				},
				Ext: map[string]string{
					"ext_key": "ext_value",
				},
			},
		},
		{
			name: "input data with multipart content",
			input: &evaluatorentity.EvaluatorInputData{
				InputFields: map[string]*evaluatorentity.Content{
					"multipart": {
						ContentType: gptr.Of(evaluatorentity.ContentType("multipart")),
						MultiPart: []*evaluatorentity.Content{
							{
								ContentType: gptr.Of(evaluatorentity.ContentType("text")),
								Text:        gptr.Of("Part 1"),
							},
							{
								ContentType: gptr.Of(evaluatorentity.ContentType("text")),
								Text:        gptr.Of("Part 2"),
							},
						},
					},
				},
			},
			expected: &evaluatordto.EvaluatorInputData{
				HistoryMessages:            []*commondto.Message{},
				EvaluateDatasetFields:      map[string]*commondto.Content{},
				EvaluateTargetOutputFields: map[string]*commondto.Content{},
				InputFields: map[string]*commondto.Content{
					"multipart": {
						ContentType: gptr.Of("multipart"),
						MultiPart: []*commondto.Content{
							{
								ContentType: gptr.Of("text"),
								Text:        gptr.Of("Part 1"),
							},
							{
								ContentType: gptr.Of("text"),
								Text:        gptr.Of("Part 2"),
							},
						},
					},
				},
			},
		},
		{
			name: "input data with audio content",
			input: &evaluatorentity.EvaluatorInputData{
				InputFields: map[string]*evaluatorentity.Content{
					"audio": {
						ContentType: gptr.Of(evaluatorentity.ContentType("audio")),
						Audio: &evaluatorentity.Audio{
							Format: gptr.Of("mp3"),
							URL:    gptr.Of("https://example.com/audio.mp3"),
						},
					},
				},
			},
			expected: &evaluatordto.EvaluatorInputData{
				HistoryMessages:            []*commondto.Message{},
				EvaluateDatasetFields:      map[string]*commondto.Content{},
				EvaluateTargetOutputFields: map[string]*commondto.Content{},
				InputFields: map[string]*commondto.Content{
					"audio": {
						ContentType: gptr.Of("audio"),
						Audio: &commondto.Audio{
							Format: gptr.Of("mp3"),
							URL:    gptr.Of("https://example.com/audio.mp3"),
						},
					},
				},
			},
		},
		{
			name: "input data with undefined role message",
			input: &evaluatorentity.EvaluatorInputData{
				HistoryMessages: []*evaluatorentity.Message{
					{
						Role: evaluatorentity.RoleUndefined,
						Content: &evaluatorentity.Content{
							ContentType: gptr.Of(evaluatorentity.ContentType("text")),
							Text:        gptr.Of("Message with undefined role"),
						},
					},
				},
			},
			expected: &evaluatordto.EvaluatorInputData{
				HistoryMessages: []*commondto.Message{
					{
						Role: nil,
						Content: &commondto.Content{
							ContentType: gptr.Of("text"),
							Text:        gptr.Of("Message with undefined role"),
						},
					},
				},
				InputFields:                map[string]*commondto.Content{},
				EvaluateDatasetFields:      map[string]*commondto.Content{},
				EvaluateTargetOutputFields: map[string]*commondto.Content{},
			},
		},
		{
			name: "input data with nil fields",
			input: &evaluatorentity.EvaluatorInputData{
				HistoryMessages:            nil,
				InputFields:                nil,
				EvaluateDatasetFields:      nil,
				EvaluateTargetOutputFields: nil,
				Ext:                        nil,
			},
			expected: &evaluatordto.EvaluatorInputData{
				HistoryMessages:            []*commondto.Message{},
				InputFields:                map[string]*commondto.Content{},
				EvaluateDatasetFields:      map[string]*commondto.Content{},
				EvaluateTargetOutputFields: map[string]*commondto.Content{},
				Ext:                        nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertEvaluatorInputDataDO2DTO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// 测试边界条件和特殊场景
func TestConvertEvaluatorInputDataDTO2DO_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *evaluatordto.EvaluatorInputData
		expected *evaluatorentity.EvaluatorInputData
	}{
		{
			name: "large number of history messages",
			input: &evaluatordto.EvaluatorInputData{
				HistoryMessages: func() []*commondto.Message {
					messages := make([]*commondto.Message, 100)
					for i := 0; i < 100; i++ {
						messages[i] = &commondto.Message{
							Role: gptr.Of(commondto.Role(1)),
							Content: &commondto.Content{
								ContentType: gptr.Of("text"),
								Text:        gptr.Of("Message " + string(rune(i))),
							},
						}
					}
					return messages
				}(),
			},
			expected: &evaluatorentity.EvaluatorInputData{
				HistoryMessages: func() []*evaluatorentity.Message {
					messages := make([]*evaluatorentity.Message, 100)
					for i := 0; i < 100; i++ {
						messages[i] = &evaluatorentity.Message{
							Role: evaluatorentity.Role(1),
							Content: &evaluatorentity.Content{
								ContentType: gptr.Of(evaluatorentity.ContentType("text")),
								Text:        gptr.Of("Message " + string(rune(i))),
							},
						}
					}
					return messages
				}(),
				InputFields:                map[string]*evaluatorentity.Content{},
				EvaluateDatasetFields:      map[string]*evaluatorentity.Content{},
				EvaluateTargetOutputFields: map[string]*evaluatorentity.Content{},
			},
		},
		{
			name: "deeply nested multipart content",
			input: &evaluatordto.EvaluatorInputData{
				InputFields: map[string]*commondto.Content{
					"nested": {
						ContentType: gptr.Of("multipart"),
						MultiPart: []*commondto.Content{
							{
								ContentType: gptr.Of("multipart"),
								MultiPart: []*commondto.Content{
									{
										ContentType: gptr.Of("text"),
										Text:        gptr.Of("Nested text"),
									},
								},
							},
						},
					},
				},
			},
			expected: &evaluatorentity.EvaluatorInputData{
				HistoryMessages:            []*evaluatorentity.Message{},
				EvaluateDatasetFields:      map[string]*evaluatorentity.Content{},
				EvaluateTargetOutputFields: map[string]*evaluatorentity.Content{},
				InputFields: map[string]*evaluatorentity.Content{
					"nested": {
						ContentType: gptr.Of(evaluatorentity.ContentType("multipart")),
						MultiPart: []*evaluatorentity.Content{
							{
								ContentType: gptr.Of(evaluatorentity.ContentType("multipart")),
								MultiPart: []*evaluatorentity.Content{
									{
										ContentType: gptr.Of(evaluatorentity.ContentType("text")),
										Text:        gptr.Of("Nested text"),
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertEvaluatorInputDataDTO2DO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertEvaluatorInputDataDO2DTO_EdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *evaluatorentity.EvaluatorInputData
		expected *evaluatordto.EvaluatorInputData
	}{
		{
			name: "large number of input fields",
			input: &evaluatorentity.EvaluatorInputData{
				InputFields: func() map[string]*evaluatorentity.Content {
					fields := make(map[string]*evaluatorentity.Content)
					for i := 0; i < 50; i++ {
						key := "field_" + string(rune('a'+i))
						fields[key] = &evaluatorentity.Content{
							ContentType: gptr.Of(evaluatorentity.ContentType("text")),
							Text:        gptr.Of("Content " + key),
						}
					}
					return fields
				}(),
			},
			expected: &evaluatordto.EvaluatorInputData{
				HistoryMessages:            []*commondto.Message{},
				EvaluateDatasetFields:      map[string]*commondto.Content{},
				EvaluateTargetOutputFields: map[string]*commondto.Content{},
				InputFields: func() map[string]*commondto.Content {
					fields := make(map[string]*commondto.Content)
					for i := 0; i < 50; i++ {
						key := "field_" + string(rune('a'+i))
						fields[key] = &commondto.Content{
							ContentType: gptr.Of("text"),
							Text:        gptr.Of("Content " + key),
						}
					}
					return fields
				}(),
			},
		},
		{
			name: "empty string values",
			input: &evaluatorentity.EvaluatorInputData{
				InputFields: map[string]*evaluatorentity.Content{
					"empty": {
						ContentType: gptr.Of(evaluatorentity.ContentType("")),
						Text:        gptr.Of(""),
					},
				},
				Ext: map[string]string{
					"": "",
				},
			},
			expected: &evaluatordto.EvaluatorInputData{
				HistoryMessages:            []*commondto.Message{},
				EvaluateDatasetFields:      map[string]*commondto.Content{},
				EvaluateTargetOutputFields: map[string]*commondto.Content{},
				InputFields: map[string]*commondto.Content{
					"empty": {
						ContentType: gptr.Of(""),
						Text:        gptr.Of(""),
					},
				},
				Ext: map[string]string{
					"": "",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertEvaluatorInputDataDO2DTO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertEvaluatorInputDataDTO2DO_NilFieldsHandling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *evaluatordto.EvaluatorInputData
		expected *evaluatorentity.EvaluatorInputData
	}{
		{
			name: "nil HistoryMessages field",
			input: &evaluatordto.EvaluatorInputData{
				HistoryMessages: nil,
				InputFields: map[string]*commondto.Content{
					"test": {
						ContentType: gptr.Of("text"),
						Text:        gptr.Of("test value"),
					},
				},
				EvaluateDatasetFields:      map[string]*commondto.Content{},
				EvaluateTargetOutputFields: map[string]*commondto.Content{},
				Ext:                        map[string]string{"key": "value"},
			},
			expected: &evaluatorentity.EvaluatorInputData{
				HistoryMessages: []*evaluatorentity.Message{},
				InputFields: map[string]*evaluatorentity.Content{
					"test": {
						ContentType: gptr.Of(evaluatorentity.ContentType("text")),
						Text:        gptr.Of("test value"),
					},
				},
				EvaluateDatasetFields:      map[string]*evaluatorentity.Content{},
				EvaluateTargetOutputFields: map[string]*evaluatorentity.Content{},
				Ext:                        map[string]string{"key": "value"},
			},
		},
		{
			name: "nil InputFields field",
			input: &evaluatordto.EvaluatorInputData{
				HistoryMessages: []*commondto.Message{
					{
						Role: gptr.Of(commondto.Role(1)),
						Content: &commondto.Content{
							ContentType: gptr.Of("text"),
							Text:        gptr.Of("Hello"),
						},
					},
				},
				InputFields:                nil,
				EvaluateDatasetFields:      map[string]*commondto.Content{},
				EvaluateTargetOutputFields: map[string]*commondto.Content{},
				Ext:                        map[string]string{},
			},
			expected: &evaluatorentity.EvaluatorInputData{
				HistoryMessages: []*evaluatorentity.Message{
					{
						Role: evaluatorentity.Role(1),
						Content: &evaluatorentity.Content{
							ContentType: gptr.Of(evaluatorentity.ContentType("text")),
							Text:        gptr.Of("Hello"),
						},
					},
				},
				InputFields:                map[string]*evaluatorentity.Content{},
				EvaluateDatasetFields:      map[string]*evaluatorentity.Content{},
				EvaluateTargetOutputFields: map[string]*evaluatorentity.Content{},
				Ext:                        map[string]string{},
			},
		},
		{
			name: "nil EvaluateDatasetFields field",
			input: &evaluatordto.EvaluatorInputData{
				HistoryMessages:            []*commondto.Message{},
				InputFields:                map[string]*commondto.Content{},
				EvaluateDatasetFields:      nil,
				EvaluateTargetOutputFields: map[string]*commondto.Content{},
				Ext:                        map[string]string{},
			},
			expected: &evaluatorentity.EvaluatorInputData{
				HistoryMessages:            []*evaluatorentity.Message{},
				InputFields:                map[string]*evaluatorentity.Content{},
				EvaluateDatasetFields:      map[string]*evaluatorentity.Content{},
				EvaluateTargetOutputFields: map[string]*evaluatorentity.Content{},
				Ext:                        map[string]string{},
			},
		},
		{
			name: "nil EvaluateTargetOutputFields field",
			input: &evaluatordto.EvaluatorInputData{
				HistoryMessages:            []*commondto.Message{},
				InputFields:                map[string]*commondto.Content{},
				EvaluateDatasetFields:      map[string]*commondto.Content{},
				EvaluateTargetOutputFields: nil,
				Ext:                        map[string]string{},
			},
			expected: &evaluatorentity.EvaluatorInputData{
				HistoryMessages:            []*evaluatorentity.Message{},
				InputFields:                map[string]*evaluatorentity.Content{},
				EvaluateDatasetFields:      map[string]*evaluatorentity.Content{},
				EvaluateTargetOutputFields: map[string]*evaluatorentity.Content{},
				Ext:                        map[string]string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertEvaluatorInputDataDTO2DO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertEvaluatorInputDataDO2DTO_NilFieldsHandling(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *evaluatorentity.EvaluatorInputData
		expected *evaluatordto.EvaluatorInputData
	}{
		{
			name: "nil HistoryMessages field",
			input: &evaluatorentity.EvaluatorInputData{
				HistoryMessages: nil,
				InputFields: map[string]*evaluatorentity.Content{
					"test": {
						ContentType: gptr.Of(evaluatorentity.ContentType("text")),
						Text:        gptr.Of("test value"),
					},
				},
				EvaluateDatasetFields:      map[string]*evaluatorentity.Content{},
				EvaluateTargetOutputFields: map[string]*evaluatorentity.Content{},
				Ext:                        map[string]string{"key": "value"},
			},
			expected: &evaluatordto.EvaluatorInputData{
				HistoryMessages: []*commondto.Message{},
				InputFields: map[string]*commondto.Content{
					"test": {
						ContentType: gptr.Of("text"),
						Text:        gptr.Of("test value"),
					},
				},
				EvaluateDatasetFields:      map[string]*commondto.Content{},
				EvaluateTargetOutputFields: map[string]*commondto.Content{},
				Ext:                        map[string]string{"key": "value"},
			},
		},
		{
			name: "nil InputFields field",
			input: &evaluatorentity.EvaluatorInputData{
				HistoryMessages: []*evaluatorentity.Message{
					{
						Role: evaluatorentity.Role(1),
						Content: &evaluatorentity.Content{
							ContentType: gptr.Of(evaluatorentity.ContentType("text")),
							Text:        gptr.Of("Hello"),
						},
					},
				},
				InputFields:                nil,
				EvaluateDatasetFields:      map[string]*evaluatorentity.Content{},
				EvaluateTargetOutputFields: map[string]*evaluatorentity.Content{},
				Ext:                        map[string]string{},
			},
			expected: &evaluatordto.EvaluatorInputData{
				HistoryMessages: []*commondto.Message{
					{
						Role: gptr.Of(commondto.Role(1)),
						Content: &commondto.Content{
							ContentType: gptr.Of("text"),
							Text:        gptr.Of("Hello"),
						},
					},
				},
				InputFields:                map[string]*commondto.Content{},
				EvaluateDatasetFields:      map[string]*commondto.Content{},
				EvaluateTargetOutputFields: map[string]*commondto.Content{},
				Ext:                        map[string]string{},
			},
		},
		{
			name: "nil EvaluateDatasetFields field",
			input: &evaluatorentity.EvaluatorInputData{
				HistoryMessages:            []*evaluatorentity.Message{},
				InputFields:                map[string]*evaluatorentity.Content{},
				EvaluateDatasetFields:      nil,
				EvaluateTargetOutputFields: map[string]*evaluatorentity.Content{},
				Ext:                        map[string]string{},
			},
			expected: &evaluatordto.EvaluatorInputData{
				HistoryMessages:            []*commondto.Message{},
				InputFields:                map[string]*commondto.Content{},
				EvaluateDatasetFields:      map[string]*commondto.Content{},
				EvaluateTargetOutputFields: map[string]*commondto.Content{},
				Ext:                        map[string]string{},
			},
		},
		{
			name: "nil EvaluateTargetOutputFields field",
			input: &evaluatorentity.EvaluatorInputData{
				HistoryMessages:            []*evaluatorentity.Message{},
				InputFields:                map[string]*evaluatorentity.Content{},
				EvaluateDatasetFields:      map[string]*evaluatorentity.Content{},
				EvaluateTargetOutputFields: nil,
				Ext:                        map[string]string{},
			},
			expected: &evaluatordto.EvaluatorInputData{
				HistoryMessages:            []*commondto.Message{},
				InputFields:                map[string]*commondto.Content{},
				EvaluateDatasetFields:      map[string]*commondto.Content{},
				EvaluateTargetOutputFields: map[string]*commondto.Content{},
				Ext:                        map[string]string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertEvaluatorInputDataDO2DTO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertEvaluatorInputDataDTO2DO_ComplexContentTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *evaluatordto.EvaluatorInputData
		expected *evaluatorentity.EvaluatorInputData
	}{
		{
			name: "text content with special characters",
			input: &evaluatordto.EvaluatorInputData{
				InputFields: map[string]*commondto.Content{
					"special_text": {
						ContentType: gptr.Of("text"),
						Text:        gptr.Of("Special characters: @#$%^&*()"),
					},
				},
			},
			expected: &evaluatorentity.EvaluatorInputData{
				HistoryMessages:            []*evaluatorentity.Message{},
				EvaluateDatasetFields:      map[string]*evaluatorentity.Content{},
				EvaluateTargetOutputFields: map[string]*evaluatorentity.Content{},
				InputFields: map[string]*evaluatorentity.Content{
					"special_text": {
						ContentType: gptr.Of(evaluatorentity.ContentType("text")),
						Text:        gptr.Of("Special characters: @#$%^&*()"),
					},
				},
			},
		},
		{
			name: "unicode content type",
			input: &evaluatordto.EvaluatorInputData{
				InputFields: map[string]*commondto.Content{
					"unicode_input": {
						ContentType: gptr.Of("text"),
						Text:        gptr.Of("Unicode: 你好世界 🌍"),
					},
				},
			},
			expected: &evaluatorentity.EvaluatorInputData{
				HistoryMessages:            []*evaluatorentity.Message{},
				EvaluateDatasetFields:      map[string]*evaluatorentity.Content{},
				EvaluateTargetOutputFields: map[string]*evaluatorentity.Content{},
				InputFields: map[string]*evaluatorentity.Content{
					"unicode_input": {
						ContentType: gptr.Of(evaluatorentity.ContentType("text")),
						Text:        gptr.Of("Unicode: 你好世界 🌍"),
					},
				},
			},
		},
		{
			name: "mixed content types in different fields",
			input: &evaluatordto.EvaluatorInputData{
				InputFields: map[string]*commondto.Content{
					"text": {
						ContentType: gptr.Of("text"),
						Text:        gptr.Of("Input text"),
					},
				},
				EvaluateDatasetFields: map[string]*commondto.Content{
					"image": {
						ContentType: gptr.Of("image"),
						Image: &commondto.Image{
							URL: gptr.Of("https://example.com/image.jpg"),
						},
					},
				},
				EvaluateTargetOutputFields: map[string]*commondto.Content{
					"audio": {
						ContentType: gptr.Of("audio"),
						Audio: &commondto.Audio{
							Format: gptr.Of("wav"),
							URL:    gptr.Of("https://example.com/audio.wav"),
						},
					},
				},
			},
			expected: &evaluatorentity.EvaluatorInputData{
				HistoryMessages: []*evaluatorentity.Message{},
				InputFields: map[string]*evaluatorentity.Content{
					"text": {
						ContentType: gptr.Of(evaluatorentity.ContentType("text")),
						Text:        gptr.Of("Input text"),
					},
				},
				EvaluateDatasetFields: map[string]*evaluatorentity.Content{
					"image": {
						ContentType: gptr.Of(evaluatorentity.ContentType("image")),
						Image: &evaluatorentity.Image{
							URL: gptr.Of("https://example.com/image.jpg"),
						},
					},
				},
				EvaluateTargetOutputFields: map[string]*evaluatorentity.Content{
					"audio": {
						ContentType: gptr.Of(evaluatorentity.ContentType("audio")),
						Audio: &evaluatorentity.Audio{
							Format: gptr.Of("wav"),
							URL:    gptr.Of("https://example.com/audio.wav"),
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertEvaluatorInputDataDTO2DO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertEvaluatorInputDataDO2DTO_ComplexContentTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *evaluatorentity.EvaluatorInputData
		expected *evaluatordto.EvaluatorInputData
	}{
		{
			name: "text content with special characters",
			input: &evaluatorentity.EvaluatorInputData{
				InputFields: map[string]*evaluatorentity.Content{
					"special_text": {
						ContentType: gptr.Of(evaluatorentity.ContentType("text")),
						Text:        gptr.Of("Special characters: @#$%^&*()"),
					},
				},
			},
			expected: &evaluatordto.EvaluatorInputData{
				HistoryMessages:            []*commondto.Message{},
				EvaluateDatasetFields:      map[string]*commondto.Content{},
				EvaluateTargetOutputFields: map[string]*commondto.Content{},
				InputFields: map[string]*commondto.Content{
					"special_text": {
						ContentType: gptr.Of("text"),
						Text:        gptr.Of("Special characters: @#$%^&*()"),
					},
				},
			},
		},
		{
			name: "unicode content type",
			input: &evaluatorentity.EvaluatorInputData{
				InputFields: map[string]*evaluatorentity.Content{
					"unicode_input": {
						ContentType: gptr.Of(evaluatorentity.ContentType("text")),
						Text:        gptr.Of("Unicode: 你好世界 🌍"),
					},
				},
			},
			expected: &evaluatordto.EvaluatorInputData{
				HistoryMessages:            []*commondto.Message{},
				EvaluateDatasetFields:      map[string]*commondto.Content{},
				EvaluateTargetOutputFields: map[string]*commondto.Content{},
				InputFields: map[string]*commondto.Content{
					"unicode_input": {
						ContentType: gptr.Of("text"),
						Text:        gptr.Of("Unicode: 你好世界 🌍"),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertEvaluatorInputDataDO2DTO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertEvaluatorInputDataDTO2DO_EmptyValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *evaluatordto.EvaluatorInputData
		expected *evaluatorentity.EvaluatorInputData
	}{
		{
			name: "empty string in content",
			input: &evaluatordto.EvaluatorInputData{
				InputFields: map[string]*commondto.Content{
					"empty_text": {
						ContentType: gptr.Of("text"),
						Text:        gptr.Of(""),
					},
				},
			},
			expected: &evaluatorentity.EvaluatorInputData{
				HistoryMessages:            []*evaluatorentity.Message{},
				EvaluateDatasetFields:      map[string]*evaluatorentity.Content{},
				EvaluateTargetOutputFields: map[string]*evaluatorentity.Content{},
				InputFields: map[string]*evaluatorentity.Content{
					"empty_text": {
						ContentType: gptr.Of(evaluatorentity.ContentType("text")),
						Text:        gptr.Of(""),
					},
				},
			},
		},
		{
			name: "empty content type",
			input: &evaluatordto.EvaluatorInputData{
				InputFields: map[string]*commondto.Content{
					"no_type": {
						ContentType: gptr.Of(""),
						Text:        gptr.Of("some text"),
					},
				},
			},
			expected: &evaluatorentity.EvaluatorInputData{
				HistoryMessages:            []*evaluatorentity.Message{},
				EvaluateDatasetFields:      map[string]*evaluatorentity.Content{},
				EvaluateTargetOutputFields: map[string]*evaluatorentity.Content{},
				InputFields: map[string]*evaluatorentity.Content{
					"no_type": {
						ContentType: gptr.Of(evaluatorentity.ContentType("")),
						Text:        gptr.Of("some text"),
					},
				},
			},
		},
		{
			name: "empty map key",
			input: &evaluatordto.EvaluatorInputData{
				InputFields: map[string]*commondto.Content{
					"": {
						ContentType: gptr.Of("text"),
						Text:        gptr.Of("value for empty key"),
					},
				},
			},
			expected: &evaluatorentity.EvaluatorInputData{
				HistoryMessages:            []*evaluatorentity.Message{},
				EvaluateDatasetFields:      map[string]*evaluatorentity.Content{},
				EvaluateTargetOutputFields: map[string]*evaluatorentity.Content{},
				InputFields: map[string]*evaluatorentity.Content{
					"": {
						ContentType: gptr.Of(evaluatorentity.ContentType("text")),
						Text:        gptr.Of("value for empty key"),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertEvaluatorInputDataDTO2DO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConvertEvaluatorInputDataDO2DTO_EmptyValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *evaluatorentity.EvaluatorInputData
		expected *evaluatordto.EvaluatorInputData
	}{
		{
			name: "empty string in content",
			input: &evaluatorentity.EvaluatorInputData{
				InputFields: map[string]*evaluatorentity.Content{
					"empty_text": {
						ContentType: gptr.Of(evaluatorentity.ContentType("text")),
						Text:        gptr.Of(""),
					},
				},
			},
			expected: &evaluatordto.EvaluatorInputData{
				HistoryMessages:            []*commondto.Message{},
				EvaluateDatasetFields:      map[string]*commondto.Content{},
				EvaluateTargetOutputFields: map[string]*commondto.Content{},
				InputFields: map[string]*commondto.Content{
					"empty_text": {
						ContentType: gptr.Of("text"),
						Text:        gptr.Of(""),
					},
				},
			},
		},
		{
			name: "empty content type",
			input: &evaluatorentity.EvaluatorInputData{
				InputFields: map[string]*evaluatorentity.Content{
					"no_type": {
						ContentType: gptr.Of(evaluatorentity.ContentType("")),
						Text:        gptr.Of("some text"),
					},
				},
			},
			expected: &evaluatordto.EvaluatorInputData{
				HistoryMessages:            []*commondto.Message{},
				EvaluateDatasetFields:      map[string]*commondto.Content{},
				EvaluateTargetOutputFields: map[string]*commondto.Content{},
				InputFields: map[string]*commondto.Content{
					"no_type": {
						ContentType: gptr.Of(""),
						Text:        gptr.Of("some text"),
					},
				},
			},
		},
		{
			name: "empty map key",
			input: &evaluatorentity.EvaluatorInputData{
				InputFields: map[string]*evaluatorentity.Content{
					"": {
						ContentType: gptr.Of(evaluatorentity.ContentType("text")),
						Text:        gptr.Of("value for empty key"),
					},
				},
			},
			expected: &evaluatordto.EvaluatorInputData{
				HistoryMessages:            []*commondto.Message{},
				EvaluateDatasetFields:      map[string]*commondto.Content{},
				EvaluateTargetOutputFields: map[string]*commondto.Content{},
				InputFields: map[string]*commondto.Content{
					"": {
						ContentType: gptr.Of("text"),
						Text:        gptr.Of("value for empty key"),
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertEvaluatorInputDataDO2DTO(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
