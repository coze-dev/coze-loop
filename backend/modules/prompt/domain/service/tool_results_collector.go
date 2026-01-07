// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
)

// CollectToolResultsParam defines the parameters for processing tool results
type CollectToolResultsParam struct {
	Prompt           *entity.Prompt
	MockTools        []*entity.MockTool
	Reply            *entity.Reply
	ResultStream     chan<- *entity.Reply                    // only used in streaming mode, can be nil
	ReplyItemWrapper func(v *entity.ReplyItem) *entity.Reply // only used in streaming mode, can be nil
}

// IToolResultsCollector defines the interface for processing tool results
//go:generate mockgen -destination=mocks/tool_results_collector.go -package=mocks . IToolResultsCollector
type IToolResultsCollector interface {
	CollectToolResults(ctx context.Context, param CollectToolResultsParam) (map[string]string, error)
}

// ToolResultsCollector provides the default implementation of IToolResultsCollector
type ToolResultsCollector struct{}

// NewToolResultsCollector creates a new instance of ToolResultsCollector
func NewToolResultsCollector() IToolResultsCollector {
	return &ToolResultsCollector{}
}

// CollectToolResults ProcessToolResults implements the IToolResultsCollector interface
func (t *ToolResultsCollector) CollectToolResults(ctx context.Context, param CollectToolResultsParam) (map[string]string, error) {
	toolResultMap := make(map[string]string, len(param.MockTools))
	for _, mockTool := range param.MockTools {
		if mockTool == nil || mockTool.Name == "" {
			continue
		}
		toolResultMap[mockTool.Name] = mockTool.MockResponse
	}
	return toolResultMap, nil
}
