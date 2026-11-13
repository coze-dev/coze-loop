// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

// Package schedule 组装中心调度主链：tick 载体、账本、适配器。
package schedule

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/google/wire"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/central_schedule"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	ledger "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/redis/central_schedule"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

var CentralScheduleSet = wire.NewSet(
	service.NewCentralScheduleAdapter,
	NewScheduler,
)

func NewScheduler(
	adapter *service.CentralScheduleAdapter,
	l *ledger.Ledger,
	configer component.IConfiger,
) *central_schedule.Scheduler {
	hostname, _ := os.Hostname()
	return &central_schedule.Scheduler{
		Leader:     l,
		Runs:       adapter,
		Admission:  adapter,
		Items:      adapter,
		Ledger:     l,
		Dispatcher: adapter,
		Projection: adapter,
		InstanceID: fmt.Sprintf("%s-%d", hostname, os.Getpid()),
		ConfigProvider: func(ctx context.Context) central_schedule.Config {
			conf := configer.GetExptExecConf(ctx, 0).GetCentralScheduleConf()
			if conf == nil {
				return central_schedule.Config{Disabled: true}
			}
			cfg := central_schedule.Config{
				Disabled:          !conf.Enabled,
				OwnerScope:        conf.OwnerScope(),
				AckTimeout:        time.Duration(conf.AckTimeoutSecond) * time.Second,
				CompensationLimit: conf.CompensationLimit,
				BatchLimit:        conf.BatchLimit,
				RunLimit:          conf.RunLimit,
				TickInterval:      time.Duration(conf.TickSecond) * time.Second,
			}
			if conf.Enabled && cfg.OwnerScope == "" {
				logs.CtxError(ctx, "[CentralSchedule] enabled but owner scope incomplete, dispatch rejected")
			}
			return cfg
		},
	}
}
