// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package component

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// IEnvironmentProvider 返回当前进程所属环境（线上/BOE/PPE 泳道）。
// 实现必须只依赖进程环境，不得用共享 TCC 作为跨环境通道；未知时返回 error 让中心调度 fail-closed。
type IEnvironmentProvider interface {
	CurrentEnvID(ctx context.Context) (string, error)
}

// ICentralSchedulingConfigProvider 中心调度配置。任一关键容量不可判定时应返回 error，调用方停止新派发。
type ICentralSchedulingConfigProvider interface {
	GetCentralSchedulingConfig(ctx context.Context) (*entity.CentralSchedulingConfig, error)
	// GetDimensionCapacity 返回某维度是否启用与容量；enabled=false 表示该维度停用。
	GetDimensionCapacity(ctx context.Context, dim entity.ResourceDimension) (enabled bool, capacity int64, err error)
	// IsAuthorized 校验运行中模式切换/创建 central 的显式授权。
	IsAuthorized(ctx context.Context, operator string, spaceID int64) bool
}
