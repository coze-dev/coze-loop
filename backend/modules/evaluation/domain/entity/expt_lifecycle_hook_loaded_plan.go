// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import "errors"

var (
	ErrHookFrozenPlanInvalid        = errors.New("invalid frozen hook plan page")
	ErrHookFrozenPlanNotReady       = errors.New("frozen hook plan is not ready")
	ErrHookFrozenItemUnavailable    = errors.New("frozen hook item or exact version unavailable")
	ErrHookFrozenSchemaInvalid      = errors.New("invalid frozen hook collection schema")
	ErrHookFrozenContentUnavailable = errors.New("frozen hook content unavailable")
)

// Frozen is authoritative; observed item metadata never upgrades its version 0 guarantee.
type HookLoadedPlanItem struct {
	Ordinal int64
	Frozen  HookPlanItem
	Item    *EvaluationSetItem
}

// Reading a page does not grant admission or permission to initialize execution records.
type HookLoadedPlanPage struct {
	Count       int64
	Hash        string
	RunVersion  int64
	NextOrdinal int64
	HasMore     bool
	Items       []HookLoadedPlanItem
}
