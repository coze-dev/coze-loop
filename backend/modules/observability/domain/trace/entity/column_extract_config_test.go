// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestColumnExtractConfigs_SelectBest(t *testing.T) {
	makeConfig := func(wsID int64, agentName, platformType, spanListType string) *ColumnExtractConfig {
		return &ColumnExtractConfig{
			WorkspaceID:  wsID,
			AgentName:    agentName,
			PlatformType: platformType,
			SpanListType: spanListType,
			Columns: []ColumnExtractRule{
				{Column: "input", JSONPath: "$.test"},
			},
		}
	}

	// Simulate DB result: default configs + workspace configs
	allConfigs := ColumnExtractConfigs{
		// workspace-specific configs
		makeConfig(42, "my_bot", "coze_loop", "llm_span"),
		makeConfig(42, "", "coze_loop", "llm_span"),
		// default configs (wsID=0, with wildcards)
		makeConfig(0, "", "*", "llm_span"), // all platform llm_span default
		makeConfig(0, "", "prompt", "*"),   // prompt platform default
		makeConfig(0, "", "*", "*"),        // universal fallback
	}

	tests := []struct {
		name         string
		configs      ColumnExtractConfigs
		workspaceId  int64
		agentName    string
		platformType string
		spanListType string
		wantWsID     int64
		wantAgent    string
		wantPlatform string
		wantSpan     string
		wantNil      bool
	}{
		{
			name:         "exact match: ws + agent + platform + spanList",
			configs:      allConfigs,
			workspaceId:  42,
			agentName:    "my_bot",
			platformType: "coze_loop",
			spanListType: "llm_span",
			wantWsID:     42,
			wantAgent:    "my_bot",
			wantPlatform: "coze_loop",
			wantSpan:     "llm_span",
		},
		{
			name:         "ws match, no agent match -> ws no_agent config",
			configs:      allConfigs,
			workspaceId:  42,
			agentName:    "other_bot",
			platformType: "coze_loop",
			spanListType: "llm_span",
			wantWsID:     42,
			wantAgent:    "",
			wantPlatform: "coze_loop",
			wantSpan:     "llm_span",
		},
		{
			name:         "no ws match, llm_span -> default *, llm_span",
			configs:      allConfigs,
			workspaceId:  999,
			agentName:    "any_bot",
			platformType: "coze_loop",
			spanListType: "llm_span",
			wantWsID:     0,
			wantAgent:    "",
			wantPlatform: "*",
			wantSpan:     "llm_span",
		},
		{
			name:         "no ws match, prompt platform -> default prompt, *",
			configs:      allConfigs,
			workspaceId:  999,
			agentName:    "",
			platformType: "prompt",
			spanListType: "all_span",
			wantWsID:     0,
			wantAgent:    "",
			wantPlatform: "prompt",
			wantSpan:     "*",
		},
		{
			name:         "prompt + llm_span: spanList exact (*,llm_span) wins over platform exact (prompt,*)",
			configs:      allConfigs,
			workspaceId:  999,
			agentName:    "",
			platformType: "prompt",
			spanListType: "llm_span",
			wantWsID:     0,
			wantAgent:    "",
			wantPlatform: "*",
			wantSpan:     "llm_span",
			// score: (*,llm_span) = 0+0+2+0 = 2; (prompt,*) = 0+0+0+1 = 1 -> llm_span wins
		},
		{
			name:         "unknown platform + unknown spanList -> universal fallback",
			configs:      allConfigs,
			workspaceId:  999,
			agentName:    "",
			platformType: "custom",
			spanListType: "custom_span",
			wantWsID:     0,
			wantAgent:    "",
			wantPlatform: "*",
			wantSpan:     "*",
		},
		{
			name:         "empty configs returns nil",
			configs:      nil,
			workspaceId:  42,
			agentName:    "bot",
			platformType: "coze_loop",
			spanListType: "llm_span",
			wantNil:      true,
		},
		{
			name:         "cross-workspace config is NOT used",
			configs:      ColumnExtractConfigs{makeConfig(200, "bot", "coze_loop", "llm_span")},
			workspaceId:  42,
			agentName:    "bot",
			platformType: "coze_loop",
			spanListType: "llm_span",
			wantNil:      true,
		},
		{
			name:         "no agentName: skip agent-specific configs, use ws empty-agent config",
			configs:      allConfigs,
			workspaceId:  42,
			agentName:    "",
			platformType: "coze_loop",
			spanListType: "llm_span",
			wantWsID:     42,
			wantAgent:    "",
			wantPlatform: "coze_loop",
			wantSpan:     "llm_span",
		},
		{
			name: "no agentName + no ws match: skip agent-specific, use default empty-agent",
			configs: ColumnExtractConfigs{
				makeConfig(0, "bot_a", "*", "llm_span"),
				makeConfig(0, "", "*", "llm_span"),
			},
			workspaceId:  999,
			agentName:    "",
			platformType: "coze_loop",
			spanListType: "llm_span",
			wantWsID:     0,
			wantAgent:    "",
			wantPlatform: "*",
			wantSpan:     "llm_span",
		},
		{
			name: "no agentName: only agent-specific configs -> nil",
			configs: ColumnExtractConfigs{
				makeConfig(42, "bot_a", "coze_loop", "llm_span"),
				makeConfig(0, "bot_b", "*", "llm_span"),
			},
			workspaceId:  42,
			agentName:    "",
			platformType: "coze_loop",
			spanListType: "llm_span",
			wantNil:      true,
		},
		{
			name: "ws match + spanList match but agent and platform mismatch -> fallback to universal",
			configs: ColumnExtractConfigs{
				makeConfig(42, "bot_a", "prompt", "llm_span"), // agent mismatch, platform mismatch
				makeConfig(0, "", "*", "*"),                   // universal fallback
			},
			workspaceId:  42,
			agentName:    "bot_x",
			platformType: "coze_loop",
			spanListType: "llm_span",
			wantWsID:     0,
			wantAgent:    "",
			wantPlatform: "*",
			wantSpan:     "*",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.configs.SelectBest(tt.workspaceId, tt.agentName, tt.platformType, tt.spanListType)
			if tt.wantNil {
				require.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			require.Equal(t, tt.wantWsID, got.WorkspaceID)
			require.Equal(t, tt.wantAgent, got.AgentName)
			require.Equal(t, tt.wantPlatform, got.PlatformType)
			require.Equal(t, tt.wantSpan, got.SpanListType)
		})
	}
}

func TestConfigScore(t *testing.T) {
	cfg := &ColumnExtractConfig{
		WorkspaceID: 42, AgentName: "bot", PlatformType: "coze_loop", SpanListType: "llm_span",
	}
	// Full exact match = 8+4+2+1 = 15
	require.Equal(t, 15, configScore(cfg, 42, "bot", "coze_loop", "llm_span"))

	// Cross-workspace = -1
	require.Equal(t, -1, configScore(cfg, 999, "bot", "coze_loop", "llm_span"))

	// Agent mismatch (non-empty agent in config, different query agent) = -1
	require.Equal(t, -1, configScore(cfg, 42, "other", "coze_loop", "llm_span"))

	// Default ws=0, wildcard platform/span
	defaultCfg := &ColumnExtractConfig{
		WorkspaceID: 0, AgentName: "", PlatformType: "*", SpanListType: "*",
	}
	// score = 0 (default ws) + 0 (empty agent) + 0 (wildcard platform) + 0 (wildcard span) = 0
	require.Equal(t, 0, configScore(defaultCfg, 42, "bot", "coze_loop", "llm_span"))

	// Default ws=0, exact span
	llmDefault := &ColumnExtractConfig{
		WorkspaceID: 0, AgentName: "", PlatformType: "*", SpanListType: "llm_span",
	}
	// score = 0 + 0 + 2 + 0 = 2
	require.Equal(t, 2, configScore(llmDefault, 42, "bot", "coze_loop", "llm_span"))

	// Default ws=0, exact platform
	promptDefault := &ColumnExtractConfig{
		WorkspaceID: 0, AgentName: "", PlatformType: "prompt", SpanListType: "*",
	}
	// score = 0 + 0 + 0 + 1 = 1
	require.Equal(t, 1, configScore(promptDefault, 42, "", "prompt", "all_span"))

	// Platform mismatch, not wildcard = -1
	require.Equal(t, -1, configScore(promptDefault, 42, "", "coze_loop", "llm_span"))
}

func TestColumnExtractConfig_Extract(t *testing.T) {
	cfg := &ColumnExtractConfig{
		Columns: []ColumnExtractRule{
			{Column: "input", JSONPath: "$.messages[-1:].content"},
			{Column: "output", JSONPath: "$.choices[0].message.content"},
		},
	}

	t.Run("extract input", func(t *testing.T) {
		content := `{"messages":[{"role":"user","content":"Hello"}]}`
		result := cfg.Extract(content, "input")
		require.Equal(t, "Hello", result)
	})

	t.Run("extract output", func(t *testing.T) {
		content := `{"choices":[{"message":{"role":"assistant","content":"Hi there!"}}]}`
		result := cfg.Extract(content, "output")
		require.Equal(t, "Hi there!", result)
	})

	t.Run("column not found", func(t *testing.T) {
		result := cfg.Extract(`{"key":"value"}`, "unknown")
		require.Equal(t, "", result)
	})

	t.Run("nil config", func(t *testing.T) {
		var nilCfg *ColumnExtractConfig
		result := nilCfg.Extract(`{"key":"value"}`, "input")
		require.Equal(t, "", result)
	})

	t.Run("empty columns", func(t *testing.T) {
		emptyCfg := &ColumnExtractConfig{Columns: nil}
		result := emptyCfg.Extract(`{"key":"value"}`, "input")
		require.Equal(t, "", result)
	})

	t.Run("invalid json returns empty", func(t *testing.T) {
		result := cfg.Extract("not json", "input")
		require.Equal(t, "", result)
	})

	t.Run("recursive descent", func(t *testing.T) {
		recursiveCfg := &ColumnExtractConfig{
			Columns: []ColumnExtractRule{
				{Column: "input", JSONPath: "$..content"},
			},
		}
		content := `{"stream":[[{"role":"user","content":"你好"}]]}`
		result := recursiveCfg.Extract(content, "input")
		require.Equal(t, "你好", result)
	})

	t.Run("nested json string", func(t *testing.T) {
		nestedCfg := &ColumnExtractConfig{
			Columns: []ColumnExtractRule{
				{Column: "input", JSONPath: "$.arguments.city"},
			},
		}
		content := `{"arguments":"{\"city\":\"Beijing\"}"}`
		result := nestedCfg.Extract(content, "input")
		require.Equal(t, "Beijing", result)
	})
}

func TestExtractByJSONPath(t *testing.T) {
	t.Run("empty content", func(t *testing.T) {
		require.Equal(t, "", extractByJSONPath("", "$.key"))
	})

	t.Run("empty jsonpath", func(t *testing.T) {
		require.Equal(t, "", extractByJSONPath(`{"key":"value"}`, ""))
	})

	t.Run("invalid json", func(t *testing.T) {
		require.Equal(t, "", extractByJSONPath("not json", "$.key"))
	})

	t.Run("normal jsonpath", func(t *testing.T) {
		result := extractByJSONPath(`{"key":"value"}`, "$.key")
		require.Equal(t, "value", result)
	})

	t.Run("recursive descent", func(t *testing.T) {
		result := extractByJSONPath(`{"a":{"content":"hello"}}`, "$..content")
		require.Equal(t, "hello", result)
	})

	t.Run("jsonpath no match returns empty", func(t *testing.T) {
		result := extractByJSONPath(`{"key":"value"}`, "$.nonexistent")
		require.Equal(t, "", result)
	})
}
