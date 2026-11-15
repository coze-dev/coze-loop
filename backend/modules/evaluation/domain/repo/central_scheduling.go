// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// ICentralSchedulingRepo 中心调度账本与活跃集合的存储抽象。
//
// 实现必须保证：
//   - 所有 Redis key 以 qn:central: 开头，且带 Env/Lane 所有权隔离；
//   - Acquire 对多类资源是原子的（全有或全无）；
//   - Release 幂等，重复释放返回 false 而不是报错；
//   - 所有写入的 key 都设置 TTL，不执行 FLUSH，不改写其它前缀 key。
type ICentralSchedulingRepo interface {
	// Acquire 原子预占多资源。全部资源都满足时返回 true 并写入 lease；否则返回 false 且不产生任何副作用。
	Acquire(ctx context.Context, scope entity.CentralEnvScope, capacities map[entity.CentralResourceKind]int64, lease *entity.CentralLease, ttl time.Duration) (bool, error)
	// Release 释放一个 lease。lease 不存在时返回 false,nil（幂等）。
	Release(ctx context.Context, scope entity.CentralEnvScope, leaseID string) (bool, error)
	// Usage 读取当前资源占用。
	Usage(ctx context.Context, scope entity.CentralEnvScope) (map[entity.CentralResourceKind]int64, error)
	// RebuildUsage 以存活 lease 为准重算占用并修写 usage，返回修复前后的占用与修复的 lease 数量。
	RebuildUsage(ctx context.Context, scope entity.CentralEnvScope, capacities map[entity.CentralResourceKind]int64, ttl time.Duration) (before, after map[entity.CentralResourceKind]int64, repaired int, err error)
	// ListLeases 返回当前存活 lease。
	ListLeases(ctx context.Context, scope entity.CentralEnvScope) ([]*entity.CentralLease, error)

	// RegisterActive 将运行登记为活跃实验（zset score=priority，member=exptID:runID）。
	RegisterActive(ctx context.Context, scope entity.CentralEnvScope, run *entity.CentralActiveRun, ttl time.Duration) error
	// UnregisterActive 注销活跃实验。
	UnregisterActive(ctx context.Context, scope entity.CentralEnvScope, exptID, runID int64) error
	// ListActive 按优先级（score 升序）返回活跃实验。
	ListActive(ctx context.Context, scope entity.CentralEnvScope) ([]*entity.CentralActiveRun, error)

	// AcquireLeader 以 SetNX+TTL 抢占中心 tick 的主实例锁，避免多实例重复派发。
	AcquireLeader(ctx context.Context, scope entity.CentralEnvScope, owner string, ttl time.Duration) (bool, error)

	// ReleaseRun 释放某个运行下所有仍存活的 lease（取消/终态/重试收敛时使用），返回释放数量。
	ReleaseRun(ctx context.Context, scope entity.CentralEnvScope, exptID, runID int64) (int, error)
}
