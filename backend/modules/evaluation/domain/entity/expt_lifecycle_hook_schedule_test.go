// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"testing"
)

func scheduleSnapshotInput() HookRunSnapshotInput {
	in := snapshotInput()
	in.Context.Experiment.Type = gptr.Of("offline")
	in.Schedule = &HookScheduleSeed{Version: 1, Key: in.Key, ExecutionScope: in.ExecutionScope,
		Mode: EvaluationModeSubmit, CreatedAt: 1700000001, ItemRetryTimes: 3,
		Session: &Session{UserID: "trusted-user", AppID: 7}, Ext: map[string]string{RetryYieldExtKey: "true", "route": "original"}}
	return in
}

func TestHookScheduleSnapshotOwnsSeed(t *testing.T) {
	in := scheduleSnapshotInput()
	s, err := NewHookRunSnapshot(in)
	require.NoError(t, err)
	in.Schedule.Session.UserID = "changed"
	in.Schedule.Ext["route"] = "changed"
	out := s.Input()
	require.Equal(t, "trusted-user", out.Schedule.Session.UserID)
	require.Equal(t, "original", out.Schedule.Ext["route"])
	out.Schedule.CreatedAt++
	out.Schedule.Session.AppID++
	out.Schedule.Ext[RetryYieldExtKey] = "false"
	next := s.Input()
	require.Equal(t, int64(1700000001), next.Schedule.CreatedAt)
	require.Equal(t, int32(7), next.Schedule.Session.AppID)
	require.Equal(t, "true", next.Schedule.Ext[RetryYieldExtKey])
}

func TestHookScheduleSnapshotRejectsMismatchedSeed(t *testing.T) {
	for _, change := range []string{"key", "scope", "creator", "mode", "version", "deadline", "retry", "yield", "session", "online"} {
		t.Run(change, func(t *testing.T) {
			in := scheduleSnapshotInput()
			switch change {
			case "key":
				in.Schedule.Key.RunID++
			case "scope":
				in.Schedule.ExecutionScope = "other"
			case "creator":
				in.Schedule.Session.UserID = "other"
			case "mode":
				in.Schedule.Mode = EvaluationModeTrialRun
			case "version":
				in.Schedule.Version++
			case "deadline":
				in.Schedule.CreatedAt = 0
			case "retry":
				in.Schedule.ItemRetryTimes = -1
			case "yield":
				delete(in.Schedule.Ext, RetryYieldExtKey)
			case "session":
				in.Schedule.Session = nil
			case "online":
				in.Context.Experiment.Type = gptr.Of("online")
			}
			_, err := NewHookRunSnapshot(in)
			require.Error(t, err)
		})
	}
}

func TestHookScheduleRetrySnapshotRetainsModeAndDeadline(t *testing.T) {
	for _, tc := range []struct {
		mode ExptRunMode
		name string
	}{{EvaluationModeFailRetry, "fail_retry"}, {EvaluationModeRetryAll, "retry_all"}, {EvaluationModeRetryItems, "retry_items"}} {
		t.Run(tc.name, func(t *testing.T) {
			in := scheduleSnapshotInput()
			in.Schedule.Mode = tc.mode
			in.Context.RunMode = gptr.Of(tc.name)
			event, err := in.Schedule.Event(in.Key, in.ExecutionScope, tc.mode, "trusted-user")
			require.NoError(t, err)
			require.Equal(t, tc.mode, event.ExptRunMode)
			require.Equal(t, int64(1700000001), event.CreatedAt)
			snapshot, err := NewHookRunSnapshot(in)
			require.NoError(t, err)
			require.Equal(t, tc.name, snapshot.Input().Context.GetRunMode())
			require.Equal(t, in.Schedule, snapshot.Input().Schedule)
		})
	}
}

func TestHookScheduleRetryStillRejectsUnsupportedModesAndOnline(t *testing.T) {
	for _, mode := range []ExptRunMode{0, 99} {
		in := scheduleSnapshotInput()
		in.Schedule.Mode = mode
		_, err := in.Schedule.Event(in.Key, in.ExecutionScope, mode, "trusted-user")
		require.Error(t, err)
	}
	appendInput := scheduleSnapshotInput()
	appendInput.Schedule.Mode = EvaluationModeAppend
	appendInput.Context.RunMode = gptr.Of("append")
	_, appendErr := NewHookRunSnapshot(appendInput)
	require.Error(t, appendErr, "Append schedule requires Online, not an offline snapshot")
	in := scheduleSnapshotInput()
	in.Schedule.Mode = EvaluationModeFailRetry
	in.Context.RunMode = gptr.Of("fail_retry")
	in.Context.Experiment.Type = gptr.Of("online")
	_, err := NewHookRunSnapshot(in)
	require.Error(t, err)
}
