// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// IExptRetryItemResetRepo atomically initializes one item for a new run.
// A current-run item log is the idempotency marker: its presence skips every write.
type IExptRetryItemResetRepo interface {
	ResetItemForRetry(ctx context.Context, spaceID, exptID, runID int64, runLog *entity.ExptItemResultRunLog, turnIDs []int64) (applied bool, err error)
}

// IExptRetryRunStatusRepo guards the final Processing transition by current run
// and preserves cancellation states such as Terminating.
type IExptRetryRunStatusRepo interface {
	MarkRetryProcessing(ctx context.Context, spaceID, exptID, runID int64) (processing bool, err error)
}
