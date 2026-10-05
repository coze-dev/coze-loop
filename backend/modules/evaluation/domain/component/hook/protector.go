// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// Protector uses a trusted key reference; implementations must not expose plaintext in errors.
// Implementations must authenticate ciphertext and reject tampering on decryption.
type Protector interface {
	Protect(context.Context, string, []byte) ([]byte, error)
	Unprotect(context.Context, string, []byte) ([]byte, error)
}

type ConfigOwnerKind string

const (
	ConfigOwnerExperiment ConfigOwnerKind = "experiment"
	ConfigOwnerTemplate   ConfigOwnerKind = "template"
)

// ConfigOwner comes from the authorized storage path, never from the envelope being read.
type ConfigOwner struct {
	WorkspaceID    int64           `json:"workspace_id"`
	ObjectID       int64           `json:"object_id"`
	Kind           ConfigOwnerKind `json:"kind"`
	ExecutionScope string          `json:"execution_scope"`
}

type StorageCodec interface {
	EncodeConfig(context.Context, string, ConfigOwner, *entity.LifecycleHookConf) ([]byte, error)
	DecodeConfig(context.Context, ConfigOwner, []byte) (*entity.LifecycleHookConf, error)
	EncodeSnapshot(context.Context, string, *entity.HookRunSnapshot) (entity.HookProtectedSnapshot, error)
	DecodeSnapshot(context.Context, entity.HookRunKey, string, entity.HookProtectedSnapshot) (*entity.HookRunSnapshot, error)
	DecodePhase(context.Context, entity.HookRunKey, string, entity.HookProtectedSnapshot, entity.HookPhase) (*entity.HookConfig, string, error)
}
