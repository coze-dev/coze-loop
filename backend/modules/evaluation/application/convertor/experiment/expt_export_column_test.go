// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
)

func TestExportColumnSpecThrift2Entity(t *testing.T) {
	tests := []struct {
		name string
		from *expt.ExptResultExportColumnSpec
		run  func(t *testing.T, from *expt.ExptResultExportColumnSpec)
	}{
		{
			name: "nil input returns nil",
			from: nil,
			run: func(t *testing.T, from *expt.ExptResultExportColumnSpec) {
				got := ExportColumnSpecThrift2Entity(from)
				assert.Nil(t, got)
			},
		},
		{
			name: "empty struct all fields nil",
			from: &expt.ExptResultExportColumnSpec{},
			run: func(t *testing.T, from *expt.ExptResultExportColumnSpec) {
				got := ExportColumnSpecThrift2Entity(from)
				assert.NotNil(t, got)
				assert.Nil(t, got.EvalSetFields)
				assert.Nil(t, got.EvalTargetOutputs)
				assert.Nil(t, got.Metrics)
				assert.Nil(t, got.EvaluatorVersionIds)
				assert.Nil(t, got.WeightedScore)
			},
		},
		{
			name: "all fields set",
			from: &expt.ExptResultExportColumnSpec{
				EvalSetFields:       []string{"f1", "f2"},
				EvalTargetOutputs:   []string{"o1", "o2"},
				Metrics:             []string{"m1"},
				EvaluatorVersionIds: []string{"100", "200"},
				WeightedScore:       gptr.Of(true),
			},
			run: func(t *testing.T, from *expt.ExptResultExportColumnSpec) {
				got := ExportColumnSpecThrift2Entity(from)
				assert.NotNil(t, got)
				assert.Equal(t, []string{"f1", "f2"}, got.EvalSetFields)
				assert.Equal(t, []string{"o1", "o2"}, got.EvalTargetOutputs)
				assert.Equal(t, []string{"m1"}, got.Metrics)
				assert.Equal(t, []string{"100", "200"}, got.EvaluatorVersionIds)
				assert.NotNil(t, got.WeightedScore)
				assert.True(t, *got.WeightedScore)
			},
		},
		{
			name: "only some fields set",
			from: &expt.ExptResultExportColumnSpec{
				EvalSetFields: []string{"x"},
				WeightedScore: gptr.Of(false),
			},
			run: func(t *testing.T, from *expt.ExptResultExportColumnSpec) {
				got := ExportColumnSpecThrift2Entity(from)
				assert.NotNil(t, got)
				assert.Equal(t, []string{"x"}, got.EvalSetFields)
				assert.Nil(t, got.EvalTargetOutputs)
				assert.Nil(t, got.Metrics)
				assert.Nil(t, got.EvaluatorVersionIds)
				assert.NotNil(t, got.WeightedScore)
				assert.False(t, *got.WeightedScore)
			},
		},
		{
			name: "weighted score true",
			from: &expt.ExptResultExportColumnSpec{
				WeightedScore: gptr.Of(true),
			},
			run: func(t *testing.T, from *expt.ExptResultExportColumnSpec) {
				got := ExportColumnSpecThrift2Entity(from)
				assert.NotNil(t, got.WeightedScore)
				assert.True(t, *got.WeightedScore)
			},
		},
		{
			name: "weighted score false",
			from: &expt.ExptResultExportColumnSpec{
				WeightedScore: gptr.Of(false),
			},
			run: func(t *testing.T, from *expt.ExptResultExportColumnSpec) {
				got := ExportColumnSpecThrift2Entity(from)
				assert.NotNil(t, got.WeightedScore)
				assert.False(t, *got.WeightedScore)
			},
		},
		{
			name: "deep copy slices not shared",
			from: &expt.ExptResultExportColumnSpec{
				EvalSetFields:       []string{"a", "b"},
				EvalTargetOutputs:   []string{"c"},
				Metrics:             []string{"d"},
				EvaluatorVersionIds: []string{"1"},
				WeightedScore:       gptr.Of(true),
			},
			run: func(t *testing.T, from *expt.ExptResultExportColumnSpec) {
				got := ExportColumnSpecThrift2Entity(from)

				from.EvalSetFields[0] = "CHANGED"
				from.EvalTargetOutputs[0] = "CHANGED"
				from.Metrics[0] = "CHANGED"
				from.EvaluatorVersionIds[0] = "CHANGED"
				*from.WeightedScore = false

				assert.Equal(t, "a", got.EvalSetFields[0])
				assert.Equal(t, "c", got.EvalTargetOutputs[0])
				assert.Equal(t, "d", got.Metrics[0])
				assert.Equal(t, "1", got.EvaluatorVersionIds[0])
				assert.True(t, *got.WeightedScore)
			},
		},
		{
			name: "empty slices become nil via append",
			from: &expt.ExptResultExportColumnSpec{
				EvalSetFields:       []string{},
				EvalTargetOutputs:   []string{},
				Metrics:             []string{},
				EvaluatorVersionIds: []string{},
			},
			run: func(t *testing.T, from *expt.ExptResultExportColumnSpec) {
				got := ExportColumnSpecThrift2Entity(from)
				assert.NotNil(t, got)
				assert.Nil(t, got.EvalSetFields)
				assert.Nil(t, got.EvalTargetOutputs)
				assert.Nil(t, got.Metrics)
				assert.Nil(t, got.EvaluatorVersionIds)
				assert.Nil(t, got.WeightedScore)
			},
		},
		{
			name: "eval set fields empty list only",
			from: &expt.ExptResultExportColumnSpec{
				EvalSetFields: []string{},
			},
			run: func(t *testing.T, from *expt.ExptResultExportColumnSpec) {
				got := ExportColumnSpecThrift2Entity(from)
				assert.NotNil(t, got)
				assert.Nil(t, got.EvalSetFields)
				assert.Nil(t, got.EvalTargetOutputs)
				assert.Nil(t, got.Metrics)
				assert.Nil(t, got.EvaluatorVersionIds)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t, tt.from)
		})
	}
}
