// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package schedule

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/central_schedule"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// StartTicker 启动不依赖旧 per-experiment MQ 的分钟级 tick 载体。
//
// 单实例生效由 Scheduler 内的 scope 级 leader lease 保证：多实例同时跑
// 也只有一个实例在这一分钟派发；其余实例直接跳过。
func StartTicker(ctx context.Context, s *central_schedule.Scheduler) {
	if s == nil {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logs.CtxError(ctx, "[CentralSchedule] ticker panic: %v", r)
			}
		}()
		timer := time.NewTimer(0)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}
			stats, err := s.Tick(ctx)
			if err != nil {
				logs.CtxWarn(ctx, "[CentralSchedule] tick fail: %v, stats: %+v", err, stats)
			} else if stats.Dispatched > 0 || stats.Errors > 0 {
				logs.CtxInfo(ctx, "[CentralSchedule] tick stats: %+v", stats)
			}
			timer.Reset(s.Interval(ctx))
		}
	}()
}
