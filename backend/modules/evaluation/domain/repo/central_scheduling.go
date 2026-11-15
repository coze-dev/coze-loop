// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// ICentralSchedulingRepo 中心调度账本、run 状态与活跃集合的存储抽象。
//
// 实现必须保证：
//   - 所有 Redis key 以 qn:central: 开头，且带 Env/Lane 所有权隔离；
//   - Acquire 对多类资源是原子的（全有或全无）；
//   - Release 幂等，重复释放返回 false 而不是报错；
//   - 所有写入的 key 都设置 TTL，不执行 FLUSH，不改写其它前缀 key；
//   - scope 非法时 fail-closed，拒绝读写。
type ICentralSchedulingRepo interface {
	// Acquire 原子预占多资源。全部资源都满足时返回 true 并写入 lease；否则返回 false 且不产生任何副作用。
	Acquire(ctx context.Context, scope entity.CentralEnvScope, capacities map[entity.CentralResourceKind]int64, lease *entity.CentralLease, ttl time.Duration) (bool, error)
	// Release 释放一个 lease。lease 不存在时返回 false,nil（幂等）。
	Release(ctx context.Context, scope entity.CentralEnvScope, leaseID string) (bool, error)
	// ReleaseItems 按 (expt, run, item) 释放全部 attempt，返回释放数量；幂等。
	ReleaseItems(ctx context.Context, scope entity.CentralEnvScope, exptID, runID int64, itemIDs []int64) (int, error)
	// ReleaseRun 释放某个运行下所有仍存活的 lease，返回释放数量。
	ReleaseRun(ctx context.Context, scope entity.CentralEnvScope, exptID, runID int64) (int, error)
	// ReleaseExpt 释放某个实验下所有仍存活的 lease（删除/取消实验时使用）。
	ReleaseExpt(ctx context.Context, scope entity.CentralEnvScope, exptID int64) (int, error)

	// MarkDispatched reserved -> dispatched；重复确认幂等。
	MarkDispatched(ctx context.Context, scope entity.CentralEnvScope, leaseID string) (bool, error)
	// MarkPendingConfirm reserved -> pending_confirm，并登记 deadline。
	MarkPendingConfirm(ctx context.Context, scope entity.CentralEnvScope, leaseID string, deadline int64) (bool, error)
	// SweepPendingConfirms 释放到期仍未确认的预占并留下审计记录，返回释放数量。
	SweepPendingConfirms(ctx context.Context, scope entity.CentralEnvScope, now int64, auditTTL time.Duration) (int, error)
	// CountRunLeases 返回某运行当前的存活预占数（在途口径的唯一真值来源）。
	CountRunLeases(ctx context.Context, scope entity.CentralEnvScope, exptID, runID int64) (int, error)
	// GetLeaseState 返回单笔预占的当前唯一状态。
	GetLeaseState(ctx context.Context, scope entity.CentralEnvScope, leaseID string) (entity.CentralLeaseState, bool, error)

	// Usage 读取当前资源占用。
	Usage(ctx context.Context, scope entity.CentralEnvScope) (map[entity.CentralResourceKind]int64, error)
	// ListLeases 返回当前存活 lease。
	ListLeases(ctx context.Context, scope entity.CentralEnvScope) ([]*entity.CentralLease, error)
	// InspectLedger 执行增量对账（只读），返回可定位的漂移。
	InspectLedger(ctx context.Context, scope entity.CentralEnvScope) ([]*entity.CentralLedgerDrift, error)
	// RebuildUsage 以存活 lease 为准原子重算占用，返回修复前后占用、修复数量与逐笔漂移。
	RebuildUsage(ctx context.Context, scope entity.CentralEnvScope, capacities map[entity.CentralResourceKind]int64, ttl time.Duration) (before, after map[entity.CentralResourceKind]int64, repaired int, drift []*entity.CentralLedgerDrift, err error)

	// EnsureRunState 首次登记时冻结 mode/priority/seq/target_concurrency；已存在则只读返回。
	EnsureRunState(ctx context.Context, scope entity.CentralEnvScope, state *entity.CentralRunState, ttl time.Duration) (*entity.CentralRunState, bool, error)
	// GetRunState 读取冻结的 run 状态。
	GetRunState(ctx context.Context, scope entity.CentralEnvScope, exptID, runID int64) (*entity.CentralRunState, bool, error)
	// SwitchRunMode 消费审批后的一次性切换授权；同一实验只允许一次。
	SwitchRunMode(ctx context.Context, scope entity.CentralEnvScope, exptID, runID int64, toMode entity.CentralSchedulerMode, auth *entity.CentralModeSwitchAuthorization) (bool, error)

	// NextAttempt 返回 run 内某 item 单调递增的预占 attempt，保证每次预占身份唯一。
	NextAttempt(ctx context.Context, scope entity.CentralEnvScope, exptID, runID, itemID int64, ttl time.Duration) (int64, error)

	// RegisterPending 登记“待处理的初始化”，与 active 维护分离。
	RegisterPending(ctx context.Context, scope entity.CentralEnvScope, run *entity.CentralActiveRun, ttl time.Duration) error
	// RegisterActive 登记/刷新“已活跃的维护”集合。
	RegisterActive(ctx context.Context, scope entity.CentralEnvScope, run *entity.CentralActiveRun, ttl time.Duration) error
	// ActivateRun 把运行从 pending 原子迁移到 active。
	ActivateRun(ctx context.Context, scope entity.CentralEnvScope, exptID, runID int64, sessionUserID string, ttl time.Duration) error
	// UnregisterActive 注销活跃/待处理实验。
	UnregisterActive(ctx context.Context, scope entity.CentralEnvScope, exptID, runID int64) error
	// ListActive 按优先级 + 提交序号（score 升序）返回活跃实验。
	ListActive(ctx context.Context, scope entity.CentralEnvScope) ([]*entity.CentralActiveRun, error)
	// ListPending 按优先级 + 提交序号（score 升序）返回待处理实验。
	ListPending(ctx context.Context, scope entity.CentralEnvScope) ([]*entity.CentralActiveRun, error)
	// RemovePending 只移除 pending 登记，不影响 active 维护。
	RemovePending(ctx context.Context, scope entity.CentralEnvScope, exptID, runID int64) error
	// RescoreRun 只更新排序 score（防饿死抬升），不修改冻结的 base priority。
	RescoreRun(ctx context.Context, scope entity.CentralEnvScope, run *entity.CentralActiveRun, effectivePriority int, pending bool) error

	// RecordHealthyDegradedTick 记录一个连续正常 tick 计数。
	RecordHealthyDegradedTick(ctx context.Context, scope entity.CentralEnvScope) (int, error)
	// ResetHealthyDegradedTicks 清零连续正常 tick 计数。
	ResetHealthyDegradedTicks(ctx context.Context, scope entity.CentralEnvScope) error
	// MarkDegradedRecovered 标记降级已恢复（短 TTL，无正常 tick 自动失效）。
	MarkDegradedRecovered(ctx context.Context, scope entity.CentralEnvScope, ttl time.Duration) error
	// IsDegradedRecovered 返回降级恢复标记。
	IsDegradedRecovered(ctx context.Context, scope entity.CentralEnvScope) (bool, error)

	// AcquireLeader 以 SetNX+TTL 抢占中心 tick 的主实例锁，避免多实例重复派发。
	AcquireLeader(ctx context.Context, scope entity.CentralEnvScope, owner string, ttl time.Duration) (bool, error)
}
