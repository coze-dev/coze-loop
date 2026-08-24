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
//
// ★★ 2026-08-24 修正了判据维度：待释放集合改由 **item run log 状态**决定，
// 不再用"未完成 turn"反推。原因见 TestReleaseCentralQuota_ReleasesWhenTurnTerminalButItemNot。

// stubIncompleteItems 让 itemResultRepo 返回指定 item 的未终态 run log。
//
// 判据从 turn 换成 item 之后，这些用例必须经 repo 注入数据 —— 这本身就是一层保障：
// 谁把实现改回按 turn 判，这些 mock 期望就不会被满足（gomock 报 missing call）。
func stubIncompleteItems(t *testing.T, ctrl *gomock.Controller, itemIDs ...int64) *repoMocks.MockIExptItemResultRepo {
	t.Helper()
	repo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	logs := make([]*entity.ExptItemResultRunLog, 0, len(itemIDs))
	for _, id := range itemIDs {
		logs = append(logs, &entity.ExptItemResultRunLog{ItemID: id})
	}
	repo.EXPECT().
		ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(logs, int64(len(logs)), nil).AnyTimes()
	return repo
}

func enforceExpt() *entity.Experiment {
	return &entity.Experiment{
		ID:               100,
		SpaceID:          7,
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
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			guard := &fakeGuard{}
			svc := &ExptMangerImpl{centralGuard: guard, itemResultRepo: stubIncompleteItems(t, ctrl, 11, 12, 13)}

			svc.releaseCentralQuotaForIncompleteItems(context.Background(),
				enforceExpt(), gptr.Of(int64(200)), tc.status)

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

// ★★★ TestReleaseCentralQuota_ReleasesWhenTurnTerminalButItemNot 本次修复的核心回归。
//
// 场景：**turn 已经落终态，而 item 的 run log 仍停在 Processing。**
// 这不是边缘情形，而是沙箱执行进程死亡的典型形态 —— turn 被判完（超时/失败），
// item 那一行却没人去改。2026-08-24 PPE 实测：9 个实验 45 条 reservation
// 就是这么卡住的（11~19 小时），kill 实验完全无效。
//
// 旧实现用 GetIncompleteTurns（只收 turn_status ∈ {Queueing, Processing}）反推待释放 item，
// 于是这一格拿到空列表、一条都不释放。而此时 Redis 侧 state 已是 running：
//   reap 只处理 reserved；对账的 isReleasableWithoutEvidence 刻意排除 running；
//   zombie 只扫 Processing，但实验已终态、daemon 不再跳。
// 三条兜底全不接 ⇒ 永久泄漏。
//
// 这条用例的构造刻意让 **turn 侧完全没有可用信息**（GetIncompleteTurns 若被调用会回空），
// 只有 item run log 有数据 —— 谁把判据改回 turn，这里必然变红。
func TestReleaseCentralQuota_ReleasesWhenTurnTerminalButItemNot(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	guard := &fakeGuard{}
	// item run log 说有两个 item 未终态；turn 侧（若有人去查）什么都给不出来。
	svc := &ExptMangerImpl{centralGuard: guard, itemResultRepo: stubIncompleteItems(t, ctrl, 21, 22)}

	svc.releaseCentralQuotaForIncompleteItems(context.Background(),
		enforceExpt(), gptr.Of(int64(200)), entity.ExptStatus_Terminated)

	releases := guard.releases()
	require.Len(t, releases, 2,
		"★ turn 已终态、item 仍 Processing 时**必须**按 item 释放 —— "+
			"用 turn 反推会拿到空列表，那正是 PPE 上 45 条额度卡死 11~19 小时的原因")
	assert.ElementsMatch(t, []int64{21, 22},
		[]int64{releases[0].ItemID, releases[1].ItemID})
}

// TestReleaseCentralQuota_ScanFailDoesNotGuessItems 查不到未终态 item 时不得瞎释放。
//
// 方向很关键：宁可这次不释放（留给对账/人工），也不能凭空构造 item 列表 ——
// 释放不属于本实验的 item 会归还**别人的**额度，那是超发，比泄漏严重。
func TestReleaseCentralQuota_ScanFailDoesNotGuessItems(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	repo.EXPECT().
		ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, int64(0), assert.AnError).AnyTimes()

	guard := &fakeGuard{}
	svc := &ExptMangerImpl{centralGuard: guard, itemResultRepo: repo}

	assert.NotPanics(t, func() {
		svc.releaseCentralQuotaForIncompleteItems(context.Background(),
			enforceExpt(), gptr.Of(int64(200)), entity.ExptStatus_Terminated)
	})
	assert.Empty(t, guard.releases(), "扫描失败时必须跳过释放，不能猜 item")
}

// TestReleaseCentralQuotaForIncompleteItems_DedupesItemIDs 同一 item 只释放一次。
//
// reservation 是 **item 粒度**，重复释放虽幂等（HDEL 后 field 不存在），
// 但会放大 Redis 往返、且日志计数失真（看起来泄漏了多份）。
func TestReleaseCentralQuotaForIncompleteItems_DedupesItemIDs(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	guard := &fakeGuard{}
	// 同一 item 出现多行（重复投递等异常形态），必须去重。
	svc := &ExptMangerImpl{centralGuard: guard, itemResultRepo: stubIncompleteItems(t, ctrl, 11, 11, 11, 12, 12)}

	svc.releaseCentralQuotaForIncompleteItems(context.Background(),
		enforceExpt(), gptr.Of(int64(200)), entity.ExptStatus_Terminated)

	releases := guard.releases()
	assert.Len(t, releases, 2, "5 行只对应 2 个 item，必须按 item 去重")
	got := []int64{releases[0].ItemID, releases[1].ItemID}
	assert.ElementsMatch(t, []int64{11, 12}, got)
}

// TestReleaseCentralQuotaForIncompleteItems_SkipsLegacy legacy 实验不得触发释放。
//
// 它们从不预占额度，调用释放只是无谓的 Redis 往返 —— 而 CompleteExpt 是所有实验
// （含大量 legacy）的公共收口，白打往返会按实验数放大。
//
// ⚠️ 这条还额外保证：legacy 实验**连 run log 都不该去扫**（省一次大表查询）。
// 所以这里刻意用一个"任何调用都会失败"的 repo —— 谁把 legacy 判断挪到扫描之后，
// gomock 会因为出现未预期的调用而报错。
func TestReleaseCentralQuotaForIncompleteItems_SkipsLegacy(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	guard := &fakeGuard{}
	svc := &ExptMangerImpl{centralGuard: guard, itemResultRepo: repoMocks.NewMockIExptItemResultRepo(ctrl)}

	legacy := enforceExpt()
	legacy.ExptDispatchMode = entity.ExptDispatchModeLegacy

	svc.releaseCentralQuotaForIncompleteItems(context.Background(),
		legacy, gptr.Of(int64(200)), entity.ExptStatus_Terminated)

	assert.Empty(t, guard.releases(), "legacy 实验从不预占，不该发释放请求")
}

// TestReleaseCentralQuotaForIncompleteItems_FallsBackToLatestRunID
// exptRunID 为 nil 时必须回落 LatestRunID。
//
// 账本 key 是 (run_id, item_id)：run 号错了就找不到那条 reservation，
// 释放变成**静默 no-op** —— 比报错更糟，因为看起来成功了。
// 而 CompleteExpt 的调用方并不总会传 exptRunID（Kill 的签名里它是 *int64）。
func TestReleaseCentralQuotaForIncompleteItems_FallsBackToLatestRunID(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	guard := &fakeGuard{}
	svc := &ExptMangerImpl{centralGuard: guard, itemResultRepo: stubIncompleteItems(t, ctrl, 11)}

	svc.releaseCentralQuotaForIncompleteItems(context.Background(),
		enforceExpt(), nil, entity.ExptStatus_Terminated)

	releases := guard.releases()
	require.Len(t, releases, 1)
	assert.Equal(t, int64(200), releases[0].RunID, "exptRunID 为 nil 时应回落 LatestRunID")
}

// TestReleaseCentralQuotaForIncompleteItems_SkipsWhenScopeMissing
// enforce 却无 Scope 时不得瞎猜一本账。
//
// 猜错会归还**别人的**额度 —— 那比不归还严重得多（直接导致超发，且静默）。
//
// ⚠️ 同 legacy 那条：无 Scope 时也不该去扫 run log。用零期望的 mock 保证这一点。
func TestReleaseCentralQuotaForIncompleteItems_SkipsWhenScopeMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	guard := &fakeGuard{}
	svc := &ExptMangerImpl{centralGuard: guard, itemResultRepo: repoMocks.NewMockIExptItemResultRepo(ctrl)}

	noScope := enforceExpt()
	noScope.SchedulerScope = ""

	svc.releaseCentralQuotaForIncompleteItems(context.Background(),
		noScope, gptr.Of(int64(200)), entity.ExptStatus_Terminated)

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
				enforceExpt(), gptr.Of(int64(200)), entity.ExptStatus_Terminated)
		})
	})

	t.Run("无未完成 item", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		guard := &fakeGuard{}
		// 扫描成功但结果为空 —— 全部 item 都已终态，无需释放。
		svc := &ExptMangerImpl{centralGuard: guard, itemResultRepo: stubIncompleteItems(t, ctrl)}
		svc.releaseCentralQuotaForIncompleteItems(context.Background(),
			enforceExpt(), gptr.Of(int64(200)), entity.ExptStatus_Success)
		assert.Empty(t, guard.releases())
	})

	t.Run("expt 为 nil", func(t *testing.T) {
		guard := &fakeGuard{}
		svc := &ExptMangerImpl{centralGuard: guard}
		assert.NotPanics(t, func() {
			svc.releaseCentralQuotaForIncompleteItems(context.Background(),
				nil, gptr.Of(int64(200)), entity.ExptStatus_Terminated)
		})
		assert.Empty(t, guard.releases())
	})

	t.Run("run log 列表含 nil 项", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		repo := repoMocks.NewMockIExptItemResultRepo(ctrl)
		repo.EXPECT().
			ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]*entity.ExptItemResultRunLog{nil, {ItemID: 11}, nil}, int64(3), nil).AnyTimes()

		guard := &fakeGuard{}
		svc := &ExptMangerImpl{centralGuard: guard, itemResultRepo: repo}
		assert.NotPanics(t, func() {
			svc.releaseCentralQuotaForIncompleteItems(context.Background(),
				enforceExpt(), gptr.Of(int64(200)), entity.ExptStatus_Terminated)
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
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	guard := &fakeGuard{releaseErr: assert.AnError}
	svc := &ExptMangerImpl{centralGuard: guard, itemResultRepo: stubIncompleteItems(t, ctrl, 11, 12, 13)}

	assert.NotPanics(t, func() {
		svc.releaseCentralQuotaForIncompleteItems(context.Background(),
			enforceExpt(), gptr.Of(int64(200)), entity.ExptStatus_Terminated)
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

	// ★★ turn 侧**刻意返回空**：模拟"turn 已全部落终态、item 仍 Processing"。
	//
	// 这是本次修复的核心场景（沙箱进程死亡的典型形态）。旧实现拿这个空列表去释放，
	// 于是一条都不放；改成按 item run log 查之后，下面的 ScanItemRunLogs 才是真值来源。
	// 谁把判据改回 turn，这条用例会因为 releases 为空而变红。
	mgr.exptResultService.(*svcMocks.MockExptResultService).EXPECT().
		GetIncompleteTurns(ctx, exptID, spaceID, session).
		Return(nil, nil).AnyTimes()

	// item run log 才是待释放集合的真值：两个 item 未终态 → 两份额度必须归还。
	mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo).EXPECT().
		ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*entity.ExptItemResultRunLog{{ItemID: 11}, {ItemID: 12}}, int64(2), nil).AnyTimes()

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

// ============================================================================
// 删除路径的额度释放（P0-2）
//
// 删除是**独立于 CompleteExpt 的一条泄漏路径**，三条独立原因：
//   ① 实验软删后 consumer 侧 GetByID 拿不到实验、直接退出 ⇒ 那些 item 永不执行，
//      "item 到终态才释放"的出口永远不会被走到；
//   ② 删除压根不经过 CompleteExpt，那里的释放不在这条路径上；而 CompleteExpt 对已删实验
//      还有一条 early return，所以"先删再 kill"同样救不回来；
//   ③ 软删后 ScanSchedulerQueue 带 deleted_at IS NULL ⇒ **连 full recovery 都扫不到**，
//      这些 reservation 会永久留在账本里。
//
// 两个删除入口（Delete / MDelete）都要有防线 —— 只守一个等于留一半泄漏。
// ============================================================================

// stubDeletableExpt 造一个"仍有未跑完 item"的 enforce 实验，供删除路径用例复用。
func stubDeletableExpt(exptID, runID int64) *entity.Experiment {
	return &entity.Experiment{
		ID:               exptID,
		SpaceID:          7,
		LatestRunID:      runID,
		SchedulerScope:   testScope,
		ExptDispatchMode: entity.ExptDispatchModeEnforce,
	}
}

// TestDelete_ReleasesCentralQuota 单实验删除必须归还额度。
func TestDelete_ReleasesCentralQuota(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)

	guard := &fakeGuard{}
	mgr.centralGuard = guard

	ctx := context.Background()
	const exptID, spaceID, runID = int64(321), int64(7), int64(654)

	mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().
		GetByID(ctx, exptID, spaceID).Return(stubDeletableExpt(exptID, runID), nil)
	mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo).EXPECT().
		ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*entity.ExptItemResultRunLog{{ItemID: 31}, {ItemID: 32}}, int64(2), nil).AnyTimes()
	mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().
		Delete(ctx, exptID, spaceID).Return(nil)

	require.NoError(t, mgr.Delete(ctx, exptID, spaceID, &entity.Session{UserID: "u"}))

	releases := guard.releases()
	require.Len(t, releases, 2,
		"删除实验必须归还未跑完 item 的额度 —— 软删后 full recovery 都扫不到它们（deleted_at IS NULL），"+
			"不在这里放就是永久泄漏")
	assert.ElementsMatch(t, []int64{31, 32}, []int64{releases[0].ItemID, releases[1].ItemID})
	for _, r := range releases {
		assert.Equal(t, testScope, r.Scope)
		assert.Equal(t, runID, r.RunID, "exptRunID 传 nil 时应回落 LatestRunID")
	}
}

// TestMDelete_ReleasesCentralQuotaForEachExpt 批量删除要对**每个**实验都释放。
//
// 单独测批量版：它与 Delete 是两份独立实现（一个 GetByID + Delete、一个 MGetByID + MDelete），
// 只改一处会留一半泄漏，而"批量入口漏了"在线上更常见（用户多选删除）。
func TestMDelete_ReleasesCentralQuotaForEachExpt(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)

	guard := &fakeGuard{}
	mgr.centralGuard = guard

	ctx := context.Background()
	const spaceID = int64(7)
	exptIDs := []int64{401, 402}

	mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().
		MGetByID(ctx, exptIDs, spaceID).
		Return([]*entity.Experiment{
			stubDeletableExpt(401, 4010),
			stubDeletableExpt(402, 4020),
		}, nil)
	// 每个实验各有一个未跑完 item。
	mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo).EXPECT().
		ScanItemRunLogs(gomock.Any(), int64(401), int64(4010), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*entity.ExptItemResultRunLog{{ItemID: 41}}, int64(1), nil)
	mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo).EXPECT().
		ScanItemRunLogs(gomock.Any(), int64(402), int64(4020), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*entity.ExptItemResultRunLog{{ItemID: 42}}, int64(1), nil)
	mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().
		MDelete(ctx, exptIDs, spaceID).Return(nil)

	require.NoError(t, mgr.MDelete(ctx, exptIDs, spaceID, &entity.Session{UserID: "u"}))

	releases := guard.releases()
	require.Len(t, releases, 2, "批量删除必须逐个实验释放，漏一个就是那个实验的额度永久泄漏")
	assert.ElementsMatch(t, []int64{41, 42}, []int64{releases[0].ItemID, releases[1].ItemID})
	// run 号必须各归各的 —— 账本 key 是 (run_id, item_id)，串了就找不到那条 reservation。
	byItem := map[int64]int64{}
	for _, r := range releases {
		byItem[r.ItemID] = r.RunID
	}
	assert.Equal(t, int64(4010), byItem[41])
	assert.Equal(t, int64(4020), byItem[42])
}

// ★ TestMDelete_ReleasesBeforeSoftDelete 释放必须发生在软删**之前**。
//
// 顺序不是风格问题：删完再释放的话，中途任何失败都让额度彻底失去归还机会
// （实验已不可查，SchedulerScope / LatestRunID 都拿不到了）。反过来"先释放但删除失败"
// 只是让一个仍存在的实验少占额度，下一拍调度会重新预占，无损。
//
// 用 gomock 的调用顺序断言钉死它 —— 只测"释放了几条"是测不出顺序的。
func TestMDelete_ReleasesBeforeSoftDelete(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)

	guard := &fakeGuard{}
	mgr.centralGuard = guard

	ctx := context.Background()
	const spaceID = int64(7)
	exptIDs := []int64{501}

	mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().
		MGetByID(ctx, exptIDs, spaceID).
		Return([]*entity.Experiment{stubDeletableExpt(501, 5010)}, nil)

	scan := mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo).EXPECT().
		ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*entity.ExptItemResultRunLog{{ItemID: 51}}, int64(1), nil)

	// ★ After(scan)：软删必须在扫描（= 释放的前置步骤）之后发生。
	mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().
		MDelete(ctx, exptIDs, spaceID).Return(nil).After(scan)

	require.NoError(t, mgr.MDelete(ctx, exptIDs, spaceID, &entity.Session{UserID: "u"}))
	assert.Len(t, guard.releases(), 1)
}

// TestMDelete_LegacyExptSkipsRelease legacy 实验删除时不该产生任何额度调用。
//
// 删除是全量实验（含大量 legacy）的公共入口，白打 Redis 往返会按删除量放大。
// 这里用零期望的 itemResultRepo mock：legacy 判断必须在扫 run log **之前**短路，
// 否则 gomock 会因未预期的 ScanItemRunLogs 调用而报错。
func TestMDelete_LegacyExptSkipsRelease(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)

	guard := &fakeGuard{}
	mgr.centralGuard = guard

	ctx := context.Background()
	const spaceID = int64(7)
	exptIDs := []int64{601}

	legacy := stubDeletableExpt(601, 6010)
	legacy.ExptDispatchMode = entity.ExptDispatchModeLegacy

	mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().
		MGetByID(ctx, exptIDs, spaceID).Return([]*entity.Experiment{legacy}, nil)
	mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().
		MDelete(ctx, exptIDs, spaceID).Return(nil)

	require.NoError(t, mgr.MDelete(ctx, exptIDs, spaceID, &entity.Session{UserID: "u"}))
	assert.Empty(t, guard.releases(), "legacy 实验从不预占，删除时不该发释放请求")
}
