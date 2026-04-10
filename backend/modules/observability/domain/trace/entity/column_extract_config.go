// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"strings"
	"time"

	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

type ColumnExtractRule struct {
	Column   string
	JSONPath string
}

type ColumnExtractConfig struct {
	ID           int64
	WorkspaceID  int64
	PlatformType string
	SpanListType string
	AgentName    string
	Columns      []ColumnExtractRule
	CreatedAt    time.Time
	CreatedBy    string
	UpdatedAt    time.Time
	UpdatedBy    string
}

// Extract extracts the value from content using the JSONPath rule for the given column.
// Returns the extracted string, or empty string if extraction fails.
func (c *ColumnExtractConfig) Extract(content string, column string) string {
	if c == nil || len(c.Columns) == 0 {
		return ""
	}
	var rule *ColumnExtractRule
	for i := range c.Columns {
		if c.Columns[i].Column == column {
			rule = &c.Columns[i]
			break
		}
	}
	if rule == nil {
		return ""
	}
	return extractByJSONPath(content, rule.JSONPath)
}

// ColumnExtractConfigs is a list of ColumnExtractConfig with selection logic.
type ColumnExtractConfigs []*ColumnExtractConfig

func (configs ColumnExtractConfigs) SelectBest(workspaceId int64, agentName, platformType, spanListType string) *ColumnExtractConfig {
	var bestConfig *ColumnExtractConfig
	for _, cfg := range configs {
		if cfg.WorkspaceID == workspaceId && cfg.PlatformType == platformType && cfg.SpanListType == spanListType && cfg.AgentName == agentName {
			bestConfig = cfg
			break
		} else if cfg.WorkspaceID == workspaceId && cfg.PlatformType == platformType && cfg.SpanListType == spanListType && cfg.AgentName == "" {
			bestConfig = cfg
		}
	}
	// Fallback to  default
	if bestConfig == nil {
		for _, cfg := range configs {
			if cfg.WorkspaceID == 0 && cfg.AgentName == "" && cfg.SpanListType == spanListType &&
				(cfg.PlatformType == platformType || cfg.PlatformType == "*") {
				bestConfig = cfg
				break
			}
		}
	}
	return bestConfig
}

func extractByJSONPath(content, jsonPath string) string {
	if content == "" || jsonPath == "" {
		return ""
	}
	if !json.Valid([]byte(content)) {
		return ""
	}
	// For recursive descent queries ($..field), take the last match
	if strings.Contains(jsonPath, "..") {
		result, err := json.GetLastStringByJSONPath(content, jsonPath)
		if err != nil {
			return ""
		}
		return result
	}
	result, err := json.GetStringByJSONPathRecursively(content, jsonPath)
	if err != nil {
		return ""
	}
	return result
}
