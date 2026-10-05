// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"errors"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

var (
	ErrScheduleReceiptInvalid     = errors.New("invalid schedule receipt request")
	ErrScheduleReceiptUnavailable = errors.New("schedule receipt readback unavailable")
	ErrScheduleReceiptMismatch    = errors.New("schedule receipt does not match registration")
)

// IExptScheduleReceiptAdapter is optional; legacy adapters and noops need not implement it.
// Namespace/group are expected server-owned routing values, not callback authentication.
type IExptScheduleReceiptAdapter interface {
	// CreatePeriodicJobWithReceipt verifies the current job after registration, including ambiguous create outcomes.
	// Only credential-free callback payloads belong here; returned metadata never contains the payload or CtxKvs.
	CreatePeriodicJobWithReceipt(ctx context.Context, param *CreatePeriodicJobParam, namespace, group string) (*entity.ExptTemplateScheduleReceipt, error)
}
