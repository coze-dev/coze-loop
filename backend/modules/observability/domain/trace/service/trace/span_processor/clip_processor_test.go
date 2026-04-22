// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package span_processor

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

func TestClipProcessor_TransformPlainText(t *testing.T) {
	processor := &ClipProcessor{}
	content := strings.Repeat("a", clipProcessorPlainTextMaxLength+5)
	spans := loop_span.SpanList{{Input: content}}

	res, err := processor.Transform(context.Background(), spans)
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.Equal(t, clipProcessorPlainTextMaxLength+len(clipProcessorSuffix), len(res[0].Input))
	require.True(t, strings.HasSuffix(res[0].Input, clipProcessorSuffix))
}

func TestClipProcessor_TransformJSONObject(t *testing.T) {
	processor := &ClipProcessor{}
	largeValue := strings.Repeat("b", clipProcessorPlainTextMaxLength+clipProcessorJSONValueMaxLength+10)
	data := map[string]interface{}{
		"large":  largeValue,
		"normal": "ok",
	}
	content, err := json.MarshalString(data)
	require.NoError(t, err)
	spans := loop_span.SpanList{{Input: content}}

	res, err := processor.Transform(context.Background(), spans)
	require.NoError(t, err)
	require.Len(t, res, 1)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(res[0].Input), &result))
	clippedLarge, ok := result["large"].(string)
	require.True(t, ok)
	require.Equal(t, clipProcessorJSONValueMaxLength+len(clipProcessorSuffix), len(clippedLarge))
	require.True(t, strings.HasSuffix(clippedLarge, clipProcessorSuffix))
	require.True(t, strings.HasPrefix(clippedLarge, "b"))
	require.Equal(t, "ok", result["normal"])
}

func TestClipProcessor_TransformJSONNestedObject(t *testing.T) {
	processor := &ClipProcessor{}
	largeValue := strings.Repeat("c", clipProcessorPlainTextMaxLength+clipProcessorJSONValueMaxLength+20)
	data := map[string]interface{}{
		"nested": map[string]interface{}{
			"inner": largeValue,
		},
	}
	content, err := json.MarshalString(data)
	require.NoError(t, err)
	spans := loop_span.SpanList{{Input: content}}

	res, err := processor.Transform(context.Background(), spans)
	require.NoError(t, err)
	require.Len(t, res, 1)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(res[0].Input), &result))
	nested, ok := result["nested"].(map[string]interface{})
	require.True(t, ok)
	inner, ok := nested["inner"].(string)
	require.True(t, ok)
	require.Equal(t, clipProcessorJSONValueMaxLength+len(clipProcessorSuffix), len(inner))
	require.True(t, strings.HasSuffix(inner, clipProcessorSuffix))
	require.True(t, strings.HasPrefix(inner, "c"))
}

func TestClipProcessor_TransformJSONArray(t *testing.T) {
	processor := &ClipProcessor{}
	largeValue := strings.Repeat("d", clipProcessorPlainTextMaxLength+clipProcessorJSONValueMaxLength+30)
	data := []interface{}{largeValue, "ok"}
	content, err := json.MarshalString(data)
	require.NoError(t, err)
	spans := loop_span.SpanList{{Input: content}}

	res, err := processor.Transform(context.Background(), spans)
	require.NoError(t, err)
	require.Len(t, res, 1)

	var result []interface{}
	require.NoError(t, json.Unmarshal([]byte(res[0].Input), &result))
	first, ok := result[0].(string)
	require.True(t, ok)
	require.Equal(t, clipProcessorJSONValueMaxLength+len(clipProcessorSuffix), len(first))
	require.True(t, strings.HasSuffix(first, clipProcessorSuffix))
	require.True(t, strings.HasPrefix(first, "d"))
	require.Equal(t, "ok", result[1])
}

func TestClipProcessor_TransformJSONString(t *testing.T) {
	processor := &ClipProcessor{}
	largeValue := strings.Repeat("e", clipProcessorPlainTextMaxLength+clipProcessorJSONValueMaxLength+40)
	content, err := json.MarshalString(largeValue)
	require.NoError(t, err)
	spans := loop_span.SpanList{{Input: content}}

	res, err := processor.Transform(context.Background(), spans)
	require.NoError(t, err)
	require.Len(t, res, 1)

	var result string
	require.NoError(t, json.Unmarshal([]byte(res[0].Input), &result))
	require.Equal(t, clipProcessorJSONValueMaxLength+len(clipProcessorSuffix), len(result))
	require.True(t, strings.HasSuffix(result, clipProcessorSuffix))
	require.True(t, strings.HasPrefix(result, "e"))
}

func TestClipProcessor_TransformJSONDeepNested(t *testing.T) {
	processor := &ClipProcessor{}
	largeValue := strings.Repeat("g", clipProcessorPlainTextMaxLength+clipProcessorJSONValueMaxLength+60)
	data := map[string]interface{}{
		"levels": []interface{}{
			map[string]interface{}{
				"inner": []interface{}{largeValue, "ok"},
			},
		},
	}
	content, err := json.MarshalString(data)
	require.NoError(t, err)
	spans := loop_span.SpanList{{Input: content}}

	res, err := processor.Transform(context.Background(), spans)
	require.NoError(t, err)
	require.Len(t, res, 1)

	var result map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(res[0].Input), &result))
	levels, ok := result["levels"].([]interface{})
	require.True(t, ok)
	require.Len(t, levels, 1)
	innerMap, ok := levels[0].(map[string]interface{})
	require.True(t, ok)
	innerArr, ok := innerMap["inner"].([]interface{})
	require.True(t, ok)
	require.Len(t, innerArr, 2)
	clippedInner, ok := innerArr[0].(string)
	require.True(t, ok)
	require.Equal(t, clipProcessorJSONValueMaxLength+len(clipProcessorSuffix), len(clippedInner))
	require.True(t, strings.HasSuffix(clippedInner, clipProcessorSuffix))
	require.Equal(t, "ok", innerArr[1])
}

func TestClipProcessor_TransformOutputPlainText(t *testing.T) {
	processor := &ClipProcessor{}
	content := strings.Repeat("f", clipProcessorPlainTextMaxLength+50)
	spans := loop_span.SpanList{{Output: content}}

	res, err := processor.Transform(context.Background(), spans)
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.Equal(t, clipProcessorPlainTextMaxLength+len(clipProcessorSuffix), len(res[0].Output))
	require.True(t, strings.HasSuffix(res[0].Output, clipProcessorSuffix))
}

func TestClipByByteLimit_EdgeCases(t *testing.T) {
	content := "abc你好"
	require.Equal(t, "", clipByByteLimit(content, 0))
	require.Equal(t, "", clipByByteLimit(content, -1))
	require.Equal(t, content, clipByByteLimit(content, len(content)))
	require.Equal(t, "abc你", clipByByteLimit(content, len("abc你")))
	require.Equal(t, "abc你", clipByByteLimit(content, len("abc你")+1))
	require.Equal(t, "", clipByByteLimit("你好", 1))
}

func TestClipPlainText_UTF8Validity(t *testing.T) {
	content := strings.Repeat("只能制定计划让执行代理分析代码仓库结构并根据实际情况进行分析。", 400)
	clipped := clipPlainText(content)
	require.True(t, strings.HasSuffix(clipped, clipProcessorSuffix))
	require.False(t, strings.Contains(clipped, "\ufffd"))
	require.True(t, strings.HasPrefix(clipped, "只能制定计划"))
	require.True(t, utf8.ValidString(clipped))
}

func TestClipSpanField_JSONFallback(t *testing.T) {
	data := map[string]interface{}{
		"message": strings.Repeat("好", clipProcessorPlainTextMaxLength+clipProcessorJSONValueMaxLength+20),
	}
	raw, err := json.MarshalString(data)
	require.NoError(t, err)
	result := clipSpanField(raw)
	require.True(t, json.Valid([]byte(result)))
	require.NotContains(t, result, "\ufffd")

	var parsed map[string]string
	require.NoError(t, json.Unmarshal([]byte(result), &parsed))
	require.True(t, strings.HasSuffix(parsed["message"], clipProcessorSuffix))
	require.True(t, strings.HasPrefix(parsed["message"], "好"))
}

func TestClipSpanField_NonJSON(t *testing.T) {
	content := strings.Repeat("目标风", 4000)
	result := clipSpanField(content)
	require.True(t, strings.HasSuffix(result, clipProcessorSuffix))
	require.NotContains(t, result, "\ufffd")
}

func TestClipSpanField_ShortContent(t *testing.T) {
	content := "short"
	require.Equal(t, content, clipSpanField(content))
}

func TestClipJSONContent_Invalid(t *testing.T) {
	clipped, ok := clipJSONContent("not-json")
	require.False(t, ok)
	require.Equal(t, "", clipped)
}

func TestClipJSONContent_NoChange(t *testing.T) {
	data := []string{"foo", "bar"}
	raw, err := json.MarshalString(data)
	require.NoError(t, err)
	clipped, ok := clipJSONContent(raw)
	require.False(t, ok)
	require.Equal(t, "", clipped)
}

func TestClipProcessor_TransformSkipNil(t *testing.T) {
	processor := &ClipProcessor{}
	spans := loop_span.SpanList{
		nil,
		{Input: "short"},
	}
	res, err := processor.Transform(context.Background(), spans)
	require.NoError(t, err)
	require.Len(t, res, 2)
	require.Nil(t, res[0])
	require.Equal(t, "short", res[1].Input)
}

func TestClipProcessorFactory(t *testing.T) {
	factory := NewClipProcessorFactory(nil)
	processor, err := factory.CreateProcessor(context.Background(), Settings{})
	require.NoError(t, err)
	require.IsType(t, &ClipProcessor{}, processor)
}

func TestClipJSONValue_DefaultBranch(t *testing.T) {
	res, changed := clipJSONValue(float64(123.456))
	require.Equal(t, float64(123.456), res)
	require.False(t, changed)
}

func TestClipProcessor_WithDBConfig(t *testing.T) {
	llmInput := `{"messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hello"}]}`
	llmOutput := `{"choices":[{"message":{"role":"assistant","content":"Hi there!"}}]}`

	tests := []struct {
		name            string
		input           string
		output          string
		expectedInput   string
		expectedOutput  string
		dbConfigReturns []*entity.ColumnExtractConfig
	}{
		{
			name:           "DB config extracts input and output",
			input:          llmInput,
			output:         llmOutput,
			expectedInput:  "Hello",
			expectedOutput: "Hi there!",
			dbConfigReturns: []*entity.ColumnExtractConfig{
				{
					WorkspaceID:  1,
					PlatformType: string(loop_span.PlatformCozeLoop),
					SpanListType: string(loop_span.SpanListTypeLLMSpan),
					Columns: []entity.ColumnExtractRule{
						{Column: "input", JSONPath: "$.messages[-1:].content"},
						{Column: "output", JSONPath: "$.choices[0].message.content"},
					},
				},
			},
		},
		{
			name:            "No DB config clips original content",
			input:           `{"key":"value"}`,
			output:          `{"key":"value"}`,
			expectedInput:   `{"key":"value"}`,
			expectedOutput:  `{"key":"value"}`,
			dbConfigReturns: nil,
		},
		{
			name:            "No DB config with plain text",
			input:           "plain text content",
			output:          "plain text output",
			expectedInput:   "plain text content",
			expectedOutput:  "plain text output",
			dbConfigReturns: nil,
		},
		{
			name:           "DB config with recursive descent",
			input:          `{"stream":[[{"role":"user","content":"你好"}]]}`,
			output:         `{"role":"assistant","content":"世界你好","extra":{"id":"123"}}`,
			expectedInput:  "你好",
			expectedOutput: "世界你好",
			dbConfigReturns: []*entity.ColumnExtractConfig{
				{
					WorkspaceID:  0,
					PlatformType: "*",
					SpanListType: "*",
					Columns: []entity.ColumnExtractRule{
						{Column: "input", JSONPath: "$..content"},
						{Column: "output", JSONPath: "$..content"},
					},
				},
			},
		},
		{
			name:           "DB config extraction fails, falls back to clip",
			input:          "not json",
			output:         "not json either",
			expectedInput:  "not json",
			expectedOutput: "not json either",
			dbConfigReturns: []*entity.ColumnExtractConfig{
				{
					WorkspaceID:  1,
					PlatformType: string(loop_span.PlatformCozeLoop),
					SpanListType: string(loop_span.SpanListTypeLLMSpan),
					Columns: []entity.ColumnExtractRule{
						{Column: "input", JSONPath: "$.messages[0].content"},
						{Column: "output", JSONPath: "$.choices[0].message.content"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockRepo := mocks.NewMockIColumnExtractConfigRepo(ctrl)

			if tt.dbConfigReturns != nil {
				mockRepo.EXPECT().ListColumnExtractConfigs(gomock.Any(), gomock.Any()).
					Return(tt.dbConfigReturns, nil).Times(1)
			} else {
				mockRepo.EXPECT().ListColumnExtractConfigs(gomock.Any(), gomock.Any()).
					Return(nil, nil).Times(1)
			}

			processor := &ClipProcessor{
				columnExtractConfigRepo: mockRepo,
				settings: Settings{
					WorkspaceId:  1,
					PlatformType: loop_span.PlatformCozeLoop,
					SpanListType: loop_span.SpanListTypeLLMSpan,
				},
			}
			spans := loop_span.SpanList{{Input: tt.input, Output: tt.output}}
			res, err := processor.Transform(context.Background(), spans)
			require.NoError(t, err)
			require.Len(t, res, 1)
			require.Equal(t, tt.expectedInput, res[0].Input)
			require.Equal(t, tt.expectedOutput, res[0].Output)
		})
	}
}

func TestClipProcessor_WithDBConfigLongContent(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockIColumnExtractConfigRepo(ctrl)

	longContent := strings.Repeat("a", clipProcessorPlainTextMaxLength+100)
	llmInput, _ := json.MarshalString(map[string]interface{}{
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": longContent},
		},
	})

	mockRepo.EXPECT().ListColumnExtractConfigs(gomock.Any(), gomock.Any()).
		Return([]*entity.ColumnExtractConfig{
			{
				WorkspaceID:  0,
				PlatformType: "*",
				SpanListType: "*",
				Columns: []entity.ColumnExtractRule{
					{Column: "input", JSONPath: "$.messages[0].content"},
				},
			},
		}, nil).Times(1)

	processor := &ClipProcessor{
		columnExtractConfigRepo: mockRepo,
		settings: Settings{
			WorkspaceId:  1,
			PlatformType: loop_span.PlatformCozeLoop,
			SpanListType: loop_span.SpanListTypeLLMSpan,
		},
	}
	spans := loop_span.SpanList{{Input: llmInput}}
	res, err := processor.Transform(context.Background(), spans)
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.True(t, strings.HasSuffix(res[0].Input, clipProcessorSuffix))
	require.True(t, len(res[0].Input) <= clipProcessorPlainTextMaxLength+len(clipProcessorSuffix))
}

func TestClipProcessor_DefaultConfigSelection(t *testing.T) {
	llmInput := `{"messages":[{"role":"system","content":"You are a helper."},{"role":"user","content":"Hello from user"}]}`
	llmOutput := `{"choices":[{"message":{"role":"assistant","content":"Hi from assistant"}}]}`

	// Mock: DB returns both default (wsID=0) and workspace-specific configs
	defaultConfig := &entity.ColumnExtractConfig{
		WorkspaceID:  0,
		AgentName:    "",
		PlatformType: "*",
		SpanListType: "*",
		Columns: []entity.ColumnExtractRule{
			{Column: "input", JSONPath: "$..content"},
			{Column: "output", JSONPath: "$..content"},
		},
	}
	wsConfig := &entity.ColumnExtractConfig{
		WorkspaceID:  42,
		AgentName:    "",
		PlatformType: string(loop_span.PlatformCozeLoop),
		SpanListType: string(loop_span.SpanListTypeLLMSpan),
		Columns: []entity.ColumnExtractRule{
			{Column: "input", JSONPath: "$.messages[-1:].content"},
			{Column: "output", JSONPath: "$.choices[0].message.content"},
		},
	}
	wsAgentConfig := &entity.ColumnExtractConfig{
		WorkspaceID:  42,
		AgentName:    "my_bot",
		PlatformType: string(loop_span.PlatformCozeLoop),
		SpanListType: string(loop_span.SpanListTypeLLMSpan),
		Columns: []entity.ColumnExtractRule{
			{Column: "input", JSONPath: "$.messages[0].content"},
			{Column: "output", JSONPath: "$.choices[0].message.content"},
		},
	}

	allConfigs := []*entity.ColumnExtractConfig{defaultConfig, wsConfig, wsAgentConfig}

	tests := []struct {
		name           string
		workspaceId    int64
		agentName      string
		expectedInput  string
		expectedOutput string
	}{
		{
			name:           "workspace+agent exact match uses wsAgentConfig",
			workspaceId:    42,
			agentName:      "my_bot",
			expectedInput:  "You are a helper.", // $.messages[0].content
			expectedOutput: "Hi from assistant", // $.choices[0].message.content
		},
		{
			name:           "workspace match, no agent match, uses wsConfig",
			workspaceId:    42,
			agentName:      "other_bot",
			expectedInput:  "Hello from user",   // $.messages[-1:].content
			expectedOutput: "Hi from assistant", // $.choices[0].message.content
		},
		{
			name:           "no workspace match, falls back to default(wsID=0)",
			workspaceId:    999,
			agentName:      "any_bot",
			expectedInput:  "Hello from user",   // $..content returns last
			expectedOutput: "Hi from assistant", // $..content returns last
		},
		{
			name:           "workspace match, empty agent, uses wsConfig",
			workspaceId:    42,
			agentName:      "",
			expectedInput:  "Hello from user", // $.messages[-1:].content
			expectedOutput: "Hi from assistant",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			mockRepo := mocks.NewMockIColumnExtractConfigRepo(ctrl)
			mockRepo.EXPECT().ListColumnExtractConfigs(gomock.Any(), gomock.Any()).
				Return(allConfigs, nil).Times(1)

			processor := &ClipProcessor{
				columnExtractConfigRepo: mockRepo,
				settings: Settings{
					WorkspaceId:  tt.workspaceId,
					AgentName:    tt.agentName,
					PlatformType: loop_span.PlatformCozeLoop,
					SpanListType: loop_span.SpanListTypeLLMSpan,
				},
			}
			spans := loop_span.SpanList{{Input: llmInput, Output: llmOutput}}
			res, err := processor.Transform(context.Background(), spans)
			require.NoError(t, err)
			require.Len(t, res, 1)
			require.Equal(t, tt.expectedInput, res[0].Input)
			require.Equal(t, tt.expectedOutput, res[0].Output)
		})
	}
}

func TestClipProcessor_NoRepoClipsOnly(t *testing.T) {
	processor := &ClipProcessor{
		columnExtractConfigRepo: nil,
		settings:                Settings{},
	}
	spans := loop_span.SpanList{{Input: `{"key":"value"}`, Output: "plain text"}}
	res, err := processor.Transform(context.Background(), spans)
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.Equal(t, `{"key":"value"}`, res[0].Input)
	require.Equal(t, "plain text", res[0].Output)
}
