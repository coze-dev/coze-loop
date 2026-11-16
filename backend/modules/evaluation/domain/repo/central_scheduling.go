// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"errors"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// ErrDefiniteDispatchFailure 表示派发在发出消息前确定失败，可立即释放预占；
// 未包装该错误的 dispatch error 视为结果不明，交给待补偿与对账。
var ErrDefiniteDispatchFailure = errors.New("central dispatch definite failure")

// ErrReservationExists 表示该 item 已有非终态预占，本轮不应重复预占/派发。
var ErrReservationExists = errors.New("central reservation already exists")

// ReserveRequest 一次原子预占请求。所有启用维度全有或全无。
type ReserveRequest struct {
	Reservation *entity.Reservation
	Enabled     map[entity.ResourceDimension]bool
	Capacity    map[entity.ResourceDimension]int64
	TTL         time.Duration
}

// ICentralLedgerRepo 中心调度账本（Redis 实现）。预占记录是额度占用的唯一真值。
type ICentralLedgerRepo interface {
	Reserve(ctx context.Context, req *ReserveRequest) (*entity.Reservation, error)
	Adopt(ctx context.Context, resv *entity.Reservation, ttl time.Duration) error
	Confirm(ctx context.Context, env, resvID string) (entity.ReservationState, error)
	MarkPendingCompensation(ctx context.Context, env, resvID string) (entity.ReservationState, error)
	Release(ctx context.Context, env, resvID, reason string) (entity.ReservationState, error)
	Expire(ctx context.Context, env, resvID, reason string) (entity.ReservationState, error)
	Get(ctx context.Context, env, resvID string) (*entity.Reservation, error)
	ListByRun(ctx context.Context, env string, exptID, runID int64) ([]*entity.Reservation, error)
	ListAll(ctx context.Context, env string, limit int) ([]*entity.Reservation, error)
	RunInflight(ctx context.Context, env string, exptID, runID int64) (int, error)
	PoolUsage(ctx context.Context, env string) (entity.ResourceVector, error)
	RebuildPool(ctx context.Context, env string, used entity.ResourceVector, ttl time.Duration) error
	CleanupEnv(ctx context.Context, env string) error
}

// ICentralActiveRepo 活跃集与租约。
type ICentralActiveRepo interface {
	AddActive(ctx context.Context, env string, a *entity.ActiveExperiment, ttl time.Duration) error
	RemoveActive(ctx context.Context, env string, exptID, runID int64) error
	ListActive(ctx context.Context, env string, limit int) ([]*entity.ActiveExperiment, error)
	TryLeader(ctx context.Context, env, owner string, ttl time.Duration) (bool, error)
	RenewLeader(ctx context.Context, env, owner string, ttl time.Duration) (bool, error)
	ReleaseLeader(ctx context.Context, env, owner string) error
	TryExptLease(ctx context.Context, env string, exptID int64, owner string, ttl time.Duration) (bool, error)
	CleanupEnv(ctx context.Context, env string) error
}

// ICentralExperimentRepo 中心调度需要的实验/item 投影读写。DB 状态只是投影，不作为额度账本。
type ICentralExperimentRepo interface {
	GetByID(ctx context.Context, exptID, spaceID int64) (*entity.Experiment, error)
	MGetBasicByID(ctx context.Context, exptIDs []int64) ([]*entity.Experiment, error)
	ListCentralActive(ctx context.Context, env string, limit int) ([]*entity.Experiment, error)
	ScanQueueingItems(ctx context.Context, exptID, runID, spaceID int64, limit int) ([]int64, error)
	ScanProcessingItems(ctx context.Context, exptID, runID, spaceID int64, limit int) ([]int64, error)
	MarkItemsProcessing(ctx context.Context, exptID, runID, spaceID int64, itemIDs []int64) error
	MarkItemsQueueing(ctx context.Context, exptID, runID, spaceID int64, itemIDs []int64) error
	GetItemStates(ctx context.Context, exptID, runID, spaceID int64, itemIDs []int64) (map[int64]entity.ItemRunState, error)
	EnsureRunItems(ctx context.Context, expt *entity.Experiment) error
}

// ICentralItemDispatcher 只负责把 item eval 事件发到下游；中心调度不直接执行 item。
type ICentralItemDispatcher interface {
	Dispatch(ctx context.Context, event *entity.ExptItemEvalEvent) error
}
