// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package central_schedule

import (
	"context"
	"time"
)

// ReservationLedger 额度账本（权威）。实现必须保证：
//   - ReserveBatch 对一批 item 的多维度额度全有或全无；
//   - 状态迁移 CAS 且幂等；
//   - Release 重复调用 no-op；
//   - Rebuild 使用影子代切换，不双算、不长时间阻塞派发。
type ReservationLedger interface {
	InitRun(ctx context.Context, account *RunAccount) error
	ReserveBatch(ctx context.Context, account *RunAccount, reservations []*Reservation) error
	MarkDispatching(ctx context.Context, reservation *Reservation) error
	Ack(ctx context.Context, reservationID string, at UpdateTime) (AckResult, error)
	MarkUncertain(ctx context.Context, reservationID string, at UpdateTime) error
	Release(ctx context.Context, reservationID string, at UpdateTime) (ReleaseResult, error)
	Get(ctx context.Context, reservationID string) (*Reservation, error)
	Usage(ctx context.Context, runID int64) (map[string]int64, error)
	ListRunReservations(ctx context.Context, runID int64, state ReservationState) ([]*Reservation, error)
	Claim(ctx context.Context, runID, itemID int64, attempt int32, reservationID string, ttlSec int64) (bool, error)
	// ResolveReservation 执行侧只拿到 run/item/attempt 时反查本次派发的
	// reservation_id；没有派发凭证（legacy）时返回 ErrReservationAbsent。
	ResolveReservation(ctx context.Context, runID, itemID int64, attempt int32) (string, error)
	Rebuild(ctx context.Context, account *RunAccount, expected map[string]int64, live []*Reservation) (RebuildResult, error)
}

// UpdateTime 便于用固定时钟做离线测试。
type UpdateTime struct {
	Now time.Time
}

// AckResult 执行侧确认结果。
type AckResult int

const (
	AckMissing   AckResult = 0 // 账本无此预占：不执行、不扣
	AckConfirmed AckResult = 1 // 首次确认：可执行
	AckDuplicate AckResult = 2 // 重复确认：跳过执行
	AckLate      AckResult = 3 // release 后迟到：跳过执行
)

// ReleaseResult 归还结果。
type ReleaseResult int

const (
	ReleaseMissing ReleaseResult = 0
	ReleaseDone    ReleaseResult = 1
	ReleaseAlready ReleaseResult = 2 // 幂等重复归还
)

// RebuildResult 重建结果。
type RebuildResult struct {
	Swapped    bool
	Generation int64
}

// AdmissionGuard 旧 200 运行实验准入。两种模式都必须在派发前通过。
type AdmissionGuard interface {
	Check(ctx context.Context, run *RunView) error
}

// ActiveRunSource 全局扫描活跃 run；实现不得只返回 legacy run。
type ActiveRunSource interface {
	ScanActive(ctx context.Context, ownerScope string, limit int) ([]*RunView, error)
}

// ItemSource item 状态来源（DB 为对账权威）。
type ItemSource interface {
	ScanQueueing(ctx context.Context, run *RunView, limit int) ([]*ItemView, error)
	GetItemStatus(ctx context.Context, runID, itemID int64) (ItemStatus, error)
	// ListProcessing 用于全量重建：DB 已 Processing 但账本可能缺失的 item。
	ListProcessing(ctx context.Context, run *RunView) ([]*ItemView, error)
}

// Dispatcher 派发 item；必须携带 reservation_id 供执行侧确认/去重。
type Dispatcher interface {
	Publish(ctx context.Context, item *ItemView, reservationID, dedupKey string) error
}

// ProjectionWriter MySQL 投影。失败不阻断账本，由 Reconciler 修复。
type ProjectionWriter interface {
	MarkProcessing(ctx context.Context, item *ItemView, reservationID string) error
	MarkReleased(ctx context.Context, runID, itemID int64, reservationID string) error
}

// Leader 单实例生效的 scope 级租约。
type Leader interface {
	Acquire(ctx context.Context, ownerScope string, instanceID string, ttl time.Duration) (bool, error)
}

// Clock 可替换时钟，超时/补偿测试用。
type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// NewRealClock 返回系统时钟。
func NewRealClock() Clock { return realClock{} }
