// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	idemMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	metricsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

// 本文件守住「实验终态时，未跑完 item 的中心额度必须被释放」。
//
// 为什么这条缺口在线上必然被走到：Kill / Cancel 是常规操作。用户取消一个正在跑
// 100 个 item 的 enforce 实验后，那些 item **不会再被执行** —— 消息可能已被丢弃、
// 可能还没投递、也可能 consumer 早已放弃。于是 consumer 侧那个"item 到终态才释放"的
// 出口永远不会被走到，reservation 永久留在账本里。
//
// 更糟的是 running 态的悬挂连调度器每拍的对账都不碰
// （reapDanglingReservations 显式 `if !view.IsReserved() { continue }`）——
// 也就是说这条路径**没有任何兜底**。审计前本文件 grep centralGuard 为 0。

func incompleteTurns(itemIDs ...int64) []*entity.ItemTurnID {
	out := make([]*entity.ItemTurnID, 0, len(itemIDs))
	for _, id := range itemIDs {
		out = append(out, &entity.ItemTurnID{ItemID: id, TurnID: id * 10})
	}
	return out
}

func enforceExpt() *entity.Experiment {
	return &entity.Experiment{
		ID:               100,
		LatestRunID:      200,
		SchedulerScope:   testScope,
		ExptDispatchMode: entity.ExptDispatchModeEnforce,
	}
}

// TestReleaseCentralQuotaForIncompleteItems_ReleasesOnTerminal 核心回归：
// enforce 实验终态时，未完成 item 的额度必须逐个释放。
func TestReleaseCentralQuotaForIncompleteItems_ReleasesOnTerminal(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status entity.ExptStatus
	}{
		{"Terminated（用户主动取消）", entity.ExptStatus_Terminated},
		{"Failed（调度锁超时 / run 级僵尸等）", entity.ExptStatus_Failed},
		{"Success（正常完成但仍有未完成 item）", entity.ExptStatus_Success},
	} {
		t.Run(tc.name, func(t *testing.T) {
			guard := &fakeGuard{}
			svc := &ExptMangerImpl{centralGuard: guard}

			svc.releaseCentralQuotaForIncompleteItems(context.Background(),
				enforceExpt(), gptr.Of(int64(200)), incompleteTurns(11, 12, 13), tc.status)

			releases := guard.releases()
			require.Len(t, releases, 3, "三个未完成 item 的额度都必须释放，否则永久泄漏")
			for _, r := range releases {
				assert.Equal(t, testScope, r.Scope)
				assert.Equal(t, int64(200), r.RunID)
				// reason 会进日志，是排查"这批额度什么时候被谁放掉的"唯一线索。
				assert.Contains(t, r.Reason, "terminal")
			}
		})
	}
}

// TestReleaseCentralQuotaForIncompleteItems_DedupesItemIDs 同一 item 的多个未完成 turn
// 只能释放一次。
//
// reservation 是 **item 粒度**而 incompleteTurnIDs 是 **turn 粒度**：一个 item 有 5 个
// 未完成 turn 时，不去重就会对同一条 reservation 发 5 次释放。Release 本身幂等
// （HDEL 后 field 不存在），但会放大 Redis 往返、且日志计数失真（看起来泄漏了 5 份）。
func TestReleaseCentralQuotaForIncompleteItems_DedupesItemIDs(t *testing.T) {
	guard := &fakeGuard{}
	svc := &ExptMangerImpl{centralGuard: guard}

	// item 11 有三个未完成 turn，item 12 有两个。
	turns := []*entity.ItemTurnID{
		{ItemID: 11, TurnID: 1}, {ItemID: 11, TurnID: 2}, {ItemID: 11, TurnID: 3},
		{ItemID: 12, TurnID: 4}, {ItemID: 12, TurnID: 5},
	}

	svc.releaseCentralQuotaForIncompleteItems(context.Background(),
		enforceExpt(), gptr.Of(int64(200)), turns, entity.ExptStatus_Terminated)

	releases := guard.releases()
	assert.Len(t, releases, 2, "5 个 turn 只对应 2 个 item，必须按 item 去重")
	got := []int64{releases[0].ItemID, releases[1].ItemID}
	assert.ElementsMatch(t, []int64{11, 12}, got)
}

// TestReleaseCentralQuotaForIncompleteItems_SkipsLegacy legacy 实验不得触发释放。
//
// 它们从不预占额度，调用释放只是无谓的 Redis 往返 —— 而 CompleteExpt 是所有实验
// （含大量 legacy）的公共收口，白打往返会按实验数放大。
func TestReleaseCentralQuotaForIncompleteItems_SkipsLegacy(t *testing.T) {
	guard := &fakeGuard{}
	svc := &ExptMangerImpl{centralGuard: guard}

	legacy := enforceExpt()
	legacy.ExptDispatchMode = entity.ExptDispatchModeLegacy

	svc.releaseCentralQuotaForIncompleteItems(context.Background(),
		legacy, gptr.Of(int64(200)), incompleteTurns(11), entity.ExptStatus_Terminated)

	assert.Empty(t, guard.releases(), "legacy 实验从不预占，不该发释放请求")
}

// TestReleaseCentralQuotaForIncompleteItems_FallsBackToLatestRunID
// exptRunID 为 nil 时必须回落 LatestRunID。
//
// 账本 key 是 (run_id, item_id)：run 号错了就找不到那条 reservation，
// 释放变成**静默 no-op** —— 比报错更糟，因为看起来成功了。
// 而 CompleteExpt 的调用方并不总会传 exptRunID（Kill 的签名里它是 *int64）。
func TestReleaseCentralQuotaForIncompleteItems_FallsBackToLatestRunID(t *testing.T) {
	guard := &fakeGuard{}
	svc := &ExptMangerImpl{centralGuard: guard}

	svc.releaseCentralQuotaForIncompleteItems(context.Background(),
		enforceExpt(), nil, incompleteTurns(11), entity.ExptStatus_Terminated)

	releases := guard.releases()
	require.Len(t, releases, 1)
	assert.Equal(t, int64(200), releases[0].RunID, "exptRunID 为 nil 时应回落 LatestRunID")
}

// TestReleaseCentralQuotaForIncompleteItems_SkipsWhenScopeMissing
// enforce 却无 Scope 时不得瞎猜一本账。
//
// 猜错会归还**别人的**额度 —— 那比不归还严重得多（直接导致超发，且静默）。
func TestReleaseCentralQuotaForIncompleteItems_SkipsWhenScopeMissing(t *testing.T) {
	guard := &fakeGuard{}
	svc := &ExptMangerImpl{centralGuard: guard}

	noScope := enforceExpt()
	noScope.SchedulerScope = ""

	svc.releaseCentralQuotaForIncompleteItems(context.Background(),
		noScope, gptr.Of(int64(200)), incompleteTurns(11), entity.ExptStatus_Terminated)

	assert.Empty(t, guard.releases(), "无 Scope 时释放会归还别人的额度，必须跳过")
}

// TestReleaseCentralQuotaForIncompleteItems_NoopOnEmptyInputs 边界：
// guard 为 nil（legacy 部署）、无未完成 item、expt 为 nil 时都必须安静返回。
//
// CompleteExpt 是所有实验终态的公共收口，这里 panic 会让**实验无法收敛** ——
// 用户看到"点了取消但状态不变"，比额度泄漏严重得多。
func TestReleaseCentralQuotaForIncompleteItems_NoopOnEmptyInputs(t *testing.T) {
	t.Run("guard 为 nil", func(t *testing.T) {
		svc := &ExptMangerImpl{}
		assert.NotPanics(t, func() {
			svc.releaseCentralQuotaForIncompleteItems(context.Background(),
				enforceExpt(), gptr.Of(int64(200)), incompleteTurns(11), entity.ExptStatus_Terminated)
		})
	})

	t.Run("无未完成 item", func(t *testing.T) {
		guard := &fakeGuard{}
		svc := &ExptMangerImpl{centralGuard: guard}
		svc.releaseCentralQuotaForIncompleteItems(context.Background(),
			enforceExpt(), gptr.Of(int64(200)), nil, entity.ExptStatus_Success)
		assert.Empty(t, guard.releases())
	})

	t.Run("expt 为 nil", func(t *testing.T) {
		guard := &fakeGuard{}
		svc := &ExptMangerImpl{centralGuard: guard}
		assert.NotPanics(t, func() {
			svc.releaseCentralQuotaForIncompleteItems(context.Background(),
				nil, gptr.Of(int64(200)), incompleteTurns(11), entity.ExptStatus_Terminated)
		})
		assert.Empty(t, guard.releases())
	})

	t.Run("turn 列表含 nil 项", func(t *testing.T) {
		guard := &fakeGuard{}
		svc := &ExptMangerImpl{centralGuard: guard}
		turns := []*entity.ItemTurnID{nil, {ItemID: 11, TurnID: 1}, nil}
		assert.NotPanics(t, func() {
			svc.releaseCentralQuotaForIncompleteItems(context.Background(),
				enforceExpt(), gptr.Of(int64(200)), turns, entity.ExptStatus_Terminated)
		})
		assert.Len(t, guard.releases(), 1, "nil 项应被跳过，有效项仍要释放")
	})
}

// TestReleaseCentralQuotaForIncompleteItems_ReleaseErrorDoesNotPanic
// 单个 item 释放失败不得中断其余 item 的释放。
//
// best-effort 的方向很关键：一个 item 释放失败就放弃剩下 99 个，等于把"泄漏 1 份"
// 放大成"泄漏 100 份"。
func TestReleaseCentralQuotaForIncompleteItems_ReleaseErrorDoesNotPanic(t *testing.T) {
	guard := &fakeGuard{releaseErr: assert.AnError}
	svc := &ExptMangerImpl{centralGuard: guard}

	assert.NotPanics(t, func() {
		svc.releaseCentralQuotaForIncompleteItems(context.Background(),
			enforceExpt(), gptr.Of(int64(200)), incompleteTurns(11, 12, 13), entity.ExptStatus_Terminated)
	})
	assert.Len(t, guard.releases(), 3, "前一个失败不该阻断后续 item 的释放尝试")
}

// TestCompleteExpt_ReleasesCentralQuotaForIncompleteItems ★ 守住**调用点**，不只是那个函数。
//
// 为什么必须单独有这条：上面那些用例都直接调 releaseCentralQuotaForIncompleteItems，
// 所以把 CompleteExpt 里的调用整行删掉，它们**依然全绿** —— 变异验证时实测过。
// 也就是说没有这条，"函数写对了但根本没被接上"这种失败模式完全没有防线，
// 而这恰恰是本次修复要补的缺口本身的形态（审计前 grep centralGuard 为 0）。
func TestCompleteExpt_ReleasesCentralQuotaForIncompleteItems(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)

	guard := &fakeGuard{}
	mgr.centralGuard = guard

	ctx := context.Background()
	session := &entity.Session{UserID: "test_user"}
	const exptID, spaceID, runID = int64(123), int64(789), int64(456)

	mgr.idem.(*idemMocks.MockIdempotentService).EXPECT().Exist(ctx, gomock.Any()).AnyTimes().Return(false, nil)

	// ★ enforce 实验 + 冻结 Scope：这是走到释放逻辑的前提。
	mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().GetByID(ctx, exptID, spaceID).Return(&entity.Experiment{
		ID:               exptID,
		SpaceID:          spaceID,
		ExptType:         entity.ExptType_Offline,
		StartAt:          gptr.Of(time.Now()),
		LatestRunID:      runID,
		SchedulerScope:   testScope,
		ExptDispatchMode: entity.ExptDispatchModeEnforce,
	}, nil)

	mgr.exptResultService.(*svcMocks.MockExptResultService).EXPECT().
		CalculateStats(ctx, exptID, spaceID, session).
		Return(&entity.ExptCalculateStats{ProcessingItemCnt: 2}, nil)

	// 两个 item 尚未跑完 —— 实验被终止后它们永远不会执行，额度必须在这里归还。
	mgr.exptResultService.(*svcMocks.MockExptResultService).EXPECT().
		GetIncompleteTurns(ctx, exptID, spaceID, session).
		Return([]*entity.ItemTurnID{{ItemID: 11, TurnID: 1}, {ItemID: 12, TurnID: 2}}, nil)

	mgr.statsRepo.(*repoMocks.MockIExptStatsRepo).EXPECT().
		UpdateByExptID(ctx, exptID, spaceID, gomock.Any()).Return(nil)
	// 这两个用 AnyTimes：Terminated 路径下是否调用取决于分支细节，本用例只关心额度释放。
	mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mgr.quotaRepo.(*repoMocks.MockQuotaRepo).EXPECT().
		CreateOrUpdate(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mgr.exptAggrResultService.(*svcMocks.MockExptAggrResultService).EXPECT().
		PublishExptAggrResultEvent(ctx, gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mgr.mtr.(*metricsMocks.MockExptMetric).EXPECT().
		EmitExptExecResult(spaceID, gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mgr.publisher.(*eventsMocks.MockExptEventPublisher).EXPECT().
		PublishExptLifecycleEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	// Terminated 走 terminateItemTurns 分支，这些是该分支的附带调用。
	mgr.turnResultRepo.(*repoMocks.MockIExptTurnResultRepo).EXPECT().
		SaveTurnRunLogs(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mgr.turnResultRepo.(*repoMocks.MockIExptTurnResultRepo).EXPECT().
		UpdateTurnResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mgr.turnResultRepo.(*repoMocks.MockIExptTurnResultRepo).EXPECT().
		MGetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).AnyTimes()
	mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo).EXPECT().
		UpdateItemsResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo).EXPECT().
		UpdateItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mgr.exptResultService.(*svcMocks.MockExptResultService).EXPECT().
		UpsertExptTurnResultFilter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	err := mgr.CompleteExpt(ctx, exptID, gptr.Of(runID), spaceID, session,
		entity.WithStatus(entity.ExptStatus_Terminated))
	assert.NoError(t, err)

	releases := guard.releases()
	require.Len(t, releases, 2,
		"CompleteExpt 必须为未完成的 item 释放额度 —— 少了这一步，取消实验就是永久泄漏")
	assert.ElementsMatch(t, []int64{11, 12},
		[]int64{releases[0].ItemID, releases[1].ItemID})
	for _, r := range releases {
		assert.Equal(t, testScope, r.Scope)
		assert.Equal(t, runID, r.RunID)
	}
}
