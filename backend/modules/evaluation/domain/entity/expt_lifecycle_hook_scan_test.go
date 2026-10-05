// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHookScanInputValidation(t *testing.T) {
	now := time.Date(2026, 9, 22, 12, 0, 0, 123000000, time.UTC)
	valid := HookScanInput{ExecutionScope: "ppe_hooks", Status: HookOperationPending, Now: now, Limit: 100}
	for _, tc := range []struct {
		name   string
		mutate func(*HookScanInput)
	}{
		{"empty_scope", func(in *HookScanInput) { in.ExecutionScope = "" }},
		{"blank_scope", func(in *HookScanInput) { in.ExecutionScope = " " }},
		{"padded_scope", func(in *HookScanInput) { in.ExecutionScope = "ppe_hooks " }},
		{"oversized_scope", func(in *HookScanInput) { in.ExecutionScope = strings.Repeat("a", 129) }},
		{"non_ascii_scope", func(in *HookScanInput) { in.ExecutionScope = "生产" }},
		{"control_scope", func(in *HookScanInput) { in.ExecutionScope = "ppe\x00hooks" }},
		{"zero_limit", func(in *HookScanInput) { in.Limit = 0 }},
		{"negative_limit", func(in *HookScanInput) { in.Limit = -1 }},
		{"excess_limit", func(in *HookScanInput) { in.Limit = 101 }},
		{"missing_now", func(in *HookScanInput) { in.Now = time.Time{} }},
		{"submillisecond_now", func(in *HookScanInput) { in.Now = now.Add(time.Nanosecond) }},
		{"out_of_mysql_range", func(in *HookScanInput) { in.Now = time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC) }},
		{"unbounded_status", func(in *HookScanInput) { in.Status = "" }},
		{"not_due_status", func(in *HookScanInput) { in.Status = HookOperationRunning }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := valid
			tc.mutate(&in)
			require.Error(t, in.Validate(HookScanDue))
		})
	}
	for _, status := range []HookOperationStatus{HookOperationPending, HookOperationRetryWait} {
		in := valid
		in.Status = status
		for _, limit := range []int{1, 100} {
			in.Limit = limit
			require.NoError(t, in.Validate(HookScanDue))
		}
	}
	require.Error(t, valid.Validate("unknown"))
	for _, kind := range []HookScanKind{HookScanExpired, HookScanPreparing, HookScanFinalize} {
		require.Error(t, valid.Validate(kind))
		in := valid
		in.Status = ""
		require.NoError(t, in.Validate(kind))
	}
}

func TestHookScanCursorValidation(t *testing.T) {
	now := time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		kind                HookScanKind
		status, boundStatus string
	}{
		{HookScanDue, "pending", "pending"}, {HookScanExpired, "", "running"},
		{HookScanPreparing, "", "preparing"}, {HookScanFinalize, "", "pending"},
	} {
		t.Run(string(tc.kind), func(t *testing.T) {
			cursor := HookScanCursor{Kind: tc.kind, ExecutionScope: "scope", Status: tc.boundStatus, Now: now}
			if tc.kind == HookScanPreparing || tc.kind == HookScanFinalize {
				cursor.WorkspaceID, cursor.RunID = 10, 20
			} else {
				cursor.ID = 20
			}
			if tc.kind != HookScanPreparing {
				cursor.At = now.Add(-time.Hour)
			}
			in := HookScanInput{ExecutionScope: "scope", Status: HookOperationStatus(tc.status), Now: now, Limit: 1, Cursor: &cursor}
			require.NoError(t, in.Validate(tc.kind))
			for _, bad := range []struct {
				name   string
				mutate func(*HookScanCursor)
			}{
				{"cross_kind", func(c *HookScanCursor) { c.Kind = "wrong" }},
				{"cross_scope", func(c *HookScanCursor) { c.ExecutionScope = "other" }},
				{"cross_status", func(c *HookScanCursor) { c.Status = "retry_wait" }},
				{"rewound_now", func(c *HookScanCursor) { c.Now = now.Add(time.Second) }},
				{"advanced_now", func(c *HookScanCursor) { c.Now = now.Add(-time.Second) }},
				{"changed_clock_location", func(c *HookScanCursor) { c.Now = now.In(time.FixedZone("other", 8*3600)) }},
				{"inverted_time", func(c *HookScanCursor) { c.At = now.Add(time.Millisecond) }},
				{"missing_identity", func(c *HookScanCursor) { c.ID, c.WorkspaceID, c.RunID = 0, 0, 0 }},
				{"negative_identity", func(c *HookScanCursor) { c.ID, c.WorkspaceID, c.RunID = -1, -1, -1 }},
				{"mixed_identity", func(c *HookScanCursor) { c.ID, c.WorkspaceID, c.RunID = 1, 1, 1 }},
				{"imprecise_time", func(c *HookScanCursor) { c.At = now.Add(-time.Nanosecond) }},
			} {
				t.Run(bad.name, func(t *testing.T) {
					c := cursor
					bad.mutate(&c)
					copy := in
					copy.Cursor = &c
					require.Error(t, copy.Validate(tc.kind))
				})
			}
			if tc.kind != HookScanPreparing {
				c := cursor
				c.At = time.Time{}
				in.Cursor = &c
				require.Error(t, in.Validate(tc.kind))
				c.At = now // Inclusive upper boundary, including lease equality.
				require.NoError(t, in.Validate(tc.kind))
			}
		})
	}
}
