// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"

	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// retryQuotaRelease 记录一次终态预占释放的完整身份，用来证明释放的永远是
// 上一轮的 token，而不会误释放重新排队后新一次执行的 token。
type retryQuotaRelease struct {
	scope  string
	runID  int64
	itemID int64
	token  string
}

type retryQuotaGuard struct {
	releases []retryQuotaRelease
	results  []component.TerminalReservationReleaseResult
	errs     []error
	// onRelease 在每次释放前执行，用于断言原调度拍释放前仍持有 run 锁。
	onRelease func()
}

func (g *retryQuotaGuard) ReleaseTerminalReservation(_ context.Context, scope string, runID, itemID int64, token string) (component.TerminalReservationReleaseResult, error) {
	if g.onRelease != nil {
		g.onRelease()
	}
	attempt := len(g.releases)
	g.releases = append(g.releases, retryQuotaRelease{scope: scope, runID: runID, itemID: itemID, token: token})
	if attempt < len(g.errs) && g.errs[attempt] != nil {
		return 0, g.errs[attempt]
	}
	if attempt < len(g.results) {
		return g.results[attempt], nil
	}
	return component.TerminalReservationReleased, nil
}

func (g *retryQuotaGuard) ConfirmRunning(context.Context, string, int64, int64, string) (bool, error) {
	panic("unexpected ConfirmRunning")
}

func (g *retryQuotaGuard) Release(context.Context, string, int64, int64, string, string) error {
	panic("unexpected Release")
}

func (g *retryQuotaGuard) RepriceRunConsumption(context.Context, string, int64, int64, *entity.ExpectedQuotaConsumption) error {
	panic("unexpected RepriceRunConsumption")
}

type retryQuotaHarness struct {
	manager *ExptMangerImpl
	port    *admissionMemoryRepo
	guard   *retryQuotaGuard
	waker   *admissionWaker
	// runLockReleased 在 mock 的 run 锁 cleanup 执行后置真。释放是已提交事务的补偿，
	// 不应该继续占着 run 锁（5 分钟上限）做重试。
	runLockReleased bool
}

// newRetryQuotaHarness 复用 admission 的 fake repo，但用真实记录的 fake guard，
// 使「释放发生在提交之前还是之后」可以被直接断言。
func newRetryQuotaHarness(t *testing.T, calls int, consumerCalls ...int) *retryQuotaHarness {
	consumerCount := calls
	if len(consumerCalls) > 0 {
		consumerCount = consumerCalls[0]
	}
	t.Helper()
	ctrl := gomock.NewController(t)
	expts := repomocks.NewMockIExperimentRepo(ctrl)
	items := repomocks.NewMockIExptItemResultRepo(ctrl)
	turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
	refs := repomocks.NewMockIExptItemRefRepo(ctrl)
	runs := repomocks.NewMockIExptRunLogRepo(ctrl)
	mutex := lockmocks.NewMockILocker(ctrl)
	publisher := eventmocks.NewMockExptEventPublisher(ctrl)
	publisher.EXPECT().PublishExptScheduleEvent(gomock.Any(), gomock.Any(), gomock.Any()).Times(0)
	guard := &retryQuotaGuard{}
	waker := &admissionWaker{}
	port := &admissionMemoryRepo{}
	h := &retryQuotaHarness{port: port, guard: guard, waker: waker}
	expt := &entity.Experiment{ID: 7, SpaceID: 9, LatestRunID: 8, Status: entity.ExptStatus_Processing, ExptType: entity.ExptType_Offline, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: "test-scope", EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig, EvalConf: &entity.EvaluationConfiguration{ItemRetryNum: gptr.Of(3), ItemConcurNum: gptr.Of(4)}}
	run := &entity.ExptRunLog{ID: 8, SpaceID: 9, ExptID: 7, ExptRunID: 8, Status: int64(entity.ExptStatus_Processing), Mode: int32(entity.EvaluationModeSubmit), CreatedBy: "original"}
	expts.EXPECT().GetByID(gomock.Any(), int64(7), int64(9)).Return(expt, nil).Times(calls + consumerCount)
	oldRun := int64(6)
	source := &entity.ExptItemResult{ID: 101, SpaceID: 9, ExptID: 7, ExptRunID: oldRun, ItemID: 11, ItemVersionID: 91, Status: entity.ItemRunState_Fail, Ext: map[string]string{"other": "keep", entity.ManualRetryExtKey: `{"run_id":6,"request_id":"prior-request","mode":5,"operator":"old","accepted_at_ms":1}`}}
	items.EXPECT().BatchGet(gomock.Any(), int64(9), int64(7), []int64{11}).Return([]*entity.ExptItemResult{source}, nil).Times(calls)
	items.EXPECT().MGetItemRunLog(gomock.Any(), int64(7), int64(8), []int64{11}, int64(9)).Return(nil, nil).Times(calls)
	refs.EXPECT().GetByExptIDAndItemID(gomock.Any(), int64(9), int64(7), int64(11)).Return(&entity.ExptItemRef{SpaceID: 9, ExptID: 7, ItemID: 11, ItemVersionID: 91}, nil).Times(calls)
	turns.EXPECT().GetItemTurnResults(gomock.Any(), int64(7), int64(11), int64(9)).Return([]*entity.ExptTurnResult{{ID: 301, SpaceID: 9, ExptID: 7, ExptRunID: oldRun, ItemID: 11, ItemVersionID: 91, TurnID: 21, Status: 2}}, nil).Times(calls)
	turns.EXPECT().GetItemTurnRunLogs(gomock.Any(), int64(7), int64(8), int64(11), int64(9)).Return(nil, nil).Times(calls)
	turns.EXPECT().BatchGetTurnEvaluatorResultRef(gomock.Any(), int64(9), []int64{301}).Return(nil, nil).Times(calls)
	mutex.EXPECT().LockWithRenew(gomock.Any(), "expt_run_exec_lock:7:8", 5*time.Second, 5*time.Minute).DoAndReturn(func(context.Context, string, time.Duration, time.Duration) (bool, context.Context, func(), error) {
		ctx, cancel := context.WithCancel(context.Background())
		t.Cleanup(cancel)
		return true, ctx, func() {
			h.runLockReleased = true
			cancel()
		}, nil
	}).Times(consumerCount)
	runs.EXPECT().Get(gomock.Any(), int64(7), int64(8)).Return(run, nil).Times(consumerCount)
	mutex.EXPECT().LockWithRenew(gomock.Any(), "expt_item_eval_run_lock:7:11", 5*time.Second, 5*time.Minute).DoAndReturn(func(parent context.Context, _ string, _, _ time.Duration) (bool, context.Context, func(), error) {
		ctx, cancel := context.WithCancel(parent)
		t.Cleanup(cancel)
		return true, ctx, cancel, nil
	}).Times(consumerCount)
	manager := &ExptMangerImpl{manualRetryRepo: port, exptRepo: expts, itemResultRepo: items, turnResultRepo: turns, itemRefRepo: refs, runLogRepo: runs, mutex: mutex, centralGuard: guard, publisher: publisher, currentRunWaker: waker, manualRetryLocker: func() lock.ILocker { return mutex }}
	h.manager = manager
	return h
}

func countRetryCalls(calls []string, want string) int {
	n := 0
	for _, call := range calls {
		if call == want {
			n++
		}
	}
	return n
}

// 释放必须发生在改排事务提交之后：事务回滚时账本不能被提前退还。
func TestManualRetryReleasesReservationOnlyAfterRequeueCommit(t *testing.T) {
	h := newRetryQuotaHarness(t, 1)
	h.port.beforeCommit = func() {
		require.Empty(t, h.guard.releases, "reservation released before requeue commit")
	}
	got, err := retryThroughOriginalTickForTest(t, h.manager, context.Background(), admissionRequest())
	require.NoError(t, err)
	require.EqualValues(t, 1, got.AcceptedCount)
	require.Equal(t, []retryQuotaRelease{{scope: "test-scope", runID: 6, itemID: 11, token: "prior-request"}}, h.guard.releases)
	require.Equal(t, 2, countRetryCalls(h.port.calls, "preflight"))
	require.Equal(t, 1, countRetryCalls(h.port.calls, "commit"))
}

// 改排失败 → 预占仍在账本里（未被释放）；随后重试必须能在提交成功后释放且只释放一次。
func TestManualRetryRequeueFailureKeepsReservationForRetry(t *testing.T) {
	h := newRetryQuotaHarness(t, 2)
	h.port.commitError = errors.New("requeue transaction failed")
	_, err := retryThroughOriginalTickForTest(t, h.manager, context.Background(), admissionRequest())
	require.ErrorIs(t, err, h.port.commitError)
	require.Empty(t, h.guard.releases, "ledger entry disappeared although the requeue rolled back")
	require.Empty(t, h.waker.calls)

	h.port.commitError = nil
	got, err := retryThroughOriginalTickForTest(t, h.manager, context.Background(), admissionRequest())
	require.NoError(t, err)
	require.EqualValues(t, 1, got.AcceptedCount)
	require.Equal(t, []retryQuotaRelease{{scope: "test-scope", runID: 6, itemID: 11, token: "prior-request"}}, h.guard.releases)
	require.Equal(t, 2, countRetryCalls(h.port.calls, "commit"))
}

// 改排提交成功但释放报错：释放不重试（重试没有持久 intent 也无法恢复），但也不能
// 把已提交的改排变成调用方失败。
func TestManualRetryReleaseErrorDoesNotFailCommittedRequeue(t *testing.T) {
	h := newRetryQuotaHarness(t, 1)
	h.guard.errs = []error{errors.New("redis unavailable")}
	got, err := retryThroughOriginalTickForTest(t, h.manager, context.Background(), admissionRequest())
	require.NoError(t, err)
	require.EqualValues(t, 1, got.AcceptedCount)
	require.Len(t, h.guard.releases, 1)
	require.Equal(t, "prior-request", h.guard.releases[0].token)
	require.Equal(t, 1, countRetryCalls(h.port.calls, "commit"))
	require.Equal(t, 1, h.waker.calls)
}

// 释放持续失败时：只尝试一次、只改排一次、永远不释放新 token（否则新一次执行会失去额度保护）。
func TestManualRetryReleaseFailureNeverReleasesNewToken(t *testing.T) {
	h := newRetryQuotaHarness(t, 1)
	h.guard.errs = []error{errors.New("redis unavailable")}
	got, err := retryThroughOriginalTickForTest(t, h.manager, context.Background(), admissionRequest())
	require.NoError(t, err)
	require.EqualValues(t, 1, got.AcceptedCount)
	require.Len(t, h.guard.releases, 1)
	require.Equal(t, 1, countRetryCalls(h.port.calls, "commit"))
	for _, release := range h.guard.releases {
		require.Equal(t, "prior-request", release.token)
		require.NotEqual(t, h.port.operationID, release.token)
	}
}

// mismatch 说明账本里那条预占已不属于本次改排的来源：不重试、不误删，改排本身仍然成功。
func TestManualRetryMismatchedReservationCompletesOnce(t *testing.T) {
	h := newRetryQuotaHarness(t, 1)
	h.guard.results = []component.TerminalReservationReleaseResult{component.TerminalReservationMismatch}
	got, err := retryThroughOriginalTickForTest(t, h.manager, context.Background(), admissionRequest())
	require.NoError(t, err)
	require.EqualValues(t, 1, got.AcceptedCount)
	require.Equal(t, []retryQuotaRelease{{scope: "test-scope", runID: 6, itemID: 11, token: "prior-request"}}, h.guard.releases)
	require.Equal(t, 1, countRetryCalls(h.port.calls, "commit"))
}

// enforce 却没有账本时必须在任何写之前失败（fail-closed），不能改排一个无法释放预占的 item。
func TestManualRetryEnforceWithoutLedgerWritesNothing(t *testing.T) {
	h := newRetryQuotaHarness(t, 1)
	h.manager.centralGuard = nil
	_, err := retryThroughOriginalTickForTest(t, h.manager, context.Background(), admissionRequest())
	require.ErrorContains(t, err, "terminal reservation release unavailable")
	require.Equal(t, 0, countRetryCalls(h.port.calls, "commit"))
}

// 释放是改排提交后的补偿，不能继续持有 run 锁：账本故障时的同步释放耗时不能吃掉
// manualRetryLockMaxHold(5min)，否则锁到期后同 run 的并发改排就可能重叠。
func TestManualRetryReleaseRetainsOriginalTickLock(t *testing.T) {
	h := newRetryQuotaHarness(t, 1)
	h.guard.onRelease = func() {
		require.False(t, h.runLockReleased, "the original tick must remain responsible for its successor")
	}
	got, err := retryThroughOriginalTickForTest(t, h.manager, context.Background(), admissionRequest())
	require.NoError(t, err)
	require.EqualValues(t, 1, got.AcceptedCount)
	require.Len(t, h.guard.releases, 1)
}

// 反例：提交成功但释放持续失败后，同一用户请求无法再触发释放（改排后 item 已不是终态，
// 预检会以 source_changed 拒收），而旧 run 的预占不在任何调度拍/对账的扫描范围内。
// 这个用例记录的是当前方案的残余：调用方没有可重放路径，只能依赖人工全量重建账本。
func TestManualRetryPostCommitReleaseFailureHasNoCallerReplay(t *testing.T) {
	h := newRetryQuotaHarness(t, 2, 1)
	h.guard.errs = []error{errors.New("redis unavailable")}
	got, err := retryThroughOriginalTickForTest(t, h.manager, context.Background(), admissionRequest())
	require.NoError(t, err)
	require.EqualValues(t, 1, got.AcceptedCount)
	require.Len(t, h.guard.releases, 1)

	// 改排已落库，再次请求时 repository 会把它当成 queueing/source_changed 拒收。
	h.port.preflightRejected = true
	replay, err := retryThroughOriginalTickForTest(t, h.manager, context.Background(), admissionRequest())
	require.NoError(t, err)
	require.EqualValues(t, 0, replay.AcceptedCount)
	require.Len(t, h.guard.releases, 1,
		"a rejected replay must not (and cannot) retry the stale reservation release")
}
