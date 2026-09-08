// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/ctxcache"
)

// 本文件守的是一个能让整个实验停摆的静默故障：
// consumer 已把 run log 兑现成 Processing/none，之后额度消失，重投消息在 ConfirmRunning
// 被丢弃 —— item 成了「既无额度也无执行者」的孤儿，却仍以 Processing 占着 item_concur_num
// 的槽位。ScanEvalItems 和 LoadDispatchRuntime 都把它当"在跑"，于是既不派新 item 也
// 等不到它完成，只能靠 3h 异步僵尸阈值兜底。
//
// 2026-08-28 线上实测：两个实验各 20 个 item 占满槽位 77 分钟，turn 表零记录、账本零 reservation。
// 所以这里断言的不是"调过某个方法"，而是**投影确实被退回、且 stats 与主表跟着退回**。

func admittedEnforceCtx(event *entity.ExptItemEvalEvent) context.Context {
	ctx := ctxcache.Init(context.Background())
	event.WithCtxCentralAdmittedExpt(ctx, &entity.Experiment{
		ID: event.ExptID, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: testScope,
	})
	return ctx
}

// TestReservationAbsent_RequeuesOrphanedProcessingItem 主场景：Processing 且账本无 reservation
// ⇒ 必须退回 Queueing，并把 stats 的 Processing 计数与主表状态一起退回。
func TestReservationAbsent_RequeuesOrphanedProcessingItem(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	dispatchRepo := repoMocks.NewMockIExptItemDispatchRepo(ctrl)
	dispatchRepo.EXPECT().MGetDispatchObservations(gomock.Any(), int64(3), int64(1), int64(2), []int64{4}).
		Return([]*repo.ExptDispatchObservation{
			{ItemID: 4, Status: int32(entity.ItemRunState_Processing), QuotaReservationState: entity.QuotaReservationStateNone},
		}, nil)
	// ★ 核心断言：退回动作真的发生在这个 item 上。
	dispatchRepo.EXPECT().RequeueProcessingItem(gomock.Any(), int64(3), int64(1), int64(2), int64(4)).
		Return(true, nil)

	var gotStatsOp *entity.StatsCntArithOp
	statsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
	statsRepo.EXPECT().ArithOperateCount(gomock.Any(), int64(1), int64(3), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ int64, op *entity.StatsCntArithOp) error {
			gotStatsOp = op
			return nil
		})

	var gotFields map[string]any
	itemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	itemResultRepo.EXPECT().UpdateItemsResult(gomock.Any(), int64(3), int64(1), []int64{4}, gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ int64, _ []int64, ufields map[string]any) error {
			gotFields = ufields
			return nil
		})

	svc := &ExptItemEventEvalServiceImpl{
		centralGuard:       &fakeGuard{confirmResult: false},
		dispatchRepo:       dispatchRepo,
		exptStatsRepo:      statsRepo,
		exptItemResultRepo: itemResultRepo,
	}

	event := &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4}
	ctx := admittedEnforceCtx(event)

	nextCalled := false
	err := svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error {
		nextCalled = true
		return nil
	})(ctx, event)

	assert.NoError(t, err, "这条消息注定不执行，返回 error 只会让 MQ 无休止重投")
	assert.False(t, nextCalled, "reservation 不存在时不得执行 item")

	assert.Equal(t, -1, gotStatsOp.OpStatusCnt[entity.ItemRunState_Processing],
		"Processing 桶必须减 1，否则 processing_turn_count 只增不减")
	assert.Equal(t, 1, gotStatsOp.OpStatusCnt[entity.ItemRunState_Queueing],
		"item 回到队列，Queueing 桶要加回来")
	assert.Equal(t, int32(entity.ItemRunState_Queueing), gotFields["status"],
		"主表是详情页的数据源，不退回会一直显示成执行中")
}

// TestReservationAbsent_RollsBackTurnTableAndFilter 派发侧推进五项（run log / 主表 / stats /
// turn 主表 / CK 加速表），回退必须同样是五项。只退前三项会留下「item 已回队列、turn 与 CK
// 仍显示执行中」的错位，而且不自愈 —— 实验若以 Failed 收口，CompleteExpt 的 default 分支
// 只销毁沙箱、不动 turn 状态。
func TestReservationAbsent_RollsBackTurnTableAndFilter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	dispatchRepo := repoMocks.NewMockIExptItemDispatchRepo(ctrl)
	dispatchRepo.EXPECT().MGetDispatchObservations(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*repo.ExptDispatchObservation{
			{ItemID: 4, Status: int32(entity.ItemRunState_Processing), QuotaReservationState: entity.QuotaReservationStateNone},
		}, nil)
	dispatchRepo.EXPECT().RequeueProcessingItem(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(true, nil)

	statsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
	statsRepo.EXPECT().ArithOperateCount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	itemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	itemResultRepo.EXPECT().UpdateItemsResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	var gotTurnFields map[string]any
	turnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	turnResultRepo.EXPECT().UpdateTurnResultsWithItemIDs(gomock.Any(), int64(1), []int64{4}, int64(3), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ int64, _ []int64, _ int64, ufields map[string]any) error {
			gotTurnFields = ufields
			return nil
		})

	var gotFilterItemIDs []int64
	resultSvc := svcMocks.NewMockExptResultService(ctrl)
	resultSvc.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), int64(3), int64(1), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ int64, itemIDs []int64) error {
			gotFilterItemIDs = itemIDs
			return nil
		})

	svc := &ExptItemEventEvalServiceImpl{
		centralGuard:       &fakeGuard{confirmResult: false},
		dispatchRepo:       dispatchRepo,
		exptStatsRepo:      statsRepo,
		exptItemResultRepo: itemResultRepo,
		exptTurnResultRepo: turnResultRepo,
		resultSvc:          resultSvc,
	}

	event := &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4}
	assert.NoError(t, svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error {
		return nil
	})(admittedEnforceCtx(event), event))

	assert.Equal(t, int32(entity.TurnRunState_Queueing), gotTurnFields["status"],
		"turn 主表不退回，详情页 turn 会一直显示运行中而 item 显示排队中")
	assert.Equal(t, []int64{4}, gotFilterItemIDs,
		"CK 不刷新，开加速器时按「运行中」筛会筛出实际在排队的 item")
}

// TestReservationAbsent_LeavesTerminalItemUntouched 迟到消息：item 已终态，
// 额度早已正常释放。这是唯一的预期路径，任何"修复"动作都会让已跑完的 item 重跑。
func TestReservationAbsent_LeavesTerminalItemUntouched(t *testing.T) {
	for _, st := range []entity.ItemRunState{
		entity.ItemRunState_Success, entity.ItemRunState_Fail, entity.ItemRunState_Terminal,
	} {
		ctrl := gomock.NewController(t)
		dispatchRepo := repoMocks.NewMockIExptItemDispatchRepo(ctrl)
		dispatchRepo.EXPECT().MGetDispatchObservations(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]*repo.ExptDispatchObservation{{ItemID: 4, Status: int32(st)}}, nil)
		// 不 EXPECT RequeueProcessingItem / ResetQuotaReserved —— gomock 会让意外调用直接失败，
		// 这正是本用例要守的：终态 item 一个字段都不能动。

		svc := &ExptItemEventEvalServiceImpl{
			centralGuard: &fakeGuard{confirmResult: false},
			dispatchRepo: dispatchRepo,
		}
		event := &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4}
		assert.NoError(t, svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error {
			t.Fatalf("终态 item 不得执行, state: %v", st)
			return nil
		})(admittedEnforceCtx(event), event))
		ctrl.Finish()
	}
}

// TestReservationAbsent_ResetsStaleQueueingReserved 还没兑现执行的那一半：
// Queueing/reserved 但账本已无 reservation ⇒ 清回 none 让它重新可授予。
// 若不清，LoadDispatchRuntime 会一直把它算进占用（Queueing/reserved 计入占用）。
func TestReservationAbsent_ResetsStaleQueueingReserved(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	dispatchRepo := repoMocks.NewMockIExptItemDispatchRepo(ctrl)
	dispatchRepo.EXPECT().MGetDispatchObservations(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*repo.ExptDispatchObservation{
			{ItemID: 4, Status: int32(entity.ItemRunState_Queueing), QuotaReservationState: entity.QuotaReservationStateReserved},
		}, nil)
	dispatchRepo.EXPECT().ResetQuotaReserved(gomock.Any(), int64(3), int64(1), int64(2), []int64{4}).
		Return([]int64{4}, nil)

	svc := &ExptItemEventEvalServiceImpl{
		centralGuard: &fakeGuard{confirmResult: false},
		dispatchRepo: dispatchRepo,
	}
	event := &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4}
	assert.NoError(t, svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error {
		t.Fatal("Queueing/reserved 的 item 本轮不得执行")
		return nil
	})(admittedEnforceCtx(event), event))
}

// TestReservationAbsent_KeepsProjectionWhenLoadFails 读不到投影时必须什么都不动。
// 盲目退回会让「其实已终态」的 item 重跑 —— 宁可退化成原来的行为（等僵尸阈值）。
func TestReservationAbsent_KeepsProjectionWhenLoadFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	dispatchRepo := repoMocks.NewMockIExptItemDispatchRepo(ctrl)
	dispatchRepo.EXPECT().MGetDispatchObservations(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, errors.New("db down"))

	svc := &ExptItemEventEvalServiceImpl{
		centralGuard: &fakeGuard{confirmResult: false},
		dispatchRepo: dispatchRepo,
	}
	event := &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4}
	assert.NoError(t, svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error {
		t.Fatal("读投影失败时不得执行 item")
		return nil
	})(admittedEnforceCtx(event), event))
}

// TestReservationAbsent_CASMissLeavesStatsAndMainTableUntouched CAS 未命中时，
// stats 与主表都不能动。
//
// 触发场景：读投影与 CAS 之间有一个无锁窗口，`handleZombies` / `sweepTerminatedSandboxItems`
// 跑在**实验锁**下、不持 item 锁，可以在这个窗口把 run log 抢先改成终态 —— 于是 CAS 落空。
//
// 为什么这条必须单独守：主表 status 不是纯展示字段，而是 stats 的锚点（完成侧 statsCntOp
// 读 items_result.Status 做「-1」）。CAS 落空却把主表改成 Queueing，会让 stats 上那笔
// Processing 永远减不掉，processing_turn_count 归不了零 —— 同仓已就此修过一次（c4a6a953）。
func TestReservationAbsent_CASMissLeavesStatsAndMainTableUntouched(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	dispatchRepo := repoMocks.NewMockIExptItemDispatchRepo(ctrl)
	dispatchRepo.EXPECT().MGetDispatchObservations(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*repo.ExptDispatchObservation{
			{ItemID: 4, Status: int32(entity.ItemRunState_Processing), QuotaReservationState: entity.QuotaReservationStateNone},
		}, nil)
	// CAS 未命中：并发路径已经把这条 run log 改走了。
	dispatchRepo.EXPECT().RequeueProcessingItem(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(false, nil)

	// 两个 mock 都不 EXPECT 任何调用 —— gomock 会让意外调用直接失败，这正是本用例的断言。
	statsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
	itemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)

	svc := &ExptItemEventEvalServiceImpl{
		centralGuard:       &fakeGuard{confirmResult: false},
		dispatchRepo:       dispatchRepo,
		exptStatsRepo:      statsRepo,
		exptItemResultRepo: itemResultRepo,
	}
	event := &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4}
	assert.NoError(t, svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error {
		t.Fatal("CAS 未命中时不得执行 item")
		return nil
	})(admittedEnforceCtx(event), event))
}

// TestReservationAbsent_NoDispatchRepoIsSafe dispatchRepo 缺失时不得执行 item，也不得 panic。
func TestReservationAbsent_NoDispatchRepoIsSafe(t *testing.T) {
	svc := &ExptItemEventEvalServiceImpl{centralGuard: &fakeGuard{confirmResult: false}}
	event := &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4}
	assert.NoError(t, svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error {
		t.Fatal("无法修正投影时不得执行 item")
		return nil
	})(admittedEnforceCtx(event), event))
}
