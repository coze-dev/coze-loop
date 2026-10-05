// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// IScheduledExptSubmissionWriter consumes only the transaction-bound provider from Commit.
type IScheduledExptSubmissionWriter interface {
	Write(context.Context, db.Provider, entity.ScheduledRunTrigger, string, *entity.PreparedScheduledExpt) error
}
