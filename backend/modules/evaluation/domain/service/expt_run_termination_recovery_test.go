// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/lock"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

// termRecoveryDeps 是终止后资源恢复用例的依赖集合。它与终止单测共用 termDualRepo，
// 唯一的额外依赖是 turnResultRepo（沙箱重放要回查 target record）与 configer（并发上限）。
type termRecoveryDeps struct {
	ctrl        *gomock.Controller
	finish      *repomocks.MockIExptRunFinishRepo
	termination *repomocks.MockIExptRunTerminationRepo
	expts       *repomocks.MockIExperimentRepo
	items       *repomocks.MockIExptItemResultRepo
	turns       *repomocks.MockIExptTurnResultRepo
	locker      *lockmocks.MockILocker
	guard       *componentmocks.MockICentralReservationGuard
	targets     *svcmocks.MockIEvalTargetService
	configer    *componentmocks.MockIConfiger
}

func newTermRecoveryDeps(t *testing.T) *termRecoveryDeps {
	t.Helper()
	ctrl := gomock.NewController(t)
	return &termRecoveryDeps{
		ctrl:        ctrl,
		finish:      repomocks.NewMockIExptRunFinishRepo(ctrl),
		termination: repomocks.NewMockIExptRunTerminationRepo(ctrl),
		expts:       repomocks.NewMockIExperimentRepo(ctrl),
		items:       repomocks.NewMockIExptItemResultRepo(ctrl),
		turns:       repomocks.NewMockIExptTurnResultRepo(ctrl),
		locker:      lockmocks.NewMockILocker(ctrl),
		guard:       componentmocks.NewMockICentralReservationGuard(ctrl),
		targets:     svcmocks.NewMockIEvalTargetService(ctrl),
		configer:    componentmocks.NewMockIConfiger(ctrl),
	}
}

func (d *termRecoveryDeps) manager() *ExptMangerImpl {
	return &ExptMangerImpl{
		runFinishRepo:     &termDualRepo{MockIExptRunFinishRepo: d.finish, MockIExptRunTerminationRepo: d.termination},
		manualRetryLocker: func() lock.ILocker { return d.locker },
		exptRepo:          d.expts,
		itemResultRepo:    d.items,
		turnResultRepo:    d.turns,
		centralGuard:      d.guard,
		evalTargetService: d.targets,
		configer:          d.configer,
	}
}

func (d *termRecoveryDeps) expectLocks() {
	d.locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _ string, _ any, _ any) (bool, context.Context, func(), error) {
			return true, context.Background(), func() {}, nil
		}).AnyTimes()
}

func termEnforceExpt() *entity.Experiment {
	return &entity.Experiment{
		ID: 1, SpaceID: 3, LatestRunID: 2, Status: entity.ExptStatus_Processing,
		ExptType: entity.ExptType_Offline, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: "scope",
	}
}

func termScheduledExpt() *entity.Experiment {
	expt := termEnforceExpt()
	expt.EvalConf = &entity.EvaluationConfiguration{}
	expt.EvalSet = &entity.EvaluationSet{EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 1}}
	return expt
}

func termSandboxExpt() *entity.Experiment {
	expt := termScheduledExpt()
	expt.Target = &entity.EvalTarget{EvalTargetVersion: &entity.EvalTargetVersion{EvalTargetType: entity.EvalTargetTypeSandboxAgent}}
	return expt
}

func termTerminalRow(itemID int64, request string, reservation entity.QuotaReservationState) *entity.ExptItemCurrentRequest {
	now := time.Now()
	return &entity.ExptItemCurrentRequest{
		SpaceID: 3, ExptID: 1, RunID: 2, ItemID: itemID, ItemVersionID: 7,
		RequestID: request, Mode: entity.EvaluationModeFailRetry,
		Status: entity.ItemRunState_Terminal, ResultState: entity.ExptItemResultStateLogged,
		QuotaReservationState: reservation, UpdatedAt: &now,
	}
}

func termScheduleEvent() *entity.ExptScheduleEvent {
	return &entity.ExptScheduleEvent{SpaceID: 3, ExptID: 1, ExptRunID: 2}
}

// R8-A / R8-B2：行级终止已提交 Terminal（运行中终止时投影是 none），Release 失败。
// 修复前错误只回给调用方，账本 reservation 再无任何读取者 → 永久泄漏。
// 修复后必须把"仍欠一次释放"落成持久标记，且标记带 request identity。
func TestTerminationRecovery_RowTerminateRetainsDebtOnReleaseFailure(t *testing.T) {
	d := newTermRecoveryDeps(t)
	m := d.manager()
	d.termination.EXPECT().GetTerminationItemSource(gomock.Any(), int64(3), int64(1), int64(2), int64(9)).
		Return(termUnitSource(9, entity.ItemRunState_Processing), nil)
	d.expectLocks()
	d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(termEnforceExpt(), nil)
	d.termination.EXPECT().TerminateCurrentRun(gomock.Any(), gomock.Any()).
		Return(&entity.ExptRunTerminationResult{Applied: true}, nil)
	d.guard.EXPECT().Release(gomock.Any(), "scope", int64(2), int64(9), "req", gomock.Any()).
		Return(errors.New("ledger unavailable"))
	d.items.EXPECT().SetTerminalQuotaReservationState(gomock.Any(), int64(3), int64(1), int64(2), int64(9), "req", entity.QuotaReservationStateReserved).
		Return(true, nil).Times(1)

	err := m.terminateCurrentItems(context.Background(), 1, 2, 3, []int64{9}, &entity.Session{})
	require.Error(t, err)
}

// Release 成功时不允许打标记：正常终止不应该让 run 多一拍 incomplete。
func TestTerminationRecovery_RowTerminateNoDebtWhenReleaseSucceeds(t *testing.T) {
	d := newTermRecoveryDeps(t)
	m := d.manager()
	d.termination.EXPECT().GetTerminationItemSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), int64(9)).
		Return(termUnitSource(9, entity.ItemRunState_Queueing), nil)
	d.expectLocks()
	d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(termEnforceExpt(), nil)
	d.termination.EXPECT().TerminateCurrentRun(gomock.Any(), gomock.Any()).
		Return(&entity.ExptRunTerminationResult{Applied: true}, nil)
	d.guard.EXPECT().Release(gomock.Any(), "scope", int64(2), int64(9), "req", gomock.Any()).Return(nil)
	// items 没有任何 SetTerminalQuotaReservationState 期望：调用即失败。
	require.NoError(t, m.terminateCurrentItems(context.Background(), 1, 2, 3, []int64{9}, &entity.Session{}))
}

// 沙箱销毁失败以前被 `_ =` 吞掉；现在必须进入错误路径并留下债务，否则没有任何重放。
func TestTerminationRecovery_RowTerminateRetainsDebtOnSandboxFailure(t *testing.T) {
	d := newTermRecoveryDeps(t)
	m := d.manager()
	source := termUnitSource(9, entity.ItemRunState_Terminal)
	source.Turns = map[int64]*entity.ExptTurnResultRunLog{21: {ItemID: 9, TurnID: 21, TargetResultID: 77}}
	d.termination.EXPECT().GetTerminationItemSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), int64(9)).Return(source, nil)
	d.expectLocks()
	d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(termEnforceExpt(), nil)
	d.termination.EXPECT().TerminateCurrentRun(gomock.Any(), gomock.Any()).
		Return(&entity.ExptRunTerminationResult{Applied: true}, nil)
	d.guard.EXPECT().Release(gomock.Any(), "scope", int64(2), int64(9), "req", gomock.Any()).Return(nil)
	d.targets.EXPECT().TerminateAsyncRecordsAndDestroySandbox(gomock.Any(), int64(3), []int64{77}, int32(errno.AsyncEvalTargetTerminatedCode), gomock.Any(), false).
		Return(errors.New("sandbox destroy unavailable"))
	d.items.EXPECT().SetTerminalQuotaReservationState(gomock.Any(), int64(3), int64(1), int64(2), int64(9), "req", entity.QuotaReservationStateReserved).
		Return(true, nil).Times(1)

	// 沙箱保持原有 best-effort 契约（不把终止判失败），但失败必须落成债务标记。
	err := m.terminateCurrentItems(context.Background(), 1, 2, 3, []int64{9}, &entity.Session{})
	require.NoError(t, err)
}

// 批量终止中途失败：失败点及其之后的 item 都还没释放，必须全部打标记，
// 不能只标记第一个（否则后面的行没有任何后续读取者）。
func TestTerminationRecovery_RowTerminateRetainsDebtForUnscheduledTail(t *testing.T) {
	d := newTermRecoveryDeps(t)
	m := d.manager()
	for _, item := range []int64{9, 11} {
		item := item
		d.termination.EXPECT().GetTerminationItemSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), item).
			Return(termUnitSource(item, entity.ItemRunState_Processing), nil)
	}
	d.expectLocks()
	d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(termEnforceExpt(), nil)
	d.termination.EXPECT().TerminateCurrentRun(gomock.Any(), gomock.Any()).
		Return(&entity.ExptRunTerminationResult{Applied: true}, nil)
	d.guard.EXPECT().Release(gomock.Any(), "scope", int64(2), int64(9), "req", gomock.Any()).Return(errors.New("ledger down"))
	d.items.EXPECT().SetTerminalQuotaReservationState(gomock.Any(), int64(3), int64(1), int64(2), int64(9), "req", entity.QuotaReservationStateReserved).Return(true, nil)
	d.items.EXPECT().SetTerminalQuotaReservationState(gomock.Any(), int64(3), int64(1), int64(2), int64(11), "req", entity.QuotaReservationStateReserved).Return(true, nil)

	require.Error(t, m.terminateCurrentItems(context.Background(), 1, 2, 3, []int64{9, 11}, &entity.Session{}))
}

// R8-B1：终态 + reserved 的行在下一拍被对账释放并清投影，于是本拍 incomplete=0，
// 不再永久 nextTick；第二拍没有新的 Release（不重复扣减）。
func TestTerminationRecovery_ScanReleasesTerminalReservedRowThenConverges(t *testing.T) {
	d := newTermRecoveryDeps(t)
	m := d.manager()
	rows := []*entity.ExptItemCurrentRequest{termTerminalRow(9, "req", entity.QuotaReservationStateReserved)}
	d.items.EXPECT().ScanCurrentRequests(gomock.Any(), int64(3), int64(1), int64(2), false).Return(rows, nil).Times(2)
	d.guard.EXPECT().Release(gomock.Any(), "scope", int64(2), int64(9), "req", "terminal item residual reservation").Return(nil).Times(1)
	d.items.EXPECT().SetTerminalQuotaReservationState(gomock.Any(), int64(3), int64(1), int64(2), int64(9), "req", entity.QuotaReservationStateNone).Return(true, nil).Times(1)
	d.configer.EXPECT().GetExptExecConf(gomock.Any(), gomock.Any()).
		Return(&entity.ExptExecConf{ExptItemEvalConf: &entity.ExptItemEvalConf{ConcurNum: 4, MaxItemConcurNum: 4}}).AnyTimes()

	base := newExptBaseExec(m, nil, d.configer, d.items, nil, nil)
	event, expt := termScheduleEvent(), termScheduledExpt()

	toSubmit, incomplete, complete, err := base.ScanEvalItems(context.Background(), event, expt)
	require.NoError(t, err)
	assert.Empty(t, toSubmit)
	assert.Empty(t, incomplete, "released terminal row must not keep consuming a concurrency slot")
	assert.Len(t, complete, 1)

	toSubmit, incomplete, complete, err = base.ScanEvalItems(context.Background(), event, expt)
	require.NoError(t, err)
	assert.Empty(t, toSubmit)
	assert.Empty(t, incomplete)
	assert.Len(t, complete, 1)
}

// 账本仍不可用时保留标记：本拍仍然 incomplete（继续 nextTick 重试），绝不"清理成功"。
func TestTerminationRecovery_ScanKeepsDebtWhenReleaseStillFails(t *testing.T) {
	d := newTermRecoveryDeps(t)
	m := d.manager()
	rows := []*entity.ExptItemCurrentRequest{termTerminalRow(9, "req", entity.QuotaReservationStateReserved)}
	d.items.EXPECT().ScanCurrentRequests(gomock.Any(), int64(3), int64(1), int64(2), false).Return(rows, nil).Times(2)
	d.guard.EXPECT().Release(gomock.Any(), "scope", int64(2), int64(9), "req", gomock.Any()).Return(errors.New("ledger down")).Times(2)
	// 没有 SetTerminalQuotaReservationState 期望：清投影即失败。
	d.configer.EXPECT().GetExptExecConf(gomock.Any(), gomock.Any()).
		Return(&entity.ExptExecConf{ExptItemEvalConf: &entity.ExptItemEvalConf{ConcurNum: 4, MaxItemConcurNum: 4}}).AnyTimes()

	base := newExptBaseExec(m, nil, d.configer, d.items, nil, nil)
	event, expt := termScheduleEvent(), termScheduledExpt()

	for tick := 0; tick < 2; tick++ {
		_, incomplete, complete, err := base.ScanEvalItems(context.Background(), event, expt)
		require.NoError(t, err)
		assert.Len(t, incomplete, 1)
		assert.Len(t, complete, 1)
	}
}

// 投影 CAS 不命中（行已被新 attempt 推进 / 已被别人清）时不能当作成功清掉本拍的内存投影，
// 否则这一拍会把仍在欠释放的行当 complete 归档出去。
func TestTerminationRecovery_ScanKeepsIncompleteWhenProjectionCASMisses(t *testing.T) {
	d := newTermRecoveryDeps(t)
	m := d.manager()
	rows := []*entity.ExptItemCurrentRequest{termTerminalRow(9, "req", entity.QuotaReservationStateReserved)}
	d.items.EXPECT().ScanCurrentRequests(gomock.Any(), int64(3), int64(1), int64(2), false).Return(rows, nil).Times(1)
	d.guard.EXPECT().Release(gomock.Any(), "scope", int64(2), int64(9), "req", gomock.Any()).Return(nil)
	d.items.EXPECT().SetTerminalQuotaReservationState(gomock.Any(), int64(3), int64(1), int64(2), int64(9), "req", entity.QuotaReservationStateNone).Return(false, nil)
	d.configer.EXPECT().GetExptExecConf(gomock.Any(), gomock.Any()).
		Return(&entity.ExptExecConf{ExptItemEvalConf: &entity.ExptItemEvalConf{ConcurNum: 4, MaxItemConcurNum: 4}}).AnyTimes()

	base := newExptBaseExec(m, nil, d.configer, d.items, nil, nil)
	_, incomplete, _, err := base.ScanEvalItems(context.Background(), termScheduleEvent(), termScheduledExpt())
	require.NoError(t, err)
	assert.Len(t, incomplete, 1)
}

// 沙箱实验：清投影之前必须真的重放一次销毁（waitForDestroy=true 才能拿到错误）。
func TestTerminationRecovery_ReplaysSandboxDestroyBeforeClearing(t *testing.T) {
	d := newTermRecoveryDeps(t)
	m := d.manager()
	rows := []*entity.ExptItemCurrentRequest{termTerminalRow(9, "req", entity.QuotaReservationStateReserved)}
	d.items.EXPECT().ScanCurrentRequests(gomock.Any(), int64(3), int64(1), int64(2), false).Return(rows, nil)
	d.guard.EXPECT().Release(gomock.Any(), "scope", int64(2), int64(9), "req", gomock.Any()).Return(nil)
	d.turns.EXPECT().MGetItemTurnRunLogs(gomock.Any(), int64(1), int64(2), []int64{9}, int64(3)).
		Return([]*entity.ExptTurnResultRunLog{{ItemID: 9, TurnID: 21, TargetResultID: 77}, {ItemID: 9, TurnID: 22, TargetResultID: 77}}, nil)
	d.targets.EXPECT().TerminateAsyncRecordsAndDestroySandbox(gomock.Any(), int64(3), []int64{77}, int32(errno.AsyncEvalTargetTerminatedCode), gomock.Any(), false, true).Return(nil)
	d.items.EXPECT().SetTerminalQuotaReservationState(gomock.Any(), int64(3), int64(1), int64(2), int64(9), "req", entity.QuotaReservationStateNone).Return(true, nil)
	d.configer.EXPECT().GetExptExecConf(gomock.Any(), gomock.Any()).
		Return(&entity.ExptExecConf{ExptItemEvalConf: &entity.ExptItemEvalConf{ConcurNum: 4, MaxItemConcurNum: 4}}).AnyTimes()

	base := newExptBaseExec(m, nil, d.configer, d.items, nil, nil)
	_, incomplete, complete, err := base.ScanEvalItems(context.Background(), termScheduleEvent(), termSandboxExpt())
	require.NoError(t, err)
	assert.Empty(t, incomplete)
	assert.Len(t, complete, 1)
}

// 沙箱重放失败 → 不清标记，下一拍重来。这是"真实后续重放路径"的核心断言。
func TestTerminationRecovery_KeepsDebtWhenSandboxReplayFails(t *testing.T) {
	d := newTermRecoveryDeps(t)
	m := d.manager()
	rows := []*entity.ExptItemCurrentRequest{termTerminalRow(9, "req", entity.QuotaReservationStateReserved)}
	d.items.EXPECT().ScanCurrentRequests(gomock.Any(), int64(3), int64(1), int64(2), false).Return(rows, nil).Times(2)
	d.guard.EXPECT().Release(gomock.Any(), "scope", int64(2), int64(9), "req", gomock.Any()).Return(nil).Times(2)
	d.turns.EXPECT().MGetItemTurnRunLogs(gomock.Any(), int64(1), int64(2), []int64{9}, int64(3)).
		Return([]*entity.ExptTurnResultRunLog{{ItemID: 9, TurnID: 21, TargetResultID: 77}}, nil).Times(2)
	d.targets.EXPECT().TerminateAsyncRecordsAndDestroySandbox(gomock.Any(), int64(3), []int64{77}, int32(errno.AsyncEvalTargetTerminatedCode), gomock.Any(), false, true).
		Return(errors.New("destroy failed")).Times(2)
	d.configer.EXPECT().GetExptExecConf(gomock.Any(), gomock.Any()).
		Return(&entity.ExptExecConf{ExptItemEvalConf: &entity.ExptItemEvalConf{ConcurNum: 4, MaxItemConcurNum: 4}}).AnyTimes()

	base := newExptBaseExec(m, nil, d.configer, d.items, nil, nil)
	for tick := 0; tick < 2; tick++ {
		_, incomplete, _, err := base.ScanEvalItems(context.Background(), termScheduleEvent(), termSandboxExpt())
		require.NoError(t, err)
		assert.Len(t, incomplete, 1)
	}
}

// legacy 实验从不预占额度：对账必须完全 no-op，不能凭空发 Release。
func TestTerminationRecovery_ReconcileSkipsLegacyDispatch(t *testing.T) {
	d := newTermRecoveryDeps(t)
	m := d.manager()
	expt := termScheduledExpt()
	expt.ExptDispatchMode = entity.ExptDispatchModeLegacy
	expt.SchedulerScope = ""
	rows := []*entity.ExptItemCurrentRequest{termTerminalRow(9, "", entity.QuotaReservationStateReserved)}
	// guard / items 都没有期望：任何调用都会让 gomock 失败。
	assert.Empty(t, m.ReconcileTerminalResourceDebt(context.Background(), termScheduleEvent(), expt, rows))
}

// 审计：现有 sweepTerminatedSandboxItems 只巡检 Processing 的 item，
// 终态行永远进不去 —— 这正是本 hotfix 需要补调度拍重放的原因。
func TestTerminationRecovery_SweepDoesNotCoverTerminalItems(t *testing.T) {
	d := newTermRecoveryDeps(t)
	scheduler := &ExptSchedulerImpl{evalTargetService: d.targets}
	items := []*entity.ExptEvalItem{{ItemID: 9, State: entity.ItemRunState_Terminal}}
	alives, terminated, err := scheduler.sweepTerminatedSandboxItems(context.Background(), termScheduleEvent(), items, termSandboxExpt())
	require.NoError(t, err)
	assert.Len(t, alives, 1)
	assert.Empty(t, terminated)
}

// legacy 实验没有账本 reservation：沙箱释放失败只能告警，绝不能写 reserved 标记 ——
// 标记一旦写入，legacy 的对账入口直接 no-op，该行会永久 incomplete，run 永不收口。
func TestTerminationRecovery_LegacyTerminateNeverRetainsMarker(t *testing.T) {
	d := newTermRecoveryDeps(t)
	m := d.manager()
	source := termUnitSource(9, entity.ItemRunState_Terminal)
	source.Turns = map[int64]*entity.ExptTurnResultRunLog{21: {ItemID: 9, TurnID: 21, TargetResultID: 77}}
	d.termination.EXPECT().GetTerminationItemSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), int64(9)).Return(source, nil)
	d.expectLocks()
	legacy := termEnforceExpt()
	legacy.ExptDispatchMode = entity.ExptDispatchModeLegacy
	legacy.SchedulerScope = ""
	d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(legacy, nil)
	d.termination.EXPECT().TerminateCurrentRun(gomock.Any(), gomock.Any()).
		Return(&entity.ExptRunTerminationResult{Applied: true}, nil)
	// legacy 不预占额度，guard 不应被调用；items 也没有写标记的期望。
	d.targets.EXPECT().TerminateAsyncRecordsAndDestroySandbox(gomock.Any(), int64(3), []int64{77}, int32(errno.AsyncEvalTargetTerminatedCode), gomock.Any(), false).
		Return(errors.New("sandbox destroy unavailable"))

	require.NoError(t, m.terminateCurrentItems(context.Background(), 1, 2, 3, []int64{9}, &entity.Session{}))
}

// enforce 但缺 scheduler_scope：账本无处可释放，对账入口也直接 no-op。
// 此时绝不能打 reserved 标记，否则行会永久 incomplete（比原问题更糟）。
func TestTerminationRecovery_EnforceWithoutScopeNeverRetainsMarker(t *testing.T) {
	d := newTermRecoveryDeps(t)
	m := d.manager()
	d.termination.EXPECT().GetTerminationItemSource(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), int64(9)).
		Return(termUnitSource(9, entity.ItemRunState_Processing), nil)
	d.expectLocks()
	scopeless := termEnforceExpt()
	scopeless.SchedulerScope = ""
	d.expts.EXPECT().GetByID(gomock.Any(), int64(1), int64(3)).Return(scopeless, nil)
	d.termination.EXPECT().TerminateCurrentRun(gomock.Any(), gomock.Any()).
		Return(&entity.ExptRunTerminationResult{Applied: true}, nil)
	// items 无写标记期望；guard 也不会被调用（scope 缺失先失败）。
	require.Error(t, m.terminateCurrentItems(context.Background(), 1, 2, 3, []int64{9}, &entity.Session{}))
}
