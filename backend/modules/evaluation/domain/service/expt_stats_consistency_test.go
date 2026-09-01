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
	"github.com/coze-dev/coze-loop/backend/pkg/ctxcache"
)

// 本文件守一条不变量：**expt_stats 计数行的桶分布必须等于主表 expt_item_result 的状态分布**。
//
// 它此前被破坏过一次，代价很大：中心调度的派发把 stats 记账绑在 run log 的 CAS 上、
// 而主表推进是无条件的，两个判据一分叉，完成侧（按主表状态做「-1」）就去减一个计数行里
// 从没加过的桶。PPE 实测一个 900 题 enforce 实验 pending 虚高 281、success 少 249，
// 而同泳道的 legacy 实验分毫不差 —— legacy 的派发是四张表无条件一起写的。
//
// 两层防护各有用例：
//   - 派发侧：记账与主表推进同一个判据（本文件前半）
//   - 调度器每拍对账：把残留偏差收敛掉（本文件后半）

func centralAdmittedCtx(t *testing.T) context.Context {
	t.Helper()
	ctx := ctxcache.Init(context.Background())
	(&entity.ExptItemEvalEvent{}).WithCtxCentralAdmittedExpt(ctx, &entity.Experiment{
		ID: 1, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: testScope,
	})
	return ctx
}

// TestCentralReservation_StatsFollowsMainTableNotCAS 核心回归：
// run log CAS 未命中（started=false），但主表还停在 Queueing 时，**stats 记账仍然必须发生**。
//
// 旧代码在这里直接跳过，于是那条 item 的 Queueing 永远不减 —— 这正是线上 pending 虚高的来源。
func TestCentralReservation_StatsFollowsMainTableNotCAS(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	dispatch := repoMocks.NewMockIExptItemDispatchRepo(ctrl)
	items := repoMocks.NewMockIExptItemResultRepo(ctrl)
	stats := repoMocks.NewMockIExptStatsRepo(ctrl)

	// CAS 未命中：重复投递或已被 repair 修正。
	dispatch.EXPECT().StartReservedItem(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(false, nil)
	// 执行链返回后的释放路径会查一次投影，与本用例无关。
	dispatch.EXPECT().MGetDispatchObservations(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).AnyTimes()
	// 但主表仍是 Queueing —— 说明这条 item 从来没被记进 Processing 桶。
	items.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*entity.ExptItemResult{{ItemID: 4, Status: entity.ItemRunState_Queueing}}, nil)
	items.EXPECT().UpdateItemsResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	var gotOp *entity.StatsCntArithOp
	stats.EXPECT().ArithOperateCount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ int64, op *entity.StatsCntArithOp) error {
			gotOp = op
			return nil
		})

	svc := &ExptItemEventEvalServiceImpl{
		centralGuard:       &fakeGuard{confirmResult: true},
		dispatchRepo:       dispatch,
		exptItemResultRepo: items,
		exptStatsRepo:      stats,
	}
	handler := svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error { return nil })
	require.NoError(t, handler(centralAdmittedCtx(t), &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4}))

	require.NotNil(t, gotOp, "CAS 未命中但主表仍在 Queueing 时必须记账 —— 跳过就是线上 pending 虚高的成因")
	assert.Equal(t, 1, gotOp.OpStatusCnt[entity.ItemRunState_Processing])
	assert.Equal(t, -1, gotOp.OpStatusCnt[entity.ItemRunState_Queueing])
}

// TestCentralReservation_NoStatsOpWhenMainTableAlreadyProcessing 主表已是 Processing 时
// 不得重复记账 —— 那会从"少计"变成"多计"，同样让计数行歪掉。
func TestCentralReservation_NoStatsOpWhenMainTableAlreadyProcessing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	dispatch := repoMocks.NewMockIExptItemDispatchRepo(ctrl)
	items := repoMocks.NewMockIExptItemResultRepo(ctrl)
	stats := repoMocks.NewMockIExptStatsRepo(ctrl)

	dispatch.EXPECT().StartReservedItem(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(true, nil) // CAS 命中，但主表已经是 Processing
	dispatch.EXPECT().MGetDispatchObservations(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).AnyTimes()
	items.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*entity.ExptItemResult{{ItemID: 4, Status: entity.ItemRunState_Processing}}, nil)
	// 主表不该被重写，stats 不该被记账
	items.EXPECT().UpdateItemsResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	stats.EXPECT().ArithOperateCount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	svc := &ExptItemEventEvalServiceImpl{
		centralGuard: &fakeGuard{confirmResult: true}, dispatchRepo: dispatch,
		exptItemResultRepo: items, exptStatsRepo: stats,
	}
	handler := svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error { return nil })
	require.NoError(t, handler(centralAdmittedCtx(t), &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4}))
}

// TestCentralReservation_StatsDecrementsActualBucket 主表停在非 Queueing 的状态（repair /
// 重投留下的）时，减的必须是**那个**桶，不能写死 Queueing —— 减错桶与不减一样会让计数行歪。
func TestCentralReservation_StatsDecrementsActualBucket(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	dispatch := repoMocks.NewMockIExptItemDispatchRepo(ctrl)
	items := repoMocks.NewMockIExptItemResultRepo(ctrl)
	stats := repoMocks.NewMockIExptStatsRepo(ctrl)

	dispatch.EXPECT().StartReservedItem(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
	dispatch.EXPECT().MGetDispatchObservations(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).AnyTimes()
	items.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*entity.ExptItemResult{{ItemID: 4, Status: entity.ItemRunState_Fail}}, nil)
	items.EXPECT().UpdateItemsResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	var gotOp *entity.StatsCntArithOp
	stats.EXPECT().ArithOperateCount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ int64, op *entity.StatsCntArithOp) error { gotOp = op; return nil })

	svc := &ExptItemEventEvalServiceImpl{
		centralGuard: &fakeGuard{confirmResult: true}, dispatchRepo: dispatch,
		exptItemResultRepo: items, exptStatsRepo: stats,
	}
	handler := svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error { return nil })
	require.NoError(t, handler(centralAdmittedCtx(t), &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4}))

	require.NotNil(t, gotOp)
	assert.Equal(t, -1, gotOp.OpStatusCnt[entity.ItemRunState_Fail], "减的必须是主表实际所在的桶")
	assert.Equal(t, 0, gotOp.OpStatusCnt[entity.ItemRunState_Queueing], "不得写死 Queueing")
}

// TestCentralReservation_NoStatsOpWhenMainTableWriteFails 主表没写进去就不该记账 ——
// 否则计数行跑到主表前面，完成侧的「-1」又会减错。
func TestCentralReservation_NoStatsOpWhenMainTableWriteFails(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	dispatch := repoMocks.NewMockIExptItemDispatchRepo(ctrl)
	items := repoMocks.NewMockIExptItemResultRepo(ctrl)
	stats := repoMocks.NewMockIExptStatsRepo(ctrl)

	dispatch.EXPECT().StartReservedItem(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
	dispatch.EXPECT().MGetDispatchObservations(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).AnyTimes()
	items.EXPECT().BatchGet(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*entity.ExptItemResult{{ItemID: 4, Status: entity.ItemRunState_Queueing}}, nil)
	items.EXPECT().UpdateItemsResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("db down"))
	stats.EXPECT().ArithOperateCount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	svc := &ExptItemEventEvalServiceImpl{
		centralGuard: &fakeGuard{confirmResult: true}, dispatchRepo: dispatch,
		exptItemResultRepo: items, exptStatsRepo: stats,
	}
	handler := svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error { return nil })
	// 主表是展示投影，写失败不阻断执行 —— 但也不能记账。
	require.NoError(t, handler(centralAdmittedCtx(t), &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4}))
}

// ---------- 每拍对账 ----------

func reconcileFixture(t *testing.T) (*ExptSchedulerImpl, *repoMocks.MockIExptItemResultRepo, *repoMocks.MockIExptStatsRepo) {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)
	items := repoMocks.NewMockIExptItemResultRepo(ctrl)
	stats := repoMocks.NewMockIExptStatsRepo(ctrl)
	return &ExptSchedulerImpl{ExptItemResultRepo: items, ExptStatsRepo: stats}, items, stats
}

func TestReconcileExptStats_CorrectsDrift(t *testing.T) {
	e, items, stats := reconcileFixture(t)

	// 线上实测过的形态：主表 597 success / 253 queueing，计数行却停在 352 / 530。
	items.EXPECT().CountItemsByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[entity.ItemRunState]int64{
		entity.ItemRunState_Queueing: 253, entity.ItemRunState_Processing: 11,
		entity.ItemRunState_Success: 597, entity.ItemRunState_Fail: 39,
	}, nil)
	stats.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.ExptStats{
		PendingItemCnt: 530, ProcessingItemCnt: 7, SuccessItemCnt: 352, FailItemCnt: 11,
	}, nil)

	var got *entity.ExptStats
	stats.EXPECT().UpdateByExptID(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ int64, s *entity.ExptStats) error { got = s; return nil })

	e.reconcileExptStats(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, SpaceID: 2}, nil)

	require.NotNil(t, got, "漂了就必须写回")
	assert.Equal(t, int32(253), got.PendingItemCnt)
	assert.Equal(t, int32(597), got.SuccessItemCnt)
	assert.Equal(t, int32(39), got.FailItemCnt)
	assert.Equal(t, int32(11), got.ProcessingItemCnt)
}

func TestReconcileExptStats_NoWriteWhenConsistent(t *testing.T) {
	e, items, stats := reconcileFixture(t)

	items.EXPECT().CountItemsByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[entity.ItemRunState]int64{
		entity.ItemRunState_Queueing: 3, entity.ItemRunState_Processing: 10, entity.ItemRunState_Success: 17,
	}, nil)
	stats.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.ExptStats{
		PendingItemCnt: 3, ProcessingItemCnt: 10, SuccessItemCnt: 17,
	}, nil)
	// 一致就不能写：每实验每拍一次写库是纯浪费，而且会把 updated_at 刷成"刚改过"，
	// 让"这行到底动没动"这个排查线索失效。
	stats.EXPECT().UpdateByExptID(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	e.reconcileExptStats(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, SpaceID: 2}, nil)
}

func TestReconcileExptStats_SkipsWhenNoItemRows(t *testing.T) {
	e, items, stats := reconcileFixture(t)

	// 实验刚建、item 还没落库。此时覆盖会把 finishExptStart 写下的 PendingItemCnt 抹成 0，
	// 详情页看起来"一条待跑都没有"。
	items.EXPECT().CountItemsByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[entity.ItemRunState]int64{}, nil)
	stats.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	stats.EXPECT().UpdateByExptID(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	e.reconcileExptStats(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, SpaceID: 2}, nil)
}

func TestReconcileExptStats_SkipsOnCountError(t *testing.T) {
	e, items, stats := reconcileFixture(t)

	// 对账是自愈机制，读不到真值时**什么都不做**，绝不能拿一个不完整的分布去覆盖。
	items.EXPECT().CountItemsByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("db down"))
	stats.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	stats.EXPECT().UpdateByExptID(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	e.reconcileExptStats(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, SpaceID: 2}, nil)
}
