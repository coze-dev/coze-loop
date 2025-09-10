// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package processor

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	dataset0 "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	tconv "github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor/task"
	task_entity "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/taskexe"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

var _ taskexe.Processor = (*DataReflowProcessor)(nil)

type DataReflowProcessor struct {
	datasetServiceAdaptor *service.DatasetServiceAdaptor
	TaskRepo              repo.ITaskRepo
}

func newDataReflowProcessor(datasetServiceProvider *service.DatasetServiceAdaptor,
	taskRepo repo.ITaskRepo) *DataReflowProcessor {
	return &DataReflowProcessor{
		datasetServiceAdaptor: datasetServiceProvider,
		TaskRepo:              taskRepo,
	}
}

func (p *DataReflowProcessor) ValidateConfig(ctx context.Context, config any, workspaceID int64) error {

	return nil
}

func (p *DataReflowProcessor) Invoke(ctx context.Context, config any, trigger *taskexe.Trigger) error {
	cfg, ok := config.(*task_entity.TaskRun)
	if !ok {
		return taskexe.ErrInvalidConfig
	}
	taskRun := tconv.TaskRunPO2DTO(ctx, cfg, nil)

	ctx = session.WithCtxUser(ctx, &session.User{ID: *trigger.Task.BaseInfo.CreatedBy.UserID})
	workspaceID := trigger.Task.GetWorkspaceID()
	sessionInfo := getSession(ctx, trigger.Task)
	var mapping []entity.FieldMapping
	for _, dataReflowConfig := range trigger.Task.TaskConfig.DataReflowConfig {
		mapping = ConvertFieldMappingsDTO2DO(dataReflowConfig.FieldMappings)
	}

	category := getCategory(cfg.TaskType)
	successItems, _, _ := buildDatasetItems(ctx, []*loop_span.Span{trigger.Span}, mapping, workspaceID, &entity.Dataset{
		ID:              taskRun.TaskRunConfig.GetDataReflowRunConfig().GetDatasetID(),
		DatasetCategory: category,
		Seesion:         sessionInfo,
	})
	_, _, err := p.datasetServiceAdaptor.GetDatasetProvider(category).AddDatasetItems(ctx, cfg.ID, category, successItems)
	if err != nil {
		return err
	}
	return nil
}

func ConvertFieldMappingsDTO2DO(mappings []*dataset0.FieldMapping) []entity.FieldMapping {
	if len(mappings) == 0 {
		return nil
	}

	result := make([]entity.FieldMapping, len(mappings))
	for i, mapping := range mappings {
		result[i] = entity.FieldMapping{
			FieldSchema: entity.FieldSchema{
				Key:         mapping.GetFieldSchema().Key,
				Name:        mapping.GetFieldSchema().GetName(),
				Description: mapping.GetFieldSchema().GetDescription(),
				ContentType: convertContentTypeDTO2DO(mapping.GetFieldSchema().GetContentType()),
				TextSchema:  mapping.GetFieldSchema().GetTextSchema(),
			},
			TraceFieldKey:      mapping.GetTraceFieldKey(),
			TraceFieldJsonpath: mapping.GetTraceFieldJsonpath(),
		}
	}

	return result
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
