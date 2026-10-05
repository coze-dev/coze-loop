// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"github.com/stretchr/testify/require"
	"math"
	"strings"
	"testing"
)

func TestHookPlanDigestGoldenAndResume(t *testing.T) {
	// Independently computed with SHA256 and six signed 64-bit big-endian values per row.
	require.Equal(t, "a37be24d461361c965b7d31c585c253628f77a928963934bd2c416b83b743867", NewHookPlanDigest().Hash)
	rows := []HookPlanItem{{ID: 100, SourceSpaceID: 10, EvalSetID: 20, ItemID: 30}, {ID: 101, SourceSpaceID: 11, EvalSetID: 21, EvalSetVersionID: 22, ItemID: 31, ItemVersionID: 32}}
	got, err := AppendHookPlanDigest(NewHookPlanDigest(), rows)
	require.NoError(t, err)
	require.Equal(t, int64(2), got.Count)
	require.Equal(t, "1251f545525196c4ce0d2dacb9c285e313b41ebc49c0617f4f5447578e1fea42", got.Hash)
	first, err := AppendHookPlanDigest(NewHookPlanDigest(), rows[:1])
	require.NoError(t, err)
	require.Equal(t, "c1bfdb0334303be70d60a2c8290543159f30018c56f72364ab819aa3812fc1ea", first.Hash)
	restarted := HookPlanDigest{Count: first.Count, Hash: first.Hash}
	resumed, err := AppendHookPlanDigest(restarted, rows[1:])
	require.NoError(t, err)
	require.Equal(t, got, resumed)
}

func TestHookPlanDigestMetadataAndOrder(t *testing.T) {
	original := HookPlanItem{ID: 100, SourceSpaceID: 10, EvalSetID: 20, EvalSetVersionID: 21, ItemID: 30, ItemVersionID: 31}
	want, err := AppendHookPlanDigest(NewHookPlanDigest(), []HookPlanItem{original})
	require.NoError(t, err)
	for _, tc := range []struct {
		name   string
		change func(*HookPlanItem)
	}{
		{"source", func(i *HookPlanItem) { i.SourceSpaceID++ }}, {"set", func(i *HookPlanItem) { i.EvalSetID++ }}, {"set_version", func(i *HookPlanItem) { i.EvalSetVersionID++ }}, {"item", func(i *HookPlanItem) { i.ItemID++ }}, {"item_version", func(i *HookPlanItem) { i.ItemVersionID++ }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			item := original
			tc.change(&item)
			got, err := AppendHookPlanDigest(NewHookPlanDigest(), []HookPlanItem{item})
			require.NoError(t, err)
			require.NotEqual(t, want.Hash, got.Hash)
		})
	}
	copy := original
	copy.ID = 999
	got, err := AppendHookPlanDigest(NewHookPlanDigest(), []HookPlanItem{copy})
	require.NoError(t, err)
	require.Equal(t, want, got)
	copy.ItemID++
	forward, err := AppendHookPlanDigest(NewHookPlanDigest(), []HookPlanItem{original, copy})
	require.NoError(t, err)
	backward, err := AppendHookPlanDigest(NewHookPlanDigest(), []HookPlanItem{copy, original})
	require.NoError(t, err)
	require.NotEqual(t, forward.Hash, backward.Hash)
}

func TestHookPlanDigestRejectsInvalidResumeWithoutMutation(t *testing.T) {
	row := HookPlanItem{SourceSpaceID: 1, EvalSetID: 2, ItemID: 3}
	for _, in := range []HookPlanDigest{{}, {Count: -1, Hash: NewHookPlanDigest().Hash}, {Hash: strings.Repeat("a", 64)}, {Count: 1, Hash: strings.Repeat("A", 64)}, {Count: math.MaxInt64, Hash: NewHookPlanDigest().Hash}} {
		before := in
		_, err := AppendHookPlanDigest(in, []HookPlanItem{row})
		require.Error(t, err)
		require.Equal(t, before, in)
	}
	_, err := AppendHookPlanDigest(NewHookPlanDigest(), make([]HookPlanItem, 101))
	require.Error(t, err)
	for _, bad := range []HookPlanItem{{SourceSpaceID: 0, EvalSetID: 2, ItemID: 3}, {SourceSpaceID: 1, EvalSetID: 0, ItemID: 3}, {SourceSpaceID: 1, EvalSetID: 2, ItemID: 0}, {SourceSpaceID: 1, EvalSetID: 2, ItemID: 3, EvalSetVersionID: -1}, {SourceSpaceID: 1, EvalSetID: 2, ItemID: 3, ItemVersionID: -1}} {
		_, err := AppendHookPlanDigest(NewHookPlanDigest(), []HookPlanItem{row, bad})
		require.Error(t, err)
	}
	empty, err := AppendHookPlanDigest(NewHookPlanDigest(), nil)
	require.NoError(t, err)
	require.Equal(t, NewHookPlanDigest(), empty)
}
