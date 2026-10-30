// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate mockgen -destination=mocks/expt_item_dispatch.go -package=mocks . IExptItemDispatchRepo

// IExptItemDispatchRepo reads primary dispatch observations and applies each
// item transition in a canonical-first transaction. Expected request IDs always
// come from the original grant/event; an empty ID only matches an empty source.
type IExptItemDispatchRepo interface {
	ClaimQuotaReserved(ctx context.Context, spaceID, exptID, exptRunID int64, itemIDs []int64, itemRequestIDs map[int64]string) ([]ClaimedDispatchItem, error)
	ResetQuotaReserved(ctx context.Context, spaceID, exptID, exptRunID int64, itemIDs []int64, itemRequestIDs map[int64]string) ([]int64, error)
	LoadDispatchRuntime(ctx context.Context, spaceID, exptID, exptRunID int64, candidateLimit int) (*ExptDispatchRuntime, error)
	// CurrentRequest excludes mismatched/terminal sources. An already Processing
	// source is current but not Started, allowing the existing recovery path.
	StartReservedItem(ctx context.Context, spaceID, exptID, exptRunID, itemID int64, expectedRequestID string) (StartReservedItemResult, error)
	RequeueProcessingItem(ctx context.Context, spaceID, exptID, exptRunID, itemID int64, expectedRequestID string) (bool, error)
	MGetDispatchObservations(ctx context.Context, spaceID, exptID, exptRunID int64, itemIDs []int64) ([]*ExptDispatchObservation, error)
}

// ClaimedDispatchItem is captured in the successful Claim transaction. Callers
// must publish this payload without replacing its source with a later read.
type ClaimedDispatchItem struct {
	ItemID     int64
	RequestID  string
	Mode       entity.ExptRunMode
	RetryTimes int32
}

type StartReservedItemResult struct {
	CurrentRequest bool
	Started        bool
}

// ExptDispatchRuntime 一个 run 的派发运行态。
type ExptDispatchRuntime struct {
	ItemRequestIDs map[int64]string
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
	RequestID             string
	ItemID                int64
	Status                int32
	QuotaReservationState entity.QuotaReservationState
}
