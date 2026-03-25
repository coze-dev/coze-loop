// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestExportSpecMeansExportAll(t *testing.T) {
	assert.True(t, exportSpecMeansExportAll(nil))
	assert.True(t, exportSpecMeansExportAll(&entity.ExptResultExportColumnSpec{}))
	assert.False(t, exportSpecMeansExportAll(&entity.ExptResultExportColumnSpec{
		EvalTargetOutputs: []string{},
	}))
}

func TestMgetParamForExportSpec_evalTargetExplicit(t *testing.T) {
	p := mgetParamForExportSpec(&entity.ExptResultExportColumnSpec{
		EvalTargetOutputs: []string{
			consts.ReportColumnNameEvalTargetTotalLatency,
			consts.ReportColumnNameEvalTargetActualOutput,
		},
	})
	require.NotNil(t, p.LoadEvalTargetFullContent)
	assert.False(t, *p.LoadEvalTargetFullContent)
	assert.Contains(t, p.LoadEvalTargetOutputFieldKeys, consts.ReportColumnNameEvalTargetActualOutput)
	assert.NotContains(t, p.LoadEvalTargetOutputFieldKeys, consts.ReportColumnNameEvalTargetTotalLatency)
}

func TestNewExportColumnSelectionFromSpec_evalTargetWhitelist(t *testing.T) {
	report := &entity.MGetExperimentReportResult{
		ColumnEvalSetFields: []*entity.ColumnEvalSetField{},
		ExptColumnsEvalTarget: []*entity.ExptColumnEvalTarget{{
			ExptID: 10,
			Columns: []*entity.ColumnEvalTarget{
				{Name: consts.ReportColumnNameEvalTargetTotalLatency},
				{Name: consts.ReportColumnNameEvalTargetInputTokens},
			},
		}},
		ColumnEvaluators: []*entity.ColumnEvaluator{},
	}
	spec := &entity.ExptResultExportColumnSpec{
		EvalSetFields:        []string{},
		EvalTargetOutputs:    []string{},
		Metrics: []string{consts.ReportColumnNameEvalTargetTotalLatency},
		EvaluatorVersionIds: []string{},
	}
	sel := newExportColumnSelectionFromSpec(spec, report, 10)
	require.False(t, sel.exportAll)
	_, ok := sel.keys[exportColPrefixTarget+consts.ReportColumnNameEvalTargetTotalLatency]
	assert.True(t, ok)
	_, ok = sel.keys[exportColPrefixTarget+consts.ReportColumnNameEvalTargetInputTokens]
	assert.False(t, ok)
}

func TestAddEvaluatorOutputToken(t *testing.T) {
	keys := make(map[string]struct{})
	addEvaluatorOutputToken(keys, "weighted_score")
	_, ok := keys[exportColKeyWeightedScore]
	assert.True(t, ok)

	keys2 := make(map[string]struct{})
	addEvaluatorOutputToken(keys2, "42:score")
	_, ok = keys2[evaluatorColumnToken(42, "score")]
	assert.True(t, ok)

	keys3 := make(map[string]struct{})
	addEvaluatorOutputToken(keys3, "evaluator:99:reason")
	_, ok = keys3[evaluatorColumnToken(99, "reason")]
	assert.True(t, ok)
}
