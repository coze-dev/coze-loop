// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package component

import "context"

//go:generate mockgen -destination=mocks/central_reservation_guard.go -package=mocks . ICentralReservationGuard

// ICentralReservationGuard 中心化调度的额度预占校验闸。
//
// OSS 只定义这个最窄的 port，完整的额度账本、调度算法与 Adapter 都在商业版实现并由 Wire 注入。
// 开源部署下注入 noop 实现：legacy 消息完全不受影响，而意外出现的 central 消息 fail-closed
// （拒绝执行），避免无账本环境下绕过额度跑 item。
//
// 为什么 item consumer 必须经过它：中心调度先原子预占额度、再投递 item MQ。若 consumer 不校验
// reservation 就执行，那么迟到的、重复的、账本已重建过的消息都会变成"无额度执行"，全局额度
// 保护即失效。
type ICentralReservationGuard interface {
	// ConfirmRunning 取得该 item 的一次性执行权。
	//
	// 返回 false 表示 reservation 不存在 —— 调用方**必须放弃执行并丢弃消息**，不得继续跑 item。
	// 已是 Running 的重复投递返回 true：同一 item 的合法原地重试要继续持有原额度，
	// 不重新预占也不重复扣减。
	//
	// schedulerScope 是 Experiment 冻结的调度域，决定去哪本额度账本查这条 reservation。
	// 由调用方从 DB 读出后传入，不由实现方按当前运行环境推断 —— 推断会让"实验属于哪本账"
	// 取决于谁在处理消息，而它本该只取决于数据本身。
	ConfirmRunning(ctx context.Context, schedulerScope string, exptRunID, itemID int64) (bool, error)

	// Release 在 item 进入终态（成功/失败/终止/僵尸清理）时幂等释放额度。
	// 重复调用为 no-op。
	Release(ctx context.Context, schedulerScope string, exptRunID, itemID int64, reason string) error
}

// NewNoopCentralReservationGuard 返回开源部署使用的 noop 实现。
func NewNoopCentralReservationGuard() ICentralReservationGuard {
	return noopCentralReservationGuard{}
}

type noopCentralReservationGuard struct{}

// ConfirmRunning 在无账本环境下一律拒绝。
//
// 选择 fail-closed 而非放行：本方法只会被判定为 enforce 的实验触达，而 enforce 意味着
// "该实验的额度由中心账本管控"。没有账本却放行，等于让实验在无任何额度约束下跑，
// 这比让它停下来等待更危险 —— 后者可见（实验不动会被发现），前者静默（资源被打爆才发现）。
func (noopCentralReservationGuard) ConfirmRunning(ctx context.Context, schedulerScope string, exptRunID, itemID int64) (bool, error) {
	return false, nil
}

// Release 无账本可释放，直接成功返回。
// 不返回错误：终态收口路径不应因为额度模块缺席而失败。
func (noopCentralReservationGuard) Release(ctx context.Context, schedulerScope string, exptRunID, itemID int64, reason string) error {
	return nil
}

//go:generate mockgen -destination=mocks/central_scope_owner.go -package=mocks . ICentralSchedulerScopeOwner

// ICentralSchedulerScopeOwner 判定本进程是否拥有某个调度域。
//
// 与 ICentralReservationGuard 分开是因为职责不同：Guard 回答"这个 item 有没有额度"，
// 本 port 回答"这个 item 该不该由我来跑"。后者是路由/归属问题，即使额度充足也可能
// 必须拒绝执行（消息投错了环境）。
//
// 为什么消息可能投错环境：item MQ 的泳道路由依赖 producer 侧的 x_tt_env tag，
// 而该 tag 会因环境变量缺失、消息重投、broker 配置差异而失效。届时一条 PPE 的 item
// 消息可能被线上 consumer 取到（或反之），若不校验归属就会用一个环境的进程去跑另一个
// 环境的 item，而两个环境共用同一个库 —— 结果直接写进对方的数据。
//
// 开源部署注入 noop（恒定拥有）：单环境部署不存在跨环境投递，无需此校验。
type ICentralSchedulerScopeOwner interface {
	// OwnsSchedulerScope 报告 schedulerScope 是否属于当前进程。
	// 返回 error 表示无法判定（如运行环境探测失败）—— 调用方应重试而非放行。
	OwnsSchedulerScope(ctx context.Context, schedulerScope string) (bool, error)
}

// NewNoopCentralSchedulerScopeOwner 返回开源部署使用的 noop 实现：恒定拥有。
//
// 与 Guard 的 noop 取 fail-closed 相反，本实现取放行。原因是二者防的风险不同：
// 无账本时放行会绕过额度（危险），而单环境部署下不存在"别的环境"，拒绝执行只会让
// 所有 enforce item 永久卡住 —— 那是自造故障，不是防护。
func NewNoopCentralSchedulerScopeOwner() ICentralSchedulerScopeOwner {
	return noopCentralSchedulerScopeOwner{}
}

type noopCentralSchedulerScopeOwner struct{}

func (noopCentralSchedulerScopeOwner) OwnsSchedulerScope(ctx context.Context, schedulerScope string) (bool, error) {
	return true, nil
}
