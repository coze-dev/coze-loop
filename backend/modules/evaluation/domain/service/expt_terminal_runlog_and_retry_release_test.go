// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	idgenMocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	lockMocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	idemMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/idem/mocks"
	metricsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventsMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
)

// 本文件守住两条独立的额度泄漏路径，两者都是 2026-08-26 PPE 长跑实测出来的：
//
//	① 实验终态时 item run log 漏写终态 ⇒ 飞行中的 item 预占永久泄漏（6 条 48h+ 僵尸）
//	② 重试新建 run 时不释放旧 run 的预占 ⇒ 增量对账永远够不着（主表 run 号已被改写）
//
// 两条的共同点是**没有任何兜底会兜住**，所以调用点本身就是唯一防线 ——
// 因此每条都必须有一个"经真实入口"的用例，而不只是直接调那个私有函数。

// ============================================================================
// ① 实验终态 ⇒ item run log 一并置终态
// ============================================================================

// TestTerminateIncompleteItemRunLogs_WritesTerminal 直接守住写入内容。
//
// 断言的是 ufields 里 status == Terminal 且 item 集合等于扫描结果 ——
// 不是"UpdateItemRunLog 被调过"。写错状态值（例如写成 Fail）与不写一样有害：
// 调度侧判据只看 IsItemRunFinished，但结果页会把用户主动取消的 item 显示成失败。
func TestTerminateIncompleteItemRunLogs_WritesTerminal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	itemRepo.EXPECT().
		ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*entity.ExptItemResultRunLog{{ItemID: 11}, {ItemID: 12}}, int64(2), nil)
	itemRepo.EXPECT().
		UpdateItemRunLog(gomock.Any(), int64(100), int64(200), gomock.Any(), gomock.Any(), int64(7)).
		DoAndReturn(func(_ context.Context, _, _ int64, itemIDs []int64, ufields map[string]any, _ int64) error {
			assert.ElementsMatch(t, []int64{11, 12}, itemIDs)
			assert.Equal(t, int32(entity.ItemRunState_Terminal), ufields["status"],
				"run log 必须落 Terminal —— 停在 Processing 会让释放判据、对账、zombie 三条路全部失效")
			return nil
		}).Times(1)

	svc := &ExptMangerImpl{itemResultRepo: itemRepo}
	svc.terminateIncompleteItemRunLogs(context.Background(), enforceExpt(), gptr.Of(int64(200)))
}

// TestTerminateIncompleteItemRunLogs_SkipsWhenAllTerminal 已全部终态时不该发写请求。
func TestTerminateIncompleteItemRunLogs_SkipsWhenAllTerminal(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	itemRepo.EXPECT().
		ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, int64(0), nil)
	// 刻意不声明 UpdateItemRunLog：真发了就是 unexpected call。

	svc := &ExptMangerImpl{itemResultRepo: itemRepo}
	svc.terminateIncompleteItemRunLogs(context.Background(), enforceExpt(), gptr.Of(int64(200)))
}

// TestTerminateIncompleteItemRunLogs_SkipsLegacy legacy 实验一次查询都不该发。
//
// run log 的 status 不是用户可见字段（结果页只从它取 log_id），实验终态后读它的只有
// 额度释放判据 / 对账 / zombie 三条中心调度链路，legacy 一条都不走。
// CompleteExpt 是每个实验必经路径，多一次全量 scan 对存量用户是纯开销。
func TestTerminateIncompleteItemRunLogs_SkipsLegacy(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	// 刻意不声明任何期望：legacy 走到 repo 就是 unexpected call。
	svc := &ExptMangerImpl{itemResultRepo: repoMocks.NewMockIExptItemResultRepo(ctrl)}
	expt := enforceExpt()
	expt.ExptDispatchMode = entity.ExptDispatchModeLegacy
	svc.terminateIncompleteItemRunLogs(context.Background(), expt, gptr.Of(int64(200)))
}

// TestTerminateIncompleteItemRunLogs_ScanFailDoesNotWrite 扫不到就别猜。
func TestTerminateIncompleteItemRunLogs_ScanFailDoesNotWrite(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	itemRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	itemRepo.EXPECT().
		ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, int64(0), assert.AnError)

	svc := &ExptMangerImpl{itemResultRepo: itemRepo}
	assert.NotPanics(t, func() {
		svc.terminateIncompleteItemRunLogs(context.Background(), enforceExpt(), gptr.Of(int64(200)))
	})
}

// TestCompleteExpt_TerminatesItemRunLogsAfterRelease ★ 守住调用点**和顺序**。
//
// 两件事必须一起断言，因为它们的失败模式相反：
//   - 调用行被删 ⇒ run log 永远停在 Processing（原 bug 复现）；
//   - 调用行被挪到释放之前 ⇒ 释放侧按 Queueing/Processing 反查待释放集合时一条都查不到，
//     变成**静默不释放** —— 比原 bug 更难发现（日志显示"已收口"，账本却没动）。
//
// 所以这里用 releaseSeenAtRunLogWrite 记录"写 run log 那一刻已经释放了几条"，
// 顺序颠倒时它会是 0，用例变红。
func TestCompleteExpt_TerminatesItemRunLogsAfterRelease(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)

	guard := &fakeGuard{}
	mgr.centralGuard = guard

	ctx := context.Background()
	session := &entity.Session{UserID: "test_user"}
	const exptID, spaceID, runID = int64(123), int64(789), int64(456)

	mgr.idem.(*idemMocks.MockIdempotentService).EXPECT().Exist(ctx, gomock.Any()).AnyTimes().Return(false, nil)
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
	mgr.exptResultService.(*svcMocks.MockExptResultService).EXPECT().
		GetIncompleteTurns(ctx, exptID, spaceID, session).Return(nil, nil).AnyTimes()
	mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo).EXPECT().
		ScanItemRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*entity.ExptItemResultRunLog{{ItemID: 11}, {ItemID: 12}}, int64(2), nil).AnyTimes()

	releaseSeenAtRunLogWrite := -1
	runLogTerminated := 0
	mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo).EXPECT().
		UpdateItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ int64, itemIDs []int64, ufields map[string]any, _ int64) error {
			releaseSeenAtRunLogWrite = len(guard.releases())
			if ufields["status"] == int32(entity.ItemRunState_Terminal) {
				runLogTerminated += len(itemIDs)
			}
			return nil
		}).AnyTimes()

	mgr.statsRepo.(*repoMocks.MockIExptStatsRepo).EXPECT().
		UpdateByExptID(ctx, exptID, spaceID, gomock.Any()).Return(nil)
	mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mgr.quotaRepo.(*repoMocks.MockQuotaRepo).EXPECT().
		CreateOrUpdate(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mgr.exptAggrResultService.(*svcMocks.MockExptAggrResultService).EXPECT().
		PublishExptAggrResultEvent(ctx, gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mgr.mtr.(*metricsMocks.MockExptMetric).EXPECT().
		EmitExptExecResult(spaceID, gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	mgr.publisher.(*eventsMocks.MockExptEventPublisher).EXPECT().
		PublishExptLifecycleEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mgr.turnResultRepo.(*repoMocks.MockIExptTurnResultRepo).EXPECT().
		SaveTurnRunLogs(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mgr.turnResultRepo.(*repoMocks.MockIExptTurnResultRepo).EXPECT().
		UpdateTurnResults(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mgr.turnResultRepo.(*repoMocks.MockIExptTurnResultRepo).EXPECT().
		MGetItemTurnRunLogs(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
	mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo).EXPECT().
		UpdateItemsResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	mgr.exptResultService.(*svcMocks.MockExptResultService).EXPECT().
		UpsertExptTurnResultFilter(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	require.NoError(t, mgr.CompleteExpt(ctx, exptID, gptr.Of(runID), spaceID, session,
		entity.WithStatus(entity.ExptStatus_Terminated)))

	assert.Equal(t, 2, runLogTerminated,
		"CompleteExpt 必须把未终态 item 的 run log 置 Terminal —— 漏写就是永久泄漏（实测 6 条 48h+ 僵尸）")
	assert.Equal(t, 2, releaseSeenAtRunLogWrite,
		"必须先释放再置终态：反过来会让释放侧一条都查不到，退化成静默不释放")
}

// ============================================================================
// ② 重试新建 run ⇒ 释放被顶替的旧 run 预占
// ============================================================================

// stubRetryLogRunMocks 备好 LogRun 走通所需的最小 mock 集合。
func stubRetryLogRunMocks(t *testing.T, mgr *ExptMangerImpl, spaceID int64, mode entity.ExptRunMode) {
	t.Helper()
	mgr.mutex.(*lockMocks.MockILocker).EXPECT().
		LockBackoff(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(true, nil)
	mgr.mtr.(*metricsMocks.MockExptMetric).EXPECT().EmitExptExecRun(spaceID, int64(mode)).AnyTimes()
	mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
	mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
}

// TestLogRun_ReleasesSupersededRunQuotaOnRetry ★ 核心回归：重试必须归还旧 run 的预占。
//
// 断言 RunID 是**旧** run 号：账本 key 是 (run_id, item_id)，拿新 run 号去释放是静默 no-op ——
// 日志照样打"已释放"，账本一动不动。这正是最容易写错、又最难从日志发现的一种失败。
func TestLogRun_ReleasesSupersededRunQuotaOnRetry(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{
		entity.EvaluationModeFailRetry,
		entity.EvaluationModeRetryAll,
		entity.EvaluationModeRetryItems,
	} {
		t.Run(fmt.Sprintf("mode=%d", int64(mode)), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mgr := newTestExptManager(ctrl)
			guard := &fakeGuard{}
			mgr.centralGuard = guard

			const exptID, spaceID, oldRunID, newRunID = int64(100), int64(7), int64(200), int64(201)
			stubRetryLogRunMocks(t, mgr, spaceID, mode)
			mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().
				GetByID(gomock.Any(), exptID, spaceID).Return(&entity.Experiment{
				ID:               exptID,
				SpaceID:          spaceID,
				LatestRunID:      oldRunID,
				SchedulerScope:   testScope,
				ExptDispatchMode: entity.ExptDispatchModeEnforce,
			}, nil)
			mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo).EXPECT().
				ScanItemRunLogs(gomock.Any(), exptID, oldRunID, gomock.Any(), gomock.Any(), gomock.Any(), spaceID).
				Return([]*entity.ExptItemResultRunLog{{ItemID: 11}}, int64(1), nil)

			require.NoError(t, mgr.LogRun(context.Background(), exptID, newRunID, mode, spaceID, nil,
				&entity.Session{UserID: "u"}))

			releases := guard.releases()
			require.Len(t, releases, 1, "重试必须释放旧 run 的残留预占 —— 增量对账够不着它")
			assert.Equal(t, oldRunID, releases[0].RunID,
				"必须按**旧** run 号释放：账本 key 是 (run_id, item_id)，用新 run 号是静默 no-op")
			assert.Equal(t, int64(11), releases[0].ItemID)
			assert.Equal(t, testScope, releases[0].Scope)
		})
	}
}

// TestLogRun_SkipsReleaseForNonRetryModes 首跑 / 试运行不得释放。
//
// Submit 的 LatestRunID 若非 0（例如重跑一个跑完的实验），按旧 run 释放没有依据；
// 更危险的是 Append —— 它可能在旧 run 仍有 item 在飞时追加，误释放等于真超发。
// 这里靠"不声明 GetByID 期望"来断言：真去查了就是 unexpected call。
func TestLogRun_SkipsReleaseForNonRetryModes(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{
		entity.EvaluationModeSubmit,
		entity.EvaluationModeTrialRun,
		entity.EvaluationModeAppend,
	} {
		t.Run(fmt.Sprintf("mode=%d", int64(mode)), func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			mgr := newTestExptManager(ctrl)
			guard := &fakeGuard{}
			mgr.centralGuard = guard

			const exptID, spaceID, newRunID = int64(100), int64(7), int64(201)
			stubRetryLogRunMocks(t, mgr, spaceID, mode)

			require.NoError(t, mgr.LogRun(context.Background(), exptID, newRunID, mode, spaceID, nil,
				&entity.Session{UserID: "u"}))
			assert.Empty(t, guard.releases(), "非重试模式没有被顶替的旧 run，不该释放任何额度")
		})
	}
}

// TestLogRetryItemsRun_ReleasesSupersededRunQuota RetryItems 走的是另一个入口，必须单独守。
func TestLogRetryItemsRun_ReleasesSupersededRunQuota(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	guard := &fakeGuard{}
	mgr.centralGuard = guard

	const exptID, spaceID, oldRunID, newRunID = int64(100), int64(7), int64(200), int64(201)

	mgr.idgenerator.(*idgenMocks.MockIIDGenerator).EXPECT().GenID(gomock.Any()).Return(newRunID, nil)
	mgr.mutex.(*lockMocks.MockILocker).EXPECT().
		BackoffLockWithValue(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(true, "", nil)
	mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
	mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().
		GetByID(gomock.Any(), exptID, spaceID).Return(&entity.Experiment{
		ID:               exptID,
		SpaceID:          spaceID,
		LatestRunID:      oldRunID,
		SchedulerScope:   testScope,
		ExptDispatchMode: entity.ExptDispatchModeEnforce,
	}, nil)
	mgr.itemResultRepo.(*repoMocks.MockIExptItemResultRepo).EXPECT().
		ScanItemRunLogs(gomock.Any(), exptID, oldRunID, gomock.Any(), gomock.Any(), gomock.Any(), spaceID).
		Return([]*entity.ExptItemResultRunLog{{ItemID: 11}, {ItemID: 12}}, int64(2), nil)
	mgr.exptRepo.(*repoMocks.MockIExperimentRepo).EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)
	mgr.mtr.(*metricsMocks.MockExptMetric).EXPECT().EmitExptExecRun(spaceID, gomock.Any()).AnyTimes()

	runID, retried, err := mgr.LogRetryItemsRun(context.Background(), exptID,
		entity.EvaluationModeRetryItems, spaceID, []int64{11, 12}, &entity.Session{UserID: "u"})
	require.NoError(t, err)
	assert.Equal(t, newRunID, runID)
	assert.False(t, retried)
	require.Len(t, guard.releases(), 2)
	for _, r := range guard.releases() {
		assert.Equal(t, oldRunID, r.RunID)
	}
}

// TestLogRetryItemsRun_SkipsReleaseWhenRunAlive ★ 反向防线：锁被活着的 run 持有时绝不能释放。
//
// retried == true 说明 runID 是**正在跑的那个 run**，LatestRunID 也还是它。
// 此时释放等于把正在执行的 item 的额度还回账本 —— 那不是泄漏，是**真超发**，
// 比泄漏严重得多（会让别的实验拿到不存在的额度）。
func TestLogRetryItemsRun_SkipsReleaseWhenRunAlive(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mgr := newTestExptManager(ctrl)
	guard := &fakeGuard{}
	mgr.centralGuard = guard

	const exptID, spaceID, aliveRunID = int64(100), int64(7), int64(200)

	mgr.idgenerator.(*idgenMocks.MockIIDGenerator).EXPECT().GenID(gomock.Any()).Return(int64(201), nil)
	mgr.mutex.(*lockMocks.MockILocker).EXPECT().
		BackoffLockWithValue(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(false, "200", nil)
	mgr.mutex.(*lockMocks.MockILocker).EXPECT().Exists(gomock.Any(), gomock.Any()).Return(false, nil)
	mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).EXPECT().
		Get(gomock.Any(), exptID, aliveRunID).Return(&entity.ExptRunLog{ExptID: exptID, ExptRunID: aliveRunID}, nil)
	mgr.runLogRepo.(*repoMocks.MockIExptRunLogRepo).EXPECT().Save(gomock.Any(), gomock.Any()).Return(nil)
	// 刻意不声明 GetByID / ScanItemRunLogs：真去查就是 unexpected call。

	runID, retried, err := mgr.LogRetryItemsRun(context.Background(), exptID,
		entity.EvaluationModeRetryItems, spaceID, []int64{11}, &entity.Session{UserID: "u"})
	require.NoError(t, err)
	assert.Equal(t, aliveRunID, runID)
	assert.True(t, retried)
	assert.Empty(t, guard.releases(), "旧 run 还活着，释放它的额度就是真超发")
}

// TestReleaseSupersededRunQuota_Guards 边界：无 guard / 查不到实验 / 新旧同号都不得动手。
func TestReleaseSupersededRunQuota_Guards(t *testing.T) {
	t.Run("guard 为 nil", func(t *testing.T) {
		svc := &ExptMangerImpl{}
		assert.NotPanics(t, func() {
			svc.releaseSupersededRunQuota(context.Background(), 100, 7, 201, entity.EvaluationModeFailRetry)
		})
	})

	t.Run("查不到实验就不猜", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		exptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
		exptRepo.EXPECT().GetByID(gomock.Any(), int64(100), int64(7)).Return(nil, assert.AnError)
		guard := &fakeGuard{}
		svc := &ExptMangerImpl{centralGuard: guard, exptRepo: exptRepo}
		svc.releaseSupersededRunQuota(context.Background(), 100, 7, 201, entity.EvaluationModeFailRetry)
		assert.Empty(t, guard.releases())
	})

	t.Run("新旧 run 同号（首跑或幂等重入）", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		exptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
		exptRepo.EXPECT().GetByID(gomock.Any(), int64(100), int64(7)).Return(&entity.Experiment{
			ID: 100, SpaceID: 7, LatestRunID: 201,
			SchedulerScope: testScope, ExptDispatchMode: entity.ExptDispatchModeEnforce,
		}, nil)
		guard := &fakeGuard{}
		svc := &ExptMangerImpl{centralGuard: guard, exptRepo: exptRepo}
		svc.releaseSupersededRunQuota(context.Background(), 100, 7, 201, entity.EvaluationModeFailRetry)
		assert.Empty(t, guard.releases(), "同号说明没有被顶替的旧 run，不该释放")
	})
}
