// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	idgenMocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem"
	idemMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	metricsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	componentMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	exptrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

type terminateAwareManager struct {
	*svcmocks.MockIExptManager
	terminated bool
}

func (m *terminateAwareManager) TerminateCurrentRun(context.Context, int64, int64, int64, *entity.Session, *entity.ExptCurrentItemRunSource, ...entity.CompleteExptOptionFn) (bool, error) {
	m.terminated = true
	return true, nil
}

// 重试 run 建在已终态实验上、ExptStart 又失败时，终止链路会拒绝受理（requires active current run），
// 必须落回 CompleteRun 只关闭新 run，否则 run 永远停在 Pending。
func TestHandleEventErrFallsBackForFinishedExperiment(t *testing.T) {
	for _, kind := range []string{"finished", "processing"} {
		t.Run(kind, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			manager := &terminateAwareManager{MockIExptManager: svcmocks.NewMockIExptManager(ctrl)}
			exptRepo := repomocks.NewMockIExperimentRepo(ctrl)
			impl := &ExptSchedulerImpl{Manager: manager, ExptRepo: exptRepo}
			event := &entity.ExptScheduleEvent{
				ExptID: 7, ExptRunID: 8, SpaceID: 9,
				ExptType: entity.ExptType_Offline, ExptRunMode: entity.EvaluationModeRetryAll,
				Session: &entity.Session{},
			}
			status := entity.ExptStatus_Processing
			if kind == "finished" {
				status = entity.ExptStatus_Success
			}
			exptRepo.EXPECT().GetByID(gomock.Any(), int64(7), int64(9)).Return(&entity.Experiment{ID: 7, SpaceID: 9, Status: status}, nil)
			if kind == "finished" {
				manager.EXPECT().CompleteRun(gomock.Any(), int64(7), int64(8), int64(9), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
			}

			handler := impl.HandleEventErr(func(context.Context, *entity.ExptScheduleEvent) error {
				return errors.New("list evaluation set items failed")
			})
			require.NoError(t, handler(context.Background(), event))
			require.Equal(t, kind == "processing", manager.terminated)
		})
	}
}

func TestHandleEventErrClosesPendingRetryOnFinishedExperiment(t *testing.T) {
	ctrl := gomock.NewController(t)
	manager := newTestExptManager(ctrl)
	configer := componentMocks.NewMockIConfiger(ctrl)
	configer.EXPECT().GetExptExecConf(gomock.Any(), int64(9)).Return(&entity.ExptExecConf{
		SpaceExptConcurLimit: 1, ZombieIntervalSecond: 3600,
	}).AnyTimes()
	manager.configer = configer
	quota := &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{7: time.Now().Unix()}}
	manager.quotaRepo.(*repomocks.MockQuotaRepo).EXPECT().CreateOrUpdate(gomock.Any(), int64(9), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ int64, update func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
			next, changed, err := update(quota.Clone())
			if err == nil && changed {
				quota = next
			}
			return err
		}).AnyTimes()
	provider := db.NewTestDB(t, &model.Experiment{}, &model.ExptRunLog{}, &model.ExptStats{}, &model.ExptEvaluatorRef{})
	ctx := context.Background()
	endedAt := time.Now().Add(-time.Minute)
	require.NoError(t, provider.NewSession(ctx).Create(&model.Experiment{
		ID: 7, SpaceID: 9, ExptType: int32(entity.ExptType_Offline), LatestRunID: 8,
		Status: int32(entity.ExptStatus_Success), EndAt: &endedAt,
	}).Error)
	require.NoError(t, provider.NewSession(ctx).Create(&model.ExptRunLog{
		ID: 8, ExptID: 7, ExptRunID: 8, SpaceID: 9, Mode: gptr.Of(int32(entity.EvaluationModeRetryAll)),
		Status: gptr.Of(int64(entity.ExptStatus_Pending)),
	}).Error)

	lockerFactory, lockServer := finishRealLockerFactory(t)
	manager.mutex, manager.manualRetryLocker = lockerFactory(), lockerFactory
	manager.exptRepo = exptrepo.NewExptRepo(exptmysql.NewExptDAO(provider), exptmysql.NewExptEvaluatorRefDAO(provider), manager.idgenerator)
	manager.runLogRepo = exptrepo.NewExptRunLogRepo(exptmysql.NewExptRunLogDAO(provider))
	manager.runFinishRepo = exptrepo.NewExptRunFinishRepo(provider)
	lockKey := manager.makeExptMutexLockKey(7)
	locked, _, err := manager.mutex.BackoffLockWithValue(ctx, lockKey, "8", time.Hour, time.Millisecond)
	require.NoError(t, err)
	require.True(t, locked)
	require.True(t, lockServer.Exists(lockKey))

	manager.turnResultRepo.(*repomocks.MockIExptTurnResultRepo).EXPECT().
		ListTurnResult(gomock.Any(), int64(9), int64(7), nil, gomock.Any(), false).
		Return([]*entity.ExptTurnResult{}, int64(0), nil).AnyTimes()
	manager.idem.(*idemMocks.MockIdempotentService).EXPECT().Exist(gomock.Any(), "CompleteRun:exptexec:onerr:8").Return(false, nil).AnyTimes()
	manager.idem.(*idemMocks.MockIdempotentService).EXPECT().Set(gomock.Any(), "CompleteRun:exptexec:onerr:8", 3*time.Minute).Return(nil).AnyTimes()
	manager.idgenerator.(*idgenMocks.MockIIDGenerator).EXPECT().GenID(gomock.Any()).Return(int64(9), nil).AnyTimes()
	manager.mtr.(*metricsMocks.MockExptMetric).EXPECT().EmitExptExecRun(int64(9), int64(entity.EvaluationModeRetryItems)).AnyTimes()

	cause := errors.New("evaluation set item 999 does not exist")
	scheduler := &ExptSchedulerImpl{Manager: manager, ExptRepo: manager.exptRepo}
	event := &entity.ExptScheduleEvent{
		ExptID: 7, ExptRunID: 8, SpaceID: 9,
		ExptType: entity.ExptType_Offline, ExptRunMode: entity.EvaluationModeRetryAll,
		Session: &entity.Session{},
	}
	handler := scheduler.HandleEventErr(func(context.Context, *entity.ExptScheduleEvent) error { return cause })
	require.NoError(t, handler(ctx, event))

	run, err := manager.runLogRepo.Get(ctx, 7, 8)
	require.NoError(t, err)
	require.Equal(t, int64(entity.ExptStatus_Failed), run.Status)
	require.Equal(t, cause.Error(), string(run.StatusMessage))
	expt, err := manager.exptRepo.GetByID(ctx, 7, 9)
	require.NoError(t, err)
	require.Equal(t, entity.ExptStatus_Success, expt.Status)
	require.NotContains(t, quota.ExptID2RunTime, int64(7), "failed retry run left its prepareRun space quota reserved")
	require.NoError(t, NewQuotaService(manager.quotaRepo, manager.configer).AllowExptRun(ctx, 100, 9, event.Session),
		"space quota limit is one; the leaked retry reservation rejects the next experiment")
	require.False(t, lockServer.Exists(lockKey))

	newRunID, retried, err := manager.LogRetryItemsRun(ctx, 7, entity.EvaluationModeRetryItems, 9, []int64{999}, event.Session)
	require.NoError(t, err)
	require.False(t, retried)
	require.Equal(t, int64(9), newRunID)
	require.True(t, lockServer.Exists(lockKey))
}

type retryAllStartIdemFailure struct {
	idem.IdempotentService
	key string
	err error
}

func (i *retryAllStartIdemFailure) Set(ctx context.Context, key string, duration time.Duration) error {
	if key == i.key {
		return i.err
	}
	return i.IdempotentService.Set(ctx, key, duration)
}

type retryStartResetFailureRepo struct {
	repo.IExptItemResultRepo
}

func (r *retryStartResetFailureRepo) ResetItemForRetry(ctx context.Context, spaceID, exptID, runID int64, runLog *entity.ExptItemResultRunLog, turnIDs []int64) (bool, error) {
	applied, err := r.IExptItemResultRepo.(repo.IExptRetryItemResetRepo).ResetItemForRetry(ctx, spaceID, exptID, runID, runLog, turnIDs)
	if err != nil {
		return false, err
	}
	return applied, errors.New("save stats error")
}

func TestRetryAllStartFailureTerminatesMutatedRun(t *testing.T) {
	for _, tc := range []struct {
		name        string
		multiSet    bool
		retryItems  bool
		failIdemSet bool
	}{
		{name: "stats_save"},
		{name: "multi_set_stats_save", multiSet: true},
		{name: "idem_set_after_stats_save", failIdemSet: true},
		{name: "retry_items_stats_save", retryItems: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mode := entity.EvaluationModeRetryAll
			if tc.retryItems {
				mode = entity.EvaluationModeRetryItems
			}
			p := newPendingRunFixture(t, mode)
			ctx := context.Background()
			ctrl := gomock.NewController(t)
			p.expt.EvalSet = &entity.EvaluationSet{ID: p.f.base + 80, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: p.f.base + 81}}

			// Turn the initialized run into the terminal result that RetryAll receives.
			endedAt := time.Now()
			require.NoError(t, p.f.conn.Model(&model.Experiment{}).Where("space_id=? AND id=?", p.f.space, p.f.expt).
				Updates(map[string]any{"status": int32(entity.ExptStatus_Success), "end_at": &endedAt}).Error)
			require.NoError(t, p.f.conn.Model(&model.ExptRunLog{}).Where("space_id=? AND expt_id=? AND expt_run_id=?", p.f.space, p.f.expt, p.f.run).
				Update("status", int64(entity.ExptStatus_Success)).Error)
			require.NoError(t, p.f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND expt_id=?", p.f.space, p.f.expt).
				Update("status", int32(entity.ItemRunState_Success)).Error)
			require.NoError(t, p.f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND expt_id=?", p.f.space, p.f.expt).
				Update("status", int32(entity.ItemRunState_Success)).Error)
			require.NoError(t, p.f.conn.Model(&model.ExptTurnResult{}).Where("space_id=? AND expt_id=?", p.f.space, p.f.expt).
				Update("status", int32(entity.TurnRunState_Success)).Error)
			require.NoError(t, p.f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND expt_id=?", p.f.space, p.f.expt).
				Update("status", int32(entity.TurnRunState_Success)).Error)
			require.NoError(t, p.f.conn.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", p.f.space, p.f.expt).
				Updates(map[string]any{"pending_cnt": 0, "processing_cnt": 0, "success_cnt": 1, "fail_cnt": 0, "terminated_cnt": 0}).Error)
			_, err := p.manager.mutex.UnlockWithValue(ctx, p.manager.makeExptMutexLockKey(p.f.expt), strconv.FormatInt(p.f.run, 10))
			require.NoError(t, err)

			if tc.multiSet {
				p.expt.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
				require.NoError(t, p.f.conn.Exec("DELETE FROM expt_item_ref WHERE space_id=? AND expt_id=? AND item_id<>?", p.f.space, p.f.expt, p.f.item).Error)
				require.NoError(t, p.f.conn.Model(&model.ExptItemRef{}).Where("space_id=? AND expt_id=? AND item_id=?", p.f.space, p.f.expt, p.f.item).
					Updates(map[string]any{"eval_set_id": p.expt.EvalSet.ID, "eval_set_version_id": p.expt.EvalSet.EvaluationSetVersion.ID}).Error)
			}

			nextRunID := p.f.run + 100
			ids := idgenMocks.NewMockIIDGenerator(ctrl)
			ids.EXPECT().GenID(gomock.Any()).DoAndReturn(func(context.Context) (int64, error) {
				runID := nextRunID
				nextRunID++
				return runID, nil
			}).AnyTimes()
			nextGeneratedID := p.f.base + 12000
			ids.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, count int) ([]int64, error) {
				generated := make([]int64, count)
				for i := range generated {
					generated[i] = nextGeneratedID
					nextGeneratedID++
				}
				return generated, nil
			}).AnyTimes()
			p.manager.idgenerator = ids
			p.manager.mtr.(*metricsMocks.MockExptMetric).EXPECT().EmitExptExecRun(p.f.space, int64(mode)).Times(2)

			runID, joined, err := p.manager.LogRetryItemsRun(ctx, p.f.expt, mode, p.f.space, []int64{p.f.item}, p.event.Session)
			require.NoError(t, err)
			require.False(t, joined)
			require.Equal(t, p.f.run+100, runID)

			itemService := svcmocks.NewMockEvaluationSetItemService(ctrl)
			items := []*entity.EvaluationSetItem{{ItemID: p.f.item, ItemVersionID: &p.f.version, Turns: []*entity.Turn{{ID: 0}}}}
			if tc.multiSet || tc.retryItems {
				itemService.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return(items, nil).Times(1)
			} else {
				itemService.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).Return(items, gptr.Of(int64(1)), nil, nil, nil).Times(1)
			}
			stats := p.manager.statsRepo
			itemResultRepo := p.manager.itemResultRepo
			if !tc.failIdemSet {
				itemResultRepo = &retryStartResetFailureRepo{IExptItemResultRepo: itemResultRepo}
			}

			event := &entity.ExptScheduleEvent{
				ExptID: p.f.expt, ExptRunID: runID, SpaceID: p.f.space,
				ExptType: entity.ExptType_Offline, ExptRunMode: mode, Session: p.event.Session,
			}
			startIdem := p.manager.idem
			if tc.failIdemSet {
				startIdem = &retryAllStartIdemFailure{IdempotentService: startIdem, key: makeStartIdemKey(event), err: errors.New("idem set error")}
				p.manager.idem = startIdem
			}
			p.expt.Status = entity.ExptStatus_Success
			p.expt.LatestRunID = runID
			var exec interface {
				ExptStart(context.Context, *entity.ExptScheduleEvent, *entity.Experiment) error
			}
			if tc.retryItems {
				event.ExecEvalSetItemIDs = []int64{p.f.item}
				exec = &ExptRetryItemsExec{
					manager: p.manager, exptItemResultRepo: itemResultRepo, exptTurnResultRepo: p.manager.turnResultRepo,
					exptStatsRepo: stats, idgenerator: ids, evaluationSetItemService: itemService, exptRepo: p.manager.exptRepo,
					idem: startIdem, configer: p.manager.configer, exptItemRefRepo: p.manager.itemRefRepo,
				}
			} else {
				exec = &ExptRetryAllExec{
					manager: p.manager, exptItemResultRepo: itemResultRepo, exptTurnResultRepo: p.manager.turnResultRepo,
					exptStatsRepo: stats, idgenerator: ids, evaluationSetItemService: itemService, exptRepo: p.manager.exptRepo,
					idem: startIdem, configer: p.manager.configer, exptItemRefRepo: p.manager.itemRefRepo,
				}
			}
			scheduler := &ExptSchedulerImpl{Manager: p.manager, ExptRepo: p.manager.exptRepo}
			handler := scheduler.HandleEventErr(func(ctx context.Context, event *entity.ExptScheduleEvent) error {
				return exec.ExptStart(ctx, event, p.expt)
			})
			require.NoError(t, handler(ctx, event))

			current, err := p.manager.exptRepo.GetByID(ctx, p.f.expt, p.f.space)
			require.NoError(t, err)
			require.Equal(t, entity.ExptStatus_Failed, current.Status)
			var item model.ExptItemResult
			require.NoError(t, p.f.conn.Where("space_id=? AND expt_id=? AND item_id=?", p.f.space, p.f.expt, p.f.item).Take(&item).Error)
			require.Equal(t, int32(entity.ItemRunState_Terminal), item.Status)
			var queueing int64
			require.NoError(t, p.f.conn.Model(&model.ExptItemResultRunLog{}).
				Where("space_id=? AND expt_id=? AND expt_run_id=? AND status=?", p.f.space, p.f.expt, runID, int32(entity.ItemRunState_Queueing)).Count(&queueing).Error)
			require.Zero(t, queueing)

			newRunID, joined, err := p.manager.LogRetryItemsRun(ctx, p.f.expt, mode, p.f.space, []int64{p.f.item}, p.event.Session)
			require.NoError(t, err)
			require.False(t, joined)
			require.Equal(t, runID+1, newRunID)
		})
	}
}

type retryResetHookLocker struct {
	lock.ILocker
	onLock func()
}

func (l *retryResetHookLocker) LockWithRenew(parent context.Context, key string, ttl, maxHold time.Duration) (bool, context.Context, func(), error) {
	locked, lockCtx, cancel, err := l.ILocker.LockWithRenew(parent, key, ttl, maxHold)
	if locked && err == nil && l.onLock != nil {
		l.onLock()
	}
	return locked, lockCtx, cancel, err
}

func TestRetryStartRedeliveryPreservesLoggedItems(t *testing.T) {
	for _, tc := range []struct {
		name       string
		multiSet   bool
		retryItems bool
		infraRetry int
		status     entity.ItemRunState
		logged     bool
	}{
		{name: "retry_all", infraRetry: 1, status: entity.ItemRunState_Success, logged: true},
		{name: "retry_all_infra_zero", status: entity.ItemRunState_Success, logged: true},
		{name: "retry_all_multi_set", multiSet: true, infraRetry: 1, status: entity.ItemRunState_Success, logged: true},
		{name: "retry_all_multi_set_infra_zero", multiSet: true, status: entity.ItemRunState_Success, logged: true},
		{name: "retry_items", retryItems: true, infraRetry: 1, status: entity.ItemRunState_Success, logged: true},
		{name: "retry_items_infra_zero", retryItems: true, status: entity.ItemRunState_Success, logged: true},
		{name: "retry_items_multi_set", retryItems: true, multiSet: true, infraRetry: 1, status: entity.ItemRunState_Success, logged: true},
		{name: "retry_items_multi_set_infra_zero", retryItems: true, multiSet: true, status: entity.ItemRunState_Success, logged: true},
		{name: "retry_all_fail_logged", infraRetry: 1, status: entity.ItemRunState_Fail, logged: true},
		{name: "retry_all_terminal_logged", infraRetry: 1, status: entity.ItemRunState_Terminal, logged: true},
		{name: "retry_all_processing", infraRetry: 1, status: entity.ItemRunState_Processing},
		{name: "retry_all_multi_set_terminal_logged", multiSet: true, infraRetry: 1, status: entity.ItemRunState_Terminal, logged: true},
		{name: "retry_items_fail_logged", retryItems: true, infraRetry: 1, status: entity.ItemRunState_Fail, logged: true},
		{name: "retry_items_terminal_logged", retryItems: true, infraRetry: 1, status: entity.ItemRunState_Terminal, logged: true},
		{name: "retry_items_processing", retryItems: true, infraRetry: 1, status: entity.ItemRunState_Processing},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mode := entity.EvaluationModeRetryAll
			if tc.retryItems {
				mode = entity.EvaluationModeRetryItems
			}
			p := newPendingRunFixture(t, mode)
			ctx := context.Background()
			ctrl := gomock.NewController(t)
			p.expt.EvalSet = &entity.EvaluationSet{ID: p.f.base + 80, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: p.f.base + 81}}

			endedAt := time.Now()
			require.NoError(t, p.f.conn.Model(&model.Experiment{}).Where("space_id=? AND id=?", p.f.space, p.f.expt).
				Updates(map[string]any{"status": int32(entity.ExptStatus_Success), "end_at": &endedAt}).Error)
			require.NoError(t, p.f.conn.Model(&model.ExptRunLog{}).Where("space_id=? AND expt_id=? AND expt_run_id=?", p.f.space, p.f.expt, p.f.run).
				Update("status", int64(entity.ExptStatus_Success)).Error)
			require.NoError(t, p.f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND expt_id=?", p.f.space, p.f.expt).
				Update("status", int32(entity.ItemRunState_Success)).Error)
			require.NoError(t, p.f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND expt_id=?", p.f.space, p.f.expt).
				Update("status", int32(entity.ItemRunState_Success)).Error)
			require.NoError(t, p.f.conn.Model(&model.ExptTurnResult{}).Where("space_id=? AND expt_id=?", p.f.space, p.f.expt).
				Update("status", int32(entity.TurnRunState_Success)).Error)
			require.NoError(t, p.f.conn.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", p.f.space, p.f.expt).
				Updates(map[string]any{"pending_cnt": 0, "processing_cnt": 0, "success_cnt": 1, "fail_cnt": 0, "terminated_cnt": 0}).Error)
			_, err := p.manager.mutex.UnlockWithValue(ctx, p.manager.makeExptMutexLockKey(p.f.expt), strconv.FormatInt(p.f.run, 10))
			require.NoError(t, err)

			if tc.multiSet {
				p.expt.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig
				require.NoError(t, p.f.conn.Exec("DELETE FROM expt_item_ref WHERE space_id=? AND expt_id=? AND item_id<>?", p.f.space, p.f.expt, p.f.item).Error)
				require.NoError(t, p.f.conn.Model(&model.ExptItemRef{}).Where("space_id=? AND expt_id=? AND item_id=?", p.f.space, p.f.expt, p.f.item).
					Updates(map[string]any{"eval_set_id": p.expt.EvalSet.ID, "eval_set_version_id": p.expt.EvalSet.EvaluationSetVersion.ID}).Error)
			}

			runID := p.f.run + 100
			ids := idgenMocks.NewMockIIDGenerator(ctrl)
			ids.EXPECT().GenID(gomock.Any()).Return(runID, nil).AnyTimes()
			nextGeneratedID := p.f.base + 12000
			ids.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, count int) ([]int64, error) {
				generated := make([]int64, count)
				for i := range generated {
					generated[i] = nextGeneratedID
					nextGeneratedID++
				}
				return generated, nil
			}).AnyTimes()
			p.manager.idgenerator = ids
			p.manager.mtr.(*metricsMocks.MockExptMetric).EXPECT().EmitExptExecRun(p.f.space, int64(mode)).Times(1)
			createdRunID, joined, err := p.manager.LogRetryItemsRun(ctx, p.f.expt, mode, p.f.space, []int64{p.f.item}, p.event.Session)
			require.NoError(t, err)
			require.False(t, joined)
			require.Equal(t, runID, createdRunID)

			itemService := svcmocks.NewMockEvaluationSetItemService(ctrl)
			items := []*entity.EvaluationSetItem{{ItemID: p.f.item, ItemVersionID: &p.f.version, Turns: []*entity.Turn{{ID: 0}}}}
			if tc.multiSet || tc.retryItems {
				itemService.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).Return(items, nil).Times(2)
			} else {
				itemService.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).Return(items, gptr.Of(int64(1)), nil, nil, nil).Times(2)
			}
			event := &entity.ExptScheduleEvent{
				ExptID: p.f.expt, ExptRunID: runID, SpaceID: p.f.space,
				ExptType: entity.ExptType_Offline, ExptRunMode: mode, Session: p.event.Session,
			}
			if tc.retryItems {
				event.ExecEvalSetItemIDs = []int64{p.f.item}
			}
			newExec := func(startIdem idem.IdempotentService) interface {
				ExptStart(context.Context, *entity.ExptScheduleEvent, *entity.Experiment) error
			} {
				if tc.retryItems {
					return &ExptRetryItemsExec{
						manager: p.manager, exptItemResultRepo: p.manager.itemResultRepo, exptTurnResultRepo: p.manager.turnResultRepo,
						exptStatsRepo: p.manager.statsRepo, idgenerator: ids, evaluationSetItemService: itemService, exptRepo: p.manager.exptRepo,
						idem: startIdem, configer: p.manager.configer, exptItemRefRepo: p.manager.itemRefRepo,
					}
				}
				return &ExptRetryAllExec{
					manager: p.manager, exptItemResultRepo: p.manager.itemResultRepo, exptTurnResultRepo: p.manager.turnResultRepo,
					exptStatsRepo: p.manager.statsRepo, idgenerator: ids, evaluationSetItemService: itemService, exptRepo: p.manager.exptRepo,
					idem: startIdem, configer: p.manager.configer, exptItemRefRepo: p.manager.itemRefRepo,
				}
			}

			p.expt.Status = entity.ExptStatus_Success
			p.expt.LatestRunID = runID
			failedIdem := &retryAllStartIdemFailure{IdempotentService: p.manager.idem, key: makeStartIdemKey(event), err: context.DeadlineExceeded}
			err = newExec(failedIdem).ExptStart(ctx, event, p.expt)
			require.ErrorIs(t, err, context.DeadlineExceeded)
			itemLog, err := p.manager.itemResultRepo.GetItemRunLog(ctx, p.f.expt, runID, p.f.item, p.f.space)
			require.NoError(t, err)
			require.Equal(t, int32(entity.ItemRunState_Queueing), itemLog.Status, "the first initializer must finish its reset before idem.Set fails")

			const targetResultID = int64(401)
			runLogFields := map[string]any{"status": int32(tc.status)}
			if tc.logged {
				runLogFields["result_state"] = int32(entity.ExptItemResultStateLogged)
			}
			require.NoError(t, p.f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND expt_id=? AND item_id=?", p.f.space, p.f.expt, p.f.item).
				Update("status", int32(tc.status)).Error)
			require.NoError(t, p.f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", p.f.space, p.f.expt, runID, p.f.item).
				Updates(runLogFields).Error)
			turnStatus := entity.TurnRunState_Success
			turnTargetResultID := targetResultID
			if tc.status == entity.ItemRunState_Processing {
				turnStatus = entity.TurnRunState_Processing
				turnTargetResultID = 0
			}
			require.NoError(t, p.f.conn.Model(&model.ExptTurnResult{}).Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", p.f.space, p.f.expt, runID, p.f.item).
				Updates(map[string]any{"status": int32(turnStatus), "target_result_id": turnTargetResultID}).Error)
			statsFields := map[string]any{"pending_cnt": 0, "processing_cnt": 0, "success_cnt": 0, "fail_cnt": 0, "terminated_cnt": 0}
			switch tc.status {
			case entity.ItemRunState_Success:
				statsFields["success_cnt"] = 1
			case entity.ItemRunState_Fail:
				statsFields["fail_cnt"] = 1
			case entity.ItemRunState_Terminal:
				statsFields["terminated_cnt"] = 1
			case entity.ItemRunState_Processing:
				statsFields["processing_cnt"] = 1
			}
			require.NoError(t, p.f.conn.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", p.f.space, p.f.expt).
				Updates(statsFields).Error)
			scoreRefs := &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{p.f.base + 51: p.f.base + 61}}
			turnLog := &entity.ExptTurnResultRunLog{
				ID: p.f.base + 13000, SpaceID: p.f.space, ExptID: p.f.expt, ExptRunID: runID, ItemID: p.f.item,
				ItemVersionID: p.f.version, TurnID: 0, Status: turnStatus, TargetResultID: turnTargetResultID,
			}
			if tc.status != entity.ItemRunState_Processing {
				turnLog.EvaluatorResultIds = scoreRefs
			}
			require.NoError(t, p.manager.turnResultRepo.BatchCreateNXRunLog(ctx, []*entity.ExptTurnResultRunLog{turnLog}))

			redelivered := *event
			redelivered.InfraErrorRetryTimes = tc.infraRetry
			require.NoError(t, newExec(p.manager.idem).ExptStart(ctx, &redelivered, p.expt))

			var item model.ExptItemResult
			require.NoError(t, p.f.conn.Where("space_id=? AND expt_id=? AND item_id=?", p.f.space, p.f.expt, p.f.item).Take(&item).Error)
			require.Equal(t, int32(tc.status), item.Status)
			itemLog, err = p.manager.itemResultRepo.GetItemRunLog(ctx, p.f.expt, runID, p.f.item, p.f.space)
			require.NoError(t, err)
			require.Equal(t, int32(tc.status), itemLog.Status)
			if tc.logged {
				require.Equal(t, int32(entity.ExptItemResultStateLogged), itemLog.ResultState)
			}
			var turn model.ExptTurnResult
			require.NoError(t, p.f.conn.Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", p.f.space, p.f.expt, runID, p.f.item).Take(&turn).Error)
			require.Equal(t, int32(turnStatus), turn.Status)
			require.Equal(t, turnTargetResultID, turn.TargetResultID)
			turnLogs, err := p.manager.turnResultRepo.GetItemTurnRunLogs(ctx, p.f.expt, runID, p.f.item, p.f.space)
			require.NoError(t, err)
			require.Len(t, turnLogs, 1)
			require.Equal(t, turnStatus, turnLogs[0].Status)
			require.Equal(t, turnTargetResultID, turnLogs[0].TargetResultID)
			if tc.status != entity.ItemRunState_Processing {
				require.Equal(t, scoreRefs.EvalVerIDToResID, turnLogs[0].EvaluatorResultIds.EvalVerIDToResID)
			}
			var stats model.ExptStats
			require.NoError(t, p.f.conn.Where("space_id=? AND expt_id=?", p.f.space, p.f.expt).Take(&stats).Error)
			require.Zero(t, stats.PendingCnt)
			switch tc.status {
			case entity.ItemRunState_Success:
				require.Equal(t, int32(1), stats.SuccessCnt)
			case entity.ItemRunState_Fail:
				require.Equal(t, int32(1), stats.FailCnt)
			case entity.ItemRunState_Terminal:
				require.Equal(t, int32(1), stats.TerminatedCnt)
			case entity.ItemRunState_Processing:
				require.Equal(t, int32(1), stats.ProcessingCnt)
			}
		})
	}
}

// A redelivery that observes a materialized item only through the item lock must
// not fall back to the pre-lock snapshot and reset it.
func TestRetryStartRedeliveryRechecksItemStateUnderLock(t *testing.T) {
	p := newPendingRunFixture(t, entity.EvaluationModeRetryAll)
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	p.expt.EvalSet = &entity.EvaluationSet{ID: p.f.base + 80, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: p.f.base + 81}}

	endedAt := time.Now()
	require.NoError(t, p.f.conn.Model(&model.Experiment{}).Where("space_id=? AND id=?", p.f.space, p.f.expt).
		Updates(map[string]any{"status": int32(entity.ExptStatus_Success), "end_at": &endedAt}).Error)
	require.NoError(t, p.f.conn.Model(&model.ExptRunLog{}).Where("space_id=? AND expt_id=? AND expt_run_id=?", p.f.space, p.f.expt, p.f.run).
		Update("status", int64(entity.ExptStatus_Success)).Error)
	require.NoError(t, p.f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND expt_id=?", p.f.space, p.f.expt).
		Update("status", int32(entity.ItemRunState_Success)).Error)
	require.NoError(t, p.f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND expt_id=?", p.f.space, p.f.expt).
		Update("status", int32(entity.ItemRunState_Success)).Error)
	require.NoError(t, p.f.conn.Model(&model.ExptTurnResult{}).Where("space_id=? AND expt_id=?", p.f.space, p.f.expt).
		Update("status", int32(entity.TurnRunState_Success)).Error)
	require.NoError(t, p.f.conn.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", p.f.space, p.f.expt).
		Updates(map[string]any{"pending_cnt": 0, "processing_cnt": 0, "success_cnt": 1, "fail_cnt": 0, "terminated_cnt": 0}).Error)
	_, err := p.manager.mutex.UnlockWithValue(ctx, p.manager.makeExptMutexLockKey(p.f.expt), strconv.FormatInt(p.f.run, 10))
	require.NoError(t, err)

	runID := p.f.run + 100
	ids := idgenMocks.NewMockIIDGenerator(ctrl)
	ids.EXPECT().GenID(gomock.Any()).Return(runID, nil).AnyTimes()
	ids.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).Return([]int64{p.f.base + 12000, p.f.base + 12001}, nil).AnyTimes()
	p.manager.idgenerator = ids
	p.manager.mtr.(*metricsMocks.MockExptMetric).EXPECT().EmitExptExecRun(p.f.space, int64(entity.EvaluationModeRetryAll)).Times(1)
	_, joined, err := p.manager.LogRetryItemsRun(ctx, p.f.expt, entity.EvaluationModeRetryAll, p.f.space, []int64{p.f.item}, p.event.Session)
	require.NoError(t, err)
	require.False(t, joined)

	itemService := svcmocks.NewMockEvaluationSetItemService(ctrl)
	items := []*entity.EvaluationSetItem{{ItemID: p.f.item, ItemVersionID: &p.f.version, Turns: []*entity.Turn{{ID: 0}}}}
	itemService.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).Return(items, gptr.Of(int64(1)), nil, nil, nil).Times(2)
	event := &entity.ExptScheduleEvent{
		ExptID: p.f.expt, ExptRunID: runID, SpaceID: p.f.space,
		ExptType: entity.ExptType_Offline, ExptRunMode: entity.EvaluationModeRetryAll, Session: p.event.Session,
	}
	newExec := func(startIdem idem.IdempotentService) *ExptRetryAllExec {
		return &ExptRetryAllExec{
			manager: p.manager, exptItemResultRepo: p.manager.itemResultRepo, exptTurnResultRepo: p.manager.turnResultRepo,
			exptStatsRepo: p.manager.statsRepo, idgenerator: ids, evaluationSetItemService: itemService, exptRepo: p.manager.exptRepo,
			idem: startIdem, configer: p.manager.configer, exptItemRefRepo: p.manager.itemRefRepo,
		}
	}

	p.expt.Status = entity.ExptStatus_Success
	p.expt.LatestRunID = runID
	failedIdem := &retryAllStartIdemFailure{IdempotentService: p.manager.idem, key: makeStartIdemKey(event), err: context.DeadlineExceeded}
	err = newExec(failedIdem).ExptStart(ctx, event, p.expt)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	// Simulate a completion that lands after the pre-lock snapshot but before the
	// reset write: only the run log evidence exists once the item lock is held.
	require.NoError(t, p.f.conn.Unscoped().Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", p.f.space, p.f.expt, runID, p.f.item).
		Delete(&model.ExptItemResultRunLog{}).Error)
	require.NoError(t, p.f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND expt_id=? AND item_id=?", p.f.space, p.f.expt, p.f.item).
		Updates(map[string]any{"status": int32(entity.ItemRunState_Success), "expt_run_id": p.f.run}).Error)

	hookRan := false
	p.manager.manualRetryLocker = func() lock.ILocker {
		return &retryResetHookLocker{ILocker: p.factory(), onLock: func() {
			if hookRan {
				return
			}
			hookRan = true
			require.NoError(t, p.f.conn.Create(&model.ExptItemResultRunLog{
				ID: p.f.base + 13000, SpaceID: p.f.space, ExptID: p.f.expt, ExptRunID: runID, ItemID: p.f.item,
				ItemVersionID: p.f.version, Status: int32(entity.ItemRunState_Success), ResultState: gptr.Of(int32(entity.ExptItemResultStateLogged)),
			}).Error)
		}}
	}

	redelivered := *event
	redelivered.InfraErrorRetryTimes = 1
	require.NoError(t, newExec(p.manager.idem).ExptStart(ctx, &redelivered, p.expt))
	require.True(t, hookRan)

	var item model.ExptItemResult
	require.NoError(t, p.f.conn.Where("space_id=? AND expt_id=? AND item_id=?", p.f.space, p.f.expt, p.f.item).Take(&item).Error)
	require.Equal(t, int32(entity.ItemRunState_Success), item.Status)
	itemLog, err := p.manager.itemResultRepo.GetItemRunLog(ctx, p.f.expt, runID, p.f.item, p.f.space)
	require.NoError(t, err)
	require.Equal(t, int32(entity.ItemRunState_Success), itemLog.Status)
	require.Equal(t, int32(entity.ExptItemResultStateLogged), itemLog.ResultState)
}

// Resetting must apply per-item deltas so a concurrent completion is not
// overwritten by a stale whole-row stats snapshot.
func TestRetryStartResetAppliesStatsDeltaWithoutSnapshotOverwrite(t *testing.T) {
	p := newPendingRunFixture(t, entity.EvaluationModeRetryAll)
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	p.expt.EvalSet = &entity.EvaluationSet{ID: p.f.base + 80, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: p.f.base + 81}}

	endedAt := time.Now()
	require.NoError(t, p.f.conn.Model(&model.Experiment{}).Where("space_id=? AND id=?", p.f.space, p.f.expt).
		Updates(map[string]any{"status": int32(entity.ExptStatus_Success), "end_at": &endedAt}).Error)
	require.NoError(t, p.f.conn.Model(&model.ExptRunLog{}).Where("space_id=? AND expt_id=? AND expt_run_id=?", p.f.space, p.f.expt, p.f.run).
		Update("status", int64(entity.ExptStatus_Success)).Error)
	require.NoError(t, p.f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND expt_id=?", p.f.space, p.f.expt).
		Update("status", int32(entity.ItemRunState_Success)).Error)
	require.NoError(t, p.f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND expt_id=?", p.f.space, p.f.expt).
		Update("status", int32(entity.ItemRunState_Success)).Error)
	require.NoError(t, p.f.conn.Model(&model.ExptTurnResult{}).Where("space_id=? AND expt_id=?", p.f.space, p.f.expt).
		Update("status", int32(entity.TurnRunState_Success)).Error)
	require.NoError(t, p.f.conn.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", p.f.space, p.f.expt).
		Updates(map[string]any{"pending_cnt": 0, "processing_cnt": 0, "success_cnt": 1, "fail_cnt": 0, "terminated_cnt": 0}).Error)
	_, err := p.manager.mutex.UnlockWithValue(ctx, p.manager.makeExptMutexLockKey(p.f.expt), strconv.FormatInt(p.f.run, 10))
	require.NoError(t, err)

	runID := p.f.run + 100
	ids := idgenMocks.NewMockIIDGenerator(ctrl)
	ids.EXPECT().GenID(gomock.Any()).Return(runID, nil).AnyTimes()
	ids.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).Return([]int64{p.f.base + 12000, p.f.base + 12001}, nil).AnyTimes()
	p.manager.idgenerator = ids
	p.manager.mtr.(*metricsMocks.MockExptMetric).EXPECT().EmitExptExecRun(p.f.space, int64(entity.EvaluationModeRetryAll)).Times(1)
	_, joined, err := p.manager.LogRetryItemsRun(ctx, p.f.expt, entity.EvaluationModeRetryAll, p.f.space, []int64{p.f.item}, p.event.Session)
	require.NoError(t, err)
	require.False(t, joined)

	itemService := svcmocks.NewMockEvaluationSetItemService(ctrl)
	items := []*entity.EvaluationSetItem{{ItemID: p.f.item, ItemVersionID: &p.f.version, Turns: []*entity.Turn{{ID: 0}}}}
	itemService.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).Return(items, gptr.Of(int64(1)), nil, nil, nil).Times(2)
	event := &entity.ExptScheduleEvent{
		ExptID: p.f.expt, ExptRunID: runID, SpaceID: p.f.space,
		ExptType: entity.ExptType_Offline, ExptRunMode: entity.EvaluationModeRetryAll, Session: p.event.Session,
	}
	newExec := func(startIdem idem.IdempotentService) *ExptRetryAllExec {
		return &ExptRetryAllExec{
			manager: p.manager, exptItemResultRepo: p.manager.itemResultRepo, exptTurnResultRepo: p.manager.turnResultRepo,
			exptStatsRepo: p.manager.statsRepo, idgenerator: ids, evaluationSetItemService: itemService, exptRepo: p.manager.exptRepo,
			idem: startIdem, configer: p.manager.configer, exptItemRefRepo: p.manager.itemRefRepo,
		}
	}

	p.expt.Status = entity.ExptStatus_Success
	p.expt.LatestRunID = runID
	failedIdem := &retryAllStartIdemFailure{IdempotentService: p.manager.idem, key: makeStartIdemKey(event), err: context.DeadlineExceeded}
	err = newExec(failedIdem).ExptStart(ctx, event, p.expt)
	require.ErrorIs(t, err, context.DeadlineExceeded)

	// The item now belongs to the previous run again, so the redelivery resets it.
	require.NoError(t, p.f.conn.Unscoped().Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", p.f.space, p.f.expt, runID, p.f.item).
		Delete(&model.ExptItemResultRunLog{}).Error)
	require.NoError(t, p.f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND expt_id=? AND item_id=?", p.f.space, p.f.expt, p.f.item).
		Updates(map[string]any{"status": int32(entity.ItemRunState_Success), "expt_run_id": p.f.run}).Error)
	// One item is still executing while the reset runs.
	require.NoError(t, p.f.conn.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", p.f.space, p.f.expt).
		Updates(map[string]any{"pending_cnt": 0, "processing_cnt": 1, "success_cnt": 1, "fail_cnt": 0, "terminated_cnt": 0}).Error)

	p.manager.manualRetryLocker = func() lock.ILocker {
		return &retryResetHookLocker{ILocker: p.factory(), onLock: func() {
			require.NoError(t, p.manager.statsRepo.ArithOperateCount(ctx, p.f.expt, p.f.space, &entity.StatsCntArithOp{
				OpStatusCnt: map[entity.ItemRunState]int{entity.ItemRunState_Processing: -1, entity.ItemRunState_Success: 1},
			}))
		}}
	}

	redelivered := *event
	redelivered.InfraErrorRetryTimes = 1
	require.NoError(t, newExec(p.manager.idem).ExptStart(ctx, &redelivered, p.expt))

	var stats model.ExptStats
	require.NoError(t, p.f.conn.Where("space_id=? AND expt_id=?", p.f.space, p.f.expt).Take(&stats).Error)
	require.Equal(t, int32(1), stats.SuccessCnt)
	require.Zero(t, stats.ProcessingCnt)
	require.Equal(t, int32(1), stats.PendingCnt)
}

func TestCompleteRunDelayedCleanupPreservesReplacementRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	manager := newTestExptManager(ctrl)
	provider := db.NewTestDB(t, &model.Experiment{}, &model.ExptRunLog{}, &model.ExptStats{}, &model.ExptEvaluatorRef{})
	ctx := context.Background()
	endedAt := time.Now().Add(-time.Minute)
	require.NoError(t, provider.NewSession(ctx).Create(&model.Experiment{
		ID: 7, SpaceID: 9, ExptType: int32(entity.ExptType_Offline), LatestRunID: 8,
		Status: int32(entity.ExptStatus_Success), EndAt: &endedAt,
	}).Error)

	lockerFactory, lockServer := finishRealLockerFactory(t)
	manager.mutex, manager.manualRetryLocker = lockerFactory(), lockerFactory
	manager.exptRepo = exptrepo.NewExptRepo(exptmysql.NewExptDAO(provider), exptmysql.NewExptEvaluatorRefDAO(provider), manager.idgenerator)
	manager.runFinishRepo = exptrepo.NewExptRunFinishRepo(provider)
	quota := &entity.QuotaSpaceExpt{ExptID2RunTime: map[int64]int64{7: time.Now().Add(-time.Minute).Unix()}}
	manager.quotaRepo.(*repomocks.MockQuotaRepo).EXPECT().CreateOrUpdate(gomock.Any(), int64(9), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ int64, update func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
			next, changed, err := update(quota.Clone())
			if err == nil && changed {
				quota = next
			}
			return err
		}).AnyTimes()

	lockKey := manager.makeExptMutexLockKey(7)
	locked, _, err := manager.mutex.BackoffLockWithValue(ctx, lockKey, "8", time.Hour, time.Millisecond)
	require.NoError(t, err)
	require.True(t, locked)
	manager.runLogRepo.(*repomocks.MockIExptRunLogRepo).EXPECT().Get(gomock.Any(), int64(7), int64(8)).
		Return(&entity.ExptRunLog{ID: 8, ExptID: 7, ExptRunID: 8, SpaceID: 9, Status: int64(entity.ExptStatus_Pending)}, nil).AnyTimes()
	manager.runLogRepo.(*repomocks.MockIExptRunLogRepo).EXPECT().Update(gomock.Any(), int64(7), int64(8), int64(9), gomock.Any()).Return(nil).AnyTimes()
	manager.turnResultRepo.(*repomocks.MockIExptTurnResultRepo).EXPECT().
		ListTurnResult(gomock.Any(), int64(9), int64(7), nil, gomock.Any(), false).
		Return([]*entity.ExptTurnResult{}, int64(0), nil).AnyTimes()
	manager.idem.(*idemMocks.MockIdempotentService).EXPECT().Exist(gomock.Any(), "CompleteRun:delayed").
		DoAndReturn(func(context.Context, string) (bool, error) {
			// A duplicate R1 close finishes while this close is paused after idempotency checking; R2 then starts.
			released, err := manager.mutex.UnlockWithValue(ctx, lockKey, "8")
			require.NoError(t, err)
			require.True(t, released)
			locked, _, err := manager.mutex.BackoffLockWithValue(ctx, lockKey, "9", time.Hour, time.Millisecond)
			require.NoError(t, err)
			require.True(t, locked)
			require.NoError(t, manager.exptRepo.Update(ctx, &entity.Experiment{ID: 7, SpaceID: 9, LatestRunID: 9}))
			quota.ExptID2RunTime[7] = time.Now().Unix()
			return false, nil
		})
	manager.idem.(*idemMocks.MockIdempotentService).EXPECT().Set(gomock.Any(), "CompleteRun:delayed", 3*time.Minute).Return(nil).AnyTimes()

	require.NoError(t, manager.CompleteRun(ctx, 7, 8, 9, &entity.Session{},
		entity.WithCID("delayed"), entity.WithStatus(entity.ExptStatus_Failed)))
	require.True(t, lockServer.Exists(lockKey), "delayed R1 cleanup deleted the replacement R2 lifecycle lock")
	owner, err := lockServer.Get(lockKey)
	require.NoError(t, err)
	require.Equal(t, "9", owner)
	require.Contains(t, quota.ExptID2RunTime, int64(7), "delayed R1 cleanup released the replacement R2 space quota")
	expt, err := manager.exptRepo.GetByID(ctx, 7, 9)
	require.NoError(t, err)
	require.Equal(t, int64(9), expt.LatestRunID)
	require.Equal(t, entity.ExptStatus_Success, expt.Status)
}
