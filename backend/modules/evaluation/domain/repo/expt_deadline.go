// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"time"
)

// IExptDeadlineStore 实验到点调度 Redis ZSET：只存 (spaceID, exptID, runID, deadlineTs)，派发时从 DB 组事件
type IExptDeadlineStore interface {
	AddDeadline(ctx context.Context, spaceID, exptID, runID int64, deadlineTs int64) error
	ScanDue(ctx context.Context, nowUnix int64, limit int) (members []string, err error)
	// TryClaim 分布式抢占：仅一个实例能抢到，用于多实例派发去重。ttl 内未完成则锁过期可被其他实例重试。
	TryClaim(ctx context.Context, member string, ttl time.Duration) (ok bool, err error)
	Remove(ctx context.Context, member string) error
}
