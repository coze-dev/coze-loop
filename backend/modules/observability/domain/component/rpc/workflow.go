// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

type IWorkflowProvider interface {
	BatchGetWorkflows(ctx context.Context, spans loop_span.SpanList) (map[string]string, error)
}
