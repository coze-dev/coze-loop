// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import "time"

// SchedulerStatus 定时器状态
type SchedulerStatus string

const (
	// SchedulerStatusEnable 启用状态
	SchedulerStatusEnable SchedulerStatus = "enable"
	// SchedulerStatusDisable 禁用状态
	SchedulerStatusDisable SchedulerStatus = "disable"
)

// ExptScheduler 实验模板定时器领域实体
//
// 与底层调度平台无关；service 层负责把该实体与 rpc.IExptScheduleAdapter 串起来：
// 启用时根据 Frequency/TriggerAt 推导 crontab 并调 CreatePeriodicJob，
// 禁用/删除时调 CloseJob，详情拉取时调 GetJob。
//
// 与 pipeline.go 中的 Scheduler 值对象（pipeline 配置片段）不同，本实体是独立持久化的领域对象。
type ExptScheduler struct {
	ID     int64
	Status SchedulerStatus
	// Frequency 触发频率，取值与 ExptSchedulerDO.Frequency 保持一致：
	//   FrequencyEveryDay / FrequencyMonday / ... / FrequencySunday
	Frequency string
	// TriggerAt 一天内的触发时刻，仅使用 Hour/Minute，日期部分忽略
	TriggerAt *time.Time
	// StartedAt 任务首次执行时间
	StartedAt *time.Time
	// EndedAt 任务截止时间
	EndedAt *time.Time
	// SchedulerBizKey 调用 IExptScheduleAdapter.CreatePeriodicJob 时使用的 BizKey
	SchedulerBizKey string

	SpaceID   int64
	CreatedBy string
	CreatedAt time.Time
	UpdatedBy string
	UpdatedAt time.Time
}

// Frequency 取值
const (
	FrequencyEveryDay  = "every_day"
	FrequencyMonday    = "monday"
	FrequencyTuesday   = "tuesday"
	FrequencyWednesday = "wednesday"
	FrequencyThursday  = "thursday"
	FrequencyFriday    = "friday"
	FrequencySaturday  = "saturday"
	FrequencySunday    = "sunday"
)

// IsEnabled 是否处于启用状态
func (s *ExptScheduler) IsEnabled() bool {
	return s != nil && s.Status == SchedulerStatusEnable
}

// GetHour 触发时间的小时部分
func (s *ExptScheduler) GetHour() int {
	if s == nil || s.TriggerAt == nil {
		return 0
	}
	return s.TriggerAt.Hour()
}

// GetMinute 触发时间的分钟部分
func (s *ExptScheduler) GetMinute() int {
	if s == nil || s.TriggerAt == nil {
		return 0
	}
	return s.TriggerAt.Minute()
}
