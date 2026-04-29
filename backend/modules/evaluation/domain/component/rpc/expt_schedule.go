// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"
	"time"
)

// IExptScheduleAdapter 实验模块周期调度任务适配器（通用）
//
// 该接口屏蔽底层调度平台（如 ByteScheduler）的差异，仅提供注册/关闭/查询周期任务的能力，
// 不感知任何业务语义。BizKey 命名、调度时机、回调方法名、回调 payload 等均由调用方决定，
// 业务侧（如"基于实验模板创建实验"）的逻辑应放在 backend evaluation 模块的 service 层实现。
//
//go:generate mockgen -destination=mocks/expt_schedule.go -package=mocks . IExptScheduleAdapter
type IExptScheduleAdapter interface {
	// CreatePeriodicJob 创建/更新周期任务
	// 同 BizKey 重复调用支持 upsert 语义（若任务已关闭，可用相同 BizKey 重新创建）
	CreatePeriodicJob(ctx context.Context, param *CreatePeriodicJobParam) error

	// CloseJob 关闭周期任务
	// 任务已关闭或不存在时返回 nil，保持幂等
	CloseJob(ctx context.Context, bizKey string) error

	// GetJob 查询任务详情
	// 任务不存在时返回 (nil, nil)
	GetJob(ctx context.Context, bizKey string) (*ScheduleJobDetail, error)
}

// CreatePeriodicJobParam 创建周期任务的参数
type CreatePeriodicJobParam struct {
	// BizKey 业务唯一标识；命名规则由调用方决定，同值重复调用为 upsert
	BizKey string
	// Crontab 标准 5 段 crontab 表达式（minute hour day-of-month month day-of-week）；
	// 实现方负责将其转换为底层调度平台所需格式
	Crontab string
	// StartedAt 首次执行时间；nil 表示按 Crontab 自然触发
	StartedAt *time.Time
	// EndedAt 截止时间；nil 时由实现方使用默认值（不超过实现方约定的最大有效期）
	EndedAt *time.Time
	// CallbackMethod 触发时回调的方法名（语义由实现方解释，如 RPC 方法名）
	CallbackMethod string
	// CallbackPayload 触发时回调的请求体（已由调用方序列化）
	CallbackPayload string
}

// ScheduleJobDetail 周期任务详情
type ScheduleJobDetail struct {
	BizKey     string
	Enabled    bool
	Crontab    string
	NextRunAt  *time.Time
	FirstRunAt *time.Time
	Deadline   *time.Time
}
