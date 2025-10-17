// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package span_processor

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

func TestClipProcessor_TransformPlainText(t *testing.T) {
	processor := &ClipProcessor{}
	content := strings.Repeat("a", clipProcessorMaxLength+5)
	spans := loop_span.SpanList{{Input: content}}

	res, err := processor.Transform(context.Background(), spans)
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.Equal(t, clipProcessorMaxLength+len(clipProcessorSuffix), len(res[0].Input))
	require.True(t, strings.HasSuffix(res[0].Input, clipProcessorSuffix))
}

func TestClipProcessor_TransformJSONObject(t *testing.T) {
	processor := &ClipProcessor{}
	largeValue := strings.Repeat("b", clipProcessorMaxLength+10)
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
	require.Equal(t, clipPlainText(largeValue), result["large"])
	require.Equal(t, "ok", result["normal"])
}

func TestClipProcessor_TransformJSONNestedObject(t *testing.T) {
	processor := &ClipProcessor{}
	largeValue := strings.Repeat("c", clipProcessorMaxLength+20)
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
	require.Equal(t, clipPlainText(largeValue), nested["inner"])
}

func TestClipProcessor_TransformJSONArray(t *testing.T) {
	processor := &ClipProcessor{}
	largeValue := strings.Repeat("d", clipProcessorMaxLength+30)
	data := []interface{}{largeValue, "ok"}
	content, err := json.MarshalString(data)
	require.NoError(t, err)
	spans := loop_span.SpanList{{Input: content}}

	res, err := processor.Transform(context.Background(), spans)
	require.NoError(t, err)
	require.Len(t, res, 1)

	var result []interface{}
	require.NoError(t, json.Unmarshal([]byte(res[0].Input), &result))
	require.Equal(t, clipPlainText(largeValue), result[0])
	require.Equal(t, "ok", result[1])
}

func TestClipProcessor_TransformJSONString(t *testing.T) {
	processor := &ClipProcessor{}
	largeValue := strings.Repeat("e", clipProcessorMaxLength+40)
	content, err := json.MarshalString(largeValue)
	require.NoError(t, err)
	spans := loop_span.SpanList{{Input: content}}

	res, err := processor.Transform(context.Background(), spans)
	require.NoError(t, err)
	require.Len(t, res, 1)

	var result string
	require.NoError(t, json.Unmarshal([]byte(res[0].Input), &result))
	require.Equal(t, clipPlainText(largeValue), result)
}

func TestClipProcessor_TransformJSONDeepNested(t *testing.T) {
	processor := &ClipProcessor{}
	largeValue := strings.Repeat("g", clipProcessorMaxLength+60)
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
	require.Equal(t, clipPlainText(largeValue), innerArr[0])
	require.Equal(t, "ok", innerArr[1])
}

func TestClipProcessor_TransformOutputPlainText(t *testing.T) {
	processor := &ClipProcessor{}
	content := strings.Repeat("f", clipProcessorMaxLength+50)
	spans := loop_span.SpanList{{Output: content}}

	res, err := processor.Transform(context.Background(), spans)
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.Equal(t, clipProcessorMaxLength+len(clipProcessorSuffix), len(res[0].Output))
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
	content := strings.Repeat("只能制定计划让执行代理分析代码仓库结构并根据实际情况进行分析。", 40)
	clipped := clipPlainText(content)
	require.True(t, strings.HasSuffix(clipped, clipProcessorSuffix))
	require.False(t, strings.Contains(clipped, "\ufffd"))
	require.True(t, strings.HasPrefix(clipped, "只能制定计划"))
	require.True(t, utf8.ValidString(clipped))
}

func TestClipSpanField_JSONFallback(t *testing.T) {
	data := map[string]interface{}{
		"message": strings.Repeat("好", clipProcessorMaxLength/3+20),
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
	content := strings.Repeat("目标风", 400)
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
	factory := NewClipProcessorFactory()
	processor, err := factory.CreateProcessor(context.Background(), Settings{})
	require.NoError(t, err)
	require.IsType(t, &ClipProcessor{}, processor)
}

func TestClipJSONValue_DefaultBranch(t *testing.T) {
	res, changed := clipJSONValue(float64(123.456))
	require.Equal(t, float64(123.456), res)
	require.False(t, changed)
}
