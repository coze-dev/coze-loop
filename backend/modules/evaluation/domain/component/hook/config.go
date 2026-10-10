// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type RuntimeConfigProvider interface {
	GetRuntimeConfig(context.Context) (entity.HookRuntimeConfig, error)
}

// SigningSecretProvider reads a dedicated operator-managed reference, never a
// business HookConfig value. Implementations must honor ctx and not log material.
type SigningSecretProvider interface {
	ReadSigningSecret(context.Context, string) ([]byte, error)
}

// WorkspaceSigningSecretProvider uses the server-resolved workspace identity.
// Implementations must honor ctx and never log or expose the returned secret.
type WorkspaceSigningSecretProvider interface {
	GetWorkspaceSigningSecret(context.Context, int64) (string, error)
}
