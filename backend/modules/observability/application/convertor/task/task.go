// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package task

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/sonic"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	entity_common "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/common"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/slices"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func TaskDOs2DTOs(ctx context.Context, taskPOs []*entity.ObservabilityTask, userInfos map[string]*entity_common.UserInfo) []*task.Task {
	var taskList []*task.Task
	if len(taskPOs) == 0 {
		return taskList
	}
	for _, v := range taskPOs {
		taskDO := TaskDO2DTO(ctx, v, userInfos)
		taskList = append(taskList, taskDO)
	}
	return taskList
}

func TaskDO2DTO(ctx context.Context, v *entity.ObservabilityTask, userMap map[string]*entity_common.UserInfo) *task.Task {
	if v == nil {
		return nil
	}
	var taskDetail *task.RunDetail
	var totalCount, successCount, failedCount int64
	for _, tr := range v.TaskRuns {
		trDO := TaskRunDO2DTO(ctx, tr, nil)
		if trDO.RunDetail != nil {
			totalCount += *trDO.RunDetail.TotalCount
			successCount += *trDO.RunDetail.SuccessCount
			failedCount += *trDO.RunDetail.FailedCount
		}
	}
	taskDetail = &task.RunDetail{
		TotalCount:   gptr.Of(totalCount),
		SuccessCount: gptr.Of(successCount),
		FailedCount:  gptr.Of(failedCount),
	}
	taskInfo := &task.Task{
		ID:          ptr.Of(v.ID),
		Name:        v.Name,
		Description: v.Description,
		WorkspaceID: ptr.Of(v.WorkspaceID),
		TaskType:    v.TaskType,
		TaskStatus:  ptr.Of(v.TaskStatus),
		Rule:        RuleDO2DTO(v.SpanFilter, v.EffectiveTime, v.Sampler, v.BackfillEffectiveTime),
		TaskConfig:  TaskConfigDO2DTO(v.TaskConfig),
		TaskDetail:  taskDetail,
		BaseInfo: &common.BaseInfo{
			CreatedAt: gptr.Of(v.CreatedAt.UnixMilli()),
			UpdatedAt: gptr.Of(v.UpdatedAt.UnixMilli()),
			CreatedBy: UserInfoPO2DO(userMap[v.CreatedBy], v.CreatedBy),
			UpdatedBy: UserInfoPO2DO(userMap[v.UpdatedBy], v.UpdatedBy),
		},
	}
	return taskInfo
}

func TaskRunDO2DTO(ctx context.Context, v *entity.TaskRun, userMap map[string]*entity_common.UserInfo) *task.TaskRun {
	if v == nil {
		return nil
	}
	taskRunInfo := &task.TaskRun{
		ID:                v.ID,
		WorkspaceID:       v.WorkspaceID,
		TaskID:            v.TaskID,
		TaskType:          v.TaskType,
		RunStatus:         v.RunStatus,
		RunDetail:         RunDetailDO2DTO(v.RunDetail),
		BackfillRunDetail: BackfillRunDetailDO2DTO(v.BackfillDetail),
		RunStartAt:        v.RunStartAt.UnixMilli(),
		RunEndAt:          v.RunEndAt.UnixMilli(),
		TaskRunConfig:     TaskRunConfigDO2DTO(v.TaskRunConfig),
		BaseInfo:          buildTaskRunBaseInfo(v, userMap),
	}
	return taskRunInfo
}

func TaskConfigDO2DTO(v *entity.TaskConfig) *task.TaskConfig {
	if v == nil {
		return nil
	}
	var autoEvaluateConfigs []*task.AutoEvaluateConfig
	if len(v.AutoEvaluateConfigs) > 0 {
		for _, config := range v.AutoEvaluateConfigs {
			autoEvaluateConfigs = append(autoEvaluateConfigs, AutoEvaluateConfigDO2DTO(config))
		}
	}
	var dataReflowConfigs []*task.DataReflowConfig
	if len(v.DataReflowConfig) > 0 {
		for _, config := range v.DataReflowConfig {
			dataReflowConfigs = append(dataReflowConfigs, DataReflowConfigDO2DTO(config))
		}
	}
	return &task.TaskConfig{
		AutoEvaluateConfigs: autoEvaluateConfigs,
		DataReflowConfig:    dataReflowConfigs,
	}
}

func AutoEvaluateConfigDO2DTO(v *entity.AutoEvaluateConfig) *task.AutoEvaluateConfig {
	if v == nil {
		return nil
	}
	var fieldMappings []*task.EvaluateFieldMapping
	if len(v.FieldMappings) > 0 {
		for _, config := range v.FieldMappings {
			fieldMappings = append(fieldMappings, &task.EvaluateFieldMapping{
				FieldSchema:        config.FieldSchema,
				TraceFieldKey:      config.TraceFieldKey,
				TraceFieldJsonpath: config.TraceFieldJsonpath,
				EvalSetName:        config.EvalSetName,
			})
		}
	}
	return &task.AutoEvaluateConfig{
		EvaluatorVersionID: v.EvaluatorVersionID,
		EvaluatorID:        v.EvaluatorID,
		FieldMappings:      fieldMappings,
	}
}

func DataReflowConfigDO2DTO(v *entity.DataReflowConfig) *task.DataReflowConfig {
	if v == nil {
		return nil
	}
	var fieldMappings []*dataset.FieldMapping
	if len(v.FieldMappings) > 0 {
		for _, config := range v.FieldMappings {
			fieldMappings = append(fieldMappings, ptr.Of(config))
		}
	}
	return &task.DataReflowConfig{
		DatasetID:     v.DatasetID,
		DatasetName:   v.DatasetName,
		DatasetSchema: ptr.Of(v.DatasetSchema),
		FieldMappings: fieldMappings,
	}
}

func RuleDO2DTO(spanFilter *entity.SpanFilterFields, effectiveTime *entity.EffectiveTime, sampler *entity.Sampler, backfillEffectiveTime *entity.EffectiveTime) *task.Rule {
	if spanFilter == nil {
		return nil
	}
	return &task.Rule{
		SpanFilters:           SpanFilterDO2DTO(spanFilter),
		Sampler:               SamplerDO2DTO(sampler),
		EffectiveTime:         EffectiveTimeDO2DTO(effectiveTime),
		BackfillEffectiveTime: EffectiveTimeDO2DTO(backfillEffectiveTime),
	}
}

func SpanFilterDO2DTO(spanFilter *entity.SpanFilterFields) *filter.SpanFilterFields {
	if spanFilter == nil {
		return nil
	}

	return &filter.SpanFilterFields{
		Filters:      FiltersDO2DTO(&spanFilter.Filters),
		PlatformType: &spanFilter.PlatformType,
		SpanListType: &spanFilter.SpanListType,
	}
}

func FiltersDO2DTO(filters *loop_span.FilterFields) *filter.FilterFields {
	if filters == nil {
		return nil
	}
	var filterFields []*filter.FilterField
	for _, f := range filters.FilterFields {
		var queryType, queryAndOr string
		if f.QueryType != nil {
			queryType = filter.QueryType(*f.QueryType)
		}
		if f.QueryAndOr != nil {
			queryAndOr = filter.QueryRelation(*f.QueryAndOr)
		}
		filterFields = append(filterFields, &filter.FilterField{
			FieldName:  ptr.Of(f.FieldName),
			FieldType:  ptr.Of(filter.FieldType(f.FieldType)),
			Values:     f.Values,
			QueryType:  ptr.Of(queryType),
			QueryAndOr: ptr.Of(queryAndOr),
		})
	}
	return &filter.FilterFields{
		QueryAndOr:   ptr.Of(filter.QueryRelation(*filters.QueryAndOr)),
		FilterFields: filterFields,
	}
}

func SpanFilterPO2DO(ctx context.Context, spanFilter *string) *filter.SpanFilterFields {
	if spanFilter == nil {
		return nil
	}
	var spanFilterDO filter.SpanFilterFields
	if err := sonic.Unmarshal([]byte(*spanFilter), &spanFilterDO); err != nil {
		logs.CtxError(ctx, "SpanFilterPO2DO sonic.Unmarshal err:%v", err)
		return nil
	}
	return &spanFilterDO
}

func SamplerDO2DTO(sampler *entity.Sampler) *task.Sampler {
	if sampler == nil {
		return nil
	}
	return &task.Sampler{
		SampleRate:    ptr.Of(sampler.SampleRate),
		SampleSize:    ptr.Of(sampler.SampleSize),
		IsCycle:       ptr.Of(sampler.IsCycle),
		CycleCount:    ptr.Of(sampler.CycleCount),
		CycleInterval: ptr.Of(sampler.CycleInterval),
		CycleTimeUnit: ptr.Of(sampler.CycleTimeUnit),
	}
}

func EffectiveTimeDO2DTO(effectiveTime *entity.EffectiveTime) *task.EffectiveTime {
	if effectiveTime == nil {
		return &task.EffectiveTime{
			StartAt: ptr.Of(int64(0)),
			EndAt:   ptr.Of(int64(0)),
		}
	}
	return &task.EffectiveTime{
		StartAt: ptr.Of(effectiveTime.StartAt),
		EndAt:   ptr.Of(effectiveTime.EndAt),
	}
}

// RunDetailDO2DTO 将JSON字符串转换为RunDetail结构体
func RunDetailDO2DTO(runDetail *entity.RunDetail) *task.RunDetail {
	if runDetail == nil {
		return nil
	}
	return &task.RunDetail{
		SuccessCount: ptr.Of(runDetail.SuccessCount),
		FailedCount:  ptr.Of(runDetail.FailedCount),
		TotalCount:   ptr.Of(runDetail.TotalCount),
	}
}

func BackfillRunDetailDO2DTO(backfillDetail *entity.BackfillDetail) *task.BackfillDetail {
	if backfillDetail == nil {
		return nil
	}
	return &task.BackfillDetail{
		SuccessCount:      backfillDetail.SuccessCount,
		FailedCount:       backfillDetail.FailedCount,
		TotalCount:        backfillDetail.TotalCount,
		BackfillStatus:    backfillDetail.BackfillStatus,
		LastSpanPageToken: backfillDetail.LastSpanPageToken,
	}
}

func TaskRunConfigDO2DTO(v *entity.TaskRunConfig) *task.TaskRunConfig {
	if v == nil {
		return nil
	}
	return &task.TaskRunConfig{
		AutoEvaluateRunConfig: AutoEvaluateRunConfigDO2DTO(v.AutoEvaluateRunConfig),
		DataReflowRunConfig:   DataReflowRunConfigDO2DTO(v.DataReflowRunConfig),
	}
}

func AutoEvaluateRunConfigDO2DTO(v *entity.AutoEvaluateRunConfig) *task.AutoEvaluateRunConfig {
	if v == nil {
		return nil
	}
	return &task.AutoEvaluateRunConfig{
		ExptID:       v.ExptID,
		ExptRunID:    v.ExptRunID,
		EvalID:       v.EvalID,
		SchemaID:     v.SchemaID,
		Schema:       v.Schema,
		EndAt:        v.EndAt,
		CycleStartAt: v.CycleStartAt,
		CycleEndAt:   v.CycleEndAt,
		Status:       v.Status,
	}
}

func DataReflowRunConfigDO2DTO(v *entity.DataReflowRunConfig) *task.DataReflowRunConfig {
	if v == nil {
		return nil
	}
	return &task.DataReflowRunConfig{
		DatasetID:    v.DatasetID,
		DatasetRunID: v.DatasetRunID,
		EndAt:        v.EndAt,
		CycleStartAt: v.CycleStartAt,
		CycleEndAt:   v.CycleEndAt,
		Status:       v.Status,
	}
}

func UserInfoPO2DO(userInfo *entity_common.UserInfo, userID string) *common.UserInfo {
	if userInfo == nil {
		return &common.UserInfo{
			UserID: gptr.Of(userID),
		}
	}
	return &common.UserInfo{
		Name:        ptr.Of(userInfo.Name),
		EnName:      ptr.Of(userInfo.EnName),
		AvatarURL:   ptr.Of(userInfo.AvatarURL),
		AvatarThumb: ptr.Of(userInfo.AvatarThumb),
		OpenID:      ptr.Of(userInfo.OpenID),
		UnionID:     ptr.Of(userInfo.UnionID),
		UserID:      ptr.Of(userInfo.UserID),
		Email:       ptr.Of(userInfo.Email),
	}
}

func TaskDTO2DO(taskDTO *task.Task, userID string, spanFilters *entity.SpanFilterFields) *entity.ObservabilityTask {
	if taskDTO == nil {
		return nil
	}
	var createdBy, updatedBy string
	if taskDTO.GetBaseInfo().GetCreatedBy() != nil {
		createdBy = taskDTO.GetBaseInfo().GetCreatedBy().GetUserID()
	}
	if taskDTO.GetBaseInfo().GetUpdatedBy() != nil {
		updatedBy = taskDTO.GetBaseInfo().GetUpdatedBy().GetUserID()
	}
	if userID != "" {
		createdBy = userID
		updatedBy = userID
	} else {
		if taskDTO.GetBaseInfo().GetCreatedBy() != nil {
			createdBy = taskDTO.GetBaseInfo().GetCreatedBy().GetUserID()
		}
		if taskDTO.GetBaseInfo().GetUpdatedBy() != nil {
			updatedBy = taskDTO.GetBaseInfo().GetUpdatedBy().GetUserID()
		}
	}
	var spanFilterDO *entity.SpanFilterFields
	if spanFilters != nil {
		spanFilterDO = spanFilters
	} else {
		spanFilterDO = SpanFilterDTO2DO(taskDTO.GetRule().GetSpanFilters())
	}

	return &entity.ObservabilityTask{
		ID:                    taskDTO.GetID(),
		WorkspaceID:           taskDTO.GetWorkspaceID(),
		Name:                  taskDTO.GetName(),
		Description:           ptr.Of(taskDTO.GetDescription()),
		TaskType:              taskDTO.GetTaskType(),
		TaskStatus:            taskDTO.GetTaskStatus(),
		TaskDetail:            RunDetailDTO2DO(taskDTO.GetTaskDetail()),
		SpanFilter:            spanFilterDO,
		EffectiveTime:         EffectiveTimeDTO2DO(taskDTO.GetRule().GetEffectiveTime()),
		Sampler:               SamplerDTO2DO(taskDTO.GetRule().GetSampler()),
		TaskConfig:            TaskConfigDTO2DO(taskDTO.GetTaskConfig()),
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
		CreatedBy:             createdBy,
		UpdatedBy:             updatedBy,
		BackfillEffectiveTime: EffectiveTimeDTO2DO(taskDTO.GetRule().GetBackfillEffectiveTime()),
	}
}

func SpanFilterDTO2DO(spanFilterFields *filter.SpanFilterFields) *entity.SpanFilterFields {
	if spanFilterFields == nil {
		return nil
	}
	return &entity.SpanFilterFields{
		PlatformType: *spanFilterFields.PlatformType,
		SpanListType: *spanFilterFields.SpanListType,
		Filters:      *SpanFilterFieldsDTO2DO(spanFilterFields.Filters),
	}
}

func SpanFilterFieldsDTO2DO(spanFilterFields *filter.FilterFields) *loop_span.FilterFields {
	if spanFilterFields == nil {
		return nil
	}
	return &loop_span.FilterFields{
		QueryAndOr:   ptr.Of(loop_span.QueryAndOrEnumAnd),
		FilterFields: FilterFieldsDTO2DO(spanFilterFields.FilterFields),
	}
}
func FilterFieldsDTO2DO(filterFields []*filter.FilterField) []*loop_span.FilterField {
	if filterFields == nil {
		return nil
	}
	filtersDO := make([]*loop_span.FilterField, 0, len(filterFields))
	for _, v := range filterFields {
		filtersDO = append(filtersDO, &loop_span.FilterField{
			FieldName:  *v.FieldName,
			FieldType:  loop_span.FieldType(*v.FieldType),
			Values:     v.Values,
			QueryType:  ptr.Of(loop_span.QueryTypeEnum(*v.QueryType)),
			QueryAndOr: ptr.Of(loop_span.QueryAndOrEnum(*v.QueryAndOr)),
			SubFilter:  SpanFilterFieldsDTO2DO(v.SubFilter),
		})
	}
	return filtersDO
}

func RunDetailDTO2DO(runDetail *task.RunDetail) *entity.RunDetail {
	if runDetail == nil {
		return nil
	}
	return &entity.RunDetail{
		SuccessCount: *runDetail.SuccessCount,
		FailedCount:  *runDetail.FailedCount,
		TotalCount:   *runDetail.TotalCount,
	}
}

func EffectiveTimeDTO2DO(effectiveTime *task.EffectiveTime) *entity.EffectiveTime {
	if effectiveTime == nil {
		return nil
	}
	return &entity.EffectiveTime{
		StartAt: *effectiveTime.StartAt,
		EndAt:   *effectiveTime.EndAt,
	}
}

func SamplerDTO2DO(sampler *task.Sampler) *entity.Sampler {
	if sampler == nil {
		return nil
	}
	return &entity.Sampler{
		SampleRate:    sampler.GetSampleRate(),
		SampleSize:    sampler.GetSampleSize(),
		IsCycle:       sampler.GetIsCycle(),
		CycleCount:    sampler.GetCycleCount(),
		CycleInterval: sampler.GetCycleInterval(),
		CycleTimeUnit: sampler.GetCycleTimeUnit(),
	}
}

func TaskConfigDTO2DO(taskConfig *task.TaskConfig) *entity.TaskConfig {
	if taskConfig == nil {
		return nil
	}
	autoEvaluateConfigs := make([]*entity.AutoEvaluateConfig, 0, len(taskConfig.AutoEvaluateConfigs))
	for _, autoEvaluateConfig := range taskConfig.AutoEvaluateConfigs {
		var fieldMappings []*entity.EvaluateFieldMapping
		if len(autoEvaluateConfig.FieldMappings) > 0 {
			var evalSetNames []string
			jspnPathMapping := make(map[string]string)
			for _, config := range autoEvaluateConfig.FieldMappings {
				var evalSetName string
				jspnPath := fmt.Sprintf("%s.%s", config.TraceFieldKey, config.TraceFieldJsonpath)
				if _, exits := jspnPathMapping[jspnPath]; exits {
					evalSetName = jspnPathMapping[jspnPath]
				} else {
					evalSetName = getLastPartAfterDot(jspnPath)
					for exists := slices.Contains(evalSetNames, evalSetName); exists; exists = slices.Contains(evalSetNames, evalSetName) {
						evalSetName += "_"
					}
				}
				evalSetNames = append(evalSetNames, evalSetName)
				jspnPathMapping[jspnPath] = evalSetName
				fieldMappings = append(fieldMappings, &entity.EvaluateFieldMapping{
					FieldSchema:        config.FieldSchema,
					TraceFieldKey:      config.TraceFieldKey,
					TraceFieldJsonpath: config.TraceFieldJsonpath,
					EvalSetName:        ptr.Of(evalSetName),
				})
			}

		}
		autoEvaluateConfigs = append(autoEvaluateConfigs, &entity.AutoEvaluateConfig{
			EvaluatorVersionID: autoEvaluateConfig.EvaluatorVersionID,
			EvaluatorID:        autoEvaluateConfig.EvaluatorID,
			FieldMappings:      fieldMappings,
		})
	}
	dataReflowConfigs := make([]*entity.DataReflowConfig, 0, len(taskConfig.DataReflowConfig))
	for _, dataReflowConfig := range taskConfig.DataReflowConfig {
		var fieldMappings []dataset.FieldMapping
		if len(dataReflowConfig.FieldMappings) > 0 {
			for _, config := range dataReflowConfig.FieldMappings {
				fieldMappings = append(fieldMappings, dataset.FieldMapping{
					FieldSchema:        config.FieldSchema,
					TraceFieldKey:      config.TraceFieldKey,
					TraceFieldJsonpath: config.TraceFieldJsonpath,
				})
			}
		}
		dataReflowConfigs = append(dataReflowConfigs, &entity.DataReflowConfig{
			DatasetID:     dataReflowConfig.DatasetID,
			DatasetName:   dataReflowConfig.DatasetName,
			DatasetSchema: *dataReflowConfig.DatasetSchema,
			FieldMappings: fieldMappings,
		})
	}
	return &entity.TaskConfig{
		AutoEvaluateConfigs: autoEvaluateConfigs,
		DataReflowConfig:    dataReflowConfigs,
	}
}

func TaskRunDTO2DO(taskRun *task.TaskRun) *entity.TaskRun {
	if taskRun == nil {
		return nil
	}
	return &entity.TaskRun{
		ID:             taskRun.ID,
		TaskID:         taskRun.TaskID,
		WorkspaceID:    taskRun.WorkspaceID,
		TaskType:       taskRun.TaskType,
		RunStatus:      taskRun.RunStatus,
		RunDetail:      RunDetailDTO2DO(taskRun.RunDetail),
		BackfillDetail: BackfillRunDetailDTO2DO(taskRun.BackfillRunDetail),
		RunStartAt:     time.UnixMilli(taskRun.RunStartAt),
		RunEndAt:       time.UnixMilli(taskRun.RunEndAt),
		TaskRunConfig:  TaskRunConfigDTO2DO(taskRun.TaskRunConfig),
		CreatedAt:      time.UnixMilli(taskRun.GetBaseInfo().GetCreatedAt()),
		UpdatedAt:      time.UnixMilli(taskRun.GetBaseInfo().GetUpdatedAt()),
	}
}

func TaskRunConfigDTO2DO(v *task.TaskRunConfig) *entity.TaskRunConfig {
	if v == nil {
		return nil
	}
	var autoEvaluateRunConfig *entity.AutoEvaluateRunConfig
	if v.GetAutoEvaluateRunConfig() != nil {
		autoEvaluateRunConfig = &entity.AutoEvaluateRunConfig{
			ExptID:       v.GetAutoEvaluateRunConfig().GetExptID(),
			ExptRunID:    v.GetAutoEvaluateRunConfig().GetExptRunID(),
			EvalID:       v.GetAutoEvaluateRunConfig().GetEvalID(),
			SchemaID:     v.GetAutoEvaluateRunConfig().GetSchemaID(),
			Schema:       v.GetAutoEvaluateRunConfig().Schema,
			EndAt:        v.GetAutoEvaluateRunConfig().GetEndAt(),
			CycleStartAt: v.GetAutoEvaluateRunConfig().GetCycleStartAt(),
			CycleEndAt:   v.GetAutoEvaluateRunConfig().GetCycleEndAt(),
			Status:       v.GetAutoEvaluateRunConfig().GetStatus(),
		}
	}
	var dataReflowRunConfig *entity.DataReflowRunConfig
	if v.GetDataReflowRunConfig() != nil {
		dataReflowRunConfig = &entity.DataReflowRunConfig{
			DatasetID:    v.GetDataReflowRunConfig().GetDatasetID(),
			DatasetRunID: v.GetDataReflowRunConfig().GetDatasetRunID(),
			EndAt:        v.GetDataReflowRunConfig().GetEndAt(),
			CycleStartAt: v.GetDataReflowRunConfig().GetCycleStartAt(),
			CycleEndAt:   v.GetDataReflowRunConfig().GetCycleEndAt(),
			Status:       v.GetDataReflowRunConfig().GetStatus(),
		}
	}
	return &entity.TaskRunConfig{
		AutoEvaluateRunConfig: autoEvaluateRunConfig,
		DataReflowRunConfig:   dataReflowRunConfig,
	}
}

func BackfillRunDetailDTO2DO(v *task.BackfillDetail) *entity.BackfillDetail {
	if v == nil {
		return nil
	}
	return &entity.BackfillDetail{
		SuccessCount:      v.SuccessCount,
		FailedCount:       v.FailedCount,
		TotalCount:        v.TotalCount,
		BackfillStatus:    v.BackfillStatus,
		LastSpanPageToken: v.LastSpanPageToken,
	}
}

func CheckEffectiveTime(ctx context.Context, effectiveTime *task.EffectiveTime, taskStatus task.TaskStatus, effectiveTimeDO *entity.EffectiveTime) (*entity.EffectiveTime, error) {
	if effectiveTimeDO == nil {
		logs.CtxError(ctx, "EffectiveTimePO2DO error")
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("effective time is nil"))
	}
	var validEffectiveTime entity.EffectiveTime
	// 开始时间不能大于结束时间
	if effectiveTime.GetStartAt() >= effectiveTime.GetEndAt() {
		logs.CtxError(ctx, "Start time must be less than end time")
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("start time must be less than end time"))
	}
	// 开始、结束时间不能小于当前时间
	if effectiveTimeDO.StartAt != effectiveTime.GetStartAt() && effectiveTime.GetStartAt() < time.Now().UnixMilli() {
		logs.CtxError(ctx, "update time must be greater than current time")
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("start time must be greater than current time"))
	}
	if effectiveTimeDO.EndAt != effectiveTime.GetEndAt() && effectiveTime.GetEndAt() < time.Now().UnixMilli() {
		logs.CtxError(ctx, "update time must be greater than current time")
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("start time must be greater than current time"))
	}
	validEffectiveTime.StartAt = effectiveTimeDO.StartAt
	validEffectiveTime.EndAt = effectiveTimeDO.EndAt
	switch taskStatus {
	case task.TaskStatusUnstarted:
		if validEffectiveTime.StartAt != 0 {
			validEffectiveTime.StartAt = *effectiveTime.StartAt
		}
		if validEffectiveTime.EndAt != 0 {
			validEffectiveTime.EndAt = *effectiveTime.EndAt
		}
	case task.TaskStatusRunning, task.TaskStatusPending:
		if validEffectiveTime.EndAt != 0 {
			validEffectiveTime.EndAt = *effectiveTime.EndAt
		}
	default:
		logs.CtxError(ctx, "Invalid task status:%s", taskStatus)
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid task status"))
	}
	return &validEffectiveTime, nil
}

func CheckTaskStatus(ctx context.Context, taskStatus task.TaskStatus, currentTaskStatus task.TaskStatus) (task.TaskStatus, error) {
	var validTaskStatus task.TaskStatus
	// [0530]todo: 任务状态校验
	switch taskStatus {
	case task.TaskStatusUnstarted:
		if currentTaskStatus == task.TaskStatusUnstarted {
			validTaskStatus = taskStatus
		} else {
			logs.CtxError(ctx, "Invalid task status:%s", taskStatus)
			return "", errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid task status"))
		}
	case task.TaskStatusRunning:
		if currentTaskStatus == task.TaskStatusUnstarted || currentTaskStatus == task.TaskStatusPending {
			validTaskStatus = taskStatus
		} else {
			logs.CtxError(ctx, "Invalid task status:%s，currentTaskStatus:%s", taskStatus, currentTaskStatus)
			return "", errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid task status"))
		}
	case task.TaskStatusPending:
		if currentTaskStatus == task.TaskStatusRunning {
			validTaskStatus = task.TaskStatusPending
		}
	case task.TaskStatusDisabled:
		if currentTaskStatus == task.TaskStatusUnstarted || currentTaskStatus == task.TaskStatusPending {
			validTaskStatus = task.TaskStatusDisabled
		}
	case task.TaskStatusSuccess:
		if currentTaskStatus != task.TaskStatusSuccess {
			validTaskStatus = task.TaskStatusSuccess
		}
	}

	return validTaskStatus, nil
}

func getLastPartAfterDot(s string) string {
	s = strings.TrimRight(s, ".")
	lastDotIndex := strings.LastIndex(s, ".")
	if lastDotIndex == -1 {
		lastPart := s
		return processBracket(lastPart)
	}
	lastPart := s[lastDotIndex+1:]
	return processBracket(lastPart)
}

// processBracket 处理字符串中的方括号，将其转换为下划线连接的形式
func processBracket(s string) string {
	openBracketIndex := strings.Index(s, "[")
	if openBracketIndex == -1 {
		return s
	}
	closeBracketIndex := strings.Index(s, "]")
	if closeBracketIndex == -1 {
		return s
	}
	base := s[:openBracketIndex]
	index := s[openBracketIndex+1 : closeBracketIndex]
	return base + "_" + index
}

// ToJSONString 通用函数，将对象转换为 JSON 字符串指针
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

// buildTaskRunBaseInfo 构建BaseInfo信息
func buildTaskRunBaseInfo(v *entity.TaskRun, userMap map[string]*entity_common.UserInfo) *common.BaseInfo {
	// 注意：TaskRun实体中没有CreatedBy和UpdatedBy字段
	// 使用空字符串作为默认值
	return &common.BaseInfo{
		CreatedAt: gptr.Of(v.CreatedAt.UnixMilli()),
		UpdatedAt: gptr.Of(v.UpdatedAt.UnixMilli()),
		CreatedBy: &common.UserInfo{UserID: gptr.Of("")},
		UpdatedBy: &common.UserInfo{UserID: gptr.Of("")},
	}
}
