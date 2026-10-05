// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"
	"encoding/json"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
	"time"
)

type selectionPlainProtector struct{}

func (selectionPlainProtector) Protect(_ context.Context, _ string, p []byte) ([]byte, error) {
	return append([]byte(nil), p...), nil
}
func (selectionPlainProtector) Unprotect(_ context.Context, _ string, p []byte) ([]byte, error) {
	return append([]byte(nil), p...), nil
}

func TestSnapshotSelectionRoundtripAndLegacyBytes(t *testing.T) {
	codec := NewStorageCodec(selectionPlainProtector{})
	s := codecSnapshot(t)
	in := s.Input()
	p, err := codec.EncodeSnapshot(context.Background(), "key-1", s)
	require.NoError(t, err)
	legacy := struct {
		Version        int                       `json:"version"`
		Purpose        string                    `json:"purpose"`
		Key            entity.HookRunKey         `json:"run_key"`
		ExecutionScope string                    `json:"execution_scope"`
		CreatedAt      time.Time                 `json:"created_at"`
		Config         *entity.LifecycleHookConf `json:"config"`
		Context        *storedHookContext        `json:"context"`
	}{1, "run_snapshot", in.Key, in.ExecutionScope, in.CreatedAt, in.Config, &storedHookContext{HookRunContext: in.Context, EvalSets: in.Context.EvalSets}}
	want, err := json.Marshal(legacy)
	require.NoError(t, err)
	require.Equal(t, want, p.Cipher)
	old, err := codec.DecodeSnapshot(context.Background(), in.Key, in.ExecutionScope, p)
	require.NoError(t, err)
	require.Nil(t, old.Input().Selection)
	in.Selection = &entity.HookSelectionSeed{Version: 1, TrialRunItemCount: 0, ConfigFingerprint: strings.Repeat("a", 64)}
	s, err = entity.NewHookRunSnapshot(in)
	require.NoError(t, err)
	p, err = codec.EncodeSnapshot(context.Background(), "key-1", s)
	require.NoError(t, err)
	got, err := NewStorageCodec(selectionPlainProtector{}).DecodeSnapshot(context.Background(), in.Key, in.ExecutionScope, p)
	require.NoError(t, err)
	require.Equal(t, in.Selection, got.Input().Selection)
	_, _, err = codec.protect(context.Background(), "key-1", make([]byte, 256*1024+1))
	require.Error(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(p.Cipher, &payload))
	payload["selection"].(map[string]any)["version"] = 2
	bad, err := json.Marshal(payload)
	require.NoError(t, err)
	p.Cipher, p.Hash = bad, hookContentHash(bad)
	_, err = codec.DecodeSnapshot(context.Background(), in.Key, in.ExecutionScope, p)
	require.Error(t, err)
}

func TestSnapshotSelectionNeverEntersSPI(t *testing.T) {
	for _, phase := range []entity.HookPhase{entity.HookPhaseBefore, entity.HookPhaseAfter} {
		t.Run(string(phase), func(t *testing.T) {
			codec, run, claim := claimedSnapshotRun(t, phase)
			old, hash, err := codec.BuildClaimedRequest(context.Background(), "platform-ppe", run, claim)
			require.NoError(t, err)
			oldBody, err := BuildRequest(old)
			require.NoError(t, err)
			s, err := codec.DecodeSnapshot(context.Background(), run.State.Key, "platform-ppe", run.Snapshot)
			require.NoError(t, err)
			in := s.Input()
			in.Selection = &entity.HookSelectionSeed{Version: 1, TrialRunItemCount: 3, ConfigFingerprint: strings.Repeat("d", 64)}
			s, err = entity.NewHookRunSnapshot(in)
			require.NoError(t, err)
			run.Snapshot, err = codec.EncodeSnapshot(context.Background(), "key-1", s)
			require.NoError(t, err)
			request, newHash, err := codec.BuildClaimedRequest(context.Background(), "platform-ppe", run, claim)
			require.NoError(t, err)
			body, err := BuildRequest(request)
			require.NoError(t, err)
			require.Equal(t, oldBody, body)
			require.Equal(t, hash, newHash)
			require.NotContains(t, string(body), "selection")
			require.NotContains(t, string(body), in.Selection.ConfigFingerprint)
		})
	}
}
