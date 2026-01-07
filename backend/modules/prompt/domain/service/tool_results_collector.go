// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	loopslices "github.com/coze-dev/coze-loop/backend/pkg/lang/slices"
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
	toolResultMap := loopslices.ToMap(param.MockTools, func(m *entity.MockTool) (string, string) {
		if m == nil {
			return "", ""
		}
		return m.Name, m.MockResponse
	})
	return toolResultMap, nil
}
