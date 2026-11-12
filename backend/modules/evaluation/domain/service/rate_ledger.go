// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// ReserveResult 一次批量预占的结果。Granted 为实际扣减成功的 item，
// Denied 为因容量不足未授予的 item（保持 Queueing，下一轮再竞争）。
type ReserveResult struct {
	Granted []*entity.Reservation
	Denied  []*entity.Reservation
}

// LedgerSnapshot 指定 env 的额度快照。
type LedgerSnapshot struct {
	Capacity         entity.ResourceVector
	Used             entity.ResourceVector
	Available        entity.ResourceVector
	ReservationCount int
}

// RateLedger 额度账本。
//
// 实现约束：
//   - Reserve 必须原子地校验并扣减所有维度，不允许出现“item 扣了、evaluator 没扣”；
//   - Release 必须幂等，重复释放不能重复返还；
//   - 所有 key 按 env 隔离；
//   - Rebuild 以 DB 为准重置容量与 reservation，是账本自修的入口。
type RateLedger interface {
	Reserve(ctx context.Context, ownerEnv string, capacity entity.ResourceVector, requests []*entity.Reservation) (*ReserveResult, error)
	Release(ctx context.Context, ownerEnv, reservationID string) (released bool, err error)
	ReleaseRun(ctx context.Context, ownerEnv string, exptRunID int64) (releasedCount int, err error)
	List(ctx context.Context, ownerEnv string) ([]*entity.Reservation, error)
	// Snapshot 读取额度快照；capacity 用于首次初始化 env 的容量 key。
	Snapshot(ctx context.Context, ownerEnv string, capacity entity.ResourceVector) (*LedgerSnapshot, error)
	SetState(ctx context.Context, ownerEnv, reservationID string, state entity.ReservationState) error
	Rebuild(ctx context.Context, ownerEnv string, capacity entity.ResourceVector, reservations []*entity.Reservation) error
}
