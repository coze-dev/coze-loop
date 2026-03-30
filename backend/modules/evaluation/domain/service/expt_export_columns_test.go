// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"encoding/json"
	"testing"

	"github.com/bytedance/sonic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestExportSpecMeansExportAll(t *testing.T) {
	assert.True(t, exportSpecMeansExportAll(nil))
	assert.False(t, exportSpecMeansExportAll(&entity.ExptResultExportColumnSpec{}))
	assert.False(t, exportSpecMeansExportAll(&entity.ExptResultExportColumnSpec{
		EvalTargetOutputs: []string{},
	}))
}

// 导出列 spec 经 MQ（JSON）与 cloneExptExportColumnSpec 往返时，空切片必须仍为 []，不能因 omitempty 丢失后与 null 混用。
func TestExptResultExportColumnSpec_JSONRoundtripEmptySlices(t *testing.T) {
	in := &entity.ExptResultExportColumnSpec{
		EvalSetFields:         []string{},
		EvalTargetOutputs:     []string{"x"},
		Metrics:               []string{},
		EvaluatorVersionIds:   []string{},
	}
	b, err := json.Marshal(in)
	require.NoError(t, err)

	var outStd entity.ExptResultExportColumnSpec
	require.NoError(t, json.Unmarshal(b, &outStd))
	require.NotNil(t, outStd.EvalSetFields)
	assert.Empty(t, outStd.EvalSetFields)
	require.NotNil(t, outStd.EvalTargetOutputs)
	assert.Equal(t, []string{"x"}, outStd.EvalTargetOutputs)
	require.NotNil(t, outStd.Metrics)
	assert.Empty(t, outStd.Metrics)
	require.NotNil(t, outStd.EvaluatorVersionIds)
	assert.Empty(t, outStd.EvaluatorVersionIds)

	var outSonic entity.ExptResultExportColumnSpec
	require.NoError(t, sonic.Unmarshal(b, &outSonic))
	require.NotNil(t, outSonic.EvalSetFields)
	assert.Empty(t, outSonic.EvalSetFields)
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

func TestMgetParamForExportSpec_whitelistEmptyObjectNoFullTargetLoad(t *testing.T) {
	p := mgetParamForExportSpec(&entity.ExptResultExportColumnSpec{})
	require.NotNil(t, p.LoadEvalTargetFullContent)
	assert.False(t, *p.LoadEvalTargetFullContent)
	assert.False(t, p.FullTrajectory)
	assert.Empty(t, p.LoadEvalTargetOutputFieldKeys)
}

func TestMgetParamForExportSpec_metricsOnlyNoEvalTargetOutputs(t *testing.T) {
	p := mgetParamForExportSpec(&entity.ExptResultExportColumnSpec{
		Metrics: []string{consts.ReportColumnNameEvalTargetTotalLatency},
	})
	require.NotNil(t, p.LoadEvalTargetFullContent)
	assert.False(t, *p.LoadEvalTargetFullContent)
	assert.Empty(t, p.LoadEvalTargetOutputFieldKeys)
	assert.False(t, p.FullTrajectory)
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

// 报告中 Target 列与 OutputSchema 不一致时，仍应接受用户显式请求的列名（否则白名单无 target:*，只剩评测集列）。
func TestNewExportColumnSelectionFromSpec_targetNamesWithoutMatchingReportSchema(t *testing.T) {
	report := &entity.MGetExperimentReportResult{
		ExptColumnsEvalTarget: []*entity.ExptColumnEvalTarget{{
			ExptID:  99,
			Columns: []*entity.ColumnEvalTarget{{Name: "only_in_schema"}},
		}},
	}
	spec := &entity.ExptResultExportColumnSpec{
		EvalTargetOutputs: []string{consts.ReportColumnNameEvalTargetActualOutput},
		Metrics:           []string{consts.ReportColumnNameEvalTargetTotalLatency},
	}
	sel := newExportColumnSelectionFromSpec(spec, report, 99)
	_, ok := sel.keys[exportColPrefixTarget+consts.ReportColumnNameEvalTargetActualOutput]
	assert.True(t, ok)
	_, ok = sel.keys[exportColPrefixTarget+consts.ReportColumnNameEvalTargetTotalLatency]
	assert.True(t, ok)

	filtered := filterColumnsEvalTargetForExport(pickEvalTargetColsForExpt(report, 99), sel)
	assert.Empty(t, filtered)
	merged := ensureTargetColumnsForExportWhitelist(spec, filtered, sel)
	require.Len(t, merged, 2)
	assert.Equal(t, consts.ReportColumnNameEvalTargetActualOutput, merged[0].Name)
	assert.Equal(t, consts.ReportColumnNameEvalTargetTotalLatency, merged[1].Name)
}

// 导出 CSV 构建列元数据时必须 pickEvalTargetColsForExpt(exptID)，不能取 ExptColumnsEvalTarget[0]，否则白名单命中但 ColumnEvalTarget 列表来自错误实验，filter 后 Target 列全丢。
func TestPickEvalTargetColsForExpt_matchesExportColumnSelection(t *testing.T) {
	report := &entity.MGetExperimentReportResult{
		ExptColumnsEvalTarget: []*entity.ExptColumnEvalTarget{
			{
				ExptID:  1,
				Columns: []*entity.ColumnEvalTarget{{Name: "wrong_expt_only"}},
			},
			{
				ExptID: 2,
				Columns: []*entity.ColumnEvalTarget{
					{Name: consts.ReportColumnNameEvalTargetActualOutput},
					{Name: consts.ReportColumnNameEvalTargetTotalLatency},
				},
			},
		},
	}
	spec := &entity.ExptResultExportColumnSpec{
		EvalTargetOutputs:   []string{consts.ReportColumnNameEvalTargetActualOutput},
		Metrics:             []string{consts.ReportColumnNameEvalTargetTotalLatency},
		EvaluatorVersionIds: []string{},
	}
	sel := newExportColumnSelectionFromSpec(spec, report, 2)
	require.False(t, sel.exportAll)

	colsFromFirst := report.ExptColumnsEvalTarget[0].Columns
	colsFromPick := pickEvalTargetColsForExpt(report, 2)
	assert.Empty(t, filterColumnsEvalTargetForExport(colsFromFirst, sel))
	assert.Len(t, filterColumnsEvalTargetForExport(colsFromPick, sel), 2)
}

func TestNewExportColumnSelectionFromSpec_weightedScoreField(t *testing.T) {
	report := &entity.MGetExperimentReportResult{}
	weighted := true
	spec := &entity.ExptResultExportColumnSpec{
		WeightedScore: &weighted,
	}
	sel := newExportColumnSelectionFromSpec(spec, report, 1)
	require.False(t, sel.exportAll)
	_, ok := sel.keys[exportColKeyWeightedScore]
	assert.True(t, ok)
}

func TestAddEvaluatorVersionIDKeysForExport(t *testing.T) {
	keys := make(map[string]struct{})
	addEvaluatorVersionIDKeysForExport(keys, " 42 ")
	_, okS := keys[evaluatorColumnToken(42, "score")]
	_, okR := keys[evaluatorColumnToken(42, "reason")]
	assert.True(t, okS)
	assert.True(t, okR)

	addEvaluatorVersionIDKeysForExport(keys, "")
	assert.Len(t, keys, 2)

	keysInvalid := make(map[string]struct{})
	addEvaluatorVersionIDKeysForExport(keysInvalid, "not-a-number")
	assert.Empty(t, keysInvalid)

	// 加权分列由 weighted_score 字段控制，不再从 evaluator_version_ids 解析
	keysNoWeighted := make(map[string]struct{})
	addEvaluatorVersionIDKeysForExport(keysNoWeighted, "weighted_score")
	_, okW := keysNoWeighted[exportColKeyWeightedScore]
	assert.False(t, okW)
}
