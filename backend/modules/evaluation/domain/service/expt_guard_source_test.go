// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	experimentrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	experimentmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/ctxcache"
)

// Removing the source check before Confirm must make these real-DAO cases fail.
func TestGuardSourceMySQLDirectConsumerChecksBeforeConfirm(t *testing.T) {
	for _, kind := range []string{"old_empty", "old_rq1", "terminal", "read_error"} {
		t.Run(kind, func(t *testing.T) {
			f := newLateWriteFixture(t, "rq1")
			event := f.event()
			switch kind {
			case "old_empty":
				event.RequestID = ""
				f.token("rq2")
			case "old_rq1":
				f.token("rq2")
			case "terminal":
				for _, table := range []string{"expt_item_result", "expt_item_result_run_log"} {
					require.NoError(t, f.conn.Table(table).Where("space_id=? AND item_id=?", f.space, f.item).Update("status", entity.ItemRunState_Success).Error)
				}
			case "read_error":
				require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("ext", []byte(`{"manual_retry":"invalid"}`)).Error)
			}
			guard := &fakeGuard{confirmResult: true}
			svc := &ExptItemEventEvalServiceImpl{centralGuard: guard, exptItemResultRepo: f.itemRepo(), dispatchRepo: experimentrepo.NewExptItemDispatchRepo(experimentmysql.NewExptItemDispatchDAO(f.provider))}
			ctx := ctxcache.Init(context.Background())
			event.WithCtxCentralAdmittedExpt(ctx, &entity.Experiment{SchedulerScope: testScope, ExptDispatchMode: entity.ExptDispatchModeEnforce})
			before := f.snapshot()
			executed := 0
			err := svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error { executed++; return nil })(ctx, event)
			if kind == "read_error" {
				require.True(t, errno.NeedMQRetry(err), "source read error must remain broker retry")
			} else {
				require.NoError(t, err)
			}
			require.Zero(t, guard.confirmCalls, "stale/terminal/unreadable original source must stop before quota")
			require.Zero(t, executed)
			require.Empty(t, guard.releases())
			require.Equal(t, before, f.snapshot())
			t.Log("REAL_MYSQL_GUARD_SOURCE_PRECHECK zero quota/execution/state effects")
		})
	}
}

func TestGuardSourceReleaseRejectsNewTerminalObservation(t *testing.T) {
	ctrl := gomock.NewController(t)
	dispatch := repomocks.NewMockIExptItemDispatchRepo(ctrl)
	dispatch.EXPECT().MGetDispatchObservations(gomock.Any(), int64(3), int64(1), int64(2), []int64{4}).Return([]*repo.ExptDispatchObservation{{ItemID: 4, RequestID: "rq2", Status: int32(entity.ItemRunState_Success)}}, nil)
	guard := &fakeGuard{}
	event := &entity.ExptItemEvalEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2, EvalSetItemID: 4, RequestID: "rq1"}
	svc := &ExptItemEventEvalServiceImpl{centralGuard: guard, dispatchRepo: dispatch}
	svc.releaseQuotaIfItemTerminal(context.Background(), event, testScope, guard, nil)
	require.Empty(t, guard.releases(), "rq2 terminal is not evidence that processing-start rq1 completed")
}

type guardSourceContextKey struct{}

type boundedSourceGuard struct {
	fakeGuard
	t *testing.T
}

func (g *boundedSourceGuard) Release(ctx context.Context, scope string, run, item int64, expectedRequestID, reason string) error {
	require.NoError(g.t, ctx.Err(), "resource release must survive parent cancellation")
	require.Equal(g.t, "scope-value", ctx.Value(guardSourceContextKey{}))
	deadline, ok := ctx.Deadline()
	require.True(g.t, ok)
	require.Positive(g.t, time.Until(deadline))
	require.LessOrEqual(g.t, time.Until(deadline), 8*time.Second)
	return g.fakeGuard.Release(ctx, scope, run, item, expectedRequestID, reason)
}

func TestGuardSourceOutsideReleaseHasIndependentBound(t *testing.T) {
	guard := &boundedSourceGuard{t: t}
	ctx, cancel := context.WithCancel(ctxcache.Init(context.WithValue(context.Background(), guardSourceContextKey{}, "scope-value")))
	event := &entity.ExptItemEvalEvent{ExptRunID: 2, EvalSetItemID: 4, RequestID: "rq1"}
	event.WithCtxCentralQuotaHeld(ctx, testScope)
	ctx = context.WithValue(ctx, currentItemSourceKey{}, &currentItemExecution{})
	cancel()
	svc := &ExptItemEventEvalServiceImpl{centralGuard: guard}
	svc.releaseCentralQuotaOutsideGate(ctx, event, errors.New("debt"), "expt terminated on indebt")
	svc.releaseCentralQuotaOutsideGate(ctx, event, errors.New("debt"), "expt terminated on indebt")
	require.Len(t, guard.releases(), 1)
	require.True(t, executionSource(ctx).QuotaReleased)
}

func TestGuardSourceMySQLCompletedThenCancelReleases(t *testing.T) {
	f := newLateWriteFixture(t, "rq1")
	source := f.source()
	guard := &boundedSourceGuard{t: t}
	guard.confirmResult = true
	svc := &ExptItemEventEvalServiceImpl{centralGuard: guard, exptItemResultRepo: f.itemRepo(), dispatchRepo: experimentrepo.NewExptItemDispatchRepo(experimentmysql.NewExptItemDispatchDAO(f.provider))}
	ctx, cancel := context.WithCancel(ctxcache.Init(context.WithValue(context.Background(), guardSourceContextKey{}, "scope-value")))
	defer cancel()
	ctx = context.WithValue(ctx, currentItemSourceKey{}, &currentItemExecution{Source: source})
	event := f.event()
	event.WithCtxCentralAdmittedExpt(ctx, &entity.Experiment{SchedulerScope: testScope, ExptDispatchMode: entity.ExptDispatchModeEnforce})
	var completed map[string]string
	err := svc.HandleCentralReservation(func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		applied, err := f.port.CompleteCurrentItemRun(ctx, source, &entity.ExptCurrentItemRunWrite{Status: entity.ItemRunState_Fail, FailPendingTurns: true})
		require.NoError(t, err)
		require.True(t, applied)
		executionSource(ctx).Completed = true
		completed = f.snapshot()
		cancel()
		return errors.New("debt")
	})(ctx, event)
	require.True(t, errno.NeedMQRetry(err))
	require.Len(t, guard.releases(), 1, "committed terminal source still owes release after cancellation")
	require.Equal(t, completed, f.snapshot(), "independent release must not authorize state writes")
	t.Log("REAL_MYSQL_GUARD_SOURCE_COMPLETED_CANCEL resource-only release")
}

func TestGuardSourceMySQLManagerUsesObservedTokens(t *testing.T) {
	for _, kind := range []string{"original", "canonical_terminal", "terminal_run", "moved", "bad_ext"} {
		t.Run(kind, func(t *testing.T) {
			f := newLateWriteFixture(t, "rq1")
			if kind == "canonical_terminal" {
				require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("status", entity.ItemRunState_Fail).Error)
			}
			if kind == "terminal_run" {
				require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=? AND expt_id=?", f.space, f.expt).Update("status", entity.ExptStatus_Terminated).Error)
			}
			if kind == "moved" {
				require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("expt_run_id", f.run+1).Error)
			}
			if kind == "bad_ext" {
				require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("ext", []byte(`{"manual_retry":"invalid"}`)).Error)
			}
			guard := &fakeGuard{}
			manager := &ExptMangerImpl{centralGuard: guard, itemResultRepo: f.itemRepo()}
			expt := &entity.Experiment{ID: f.expt, SpaceID: f.space, LatestRunID: f.run, SchedulerScope: testScope, ExptDispatchMode: entity.ExptDispatchModeEnforce}
			before := f.snapshot()
			manager.releaseCentralQuotaForRun(context.Background(), expt, &f.run, "run terminal", "test")
			if kind == "original" || kind == "canonical_terminal" || kind == "terminal_run" {
				require.Len(t, guard.releases(), 3)
				for _, release := range guard.releases() {
					if release.ItemID == f.item {
						require.Equal(t, "rq1", release.RequestID)
					} else {
						require.Empty(t, release.RequestID)
					}
				}
			} else {
				require.Empty(t, guard.releases(), "incomplete source cannot manufacture an empty or latest token")
			}
			require.Equal(t, before, f.snapshot())
			t.Log("REAL_MYSQL_GUARD_MANAGER original primary observations retained")
		})
	}
}

func TestGuardSourceOutsideReleaseUsesFrozenExecution(t *testing.T) {
	guard := &fakeGuard{}
	event := &entity.ExptItemEvalEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2, EvalSetItemID: 4, RequestID: "rq2"}
	ctx := ctxcache.Init(context.Background())
	event.WithCtxCentralQuotaHeld(ctx, testScope)
	ctx = context.WithValue(ctx, currentItemSourceKey{}, &currentItemExecution{Source: &entity.ExptCurrentItemRunSource{Current: entity.ExptItemCurrentRequest{SpaceID: 3, ExptID: 1, RunID: 2, ItemID: 4, RequestID: "rq1"}}})
	svc := &ExptItemEventEvalServiceImpl{centralGuard: guard}
	svc.releaseCentralQuotaOutsideGate(ctx, event, errors.New("debt"), "terminal")
	require.Len(t, guard.releases(), 1)
	require.Equal(t, "rq1", guard.releases()[0].RequestID)
}

func TestGuardSourceMySQLConfirmReceivesOriginalToken(t *testing.T) {
	for _, token := range []string{"", "rq1", "rq2"} {
		t.Run("token="+token, func(t *testing.T) {
			f := newLateWriteFixture(t, token)
			event := f.event()
			event.RequestID = token
			if token == "" {
				event.ExptRunMode = entity.EvaluationModeSubmit
			}
			guard := &fakeGuard{confirmResult: true}
			svc := &ExptItemEventEvalServiceImpl{centralGuard: guard, exptItemResultRepo: f.itemRepo(), dispatchRepo: experimentrepo.NewExptItemDispatchRepo(experimentmysql.NewExptItemDispatchDAO(f.provider))}
			ctx := admittedEnforceCtx(event)
			executed := 0
			err := svc.HandleCentralReservation(func(ctx context.Context, ev *entity.ExptItemEvalEvent) error {
				require.Equal(t, token, executionSource(ctx).Source.Current.RequestID)
				require.Equal(t, token, ev.RequestID)
				executed++
				// A downstream mutation cannot change the caller's original event.
				ev.RequestID = "changed"
				return nil
			})(ctx, event)
			require.NoError(t, err)
			require.Equal(t, []string{token}, guard.confirmTokens)
			require.Equal(t, 1, executed)
			require.Equal(t, token, event.RequestID)
			require.Empty(t, guard.releases())
			t.Log("REAL_MYSQL_GUARD_CONFIRM_ORIGINAL_TOKEN exact source checked before guard")
		})
	}
}

func TestGuardSourceMySQLCompletionThenRequeueDoesNotReleaseNew(t *testing.T) {
	f := newLateWriteFixture(t, "rq1")
	source := f.source()
	applied, err := f.port.CompleteCurrentItemRun(context.Background(), source, &entity.ExptCurrentItemRunWrite{Status: entity.ItemRunState_Fail, FailPendingTurns: true})
	require.NoError(t, err)
	require.True(t, applied)
	f.token("rq2")
	for _, table := range []string{"expt_item_result", "expt_item_result_run_log"} {
		require.NoError(t, f.conn.Table(table).Where("space_id=? AND item_id=?", f.space, f.item).Update("status", entity.ItemRunState_Success).Error)
	}
	guard := &fakeGuard{}
	svc := &ExptItemEventEvalServiceImpl{centralGuard: guard, dispatchRepo: experimentrepo.NewExptItemDispatchRepo(experimentmysql.NewExptItemDispatchDAO(f.provider))}
	ctx := context.WithValue(context.Background(), currentItemSourceKey{}, &currentItemExecution{Source: source, Completed: true})
	before := f.snapshot()
	svc.releaseQuotaIfItemTerminal(ctx, f.event(), testScope, guard, nil)
	require.Empty(t, guard.releases())
	require.False(t, executionSource(ctx).QuotaReleased)
	require.Equal(t, before, f.snapshot())
	t.Log("REAL_MYSQL_GUARD_LATE_RELEASE rq2 terminal never releases rq1 execution")
}

func TestGuardSourceOutsideReleaseFailureDoesNotSetFlag(t *testing.T) {
	guard := &fakeGuard{releaseErr: errors.New("storage unavailable")}
	event := &entity.ExptItemEvalEvent{ExptRunID: 2, EvalSetItemID: 4, RequestID: "rq1"}
	ctx := ctxcache.Init(context.Background())
	event.WithCtxCentralQuotaHeld(ctx, testScope)
	ctx = context.WithValue(ctx, currentItemSourceKey{}, &currentItemExecution{})
	svc := &ExptItemEventEvalServiceImpl{centralGuard: guard}
	svc.releaseCentralQuotaOutsideGate(ctx, event, errors.New("debt"), "terminal")
	require.False(t, executionSource(ctx).QuotaReleased)
	guard.releaseErr = nil
	svc.releaseCentralQuotaOutsideGate(ctx, event, errors.New("debt"), "terminal")
	svc.releaseCentralQuotaOutsideGate(ctx, event, errors.New("debt"), "terminal")
	require.Len(t, guard.releases(), 2)
	require.True(t, executionSource(ctx).QuotaReleased)
}

func TestGuardSourceMySQLDebtFinishThenCancelStillReleases(t *testing.T) {
	f := newLateWriteFixture(t, "rq1")
	source := f.source()
	event := f.event()
	event.RetryTimes = 3
	ctrl := gomock.NewController(t)
	config := componentmocks.NewMockIConfiger(ctrl)
	config.EXPECT().GetErrRetryConf(gomock.Any(), f.space, gomock.Any()).Return(&entity.RetryConf{RetryTimes: 3, IsInDebt: true}).AnyTimes()
	manager := svcmocks.NewMockIExptManager(ctrl)
	ctx, cancel := context.WithCancel(ctxcache.Init(context.WithValue(context.Background(), guardSourceContextKey{}, "scope-value")))
	defer cancel()
	event.WithCtxCentralQuotaHeld(ctx, testScope)
	guard := &boundedSourceGuard{t: t}
	terminator := &guardSourceTestTerminator{IExptManager: manager, finish: func(ctx context.Context, original *entity.ExptCurrentItemRunSource) (bool, error) {
		require.True(t, sameExecutionIdentity(source, original))
		cancel()
		releaseCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), 8*time.Second)
		defer stop()
		if err := guard.Release(releaseCtx, testScope, f.run, f.item, original.Current.RequestID, "debt"); err != nil {
			return true, err
		}
		return true, errno.WrapMQRetryErr(ctx.Err())
	}}
	svc := &ExptItemEventEvalServiceImpl{experimentRepo: f.experimentRepo(), centralGuard: guard, exptItemResultRepo: f.itemRepo(), exptTurnResultRepo: f.turnRepo(), manager: terminator, configer: config, itemLocker: lateWriteLocker(t, ctrl)}
	var completed map[string]string
	err := svc.HandleEventErr(func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		executionSource(ctx).Source = source
		executor := &ExptItemEvalCtxExecutor{ItemResultRepo: f.itemRepo(), Configer: config}
		err := executor.CompleteItemRun(ctx, &entity.ExptItemEvalCtx{Event: event}, errors.New("debt"))
		completed = f.snapshot()
		return err
	})(ctx, event)
	require.True(t, errno.NeedMQRetry(err))
	require.Len(t, guard.releases(), 1)
	require.Equal(t, "rq1", guard.releases()[0].RequestID)
	require.Equal(t, completed, f.snapshot())
	t.Log("REAL_MYSQL_GUARD_DEBT_CANCEL_AFTER_FINISH bounded resource release, no new writes")
}

type guardSourceScannerRepo struct {
	repo.IExptItemResultRepo
	repo.IExptCurrentResultRepo
	afterComplete func()
}

func (r *guardSourceScannerRepo) CompleteCurrentItemRun(ctx context.Context, source *entity.ExptCurrentItemRunSource, write *entity.ExptCurrentItemRunWrite) (bool, error) {
	applied, err := r.IExptCurrentResultRepo.CompleteCurrentItemRun(ctx, source, write)
	if applied && err == nil {
		r.afterComplete()
	}
	return applied, err
}

func TestGuardSourceMySQLZombieRetainsScannedToken(t *testing.T) {
	f := newLateWriteFixture(t, "rq1")
	old := time.Now().Add(-time.Hour)
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Updates(map[string]any{"updated_at": old}).Error)
	require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("evaluator_result_ids", []byte(`{}`)).Error)
	source := f.source()
	guard := &fakeGuard{}
	items := &guardSourceScannerRepo{IExptItemResultRepo: f.itemRepo(), IExptCurrentResultRepo: f.port, afterComplete: func() { f.token("rq2") }}
	ctrl := gomock.NewController(t)
	scheduler := &ExptSchedulerImpl{ExptItemResultRepo: items, centralGuard: guard, itemLocker: lateWriteLocker(t, ctrl)}
	scanned := &entity.ExptEvalItem{ItemID: f.item, Source: source, RequestID: "rq1", State: entity.ItemRunState_Processing, UpdatedAt: source.Current.UpdatedAt, RetryTimes: source.Current.RetryTimes}
	event := &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run}
	expt := &entity.Experiment{ID: f.expt, SchedulerScope: testScope, ExptDispatchMode: entity.ExptDispatchModeEnforce}
	_, zombies, err := scheduler.handleCurrentZombies(context.Background(), event, []*entity.ExptEvalItem{scanned}, expt, 1, false)
	require.NoError(t, err)
	require.Len(t, zombies, 1)
	require.Len(t, guard.releases(), 1)
	require.Equal(t, "rq1", guard.releases()[0].RequestID)
	require.Equal(t, "rq2", f.source().Current.RequestID)
	t.Log("REAL_MYSQL_GUARD_ZOMBIE captured rq1 retained after rq2 replacement")
}

func TestGuardSourceMySQLProductionChainStopsBeforeQuota(t *testing.T) {
	for _, kind := range []string{"old", "terminal", "error"} {
		t.Run(kind, func(t *testing.T) {
			f := newLateWriteFixture(t, "rq1")
			event := f.event()
			switch kind {
			case "old":
				f.token("rq2")
			case "terminal":
				require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("status", entity.ItemRunState_Success).Error)
			case "error":
				require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("ext", []byte(`{"manual_retry":"invalid"}`)).Error)
			}
			ctrl := gomock.NewController(t)
			guard := &fakeGuard{confirmResult: true}
			svc := &ExptItemEventEvalServiceImpl{centralGuard: guard, exptItemResultRepo: f.itemRepo(), itemLocker: lateWriteLocker(t, ctrl)}
			calls := 0
			before := f.snapshot()
			handler := RecordEvalChain(svc.HandleEventErr, svc.HandleEventLock, svc.HandleCurrentSource, svc.HandleCentralReservation)(func(context.Context, *entity.ExptItemEvalEvent) error { calls++; return nil })
			err := handler(admittedEnforceCtx(event), event)
			if kind == "error" {
				require.True(t, errno.NeedMQRetry(err))
			} else {
				require.NoError(t, err)
			}
			require.Zero(t, calls)
			require.Zero(t, guard.confirmCalls)
			require.Empty(t, guard.releases())
			require.Equal(t, before, f.snapshot())
			t.Log("REAL_MYSQL_GUARD_PRODUCTION_CHAIN source/lock before quota, error reaches broker")
		})
	}
}

func TestGuardSourceMySQLAbsentConfirmRequeuesOriginal(t *testing.T) {
	f := newLateWriteFixture(t, "rq1")
	event := f.event()
	guard := &fakeGuard{confirmResult: false}
	svc := &ExptItemEventEvalServiceImpl{centralGuard: guard, exptItemResultRepo: f.itemRepo(), dispatchRepo: experimentrepo.NewExptItemDispatchRepo(experimentmysql.NewExptItemDispatchDAO(f.provider))}
	err := svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error {
		t.Fatal("absent must stop execution")
		return nil
	})(admittedEnforceCtx(event), event)
	require.NoError(t, err)
	require.Equal(t, []string{"rq1"}, guard.confirmTokens)
	current, err := f.itemRepo().GetCurrentRequest(context.Background(), f.space, f.expt, f.run, f.item)
	require.NoError(t, err)
	require.Equal(t, "rq1", current.RequestID)
	require.Equal(t, entity.ItemRunState_Queueing, current.Status)
	require.Empty(t, guard.releases())
	t.Log("REAL_MYSQL_GUARD_FALSE original rq1 requeue CAS, no wildcard/new token")
}

func TestGuardSourceMySQLConcurrentOldAndCurrentConsumer(t *testing.T) {
	f := newFilterSyncFixture(t)
	// Use the real Redis item mutex with the real MySQL source repository.
	require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("status", entity.ItemRunState_Processing).Error)
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Updates(map[string]any{"status": int32(entity.ItemRunState_Processing), "result_state": int32(entity.ExptItemResultStateDefault)}).Error)
	f.token("rq2")
	guard := &fakeGuard{confirmResult: true}
	svc := &ExptItemEventEvalServiceImpl{centralGuard: guard, exptItemResultRepo: f.itemRepo(), itemLocker: func() lock.ILocker { return filterRealTestLocker(f.client) }}
	var wg sync.WaitGroup
	start := make(chan struct{})
	type delivery struct {
		token string
		err   error
	}
	errs := make(chan delivery, 2)
	for _, token := range []string{"rq1", "rq2"} {
		wg.Add(1)
		go func(token string) {
			defer wg.Done()
			<-start
			event := f.event()
			event.RequestID = token
			handler := RecordEvalChain(svc.HandleEventLock, svc.HandleCurrentSource, svc.HandleCentralReservation)(func(context.Context, *entity.ExptItemEvalEvent) error { return nil })
			errs <- delivery{token: token, err: handler(admittedEnforceCtx(event), event)}
		}(token)
	}
	close(start)
	wg.Wait()
	close(errs)
	for result := range errs {
		if errno.NeedMQRetry(result.err) {
			event := f.event()
			event.RequestID = result.token
			handler := RecordEvalChain(svc.HandleEventLock, svc.HandleCurrentSource, svc.HandleCentralReservation)(func(context.Context, *entity.ExptItemEvalEvent) error { return nil })
			require.NoError(t, handler(admittedEnforceCtx(event), event))
		} else {
			require.NoError(t, result.err)
		}
	}
	require.Equal(t, []string{"rq2"}, guard.confirmTokens)
	require.Empty(t, guard.releases())
	t.Log("REAL_MYSQL_GUARD_CONCURRENT real item lock serializes rq1/rq2; old quota zero")
}

type guardSourceTestTerminator struct {
	IExptManager
	finish func(context.Context, *entity.ExptCurrentItemRunSource) (bool, error)
}

func (t *guardSourceTestTerminator) TerminateCurrentRun(ctx context.Context, _, _, _ int64, _ *entity.Session, source *entity.ExptCurrentItemRunSource, _ ...entity.CompleteExptOptionFn) (bool, error) {
	return t.finish(ctx, source)
}
