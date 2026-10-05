// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

const privatePrecisionJSON = `{"fraction":0.123456789012345678901,"nested":[-9007199254740993,{"exponent":1.2300e+20,"tiny":1e-1000}],"opaque_id":9007199254740993}`

func privatePrecisionInput(t *testing.T) entity.HookRunSnapshotInput {
	t.Helper()
	in := executionCodecInput(t)
	in.Execution.Sets[0].ItemConfig.EvalTargetConf.RunConf = &entity.ItemRunConf{FixedQueryList: []*entity.FixedQuery{{Evaluators: map[string]interface{}{
		"opaque_id": json.Number("9007199254740993"), "fraction": json.Number("0.123456789012345678901"),
		"nested": []interface{}{json.Number("-9007199254740993"), map[string]interface{}{"exponent": json.Number("1.2300e+20"), "tiny": json.Number("1e-1000")}},
	}}}}
	return in
}

func assertPrivatePrecision(t *testing.T, s *entity.HookRunSnapshot) {
	t.Helper()
	encoded, err := json.Marshal(s.Input().Execution.Sets[0].ItemConfig.EvalTargetConf.RunConf.FixedQueryList[0].Evaluators)
	require.NoError(t, err)
	require.Equal(t, privatePrecisionJSON, string(encoded))
}

func TestPrivateExecutionPrecisionEncryptedRoundtrip(t *testing.T) {
	ctx := context.Background()
	in := privatePrecisionInput(t)
	dkms := newSnapshotDKMS()
	codec := NewStorageCodec(NewDKMSProtector(dkms))
	s, err := entity.NewHookRunSnapshot(in)
	require.NoError(t, err)
	protected, err := codec.EncodeSnapshot(ctx, "key-1", s)
	require.NoError(t, err)
	plain, err := dkms.Decrypt(ctx, protected.KeyID, string(protected.Cipher))
	require.NoError(t, err)
	require.Contains(t, plain, `"opaque_id":9007199254740993`)
	require.Contains(t, plain, `"fraction":0.123456789012345678901`)
	got, err := codec.DecodeSnapshot(ctx, in.Key, in.ExecutionScope, protected)
	require.NoError(t, err)
	assertPrivatePrecision(t, got)
	again, err := codec.EncodeSnapshot(ctx, "key-1", got)
	require.NoError(t, err)
	require.Equal(t, protected.Hash, again.Hash)
}

func TestPrivateExecutionPrecisionDecodeOriginalEncryptedPayload(t *testing.T) {
	ctx := context.Background()
	in := privatePrecisionInput(t)
	dkms := newSnapshotDKMS()
	codec := NewStorageCodec(NewDKMSProtector(dkms))
	base := executionCodecInput(t)
	s, err := entity.NewHookRunSnapshot(base)
	require.NoError(t, err)
	p, err := codec.EncodeSnapshot(ctx, "key-1", s)
	require.NoError(t, err)
	plain, err := dkms.Decrypt(ctx, p.KeyID, string(p.Cipher))
	require.NoError(t, err)
	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(plain), &payload))
	payload["execution"], err = json.Marshal(in.Execution)
	require.NoError(t, err)
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"opaque_id":9007199254740993`)
	sealed, err := dkms.Encrypt(ctx, p.KeyID, string(raw))
	require.NoError(t, err)
	p.Cipher, p.Hash = []byte(sealed), hookContentHash(raw)
	got, err := codec.DecodeSnapshot(ctx, in.Key, in.ExecutionScope, p)
	require.NoError(t, err)
	assertPrivatePrecision(t, got)
	_, hash, err := codec.DecodePhase(ctx, in.Key, in.ExecutionScope, p, entity.HookPhaseBefore)
	require.NoError(t, err)
	require.Equal(t, p.Hash, hash)
}

func TestPrivateExecutionPrecisionDoesNotChangeGenericJSON(t *testing.T) {
	var old map[string]interface{}
	require.NoError(t, decodeHookStorageJSON([]byte(`{"opaque_id":9007199254740993}`), &old))
	require.IsType(t, float64(0), old["opaque_id"], "legacy generic decoder must retain its semantics")
}

func TestPrivateExecutionPrecisionKeepsStrictPayloadRejection(t *testing.T) {
	ctx := context.Background()
	in := privatePrecisionInput(t)
	dkms := newSnapshotDKMS()
	codec := NewStorageCodec(NewDKMSProtector(dkms))
	s, err := entity.NewHookRunSnapshot(in)
	require.NoError(t, err)
	p, err := codec.EncodeSnapshot(ctx, "key-1", s)
	require.NoError(t, err)
	plain, err := dkms.Decrypt(ctx, p.KeyID, string(p.Cipher))
	require.NoError(t, err)
	for _, tc := range []struct{ name, raw string }{
		{"unknown-execution", strings.Replace(plain, `"execution":{`, `"execution":{"unexpected":1,`, 1)},
		{"unknown-typed-nested", strings.Replace(plain, `"run_conf":{`, `"run_conf":{"unexpected":1,`, 1)},
		{"malformed-number", strings.Replace(plain, `"opaque_id":9007199254740993`, `"opaque_id":01`, 1)},
		{"duplicate-opaque-key", strings.Replace(plain, `"opaque_id":9007199254740993`, `"opaque_id":9007199254740993,"opaque_id":2`, 1)},
		{"trailing-value", plain + ` {}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NotEqual(t, plain, tc.raw)
			sealed, err := dkms.Encrypt(ctx, p.KeyID, tc.raw)
			require.NoError(t, err)
			bad := p
			bad.Cipher, bad.Hash = []byte(sealed), hookContentHash([]byte(tc.raw))
			got, err := codec.DecodeSnapshot(ctx, in.Key, in.ExecutionScope, bad)
			require.Error(t, err)
			require.Nil(t, got)
		})
	}
}

func TestPrivateExecutionPrecisionSPIBodyAndBusinessHashUnchanged(t *testing.T) {
	ctx := context.Background()
	for _, phase := range []entity.HookPhase{entity.HookPhaseBefore, entity.HookPhaseAfter} {
		t.Run(string(phase), func(t *testing.T) {
			codec, run, claim := claimedSnapshotRun(t, phase)
			original, err := codec.DecodeSnapshot(ctx, run.State.Key, "platform-ppe", run.Snapshot)
			require.NoError(t, err)
			in := privatePrecisionInput(t)
			in.Config = original.Input().Config
			private := in.Execution
			in.Execution = nil
			s, err := entity.NewHookRunSnapshot(in)
			require.NoError(t, err)
			run.Snapshot, err = codec.EncodeSnapshot(ctx, "key-1", s)
			require.NoError(t, err)
			request, businessHash, err := codec.BuildClaimedRequest(ctx, "platform-ppe", run, claim)
			require.NoError(t, err)
			body, err := BuildRequest(request)
			require.NoError(t, err)
			oldSnapshotHash := run.Snapshot.Hash
			in.Execution = private
			s, err = entity.NewHookRunSnapshot(in)
			require.NoError(t, err)
			run.Snapshot, err = codec.EncodeSnapshot(ctx, "key-1", s)
			require.NoError(t, err)
			require.NotEqual(t, oldSnapshotHash, run.Snapshot.Hash)
			decoded, err := codec.DecodeSnapshot(ctx, in.Key, in.ExecutionScope, run.Snapshot)
			require.NoError(t, err)
			assertPrivatePrecision(t, decoded)
			got, hash, err := codec.BuildClaimedRequest(ctx, "platform-ppe", run, claim)
			require.NoError(t, err)
			gotBody, err := BuildRequest(got)
			require.NoError(t, err)
			require.Equal(t, body, gotBody)
			require.Equal(t, businessHash, hash)
			require.NotContains(t, string(gotBody), "opaque_id")
		})
	}
}
