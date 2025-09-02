// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/gorm_gen/model"
)

func TaskDO2PO(task *entity.ObservabilityTask) *model.ObservabilityTask {
	return &model.ObservabilityTask{
		ID:            task.ID,
		WorkspaceID:   task.WorkspaceID,
		Name:          task.Name,
		Description:   task.Description,
		TaskType:      task.TaskType,
		TaskStatus:    task.TaskStatus,
		TaskDetail:    task.TaskDetail,
		SpanFilter:    task.SpanFilter,
		EffectiveTime: task.EffectiveTime,
		Sampler:       task.Sampler,
		TaskConfig:    task.TaskConfig,
		CreatedAt:     task.CreatedAt,
		UpdatedAt:     task.UpdatedAt,
		CreatedBy:     task.CreatedBy,
		UpdatedBy:     task.UpdatedBy,
	}
}

func TaskPO2DO(task *model.ObservabilityTask) *entity.ObservabilityTask {
	return &entity.ObservabilityTask{
		ID:            task.ID,
		WorkspaceID:   task.WorkspaceID,
		Name:          task.Name,
		Description:   task.Description,
		TaskType:      task.TaskType,
		TaskStatus:    task.TaskStatus,
		TaskDetail:    task.TaskDetail,
		SpanFilter:    task.SpanFilter,
		EffectiveTime: task.EffectiveTime,
		Sampler:       task.Sampler,
		TaskConfig:    task.TaskConfig,
		CreatedAt:     task.CreatedAt,
		UpdatedAt:     task.UpdatedAt,
		CreatedBy:     task.CreatedBy,
		UpdatedBy:     task.UpdatedBy,
	}
}
