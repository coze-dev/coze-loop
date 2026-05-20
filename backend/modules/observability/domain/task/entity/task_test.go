// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package entity

import (
	"context"
	"reflect"
	"testing"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/dataset"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func TestObservabilityTask_SetTaskStatus(t *testing.T) {
	tests := []struct {
		name         string             // 测试用例名称
		initialTask  ObservabilityTask  // 任务的初始状态
		targetStatus TaskStatus         // 目标设置的状态
		wantEvent    *StatusChangeEvent // 期望返回的事件
		wantErr      bool               // 是否期望发生错误
		finalStatus  TaskStatus         // 期望的最终任务状态
	}{
		{
			name:         "状态相同时不进行变更",
			initialTask:  ObservabilityTask{TaskStatus: TaskStatusRunning},
			targetStatus: TaskStatusRunning,
			wantEvent:    nil,
			wantErr:      false,
			finalStatus:  TaskStatusRunning,
		},
		{
			name:         "有效状态流转：从未开始到运行中",
			initialTask:  ObservabilityTask{TaskStatus: TaskStatusUnstarted},
			targetStatus: TaskStatusRunning,
			wantEvent: &StatusChangeEvent{
				Before: TaskStatusUnstarted,
				After:  TaskStatusRunning,
			},
			wantErr:     false,
			finalStatus: TaskStatusRunning,
		},
		{
			name:         "有效状态流转：从挂起到运行中",
			initialTask:  ObservabilityTask{TaskStatus: TaskStatusPending},
			targetStatus: TaskStatusRunning,
			wantEvent: &StatusChangeEvent{
				Before: TaskStatusPending,
				After:  TaskStatusRunning,
			},
			wantErr:     false,
			finalStatus: TaskStatusRunning,
		},
		{
			name:         "无效状态流转：从禁用状态到其他状态",
			initialTask:  ObservabilityTask{TaskStatus: TaskStatusDisabled},
			targetStatus: TaskStatusRunning,
			wantEvent:    nil,
			wantErr:      true,
			finalStatus:  TaskStatusDisabled,
		},
	}

	// 遍历并执行所有测试用例
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: 创建一个任务副本以防止并发测试时修改原始测试用例数据
			task := tt.initialTask

			// Act: 调用被测方法
			gotEvent, err := task.SetTaskStatus(context.Background(), tt.targetStatus)

			// Assert: 校验错误是否符合预期
			if (err != nil) != tt.wantErr {
				t.Errorf("SetTaskStatus() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Assert: 校验返回的事件是否符合预期
			if !reflect.DeepEqual(gotEvent, tt.wantEvent) {
				t.Errorf("SetTaskStatus() gotEvent = %v, want %v", gotEvent, tt.wantEvent)
			}

			// Assert: 校验任务的最终状态是否符合预期
			if task.TaskStatus != tt.finalStatus {
				t.Errorf("Final task status = %v, want %v", task.TaskStatus, tt.finalStatus)
			}
		})
	}
}

func TestObservabilityTask_needAnnotationForOutput(t *testing.T) {
	tests := []struct {
		name string
		task *ObservabilityTask
		want bool
	}{
		{
			name: "TaskConfig 为 nil 时返回 false",
			task: &ObservabilityTask{TaskConfig: nil},
			want: false,
		},
		{
			name: "DataReflowConfig 和 AutoEvaluateConfigs 都为空时返回 false",
			task: &ObservabilityTask{TaskConfig: &TaskConfig{}},
			want: false,
		},
		{
			name: "DataReflowConfig 中有 Feedback 前缀字段时返回 true",
			task: &ObservabilityTask{
				TaskConfig: &TaskConfig{
					DataReflowConfig: []*DataReflowConfig{
						{
							FieldMappings: []dataset.FieldMapping{
								{TraceFieldKey: "Input.content"},
								{TraceFieldKey: "Feedback.score"},
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "AutoEvaluateConfigs 中有 Feedback 前缀字段时返回 true",
			task: &ObservabilityTask{
				TaskConfig: &TaskConfig{
					AutoEvaluateConfigs: []*AutoEvaluateConfig{
						{
							FieldMappings: []*EvaluateFieldMapping{
								{TraceFieldKey: "Feedback.rating"},
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "无 Feedback 前缀字段时返回 false",
			task: &ObservabilityTask{
				TaskConfig: &TaskConfig{
					DataReflowConfig: []*DataReflowConfig{
						{
							FieldMappings: []dataset.FieldMapping{
								{TraceFieldKey: "Input.content"},
								{TraceFieldKey: "Output.result"},
							},
						},
					},
					AutoEvaluateConfigs: []*AutoEvaluateConfig{
						{
							FieldMappings: []*EvaluateFieldMapping{
								{TraceFieldKey: "Input.query"},
							},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "多个 DataReflowConfig 第二个包含 Feedback",
			task: &ObservabilityTask{
				TaskConfig: &TaskConfig{
					DataReflowConfig: []*DataReflowConfig{
						{
							FieldMappings: []dataset.FieldMapping{
								{TraceFieldKey: "Input.content"},
							},
						},
						{
							FieldMappings: []dataset.FieldMapping{
								{TraceFieldKey: "Feedback.thumbs_up"},
							},
						},
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.task.needAnnotationForOutput()
			if got != tt.want {
				t.Errorf("needAnnotationForOutput() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestObservabilityTask_needAnnotationForFilter(t *testing.T) {
	tests := []struct {
		name string
		task *ObservabilityTask
		want bool
	}{
		{
			name: "SpanFilter 为 nil 时返回 false",
			task: &ObservabilityTask{SpanFilter: nil},
			want: false,
		},
		{
			name: "过滤条件为空时返回 false",
			task: &ObservabilityTask{
				SpanFilter: &SpanFilterFields{
					Filters: loop_span.FilterFields{
						FilterFields: []*loop_span.FilterField{},
					},
				},
			},
			want: false,
		},
		{
			name: "过滤条件包含 manual_feedback 前缀字段时返回 true",
			task: &ObservabilityTask{
				SpanFilter: &SpanFilterFields{
					Filters: loop_span.FilterFields{
						QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
						FilterFields: []*loop_span.FilterField{
							{FieldName: "manual_feedback_score", FieldType: loop_span.FieldTypeLong, QueryType: ptr.Of(loop_span.QueryTypeEnumGte)},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "过滤条件包含 feedback_openapi 前缀字段时返回 true",
			task: &ObservabilityTask{
				SpanFilter: &SpanFilterFields{
					Filters: loop_span.FilterFields{
						QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
						FilterFields: []*loop_span.FilterField{
							{FieldName: "feedback_openapi_like", FieldType: loop_span.FieldTypeLong, QueryType: ptr.Of(loop_span.QueryTypeEnumEq)},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "过滤条件包含 auto_evaluate 前缀字段时返回 true",
			task: &ObservabilityTask{
				SpanFilter: &SpanFilterFields{
					Filters: loop_span.FilterFields{
						QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
						FilterFields: []*loop_span.FilterField{
							{FieldName: "auto_evaluate_coherence", FieldType: loop_span.FieldTypeLong, QueryType: ptr.Of(loop_span.QueryTypeEnumGte)},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "过滤条件不包含 annotation 字段时返回 false",
			task: &ObservabilityTask{
				SpanFilter: &SpanFilterFields{
					Filters: loop_span.FilterFields{
						QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
						FilterFields: []*loop_span.FilterField{
							{FieldName: "status_code", FieldType: loop_span.FieldTypeLong, QueryType: ptr.Of(loop_span.QueryTypeEnumEq)},
						},
					},
				},
			},
			want: false,
		},
		{
			name: "嵌套 SubFilter 中包含 annotation 字段时返回 true",
			task: &ObservabilityTask{
				SpanFilter: &SpanFilterFields{
					Filters: loop_span.FilterFields{
						QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
						FilterFields: []*loop_span.FilterField{
							{
								QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumOr),
								SubFilter: &loop_span.FilterFields{
									QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumOr),
									FilterFields: []*loop_span.FilterField{
										{FieldName: "manual_feedback_rating", FieldType: loop_span.FieldTypeLong, QueryType: ptr.Of(loop_span.QueryTypeEnumGte)},
									},
								},
							},
						},
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.task.needAnnotationForFilter()
			if got != tt.want {
				t.Errorf("needAnnotationForFilter() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestObservabilityTask_BackfillNeedQueryAnnotation(t *testing.T) {
	tests := []struct {
		name string
		task *ObservabilityTask
		want bool
	}{
		{
			name: "无 Feedback 输出字段时返回 false",
			task: &ObservabilityTask{TaskConfig: &TaskConfig{}},
			want: false,
		},
		{
			name: "有 Feedback 输出字段时返回 true",
			task: &ObservabilityTask{
				TaskConfig: &TaskConfig{
					DataReflowConfig: []*DataReflowConfig{
						{
							FieldMappings: []dataset.FieldMapping{
								{TraceFieldKey: "Feedback.score"},
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "即使有 annotation filter 也只看 output，filter 不影响",
			task: &ObservabilityTask{
				TaskConfig: &TaskConfig{},
				SpanFilter: &SpanFilterFields{
					Filters: loop_span.FilterFields{
						QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
						FilterFields: []*loop_span.FilterField{
							{FieldName: "manual_feedback_score", FieldType: loop_span.FieldTypeLong, QueryType: ptr.Of(loop_span.QueryTypeEnumGte)},
						},
					},
				},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.task.BackfillNeedQueryAnnotation()
			if got != tt.want {
				t.Errorf("BackfillNeedQueryAnnotation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestObservabilityTask_NewDataNeedQueryAnnotation(t *testing.T) {
	tests := []struct {
		name string
		task *ObservabilityTask
		want bool
	}{
		{
			name: "无 annotation filter 且无 Feedback 输出时返回 false",
			task: &ObservabilityTask{
				TaskConfig: &TaskConfig{},
				SpanFilter: &SpanFilterFields{
					Filters: loop_span.FilterFields{
						FilterFields: []*loop_span.FilterField{},
					},
				},
			},
			want: false,
		},
		{
			name: "有 annotation filter 时返回 true",
			task: &ObservabilityTask{
				TaskConfig: &TaskConfig{},
				SpanFilter: &SpanFilterFields{
					Filters: loop_span.FilterFields{
						QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
						FilterFields: []*loop_span.FilterField{
							{FieldName: "manual_feedback_score", FieldType: loop_span.FieldTypeLong, QueryType: ptr.Of(loop_span.QueryTypeEnumGte)},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "有 Feedback 输出字段时返回 true",
			task: &ObservabilityTask{
				TaskConfig: &TaskConfig{
					AutoEvaluateConfigs: []*AutoEvaluateConfig{
						{
							FieldMappings: []*EvaluateFieldMapping{
								{TraceFieldKey: "Feedback.rating"},
							},
						},
					},
				},
			},
			want: true,
		},
		{
			name: "同时有 annotation filter 和 Feedback 输出时返回 true",
			task: &ObservabilityTask{
				TaskConfig: &TaskConfig{
					DataReflowConfig: []*DataReflowConfig{
						{
							FieldMappings: []dataset.FieldMapping{
								{TraceFieldKey: "Feedback.score"},
							},
						},
					},
				},
				SpanFilter: &SpanFilterFields{
					Filters: loop_span.FilterFields{
						QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
						FilterFields: []*loop_span.FilterField{
							{FieldName: "auto_evaluate_coherence", FieldType: loop_span.FieldTypeLong, QueryType: ptr.Of(loop_span.QueryTypeEnumGte)},
						},
					},
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.task.NewDataNeedQueryAnnotation()
			if got != tt.want {
				t.Errorf("NewDataNeedQueryAnnotation() = %v, want %v", got, tt.want)
			}
		})
	}
}
