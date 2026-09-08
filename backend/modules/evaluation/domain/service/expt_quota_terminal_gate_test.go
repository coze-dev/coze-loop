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
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repoMocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
)

// 本文件覆盖**主释放点** releaseQuotaIfItemTerminal 的判定闸。
//
// 既有 expt_central_quota_release_test.go 覆盖的是 HandleEventErr 那层的兜底释放；
// 而 item 正常跑完/正常失败走的是这一层，它此前没有任何测试 —— 而它恰好是测试矩阵
// R2（失败即释放）/ R3（重试不得释放）/ R8（不重复释放）三格共同的判据所在。
//
// 为什么用单测而不是泳道 E2E 覆盖这三格：这一层的全部分支都由「回查 run log 投影拿到
// 什么状态」决定，单测能精确摆出每种状态（含"查不到"、"仍在 Processing"），
// 而在泳道上构造"失败但可重试"与"失败且终态"的区别要靠让评测对象按特定方式报错，
// 既慢又不稳定。真机负责验证链路连通（已完成），分支穷举交给这里。

// terminalGateFixture 装配一个只关心额度闸的 service。
type terminalGateFixture struct {
	svc   *ExptItemEventEvalServiceImpl
	guard *fakeGuard
	event *entity.ExptItemEvalEvent
}

// newTerminalGateFixture 让 MGetDispatchObservations 回指定的投影观测。
// obs 为 nil 表示"查不到记录"。
func newTerminalGateFixture(t *testing.T, obs []*repo.ExptDispatchObservation) *terminalGateFixture {
	t.Helper()
	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	dispatchRepo := repoMocks.NewMockIExptItemDispatchRepo(ctrl)
	dispatchRepo.EXPECT().
		MGetDispatchObservations(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(obs, nil).AnyTimes()

	guard := &fakeGuard{}
	return &terminalGateFixture{
		svc:   &ExptItemEventEvalServiceImpl{centralGuard: guard, dispatchRepo: dispatchRepo},
		guard: guard,
		event: &entity.ExptItemEvalEvent{ExptID: 1, ExptRunID: 2, SpaceID: 3, EvalSetItemID: 4},
	}
}

func observation(status entity.ItemRunState) []*repo.ExptDispatchObservation {
	return []*repo.ExptDispatchObservation{{ItemID: 4, Status: int32(status)}}
}

// TestReleaseQuotaIfItemTerminal_ReleasesOnEveryTerminalState 矩阵 R2：
// **终态即释放，不分成败**。
//
// Fail 与 Terminal 必须和 Success 一样释放：额度是"占着资源的凭据"，item 无论以哪种方式
// 结束都不再占资源。漏掉失败态是最容易犯的错（直觉上"失败了就不用管了"），
// 后果是失败越多、泄漏越多，而失败在评测里是常态。
func TestReleaseQuotaIfItemTerminal_ReleasesOnEveryTerminalState(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  entity.ItemRunState
		execErr error
		wantSub string
	}{
		{"成功 → 释放", entity.ItemRunState_Success, nil, "item success"},
		{"失败 → 同样释放", entity.ItemRunState_Fail, errors.New("evaluator boom"), "item failed"},
		{"提前终止 → 同样释放", entity.ItemRunState_Terminal, nil, "item terminated"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newTerminalGateFixture(t, observation(tc.status))

			f.svc.releaseQuotaIfItemTerminal(context.Background(), f.event, testScope, f.guard, tc.execErr)

			releases := f.guard.releases()
			require.Len(t, releases, 1, "终态必须释放额度，否则该 item 的配额永久泄漏")
			assert.Equal(t, testScope, releases[0].Scope)
			assert.Equal(t, f.event.ExptRunID, releases[0].RunID)
			assert.Equal(t, f.event.EvalSetItemID, releases[0].ItemID)
			// reason 会进日志，是排查"额度什么时候被谁放掉的"唯一线索，必须能区分成败。
			assert.Contains(t, releases[0].Reason, tc.wantSub)
		})
	}
}

// TestReleaseQuotaIfItemTerminal_KeepsReservationWhileRetriable 矩阵 R3：
// **MQ 重试路径不得释放** —— 这是代码里刻意的反直觉设计，最容易被后来人当 bug 改掉。
//
// 若在此释放：重投消息稍后到达，ConfirmRunning 会因 reservation 不存在而**丢弃消息**，
// item 就永久停在 Processing（既不完成也不失败），比"额度多占一会"严重得多。
//
// 判据刻意是"回查投影的真实状态"而不是"execErr 是否为空"：execErr != nil 既可能是
// 可重试的瞬时错、也可能是已落终态的失败，只看 err 无法区分。
func TestReleaseQuotaIfItemTerminal_KeepsReservationWhileRetriable(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status entity.ItemRunState
	}{
		{"仍在执行/等待重投", entity.ItemRunState_Processing},
		{"排队中", entity.ItemRunState_Queueing},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// 带上 execErr：模拟"执行报错了，但 item 还没被判终态"这个最危险的组合 ——
			// 只看 err 就释放的实现会在这里放掉额度。
			f := newTerminalGateFixture(t, observation(tc.status))

			f.svc.releaseQuotaIfItemTerminal(context.Background(), f.event, testScope, f.guard, errors.New("transient"))

			assert.Empty(t, f.guard.releases(),
				"未终态就释放会让重投消息在 ConfirmRunning 处被丢弃，item 永久卡在 %v", tc.status)
		})
	}
}

// TestReleaseQuotaIfItemTerminal_SkipsWhenProjectionMissing 投影查不到时不得释放。
//
// "查不到"通常意味着 run log 已被清理（重跑清表等），此时手上这条 reservation 可能属于
// **新一轮 run**，贸然释放等于放掉别人的额度 —— 而账本 key 是 (run_id,item_id)，
// 新一轮 run 的 item 完成时会再释放一次，就成了双重释放。交给对账处理。
func TestReleaseQuotaIfItemTerminal_SkipsWhenProjectionMissing(t *testing.T) {
	f := newTerminalGateFixture(t, nil)

	f.svc.releaseQuotaIfItemTerminal(context.Background(), f.event, testScope, f.guard, nil)

	assert.Empty(t, f.guard.releases(), "投影查不到时释放可能放掉新一轮 run 的额度")
}

// TestReleaseQuotaIfItemTerminal_NoopWithoutGuardOrRepo 依赖缺失时必须安静返回。
//
// legacy 部署（未接中心调度）里这两个依赖都是 nil，此处 panic 会打挂整条 item 执行链，
// 把"中心调度没启用"变成"所有实验都跑不了"。
func TestReleaseQuotaIfItemTerminal_NoopWithoutGuardOrRepo(t *testing.T) {
	t.Run("guard 为 nil", func(t *testing.T) {
		svc := &ExptItemEventEvalServiceImpl{}
		assert.NotPanics(t, func() {
			svc.releaseQuotaIfItemTerminal(context.Background(),
				&entity.ExptItemEvalEvent{}, testScope, nil, nil)
		})
	})

	t.Run("dispatchRepo 为 nil", func(t *testing.T) {
		guard := &fakeGuard{}
		svc := &ExptItemEventEvalServiceImpl{centralGuard: guard}
		assert.NotPanics(t, func() {
			svc.releaseQuotaIfItemTerminal(context.Background(),
				&entity.ExptItemEvalEvent{}, testScope, guard, nil)
		})
		assert.Empty(t, guard.releases(), "拿不到投影就不该释放")
	})
}

// TestReleaseQuotaIfItemTerminal_ReleaseErrorIsSwallowed 释放失败只记日志，不得冒泡。
//
// 这一层在中间件出口，返回 error 会覆盖掉 item 真正的执行结果 —— 一次 Redis 抖动就会把
// 成功的 item 变成失败。释放失败留给对账兜底（现已实现）。
func TestReleaseQuotaIfItemTerminal_ReleaseErrorIsSwallowed(t *testing.T) {
	f := newTerminalGateFixture(t, observation(entity.ItemRunState_Success))
	f.guard.releaseErr = errors.New("redis down")

	assert.NotPanics(t, func() {
		f.svc.releaseQuotaIfItemTerminal(context.Background(), f.event, testScope, f.guard, nil)
	})
	assert.Len(t, f.guard.releases(), 1, "失败也应记录已尝试释放")
}

// TestTerminalReleaseReason_DerivedFromStatusNotExecErr 矩阵 R2 的补强：
// 释放原因必须由**回查到的 run log 终态**推导，不能由 execErr 推导。
//
// 这是一个真机踩出来的 bug：`CompleteItemRun` 对普通 item 失败 `return nil`
// （只有 in-debt 错误才向上抛），`Eval` 又只透传其返回值，于是 execErr 恒为 nil。
// 旧实现 `reason := "item success"; if execErr != nil {...}` 因此永远走第一行 ——
// BOE 实测 129 条释放日志**全部**写着 item success，其中 104 个 item 的 DB 状态是 Fail。
//
// 后果不是额度算错（释放本身按 IsItemRunFinished 判定，是对的），而是**可观测性失真**：
// 靠日志查"额度被谁吃了"会得到完全错误的图景，也发现不了实验在大面积失败。
func TestTerminalReleaseReason_DerivedFromStatusNotExecErr(t *testing.T) {
	for _, tc := range []struct {
		name    string
		status  entity.ItemRunState
		execErr error
		want    string
	}{
		// ★ 核心：execErr 为 nil（普通失败被吞掉后的真实形态）时，Fail 仍须标成 failed。
		{"Fail + execErr=nil（被吞掉的普通失败）", entity.ItemRunState_Fail, nil, "item failed"},
		{"Success + execErr=nil", entity.ItemRunState_Success, nil, "item success"},
		{"Terminal（提前终止）", entity.ItemRunState_Terminal, nil, "item terminated"},
		// in-debt 一类会向上抛的错误：状态与错误文本都要带上。
		{"Fail + execErr 非 nil（in-debt）", entity.ItemRunState_Fail, errors.New("in debt"), "item failed: in debt"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, terminalReleaseReason(tc.status, tc.execErr))
		})
	}
}

// TestTerminalReleaseReason_UnknownTerminalStateCarriesCode 终态集合扩展但本函数没跟上时，
// 必须带出原始状态码而不是静默标成 success。
//
// 静默标 success 正是上一个 bug 的形态 —— 同样的错不该以另一种方式再犯一次。
func TestTerminalReleaseReason_UnknownTerminalStateCarriesCode(t *testing.T) {
	got := terminalReleaseReason(entity.ItemRunState(99), nil)
	assert.Contains(t, got, "99", "未识别的终态必须带出状态码，便于反查是哪个新状态")
	assert.NotContains(t, got, "success", "不得把未知终态静默标成成功")
}

// TestReleaseQuotaIfItemTerminal_FailedItemGetsFailedReason 端到端串起来：
// 投影是 Fail、execErr 为 nil 时，实际传给 guard.Release 的 reason 必须能看出是失败。
//
// 单独测纯函数不够 —— 要防的是"函数改对了但调用点还在用 execErr"。
func TestReleaseQuotaIfItemTerminal_FailedItemGetsFailedReason(t *testing.T) {
	f := newTerminalGateFixture(t, observation(entity.ItemRunState_Fail))

	// execErr 刻意传 nil：这正是普通 item 失败到达本层时的真实形态。
	f.svc.releaseQuotaIfItemTerminal(context.Background(), f.event, testScope, f.guard, nil)

	releases := f.guard.releases()
	require.Len(t, releases, 1, "Fail 是终态，额度必须释放")
	assert.Contains(t, releases[0].Reason, "failed",
		"execErr 为 nil 但投影是 Fail 时，reason 仍须标成失败 —— 否则失败会伪装成成功")
	assert.NotContains(t, releases[0].Reason, "success")
}
