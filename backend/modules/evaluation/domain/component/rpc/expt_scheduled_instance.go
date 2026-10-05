// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"errors"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

var (
	ErrScheduledInstanceInvalid     = errors.New("invalid scheduled instance verification input")
	ErrScheduledInstanceUnavailable = errors.New("scheduled instance lookup unavailable")
	ErrScheduledInstanceMismatch    = errors.New("scheduled instance ownership mismatch")
)

// ScheduledInstanceVerifier consumes service-authenticated metadata and a current DB binding.
// It neither authenticates a caller nor authorizes the bound user. Recheck the binding CAS before creating a Run.
type ScheduledInstanceVerifier interface {
	Verify(context.Context, *VerifiedScheduledCallback, *entity.ExptTemplateScheduleBinding) (*ScheduledInstanceOwnership, error)
}

// ScheduledInstanceOwnership contains only registry identifiers. Use InstanceID verbatim for durable deduplication.
type ScheduledInstanceOwnership struct {
	InstanceID     string
	JobID          string
	JobHistoryID   string
	BindingID      string
	BindingVersion int64
	ExecutionScope string
}
