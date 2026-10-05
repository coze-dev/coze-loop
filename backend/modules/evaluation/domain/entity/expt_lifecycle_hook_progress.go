// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type HookTurnProgressKey struct {
	HookRunKey
	LogID, ItemID, ItemVersionID, TurnID int64
}

func HookTurnProgressIdentity(row *ExptTurnResultRunLog) HookTurnProgressKey {
	if row == nil {
		return HookTurnProgressKey{}
	}
	return HookTurnProgressKey{HookRunKey: HookRunKey{WorkspaceID: row.SpaceID, ExperimentID: row.ExptID, RunID: row.ExptRunID}, LogID: row.ID, ItemID: row.ItemID, ItemVersionID: row.ItemVersionID, TurnID: row.TurnID}
}

func (k HookTurnProgressKey) Validate() error {
	if (HookStoreGuard{Key: k.HookRunKey}).Validate() != nil || k.LogID <= 0 || k.ItemID <= 0 || k.TurnID < 0 || k.ItemVersionID < 0 {
		return ErrHookStoreCorrupt
	}
	return nil
}

// Base is the read snapshot; Progress carries no authority to replace the current row.
type HookTurnProgressInput struct {
	Base, Progress *ExptTurnResultRunLog
}

type HookItemRunWriteInput struct {
	HookRunKey
	ItemID, ItemVersionID int64
	Status                ItemRunState
	ErrMsg                *string
}
