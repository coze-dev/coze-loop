// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package span_processor

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

type ClipProcessor struct{}

const (
	clipProcessorMaxLength = 10 * 1024
	clipProcessorSuffix    = "..."
)

func (c *ClipProcessor) Transform(ctx context.Context, spans loop_span.SpanList) (loop_span.SpanList, error) {
	for _, span := range spans {
		if span == nil {
			continue
		}
		span.Input = clipSpanField(span.Input)
		span.Output = clipSpanField(span.Output)
	}
	return spans, nil
}

type ClipProcessorFactory struct{}

func (c *ClipProcessorFactory) CreateProcessor(ctx context.Context, set Settings) (Processor, error) {
	return &ClipProcessor{}, nil
}

func NewClipProcessorFactory() Factory {
	return new(ClipProcessorFactory)
}

func clipSpanField(content string) string {
	if content == "" || len(content) <= clipProcessorMaxLength {
		return content
	}
	if clipped, ok := clipJSONContent(content); ok {
		return clipped
	}
	return clipPlainText(content)
}

func clipPlainText(content string) string {
	if len(content) <= clipProcessorMaxLength {
		return content
	}
	return content[:clipProcessorMaxLength] + clipProcessorSuffix
}

func clipJSONContent(content string) (string, bool) {
	if !json.Valid([]byte(content)) {
		return "", false
	}
	var data interface{}
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return "", false
	}
	clippedData, changed := clipJSONTopLevel(data)
	if !changed {
		return "", false
	}
	clippedStr, err := json.MarshalString(clippedData)
	if err != nil {
		return "", false
	}
	return clippedStr, true
}

func clipJSONTopLevel(data interface{}) (interface{}, bool) {
	switch val := data.(type) {
	case map[string]interface{}:
		changed := false
		for key, v := range val {
			newVal, clipped := clipJSONFirstLevelValue(v)
			if clipped {
				val[key] = newVal
				changed = true
			}
		}
		return val, changed
	case []interface{}:
		changed := false
		for idx, v := range val {
			newVal, clipped := clipJSONFirstLevelValue(v)
			if clipped {
				val[idx] = newVal
				changed = true
			}
		}
		return val, changed
	case string:
		clipped := clipPlainText(val)
		if clipped != val {
			return clipped, true
		}
		return val, false
	default:
		return val, false
	}
}

func clipJSONFirstLevelValue(value interface{}) (interface{}, bool) {
	switch val := value.(type) {
	case string:
		clipped := clipPlainText(val)
		if clipped != val {
			return clipped, true
		}
		return val, false
	case map[string]interface{}:
		return clipNonStringJSONValue(val)
	case []interface{}:
		return clipNonStringJSONValue(val)
	default:
		return clipNonStringJSONValue(val)
	}
}

func clipNonStringJSONValue(value interface{}) (interface{}, bool) {
	marshaled, err := json.MarshalString(value)
	if err != nil {
		return value, false
	}
	if len(marshaled) <= clipProcessorMaxLength {
		return value, false
	}
	return clipProcessorSuffix, true
}
