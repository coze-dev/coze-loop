// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

const WakeMessageTag = "lifecycle_hook_wake"

// WakePublisher publishes only after the database commit. Failure leaves the
// durable pending work for the scanner; it must not roll back or recreate a Run.
type WakePublisher interface {
	PublishWake(context.Context, entity.HookWakeEvent) error
}

// WakeHandler accepts untrusted hints, not sending permits. The worker must read
// the scoped database state and acquire a fenced claim before sending HTTP.
type WakeHandler interface {
	Wake(context.Context, entity.HookWakeEvent) error
}
