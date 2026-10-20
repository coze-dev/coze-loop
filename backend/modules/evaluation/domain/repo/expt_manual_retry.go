// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate mockgen -destination=mocks/expt_manual_retry.go -package=mocks . IExptManualRetryRepo
type IExptManualRetryRepo interface {
	PreflightTerminalItems(context.Context, *entity.RequeueTerminalItemsRequest) (*entity.ManualRetryPreparation, error)
	FindReceipt(context.Context, int64, int64, int64, string, string) (*entity.ManualRetryReceipt, error)
	RequeueTerminalItems(context.Context, *entity.RequeueTerminalItemsRequest) (*entity.ManualRetryReceipt, error)
}
