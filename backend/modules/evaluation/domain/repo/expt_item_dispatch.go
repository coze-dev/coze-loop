// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate mockgen -destination=mocks/expt_item_dispatch.go -package=mocks . IExptItemDispatchRepo

// IExptItemDispatchRepo 中心化调度的 item 派发投影读写。
//
// 与既有 IExptItemResultRepo 的分工：后者面向 item 执行结果的完整生命周期；本接口只关心
// 「哪些 item 可被授予、哪些已占用并发」这一个问题，因此方法都围绕 run log 的
// (status, quota_reservation_state) 二元组，且全部走 idx_expt_run_dispatch。
//
// 为什么单独立一个窄接口而不是往 IExptItemResultRepo 加方法：那个接口已有 15+ 方法、被十余处
// 依赖，调度只需其中极小一部分；混进去会让所有实现方和 mock 都被迫感知调度概念。
type IExptItemDispatchRepo interface {
	// ClaimQuotaReserved 把一批 item 的 run log 从 Queueing/none CAS 成 Queueing/reserved。
	//
	// 返回实际 claim 成功的 item ID。失败的 item 说明它已被别的路径改动（并发 tick、
	// consumer 抢先启动、或 item 已终态）—— 调用方必须只发布成功的那些，并释放失败项
	// 在 Redis 侧仍为 Reserved 的 reservation。
	//
	// 这是 Redis 预占与 MQ 发布之间的必经关卡：没有它，同一 item 可能被两拍各自预占各自发布。
	ClaimQuotaReserved(ctx context.Context, spaceID, exptID, exptRunID int64, itemIDs []int64) (claimed []int64, err error)

	// ResetQuotaReserved 把 Queueing/reserved 退回 Queueing/none，用于 MQ 明确发布失败的补偿。
	//
	// 只处理仍为 Queueing/reserved 的记录：若 item 已被 consumer 推进到 Processing，
	// 说明消息其实已投递成功，退回投影会让它被重复授予。
	ResetQuotaReserved(ctx context.Context, spaceID, exptID, exptRunID int64, itemIDs []int64) (reset []int64, err error)

	// LoadDispatchRuntime 一次查询取回该 run 的并发占用与可授予候选。
	//
	// 并发占用 = status=Processing 的 item + status=Queueing 且 reserved 的 item；
	// 可授予候选 = status=Queueing 且 none 的 item（按 id 升序，取前 limit 个）。
	// 两者由同一次查询产出，避免分两次查询之间状态漂移导致 deficit 算错。
	LoadDispatchRuntime(ctx context.Context, spaceID, exptID, exptRunID int64, candidateLimit int) (*ExptDispatchRuntime, error)

	// StartReservedItem 由 consumer 在取得执行权后调用，把 run log 从 Queueing/reserved
	// 推进到 Processing/none，表示「预占已兑现为实际执行」。
	//
	// 返回 false 表示 CAS 未命中：可能是重复投递（已 Processing）或 reservation 已被回收。
	// 调用方据此决定是否继续执行 item。
	StartReservedItem(ctx context.Context, spaceID, exptID, exptRunID, itemID int64) (started bool, err error)

	// MGetDispatchObservations 供分钟级对账使用：批量读取指定 item 的 (status, 预占态)。
	// 对账据此与 Redis reservation 比对，识别四类漂移（Terminal 遗留、Processing 缺 reservation、
	// Queueing/reserved 超时未消费、Queueing/none 却有 reservation）。
	MGetDispatchObservations(ctx context.Context, spaceID, exptID, exptRunID int64, itemIDs []int64) ([]*ExptDispatchObservation, error)
}

// ExptDispatchRuntime 一个 run 的派发运行态。
type ExptDispatchRuntime struct {
	// OccupiedItemIDs 占用并发的 item（Processing ∪ Queueing/reserved）。
	// 已在 DB 层去重 —— 同一 item 不可能同时是 Processing 和 Queueing。
	OccupiedItemIDs []int64
	// CandidateItemIDs 可授予候选（Queueing/none），按 id 升序保证翻页稳定。
	CandidateItemIDs []int64
}

// OccupiedCount 当前并发占用数。
func (r *ExptDispatchRuntime) OccupiedCount() int {
	if r == nil {
		return 0
	}
	return len(r.OccupiedItemIDs)
}

// ExptDispatchObservation 单个 item 的派发投影观测，供对账使用。
type ExptDispatchObservation struct {
	ItemID                int64
	Status                int32
	QuotaReservationState entity.QuotaReservationState
}
