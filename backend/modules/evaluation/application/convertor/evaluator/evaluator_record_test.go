// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	commondto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/common"
	evaluatordto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/evaluator"
	evaluatordo "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestConvertEvaluatorRecordDTO2DO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *evaluatordto.EvaluatorRecord
		expected *evaluatordo.EvaluatorRecord
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "完整数据_包含Ext字段",
			input: &evaluatordto.EvaluatorRecord{
				ID:                 gptr.Of(int64(1)),
				ExperimentID:       gptr.Of(int64(2)),
				ExperimentRunID:    gptr.Of(int64(3)),
				ItemID:             gptr.Of(int64(4)),
				TurnID:             gptr.Of(int64(5)),
				EvaluatorVersionID: gptr.Of(int64(6)),
				TraceID:            gptr.Of("trace1"),
				LogID:              gptr.Of("log1"),
				Status:             evaluatordto.EvaluatorRunStatusPtr(evaluatordto.EvaluatorRunStatus_Success),
				BaseInfo:           &commondto.BaseInfo{},
				Ext: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
			expected: &evaluatordo.EvaluatorRecord{
				ID:                 1,
				ExperimentID:       2,
				ExperimentRunID:    3,
				ItemID:             4,
				TurnID:             5,
				EvaluatorVersionID: 6,
				TraceID:            "trace1",
				LogID:              "log1",
				Status:             evaluatordo.EvaluatorRunStatusSuccess,
				Ext: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
		},
		{
			name: "Ext字段为空",
			input: &evaluatordto.EvaluatorRecord{
				ID:                 gptr.Of(int64(1)),
				ExperimentID:       gptr.Of(int64(2)),
				ExperimentRunID:    gptr.Of(int64(3)),
				ItemID:             gptr.Of(int64(4)),
				TurnID:             gptr.Of(int64(5)),
				EvaluatorVersionID: gptr.Of(int64(6)),
				TraceID:            gptr.Of("trace1"),
				LogID:              gptr.Of("log1"),
				Status:             evaluatordto.EvaluatorRunStatusPtr(evaluatordto.EvaluatorRunStatus_Success),
				BaseInfo:           &commondto.BaseInfo{},
				Ext:                map[string]string{},
			},
			expected: &evaluatordo.EvaluatorRecord{
				ID:                 1,
				ExperimentID:       2,
				ExperimentRunID:    3,
				ItemID:             4,
				TurnID:             5,
				EvaluatorVersionID: 6,
				TraceID:            "trace1",
				LogID:              "log1",
				Status:             evaluatordo.EvaluatorRunStatusSuccess,
				Ext:                nil,
			},
		},
		{
			name: "Ext字段为nil",
			input: &evaluatordto.EvaluatorRecord{
				ID:                 gptr.Of(int64(1)),
				ExperimentID:       gptr.Of(int64(2)),
				ExperimentRunID:    gptr.Of(int64(3)),
				ItemID:             gptr.Of(int64(4)),
				TurnID:             gptr.Of(int64(5)),
				EvaluatorVersionID: gptr.Of(int64(6)),
				TraceID:            gptr.Of("trace1"),
				LogID:              gptr.Of("log1"),
				Status:             evaluatordto.EvaluatorRunStatusPtr(evaluatordto.EvaluatorRunStatus_Success),
				BaseInfo:           &commondto.BaseInfo{},
				Ext:                nil,
			},
			expected: &evaluatordo.EvaluatorRecord{
				ID:                 1,
				ExperimentID:       2,
				ExperimentRunID:    3,
				ItemID:             4,
				TurnID:             5,
				EvaluatorVersionID: 6,
				TraceID:            "trace1",
				LogID:              "log1",
				Status:             evaluatordo.EvaluatorRunStatusSuccess,
				Ext:                nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertEvaluatorRecordDTO2DO(tt.input)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected.ID, result.ID)
				assert.Equal(t, tt.expected.ExperimentID, result.ExperimentID)
				assert.Equal(t, tt.expected.ExperimentRunID, result.ExperimentRunID)
				assert.Equal(t, tt.expected.ItemID, result.ItemID)
				assert.Equal(t, tt.expected.TurnID, result.TurnID)
				assert.Equal(t, tt.expected.EvaluatorVersionID, result.EvaluatorVersionID)
				assert.Equal(t, tt.expected.TraceID, result.TraceID)
				assert.Equal(t, tt.expected.LogID, result.LogID)
				assert.Equal(t, tt.expected.Status, result.Status)
				assert.Equal(t, tt.expected.Ext, result.Ext)
				// BaseInfo, EvaluatorInputData, EvaluatorOutputData 由转换函数处理，不在这里比较
			}
		})
	}
}

func TestConvertEvaluatorRecordDO2DTO(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    *evaluatordo.EvaluatorRecord
		expected *evaluatordto.EvaluatorRecord
	}{
		{
			name:     "nil input",
			input:    nil,
			expected: nil,
		},
		{
			name: "完整数据_包含Ext字段",
			input: &evaluatordo.EvaluatorRecord{
				ID:                 1,
				ExperimentID:       2,
				ExperimentRunID:    3,
				ItemID:             4,
				TurnID:             5,
				EvaluatorVersionID: 6,
				TraceID:            "trace1",
				LogID:              "log1",
				Status:             evaluatordo.EvaluatorRunStatusSuccess,
				BaseInfo:           &evaluatordo.BaseInfo{},
				Ext: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
			expected: &evaluatordto.EvaluatorRecord{
				ID:                 gptr.Of(int64(1)),
				ExperimentID:       gptr.Of(int64(2)),
				ExperimentRunID:    gptr.Of(int64(3)),
				ItemID:             gptr.Of(int64(4)),
				TurnID:             gptr.Of(int64(5)),
				EvaluatorVersionID: gptr.Of(int64(6)),
				TraceID:            gptr.Of("trace1"),
				LogID:              gptr.Of("log1"),
				Status:             evaluatordto.EvaluatorRunStatusPtr(evaluatordto.EvaluatorRunStatus_Success),
				Ext: map[string]string{
					"key1": "value1",
					"key2": "value2",
				},
			},
		},
		{
			name: "Ext字段为空",
			input: &evaluatordo.EvaluatorRecord{
				ID:                 1,
				ExperimentID:       2,
				ExperimentRunID:    3,
				ItemID:             4,
				TurnID:             5,
				EvaluatorVersionID: 6,
				TraceID:            "trace1",
				LogID:              "log1",
				Status:             evaluatordo.EvaluatorRunStatusSuccess,
				BaseInfo:           &evaluatordo.BaseInfo{},
				Ext:                map[string]string{},
			},
			expected: &evaluatordto.EvaluatorRecord{
				ID:                 gptr.Of(int64(1)),
				ExperimentID:       gptr.Of(int64(2)),
				ExperimentRunID:    gptr.Of(int64(3)),
				ItemID:             gptr.Of(int64(4)),
				TurnID:             gptr.Of(int64(5)),
				EvaluatorVersionID: gptr.Of(int64(6)),
				TraceID:            gptr.Of("trace1"),
				LogID:              gptr.Of("log1"),
				Status:             evaluatordto.EvaluatorRunStatusPtr(evaluatordto.EvaluatorRunStatus_Success),
				Ext:                nil,
			},
		},
		{
			name: "Ext字段为nil",
			input: &evaluatordo.EvaluatorRecord{
				ID:                 1,
				ExperimentID:       2,
				ExperimentRunID:    3,
				ItemID:             4,
				TurnID:             5,
				EvaluatorVersionID: 6,
				TraceID:            "trace1",
				LogID:              "log1",
				Status:             evaluatordo.EvaluatorRunStatusSuccess,
				BaseInfo:           &evaluatordo.BaseInfo{},
				Ext:                nil,
			},
			expected: &evaluatordto.EvaluatorRecord{
				ID:                 gptr.Of(int64(1)),
				ExperimentID:       gptr.Of(int64(2)),
				ExperimentRunID:    gptr.Of(int64(3)),
				ItemID:             gptr.Of(int64(4)),
				TurnID:             gptr.Of(int64(5)),
				EvaluatorVersionID: gptr.Of(int64(6)),
				TraceID:            gptr.Of("trace1"),
				LogID:              gptr.Of("log1"),
				Status:             evaluatordto.EvaluatorRunStatusPtr(evaluatordto.EvaluatorRunStatus_Success),
				Ext:                nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := ConvertEvaluatorRecordDO2DTO(tt.input)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected.ID, result.ID)
				assert.Equal(t, tt.expected.ExperimentID, result.ExperimentID)
				assert.Equal(t, tt.expected.ExperimentRunID, result.ExperimentRunID)
				assert.Equal(t, tt.expected.ItemID, result.ItemID)
				assert.Equal(t, tt.expected.TurnID, result.TurnID)
				assert.Equal(t, tt.expected.EvaluatorVersionID, result.EvaluatorVersionID)
				assert.Equal(t, tt.expected.TraceID, result.TraceID)
				assert.Equal(t, tt.expected.LogID, result.LogID)
				assert.Equal(t, tt.expected.Status, result.Status)
				assert.Equal(t, tt.expected.Ext, result.Ext)
				// BaseInfo, EvaluatorInputData, EvaluatorOutputData 由转换函数处理，不在这里比较
			}
		})
	}
}
