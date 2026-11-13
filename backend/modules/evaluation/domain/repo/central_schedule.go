// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// IRunScheduleRepo run 级中心调度元数据的持久化投影。
//
// 全局扫描必须能按 owner_scope + 活跃状态 + priority 查询；schedule_mode /
// priority / owner_scope 都要真实落库，不能只放在内存。
//
//go:generate mockgen -destination ./mocks/central_schedule.go --package mocks . IRunScheduleRepo
type IRunScheduleRepo interface {
	Upsert(ctx context.Context, schedule *entity.ExptRunSchedule) error
	Get(ctx context.Context, spaceID, exptID, runID int64) (*entity.ExptRunSchedule, error)
	ScanActive(ctx context.Context, ownerScope string, limit int) ([]*entity.ExptRunSchedule, error)
	UpdateStatus(ctx context.Context, spaceID, exptID, runID int64, status int32, ledgerGeneration int64) error
}
