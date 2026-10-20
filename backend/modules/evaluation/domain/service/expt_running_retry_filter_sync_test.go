// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	lwtmocks "github.com/coze-dev/coze-loop/backend/infra/platestwrite/mocks"
	infraredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	idemmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

// Removing the public item's mutex exposes a source read while requeue owns it.
func TestRunningRetryFilterSync_BusyDoesNotReadPayload(t *testing.T) {
	ctrl := gomock.NewController(t)
	server := miniredis.RunT(t)
	client, err := infraredis.NewClient(&redis.Options{Addr: server.Addr()})
	require.NoError(t, err)
	raw, _ := infraredis.Unwrap(client)
	t.Cleanup(func() { require.NoError(t, raw.Close()) })
	const space, expt, item = int64(7384603622000000001), int64(7384603622000000002), int64(7384603622000000011)
	key := fmt.Sprintf("expt_item_eval_run_lock:%d:%d", expt, item)
	holder := filterRealTestLocker(client)
	ok, err := holder.Lock(context.Background(), key, time.Minute)
	require.NoError(t, err)
	require.True(t, ok)
	t.Cleanup(func() { _, _ = holder.Unlock(key) })
	turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
	var reads atomic.Int32
	turns.EXPECT().ListTurnResultByItemIDs(gomock.Any(), space, expt, gomock.Any(), gomock.Any(), false).DoAndReturn(func(context.Context, int64, int64, []int64, entity.Page, bool) ([]*entity.ExptTurnResult, int64, error) {
		reads.Add(1)
		return nil, 0, nil
	}).AnyTimes()
	svc := &ExptResultServiceImpl{ExptTurnResultRepo: turns, resultPublishLocker: func() lock.ILocker { return filterRealTestLocker(client) }}
	err = svc.UpsertExptTurnResultFilter(context.Background(), space, expt, []int64{item})
	require.True(t, errno.NeedMQRetry(err), "held item must broker retry, got %v", err)
	require.Zero(t, reads.Load(), "current payload was read before acquiring item mutex")
}

// A Resulted row must still pass through final calibration, and an invisible
// asynchronous write must leave it pending rather than claim completion.
func TestRunningRetryFilterSync_ResultedNeedsCalibration(t *testing.T) {
	f := newLateWriteFixture(t, "rq1")
	require.NoError(t, f.conn.Exec("UPDATE expt_item_result SET status=? WHERE space_id=? AND expt_id=? AND item_id=?", entity.ItemRunState_Success, f.space, f.expt, f.item).Error)
	require.NoError(t, f.conn.Exec("UPDATE expt_item_result_run_log SET status=?,result_state=? WHERE space_id=? AND expt_id=? AND item_id=?", entity.ItemRunState_Success, entity.ExptItemResultStateResulted, f.space, f.expt, f.item).Error)
	svc := &ExptResultServiceImpl{ExptItemResultRepo: f.itemRepo()}
	port, ok := any(svc).(interface {
		ReconcileManualRetryFilterBeforeFinish(context.Context, int64, int64, int64) (bool, error)
	})
	require.True(t, ok, "final calibration entry is missing; Resulted is excluded from Logged scans")
	if ok {
		_, err := port.ReconcileManualRetryFilterBeforeFinish(context.Background(), f.space, f.expt, f.run)
		require.True(t, errno.NeedMQRetry(err), "missing filter dependencies must keep scheduler retrying")
	}
}

// This test catches a calibration that only compares scores/status while an
// old per-item snapshot or target field remains materialized.
func TestRunningRetryFilterSync_FullProjectionMismatch(t *testing.T) {
	ctrl := gomock.NewController(t)
	turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
	items := repomocks.NewMockIExptItemResultRepo(ctrl)
	filters := repomocks.NewMockIExptTurnResultFilterRepo(ctrl)
	experiments := repomocks.NewMockIExperimentRepo(ctrl)
	annotations := repomocks.NewMockIExptAnnotateRepo(ctrl)
	records := svcmocks.NewMockEvaluatorRecordService(ctrl)
	target := svcmocks.NewMockIEvalTargetService(ctrl)
	now := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	turns.EXPECT().ListTurnResultByItemIDs(gomock.Any(), int64(3), int64(1), []int64{10}, gomock.Any(), false).Return([]*entity.ExptTurnResult{{ID: 101, ItemID: 10, ItemVersionID: 222, TurnID: 0, WeightedScore: gptr.Of(0.0)}}, int64(1), nil)
	items.EXPECT().BatchGet(gomock.Any(), int64(3), int64(1), []int64{10}).Return([]*entity.ExptItemResult{{ItemID: 10, Status: entity.ItemRunState_Success}}, nil)
	filters.EXPECT().GetExptTurnResultFilterKeyMappings(gomock.Any(), int64(3), int64(1)).Return(nil, nil)
	experiments.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(&entity.Experiment{ID: 1, SpaceID: 3, StartAt: &now, EvalSetID: 55, EvalSetVersionID: 66}, nil)
	turns.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), int64(3), []int64{101}).Return(nil, nil)
	records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), gomock.Any(), false, false).Return(nil, nil)
	target.EXPECT().BatchGetRecordByIDs(gomock.Any(), int64(3), gomock.Any()).Return(nil, nil)
	annotations.EXPECT().GetExptTurnAnnotateRecordRefsByTurnResultIDs(gomock.Any(), int64(3), []int64{101}).Return(nil, nil)
	annotations.EXPECT().GetAnnotateRecordsByIDs(gomock.Any(), int64(3), gomock.Any()).Return(nil, nil)
	filters.EXPECT().GetByExptIDItemIDs(gomock.Any(), "3", "1", "2026-10-20", []string{"10"}).Return([]*entity.ExptTurnResultFilterEntity{{SpaceID: 3, ExptID: 1, ItemID: 10, TurnID: 0, Status: entity.ItemRunState_Success, ItemVersionID: 111, EvalSetID: 55, EvalSetVersionID: 66}}, nil)
	sink := &fullFilterTestSink{IExptTurnResultFilterRepo: filters}
	svc := ExptResultServiceImpl{ExptTurnResultRepo: turns, ExptItemResultRepo: items, ExperimentRepo: experiments, ExptAnnotateRepo: annotations, evaluatorRecordService: records, evalTargetService: target, exptTurnResultFilterRepo: sink}
	equal, err := svc.materializedFilterMatchesLocked(context.Background(), 3, 1, 10)
	require.NoError(t, err)
	require.False(t, equal, "different item snapshot must keep calibration pending")
}

type fullFilterTestSink struct{ repo.IExptTurnResultFilterRepo }

func (s *fullFilterTestSink) GetMaterializedByExptIDItemIDs(ctx context.Context, space, expt, date string, ids []string) ([]*entity.ExptTurnResultFilterEntity, error) {
	return s.GetByExptIDItemIDs(ctx, space, expt, date, ids)
}

// A producer already owns the execution mutex; using the public refresh here
// would return busy and silently skip the real CK write.
func TestRunningRetryFilterSync_ProducerDoesNotReenter(t *testing.T) {
	ctrl := gomock.NewController(t)
	server := miniredis.RunT(t)
	client, err := infraredis.NewClient(&redis.Options{Addr: server.Addr()})
	require.NoError(t, err)
	raw, _ := infraredis.Unwrap(client)
	t.Cleanup(func() { _ = raw.Close() })
	items := repomocks.NewMockIExptItemResultRepo(ctrl)
	turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
	var reads atomic.Int32
	turns.EXPECT().ListTurnResultByItemIDs(gomock.Any(), int64(3), int64(1), []int64{10}, gomock.Any(), false).DoAndReturn(func(context.Context, int64, int64, []int64, entity.Page, bool) ([]*entity.ExptTurnResult, int64, error) {
		reads.Add(1)
		return nil, 0, nil
	}).AnyTimes()
	items.EXPECT().GetCurrentRequest(gomock.Any(), int64(3), int64(1), int64(2), int64(10)).Return(&entity.ExptItemCurrentRequest{ItemID: 10, RunID: 2, Mode: entity.EvaluationModeSubmit, Status: entity.ItemRunState_Queueing}, nil)
	items.EXPECT().MarkProcessingIfCurrent(gomock.Any(), int64(3), int64(1), int64(2), int64(10), "").Return(true, nil)
	publisher := eventmocks.NewMockExptEventPublisher(ctrl)
	publisher.EXPECT().BatchPublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	config := componentmocks.NewMockIConfiger(ctrl)
	config.EXPECT().GetExptExecConf(gomock.Any(), int64(3)).Return(&entity.ExptExecConf{})
	result := &ExptResultServiceImpl{ExptTurnResultRepo: turns, resultPublishLocker: func() lock.ILocker { return filterRealTestLocker(client) }}
	scheduler := &ExptSchedulerImpl{ExptItemResultRepo: items, Publisher: publisher, Configer: config, ResultSvc: result}
	require.NoError(t, scheduler.submitCurrentItem(context.Background(), &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2}, &entity.ExptEvalItem{ItemID: 10}, filterRealTestLocker(client)))
	require.Equal(t, int32(1), reads.Load(), "producer must refresh under its existing lock without reacquiring it")
}

// Manual mapping refresh must retain its event and also enter the same locked
// reconstruction path. The old method ignores the supplied item IDs entirely.
func TestRunningRetryFilterSync_ManualRefreshUsesItemLock(t *testing.T) {
	ctrl := gomock.NewController(t)
	server := miniredis.RunT(t)
	client, err := infraredis.NewClient(&redis.Options{Addr: server.Addr()})
	require.NoError(t, err)
	raw, _ := infraredis.Unwrap(client)
	t.Cleanup(func() { _ = raw.Close() })
	experiments := repomocks.NewMockIExperimentRepo(ctrl)
	annotations := repomocks.NewMockIExptAnnotateRepo(ctrl)
	filters := repomocks.NewMockIExptTurnResultFilterRepo(ctrl)
	turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
	publisher := eventmocks.NewMockExptEventPublisher(ctrl)
	lwt := lwtmocks.NewMockILatestWriteTracker(ctrl)
	lwt.EXPECT().CheckWriteFlagByID(gomock.Any(), gomock.Any(), int64(1)).Return(false)
	experiments.EXPECT().MGetByID(gomock.Any(), []int64{1}, int64(3)).Return([]*entity.Experiment{{ID: 1, SpaceID: 3}}, nil)
	annotations.EXPECT().GetExptTurnResultTagRefs(gomock.Any(), int64(1), int64(3)).Return(nil, nil)
	filters.EXPECT().InsertExptTurnResultFilterKeyMappings(gomock.Any(), gomock.Any()).Return(nil)
	publisher.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	var reads atomic.Int32
	turns.EXPECT().ListTurnResultByItemIDs(gomock.Any(), int64(3), int64(1), []int64{10}, gomock.Any(), false).DoAndReturn(func(context.Context, int64, int64, []int64, entity.Page, bool) ([]*entity.ExptTurnResult, int64, error) {
		reads.Add(1)
		require.True(t, server.Exists("expt_item_eval_run_lock:1:10"))
		return nil, 0, nil
	}).AnyTimes()
	svc := ExptResultServiceImpl{ExperimentRepo: experiments, ExptAnnotateRepo: annotations, ExptTurnResultRepo: turns, exptTurnResultFilterRepo: filters, publisher: publisher, lwt: lwt, resultPublishLocker: func() lock.ILocker { return filterRealTestLocker(client) }}
	require.NoError(t, svc.ManualUpsertExptTurnResultFilter(context.Background(), 3, 1, []int64{10}))
	require.Equal(t, int32(1), reads.Load(), "manual entry never refreshed supplied candidates")
}

func TestRunningRetryFilterSync_IndependentItems(t *testing.T) {
	ctrl := gomock.NewController(t)
	server := miniredis.RunT(t)
	client, err := infraredis.NewClient(&redis.Options{Addr: server.Addr()})
	require.NoError(t, err)
	raw, _ := infraredis.Unwrap(client)
	t.Cleanup(func() { _ = raw.Close() })
	turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
	entered := make(chan struct{})
	release := make(chan struct{})
	turns.EXPECT().ListTurnResultByItemIDs(gomock.Any(), int64(3), int64(1), []int64{10}, gomock.Any(), false).DoAndReturn(func(context.Context, int64, int64, []int64, entity.Page, bool) ([]*entity.ExptTurnResult, int64, error) {
		close(entered)
		<-release
		return nil, 0, nil
	})
	turns.EXPECT().ListTurnResultByItemIDs(gomock.Any(), int64(3), int64(1), []int64{11}, gomock.Any(), false).Return(nil, int64(0), nil)
	svc := ExptResultServiceImpl{ExptTurnResultRepo: turns, resultPublishLocker: func() lock.ILocker { return filterRealTestLocker(client) }}
	first := make(chan error, 1)
	go func() { first <- svc.UpsertExptTurnResultFilter(context.Background(), 3, 1, []int64{10}) }()
	<-entered
	second := make(chan error, 1)
	go func() { second <- svc.UpsertExptTurnResultFilter(context.Background(), 3, 1, []int64{11}) }()
	select {
	case err := <-second:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Error("different item blocked behind experiment mutex")
	}
	close(release)
	require.NoError(t, <-first)
}

// All offline modes must carry the calibration result into the real completion
// branch. Broker retries bypass the scheduler's business failure completion.
func TestRunningRetryFilterSync_OfflineFinishBlocksUntilMaterialized(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeFailRetry, entity.EvaluationModeRetryAll, entity.EvaluationModeRetryItems} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			manager := svcmocks.NewMockIExptManager(ctrl)
			items := repomocks.NewMockIExptItemResultRepo(ctrl)
			idem := idemmocks.NewMockIdempotentService(ctrl)
			config := componentmocks.NewMockIConfiger(ctrl)
			baseResult := svcmocks.NewMockExptResultService(ctrl)
			result := &finishCalibrationTestResult{ExptResultService: baseResult}
			items.EXPECT().ScanItemRunLogs(gomock.Any(), int64(1), int64(2), gomock.Any(), int64(0), int64(1), int64(3)).Return(nil, int64(0), nil).AnyTimes()
			idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil).AnyTimes()
			var complete atomic.Int32
			manager.EXPECT().CompleteRun(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, int64, int64, int64, *entity.Session, ...entity.CompleteExptOptionFn) error {
				complete.Add(1)
				return nil
			}).AnyTimes()
			manager.EXPECT().CompleteExpt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			config.EXPECT().GetExptExecConf(gomock.Any(), int64(3)).Return(&entity.ExptExecConf{}).AnyTimes()
			idem.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
			event := &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2, ExptRunMode: mode}
			expt := &entity.Experiment{ExptType: entity.ExptType_Offline}
			var end func(context.Context, *entity.ExptScheduleEvent, *entity.Experiment, int, int) (bool, error)
			switch mode {
			case entity.EvaluationModeSubmit:
				end = (&ExptSubmitExec{manager: manager, idem: idem, configer: config, exptItemResultRepo: items, resultSvc: result}).ExptEnd
			case entity.EvaluationModeFailRetry:
				end = (&ExptFailRetryExec{manager: manager, idem: idem, configer: config, exptItemResultRepo: items, resultSvc: result}).ExptEnd
			case entity.EvaluationModeRetryAll:
				end = (&ExptRetryAllExec{manager: manager, idem: idem, configer: config, exptItemResultRepo: items, resultSvc: result}).ExptEnd
			case entity.EvaluationModeRetryItems:
				logs := repomocks.NewMockIExptRunLogRepo(ctrl)
				logs.EXPECT().Get(gomock.Any(), int64(1), int64(2)).Return(&entity.ExptRunLog{}, nil).AnyTimes()
				manager.EXPECT().LockCompletingRun(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				manager.EXPECT().UnlockCompletingRun(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
				end = (&ExptRetryItemsExec{manager: manager, idem: idem, configer: config, exptItemResultRepo: items, exptRunLogRepo: logs, resultSvc: result}).ExptEnd
			}
			for _, fault := range []error{errors.New("filter read unavailable"), nil} {
				result.err = fault
				result.ready = false
				scheduler := &ExptSchedulerImpl{Manager: manager}
				err := scheduler.HandleEventErr(func(ctx context.Context, event *entity.ExptScheduleEvent) error {
					_, err := end(ctx, event, expt, 0, 0)
					return err
				})(context.Background(), event)
				require.True(t, errno.NeedMQRetry(err))
				require.Zero(t, complete.Load(), "unmaterialized filter completed offline run")
			}
			result.ready = true
			result.err = nil
			_, err := end(context.Background(), event, expt, 0, 0)
			require.NoError(t, err)
			require.Equal(t, int32(1), complete.Load())
		})
	}
}

type finishCalibrationTestResult struct {
	ExptResultService
	ready      bool
	err        error
	panicValue any
}

func (r *finishCalibrationTestResult) ReconcileManualRetryFilterBeforeFinish(context.Context, int64, int64, int64) (bool, error) {
	if r.panicValue != nil {
		panic(r.panicValue)
	}
	return r.ready, r.err
}

func TestRunningRetryFilterSync_CalibrationPanicBrokerRetry(t *testing.T) {
	ctrl := gomock.NewController(t)
	manager := svcmocks.NewMockIExptManager(ctrl)
	idem := idemmocks.NewMockIdempotentService(ctrl)
	idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil)
	result := &finishCalibrationTestResult{panicValue: "filter runtime failure"}
	base := &exptBaseExec{Manager: manager, idem: idem, resultSvc: result}
	scheduler := &ExptSchedulerImpl{Manager: manager}
	err := scheduler.HandleEventErr(func(ctx context.Context, event *entity.ExptScheduleEvent) error {
		return base.exptEnd(ctx, event, &entity.Experiment{ExptType: entity.ExptType_Offline})
	})(context.Background(), &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2, ExptRunMode: entity.EvaluationModeSubmit})
	require.True(t, errno.NeedMQRetry(err), "calibration runtime failure must never enter CompleteRun/Failed branch")
}

func TestRunningRetryFilterSync_LeaseLostBeforeSave(t *testing.T) {
	ctrl := gomock.NewController(t)
	server := miniredis.RunT(t)
	client, err := infraredis.NewClient(&redis.Options{Addr: server.Addr()})
	require.NoError(t, err)
	raw, _ := infraredis.Unwrap(client)
	t.Cleanup(func() { _ = raw.Close() })
	base := filterRealTestLocker(client)
	lockCtx, cancel := context.WithCancel(context.Background())
	locker := lockmocks.NewMockILocker(ctrl)
	locker.EXPECT().LockWithRenew(gomock.Any(), "expt_item_eval_run_lock:1:10", gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, string, time.Duration, time.Duration) (bool, context.Context, func(), error) {
		ok, err := base.Lock(lockCtx, "expt_item_eval_run_lock:1:10", time.Minute)
		return ok, lockCtx, cancel, err
	})
	locker.EXPECT().Unlock("expt_item_eval_run_lock:1:10").DoAndReturn(base.Unlock)
	turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
	turns.EXPECT().ListTurnResultByItemIDs(gomock.Any(), int64(3), int64(1), []int64{10}, gomock.Any(), false).DoAndReturn(func(context.Context, int64, int64, []int64, entity.Page, bool) ([]*entity.ExptTurnResult, int64, error) {
		cancel()
		return nil, 0, nil
	})
	svc := ExptResultServiceImpl{ExptTurnResultRepo: turns, resultPublishLocker: func() lock.ILocker { return locker }}
	err = svc.UpsertExptTurnResultFilter(context.Background(), 3, 1, []int64{10})
	require.True(t, errno.NeedMQRetry(err), "lost lease may not be acknowledged as successful refresh")
}

// Public Compare must rebuild under the same item lock as Upsert, including
// records whose scores are absent rather than using stale cached DTOs.
func TestRunningRetryFilterSync_CompareReadsInsideItemLock(t *testing.T) {
	f := newFilterSyncFixture(t)
	store := &filterStore{visible: true}
	f.svc.exptTurnResultFilterRepo = store
	require.NoError(t, f.svc.UpsertExptTurnResultFilter(context.Background(), f.space, f.expt, []int64{f.item}))
	ctrl := gomock.NewController(t)
	publisher := eventmocks.NewMockExptEventPublisher(ctrl)
	f.svc.publisher = publisher
	// A missing filter must retain the original check-event retry budget.
	store.visible = false
	publisher.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, event *entity.ExptTurnResultFilterEvent, _ *time.Duration) error {
		require.True(t, f.server.Exists(f.key()))
		require.Equal(t, []int64{f.item}, event.ItemID)
		require.Equal(t, int32(2), *event.RetryTimes)
		return nil
	})
	require.NoError(t, f.svc.CompareExptTurnResultFilters(context.Background(), f.space, f.expt, []int64{f.item, f.item}, 1))
}

// A private wrapper waits for the real lock's renewal goroutine to release its
// holder before test clients are closed. Production lock behavior stays real.
type filterTestRedis struct {
	infraredis.Cmdable
	mu      sync.Mutex
	unlocks map[string]int
	changed chan struct{}
}

func (r *filterTestRedis) Eval(ctx context.Context, script string, keys []string, args ...any) *redis.Cmd {
	result := r.Cmdable.Eval(ctx, script, keys, args...)
	if strings.Contains(script, "redis.call('DEL'") && len(args) > 0 {
		r.mu.Lock()
		r.unlocks[fmt.Sprint(args[0])]++
		close(r.changed)
		r.changed = make(chan struct{})
		r.mu.Unlock()
	}
	return result
}
func (r *filterTestRedis) waitUnlocks(holder string, n int) {
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	for {
		r.mu.Lock()
		count, ch := r.unlocks[holder], r.changed
		r.mu.Unlock()
		if count >= n {
			return
		}
		select {
		case <-ch:
		case <-deadline.C:
			return
		}
	}
}

type filterTestLocker struct {
	lock.ILocker
	client   *filterTestRedis
	holder   string
	releases int
}

func (l *filterTestLocker) LockWithRenew(ctx context.Context, key string, ttl, hold time.Duration) (bool, context.Context, func(), error) {
	ok, child, cancel, err := l.ILocker.LockWithRenew(ctx, key, ttl, hold)
	if !ok || err != nil {
		return ok, child, cancel, err
	}
	l.releases += 2
	return ok, child, func() { cancel(); l.client.waitUnlocks(l.holder, l.releases) }, nil
}

var filterTestHolderSequence atomic.Int64

func filterRealTestLocker(client infraredis.Cmdable) lock.ILocker {
	observed := &filterTestRedis{Cmdable: client, unlocks: map[string]int{}, changed: make(chan struct{})}
	holder := fmt.Sprint("filter-test-", filterTestHolderSequence.Add(1))
	return &filterTestLocker{ILocker: lock.NewRedisLockerWithHolder(observed, holder), client: observed, holder: holder}
}
