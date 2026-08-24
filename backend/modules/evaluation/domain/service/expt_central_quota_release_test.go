// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	componentMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	eventmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/ctxcache"
)

// releaseCall 记录一次 Release 调用，供断言"释放了几次、释放的是谁"。
type releaseCall struct {
	Scope  string
	RunID  int64
	ItemID int64
	Reason string
}

// fakeGuard 手写的额度闸替身。
//
// 不用 mockgen：这些测试关心的是「调用序列」与「调用次数」，手写替身能直接把序列存下来断言，
// 而 gomock 表达"恰好一次、参数是这个"需要更多样板，且 ICentralReservationGuard 目前
// 没有生成 mock。
type fakeGuard struct {
	mu sync.Mutex

	confirmResult bool
	confirmErr    error
	confirmCalls  int

	releaseErr   error
	releaseCalls []releaseCall
}

func (f *fakeGuard) ConfirmRunning(ctx context.Context, schedulerScope string, exptRunID, itemID int64) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.confirmCalls++
	return f.confirmResult, f.confirmErr
}

func (f *fakeGuard) Release(ctx context.Context, schedulerScope string, exptRunID, itemID int64, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.releaseCalls = append(f.releaseCalls, releaseCall{Scope: schedulerScope, RunID: exptRunID, ItemID: itemID, Reason: reason})
	return f.releaseErr
}

func (f *fakeGuard) releases() []releaseCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]releaseCall, len(f.releaseCalls))
	copy(out, f.releaseCalls)
	return out
}

const testScope = "fornax_cn_ppe_fornax_evalx"

// TestReleaseCentralQuotaOutsideGate_ReleasesWhenQuotaHeld 覆盖本次修复的核心场景：
// 前置阶段（BuildExptRecordEvalCtx 等）失败且不可重试时，额度必须被归还。
//
// 回归的是一个**必然发生**的泄漏：额度闸在中间件链内层，按 run log 状态判定时 item 还是
// Processing 因而正确地保留了 reservation；把 item 落成 Fail 的兜底在更外层，那时额度闸
// 已经返回。修复前这条路径上的额度永久留在账本里（无 TTL 清理、也还没有对账）。
func TestReleaseCentralQuotaOutsideGate_ReleasesWhenQuotaHeld(t *testing.T) {
	guard := &fakeGuard{}
	svc := &ExptItemEventEvalServiceImpl{centralGuard: guard}

	event := &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4}

	ctx := ctxcache.Init(context.Background())
	event.WithCtxCentralQuotaHeld(ctx, testScope)

	svc.releaseCentralQuotaOutsideGate(ctx, event, errors.New("build eval ctx fail"), "item unretriable pre-exec failure")

	calls := guard.releases()
	assert.Len(t, calls, 1, "持有预占且已落终态时必须归还额度，否则永久泄漏")
	assert.Equal(t, testScope, calls[0].Scope)
	assert.Equal(t, int64(2), calls[0].RunID)
	assert.Equal(t, int64(4), calls[0].ItemID)
	assert.Contains(t, calls[0].Reason, "item unretriable pre-exec failure")
	assert.Contains(t, calls[0].Reason, "build eval ctx fail", "reason 要带上原始错误，否则排查时看不出为什么释放")
}

// TestReleaseCentralQuotaOutsideGate_SkipsWithoutCredential legacy 实验与"取得执行权之前
// 就失败的消息"都不该触发释放 —— 前者从不预占，后者预占不属于本次处理。
// 多释放的后果不是浪费一次 Redis 往返，而是可能归还别人的额度。
func TestReleaseCentralQuotaOutsideGate_SkipsWithoutCredential(t *testing.T) {
	guard := &fakeGuard{}
	svc := &ExptItemEventEvalServiceImpl{centralGuard: guard}
	event := &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4}

	// ctx 里没有凭据（legacy 实验走的就是这条）
	ctx := ctxcache.Init(context.Background())
	svc.releaseCentralQuotaOutsideGate(ctx, event, errors.New("boom"), "reason")
	assert.Empty(t, guard.releases(), "无凭据不得释放")

	// ctx 压根没 Init（防御：ctxcache.Get 在无缓存 ctx 上返回 false）
	svc.releaseCentralQuotaOutsideGate(context.Background(), event, errors.New("boom"), "reason")
	assert.Empty(t, guard.releases(), "ctx 无缓存时不得释放")
}

// TestReleaseCentralQuotaOutsideGate_NoopOnNilInputs 无错误 / 无 guard / 无 event 时静默返回。
// 成功路径由额度闸内的 releaseQuotaIfItemTerminal 负责，本函数只补终态失败那两条。
func TestReleaseCentralQuotaOutsideGate_NoopOnNilInputs(t *testing.T) {
	guard := &fakeGuard{}
	event := &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, EvalSetItemID: 4}
	ctx := ctxcache.Init(context.Background())
	event.WithCtxCentralQuotaHeld(ctx, testScope)

	svcWithGuard := &ExptItemEventEvalServiceImpl{centralGuard: guard}
	svcWithGuard.releaseCentralQuotaOutsideGate(ctx, event, nil, "reason")
	assert.Empty(t, guard.releases(), "evalErr 为 nil 说明没失败，释放该由额度闸内的终态判定负责")

	svcWithGuard.releaseCentralQuotaOutsideGate(ctx, nil, errors.New("boom"), "reason")
	assert.Empty(t, guard.releases())

	svcNoGuard := &ExptItemEventEvalServiceImpl{}
	svcNoGuard.releaseCentralQuotaOutsideGate(ctx, event, errors.New("boom"), "reason") // 不 panic 即通过
}

// TestReleaseCentralQuotaOutsideGate_SwallowsReleaseError 释放失败只告警。
// 让"额度归还失败"阻断终态收口，会把额度泄漏升级成实验永不收敛 —— 后者严重得多。
func TestReleaseCentralQuotaOutsideGate_SwallowsReleaseError(t *testing.T) {
	guard := &fakeGuard{releaseErr: errors.New("redis down")}
	svc := &ExptItemEventEvalServiceImpl{centralGuard: guard}
	event := &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, EvalSetItemID: 4}
	ctx := ctxcache.Init(context.Background())
	event.WithCtxCentralQuotaHeld(ctx, testScope)

	svc.releaseCentralQuotaOutsideGate(ctx, event, errors.New("boom"), "reason") // 不 panic、不冒泡
	assert.Len(t, guard.releases(), 1)
}

// TestHandleEventErr_ReleasesQuotaOnUnretriableErr 端到端验证 HandleEventErr 这一层：
// 不可重试失败时，落 Fail 与归还额度**两件事都要做**。
//
// 这是修复前真正漏掉的那条链路，因此断言的是 HandleEventErr 而不只是辅助函数。
func TestHandleEventErr_ReleasesQuotaOnUnretriableErr(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfiger := componentMocks.NewMockIConfiger(ctrl)
	mockMetric := metricsmocks.NewMockExptMetric(ctrl)
	mockItemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	mockTurnResultRepo := repoMocks.NewMockIExptTurnResultRepo(ctrl)

	// RetryTimes 已达上限 → 不重试 → 走兜底落 Fail + 归还额度
	mockConfiger.EXPECT().GetErrRetryConf(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&entity.RetryConf{RetryTimes: 3, RetryIntervalSecond: 60, IsInDebt: false})
	mockMetric.EXPECT().EmitItemExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
	mockItemResultRepo.EXPECT().UpdateItemRunLog(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	mockTurnResultRepo.EXPECT().CreateOrUpdateItemsTurnRunLogStatus(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	guard := &fakeGuard{}
	svc := &ExptItemEventEvalServiceImpl{
		configer:           mockConfiger,
		metric:             mockMetric,
		exptItemResultRepo: mockItemResultRepo,
		exptTurnResultRepo: mockTurnResultRepo,
		centralGuard:       guard,
	}

	event := &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4, RetryTimes: 3}
	ctx := ctxcache.Init(context.Background())
	event.WithCtxCentralQuotaHeld(ctx, testScope)

	handler := svc.HandleEventErr(func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		return errors.New("build eval ctx fail")
	})
	assert.NoError(t, handler(ctx, event), "兜底路径吞掉错误，不让 MQ 无限重投")

	assert.Len(t, guard.releases(), 1, "不可重试失败落 Fail 后必须归还额度")
}

// TestHandleEventErr_ReleasesQuotaOnIndebtTermination 欠费终止分支同样要归还额度。
//
// 该分支把整个实验落 Terminated 却不动 item run log，所以额度闸内按 run log 状态判定时
// 一定不会释放 —— 不在这里补，预占就跟着终止的实验一起永久留在账本里。
func TestHandleEventErr_ReleasesQuotaOnIndebtTermination(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfiger := componentMocks.NewMockIConfiger(ctrl)
	mockMetric := metricsmocks.NewMockExptMetric(ctrl)
	mockManager := svcmocks.NewMockIExptManager(ctrl)

	mockConfiger.EXPECT().GetErrRetryConf(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&entity.RetryConf{RetryTimes: 3, RetryIntervalSecond: 60, IsInDebt: true})
	mockMetric.EXPECT().EmitItemExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
	mockManager.EXPECT().CompleteRun(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	mockManager.EXPECT().CompleteExpt(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	guard := &fakeGuard{}
	svc := &ExptItemEventEvalServiceImpl{
		configer:     mockConfiger,
		metric:       mockMetric,
		manager:      mockManager,
		centralGuard: guard,
	}

	event := &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4}
	ctx := ctxcache.Init(context.Background())
	event.WithCtxCentralQuotaHeld(ctx, testScope)

	handler := svc.HandleEventErr(func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		return errors.New("insufficient benefit")
	})
	assert.NoError(t, handler(ctx, event))

	calls := guard.releases()
	assert.Len(t, calls, 1, "欠费终止后必须归还额度")
	assert.Contains(t, calls[0].Reason, "indebt")
}

// TestHandleEventErr_NoReleaseWhenRetrying 重试路径**绝不能**释放。
//
// 这是反方向的错误、且后果更隐蔽：若在重试前释放了额度，重投的消息会在 ConfirmRunning
// 处因 reservation 不存在被丢弃，item 永久停在 Processing、实验永不收敛。
func TestHandleEventErr_NoReleaseWhenRetrying(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockConfiger := componentMocks.NewMockIConfiger(ctrl)
	mockMetric := metricsmocks.NewMockExptMetric(ctrl)
	mockPublisher := eventmocks.NewMockExptEventPublisher(ctrl)

	mockConfiger.EXPECT().GetErrRetryConf(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&entity.RetryConf{RetryTimes: 3, RetryIntervalSecond: 60, IsInDebt: false})
	mockMetric.EXPECT().EmitItemExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
	mockPublisher.EXPECT().PublishExptRecordEvalEvent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

	guard := &fakeGuard{}
	svc := &ExptItemEventEvalServiceImpl{
		configer:     mockConfiger,
		metric:       mockMetric,
		publisher:    mockPublisher,
		centralGuard: guard,
	}

	event := &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4, RetryTimes: 1}
	ctx := ctxcache.Init(context.Background())
	event.WithCtxCentralQuotaHeld(ctx, testScope)

	handler := svc.HandleEventErr(func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		return errors.New("transient fail")
	})
	assert.NoError(t, handler(ctx, event))

	assert.Empty(t, guard.releases(),
		"重试路径释放额度会让重投消息被丢弃，item 永久卡 Processing")
}

// TestHandleCentralAdmission_LegacyPassesThroughWithoutCredential legacy 实验必须原样直通，
// 且不在 ctx 留下准入凭据 —— 否则锁内那层会为它去查额度账本。
func TestHandleCentralAdmission_LegacyPassesThroughWithoutCredential(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	mockExptRepo.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&entity.Experiment{ID: 1, ExptDispatchMode: entity.ExptDispatchModeLegacy}, nil)

	svc := &ExptItemEventEvalServiceImpl{experimentRepo: mockExptRepo}
	event := &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4}
	ctx := ctxcache.Init(context.Background())

	nextCalled := false
	handler := svc.HandleCentralAdmission(func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		nextCalled = true
		_, admitted := event.CtxCentralAdmittedExpt(ctx)
		assert.False(t, admitted, "legacy 实验不得留下准入凭据")
		return nil
	})

	assert.NoError(t, handler(ctx, event))
	assert.True(t, nextCalled, "legacy 实验必须直通执行")
}

// TestHandleCentralAdmission_EnforceStoresCredential enforce 实验通过全部准入检查后，
// 必须把实验挂到 ctx —— 锁内那层据此判断"要走额度闸"，并复用这个实验免去二次 GetByID。
func TestHandleCentralAdmission_EnforceStoresCredential(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	expt := &entity.Experiment{ID: 1, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: testScope}
	mockExptRepo.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).Return(expt, nil)

	svc := &ExptItemEventEvalServiceImpl{experimentRepo: mockExptRepo, centralGuard: &fakeGuard{}}
	event := &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4}
	ctx := ctxcache.Init(context.Background())

	nextCalled := false
	handler := svc.HandleCentralAdmission(func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		nextCalled = true
		got, admitted := event.CtxCentralAdmittedExpt(ctx)
		assert.True(t, admitted, "enforce 实验通过准入后必须留下凭据")
		assert.Equal(t, testScope, got.SchedulerScope)
		return nil
	})

	assert.NoError(t, handler(ctx, event))
	assert.True(t, nextCalled)
}

// TestHandleCentralAdmission_DropsEnforceWithEmptyScope 空 Scope 的 enforce 实验必须丢弃
// 且不进入下游：没有 Scope 就无法确定去哪本账查 reservation，猜一本账等于用别人的额度跑 item。
func TestHandleCentralAdmission_DropsEnforceWithEmptyScope(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	mockExptRepo.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&entity.Experiment{ID: 1, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: ""}, nil)

	svc := &ExptItemEventEvalServiceImpl{experimentRepo: mockExptRepo, centralGuard: &fakeGuard{}}
	event := &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4}

	nextCalled := false
	handler := svc.HandleCentralAdmission(func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		nextCalled = true
		return nil
	})

	assert.NoError(t, handler(ctxcache.Init(context.Background()), event), "丢弃而非报错，避免 MQ 无限重投")
	assert.False(t, nextCalled, "空 Scope 的 enforce 实验不得执行")
}

// TestHandleCentralAdmission_DropsWhenGuardMissing 判定为 enforce 却没注入闸门时 fail-closed。
// 放行等于让实验在无额度约束下跑（静默）；停下来则可见。
func TestHandleCentralAdmission_DropsWhenGuardMissing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockExptRepo := repoMocks.NewMockIExperimentRepo(ctrl)
	mockExptRepo.EXPECT().GetByID(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&entity.Experiment{ID: 1, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: testScope}, nil)

	svc := &ExptItemEventEvalServiceImpl{experimentRepo: mockExptRepo} // centralGuard 为 nil
	event := &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4}

	nextCalled := false
	handler := svc.HandleCentralAdmission(func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		nextCalled = true
		return nil
	})

	assert.NoError(t, handler(ctxcache.Init(context.Background()), event))
	assert.False(t, nextCalled, "无闸门时 enforce 实验必须 fail-closed")
}

// TestHandleCentralReservation_PassesThroughWhenNotAdmitted 锁内那层拿不到准入凭据时必须直通。
//
// 方向很关键：这里的"缺失"含义是"legacy 或已被上层丢弃"，取 fail-closed 会把全部 legacy
// 实验误挡住 —— 那是自造的全量故障。
func TestHandleCentralReservation_PassesThroughWhenNotAdmitted(t *testing.T) {
	guard := &fakeGuard{}
	svc := &ExptItemEventEvalServiceImpl{centralGuard: guard}
	event := &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4}

	nextCalled := false
	handler := svc.HandleCentralReservation(func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		nextCalled = true
		return nil
	})

	assert.NoError(t, handler(ctxcache.Init(context.Background()), event))
	assert.True(t, nextCalled, "无凭据必须直通（legacy 语义）")
	assert.Zero(t, guard.confirmCalls, "legacy 不该查额度账本")
}

// TestHandleCentralReservation_DiscardsWhenReservationAbsent ConfirmRunning 返回 false
// （迟到消息 / 账本已重建 / 已被释放）时必须丢弃，不得执行。
func TestHandleCentralReservation_DiscardsWhenReservationAbsent(t *testing.T) {
	guard := &fakeGuard{confirmResult: false}
	svc := &ExptItemEventEvalServiceImpl{centralGuard: guard}
	event := &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4}

	ctx := ctxcache.Init(context.Background())
	event.WithCtxCentralAdmittedExpt(ctx, &entity.Experiment{
		ID: 1, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: testScope,
	})

	nextCalled := false
	handler := svc.HandleCentralReservation(func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		nextCalled = true
		return nil
	})

	assert.NoError(t, handler(ctx, event))
	assert.False(t, nextCalled, "reservation 不存在时不得执行 item")
	assert.Equal(t, 1, guard.confirmCalls)
}

// TestHandleCentralReservation_RunsInsideLock 验证本次调整的目标：
// ConfirmRunning 必须在 item 锁**内**发生。
//
// 断言方式是把 Lock 与 Reservation 按生产链的顺序串起来，记录事件序列 ——
// 直接断言"顺序"而不是断言"调用过"，这样以后有人把两层顺序改回去，测试会失败而不是静默通过。
func TestHandleCentralReservation_RunsInsideLock(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var seq []string

	mockMutex := lockmocks.NewMockILocker(ctrl)
	mockMutex.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, key string, _, _ interface{}) (bool, context.Context, func(), error) {
			seq = append(seq, "lock")
			return true, ctx, func() {}, nil
		})
	mockMutex.EXPECT().Unlock(gomock.Any()).DoAndReturn(func(key string) (bool, error) {
		seq = append(seq, "unlock")
		return true, nil
	})

	guard := &recordingGuard{seq: &seq, confirmResult: true}
	dispatchRepo := repoMocks.NewMockIExptItemDispatchRepo(ctrl)
	dispatchRepo.EXPECT().StartReservedItem(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _, _, _ int64) (bool, error) {
			seq = append(seq, "start_reserved")
			return true, nil
		})
	dispatchRepo.EXPECT().MGetDispatchObservations(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*repo.ExptDispatchObservation{
			{ItemID: 4, Status: int32(entity.ItemRunState_Success)},
		}, nil)

	svc := &ExptItemEventEvalServiceImpl{
		mutex:        mockMutex,
		centralGuard: guard,
		dispatchRepo: dispatchRepo,
	}

	event := &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4}
	ctx := ctxcache.Init(context.Background())
	event.WithCtxCentralAdmittedExpt(ctx, &entity.Experiment{
		ID: 1, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: testScope,
	})

	// 按生产链顺序组装：Lock 在外，Reservation 在内。
	inner := svc.HandleCentralReservation(func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		seq = append(seq, "exec")
		return nil
	})
	handler := svc.HandleEventLock(inner)

	assert.NoError(t, handler(ctx, event))

	// lock 必须先于 confirm，unlock 必须后于 exec。
	assert.Equal(t, []string{"lock", "confirm", "start_reserved", "exec", "release", "unlock"}, seq,
		"ConfirmRunning / StartReservedItem / 执行 必须都在 item 锁的临界区内")
}

// recordingGuard 在 fakeGuard 之外单独一个类型：这里只需要把调用顺序追加到共享切片，
// 不需要计数与并发保护（RunsInsideLock 是单 goroutine 的顺序断言）。
type recordingGuard struct {
	seq           *[]string
	confirmResult bool
}

func (r *recordingGuard) ConfirmRunning(ctx context.Context, schedulerScope string, exptRunID, itemID int64) (bool, error) {
	*r.seq = append(*r.seq, "confirm")
	return r.confirmResult, nil
}

func (r *recordingGuard) Release(ctx context.Context, schedulerScope string, exptRunID, itemID int64, reason string) error {
	*r.seq = append(*r.seq, "release")
	return nil
}

// TestHandleCentralReservation_AdvancesMainTableForDisplay 中心调度取得执行权后，
// **主表 expt_item_result 也必须推进到 Processing**，否则用户看到的状态是错的。
//
// ★ 这条守的是一个真实缺陷（2026-08-23 PPE 实测）：
// run log 是执行真值，但用户看到的是主表 —— MGetExperimentResult 走 expt_item_result
// 构造 run_state。中心调度这条新派发路径只写了 run log，于是 5 个已 Processing 14 小时的
// item 在 results 接口里**全部显示 queueing**，现象是"实验看着没动"，而它跑得很正常。
//
// legacy 的 handleToSubmits 一直成对写这两张表（UpdateItemRunLog + UpdateItemsResult），
// 新路径漏了后者 —— 平行实现漏字段的又一例，与团队记忆里
// 「新增派发路径漏设 ExptRunMode」同族：**老路径对、新路径漏，且单测不断言就发现不了**。
//
// 断言的是"主表被推进到 Processing"这件事本身，而不只是"某个方法被调过"，
// 所以把 ufields 的内容也钉住：只写别的字段不算修好。
func TestHandleCentralReservation_AdvancesMainTableForDisplay(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	guard := &fakeGuard{confirmResult: true}
	dispatchRepo := repoMocks.NewMockIExptItemDispatchRepo(ctrl)
	dispatchRepo.EXPECT().StartReservedItem(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(true, nil)
	dispatchRepo.EXPECT().MGetDispatchObservations(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*repo.ExptDispatchObservation{
			{ItemID: 4, Status: int32(entity.ItemRunState_Success)},
		}, nil)

	var gotFields map[string]any
	var gotItemIDs []int64
	itemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	itemResultRepo.EXPECT().UpdateItemsResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ int64, itemIDs []int64, ufields map[string]any) error {
			gotItemIDs = itemIDs
			gotFields = ufields
			return nil
		})

	svc := &ExptItemEventEvalServiceImpl{
		centralGuard:       guard,
		dispatchRepo:       dispatchRepo,
		exptItemResultRepo: itemResultRepo,
	}

	event := &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4}
	ctx := ctxcache.Init(context.Background())
	event.WithCtxCentralAdmittedExpt(ctx, &entity.Experiment{
		ID: 1, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: testScope,
	})

	handler := svc.HandleCentralReservation(func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		return nil
	})
	assert.NoError(t, handler(ctx, event))

	assert.NotNil(t, gotFields, "主表必须被推进 —— 只写 run log 会让用户一直看到 queueing")
	assert.Equal(t, []int64{4}, gotItemIDs, "推进的必须是本次取得执行权的那个 item")
	assert.Equal(t, int32(entity.ItemRunState_Processing), gotFields["status"],
		"主表状态必须是 Processing，与 run log 一致")
}

// TestHandleCentralReservation_AdvancesStatsToProcessing 中心调度取得执行权后，
// **expt_stats 也必须记 Queueing→Processing**，否则计数会单向下溢。
//
// ★ 这条守的是一个真实缺陷（2026-08-24 PPE 长跑压测实测）：
// 完成侧（expt_result_impl.go 的 statsCntOp）做的是「从 item 原状态减 1、往新状态加 1」，
// 减的是 Processing 桶。legacy 在 handleToSubmits 派发时就把 item 计入 Processing，两边配对；
// 中心调度的派发只把 run log CAS 成 Queueing/reserved（**status 仍是 Queueing**），
// 从未有人往 Processing 桶加过 —— 完成时减一个从未加过的计数。
// 实测一个 14 题 enforce 实验：fail 累到 4 时 processing = -4，pending 恒 14 从不下降。
//
// 断言 op 的内容而非"调过就行"：只加 Processing 不减 Queueing，pending 依旧不降。
func TestHandleCentralReservation_AdvancesStatsToProcessing(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	guard := &fakeGuard{confirmResult: true}
	dispatchRepo := repoMocks.NewMockIExptItemDispatchRepo(ctrl)
	dispatchRepo.EXPECT().StartReservedItem(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(true, nil)
	dispatchRepo.EXPECT().MGetDispatchObservations(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*repo.ExptDispatchObservation{
			{ItemID: 4, Status: int32(entity.ItemRunState_Success)},
		}, nil)

	itemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	itemResultRepo.EXPECT().UpdateItemsResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()

	var gotOp *entity.StatsCntArithOp
	statsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
	statsRepo.EXPECT().ArithOperateCount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, _, _ int64, op *entity.StatsCntArithOp) error {
			gotOp = op
			return nil
		})

	svc := &ExptItemEventEvalServiceImpl{
		centralGuard:       guard,
		dispatchRepo:       dispatchRepo,
		exptItemResultRepo: itemResultRepo,
		exptStatsRepo:      statsRepo,
	}

	event := &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4}
	ctx := ctxcache.Init(context.Background())
	event.WithCtxCentralAdmittedExpt(ctx, &entity.Experiment{
		ID: 1, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: testScope,
	})

	handler := svc.HandleCentralReservation(func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		return nil
	})
	assert.NoError(t, handler(ctx, event))

	assert.NotNil(t, gotOp, "必须记 stats —— 不记会让完成侧减一个从未加过的计数")
	assert.Equal(t, 1, gotOp.OpStatusCnt[entity.ItemRunState_Processing],
		"Processing 必须 +1，与完成侧的 -1 配对")
	assert.Equal(t, -1, gotOp.OpStatusCnt[entity.ItemRunState_Queueing],
		"Queueing 必须 -1，否则 pending_turn_count 永不下降")
}

// TestHandleCentralReservation_SkipsStatsOnDuplicateDelivery 重复投递时**不得**再记 stats。
//
// started=false 表示 CAS 未命中 —— item 早已是 Processing（重复投递）或已被 repair 修正。
// 此时再加一次就从"少计"变成"多计"，方向相反但同样是错的。
// CAS 结果是这条路径上唯一的"恰好一次"信号，所以记账必须绑定它。
//
// ★ 这条是上一个用例的反面：只断言"会记"，把记账写在 started 判断之外也能通过。
func TestHandleCentralReservation_SkipsStatsOnDuplicateDelivery(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	guard := &fakeGuard{confirmResult: true}
	dispatchRepo := repoMocks.NewMockIExptItemDispatchRepo(ctrl)
	// CAS 未命中：重复投递或已被 repair 修正
	dispatchRepo.EXPECT().StartReservedItem(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(false, nil)
	dispatchRepo.EXPECT().MGetDispatchObservations(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]*repo.ExptDispatchObservation{
			{ItemID: 4, Status: int32(entity.ItemRunState_Success)},
		}, nil)

	itemResultRepo := repoMocks.NewMockIExptItemResultRepo(ctrl)
	itemResultRepo.EXPECT().UpdateItemsResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()

	statsRepo := repoMocks.NewMockIExptStatsRepo(ctrl)
	// Times(0)：一次都不能调 —— 这是本用例的全部意义
	statsRepo.EXPECT().ArithOperateCount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).Times(0)

	svc := &ExptItemEventEvalServiceImpl{
		centralGuard:       guard,
		dispatchRepo:       dispatchRepo,
		exptItemResultRepo: itemResultRepo,
		exptStatsRepo:      statsRepo,
	}

	event := &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4}
	ctx := ctxcache.Init(context.Background())
	event.WithCtxCentralAdmittedExpt(ctx, &entity.Experiment{
		ID: 1, ExptDispatchMode: entity.ExptDispatchModeEnforce, SchedulerScope: testScope,
	})

	handler := svc.HandleCentralReservation(func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		return nil
	})
	assert.NoError(t, handler(ctx, event))
}
