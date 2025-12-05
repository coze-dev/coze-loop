// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package processor

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_set"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
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
	"github.com/spf13/cast"
)

var _ taskexe.Processor = (*AutoEvaluateProcessor)(nil)

type AutoEvaluateProcessor struct {
	evalSvc               rpc.IEvaluatorRPCAdapter
	evaluationSvc         rpc.IEvaluationRPCAdapter
	datasetServiceAdaptor *service.DatasetServiceAdaptor
	taskRepo              repo.ITaskRepo
	aid                   int32
	evalTargetBuilder     EvalTargetBuilder
}

func NewAutoEvaluateProcessor(
	aid int32,
	datasetServiceProvider *service.DatasetServiceAdaptor,
	evalService rpc.IEvaluatorRPCAdapter,
	evaluationService rpc.IEvaluationRPCAdapter,
	taskRepo repo.ITaskRepo,
	evalTargetBuilder EvalTargetBuilder,
) *AutoEvaluateProcessor {
	return &AutoEvaluateProcessor{
		datasetServiceAdaptor: datasetServiceProvider,
		evalSvc:               evalService,
		evaluationSvc:         evaluationService,
		taskRepo:              taskRepo,
		aid:                   aid,
		evalTargetBuilder:     evalTargetBuilder,
	}
}

func (p *AutoEvaluateProcessor) ValidateConfig(ctx context.Context, config any) error {
	cfg, ok := config.(*task_entity.ObservabilityTask)
	if !ok {
		return errorx.NewByCode(obErrorx.CommonInvalidParamCode)
	}
	if cfg.EffectiveTime != nil {
		startAt := cfg.EffectiveTime.StartAt
		endAt := cfg.EffectiveTime.EndAt
		if startAt <= time.Now().Add(-10*time.Minute).UnixMilli() {
			return errorx.NewByCode(obErrorx.CommonInvalidParamCode)
		}
		if startAt >= endAt {
			return errorx.NewByCode(obErrorx.CommonInvalidParamCode)
		}
	}
	var evaluatorVersionIDs []int64
	for _, autoEvaluateConfig := range cfg.TaskConfig.AutoEvaluateConfigs {
		evaluatorVersionIDs = append(evaluatorVersionIDs, autoEvaluateConfig.EvaluatorVersionID)
	}
	if len(evaluatorVersionIDs) == 0 {
		return errorx.NewByCode(obErrorx.CommonInvalidParamCode)
	}
	// Verify evaluator version validity
	evaluators, _, err := p.evalSvc.BatchGetEvaluatorVersions(ctx, &rpc.BatchGetEvaluatorVersionsParam{
		WorkspaceID:         cfg.WorkspaceID,
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

func (p *AutoEvaluateProcessor) Invoke(ctx context.Context, trigger *taskexe.Trigger) error {
	taskRun := tconv.TaskRunDO2DTO(ctx, trigger.TaskRun, nil)
	if taskRun.GetTaskRunConfig().GetAutoEvaluateRunConfig() == nil {
		return nil
	}
	workspaceID := trigger.Task.WorkspaceID
	sessionInfo := p.getSession(ctx, trigger.Task)
	var mapping []*task_entity.EvaluateFieldMapping
	for _, autoEvaluateConfig := range trigger.Task.TaskConfig.AutoEvaluateConfigs {
		mapping = append(mapping, autoEvaluateConfig.FieldMappings...)
	}
	turns := buildItems(ctx, []*loop_span.Span{trigger.Span}, mapping, taskRun.GetTaskRunConfig().GetAutoEvaluateRunConfig().GetSchema(), strconv.FormatInt(taskRun.ID, 10))
	if len(turns) == 0 {
		logs.CtxInfo(ctx, "[task-debug] AutoEvaluateProcessor Invoke, turns is empty")
		return nil
	}
	taskTTL := trigger.Task.GetTaskttl()
	_ = p.taskRepo.IncrTaskCount(ctx, trigger.Task.ID, taskTTL)
	_ = p.taskRepo.IncrTaskRunCount(ctx, trigger.Task.ID, taskRun.ID, taskTTL)
	taskCount, _ := p.taskRepo.GetTaskCount(ctx, trigger.Task.ID)
	taskRunCount, _ := p.taskRepo.GetTaskRunCount(ctx, trigger.Task.ID, taskRun.ID)
	if (trigger.Task.Sampler.CycleCount != 0 && taskRunCount > trigger.Task.Sampler.CycleCount) ||
		(taskCount > trigger.Task.Sampler.SampleSize) {
		logs.CtxInfo(ctx, "[task-debug] AutoEvaluateProcessor Invoke, subCount:%v,taskCount:%v", taskRunCount, taskCount)
		_ = p.taskRepo.DecrTaskCount(ctx, trigger.Task.ID, taskTTL)
		_ = p.taskRepo.DecrTaskRunCount(ctx, trigger.Task.ID, taskRun.ID, taskTTL)
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
		Session:          sessionInfo,
		Ext: map[string]string{
			"workspace_id": strconv.FormatInt(trigger.Task.WorkspaceID, 10),
			"span_id":      trigger.Span.SpanID,
			"task_id":      cast.ToString(trigger.Task.ID),
			"task_run_id":  cast.ToString(taskRun.ID),
		},
	})
	if err != nil {
		_ = p.taskRepo.DecrTaskCount(ctx, trigger.Task.ID, taskTTL)
		_ = p.taskRepo.DecrTaskRunCount(ctx, trigger.Task.ID, taskRun.ID, taskTTL)
		return err
	}
	return nil
}

func (p *AutoEvaluateProcessor) OnTaskCreated(ctx context.Context, currentTask *task_entity.ObservabilityTask) error {
	taskRuns, err := p.taskRepo.GetBackfillTaskRun(ctx, nil, currentTask.ID)
	if err != nil {
		logs.CtxError(ctx, "GetBackfillTaskRun failed, taskID:%d, err:%v", currentTask.ID, err)
		return err
	}
	if ShouldTriggerBackfill(currentTask) && taskRuns == nil {
		err = p.OnTaskRunCreated(ctx, taskexe.OnTaskRunCreatedReq{
			CurrentTask: currentTask,
			RunType:     task_entity.TaskRunTypeBackFill,
			RunStartAt:  time.Now().UnixMilli(),
			RunEndAt:    time.Now().UnixMilli() + (currentTask.BackfillEffectiveTime.EndAt - currentTask.BackfillEffectiveTime.StartAt),
		})
		if err != nil {
			logs.CtxError(ctx, "OnTaskCreated failed, taskID:%d, err:%v", currentTask.ID, err)
			return err
		}
		err = p.OnTaskUpdated(ctx, currentTask, task.TaskStatusRunning)
		if err != nil {
			logs.CtxError(ctx, "OnTaskCreated failed, taskID:%d, err:%v", currentTask.ID, err)
			return err
		}
	}
	if ShouldTriggerNewData(ctx, currentTask) {
		runStartAt, runEndAt := currentTask.GetRunTimeRange()
		err = p.OnTaskRunCreated(ctx, taskexe.OnTaskRunCreatedReq{
			CurrentTask: currentTask,
			RunType:     task_entity.TaskRunTypeNewData,
			RunStartAt:  runStartAt,
			RunEndAt:    runEndAt,
		})
		if err != nil {
			logs.CtxError(ctx, "OnTaskCreated failed, taskID:%d, err:%v", currentTask.ID, err)
			return err
		}
		err = p.OnTaskUpdated(ctx, currentTask, task.TaskStatusRunning)
		if err != nil {
			logs.CtxError(ctx, "OnTaskCreated failed, taskID:%d, err:%v", currentTask.ID, err)
			return err
		}
	}
	return nil
}

func (p *AutoEvaluateProcessor) OnTaskUpdated(ctx context.Context, currentTask *task_entity.ObservabilityTask, taskOp task_entity.TaskStatus) error {
	switch taskOp {
	case task_entity.TaskStatusSuccess:
		if currentTask.TaskStatus != task_entity.TaskStatusDisabled {
			currentTask.TaskStatus = task_entity.TaskStatusSuccess
		}
	case task_entity.TaskStatusRunning:
		if currentTask.TaskStatus != task_entity.TaskStatusDisabled && currentTask.TaskStatus != task_entity.TaskStatusSuccess {
			currentTask.TaskStatus = task_entity.TaskStatusRunning
		}
	case task_entity.TaskStatusDisabled:
		if currentTask.TaskStatus != task_entity.TaskStatusDisabled {
			currentTask.TaskStatus = task_entity.TaskStatusDisabled
		}
	case task_entity.TaskStatusPending:
		if currentTask.TaskStatus == task_entity.TaskStatusPending || currentTask.TaskStatus == task_entity.TaskStatusUnstarted {
			currentTask.TaskStatus = task_entity.TaskStatusPending
		}
	default:
		return fmt.Errorf("OnUpdateChangeProcessor, valid taskOp:%s", taskOp)
	}
	// Step 2: update task
	err := p.taskRepo.UpdateTask(ctx, currentTask)
	if err != nil {
		logs.CtxError(ctx, "[auto_task] OnUpdateChangeProcessor, UpdateTask err, taskID:%d, err:%v", currentTask.ID, err)
		return err
	}
	return nil
}

func (p *AutoEvaluateProcessor) OnTaskFinished(ctx context.Context, param taskexe.OnTaskFinishedReq) error {
	err := p.OnTaskRunFinished(ctx, taskexe.OnTaskRunFinishedReq{
		Task:    param.Task,
		TaskRun: param.TaskRun,
	})
	if err != nil {
		logs.CtxError(ctx, "OnTaskRunFinished failed, taskRun:%+v, err:%v", param.TaskRun, err)
		return err
	}
	if param.IsFinish {
		logs.CtxWarn(ctx, "OnTaskFinished, taskID:%d, taskRun:%+v, isFinish:%v", param.Task.ID, param.TaskRun, param.IsFinish)
		if err := p.OnTaskUpdated(ctx, param.Task, task.TaskStatusSuccess); err != nil {
			logs.CtxError(ctx, "OnUpdateChangeProcessor failed, taskID:%d, err:%v", param.Task.ID, err)
			return err
		}
		if err := p.taskRepo.RemoveNonFinalTask(ctx, strconv.FormatInt(param.Task.WorkspaceID, 10), param.Task.ID); err != nil {
			logs.CtxError(ctx, "RemoveNonFinalTask failed, taskID:%d, err:%v", param.Task.ID, err)
			return err
		}
	}
	return nil
}

const (
	AutoEvaluateCN   = "自动化任务实验"
	AutoEvaluateI18N = "AutoEvaluate"
	BackFillCN       = "历史回溯"
	BackFillI18N     = "BackFill"
)

func (p *AutoEvaluateProcessor) OnTaskRunCreated(ctx context.Context, param taskexe.OnTaskRunCreatedReq) error {
	currentTask := param.CurrentTask
	ctx = session.WithCtxUser(ctx, &session.User{ID: currentTask.CreatedBy})
	sessionInfo := p.getSession(ctx, currentTask)
	var evaluationSetColumns []string
	var evaluatorVersionIds []int64
	var evaluatorFieldMappings []*expt.EvaluatorFieldMapping
	evaluationSetColumns = append(evaluationSetColumns, "span_id", "trace_id", "run_id")
	autoEvaluateConfigs := currentTask.TaskConfig.AutoEvaluateConfigs
	evaluationSetSchema, fromEvalSet := getBasicEvaluationSetSchema(evaluationSetColumns)
	for _, autoEvaluateConfig := range autoEvaluateConfigs {
		evaluatorVersionIds = append(evaluatorVersionIds, autoEvaluateConfig.EvaluatorVersionID)
		filedMappings := autoEvaluateConfig.FieldMappings
		for _, fieldMapping := range filedMappings {
			if fieldMapping.FieldSchema == nil {
				continue
			}
			fromEvalSet = append(fromEvalSet, &expt.FieldMapping{
				FieldName:     fieldMapping.FieldSchema.Name,
				FromFieldName: fieldMapping.EvalSetName,
			})
			if slices.Contains(evaluationSetColumns, *fieldMapping.EvalSetName) {
				continue
			}
			// historical data compatibility, convert plain_text to text, data needs to be refreshed
			evaluationSetSchema.FieldSchemas = append(evaluationSetSchema.FieldSchemas, &dataset0.FieldSchema{
				Key:         gptr.Of(*fieldMapping.EvalSetName),
				Name:        gptr.Of(*fieldMapping.EvalSetName),
				Description: gptr.Of(fieldMapping.TraceFieldJsonpath),
				ContentType: fieldMapping.FieldSchema.ContentType,
				// DefaultDisplayFormat: gptr.Of(dataset.FieldDisplayFormat_PlainText),
				TextSchema: fieldMapping.FieldSchema.TextSchema,
				// Hidden:               gptr.Of(false),
			})
			evaluationSetColumns = append(evaluationSetColumns, *fieldMapping.EvalSetName)
		}

		evaluatorFieldMappings = append(evaluatorFieldMappings, &expt.EvaluatorFieldMapping{
			EvaluatorVersionID: autoEvaluateConfig.EvaluatorVersionID,
			FromEvalSet:        fromEvalSet,
		})
	}
	category := getCategory(task.TaskType(currentTask.TaskType))
	schema := convertDatasetSchemaDTO2DO(evaluationSetSchema)
	logs.CtxInfo(ctx, "[auto_task] CreateDataset,category:%s", category)
	var datasetName, exptName string
	if param.RunType == task_entity.TaskRunTypeBackFill {
		datasetName = fmt.Sprintf("%s_%s_%s_%d.%d.%d.%d", AutoEvaluateCN, BackFillCN, currentTask.Name, time.Now().Year(), time.Now().Month(), time.Now().Day(), time.Now().Unix())
		exptName = fmt.Sprintf("%s_%s_%s_%d.%d.%d.%d", AutoEvaluateCN, BackFillCN, currentTask.Name, time.Now().Year(), time.Now().Month(), time.Now().Day(), time.Now().Unix())
	} else {
		datasetName = fmt.Sprintf("%s_%s_%d.%d.%d.%d", AutoEvaluateCN, currentTask.Name, time.Now().Year(), time.Now().Month(), time.Now().Day(), time.Now().Unix())
		exptName = fmt.Sprintf("%s_%s_%d.%d.%d.%d", AutoEvaluateCN, currentTask.Name, time.Now().Year(), time.Now().Month(), time.Now().Day(), time.Now().Unix())
	}
	// Step 1: create evaluation dataset
	datasetID, err := p.datasetServiceAdaptor.GetDatasetProvider(category).CreateDataset(ctx, entity.NewDataset(
		0,
		currentTask.WorkspaceID,
		datasetName,
		category,
		schema,
		sessionInfo,
		ptr.Of(entity.BizCategoryFromOnlineTrace),
	))
	if err != nil {
		logs.CtxError(ctx, "CreateDataset failed, workspace_id=%d, err=%#v", currentTask.WorkspaceID, err)
		return err
	}
	logs.CtxInfo(ctx, "[auto_task] AutoEvaluateProcessor OnChangeProcessor, datasetID:%d", datasetID)
	// Step 2: create experiment
	maxAliveTime := param.RunEndAt - param.RunStartAt
	submitExperimentReq := rpc.SubmitExperimentReq{
		WorkspaceID:           currentTask.WorkspaceID,
		EvalSetVersionID:      gptr.Of(datasetID),
		EvaluatorVersionIds:   evaluatorVersionIds,
		Name:                  ptr.Of(exptName),
		Desc:                  gptr.Of("Auto Task Experiment"),
		EvalSetID:             gptr.Of(datasetID),
		EvaluatorFieldMapping: evaluatorFieldMappings,
		TargetFieldMapping: &expt.TargetFieldMapping{
			FromEvalSet: []*expt.FieldMapping{},
		},
		CreateEvalTargetParam: p.evalTargetBuilder.Build(ctx, currentTask),
		ExptType:              gptr.Of(expt.ExptType_Online),
		MaxAliveTime:          gptr.Of(maxAliveTime),
		SourceType:            gptr.Of(expt.SourceType_AutoTask),
		SourceID:              gptr.Of(cast.ToString(currentTask.ID)),
		Session:               sessionInfo,
	}
	logs.CtxInfo(ctx, "[auto_task] SubmitExperiment:%+v", submitExperimentReq)
	exptID, exptRunID, err := p.evaluationSvc.SubmitExperiment(ctx, &submitExperimentReq)
	if err != nil {
		logs.CtxError(ctx, "SubmitExperiment failed, workspace_id=%d, err=%#v", currentTask.WorkspaceID, err)
		return err
	}
	logs.CtxInfo(ctx, "[auto_task] AutoEvaluateProcessor OnChangeProcessor, exptID:%d, exptRunID:%d", exptID, exptRunID)

	evaluationSetConfig, err := p.datasetServiceAdaptor.GetDatasetProvider(category).GetDataset(ctx, currentTask.WorkspaceID, datasetID, category)
	if err != nil {
		logs.CtxError(ctx, "[task-debug] GetEvaluationSet err:%v", err)
		return err
	}

	// Step 5: create task run
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
		TaskID:        currentTask.ID,
		WorkspaceID:   currentTask.WorkspaceID,
		TaskType:      param.RunType,
		RunStatus:     task_entity.TaskRunStatusRunning,
		RunStartAt:    time.UnixMilli(param.RunStartAt),
		RunEndAt:      time.UnixMilli(param.RunEndAt),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		TaskRunConfig: tconv.TaskRunConfigDTO2DO(taskRunConfig),
	}
	_, err = p.taskRepo.CreateTaskRun(ctx, taskRun)
	if err != nil {
		logs.CtxError(ctx, "[auto_task] OnCreateTaskRunProcessor, CreateTaskRun err, taskRun:%+v, err:%v", taskRun, err)
		return err
	}
	return nil
}

func (p *AutoEvaluateProcessor) OnTaskRunFinished(ctx context.Context, param taskexe.OnTaskRunFinishedReq) error {
	if param.TaskRun == nil || param.TaskRun.TaskRunConfig == nil || param.TaskRun.TaskRunConfig.AutoEvaluateRunConfig == nil {
		return nil
	}
	session := p.getSession(ctx, param.Task)
	taskRun := param.TaskRun
	if err := p.evaluationSvc.FinishExperiment(ctx, &rpc.FinishExperimentReq{
		WorkspaceID:     param.Task.WorkspaceID,
		ExperimentID:    taskRun.TaskRunConfig.AutoEvaluateRunConfig.ExptID,
		ExperimentRunID: taskRun.TaskRunConfig.AutoEvaluateRunConfig.ExptRunID,
		Session:         session,
	}); err != nil {
		return err
	}
	// Set task run status to completed
	taskRun.RunStatus = task.RunStatusDone
	// Update task run
	err := p.taskRepo.UpdateTaskRun(ctx, taskRun)
	if err != nil {
		logs.CtxError(ctx, "[auto_task] OnFinishTaskRunProcessor, UpdateTaskRun err, taskRunID:%d, err:%v", taskRun.ID, err)
		return err
	}
	return nil
}

func (p *AutoEvaluateProcessor) getSession(ctx context.Context, task *task_entity.ObservabilityTask) *common.Session {
	userIDStr := session.UserIDInCtxOrEmpty(ctx)
	if userIDStr == "" {
		userIDStr = task.CreatedBy
	}
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		logs.CtxError(ctx, "[task-debug] AutoEvaluateProcessor OnChangeProcessor, ParseInt err:%v", err)
	}
	return &common.Session{
		UserID: gptr.Of(userID),
		AppID:  gptr.Of(p.aid),
	}
}
