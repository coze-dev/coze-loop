// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	idemMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	metricsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	componentMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	entityMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity/mocks"
	eventsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

const (
	terminateTestExptID    = int64(1001)
	terminateTestRunID     = int64(2002)
	terminateTestSpaceID   = int64(7)
	terminateTestTargetSpc = int64(0)
)

// terminateRunLog 构造指定状态的 item run log。
func terminateRunLog(itemID int64, state entity.ItemRunState) *entity.ExptItemResultRunLog {
	return &entity.ExptItemResultRunLog{
		SpaceID:   terminateTestSpaceID,
		ExptID:    terminateTestExptID,
		ExptRunID: terminateTestRunID,
		ItemID:    itemID,
		Status:    int32(state),
	}
}

// TestExptMangerImpl_TerminateItems_HappyPath 正常路径（tasks 6.1 + 6.2）：
//   - item run log 落 Terminal + Logged + 非空 err_msg
//   - turn run log 与之成对写 TurnRunState_Terminal（design D3）
//   - **主表 ufields 只含 err_msg、不含 status**（design D2 回归防线）
func TestExptMangerImpl_TerminateItems_HappyPath(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "test_user"}
	itemIDs := []int64{11, 12}

	itemRepo := mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo)
	turnRepo := mgr.turnResultRepo.(*repoMocks.MockIExptTurnResultRepo)
	exptRepo := mgr.exptRepo.(*repoMocks.MockIExperimentRepo)
	resultSvc := mgr.exptResultService.(*svcMocks.MockExptResultService)
	targetSvc := mgr.evalTargetService.(*svcMocks.MockIEvalTargetService)

	itemRepo.EXPECT().
		MGetItemRunLog(ctx, terminateTestExptID, terminateTestRunID, itemIDs, terminateTestSpaceID).
		Return([]*entity.ExptItemResultRunLog{
			terminateRunLog(11, entity.ItemRunState_Processing),
			terminateRunLog(12, entity.ItemRunState_Queueing),
		}, nil)

	// ② item run log: Terminal + Logged + err_msg
	itemRepo.EXPECT().
		UpdateItemRunLog(ctx, terminateTestExptID, terminateTestRunID, []int64{11, 12}, gomock.Any(), terminateTestSpaceID).
		DoAndReturn(func(_ context.Context, _, _ int64, _ []int64, ufields map[string]any, _ int64) error {
			assert.Equal(t, int32(entity.ItemRunState_Terminal), ufields["status"])
			assert.Equal(t, int32(entity.ExptItemResultStateLogged), ufields["result_state"])
			assert.NotEmpty(t, ufields["err_msg"], "err_msg 必须落库，前端要展示终止原因")
			return nil
		})

	// ③ turn run log 与 ② 成对（D3）
	turnRepo.EXPECT().
		CreateOrUpdateItemsTurnRunLogStatus(ctx, terminateTestSpaceID, terminateTestExptID, terminateTestRunID, []int64{11, 12}, entity.TurnRunState_Terminal).
		Return(nil)

	// ④ 主表只补 err_msg —— D2 回归防线：一旦有人加回 status，statsCntOp 的 Processing 减项会丢，
	// processing_cnt 永远归不了零，实验永远不完成。
	itemRepo.EXPECT().
		UpdateItemsResult(ctx, terminateTestSpaceID, terminateTestExptID, []int64{11, 12}, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ int64, _ []int64, ufields map[string]any) error {
			assert.NotEmpty(t, ufields["err_msg"])
			_, hasStatus := ufields["status"]
			assert.False(t, hasStatus, "D2: 主表 UpdateItemsResult 禁止预写 status")
			assert.Len(t, ufields, 1, "D2: 主表 ufields 只允许 err_msg 一个字段")
			return nil
		})

	// ⑤ 读侧 turn result
	turnRepo.EXPECT().
		BatchGet(ctx, terminateTestSpaceID, terminateTestExptID, []int64{11, 12}).
		Return([]*entity.ExptTurnResult{
			{ItemID: 11, TurnID: 101},
			{ItemID: 12, TurnID: 102},
		}, nil)
	turnRepo.EXPECT().
		UpdateTurnResults(ctx, terminateTestExptID, gomock.Any(), terminateTestSpaceID, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ int64, itemTurnIDs []*entity.ItemTurnID, _ int64, ufields map[string]any) error {
			assert.Equal(t, int32(entity.TurnRunState_Terminal), ufields["status"])
			assert.Len(t, itemTurnIDs, 2)
			return nil
		})

	// ⑥ CK 读侧刷新
	resultSvc.EXPECT().
		UpsertExptTurnResultFilter(ctx, terminateTestSpaceID, terminateTestExptID, []int64{11, 12}).
		Return(nil)

	// ⑦ 沙箱释放
	exptRepo.EXPECT().
		GetByID(ctx, terminateTestExptID, terminateTestSpaceID).
		Return(&entity.Experiment{ID: terminateTestExptID, SpaceID: terminateTestSpaceID, TargetSpaceID: terminateTestTargetSpc}, nil)
	turnRepo.EXPECT().
		MGetItemTurnRunLogs(ctx, terminateTestExptID, terminateTestRunID, []int64{11, 12}, terminateTestSpaceID).
		Return([]*entity.ExptTurnResultRunLog{{ItemID: 11, TargetResultID: 9001}}, nil)
	targetSvc.EXPECT().
		TerminateAsyncRecordsAndDestroySandbox(ctx, terminateTestSpaceID, []int64{9001}, gomock.Any(), gomock.Any(), false).
		Times(1)

	require.NoError(t, mgr.TerminateItems(ctx, terminateTestExptID, terminateTestRunID, terminateTestSpaceID, itemIDs, session))
}

// TestExptMangerImpl_TerminateItems_Idempotent 幂等（tasks 6.3）：
// 混合列表只处理可终止行；全部已终态时不产生任何写调用且返回 nil。
func TestExptMangerImpl_TerminateItems_Idempotent(t *testing.T) {
	ctx := context.Background()
	session := &entity.Session{UserID: "test_user"}

	t.Run("mixed_list_only_processes_unfinished", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mgr := newTestExptManager(ctrl)

		itemRepo := mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo)
		turnRepo := mgr.turnResultRepo.(*repoMocks.MockIExptTurnResultRepo)
		exptRepo := mgr.exptRepo.(*repoMocks.MockIExperimentRepo)
		resultSvc := mgr.exptResultService.(*svcMocks.MockExptResultService)

		// 21=已终止 / 22=成功 / 23=失败 / 24=执行中 → 只有 24 该被处理
		itemIDs := []int64{21, 22, 23, 24}
		itemRepo.EXPECT().
			MGetItemRunLog(ctx, terminateTestExptID, terminateTestRunID, itemIDs, terminateTestSpaceID).
			Return([]*entity.ExptItemResultRunLog{
				terminateRunLog(21, entity.ItemRunState_Terminal),
				terminateRunLog(22, entity.ItemRunState_Success),
				terminateRunLog(23, entity.ItemRunState_Fail),
				terminateRunLog(24, entity.ItemRunState_Processing),
			}, nil)

		onlyLive := []int64{24}
		itemRepo.EXPECT().
			UpdateItemRunLog(ctx, terminateTestExptID, terminateTestRunID, onlyLive, gomock.Any(), terminateTestSpaceID).
			Return(nil)
		turnRepo.EXPECT().
			CreateOrUpdateItemsTurnRunLogStatus(ctx, terminateTestSpaceID, terminateTestExptID, terminateTestRunID, onlyLive, entity.TurnRunState_Terminal).
			Return(nil)
		itemRepo.EXPECT().
			UpdateItemsResult(ctx, terminateTestSpaceID, terminateTestExptID, onlyLive, gomock.Any()).
			Return(nil)
		turnRepo.EXPECT().
			BatchGet(ctx, terminateTestSpaceID, terminateTestExptID, onlyLive).
			Return([]*entity.ExptTurnResult{{ItemID: 24, TurnID: 240}}, nil)
		turnRepo.EXPECT().
			UpdateTurnResults(ctx, terminateTestExptID, gomock.Any(), terminateTestSpaceID, gomock.Any()).
			Return(nil)
		resultSvc.EXPECT().
			UpsertExptTurnResultFilter(ctx, terminateTestSpaceID, terminateTestExptID, onlyLive).
			Return(nil)
		exptRepo.EXPECT().
			GetByID(ctx, terminateTestExptID, terminateTestSpaceID).
			Return(&entity.Experiment{ID: terminateTestExptID, SpaceID: terminateTestSpaceID}, nil)
		turnRepo.EXPECT().
			MGetItemTurnRunLogs(ctx, terminateTestExptID, terminateTestRunID, onlyLive, terminateTestSpaceID).
			Return(nil, nil)

		require.NoError(t, mgr.TerminateItems(ctx, terminateTestExptID, terminateTestRunID, terminateTestSpaceID, itemIDs, session))
	})

	t.Run("all_finished_no_write_at_all", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mgr := newTestExptManager(ctrl)

		itemIDs := []int64{31, 32}
		mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo).EXPECT().
			MGetItemRunLog(ctx, terminateTestExptID, terminateTestRunID, itemIDs, terminateTestSpaceID).
			Return([]*entity.ExptItemResultRunLog{
				terminateRunLog(31, entity.ItemRunState_Terminal),
				terminateRunLog(32, entity.ItemRunState_Success),
			}, nil)
		// 不注册任何写调用期望 —— 一旦有写就会被 gomock 判为 Unexpected call

		require.NoError(t, mgr.TerminateItems(ctx, terminateTestExptID, terminateTestRunID, terminateTestSpaceID, itemIDs, session))
	})

	t.Run("empty_item_ids_returns_nil", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mgr := newTestExptManager(ctrl)

		require.NoError(t, mgr.TerminateItems(ctx, terminateTestExptID, terminateTestRunID, terminateTestSpaceID, nil, session))
	})
}

// TestExptMangerImpl_TerminateItems_BestEffort best-effort（tasks 6.4）：
// CK 刷新失败、实验读取失败、沙箱销毁链路失败，均不影响 TerminateItems 返回 nil。
func TestExptMangerImpl_TerminateItems_BestEffort(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mgr := newTestExptManager(ctrl)
	ctx := context.Background()
	session := &entity.Session{UserID: "test_user"}
	itemIDs := []int64{41}

	itemRepo := mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo)
	turnRepo := mgr.turnResultRepo.(*repoMocks.MockIExptTurnResultRepo)

	itemRepo.EXPECT().MGetItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*entity.ExptItemResultRunLog{terminateRunLog(41, entity.ItemRunState_Processing)}, nil)
	itemRepo.EXPECT().UpdateItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	turnRepo.EXPECT().CreateOrUpdateItemsTurnRunLogStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	itemRepo.EXPECT().UpdateItemsResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	turnRepo.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*entity.ExptTurnResult{{ItemID: 41, TurnID: 410}}, nil)
	turnRepo.EXPECT().UpdateTurnResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	// ⑥ CK 刷新失败 → 仅日志
	mgr.exptResultService.(*svcMocks.MockExptResultService).EXPECT().
		UpsertExptTurnResultFilter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("ck unavailable"))

	// ⑦ 沙箱释放链路失败 → 仅日志
	mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().
		GetByID(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&entity.Experiment{ID: terminateTestExptID, SpaceID: terminateTestSpaceID}, nil)
	turnRepo.EXPECT().MGetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("db down"))

	require.NoError(t, mgr.TerminateItems(ctx, terminateTestExptID, terminateTestRunID, terminateTestSpaceID, itemIDs, session),
		"资源释放失败 MUST NOT 导致终止请求失败")
}

// TestExptMangerImpl_TerminateItems_WriteErrPropagates ②③④⑤ 任一失败必须向上抛错，
// 让前端提示失败、用户可安全重试（①的幂等过滤保证重放安全）。
func TestExptMangerImpl_TerminateItems_WriteErrPropagates(t *testing.T) {
	ctx := context.Background()
	session := &entity.Session{UserID: "test_user"}
	itemIDs := []int64{51}

	t.Run("update_item_run_log_err", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mgr := newTestExptManager(ctrl)

		itemRepo := mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo)
		itemRepo.EXPECT().MGetItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]*entity.ExptItemResultRunLog{terminateRunLog(51, entity.ItemRunState_Processing)}, nil)
		itemRepo.EXPECT().UpdateItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(errors.New("db down"))

		assert.Error(t, mgr.TerminateItems(ctx, terminateTestExptID, terminateTestRunID, terminateTestSpaceID, itemIDs, session))
	})

	t.Run("mget_run_log_err", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mgr := newTestExptManager(ctrl)

		mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo).EXPECT().
			MGetItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, errors.New("db down"))

		assert.Error(t, mgr.TerminateItems(ctx, terminateTestExptID, terminateTestRunID, terminateTestSpaceID, itemIDs, session))
	})

	t.Run("turn_run_log_err", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mgr := newTestExptManager(ctrl)

		itemRepo := mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo)
		itemRepo.EXPECT().MGetItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]*entity.ExptItemResultRunLog{terminateRunLog(51, entity.ItemRunState_Processing)}, nil)
		itemRepo.EXPECT().UpdateItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
		mgr.turnResultRepo.(*repoMocks.MockIExptTurnResultRepo).EXPECT().
			CreateOrUpdateItemsTurnRunLogStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(errors.New("db down"))

		assert.Error(t, mgr.TerminateItems(ctx, terminateTestExptID, terminateTestRunID, terminateTestSpaceID, itemIDs, session))
	})
}

// TestRecordEvalItemRunLogs_AllowsTerminalState 调度白名单放开 Terminal（tasks 6.5，design D4）：
// Terminal 不再报错（否则调度 tick 每拍失败、实验卡死），且 **不触发 sendItemComplete**
// （下游只消费成功行；itemCompletePublisher 为 stub，断言其 events 为空）。
func TestRecordEvalItemRunLogs_AllowsTerminalState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	stub := &stubItemCompletePublisher{}
	resultSvc := svcMocks.NewMockExptResultService(ctrl)
	publisher := eventsMocks.NewMockExptEventPublisher(ctrl)
	mode := entityMocks.NewMockExptSchedulerMode(ctrl)

	svc := &ExptSchedulerImpl{
		itemCompletePublisher: stub,
		ResultSvc:             resultSvc,
		Publisher:             publisher,
	}

	event := &entity.ExptScheduleEvent{SpaceID: 9, ExptID: 100, ExptRunID: 200}
	items := []*entity.ExptEvalItem{{ItemID: 10, State: entity.ItemRunState_Terminal}}

	resultSvc.EXPECT().RecordItemRunLogs(gomock.Any(), int64(100), int64(200), int64(10), int64(9), gomock.Any()).
		Return(nil, nil).Times(1)
	mode.EXPECT().PublishResult(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
	resultSvc.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)
	publisher.EXPECT().PublishExptTurnResultFilterEvent(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)

	err := svc.recordEvalItemRunLogs(context.Background(), event, items, mode, &entity.Experiment{})
	assert.NoError(t, err, "D4: Terminal 必须在白名单内，否则调度 tick 报错、实验卡死")
	assert.Empty(t, stub.events, "Terminal 不是成功行，MUST NOT 发 item-complete")
}

// TestRecordEvalItemRunLogs_RejectsNonTerminalState 白名单只放开三个终态，其余（如 Processing）仍必须报错。
func TestRecordEvalItemRunLogs_RejectsNonTerminalState(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svc := &ExptSchedulerImpl{ResultSvc: svcMocks.NewMockExptResultService(ctrl)}
	event := &entity.ExptScheduleEvent{SpaceID: 9, ExptID: 100, ExptRunID: 200}
	items := []*entity.ExptEvalItem{{ItemID: 10, State: entity.ItemRunState_Processing}}

	err := svc.recordEvalItemRunLogs(context.Background(), event, items, entityMocks.NewMockExptSchedulerMode(ctrl), &entity.Experiment{})
	assert.ErrorContains(t, err, "invalid item run state")
}

// TestExptMangerImpl_CompleteExpt_TerminatedNotFailed 终态推导双向覆盖（tasks 6.6，design D5）：
//   - 有 terminated 行、无 fail 行 → Success（不能因为用户主动终止就把实验判失败）
//   - 有 fail 行 → 仍 Failed（不误伤既有失败判定）
func TestExptMangerImpl_CompleteExpt_TerminatedNotFailed(t *testing.T) {
	ctx := context.Background()
	session := &entity.Session{UserID: "test_user"}
	const exptID, spaceID = int64(123), int64(789)
	runID := int64(456)

	t.Run("terminated_without_fail_is_success", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mgr := newTestExptManager(ctrl)

		var captured *entity.Experiment
		mgr.idem.(*idemMocks.MockIdempotentService).EXPECT().Exist(gomock.Any(), gomock.Any()).AnyTimes().Return(false, nil)
		mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().GetByID(ctx, exptID, spaceID).
			Return(&entity.Experiment{ID: exptID, SpaceID: spaceID, ExptType: entity.ExptType_Offline}, nil)
		mgr.exptResultService.(*svcMocks.MockExptResultService).EXPECT().CalculateStats(ctx, exptID, spaceID, session).
			Return(&entity.ExptCalculateStats{SuccessItemCnt: 253, TerminatedItemCnt: 2}, nil)
		mgr.exptResultService.(*svcMocks.MockExptResultService).EXPECT().GetIncompleteTurns(ctx, exptID, spaceID, session).
			Return([]*entity.ItemTurnID{}, nil)
		mgr.statsRepo.(*repoMocks.MockIExptStatsRepo).EXPECT().UpdateByExptID(ctx, exptID, spaceID, gomock.Any()).Return(nil)
		mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().Update(ctx, gomock.Any()).
			DoAndReturn(func(_ context.Context, e *entity.Experiment) error { captured = e; return nil })
		mgr.quotaRepo.(*repoMocks.MockQuotaRepo).EXPECT().CreateOrUpdate(ctx, spaceID, gomock.Any(), session).Return(nil)
		mgr.exptAggrResultService.(*svcMocks.MockExptAggrResultService).EXPECT().
			PublishExptAggrResultEvent(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(nil)
		mgr.publisher.(*eventsMocks.MockExptEventPublisher).EXPECT().
			PublishExptLifecycleEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(nil)
		mgr.mtr.(*metricsMocks.MockExptMetric).EXPECT().
			EmitExptExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

		require.NoError(t, mgr.CompleteExpt(ctx, exptID, &runID, spaceID, session))
		require.NotNil(t, captured)
		assert.Equal(t, entity.ExptStatus_Success, captured.Status,
			"D5: terminated 是用户主动放弃，不参与 Failed 推导")
	})

	t.Run("fail_still_failed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		mgr := newTestExptManager(ctrl)

		var captured *entity.Experiment
		mgr.idem.(*idemMocks.MockIdempotentService).EXPECT().Exist(gomock.Any(), gomock.Any()).AnyTimes().Return(false, nil)
		mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().GetByID(ctx, exptID, spaceID).
			Return(&entity.Experiment{ID: exptID, SpaceID: spaceID, ExptType: entity.ExptType_Offline}, nil)
		mgr.exptResultService.(*svcMocks.MockExptResultService).EXPECT().CalculateStats(ctx, exptID, spaceID, session).
			Return(&entity.ExptCalculateStats{SuccessItemCnt: 250, FailItemCnt: 3, TerminatedItemCnt: 2}, nil)
		mgr.exptResultService.(*svcMocks.MockExptResultService).EXPECT().GetIncompleteTurns(ctx, exptID, spaceID, session).
			Return([]*entity.ItemTurnID{}, nil)
		mgr.statsRepo.(*repoMocks.MockIExptStatsRepo).EXPECT().UpdateByExptID(ctx, exptID, spaceID, gomock.Any()).Return(nil)
		mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().Update(ctx, gomock.Any()).
			DoAndReturn(func(_ context.Context, e *entity.Experiment) error { captured = e; return nil })
		mgr.quotaRepo.(*repoMocks.MockQuotaRepo).EXPECT().CreateOrUpdate(ctx, spaceID, gomock.Any(), session).Return(nil)
		mgr.exptAggrResultService.(*svcMocks.MockExptAggrResultService).EXPECT().
			PublishExptAggrResultEvent(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(nil)
		mgr.publisher.(*eventsMocks.MockExptEventPublisher).EXPECT().
			PublishExptLifecycleEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(nil)
		mgr.mtr.(*metricsMocks.MockExptMetric).EXPECT().
			EmitExptExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

		require.NoError(t, mgr.CompleteExpt(ctx, exptID, &runID, spaceID, session))
		require.NotNil(t, captured)
		assert.Equal(t, entity.ExptStatus_Failed, captured.Status, "有 fail 行仍必须判 Failed")
	})
}

// TestCompleteItemRun_TerminalNotOverwritten Terminal 覆盖保护（tasks 6.7）：
// 已 Terminal 的行，在途执行结果只写 result_state，MUST NOT 回写 status / err_msg。
func TestCompleteItemRun_TerminalNotOverwritten(t *testing.T) {
	newExec := func(ctrl *gomock.Controller, curState entity.ItemRunState) (*ExptItemEvalCtxExecutor, *repoMocks.MockIExptItemResultRepo) {
		itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
		configer := componentMocks.NewMockIConfiger(ctrl)
		configer.EXPECT().GetErrRetryConf(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(&entity.RetryConf{})
		itemRepo.EXPECT().GetItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			AnyTimes().Return(&entity.ExptItemResultRunLog{Status: int32(curState)}, nil)
		return &ExptItemEvalCtxExecutor{ItemResultRepo: itemRepo, Configer: configer}, itemRepo
	}
	eiec := func() *entity.ExptItemEvalCtx {
		return &entity.ExptItemEvalCtx{
			Event: &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, EvalSetItemID: 3, SpaceID: 4, RetryTimes: 1},
			Expt:  &entity.Experiment{ID: 1},
		}
	}

	t.Run("terminal_success_result_does_not_overwrite", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		exec, itemRepo := newExec(ctrl, entity.ItemRunState_Terminal)

		itemRepo.EXPECT().UpdateItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, _, _ int64, _ []int64, ufields map[string]any, _ int64) error {
				_, hasStatus := ufields["status"]
				assert.False(t, hasStatus, "Terminal 是吸收态，成功结果 MUST NOT 回写 status")
				assert.Equal(t, entity.ExptItemResultStateLogged, ufields["result_state"])
				return nil
			})
		require.NoError(t, exec.CompleteItemRun(context.Background(), eiec(), nil))
	})

	t.Run("terminal_fail_result_does_not_overwrite", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		exec, itemRepo := newExec(ctrl, entity.ItemRunState_Terminal)

		itemRepo.EXPECT().UpdateItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, _, _ int64, _ []int64, ufields map[string]any, _ int64) error {
				_, hasStatus := ufields["status"]
				_, hasErrMsg := ufields["err_msg"]
				assert.False(t, hasStatus, "Terminal 是吸收态，失败结果 MUST NOT 回写 status")
				assert.False(t, hasErrMsg, "err_msg 也不覆盖，用户要看到的是被主动终止")
				return nil
			})
		require.NoError(t, exec.CompleteItemRun(context.Background(), eiec(), errors.New("target boom")))
	})

	t.Run("non_terminal_writes_status_as_before", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		exec, itemRepo := newExec(ctrl, entity.ItemRunState_Processing)

		itemRepo.EXPECT().UpdateItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, _, _ int64, _ []int64, ufields map[string]any, _ int64) error {
				assert.Equal(t, int32(entity.ItemRunState_Success), ufields["status"], "非 Terminal 行为不变")
				return nil
			})
		require.NoError(t, exec.CompleteItemRun(context.Background(), eiec(), nil))
	})
}

// TestCompleteItemRunOnUnretriableErr_TerminalNotOverwritten Terminal 覆盖保护第二点（tasks 6.7）：
// 兜底落 Fail 路径同样不得覆盖 Terminal；turn run log 随之写 Terminal 而非 Fail（保持 item/turn 一致）。
func TestCompleteItemRunOnUnretriableErr_TerminalNotOverwritten(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	turnRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	svc := &ExptItemEventEvalServiceImpl{exptItemResultRepo: itemRepo, exptTurnResultRepo: turnRepo}

	event := &entity.ExptItemEvalEvent{SpaceID: 7, ExptID: 1001, ExptRunID: 2002, EvalSetItemID: 3003}

	itemRepo.EXPECT().GetItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		AnyTimes().Return(&entity.ExptItemResultRunLog{Status: int32(entity.ItemRunState_Terminal)}, nil)
	itemRepo.EXPECT().UpdateItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ int64, _ []int64, ufields map[string]any, _ int64) error {
			_, hasStatus := ufields["status"]
			assert.False(t, hasStatus, "Terminal 是吸收态，兜底 Fail MUST NOT 回写 status")
			return nil
		})
	turnRepo.EXPECT().
		CreateOrUpdateItemsTurnRunLogStatus(gomock.Any(), int64(7), int64(1001), int64(2002), []int64{3003}, entity.TurnRunState_Terminal).
		Return(nil)

	svc.completeItemRunOnUnretriableErr(context.Background(), event, errors.New("build ctx failed"))
}

// TestHandleEventCheck_ItemLevelTerminalGate item 级闸门（tasks 6.8）：
// 该行已 Terminal → 返回 nil 且 **不进入** next（对应「排队中的行被终止后不再被调度」）。
// 闸门优先级：实验级终态判定 > item 级 Terminal 判定 > 正常执行。
func TestHandleEventCheck_ItemLevelTerminalGate(t *testing.T) {
	event := &entity.ExptItemEvalEvent{SpaceID: 7, ExptID: 1001, ExptRunID: 2002, EvalSetItemID: 3003}

	newSvc := func(ctrl *gomock.Controller, itemState entity.ItemRunState) (*ExptItemEventEvalServiceImpl, *bool) {
		manager := svcMocks.NewMockIExptManager(ctrl)
		manager.EXPECT().GetRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&entity.ExptRunLog{Status: int64(entity.ExptStatus_Processing)}, nil)
		itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
		itemRepo.EXPECT().GetItemRunLog(gomock.Any(), int64(1001), int64(2002), int64(3003), int64(7)).
			Return(&entity.ExptItemResultRunLog{Status: int32(itemState)}, nil)
		reached := false
		return &ExptItemEventEvalServiceImpl{manager: manager, exptItemResultRepo: itemRepo}, &reached
	}

	t.Run("terminated_item_event_dropped", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, reached := newSvc(ctrl, entity.ItemRunState_Terminal)

		err := svc.HandleEventCheck(func(context.Context, *entity.ExptItemEvalEvent) error {
			*reached = true
			return nil
		})(context.Background(), event)

		assert.NoError(t, err)
		assert.False(t, *reached, "已 Terminal 的行 MUST NOT 进入执行")
	})

	t.Run("live_item_event_passes_through", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		svc, reached := newSvc(ctrl, entity.ItemRunState_Processing)

		err := svc.HandleEventCheck(func(context.Context, *entity.ExptItemEvalEvent) error {
			*reached = true
			return nil
		})(context.Background(), event)

		assert.NoError(t, err)
		assert.True(t, *reached, "未终止的行必须照常执行")
	})

	t.Run("expt_finished_gate_wins_before_item_query", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		manager := svcMocks.NewMockIExptManager(ctrl)
		manager.EXPECT().GetRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&entity.ExptRunLog{Status: int64(entity.ExptStatus_Success)}, nil)
		// itemRepo 不注册任何期望：实验级闸门先命中，不该再查 item run log
		svc := &ExptItemEventEvalServiceImpl{manager: manager, exptItemResultRepo: repoMocks.NewMockIExptItemResultRepo(ctrl)}

		reached := false
		err := svc.HandleEventCheck(func(context.Context, *entity.ExptItemEvalEvent) error {
			reached = true
			return nil
		})(context.Background(), event)

		assert.NoError(t, err)
		assert.False(t, reached)
	})
}

// TestEvalTurns_StopsOnTerminatedItem 多轮循环闸门（tasks 4.3 对应验证）：
// item 已 Terminal 时第一轮就不开跑（不调用 buildExptTurnEvalCtx 依赖的任何服务），直接返回 nil。
func TestEvalTurns_StopsOnTerminatedItem(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	itemRepo.EXPECT().GetItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&entity.ExptItemResultRunLog{Status: int32(entity.ItemRunState_Terminal)}, nil)

	// TurnResultRepo / evalTargetService 等一概不注册期望：一旦真的开跑就会 Unexpected call
	exec := &ExptItemEvalCtxExecutor{ItemResultRepo: itemRepo}

	eiec := &entity.ExptItemEvalCtx{
		Event:       &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, EvalSetItemID: 3, SpaceID: 4},
		Expt:        &entity.Experiment{ID: 1},
		EvalSetItem: &entity.EvaluationSetItem{Turns: []*entity.Turn{{ID: 11}, {ID: 12}}},
	}

	asyncAbort, err := exec.EvalTurns(context.Background(), eiec)
	assert.NoError(t, err)
	assert.False(t, asyncAbort)
}

// TestParseItemVisibleRunError item 级 err_msg → 用户可见 RunError 的反解（对应 P1 #1）。
//
// 这是 spec 验收点「err_msg 为『该行被用户主动终止』语义（601205087）并透给前端展示」的回归防线：
// ItemSystemInfo.Error 是 item 级错误透出前端的唯一赋值点，行级终止若不在此反解，
// TerminateItems 步骤④ 写的 err_msg 就是白写，用户只能看到裸 Terminal 状态。
func TestParseItemVisibleRunError(t *testing.T) {
	t.Run("manually_terminated_is_surfaced_with_chinese_detail", func(t *testing.T) {
		raw := []byte(errno.SerializeErr(errno.NewItemManuallyTerminatedErr()))

		got := parseItemVisibleRunError(raw)

		require.NotNil(t, got, "行级终止必须透出 RunError，否则前端看不到终止原因")
		assert.Equal(t, int64(errno.ItemManuallyTerminatedCode), got.Code, "code 必须是 601205087")
		require.NotNil(t, got.Detail)
		assert.Equal(t, "该行被用户主动终止", *got.Detail, "Detail 必须是中文可读语义，不能是内部错误串")
	})

	t.Run("zombie_timeout_behavior_unchanged", func(t *testing.T) {
		raw := []byte(errno.SerializeErr(errno.NewItemZombieTimeoutErr(600, false)))

		got := parseItemVisibleRunError(raw)

		require.NotNil(t, got)
		assert.Equal(t, int64(errno.ItemZombieTimeoutCode), got.Code, "既有僵尸超时分支行为不变")
		require.NotNil(t, got.Detail)
		assert.Contains(t, *got.Detail, "僵尸")
	})

	t.Run("unknown_code_returns_nil", func(t *testing.T) {
		// 非白名单错误码不透出（沿用既有语义，避免把内部错误串抖给前端）
		assert.Nil(t, parseItemVisibleRunError([]byte(errno.SerializeErr(errno.NewTargetResultErr("internal boom")))))
		assert.Nil(t, parseItemVisibleRunError([]byte("not a serialized err")))
	})
}

// TestGetTurnSystemInfo_ManuallyTerminated turn 级 err_msg 反解（对应 P1 #1 + #3 联动）。
// CreateOrUpdateItemsTurnRunLogStatus(Terminal) 写的是 ItemManuallyTerminated（非 turnOtherErrCode），
// RecordItemRunLogs 会把它回抄进 turn result；读侧不接上，turn 级就只剩裸 Terminal 状态。
func TestGetTurnSystemInfo_ManuallyTerminated(t *testing.T) {
	ctx := context.Background()

	newBuilder := func(errMsg string, status int32) *ExptResultBuilder {
		return &ExptResultBuilder{
			ItemIDTurnID2TurnResultID: map[int64]map[int64]int64{1: {10: 100}},
			turnResultDO: []*entity.ExptTurnResult{
				{ID: 100, ItemID: 1, TurnID: 10, Status: status, LogID: "log1", ErrMsg: errMsg},
			},
		}
	}

	t.Run("terminated_turn_surfaces_manually_terminated", func(t *testing.T) {
		b := newBuilder(errno.SerializeErr(errno.NewItemManuallyTerminatedErr()), int32(entity.TurnRunState_Terminal))

		got := b.getTurnSystemInfo(ctx, 1, 10)

		require.NotNil(t, got.Error, "turn 级也必须透出「被主动终止」，否则与 item 级自相矛盾")
		assert.Equal(t, int64(errno.ItemManuallyTerminatedCode), got.Error.Code)
		require.NotNil(t, got.Error.Detail)
		assert.Equal(t, "该行被用户主动终止", *got.Error.Detail)
	})

	t.Run("turn_other_err_behavior_unchanged", func(t *testing.T) {
		b := newBuilder(errno.SerializeErr(errno.NewTurnOtherErr("turn status not updated for long interval", errors.New("timeout"))), int32(entity.TurnRunState_Fail))

		got := b.getTurnSystemInfo(ctx, 1, 10)

		require.NotNil(t, got.Error, "既有 ParseTurnOtherErr 分支行为不变")
		assert.Contains(t, *got.Error.Detail, "turn status not updated")
		assert.Zero(t, got.Error.Code, "既有分支不设 Code，保持向前兼容")
	})
}
