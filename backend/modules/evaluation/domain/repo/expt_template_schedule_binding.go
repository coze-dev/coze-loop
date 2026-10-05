// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// IExptTemplateScheduleBindingRepo accepts trusted server input only; it does not authorize users or callbacks.
// WithTransaction joins a caller-owned template transaction; changed=true is durable only after its commit.
type IExptTemplateScheduleBindingRepo interface {
	// Get returns nil only for SQL NULL; missing templates and malformed envelopes are errors.
	Get(context.Context, entity.ExptTemplateScheduleBindingKey, ...db.Option) (*entity.ExptTemplateScheduleBinding, error)
	// SaveCAS saves a pending binding at expectedVersion+1; expectedVersion=0 requires an unbound template.
	SaveCAS(context.Context, entity.ExptTemplateScheduleBindingKey, int64, *entity.ExptTemplateScheduleBinding, ...db.Option) (bool, error)
	// ActivateCAS preserves the binding version; the adapter must supply a successful, matched job receipt.
	ActivateCAS(context.Context, entity.ExptTemplateScheduleBindingKey, string, int64, entity.ExptTemplateScheduleReceipt, ...db.Option) (bool, error)
	// DisableCAS advances the version so an earlier registration receipt cannot reactivate the binding.
	DisableCAS(context.Context, entity.ExptTemplateScheduleBindingKey, string, int64, ...db.Option) (bool, error)
}
