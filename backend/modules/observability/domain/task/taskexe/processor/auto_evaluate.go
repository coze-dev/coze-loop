// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package processor

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/bytedance/sonic"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	tconv "github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor/task"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"

	"github.com/apaxa-go/helper/strconvh"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_set"
	eval_target_d "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_target"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_target"
	dataset0 "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	task_entity "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/taskexe"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/slices"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"github.com/coze-dev/cozeloop-go/spec/tracespec"
)

var _ taskexe.Processor = (*AutoEvaluteProcessor)(nil)

type AutoEvaluteProcessor struct {
	evalSvc               rpc.IEvaluatorRPCAdapter
	evaluationSvc         rpc.IEvaluationRPCAdapter
	datasetServiceAdaptor *service.DatasetServiceAdaptor
	taskRepo              repo.ITaskRepo
	taskRunRepo           repo.ITaskRunRepo
}

func newAutoEvaluteProcessor(
	datasetServiceProvider *service.DatasetServiceAdaptor,
	evalService rpc.IEvaluatorRPCAdapter,
	evaluationService rpc.IEvaluationRPCAdapter,
	taskRepo repo.ITaskRepo,
	taskRunRepo repo.ITaskRunRepo) *AutoEvaluteProcessor {
	return &AutoEvaluteProcessor{
		datasetServiceAdaptor: datasetServiceProvider,
		evalSvc:               evalService,
		evaluationSvc:         evaluationService,
		taskRepo:              taskRepo,
		taskRunRepo:           taskRunRepo,
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
	session := getSession(ctx, trigger.Task)
	var mapping []*task.EvaluateFieldMapping
	for _, autoEvaluateConfig := range trigger.Task.TaskConfig.AutoEvaluateConfigs {
		mapping = append(mapping, autoEvaluateConfig.FieldMappings...)
	}
	turns := buildItems(ctx, []*loop_span.Span{trigger.Span}, mapping, taskRun.GetTaskRunConfig().GetAutoEvaluateRunConfig().GetSchema())
	if len(turns) == 0 {
		logs.CtxInfo(ctx, "[task-debug] AutoEvaluteProcessor Invoke, turns is empty")
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
		Ext:              map[string]string{"workspace_id": strconv.FormatInt(workspaceID, 10), "span_id": trigger.Span.SpanID, "trace_id": trigger.Span.TraceID, "start_time": strconvh.FormatInt64(trigger.Span.StartTime), "task_id": strconvh.FormatInt64(trigger.Task.GetID()), "task_run_id": strconvh.FormatInt64(taskRun.ID), "platform_type": trigger.Task.GetRule().GetSpanFilters().GetPlatformType()},
		Session:          session,
	})
	if err != nil {
		return err
	}
	return nil
}

func (p *AutoEvaluteProcessor) Finish(ctx context.Context, config any, trigger *taskexe.Trigger) error {
	//todo:[xun]加锁
	session := getSession(ctx, trigger.Task)
	cfg, ok := config.(*task.TaskRun)
	if !ok {
		return taskexe.ErrInvalidConfig
	}
	if err := p.evaluationSvc.FinishExperiment(ctx, &rpc.FinishExperimentReq{
		WorkspaceID:     trigger.Task.GetWorkspaceID(),
		ExperimentID:    cfg.GetTaskRunConfig().GetAutoEvaluateRunConfig().GetExptID(),
		ExperimentRunID: cfg.GetTaskRunConfig().GetAutoEvaluateRunConfig().GetExptRunID(),
		Session:         session,
	}); err != nil {
		return err
	}
	//todo:[xun]根据是否是真的结束实验做处理
	return nil
}

func (p *AutoEvaluteProcessor) OnChangeProcessor(ctx context.Context, currentTask *task.Task, taskOp task.TaskStatus) error {
	logs.CtxInfo(ctx, "[auto_task] AutoEvaluteProcessor OnChangeProcessor, taskID:%d, taskOp:%s, task:%+v", currentTask.GetID(), taskOp, currentTask)
	//todo:[xun]加锁
	ctx = session.WithCtxUser(ctx, &session.User{ID: currentTask.GetBaseInfo().GetCreatedBy().GetUserID()})
	sessionInfo := getSession(ctx, currentTask)
	var evaluationSetColumns []string
	var evaluatorVersionIds []int64
	var evaluatorFieldMappings []*expt.EvaluatorFieldMapping
	evaluationSetColumns = append(evaluationSetColumns, "span_id", "trace_id")
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
	// 1、创建评测集
	logs.CtxInfo(ctx, "[auto_task] CreateDataset,category:%s", category)
	datasetID, err := p.datasetServiceAdaptor.GetDatasetProvider(category).CreateDataset(ctx, entity.NewDataset(
		0,
		currentTask.GetWorkspaceID(),
		fmt.Sprintf("自动化任务评测集_%s_%d.%d.%d", currentTask.Name, time.Now().Year(), time.Now().Month(), time.Now().Day()),
		category,
		schema,
		sessionInfo,
	))
	if err != nil {
		logs.CtxError(ctx, "CreateDataset failed, workspace_id=%d, err=%#v", currentTask.GetWorkspaceID(), err)
		return err
		//datasetID = 7548288691995672577
	}
	logs.CtxInfo(ctx, "[auto_task] AutoEvaluteProcessor OnChangeProcessor, datasetID:%d", datasetID)
	// 2、创建实验
	maxAliveTime := currentTask.GetRule().GetEffectiveTime().GetEndAt() - currentTask.GetRule().GetEffectiveTime().GetStartAt()
	if currentTask.GetRule().GetSampler().GetIsCycle() {
		switch *currentTask.GetRule().GetSampler().CycleTimeUnit {
		case task.TimeUnitDay:
			maxAliveTime = (*currentTask.GetRule().GetSampler().CycleInterval) * 24 * time.Hour.Milliseconds()
		case task.TimeUnitWeek:
			maxAliveTime = (*currentTask.GetRule().GetSampler().CycleInterval) * 7 * 24 * time.Hour.Milliseconds()
		default:
			maxAliveTime = (*currentTask.GetRule().GetSampler().CycleInterval) * 10 * time.Minute.Milliseconds()
		}
	}
	submitExperimentReq := rpc.SubmitExperimentReq{
		WorkspaceID:           currentTask.GetWorkspaceID(),
		EvalSetVersionID:      gptr.Of(datasetID),
		EvaluatorVersionIds:   evaluatorVersionIds,
		Name:                  gptr.Of(fmt.Sprintf("自动化任务实验_%s_%d.%d.%d", currentTask.Name, time.Now().Year(), time.Now().Month(), time.Now().Day())),
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
	// 3、更新任务状态
	//if currentTask.GetTaskStatus() == task.TaskStatusUnstarted {
	//	updateMap := map[string]interface{}{
	//		"task_status": task.TaskStatusRunning,
	//	}
	//	logs.CtxInfo(ctx, "currentTask.GetID():%d, currentTask.GetWorkspaceID():%d", currentTask.GetID(), currentTask.GetWorkspaceID())
	//	err = p.taskRepo.UpdateTaskWithOCC(ctx, currentTask.GetID(), currentTask.GetWorkspaceID(), updateMap)
	//	if err != nil {
	//		return err
	//	}
	//}
	// 4、更新任务配置
	effectiveTime := currentTask.GetRule().GetEffectiveTime()
	taskConfig, err := p.taskRepo.GetTask(ctx, currentTask.GetID(), nil, nil)
	if err != nil {
		return err
	}
	taskConfig.TaskStatus = task.TaskStatusRunning

	var cycleStartAt, cycleEndAt, endAt int64
	currentTime := time.Now().UnixMilli()

	if effectiveTime.StartAt != nil && effectiveTime.EndAt != nil {
		endAt = effectiveTime.GetEndAt()
		if len(taskConfig.TaskRuns) == 0 {
			// 首次创建 taskrun，从任务生效时间开始
			cycleStartAt = resetStartTime(currentTime, effectiveTime.GetStartAt(), maxAliveTime)
		} else {
			// 找到最新的 cycleEndAt 作为新的 cycleStartAt
			for _, run := range taskConfig.TaskRuns {
				if run.RunStartAt.UnixMilli() > cycleStartAt {
					cycleStartAt = run.RunEndAt.UnixMilli()
				}
			}
			cycleStartAt = resetStartTime(currentTime, cycleStartAt, maxAliveTime)
		}
		cycleEndAt = cycleStartAt + maxAliveTime

		// 确保周期开始时间不早于任务生效时间
		if cycleStartAt < effectiveTime.GetStartAt() {
			cycleStartAt = effectiveTime.GetStartAt()
			cycleEndAt = cycleStartAt + maxAliveTime
		}

		// 确保周期结束时间不晚于任务结束时间
		if cycleEndAt > effectiveTime.GetEndAt() {
			cycleEndAt = effectiveTime.GetEndAt()
		}
	}

	logs.CtxInfo(ctx, "Creating taskrun with cycle: startAt=%d, endAt=%d, currentTime=%d", cycleStartAt, cycleEndAt, currentTime)
	// 5、创建 taskrun
	taskRunConfig := &task.TaskRunConfig{
		AutoEvaluateRunConfig: &task.AutoEvaluateRunConfig{
			ExptID:       exptID,
			ExptRunID:    exptRunID,
			EvalID:       datasetID,
			SchemaID:     evaluationSetConfig.DatasetVersion.DatasetSchema.ID,
			Schema:       ptr.Of(ToJSONString(ctx, evaluationSetConfig.DatasetVersion.DatasetSchema.FieldSchemas)),
			EndAt:        endAt,
			CycleStartAt: cycleStartAt,
			CycleEndAt:   cycleEndAt,
			Status:       task.TaskStatusRunning,
		},
	}
	taskRun := &task_entity.TaskRun{
		TaskID:      currentTask.GetID(),
		WorkspaceID: currentTask.GetWorkspaceID(),
		TaskType:    currentTask.GetTaskType(),
		RunStatus:   task.RunStatusRunning,
		RunStartAt:  time.UnixMilli(cycleStartAt),
		RunEndAt:    time.UnixMilli(cycleEndAt),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		RunConfig:   ptr.Of(ToJSONString(ctx, taskRunConfig)),
	}

	// 6、更新任务配置
	// todo:[xun]改task_run?
	_, err = p.taskRunRepo.CreateTaskRun(ctx, taskRun)
	err = p.taskRepo.UpdateTask(ctx, taskConfig)
	if err != nil {
		return err
	}
	return nil

}

func ToJSONString(ctx context.Context, obj interface{}) string {
	if obj == nil {
		return ""
	}
	jsonData, err := sonic.Marshal(obj)
	if err != nil {
		logs.CtxError(ctx, "JSON marshal error: %v", err)
		return ""
	}
	jsonStr := string(jsonData)
	return jsonStr
}

func resetStartTime(currentTime int64, originalStartTime int64, maxAliveTime int64) int64 {
	if currentTime > originalStartTime {
		// 计算需要跳过的周期数
		timeDiff := currentTime - originalStartTime
		skipCycles := timeDiff / maxAliveTime

		// 跳过过期的时间段，直接计算新的周期开始时间
		return originalStartTime + (skipCycles * maxAliveTime)
	}
	return originalStartTime
}

func getSession(ctx context.Context, task *task.Task) *common.Session {
	userID, err := strconv.ParseInt(task.BaseInfo.CreatedBy.GetUserID(), 10, 64)
	if err != nil {
		logs.CtxError(ctx, "[task-debug] AutoEvaluteProcessor OnChangeProcessor, ParseInt err:%v", err)
	}
	return &common.Session{
		UserID: gptr.Of(userID),
		//AppID:  gptr.Of(int32(717152)),
	}
}
func getBasicEvaluationSetSchema(basicColumns []string) (*dataset0.DatasetSchema, []*expt.FieldMapping) {
	evaluationSetSchema := dataset0.NewDatasetSchema()
	var fromEvalSet []*expt.FieldMapping
	for _, column := range basicColumns {
		evaluationSetSchema.FieldSchemas = append(evaluationSetSchema.FieldSchemas, &dataset0.FieldSchema{
			Key:         gptr.Of(column),
			Name:        gptr.Of(column),
			Description: gptr.Of(column),
			ContentType: gptr.Of(common.ContentTypeText),
			TextSchema:  gptr.Of("{\"type\": \"string\"}"),
		})
		fromEvalSet = append(fromEvalSet, &expt.FieldMapping{
			FieldName:     gptr.Of(column),
			FromFieldName: gptr.Of(column),
		})
	}
	return evaluationSetSchema, fromEvalSet
}

// todo:[xun]和手动回流的代码逻辑一样，需要抽取公共代码
// convertDatasetSchemaDTO2DO 转换数据集模式
func convertDatasetSchemaDTO2DO(schema *dataset0.DatasetSchema) entity.DatasetSchema {
	if schema == nil {
		return entity.DatasetSchema{}
	}

	result := entity.DatasetSchema{}

	if schema.IsSetFieldSchemas() {
		fieldSchemas := schema.GetFieldSchemas()
		result.FieldSchemas = make([]entity.FieldSchema, len(fieldSchemas))
		for i, fs := range fieldSchemas {
			key := fs.GetKey()
			if key == "" {
				key = fs.GetName()
			}
			name := fs.GetName()
			description := fs.GetDescription()
			textSchema := fs.GetTextSchema()
			result.FieldSchemas[i] = entity.FieldSchema{
				Key:         &key,
				Name:        name,
				Description: description,
				ContentType: convertContentTypeDTO2DO(fs.GetContentType()),
				TextSchema:  textSchema,
			}
		}
	}

	return result
}

// todo:[xun]和手动回流的代码逻辑一样，需要抽取公共代码
// convertContentTypeDTO2DO 转换内容类型
func convertContentTypeDTO2DO(contentType common.ContentType) entity.ContentType {
	switch contentType {
	case common.ContentTypeText:
		return entity.ContentType_Text
	case common.ContentTypeImage:
		return entity.ContentType_Image
	case common.ContentTypeAudio:
		return entity.ContentType_Audio
	case common.ContentTypeMultiPart:
		return entity.ContentType_MultiPart
	default:
		return entity.ContentType_Text
	}
}

func getCategory(taskType task.TaskType) entity.DatasetCategory {
	switch taskType {
	case task.TaskTypeAutoEval:
		return entity.DatasetCategory_Evaluation
	default:
		return entity.DatasetCategory_General
	}
}

// todo:[xun]和手动回流的代码逻辑一样，需要抽取公共代码
func buildItems(ctx context.Context, spans []*loop_span.Span, fieldMappings []*task.EvaluateFieldMapping,
	evaluationSetSchema string) (turns []*eval_set.Turn) {
	turns = make([]*eval_set.Turn, 0, len(spans))
	for _, span := range spans {
		fieldData := buildItem(ctx, span, fieldMappings, evaluationSetSchema)
		if len(fieldData) == 0 {
			continue
		}
		turns = append(turns, &eval_set.Turn{
			FieldDataList: fieldData,
		})
	}
	return turns
}

func buildDatasetItems(ctx context.Context, spans []*loop_span.Span, fieldMappings []entity.FieldMapping,
	workspaceID, taskID int64, dataset *entity.Dataset,
) (successItems, failedItems, allItems []*entity.DatasetItem) {
	successItems = make([]*entity.DatasetItem, 0, len(spans))
	failedItems = make([]*entity.DatasetItem, 0)
	allItems = make([]*entity.DatasetItem, 0, len(spans))
	for i, span := range spans {
		item := buildDatasetItem(ctx, span, i, fieldMappings, workspaceID, taskID, dataset)
		allItems = append(allItems, item)
		if len(item.Error) > 0 {
			failedItems = append(failedItems, item)
		} else {
			successItems = append(successItems, item)
		}
	}

	return successItems, failedItems, allItems
}

func buildDatasetItem(ctx context.Context, span *loop_span.Span, i int, fieldMappings []entity.FieldMapping, workspaceID, taskID int64,
	dataset *entity.Dataset,
) *entity.DatasetItem {
	item := entity.NewDatasetItem(workspaceID, dataset.ID, span)
	for _, mapping := range fieldMappings {
		value, err := span.ExtractByJsonpath(ctx, mapping.TraceFieldKey, mapping.TraceFieldJsonpath)
		if err != nil {
			// 非json但使用了jsonpath，也不报错，置空
			logs.CtxInfo(ctx, "Extract field failed, err:%v", err)
		}

		content, errCode := entity.GetContentInfo(ctx, mapping.FieldSchema.ContentType, value)
		if errCode == entity.DatasetErrorType_MismatchSchema {
			item.AddError("invalid multi part", entity.DatasetErrorType_MismatchSchema, nil)
			continue
		}

		// 前端传入的是Name，评测集需要的是key，需要做一下mapping
		key := dataset.GetFieldSchemaKeyByName(mapping.FieldSchema.Name)
		if key == "" {
			logs.CtxInfo(ctx, "Dataset field key is empty, name:%v", mapping.FieldSchema.Name)
			item.AddError("Dataset field key is empty", entity.DatasetErrorType_InternalError, nil)
			continue
		}
		item.AddFieldData(key, mapping.FieldSchema.Name, content)
		item.ItemKey = gptr.Of(fmt.Sprintf("%s_%s_%s", taskID, span.TraceID, span.SpanID))
	}
	return item
}

// todo:[xun]和手动回流的代码逻辑一样，需要抽取公共代码
func buildItem(ctx context.Context, span *loop_span.Span, fieldMappings []*task.EvaluateFieldMapping,
	evaluationSetSchema string) []*eval_set.FieldData {
	var fieldDatas []*eval_set.FieldData
	fieldDatas = append(fieldDatas, &eval_set.FieldData{
		Key:  gptr.Of("trace_id"),
		Name: gptr.Of("trace_id"),
		Content: &common.Content{
			ContentType: gptr.Of(common.ContentTypeText),
			Text:        gptr.Of(span.TraceID),
		},
	})
	fieldDatas = append(fieldDatas, &eval_set.FieldData{
		Key:  gptr.Of("span_id"),
		Name: gptr.Of("span_id"),
		Content: &common.Content{
			ContentType: gptr.Of(common.ContentTypeText),
			Text:        gptr.Of(span.SpanID),
		},
	})
	for _, mapping := range fieldMappings {
		// 前端传入的是Name，评测集需要的是key，需要做一下mapping
		if mapping.EvalSetName == nil {
			logs.CtxInfo(ctx, "Evaluator field name is nil")
			continue
		}
		var evaluationSetSchemas []*eval_set.FieldSchema
		if evaluationSetSchema == "" {
			logs.CtxInfo(ctx, "Evaluation set schema is nil")
			continue
		}
		err := json.Unmarshal([]byte(evaluationSetSchema), &evaluationSetSchemas)
		if err != nil {
			logs.CtxInfo(ctx, "Unmarshal evaluation set schema failed, err:%v", err)
			continue
		}
		for _, fieldSchema := range evaluationSetSchemas {
			if fieldSchema.GetKey() == *mapping.EvalSetName {
				key := fieldSchema.GetKey()
				if key == "" {
					logs.CtxInfo(ctx, "Evaluator field key is empty, name:%v", *mapping.FieldSchema.Name)
					continue
				}
				value, err := span.ExtractByJsonpath(ctx, mapping.TraceFieldKey, mapping.TraceFieldJsonpath)
				if err != nil {
					logs.CtxInfo(ctx, "Extract field failed, err:%v", err)
					continue
				}
				content, err := GetContentInfo(ctx, fieldSchema.GetContentType(), value)
				if err != nil {
					logs.CtxInfo(ctx, "GetContentInfo failed, err:%v", err)
					return nil
				}
				fieldDatas = append(fieldDatas, &eval_set.FieldData{
					Key:     gptr.Of(key),
					Name:    gptr.Of(fieldSchema.GetName()),
					Content: content,
				})
			}
		}
	}
	return fieldDatas
}

// todo:[xun]和手动回流的代码逻辑一样，需要抽取公共代码
func GetContentInfo(ctx context.Context, contentType common.ContentType, value string) (*common.Content, error) {
	var content *common.Content
	switch contentType {
	case common.ContentTypeMultiPart:
		var parts []tracespec.ModelMessagePart
		err := json.Unmarshal([]byte(value), &parts)
		if err != nil {
			logs.CtxInfo(ctx, "Unmarshal multi part failed, err:%v", err)
			return nil, err
		}
		var multiPart []*common.Content
		for _, part := range parts {
			// 本期仅支持回流图片的多模态数据，非ImageURL信息的，打包放进text
			switch part.Type {
			case tracespec.ModelMessagePartTypeImage:
				if part.ImageURL == nil {
					continue
				}
				multiPart = append(multiPart, &common.Content{
					ContentType: gptr.Of(common.ContentTypeImage),
					Image: &common.Image{
						Name: gptr.Of(part.ImageURL.Name),
						URL:  gptr.Of(part.ImageURL.URL),
					},
				})
			case tracespec.ModelMessagePartTypeText, tracespec.ModelMessagePartTypeFile:
				multiPart = append(multiPart, &common.Content{
					ContentType: gptr.Of(common.ContentTypeText),
					Text:        gptr.Of(part.Text),
				})
			default:
				logs.CtxWarn(ctx, "Unsupported part type: %s", part.Type)
				return nil, err
			}
		}
		content = &common.Content{
			ContentType: gptr.Of(common.ContentTypeMultiPart),
			MultiPart:   multiPart,
		}
	default:
		content = &common.Content{
			ContentType: gptr.Of(common.ContentTypeText),
			Text:        gptr.Of(value),
		}
	}
	return content, nil
}

func (p *AutoEvaluteProcessor) OnCreateChangeProcessor(ctx context.Context, task *task.Task) error {
	// 如果是历史回溯，立即创建taskrun

	return nil
}
func (p *AutoEvaluteProcessor) OnUpdateChangeProcessor(ctx context.Context, task *task.Task) error {
	return nil
}
func (p *AutoEvaluteProcessor) OnFinishChangeProcessor(ctx context.Context, task *task.Task) error {
	return nil
}

func (p *AutoEvaluteProcessor) OnCreateTaskRunProcessor(ctx context.Context, taskRun *task.TaskRun) error {
	return nil
}
func (p *AutoEvaluteProcessor) OnFinishTaskRunProcessor(ctx context.Context, taskRun *task.TaskRun) error {
	return nil
}
