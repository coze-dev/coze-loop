// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
)

// IdentityProvider enriches an already authorized Run.CreatedBy once at creation.
// It cannot establish authority or update a persisted Run snapshot.
type IdentityProvider interface {
	ResolveInitiator(context.Context, string) (*spi.HookInitiator, error)
}
