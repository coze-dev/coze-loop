// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package span_processor

import (
	"context"
	"strings"
	"testing"

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
