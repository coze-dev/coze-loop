// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package span_processor

import (
	"context"
	"strings"
	"unicode/utf8"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

type ClipProcessor struct {
	columnExtractConfigRepo repo.IColumnExtractConfigRepo
	settings                Settings
}

const (
	clipProcessorPlainTextMaxLength = 10 * 1024
	clipProcessorJSONValueMaxLength = 1 * 1024
	clipProcessorSuffix             = "..."
)

var defaultExtractRules = map[loop_span.SpanListType][]entity.ColumnExtractRule{
	loop_span.SpanListTypeLLMSpan: {
		{Column: "input", JSONPath: "$.messages[0].content"},
		{Column: "output", JSONPath: "$.messages[0].content"},
	},
}

func (c *ClipProcessor) Transform(ctx context.Context, spans loop_span.SpanList) (loop_span.SpanList, error) {
	var rules []entity.ColumnExtractRule
	if c.columnExtractConfigRepo != nil && c.settings.WorkspaceId > 0 {
		cfg, err := c.columnExtractConfigRepo.GetColumnExtractConfig(ctx, repo.GetColumnExtractConfigParam{
			WorkspaceId:  c.settings.WorkspaceId,
			PlatformType: string(c.settings.PlatformType),
			SpanListType: string(c.settings.SpanListType),
			AgentName:    c.settings.AgentName,
		})
		if err == nil && cfg != nil {
			rules = cfg.Columns
		}
	}

	if len(rules) == 0 {
		if defaults, ok := defaultExtractRules[c.settings.SpanListType]; ok {
			rules = defaults
		}
	}

	inputRule, outputRule := findExtractRules(rules)

	for _, s := range spans {
		if s == nil {
			continue
		}
		s.Input = extractAndClip(s.Input, "input", inputRule)
		s.Output = extractAndClip(s.Output, "output", outputRule)
	}
	return spans, nil
}

type ClipProcessorFactory struct {
	columnExtractConfigRepo repo.IColumnExtractConfigRepo
}

func (c *ClipProcessorFactory) CreateProcessor(ctx context.Context, set Settings) (Processor, error) {
	return &ClipProcessor{
		columnExtractConfigRepo: c.columnExtractConfigRepo,
		settings:                set,
	}, nil
}

func NewClipProcessorFactory(columnExtractConfigRepo repo.IColumnExtractConfigRepo) Factory {
	return &ClipProcessorFactory{columnExtractConfigRepo: columnExtractConfigRepo}
}

func clipSpanField(content string) string {
	if content == "" || len(content) <= clipProcessorPlainTextMaxLength {
		return content
	}
	if clipped, ok := clipJSONContent(content); ok {
		return clipped
	}
	return clipPlainText(content)
}

func clipPlainText(content string) string {
	if len(content) <= clipProcessorPlainTextMaxLength {
		return content
	}
	return clipByByteLimit(content, clipProcessorPlainTextMaxLength) + clipProcessorSuffix
}

func clipJSONValueString(content string) string {
	if len(content) <= clipProcessorJSONValueMaxLength {
		return content
	}
	return clipByByteLimit(content, clipProcessorJSONValueMaxLength) + clipProcessorSuffix
}

func clipByByteLimit(content string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if limit >= len(content) {
		return content
	}
	cutoff := limit
	for shift := 0; cutoff > 0 && shift < utf8.UTFMax && !utf8.RuneStart(content[cutoff]); shift++ {
		cutoff--
	}
	if cutoff == 0 {
		return ""
	}
	return content[:cutoff]
}

func clipJSONContent(content string) (string, bool) {
	if !json.Valid([]byte(content)) {
		return "", false
	}
	var data interface{}
	if err := json.Unmarshal([]byte(content), &data); err != nil {
		return "", false
	}
	clippedData, changed := clipJSONValue(data)
	if !changed {
		return "", false
	}
	clippedStr, err := json.MarshalString(clippedData)
	if err != nil {
		return "", false
	}
	return clippedStr, true
}

func clipJSONValue(value interface{}) (interface{}, bool) {
	switch val := value.(type) {
	case map[string]interface{}:
		changed := false
		for key, v := range val {
			newVal, subChanged := clipJSONValue(v)
			if subChanged {
				val[key] = newVal
				changed = true
			}
		}
		return val, changed
	case []interface{}:
		changed := false
		for idx, v := range val {
			newVal, subChanged := clipJSONValue(v)
			if subChanged {
				val[idx] = newVal
				changed = true
			}
		}
		return val, changed
	case string:
		clipped := clipJSONValueString(val)
		if clipped != val {
			return clipped, true
		}
		return val, false
	default:
		return val, false
	}
}

func findExtractRules(rules []entity.ColumnExtractRule) (inputRule, outputRule *entity.ColumnExtractRule) {
	for i := range rules {
		switch rules[i].Column {
		case "input":
			inputRule = &rules[i]
		case "output":
			outputRule = &rules[i]
		}
	}
	return
}

func extractAndClip(content string, column string, rule *entity.ColumnExtractRule) string {
	if rule != nil {
		jsonPath := normalizeJSONPath(column, rule.JSONPath)
		if extracted := extractByJSONPath(content, jsonPath); extracted != "" {
			return clipSpanField(extracted)
		}
	}
	return clipSpanField(content)
}

func normalizeJSONPath(column, jsonPath string) string {
	jsonPath = strings.TrimPrefix(jsonPath, column+".")
	if !strings.HasPrefix(jsonPath, "$") {
		jsonPath = "$." + jsonPath
	}
	return jsonPath
}

func extractByJSONPath(content, jsonPath string) string {
	if content == "" || jsonPath == "" {
		return ""
	}
	if !json.Valid([]byte(content)) {
		return ""
	}
	result, err := json.GetStringByJSONPath(content, jsonPath)
	if err != nil {
		return ""
	}
	return result
}
