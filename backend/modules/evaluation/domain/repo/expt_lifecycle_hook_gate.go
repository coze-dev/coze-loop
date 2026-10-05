// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// IHookGateRepo provides a shared read-only prefilter, not atomic item admission.
// Unavailable state returns waiting plus a safe error; ordinary waiting/closed is not an error.
type IHookGateRepo interface {
	CanDispatch(context.Context, entity.HookRunKey) (entity.HookAdmissionDecision, error)
}

// IHookItemSourceRepo classifies a persisted Run without locks, configuration or Hook tables.
// Only LatestRunID, Managed and RunLog identity/status are populated; this is not initialization.
type IHookItemSourceRepo interface {
	ReadItemSource(context.Context, entity.HookRunKey) (*entity.HookRunInitialization, error)
}
