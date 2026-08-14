// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package span_processor

import (
	"context"
	"strconv"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

type CheckProcessor struct {
	workspaceId int64
}

func (c *CheckProcessor) GetName() string {
	return "CheckProcessor"
}

func (c *CheckProcessor) Transform(ctx context.Context, spans loop_span.SpanList) (loop_span.SpanList, error) {
	workspaceIdStr := strconv.FormatInt(c.workspaceId, 10)
	for _, span := range spans {
		if span.WorkspaceID == workspaceIdStr {
			return spans, nil
		}
	}
	if len(spans) > 0 {
		return nil, errorx.NewByCode(obErrorx.TraceNotInSpaceErrorCode)
	} else {
		return spans, nil
	}
}

type CheckProcessorFactory struct{}

func (c *CheckProcessorFactory) CreateProcessor(ctx context.Context, set Settings) (Processor, error) {
	return &CheckProcessor{
		workspaceId: set.WorkspaceId,
	}, nil
}

func NewCheckProcessorFactory() Factory {
	return new(CheckProcessorFactory)
}
