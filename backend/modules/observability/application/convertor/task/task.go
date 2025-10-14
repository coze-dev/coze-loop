// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package task

//func TaskPOs2DOs(ctx context.Context, taskPOs []*entity.ObservabilityTask, userInfos map[string]*entity_common.UserInfo) []*task.Task {
//	var taskList []*task.Task
//	if len(taskPOs) == 0 {
//		return taskList
//	}
//	for _, v := range taskPOs {
//		taskDO := TaskPO2DTO(ctx, v, userInfos)
//		taskList = append(taskList, taskDO)
//	}
//	return taskList
//}
//func TaskPO2DTO(ctx context.Context, v *entity.ObservabilityTask, userMap map[string]*entity_common.UserInfo) *task.Task {
//	if v == nil {
//		return nil
//	}
//	var taskDetail *task.RunDetail
//	var totalCount, successCount, failedCount int64
//	for _, tr := range v.TaskRuns {
//		trDO := TaskRunPO2DTO(ctx, tr, nil)
//		if trDO.RunDetail != nil {
//			totalCount += *trDO.RunDetail.TotalCount
//			successCount += *trDO.RunDetail.SuccessCount
//			failedCount += *trDO.RunDetail.FailedCount
//		}
//	}
//	taskDetail = &task.RunDetail{
//		TotalCount:   gptr.Of(totalCount),
//		SuccessCount: gptr.Of(successCount),
//		FailedCount:  gptr.Of(failedCount),
//	}
//	taskInfo := &task.Task{
//		ID:          ptr.Of(v.ID),
//		Name:        v.Name,
//		Description: v.Description,
//		WorkspaceID: ptr.Of(v.WorkspaceID),
//		TaskType:    v.TaskType,
//		TaskStatus:  ptr.Of(v.TaskStatus),
//		Rule:        RulePO2DO(ctx, v.SpanFilter, v.EffectiveTime, v.Sampler, v.BackfillEffectiveTime),
//		TaskConfig:  TaskConfigPO2DO(ctx, v.TaskConfig),
//		TaskDetail:  taskDetail,
//		BaseInfo: &common.BaseInfo{
//			CreatedAt: gptr.Of(v.CreatedAt.UnixMilli()),
//			UpdatedAt: gptr.Of(v.UpdatedAt.UnixMilli()),
//			CreatedBy: UserInfoPO2DO(userMap[v.CreatedBy], v.CreatedBy),
//			UpdatedBy: UserInfoPO2DO(userMap[v.UpdatedBy], v.UpdatedBy),
//		},
//	}
//	return taskInfo
//}

//func RulePO2DO(ctx context.Context, spanFilter, effectiveTime, sampler, backFillEffectiveTime *string) *task.Rule {
//	var spanFilterDO *filter.SpanFilterFields
//	if spanFilter != nil {
//		spanFilterDO = SpanFilterPO2DO(ctx, spanFilter)
//	}
//	rule := &task.Rule{
//		SpanFilters:           spanFilterDO,
//		EffectiveTime:         EffectiveTimePO2DO(ctx, effectiveTime),
//		Sampler:               SamplerPO2DO(ctx, sampler),
//		BackfillEffectiveTime: EffectiveTimePO2DO(ctx, backFillEffectiveTime),
//	}
//	return rule
//}
//func SamplerPO2DO(ctx context.Context, sampler *string) *task.Sampler {
//	if sampler == nil {
//		return nil
//	}
//	var samplerDO task.Sampler
//	if err := sonic.Unmarshal([]byte(*sampler), &samplerDO); err != nil {
//		logs.CtxError(ctx, "SamplerPO2DO sonic.Unmarshal err:%v", err)
//		return nil
//	}
//	return &samplerDO
//}

//func TaskConfigPO2DO(ctx context.Context, taskConfig *string) *task.TaskConfig {
//	if taskConfig == nil {
//		return nil
//	}
//	var taskConfigDO task.TaskConfig
//	if err := sonic.Unmarshal([]byte(*taskConfig), &taskConfigDO); err != nil {
//		logs.CtxError(ctx, "TaskConfigPO2DO sonic.Unmarshal err:%v", err)
//		return nil
//	}
//	return &taskConfigDO
//}

//func BatchTaskPO2DTO(ctx context.Context, Tasks []*entity.ObservabilityTask) []*task.Task {
//	ret := make([]*task.Task, len(Tasks))
//	for i, v := range Tasks {
//		ret[i] = TaskPO2DTO(ctx, v, nil)
//	}
//	return ret
//}
//func EffectiveTimePO2DO(ctx context.Context, effectiveTime *string) *task.EffectiveTime {
//	if effectiveTime == nil {
//		return nil
//	}
//	var effectiveTimeDO task.EffectiveTime
//	if err := sonic.Unmarshal([]byte(*effectiveTime), &effectiveTimeDO); err != nil {
//		logs.CtxError(ctx, "EffectiveTimePO2DO sonic.Unmarshal err:%v", err)
//		return nil
//	}
//	return &effectiveTimeDO
//}

//func TaskDTO2PO(ctx context.Context, taskDO *task.Task, userID string, spanFilters *filter.SpanFilterFields) *entity.ObservabilityTask {
//	if taskDO == nil {
//		return nil
//	}
//	var createdBy, updatedBy string
//	if taskDO.GetBaseInfo().GetCreatedBy() != nil {
//		createdBy = taskDO.GetBaseInfo().GetCreatedBy().GetUserID()
//	}
//	if taskDO.GetBaseInfo().GetUpdatedBy() != nil {
//		updatedBy = taskDO.GetBaseInfo().GetUpdatedBy().GetUserID()
//	}
//	if userID != "" {
//		createdBy = userID
//		updatedBy = userID
//	} else {
//		if taskDO.GetBaseInfo().GetCreatedBy() != nil {
//			createdBy = taskDO.GetBaseInfo().GetCreatedBy().GetUserID()
//		}
//		if taskDO.GetBaseInfo().GetUpdatedBy() != nil {
//			updatedBy = taskDO.GetBaseInfo().GetUpdatedBy().GetUserID()
//		}
//	}
//	var spanFilterDO *filter.SpanFilterFields
//	if spanFilters != nil {
//		spanFilterDO = spanFilters
//	} else {
//		spanFilterDO = taskDO.GetRule().GetSpanFilters()
//	}
//
//	return &entity.ObservabilityTask{
//		ID:                    taskDO.GetID(),
//		WorkspaceID:           taskDO.GetWorkspaceID(),
//		Name:                  taskDO.GetName(),
//		Description:           ptr.Of(taskDO.GetDescription()),
//		TaskType:              taskDO.GetTaskType(),
//		TaskStatus:            taskDO.GetTaskStatus(),
//		TaskDetail:            ptr.Of(ToJSONString(ctx, taskDO.GetTaskDetail())),
//		SpanFilter:            SpanFilterDTO2PO(ctx, spanFilterDO),
//		EffectiveTime:         ptr.Of(ToJSONString(ctx, taskDO.GetRule().GetEffectiveTime())),
//		Sampler:               ptr.Of(ToJSONString(ctx, taskDO.GetRule().GetSampler())),
//		TaskConfig:            TaskConfigDTO2PO(ctx, taskDO.GetTaskConfig()),
//		CreatedAt:             time.Now(),
//		UpdatedAt:             time.Now(),
//		CreatedBy:             createdBy,
//		UpdatedBy:             updatedBy,
//		BackfillEffectiveTime: ptr.Of(ToJSONString(ctx, taskDO.GetRule().GetBackfillEffectiveTime())),
//	}
//}
//func SpanFilterDTO2PO(ctx context.Context, filters *filter.SpanFilterFields) *string {
//	var filtersDO *loop_span.FilterFields
//	if filters.GetFilters() != nil {
//		filtersDO = convertor.FilterFieldsDTO2DO(filters.GetFilters())
//	}
//	filterDO := entity.SpanFilter{
//		PlatformType: filters.GetPlatformType(),
//		SpanListType: filters.GetSpanListType(),
//	}
//	if filtersDO != nil {
//		filterDO.Filters = *filtersDO
//	}
//
//	return ptr.Of(ToJSONString(ctx, filterDO))
//}
//
//func TaskConfigDTO2PO(ctx context.Context, taskConfig *task.TaskConfig) *string {
//	if taskConfig == nil {
//		return nil
//	}
//	var evalSetNames []string
//	jspnPathMapping := make(map[string]string)
//	for _, autoEvaluateConfig := range taskConfig.GetAutoEvaluateConfigs() {
//		for _, mapping := range autoEvaluateConfig.GetFieldMappings() {
//			jspnPath := fmt.Sprintf("%s.%s", mapping.TraceFieldKey, mapping.TraceFieldJsonpath)
//			if _, exits := jspnPathMapping[jspnPath]; exits {
//				mapping.EvalSetName = gptr.Of(jspnPathMapping[jspnPath])
//				continue
//			}
//			evalSetName := getLastPartAfterDot(jspnPath)
//			for exists := slices.Contains(evalSetNames, evalSetName); exists; exists = slices.Contains(evalSetNames, evalSetName) {
//				evalSetName += "_"
//			}
//			mapping.EvalSetName = gptr.Of(evalSetName)
//			evalSetNames = append(evalSetNames, evalSetName)
//			jspnPathMapping[jspnPath] = evalSetName
//		}
//	}
//
//	return gptr.Of(ToJSONString(ctx, taskConfig))
//}
