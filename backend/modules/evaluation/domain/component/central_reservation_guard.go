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
	ConfirmRunning(ctx context.Context, exptRunID, itemID int64) (bool, error)

	// Release 在 item 进入终态（成功/失败/终止/僵尸清理）时幂等释放额度。
	// 重复调用为 no-op。
	Release(ctx context.Context, exptRunID, itemID int64, reason string) error
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
func (noopCentralReservationGuard) ConfirmRunning(ctx context.Context, exptRunID, itemID int64) (bool, error) {
	return false, nil
}

// Release 无账本可释放，直接成功返回。
// 不返回错误：终态收口路径不应因为额度模块缺席而失败。
func (noopCentralReservationGuard) Release(ctx context.Context, exptRunID, itemID int64, reason string) error {
	return nil
}
