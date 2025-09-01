// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"time"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

type ObservabilityTask struct {
	ID            int64     // Task ID
	WorkspaceID   int64     // 空间ID
	Name          string    // 任务名称
	Description   *string   // 任务描述
	TaskType      string    // 任务类型
	TaskStatus    string    // 任务状态
	TaskDetail    *string   // 任务运行状态详情
	SpanFilter    *string   // span 过滤条件
	EffectiveTime *string   // 生效时间
	Sampler       *string   // 采样器
	TaskConfig    *string   // 相关任务的配置信息
	CreatedAt     time.Time // 创建时间
	UpdatedAt     time.Time // 更新时间
	CreatedBy     string    // 创建人
	UpdatedBy     string    // 更新人
}
type SpanFilter struct {
	Filters      loop_span.FilterFields `json:"filters,omitempty"`
	PlatformType common.PlatformType    `json:"platform_type,omitempty"`
	SpanListType common.SpanListType    `json:"span_list_type,omitempty"`
}
