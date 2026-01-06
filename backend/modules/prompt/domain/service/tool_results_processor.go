// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	loopslices "github.com/coze-dev/coze-loop/backend/pkg/lang/slices"
)

// ProcessToolResultsParam defines the parameters for processing tool results
type ProcessToolResultsParam struct {
	Prompt           *entity.Prompt
	MockTools        []*entity.MockTool
	Reply            *entity.Reply
	ResultStream     chan<- *entity.Reply                    // only used in streaming mode, can be nil
	ReplyItemWrapper func(v *entity.ReplyItem) *entity.Reply // only used in streaming mode, can be nil
}

// IToolResultsProcessor defines the interface for processing tool results
type IToolResultsProcessor interface {
	ProcessToolResults(ctx context.Context, param ProcessToolResultsParam) (map[string]string, error)
}

// ToolResultsProcessor provides the default implementation of IToolResultsProcessor
type ToolResultsProcessor struct{}

// NewToolResultsProcessor creates a new instance of ToolResultsProcessor
func NewToolResultsProcessor() IToolResultsProcessor {
	return &ToolResultsProcessor{}
}

// ProcessToolResults implements the IToolResultsProcessor interface
func (t *ToolResultsProcessor) ProcessToolResults(ctx context.Context, param ProcessToolResultsParam) (map[string]string, error) {
	toolResultMap := loopslices.ToMap(param.MockTools, func(m *entity.MockTool) (string, string) {
		if m == nil {
			return "", ""
		}
		return m.Name, m.MockResponse
	})
	return toolResultMap, nil
}
