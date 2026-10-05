// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"strings"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
)

func TestHookAttemptStorageValidation(t *testing.T) {
	valid := HookClaimAttemptInput{HookAttemptScope: HookAttemptScope{Key: HookRunKey{1, 2, 3}, OperationID: "op", Phase: HookPhaseBefore, ExecutionScope: "local", SnapshotHash: strings.Repeat("a", 64)}, Owner: "worker", AttemptID: 4, Config: &HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &HookHTTPInfo{URL: gptr.Of("https://example.com/hook")}}}
	require.NoError(t, valid.Validate())
	for name, mutate := range map[string]func(*HookClaimAttemptInput){
		"missing key":        func(in *HookClaimAttemptInput) { in.Key.RunID = 0 },
		"missing owner":      func(in *HookClaimAttemptInput) { in.Owner = "" },
		"missing attempt id": func(in *HookClaimAttemptInput) { in.AttemptID = 0 },
		"missing hash":       func(in *HookClaimAttemptInput) { in.SnapshotHash = "" },
		"missing scope":      func(in *HookClaimAttemptInput) { in.ExecutionScope = "" },
		"negative version":   func(in *HookClaimAttemptInput) { in.ExpectedVersion = -1 },
		"unknown phase":      func(in *HookClaimAttemptInput) { in.Phase = "other" },
		"missing config":     func(in *HookClaimAttemptInput) { in.Config = nil },
		"disabled config":    func(in *HookClaimAttemptInput) { in.Config = &HookConfig{} },
	} {
		t.Run(name, func(t *testing.T) { in := valid; mutate(&in); require.Error(t, in.Validate()) })
	}
}
