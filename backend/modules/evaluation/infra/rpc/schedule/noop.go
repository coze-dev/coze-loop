// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package schedule

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
)

// noopExptScheduleAdapter 在尚未接入底层调度平台时占位，保证依赖注入图可构建。
type noopExptScheduleAdapter struct{}

// NewNoopExptScheduleAdapter 返回不做实际调度注册的空实现。
func NewNoopExptScheduleAdapter() rpc.IExptScheduleAdapter {
	return &noopExptScheduleAdapter{}
}

func (n *noopExptScheduleAdapter) CreatePeriodicJob(ctx context.Context, param *rpc.CreatePeriodicJobParam) error {
	return nil
}

func (n *noopExptScheduleAdapter) CloseJob(ctx context.Context, bizKey string) error {
	return nil
}

func (n *noopExptScheduleAdapter) GetJob(ctx context.Context, bizKey string) (*rpc.ScheduleJobDetail, error) {
	return nil, nil
}
