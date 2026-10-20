// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// Requeue owns canonical/turn/log/stats atomically. The consumer refreshes
// only the committed filter and never repeats the database writes.
func TestReservationAbsent_RequeueAtomicAndFilter(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(fmt.Sprint(changed), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			dispatch := repoMocks.NewMockIExptItemDispatchRepo(ctrl)
			dispatch.EXPECT().MGetDispatchObservations(gomock.Any(), int64(3), int64(1), int64(2), []int64{4}).Return([]*repo.ExptDispatchObservation{{ItemID: 4, Status: int32(entity.ItemRunState_Processing), RequestID: "rq2"}}, nil)
			// Deliberately observe RQ2 while the event is RQ1: never substitute latest.
			dispatch.EXPECT().RequeueProcessingItem(gomock.Any(), int64(3), int64(1), int64(2), int64(4), "rq1").Return(changed, nil)
			result := svcMocks.NewMockExptResultService(ctrl)
			if changed {
				result.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), int64(3), int64(1), []int64{4}).Return(nil)
			}
			svc := &ExptItemEventEvalServiceImpl{
				centralGuard: &fakeGuard{confirmResult: false}, dispatchRepo: dispatch, resultSvc: result,
				exptItemResultRepo: repoMocks.NewMockIExptItemResultRepo(ctrl), exptTurnResultRepo: repoMocks.NewMockIExptTurnResultRepo(ctrl), exptStatsRepo: repoMocks.NewMockIExptStatsRepo(ctrl),
			}
			event := &entity.ExptItemEvalEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2, EvalSetItemID: 4, RequestID: "rq1"}
			require.NoError(t, svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error {
				t.Fatal("absent reservation must stop")
				return nil
			})(admittedEnforceCtx(event), event))
		})
	}
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
	dispatchRepo.EXPECT().ResetQuotaReserved(gomock.Any(), int64(3), int64(1), int64(2), []int64{4}, map[int64]string{4: "rq1"}).
		Return([]int64{4}, nil)

	svc := &ExptItemEventEvalServiceImpl{
		centralGuard: &fakeGuard{confirmResult: false},
		dispatchRepo: dispatchRepo,
	}
	event := &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4, RequestID: "rq1"}
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
	dispatchRepo.EXPECT().RequeueProcessingItem(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), "").
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
