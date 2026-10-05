// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	sm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func terminationEntryFixture(t *testing.T) *finalizationManagerFixture {
	t.Helper()
	f := neverAdmittedFinalizationFixture(t, true, true)
	m := f.base.(*ExptMangerImpl)
	m.runLogRepo = exptinfra.NewExptRunLogRepo(exptmysql.NewExptRunLogDAO(f.p))
	m.exptRepo = exptinfra.NewExptRepo(exptmysql.NewExptDAO(f.p), nil, nil)
	finalizationRecreate(t, f)
	return f
}

// Exercises the exact public Kill manager sequence against real MySQL and Redis.
func TestHookTerminationEntryPublicSequence(t *testing.T) {
	f := terminationEntryFixture(t)
	ctx := context.Background()
	completion := claimTerminationBefore(t, f)
	f.base.(*ExptMangerImpl).exptAggrResultService = sm.NewMockExptAggrResultService(gomock.NewController(t))
	finalizationRecreate(t, f)
	require.NoError(t, f.manager.SetExptTerminating(ctx, f.expt, f.key.RunID, f.space, nil))
	accepted := finalizationRead(t, f)
	require.Equal(t, entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated}, accepted.State.Intent)
	require.Equal(t, entity.HookFinalizePending, accepted.State.Finalize)
	require.Equal(t, entity.HookGateClosed, accepted.State.Gate)
	require.Equal(t, entity.ExptStatus_Terminating, accepted.State.Status)
	require.False(t, accepted.State.After.Activated)
	require.Equal(t, completion.Token.Generation+1, accepted.State.Before.Generation)
	late, err := f.repo.CompleteAttempt(ctx, completion)
	require.NoError(t, err)
	require.True(t, late.Effects.LateIgnored)
	require.Equal(t, accepted, late.Run)
	var projected model.Experiment
	require.NoError(t, f.sql.First(&projected, f.expt).Error)
	require.Equal(t, int32(entity.ExptStatus_Terminating), projected.Status)
	require.NoError(t, f.manager.SetExptTerminating(ctx, f.expt, f.key.RunID, f.space, nil))
	require.Equal(t, accepted, finalizationRead(t, f))
	require.NoError(t, f.manager.CompleteRun(ctx, f.expt, f.key.RunID, f.space, nil, entity.WithStatus(entity.ExptStatus_Terminated)))
	require.Equal(t, accepted, finalizationRead(t, f), "preparation cannot commit or unlock")
	require.Zero(t, f.notifications)
	require.NoError(t, f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil,
		entity.WithStatus(entity.ExptStatus_Terminated), entity.WithCompleteInterval(time.Second), entity.NoAggrCalculate()))
	done := finalizationRead(t, f)
	require.Equal(t, entity.HookFinalizeCommitted, done.State.Finalize)
	require.True(t, done.State.After.Activated)
	require.Equal(t, 1, f.notifications)
	require.NoError(t, f.manager.SetExptTerminating(ctx, f.expt, f.key.RunID, f.space, nil))
	require.Equal(t, done, finalizationRead(t, f), "repeated acceptance must not resurrect Terminating")
}

func TestHookTerminationEntryAtomicFailure(t *testing.T) {
	f := terminationEntryFixture(t)
	before := finalizationRead(t, f)
	failure := errors.New("termination projection unavailable")
	name := fmt.Sprintf("termination-entry-failure-%d", f.expt)
	require.NoError(t, f.sql.Callback().Update().Before("gorm:update").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table == "experiment" {
			tx.AddError(failure)
		}
	}))
	t.Cleanup(func() { require.NoError(t, f.sql.Callback().Update().Remove(name)) })
	require.ErrorIs(t, f.manager.SetExptTerminating(context.Background(), f.expt, f.key.RunID, f.space, nil), failure)
	require.Equal(t, before, finalizationRead(t, f), "intent, fence and both status projections must roll back together")
	require.Zero(t, f.notifications)
}

func TestHookTerminationEntryDirectMessageAndReplay(t *testing.T) {
	for _, status := range []entity.ExptStatus{entity.ExptStatus_Terminated, entity.ExptStatus_SystemTerminated} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			f := terminationEntryFixture(t)
			ctx := context.Background()
			q := &finalizationQuotaFault{QuotaRepo: f.quota, fail: true}
			f.base.(*ExptMangerImpl).quotaRepo = q
			finalizationRecreate(t, f)
			var err error
			if status == entity.ExptStatus_Terminated {
				err = f.manager.Kill(ctx, f.expt, &f.key.RunID, f.space, strings.Repeat("中", 67), nil)
			} else {
				err = f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil, entity.WithStatus(status), entity.WithStatusMessage(strings.Repeat("中", 67)))
			}
			require.ErrorContains(t, err, "quota unavailable")
			pending := finalizationRead(t, f)
			require.Equal(t, entity.HookTerminalIntent{Status: status}, pending.State.Intent)
			require.Equal(t, strings.Repeat("中", 66), gptr.Indirect(pending.DisplayMessage))
			require.False(t, pending.State.After.Activated)
			require.ErrorIs(t, f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil, entity.WithStatus(status), entity.WithStatusMessage("conflicting")), entity.ErrHookStoreConflict)
			require.Equal(t, pending, finalizationRead(t, f))
			q.fail = false
			require.NoError(t, f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil, entity.WithStatus(status)))
			done := finalizationRead(t, f)
			require.Equal(t, pending.State.Intent, done.State.Intent)
			require.Equal(t, pending.DisplayMessage, done.DisplayMessage)
			require.True(t, done.State.After.Activated)
			require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
			require.Equal(t, done, finalizationRead(t, f))
		})
	}
}

func TestHookTerminationEntryAcceptsBeforeDelay(t *testing.T) {
	f := terminationEntryFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		result <- f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil, entity.WithStatus(entity.ExptStatus_Terminated), entity.WithStatusMessage("delayed"), entity.WithCompleteInterval(time.Hour))
	}()
	defer func() { cancel(); <-result }()
	require.Eventually(t, func() bool {
		s, err := f.repo.GetRun(context.Background(), f.key)
		return err == nil && s.State.Finalize == entity.HookFinalizePending
	}, 2*time.Second, 10*time.Millisecond)
	pending := finalizationRead(t, f)
	require.Equal(t, entity.ExptStatus_Terminating, pending.State.Status)
	require.Equal(t, "delayed", gptr.Indirect(pending.DisplayMessage))
	require.False(t, pending.State.After.Activated)
}

func TestHookTerminationEntryActivePendingDoesNotComplete(t *testing.T) {
	f := terminationEntryFixture(t)
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("execution_started", true).Error)
	ctx := context.Background()
	require.NoError(t, f.manager.SetExptTerminating(ctx, f.expt, f.key.RunID, f.space, nil))
	pending := finalizationRead(t, f)
	require.Equal(t, entity.HookFinalizePending, pending.State.Finalize)
	require.ErrorIs(t, f.manager.CompleteRun(ctx, f.expt, f.key.RunID, f.space, nil, entity.WithStatus(entity.ExptStatus_Terminated)), entity.ErrHookFinalizationUnsettled)
	require.ErrorIs(t, f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil, entity.WithStatus(entity.ExptStatus_Terminated)), entity.ErrHookFinalizationUnsettled)
	require.Equal(t, pending, finalizationRead(t, f))
	require.Zero(t, f.notifications)
}

func TestHookTerminationEntryAcceptedEmptyDisplayImmutable(t *testing.T) {
	f := terminationEntryFixture(t)
	ctx := context.Background()
	require.NoError(t, f.manager.SetExptTerminating(ctx, f.expt, f.key.RunID, f.space, nil))
	pending := finalizationRead(t, f)
	require.ErrorIs(t, f.manager.Kill(ctx, f.expt, &f.key.RunID, f.space, "late message", nil), entity.ErrHookStoreConflict)
	require.Equal(t, pending, finalizationRead(t, f))
	require.NoError(t, f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil, entity.WithStatus(entity.ExptStatus_Terminated)))
	require.Empty(t, gptr.Indirect(finalizationRead(t, f).DisplayMessage))
}

func TestHookTerminationEntryStaleRunLeavesSuccessor(t *testing.T) {
	f := terminationEntryFixture(t)
	ctx := context.Background()
	next := finalizationTestIDs.Add(1)
	key := entity.HookRunKey{WorkspaceID: f.space, ExperimentID: f.expt, RunID: next}
	fresh, err := f.repo.CreateRunWithHooks(ctx, entity.HookCreateRunInput{Key: key, ExpectedLatestRunID: f.key.RunID,
		RunLog:   &entity.ExptRunLog{ID: next, ExptRunID: next, SpaceID: f.space, ExptID: f.expt, Mode: 1, Status: 3, CreatedBy: "user"},
		Snapshot: entity.HookProtectedSnapshot{Cipher: []byte{1}, Hash: strings.Repeat("b", 64), KeyID: "key", ExecutionScope: "local"},
		After:    &entity.HookOperationSeed{ID: finalizationTestIDs.Add(1), OperationID: fmt.Sprint(next), IdempotencyKey: fmt.Sprint(next)}})
	require.NoError(t, err)
	var before model.Experiment
	require.NoError(t, f.sql.First(&before, f.expt).Error)
	owner := fmt.Sprintf("hook_run:%d:abcdef0123456789abcdef0123456789", next)
	lockKey := fmt.Sprintf("expt_run_mutex_lock:%d", f.expt)
	require.NoError(t, f.redis.Set(ctx, lockKey, owner, time.Hour).Err())
	require.NoError(t, f.manager.SetExptTerminating(ctx, f.expt, f.key.RunID, f.space, nil))
	require.NoError(t, f.manager.CompleteRun(ctx, f.expt, f.key.RunID, f.space, nil, entity.WithStatus(entity.ExptStatus_Terminated)))
	require.NoError(t, f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil, entity.WithStatus(entity.ExptStatus_Terminated)))
	var after model.Experiment
	require.NoError(t, f.sql.First(&after, f.expt).Error)
	require.Equal(t, before, after)
	got, err := f.repo.GetRun(ctx, key)
	require.NoError(t, err)
	require.Equal(t, fresh.Run, got)
	require.Equal(t, owner, f.redis.Get(ctx, lockKey).Val())
	require.Zero(t, f.notifications)
}

func TestHookTerminationEntryCannotResurrect(t *testing.T) {
	for _, state := range []string{"terminal-experiment", "terminal-run", "deleted", "wrong-workspace"} {
		t.Run(state, func(t *testing.T) {
			f := terminationEntryFixture(t)
			space := f.space
			switch state {
			case "terminal-experiment":
				require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumn("status", int32(entity.ExptStatus_Success)).Error)
			case "terminal-run":
				require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", f.key.RunID).UpdateColumn("status", int64(entity.ExptStatus_Success)).Error)
			case "deleted":
				require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumn("deleted_at", time.Now()).Error)
			case "wrong-workspace":
				space++
			}
			before := finalizationRead(t, f)
			var projection model.Experiment
			require.NoError(t, f.sql.Unscoped().First(&projection, f.expt).Error)
			require.Error(t, f.manager.SetExptTerminating(context.Background(), f.expt, f.key.RunID, space, nil))
			require.Equal(t, before, finalizationRead(t, f))
			var after model.Experiment
			require.NoError(t, f.sql.Unscoped().First(&after, f.expt).Error)
			require.Equal(t, projection, after)
		})
	}
}

func TestHookTerminationEntryConcurrentAcceptance(t *testing.T) {
	f := terminationEntryFixture(t)
	before := finalizationRead(t, f)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { errs <- f.manager.SetExptTerminating(context.Background(), f.expt, f.key.RunID, f.space, nil) }()
	}
	for i := 0; i < 2; i++ {
		require.NoError(t, <-errs)
	}
	accepted := finalizationRead(t, f)
	require.Equal(t, before.Version+1, accepted.Version)
	require.Equal(t, before.State.Before.Generation+1, accepted.State.Before.Generation)
	require.Equal(t, entity.HookFinalizePending, accepted.State.Finalize)
	require.False(t, accepted.State.After.Activated)
}

func TestHookTerminationEntryBeforeFailedReasonUnchanged(t *testing.T) {
	f := terminationEntryFixture(t)
	ctx := context.Background()
	complete := claimTerminationBefore(t, f)
	complete.Outcome.Code = entity.HookFailed
	failed, err := f.repo.CompleteAttempt(ctx, complete)
	require.NoError(t, err)
	require.NoError(t, f.manager.SetExptTerminating(ctx, f.expt, f.key.RunID, f.space, nil))
	accepted := finalizationRead(t, f)
	require.Equal(t, failed.Run.TerminalAt, accepted.TerminalAt)
	require.Equal(t, entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated, Reason: "HOOK_BEFORE_FAILED"}, accepted.State.Intent)
	require.NoError(t, f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil, entity.WithStatus(entity.ExptStatus_Terminated)))
	done := finalizationRead(t, f)
	require.Equal(t, accepted.State.Intent, done.State.Intent)
	require.Empty(t, gptr.Indirect(done.DisplayMessage))
}

func TestHookTerminationEntryInDebtUnknownFirstCallStaysGuarded(t *testing.T) {
	f := newFinalizationManagerFixture(t)
	require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("status", int32(entity.ItemRunState_Processing)).Error)
	before := finalizationRead(t, f)
	err := f.manager.CompleteRun(context.Background(), f.expt, f.key.RunID, f.space, nil, entity.WithCID("terminate:indebt"))
	require.ErrorIs(t, err, entity.ErrHookFinalizationUnsettled)
	require.Equal(t, before, finalizationRead(t, f))
	require.Equal(t, entity.HookFinalizeNone, before.State.Finalize)
}

func TestHookTerminationEntryNoHookConstructorUnchanged(t *testing.T) {
	f := terminationEntryFixture(t)
	before := finalizationRead(t, f)
	require.NoError(t, f.base.SetExptTerminating(context.Background(), f.expt, f.key.RunID, f.space, nil))
	after := finalizationRead(t, f)
	require.Equal(t, entity.ExptStatus_Terminating, after.State.Status)
	require.Equal(t, before.State.Gate, after.State.Gate)
	require.Equal(t, entity.HookFinalizeNone, after.State.Finalize)
	require.Empty(t, after.State.Intent)
}
