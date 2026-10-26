// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	idmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	infraredis "github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	exptrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	idemrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/idem"
	idemredis "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/idem/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type pendingRunFixture struct {
	f                        *lateWriteFixture
	manager                  *ExptMangerImpl
	start                    *ExptSubmitExec
	event                    *entity.ExptScheduleEvent
	expt                     *entity.Experiment
	result                   *ExptResultServiceImpl
	factory                  ManualRetryLockerFactory
	records                  *svcmocks.MockEvaluatorRecordService
	resources, notifications *atomic.Int32
}

// Runs the actual two initializers and actual Submit.ExptStart; no fixture
// assigns Pending to the run. Item/turn SQL writes also use production ports.
func newPendingRunFixture(t *testing.T, initMode entity.ExptRunMode) *pendingRunFixture {
	t.Helper()
	f := newLateWriteFixture(t, "")
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	for _, table := range []string{"expt_turn_evaluator_result_ref", "expt_turn_result_run_log", "expt_turn_result", "expt_item_result_run_log", "expt_item_result", "expt_run_log"} {
		require.NoError(t, f.conn.Exec("DELETE FROM "+table+" WHERE space_id=? AND expt_id=?", f.space, f.expt).Error)
	}
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).Updates(map[string]any{"status": int32(entity.ExptStatus_Pending), "latest_run_id": 0, "eval_set_id": f.base + 80, "eval_set_version_id": f.base + 81}).Error)
	require.NoError(t, f.conn.Model(&model.ExptStats{}).Where("space_id=?", f.space).Updates(map[string]any{"pending_cnt": 0, "processing_cnt": 0, "success_cnt": 0}).Error)
	factory, server := finishRealLockerFactory(t)
	client, err := infraredis.NewClient(&redis.Options{Addr: server.Addr()})
	require.NoError(t, err)
	raw, _ := infraredis.Unwrap(client)
	t.Cleanup(func() { _ = raw.Close() })
	idem := idemrepo.NewIdempotentService(idemredis.NewIdemDAO(client))
	config := confmocks.NewMockIConfiger(ctrl)
	config.EXPECT().GetExptExecConf(gomock.Any(), f.space).Return(&entity.ExptExecConf{RunningRetryEnabled: gptr.Of(true)}).AnyTimes()
	ids := idmocks.NewMockIIDGenerator(ctrl)
	ids.EXPECT().GenID(gomock.Any()).Return(f.run, nil).AnyTimes()
	var seq atomic.Int64
	ids.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, n int) ([]int64, error) {
		out := make([]int64, n)
		for i := range out {
			out[i] = f.base + 9000 + seq.Add(1)
		}
		return out, nil
	}).AnyTimes()
	experiments := exptrepo.NewExptRepo(exptmysql.NewExptDAO(f.provider), exptmysql.NewExptEvaluatorRefDAO(f.provider), ids)
	runs := exptrepo.NewExptRunLogRepo(exptmysql.NewExptRunLogDAO(f.provider))
	stats := exptrepo.NewExptStatsRepo(exptmysql.NewExptStatsDAO(f.provider))
	records := svcmocks.NewMockEvaluatorRecordService(ctrl)
	result := &ExptResultServiceImpl{ExptItemResultRepo: f.itemRepo(), ExptTurnResultRepo: f.turnRepo(), ExptStatsRepo: stats, idgen: ids, evaluatorRecordService: records, scoreCalculator: NewEvaluatorScoreCalculator(nil, nil), resultPublishRepo: exptrepo.NewExptResultPublishRepo(f.provider), resultPublishLocker: factory}
	resources, notifications := &atomic.Int32{}, &atomic.Int32{}
	quota := repomocks.NewMockQuotaRepo(ctrl)
	quota.EXPECT().CreateOrUpdate(gomock.Any(), f.space, gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, _ int64, _ func(*entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error), _ *entity.Session) error {
		resources.Add(1)
		var row model.Experiment
		require.NoError(t, f.conn.WithContext(ctx).Where("space_id=?", f.space).Take(&row).Error)
		require.True(t, entity.IsExptFinished(entity.ExptStatus(row.Status)))
		return nil
	}).AnyTimes()
	metric := metricsmocks.NewMockExptMetric(ctrl)
	metric.EXPECT().EmitExptExecRun(f.space, int64(initMode))
	metric.EXPECT().EmitExptExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Do(func(int64, int64, int64, time.Time) { notifications.Add(1) }).AnyTimes()
	cleanupResult := &finishCalibrationTestResult{ExptResultService: svcmocks.NewMockExptResultService(ctrl), ready: true}
	cleanupResult.ExptResultService.(*svcmocks.MockExptResultService).EXPECT().GetIncompleteTurns(gomock.Any(), f.expt, f.space, gomock.Any()).Return(nil, nil).AnyTimes()
	cleanupResult.ExptResultService.(*svcmocks.MockExptResultService).EXPECT().UpsertExptTurnResultFilter(gomock.Any(), f.space, f.expt, gomock.Any()).DoAndReturn(func(ctx context.Context, _, _ int64, ids []int64) error {
		var rows []model.ExptItemResult
		require.NoError(t, f.conn.WithContext(ctx).Where("space_id=? AND item_id IN ?", f.space, ids).Find(&rows).Error)
		for _, row := range rows {
			require.Equal(t, int32(entity.ItemRunState_Terminal), row.Status)
		}
		return nil
	}).AnyTimes()
	manager := &ExptMangerImpl{configer: config, mutex: factory(), manualRetryLocker: factory, runLogRepo: runs, exptRepo: experiments, itemResultRepo: f.itemRepo(), itemRefRepo: exptrepo.NewExptItemRefRepo(exptmysql.NewExptItemRefDAO(f.provider)), turnResultRepo: f.turnRepo(), statsRepo: stats, runFinishRepo: exptrepo.NewExptRunFinishRepo(f.provider), manualRetryRepo: exptrepo.NewExptManualRetryRepo(f.provider, ids), mtr: metric, idgenerator: ids, idem: idem, centralGuard: component.NewNoopCentralReservationGuard(), currentRunWaker: component.NewNoopCurrentRunSchedulerWaker(), quotaRepo: quota, exptResultService: cleanupResult}
	session := &entity.Session{UserID: "pending-run-test"}
	if initMode == entity.EvaluationModeRetryItems {
		run, joined, err := manager.LogRetryItemsRun(ctx, f.expt, initMode, f.space, []int64{f.item}, session)
		require.NoError(t, err)
		require.False(t, joined)
		require.Equal(t, f.run, run)
	} else {
		require.NoError(t, manager.LogRun(ctx, f.expt, f.run, initMode, f.space, []int64{f.item}, session))
	}
	itemSvc := svcmocks.NewMockEvaluationSetItemService(ctrl)
	itemSvc.EXPECT().ListEvaluationSetItems(gomock.Any(), gomock.Any()).Return([]*entity.EvaluationSetItem{{ItemID: f.item, ItemVersionID: &f.version, Turns: []*entity.Turn{{ID: 0}}}}, gptr.Of(int64(1)), nil, nil, nil)
	startResult := svcmocks.NewMockExptResultService(ctrl)
	startResult.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), f.space, f.expt, gomock.Any()).Return(nil)
	event := &entity.ExptScheduleEvent{ExptID: f.expt, ExptRunID: f.run, SpaceID: f.space, ExptRunMode: initMode, ExptType: entity.ExptType_Offline, Session: session}
	expt := &entity.Experiment{ID: f.expt, SpaceID: f.space, LatestRunID: f.run, ExptType: entity.ExptType_Offline, EvalSet: &entity.EvaluationSet{ID: f.base + 80, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: f.base + 81}}}
	start := &ExptSubmitExec{manager: manager, exptItemResultRepo: f.itemRepo(), exptTurnResultRepo: f.turnRepo(), exptStatsRepo: stats, exptRepo: experiments, idem: idem, configer: config, idgenerator: ids, evaluationSetItemService: itemSvc, resultSvc: startResult}
	require.NoError(t, start.ExptStart(ctx, event, expt))
	run, err := runs.Get(ctx, f.expt, f.run)
	require.NoError(t, err)
	require.Equal(t, int64(entity.ExptStatus_Pending), run.Status)
	current, err := experiments.GetByID(ctx, f.expt, f.space)
	require.NoError(t, err)
	require.Equal(t, entity.ExptStatus_Processing, current.Status)
	require.NoError(t, f.turnRepo().BatchCreateNXRunLog(ctx, []*entity.ExptTurnResultRunLog{{ID: f.base + 9900, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: f.item, ItemVersionID: f.version, TurnID: 0, Status: entity.TurnRunState_Queueing}}))
	return &pendingRunFixture{f: f, manager: manager, start: start, event: event, expt: current, result: result, factory: factory, records: records, resources: resources, notifications: notifications}
}

func (p *pendingRunFixture) publishSuccess(t *testing.T, withRecord bool) {
	t.Helper()
	f := p.f
	ctx := context.Background()
	current, err := p.manager.itemResultRepo.GetCurrentRequest(ctx, f.space, f.expt, f.run, f.item)
	require.NoError(t, err)
	require.NotNil(t, current)
	dispatch := exptrepo.NewExptItemDispatchRepo(exptmysql.NewExptItemDispatchDAO(f.provider))
	runtime, err := dispatch.LoadDispatchRuntime(ctx, f.space, f.expt, f.run, 1)
	require.NoError(t, err)
	require.Equal(t, []int64{f.item}, runtime.CandidateItemIDs)
	require.Equal(t, current.RequestID, runtime.ItemRequestIDs[f.item])
	if current.RequestID != "" {
		for _, stale := range []string{"", "obsolete-request"} {
			before := f.snapshot()
			claimed, err := dispatch.ClaimQuotaReserved(ctx, f.space, f.expt, f.run, []int64{f.item}, map[int64]string{f.item: stale})
			require.NoError(t, err)
			require.Empty(t, claimed)
			require.Equal(t, before, f.snapshot(), "stale grant cannot mutate the admitted request")
		}
	}
	claimed, err := dispatch.ClaimQuotaReserved(ctx, f.space, f.expt, f.run, []int64{f.item}, runtime.ItemRequestIDs)
	require.NoError(t, err)
	require.Len(t, claimed, 1)
	payload := claimed[0]
	require.Equal(t, f.item, payload.ItemID)
	require.Equal(t, current.RequestID, payload.RequestID)
	require.Equal(t, current.Mode, payload.Mode)
	require.Equal(t, current.RetryTimes, payload.RetryTimes)
	assertPending := func(stage string) {
		before := f.snapshot()
		finish, err := p.manager.runFinishRepo.FinishCurrentRun(ctx, &entity.ExptRunFinishRequest{SpaceID: f.space, ExptID: f.expt, RunID: f.run})
		require.NoError(t, err)
		require.Equal(t, entity.ExptRunFinishPending, finish.Disposition, stage)
		require.Equal(t, before, f.snapshot(), stage)
	}
	assertPending("reserved grant")
	started, err := dispatch.StartReservedItem(ctx, f.space, f.expt, f.run, payload.ItemID, payload.RequestID)
	require.NoError(t, err)
	require.True(t, started.CurrentRequest)
	require.True(t, started.Started)
	before := f.snapshot()
	duplicate, err := dispatch.StartReservedItem(ctx, f.space, f.expt, f.run, payload.ItemID, payload.RequestID)
	require.NoError(t, err)
	require.True(t, duplicate.CurrentRequest)
	require.False(t, duplicate.Started)
	require.Equal(t, before, f.snapshot(), "duplicate Start cannot apply a second stats delta")
	assertPending("processing source")
	consumer := &ExptItemEventEvalServiceImpl{exptItemResultRepo: f.itemRepo()}
	invoked := false
	require.NoError(t, consumer.HandleCurrentSource(func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		invoked = true
		source := executionSource(ctx).Source
		require.Equal(t, payload.RequestID, source.Current.RequestID)
		require.Equal(t, payload.Mode, source.Current.Mode)
		require.Equal(t, int(payload.RetryTimes), event.RetryTimes)
		return nil
	})(ctx, &entity.ExptItemEvalEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, EvalSetItemID: payload.ItemID, RequestID: payload.RequestID, ExptRunMode: payload.Mode}))
	require.True(t, invoked)
	source := f.source()
	turn := *source.Turns[0]
	turn.Status = entity.TurnRunState_Success
	turn.TargetResultID = 0
	turn.EvaluatorResultIds = &entity.EvaluatorResults{}
	if withRecord {
		turn.EvaluatorResultIds = &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{f.base + 51: f.base + 61}}
		p.records.EXPECT().BatchGetEvaluatorRecord(gomock.Any(), []int64{f.base + 61}, false, false).Return([]*entity.EvaluatorRecord{{ID: f.base + 61, SpaceID: f.space, ExperimentID: f.expt, ExperimentRunID: f.run, ItemID: f.item, ItemVersionID: f.version, TurnID: 0, EvaluatorVersionID: f.base + 51, Status: entity.EvaluatorRunStatusSuccess}}, nil)
	}
	applied, err := f.port.StoreCurrentTurnRunResult(ctx, source, &turn)
	require.NoError(t, err)
	require.True(t, applied)
	source = f.source()
	applied, err = f.port.CompleteCurrentItemRun(ctx, source, &entity.ExptCurrentItemRunWrite{Status: entity.ItemRunState_Success, RetryTimes: source.Current.RetryTimes})
	require.NoError(t, err)
	require.True(t, applied)
	assertPending("logged result awaiting publication")
	_, err = p.result.RecordItemRunLogs(ctx, f.expt, f.run, f.item, f.space, p.expt)
	require.NoError(t, err)
	log, err := p.manager.itemResultRepo.GetItemRunLog(ctx, f.expt, f.run, f.item, f.space)
	require.NoError(t, err)
	require.Equal(t, int32(entity.ExptItemResultStateResulted), log.ResultState)
	t.Logf("REAL_MYSQL_PENDING_DISPATCH_SOURCE_PUBLICATION request=%q mode=%d duplicate_start_stats_once=true reserved_processing_logged_block_finish=true", payload.RequestID, payload.Mode)
}

func TestRunFinishPendingRealInitializers(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeRetryItems} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			p := newPendingRunFixture(t, mode)
			before := p.f.snapshot()
			result, err := exptrepo.NewExptRunFinishRepo(p.f.provider).FinishCurrentRun(context.Background(), &entity.ExptRunFinishRequest{SpaceID: p.f.space, ExptID: p.f.expt, RunID: p.f.run})
			require.NoError(t, err)
			require.Equal(t, entity.ExptRunFinishPending, result.Disposition)
			require.Equal(t, before, p.f.snapshot())
		})
	}
}

func TestRunFinishPendingRealAdmissionPublicationAndFourModes(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeFailRetry, entity.EvaluationModeRetryAll, entity.EvaluationModeRetryItems} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			initMode := mode
			if mode != entity.EvaluationModeRetryItems {
				initMode = entity.EvaluationModeSubmit
			}
			p := newPendingRunFixture(t, initMode)
			f := p.f
			p.publishSuccess(t, false)
			req := &entity.RetryInCurrentRunRequest{SpaceID: f.space, ExptID: f.expt, ExpectedRunID: f.run, RequestID: "72cc8921-40f4-4a43-b772-aaf2b5dcdf05", Mode: entity.EvaluationModeRetryItems, ItemIDs: []int64{f.item}, Session: p.event.Session}
			receipt, err := p.manager.RetryInCurrentRun(context.Background(), req)
			require.NoError(t, err)
			require.Equal(t, int32(1), receipt.AcceptedCount)
			run, err := p.manager.runLogRepo.Get(context.Background(), f.expt, f.run)
			require.NoError(t, err)
			require.Equal(t, int64(entity.ExptStatus_Pending), run.Status)
			finish, err := exptrepo.NewExptRunFinishRepo(f.provider).FinishCurrentRun(context.Background(), &entity.ExptRunFinishRequest{SpaceID: f.space, ExptID: f.expt, RunID: f.run})
			require.NoError(t, err)
			require.Equal(t, entity.ExptRunFinishPending, finish.Disposition)
			p.publishSuccess(t, true)
			event := *p.event
			event.ExptRunMode = mode
			event.ExecEvalSetItemIDs = []int64{f.item}
			base := p.manager.exptResultService
			var end func(context.Context, *entity.ExptScheduleEvent, *entity.Experiment, int, int) (bool, error)
			switch mode {
			case entity.EvaluationModeSubmit:
				end = (&ExptSubmitExec{manager: p.manager, idem: p.manager.idem, configer: p.manager.configer, exptItemResultRepo: f.itemRepo(), resultSvc: base}).ExptEnd
			case entity.EvaluationModeFailRetry:
				end = (&ExptFailRetryExec{manager: p.manager, idem: p.manager.idem, configer: p.manager.configer, exptItemResultRepo: f.itemRepo(), resultSvc: base}).ExptEnd
			case entity.EvaluationModeRetryAll:
				end = (&ExptRetryAllExec{manager: p.manager, idem: p.manager.idem, configer: p.manager.configer, exptItemResultRepo: f.itemRepo(), resultSvc: base}).ExptEnd
			case entity.EvaluationModeRetryItems:
				end = (&ExptRetryItemsExec{manager: p.manager, idem: p.manager.idem, configer: p.manager.configer, exptItemResultRepo: f.itemRepo(), exptRunLogRepo: p.manager.runLogRepo, resultSvc: base}).ExptEnd
			}
			scheduler := &ExptSchedulerImpl{Manager: p.manager, Mutex: p.factory()}
			require.NoError(t, scheduler.HandleEventLock(func(ctx context.Context, e *entity.ExptScheduleEvent) error {
				next, err := end(ctx, e, p.expt, 0, 0)
				require.False(t, next)
				return err
			})(context.Background(), &event))
			after := f.snapshot()
			require.Equal(t, int32(1), p.notifications.Load())
			require.NoError(t, scheduler.HandleEventCheck(func(context.Context, *entity.ExptScheduleEvent) error {
				t.Fatal("duplicate finished event reentered execution")
				return nil
			})(context.Background(), &event))
			require.Equal(t, int32(1), p.notifications.Load(), "real idem must suppress normal AlreadyFinished duplicates")
			require.Equal(t, int32(2), p.resources.Load(), "resource cleanup still replays after notification marker")
			require.Equal(t, after, f.snapshot(), "AlreadyFinished cannot recalculate stats or endAt")
			again, err := p.manager.RetryInCurrentRun(context.Background(), req)
			require.NoError(t, err)
			a, _ := json.Marshal(receipt)
			b, _ := json.Marshal(again)
			require.Equal(t, string(a), string(b))
			require.Equal(t, after, f.snapshot())
			req.RequestID = "c551f9c8-e195-48a0-8505-f63b20802206"
			_, err = p.manager.RetryInCurrentRun(context.Background(), req)
			require.Error(t, err)
			require.Equal(t, after, f.snapshot())
		})
	}
}

func TestRunFinishPendingRealHalfTerminalCheckRetries(t *testing.T) {
	p := newPendingRunFixture(t, entity.EvaluationModeSubmit)
	p.publishSuccess(t, false)
	f := p.f
	// Reproduce the old split CompleteRun/CompleteExpt half-state explicitly.
	require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=?", f.space).Update("status", int64(entity.ExptStatus_Success)).Error)
	before := f.snapshot()
	scheduler := &ExptSchedulerImpl{Manager: p.manager}
	err := scheduler.HandleEventCheck(func(context.Context, *entity.ExptScheduleEvent) error {
		t.Fatal("half-terminal event entered execution")
		return nil
	})(context.Background(), p.event)
	require.True(t, errno.NeedMQRetry(err))
	require.Equal(t, before, f.snapshot())
	require.Zero(t, p.resources.Load())
	require.Zero(t, p.notifications.Load())
}
