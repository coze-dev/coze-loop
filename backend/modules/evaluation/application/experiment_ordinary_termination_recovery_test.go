// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bytedance/gg/gptr"
	red "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/backoff"
	idm "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	"github.com/coze-dev/coze-loop/backend/infra/redis"
	exptpb "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	idemm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	mm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	cm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	rpcm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	em "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	rm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	sm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
)

type ordinaryRecoveryStore struct {
	repo.IHookSchedulerRepo
	repo.IHookItemArchiveRepo
	repo.IHookSchedulerFailureRepo
	source             *entity.HookFinalizationSource
	err                error
	beforeCommitIO     func(context.Context, string)
	rowMu              sync.Mutex
	stores             repo.ManualCancellationRepositories
	beforeLatestUpdate func()
	afterFence         func(context.Context)
}

type manualFenceHeld struct{}

func (s *ordinaryRecoveryStore) WithManualCancellationFence(ctx context.Context, key entity.HookRunKey, fn func(context.Context, repo.ManualCancellationRepositories) error) error {
	if s.beforeCommitIO != nil {
		s.beforeCommitIO(ctx, "fence")
	}
	s.rowMu.Lock()
	defer func() {
		s.rowMu.Unlock()
		if s.afterFence != nil {
			s.afterFence(ctx)
		}
	}()
	if s.err != nil {
		return s.err
	}
	if s.source.Key != key || s.source.Managed || s.source.Experiment.LatestRunID != key.RunID {
		return entity.ErrHookStoreConflict
	}
	return fn(context.WithValue(ctx, manualFenceHeld{}, true), s.stores)
}

func (s *ordinaryRecoveryStore) ReadFinalizationSource(_ context.Context, _ entity.HookRunKey) (*entity.HookFinalizationSource, error) {
	if s.err != nil {
		return nil, s.err
	}
	copy := *s.source
	run, expt := *copy.RunLog, *copy.Experiment
	copy.RunLog, copy.Experiment = &run, &expt
	return &copy, nil
}

func (s *ordinaryRecoveryStore) ReadFinalizationStats(context.Context, entity.HookRunKey, string) (*entity.HookFinalizationStats, error) {
	panic("ordinary cancellation must not enter managed finalization")
}

type ordinaryRecoveryRuns struct {
	repo.IHookRepo
	repo.IHookExecutionInitializationRepo
}
type ordinaryRecoveryGate struct{}

func (ordinaryRecoveryGate) CanDispatch(context.Context, entity.HookRunKey) (entity.HookAdmissionDecision, error) {
	return entity.HookAdmissionDecision{Gate: entity.HookGateClosed, Reason: "RUN_CLOSED"}, nil
}

type ordinaryRecoveryFixture struct {
	app               *experimentApplication
	router            *hookRuntimeRouter
	store             *ordinaryRecoveryStore
	item              *entity.ExptItemResultRunLog
	turn              *entity.ExptTurnResultRunLog
	canonical         *entity.ExptItemResult
	canonicalTurn     *entity.ExptTurnResult
	events            []*entity.ExptScheduleEvent
	saveError         error
	pubError          error
	writes            int
	lifecycle         int
	completed         chan struct{}
	redis             redis.Cmdable
	consumer          *service.ExptItemEventEvalServiceImpl
	config            *cm.MockIConfiger
	aggregations      int
	cleanupLocks      int
	terminatingWrites int
	quotaRunID        int64
	templateStatus    entity.ExptStatus
	newRuns           map[int64]*entity.ExptRunLog
	redisClock        *miniredis.Miniredis
}

func newOrdinaryRecoveryFixture(t *testing.T, twoItems ...bool) *ordinaryRecoveryFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	clock := miniredis.RunT(t)
	c, err := redis.NewClient(&red.Options{Addr: clock.Addr()})
	require.NoError(t, err)
	if client, ok := redis.Unwrap(c); ok {
		t.Cleanup(func() { require.NoError(t, client.Close()) })
	}
	key := entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30}
	f := &ordinaryRecoveryFixture{redis: c, completed: make(chan struct{}, 4), store: &ordinaryRecoveryStore{source: &entity.HookFinalizationSource{Key: key,
		Experiment: &entity.Experiment{ID: 20, SpaceID: 10, LatestRunID: 30, Status: entity.ExptStatus_Processing},
		RunLog:     &entity.ExptRunLog{ID: 30, ExptRunID: 30, ExptID: 20, SpaceID: 10, Mode: int32(entity.EvaluationModeSubmit), Status: int64(entity.ExptStatus_Processing)}}},
		item:          &entity.ExptItemResultRunLog{ID: 40, SpaceID: 10, ExptID: 20, ExptRunID: 30, ItemID: 50, Status: int32(entity.ItemRunState_Processing)},
		turn:          &entity.ExptTurnResultRunLog{ID: 41, SpaceID: 10, ExptID: 20, ExptRunID: 30, ItemID: 50, TurnID: 60, Status: entity.TurnRunState_Processing},
		canonical:     &entity.ExptItemResult{ID: 42, SpaceID: 10, ExptID: 20, ExptRunID: 30, ItemID: 50, Status: entity.ItemRunState_Processing},
		canonicalTurn: &entity.ExptTurnResult{ID: 43, SpaceID: 10, ExptID: 20, ExptRunID: 30, ItemID: 50, TurnID: 60, Status: int32(entity.TurnRunState_Processing)}}
	f.newRuns = make(map[int64]*entity.ExptRunLog)
	f.redisClock = clock
	itemRows := []*entity.ExptItemResultRunLog{f.item}
	itemResults := []*entity.ExptItemResult{f.canonical}
	turnLogs := map[int64]*entity.ExptTurnResultRunLog{50: f.turn}
	turnResults := []*entity.ExptTurnResult{f.canonicalTurn}
	if len(twoItems) > 0 && twoItems[0] {
		itemRows = append(itemRows, &entity.ExptItemResultRunLog{ID: 44, SpaceID: 10, ExptID: 20, ExptRunID: 30, ItemID: 51, Status: int32(entity.ItemRunState_Processing)})
		itemResults = append(itemResults, &entity.ExptItemResult{ID: 45, SpaceID: 10, ExptID: 20, ExptRunID: 30, ItemID: 51, Status: entity.ItemRunState_Processing})
		turnLogs[51] = &entity.ExptTurnResultRunLog{ID: 46, SpaceID: 10, ExptID: 20, ExptRunID: 30, ItemID: 51, TurnID: 61, Status: entity.TurnRunState_Processing}
		turnResults = append(turnResults, &entity.ExptTurnResult{ID: 47, SpaceID: 10, ExptID: 20, ExptRunID: 30, ItemID: 51, TurnID: 61, Status: int32(entity.TurnRunState_Processing)})
	}
	expts, runs, items, turns, stats := rm.NewMockIExperimentRepo(ctrl), rm.NewMockIExptRunLogRepo(ctrl), rm.NewMockIExptItemResultRepo(ctrl), rm.NewMockIExptTurnResultRepo(ctrl), rm.NewMockIExptStatsRepo(ctrl)
	f.store.stores = repo.ManualCancellationRepositories{Source: f.store, Experiments: expts, Runs: runs, Items: items, Turns: turns, Stats: stats}
	expts.EXPECT().MGetByID(gomock.Any(), []int64{20}, int64(10)).DoAndReturn(func(context.Context, []int64, int64) ([]*entity.Experiment, error) {
		copy := *f.store.source.Experiment
		return []*entity.Experiment{&copy}, nil
	}).AnyTimes()
	expts.EXPECT().GetByID(gomock.Any(), int64(20), int64(10)).DoAndReturn(func(ctx context.Context, _, _ int64) (*entity.Experiment, error) {
		if f.store.beforeCommitIO != nil {
			f.store.beforeCommitIO(ctx, "experiment")
		}
		copy := *f.store.source.Experiment
		return &copy, nil
	}).AnyTimes()
	expts.EXPECT().Update(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, e *entity.Experiment) error {
		if e.LatestRunID != 0 && f.store.beforeLatestUpdate != nil {
			f.store.beforeLatestUpdate()
		}
		if ctx.Value(manualFenceHeld{}) != true {
			f.store.rowMu.Lock()
			defer f.store.rowMu.Unlock()
		}
		if e.Status == entity.ExptStatus_Terminating {
			f.terminatingWrites++
		}
		if e.Status != 0 {
			f.store.source.Experiment.Status = e.Status
		}
		if e.LatestRunID != 0 {
			f.store.source.Experiment.LatestRunID = e.LatestRunID
		}
		if e.StatusMessage != "" {
			f.store.source.Experiment.StatusMessage = e.StatusMessage
		}
		f.writes++
		return nil
	}).AnyTimes()
	runs.EXPECT().Update(gomock.Any(), int64(20), int64(30), gomock.Any()).DoAndReturn(func(_ context.Context, _, _ int64, fields map[string]any) error {
		if fields["status"] == int64(entity.ExptStatus_Terminating) {
			f.terminatingWrites++
		}
		f.store.source.RunLog.Status = fields["status"].(int64)
		f.writes++
		return nil
	}).AnyTimes()
	runs.EXPECT().Get(gomock.Any(), int64(20), int64(30)).DoAndReturn(func(ctx context.Context, _, _ int64) (*entity.ExptRunLog, error) {
		if f.store.beforeCommitIO != nil {
			f.store.beforeCommitIO(ctx, "run")
		}
		copy := *f.store.source.RunLog
		return &copy, nil
	}).AnyTimes()
	runs.EXPECT().Create(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, row *entity.ExptRunLog) error {
		copy := *row
		f.newRuns[row.ExptRunID] = &copy
		return nil
	}).AnyTimes()
	runs.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, run *entity.ExptRunLog) error {
		*f.store.source.RunLog = *run
		f.writes++
		return nil
	}).AnyTimes()
	items.EXPECT().ScanItemRunLogs(gomock.Any(), int64(20), int64(30), gomock.Any(), gomock.Any(), gomock.Any(), int64(10)).DoAndReturn(func(context.Context, int64, int64, *entity.ExptItemRunLogFilter, int64, int64, int64) ([]*entity.ExptItemResultRunLog, int64, error) {
		return itemRows, itemRows[len(itemRows)-1].ID, nil
	}).AnyTimes()
	items.EXPECT().GetItemRunLog(gomock.Any(), int64(20), int64(30), gomock.Any(), int64(10)).DoAndReturn(func(_ context.Context, _, _ int64, id, _ int64) (*entity.ExptItemResultRunLog, error) {
		for _, row := range itemRows {
			if row.ItemID == id {
				return row, nil
			}
		}
		return nil, entity.ErrHookStoreMissing
	}).AnyTimes()
	items.EXPECT().GetItemTurnResults(gomock.Any(), int64(10), int64(20), gomock.Any()).DoAndReturn(func(_ context.Context, _, _ int64, id int64) ([]*entity.ExptTurnResult, error) {
		for _, row := range turnResults {
			if row.ItemID == id {
				return []*entity.ExptTurnResult{row}, nil
			}
		}
		return nil, entity.ErrHookStoreMissing
	}).AnyTimes()
	items.EXPECT().BatchGet(gomock.Any(), int64(10), int64(20), gomock.Any()).DoAndReturn(func(_ context.Context, _, _ int64, ids []int64) ([]*entity.ExptItemResult, error) {
		var out []*entity.ExptItemResult
		for _, id := range ids {
			for _, row := range itemResults {
				if row.ItemID == id {
					out = append(out, row)
				}
			}
		}
		return out, nil
	}).AnyTimes()
	items.EXPECT().UpdateItemRunLog(gomock.Any(), int64(20), int64(30), gomock.Any(), gomock.Any(), int64(10)).DoAndReturn(func(_ context.Context, _, _ int64, ids []int64, fields map[string]any, _ int64) error {
		for _, id := range ids {
			for _, row := range itemRows {
				if row.ItemID != id {
					continue
				}
				if v, ok := fields["status"].(int32); ok {
					row.Status = v
				}
				if v, ok := fields["result_state"].(int32); ok {
					row.ResultState = v
				}
			}
		}
		f.writes++
		return nil
	}).AnyTimes()
	items.EXPECT().UpdateItemsResult(gomock.Any(), int64(10), int64(20), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _, _ int64, ids []int64, fields map[string]any) error {
		for _, id := range ids {
			for _, row := range itemResults {
				if row.ItemID == id {
					row.Status = entity.ItemRunState(fields["status"].(int32))
				}
			}
		}
		f.writes++
		return nil
	}).AnyTimes()
	items.EXPECT().ListItemResultsByExptID(gomock.Any(), int64(20), int64(10), gomock.Any(), false).Return(itemResults, int64(len(itemResults)), nil).AnyTimes()
	turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), int64(20), int64(30), gomock.Any(), int64(10)).DoAndReturn(func(_ context.Context, _, _ int64, id, _ int64) ([]*entity.ExptTurnResultRunLog, error) {
		return []*entity.ExptTurnResultRunLog{turnLogs[id]}, nil
	}).AnyTimes()
	turns.EXPECT().SaveTurnRunLogs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, rows []*entity.ExptTurnResultRunLog) error {
		if f.saveError != nil {
			return f.saveError
		}
		for _, row := range rows {
			*turnLogs[row.ItemID] = *row
		}
		f.writes++
		return nil
	}).AnyTimes()
	turns.EXPECT().SaveTurnResults(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, rows []*entity.ExptTurnResult) error {
		for _, row := range rows {
			for _, stored := range turnResults {
				if row.ItemID == stored.ItemID {
					*stored = *row
				}
			}
		}
		f.writes++
		return nil
	}).AnyTimes()
	turns.EXPECT().ListTurnResult(gomock.Any(), int64(10), int64(20), nil, gomock.Any(), false).Return(turnResults, int64(len(turnResults)), nil).AnyTimes()
	turns.EXPECT().ListTurnResultByItemIDs(gomock.Any(), int64(10), int64(20), gomock.Any(), gomock.Any(), false).Return(nil, int64(0), nil).AnyTimes()
	turns.EXPECT().UpdateTurnResults(gomock.Any(), int64(20), gomock.Any(), int64(10), gomock.Any()).DoAndReturn(func(_ context.Context, _ int64, ids []*entity.ItemTurnID, _ int64, fields map[string]any) error {
		for _, id := range ids {
			for _, row := range turnResults {
				if row.ItemID == id.ItemID && row.TurnID == id.TurnID {
					row.Status = fields["status"].(int32)
				}
			}
		}
		return nil
	}).AnyTimes()
	stats.EXPECT().ArithOperateCount(gomock.Any(), int64(20), int64(10), gomock.Any()).Return(nil).AnyTimes()
	stats.EXPECT().UpdateByExptID(gomock.Any(), int64(20), int64(10), gomock.Any()).Return(nil).AnyTimes()
	config := cm.NewMockIConfiger(ctrl)
	f.config = config
	config.EXPECT().GetMaintainerUserIDs(gomock.Any()).Return(map[string]bool{"owner": true}).AnyTimes()
	config.EXPECT().GetSchedulerAbortCtrl(gomock.Any()).Return(&entity.SchedulerAbortCtrl{}).AnyTimes()
	config.EXPECT().GetExptExecConf(gomock.Any(), int64(10)).Return(&entity.ExptExecConf{ZombieIntervalSecond: 60, ExptItemEvalConf: &entity.ExptItemEvalConf{}}).AnyTimes()
	metric, pub, quota, ids := mm.NewMockExptMetric(ctrl), em.NewMockExptEventPublisher(ctrl), rm.NewMockQuotaRepo(ctrl), idm.NewMockIIDGenerator(ctrl)
	metric.EXPECT().EmitExptExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	metric.EXPECT().EmitExptExecRun(gomock.Any(), gomock.Any()).AnyTimes()
	metric.EXPECT().EmitItemExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	quota.EXPECT().CreateOrUpdate(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Do(func(context.Context, int64, any, *entity.Session) { f.quotaRunID = 0 }).Return(nil).AnyTimes()
	pub.EXPECT().PublishExptLifecycleEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, *entity.ExptLifecycleEvent, *time.Duration, string) error {
		f.lifecycle++
		f.completed <- struct{}{}
		return nil
	}).AnyTimes()
	pub.EXPECT().PublishExptScheduleEvent(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, event *entity.ExptScheduleEvent, _ *time.Duration) error {
		if f.pubError != nil {
			return f.pubError
		}
		copy := *event
		f.events = append(f.events, &copy)
		return nil
	}).AnyTimes()
	result := service.NewExptResultService(items, turns, nil, stats, expts, metric, nil, ids, nil, nil, nil, nil, nil, nil, nil, nil, pub, nil, nil, nil, nil)
	locker := lock.NewRedisLocker(c)
	idem := idemm.NewMockIdempotentService(ctrl)
	idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil).AnyTimes()
	idem.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	aggregator := sm.NewMockExptAggrResultService(ctrl)
	aggregator.EXPECT().PublishExptAggrResultEvent(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, *entity.AggrCalculateEvent, *time.Duration) error { f.aggregations++; return nil }).AnyTimes()
	templates := sm.NewMockIExptTemplateManager(ctrl)
	templates.EXPECT().UpdateExptInfo(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Do(func(_ context.Context, _, _, _ int64, status entity.ExptStatus, _ int64, _ *int64) {
		f.templateStatus = status
	}).Return(nil).AnyTimes()
	base := service.NewExptManager(result, expts, runs, stats, items, nil, turns, config, quota, locker, idem, pub, nil, ids, metric, platestwrite.NewLatestWriteTracker(c), nil, nil, nil, nil, nil, aggregator, nil, templates, nil, nil, nil, nil, nil, nil, nil, nil)
	manager, err := service.NewExptManagerWithHookFinalization(base, service.ExptManagerFinalizationDependencies{Runs: ordinaryRecoveryRuns{}, Repository: f.store, Owners: exptinfra.NewHookFinalizationOwnerReader(c), ExecutionScope: "test", NewItemLocker: func() lock.ILocker { f.cleanupLocks++; return lock.NewRedisLocker(c) }})
	require.NoError(t, err)
	if len(twoItems) > 1 && twoItems[1] {
		// Exercise the exact production cleanup locker factory, with only storage replaced.
		stores := &HookRuntimeStores{Platform: &HookRuntimePlatform{Scope: "test"}, Runs: ordinaryRecoveryRuns{}, Finalization: f.store, Redis: c}
		wired, err := stores.manager(base)
		require.NoError(t, err)
		manager = wired.(*service.ExptMangerImpl)
	}
	scheduler := service.NewExptSchedulerSvc(manager, expts, items, turns, nil, stats, runs, nil, config, quota, locker, pub, nil, metric, result, ids, nil, nil, nil, nil, nil, nil, nil)
	scheduler, err = service.NewHookAwareExptSchedulerSvc(scheduler, ordinaryRecoveryGate{})
	require.NoError(t, err)
	f.router = &hookRuntimeRouter{IExptManager: manager, source: f.store, scheduler: scheduler}
	f.consumer = service.NewExptRecordEvalService(f.router, config, pub, items, turns, stats, expts, nil, quota, locker, idem, nil, metric, result, nil, nil, nil, nil, ids, nil, nil, nil, nil, nil, nil, nil).(*service.ExptItemEventEvalServiceImpl)
	f.app = &experimentApplication{manager: f.router, configer: config}
	return f
}

func TestOrdinaryNonManualCompletionKeepsReasonAndAggregationWithBusyItem(t *testing.T) {
	for _, mode := range []string{"debt", "system"} {
		t.Run(mode, func(t *testing.T) {
			f := newOrdinaryRecoveryFixture(t, true)
			ctx := context.Background()
			holder := lock.NewRedisLocker(f.redis)
			locked, _, cancel, err := holder.LockWithRenew(ctx, "expt_item_eval_run_lock:20:50", time.Hour, time.Hour)
			require.NoError(t, err)
			require.True(t, locked)
			defer cancel()
			cause := errors.New(mode + " ending")
			want := entity.ExptStatus_Terminated
			var completeErr error
			if mode == "debt" {
				f.config.EXPECT().GetErrRetryConf(gomock.Any(), int64(10), cause).Return(&entity.RetryConf{IsInDebt: true})
				completeErr = f.consumer.HandleEventErr(func(context.Context, *entity.ExptItemEvalEvent) error { return cause })(ctx, &entity.ExptItemEvalEvent{SpaceID: 10, ExptID: 20, ExptRunID: 30, EvalSetItemID: 51, Session: &entity.Session{UserID: "owner"}})
			} else {
				want = entity.ExptStatus_SystemTerminated
				completeErr = f.router.CompleteExpt(ctx, 20, gptr.Of(int64(30)), 10, &entity.Session{}, entity.WithStatus(want), entity.WithStatusMessage(cause.Error()))
			}
			// The consumer owning item B has not released its lock yet.
			exists, err := holder.Exists(ctx, "expt_item_eval_run_lock:20:50")
			require.NoError(t, err)
			require.True(t, exists)
			cancel()
			_, err = holder.Unlock("expt_item_eval_run_lock:20:50")
			require.NoError(t, err)
			// Replay the ordinary tick that previously hijacked the interrupted completion.
			require.NoError(t, f.router.Schedule(ctx, &entity.ExptScheduleEvent{SpaceID: 10, ExptID: 20, ExptRunID: 30, Session: &entity.Session{UserID: "owner"}}))
			assert.Equal(t, cause.Error(), f.store.source.Experiment.StatusMessage)
			assert.Equal(t, 1, f.aggregations)
			assert.Equal(t, want, f.store.source.Experiment.Status)
			assert.Zero(t, f.cleanupLocks, "nonmanual completion must not enter new item cleanup")
			assert.Zero(t, f.terminatingWrites, "only manual cancellation writes ordinary Terminating")
			if mode == "debt" {
				assert.Equal(t, int64(entity.ExptStatus_Success), f.store.source.RunLog.Status)
			} else {
				assert.Equal(t, int64(entity.ExptStatus_Processing), f.store.source.RunLog.Status)
			}
			assert.NoError(t, completeErr)
		})
	}
}

func TestOrdinaryManualCancelAPIRecoversTwoItemsAfterRetryWindow(t *testing.T) {
	f := newOrdinaryRecoveryFixture(t, true)
	ctx := session.WithCtxUser(context.Background(), &session.User{ID: "owner"})
	holder := lock.NewRedisLocker(f.redis)
	locked, _, cancel, err := holder.LockWithRenew(ctx, "expt_item_eval_run_lock:20:50", time.Hour, time.Hour)
	require.NoError(t, err)
	require.True(t, locked)
	defer cancel()
	f.pubError = errors.New("temporary MQ outage before acceptance response")
	_, err = f.app.KillExperiment(ctx, &exptpb.KillExperimentRequest{WorkspaceID: gptr.Of(int64(10)), ExptID: gptr.Of(int64(20))})
	require.ErrorIs(t, err, f.pubError)
	require.Equal(t, entity.ExptStatus_Terminating, f.store.source.Experiment.Status)
	f.pubError = nil
	f.kill(t)
	require.Len(t, f.events, 1)
	key := entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30}
	err = backoff.RetryWithElapsedTime(ctx, time.Nanosecond, func() error { return f.router.CompleteManualCancellation(ctx, key, &entity.Session{UserID: "owner"}) })
	require.ErrorIs(t, err, entity.ErrHookFinalizationUnsettled)
	f.events[0].CreatedAt = time.Now().Add(-4 * time.Minute).Unix()
	require.NoError(t, f.router.Schedule(ctx, f.events[0]))
	require.Len(t, f.events, 2)
	cancel()
	_, err = holder.Unlock("expt_item_eval_run_lock:20:50")
	require.NoError(t, err)
	require.NoError(t, f.router.Schedule(ctx, f.events[1]))
	require.Equal(t, entity.ExptStatus_Terminated, f.store.source.Experiment.Status)
	require.Equal(t, int32(2), f.store.source.RunLog.TerminatedCnt)
	require.Zero(t, f.store.source.RunLog.PendingCnt)
	require.Zero(t, f.store.source.RunLog.ProcessingCnt)
	require.Zero(t, f.aggregations, "manual cancellation preserves NoAggrCalculate")
	writes := f.writes
	require.NoError(t, f.router.Schedule(ctx, f.events[1]))
	require.Equal(t, writes, f.writes)
}

func (f *ordinaryRecoveryFixture) kill(t *testing.T) {
	t.Helper()
	ctx := session.WithCtxUser(context.Background(), &session.User{ID: "owner"})
	_, err := f.app.KillExperiment(ctx, &exptpb.KillExperimentRequest{WorkspaceID: gptr.Of(int64(10)), ExptID: gptr.Of(int64(20))})
	require.NoError(t, err)
}

type delayedManualCompletion struct{}

func TestManualCancellationProductionItemOwnerCannotUnlockSuccessor(t *testing.T) {
	f := newOrdinaryRecoveryFixture(t, false, true)
	ctx := context.Background()
	key := entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30}
	f.store.source.Experiment.Status, f.store.source.RunLog.Status = entity.ExptStatus_Terminating, int64(entity.ExptStatus_Terminating)
	paused, resume, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	var once sync.Once
	f.store.afterFence = func(ctx context.Context) {
		if ctx.Value(delayedManualCompletion{}) == true {
			once.Do(func() { close(paused); <-resume })
		}
	}
	go func() {
		done <- f.router.CompleteManualCancellation(context.WithValue(ctx, delayedManualCompletion{}, true), key, &entity.Session{})
	}()
	select {
	case <-paused:
	case <-time.After(5 * time.Second):
		t.Fatal("item archive did not commit")
	}
	require.NoError(t, f.router.CompleteManualCancellation(ctx, key, &entity.Session{}))
	require.NoError(t, f.router.IExptManager.LogRun(ctx, 20, 31, entity.EvaluationModeRetryAll, 10, nil, &entity.Session{UserID: "creator"}))
	f.store.source.Experiment.Status = entity.ExptStatus_Processing
	f.canonical.ExptRunID, f.canonical.Status = 31, entity.ItemRunState_Processing
	f.canonicalTurn.ExptRunID, f.canonicalTurn.Status = 31, int32(entity.TurnRunState_Processing)
	f.redisClock.FastForward(6 * time.Second)
	b := lock.NewRedisLockerWithHolder(f.redis, "evaluation")
	locked, _, cancel, err := b.LockWithRenew(ctx, "expt_item_eval_run_lock:20:50", time.Hour, time.Hour)
	require.NoError(t, err)
	require.True(t, locked)
	defer cancel()
	close(resume)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("late cleaner did not finish")
	}
	owner, err := f.redis.Get(ctx, "expt_item_eval_run_lock:20:50").Result()
	assert.NoError(t, err)
	assert.Equal(t, "evaluation", owner)
	assert.Equal(t, entity.ExptStatus_Processing, f.store.source.Experiment.Status)
	assert.Equal(t, int64(31), f.canonical.ExptRunID)
}

func TestManualCancellationCannotDeleteNewCreationOwnerAfterLeaseExpiry(t *testing.T) {
	f := newOrdinaryRecoveryFixture(t)
	ctx := context.Background()
	f.store.source.Experiment.LatestRunID = 0
	f.store.source.RunLog = nil
	require.NoError(t, f.router.IExptManager.LogRun(ctx, 20, 30, entity.EvaluationModeSubmit, 10, nil, &entity.Session{UserID: "creator"}))
	f.store.source.RunLog = f.newRuns[30]
	f.store.source.Experiment.Status, f.store.source.RunLog.Status = entity.ExptStatus_Terminating, int64(entity.ExptStatus_Terminating)
	f.item.Status, f.item.ResultState = int32(entity.ItemRunState_Terminal), int32(entity.ExptItemResultStateResulted)
	f.turn.Status, f.canonical.Status, f.canonicalTurn.Status = entity.TurnRunState_Terminal, entity.ItemRunState_Terminal, int32(entity.TurnRunState_Terminal)
	paused, resume, bAtParent := make(chan struct{}), make(chan struct{}), make(chan struct{})
	cancelDone, createDone := make(chan error, 1), make(chan error, 1)
	var once sync.Once
	f.store.beforeCommitIO = func(ctx context.Context, point string) {
		if ctx.Value(delayedManualCompletion{}) == true && point == "experiment" {
			once.Do(func() { close(paused); <-resume })
		}
	}
	go func() {
		cancelDone <- f.router.CompleteManualCancellation(context.WithValue(ctx, delayedManualCompletion{}, true), entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30}, &entity.Session{})
	}()
	select {
	case <-paused:
	case <-time.After(5 * time.Second):
		t.Fatal("cancellation did not hold parent")
	}
	f.redisClock.FastForward(61 * time.Second)
	f.store.beforeLatestUpdate = func() { close(bAtParent) }
	go func() {
		createDone <- f.router.IExptManager.LogRun(ctx, 20, 31, entity.EvaluationModeRetryAll, 10, nil, &entity.Session{UserID: "creator"})
	}()
	select {
	case <-bAtParent:
	case <-time.After(5 * time.Second):
		t.Fatal("creation did not reach shared parent")
	}
	require.Equal(t, int64(30), f.store.source.Experiment.LatestRunID, "creation must wait for the same parent row")
	owner, err := f.redis.Get(ctx, "expt_run_mutex_lock:20").Result()
	require.NoError(t, err)
	close(resume)
	require.NoError(t, <-cancelDone)
	require.NoError(t, <-createDone)
	require.Equal(t, int64(31), f.store.source.Experiment.LatestRunID)
	after, err := f.redis.Get(ctx, "expt_run_mutex_lock:20").Result()
	assert.NoError(t, err)
	assert.Equal(t, owner, after, "A must not delete B's lock even if A's old lease expired")
}

func TestManualCancellationStaleCompletionCannotTouchSuccessor(t *testing.T) {
	for _, phase := range []string{"run", "experiment"} {
		t.Run(phase, func(t *testing.T) {
			f := newOrdinaryRecoveryFixture(t)
			ctx := context.Background()
			key := entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30}
			f.store.source.Experiment.Status = entity.ExptStatus_Terminating
			f.store.source.Experiment.ExptTemplateMeta = &entity.ExptTemplateMeta{ID: 70}
			f.store.source.RunLog.Status = int64(entity.ExptStatus_Terminating)
			f.item.Status, f.item.ResultState = int32(entity.ItemRunState_Terminal), int32(entity.ExptItemResultStateResulted)
			f.turn.Status, f.canonical.Status, f.canonicalTurn.Status = entity.TurnRunState_Terminal, entity.ItemRunState_Terminal, int32(entity.TurnRunState_Terminal)
			paused, resume, done := make(chan struct{}), make(chan struct{}), make(chan error, 1)
			var once sync.Once
			f.store.beforeCommitIO = func(ctx context.Context, point string) {
				if ctx.Value(delayedManualCompletion{}) == true && (point == phase || point == "fence") {
					once.Do(func() { close(paused); <-resume })
				}
			}
			go func() {
				done <- f.router.CompleteManualCancellation(context.WithValue(ctx, delayedManualCompletion{}, true), key, &entity.Session{UserID: "C2"})
			}()
			select {
			case <-paused:
			case <-time.After(5 * time.Second):
				t.Fatal("C2 did not reach commit boundary")
			}
			require.NoError(t, f.router.CompleteManualCancellation(ctx, key, &entity.Session{UserID: "C1"}))
			// Real creation acquires the existing experiment run mutex and publishes Latest B.
			require.NoError(t, f.router.IExptManager.LogRun(ctx, 20, 31, entity.EvaluationModeRetryAll, 10, nil, &entity.Session{UserID: "creator"}))
			require.Equal(t, int64(31), f.store.source.Experiment.LatestRunID)
			require.NotNil(t, f.newRuns[31])
			// Stateful storage boundary for B's normal startup/reset and quota admission.
			f.store.source.Experiment.Status = entity.ExptStatus_Processing
			f.canonical.ExptRunID, f.canonical.Status = 31, entity.ItemRunState_Processing
			f.canonicalTurn.ExptRunID, f.canonicalTurn.Status, f.canonicalTurn.TargetResultID = 31, int32(entity.TurnRunState_Processing), 901
			f.quotaRunID, f.templateStatus = 31, entity.ExptStatus_Processing
			owner, err := f.redis.Get(ctx, "expt_run_mutex_lock:20").Result()
			require.NoError(t, err)
			lockValues := map[string]string{"expt_run_mutex_lock:20": owner}
			beforeEvents := f.lifecycle
			close(resume)
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("C2 did not finish")
			}
			assert.Equal(t, entity.ExptStatus_Processing, f.store.source.Experiment.Status)
			assert.Equal(t, int64(31), f.canonical.ExptRunID)
			assert.Equal(t, entity.ItemRunState_Processing, f.canonical.Status)
			assert.Equal(t, int32(entity.TurnRunState_Processing), f.canonicalTurn.Status)
			assert.Equal(t, int64(901), f.canonicalTurn.TargetResultID)
			assert.Equal(t, int64(31), f.quotaRunID)
			assert.Equal(t, entity.ExptStatus_Processing, f.templateStatus)
			assert.Equal(t, beforeEvents, f.lifecycle)
			for key, value := range lockValues {
				got, err := f.redis.Get(ctx, key).Result()
				assert.NoError(t, err)
				assert.Equal(t, value, got)
			}
		})
	}
}

func exhaustOrdinaryApplicationRetry(t *testing.T, f *ordinaryRecoveryFixture) {
	t.Helper()
	ctx := context.Background()
	key := entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30}
	handled, err := f.router.StartManualCancellation(ctx, key, &entity.Session{UserID: "owner"})
	require.NoError(t, err)
	require.True(t, handled)
	// Same production retry engine and completion calls as KillExperiment; shrink only
	// the elapsed budget so the holder outlives it without sleeping three minutes.
	err = backoff.RetryWithElapsedTime(ctx, time.Nanosecond, func() error {
		return f.router.CompleteManualCancellation(ctx, key, &entity.Session{UserID: "owner"})
	})
	require.Error(t, err)
}

func TestOrdinaryTerminationRecoveryAfterApplicationWindow(t *testing.T) {
	f := newOrdinaryRecoveryFixture(t)
	ctx := context.Background()
	holder := lock.NewRedisLocker(f.redis)
	locked, _, cancel, err := holder.LockWithRenew(ctx, "expt_item_eval_run_lock:20:50", time.Hour, time.Hour)
	require.NoError(t, err)
	require.True(t, locked)
	defer cancel()
	exhaustOrdinaryApplicationRetry(t, f)
	require.Equal(t, int64(entity.ExptStatus_Terminating), f.store.source.RunLog.Status)
	// An already queued scheduler tick survives an exhausted application retry budget.
	event := &entity.ExptScheduleEvent{SpaceID: 10, ExptID: 20, ExptRunID: 30, ExptRunMode: entity.EvaluationModeSubmit, CreatedAt: time.Now().Add(-4 * time.Minute).Unix(), Session: &entity.Session{UserID: "owner"}}
	before := len(f.events)
	require.NoError(t, f.router.Schedule(ctx, event))
	require.Greater(t, len(f.events), before, "busy cancellation needs a durable continuation after the 3-minute application window")
	require.Equal(t, int32(entity.ItemRunState_Processing), f.item.Status)
	require.Equal(t, entity.ExptStatus_Terminating, f.store.source.Experiment.Status)
	cancel()
	_, err = holder.Unlock("expt_item_eval_run_lock:20:50")
	require.NoError(t, err)
	f.saveError = errors.New("temporary turn persistence failure")
	require.NoError(t, f.router.Schedule(ctx, f.events[len(f.events)-1]))
	require.Equal(t, int32(entity.ItemRunState_Processing), f.item.Status)
	f.saveError = nil
	require.NoError(t, f.router.Schedule(ctx, f.events[len(f.events)-1]))
	require.Equal(t, entity.ExptStatus_Terminated, f.store.source.Experiment.Status)
	require.Equal(t, int64(entity.ExptStatus_Terminated), f.store.source.RunLog.Status)
	require.Zero(t, f.store.source.RunLog.ProcessingCnt)
	require.Equal(t, int32(1), f.store.source.RunLog.TerminatedCnt)
	require.Equal(t, int32(entity.ExptItemResultStateResulted), f.item.ResultState)
	require.Equal(t, entity.ItemRunState_Terminal, f.canonical.Status)
	writes, lifecycle := f.writes, f.lifecycle
	require.NoError(t, f.router.Schedule(ctx, event))
	require.Equal(t, writes, f.writes)
	require.Equal(t, lifecycle, f.lifecycle)
}

func TestOrdinaryTerminationRepeatedCancelCanResume(t *testing.T) {
	f := newOrdinaryRecoveryFixture(t)
	f.store.source.Experiment.Status = entity.ExptStatus_Terminating
	f.store.source.RunLog.Status = int64(entity.ExptStatus_Terminating)
	f.kill(t)
	require.NotEmpty(t, f.events)
	f.kill(t)
	require.Len(t, f.events, 2)
	require.NoError(t, f.router.Schedule(context.Background(), f.events[0]))
	writes := f.writes
	require.NoError(t, f.router.Schedule(context.Background(), f.events[1]))
	require.Equal(t, writes, f.writes)
	require.Equal(t, entity.ExptStatus_Terminated, f.store.source.Experiment.Status)
}

func TestOrdinaryTerminationInitialCancelQueuesRecovery(t *testing.T) {
	f := newOrdinaryRecoveryFixture(t)
	f.kill(t)
	require.Len(t, f.events, 1, "successful API acceptance must already have published recovery")
	require.Equal(t, int64(30), f.events[0].ExptRunID)
	select {
	case <-f.completed:
	case <-time.After(5 * time.Second):
		t.Fatal("initial cancellation did not complete")
	}
	require.Equal(t, entity.ExptStatus_Terminated, f.store.source.Experiment.Status)
}

func TestOrdinaryTerminationRecoversWithinApplicationWindow(t *testing.T) {
	f := newOrdinaryRecoveryFixture(t)
	ctx := context.Background()
	holder := lock.NewRedisLocker(f.redis)
	locked, _, cancel, err := holder.LockWithRenew(ctx, "expt_item_eval_run_lock:20:50", time.Hour, time.Hour)
	require.NoError(t, err)
	require.True(t, locked)
	defer cancel()
	key := entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30}
	handled, err := f.router.StartManualCancellation(ctx, key, &entity.Session{UserID: "owner"})
	require.NoError(t, err)
	require.True(t, handled)
	attempts := 0
	err = backoff.RetryWithElapsedTime(ctx, time.Second, func() error {
		attempts++
		err := f.router.CompleteManualCancellation(ctx, key, &entity.Session{UserID: "owner"})
		if attempts == 1 {
			require.Error(t, err)
			cancel()
			_, unlockErr := holder.Unlock("expt_item_eval_run_lock:20:50")
			require.NoError(t, unlockErr)
		}
		return err
	})
	require.NoError(t, err)
	require.Equal(t, 2, attempts)
	require.Equal(t, entity.ExptStatus_Terminated, f.store.source.Experiment.Status)
}

func TestOrdinaryTerminationRecoveryIsolation(t *testing.T) {
	for _, scenario := range []string{"superseded", "wrong-space", "deleted", "success", "managed", "completed-item"} {
		t.Run(scenario, func(t *testing.T) {
			f := newOrdinaryRecoveryFixture(t)
			f.store.source.Experiment.Status, f.store.source.RunLog.Status = entity.ExptStatus_Terminating, int64(entity.ExptStatus_Terminating)
			event := &entity.ExptScheduleEvent{SpaceID: 10, ExptID: 20, ExptRunID: 30, Session: &entity.Session{UserID: "owner"}}
			switch scenario {
			case "superseded":
				f.store.source.Experiment.LatestRunID = 31
			case "wrong-space":
				event.SpaceID = 11
			case "deleted":
				f.store.err = entity.ErrHookStoreMissing
			case "success":
				f.store.source.RunLog.Status = int64(entity.ExptStatus_Success)
			case "managed":
				f.store.source.Managed = true
			case "completed-item":
				f.item.Status, f.item.ResultState = int32(entity.ItemRunState_Success), int32(entity.ExptItemResultStateResulted)
				f.turn.Status, f.turn.TargetResultID = entity.TurnRunState_Success, 701
				f.canonical.Status, f.canonicalTurn.Status = entity.ItemRunState_Success, int32(entity.TurnRunState_Success)
				f.canonicalTurn.TargetResultID = 701
			}
			_ = f.router.Schedule(context.Background(), event)
			if scenario == "completed-item" {
				require.Equal(t, entity.ExptStatus_Terminated, f.store.source.Experiment.Status)
				require.Equal(t, entity.ItemRunState_Success, f.canonical.Status)
				require.Equal(t, int64(701), f.canonicalTurn.TargetResultID)
				require.Equal(t, entity.TurnRunState_Success, f.turn.Status)
			} else {
				require.Zero(t, f.writes)
				require.Empty(t, f.events)
			}
		})
	}
}

func TestOrdinaryTerminationRecoveryPublicationFailureAndReadRetry(t *testing.T) {
	f := newOrdinaryRecoveryFixture(t)
	f.store.source.Experiment.Status, f.store.source.RunLog.Status = entity.ExptStatus_Terminating, int64(entity.ExptStatus_Terminating)
	f.pubError = errors.New("MQ temporarily unavailable")
	ctx := session.WithCtxUser(context.Background(), &session.User{ID: "owner"})
	req := &exptpb.KillExperimentRequest{WorkspaceID: gptr.Of(int64(10)), ExptID: gptr.Of(int64(20))}
	_, err := f.app.KillExperiment(ctx, req)
	require.ErrorIs(t, err, f.pubError)
	require.Empty(t, f.events)
	f.pubError = nil
	f.kill(t)
	f.store.err = errors.New("DB temporarily unavailable")
	require.Error(t, f.router.Schedule(ctx, f.events[0]))
	f.store.err = nil
	require.NoError(t, f.router.Schedule(ctx, f.events[0]))
	require.Equal(t, entity.ExptStatus_Terminated, f.store.source.Experiment.Status)
}

func TestOrdinaryTerminationRepeatedCancelStillAuthorizes(t *testing.T) {
	f := newOrdinaryRecoveryFixture(t)
	f.store.source.Experiment.Status, f.store.source.RunLog.Status = entity.ExptStatus_Terminating, int64(entity.ExptStatus_Terminating)
	auth := rpcm.NewMockIAuthProvider(gomock.NewController(t))
	auth.EXPECT().AuthorizationWithoutSPI(gomock.Any(), gomock.Any()).Return(errors.New("permission denied"))
	f.app.auth = auth
	_, err := f.app.KillExperiment(context.Background(), &exptpb.KillExperimentRequest{WorkspaceID: gptr.Of(int64(10)), ExptID: gptr.Of(int64(20))})
	require.Error(t, err)
	require.Empty(t, f.events)
	require.Zero(t, f.writes)
}
