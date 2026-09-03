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

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

// 本文件守的不变量：**中心调度派发时，展示投影的四张表要一起前进**
// —— run log、主表 expt_item_result、turn 主表 expt_turn_result、CK 加速表 expt_turn_result_filter。
//
// legacy 的 handleToSubmits 一直是五写（含 stats），中心调度这条平行路径逐项漏过：
// 先漏主表（item 跑了 14h 仍显示排队中），再漏 stats（计数走负），最后漏这两张。
//
// 漏 CK 的表现最难察觉：`item_run_state` 筛选打的是 etrf.status，且开启加速器时结果里的
// run_state 也从 CK 读，于是执行期间按「运行中」筛恒为空、按「排队中」筛反而捞出正在跑的 item。
// 完成时那次 upsert 会纠回来，所以只有 in-flight 窗口是错的，对着终态数据复盘看不出来。
// PPE 实测：主表已 status=1，CK 仍为 0。

// centralDisplayFixture 组一套「CAS 命中、主表从 Queueing 前进到 Processing」的最小依赖。
func centralDisplayFixture(t *testing.T, prev entity.ItemRunState, mainTableErr error) (
	*ExptItemEventEvalServiceImpl, *repoMocks.MockIExptTurnResultRepo, *svcMocks.MockExptResultService,
) {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	dispatch := repoMocks.NewMockIExptItemDispatchRepo(ctrl)
	dispatch.EXPECT().StartReservedItem(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
	dispatch.EXPECT().MGetDispatchObservations(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).AnyTimes()

	items := repoMocks.NewMockIExptItemResultRepo(ctrl)
	items.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*entity.ExptItemResult{{ItemID: 4, Status: prev}}, nil)
	if prev != entity.ItemRunState_Processing {
		items.EXPECT().UpdateItemsResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(mainTableErr)
	}

	stats := repoMocks.NewMockIExptStatsRepo(ctrl)
	stats.EXPECT().ArithOperateCount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	turns := repoMocks.NewMockIExptTurnResultRepo(ctrl)
	result := svcMocks.NewMockExptResultService(ctrl)

	return &ExptItemEventEvalServiceImpl{
		centralGuard:       &fakeGuard{confirmResult: true},
		dispatchRepo:       dispatch,
		exptItemResultRepo: items,
		exptStatsRepo:      stats,
		exptTurnResultRepo: turns,
		resultSvc:          result,
	}, turns, result
}

// TestCentralReservation_AdvancesTurnTableAndFilter 核心回归：主表真的前进时，
// turn 主表与 CK 加速表都必须跟着写。
func TestCentralReservation_AdvancesTurnTableAndFilter(t *testing.T) {
	svc, turns, result := centralDisplayFixture(t, entity.ItemRunState_Queueing, nil)

	var gotTurnFields map[string]any
	turns.EXPECT().UpdateTurnResultsWithItemIDs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ int64, itemIDs []int64, _ int64, ufields map[string]any) error {
			assert.Equal(t, []int64{4}, itemIDs)
			gotTurnFields = ufields
			return nil
		})

	var gotFilterItemIDs []int64
	result.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, spaceID, exptID int64, itemIDs []int64) error {
			assert.Equal(t, int64(3), spaceID)
			assert.Equal(t, int64(1), exptID)
			gotFilterItemIDs = itemIDs
			return nil
		})

	handler := svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error { return nil })
	require.NoError(t, handler(centralAdmittedCtx(t), &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4}))

	require.NotNil(t, gotTurnFields, "turn 主表没写 —— turn 层状态会一直停在排队中")
	assert.Equal(t, int32(entity.TurnRunState_Processing), gotTurnFields["status"])
	require.NotNil(t, gotFilterItemIDs, "CK 加速表没写 —— 按「运行中」筛会恒为空")
	assert.Equal(t, []int64{4}, gotFilterItemIDs)
}

// TestCentralReservation_SkipsDisplayProjectionWhenAlreadyProcessing 主表已是 Processing 时
// 不重复写：与 stats 记账同一个判据，避免重复投递把 CK 反复刷一遍。
func TestCentralReservation_SkipsDisplayProjectionWhenAlreadyProcessing(t *testing.T) {
	svc, turns, result := centralDisplayFixture(t, entity.ItemRunState_Processing, nil)

	turns.EXPECT().UpdateTurnResultsWithItemIDs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	result.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	handler := svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error { return nil })
	require.NoError(t, handler(centralAdmittedCtx(t), &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4}))
}

// TestCentralReservation_FilterWrittenEvenIfTurnTableFails 两个写彼此独立：
// turn 主表写失败不能把 CK 那一写连带跳过，否则一次 DB 抖动就让筛选长期失真。
func TestCentralReservation_FilterWrittenEvenIfTurnTableFails(t *testing.T) {
	svc, turns, result := centralDisplayFixture(t, entity.ItemRunState_Queueing, nil)

	turns.EXPECT().UpdateTurnResultsWithItemIDs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("db down"))
	result.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	handler := svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error { return nil })
	require.NoError(t, handler(centralAdmittedCtx(t), &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4}))
}

// TestCentralReservation_NoDisplayProjectionWhenMainTableWriteFails 主表没写成功就不是
// 「真的发生了迁移」，此时写 turn / CK 会让它们领先于主表 —— 反而制造新的不一致。
func TestCentralReservation_NoDisplayProjectionWhenMainTableWriteFails(t *testing.T) {
	svc, turns, result := centralDisplayFixture(t, entity.ItemRunState_Queueing, errors.New("db down"))

	turns.EXPECT().UpdateTurnResultsWithItemIDs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	result.EXPECT().UpsertExptTurnResultFilter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	handler := svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error { return nil })
	require.NoError(t, handler(centralAdmittedCtx(t), &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4}))
}

// TestCentralReservation_DisplayProjectionToleratesNilDeps 老调用方（未注入这两个依赖的单测/
// 部署形态）不能因此 panic —— 与相邻两项一致，缺依赖就退化为不写。
func TestCentralReservation_DisplayProjectionToleratesNilDeps(t *testing.T) {
	svc, _, _ := centralDisplayFixture(t, entity.ItemRunState_Queueing, nil)
	svc.exptTurnResultRepo = nil
	svc.resultSvc = nil

	handler := svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error { return nil })
	require.NoError(t, handler(centralAdmittedCtx(t), &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4}))
}
