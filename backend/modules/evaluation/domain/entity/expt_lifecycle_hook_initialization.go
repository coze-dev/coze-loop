// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// Initialization reads only the experiment and RunLog, never a protected snapshot.
type HookRunInitialization struct {
	LatestRunID    int64
	ConfigRevision string
	HooksEnabled   bool
	RunLog         *ExptRunLog
	Managed        bool
}

type HookAppendRunItemsInput struct {
	HookStoreGuard
	ExecutionScope string
	ItemIDs        []int64
}
