// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/gorm_gen/model"
)

func TaskDO2PO(task *entity.ObservabilityTask) *model.ObservabilityTask {
	return &model.ObservabilityTask{
		ID:            task.ID,
		WorkspaceID:   task.WorkspaceID,
		Name:          task.Name,
		Description:   task.Description,
		TaskType:      "",
		TaskStatus:    "",
		TaskDetail:    nil,
		SpanFilter:    nil,
		EffectiveTime: nil,
		Sampler:       nil,
		TaskConfig:    nil,
		CreatedAt:     time.Time{},
		UpdatedAt:     time.Time{},
		CreatedBy:     "",
		UpdatedBy:     "",
	}
}

func TaskPO2DO(task *model.ObservabilityTask) *entity.ObservabilityTask {
	return &entity.ObservabilityTask{
		ID:            0,
		WorkspaceID:   0,
		Name:          "",
		Description:   nil,
		TaskType:      "",
		TaskStatus:    "",
		TaskDetail:    nil,
		SpanFilter:    nil,
		EffectiveTime: nil,
		Sampler:       nil,
		TaskConfig:    nil,
		CreatedAt:     time.Time{},
		UpdatedAt:     time.Time{},
		CreatedBy:     "",
		UpdatedBy:     "",
	}
}
