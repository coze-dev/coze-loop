// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package processor

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/apaxa-go/helper/strconvh"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_set"
	eval_target_d "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_target"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_target"
	dataset0 "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	tconv "github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	task_entity "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service/taskexe"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/slices"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

var _ taskexe.Processor = (*AutoEvaluteProcessor)(nil)

type AutoEvaluteProcessor struct {
	evalSvc               rpc.IEvaluatorRPCAdapter
	evaluationSvc         rpc.IEvaluationRPCAdapter
	datasetServiceAdaptor *service.DatasetServiceAdaptor
	taskRepo              repo.ITaskRepo
	aid                   int32
}

func NewAutoEvaluteProcessor(
	aid int32,
	datasetServiceProvider *service.DatasetServiceAdaptor,
	evalService rpc.IEvaluatorRPCAdapter,
	evaluationService rpc.IEvaluationRPCAdapter,
	taskRepo repo.ITaskRepo) *AutoEvaluteProcessor {
	return &AutoEvaluteProcessor{
		datasetServiceAdaptor: datasetServiceProvider,
		evalSvc:               evalService,
		evaluationSvc:         evaluationService,
		taskRepo:              taskRepo,
		aid:                   aid,
	}
}

func (p *AutoEvaluteProcessor) ValidateConfig(ctx context.Context, config any) error {
	cfg, ok := config.(*task.Task)
	if !ok {
		return taskexe.ErrInvalidConfig
	}
	if cfg.GetRule() != nil && cfg.GetRule().GetEffectiveTime() != nil {
		startAt := cfg.GetRule().GetEffectiveTime().GetStartAt()
		endAt := cfg.GetRule().GetEffectiveTime().GetEndAt()
		if startAt <= time.Now().Add(-10*time.Minute).UnixMilli() {
			return errorx.NewByCode(obErrorx.CommonInvalidParamCode)
		}
		if startAt >= endAt {
			return errorx.NewByCode(obErrorx.CommonInvalidParamCode)
		}
	}
	var evaluatorVersionIDs []int64
	for _, autoEvaluateConfig := range cfg.GetTaskConfig().GetAutoEvaluateConfigs() {
		evaluatorVersionIDs = append(evaluatorVersionIDs, autoEvaluateConfig.GetEvaluatorVersionID())
	}
	if len(evaluatorVersionIDs) == 0 {
		return errorx.NewByCode(obErrorx.CommonInvalidParamCode)
	}
	// 检查评估器版本是否合法
	evaluators, _, err := p.evalSvc.BatchGetEvaluatorVersions(ctx, &rpc.BatchGetEvaluatorVersionsParam{
		WorkspaceID:         cfg.GetWorkspaceID(),
		EvaluatorVersionIds: evaluatorVersionIDs,
	})
	if err != nil {
		return errorx.NewByCode(obErrorx.CommonInvalidParamCode)
	}
	if len(evaluators) != len(evaluatorVersionIDs) {
		return errorx.NewByCode(obErrorx.CommonInvalidParamCode)
	}
	return nil
}

func (p *AutoEvaluteProcessor) Invoke(ctx context.Context, config any, trigger *taskexe.Trigger) error {
	cfg, ok := config.(*task_entity.TaskRun)
	if !ok {
		return taskexe.ErrInvalidConfig
	}
	taskRun := tconv.TaskRunPO2DTO(ctx, cfg, nil)
	workspaceID := trigger.Task.GetWorkspaceID()
	session := p.getSession(ctx, trigger.Task)
	var mapping []*task.EvaluateFieldMapping
	for _, autoEvaluateConfig := range trigger.Task.TaskConfig.AutoEvaluateConfigs {
		mapping = append(mapping, autoEvaluateConfig.FieldMappings...)
	}
	turns := buildItems(ctx, []*loop_span.Span{trigger.Span}, mapping, taskRun.GetTaskRunConfig().GetAutoEvaluateRunConfig().GetSchema(), strconv.FormatInt(taskRun.ID, 10))
	if len(turns) == 0 {
		logs.CtxInfo(ctx, "[task-debug] AutoEvaluteProcessor Invoke, turns is empty")
		return nil
	}
	taskTTL := trigger.Task.GetRule().GetEffectiveTime().GetEndAt() - trigger.Task.GetRule().GetEffectiveTime().GetStartAt()
	taskCount, _ := p.taskRepo.GetTaskCount(ctx, *trigger.Task.ID)
	taskRunCount, _ := p.taskRepo.GetTaskRunCount(ctx, *trigger.Task.ID, taskRun.ID)
	p.taskRepo.IncrTaskCount(ctx, *trigger.Task.ID, taskTTL)
	p.taskRepo.IncrTaskRunCount(ctx, *trigger.Task.ID, taskRun.ID, taskTTL)
	if (trigger.Task.GetRule().GetSampler().GetCycleCount() != 0 && taskRunCount+1 > trigger.Task.GetRule().GetSampler().GetCycleCount()) ||
		(taskCount+1 > trigger.Task.GetRule().GetSampler().GetSampleSize()) {
		logs.CtxInfo(ctx, "[task-debug] AutoEvaluteProcessor Invoke, subCount:%v,taskCount:%v", taskRunCount, taskCount)
		p.taskRepo.DecrTaskCount(ctx, *trigger.Task.ID, taskTTL)
		p.taskRepo.DecrTaskRunCount(ctx, *trigger.Task.ID, taskRun.ID, taskTTL)
		return nil
	}
	_, err := p.evaluationSvc.InvokeExperiment(ctx, &rpc.InvokeExperimentReq{
		WorkspaceID:     workspaceID,
		EvaluationSetID: taskRun.GetTaskRunConfig().GetAutoEvaluateRunConfig().GetEvalID(),
		Items: []*eval_set.EvaluationSetItem{
			{
				WorkspaceID:     gptr.Of(workspaceID),
				EvaluationSetID: gptr.Of(taskRun.GetTaskRunConfig().GetAutoEvaluateRunConfig().GetEvalID()),
				SchemaID:        gptr.Of(taskRun.GetTaskRunConfig().GetAutoEvaluateRunConfig().GetSchemaID()),
				Turns:           turns,
				ItemKey:         gptr.Of(trigger.Span.SpanID),
			},
		},
		SkipInvalidItems: gptr.Of(true),
		AllowPartialAdd:  gptr.Of(true),
		ExperimentID:     gptr.Of(taskRun.GetTaskRunConfig().GetAutoEvaluateRunConfig().GetExptID()),
		ExperimentRunID:  gptr.Of(taskRun.GetTaskRunConfig().GetAutoEvaluateRunConfig().GetExptRunID()),
		//Ext: map[string]string{"workspace_id": strconv.FormatInt(workspaceID, 10),
		//	"span_id": trigger.Span.SpanID, "trace_id": trigger.Span.TraceID,
		//	"start_time":    strconvh.FormatInt64(trigger.Span.StartTime),
		//	"task_id":       strconvh.FormatInt64(trigger.Task.GetID()),
		//	"task_run_id":   strconvh.FormatInt64(taskRun.ID),
		//	"platform_type": trigger.Task.GetRule().GetSpanFilters().GetPlatformType()},
		Session: session,
	})
	if err != nil {
		p.taskRepo.DecrTaskCount(ctx, *trigger.Task.ID, taskTTL)
		p.taskRepo.DecrTaskRunCount(ctx, *trigger.Task.ID, taskRun.ID, taskTTL)
		return err
	}
	return nil
}

func (p *AutoEvaluteProcessor) OnCreateTaskChange(ctx context.Context, currentTask *task.Task) error {
	taskRuns, err := p.taskRepo.GetBackfillTaskRun(ctx, nil, currentTask.GetID())
	if err != nil {
		logs.CtxError(ctx, "GetBackfillTaskRun failed, taskID:%d, err:%v", currentTask.GetID(), err)
		return err
	}
	if ShouldTriggerBackfill(currentTask) && taskRuns == nil {
		err = p.OnCreateTaskRunChange(ctx, taskexe.OnCreateTaskRunChangeReq{
			CurrentTask: currentTask,
			RunType:     task.TaskRunTypeBackFill,
			RunStartAt:  time.Now().UnixMilli(),
			RunEndAt:    time.Now().UnixMilli() + (currentTask.GetRule().GetBackfillEffectiveTime().GetEndAt() - currentTask.GetRule().GetBackfillEffectiveTime().GetStartAt()),
		})
		if err != nil {
			logs.CtxError(ctx, "OnCreateTaskChange failed, taskID:%d, err:%v", currentTask.GetID(), err)
			return err
		}
		err = p.OnUpdateTaskChange(ctx, currentTask, task.TaskStatusRunning)
		if err != nil {
			logs.CtxError(ctx, "OnCreateTaskChange failed, taskID:%d, err:%v", currentTask.GetID(), err)
			return err
		}
	}
	if ShouldTriggerNewData(ctx, currentTask) {
		var runStartAt, runEndAt int64
		runStartAt = currentTask.GetRule().GetEffectiveTime().GetStartAt()
		if !currentTask.GetRule().GetSampler().GetIsCycle() {
			runEndAt = currentTask.GetRule().GetEffectiveTime().GetEndAt()
		} else {
			switch *currentTask.GetRule().GetSampler().CycleTimeUnit {
			case task.TimeUnitDay:
				runEndAt = runStartAt + (*currentTask.GetRule().GetSampler().CycleInterval)*24*time.Hour.Milliseconds()
			case task.TimeUnitWeek:
				runEndAt = runStartAt + (*currentTask.GetRule().GetSampler().CycleInterval)*7*24*time.Hour.Milliseconds()
			default:
				runEndAt = runStartAt + (*currentTask.GetRule().GetSampler().CycleInterval)*10*time.Minute.Milliseconds()
			}
		}
		err = p.OnCreateTaskRunChange(ctx, taskexe.OnCreateTaskRunChangeReq{
			CurrentTask: currentTask,
			RunType:     task.TaskRunTypeNewData,
			RunStartAt:  currentTask.GetRule().GetEffectiveTime().GetStartAt(),
			RunEndAt:    runEndAt,
		})
		err = p.OnUpdateTaskChange(ctx, currentTask, task.TaskStatusRunning)
		if err != nil {
			logs.CtxError(ctx, "OnCreateTaskChange failed, taskID:%d, err:%v", currentTask.GetID(), err)
			return err
		}
	}
	return nil
}

func (p *AutoEvaluteProcessor) OnUpdateTaskChange(ctx context.Context, currentTask *task.Task, taskOp task.TaskStatus) error {
	switch taskOp {
	case task.TaskStatusSuccess:
		if currentTask.GetTaskStatus() != task.TaskStatusDisabled {
			*currentTask.TaskStatus = task.TaskStatusSuccess
		}
	case task.TaskStatusRunning:
		if currentTask.GetTaskStatus() != task.TaskStatusDisabled && currentTask.GetTaskStatus() != task.TaskStatusSuccess {
			*currentTask.TaskStatus = task.TaskStatusRunning
		}
	case task.TaskStatusDisabled:
		if currentTask.GetTaskStatus() != task.TaskStatusDisabled {
			*currentTask.TaskStatus = task.TaskStatusDisabled
		}
	case task.TaskStatusPending:
		if currentTask.GetTaskStatus() == task.TaskStatusPending || currentTask.GetTaskStatus() == task.TaskStatusUnstarted {
			*currentTask.TaskStatus = task.TaskStatusPending
		}
	default:
		return fmt.Errorf("OnUpdateChangeProcessor, valid taskOp:%s", taskOp)
	}
	// 2、更新任务
	taskPO := tconv.TaskDTO2PO(ctx, currentTask, "", nil)
	err := p.taskRepo.UpdateTask(ctx, taskPO)
	if err != nil {
		logs.CtxError(ctx, "[auto_task] OnUpdateChangeProcessor, UpdateTask err, taskID:%d, err:%v", currentTask.GetID(), err)
		return err
	}
	return nil
}

func (p *AutoEvaluteProcessor) OnFinishTaskChange(ctx context.Context, param taskexe.OnFinishTaskChangeReq) error {
	err := p.OnFinishTaskRunChange(ctx, taskexe.OnFinishTaskRunChangeReq{
		Task:    param.Task,
		TaskRun: param.TaskRun,
	})
	if err != nil {
		logs.CtxError(ctx, "OnFinishTaskRunChange failed, taskRun:%+v, err:%v", param.TaskRun, err)
		return err
	}
	if param.IsFinish {
		logs.CtxWarn(ctx, "OnFinishTaskChange, taskID:%d, taskRun:%+v，isFinish:%v", param.Task.GetID(), param.TaskRun, param.IsFinish)
		if err := p.OnUpdateTaskChange(ctx, param.Task, task.TaskStatusSuccess); err != nil {
			logs.CtxError(ctx, "OnUpdateChangeProcessor failed, taskID:%d, err:%v", param.Task.GetID(), err)
			return err
		}
	}
	return nil
}

func (p *AutoEvaluteProcessor) OnCreateTaskRunChange(ctx context.Context, param taskexe.OnCreateTaskRunChangeReq) error {
	//todo:[xun]加锁
	currentTask := param.CurrentTask
	ctx = session.WithCtxUser(ctx, &session.User{ID: currentTask.GetBaseInfo().GetCreatedBy().GetUserID()})
	sessionInfo := p.getSession(ctx, currentTask)
	var evaluationSetColumns []string
	var evaluatorVersionIds []int64
	var evaluatorFieldMappings []*expt.EvaluatorFieldMapping
	evaluationSetColumns = append(evaluationSetColumns, "span_id", "trace_id", "run_id")
	autoEvaluateConfigs := currentTask.GetTaskConfig().GetAutoEvaluateConfigs()
	evaluationSetSchema, fromEvalSet := getBasicEvaluationSetSchema(evaluationSetColumns)
	for _, autoEvaluateConfig := range autoEvaluateConfigs {
		evaluatorVersionIds = append(evaluatorVersionIds, autoEvaluateConfig.EvaluatorVersionID)
		filedMappings := autoEvaluateConfig.GetFieldMappings()
		for _, fieldMapping := range filedMappings {
			if fieldMapping.GetFieldSchema() == nil {
				continue
			}
			fromEvalSet = append(fromEvalSet, &expt.FieldMapping{
				FieldName:     gptr.Of(fieldMapping.GetFieldSchema().GetName()),
				FromFieldName: gptr.Of(fieldMapping.GetEvalSetName()),
			})
			if slices.Contains(evaluationSetColumns, fieldMapping.GetEvalSetName()) {
				continue
			}
			// todo[xun]:原来有历史数据兼容，plain_text 转为 text，需要刷数据，
			evaluationSetSchema.FieldSchemas = append(evaluationSetSchema.FieldSchemas, &dataset0.FieldSchema{
				Key:         gptr.Of(fieldMapping.GetEvalSetName()),
				Name:        gptr.Of(fieldMapping.GetEvalSetName()),
				Description: gptr.Of(fieldMapping.TraceFieldJsonpath),
				ContentType: gptr.Of(fieldMapping.GetFieldSchema().GetContentType()),
				//DefaultDisplayFormat: gptr.Of(dataset.FieldDisplayFormat_PlainText),
				TextSchema: fieldMapping.GetFieldSchema().TextSchema,
				//Hidden:               gptr.Of(false),
			})
			evaluationSetColumns = append(evaluationSetColumns, fieldMapping.GetEvalSetName())
		}

		evaluatorFieldMappings = append(evaluatorFieldMappings, &expt.EvaluatorFieldMapping{
			EvaluatorVersionID: autoEvaluateConfig.GetEvaluatorVersionID(),
			FromEvalSet:        fromEvalSet,
		})
	}
	category := getCategory(currentTask.TaskType)
	schema := convertDatasetSchemaDTO2DO(evaluationSetSchema)
	logs.CtxInfo(ctx, "[auto_task] CreateDataset,category:%s", category)
	var datasetName, exptName string
	if param.RunType == task.TaskRunTypeBackFill {
		datasetName = fmt.Sprintf("自动化任务评测集_历史回溯_%s_%d.%d.%d.%d", currentTask.Name, time.Now().Year(), time.Now().Month(), time.Now().Day(), time.Now().Unix())
		exptName = fmt.Sprintf("自动化任务实验_历史回溯_%s_%d.%d.%d.%d", currentTask.Name, time.Now().Year(), time.Now().Month(), time.Now().Day(), time.Now().Unix())
	} else {
		datasetName = fmt.Sprintf("自动化任务评测集_%s_%d.%d.%d.%d", currentTask.Name, time.Now().Year(), time.Now().Month(), time.Now().Day(), time.Now().Unix())
		exptName = fmt.Sprintf("自动化任务实验_%s_%d.%d.%d.%d", currentTask.Name, time.Now().Year(), time.Now().Month(), time.Now().Day(), time.Now().Unix())
	}
	// 1、创建评测集
	datasetID, err := p.datasetServiceAdaptor.GetDatasetProvider(category).CreateDataset(ctx, entity.NewDataset(
		0,
		currentTask.GetWorkspaceID(),
		datasetName,
		category,
		schema,
		sessionInfo,
	))
	if err != nil {
		logs.CtxError(ctx, "CreateDataset failed, workspace_id=%d, err=%#v", currentTask.GetWorkspaceID(), err)
		return err
	}
	logs.CtxInfo(ctx, "[auto_task] AutoEvaluteProcessor OnChangeProcessor, datasetID:%d", datasetID)
	// 2、创建实验
	maxAliveTime := param.RunEndAt - param.RunStartAt
	submitExperimentReq := rpc.SubmitExperimentReq{
		WorkspaceID:           currentTask.GetWorkspaceID(),
		EvalSetVersionID:      gptr.Of(datasetID),
		EvaluatorVersionIds:   evaluatorVersionIds,
		Name:                  ptr.Of(exptName),
		Desc:                  gptr.Of("自动化任务实验"),
		EvalSetID:             gptr.Of(datasetID),
		EvaluatorFieldMapping: evaluatorFieldMappings,
		TargetFieldMapping: &expt.TargetFieldMapping{
			FromEvalSet: []*expt.FieldMapping{},
		},
		CreateEvalTargetParam: &eval_target.CreateEvalTargetParam{
			SourceTargetID: gptr.Of(strconvh.FormatInt64(currentTask.GetID())),
			EvalTargetType: gptr.Of(eval_target_d.EvalTargetType_Trace),
		},
		ExptType:     gptr.Of(expt.ExptType_Online),
		MaxAliveTime: gptr.Of(maxAliveTime),
		SourceType:   gptr.Of(expt.SourceType_AutoTask),
		SourceID:     gptr.Of(strconvh.FormatInt64(currentTask.GetID())),
		Session:      sessionInfo,
	}
	logs.CtxInfo(ctx, "[auto_task] SubmitExperiment:%+v", submitExperimentReq)
	exptID, exptRunID, err := p.evaluationSvc.SubmitExperiment(ctx, &submitExperimentReq)
	if err != nil {
		logs.CtxError(ctx, "SubmitExperiment failed, workspace_id=%d, err=%#v", currentTask.GetWorkspaceID(), err)
		return err
	}
	logs.CtxInfo(ctx, "[auto_task] AutoEvaluteProcessor OnChangeProcessor, exptID:%d, exptRunID:%d", exptID, exptRunID)

	evaluationSetConfig, err := p.datasetServiceAdaptor.GetDatasetProvider(category).GetDataset(ctx, currentTask.GetWorkspaceID(), datasetID, category)
	if err != nil {
		logs.CtxError(ctx, "[task-debug] GetEvaluationSet err:%v", err)
		return err
	}

	// 5、创建 taskrun
	taskRunConfig := &task.TaskRunConfig{
		AutoEvaluateRunConfig: &task.AutoEvaluateRunConfig{
			ExptID:       exptID,
			ExptRunID:    exptRunID,
			EvalID:       datasetID,
			SchemaID:     evaluationSetConfig.DatasetVersion.DatasetSchema.ID,
			Schema:       ptr.Of(ToJSONString(ctx, evaluationSetConfig.DatasetVersion.DatasetSchema.FieldSchemas)),
			EndAt:        param.RunEndAt,
			CycleStartAt: param.RunStartAt,
			CycleEndAt:   param.RunEndAt,
			Status:       task.TaskStatusRunning,
		},
	}
	taskRun := &task_entity.TaskRun{
		TaskID:      currentTask.GetID(),
		WorkspaceID: currentTask.GetWorkspaceID(),
		TaskType:    param.RunType,
		RunStatus:   task.RunStatusRunning,
		RunStartAt:  time.UnixMilli(param.RunStartAt),
		RunEndAt:    time.UnixMilli(param.RunEndAt),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		RunConfig:   ptr.Of(ToJSONString(ctx, taskRunConfig)),
	}
	id, err := p.taskRepo.CreateTaskRun(ctx, taskRun)
	if err != nil {
		logs.CtxError(ctx, "[auto_task] OnCreateTaskRunProcessor, CreateTaskRun err, taskRun:%+v, err:%v", taskRun, err)
		return err
	}
	taskRun.ID = id
	taskConfig, err := p.taskRepo.GetTask(ctx, currentTask.GetID(), nil, nil)
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

func (p *AutoEvaluteProcessor) OnFinishTaskRunChange(ctx context.Context, param taskexe.OnFinishTaskRunChangeReq) error {
	session := p.getSession(ctx, param.Task)
	taskRun := param.TaskRun
	taskRunPO := tconv.TaskRunPO2DTO(ctx, taskRun, nil)
	if err := p.evaluationSvc.FinishExperiment(ctx, &rpc.FinishExperimentReq{
		WorkspaceID:     param.Task.GetWorkspaceID(),
		ExperimentID:    taskRunPO.GetTaskRunConfig().GetAutoEvaluateRunConfig().GetExptID(),
		ExperimentRunID: taskRunPO.GetTaskRunConfig().GetAutoEvaluateRunConfig().GetExptRunID(),
		Session:         session,
	}); err != nil {
		return err
	}
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

func (p *AutoEvaluteProcessor) getSession(ctx context.Context, task *task.Task) *common.Session {
	userIDStr := session.UserIDInCtxOrEmpty(ctx)
	if userIDStr == "" {
		userIDStr = task.GetBaseInfo().GetCreatedBy().GetUserID()
	}
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		logs.CtxError(ctx, "[task-debug] AutoEvaluteProcessor OnChangeProcessor, ParseInt err:%v", err)
	}
	return &common.Session{
		UserID: gptr.Of(userID),
		AppID:  gptr.Of(p.aid),
	}
}
