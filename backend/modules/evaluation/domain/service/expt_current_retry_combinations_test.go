// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bytedance/gg/gptr"
	idmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	idemmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	confmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	exptrepo "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type currentRetryCombinationFixture struct {
	f          *lateWriteFixture
	manager    *ExptMangerImpl
	result     *ExptResultServiceImpl
	dispatch   repo.IExptItemDispatchRepo
	mode       entity.ExptSchedulerMode
	expt       *entity.Experiment
	event      *entity.ExptScheduleEvent
	server     *miniredis.Miniredis
	parentMode entity.ExptRunMode
	oldRun     int64
	items      []int64
}

// Seed a small, legal mid-run projection, reusing the existing local MySQL
// fixture and production repositories. A's Resulted is never assigned by this
// fixture: it reaches that state through the real completion/publication paths.
// FailRetry alone starts with an inherited successful canonical turn from S.
func newCurrentRetryCombinationFixture(t *testing.T, parent entity.ExptRunMode) *currentRetryCombinationFixture {
	t.Helper()
	dsn := os.Getenv("EXPT_RETRY_MYSQL_DSN")
	require.NotEmpty(t, dsn, "combination acceptance requires real local MySQL; do not silently skip")
	require.Contains(t, dsn, "@tcp(127.0.0.1:3306)/expt_running_retry_test?", "only the authorized local fixture database may be used")
	f := newLateWriteFixture(t, "")
	ctrl := gomock.NewController(t)
	ctx := context.Background()
	oldRun := f.base + 7
	items := []int64{f.item, f.item + 1}
	if parent == entity.EvaluationModeRetryItems {
		items = append(items, f.item+2, f.item+3)
	}
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=? AND id=?", f.space, f.expt).Updates(map[string]any{
		"eval_set_id": f.base + 80, "eval_set_version_id": f.base + 81,
	}).Error)
	chunks := []byte(fmt.Sprintf(`[{"ItemIDs":[%d,%d],"CreateAt":1700000000,"future":{"opaque":"keep"}}]`, items[0], items[1]))
	require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=? AND expt_run_id=?", f.space, f.run).Updates(map[string]any{"mode": int32(parent), "item_ids": chunks, "processing_cnt": 2}).Error)
	require.NoError(t, f.conn.Create(&model.ExptRunLog{ID: oldRun, SpaceID: f.space, ExptID: f.expt, ExptRunID: oldRun, Status: gptr.Of(int64(entity.ExptStatus_Failed)), Mode: gptr.Of(int32(entity.EvaluationModeSubmit)), ItemIds: &chunks}).Error)
	// S failed A/B. This makes FailRetry's inherited successful first turn and
	// newly running second turn a normal history, rather than a mixed fixture.
	require.NoError(t, f.conn.Create(&model.ExptItemResultRunLog{ID: f.base + 690, SpaceID: f.space, ExptID: f.expt, ExptRunID: oldRun, ItemID: f.item, ItemVersionID: f.version, Status: int32(entity.ItemRunState_Fail), ResultState: gptr.Of(int32(entity.ExptItemResultStateResulted))}).Error)
	require.NoError(t, f.conn.Create(&model.ExptTurnResultRunLog{ID: f.base + 650, SpaceID: f.space, ExptID: f.expt, ExptRunID: oldRun, ItemID: f.item, ItemVersionID: f.version, TurnID: 0, Status: int32(entity.TurnRunState_Success), EvaluatorResultIds: gptr.Of([]byte(`{}`))}).Error)
	require.NoError(t, f.conn.Create(&model.ExptTurnResultRunLog{ID: f.base + 651, SpaceID: f.space, ExptID: f.expt, ExptRunID: oldRun, ItemID: f.item, ItemVersionID: f.version, TurnID: f.base + 22, Status: int32(entity.TurnRunState_Fail), EvaluatorResultIds: gptr.Of([]byte(`{}`))}).Error)
	require.NoError(t, f.conn.Create(&model.ExptItemResultRunLog{ID: f.base + 691, SpaceID: f.space, ExptID: f.expt, ExptRunID: oldRun, ItemID: items[1], ItemVersionID: f.version, Status: int32(entity.ItemRunState_Fail), ResultState: gptr.Of(int32(entity.ExptItemResultStateResulted))}).Error)
	require.NoError(t, f.conn.Create(&model.ExptTurnResultRunLog{ID: f.base + 652, SpaceID: f.space, ExptID: f.expt, ExptRunID: oldRun, ItemID: items[1], ItemVersionID: f.version, TurnID: 0, Status: int32(entity.TurnRunState_Fail), EvaluatorResultIds: gptr.Of([]byte(`{}`))}).Error)
	// The reused fixture has old evaluator references; this acceptance uses
	// record-free execution results. Record-specific guards have separate tests.
	require.NoError(t, f.conn.Where("space_id=?", f.space).Delete(&model.ExptTurnEvaluatorResultRef{}).Error)
	for _, table := range []string{"expt_turn_result", "expt_turn_result_run_log"} {
		fields := map[string]any{"target_result_id": 0}
		if table == "expt_turn_result_run_log" {
			fields["evaluator_result_ids"] = []byte(`{}`)
		}
		require.NoError(t, f.conn.Table(table).Where("space_id=?", f.space).Updates(fields).Error)
	}
	if parent == entity.EvaluationModeFailRetry {
		require.NoError(t, f.conn.Model(&model.ExptTurnResult{}).Where("space_id=? AND item_id=? AND turn_id=0", f.space, f.item).Updates(map[string]any{"expt_run_id": oldRun, "status": int32(entity.TurnRunState_Success)}).Error)
		require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND item_id=? AND turn_id=0", f.space, f.item).Update("status", int32(entity.TurnRunState_Success)).Error)
	}
	// B remains active throughout Q1/Q2. C/D are historical completed items;
	// the RetryItems parent will actually append and initialize them below.
	require.NoError(t, f.conn.Create(&model.ExptTurnResult{ID: f.base + 450, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: items[1], ItemVersionID: f.version, TurnID: 0, Status: int32(entity.TurnRunState_Processing)}).Error)
	require.NoError(t, f.conn.Create(&model.ExptTurnResultRunLog{ID: f.base + 550, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: items[1], ItemVersionID: f.version, TurnID: 0, Status: int32(entity.TurnRunState_Processing), EvaluatorResultIds: gptr.Of([]byte(`{}`))}).Error)
	require.NoError(t, f.conn.Unscoped().Where("space_id=? AND item_id=? AND expt_run_id=?", f.space, f.item+2, f.run).Delete(&model.ExptItemResultRunLog{}).Error)
	if parent == entity.EvaluationModeRetryItems {
		require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, items[2]).Updates(map[string]any{"expt_run_id": oldRun, "status": int32(entity.ItemRunState_Success)}).Error)
		require.NoError(t, f.conn.Create(&model.ExptItemResult{ID: f.base + 103, SpaceID: f.space, ExptID: f.expt, ExptRunID: oldRun, ItemID: items[3], ItemVersionID: f.version, Status: int32(entity.ItemRunState_Success), Ext: gptr.Of([]byte(`{"other":"keep"}`))}).Error)
		require.NoError(t, f.conn.Create(&model.ExptItemRef{ID: f.base + 303, SpaceID: f.space, ExptID: f.expt, ItemID: items[3], ItemVersionID: f.version}).Error)
	} else {
		// RetryAll's current evaluation set has exactly A/B; do not leave
		// uninitialized C/D in a supposedly normal RetryAll projection.
		require.NoError(t, f.conn.Unscoped().Where("space_id=? AND item_id=?", f.space, f.item+2).Delete(&model.ExptItemResult{}).Error)
		require.NoError(t, f.conn.Unscoped().Where("space_id=? AND item_id=?", f.space, f.item+2).Delete(&model.ExptItemRef{}).Error)
	}
	for i, item := range items[2:] {
		require.NoError(t, f.conn.Create(&model.ExptItemResultRunLog{ID: f.base + 700 + int64(i), SpaceID: f.space, ExptID: f.expt, ExptRunID: oldRun, ItemID: item, ItemVersionID: f.version, Status: int32(entity.ItemRunState_Success), ResultState: gptr.Of(int32(entity.ExptItemResultStateResulted))}).Error)
		require.NoError(t, f.conn.Create(&model.ExptTurnResult{ID: f.base + 460 + int64(i), SpaceID: f.space, ExptID: f.expt, ExptRunID: oldRun, ItemID: item, ItemVersionID: f.version, TurnID: 0, Status: int32(entity.TurnRunState_Success)}).Error)
		require.NoError(t, f.conn.Create(&model.ExptTurnResultRunLog{ID: f.base + 720 + int64(i), SpaceID: f.space, ExptID: f.expt, ExptRunID: oldRun, ItemID: item, ItemVersionID: f.version, TurnID: 0, Status: int32(entity.TurnRunState_Success), EvaluatorResultIds: gptr.Of([]byte(`{}`))}).Error)
	}
	require.NoError(t, f.conn.Model(&model.ExptStats{}).Where("space_id=?", f.space).Updates(map[string]any{"pending_cnt": 0, "processing_cnt": 2, "success_cnt": len(items) - 2, "fail_cnt": 0}).Error)
	factory, server := finishRealLockerFactory(t)
	ids := idmocks.NewMockIIDGenerator(ctrl)
	ids.EXPECT().GenID(gomock.Any()).Return(f.run, nil).AnyTimes()
	var nextID int64 = f.base + 9000
	ids.EXPECT().GenMultiIDs(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, n int) ([]int64, error) {
		out := make([]int64, n)
		for i := range out {
			nextID++
			out[i] = nextID
		}
		return out, nil
	}).AnyTimes()
	config := confmocks.NewMockIConfiger(ctrl)
	config.EXPECT().GetExptExecConf(gomock.Any(), f.space).Return(&entity.ExptExecConf{ZombieIntervalSecond: 300, ExptItemEvalConf: &entity.ExptItemEvalConf{ConcurNum: 4, MaxItemConcurNum: 4}}).AnyTimes()
	idem := idemmocks.NewMockIdempotentService(ctrl)
	idem.EXPECT().Exist(gomock.Any(), gomock.Any()).Return(false, nil).AnyTimes()
	idem.EXPECT().Set(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	runs := exptrepo.NewExptRunLogRepo(exptmysql.NewExptRunLogDAO(f.provider))
	experiments := f.experimentRepo()
	stats := exptrepo.NewExptStatsRepo(exptmysql.NewExptStatsDAO(f.provider))
	result := &ExptResultServiceImpl{ExptItemResultRepo: f.itemRepo(), ExptTurnResultRepo: f.turnRepo(), ExptStatsRepo: stats, idgen: ids, scoreCalculator: NewEvaluatorScoreCalculator(nil, nil), resultPublishRepo: exptrepo.NewExptResultPublishRepo(f.provider), resultPublishLocker: factory}
	manager := &ExptMangerImpl{exptRepo: experiments, runLogRepo: runs, itemResultRepo: f.itemRepo(), turnResultRepo: f.turnRepo(), itemRefRepo: exptrepo.NewExptItemRefRepo(exptmysql.NewExptItemRefDAO(f.provider)), manualRetryRepo: exptrepo.NewExptManualRetryRepo(f.provider, ids), runFinishRepo: exptrepo.NewExptRunFinishRepo(f.provider), manualRetryLocker: factory, mutex: factory(), configer: config, idgenerator: ids, idem: idem}
	expt, err := experiments.GetByID(ctx, f.expt, f.space)
	require.NoError(t, err)
	expt.EvalSet = &entity.EvaluationSet{ID: f.base + 80, ItemCount: int64(len(items)), EvaluationSetVersion: &entity.EvaluationSetVersion{ID: f.base + 81}}
	expt.EvalConf = &entity.EvaluationConfiguration{ItemConcurNum: gptr.Of(4)}
	event := &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ExptRunMode: parent, ExptType: entity.ExptType_Offline, ExecEvalSetItemIDs: append([]int64(nil), items[:2]...), Session: &entity.Session{UserID: "combination-test"}}
	var mode entity.ExptSchedulerMode
	switch parent {
	case entity.EvaluationModeFailRetry:
		mode = &ExptFailRetryExec{manager: manager, exptItemResultRepo: f.itemRepo(), configer: config}
	case entity.EvaluationModeRetryAll:
		mode = &ExptRetryAllExec{manager: manager, exptItemResultRepo: f.itemRepo(), configer: config}
	case entity.EvaluationModeRetryItems:
		dataset := svcmocks.NewMockEvaluationSetItemService(ctrl)
		dataset.EXPECT().BatchGetEvaluationSetItems(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, p *entity.BatchGetEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, error) {
			out := make([]*entity.EvaluationSetItem, 0, len(p.ItemIDs))
			for _, id := range p.ItemIDs {
				out = append(out, &entity.EvaluationSetItem{ItemID: id, ItemVersionID: &f.version, Turns: []*entity.Turn{{ID: 0}}})
			}
			return out, nil
		}).AnyTimes()
		mode = &ExptRetryItemsExec{manager: manager, exptItemResultRepo: f.itemRepo(), exptRunLogRepo: runs, exptTurnResultRepo: f.turnRepo(), exptStatsRepo: stats, configer: config, idgenerator: ids, evaluationSetItemService: dataset}
	default:
		t.Fatalf("unsupported parent mode %d", parent)
	}
	p := &currentRetryCombinationFixture{f: f, manager: manager, result: result, dispatch: exptrepo.NewExptItemDispatchRepo(exptmysql.NewExptItemDispatchDAO(f.provider)), mode: mode, expt: expt, event: event, server: server, parentMode: parent, oldRun: oldRun, items: items}
	if parent == entity.EvaluationModeRetryItems {
		require.NoError(t, server.Set(fmt.Sprintf("expt_run_mutex_lock:%d", f.expt), fmt.Sprint(f.run)))
		joinedRun, joined, err := manager.LogRetryItemsRun(ctx, f.expt, parent, f.space, items[2:], event.Session)
		require.NoError(t, err)
		require.True(t, joined)
		require.Equal(t, f.run, joinedRun)
		require.NoError(t, mode.ScheduleStart(ctx, event, expt))
		require.Equal(t, items, event.ExecEvalSetItemIDs)
		for _, item := range items[2:] {
			current, err := f.itemRepo().GetCurrentRequest(ctx, f.space, f.expt, f.run, item)
			require.NoError(t, err)
			require.Equal(t, entity.ItemRunState_Queueing, current.Status, "Append must actually initialize C/D before acceptance")
			// Raw item execution initializes current-run turn logs after the
			// scheduler materializes the canonical/item rows. Use the same repo
			// write as the existing raw execution fixture, before controlled results.
			logIDs, err := ids.GenMultiIDs(ctx, 1)
			require.NoError(t, err)
			require.NoError(t, f.turnRepo().BatchCreateNXRunLog(ctx, []*entity.ExptTurnResultRunLog{{ID: logIDs[0], SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: item, ItemVersionID: f.version, TurnID: 0, Status: entity.TurnRunState_Queueing}}))
		}
	}
	run := p.identity(t)
	raw, err := json.Marshal(run.ItemIds[0])
	require.NoError(t, err)
	require.JSONEq(t, string(chunks[1:len(chunks)-1]), string(raw), "the original chunk and unknown fields survive a successful Append")
	return p
}

func (p *currentRetryCombinationFixture) request(mode entity.ExptRunMode) *entity.RetryInCurrentRunRequest {
	r := &entity.RetryInCurrentRunRequest{SpaceID: p.f.space, ExptID: p.f.expt, ExpectedRunID: p.f.run, Mode: mode, Session: p.event.Session}
	if mode == entity.EvaluationModeRetryItems {
		r.ItemIDs = []int64{p.f.item}
	}
	return r
}

func (p *currentRetryCombinationFixture) awaitReleased(t *testing.T) {
	t.Helper()
	require.Eventually(t, func() bool {
		return !p.server.Exists(fmt.Sprintf("expt_run_exec_lock:%d:%d", p.f.expt, p.f.run)) && !p.server.Exists(fmt.Sprintf("expt_item_eval_run_lock:%d:%d", p.f.expt, p.f.item))
	}, 2*time.Second, time.Millisecond, "wait only for the real lock's asynchronous cancel/release")
}

func (p *currentRetryCombinationFixture) start(t *testing.T, item int64) *entity.ExptCurrentItemRunSource {
	t.Helper()
	ctx := context.Background()
	current, err := p.f.itemRepo().GetCurrentRequest(ctx, p.f.space, p.f.expt, p.f.run, item)
	require.NoError(t, err)
	if current.Status == entity.ItemRunState_Queueing {
		claimed, err := p.dispatch.ClaimQuotaReserved(ctx, p.f.space, p.f.expt, p.f.run, []int64{item}, map[int64]string{item: current.RequestID})
		require.NoError(t, err)
		require.Len(t, claimed, 1)
		require.Equal(t, current.Mode, claimed[0].Mode)
		require.Equal(t, current.RequestID, claimed[0].RequestID)
		started, err := p.dispatch.StartReservedItem(ctx, p.f.space, p.f.expt, p.f.run, item, current.RequestID)
		require.NoError(t, err)
		require.True(t, started.Started)
	}
	source, err := p.f.port.GetCurrentItemRunSource(ctx, p.f.space, p.f.expt, p.f.run, item)
	require.NoError(t, err)
	require.Equal(t, entity.ItemRunState_Processing, source.Current.Status)
	return source
}

func (p *currentRetryCombinationFixture) completeAndPublish(t *testing.T, item int64, status entity.ItemRunState) *entity.ExptItemResultPublication {
	t.Helper()
	ctx := context.Background()
	source := p.start(t, item)
	turnIDs := make([]int64, 0, len(source.Turns))
	for id := range source.Turns {
		turnIDs = append(turnIDs, id)
	}
	sort.Slice(turnIDs, func(i, j int) bool { return turnIDs[i] < turnIDs[j] })
	for i, id := range turnIDs {
		source, err := p.f.port.GetCurrentItemRunSource(ctx, p.f.space, p.f.expt, p.f.run, item)
		require.NoError(t, err)
		old := source.Turns[id]
		if old.Status == entity.TurnRunState_Success {
			continue // FailRetry's already successful stage is preserved.
		}
		turn := *old
		turn.Status = entity.TurnRunState_Success
		if status == entity.ItemRunState_Fail && i == len(turnIDs)-1 {
			turn.Status = entity.TurnRunState_Fail
		}
		turn.TargetResultID, turn.EvaluatorResultIds = 0, &entity.EvaluatorResults{}
		applied, err := p.f.port.StoreCurrentTurnRunResult(ctx, source, &turn)
		require.NoError(t, err)
		require.True(t, applied)
	}
	source, err := p.f.port.GetCurrentItemRunSource(ctx, p.f.space, p.f.expt, p.f.run, item)
	require.NoError(t, err)
	applied, err := p.f.port.CompleteCurrentItemRun(ctx, source, &entity.ExptCurrentItemRunWrite{Status: status, RetryTimes: source.Current.RetryTimes})
	require.NoError(t, err)
	require.True(t, applied)
	log, err := p.f.itemRepo().GetItemRunLog(ctx, p.f.expt, p.f.run, item, p.f.space)
	require.NoError(t, err)
	require.Equal(t, int32(entity.ExptItemResultStateLogged), log.ResultState)
	publishSource, err := p.result.GetItemResultPublishSource(ctx, p.f.space, p.f.expt, p.f.run, item)
	require.NoError(t, err)
	publication, err := p.result.PrepareItemResultPublication(ctx, publishSource, p.expt)
	require.NoError(t, err)
	require.NotNil(t, publication)
	applied, err = p.result.PublishPreparedItemResult(ctx, publication, nil)
	require.NoError(t, err)
	require.True(t, applied)
	log, err = p.f.itemRepo().GetItemRunLog(ctx, p.f.expt, p.f.run, item, p.f.space)
	require.NoError(t, err)
	require.Equal(t, int32(entity.ExptItemResultStateResulted), log.ResultState)
	turns, err := p.f.turnRepo().GetItemTurnResults(ctx, p.f.expt, item, p.f.space)
	require.NoError(t, err)
	for _, turn := range turns {
		require.Equal(t, p.f.run, turn.ExptRunID, "real Resulted publication must move inherited canonical turns to R")
	}
	p.awaitReleased(t)
	return publication
}

func (p *currentRetryCombinationFixture) identity(t *testing.T) *entity.ExptRunLog {
	t.Helper()
	run, err := p.manager.runLogRepo.Get(context.Background(), p.f.expt, p.f.run)
	require.NoError(t, err)
	require.Equal(t, p.f.run, run.ExptRunID)
	require.Equal(t, int32(p.parentMode), run.Mode)
	expt, err := p.manager.exptRepo.GetByID(context.Background(), p.f.expt, p.f.space)
	require.NoError(t, err)
	require.Equal(t, p.f.run, expt.LatestRunID)
	return run
}

// Exercise old events/writes against the new Processing source. The event
// chain and SQL CAS are production code; the quota guard is the existing
// observable ledger double. Record-specific callback checks are run separately.
func (p *currentRetryCombinationFixture) rejectOldSource(t *testing.T, old *entity.ExptCurrentItemRunSource, publication *entity.ExptItemResultPublication) {
	t.Helper()
	f := p.f
	before := f.snapshot()
	guard := &fakeGuard{confirmResult: true}
	svc := &ExptItemEventEvalServiceImpl{exptItemResultRepo: f.itemRepo(), exptTurnResultRepo: f.turnRepo(), dispatchRepo: p.dispatch, centralGuard: guard, itemLocker: p.manager.manualRetryLocker}
	calls := 0
	handler := RecordEvalChain(svc.HandleEventErr, svc.HandleEventLock, svc.HandleCurrentSource, svc.HandleCentralReservation)(func(context.Context, *entity.ExptItemEvalEvent) error { calls++; return nil })
	for _, kind := range []string{"execution", "target", "evaluator"} {
		event := &entity.ExptItemEvalEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, EvalSetItemID: f.item, ExptRunMode: old.Current.Mode, RequestID: old.Current.RequestID}
		if kind != "execution" {
			event.ResumeSource = &entity.ExptResumeSource{Kind: kind, RecordID: f.base + 41, TurnID: 0}
		}
		require.NoError(t, handler(admittedEnforceCtx(event), event))
	}
	require.Zero(t, calls)
	require.Zero(t, guard.confirmCalls)
	require.Empty(t, guard.releases())
	turn := *old.Turns[0]
	turn.Status = entity.TurnRunState_Fail
	applied, err := f.port.StoreCurrentTurnRunResult(context.Background(), old, &turn)
	require.NoError(t, err)
	require.False(t, applied)
	applied, err = f.port.CompleteCurrentItemRun(context.Background(), old, &entity.ExptCurrentItemRunWrite{Status: entity.ItemRunState_Fail, FailPendingTurns: true})
	require.NoError(t, err)
	require.False(t, applied)
	notifications := 0
	applied, err = p.result.PublishPreparedItemResult(context.Background(), publication, func(context.Context, *entity.ExptItemResultPublishSource) error { notifications++; return nil })
	require.NoError(t, err)
	require.False(t, applied)
	require.Zero(t, notifications)
	event := &entity.ExptItemEvalEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, EvalSetItemID: f.item, RequestID: old.Current.RequestID}
	ctx := context.WithValue(context.Background(), currentItemSourceKey{}, &currentItemExecution{Source: old, Completed: true})
	svc.releaseQuotaIfItemTerminal(ctx, event, testScope, guard, nil)
	require.Empty(t, guard.releases(), "old attempt must not release the new reservation")
	require.Equal(t, before, f.snapshot())
}

// Read the committed storage before controlled completion can skip Success.
// Expected references come from the real prepare plan, including valid target
// and evaluator stages retained by FailRetry; they are not cleared wholesale.
func (p *currentRetryCombinationFixture) assertRequeuedTurns(t *testing.T, item *entity.PreparedRetryItem, mode entity.ExptRunMode) {
	t.Helper()
	current := p.f.source()
	require.Len(t, current.CanonicalTurns, len(item.Turns))
	require.Len(t, current.Turns, len(item.Turns))
	canonical := make(map[int64]*entity.ExptTurnResult, len(current.CanonicalTurns))
	for _, turn := range current.CanonicalTurns {
		canonical[turn.TurnID] = turn
	}
	var expectedRefIDs, actualRefIDs []int64
	for _, prepared := range item.Turns {
		before := prepared.SourceTurn
		turn, log := canonical[before.TurnID], current.Turns[before.TurnID]
		require.NotNil(t, turn)
		require.NotNil(t, log)
		require.Equal(t, before.ID, turn.ID)
		require.Equal(t, p.f.run, turn.ExptRunID)
		require.Equal(t, p.f.run, log.ExptRunID)
		expectedState := entity.TurnRunState_Queueing
		expectedResults := prepared.EvaluatorResults
		if mode == entity.EvaluationModeRetryItems {
			require.False(t, prepared.PreserveSuccess, "manual item retry must rerun every prepared turn")
			require.Zero(t, prepared.TargetResultID)
			require.Empty(t, prepared.KeepRefIDs)
			require.True(t, entity.SameCurrentEvaluatorRefs(prepared.EvaluatorResults, nil))
		} else {
			require.Equal(t, before.Status == int32(entity.TurnRunState_Success), prepared.PreserveSuccess)
		}
		if prepared.PreserveSuccess {
			expectedState = entity.TurnRunState_Success
			require.Equal(t, before.TargetResultID, turn.TargetResultID)
			require.Equal(t, before.WeightedScore, turn.WeightedScore)
			if prepared.CurrentRunLog != nil {
				expectedResults = prepared.CurrentRunLog.EvaluatorResultIds
			}
		} else {
			require.Nil(t, turn.WeightedScore)
			require.Empty(t, turn.ErrMsg)
			require.Empty(t, log.ErrMsg)
		}
		require.Equal(t, int32(expectedState), turn.Status, "canonical turn reset must be correct before the helper runs")
		require.Equal(t, expectedState, log.Status, "run-log turn reset must be correct before the helper runs")
		require.Equal(t, prepared.TargetResultID, turn.TargetResultID)
		require.Equal(t, prepared.TargetResultID, log.TargetResultID)
		require.True(t, entity.SameCurrentEvaluatorRefs(expectedResults, log.EvaluatorResultIds), "run-log references must match the prepared retry contract")
		expectedRefIDs = append(expectedRefIDs, prepared.KeepRefIDs...)
	}
	for _, ref := range current.CurrentRefs {
		actualRefIDs = append(actualRefIDs, ref.ID)
	}
	require.ElementsMatch(t, expectedRefIDs, actualRefIDs, "stored references must retain only the prepared IDs")
	t.Logf("COMBINATIONS_TURN_RESET mode=%d turns=%d preserved_refs=%d committed_storage_checked=true", mode, len(item.Turns), len(actualRefIDs))
}

func TestCurrentRetryCombinationsParentModesAndContinuousRequests(t *testing.T) {
	for _, parent := range []entity.ExptRunMode{entity.EvaluationModeFailRetry, entity.EvaluationModeRetryAll, entity.EvaluationModeRetryItems} {
		for _, requestMode := range []entity.ExptRunMode{entity.EvaluationModeRetryItems, entity.EvaluationModeFailRetry} {
			t.Run(fmt.Sprintf("parent%d/request%d", parent, requestMode), func(t *testing.T) {
				nextMode := entity.EvaluationModeFailRetry
				if requestMode == entity.EvaluationModeFailRetry {
					nextMode = entity.EvaluationModeRetryItems
				}
				runCurrentRetryCombination(t, parent, requestMode, nextMode)
			})
		}
	}
}

func TestCurrentRetryCombinationsContinuousSameMode(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeRetryItems, entity.EvaluationModeFailRetry} {
		t.Run(fmt.Sprintf("parent%d/mode%d", mode, mode), func(t *testing.T) {
			runCurrentRetryCombination(t, mode, mode, mode)
		})
	}
}

func runCurrentRetryCombination(t *testing.T, parent, requestMode, nextMode entity.ExptRunMode) {
	t.Helper()
	p := newCurrentRetryCombinationFixture(t, parent)
	f := p.f
	initialStatus := entity.ItemRunState_Success
	if requestMode == entity.EvaluationModeFailRetry {
		initialStatus = entity.ItemRunState_Fail
	}
	publication := p.completeAndPublish(t, f.item, initialStatus)
	originalChunks := p.identity(t).ItemIds
	otherItems, err := f.itemRepo().BatchGet(context.Background(), f.space, f.expt, p.items[1:])
	require.NoError(t, err)
	old := f.source()
	req := p.request(requestMode)
	frozen, err := p.manager.prepareTerminalCandidatesAndReusableRefs(context.Background(), p.expt, f.run, req.Mode, req.ItemIDs, nil)
	require.NoError(t, err)
	require.Len(t, frozen.Items, 1)
	receipt, err := p.manager.RetryInCurrentRun(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, int32(1), receipt.AcceptedCount)
	require.Equal(t, f.run, receipt.RunID)
	p.awaitReleased(t)
	q1 := f.source().Current
	require.NotEmpty(t, q1.RequestID)
	require.NotEqual(t, old.Current.RequestID, q1.RequestID)
	require.Equal(t, req.Mode, q1.Mode)
	require.Equal(t, entity.ItemRunState_Queueing, q1.Status)
	p.assertRequeuedTurns(t, frozen.Items[0], requestMode)
	require.Equal(t, originalChunks, p.identity(t).ItemIds)
	// The same frozen terminal source loses to Q1, both before Start
	// and after Start. A new request ID cannot bypass the SQL gate.
	for _, stage := range []string{"queueing", "processing"} {
		before := f.snapshot()
		stale := &entity.RequeueTerminalItemsRequest{SpaceID: f.space, ExptID: f.expt, RunID: f.run, RequestID: "old-snapshot-" + stage, Mode: requestMode, Items: frozen.Items, SnapshotAtMS: frozen.SnapshotAtMS}
		duplicate, err := p.manager.manualRetryRepo.RequeueTerminalItems(context.Background(), stale)
		require.NoError(t, err)
		require.Zero(t, duplicate.AcceptedCount)
		require.NotEmpty(t, duplicate.RejectedItems)
		duplicate, err = p.manager.RetryInCurrentRun(context.Background(), p.request(entity.EvaluationModeRetryItems))
		require.NoError(t, err)
		require.Zero(t, duplicate.AcceptedCount)
		require.Equal(t, stage, duplicate.RejectedItems[0].Reason)
		p.awaitReleased(t)
		require.Equal(t, before, f.snapshot())
		if stage == "queueing" {
			if parent == entity.EvaluationModeRetryItems {
				staleTick := *p.event
				staleTick.ExecEvalSetItemIDs = append([]int64(nil), p.items[1:]...)
				require.NoError(t, p.mode.ScheduleStart(context.Background(), &staleTick, p.expt))
				require.Equal(t, p.items, staleTick.ExecEvalSetItemIDs)
				require.Equal(t, before, f.snapshot(), "old RetryItems snapshot must not reset materialized Q1")
			}
			toSubmit, incomplete, _, err := p.mode.ScanEvalItems(context.Background(), p.event, p.expt)
			require.NoError(t, err)
			found := false
			for _, item := range toSubmit {
				if item.ItemID == f.item {
					found = true
					require.Equal(t, q1.RequestID, item.RequestID)
					require.Equal(t, requestMode, item.Mode)
				}
			}
			require.True(t, found, "the real parent mode's next scan must see Q1")
			require.Contains(t, entity.ExptEvalItems(incomplete).GetItemIDs(), p.items[1])
			p.start(t, f.item)
		}
	}
	p.rejectOldSource(t, old, publication)
	nextStatus := entity.ItemRunState_Success
	if nextMode == entity.EvaluationModeFailRetry {
		nextStatus = entity.ItemRunState_Fail
	}
	publication = p.completeAndPublish(t, f.item, nextStatus)
	old = f.source()
	nextRequest := p.request(nextMode)
	nextPlan, err := p.manager.prepareTerminalCandidatesAndReusableRefs(context.Background(), p.expt, f.run, nextMode, nextRequest.ItemIDs, nil)
	require.NoError(t, err)
	require.Len(t, nextPlan.Items, 1)
	receipt, err = p.manager.RetryInCurrentRun(context.Background(), nextRequest)
	require.NoError(t, err)
	require.Equal(t, int32(1), receipt.AcceptedCount, "Resulted Q1 permits a legitimate Q2")
	p.awaitReleased(t)
	q2 := f.source().Current
	require.NotEqual(t, q1.RequestID, q2.RequestID)
	require.Equal(t, nextMode, q2.Mode)
	require.Equal(t, entity.ItemRunState_Queueing, q2.Status)
	p.assertRequeuedTurns(t, nextPlan.Items[0], nextMode)
	if nextMode == requestMode {
		require.NotEmpty(t, old.Current.RequestID, "the stale Q1 source must have a real nonempty RequestID")
		require.Equal(t, old.Current.Mode, q2.Mode, "same-mode isolation must reject the stale RequestID")
	}
	require.Equal(t, originalChunks, p.identity(t).ItemIds)
	p.start(t, f.item)
	p.rejectOldSource(t, old, publication)
	afterOthers, err := f.itemRepo().BatchGet(context.Background(), f.space, f.expt, p.items[1:])
	require.NoError(t, err)
	require.Equal(t, otherItems, afterOthers, "A retries must not mutate B/C/D canonical state")
	t.Logf("COMBINATIONS_REAL_MYSQL parent=%d Q1_mode=%d Q2_mode=%d run_and_chunks_preserved=true old_source_noop=true old_reservation_release=0", parent, requestMode, nextMode)
}

func TestCurrentRetryCombinationsCompletionOrdering(t *testing.T) {
	for _, parent := range []entity.ExptRunMode{entity.EvaluationModeFailRetry, entity.EvaluationModeRetryAll, entity.EvaluationModeRetryItems} {
		for _, first := range []string{"retry", "complete"} {
			t.Run(fmt.Sprintf("parent%d/%s_first", parent, first), func(t *testing.T) {
				p := newCurrentRetryCombinationFixture(t, parent)
				p.completeAndPublish(t, p.f.item, entity.ItemRunState_Success)
				p.completeAndPublish(t, p.items[1], entity.ItemRunState_Success)
				if parent == entity.EvaluationModeRetryItems {
					for _, item := range p.items[2:] {
						p.completeAndPublish(t, item, entity.ItemRunState_Success)
					}
				}
				// Use the atomic repository primitive, independent of the
				// service Finish/Terminal orchestration being removed elsewhere.
				port := exptmysql.NewExptRunFinishDAO(p.f.provider)
				request := &entity.ExptRunFinishRequest{SpaceID: p.f.space, ExptID: p.f.expt, RunID: p.f.run}
				if first == "retry" {
					receipt, err := p.manager.RetryInCurrentRun(context.Background(), p.request(entity.EvaluationModeRetryItems))
					require.NoError(t, err)
					require.Equal(t, int32(1), receipt.AcceptedCount)
					p.awaitReleased(t)
					before := p.f.snapshot()
					result, err := port.FinishCurrentRun(context.Background(), request)
					require.NoError(t, err)
					require.Equal(t, entity.ExptRunFinishPending, result.Disposition)
					require.Equal(t, before, p.f.snapshot())
				} else {
					result, err := port.FinishCurrentRun(context.Background(), request)
					require.NoError(t, err)
					require.Equal(t, entity.ExptRunFinishFinished, result.Disposition)
					before := p.f.snapshot()
					receipt, err := p.manager.RetryInCurrentRun(context.Background(), p.request(entity.EvaluationModeRetryItems))
					require.ErrorContains(t, err, "run_finished")
					require.Nil(t, receipt)
					require.Equal(t, before, p.f.snapshot())
				}
				p.identity(t)
				t.Logf("COMBINATIONS_REAL_MYSQL parent=%d order=%s_first valid_commit_order=true", parent, first)
			})
		}
	}
}

func TestCurrentRetryCombinationsBusyRunLockDoesNotMutate(t *testing.T) {
	for _, parent := range []entity.ExptRunMode{entity.EvaluationModeFailRetry, entity.EvaluationModeRetryAll, entity.EvaluationModeRetryItems} {
		t.Run(fmt.Sprint(parent), func(t *testing.T) {
			p := newCurrentRetryCombinationFixture(t, parent)
			p.completeAndPublish(t, p.f.item, entity.ItemRunState_Success)
			key := fmt.Sprintf("expt_run_exec_lock:%d:%d", p.f.expt, p.f.run)
			require.NoError(t, p.server.Set(key, "another-tick"))
			p.server.SetTTL(key, time.Minute)
			before := p.f.snapshot()
			receipt, err := p.manager.RetryInCurrentRun(context.Background(), p.request(entity.EvaluationModeRetryItems))
			require.Error(t, err)
			require.True(t, strings.Contains(err.Error(), "busy"), "%v", err)
			require.Nil(t, receipt)
			require.Equal(t, before, p.f.snapshot())
			owner, err := p.server.Get(key)
			require.NoError(t, err)
			require.Equal(t, "another-tick", owner)
			t.Logf("COMBINATIONS_REAL_MYSQL parent=%d run_tick_busy_zero_mutation=true", parent)
		})
	}
}
