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
}

func (g *retryQuotaGuard) ReleaseTerminalReservation(_ context.Context, scope string, runID, itemID int64, token string) (component.TerminalReservationReleaseResult, error) {
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
}

// newRetryQuotaHarness 复用 admission 的 fake repo，但用真实记录的 fake guard，
// 使「释放发生在提交之前还是之后」可以被直接断言。
func newRetryQuotaHarness(t *testing.T, calls int) *retryQuotaHarness {
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
	expt := &entity.Experiment{ID: 7, SpaceID: 9, LatestRunID: 8, Status: entity.ExptStatus_Processing, ExptType: entity.ExptType_Offline, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: "test-scope", EvalSetSourceType: entity.ExptEvalSetSourceType_MultiSetConfig, EvalConf: &entity.EvaluationConfiguration{ItemRetryNum: gptr.Of(3), ItemConcurNum: gptr.Of(4)}}
	run := &entity.ExptRunLog{ID: 8, SpaceID: 9, ExptID: 7, ExptRunID: 8, Status: int64(entity.ExptStatus_Processing), Mode: int32(entity.EvaluationModeSubmit), CreatedBy: "original"}
	expts.EXPECT().GetByID(gomock.Any(), int64(7), int64(9)).Return(expt, nil).Times(2 * calls)
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
		return true, ctx, cancel, nil
	}).Times(calls)
	runs.EXPECT().Get(gomock.Any(), int64(7), int64(8)).Return(run, nil).Times(calls)
	mutex.EXPECT().LockWithRenew(gomock.Any(), "expt_item_eval_run_lock:7:11", 5*time.Second, 5*time.Minute).DoAndReturn(func(parent context.Context, _ string, _, _ time.Duration) (bool, context.Context, func(), error) {
		ctx, cancel := context.WithCancel(parent)
		t.Cleanup(cancel)
		return true, ctx, cancel, nil
	}).Times(calls)
	manager := &ExptMangerImpl{manualRetryRepo: port, exptRepo: expts, itemResultRepo: items, turnResultRepo: turns, itemRefRepo: refs, runLogRepo: runs, mutex: mutex, centralGuard: guard, publisher: publisher, currentRunWaker: waker, manualRetryLocker: func() lock.ILocker { return mutex }}
	return &retryQuotaHarness{manager: manager, port: port, guard: guard, waker: waker}
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
	got, err := h.manager.RetryInCurrentRun(context.Background(), admissionRequest())
	require.NoError(t, err)
	require.EqualValues(t, 1, got.AcceptedCount)
	require.Equal(t, []retryQuotaRelease{{scope: "test-scope", runID: 6, itemID: 11, token: "prior-request"}}, h.guard.releases)
	require.Equal(t, 1, countRetryCalls(h.port.calls, "preflight"))
	require.Equal(t, 1, countRetryCalls(h.port.calls, "commit"))
}

// 改排失败 → 预占仍在账本里（未被释放）；随后重试必须能在提交成功后释放且只释放一次。
func TestManualRetryRequeueFailureKeepsReservationForRetry(t *testing.T) {
	h := newRetryQuotaHarness(t, 2)
	h.port.commitError = errors.New("requeue transaction failed")
	_, err := h.manager.RetryInCurrentRun(context.Background(), admissionRequest())
	require.ErrorIs(t, err, h.port.commitError)
	require.Empty(t, h.guard.releases, "ledger entry disappeared although the requeue rolled back")
	require.Empty(t, h.waker.calls)

	h.port.commitError = nil
	got, err := h.manager.RetryInCurrentRun(context.Background(), admissionRequest())
	require.NoError(t, err)
	require.EqualValues(t, 1, got.AcceptedCount)
	require.Equal(t, []retryQuotaRelease{{scope: "test-scope", runID: 6, itemID: 11, token: "prior-request"}}, h.guard.releases)
	require.Equal(t, 2, countRetryCalls(h.port.calls, "commit"))
}

// 改排提交成功但释放失败：必须有界重试，最终成功后账本恢复一致。
func TestManualRetryReleaseRetriedAfterCommit(t *testing.T) {
	h := newRetryQuotaHarness(t, 1)
	h.guard.errs = []error{errors.New("redis unavailable"), nil}
	got, err := h.manager.RetryInCurrentRun(context.Background(), admissionRequest())
	require.NoError(t, err)
	require.EqualValues(t, 1, got.AcceptedCount)
	require.Len(t, h.guard.releases, 2)
	for _, release := range h.guard.releases {
		require.Equal(t, "prior-request", release.token)
	}
	require.Equal(t, 1, countRetryCalls(h.port.calls, "commit"))
	require.Equal(t, 1, h.waker.calls)
}

// 释放持续失败时：重试次数有界、只改排一次、永远不释放新 token（否则新一次执行会失去额度保护）。
func TestManualRetryReleaseFailureNeverReleasesNewToken(t *testing.T) {
	h := newRetryQuotaHarness(t, 1)
	h.guard.errs = []error{errors.New("redis unavailable"), errors.New("redis unavailable"), errors.New("redis unavailable")}
	got, err := h.manager.RetryInCurrentRun(context.Background(), admissionRequest())
	require.NoError(t, err)
	require.EqualValues(t, 1, got.AcceptedCount)
	require.Len(t, h.guard.releases, retryTerminalReservationReleaseAttempts)
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
	got, err := h.manager.RetryInCurrentRun(context.Background(), admissionRequest())
	require.NoError(t, err)
	require.EqualValues(t, 1, got.AcceptedCount)
	require.Equal(t, []retryQuotaRelease{{scope: "test-scope", runID: 6, itemID: 11, token: "prior-request"}}, h.guard.releases)
	require.Equal(t, 1, countRetryCalls(h.port.calls, "commit"))
}

// enforce 却没有账本时必须在任何写之前失败（fail-closed），不能改排一个无法释放预占的 item。
func TestManualRetryEnforceWithoutLedgerWritesNothing(t *testing.T) {
	h := newRetryQuotaHarness(t, 1)
	h.manager.centralGuard = nil
	_, err := h.manager.RetryInCurrentRun(context.Background(), admissionRequest())
	require.ErrorContains(t, err, "terminal reservation release unavailable")
	require.Equal(t, 0, countRetryCalls(h.port.calls, "commit"))
}
