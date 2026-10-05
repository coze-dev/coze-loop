// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import "context"

// PlanPreparer prepares one bounded page; finalization and wake effects have separate owners.
type PlanPreparer interface {
	PreparePlan(context.Context, WorkerRunInput) error
}
