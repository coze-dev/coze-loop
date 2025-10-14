// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"github.com/bytedance/sonic"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func TaskDO2PO(task *entity.ObservabilityTask) *model.ObservabilityTask {
	return &model.ObservabilityTask{
		ID:                    task.ID,
		WorkspaceID:           task.WorkspaceID,
		Name:                  task.Name,
		Description:           task.Description,
		TaskType:              task.TaskType,
		TaskStatus:            task.TaskStatus,
		TaskDetail:            ptr.Of(ToJSONString(task.TaskDetail)),
		SpanFilter:            ptr.Of(ToJSONString(task.SpanFilter)),
		EffectiveTime:         ptr.Of(ToJSONString(task.EffectiveTime)),
		BackfillEffectiveTime: ptr.Of(ToJSONString(task.BackfillEffectiveTime)),
		Sampler:               ptr.Of(ToJSONString(task.Sampler)),
		TaskConfig:            ptr.Of(ToJSONString(task.TaskConfig)),
		CreatedAt:             task.CreatedAt,
		UpdatedAt:             task.UpdatedAt,
		CreatedBy:             task.CreatedBy,
		UpdatedBy:             task.UpdatedBy,
	}
}

func TaskPO2DO(task *model.ObservabilityTask) *entity.ObservabilityTask {
	return &entity.ObservabilityTask{
		ID:                    task.ID,
		WorkspaceID:           task.WorkspaceID,
		Name:                  task.Name,
		Description:           task.Description,
		TaskType:              task.TaskType,
		TaskStatus:            task.TaskStatus,
		TaskDetail:            TaskDetailJSON2DO(task.TaskDetail),
		SpanFilter:            SpanFilterJSON2DO(task.SpanFilter),
		EffectiveTime:         EffectiveTimeJSON2DO(task.EffectiveTime),
		BackfillEffectiveTime: EffectiveTimeJSON2DO(task.BackfillEffectiveTime),
		Sampler:               SamplerJSON2DO(task.Sampler),
		TaskConfig:            TaskConfigJSON2DO(task.TaskConfig),
		CreatedAt:             task.CreatedAt,
		UpdatedAt:             task.UpdatedAt,
		CreatedBy:             task.CreatedBy,
		UpdatedBy:             task.UpdatedBy,
	}
}

func TaskDetailJSON2DO(taskDetail *string) *entity.RunDetail {
	if taskDetail == nil || *taskDetail == "" {
		return nil
	}
	var taskDetailDO *entity.RunDetail
	if err := sonic.UnmarshalString(*taskDetail, &taskDetailDO); err != nil {
		logs.Error("TaskDetailJSON2DO UnmarshalString err: %v", err)
		return nil
	}
	return taskDetailDO
}

func SpanFilterJSON2DO(spanFilter *string) *filter.SpanFilterFields {
	if spanFilter == nil || *spanFilter == "" {
		return nil
	}
	var spanFilterDO *filter.SpanFilterFields
	if err := sonic.UnmarshalString(*spanFilter, &spanFilterDO); err != nil {
		logs.Error("SpanFilterJSON2DO UnmarshalString err: %v", err)
		return nil
	}
	return spanFilterDO
}

func EffectiveTimeJSON2DO(effectiveTime *string) *entity.EffectiveTime {
	if effectiveTime == nil || *effectiveTime == "" {
		return nil
	}
	var effectiveTimeDO *entity.EffectiveTime
	if err := sonic.UnmarshalString(*effectiveTime, &effectiveTimeDO); err != nil {
		logs.Error("EffectiveTimeJSON2DO UnmarshalString err: %v", err)
		return nil
	}
	return effectiveTimeDO
}

func SamplerJSON2DO(sampler *string) *entity.Sampler {
	if sampler == nil || *sampler == "" {
		return nil
	}
	var samplerDO *entity.Sampler
	if err := sonic.UnmarshalString(*sampler, &samplerDO); err != nil {
		logs.Error("SamplerJSON2DO UnmarshalString err: %v", err)
		return nil
	}
	return samplerDO
}

func TaskConfigJSON2DO(taskConfig *string) *entity.TaskConfig {
	if taskConfig == nil || *taskConfig == "" {
		return nil
	}
	var taskConfigDO *entity.TaskConfig
	if err := sonic.UnmarshalString(*taskConfig, &taskConfigDO); err != nil {
		logs.Error("TaskConfigJSON2DO UnmarshalString err: %v", err)
		return nil
	}
	return taskConfigDO
}

func TaskRunDO2PO(taskRun *entity.TaskRun) *model.ObservabilityTaskRun {
	return &model.ObservabilityTaskRun{
		ID:             taskRun.ID,
		TaskID:         taskRun.TaskID,
		WorkspaceID:    taskRun.WorkspaceID,
		TaskType:       taskRun.TaskType,
		RunStatus:      taskRun.RunStatus,
		RunDetail:      ptr.Of(ToJSONString(taskRun.RunDetail)),
		BackfillDetail: ptr.Of(ToJSONString(taskRun.BackfillDetail)),
		RunStartAt:     taskRun.RunStartAt,
		RunEndAt:       taskRun.RunEndAt,
		RunConfig:      ptr.Of(ToJSONString(taskRun.TaskRunConfig)),
		CreatedAt:      taskRun.CreatedAt,
		UpdatedAt:      taskRun.UpdatedAt,
	}
}

func TaskRunPO2DO(taskRun *model.ObservabilityTaskRun) *entity.TaskRun {
	return &entity.TaskRun{
		ID:             taskRun.ID,
		TaskID:         taskRun.TaskID,
		WorkspaceID:    taskRun.WorkspaceID,
		TaskType:       taskRun.TaskType,
		RunStatus:      taskRun.RunStatus,
		RunDetail:      RunDetailJSON2DO(taskRun.RunDetail),
		BackfillDetail: BackfillRunDetailJSON2DO(taskRun.BackfillDetail),
		RunStartAt:     taskRun.RunStartAt,
		RunEndAt:       taskRun.RunEndAt,
		TaskRunConfig:  TaskRunConfigJSON2DO(taskRun.RunConfig),
		CreatedAt:      taskRun.CreatedAt,
		UpdatedAt:      taskRun.UpdatedAt,
	}
}

func RunDetailJSON2DO(runDetail *string) *entity.RunDetail {
	if runDetail == nil || *runDetail == "" {
		return nil
	}
	var runDetailDO *entity.RunDetail
	if err := sonic.UnmarshalString(*runDetail, &runDetailDO); err != nil {
		logs.Error("RunDetailJSON2DO UnmarshalString err: %v", err)
		return nil
	}
	return runDetailDO
}
func BackfillRunDetailJSON2DO(backfillDetail *string) *entity.BackfillDetail {
	if backfillDetail == nil || *backfillDetail == "" {
		return nil
	}
	var backfillDetailDO *entity.BackfillDetail
	if err := sonic.UnmarshalString(*backfillDetail, &backfillDetailDO); err != nil {
		logs.Error("BackfillRunDetailJSON2DO UnmarshalString err: %v", err)
		return nil
	}
	return backfillDetailDO
}
func TaskRunConfigJSON2DO(taskRunConfig *string) *entity.TaskRunConfig {
	if taskRunConfig == nil || *taskRunConfig == "" {
		return nil
	}
	var taskRunConfigDO *entity.TaskRunConfig
	if err := sonic.UnmarshalString(*taskRunConfig, &taskRunConfigDO); err != nil {
		logs.Error("TaskRunConfigJSON2DO UnmarshalString err: %v", err)
		return nil
	}
	return taskRunConfigDO
}

func TaskRunsPO2DO(taskRun []*model.ObservabilityTaskRun) []*entity.TaskRun {
	if taskRun == nil {
		return nil
	}
	resp := make([]*entity.TaskRun, len(taskRun))
	for i, tr := range taskRun {
		resp[i] = TaskRunPO2DO(tr)
	}
	return resp
}

// ToJSONString 通用函数，将对象转换为 JSON 字符串指针
func ToJSONString(obj interface{}) string {
	if obj == nil {
		return ""
	}
	jsonData, err := sonic.Marshal(obj)
	if err != nil {
		return ""
	}
	jsonStr := string(jsonData)
	return jsonStr
}
