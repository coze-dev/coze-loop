package entity

import (
	"time"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/filter"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
)

// do
type ObservabilityTask struct {
	ID                    int64                    // Task ID
	WorkspaceID           int64                    // 空间ID
	Name                  string                   // 任务名称
	Description           *string                  // 任务描述
	TaskType              string                   // 任务类型
	TaskStatus            string                   // 任务状态
	TaskDetail            *RunDetail               // 任务运行详情
	SpanFilter            *filter.SpanFilterFields // span 过滤条件
	EffectiveTime         *EffectiveTime           // 生效时间
	BackfillEffectiveTime *EffectiveTime           // 历史回溯生效时间
	Sampler               *Sampler                 // 采样器
	TaskConfig            *TaskConfig              // 相关任务的配置信息
	CreatedAt             time.Time                // 创建时间
	UpdatedAt             time.Time                // 更新时间
	CreatedBy             string                   // 创建人
	UpdatedBy             string                   // 更新人

	TaskRuns []*TaskRun
}

type RunDetail struct {
	SuccessCount int64
	FailedCount  int64
	TotalCount   int64
}
type SpanFilterFields struct {
	Filters      filter.SpanFilterFields
	PlatformType common.PlatformType
	SpanListType common.SpanListType
}
type EffectiveTime struct {
	// ms timestamp
	StartAt int64
	// ms timestamp
	EndAt int64
}
type Sampler struct {
	SampleRate    float64
	SampleSize    int64
	IsCycle       bool
	CycleCount    int64
	CycleInterval int64
	CycleTimeUnit string
}
type TaskConfig struct {
	AutoEvaluateConfigs []*AutoEvaluateConfig
	DataReflowConfig    []*DataReflowConfig
}
type AutoEvaluateConfig struct {
	EvaluatorVersionID int64
	EvaluatorID        int64
	FieldMappings      []*EvaluateFieldMapping
}
type EvaluateFieldMapping struct {
	// 数据集字段约束
	FieldSchema        *dataset.FieldSchema
	TraceFieldKey      string
	TraceFieldJsonpath string
	EvalSetName        *string
}
type DataReflowConfig struct {
	DatasetID     *int64
	DatasetName   *string
	DatasetSchema dataset.DatasetSchema
	FieldMappings []dataset.FieldMapping
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
	SuccessCount      *int64
	FailedCount       *int64
	TotalCount        *int64
	BackfillStatus    *string
	LastSpanPageToken *string
}
type TaskRunConfig struct {
	AutoEvaluateRunConfig *AutoEvaluateRunConfig
	DataReflowRunConfig   *DataReflowRunConfig
}
type AutoEvaluateRunConfig struct {
	ExptID       int64
	ExptRunID    int64
	EvalID       int64
	SchemaID     int64
	Schema       *string
	EndAt        int64
	CycleStartAt int64
	CycleEndAt   int64
	Status       string
}
type DataReflowRunConfig struct {
	DatasetID    int64
	DatasetRunID int64
	EndAt        int64
	CycleStartAt int64
	CycleEndAt   int64
	Status       string
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
