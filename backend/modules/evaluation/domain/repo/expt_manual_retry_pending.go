// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// A pending retry is applied only by the consumer holding the original tick.
type ManualRetryPending struct {
	Request         *entity.RequeueTerminalItemsRequest
	ExpiresAtMS     int64
	PreparedReceipt *entity.ManualRetryReceipt
	AcceptedItemIDs []int64
}

type IExptManualRetryPendingRepo interface {
	RegisterPendingRetry(context.Context, *ManualRetryPending) error
	GetPendingRetry(context.Context, int64, int64, int64) (*ManualRetryPending, error)
	ResolvePendingRetry(context.Context, *ManualRetryPending, bool) (*entity.ManualRetryReceipt, error)
	PreparePendingRetryCommit(context.Context, *ManualRetryPending, *entity.ManualRetryReceipt, []int64) error
	CheckPendingRetry(context.Context, *ManualRetryPending) error
	CompletePendingRetry(context.Context, *ManualRetryPending, *entity.ManualRetryReceipt, error) error
}
