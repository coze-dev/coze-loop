// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestHookScheduleCodecRoundTripAndLegacyBytes(t *testing.T) {
	// This existing test compares legacy serialized bytes field-for-field.
	TestSnapshotSelectionRoundtripAndLegacyBytes(t)
	codec := NewStorageCodec(selectionPlainProtector{})
	in := codecSnapshot(t).Input()
	in.Context.Experiment.Type = gptr.Of("offline")
	in.Schedule = &entity.HookScheduleSeed{Version: 1, Key: in.Key, ExecutionScope: in.ExecutionScope,
		Mode: entity.EvaluationModeSubmit, CreatedAt: 1700000010, ItemRetryTimes: 5,
		Session: &entity.Session{UserID: in.Context.GetInitiator().GetUserID(), AppID: 7},
		Ext:     map[string]string{entity.RetryYieldExtKey: "true", "private": "launch-secret"}}
	s, err := entity.NewHookRunSnapshot(in)
	require.NoError(t, err)
	p, err := codec.EncodeSnapshot(context.Background(), "key-1", s)
	require.NoError(t, err)
	got, err := codec.DecodeSnapshot(context.Background(), in.Key, in.ExecutionScope, p)
	require.NoError(t, err)
	require.Equal(t, in.Schedule, got.Input().Schedule)
}

func TestHookScheduleCodecNeverEntersSPI(t *testing.T) {
	codec, run, claim := claimedSnapshotRun(t, entity.HookPhaseBefore)
	s, err := codec.DecodeSnapshot(context.Background(), run.State.Key, "platform-ppe", run.Snapshot)
	require.NoError(t, err)
	in := s.Input()
	in.Context.Experiment.Type = gptr.Of("offline")
	s, err = entity.NewHookRunSnapshot(in)
	require.NoError(t, err)
	run.Snapshot, err = codec.EncodeSnapshot(context.Background(), "key-1", s)
	require.NoError(t, err)
	original, hash, err := codec.BuildClaimedRequest(context.Background(), "platform-ppe", run, claim)
	require.NoError(t, err)
	body, err := BuildRequest(original)
	require.NoError(t, err)
	in.Schedule = &entity.HookScheduleSeed{Version: 1, Key: in.Key, ExecutionScope: in.ExecutionScope,
		Mode: entity.EvaluationModeSubmit, CreatedAt: 1700000010, ItemRetryTimes: 5,
		Session: &entity.Session{UserID: in.Context.GetInitiator().GetUserID()},
		Ext:     map[string]string{entity.RetryYieldExtKey: "true", "private": "schedule-secret"}}
	s, err = entity.NewHookRunSnapshot(in)
	require.NoError(t, err)
	run.Snapshot, err = codec.EncodeSnapshot(context.Background(), "key-1", s)
	require.NoError(t, err)
	request, gotHash, err := codec.BuildClaimedRequest(context.Background(), "platform-ppe", run, claim)
	require.NoError(t, err)
	gotBody, err := BuildRequest(request)
	require.NoError(t, err)
	require.Equal(t, body, gotBody)
	require.Equal(t, hash, gotHash)
}
