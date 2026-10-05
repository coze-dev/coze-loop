// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestHookRetryItemsPromotesFrozenB0(t *testing.T) {
	key := HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}
	logs := []ExptRunLogItems{{ItemIDs: []int64{11}}, {ItemIDs: []int64{12, 13}}}
	digest := NewHookPlanDigest()
	b := retryItemsBootstrap{Version: 1, Key: key, Fingerprint: strings.Repeat("a", 64), Phase: "verify", Route: "normal", Batch: 1, Selected: digest, Verified: digest, SourceDigest: digest}
	raw, err := json.Marshal(b)
	require.NoError(t, err)
	promoted, err := PromoteHookRetryItemsCursor(string(raw), key, logs, 0, digest.Hash)
	require.NoError(t, err)
	c, err := DecodeHookRetryItemsCursor(promoted, key, logs, 0, digest.Hash)
	require.NoError(t, err)
	require.Equal(t, 1, c.Batch)
	ids, pending := c.Page(logs)
	require.True(t, pending)
	require.Equal(t, []int64{12, 13}, ids)
	c = c.Advance(logs)
	require.Equal(t, 2, c.Batch)
	require.Zero(t, c.Offset)
	closed, err := CloseHookRetryItemsCursor(promoted, key, logs, 0, digest.Hash, "cancel")
	require.NoError(t, err)
	c, err = DecodeHookRetryItemsCursor(closed, key, logs, 0, digest.Hash)
	require.NoError(t, err)
	require.Equal(t, 1, c.Terminal.Batch)
	require.Equal(t, 2, c.Terminal.AcceptedBatches)
	_, err = DecodeHookRetryItemsCursor(closed, key, append(logs, ExptRunLogItems{}), 0, digest.Hash)
	require.Error(t, err)
}

func TestHookRetryItemsBootstrapCancellation(t *testing.T) {
	key := HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}
	logs := []ExptRunLogItems{{ItemIDs: []int64{11, 12}}}
	closed, err := CloseHookRetryItemsCursor("", key, logs, 0, "", "cancel")
	require.NoError(t, err)
	c, err := DecodeHookRetryItemsCursor(closed, key, logs, 0, "")
	require.NoError(t, err)
	require.Equal(t, "bootstrap_closed", c.Phase)
	require.Zero(t, c.Batch)
	require.Equal(t, 1, c.Terminal.AcceptedBatches)
	_, err = PromoteHookRetryItemsCursor("", key, logs, 0, NewHookPlanDigest().Hash)
	require.Error(t, err)
}

func TestHookRetryItemsPromoteLastVerifyReceipt(t *testing.T) {
	key := HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}
	root := NewHookPlanDigest()
	digest, err := AppendHookPlanDigest(root, []HookPlanItem{{ID: 90, SourceSpaceID: 1, EvalSetID: 8, ItemID: 11}})
	require.NoError(t, err)
	b := retryItemsBootstrap{Version: 1, Key: key, Fingerprint: strings.Repeat("a", 64), Phase: "verify", Route: "normal", Batch: 1, Selected: digest, Verified: root, SourceDigest: root}
	raw, err := json.Marshal(b)
	require.NoError(t, err)
	_, err = PromoteHookRetryItemsCursor(string(raw), key, []ExptRunLogItems{{ItemIDs: []int64{11}}, {ItemIDs: []int64{12}}}, 1, digest.Hash)
	require.NoError(t, err, "FinishPlan verifies its final page without persisting that page's cursor")
}

func TestHookRetryItemsScheduleSeed(t *testing.T) {
	key := HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}
	s := &HookScheduleSeed{Version: 1, Key: key, ExecutionScope: "local", Mode: EvaluationModeRetryItems, CreatedAt: 100, Session: &Session{UserID: "opaque-user"}, Ext: map[string]string{RetryYieldExtKey: "false"}}
	event, err := s.Event(key, "local", EvaluationModeRetryItems, "opaque-user")
	require.NoError(t, err)
	require.Equal(t, int64(100), event.CreatedAt)
	require.Equal(t, "opaque-user", event.Session.UserID)
}
