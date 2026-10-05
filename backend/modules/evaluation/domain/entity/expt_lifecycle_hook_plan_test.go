// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"github.com/stretchr/testify/require"
	"math"
	"strings"
	"testing"
)

func TestHookPlanReadInputValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*HookPlanReadInput)
		valid  bool
	}{
		{"valid", func(*HookPlanReadInput) {}, true},
		{"zero key", func(in *HookPlanReadInput) { in.Key.RunID = 0 }, false},
		{"negative workspace", func(in *HookPlanReadInput) { in.Key.WorkspaceID = -1 }, false},
		{"missing scope", func(in *HookPlanReadInput) { in.ExecutionScope = "" }, false},
		{"scope space", func(in *HookPlanReadInput) { in.ExecutionScope = "local scope" }, false},
		{"scope too long", func(in *HookPlanReadInput) { in.ExecutionScope = strings.Repeat("a", 129) }, false},
		{"negative ordinal", func(in *HookPlanReadInput) { in.StartOrdinal = -1 }, false},
		{"zero limit", func(in *HookPlanReadInput) { in.Limit = 0 }, false},
		{"large limit", func(in *HookPlanReadInput) { in.Limit = 101 }, false},
		{"maximum ordinal", func(in *HookPlanReadInput) { in.StartOrdinal = math.MaxInt64; in.Limit = 100 }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := HookPlanReadInput{Key: HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}, ExecutionScope: "local", Limit: 1}
			tc.change(&in)
			validator, ok := any(in).(interface{ Validate() error })
			require.True(t, ok, "page boundaries must be checked before querying storage")
			if tc.valid {
				require.NoError(t, validator.Validate())
			} else {
				require.Error(t, validator.Validate())
			}
		})
	}
}

func TestHookAdvancePlanInputValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*HookAdvancePlanInput)
		valid  bool
	}{
		{"valid", func(*HookAdvancePlanInput) {}, true},
		{"bad key", func(in *HookAdvancePlanInput) { in.Key.ExperimentID = 0 }, false},
		{"overflow version", func(in *HookAdvancePlanInput) { in.ExpectedVersion = math.MaxInt64 }, false},
		{"negative version", func(in *HookAdvancePlanInput) { in.ExpectedVersion = -1 }, false},
		{"negative count", func(in *HookAdvancePlanInput) { in.ExpectedCount = -1 }, false},
		{"bad scope", func(in *HookAdvancePlanInput) { in.ExecutionScope = "\n" }, false},
		{"unchanged cursor", func(in *HookAdvancePlanInput) { in.NextCursor = in.Cursor }, false},
		{"invalid cursor UTF8", func(in *HookAdvancePlanInput) { in.Cursor = string([]byte{255}) }, false},
		{"invalid next UTF8", func(in *HookAdvancePlanInput) { in.NextCursor = string([]byte{255}) }, false},
		{"long cursor", func(in *HookAdvancePlanInput) { in.Cursor = strings.Repeat("a", 65536) }, false},
		{"long next", func(in *HookAdvancePlanInput) { in.NextCursor = strings.Repeat("a", 65536) }, false},
		{"cursor boundary", func(in *HookAdvancePlanInput) { in.NextCursor = strings.Repeat("a", 65535) }, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := HookAdvancePlanInput{HookStoreGuard: HookStoreGuard{Key: HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}}, ExecutionScope: "local", Cursor: "", NextCursor: "next"}
			tc.change(&in)
			validator, ok := any(in).(interface{ Validate() error })
			require.True(t, ok, "cursor validation must precede the transaction")
			if tc.valid {
				require.NoError(t, validator.Validate())
			} else {
				require.Error(t, validator.Validate())
			}
		})
	}
}
