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
	if jsonPath == "" {
		return ""
	}
	if strings.HasPrefix(jsonPath, "$") {
		return escapeJSONPathKeys(jsonPath)
	}
	// strip column prefix: "input.xxx" -> "xxx", "input[0]" -> "[0]"
	stripped := strings.TrimPrefix(jsonPath, column)
	stripped = strings.TrimPrefix(stripped, ".")
	if stripped == "" {
		return ""
	}
	var result string
	if strings.HasPrefix(stripped, "[") {
		result = "$" + stripped
	} else {
		result = "$." + stripped
	}
	return escapeJSONPathKeys(result)
}

// escapeJSONPathKeys converts dot-notation keys containing special characters
// to bracket notation. e.g. "$.extra.openai-request-id" -> `$.extra["openai-request-id"]`
func escapeJSONPathKeys(jsonPath string) string {
	if !strings.HasPrefix(jsonPath, "$") {
		return jsonPath
	}
	rest := jsonPath[1:]
	if rest == "" {
		return jsonPath
	}

	var builder strings.Builder
	builder.WriteByte('$')

	for len(rest) > 0 {
		if rest[0] == '.' {
			rest = rest[1:]
			// find end of key (next '.', '[', or end)
			end := 0
			for end < len(rest) && rest[end] != '.' && rest[end] != '[' {
				end++
			}
			key := rest[:end]
			rest = rest[end:]
			if needsBracket(key) {
				builder.WriteString(`["`)
				builder.WriteString(key)
				builder.WriteString(`"]`)
			} else {
				builder.WriteByte('.')
				builder.WriteString(key)
			}
		} else if rest[0] == '[' {
			// find closing bracket
			end := strings.IndexByte(rest, ']')
			if end == -1 {
				builder.WriteString(rest)
				break
			}
			builder.WriteString(rest[:end+1])
			rest = rest[end+1:]
		} else {
			builder.WriteByte(rest[0])
			rest = rest[1:]
		}
	}
	return builder.String()
}

// needsBracket returns true if a key contains characters that require bracket notation.
func needsBracket(key string) bool {
	for _, c := range key {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '_' {
			return true
		}
	}
	return false
}

func extractByJSONPath(content, jsonPath string) string {
	if content == "" || jsonPath == "" {
		return ""
	}
	if !json.Valid([]byte(content)) {
		return ""
	}
	result, err := json.GetStringByJSONPathRecursively(content, jsonPath)
	if err != nil {
		return ""
	}
	return result
}
