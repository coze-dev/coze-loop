// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// IHookPlanResultReader checks current base results after validating the Hook Run.
// Results from any Run/status count; executionScope is supplied by trusted wiring.
type IHookPlanResultReader interface {
	HasExperimentResults(context.Context, entity.HookRunKey, string) (bool, error)
}
