// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/gorm_gen/model"
)

func TaskDO2PO(task *entity.ObservabilityTask) *model.ObservabilityTask {
	return &model.ObservabilityTask{
		ID:                    task.ID,
		WorkspaceID:           task.WorkspaceID,
		Name:                  task.Name,
		Description:           task.Description,
		TaskType:              task.TaskType,
		TaskStatus:            task.TaskStatus,
		TaskDetail:            task.TaskDetail,
		SpanFilter:            task.SpanFilter,
		EffectiveTime:         task.EffectiveTime,
		BackfillEffectiveTime: task.BackfillEffectiveTime,
		Sampler:               task.Sampler,
		TaskConfig:            task.TaskConfig,
		CreatedAt:             task.CreatedAt,
		UpdatedAt:             task.UpdatedAt,
		CreatedBy:             task.CreatedBy,
		UpdatedBy:             task.UpdatedBy,
	}
}

func TaskPO2DO(task *model.ObservabilityTask) *entity.ObservabilityTask {
	return &entity.ObservabilityTask{
		ID:                    task.ID,
		WorkspaceID:           task.WorkspaceID,
		Name:                  task.Name,
		Description:           task.Description,
		TaskType:              task.TaskType,
		TaskStatus:            task.TaskStatus,
		TaskDetail:            task.TaskDetail,
		SpanFilter:            task.SpanFilter,
		EffectiveTime:         task.EffectiveTime,
		BackfillEffectiveTime: task.BackfillEffectiveTime,
		Sampler:               task.Sampler,
		TaskConfig:            task.TaskConfig,
		CreatedAt:             task.CreatedAt,
		UpdatedAt:             task.UpdatedAt,
		CreatedBy:             task.CreatedBy,
		UpdatedBy:             task.UpdatedBy,
	}
}

func TaskRunDO2PO(taskRun *entity.TaskRun) *model.ObservabilityTaskRun {
	return &model.ObservabilityTaskRun{
		ID:             taskRun.ID,
		TaskID:         taskRun.TaskID,
		WorkspaceID:    taskRun.WorkspaceID,
		TaskType:       taskRun.TaskType,
		RunStatus:      taskRun.RunStatus,
		RunDetail:      taskRun.RunDetail,
		BackfillDetail: taskRun.BackfillDetail,
		RunStartAt:     taskRun.RunStartAt,
		RunEndAt:       taskRun.RunEndAt,
		RunConfig:      taskRun.RunConfig,
		CreatedAt:      taskRun.CreatedAt,
		UpdatedAt:      taskRun.UpdatedAt,
	}
}

func TaskRunPO2DO(taskRun *model.ObservabilityTaskRun) *entity.TaskRun {
	return &entity.TaskRun{
		ID:             taskRun.ID,
		TaskID:         taskRun.TaskID,
		WorkspaceID:    taskRun.WorkspaceID,
		TaskType:       taskRun.TaskType,
		RunStatus:      taskRun.RunStatus,
		RunDetail:      taskRun.RunDetail,
		BackfillDetail: taskRun.BackfillDetail,
		RunStartAt:     taskRun.RunStartAt,
		RunEndAt:       taskRun.RunEndAt,
		RunConfig:      taskRun.RunConfig,
		CreatedAt:      taskRun.CreatedAt,
		UpdatedAt:      taskRun.UpdatedAt,
	}
}

func TaskRunsPO2DO(taskRun []*model.ObservabilityTaskRun) []*entity.TaskRun {
	if taskRun == nil {
		return nil
	}
	resp := make([]*entity.TaskRun, len(taskRun))
	for i, tr := range taskRun {
		resp[i] = TaskRunPO2DO(tr)
	}
	return resp
}
