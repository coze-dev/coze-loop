// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package converter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/dao"
)

func TestAnnotationPO2DO(t *testing.T) {
	tests := []struct {
		name   string
		input  *dao.Annotation
		verify func(t *testing.T, result *loop_span.Annotation)
	}{
		{
			name:  "nil input returns nil",
			input: nil,
			verify: func(t *testing.T, result *loop_span.Annotation) {
				assert.Nil(t, result)
			},
		},
		{
			name: "basic fields mapping with string value type",
			input: &dao.Annotation{
				ID:              "test-id",
				SpanID:          "span-1",
				TraceID:         "trace-1",
				StartTime:       1700000000000000,
				SpaceID:         "workspace-1",
				AnnotationType:  "auto_evaluate",
				AnnotationIndex: []string{"idx1"},
				Key:             "test-key",
				Reasoning:       "test-reasoning",
				Status:          "normal",
				CreatedBy:       "user1",
				CreatedAt:       1700000000000000,
				UpdatedBy:       "user2",
				UpdatedAt:       1700000000000001,
				ValueType:       "string",
				ValueString:     "hello",
			},
			verify: func(t *testing.T, result *loop_span.Annotation) {
				require.NotNil(t, result)
				assert.Equal(t, "test-id", result.ID)
				assert.Equal(t, "span-1", result.SpanID)
				assert.Equal(t, "trace-1", result.TraceID)
				assert.Equal(t, time.UnixMicro(1700000000000000), result.StartTime)
				assert.Equal(t, "workspace-1", result.WorkspaceID)
				assert.Equal(t, loop_span.AnnotationType("auto_evaluate"), result.AnnotationType)
				assert.Equal(t, []string{"idx1"}, result.AnnotationIndex)
				assert.Equal(t, "test-key", result.Key)
				assert.Equal(t, "test-reasoning", result.Reasoning)
				assert.Equal(t, loop_span.AnnotationStatus("normal"), result.Status)
				assert.Equal(t, "user1", result.CreatedBy)
				assert.Equal(t, time.UnixMicro(1700000000000000), result.CreatedAt)
				assert.Equal(t, "user2", result.UpdatedBy)
				assert.Equal(t, time.UnixMicro(1700000000000001), result.UpdatedAt)
				assert.Equal(t, loop_span.AnnotationValueTypeString, result.Value.ValueType)
				assert.Equal(t, "hello", result.Value.StringValue)
			},
		},
		{
			name: "long value type",
			input: &dao.Annotation{
				ID:        "id-long",
				SpanID:    "span-1",
				TraceID:   "trace-1",
				ValueType: "long",
				ValueLong: 42,
			},
			verify: func(t *testing.T, result *loop_span.Annotation) {
				require.NotNil(t, result)
				assert.Equal(t, loop_span.AnnotationValueTypeLong, result.Value.ValueType)
				assert.Equal(t, int64(42), result.Value.LongValue)
			},
		},
		{
			name: "double value type",
			input: &dao.Annotation{
				ID:         "id-double",
				SpanID:     "span-1",
				TraceID:    "trace-1",
				ValueType:  "double",
				ValueFloat: 3.14,
			},
			verify: func(t *testing.T, result *loop_span.Annotation) {
				require.NotNil(t, result)
				assert.Equal(t, loop_span.AnnotationValueTypeDouble, result.Value.ValueType)
				assert.Equal(t, 3.14, result.Value.FloatValue)
			},
		},
		{
			name: "bool value type",
			input: &dao.Annotation{
				ID:        "id-bool",
				SpanID:    "span-1",
				TraceID:   "trace-1",
				ValueType: "bool",
				ValueBool: true,
			},
			verify: func(t *testing.T, result *loop_span.Annotation) {
				require.NotNil(t, result)
				assert.Equal(t, loop_span.AnnotationValueTypeBool, result.Value.ValueType)
				assert.Equal(t, true, result.Value.BoolValue)
			},
		},
		{
			name: "auto_evaluate metadata parsing",
			input: &dao.Annotation{
				ID:             "id-auto",
				SpanID:         "span-1",
				TraceID:        "trace-1",
				AnnotationType: "auto_evaluate",
				Metadata:       `{"task_id":100,"evaluator_record_id":200,"evaluator_version_id":300,"expt_id":400,"expt_template_id":500}`,
			},
			verify: func(t *testing.T, result *loop_span.Annotation) {
				require.NotNil(t, result)
				metadata, ok := result.Metadata.(loop_span.AutoEvaluateMetadata)
				require.True(t, ok)
				assert.Equal(t, int64(100), metadata.TaskID)
				assert.Equal(t, int64(200), metadata.EvaluatorRecordID)
				assert.Equal(t, int64(300), metadata.EvaluatorVersionID)
				assert.Equal(t, int64(400), metadata.ExptID)
				assert.Equal(t, int64(500), metadata.ExptTemplateID)
			},
		},
		{
			name: "manual_dataset metadata parsing",
			input: &dao.Annotation{
				ID:             "id-dataset",
				SpanID:         "span-1",
				TraceID:        "trace-1",
				AnnotationType: "manual_dataset",
				Metadata:       `{"psm":"test-psm","agent_name":"test-agent"}`,
			},
			verify: func(t *testing.T, result *loop_span.Annotation) {
				require.NotNil(t, result)
				metadata, ok := result.Metadata.(loop_span.ManualDatasetMetadata)
				require.True(t, ok)
				assert.Equal(t, "test-psm", metadata.PSM)
				assert.Equal(t, "test-agent", metadata.AgentName)
			},
		},
		{
			name: "manual_evaluation_set metadata parsing",
			input: &dao.Annotation{
				ID:             "id-eval-set",
				SpanID:         "span-1",
				TraceID:        "trace-1",
				AnnotationType: "manual_evaluation_set",
				Metadata:       `{"psm":"eval-psm"}`,
			},
			verify: func(t *testing.T, result *loop_span.Annotation) {
				require.NotNil(t, result)
				metadata, ok := result.Metadata.(loop_span.ManualDatasetMetadata)
				require.True(t, ok)
				assert.Equal(t, "eval-psm", metadata.PSM)
			},
		},
		{
			name: "openapi_feedback metadata parsing",
			input: &dao.Annotation{
				ID:             "id-openapi",
				SpanID:         "span-1",
				TraceID:        "trace-1",
				AnnotationType: "openapi_feedback",
				Metadata:       `{"psm":"feedback-psm","agent_name":"feedback-agent"}`,
			},
			verify: func(t *testing.T, result *loop_span.Annotation) {
				require.NotNil(t, result)
				metadata, ok := result.Metadata.(loop_span.FeedbackMetadata)
				require.True(t, ok)
				assert.Equal(t, "feedback-psm", metadata.PSM)
				assert.Equal(t, "feedback-agent", metadata.AgentName)
			},
		},
		{
			name: "coze_feedback metadata parsing",
			input: &dao.Annotation{
				ID:             "id-coze",
				SpanID:         "span-1",
				TraceID:        "trace-1",
				AnnotationType: "coze_feedback",
				Metadata:       `{"psm":"coze-psm","agent_name":"coze-agent"}`,
			},
			verify: func(t *testing.T, result *loop_span.Annotation) {
				require.NotNil(t, result)
				metadata, ok := result.Metadata.(loop_span.FeedbackMetadata)
				require.True(t, ok)
				assert.Equal(t, "coze-psm", metadata.PSM)
				assert.Equal(t, "coze-agent", metadata.AgentName)
			},
		},
		{
			name: "manual_feedback metadata parsing",
			input: &dao.Annotation{
				ID:             "id-manual-fb",
				SpanID:         "span-1",
				TraceID:        "trace-1",
				AnnotationType: "manual_feedback",
				Metadata:       `{"psm":"manual-psm"}`,
			},
			verify: func(t *testing.T, result *loop_span.Annotation) {
				require.NotNil(t, result)
				metadata, ok := result.Metadata.(loop_span.FeedbackMetadata)
				require.True(t, ok)
				assert.Equal(t, "manual-psm", metadata.PSM)
			},
		},
		{
			name: "invalid metadata JSON does not panic",
			input: &dao.Annotation{
				ID:             "id-invalid-meta",
				SpanID:         "span-1",
				TraceID:        "trace-1",
				AnnotationType: "auto_evaluate",
				Metadata:       `{invalid json`,
			},
			verify: func(t *testing.T, result *loop_span.Annotation) {
				require.NotNil(t, result)
				assert.Nil(t, result.Metadata)
			},
		},
		{
			name: "with corrections",
			input: &dao.Annotation{
				ID:         "id-corrections",
				SpanID:     "span-1",
				TraceID:    "trace-1",
				Correction: `[{"reasoning":"corrected","value":{"value_type":"double","float_value":0.9},"type":"manual","update_at":"2024-01-01T00:00:00Z","updated_by":"admin"}]`,
			},
			verify: func(t *testing.T, result *loop_span.Annotation) {
				require.NotNil(t, result)
				require.Len(t, result.Corrections, 1)
				assert.Equal(t, "corrected", result.Corrections[0].Reasoning)
				assert.Equal(t, loop_span.AnnotationCorrectionTypeManual, result.Corrections[0].Type)
				assert.Equal(t, loop_span.AnnotationValueTypeDouble, result.Corrections[0].Value.ValueType)
				assert.Equal(t, 0.9, result.Corrections[0].Value.FloatValue)
				assert.Equal(t, "admin", result.Corrections[0].UpdatedBy)
			},
		},
		{
			name: "invalid correction JSON does not panic",
			input: &dao.Annotation{
				ID:         "id-invalid-corr",
				SpanID:     "span-1",
				TraceID:    "trace-1",
				Correction: `{not valid json`,
			},
			verify: func(t *testing.T, result *loop_span.Annotation) {
				require.NotNil(t, result)
				assert.Nil(t, result.Corrections)
			},
		},
		{
			name: "DeletedAt > 0 sets IsDeleted true",
			input: &dao.Annotation{
				ID:        "id-deleted",
				SpanID:    "span-1",
				TraceID:   "trace-1",
				DeletedAt: 1700000000000000,
			},
			verify: func(t *testing.T, result *loop_span.Annotation) {
				require.NotNil(t, result)
				assert.True(t, result.IsDeleted)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AnnotationPO2DO(tt.input)
			tt.verify(t, result)
		})
	}
}

func TestAnnotationDO2PO(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name    string
		input   *loop_span.Annotation
		verify  func(t *testing.T, result *dao.Annotation)
		wantErr bool
	}{
		{
			name: "basic fields mapping with string value",
			input: &loop_span.Annotation{
				ID:              "test-id",
				SpanID:          "span-1",
				TraceID:         "trace-1",
				StartTime:       now,
				WorkspaceID:     "workspace-1",
				AnnotationType:  loop_span.AnnotationTypeAutoEvaluate,
				AnnotationIndex: []string{"idx1"},
				Key:             "test-key",
				Reasoning:       "test-reasoning",
				Status:          loop_span.AnnotationStatusNormal,
				CreatedBy:       "user1",
				CreatedAt:       now,
				UpdatedBy:       "user2",
				UpdatedAt:       now,
				Value: loop_span.AnnotationValue{
					ValueType:   loop_span.AnnotationValueTypeString,
					StringValue: "hello",
				},
			},
			verify: func(t *testing.T, result *dao.Annotation) {
				assert.Equal(t, "test-id", result.ID)
				assert.Equal(t, "span-1", result.SpanID)
				assert.Equal(t, "trace-1", result.TraceID)
				assert.Equal(t, now.UnixMicro(), result.StartTime)
				assert.Equal(t, "workspace-1", result.SpaceID)
				assert.Equal(t, "auto_evaluate", result.AnnotationType)
				assert.Equal(t, []string{"idx1"}, result.AnnotationIndex)
				assert.Equal(t, "test-key", result.Key)
				assert.Equal(t, "test-reasoning", result.Reasoning)
				assert.Equal(t, "normal", result.Status)
				assert.Equal(t, "user1", result.CreatedBy)
				assert.Equal(t, uint64(now.UnixMicro()), result.CreatedAt)
				assert.Equal(t, "user2", result.UpdatedBy)
				assert.Equal(t, uint64(now.UnixMicro()), result.UpdatedAt)
				assert.Equal(t, "string", result.ValueType)
				assert.Equal(t, "hello", result.ValueString)
				assert.Equal(t, uint64(0), result.DeletedAt)
				assert.Equal(t, now.Format("2006-01-02"), result.StartDate)
			},
		},
		{
			name: "IsDeleted true sets DeletedAt > 0",
			input: &loop_span.Annotation{
				ID:        "test-id",
				SpanID:    "span-1",
				TraceID:   "trace-1",
				StartTime: now,
				IsDeleted: true,
				Value:     loop_span.AnnotationValue{ValueType: loop_span.AnnotationValueTypeString},
			},
			verify: func(t *testing.T, result *dao.Annotation) {
				assert.True(t, result.DeletedAt > 0)
			},
		},
		{
			name: "with corrections",
			input: &loop_span.Annotation{
				ID:        "test-id",
				SpanID:    "span-1",
				TraceID:   "trace-1",
				StartTime: now,
				Value:     loop_span.AnnotationValue{ValueType: loop_span.AnnotationValueTypeDouble, FloatValue: 0.8},
				Corrections: []loop_span.AnnotationCorrection{
					{
						Reasoning: "original",
						Value:     loop_span.AnnotationValue{ValueType: loop_span.AnnotationValueTypeDouble, FloatValue: 0.7},
						Type:      loop_span.AnnotationCorrectionTypeLLM,
						UpdateAt:  now,
						UpdatedBy: "system",
					},
				},
			},
			verify: func(t *testing.T, result *dao.Annotation) {
				assert.NotEmpty(t, result.Correction)
				assert.Contains(t, result.Correction, "original")
				assert.Contains(t, result.Correction, "llm")
			},
		},
		{
			name: "with metadata",
			input: &loop_span.Annotation{
				ID:             "test-id",
				SpanID:         "span-1",
				TraceID:        "trace-1",
				StartTime:      now,
				AnnotationType: loop_span.AnnotationTypeAutoEvaluate,
				Value:          loop_span.AnnotationValue{ValueType: loop_span.AnnotationValueTypeDouble, FloatValue: 0.9},
				Metadata: loop_span.AutoEvaluateMetadata{
					TaskID:             100,
					EvaluatorRecordID:  200,
					EvaluatorVersionID: 300,
				},
			},
			verify: func(t *testing.T, result *dao.Annotation) {
				assert.NotEmpty(t, result.Metadata)
				assert.Contains(t, result.Metadata, "task_id")
				assert.Contains(t, result.Metadata, "100")
			},
		},
		{
			name: "long value type",
			input: &loop_span.Annotation{
				ID:        "test-id",
				SpanID:    "span-1",
				TraceID:   "trace-1",
				StartTime: now,
				Value: loop_span.AnnotationValue{
					ValueType: loop_span.AnnotationValueTypeLong,
					LongValue: 999,
				},
			},
			verify: func(t *testing.T, result *dao.Annotation) {
				assert.Equal(t, "long", result.ValueType)
				assert.Equal(t, int64(999), result.ValueLong)
			},
		},
		{
			name: "double value type",
			input: &loop_span.Annotation{
				ID:        "test-id",
				SpanID:    "span-1",
				TraceID:   "trace-1",
				StartTime: now,
				Value: loop_span.AnnotationValue{
					ValueType:  loop_span.AnnotationValueTypeDouble,
					FloatValue: 2.718,
				},
			},
			verify: func(t *testing.T, result *dao.Annotation) {
				assert.Equal(t, "double", result.ValueType)
				assert.Equal(t, 2.718, result.ValueFloat)
			},
		},
		{
			name: "bool value type",
			input: &loop_span.Annotation{
				ID:        "test-id",
				SpanID:    "span-1",
				TraceID:   "trace-1",
				StartTime: now,
				Value: loop_span.AnnotationValue{
					ValueType: loop_span.AnnotationValueTypeBool,
					BoolValue: true,
				},
			},
			verify: func(t *testing.T, result *dao.Annotation) {
				assert.Equal(t, "bool", result.ValueType)
				assert.Equal(t, true, result.ValueBool)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := AnnotationDO2PO(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, result)
			tt.verify(t, result)
		})
	}
}

func TestAnnotationListPO2DO(t *testing.T) {
	annotations := []*dao.Annotation{
		{
			ID:             "id-1",
			SpanID:         "span-1",
			TraceID:        "trace-1",
			AnnotationType: "auto_evaluate",
			ValueType:      "double",
			ValueFloat:     0.8,
		},
		{
			ID:             "id-2",
			SpanID:         "span-2",
			TraceID:        "trace-2",
			AnnotationType: "manual_feedback",
			ValueType:      "string",
			ValueString:    "good",
		},
		{
			ID:             "id-3",
			SpanID:         "span-3",
			TraceID:        "trace-3",
			AnnotationType: "coze_feedback",
			ValueType:      "long",
			ValueLong:      5,
		},
	}

	result := AnnotationListPO2DO(annotations)
	require.Len(t, result, 3)
	assert.Equal(t, "id-1", result[0].ID)
	assert.Equal(t, "id-2", result[1].ID)
	assert.Equal(t, "id-3", result[2].ID)
	assert.Equal(t, loop_span.AnnotationValueTypeDouble, result[0].Value.ValueType)
	assert.Equal(t, loop_span.AnnotationValueTypeString, result[1].Value.ValueType)
	assert.Equal(t, loop_span.AnnotationValueTypeLong, result[2].Value.ValueType)
}

func TestGetStartDate(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected string
	}{
		{
			name:     "format date correctly",
			input:    time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC),
			expected: "2024-03-15",
		},
		{
			name:     "format date with single digit month and day",
			input:    time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC),
			expected: "2024-01-05",
		},
		{
			name:     "format date end of year",
			input:    time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC),
			expected: "2023-12-31",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getStartDate(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
