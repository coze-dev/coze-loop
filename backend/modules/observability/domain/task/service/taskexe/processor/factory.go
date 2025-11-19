// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package processor

import (
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe"
)

type TaskProcessor struct {
	taskProcessorMap map[entity.TaskType]taskexe.Processor
}

func NewTaskProcessor() *TaskProcessor {
	return &TaskProcessor{}
}

func (t *TaskProcessor) Register(taskType entity.TaskType, taskProcessor taskexe.Processor) {
	if t.taskProcessorMap == nil {
		t.taskProcessorMap = make(map[entity.TaskType]taskexe.Processor)
	}
	t.taskProcessorMap[taskType] = taskProcessor
}

func (t *TaskProcessor) GetTaskProcessor(taskType entity.TaskType) taskexe.Processor {
	datasetProvider, ok := t.taskProcessorMap[taskType]
	if !ok {
		return NewNoopTaskProcessor()
	}
	return datasetProvider
}
