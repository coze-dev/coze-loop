// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// ScheduledRunSQLSubmit writes a PREPARED local submission using only the supplied transaction-bound provider.
// The trigger IDs and user ID come from locked database rows. Do not retain the provider, commit it,
// call external services, allocate IDs, or invoke the ordinary Submit/Create path under this lock.
// All rows (including evaluator refs and Hook state) must join this transaction. Publish only after Commit succeeds.
type ScheduledRunSQLSubmit func(context.Context, entity.ScheduledRunTrigger, string, db.Provider) error

// IScheduledRunTriggerRepo is an optional internal boundary, never a request-body persistence API.
// Callers must verify the service, job/instance ownership and current user permissions first.
// Both methods reread and match the complete active binding plus cron state under a master row lock.
// Errors return no receipt; on an ambiguous commit retry the same binding/instance, never new logical IDs.
type IScheduledRunTriggerRepo interface {
	Reserve(context.Context, *entity.ExptTemplateScheduleBinding, string, entity.ScheduledRunTriggerIDs) (*entity.ScheduledRunTrigger, error)
	// Commit serializes binding changes with SQL submission and pending->submitted. Replays never invoke SQL again.
	Commit(context.Context, *entity.ExptTemplateScheduleBinding, string, ScheduledRunSQLSubmit) (*entity.ScheduledRunTrigger, error)
}
