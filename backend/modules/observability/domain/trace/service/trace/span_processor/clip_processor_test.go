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

func TestClipProcessor_DefaultExtractRules(t *testing.T) {
	llmInput := `{"messages":[{"role":"system","content":"You are a helpful assistant."},{"role":"user","content":"Hello"}]}`
	llmOutput := `{"choices":[{"message":{"role":"assistant","content":"Hi there!"}}]}`

	tests := []struct {
		name            string
		spanListType    loop_span.SpanListType
		input           string
		output          string
		expectedInput   string
		expectedOutput  string
		dbConfigReturns []*entity.ColumnExtractConfig
	}{
		{
			name:            "LLMSpan uses default JSONPath when no DB config",
			spanListType:    loop_span.SpanListTypeLLMSpan,
			input:           llmInput,
			output:          llmOutput,
			expectedInput:   "Hello",
			expectedOutput:  "Hi there!",
			dbConfigReturns: nil,
		},
		{
			name:           "LLMSpan DB config overrides default",
			spanListType:   loop_span.SpanListTypeLLMSpan,
			input:          llmInput,
			output:         `{"messages":[{"role":"assistant","content":"Hi there!"},{"role":"user","content":"Bye"}]}`,
			expectedInput:  "Hello",
			expectedOutput: "Bye",
			dbConfigReturns: []*entity.ColumnExtractConfig{
				{
					WorkspaceID: 1,
					Columns: []entity.ColumnExtractRule{
						{Column: "input", JSONPath: "$.messages[1].content"},
						{Column: "output", JSONPath: "$.messages[1].content"},
					},
				},
			},
		},
		{
			name:            "RootSpan has no default, clips original JSON",
			spanListType:    loop_span.SpanListTypeRootSpan,
			input:           `{"key":"value"}`,
			output:          `{"key":"value"}`,
			expectedInput:   `{"key":"value"}`,
			expectedOutput:  `{"key":"value"}`,
			dbConfigReturns: nil,
		},
		{
			name:            "AllSpan has no default, clips original JSON",
			spanListType:    loop_span.SpanListTypeAllSpan,
			input:           "plain text content",
			output:          "plain text output",
			expectedInput:   "plain text content",
			expectedOutput:  "plain text output",
			dbConfigReturns: nil,
		},
		{
			name:            "LLMSpan default with non-JSON input falls back to clip",
			spanListType:    loop_span.SpanListTypeLLMSpan,
			input:           "not json",
			output:          "not json either",
			expectedInput:   "not json",
			expectedOutput:  "not json either",
			dbConfigReturns: nil,
		},
		{
			name:            "RootSpan default extracts last content via recursive descent",
			spanListType:    loop_span.SpanListTypeRootSpan,
			input:           `{"stream":[[{"role":"user","content":"你好"}]]}`,
			output:          `{"role":"assistant","content":"世界你好","extra":{"id":"123"}}`,
			expectedInput:   "你好",
			expectedOutput:  "世界你好",
			dbConfigReturns: nil,
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
					SpanListType: tt.spanListType,
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

func TestClipProcessor_DefaultExtractRulesWithLongContent(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockRepo := mocks.NewMockIColumnExtractConfigRepo(ctrl)
	mockRepo.EXPECT().ListColumnExtractConfigs(gomock.Any(), gomock.Any()).
		Return(nil, nil).Times(1)

	longContent := strings.Repeat("a", clipProcessorPlainTextMaxLength+100)
	llmInput, _ := json.MarshalString(map[string]interface{}{
		"messages": []interface{}{
			map[string]interface{}{"role": "user", "content": longContent},
		},
	})

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

func TestClipProcessor_NoRepoUsesDefault(t *testing.T) {
	llmInput := `{"messages":[{"role":"system","content":"Hello world"}]}`

	processor := &ClipProcessor{
		columnExtractConfigRepo: nil,
		settings: Settings{
			SpanListType: loop_span.SpanListTypeLLMSpan,
		},
	}
	spans := loop_span.SpanList{{Input: llmInput}}
	res, err := processor.Transform(context.Background(), spans)
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.Equal(t, "Hello world", res[0].Input)
}

func TestSelectBestConfig(t *testing.T) {
	makeConfig := func(wsID int64, agentName string) *entity.ColumnExtractConfig {
		return &entity.ColumnExtractConfig{
			WorkspaceID: wsID,
			AgentName:   agentName,
			Columns: []entity.ColumnExtractRule{
				{Column: "input", JSONPath: "$.test"},
			},
		}
	}

	allConfigs := []*entity.ColumnExtractConfig{
		makeConfig(100, "bot_a"),
		makeConfig(100, ""),
		makeConfig(200, "bot_a"),
		makeConfig(200, ""),
		makeConfig(0, "bot_a"),
		makeConfig(0, ""),
	}

	tests := []struct {
		name        string
		configs     []*entity.ColumnExtractConfig
		workspaceId int64
		agentName   string
		wantWsID    int64
		wantAgent   string
		wantNil     bool
	}{
		{
			name:        "exact match: workspace + agent",
			configs:     allConfigs,
			workspaceId: 100,
			agentName:   "bot_a",
			wantWsID:    100,
			wantAgent:   "bot_a",
		},
		{
			name:        "fallback: workspace match + no agent config",
			configs:     allConfigs,
			workspaceId: 100,
			agentName:   "bot_b",
			wantWsID:    100,
			wantAgent:   "",
		},
		{
			name:        "fallback: no workspace + agent match",
			configs:     allConfigs,
			workspaceId: 999,
			agentName:   "bot_a",
			wantWsID:    100,
			wantAgent:   "bot_a",
		},
		{
			name:        "fallback: no workspace + no agent -> first non-ws match",
			configs:     allConfigs,
			workspaceId: 999,
			agentName:   "bot_b",
			wantWsID:    100,
			wantAgent:   "",
		},
		{
			name:        "workspace match + empty agent query",
			configs:     allConfigs,
			workspaceId: 100,
			agentName:   "",
			wantWsID:    100,
			wantAgent:   "",
		},
		{
			name:        "empty configs returns nil",
			configs:     nil,
			workspaceId: 100,
			agentName:   "bot_a",
			wantNil:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := selectBestConfig(tt.configs, tt.workspaceId, tt.agentName)
			if tt.wantNil {
				require.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			require.Equal(t, tt.wantWsID, got.WorkspaceID)
			require.Equal(t, tt.wantAgent, got.AgentName)
		})
	}
}
