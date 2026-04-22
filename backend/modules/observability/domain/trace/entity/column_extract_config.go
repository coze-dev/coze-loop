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

// SelectBest selects the best matching config using a scoring strategy.
//
// Priority dimensions (highest to lowest weight):
//  1. Workspace: target workspace > default (wsID=0). Cross-workspace configs are excluded.
//  2. Agent: exact agent match > empty agent (wildcard).
//  3. SpanListType: exact match > wildcard '*'.
//  4. PlatformType: exact match > wildcard '*'.
//
// Returns nil if no config matches.
func (configs ColumnExtractConfigs) SelectBest(workspaceId int64, agentName, platformType, spanListType string) *ColumnExtractConfig {
	var (
		best      *ColumnExtractConfig
		bestScore int
	)

	for _, cfg := range configs {
		score := configScore(cfg, workspaceId, agentName, platformType, spanListType)
		if score < 0 {
			continue // not a valid match
		}
		if best == nil || score > bestScore {
			best = cfg
			bestScore = score
		}
	}

	return best
}

// configScore computes a match score for a config. Returns -1 if the config doesn't match.
// Higher score = better match. Score layout (4 bits):
//
//	bit 3 (8): workspace match (target ws=1, default ws=0)
//	bit 2 (4): agent match (exact=1, empty=0)
//	bit 1 (2): spanListType match (exact=1, wildcard=0)
//	bit 0 (1): platformType match (exact=1, wildcard=0)
func configScore(cfg *ColumnExtractConfig, workspaceId int64, agentName, platformType, spanListType string) int {
	// workspace: must be target or default(0), reject cross-workspace
	isTarget := cfg.WorkspaceID == workspaceId
	isDefault := cfg.WorkspaceID == 0
	if !isTarget && !isDefault {
		return -1
	}

	// agent: must be exact match or empty wildcard
	agentMatch := agentName != "" && cfg.AgentName == agentName
	agentEmpty := cfg.AgentName == ""
	if !agentMatch && !agentEmpty {
		return -1
	}

	// platformType: must be exact or wildcard '*'
	platformExact := cfg.PlatformType == platformType
	platformWild := cfg.PlatformType == "*"
	if !platformExact && !platformWild {
		return -1
	}

	// spanListType: must be exact or wildcard '*'
	spanExact := cfg.SpanListType == spanListType
	spanWild := cfg.SpanListType == "*"
	if !spanExact && !spanWild {
		return -1
	}

	score := 0
	if isTarget {
		score |= 8
	}
	if agentMatch {
		score |= 4
	}
	if platformExact {
		score |= 1
	}
	if spanExact {
		score |= 2
	}
	return score
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
