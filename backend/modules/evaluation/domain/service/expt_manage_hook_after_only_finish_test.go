// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/spi"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	hookinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	idemrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/idem"
	idemredis "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/idem/redis"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type afterOnlyTransport func(context.Context, entity.HookTransportInput) entity.HookTransportResult

func (f afterOnlyTransport) Invoke(ctx context.Context, in entity.HookTransportInput) entity.HookTransportResult {
	return f(ctx, in)
}

func (f *afterOnlyFixture) settle(t *testing.T, success bool) {
	t.Helper()
	ctx := context.Background()
	f.prepare(t)
	ms := f.initialize(t)
	archive, err := (&ExptResultServiceImpl{}).WithHookArchive(f.deps.Repository.(repo.IHookItemArchiveRepo), "local")
	require.NoError(t, err)
	source, err := f.deps.Repository.ReadFinalizationSource(ctx, f.key)
	require.NoError(t, err)
	for _, m := range ms {
		changed, err := f.deps.Repository.(repo.IHookSchedulerRepo).PersistHookDispatch(ctx, entity.HookSchedulerDispatchInput{Key: f.key, ExecutionScope: "local", ItemIDs: []int64{m.Frozen.ItemID}})
		require.NoError(t, err)
		require.Equal(t, []int64{m.Frozen.ItemID}, changed)
		ctx, eiec, pre := admitLazyTurn(t, f.finalizationManagerFixture, m)
		require.NoError(t, pre.PreEval(ctx, eiec))
		var row model.ExptTurnResultRunLog
		require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=? AND item_id=?", f.space, f.key.RunID, m.Frozen.ItemID).First(&row).Error)
		base, err := convert.NewExptTurnResultRunLogConvertor().PO2DO(&row)
		require.NoError(t, err)
		next := *base
		next.Status = entity.TurnRunState_Success
		status := entity.ItemRunState_Success
		if !success {
			next.Status, status = entity.TurnRunState_Fail, entity.ItemRunState_Fail
		}
		writer := ctx.Value(itemHookProgressContextKey{}).(itemHookProgressBinding).repo
		_, err = writer.(repo.IHookTurnResultWriteRepo).WriteTurnResult(ctx, entity.HookTurnProgressInput{Base: base, Progress: &next})
		require.NoError(t, err)
		_, err = writer.(repo.IHookItemRunWriteRepo).WriteItemRun(ctx, entity.HookItemRunWriteInput{HookRunKey: f.key, ItemID: m.Frozen.ItemID, ItemVersionID: m.Frozen.ItemVersionID, Status: status})
		require.NoError(t, err)
		_, err = archive.RecordItemRunLogs(ctx, f.expt, f.key.RunID, m.Frozen.ItemID, f.space, source.Experiment)
		require.NoError(t, err)
	}
}

func (f *afterOnlyFixture) deliver(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	run := finalizationRead(t, f.finalizationManagerFixture)
	f.noBefore(t)
	calls := 0
	codec := f.manager.hooks.Codec
	executor, err := NewHookAttemptExecutor(f.repo, codec, codec.(hook.RequestBuilder), afterOnlyTransport(func(_ context.Context, in entity.HookTransportInput) entity.HookTransportResult {
		calls++
		require.Equal(t, run.State.After.ID, in.Request.GetOperationID())
		require.Equal(t, run.Operations[0].IdempotencyKey, in.Request.GetIdempotencyKey())
		require.Equal(t, fmt.Sprint(f.key.RunID), in.Request.Context.GetRunID())
		require.Equal(t, "user", in.Request.Context.Initiator.GetUserID())
		require.Equal(t, int32(1), in.Request.GetAttempt())
		return entity.HookTransportResult{Outcome: entity.HookOutcome{Code: entity.HookSucceeded, HTTPStatus: 200}, Response: &spi.InvokeExperimentHookResponse{Status: gptr.Of("succeeded"), Result_: map[string]string{"after": "original"}}, LocalCompletedAt: time.Now()}
	}), hookinfra.NewCompletionProjector())
	require.NoError(t, err)
	in := hook.AttemptExecutionInput{Key: f.key, OperationID: run.State.After.ID, Phase: entity.HookPhaseAfter, ExecutionScope: "local", Owner: "after-only-test", AttemptID: finalizationTestIDs.Add(1)}
	t.Cleanup(func() {
		require.NoError(t, f.sql.Where("operation_id=?", in.OperationID).Delete(&model.ExptLifecycleHookAttempt{}).Error)
	})
	result, err := executor.Execute(ctx, in)
	require.NoError(t, err)
	require.Equal(t, entity.HookOperationSucceeded, result.Run.State.After.Status)
	_, err = executor.Execute(ctx, in)
	require.NoError(t, err)
	require.Equal(t, 1, calls)
}

func TestHookAfterOnlyNormalFinalizationAndDeliveryMySQL(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeTrialRun} {
		for _, success := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d/success=%t", mode, success), func(t *testing.T) {
				f := newAfterOnlyFixture(t, mode)
				f.settle(t, success)
				ctx := context.Background()
				require.NoError(t, f.manager.CompleteRun(ctx, f.expt, f.key.RunID, f.space, nil))
				require.False(t, finalizationRead(t, f.finalizationManagerFixture).State.After.Activated)
				require.NoError(t, f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil))
				done := finalizationRead(t, f.finalizationManagerFixture)
				want := entity.ExptStatus_Failed
				if success {
					want = entity.ExptStatus_Success
				}
				require.Equal(t, want, done.State.Status)
				f.deliver(t)
			})
		}
	}
}

func TestHookAfterOnlyRecoveryAndSuccessorIsolationMySQL(t *testing.T) {
	for _, stage := range []string{"partial", "initialized"} {
		t.Run(stage, func(t *testing.T) {
			f := newAfterOnlyFixture(t, entity.EvaluationModeTrialRun)
			f.prepare(t)
			if stage == "partial" {
				f.partial(t)
			} else {
				f.initialize(t)
			}
			ctx := context.Background()
			injected := errors.New("cleanup storage unavailable")
			require.NoError(t, f.sql.Callback().Update().Before("gorm:update").Register("after-only-cleanup-failure", func(tx *gorm.DB) {
				if tx.Statement.Table == "expt_item_result_run_log" {
					tx.AddError(injected)
				}
			}))
			err := f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated})
			require.Error(t, err)
			require.NoError(t, f.sql.Callback().Update().Remove("after-only-cleanup-failure"))
			pending := finalizationRead(t, f.finalizationManagerFixture)
			require.Equal(t, entity.HookFinalizePending, pending.State.Finalize)
			require.False(t, pending.State.After.Activated)
			// Create the successor with the same genuine Manager; snapshot/creator stay per Run.
			nextKey := f.key
			nextKey.RunID = finalizationTestIDs.Add(1)
			initial, err := f.manager.hooks.Initialization.ReadRunInitialization(ctx, nextKey)
			require.NoError(t, err)
			attempted := false
			raw := "[82]"
			require.NoError(t, f.manager.initializeHookRun(ctx, &entity.ExptRunLog{ID: nextKey.RunID, ExptRunID: nextKey.RunID, SpaceID: f.space, ExptID: f.expt, Mode: int32(f.mode), Status: int64(entity.ExptStatus_Pending), CreatedBy: "successor"}, initial, &attempted, &raw))
			next, err := f.repo.GetRun(ctx, nextKey)
			require.NoError(t, err)
			owner := fmt.Sprintf("hook_run:%d:0123456789abcdef0123456789abcdef", nextKey.RunID)
			lockKey := fmt.Sprintf("expt_run_mutex_lock:%d", f.expt)
			require.NoError(t, f.redis.Set(ctx, lockKey, owner, time.Hour).Err())
			var before model.Experiment
			require.NoError(t, f.sql.First(&before, f.expt).Error)
			require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
			var after model.Experiment
			require.NoError(t, f.sql.First(&after, f.expt).Error)
			require.Equal(t, before, after)
			require.Equal(t, owner, f.redis.Get(ctx, lockKey).Val())
			f.deliver(t)
			nextAfter, err := f.repo.GetRun(ctx, nextKey)
			require.NoError(t, err)
			require.Equal(t, next, nextAfter)
		})
	}
}

func TestHookAfterOnlyConsumerFailureMySQL(t *testing.T) {
	f := newAfterOnlyFixture(t, entity.EvaluationModeSubmit)
	f.prepare(t)
	f.initialize(t)
	ctx := context.Background()
	control := exptinfra.NewHookTurnProgressRepo(f.p, func(context.Context) (string, error) { return "local", nil }).(repo.IHookConsumerControlRepo)
	for _, id := range []int64{81, 82} {
		result, err := control.ApplyHookConsumerControl(ctx, entity.HookConsumerControlInput{Key: f.key, ItemID: id, Action: entity.HookConsumerFail, ErrorMessage: "retry exhausted before execution"})
		require.NoError(t, err)
		require.True(t, result.Handled)
		require.True(t, result.Changed)
		archive, err := (&ExptResultServiceImpl{}).WithHookArchive(f.deps.Repository.(repo.IHookItemArchiveRepo), "local")
		require.NoError(t, err)
		source, err := f.deps.Repository.ReadFinalizationSource(ctx, f.key)
		require.NoError(t, err)
		_, err = archive.RecordItemRunLogs(ctx, f.expt, f.key.RunID, id, f.space, source.Experiment)
		require.NoError(t, err)
	}
	require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
	require.Equal(t, entity.ExptStatus_Failed, finalizationRead(t, f.finalizationManagerFixture).State.Status)
}

func TestHookAfterOnlyDefaultSchedulerConsumesFrozenInitializationMySQL(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeTrialRun} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			f := newAfterOnlyFixture(t, mode)
			ctx := context.Background()
			s := hookPersistenceScheduler(t, f.finalizationManagerFixture, func(context.Context) error { return nil })
			pub := &schedulerHookPublisher{}
			s.Publisher = pub
			source, err := f.deps.Repository.ReadFinalizationSource(ctx, f.key)
			require.NoError(t, err)
			s.Manager = startupDetailManager{IExptManager: f.manager, expt: source.Experiment}
			event := &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ExptRunMode: mode}
			require.NoError(t, s.schedule(ctx, event))
			require.Len(t, pub.published, 1, "unprepared after-only must request continuation without starting")
			f.prepare(t)
			ms := f.initialize(t)
			before := hookPersistenceRead(t, f.finalizationManagerFixture, ms[0])
			data := &startupDataset{}
			idem := idemrepo.NewIdempotentService(idemredis.NewIdemDAO(f.redis))
			factory := &startupDefaultFactory{SchedulerModeFactory: NewSchedulerModeFactory(f.manager, s.ExptItemResultRepo, s.ExptStatsRepo, s.ExptTurnResultRepo, activeReferenceIDs{}, data, f.manager.exptRepo, nil, idem, s.Configer, s.Publisher, nil, hookPersistenceFilters{}, nil, nil, nil)}
			s.schedulerModeFactory = factory
			source, err = f.deps.Repository.ReadFinalizationSource(ctx, f.key)
			require.NoError(t, err)
			s.Manager = startupDetailManager{IExptManager: f.manager, expt: source.Experiment}
			require.ErrorIs(t, s.schedule(ctx, event), errStartupScanReached)
			require.Equal(t, 1, factory.scans)
			require.Zero(t, data.calls, "no legacy live selection after frozen initialization")
			require.Equal(t, before, hookPersistenceRead(t, f.finalizationManagerFixture, ms[0]))
		})
	}
}

func TestHookAfterOnlyLateResultCannotReopenFinalizedRunMySQL(t *testing.T) {
	f := newAfterOnlyFixture(t, entity.EvaluationModeTrialRun)
	f.prepare(t)
	ms := f.initialize(t)
	ctx := context.Background()
	_, err := f.deps.Repository.(repo.IHookSchedulerRepo).PersistHookDispatch(ctx, entity.HookSchedulerDispatchInput{Key: f.key, ExecutionScope: "local", ItemIDs: []int64{81}})
	require.NoError(t, err)
	itemCtx, item, pre := admitLazyTurn(t, f.finalizationManagerFixture, ms[0])
	require.NoError(t, pre.PreEval(itemCtx, item))
	base := lateProofTurn(t, f.finalizationManagerFixture)
	require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated}))
	done := finalizationRead(t, f.finalizationManagerFixture)
	before := hookPersistenceRead(t, f.finalizationManagerFixture, ms[0])
	next := *base
	next.Status = entity.TurnRunState_Success
	writer := itemCtx.Value(itemHookProgressContextKey{}).(itemHookProgressBinding).repo
	_, err = writer.(repo.IHookTurnResultWriteRepo).WriteTurnResult(ctx, entity.HookTurnProgressInput{Base: base, Progress: &next})
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	require.Equal(t, done, finalizationRead(t, f.finalizationManagerFixture))
	require.Equal(t, before, hookPersistenceRead(t, f.finalizationManagerFixture, ms[0]))
	f.deliver(t)
}
