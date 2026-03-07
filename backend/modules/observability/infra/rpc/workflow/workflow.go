// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package workflow

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

type WorkflowProvider struct {
}

func NewWorkflowProvider() rpc.IWorkflowProvider {
	return &WorkflowProvider{}
}

func (w *WorkflowProvider) BatchGetWorkflows(ctx context.Context, spans loop_span.SpanList) (map[string]string, error) {
	return make(map[string]string), nil
}
