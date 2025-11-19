// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package scheduledtask

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/llm/pkg/goroutineutil"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type ScheduledTask interface {
	Run() error
	RunOnce(ctx context.Context) error
	Stop() error
}

type BaseScheduledTask struct {
	name         string
	timeInterval time.Duration
	stopChan     chan struct{}
}

func NewBaseScheduledTask(name string, timeInterval time.Duration) BaseScheduledTask {
	return BaseScheduledTask{
		name:         name,
		timeInterval: timeInterval,
		stopChan:     make(chan struct{}),
	}
}

func (b *BaseScheduledTask) Run() error {
	ticker := time.NewTicker(b.timeInterval)
	goroutineutil.GoWithDefaultRecovery(context.Background(), func() {
		for {
			select {
			case <-ticker.C:
				ctx := context.Background()
				startTime := time.Now()
				if err := b.RunOnce(ctx); err != nil {
					duration := time.Since(startTime)
					logs.CtxError(ctx, "ScheduledTask [%s] run error: %v, cost: %v", b.name, err, duration)
				} else {
					duration := time.Since(startTime)
					logs.CtxInfo(ctx, "ScheduledTask [%s] run success, cost: %v", b.name, duration)
				}
			case <-b.stopChan:
				return
			}
		}
	})
	return nil
}

func (b *BaseScheduledTask) RunOnce(ctx context.Context) error {
	panic("implement me")
}

func (b *BaseScheduledTask) Stop() error {
	close(b.stopChan)
	return nil
}
