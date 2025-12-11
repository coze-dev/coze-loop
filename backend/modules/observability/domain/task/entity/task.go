// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/dataset"
	taskdto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type TimeUnit string

const (
	TimeUnitDay  = "day"
	TimeUnitWeek = "week"
	TimeUnitNull = "null"
)

type TaskStatus string

const (
	TaskStatusUnstarted TaskStatus = "unstarted"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusSuccess   TaskStatus = "success"
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusDisabled  TaskStatus = "disabled"
)

type TaskType string

const (
	TaskTypeAutoEval       TaskType = "auto_evaluate"
	TaskTypeAutoDataReflow TaskType = "auto_data_reflow"
)

type TaskRunType string

const (
	TaskRunTypeBackFill TaskRunType = "back_fill"
	TaskRunTypeNewData  TaskRunType = "new_data"
)

type TaskRunStatus string

const (
	TaskRunStatusRunning TaskRunStatus = "running"
	TaskRunStatusDone    TaskRunStatus = "done"
)

type StatusChangeEvent struct {
	Before TaskStatus
	After  TaskStatus
}

// do
type ObservabilityTask struct {
	ID                    int64             // Task ID
	WorkspaceID           int64             // 空间ID
	Name                  string            // 任务名称
	Description           *string           // 任务描述
	TaskType              TaskType          // 任务类型
	TaskStatus            TaskStatus        // 任务状态
	TaskDetail            *RunDetail        // 任务运行详情
	SpanFilter            *SpanFilterFields // span 过滤条件
	EffectiveTime         *EffectiveTime    // 生效时间
	BackfillEffectiveTime *EffectiveTime    // 历史回溯生效时间
	Sampler               *Sampler          // 采样器
	TaskConfig            *TaskConfig       // 相关任务的配置信息
	CreatedAt             time.Time         // 创建时间
	UpdatedAt             time.Time         // 更新时间
	CreatedBy             string            // 创建人
	UpdatedBy             string            // 更新人
	TaskSource            *string           // 创建来源

	TaskRuns []*TaskRun
}

type RunDetail struct {
	SuccessCount int64 `json:"success_count"`
	FailedCount  int64 `json:"failed_count"`
	TotalCount   int64 `json:"total_count"`
}
type SpanFilterFields struct {
	Filters      loop_span.FilterFields `json:"filters"`
	PlatformType loop_span.PlatformType `json:"platform_type"`
	SpanListType loop_span.SpanListType `json:"span_list_type"`
}
type EffectiveTime struct {
	// ms timestamp
	StartAt int64 `json:"start_at"`
	// ms timestamp
	EndAt int64 `json:"end_at"`
}
type Sampler struct {
	SampleRate    float64  `json:"sample_rate"`
	SampleSize    int64    `json:"sample_size"`
	IsCycle       bool     `json:"is_cycle"`
	CycleCount    int64    `json:"cycle_count"`
	CycleInterval int64    `json:"cycle_interval"`
	CycleTimeUnit TimeUnit `json:"cycle_time_unit"`
}
type TaskConfig struct {
	AutoEvaluateConfigs []*AutoEvaluateConfig `json:"auto_evaluate_configs"`
	DataReflowConfig    []*DataReflowConfig
}
type AutoEvaluateConfig struct {
	EvaluatorVersionID int64                   `json:"evaluator_version_id"`
	EvaluatorID        int64                   `json:"evaluator_id"`
	FieldMappings      []*EvaluateFieldMapping `json:"field_mappings"`
}
type EvaluateFieldMapping struct {
	// 数据集字段约束
	FieldSchema        *dataset.FieldSchema `json:"field_schema"`
	TraceFieldKey      string               `json:"trace_field_key"`
	TraceFieldJsonpath string               `json:"trace_field_jsonpath"`
	EvalSetName        *string              `json:"eval_set_name"`
}
type DataReflowConfig struct {
	DatasetID     *int64                 `json:"dataset_id"`
	DatasetName   *string                `json:"dataset_name"`
	DatasetSchema dataset.DatasetSchema  `json:"dataset_schema"`
	FieldMappings []dataset.FieldMapping `json:"field_mappings"`
}

type TaskRun struct {
	ID             int64           // Task Run ID
	TaskID         int64           // Task ID
	WorkspaceID    int64           // 空间ID
	TaskType       TaskRunType     // 任务类型
	RunStatus      TaskRunStatus   // Task Run状态
	RunDetail      *RunDetail      // Task Run运行详情
	BackfillDetail *BackfillDetail // 历史回溯运行详情
	RunStartAt     time.Time       // run 开始时间
	RunEndAt       time.Time       // run 结束时间
	TaskRunConfig  *TaskRunConfig  // 相关任务的配置信息
	CreatedAt      time.Time       // 创建时间
	UpdatedAt      time.Time       // 更新时间
}
type BackfillDetail struct {
	SuccessCount      int64  `json:"success_count,omitempty"`
	FailedCount       int64  `json:"failed_count,omitempty"`
	TotalCount        int64  `json:"total_count,omitempty"`
	BackfillStatus    string `json:"backfill_status,omitempty"`
	LastSpanPageToken string `json:"last_span_page_token,omitempty"`
}

type TaskRunConfig struct {
	AutoEvaluateRunConfig *AutoEvaluateRunConfig `json:"auto_evaluate_run_config"`
	DataReflowRunConfig   *DataReflowRunConfig   `json:"data_reflow_run_config"`
}
type AutoEvaluateRunConfig struct {
	ExptID       int64   `json:"expt_id"`
	ExptRunID    int64   `json:"expt_run_id"`
	EvalID       int64   `json:"eval_id"`
	SchemaID     int64   `json:"schema_id"`
	Schema       *string `json:"schema"`
	EndAt        int64   `json:"end_at"`
	CycleStartAt int64   `json:"cycle_start_at"`
	CycleEndAt   int64   `json:"cycle_end_at"`
	Status       string  `json:"status"`
}
type DataReflowRunConfig struct {
	DatasetID    int64  `json:"dataset_id"`
	DatasetRunID int64  `json:"dataset_run_id"`
	EndAt        int64  `json:"end_at"`
	CycleStartAt int64  `json:"cycle_start_at"`
	CycleEndAt   int64  `json:"cycle_end_at"`
	Status       string `json:"status"`
}

func (t *ObservabilityTask) GetRunTimeRange() (startAt, endAt int64) {
	if t.EffectiveTime == nil {
		return 0, 0
	}
	startAt = t.EffectiveTime.StartAt
	if !t.Sampler.IsCycle {
		endAt = t.EffectiveTime.EndAt
	} else {
		switch t.Sampler.CycleTimeUnit {
		case TimeUnitDay:
			endAt = startAt + (t.Sampler.CycleInterval)*24*time.Hour.Milliseconds()
		case TimeUnitWeek:
			endAt = startAt + (t.Sampler.CycleInterval)*7*24*time.Hour.Milliseconds()
		default:
			endAt = startAt + (t.Sampler.CycleInterval)*24*time.Hour.Milliseconds()
		}
	}
	return startAt, endAt
}

func (t *ObservabilityTask) IsFinished() bool {
	switch t.TaskStatus {
	case TaskStatusSuccess, TaskStatusDisabled, TaskStatusPending:
		return true
	default:
		return false
	}
}

func (t *ObservabilityTask) GetBackfillTaskRun() *TaskRun {
	for _, taskRun := range t.TaskRuns {
		if taskRun.TaskType == TaskRunTypeBackFill {
			return taskRun
		}
	}
	return nil
}

func (t *ObservabilityTask) GetCurrentTaskRun() *TaskRun {
	for _, taskRun := range t.TaskRuns {
		if taskRun.TaskType == TaskRunTypeNewData && taskRun.RunStatus == TaskRunStatusRunning {
			return taskRun
		}
	}
	return nil
}

func (t *ObservabilityTask) GetTaskttl() int64 {
	ttl := 30 * 24 * time.Hour.Milliseconds()
	if t.EffectiveTime != nil && t.EffectiveTime.EndAt != 0 && t.EffectiveTime.EndAt > time.Now().UnixMilli() {
		ttl += t.EffectiveTime.EndAt - time.Now().UnixMilli()
	}
	return ttl
}

func (t *ObservabilityTask) SetEffectiveTime(ctx context.Context, effectiveTime EffectiveTime) error {
	if t.EffectiveTime == nil {
		logs.CtxError(ctx, "EffectiveTime is null.")
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("effective time is nil"))
	}
	// 开始时间不能大于结束时间
	if effectiveTime.StartAt >= effectiveTime.EndAt {
		logs.CtxError(ctx, "Start time must be less than end time")
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("start time must be less than end time"))
	}
	// 开始、结束时间不能小于当前时间
	if t.EffectiveTime.StartAt != effectiveTime.StartAt && effectiveTime.StartAt < time.Now().UnixMilli() {
		logs.CtxError(ctx, "update time must be greater than current time")
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("start time must be greater than current time"))
	}
	if t.EffectiveTime.EndAt != effectiveTime.EndAt && effectiveTime.EndAt < time.Now().UnixMilli() {
		logs.CtxError(ctx, "update time must be greater than current time")
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("start time must be greater than current time"))
	}
	switch t.TaskStatus {
	case TaskStatusUnstarted:
		if effectiveTime.StartAt != 0 {
			t.EffectiveTime.StartAt = effectiveTime.StartAt
		}
		if effectiveTime.EndAt != 0 {
			t.EffectiveTime.EndAt = effectiveTime.EndAt
		}
	case TaskStatusRunning, TaskStatusPending:
		if effectiveTime.EndAt != 0 {
			t.EffectiveTime.EndAt = effectiveTime.EndAt
		}
	default:
		logs.CtxError(ctx, "Invalid task status:%s", t.TaskStatus)
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid task status"))
	}
	return nil
}

func (t *ObservabilityTask) SetTaskStatus(ctx context.Context, taskStatus TaskStatus) (*StatusChangeEvent, error) {
	currentTaskStatus := t.TaskStatus
	if currentTaskStatus == taskStatus {
		return nil, nil
	}

	switch taskStatus {
	case taskdto.TaskStatusUnstarted:
		break
	case taskdto.TaskStatusRunning:
		if currentTaskStatus == taskdto.TaskStatusUnstarted || currentTaskStatus == taskdto.TaskStatusPending {
			t.TaskStatus = taskStatus
			return &StatusChangeEvent{
				Before: currentTaskStatus,
				After:  taskStatus,
			}, nil
		}
	case taskdto.TaskStatusPending:
		if currentTaskStatus == taskdto.TaskStatusRunning {
			t.TaskStatus = taskStatus
			return &StatusChangeEvent{
				Before: currentTaskStatus,
				After:  taskStatus,
			}, nil
		}
	case taskdto.TaskStatusDisabled:
		if currentTaskStatus == taskdto.TaskStatusUnstarted || currentTaskStatus == taskdto.TaskStatusPending {
			t.TaskStatus = taskStatus
			return &StatusChangeEvent{
				Before: currentTaskStatus,
				After:  taskStatus,
			}, nil
		}
	case taskdto.TaskStatusSuccess:
		break
	}

	logs.CtxError(ctx, "Invalid task status. Before:[%s], after:[%s]", currentTaskStatus, taskStatus)
	return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("invalid task status"))
}

func (t *ObservabilityTask) ShouldTriggerBackfill() bool {
	// 检查回填时间配置
	if t.BackfillEffectiveTime == nil {
		return false
	}

	return t.BackfillEffectiveTime.StartAt > 0 &&
		t.BackfillEffectiveTime.EndAt > 0 &&
		t.BackfillEffectiveTime.StartAt < t.BackfillEffectiveTime.EndAt
}

func (t *ObservabilityTask) GetPlatformType() loop_span.PlatformType {
	if t.SpanFilter != nil {
		return t.SpanFilter.PlatformType
	}
	return loop_span.PlatformDefault
}
