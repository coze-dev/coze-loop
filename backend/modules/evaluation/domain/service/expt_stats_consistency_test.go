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

	idemMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
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

// Start commits stats with canonical; consumer must not write either again.
func TestCentralReservation_NoSecondStatsWrite(t *testing.T) {
	for _, started := range []bool{false, true} {
		t.Run(fmt.Sprint(started), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			dispatch := repoMocks.NewMockIExptItemDispatchRepo(ctrl)
			dispatch.EXPECT().StartReservedItem(gomock.Any(), int64(3), int64(1), int64(2), int64(4), "rq1").Return(repo.StartReservedItemResult{CurrentRequest: true, Started: started}, nil)
			dispatch.EXPECT().MGetDispatchObservations(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil)
			svc := &ExptItemEventEvalServiceImpl{
				centralGuard: &fakeGuard{confirmResult: true}, dispatchRepo: dispatch,
				exptItemResultRepo: repoMocks.NewMockIExptItemResultRepo(ctrl), exptStatsRepo: repoMocks.NewMockIExptStatsRepo(ctrl),
			}
			svc.exptItemResultRepo.(*repoMocks.MockIExptItemResultRepo).EXPECT().GetCurrentRequest(gomock.Any(), int64(3), int64(1), int64(2), int64(4)).Return(&entity.ExptItemCurrentRequest{RunID: 2, ItemID: 4, RequestID: "rq1", Status: entity.ItemRunState_Processing}, nil)
			called := false
			require.NoError(t, svc.HandleCentralReservation(func(context.Context, *entity.ExptItemEvalEvent) error { called = true; return nil })(centralAdmittedCtx(t), &entity.ExptItemEvalEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2, EvalSetItemID: 4, RequestID: "rq1"}))
			require.True(t, called)
		})
	}
}

// ---------- 每拍对账 ----------

// enforceExptForReconcile 对账只对 enforce 实验做，本文件的用例统一用它。
func enforceExptForReconcile() *entity.Experiment {
	return &entity.Experiment{ID: 1, ExptDispatchMode: entity.ExptDispatchModeEnforce}
}

func reconcileFixture(t *testing.T) (*ExptSchedulerImpl, *repoMocks.MockIExptItemResultRepo, *repoMocks.MockIExptStatsRepo, *idemMocks.MockIdempotentService) {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)
	items := repoMocks.NewMockIExptItemResultRepo(ctrl)
	stats := repoMocks.NewMockIExptStatsRepo(ctrl)
	idm := idemMocks.NewMockIdempotentService(ctrl)
	return &ExptSchedulerImpl{ExptItemResultRepo: items, ExptStatsRepo: stats, Idem: idm}, items, stats, idm
}

func TestReconcileExptStats_CorrectsDrift(t *testing.T) {
	e, items, stats, idm := reconcileFixture(t)

	// 线上实测过的形态：主表 597 success / 253 queueing，计数行却停在 352 / 530。
	idm.EXPECT().SetNX(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
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

	e.reconcileExptStats(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, SpaceID: 2}, enforceExptForReconcile())

	require.NotNil(t, got, "漂了就必须写回")
	assert.Equal(t, int32(253), got.PendingItemCnt)
	assert.Equal(t, int32(597), got.SuccessItemCnt)
	assert.Equal(t, int32(39), got.FailItemCnt)
	assert.Equal(t, int32(11), got.ProcessingItemCnt)
}

func TestReconcileExptStats_NoWriteWhenConsistent(t *testing.T) {
	e, items, stats, idm := reconcileFixture(t)

	idm.EXPECT().SetNX(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
	items.EXPECT().CountItemsByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[entity.ItemRunState]int64{
		entity.ItemRunState_Queueing: 3, entity.ItemRunState_Processing: 10, entity.ItemRunState_Success: 17,
	}, nil)
	stats.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(&entity.ExptStats{
		PendingItemCnt: 3, ProcessingItemCnt: 10, SuccessItemCnt: 17,
	}, nil)
	// 一致就不能写：每实验每拍一次写库是纯浪费，而且会把 updated_at 刷成"刚改过"，
	// 让"这行到底动没动"这个排查线索失效。
	stats.EXPECT().UpdateByExptID(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	e.reconcileExptStats(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, SpaceID: 2}, enforceExptForReconcile())
}

func TestReconcileExptStats_SkipsWhenNoItemRows(t *testing.T) {
	e, items, stats, idm := reconcileFixture(t)

	// 实验刚建、item 还没落库。此时覆盖会把 finishExptStart 写下的 PendingItemCnt 抹成 0，
	// 详情页看起来"一条待跑都没有"。
	idm.EXPECT().SetNX(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
	items.EXPECT().CountItemsByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(map[entity.ItemRunState]int64{}, nil)
	stats.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	stats.EXPECT().UpdateByExptID(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	e.reconcileExptStats(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, SpaceID: 2}, enforceExptForReconcile())
}

func TestReconcileExptStats_SkipsOnCountError(t *testing.T) {
	e, items, stats, idm := reconcileFixture(t)

	// 对账是自愈机制，读不到真值时**什么都不做**，绝不能拿一个不完整的分布去覆盖。
	idm.EXPECT().SetNX(gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
	items.EXPECT().CountItemsByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("db down"))
	stats.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	stats.EXPECT().UpdateByExptID(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	e.reconcileExptStats(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, SpaceID: 2}, enforceExptForReconcile())
}

func TestReconcileExptStats_SkipsLegacyExperiment(t *testing.T) {
	e, items, stats, idm := reconcileFixture(t)

	// legacy 的派发是四张表无条件一起写、失败重投整批，计数行天然镜像主表。
	// 实测两个正在跑的 legacy 实验分毫不差 —— 给它对账是纯开销，连节流键都不该占。
	idm.EXPECT().SetNX(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	items.EXPECT().CountItemsByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	stats.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	e.reconcileExptStats(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, SpaceID: 2},
		&entity.Experiment{ID: 1, ExptDispatchMode: entity.ExptDispatchModeLegacy})
}

func TestReconcileExptStats_ThrottledWithinInterval(t *testing.T) {
	e, items, stats, idm := reconcileFixture(t)

	// 节流键还在（本周期已对过）→ 一条 GROUP BY 都不该发。
	// 没有这道闸就是"每实验每拍一条 count"，几百个在跑的实验会打成每分钟几百条。
	idm.EXPECT().SetNX(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil)
	items.EXPECT().CountItemsByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	stats.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	e.reconcileExptStats(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, SpaceID: 2}, enforceExptForReconcile())
}

func TestReconcileExptStats_SkipsWhenThrottleKeyErrors(t *testing.T) {
	e, items, stats, idm := reconcileFixture(t)

	// Redis 抖动时**少做事**：反过来"出错就对账"会让 Redis 一挂就退化成每拍全量 count，
	// 正好是最不该加压的时候。
	idm.EXPECT().SetNX(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, errors.New("redis down"))
	items.EXPECT().CountItemsByStatus(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	stats.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)

	e.reconcileExptStats(context.Background(), &entity.ExptScheduleEvent{ExptID: 1, SpaceID: 2}, enforceExptForReconcile())
}
