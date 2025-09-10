// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package processor

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/taskexe"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

var _ taskexe.Processor = (*DataReflowProcessor)(nil)

type DataReflowProcessor struct {
	datasetServiceAdaptor *service.DatasetServiceAdaptor
	TaskRepo              repo.ITaskRepo
}

func newDataReflowProcessor() *DataReflowProcessor {
	return &DataReflowProcessor{}
}

func (p *DataReflowProcessor) ValidateConfig(ctx context.Context, config any, workspaceID int64) error {

	return nil
}

func (p *DataReflowProcessor) Invoke(ctx context.Context, config any, trigger *taskexe.Trigger) error {
	//cfg, ok := config.(*task.TaskRun)
	//if !ok {
	//	return taskexe.ErrInvalidConfig
	//}
	//
	//ctx = session.WithCtxUser(ctx, &session.User{ID: *cfg.BaseInfo.CreatedBy.UserID})
	////workspaceID := trigger.Task.GetWorkspaceID()
	////session := getSession(ctx, trigger.Task)
	//var mapping []*task.FieldMapping
	//for _, autoEvaluateConfig := range trigger.Task.TaskConfig.AutoEvaluateConfigs {
	//	mapping = append(mapping, autoEvaluateConfig.FieldMappings...)
	//}
	//turns := buildItems(ctx, []*loop_span.Span{trigger.Span}, mapping, cfg.GetTaskRunConfig().GetAutoEvaluateRunConfig().GetSchema())
	//if len(turns) == 0 {
	//	logs.CtxInfo(ctx, "[task-debug] AutoEvaluteProcessor Invoke, turns is empty")
	//	return nil
	//}
	//category := getCategory(cfg.TaskType)
	//
	//addSuccess, errorGroups, err := p.datasetServiceAdaptor.GetDatasetProvider(category).AddDatasetItems(ctx, cfg.ID, category, turns)
	//if err != nil {
	//	return err
	//}
	return nil
}

func (p *DataReflowProcessor) Finish(ctx context.Context, config any, trigger *taskexe.Trigger) error {
	return nil
}

func (p *DataReflowProcessor) OnChangeProcessor(ctx context.Context, currentTask *task.Task, taskOp task.TaskStatus) error {
	logs.CtxInfo(ctx, "[auto_task] DataReflowProcessor OnChangeProcessor, taskID:%d, taskOp:%s, task:%+v", currentTask.GetID(), taskOp, currentTask)
	session := getSession(ctx, currentTask)
	category := getCategory(currentTask.TaskType)
	dataReflowConfigs := currentTask.GetTaskConfig().GetDataReflowConfig()
	// 1、创建数据集
	logs.CtxInfo(ctx, "[auto_task] CreateDataset,category:%s", category)
	for _, dataReflowConfig := range dataReflowConfigs {
		if dataReflowConfig.DatasetID != nil {
			logs.CtxInfo(ctx, "[auto_task] AutoEvaluteProcessor OnChangeProcessor, datasetID:%d", dataReflowConfig.DatasetID)
			continue
		}
		schema := convertDatasetSchemaDTO2DO(dataReflowConfig.GetDatasetSchema())
		datasetID, err := p.datasetServiceAdaptor.GetDatasetProvider(category).CreateDataset(ctx, entity.NewDataset(
			0,
			currentTask.GetWorkspaceID(),
			dataReflowConfig.GetDatasetName(),
			category,
			schema,
			session,
		))
		if err != nil {
			return err
		}
		logs.CtxInfo(ctx, "[auto_task] AutoEvaluteProcessor OnChangeProcessor, datasetID:%d", datasetID)
	}

	return nil
}
