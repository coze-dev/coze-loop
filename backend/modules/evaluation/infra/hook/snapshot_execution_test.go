// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

func executionCodecInput(t *testing.T) entity.HookRunSnapshotInput {
	t.Helper()
	in := codecSnapshot(t).Input()
	in.Context.Experiment.Type = gptr.Of("offline")
	in.Context.Target = nil
	in.Context.EvalSets = []*spi.HookEvalSetRef{{WorkspaceID: gptr.Of("1"), ID: gptr.Of("11")}}
	in.Execution = &entity.HookExecutionSnapshot{Version: 1, Key: in.Key, ExecutionScope: in.ExecutionScope, Mode: entity.EvaluationModeSubmit, RunModeConfig: &entity.RunModeConfig{SuaGoal: "private execution goal"}, Sets: []entity.HookExecutionSet{{EvalSetID: 11, EvalSetVersionID: 11, ItemConfig: &entity.ExptItemConfig{EvalTargetConf: &entity.ItemTargetConf{TargetVersionID: 92, DynamicConf: map[string]string{"private config": "value"}}}}}}
	return in
}

func TestExecutionSnapshotAuthenticatedRoundtripAndHash(t *testing.T) {
	in := executionCodecInput(t)
	codec := NewStorageCodec(NewDKMSProtector(newSnapshotDKMS()))
	s, err := entity.NewHookRunSnapshot(in)
	require.NoError(t, err)
	protected, err := codec.EncodeSnapshot(context.Background(), "key-1", s)
	require.NoError(t, err)
	require.NotContains(t, string(protected.Cipher), "private execution goal")
	got, err := codec.DecodeSnapshot(context.Background(), in.Key, in.ExecutionScope, protected)
	require.NoError(t, err)
	require.Equal(t, in.Execution, got.Input().Execution)
	policy, hash, err := codec.DecodePhase(context.Background(), in.Key, in.ExecutionScope, protected, entity.HookPhaseBefore)
	require.NoError(t, err)
	require.True(t, *policy.Enabled)
	require.Equal(t, protected.Hash, hash)
	again, err := codec.EncodeSnapshot(context.Background(), "key-1", s)
	require.NoError(t, err)
	require.Equal(t, protected.Hash, again.Hash, "random AEAD nonce does not change content hash")
	got.Input().Execution.Sets[0].ItemConfig.EvalTargetConf.DynamicConf["private config"] = "copy change"
	require.Equal(t, "value", got.Input().Execution.Sets[0].ItemConfig.EvalTargetConf.DynamicConf["private config"])
	in.Execution.Sets[0].ItemConfig.EvalTargetConf.DynamicConf["private config"] = "new config"
	s, err = entity.NewHookRunSnapshot(in)
	require.NoError(t, err)
	changed, err := codec.EncodeSnapshot(context.Background(), "key-1", s)
	require.NoError(t, err)
	require.NotEqual(t, protected.Hash, changed.Hash)
	changed.Hash = protected.Hash
	_, err = codec.DecodeSnapshot(context.Background(), in.Key, in.ExecutionScope, changed)
	require.Error(t, err)
}

func TestExecutionSnapshotNeverEntersSPIOrBusinessHash(t *testing.T) {
	for _, phase := range []entity.HookPhase{entity.HookPhaseBefore, entity.HookPhaseAfter} {
		t.Run(string(phase), func(t *testing.T) {
			codec, run, claim := claimedSnapshotRun(t, phase)
			original, err := codec.DecodeSnapshot(context.Background(), run.State.Key, "platform-ppe", run.Snapshot)
			require.NoError(t, err)
			in := executionCodecInput(t)
			in.Config = original.Input().Config
			in.Execution = nil
			s, err := entity.NewHookRunSnapshot(in)
			require.NoError(t, err)
			run.Snapshot, err = codec.EncodeSnapshot(context.Background(), "key-1", s)
			require.NoError(t, err)
			old, oldHash, err := codec.BuildClaimedRequest(context.Background(), "platform-ppe", run, claim)
			require.NoError(t, err)
			oldBody, err := BuildRequest(old)
			require.NoError(t, err)
			oldSnapshotHash := run.Snapshot.Hash
			in.Execution = executionCodecInput(t).Execution
			s, err = entity.NewHookRunSnapshot(in)
			require.NoError(t, err)
			run.Snapshot, err = codec.EncodeSnapshot(context.Background(), "key-1", s)
			require.NoError(t, err)
			require.NotEqual(t, oldSnapshotHash, run.Snapshot.Hash)
			request, hash, err := codec.BuildClaimedRequest(context.Background(), "platform-ppe", run, claim)
			require.NoError(t, err)
			body, err := BuildRequest(request)
			require.NoError(t, err)
			require.Equal(t, oldBody, body)
			require.Equal(t, oldHash, hash)
			require.NotContains(t, string(body), "execution")
			require.NotContains(t, string(body), "private config")
			run.CreatedBy = "another-user"
			_, _, err = codec.BuildClaimedRequest(context.Background(), "platform-ppe", run, claim)
			require.Error(t, err)
		})
	}
}

func TestExecutionSnapshotProtectedInvalidMetadataAndLegacy(t *testing.T) {
	codec := NewStorageCodec(selectionPlainProtector{})
	in := executionCodecInput(t)
	s, err := entity.NewHookRunSnapshot(in)
	require.NoError(t, err)
	p, err := codec.EncodeSnapshot(context.Background(), "key-1", s)
	require.NoError(t, err)
	for _, field := range []string{"version", "mode", "execution_scope", "run_key", "sets", "unknown"} {
		t.Run(field, func(t *testing.T) {
			var payload map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(p.Cipher, &payload))
			var execution map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(payload["execution"], &execution))
			values := map[string]string{"version": "2", "mode": "2", "execution_scope": `"wrong"`, "run_key": `{"workspace_id":1,"experiment_id":2,"run_id":4}`, "sets": "[]", "unknown": "true"}
			execution[field] = json.RawMessage(values[field])
			payload["execution"], err = json.Marshal(execution)
			require.NoError(t, err)
			raw, err := json.Marshal(payload)
			require.NoError(t, err)
			bad := p
			bad.Cipher, bad.Hash = raw, hookContentHash(raw)
			_, err = codec.DecodeSnapshot(context.Background(), in.Key, in.ExecutionScope, bad)
			require.Error(t, err)
		})
	}
	var legacy map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(p.Cipher, &legacy))
	delete(legacy, "execution")
	raw, err := json.Marshal(legacy)
	require.NoError(t, err)
	p.Cipher, p.Hash = raw, hookContentHash(raw)
	old, err := codec.DecodeSnapshot(context.Background(), in.Key, in.ExecutionScope, p)
	require.NoError(t, err)
	require.Nil(t, old.Input().Execution, "do not synthesize absent private metadata")
	for _, size := range []int{128 * 1024, maxHookPlaintextBytes + 1} {
		in.Execution.RunModeConfig.SuaGoal = strings.Repeat("x", size)
		s, err = entity.NewHookRunSnapshot(in)
		require.NoError(t, err)
		_, err = codec.EncodeSnapshot(context.Background(), "key-1", s)
		if size < maxHookPlaintextBytes {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}
}
