// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package processor

// import (
//
//	"context"
//	"fmt"
//	"time"
//
//	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
//	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
//	dataset0 "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/dataset"
//	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
//	tconv "github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor/task"
//	task_entity "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
//	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo"
//	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe"
//	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
//	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
//	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
//	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
//	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
//	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
//	"github.com/coze-dev/coze-loop/backend/pkg/logs"
//
// )
//
// var _ taskexe.Processor = (*DataReflowProcessor)(nil)
//
//	type DataReflowProcessor struct {
//		datasetServiceAdaptor *service.DatasetServiceAdaptor
//		taskRepo              repo.ITaskRepo
//		taskRunRepo           repo.ITaskRunRepo
//	}
//
// func NewDataReflowProcessor(datasetServiceProvider *service.DatasetServiceAdaptor,
//
//		taskRepo repo.ITaskRepo,
//		taskRunRepo repo.ITaskRunRepo) *DataReflowProcessor {
//		return &DataReflowProcessor{
//			datasetServiceAdaptor: datasetServiceProvider,
//			taskRepo:              taskRepo,
//			taskRunRepo:           taskRunRepo,
//		}
//	}
//
//	func (p *DataReflowProcessor) ValidateConfig(ctx context.Context, config any) error {
//		cfg, ok := config.(*task.Task)
//		if !ok {
//			return taskexe.ErrInvalidConfig
//		}
//		if cfg.GetTaskConfig().GetDataReflowConfig() == nil || len(cfg.GetTaskConfig().GetDataReflowConfig()) == 0 {
//			return taskexe.ErrInvalidConfig
//		}
//
//		// todo:[xun]1、数据集是否存在，2、数据集是否重名
//		return nil
//	}
//
//	func (p *DataReflowProcessor) Invoke(ctx context.Context, config any, trigger *taskexe.Trigger) error {
//		cfg, ok := config.(*task_entity.TaskRun)
//		if !ok {
//			return taskexe.ErrInvalidConfig
//		}
//		taskRun := tconv.TaskRunPO2DTO(ctx, cfg, nil)
//
//		taskCount, _ := p.taskRepo.GetTaskCount(ctx, *trigger.Task.ID)
//		taskRunCount, _ := p.taskRepo.GetTaskRunCount(ctx, *trigger.Task.ID, taskRun.ID)
//
//		p.taskRepo.IncrTaskCount(ctx, *trigger.Task.ID)
//		p.taskRepo.IncrTaskRunCount(ctx, *trigger.Task.ID, taskRun.ID)
//		p.taskRepo.IncrTaskRunSuccessCount(ctx, *trigger.Task.ID, taskRun.ID)
//
//		if (trigger.Task.GetRule().GetSampler().GetCycleCount() != 0 && taskRunCount+1 > trigger.Task.GetRule().GetSampler().GetCycleCount()) ||
//			(taskCount+1 > trigger.Task.GetRule().GetSampler().GetSampleSize()) {
//			logs.CtxInfo(ctx, "[task-debug] AutoEvaluteProcessor Invoke, subCount:%v,taskCount:%v", taskRunCount, taskCount)
//			p.taskRepo.DecrTaskCount(ctx, *trigger.Task.ID)
//			p.taskRepo.DecrTaskRunCount(ctx, *trigger.Task.ID, taskRun.ID)
//			p.taskRepo.DecrTaskRunSuccessCount(ctx, *trigger.Task.ID, taskRun.ID)
//			return nil
//		}
//		ctx = session.WithCtxUser(ctx, &session.User{ID: *trigger.Task.BaseInfo.CreatedBy.UserID})
//		workspaceID := trigger.Task.GetWorkspaceID()
//		sessionInfo := getSession(ctx, trigger.Task)
//		var mapping []entity.FieldMapping
//		for _, dataReflowConfig := range trigger.Task.TaskConfig.DataReflowConfig {
//			mapping = ConvertFieldMappingsDTO2DO(dataReflowConfig.FieldMappings)
//		}
//
//		category := entity.DatasetCategory_General
//		successItems, _, _ := buildDatasetItems(ctx, []*loop_span.Span{trigger.Span}, mapping, workspaceID, *trigger.Task.ID, entity.NewDataset(
//			taskRun.TaskRunConfig.GetDataReflowRunConfig().GetDatasetID(),
//			workspaceID,
//			"",
//			category,
//			convertDatasetSchemaDTO2DO(trigger.Task.TaskConfig.DataReflowConfig[0].GetDatasetSchema()),
//			sessionInfo,
//		))
//		_, _, err := p.datasetServiceAdaptor.GetDatasetProvider(category).AddDatasetItems(ctx, taskRun.TaskRunConfig.DataReflowRunConfig.DatasetID, category, successItems)
//		if err != nil {
//			logs.CtxError(ctx, "[task-debug] AutoEvaluteProcessor Invoke, AddDatasetItems err, taskID:%d, err:%v", *trigger.Task.ID, err)
//			p.taskRepo.IncrTaskRunFailCount(ctx, *trigger.Task.ID, taskRun.ID)
//			return err
//		}
//
//		return nil
//	}
//
//	func ConvertFieldMappingsDTO2DO(mappings []*dataset0.FieldMapping) []entity.FieldMapping {
//		if len(mappings) == 0 {
//			return nil
//		}
//
//		result := make([]entity.FieldMapping, len(mappings))
//		for i, mapping := range mappings {
//			result[i] = entity.FieldMapping{
//				FieldSchema: entity.FieldSchema{
//					Key:         mapping.GetFieldSchema().Key,
//					Name:        mapping.GetFieldSchema().GetName(),
//					Description: mapping.GetFieldSchema().GetDescription(),
//					ContentType: convertContentTypeDTO2DO(mapping.GetFieldSchema().GetContentType()),
//					TextSchema:  mapping.GetFieldSchema().GetTextSchema(),
//				},
//				TraceFieldKey:      mapping.GetTraceFieldKey(),
//				TraceFieldJsonpath: mapping.GetTraceFieldJsonpath(),
//			}
//		}
//
//		return result
//	}
//

//func (p *DataReflowProcessor) createOrUpdateDataset(ctx context.Context, workspaceID int64, category entity.DatasetCategory, dataReflowConfig *task.DataReflowConfig, session *common.Session) (*entity.Dataset, error) {
//	var err error
//	var datasetID int64
//
//	if dataReflowConfig.GetDatasetID() == 0 {
//		if dataReflowConfig.DatasetName == nil || *dataReflowConfig.DatasetName == "" {
//			return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("task name exist"))
//		}
//		if len(dataReflowConfig.DatasetSchema.FieldSchemas) == 0 {
//			return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("dataset schema is empty"))
//		}
//
//		schema := convertDatasetSchemaDTO2DO(dataReflowConfig.GetDatasetSchema())
//		datasetID, err = p.datasetServiceAdaptor.GetDatasetProvider(category).CreateDataset(ctx, entity.NewDataset(
//			0,
//			workspaceID,
//			dataReflowConfig.GetDatasetName(),
//			category,
//			schema,
//			session,
//		))
//		if err != nil {
//			return nil, err
//		}
//	} else {
//		if dataReflowConfig.DatasetID == nil {
//			return nil, errorx.NewByCode(obErrorx.CommonInvalidParamCode, errorx.WithExtraMsg("dataset id is nil"))
//		}
//		datasetID = *dataReflowConfig.DatasetID
//		needUpdate := false
//		for _, schema := range dataReflowConfig.DatasetSchema.FieldSchemas {
//			if schema.Key == nil || *schema.Key == "" {
//				needUpdate = true
//				break
//			}
//		}
//		if needUpdate {
//			if err := p.datasetServiceAdaptor.GetDatasetProvider(category).UpdateDatasetSchema(ctx, entity.NewDataset(
//				datasetID,
//				workspaceID,
//				"",
//				category,
//				convertDatasetSchemaDTO2DO(dataReflowConfig.DatasetSchema),
//				nil,
//			)); err != nil {
//				return nil, err
//			}
//		}
//	}
//
//	// 新增或修改评测集后，都需要重新查询一次，拿到fieldSchema里的key
//	return p.datasetServiceAdaptor.GetDatasetProvider(category).GetDataset(ctx, workspaceID, datasetID, category)
//}
//
//func (p *DataReflowProcessor) OnCreateTaskChange(ctx context.Context, currentTask *task.Task) error {
//	// 1、创建/更新数据集
//	session := getSession(ctx, currentTask)
//	category := getCategory(currentTask.TaskType)
//	dataReflowConfigs := currentTask.GetTaskConfig().GetDataReflowConfig()
//	var err error
//	// 1、创建数据集
//	logs.CtxInfo(ctx, "[auto_task] CreateDataset,category:%s", category)
//	for _, dataReflowConfig := range dataReflowConfigs {
//		dataset, err := p.createOrUpdateDataset(ctx, currentTask.GetWorkspaceID(), category, dataReflowConfig, session)
//		if err != nil {
//			return err
//		}
//		currentTask.TaskConfig.DataReflowConfig[0] = &task.DataReflowConfig{
//			DatasetID:     ptr.Of(dataset.ID),
//			DatasetName:   dataReflowConfig.DatasetName,
//			DatasetSchema: dataReflowConfig.DatasetSchema,
//			FieldMappings: dataReflowConfig.FieldMappings,
//		}
//		taskPO := tconv.TaskDTO2PO(ctx, currentTask, "")
//		err = p.taskRepo.UpdateTask(ctx, taskPO)
//		if err != nil {
//			logs.CtxError(ctx, "[auto_task] AutoEvaluteProcessor OnChangeProcessor, UpdateTask err, taskID:%d, err:%v", currentTask.GetID(), err)
//			return err
//		}
//	}
//	taskRuns, err := p.taskRunRepo.GetBackfillTaskRun(ctx, nil, currentTask.GetID())
//	if err != nil {
//		logs.CtxError(ctx, "GetBackfillTaskRun failed, taskID:%d, err:%v", currentTask.GetID(), err)
//		return err
//	}
//	if ShouldTriggerBackfill(currentTask) && taskRuns == nil {
//		err = p.OnCreateTaskRunChange(ctx, taskexe.OnCreateTaskRunChangeReq{
//			CurrentTask: currentTask,
//			RunType:     task.TaskRunTypeBackFill,
//			RunStartAt:  time.Now().UnixMilli(),
//			RunEndAt:    time.Now().UnixMilli() + (currentTask.GetRule().GetBackfillEffectiveTime().GetEndAt() - currentTask.GetRule().GetBackfillEffectiveTime().GetStartAt()),
//		})
//		if err != nil {
//			logs.CtxError(ctx, "OnCreateChangeProcessor failed, taskID:%d, err:%v", currentTask.GetID(), err)
//			return err
//		}
//		err = p.OnUpdateTaskChange(ctx, currentTask, task.TaskStatusRunning)
//		if err != nil {
//			logs.CtxError(ctx, "OnCreateChangeProcessor failed, taskID:%d, err:%v", currentTask.GetID(), err)
//			return err
//		}
//	}
//	if ShouldTriggerNewData(ctx, currentTask) {
//		var runStartAt, runEndAt int64
//		runStartAt = currentTask.GetRule().GetEffectiveTime().GetStartAt()
//		if !currentTask.GetRule().GetSampler().GetIsCycle() {
//			runEndAt = currentTask.GetRule().GetEffectiveTime().GetEndAt()
//		} else {
//			switch *currentTask.GetRule().GetSampler().CycleTimeUnit {
//			case task.TimeUnitDay:
//				runEndAt = runStartAt + (*currentTask.GetRule().GetSampler().CycleInterval)*24*time.Hour.Milliseconds()
//			case task.TimeUnitWeek:
//				runEndAt = runStartAt + (*currentTask.GetRule().GetSampler().CycleInterval)*7*24*time.Hour.Milliseconds()
//			default:
//				runEndAt = runStartAt + (*currentTask.GetRule().GetSampler().CycleInterval)*10*time.Minute.Milliseconds()
//			}
//		}
//		err = p.OnCreateTaskRunChange(ctx, taskexe.OnCreateTaskRunChangeReq{
//			CurrentTask: currentTask,
//			RunType:     task.TaskRunTypeNewData,
//			RunStartAt:  currentTask.GetRule().GetEffectiveTime().GetStartAt(),
//			RunEndAt:    runEndAt,
//		})
//		err = p.OnUpdateTaskChange(ctx, currentTask, task.TaskStatusRunning)
//		if err != nil {
//			logs.CtxError(ctx, "OnCreateChangeProcessor failed, taskID:%d, err:%v", currentTask.GetID(), err)
//			return err
//		}
//	}
//	return nil
//}
//
//func (p *DataReflowProcessor) OnUpdateTaskChange(ctx context.Context, currentTask *task.Task, taskOp task.TaskStatus) error {
//	switch taskOp {
//	case task.TaskStatusSuccess:
//		if currentTask.GetTaskStatus() != task.TaskStatusDisabled {
//			*currentTask.TaskStatus = task.TaskStatusSuccess
//		}
//	case task.TaskStatusRunning:
//		if currentTask.GetTaskStatus() != task.TaskStatusDisabled && currentTask.GetTaskStatus() != task.TaskStatusSuccess {
//			*currentTask.TaskStatus = task.TaskStatusRunning
//		}
//	case task.TaskStatusDisabled:
//		if currentTask.GetTaskStatus() != task.TaskStatusDisabled {
//			*currentTask.TaskStatus = task.TaskStatusDisabled
//		}
//	case task.TaskStatusPending:
//		if currentTask.GetTaskStatus() == task.TaskStatusPending || currentTask.GetTaskStatus() == task.TaskStatusUnstarted {
//			*currentTask.TaskStatus = task.TaskStatusPending
//		}
//	default:
//		return fmt.Errorf("OnUpdateChangeProcessor, valid taskOp:%s", taskOp)
//	}
//	// 2、更新任务
//	taskPO := tconv.TaskDTO2PO(ctx, currentTask, "")
//	err := p.taskRepo.UpdateTask(ctx, taskPO)
//	if err != nil {
//		logs.CtxError(ctx, "[auto_task] OnUpdateChangeProcessor, UpdateTask err, taskID:%d, err:%v", currentTask.GetID(), err)
//		return err
//	}
//	return nil
//}
//
//func (p *DataReflowProcessor) OnFinishTaskChange(ctx context.Context, param taskexe.OnFinishTaskChangeReq) error {
//	err := p.OnFinishTaskRunChange(ctx, taskexe.OnFinishTaskRunChangeReq{
//		Task:    param.Task,
//		TaskRun: param.TaskRun,
//	})
//	if err != nil {
//		logs.CtxError(ctx, "OnFinishTaskRunChange failed, taskRun:%+v, err:%v", param.TaskRun, err)
//		return err
//	}
//	if param.IsFinish {
//		logs.CtxWarn(ctx, "OnFinishTaskChange, taskID:%d, taskRun:%+v", param.Task.GetID(), param.TaskRun)
//		if err := p.OnUpdateTaskChange(ctx, param.Task, task.TaskStatusSuccess); err != nil {
//			logs.CtxError(ctx, "OnUpdateChangeProcessor failed, taskID:%d, err:%v", param.Task.GetID(), err)
//			return err
//		}
//	}
//	return nil
//}
//
//func (p *DataReflowProcessor) OnCreateTaskRunChange(ctx context.Context, param taskexe.OnCreateTaskRunChangeReq) error {
//	var taskRunConfig *task.TaskRunConfig
//	currentTask := param.CurrentTask
//
//	taskRunConfig = &task.TaskRunConfig{
//		DataReflowRunConfig: &task.DataReflowRunConfig{
//			DatasetID:    *currentTask.GetTaskConfig().GetDataReflowConfig()[0].DatasetID,
//			EndAt:        param.RunEndAt,
//			CycleStartAt: param.RunStartAt,
//			CycleEndAt:   param.RunEndAt,
//			Status:       task.RunStatusRunning,
//		},
//	}
//	taskRun := &task_entity.TaskRun{
//		TaskID:      currentTask.GetID(),
//		WorkspaceID: currentTask.GetWorkspaceID(),
//		TaskType:    param.RunType,
//		RunStatus:   task.RunStatusRunning,
//		RunStartAt:  time.UnixMilli(param.RunStartAt),
//		RunEndAt:    time.UnixMilli(param.RunEndAt),
//		CreatedAt:   time.Now(),
//		UpdatedAt:   time.Now(),
//		RunConfig:   ptr.Of(ToJSONString(ctx, taskRunConfig)),
//	}
//	id, err := p.taskRepo.CreateTaskRun(ctx, taskRun)
//	if err != nil {
//		logs.CtxError(ctx, "[auto_task] OnCreateTaskRunProcessor, CreateTaskRun err, taskRun:%+v, err:%v", taskRun, err)
//		return err
//	}
//	taskRun.ID = id
//	taskConfig, err := p.taskRepo.GetTask(ctx, currentTask.GetID(), nil, nil)
//	if err != nil {
//		return err
//	}
//	taskConfig.TaskRuns = append(taskConfig.TaskRuns, taskRun)
//	err = p.taskRepo.UpdateTask(ctx, taskConfig)
//	if err != nil {
//		return err
//	}
//	return nil
//}
//func (p *DataReflowProcessor) OnFinishTaskRunChange(ctx context.Context, param taskexe.OnFinishTaskRunChangeReq) error {
//	taskRun := param.TaskRun
//	// 设置taskRun状态为已完成
//	taskRun.RunStatus = task.RunStatusDone
//	// 更新taskRun
//	err := p.taskRepo.UpdateTaskRun(ctx, taskRun)
//	if err != nil {
//		logs.CtxError(ctx, "[auto_task] OnFinishTaskRunProcessor, UpdateTaskRun err, taskRunID:%d, err:%v", taskRun.ID, err)
//		return err
//	}
//	return nil
//}
