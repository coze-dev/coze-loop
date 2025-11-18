// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"time"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

// do
type ObservabilityTask struct {
	ID                    int64             // Task ID
	WorkspaceID           int64             // 空间ID
	Name                  string            // 任务名称
	Description           *string           // 任务描述
	TaskType              string            // 任务类型
	TaskStatus            string            // 任务状态
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
	PlatformType common.PlatformType    `json:"platform_type"`
	SpanListType common.SpanListType    `json:"span_list_type"`
}
type EffectiveTime struct {
	// ms timestamp
	StartAt int64 `json:"start_at"`
	// ms timestamp
	EndAt int64 `json:"end_at"`
}
type Sampler struct {
	SampleRate    float64 `json:"sample_rate"`
	SampleSize    int64   `json:"sample_size"`
	IsCycle       bool    `json:"is_cycle"`
	CycleCount    int64   `json:"cycle_count"`
	CycleInterval int64   `json:"cycle_interval"`
	CycleTimeUnit string  `json:"cycle_time_unit"`
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
	TaskType       string          // 任务类型
	RunStatus      string          // Task Run状态
	RunDetail      *RunDetail      // Task Run运行详情
	BackfillDetail *BackfillDetail // 历史回溯运行详情
	RunStartAt     time.Time       // run 开始时间
	RunEndAt       time.Time       // run 结束时间
	TaskRunConfig  *TaskRunConfig  // 相关任务的配置信息
	CreatedAt      time.Time       // 创建时间
	UpdatedAt      time.Time       // 更新时间
}
type BackfillDetail struct {
	SuccessCount      *int64  `json:"success_count"`
	FailedCount       *int64  `json:"failed_count"`
	TotalCount        *int64  `json:"total_count"`
	BackfillStatus    *string `json:"backfill_status"`
	LastSpanPageToken *string `json:"last_span_page_token"`
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

func (t ObservabilityTask) GetRunTimeRange() (startAt, endAt int64) {
	if t.EffectiveTime == nil {
		return 0, 0
	}
	startAt = t.EffectiveTime.StartAt
	if !t.Sampler.IsCycle {
		endAt = t.EffectiveTime.EndAt
	} else {
		switch t.Sampler.CycleTimeUnit {
		case task.TimeUnitDay:
			endAt = startAt + (t.Sampler.CycleInterval)*24*time.Hour.Milliseconds()
		case task.TimeUnitWeek:
			endAt = startAt + (t.Sampler.CycleInterval)*7*24*time.Hour.Milliseconds()
		default:
			endAt = startAt + (t.Sampler.CycleInterval)*24*time.Hour.Milliseconds()
		}
	}
	return startAt, endAt
}

func (t ObservabilityTask) IsFinished() bool {
	switch t.TaskStatus {
	case task.TaskStatusSuccess, task.TaskStatusDisabled, task.TaskStatusPending:
		return true
	default:
		return false
	}
}

func (t ObservabilityTask) GetBackfillTaskRun() *TaskRun {
	for _, taskRunPO := range t.TaskRuns {
		if taskRunPO.TaskType == task.TaskRunTypeBackFill {
			return taskRunPO
		}
	}
	return nil
}

func (t ObservabilityTask) GetCurrentTaskRun() *TaskRun {
	for _, taskRunPO := range t.TaskRuns {
		if taskRunPO.TaskType == task.TaskRunTypeNewData && taskRunPO.RunStatus == task.TaskStatusRunning {
			return taskRunPO
		}
	}
	return nil
}

func (t ObservabilityTask) GetTaskttl() int64 {
	ttl := 30 * 24 * time.Hour.Milliseconds()
	if t.EffectiveTime != nil && t.EffectiveTime.EndAt != 0 && t.EffectiveTime.EndAt > time.Now().UnixMilli() {
		ttl += t.EffectiveTime.EndAt - time.Now().UnixMilli()
	}
	return ttl
}
