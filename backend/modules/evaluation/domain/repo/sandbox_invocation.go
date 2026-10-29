// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// SandboxInvocationRepo is an optional extension for durable pre-dispatch ownership.
// Reads must use the primary; transitions must compare phase and AsyncInvoking status.
type SandboxInvocationRepo interface {
	SandboxInvocationOwnerActive(context.Context, int64, int64, int64, int64) (bool, error)
	LatestSandboxInvocation(context.Context, int64, int64, int64, int64) (*entity.EvalTargetRecord, error)
	TransitionSandboxInvocation(context.Context, *entity.EvalTargetRecord, string) (bool, error)
}
