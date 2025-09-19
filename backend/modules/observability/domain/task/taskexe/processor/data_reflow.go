// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package processor

import (
	"context"
	"fmt"
	"time"

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
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

var _ taskexe.Processor = (*DataReflowProcessor)(nil)

type DataReflowProcessor struct {
	datasetServiceAdaptor *service.DatasetServiceAdaptor
	taskRepo              repo.ITaskRepo
	taskRunRepo           repo.ITaskRunRepo
}

func newDataReflowProcessor(datasetServiceProvider *service.DatasetServiceAdaptor,
	taskRepo repo.ITaskRepo,
	taskRunRepo repo.ITaskRunRepo) *DataReflowProcessor {
	return &DataReflowProcessor{
		datasetServiceAdaptor: datasetServiceProvider,
		taskRepo:              taskRepo,
		taskRunRepo:           taskRunRepo,
	}
}

func (p *DataReflowProcessor) ValidateConfig(ctx context.Context, config any) error {
	cfg, ok := config.(*task.Task)
	if !ok {
		return taskexe.ErrInvalidConfig
	}
	if cfg.GetTaskConfig().GetDataReflowConfig() == nil || len(cfg.GetTaskConfig().GetDataReflowConfig()) == 0 {
		return taskexe.ErrInvalidConfig
	}

	// todo:[xun]1、数据集是否存在，2、数据集是否重名
	return nil
}

func (p *DataReflowProcessor) Invoke(ctx context.Context, config any, trigger *taskexe.Trigger) error {
	cfg, ok := config.(*task_entity.TaskRun)
	if !ok {
		return taskexe.ErrInvalidConfig
	}
	taskRun := tconv.TaskRunPO2DTO(ctx, cfg, nil)

	taskCount, _ := p.taskRepo.GetTaskCount(ctx, *trigger.Task.ID)
	taskRunCount, _ := p.taskRepo.GetTaskRunCount(ctx, *trigger.Task.ID, taskRun.ID)

	p.taskRepo.IncrTaskCount(ctx, *trigger.Task.ID)
	p.taskRepo.IncrTaskRunCount(ctx, *trigger.Task.ID, taskRun.ID)
	p.taskRepo.IncrTaskRunSuccessCount(ctx, *trigger.Task.ID, taskRun.ID)

	if (trigger.Task.GetRule().GetSampler().GetCycleCount() != 0 && taskRunCount > trigger.Task.GetRule().GetSampler().GetCycleCount()) ||
		(taskCount > trigger.Task.GetRule().GetSampler().GetSampleSize()) {
		logs.CtxInfo(ctx, "[task-debug] AutoEvaluteProcessor Invoke, subCount:%v,taskCount:%v", taskRunCount, taskCount)
		p.taskRepo.DecrTaskCount(ctx, *trigger.Task.ID)
		p.taskRepo.DecrTaskRunCount(ctx, *trigger.Task.ID, taskRun.ID)
		p.taskRepo.DecrTaskRunSuccessCount(ctx, *trigger.Task.ID, taskRun.ID)
		return nil
	}
	ctx = session.WithCtxUser(ctx, &session.User{ID: *trigger.Task.BaseInfo.CreatedBy.UserID})
	workspaceID := trigger.Task.GetWorkspaceID()
	sessionInfo := getSession(ctx, trigger.Task)
	var mapping []entity.FieldMapping
	for _, dataReflowConfig := range trigger.Task.TaskConfig.DataReflowConfig {
		mapping = ConvertFieldMappingsDTO2DO(dataReflowConfig.FieldMappings)
	}

	category := entity.DatasetCategory_General
	successItems, _, _ := buildDatasetItems(ctx, []*loop_span.Span{trigger.Span}, mapping, workspaceID, *trigger.Task.ID, entity.NewDataset(
		taskRun.TaskRunConfig.GetDataReflowRunConfig().GetDatasetID(),
		workspaceID,
		"",
		category,
		convertDatasetSchemaDTO2DO(trigger.Task.TaskConfig.DataReflowConfig[0].GetDatasetSchema()),
		sessionInfo,
	))
	_, _, err := p.datasetServiceAdaptor.GetDatasetProvider(category).AddDatasetItems(ctx, taskRun.TaskRunConfig.DataReflowRunConfig.DatasetID, category, successItems)
	if err != nil {
		logs.CtxError(ctx, "[task-debug] AutoEvaluteProcessor Invoke, AddDatasetItems err, taskID:%d, err:%v", *trigger.Task.ID, err)
		p.taskRepo.IncrTaskRunFailCount(ctx, *trigger.Task.ID, taskRun.ID)
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

// shouldTriggerBackfill 判断是否需要发送历史回溯MQ
func ShouldTriggerBackfill(taskDO *task.Task) bool {
	// 检查任务类型
	taskType := taskDO.GetTaskType()
	if taskType != task.TaskTypeAutoEval && taskType != task.TaskTypeAutoDataReflow {
		return false
	}

	// 检查回填时间配置
	rule := taskDO.GetRule()
	if rule == nil {
		return false
	}

	backfillTime := rule.GetBackfillEffectiveTime()
	if backfillTime == nil {
		return false
	}

	return backfillTime.GetStartAt() > 0 &&
		backfillTime.GetEndAt() > 0 &&
		backfillTime.GetStartAt() < backfillTime.GetEndAt()
}

func ShouldTriggerNewData(taskDO *task.Task) bool {
	// 检查任务类型
	taskType := taskDO.GetTaskType()
	if taskType != task.TaskTypeAutoEval && taskType != task.TaskTypeAutoDataReflow {
		return false
	}
	rule := taskDO.GetRule()
	if rule == nil {
		return false
	}

	effectiveTime := rule.GetEffectiveTime()
	if effectiveTime == nil {
		return false
	}

	return effectiveTime.GetEndAt() > 0 &&
		effectiveTime.GetStartAt() > 0 &&
		time.Now().Before(time.Unix(effectiveTime.GetStartAt(), 0))
}

func (p *DataReflowProcessor) OnCreateChangeProcessor(ctx context.Context, currentTask *task.Task) error {
	logs.CtxInfo(ctx, "[auto_task] DataReflowProcessor OnChangeProcessor, taskID:%d, task:%+v", currentTask.GetID(), currentTask)
	taskRuns, err := p.taskRunRepo.GetBackfillTaskRun(ctx, nil, currentTask.GetID())
	if err != nil {
		logs.CtxError(ctx, "GetBackfillTaskRun failed, taskID:%d, err:%v", currentTask.GetID(), err)
		return err
	}
	if ShouldTriggerBackfill(currentTask) && taskRuns == nil {
		err = p.OnChangeProcessor(ctx, currentTask, true)
		if err != nil {
			logs.CtxError(ctx, "OnCreateChangeProcessor failed, taskID:%d, err:%v", currentTask.GetID(), err)
			return err
		}
		err = p.OnUpdateChangeProcessor(ctx, currentTask, task.TaskStatusRunning)
		if err != nil {
			logs.CtxError(ctx, "OnCreateChangeProcessor failed, taskID:%d, err:%v", currentTask.GetID(), err)
			return err
		}
	}

	if ShouldTriggerNewData(currentTask) {
		err = p.OnChangeProcessor(ctx, currentTask, false)
		if err != nil {
			logs.CtxError(ctx, "OnCreateChangeProcessor failed, taskID:%d, err:%v", currentTask.GetID(), err)
			return err
		}
		err = p.OnUpdateChangeProcessor(ctx, currentTask, task.TaskStatusRunning)
		if err != nil {
			logs.CtxError(ctx, "OnCreateChangeProcessor failed, taskID:%d, err:%v", currentTask.GetID(), err)
			return err
		}
	}
	return nil
}

func (p *DataReflowProcessor) OnChangeProcessor(ctx context.Context, currentTask *task.Task, isBackFill bool) error {
	// 1、创建/更新数据集
	session := getSession(ctx, currentTask)
	category := getCategory(currentTask.TaskType)
	dataReflowConfigs := currentTask.GetTaskConfig().GetDataReflowConfig()
	var err error
	// 1、创建数据集
	logs.CtxInfo(ctx, "[auto_task] CreateDataset,category:%s", category)
	var datasetID int64
	for _, dataReflowConfig := range dataReflowConfigs {
		if dataReflowConfig.DatasetID != nil {
			datasetID = *dataReflowConfig.DatasetID
			logs.CtxInfo(ctx, "[auto_task] AutoEvaluteProcessor OnChangeProcessor, datasetID:%d", dataReflowConfig.DatasetID)
			continue
		}
		schema := convertDatasetSchemaDTO2DO(dataReflowConfig.GetDatasetSchema())
		datasetID, err = p.datasetServiceAdaptor.GetDatasetProvider(category).CreateDataset(ctx, entity.NewDataset(
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
	// 2、更新任务配置
	taskConfig, err := p.taskRepo.GetTask(ctx, currentTask.GetID(), nil, nil)
	if err != nil {
		return err
	}
	// 3、创建 taskrun：历史回溯生成一个taskRun,新数据生成一个taskRun
	cycleStartAt := currentTask.GetRule().GetEffectiveTime().GetStartAt()
	cycleEndAt := currentTask.GetRule().GetEffectiveTime().GetEndAt()
	var taskRun *task_entity.TaskRun
	taskRunConfig := &task.TaskRunConfig{
		DataReflowRunConfig: &task.DataReflowRunConfig{
			DatasetID:    datasetID,
			EndAt:        currentTask.GetRule().GetEffectiveTime().GetEndAt(),
			CycleStartAt: cycleStartAt,
			CycleEndAt:   cycleEndAt,
			Status:       task.RunStatusRunning,
		},
	}
	var runType task.TaskRunType
	if isBackFill {
		runType = task.TaskRunTypeBackFill
	} else {
		runType = task.TaskRunTypeNewData
	}
	taskRun, err = p.OnCreateTaskRunProcessor(ctx, currentTask, taskRunConfig, runType)
	if err != nil {
		return err
	}
	taskConfig.TaskRuns = append(taskConfig.TaskRuns, taskRun)

	err = p.taskRepo.UpdateTask(ctx, taskConfig)
	if err != nil {
		return err
	}
	return nil
}
func (p *DataReflowProcessor) OnUpdateChangeProcessor(ctx context.Context, currentTask *task.Task, taskOp task.TaskStatus) error {
	switch taskOp {
	case task.TaskStatusSuccess:
		if currentTask.GetTaskStatus() != task.TaskStatusDisabled {
			currentTask.TaskStatus = ptr.Of(task.TaskStatusSuccess)
		}
	case task.TaskStatusRunning:
		if currentTask.GetTaskStatus() != task.TaskStatusDisabled && currentTask.GetTaskStatus() != task.TaskStatusSuccess {
			currentTask.TaskStatus = ptr.Of(task.TaskStatusRunning)
		}
	case task.TaskStatusDisabled:
		if currentTask.GetTaskStatus() != task.TaskStatusDisabled {
			currentTask.TaskStatus = ptr.Of(task.TaskStatusDisabled)
		}
	case task.TaskStatusPending:
		if currentTask.GetTaskStatus() == task.TaskStatusPending || currentTask.GetTaskStatus() == task.TaskStatusUnstarted {
			currentTask.TaskStatus = ptr.Of(task.TaskStatusPending)
		}
	default:
		return fmt.Errorf("OnUpdateChangeProcessor, valid taskOp:%s", taskOp)
	}
	// 2、更新任务
	taskPO := tconv.CreateTaskDTO2PO(ctx, currentTask, "")
	err := p.taskRepo.UpdateTask(ctx, taskPO)
	if err != nil {
		logs.CtxError(ctx, "[auto_task] OnUpdateChangeProcessor, UpdateTask err, taskID:%d, err:%v", currentTask.GetID(), err)
		return err
	}
	return nil
}
func (p *DataReflowProcessor) OnFinishChangeProcessor(ctx context.Context, currentTask *task.Task) error {
	// 更新任务配置
	// 更新TaskRun
	return nil
}

func (p *DataReflowProcessor) OnCreateTaskRunProcessor(ctx context.Context, currentTask *task.Task, runConfig *task.TaskRunConfig, runType task.TaskRunType) (*task_entity.TaskRun, error) {
	// 创建taskRun
	cycleStartAt := currentTask.GetRule().GetEffectiveTime().GetStartAt()
	cycleEndAt := currentTask.GetRule().GetEffectiveTime().GetEndAt()
	var taskRun *task_entity.TaskRun
	taskRunConfig := runConfig
	taskRun = &task_entity.TaskRun{
		TaskID:      currentTask.GetID(),
		WorkspaceID: currentTask.GetWorkspaceID(),
		TaskType:    runType,
		RunStatus:   task.RunStatusRunning,
		RunStartAt:  time.UnixMilli(cycleStartAt),
		RunEndAt:    time.UnixMilli(cycleEndAt),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		RunConfig:   ptr.Of(ToJSONString(ctx, taskRunConfig)),
	}
	id, err := p.taskRepo.CreateTaskRun(ctx, taskRun)
	if err != nil {
		logs.CtxError(ctx, "[auto_task] OnCreateTaskRunProcessor, CreateTaskRun err, taskRun:%+v, err:%v", taskRun, err)
		return nil, err
	}
	taskRun.ID = id
	return taskRun, nil
}
func (p *DataReflowProcessor) OnFinishTaskRunProcessor(ctx context.Context, taskRun *task_entity.TaskRun) error {
	// 设置taskRun状态为已完成
	taskRun.RunStatus = task.RunStatusDone
	// 更新taskRun
	err := p.taskRepo.UpdateTaskRun(ctx, taskRun)
	if err != nil {
		logs.CtxError(ctx, "[auto_task] OnFinishTaskRunProcessor, UpdateTaskRun err, taskRunID:%d, err:%v", taskRun.ID, err)
		return err
	}
	return nil
}
