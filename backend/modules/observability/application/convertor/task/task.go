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
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	tconv "github.com/coze-dev/coze-loop/backend/modules/observability/application/convertor/trace"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/service"
	entity_common "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/common"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/slices"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func TaskPOs2DOs(ctx context.Context, taskPOs []*entity.ObservabilityTask, userInfos map[string]*entity_common.UserInfo) ([]*task.Task, error) {
	var taskList []*task.Task
	if len(taskPOs) == 0 {
		return taskList, nil
	}
	for _, v := range taskPOs {
		taskDO := TaskPO2DTO(ctx, v, userInfos)
		taskList = append(taskList, taskDO)
	}
	return taskList, nil
}
func TaskPO2DTO(ctx context.Context, v *entity.ObservabilityTask, userMap map[string]*entity_common.UserInfo) *task.Task {
	if v == nil {
		return nil
	}
	taskInfo := &task.Task{
		ID:          ptr.Of(v.ID),
		Name:        v.Name,
		Description: v.Description,
		WorkspaceID: ptr.Of(v.WorkspaceID),
		TaskType:    v.TaskType,
		TaskStatus:  ptr.Of(v.TaskStatus),
		Rule:        RulePO2DO(ctx, v.SpanFilter, v.EffectiveTime, v.Sampler),
		TaskConfig:  TaskConfigPO2DO(ctx, v.TaskConfig),
		TaskDetail:  TaskDetailsPO2DO(ctx, v.TaskDetail),
		BaseInfo: &common.BaseInfo{
			CreatedAt: gptr.Of(v.CreatedAt.UnixMilli()),
			UpdatedAt: gptr.Of(v.UpdatedAt.UnixMilli()),
			CreatedBy: UserInfoPO2DO(userMap[v.CreatedBy], v.CreatedBy),
			UpdatedBy: UserInfoPO2DO(userMap[v.UpdatedBy], v.UpdatedBy),
		},
	}
	return taskInfo
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
func TaskDetailsPO2DO(ctx context.Context, taskDetails *string) *task.TaskDetail {
	if taskDetails == nil {
		return nil
	}
	var taskDetailsDO *task.TaskDetail
	if err := sonic.Unmarshal([]byte(*taskDetails), &taskDetailsDO); err != nil {
		logs.CtxError(ctx, "TaskDetailsPO2DO sonic.Unmarshal err:%v", err)
		return nil
	}
	return taskDetailsDO
}
func RulePO2DO(ctx context.Context, spanFilter, effectiveTime, sampler *string) *task.Rule {
	var spanFilterDO *filter.SpanFilterFields
	if spanFilter != nil {
		spanFilterDO = SpanFilterPO2DO(ctx, spanFilter)
	}
	rule := &task.Rule{
		SpanFilters:   spanFilterDO,
		EffectiveTime: EffectiveTimePO2DO(ctx, effectiveTime),
		Sampler:       SamplerPO2DO(ctx, sampler),
	}
	return rule
}
func SamplerPO2DO(ctx context.Context, sampler *string) *task.Sampler {
	if sampler == nil {
		return nil
	}
	var samplerDO task.Sampler
	if err := sonic.Unmarshal([]byte(*sampler), &samplerDO); err != nil {
		logs.CtxError(ctx, "SamplerPO2DO sonic.Unmarshal err:%v", err)
		return nil
	}
	return &samplerDO
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

func TaskConfigPO2DO(ctx context.Context, taskConfig *string) *task.TaskConfig {
	if taskConfig == nil {
		return nil
	}
	var taskConfigDO task.TaskConfig
	if err := sonic.Unmarshal([]byte(*taskConfig), &taskConfigDO); err != nil {
		logs.CtxError(ctx, "TaskConfigPO2DO sonic.Unmarshal err:%v", err)
		return nil
	}
	return &taskConfigDO
}

func BatchTaskPO2DTO(ctx context.Context, Tasks []*entity.ObservabilityTask) []*task.Task {
	ret := make([]*task.Task, len(Tasks))
	for i, v := range Tasks {
		ret[i] = TaskPO2DTO(ctx, v, nil)
	}
	return ret
}
func EffectiveTimePO2DO(ctx context.Context, effectiveTime *string) *task.EffectiveTime {
	if effectiveTime == nil {
		return nil
	}
	var effectiveTimeDO task.EffectiveTime
	if err := sonic.Unmarshal([]byte(*effectiveTime), &effectiveTimeDO); err != nil {
		logs.CtxError(ctx, "EffectiveTimePO2DO sonic.Unmarshal err:%v", err)
		return nil
	}
	return &effectiveTimeDO
}
func CheckEffectiveTime(ctx context.Context, effectiveTime *task.EffectiveTime, taskStatus task.TaskStatus, effectiveTimePO *string) (*task.EffectiveTime, error) {
	effectiveTimeDO := EffectiveTimePO2DO(ctx, effectiveTimePO)
	if effectiveTimeDO == nil {
		logs.CtxError(ctx, "EffectiveTimePO2DO error")
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("effective time is nil"))
	}
	var validEffectiveTime task.EffectiveTime
	// 开始时间不能大于结束时间
	if effectiveTime.GetStartAt() >= effectiveTime.GetEndAt() {
		logs.CtxError(ctx, "Start time must be less than end time")
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("start time must be less than end time"))
	}
	// 开始、结束时间不能小于当前时间
	if effectiveTimeDO.GetStartAt() != effectiveTime.GetStartAt() && effectiveTime.GetStartAt() < time.Now().UnixMilli() {
		logs.CtxError(ctx, "update time must be greater than current time")
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("start time must be greater than current time"))
	}
	if effectiveTimeDO.GetEndAt() != effectiveTime.GetEndAt() && effectiveTime.GetEndAt() < time.Now().UnixMilli() {
		logs.CtxError(ctx, "update time must be greater than current time")
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("start time must be greater than current time"))
	}
	validEffectiveTime.StartAt = effectiveTimeDO.StartAt
	validEffectiveTime.EndAt = effectiveTimeDO.EndAt
	switch taskStatus {
	case task.TaskStatusUnstarted:
		if validEffectiveTime.StartAt != nil {
			validEffectiveTime.StartAt = effectiveTime.StartAt
		}
		if validEffectiveTime.EndAt != nil {
			validEffectiveTime.EndAt = effectiveTime.EndAt
		}
	case task.TaskStatusRunning, task.TaskStatusPending:
		if validEffectiveTime.EndAt != nil {
			validEffectiveTime.EndAt = effectiveTime.EndAt
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

func CreateTaskDTO2PO(ctx context.Context, req *service.CreateTaskReq, userID string) *entity.ObservabilityTask {
	if req == nil {
		return nil
	}
	return &entity.ObservabilityTask{
		WorkspaceID:   req.Task.GetWorkspaceID(),
		Name:          req.Task.GetName(),
		Description:   ptr.Of(req.Task.GetDescription()),
		TaskType:      req.Task.GetTaskType(),
		TaskStatus:    task.TaskStatusUnstarted,
		TaskDetail:    ptr.Of(ToJSONString(ctx, req.Task.GetTaskDetail())),
		SpanFilter:    SpanFilterDTO2PO(ctx, req.Task.GetRule().GetSpanFilters(), req.Task.GetWorkspaceID()),
		EffectiveTime: ptr.Of(ToJSONString(ctx, req.Task.GetRule().GetEffectiveTime())),
		Sampler:       ptr.Of(ToJSONString(ctx, req.Task.GetRule().GetSampler())),
		TaskConfig:    TaskConfigDTO2PO(ctx, req.Task.GetTaskConfig()),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		CreatedBy:     userID,
		UpdatedBy:     userID,
	}
}
func SpanFilterDTO2PO(ctx context.Context, filters *filter.SpanFilterFields, workspaceID int64) *string {
	var filtersDO *loop_span.FilterFields
	if filters.GetFilters() != nil {
		filtersDO = tconv.FilterFieldsDTO2DO(filters.GetFilters())
	}
	filterDO := entity.SpanFilter{
		PlatformType: filters.GetPlatformType(),
		SpanListType: filters.GetSpanListType(),
		Filters:      *filtersDO,
	}

	//todo[xun]:coze数据处理

	return ptr.Of(ToJSONString(ctx, filterDO))
}

func TaskConfigDTO2PO(ctx context.Context, taskConfig *task.TaskConfig) *string {
	if taskConfig == nil {
		return nil
	}
	var evalSetNames []string
	jspnPathMapping := make(map[string]string)
	for _, autoEvaluateConfig := range taskConfig.GetAutoEvaluateConfigs() {
		for _, mapping := range autoEvaluateConfig.GetFieldMappings() {
			jspnPath := fmt.Sprintf("%s.%s", mapping.TraceFieldKey, mapping.TraceFieldJsonpath)
			if _, exits := jspnPathMapping[jspnPath]; exits {
				mapping.EvalSetName = gptr.Of(jspnPathMapping[jspnPath])
				continue
			}
			evalSetName := getLastPartAfterDot(jspnPath)
			for exists := slices.Contains(evalSetNames, evalSetName); exists; exists = slices.Contains(evalSetNames, evalSetName) {
				evalSetName += "_"
			}
			mapping.EvalSetName = gptr.Of(evalSetName)
			evalSetNames = append(evalSetNames, evalSetName)
			jspnPathMapping[jspnPath] = evalSetName
		}
	}

	return gptr.Of(ToJSONString(ctx, taskConfig))
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
