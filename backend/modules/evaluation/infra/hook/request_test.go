// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hook

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

const requestGolden = `{"schema_version":"1.0","event_type":"experiment.run.before","operation_id":"op","idempotency_key":"key","delivery_id":"delivery","attempt":1,"occurred_at":"2026-01-02T03:04:05Z","context":{"workspace_id":"1","experiment_id":"2","run_id":"3","run_mode":"submit","initiator":{"user_id":"user","identity_type":"fornax_user"},"experiment":{"name":"experiment","type":"offline"},"eval_sets":[]},"parameters":{}}`

func validRequest() *spi.InvokeExperimentHookRequest {
	return &spi.InvokeExperimentHookRequest{
		SchemaVersion: gptr.Of("1.0"), EventType: gptr.Of("experiment.run.before"),
		OperationID: gptr.Of("op"), IdempotencyKey: gptr.Of("key"), DeliveryID: gptr.Of("delivery"),
		Attempt: gptr.Of(int32(1)), OccurredAt: gptr.Of("2026-01-02T03:04:05Z"),
		Context: &spi.HookRunContext{
			WorkspaceID: gptr.Of("1"), ExperimentID: gptr.Of("2"), RunID: gptr.Of("3"), RunMode: gptr.Of("submit"),
			Initiator:  &spi.HookInitiator{UserID: gptr.Of("user"), IdentityType: gptr.Of("fornax_user")},
			Experiment: &spi.HookExperimentRef{Name: gptr.Of("experiment"), Type: gptr.Of("offline")},
			EvalSets:   []*spi.HookEvalSetRef{},
		},
	}
}

func TestRequestBusinessHashHasIndependentGoldenContent(t *testing.T) {
	r := validRequest()
	stable := `{"context":{"workspace_id":"1","experiment_id":"2","run_id":"3","run_mode":"submit","initiator":{"user_id":"user","identity_type":"fornax_user"},"experiment":{"name":"experiment","type":"offline"},"eval_sets":[]},"event_type":"experiment.run.before","occurred_at":"2026-01-02T03:04:05Z","parameters":{},"schema_version":"1.0"}`
	sum := sha256.Sum256([]byte(stable))
	want := hex.EncodeToString(sum[:])
	got, err := RequestBusinessHash(r)
	require.NoError(t, err)
	require.Equal(t, want, got)
	r.OperationID = gptr.Of("other-operation")
	r.IdempotencyKey = gptr.Of("other-key")
	r.DeliveryID = gptr.Of("delivery-2")
	r.Attempt = gptr.Of(int32(2))
	got, err = RequestBusinessHash(r)
	require.NoError(t, err)
	require.Equal(t, want, got)
	for _, change := range []func(*spi.InvokeExperimentHookRequest){
		func(r *spi.InvokeExperimentHookRequest) { r.OccurredAt = gptr.Of("2026-01-02T03:04:06Z") },
		func(r *spi.InvokeExperimentHookRequest) { r.Context.Initiator.UserID = gptr.Of("another-user") },
		func(r *spi.InvokeExperimentHookRequest) { r.Parameters = gptr.Of(`{"x":false}`) },
		func(r *spi.InvokeExperimentHookRequest) {
			r.EventType = gptr.Of(spi.HookEventTypeAfter)
			r.Context.TerminalStatus = gptr.Of(spi.HookTerminalStatusSuccess)
		},
	} {
		q := validRequest()
		change(q)
		hash, err := RequestBusinessHash(q)
		require.NoError(t, err)
		require.NotEqual(t, want, hash)
	}
	_, err = RequestBusinessHash(nil)
	require.Error(t, err)
}

func claimedSnapshotRun(t *testing.T, phase entity.HookPhase) (*StorageCodec, *entity.HookStoredRun, *entity.HookAttemptClaim) {
	t.Helper()
	codec := NewStorageCodec(NewDKMSProtector(newSnapshotDKMS()))
	in := codecSnapshot(t).Input()
	in.Config.After = &entity.HookConfig{Enabled: gptr.Of(true), InvokeHTTPInfo: &entity.HookHTTPInfo{URL: gptr.Of("https://after.example/run")}, ParametersJSON: gptr.Of(`{"phase":"after"}`)}
	s, err := entity.NewHookRunSnapshot(in)
	require.NoError(t, err)
	p, err := codec.EncodeSnapshot(context.Background(), "key-1", s)
	require.NoError(t, err)
	occurred := time.Date(2026, 9, 23, 1, 4, 5, 0, time.UTC)
	run := &entity.HookStoredRun{State: entity.HookRunState{Key: in.Key, Status: entity.ExptStatus_Processing, Gate: entity.HookGateWaiting, Finalize: entity.HookFinalizeNone, Before: entity.HookOperation{ID: "hook_before", Status: entity.HookOperationRunning, Activated: true, Attempt: 1, Generation: 1, LeaseUntil: occurred.Add(30 * time.Second), AttemptDeadline: occurred.Add(180 * time.Second), Deadline: occurred.Add(365 * time.Second)}, After: entity.HookOperation{ID: "hook_after", Status: entity.HookOperationPending}}, Snapshot: p, CreatedBy: "user", Mode: entity.ExptRunMode(1), PlanReady: true,
		Operations: []entity.HookStoredOperation{{HookOperationSeed: entity.HookOperationSeed{ID: 1, OperationID: "hook_before", IdempotencyKey: "before-key"}, Phase: entity.HookPhaseBefore, Version: 1, ActivatedAt: &occurred, OccurredAt: &occurred}, {HookOperationSeed: entity.HookOperationSeed{ID: 2, OperationID: "hook_after", IdempotencyKey: "after-key"}, Phase: entity.HookPhaseAfter}}}
	claim := &entity.HookAttemptClaim{HookAttemptIdentity: entity.HookAttemptIdentity{Token: entity.HookClaimToken{Run: in.Key, OperationID: "hook_before", Phase: phase, Attempt: 1, Generation: 1}, Owner: "worker", DeliveryID: "delivery-1"}, Version: 1, IdempotencyKey: "before-key", StartedAt: occurred, LeaseUntil: run.State.Before.LeaseUntil, AttemptDeadline: run.State.Before.AttemptDeadline, Deadline: run.State.Before.Deadline}
	if phase == entity.HookPhaseAfter {
		run.State.Status = entity.ExptStatus_Terminated
		run.State.Gate = entity.HookGateClosed
		run.State.Finalize = entity.HookFinalizeCommitted
		run.State.Intent = entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated, Reason: "HOOK_BEFORE_FAILED"}
		run.TerminalAt = &occurred
		run.State.After = run.State.Before
		run.State.After.ID = "hook_after"
		run.State.Before.Status = entity.HookOperationFailed
		run.State.Before.Activated = false
		run.Operations[1].Version = 1
		run.Operations[1].ActivatedAt = &occurred
		run.Operations[1].OccurredAt = &occurred
		claim.Token.OperationID = "hook_after"
		claim.IdempotencyKey = "after-key"
	}
	return codec, run, claim
}

func TestSnapshotRequestUsesFrozenRunAndRetryStableHash(t *testing.T) {
	codec, run, claim := claimedSnapshotRun(t, entity.HookPhaseBefore)
	first, hash, err := codec.BuildClaimedRequest(context.Background(), "platform-ppe", run, claim)
	require.NoError(t, err)
	require.Equal(t, "hook_before", first.GetOperationID())
	require.Equal(t, "before-key", first.GetIdempotencyKey())
	require.Equal(t, "2026-09-23T01:04:05Z", first.GetOccurredAt())
	require.Equal(t, "user", first.Context.Initiator.GetUserID())
	body, err := BuildRequest(first)
	require.NoError(t, err)
	require.Contains(t, string(body), `"eval_sets":[]`)
	require.Contains(t, string(body), `"large":9007199254740993`)
	claim.Token.Attempt = 2
	claim.Token.Generation = 2
	claim.DeliveryID = "delivery-2"
	claim.Version = 2
	run.State.Before.Attempt = 2
	run.State.Before.Generation = 2
	run.Operations[0].Version = 2
	second, hash2, err := codec.BuildClaimedRequest(context.Background(), "platform-ppe", run, claim)
	require.NoError(t, err)
	require.Equal(t, hash, hash2)
	require.Equal(t, int32(2), second.GetAttempt())
	require.Equal(t, "delivery-2", second.GetDeliveryID())
	*first.Context.Initiator.UserID = "mutated-output"
	*first.Parameters = "{}"
	require.Equal(t, "user", second.Context.Initiator.GetUserID())
	require.Contains(t, second.GetParameters(), "9007199254740993")
	mutated := *second
	mutated.Parameters = gptr.Of(`{"different":true}`)
	different, err := RequestBusinessHash(&mutated)
	require.NoError(t, err)
	require.NotEqual(t, hash, different)
}

func TestSnapshotRequestAfterUsesOriginalRunFinalIntent(t *testing.T) {
	for _, tc := range []struct {
		status entity.ExptStatus
		want   string
	}{{11, "success"}, {12, "failed"}, {13, "terminated"}, {14, "system_terminated"}} {
		codec, run, claim := claimedSnapshotRun(t, entity.HookPhaseAfter)
		run.State.Status = tc.status
		run.State.Intent.Status = tc.status
		r, _, err := codec.BuildClaimedRequest(context.Background(), "platform-ppe", run, claim)
		require.NoError(t, err)
		require.Equal(t, tc.want, r.Context.GetTerminalStatus())
		require.Equal(t, "HOOK_BEFORE_FAILED", r.Context.GetTerminalReason())
		require.Equal(t, "3", r.Context.GetRunID())
		require.Equal(t, `{"phase":"after"}`, r.GetParameters())
		run.State.Intent.Reason = ""
		r, _, err = codec.BuildClaimedRequest(context.Background(), "platform-ppe", run, claim)
		require.NoError(t, err)
		require.Nil(t, r.Context.TerminalReason)
	}
}

func TestSnapshotRequestRejectsMixedOrStaleClaim(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*entity.HookStoredRun, *entity.HookAttemptClaim)
	}{
		{"run", func(_ *entity.HookStoredRun, c *entity.HookAttemptClaim) { c.Token.Run.RunID++ }},
		{"author", func(r *entity.HookStoredRun, _ *entity.HookAttemptClaim) { r.CreatedBy = "other" }},
		{"mode", func(r *entity.HookStoredRun, _ *entity.HookAttemptClaim) { r.Mode = 2 }},
		{"operation", func(_ *entity.HookStoredRun, c *entity.HookAttemptClaim) { c.Token.OperationID = "another" }},
		{"idempotency", func(_ *entity.HookStoredRun, c *entity.HookAttemptClaim) { c.IdempotencyKey = "another" }},
		{"phase", func(_ *entity.HookStoredRun, c *entity.HookAttemptClaim) { c.Token.Phase = entity.HookPhaseAfter }},
		{"version", func(_ *entity.HookStoredRun, c *entity.HookAttemptClaim) { c.Version++ }},
		{"attempt", func(_ *entity.HookStoredRun, c *entity.HookAttemptClaim) { c.Token.Attempt++ }},
		{"generation", func(_ *entity.HookStoredRun, c *entity.HookAttemptClaim) { c.Token.Generation++ }},
		{"delivery", func(_ *entity.HookStoredRun, c *entity.HookAttemptClaim) { c.DeliveryID = "" }},
		{"occurred", func(r *entity.HookStoredRun, _ *entity.HookAttemptClaim) { r.Operations[0].OccurredAt = nil }},
		{"not activated", func(r *entity.HookStoredRun, _ *entity.HookAttemptClaim) { r.State.Before.Activated = false }},
		{"plan preparing", func(r *entity.HookStoredRun, _ *entity.HookAttemptClaim) { r.PlanReady = false }},
		{"missing phase", func(r *entity.HookStoredRun, _ *entity.HookAttemptClaim) { r.Operations = r.Operations[:1] }},
		{"duplicate phase", func(r *entity.HookStoredRun, _ *entity.HookAttemptClaim) {
			r.Operations = append(r.Operations, r.Operations[0])
		}},
		{"cancelled before", func(r *entity.HookStoredRun, _ *entity.HookAttemptClaim) { r.State.Gate = entity.HookGateClosed }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			codec, r, c := claimedSnapshotRun(t, entity.HookPhaseBefore)
			tc.change(r, c)
			got, hash, err := codec.BuildClaimedRequest(context.Background(), "platform-ppe", r, c)
			require.Error(t, err)
			require.Nil(t, got)
			require.Empty(t, hash)
		})
	}
	codec, r, c := claimedSnapshotRun(t, entity.HookPhaseAfter)
	r.State.Finalize = entity.HookFinalizePending
	_, _, err := codec.BuildClaimedRequest(context.Background(), "platform-ppe", r, c)
	require.Error(t, err)
}

func TestRequestGoldenAndLexicalParameters(t *testing.T) {
	r := validRequest()
	body, err := BuildRequest(r)
	require.NoError(t, err)
	require.JSONEq(t, requestGolden, string(body))
	require.Nil(t, r.Parameters)
	r.Parameters = gptr.Of(` {"number":9007199254740993,"float":1.2300e+20,"text":"9223372036854775807","v":[true,null,{"中":"文"}]} `)
	r.Context.WorkspaceID = gptr.Of("9223372036854775807")
	before, err := json.Marshal(r)
	require.NoError(t, err)
	body, err = BuildRequest(r)
	require.NoError(t, err)
	require.Contains(t, string(body), `"number":9007199254740993`)
	require.Contains(t, string(body), `"float":1.2300e+20`)
	var payload map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(body, &payload))
	require.Len(t, payload, 9)
	require.Equal(t, byte('{'), payload["parameters"][0])
	after, err := json.Marshal(r)
	require.NoError(t, err)
	require.Equal(t, before, after)
}

func TestRequestValidation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*spi.InvokeExperimentHookRequest)
	}{
		{"schema missing", func(r *spi.InvokeExperimentHookRequest) { r.SchemaVersion = nil }},
		{"schema", func(r *spi.InvokeExperimentHookRequest) { r.SchemaVersion = gptr.Of("2.0") }},
		{"event", func(r *spi.InvokeExperimentHookRequest) { r.EventType = gptr.Of("other") }},
		{"operation", func(r *spi.InvokeExperimentHookRequest) { r.OperationID = gptr.Of(strings.Repeat("x", 129)) }},
		{"idempotency", func(r *spi.InvokeExperimentHookRequest) { r.IdempotencyKey = gptr.Of(strings.Repeat("x", 257)) }},
		{"delivery", func(r *spi.InvokeExperimentHookRequest) { r.DeliveryID = gptr.Of("") }},
		{"attempt zero", func(r *spi.InvokeExperimentHookRequest) { r.Attempt = gptr.Of(int32(0)) }},
		{"attempt excess", func(r *spi.InvokeExperimentHookRequest) { r.Attempt = gptr.Of(int32(12)) }},
		{"no timezone", func(r *spi.InvokeExperimentHookRequest) { r.OccurredAt = gptr.Of("2026-01-02T03:04:05") }},
		{"bad timezone", func(r *spi.InvokeExperimentHookRequest) { r.OccurredAt = gptr.Of("2026-01-02T03:04:05+24:00") }},
		{"context", func(r *spi.InvokeExperimentHookRequest) { r.Context = nil }},
		{"workspace zero", func(r *spi.InvokeExperimentHookRequest) { r.Context.WorkspaceID = gptr.Of("0") }},
		{"experiment overflow", func(r *spi.InvokeExperimentHookRequest) { r.Context.ExperimentID = gptr.Of("9223372036854775808") }},
		{"run plus", func(r *spi.InvokeExperimentHookRequest) { r.Context.RunID = gptr.Of("+1") }},
		{"mode", func(r *spi.InvokeExperimentHookRequest) { r.Context.RunMode = gptr.Of("offline") }},
		{"initiator", func(r *spi.InvokeExperimentHookRequest) { r.Context.Initiator = nil }},
		{"identity", func(r *spi.InvokeExperimentHookRequest) { r.Context.Initiator.IdentityType = gptr.Of("session") }},
		{"user", func(r *spi.InvokeExperimentHookRequest) {
			r.Context.Initiator.UserID = gptr.Of(strings.Repeat("x", 129))
		}},
		{"email", func(r *spi.InvokeExperimentHookRequest) {
			r.Context.Initiator.Email = gptr.Of(strings.Repeat("x", 321))
		}},
		{"user name", func(r *spi.InvokeExperimentHookRequest) { r.Context.Initiator.Name = gptr.Of(strings.Repeat("x", 257)) }},
		{"experiment nil", func(r *spi.InvokeExperimentHookRequest) { r.Context.Experiment = nil }},
		{"experiment name bytes", func(r *spi.InvokeExperimentHookRequest) {
			r.Context.Experiment.Name = gptr.Of(strings.Repeat("中", 171))
		}},
		{"experiment type", func(r *spi.InvokeExperimentHookRequest) { r.Context.Experiment.Type = gptr.Of("other") }},
		{"eval sets omitted", func(r *spi.InvokeExperimentHookRequest) { r.Context.EvalSets = nil }},
		{"eval set nil", func(r *spi.InvokeExperimentHookRequest) { r.Context.EvalSets = []*spi.HookEvalSetRef{nil} }},
		{"eval set id", func(r *spi.InvokeExperimentHookRequest) {
			r.Context.EvalSets = []*spi.HookEvalSetRef{{WorkspaceID: gptr.Of("1"), ID: gptr.Of("-1")}}
		}},
		{"eval version zero", func(r *spi.InvokeExperimentHookRequest) {
			r.Context.EvalSets = []*spi.HookEvalSetRef{{WorkspaceID: gptr.Of("1"), ID: gptr.Of("2"), VersionID: gptr.Of("0")}}
		}},
		{"target type missing", func(r *spi.InvokeExperimentHookRequest) { r.Context.Target = &spi.HookTargetRef{ID: gptr.Of("1")} }},
		{"target version zero", func(r *spi.InvokeExperimentHookRequest) {
			r.Context.Target = &spi.HookTargetRef{ID: gptr.Of("1"), Type: gptr.Of("any"), VersionID: gptr.Of("0")}
		}},
		{"before terminal", func(r *spi.InvokeExperimentHookRequest) { r.Context.TerminalStatus = gptr.Of("success") }},
		{"before reason", func(r *spi.InvokeExperimentHookRequest) { r.Context.TerminalReason = gptr.Of("") }},
		{"after terminal missing", func(r *spi.InvokeExperimentHookRequest) { r.EventType = gptr.Of("experiment.run.after") }},
		{"after terminal unknown", func(r *spi.InvokeExperimentHookRequest) {
			r.EventType = gptr.Of("experiment.run.after")
			r.Context.TerminalStatus = gptr.Of("canceled")
		}},
		{"parameters null", func(r *spi.InvokeExperimentHookRequest) { r.Parameters = gptr.Of("null") }},
		{"parameters duplicate", func(r *spi.InvokeExperimentHookRequest) { r.Parameters = gptr.Of(`{"a":1,"\u0061":2}`) }},
		{"invalid UTF8", func(r *spi.InvokeExperimentHookRequest) { r.Context.Initiator.Name = gptr.Of("\xff") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := validRequest()
			tc.change(r)
			before, _ := json.Marshal(r)
			body, err := BuildRequest(r)
			require.Error(t, err)
			require.Nil(t, body)
			after, _ := json.Marshal(r)
			require.Equal(t, before, after)
		})
	}
	_, err := BuildRequest(nil)
	require.Error(t, err)
	for _, mode := range []string{"submit", "fail_retry", "append", "retry_all", "retry_items", "trial_run"} {
		r := validRequest()
		r.Context.RunMode = &mode
		_, err := BuildRequest(r)
		require.NoError(t, err)
	}
	for _, terminal := range []string{"success", "failed", "terminated", "system_terminated"} {
		r := validRequest()
		r.EventType = gptr.Of("experiment.run.after")
		r.Context.TerminalStatus = &terminal
		r.Context.Experiment.Type = gptr.Of("online")
		_, err := BuildRequest(r)
		require.NoError(t, err)
	}
}

func TestRequestSizeBoundary(t *testing.T) {
	baseLength := len(requestGolden) + len(`,"target":{"id":"1","type":""}`)
	for _, size := range []int{65536, 65537} {
		r := validRequest()
		r.Context.Target = &spi.HookTargetRef{ID: gptr.Of("1"), Type: gptr.Of(strings.Repeat("x", size-baseLength))}
		body, err := BuildRequest(r)
		if size == 65536 {
			require.NoError(t, err)
			require.Len(t, body, 65536)
		} else {
			require.Error(t, err)
			require.Nil(t, body)
		}
	}
}
