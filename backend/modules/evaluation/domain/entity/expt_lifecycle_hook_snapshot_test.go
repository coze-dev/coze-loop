// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"strings"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	"github.com/stretchr/testify/require"
)

func snapshotInput() HookRunSnapshotInput {
	return HookRunSnapshotInput{
		Key: HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}, ExecutionScope: "ppe_platform",
		CreatedAt: time.Date(2026, 9, 23, 1, 2, 3, 0, time.UTC),
		Config:    &LifecycleHookConf{Before: &HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &HookHTTPInfo{URL: gptr.Of("https://hook.example/run?private=yes")}, ParametersJSON: gptr.Of(`{"context":{"user_id":"not-an-identity"},"nested":[false,null,9007199254740993]}`)}, After: &HookConfig{Enabled: gptr.Of(false)}},
		Context: &spi.HookRunContext{WorkspaceID: gptr.Of("1"), ExperimentID: gptr.Of("2"), RunID: gptr.Of("3"), RunMode: gptr.Of("submit"),
			Initiator:  &spi.HookInitiator{UserID: gptr.Of("trusted-user"), IdentityType: gptr.Of("fornax_user"), Email: gptr.Of("user@example.com"), Name: gptr.Of("名字")},
			Experiment: &spi.HookExperimentRef{Name: gptr.Of("实验"), Type: gptr.Of("online")}, EvalSets: []*spi.HookEvalSetRef{},
		},
	}
}

func TestHookSnapshotOwnsAllInputAndOutputPointers(t *testing.T) {
	in := snapshotInput()
	in.Context.EvalSets = []*spi.HookEvalSetRef{{WorkspaceID: gptr.Of("4"), ID: gptr.Of("5"), VersionID: gptr.Of("6"), Version: gptr.Of("v1")}}
	in.Context.Target = &spi.HookTargetRef{ID: gptr.Of("7"), VersionID: gptr.Of("8"), Type: gptr.Of("agent")}
	frozen, err := NewHookRunSnapshot(in)
	require.NoError(t, err)
	*in.Context.Initiator.UserID = "changed"
	*in.Config.Before.Enabled = false
	*in.Config.Before.InvokeHTTPInfo.URL = "https://changed.example"
	*in.Context.EvalSets[0].Version = "changed"
	*in.Context.Target.ID = "9"
	out := frozen.Input()
	require.Equal(t, "trusted-user", out.Context.Initiator.GetUserID())
	require.Equal(t, "v1", out.Context.EvalSets[0].GetVersion())
	require.Equal(t, "7", out.Context.Target.GetID())
	require.True(t, *out.Config.Before.Enabled)
	require.Equal(t, "https://hook.example/run?private=yes", *out.Config.Before.InvokeHTTPInfo.URL)
	require.Equal(t, int32(180), *out.Config.Before.TimeoutSeconds)
	require.Equal(t, HookEnvironmentProd, *out.Config.Before.Environment)
	require.Equal(t, "ppe_platform", out.ExecutionScope)
	*out.Context.WorkspaceID = "9"
	*out.Context.Initiator.Name = "changed"
	*out.Context.Experiment.Name = "changed"
	*out.Config.Before.Retry.Enabled = false
	*out.Config.Before.ParametersJSON = "{}"
	out.Context.EvalSets[0] = nil
	next := frozen.Input()
	require.Equal(t, "1", next.Context.GetWorkspaceID())
	require.Equal(t, "名字", next.Context.Initiator.GetName())
	require.Equal(t, "实验", next.Context.Experiment.GetName())
	require.True(t, *next.Config.Before.Retry.Enabled)
	require.Contains(t, *next.Config.Before.ParametersJSON, "9007199254740993")
	require.NotNil(t, next.Context.EvalSets[0])
	require.False(t, *next.Config.After.Enabled)
}

func TestHookSnapshotOnlineEmptyAndOptionalValues(t *testing.T) {
	in := snapshotInput()
	in.Context.Initiator.Email, in.Context.Initiator.Name = nil, nil
	in.Config.After = nil
	frozen, err := NewHookRunSnapshot(in)
	require.NoError(t, err)
	out := frozen.Input()
	require.NotNil(t, out.Context.EvalSets)
	require.Empty(t, out.Context.EvalSets)
	require.Nil(t, out.Context.Target)
	require.Nil(t, out.Context.Initiator.Email)
	require.Nil(t, out.Config.After)
	require.Equal(t, in.CreatedAt, out.CreatedAt)
}

func TestHookSnapshotRejectsInvalidFrozenInputs(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*HookRunSnapshotInput)
	}{
		{"no enabled phase", func(in *HookRunSnapshotInput) { in.Config.Before.Enabled = gptr.Of(false) }},
		{"no configuration", func(in *HookRunSnapshotInput) { in.Config = nil }},
		{"no context", func(in *HookRunSnapshotInput) { in.Context = nil }},
		{"wrong run", func(in *HookRunSnapshotInput) { in.Context.RunID = gptr.Of("4") }},
		{"wrong space", func(in *HookRunSnapshotInput) { in.Key.WorkspaceID = 2 }},
		{"missing scope", func(in *HookRunSnapshotInput) { in.ExecutionScope = "" }},
		{"invalid scope", func(in *HookRunSnapshotInput) { in.ExecutionScope = "ppe lane" }},
		{"scope limit", func(in *HookRunSnapshotInput) { in.ExecutionScope = strings.Repeat("s", 129) }},
		{"missing creation", func(in *HookRunSnapshotInput) { in.CreatedAt = time.Time{} }},
		{"untrusted identity shape", func(in *HookRunSnapshotInput) { in.Context.Initiator.IdentityType = gptr.Of("service") }},
		{"missing user", func(in *HookRunSnapshotInput) { in.Context.Initiator.UserID = nil }},
		{"terminal at creation", func(in *HookRunSnapshotInput) { in.Context.TerminalStatus = gptr.Of("success") }},
		{"nil online sets", func(in *HookRunSnapshotInput) { in.Context.EvalSets = nil }},
		{"UTF8", func(in *HookRunSnapshotInput) { in.Context.Initiator.Name = gptr.Of("\xff") }},
		{"name byte limit", func(in *HookRunSnapshotInput) { in.Context.Experiment.Name = gptr.Of(strings.Repeat("中", 171)) }},
		{"parameter type", func(in *HookRunSnapshotInput) { in.Config.Before.ParametersJSON = gptr.Of(`[]`) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := snapshotInput()
			tc.change(&in)
			got, err := NewHookRunSnapshot(in)
			require.Error(t, err)
			require.Nil(t, got)
			require.NotContains(t, err.Error(), "private=yes")
		})
	}
}
