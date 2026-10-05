// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/bytedance/gg/gptr"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func finalizationMessageReadback(t *testing.T, f *finalizationManagerFixture, display, reason string) {
	t.Helper()
	var log model.ExptRunLog
	require.NoError(t, f.sql.First(&log, f.key.RunID).Error)
	var expt model.Experiment
	require.NoError(t, f.sql.First(&expt, f.expt).Error)
	require.Equal(t, display, string(gptr.Indirect(log.StatusMessage)))
	require.Equal(t, display, string(gptr.Indirect(expt.StatusMessage)))
	state := finalizationRead(t, f)
	require.Equal(t, reason, state.State.Intent.Reason)
	require.True(t, utf8.ValidString(state.State.Intent.Reason))
	require.LessOrEqual(t, utf8.RuneCountInString(reason), 128)
	require.Equal(t, entity.ExptStatus_Failed, state.State.Status)
	require.True(t, state.State.After.Activated)
}

type finalizationMessageBeginRace struct {
	repo.IHookRepo
	message string
}

func (r *finalizationMessageBeginRace) BeginFinalize(ctx context.Context, in entity.HookFinalizeInput) (entity.HookStoreResult, error) {
	winner := in
	winner.DisplayMessage = &r.message
	if _, err := r.IHookRepo.BeginFinalize(ctx, winner); err != nil {
		return entity.HookStoreResult{}, err
	}
	return r.IHookRepo.BeginFinalize(ctx, in)
}

func TestHookFinalizationMessageConcurrentBeginKeepsPersistedDisplay(t *testing.T) {
	f := newFinalizationManagerFixture(t)
	ctx := context.Background()
	display := strings.Repeat("winner", 30)
	f.deps.Runs = &finalizationMessageBeginRace{IHookRepo: f.repo, message: display}
	finalizationRecreate(t, f)
	require.NoError(t, f.manager.CompleteRun(ctx, f.expt, f.key.RunID, f.space, nil))
	// A different caller freezes the full message after GetRun, before this caller's Begin.
	require.NoError(t, f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil, entity.WithStatus(entity.ExptStatus_Failed)))
	finalizationMessageReadback(t, f, display, "")
	require.Equal(t, 1, f.notifications)
}

func TestHookFinalizationMessageNoHookByteBoundaryUnchanged(t *testing.T) {
	for _, tc := range []struct{ name, input, want string }{
		{"129", strings.Repeat("a", 129), strings.Repeat("a", 129)},
		{"200", strings.Repeat("a", 200), strings.Repeat("a", 200)},
		{"cut", strings.Repeat("界", 67), strings.Repeat("界", 66) + "\xe7\x95"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			m := newTestExptManager(ctrl)
			ctx := context.Background()
			runs := repomocks.NewMockIExptRunLogRepo(ctrl)
			turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
			locker := lockmocks.NewMockILocker(ctrl)
			m.runLogRepo = runs
			m.turnResultRepo = turns
			m.mutex = locker
			runs.EXPECT().Get(ctx, int64(2), int64(3)).Return(&entity.ExptRunLog{ID: 3, ExptID: 2, ExptRunID: 3, SpaceID: 1, Status: 3}, nil)
			turns.EXPECT().ListTurnResult(ctx, int64(1), int64(2), nil, gomock.Any(), false).Return([]*entity.ExptTurnResult{{Status: 1}}, int64(1), nil)
			locker.EXPECT().UnlockForce(ctx, "expt_run_mutex_lock:2").Return(true, nil)
			runs.EXPECT().Save(ctx, gomock.Any()).DoAndReturn(func(_ context.Context, log *entity.ExptRunLog) error {
				require.Equal(t, []byte(tc.want), log.StatusMessage)
				return nil
			})
			require.NoError(t, m.CompleteRun(ctx, 2, 3, 1, nil, entity.WithStatusMessage(tc.input)))
		})
	}
}

func TestHookFinalizationMessageTwoCallBoundaries(t *testing.T) {
	cases := []struct{ name, input, display string }{
		{"ascii128", strings.Repeat("x", 128), strings.Repeat("x", 128)},
		{"ascii129", strings.Repeat("x", 129), strings.Repeat("x", 129)},
		{"ascii200", strings.Repeat("x", 200), strings.Repeat("x", 200)},
		{"unicode199", strings.Repeat("界", 66) + "a", strings.Repeat("界", 66) + "a"},
		{"unicode_cut", strings.Repeat("界", 67), strings.Repeat("界", 66)},
		{"emoji200", strings.Repeat("a", 196) + "🙂", strings.Repeat("a", 196) + "🙂"},
		{"emoji_cut", strings.Repeat("a", 199) + "🙂", strings.Repeat("a", 199)},
		{"emoji_cut_two_bytes", strings.Repeat("a", 198) + "🙂", strings.Repeat("a", 198)},
		{"emoji_cut_three_bytes", strings.Repeat("a", 197) + "🙂", strings.Repeat("a", 197)},
		{"unicode_mixed", strings.Repeat("a", 126) + "界界tail", strings.Repeat("a", 126) + "界界tail"},
	}
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeTrialRun} {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%d/%s", mode, tc.name), func(t *testing.T) {
				f := newFinalizationManagerFixture(t)
				ctx := context.Background()
				require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", f.key.RunID).UpdateColumn("mode", int32(mode)).Error)
				require.NoError(t, f.manager.CompleteRun(ctx, f.expt, f.key.RunID, f.space, nil))
				require.Equal(t, entity.HookFinalizeNone, finalizationRead(t, f).State.Finalize)
				require.NoError(t, f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil, entity.WithStatus(entity.ExptStatus_Failed), entity.WithStatusMessage(tc.input)))
				finalizationMessageReadback(t, f, tc.display, "")
				state := finalizationRead(t, f)
				require.NoError(t, f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil, entity.WithStatus(entity.ExptStatus_Failed), entity.WithStatusMessage(tc.input)))
				require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
				require.Equal(t, state, finalizationRead(t, f))
				require.Equal(t, 1, f.notifications)
			})
		}
	}
}

func TestHookFinalizationMessageSuccessorLatestUntouched(t *testing.T) {
	f := newFinalizationManagerFixture(t)
	ctx := context.Background()
	display := strings.Repeat("d", 200)
	fault := &finalizationRunFault{IHookRepo: f.repo, failCommit: true}
	f.deps.Runs = fault
	finalizationRecreate(t, f)
	require.NoError(t, f.manager.CompleteRun(ctx, f.expt, f.key.RunID, f.space, nil))
	require.Error(t, f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil, entity.WithStatusMessage(display)))
	next := finalizationTestIDs.Add(1)
	_, err := f.repo.CreateRunWithHooks(ctx, entity.HookCreateRunInput{
		Key: entity.HookRunKey{WorkspaceID: f.space, ExperimentID: f.expt, RunID: next}, ExpectedLatestRunID: f.key.RunID,
		RunLog:   &entity.ExptRunLog{ID: next, SpaceID: f.space, ExptID: f.expt, ExptRunID: next, Mode: 1, Status: 3, CreatedBy: "user"},
		Snapshot: entity.HookProtectedSnapshot{Cipher: []byte{1}, KeyID: "key", Hash: strings.Repeat("b", 64), ExecutionScope: "local"},
		After:    &entity.HookOperationSeed{ID: finalizationTestIDs.Add(1), OperationID: fmt.Sprint(next), IdempotencyKey: fmt.Sprint(next)},
	})
	require.NoError(t, err)
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumn("status_message", []byte("successor-message")).Error)
	fault.failCommit = false
	finalizationRecreate(t, f)
	require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
	var log model.ExptRunLog
	require.NoError(t, f.sql.First(&log, f.key.RunID).Error)
	require.Equal(t, display, string(gptr.Indirect(log.StatusMessage)))
	var expt model.Experiment
	require.NoError(t, f.sql.First(&expt, f.expt).Error)
	require.Equal(t, next, expt.LatestRunID)
	require.Equal(t, "successor-message", string(gptr.Indirect(expt.StatusMessage)))
	require.Equal(t, int32(3), expt.Status)
	state := finalizationRead(t, f)
	require.Empty(t, state.State.Intent.Reason)
	require.True(t, state.State.After.Activated)
	require.Zero(t, f.notifications)
}

func TestHookFinalizationMessageExplicitCodePreserved(t *testing.T) {
	const code = "HOOK_BEFORE_FAILED"
	for _, pending := range []bool{false, true} {
		t.Run(fmt.Sprint(pending), func(t *testing.T) {
			f := newFinalizationManagerFixture(t)
			ctx := context.Background()
			intent := entity.HookTerminalIntent{Status: entity.ExptStatus_Failed, Reason: code}
			display := strings.Repeat("detail", 30)
			if pending {
				run := finalizationRead(t, f)
				_, err := f.repo.BeginFinalize(ctx, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: run.Version}, Intent: intent, DisplayMessage: &display})
				require.NoError(t, err)
			}
			// Existing code preservation only; this does not add before-block/Terminated support.
			require.NoError(t, f.manager.FinalizeRun(ctx, f.key, intent))
			state := finalizationRead(t, f)
			require.Equal(t, code, state.State.Intent.Reason)
			require.True(t, state.State.After.Activated)
			require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
			require.Equal(t, state, finalizationRead(t, f))
			require.Equal(t, 1, f.notifications)
			if pending {
				finalizationMessageReadback(t, f, display, code)
			}
		})
	}
}

func TestHookFinalizationMessageInvalidInteriorAndReasonLimit(t *testing.T) {
	for _, kind := range []string{"interior", "reason_limit"} {
		t.Run(kind, func(t *testing.T) {
			f := newFinalizationManagerFixture(t)
			ctx := context.Background()
			require.NoError(t, f.manager.CompleteRun(ctx, f.expt, f.key.RunID, f.space, nil))
			var err error
			if kind == "interior" {
				err = f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil, entity.WithStatusMessage("a\xffb"))
			} else {
				err = f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{Status: entity.ExptStatus_Failed, Reason: strings.Repeat("r", 129)})
			}
			require.Error(t, err)
			state := finalizationRead(t, f)
			require.Equal(t, entity.HookFinalizeNone, state.State.Finalize)
			require.False(t, state.State.After.Activated)
			require.Zero(t, f.notifications)
		})
	}
}

func TestHookFinalizationMessagePendingRecoveryAndCollision(t *testing.T) {
	f := newFinalizationManagerFixture(t)
	ctx := context.Background()
	a, b := strings.Repeat("p", 128)+"A", strings.Repeat("p", 128)+"B"
	fault := &finalizationRunFault{IHookRepo: f.repo, failCommit: true}
	f.deps.Runs = fault
	finalizationRecreate(t, f)
	require.NoError(t, f.manager.CompleteRun(ctx, f.expt, f.key.RunID, f.space, nil))
	require.Error(t, f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil, entity.WithStatus(entity.ExptStatus_Failed), entity.WithStatusMessage(a)))
	state := finalizationRead(t, f)
	require.Equal(t, entity.HookFinalizePending, state.State.Finalize)
	require.False(t, state.State.After.Activated)
	var log model.ExptRunLog
	require.NoError(t, f.sql.First(&log, f.key.RunID).Error)
	require.Equal(t, a, string(gptr.Indirect(log.StatusMessage)))
	var expt model.Experiment
	require.NoError(t, f.sql.First(&expt, f.expt).Error)
	require.Empty(t, gptr.Indirect(expt.StatusMessage))
	require.ErrorIs(t, f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil, entity.WithStatus(entity.ExptStatus_Failed), entity.WithStatusMessage(b)), entity.ErrHookStoreConflict)
	require.Equal(t, state, finalizationRead(t, f))
	fault.failCommit = false
	finalizationRecreate(t, f)
	require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
	finalizationMessageReadback(t, f, a, "")
	state = finalizationRead(t, f)
	require.ErrorIs(t, f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil, entity.WithStatus(entity.ExptStatus_Failed), entity.WithStatusMessage(b)), entity.ErrHookStoreConflict)
	require.NoError(t, f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil, entity.WithStatus(entity.ExptStatus_Failed)))
	require.Equal(t, state, finalizationRead(t, f))
	require.Equal(t, 1, f.notifications)
}
