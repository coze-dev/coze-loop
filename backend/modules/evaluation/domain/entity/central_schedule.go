// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import "time"

// ExptRunSchedule run 级中心调度元数据（持久化投影）。
//
// 该表是“调度模式和优先级真实存下来、全局扫描能查到”的落点：
//   - schedule_mode 在实验创建时固定，run 创建时从实验复制；
//   - priority 同级按 created_at 等待时长排序；
//   - owner_scope 从创建一直覆盖到查询，缺失即拒绝；
//   - status 是中心调度自己的 active 生命周期，不依赖旧 MQ 心跳。
type ExptRunSchedule struct {
	ID                int64
	SpaceID           int64
	ExptID            int64
	RunID             int64
	ScheduleMode      int32
	Priority          int32
	OwnerScope        string
	TargetConcurrency int64
	Status            int32
	AdmissionToken    string
	LedgerGeneration  int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// 中心调度 run 状态。
const (
	ExptRunScheduleStatusPending  int32 = 1
	ExptRunScheduleStatusActive   int32 = 2
	ExptRunScheduleStatusDraining int32 = 3
	ExptRunScheduleStatusTerminal int32 = 4
)

// ActiveStatuses 全局扫描的活跃状态集合。
var ActiveStatuses = []int32{
	ExptRunScheduleStatusPending,
	ExptRunScheduleStatusActive,
	ExptRunScheduleStatusDraining,
}
