// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"strings"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
)

func validHookStorageCreate() HookCreateRunInput {
	return HookCreateRunInput{Key: HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3},
		RunLog:   &ExptRunLog{ID: 3, SpaceID: 1, ExptID: 2, ExptRunID: 3, CreatedBy: "user", Mode: 3, Status: 2},
		Snapshot: HookProtectedSnapshot{Cipher: []byte{1, 2, 3}, KeyID: "key", Hash: strings.Repeat("a", 64), ExecutionScope: "local"},
		Before:   &HookOperationSeed{ID: 4, OperationID: "hook_before", IdempotencyKey: "before-key"},
		After:    &HookOperationSeed{ID: 5, OperationID: "hook_after", IdempotencyKey: "after-key"}}
}

func TestHookStorageCreateValidation(t *testing.T) {
	require.NoError(t, validHookStorageCreate().Validate())
	for _, tc := range []struct {
		name   string
		change func(*HookCreateRunInput)
	}{
		{"missing run", func(i *HookCreateRunInput) { i.RunLog = nil }},
		{"wrong workspace", func(i *HookCreateRunInput) { i.Key.WorkspaceID = 0 }},
		{"mismatched identity", func(i *HookCreateRunInput) { i.RunLog.ExptID = 99 }},
		{"mismatched log id", func(i *HookCreateRunInput) { i.RunLog.ID = 99 }},
		{"no author", func(i *HookCreateRunInput) { i.RunLog.CreatedBy = "" }},
		{"unknown mode", func(i *HookCreateRunInput) { i.RunLog.Mode = 0 }},
		{"terminal create", func(i *HookCreateRunInput) { i.RunLog.Status = 11 }},
		{"no cipher", func(i *HookCreateRunInput) { i.Snapshot.Cipher = nil }},
		{"no key", func(i *HookCreateRunInput) { i.Snapshot.KeyID = "" }},
		{"bad hash", func(i *HookCreateRunInput) { i.Snapshot.Hash = "invalid" }},
		{"scope not ASCII", func(i *HookCreateRunInput) { i.Snapshot.ExecutionScope = "本地" }},
		{"no hooks", func(i *HookCreateRunInput) { i.Before = nil; i.After = nil }},
		{"duplicate id", func(i *HookCreateRunInput) { i.After.ID = i.Before.ID }},
		{"duplicate operation", func(i *HookCreateRunInput) { i.After.OperationID = i.Before.OperationID }},
		{"duplicate key", func(i *HookCreateRunInput) { i.After.IdempotencyKey = i.Before.IdempotencyKey }},
		{"unallocated id", func(i *HookCreateRunInput) { i.Before.ID = 0 }},
		{"trailing space", func(i *HookCreateRunInput) { i.Before.IdempotencyKey = "key " }},
		{"source is self", func(i *HookCreateRunInput) { i.SourceRunID = gptr.Of(int64(3)) }},
		{"invalid expected latest", func(i *HookCreateRunInput) { i.ExpectedLatestRunID = -1 }},
	} {
		t.Run(tc.name, func(t *testing.T) { in := validHookStorageCreate(); tc.change(&in); require.Error(t, in.Validate()) })
	}
}

func TestHookStoragePlanAndGuardValidation(t *testing.T) {
	g := HookStoreGuard{Key: HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}}
	require.NoError(t, g.Validate())
	p := HookPlanPageInput{HookStoreGuard: g, NextCursor: "next", Items: []HookPlanItem{{ID: 4, SourceSpaceID: 1, EvalSetID: 8, ItemID: 10}}}
	require.NoError(t, p.Validate())
	require.NoError(t, HookFinishPlanInput{HookStoreGuard: g, Hash: strings.Repeat("b", 64)}.Validate())
	require.NoError(t, HookFinalizeInput{HookStoreGuard: g, Intent: HookTerminalIntent{Status: ExptStatus_Terminated}}.Validate())
	require.NoError(t, HookAdmitItemInput{HookStoreGuard: g, ItemID: 10}.Validate())
	require.Error(t, HookStoreGuard{Key: g.Key, ExpectedVersion: -1}.Validate())
	require.Error(t, HookStoreGuard{}.Validate())
	require.Error(t, HookPlanPageInput{HookStoreGuard: g}.Validate())
	p.Items = append(p.Items, p.Items[0])
	require.Error(t, p.Validate())
	p.Items = p.Items[:1]
	p.Items[0].ItemVersionID = -1
	require.Error(t, p.Validate())
	p.Items[0].ItemVersionID = 0
	p.StartOrdinal = -1
	require.Error(t, p.Validate())
	require.Error(t, HookFinishPlanInput{HookStoreGuard: g, Count: -1, Hash: strings.Repeat("b", 64)}.Validate())
	require.Error(t, HookFinishPlanInput{HookStoreGuard: g, Hash: ""}.Validate())
	require.Error(t, HookFinalizeInput{HookStoreGuard: g, Intent: HookTerminalIntent{Status: ExptStatus_Draining}}.Validate())
	require.Error(t, HookFinalizeInput{HookStoreGuard: g, Intent: HookTerminalIntent{Status: ExptStatus_Success, Reason: strings.Repeat("x", 129)}}.Validate())
	require.Error(t, HookAdmitItemInput{HookStoreGuard: g}.Validate())
}
