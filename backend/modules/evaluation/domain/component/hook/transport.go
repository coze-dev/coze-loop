// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type HTTPTransport interface {
	Invoke(context.Context, entity.HookTransportInput) entity.HookTransportResult
}

// KeyResolver resolves only trusted workspace/target bindings, never user-supplied keys.
// Implementations must honor ctx and must not log or expose secret material.
// The returned key must include its same-snapshot egress policy fingerprint;
// HTTPTransport rejects absent or mismatched fingerprints before DNS or dialing.
type KeyResolver interface {
	Resolve(context.Context, entity.HookKeyBinding) (entity.HookSigningKey, error)
}
