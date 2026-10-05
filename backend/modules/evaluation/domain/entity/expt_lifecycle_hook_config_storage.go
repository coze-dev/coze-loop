// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import "errors"

var (
	ErrHookConfigImmutable = errors.New("HOOK_CONFIG_IMMUTABLE")
	ErrHookConfigStorage   = errors.New("hook configuration storage failed")
)

// Revision is opaque and bound to the stored bytes; empty means SQL NULL/zero length.
type HookConfigRecord struct {
	Config   *LifecycleHookConf `json:"-"`
	Revision string
}

type HookConfigUpdateInput struct {
	Config           *LifecycleHookConf `json:"-"`
	ExpectedRevision string
	KeyID            string `json:"-"`
}
