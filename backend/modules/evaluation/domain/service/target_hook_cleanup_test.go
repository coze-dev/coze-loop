// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	rpcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
)

func hookCleanupFixture(t *testing.T) (*EvalTargetServiceImpl, *repomocks.MockIEvalTargetRepo, *rpcmocks.MockISandboxSchedulerAdapter, *entity.EvalTargetRecord) {
	t.Helper()
	ctrl := gomock.NewController(t)
	r := repomocks.NewMockIEvalTargetRepo(ctrl)
	s := rpcmocks.NewMockISandboxSchedulerAdapter(ctrl)
	record := &entity.EvalTargetRecord{
		ID: 51, SpaceID: 9, TargetID: 21, TargetVersionID: 31, ExperimentRunID: 3,
		ItemID: 41, ItemVersionID: 42, TurnID: 0, TraceID: "trace", LogID: "log",
		Status:              gptr.Of(entity.EvalTargetRunStatusAsyncInvoking),
		EvalTargetInputData: &entity.EvalTargetInputData{Ext: map[string]string{"input": "preserved"}},
		EvalTargetOutputData: &entity.EvalTargetOutputData{
			OutputFields:       map[string]*entity.Content{"answer": {Text: gptr.Of("preserved")}},
			Ext:                map[string]string{"artifact_manifest_url": "tos://unchanged"},
			EvalTargetRunError: &entity.EvalTargetRunError{Code: 17, Message: "original"},
			EvalTargetUsage:    &entity.EvalTargetUsage{TotalTokens: 100},
			EvalTargetSteps:    []*entity.EvalTargetStep{{StepName: "original", EventType: "STARTED"}},
		},
		Ext: map[string]string{"ref": "preserved"},
	}
	return &EvalTargetServiceImpl{evalTargetRepo: r, sandboxSchedulerAdapter: s}, r, s, record
}

func callHookCleanup(t *testing.T, ctx context.Context, svc *EvalTargetServiceImpl, key entity.HookRunKey, records ...*entity.EvalTargetRecord) error {
	t.Helper()
	cleaner, ok := any(svc).(interface {
		CleanupHookTargetSandboxes(context.Context, entity.HookRunKey, []*entity.EvalTargetRecord) error
	})
	require.True(t, ok, "EvalTargetServiceImpl must expose optional synchronous Hook cleanup")
	return cleaner.CleanupHookTargetSandboxes(ctx, key, records)
}

func hookCleanupVersion(targetType entity.EvalTargetType) *entity.EvalTarget {
	return &entity.EvalTarget{ID: 21, SpaceID: 9, EvalTargetType: targetType,
		EvalTargetVersion: &entity.EvalTargetVersion{ID: 31, SpaceID: 9, TargetID: 21, EvalTargetType: targetType}}
}

func TestHookTargetCleanupPositiveAcknowledgementPreservesRecord(t *testing.T) {
	svc, r, s, record := hookCleanupFixture(t)
	before, err := json.Marshal(record)
	require.NoError(t, err)
	input, output, status := record.EvalTargetInputData, record.EvalTargetOutputData, record.Status
	r.EXPECT().GetEvalTargetVersion(gomock.Any(), int64(9), int64(31)).Return(hookCleanupVersion(entity.EvalTargetTypeSandboxAgent), nil)
	s.EXPECT().Destroy(gomock.Any(), &rpc.SandboxDestroyRequest{
		TaskID: "2", DestroyType: rpc.SandboxDestroyTypeExecute, ExecuteIDs: []string{"51"}, WorkspaceID: 9,
	}).Return(&rpc.SandboxDestroyResponse{AffectedCount: 1}, nil)
	require.NoError(t, callHookCleanup(t, context.Background(), svc, entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}, record))
	after, err := json.Marshal(record)
	require.NoError(t, err)
	require.Equal(t, string(before), string(after))
	require.Same(t, input, record.EvalTargetInputData)
	require.Same(t, output, record.EvalTargetOutputData)
	require.Same(t, status, record.Status)
}

func TestHookTargetCleanupAmbiguousAcknowledgementRequiresExactConfirmation(t *testing.T) {
	getFailure, destroyFailure := errors.New("Get unavailable"), errors.New("Destroy response lost")
	for _, tc := range []struct {
		name       string
		response   *rpc.SandboxDestroyResponse
		destroyErr error
		info       *rpc.SandboxExecuteInfo
		getErr     error
		wantError  bool
	}{
		{"zero_canceling", &rpc.SandboxDestroyResponse{}, nil, &rpc.SandboxExecuteInfo{ExecuteID: "51", TaskID: "2", Status: 3}, nil, false},
		{"zero_succeeded", &rpc.SandboxDestroyResponse{}, nil, &rpc.SandboxExecuteInfo{ExecuteID: "51", TaskID: "2", Status: rpc.SandboxExecuteStatusSucceeded}, nil, false},
		{"zero_failed", &rpc.SandboxDestroyResponse{}, nil, &rpc.SandboxExecuteInfo{ExecuteID: "51", TaskID: "2", Status: rpc.SandboxExecuteStatusFailed}, nil, false},
		{"zero_canceled", &rpc.SandboxDestroyResponse{}, nil, &rpc.SandboxExecuteInfo{ExecuteID: "51", TaskID: "2", Status: rpc.SandboxExecuteStatusCanceled}, nil, false},
		{"zero_finished", &rpc.SandboxDestroyResponse{}, nil, &rpc.SandboxExecuteInfo{ExecuteID: "51", TaskID: "2", Status: rpc.SandboxExecuteStatusFinished}, nil, false},
		{"zero_running", &rpc.SandboxDestroyResponse{}, nil, &rpc.SandboxExecuteInfo{ExecuteID: "51", TaskID: "2", Status: rpc.SandboxExecuteStatusRunning}, nil, true},
		{"zero_pending", &rpc.SandboxDestroyResponse{}, nil, &rpc.SandboxExecuteInfo{ExecuteID: "51", TaskID: "2", Status: rpc.SandboxExecuteStatusPending}, nil, true},
		{"zero_creating", &rpc.SandboxDestroyResponse{}, nil, &rpc.SandboxExecuteInfo{ExecuteID: "51", TaskID: "2", Status: rpc.SandboxExecuteStatusCreating}, nil, true},
		{"zero_unknown", &rpc.SandboxDestroyResponse{}, nil, &rpc.SandboxExecuteInfo{ExecuteID: "51", TaskID: "2", Status: 99}, nil, true},
		{"zero_missing", &rpc.SandboxDestroyResponse{}, nil, nil, nil, true},
		{"zero_get_error", &rpc.SandboxDestroyResponse{}, nil, nil, getFailure, true},
		{"zero_wrong_execute", &rpc.SandboxDestroyResponse{}, nil, &rpc.SandboxExecuteInfo{ExecuteID: "other", TaskID: "2", Status: 3}, nil, true},
		{"zero_wrong_task", &rpc.SandboxDestroyResponse{}, nil, &rpc.SandboxExecuteInfo{ExecuteID: "51", TaskID: "other", Status: 3}, nil, true},
		{"nil_response_running", nil, nil, &rpc.SandboxExecuteInfo{ExecuteID: "51", TaskID: "2", Status: rpc.SandboxExecuteStatusRunning}, nil, true},
		{"nil_response_canceling", nil, nil, &rpc.SandboxExecuteInfo{ExecuteID: "51", TaskID: "2", Status: 3}, nil, false},
		{"lost_response_canceling", nil, destroyFailure, &rpc.SandboxExecuteInfo{ExecuteID: "51", TaskID: "2", Status: 3}, nil, false},
		{"lost_response_running", nil, destroyFailure, &rpc.SandboxExecuteInfo{ExecuteID: "51", TaskID: "2", Status: rpc.SandboxExecuteStatusRunning}, nil, true},
		{"both_errors", nil, destroyFailure, nil, getFailure, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, r, s, record := hookCleanupFixture(t)
			before, err := json.Marshal(record)
			require.NoError(t, err)
			r.EXPECT().GetEvalTargetVersion(gomock.Any(), int64(9), int64(31)).Return(hookCleanupVersion(entity.EvalTargetTypeSandboxAgent), nil)
			gomock.InOrder(
				s.EXPECT().Destroy(gomock.Any(), &rpc.SandboxDestroyRequest{TaskID: "2", DestroyType: rpc.SandboxDestroyTypeExecute, ExecuteIDs: []string{"51"}, WorkspaceID: 9}).Return(tc.response, tc.destroyErr),
				s.EXPECT().Get(gomock.Any(), &rpc.SandboxGetRequest{ExecuteID: "51", WorkspaceID: 9}).Return(&rpc.SandboxGetResponse{ExecuteInfo: tc.info}, tc.getErr),
			)
			err = callHookCleanup(t, context.Background(), svc, entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}, record)
			if tc.wantError {
				require.Error(t, err)
				require.ErrorIs(t, err, errHookTargetCleanupUncertain)
				if tc.getErr != nil {
					require.ErrorIs(t, err, tc.getErr)
				}
				if tc.destroyErr != nil {
					require.ErrorIs(t, err, tc.destroyErr)
				}
			} else {
				require.NoError(t, err)
			}
			after, marshalErr := json.Marshal(record)
			require.NoError(t, marshalErr)
			require.Equal(t, string(before), string(after))
		})
	}
}

func TestHookTargetCleanupRejectsWrongRunBeforeAnyRPC(t *testing.T) {
	svc, _, _, record := hookCleanupFixture(t)
	wrongRun := *record
	wrongRun.ID, wrongRun.ExperimentRunID = 52, 4
	require.Error(t, callHookCleanup(t, context.Background(), svc, entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}, record, &wrongRun))
}

func TestHookTargetCleanupOrdinaryTargetIsNoop(t *testing.T) {
	svc, r, _, record := hookCleanupFixture(t)
	svc.sandboxSchedulerAdapter = nil
	r.EXPECT().GetEvalTargetVersion(gomock.Any(), int64(9), int64(31)).Return(hookCleanupVersion(entity.EvalTargetTypeLoopPrompt), nil)
	require.NoError(t, callHookCleanup(t, context.Background(), svc, entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}, record))
}

func TestHookTargetCleanupExactDualAndMacVMSourceMapping(t *testing.T) {
	for _, tc := range []struct {
		name string
		ext  map[string]string
		want []rpc.SandboxDestroyRequest
	}{
		{"dual", map[string]string{consts.OutputDataExtKeySandboxExecuteIDs: `["51-agent","51-orch","51-agent"]`}, []rpc.SandboxDestroyRequest{
			{TaskID: "2", DestroyType: rpc.SandboxDestroyTypeExecute, ExecuteIDs: []string{"51-agent"}, WorkspaceID: 9},
			{TaskID: "2", DestroyType: rpc.SandboxDestroyTypeExecute, ExecuteIDs: []string{"51-orch"}, WorkspaceID: 9},
		}},
		{"macvm_extra_union", map[string]string{consts.OutputDataExtKeySandboxExecuteIDs: `["51-orch","51-macvm"]`, entity.SandboxAgentExtKeyExtraExecuteID: "extra"}, []rpc.SandboxDestroyRequest{
			{TaskID: "2", DestroyType: rpc.SandboxDestroyTypeExecute, ExecuteIDs: []string{"51-orch"}, WorkspaceID: 9},
			{TaskID: "2-macvm", DestroyType: rpc.SandboxDestroyTypeExecute, ExecuteIDs: []string{"51-macvm"}, WorkspaceID: 9},
			{TaskID: "2", DestroyType: rpc.SandboxDestroyTypeExecute, ExecuteIDs: []string{"51"}, WorkspaceID: 9},
			{TaskID: "2", DestroyType: rpc.SandboxDestroyTypeExecute, ExecuteIDs: []string{"extra"}, WorkspaceID: 9},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, r, s, record := hookCleanupFixture(t)
			record.EvalTargetOutputData.Ext = tc.ext
			r.EXPECT().GetEvalTargetVersion(gomock.Any(), int64(9), int64(31)).Return(hookCleanupVersion(entity.EvalTargetTypeSandboxAgent), nil)
			for _, want := range tc.want {
				s.EXPECT().Destroy(gomock.Any(), &want).Return(&rpc.SandboxDestroyResponse{}, nil)
				s.EXPECT().Get(gomock.Any(), &rpc.SandboxGetRequest{ExecuteID: want.ExecuteIDs[0], WorkspaceID: 9}).Return(&rpc.SandboxGetResponse{ExecuteInfo: &rpc.SandboxExecuteInfo{ExecuteID: want.ExecuteIDs[0], TaskID: want.TaskID, Status: 3}}, nil)
			}
			require.NoError(t, callHookCleanup(t, context.Background(), svc, entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}, record))
		})
	}
}

func TestHookTargetCleanupPartialRetryAndReplay(t *testing.T) {
	svc, r, s, record := hookCleanupFixture(t)
	record.EvalTargetOutputData.Ext[consts.OutputDataExtKeySandboxExecuteIDs] = `["51-agent","51-orch"]`
	r.EXPECT().GetEvalTargetVersion(gomock.Any(), int64(9), int64(31)).Return(hookCleanupVersion(entity.EvalTargetTypeSandboxAgent), nil).Times(3)
	agent := &rpc.SandboxDestroyRequest{TaskID: "2", DestroyType: rpc.SandboxDestroyTypeExecute, ExecuteIDs: []string{"51-agent"}, WorkspaceID: 9}
	orch := &rpc.SandboxDestroyRequest{TaskID: "2", DestroyType: rpc.SandboxDestroyTypeExecute, ExecuteIDs: []string{"51-orch"}, WorkspaceID: 9}
	getAgent := &rpc.SandboxGetRequest{ExecuteID: "51-agent", WorkspaceID: 9}
	getOrch := &rpc.SandboxGetRequest{ExecuteID: "51-orch", WorkspaceID: 9}
	agentCanceled := &rpc.SandboxGetResponse{ExecuteInfo: &rpc.SandboxExecuteInfo{ExecuteID: "51-agent", TaskID: "2", Status: 3}}
	orchCanceled := &rpc.SandboxGetResponse{ExecuteInfo: &rpc.SandboxExecuteInfo{ExecuteID: "51-orch", TaskID: "2", Status: 3}}
	gomock.InOrder(
		s.EXPECT().Destroy(gomock.Any(), agent).Return(&rpc.SandboxDestroyResponse{AffectedCount: 1}, nil),
		s.EXPECT().Destroy(gomock.Any(), orch).Return(&rpc.SandboxDestroyResponse{}, nil),
		s.EXPECT().Get(gomock.Any(), getOrch).Return(&rpc.SandboxGetResponse{ExecuteInfo: &rpc.SandboxExecuteInfo{ExecuteID: "51-orch", TaskID: "2", Status: rpc.SandboxExecuteStatusRunning}}, nil),
		s.EXPECT().Destroy(gomock.Any(), agent).Return(&rpc.SandboxDestroyResponse{}, nil),
		s.EXPECT().Get(gomock.Any(), getAgent).Return(agentCanceled, nil),
		s.EXPECT().Destroy(gomock.Any(), orch).Return(&rpc.SandboxDestroyResponse{AffectedCount: 1}, nil),
		s.EXPECT().Destroy(gomock.Any(), agent).Return(&rpc.SandboxDestroyResponse{}, nil),
		s.EXPECT().Get(gomock.Any(), getAgent).Return(agentCanceled, nil),
		s.EXPECT().Destroy(gomock.Any(), orch).Return(&rpc.SandboxDestroyResponse{}, nil),
		s.EXPECT().Get(gomock.Any(), getOrch).Return(orchCanceled, nil),
	)
	key := entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}
	require.Error(t, callHookCleanup(t, context.Background(), svc, key, record))
	require.NoError(t, callHookCleanup(t, context.Background(), svc, key, record))
	require.NoError(t, callHookCleanup(t, context.Background(), svc, key, record))
}

func TestHookTargetCleanupInvalidInputDoesNotCallDependencies(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*entity.HookRunKey, **entity.EvalTargetRecord)
	}{
		{"workspace", func(k *entity.HookRunKey, _ **entity.EvalTargetRecord) { k.WorkspaceID = 0 }},
		{"experiment", func(k *entity.HookRunKey, _ **entity.EvalTargetRecord) { k.ExperimentID = 0 }},
		{"run", func(k *entity.HookRunKey, _ **entity.EvalTargetRecord) { k.RunID = 0 }},
		{"nil_record", func(_ *entity.HookRunKey, r **entity.EvalTargetRecord) { *r = nil }},
		{"record_id", func(_ *entity.HookRunKey, r **entity.EvalTargetRecord) { (*r).ID = 0 }},
		{"source_workspace", func(_ *entity.HookRunKey, r **entity.EvalTargetRecord) { (*r).SpaceID = 0 }},
		{"target", func(_ *entity.HookRunKey, r **entity.EvalTargetRecord) { (*r).TargetID = 0 }},
		{"version", func(_ *entity.HookRunKey, r **entity.EvalTargetRecord) { (*r).TargetVersionID = 0 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, _, record := hookCleanupFixture(t)
			key := entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}
			tc.mutate(&key, &record)
			require.Error(t, callHookCleanup(t, context.Background(), svc, key, record))
		})
	}
}

func TestHookTargetCleanupVersionFailureRemainsRecoverable(t *testing.T) {
	repoErr := errors.New("version lookup failed")
	for _, tc := range []struct {
		name    string
		version *entity.EvalTarget
		err     error
	}{
		{"repository_error", nil, repoErr},
		{"missing_version", nil, nil},
		{"incomplete_version", &entity.EvalTarget{ID: 21, SpaceID: 9}, nil},
		{"wrong_version", &entity.EvalTarget{ID: 21, SpaceID: 9, EvalTargetType: entity.EvalTargetTypeSandboxAgent, EvalTargetVersion: &entity.EvalTargetVersion{ID: 32, SpaceID: 9, TargetID: 21}}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, r, _, record := hookCleanupFixture(t)
			r.EXPECT().GetEvalTargetVersion(gomock.Any(), int64(9), int64(31)).Return(tc.version, tc.err)
			err := callHookCleanup(t, context.Background(), svc, entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}, record)
			require.Error(t, err)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
			}
		})
	}
}

func TestHookTargetCleanupEmptyAndCanceled(t *testing.T) {
	svc, _, _, record := hookCleanupFixture(t)
	key := entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}
	require.NoError(t, callHookCleanup(t, context.Background(), svc, key))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.ErrorIs(t, callHookCleanup(t, ctx, svc, key, record), context.Canceled)
}

func TestHookTargetCleanupMissingDependencies(t *testing.T) {
	for _, typedNil := range []bool{false, true} {
		t.Run(map[bool]string{false: "nil", true: "typed_nil"}[typedNil], func(t *testing.T) {
			svc, r, _, record := hookCleanupFixture(t)
			key := entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}
			svc.evalTargetRepo = nil
			if typedNil {
				svc.evalTargetRepo = (*repomocks.MockIEvalTargetRepo)(nil)
			}
			require.Error(t, callHookCleanup(t, context.Background(), svc, key, record))
			svc.evalTargetRepo = r
			svc.sandboxSchedulerAdapter = nil
			if typedNil {
				svc.sandboxSchedulerAdapter = (*rpcmocks.MockISandboxSchedulerAdapter)(nil)
			}
			r.EXPECT().GetEvalTargetVersion(gomock.Any(), int64(9), int64(31)).Return(hookCleanupVersion(entity.EvalTargetTypeSandboxAgent), nil)
			require.Error(t, callHookCleanup(t, context.Background(), svc, key, record))
		})
	}
}

func TestHookTargetCleanupNilGetResponseIsUncertain(t *testing.T) {
	svc, r, s, record := hookCleanupFixture(t)
	r.EXPECT().GetEvalTargetVersion(gomock.Any(), int64(9), int64(31)).Return(hookCleanupVersion(entity.EvalTargetTypeSandboxAgent), nil)
	s.EXPECT().Destroy(gomock.Any(), &rpc.SandboxDestroyRequest{TaskID: "2", DestroyType: rpc.SandboxDestroyTypeExecute, ExecuteIDs: []string{"51"}, WorkspaceID: 9}).Return(&rpc.SandboxDestroyResponse{}, nil)
	s.EXPECT().Get(gomock.Any(), &rpc.SandboxGetRequest{ExecuteID: "51", WorkspaceID: 9}).Return(nil, nil)
	require.ErrorIs(t, callHookCleanup(t, context.Background(), svc, entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}, record), errHookTargetCleanupUncertain)
}

func TestHookTargetCleanupKeepsCallerDeadlineAndReturnsRPCError(t *testing.T) {
	svc, r, s, record := hookCleanupFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	deadline, _ := ctx.Deadline()
	r.EXPECT().GetEvalTargetVersion(ctx, int64(9), int64(31)).Return(hookCleanupVersion(entity.EvalTargetTypeSandboxAgent), nil)
	finished := false
	s.EXPECT().Destroy(ctx, &rpc.SandboxDestroyRequest{TaskID: "2", DestroyType: rpc.SandboxDestroyTypeExecute, ExecuteIDs: []string{"51"}, WorkspaceID: 9}).
		DoAndReturn(func(callCtx context.Context, _ *rpc.SandboxDestroyRequest) (*rpc.SandboxDestroyResponse, error) {
			gotDeadline, ok := callCtx.Deadline()
			require.True(t, ok)
			require.Equal(t, deadline, gotDeadline)
			cancel()
			finished = true
			return nil, callCtx.Err()
		})
	s.EXPECT().Get(ctx, &rpc.SandboxGetRequest{ExecuteID: "51", WorkspaceID: 9}).Return(nil, context.Canceled)
	err := callHookCleanup(t, ctx, svc, entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}, record)
	require.ErrorIs(t, err, context.Canceled)
	require.ErrorIs(t, err, errHookTargetCleanupUncertain)
	require.True(t, finished)
}
