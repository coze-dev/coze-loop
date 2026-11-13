// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

// 该 model 由中心调度需求手工补充；字段与 DDL 保持一致，避免依赖未提供的生成链。
package model

import "time"

const TableNameExptRunSchedule = "expt_run_schedule"

// ExptRunSchedule 中心调度 run 级元数据投影。
type ExptRunSchedule struct {
	ID                int64     `gorm:"column:id;type:bigint(20) unsigned;primaryKey;autoIncrement;comment:id" json:"id"`
	SpaceID           int64     `gorm:"column:space_id;type:bigint(20) unsigned;not null;uniqueIndex:uk_space_expt_run,priority:1;index:idx_scope_status_priority,priority:1;comment:空间 id" json:"space_id"`
	ExptID            int64     `gorm:"column:expt_id;type:bigint(20) unsigned;not null;uniqueIndex:uk_space_expt_run,priority:2;comment:实验 id" json:"expt_id"`
	RunID             int64     `gorm:"column:run_id;type:bigint(20) unsigned;not null;uniqueIndex:uk_space_expt_run,priority:3;comment:运行 id" json:"run_id"`
	ScheduleMode      int32     `gorm:"column:schedule_mode;type:int(11) unsigned;not null;default:0;comment:调度模式 0=legacy 1=central" json:"schedule_mode"`
	Priority          int32     `gorm:"column:priority;type:int(11);not null;default:0;comment:优先级，越大越优先" json:"priority"`
	OwnerScope        string    `gorm:"column:owner_scope;type:varchar(255) character set utf8mb4;not null;default:'';index:idx_scope_status_priority,priority:2;comment:所有权隔离键 env+lane+region+psm" json:"owner_scope"`
	TargetConcurrency int64     `gorm:"column:target_concurrency;type:bigint(20);not null;default:0;comment:目标 item 并发" json:"target_concurrency"`
	Status            int32     `gorm:"column:status;type:int(11) unsigned;not null;default:1;index:idx_scope_status_priority,priority:3;comment:1=pending 2=active 3=draining 4=terminal" json:"status"`
	AdmissionToken    string    `gorm:"column:admission_token;type:varchar(128);not null;default:'';comment:旧 200 准入 token" json:"admission_token"`
	LedgerGeneration  int64     `gorm:"column:ledger_generation;type:bigint(20);not null;default:0;comment:账本代次投影" json:"ledger_generation"`
	CreatedAt         time.Time `gorm:"column:created_at;type:timestamp;not null;default:CURRENT_TIMESTAMP;comment:创建时间" json:"created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at;type:timestamp;not null;default:CURRENT_TIMESTAMP;comment:更新时间" json:"updated_at"`
}

func (*ExptRunSchedule) TableName() string { return TableNameExptRunSchedule }
