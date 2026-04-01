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
		EvalSetFields:       []string{},
		EvalTargetOutputs:   []string{"x"},
		Metrics:             []string{},
		EvaluatorVersionIds: []string{},
		TagKeyIds:           []string{},
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
	require.NotNil(t, outStd.TagKeyIds)
	assert.Empty(t, outStd.TagKeyIds)

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
		EvalSetFields:       []string{},
		EvalTargetOutputs:   []string{},
		Metrics:             []string{consts.ReportColumnNameEvalTargetTotalLatency},
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

func TestDedupeStrings(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "nil input",
			in:   nil,
			want: []string{},
		},
		{
			name: "empty input",
			in:   []string{},
			want: []string{},
		},
		{
			name: "duplicates removed",
			in:   []string{"a", "b", "a", "c", "b"},
			want: []string{"a", "b", "c"},
		},
		{
			name: "whitespace trimming",
			in:   []string{" a ", "  b", "a"},
			want: []string{"a", "b"},
		},
		{
			name: "blank-only entries skipped",
			in:   []string{"", " ", "  ", "x", " "},
			want: []string{"x"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dedupeStrings(tt.in)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMgetParamForExportSpec_nilSpecFull(t *testing.T) {
	p := mgetParamForExportSpec(nil)
	require.NotNil(t, p.LoadEvalTargetFullContent)
	assert.True(t, *p.LoadEvalTargetFullContent)
	assert.True(t, p.FullTrajectory)
}

func TestMgetParamForExportSpec_trajectoryInOutputs(t *testing.T) {
	p := mgetParamForExportSpec(&entity.ExptResultExportColumnSpec{
		EvalTargetOutputs: []string{
			consts.ReportColumnNameEvalTargetTrajectory,
			consts.ReportColumnNameEvalTargetActualOutput,
		},
	})
	assert.True(t, p.FullTrajectory)
	assert.Contains(t, p.LoadEvalTargetOutputFieldKeys, consts.ReportColumnNameEvalTargetActualOutput)
	assert.Contains(t, p.LoadEvalTargetOutputFieldKeys, consts.ReportColumnNameEvalTargetTrajectory)
}

func TestMgetParamForExportSpec_overlappingOutputsAndMetrics(t *testing.T) {
	p := mgetParamForExportSpec(&entity.ExptResultExportColumnSpec{
		EvalTargetOutputs: []string{
			consts.ReportColumnNameEvalTargetActualOutput,
			consts.ReportColumnNameEvalTargetTotalLatency,
		},
		Metrics: []string{
			consts.ReportColumnNameEvalTargetTotalLatency,
			consts.ReportColumnNameEvalTargetInputTokens,
		},
	})
	assert.False(t, *p.LoadEvalTargetFullContent)
	assert.Contains(t, p.LoadEvalTargetOutputFieldKeys, consts.ReportColumnNameEvalTargetActualOutput)
	assert.NotContains(t, p.LoadEvalTargetOutputFieldKeys, consts.ReportColumnNameEvalTargetTotalLatency)
	assert.NotContains(t, p.LoadEvalTargetOutputFieldKeys, consts.ReportColumnNameEvalTargetInputTokens)
}

func TestMgetParamForExportSpec_onlyMetricsThatAreInExportColTargetMetricNames(t *testing.T) {
	p := mgetParamForExportSpec(&entity.ExptResultExportColumnSpec{
		Metrics: []string{
			consts.ReportColumnNameEvalTargetTotalLatency,
			consts.ReportColumnNameEvalTargetInputTokens,
			consts.ReportColumnNameEvalTargetOutputTokens,
			consts.ReportColumnNameEvalTargetTotalTokens,
		},
	})
	assert.False(t, *p.LoadEvalTargetFullContent)
	assert.Empty(t, p.LoadEvalTargetOutputFieldKeys)
	assert.False(t, p.FullTrajectory)
}

func TestNewExportColumnSelectionFromSpec_tagKeyIds(t *testing.T) {
	sel := newExportColumnSelectionFromSpec(&entity.ExptResultExportColumnSpec{
		TagKeyIds: []string{"10", " 20 "},
	}, &entity.MGetExperimentReportResult{}, 1)
	require.False(t, sel.exportAll)
	_, ok10 := sel.keys[exportColPrefixAnnotation+"10"]
	_, ok20 := sel.keys[exportColPrefixAnnotation+"20"]
	assert.True(t, ok10)
	assert.True(t, ok20)
}

func TestNewExportColumnSelectionFromSpec_withEvaluatorVersionIDs(t *testing.T) {
	sel := newExportColumnSelectionFromSpec(&entity.ExptResultExportColumnSpec{
		EvaluatorVersionIds: []string{"100", "200"},
	}, &entity.MGetExperimentReportResult{}, 1)
	require.False(t, sel.exportAll)
	_, okS1 := sel.keys[evaluatorColumnToken(100, "score")]
	_, okR1 := sel.keys[evaluatorColumnToken(100, "reason")]
	_, okS2 := sel.keys[evaluatorColumnToken(200, "score")]
	_, okR2 := sel.keys[evaluatorColumnToken(200, "reason")]
	assert.True(t, okS1)
	assert.True(t, okR1)
	assert.True(t, okS2)
	assert.True(t, okR2)
}

func TestNewExportColumnSelectionFromSpec_weightedScoreFalse(t *testing.T) {
	ws := false
	sel := newExportColumnSelectionFromSpec(&entity.ExptResultExportColumnSpec{
		WeightedScore: &ws,
	}, &entity.MGetExperimentReportResult{}, 1)
	_, ok := sel.keys[exportColKeyWeightedScore]
	assert.False(t, ok)
}

func TestNewExportColumnSelectionFromSpec_trajectoryWhenRequested(t *testing.T) {
	sel := newExportColumnSelectionFromSpec(&entity.ExptResultExportColumnSpec{
		EvalTargetOutputs: []string{
			consts.ReportColumnNameEvalTargetTrajectory,
			consts.ReportColumnNameEvalTargetActualOutput,
		},
	}, &entity.MGetExperimentReportResult{}, 1)
	_, okTraj := sel.keys[exportColPrefixTarget+consts.ReportColumnNameEvalTargetTrajectory]
	assert.True(t, okTraj)
	_, okActual := sel.keys[exportColPrefixTarget+consts.ReportColumnNameEvalTargetActualOutput]
	assert.True(t, okActual)
}

func TestNewExportColumnSelectionFromSpec_nilSpec(t *testing.T) {
	sel := newExportColumnSelectionFromSpec(nil, &entity.MGetExperimentReportResult{}, 1)
	assert.True(t, sel.exportAll)
}

func TestExportColumnSelection_mgetExperimentResultParam_nil(t *testing.T) {
	var sel *exportColumnSelection
	p := sel.mgetExperimentResultParam(10, 20)
	assert.True(t, *p.LoadEvalTargetFullContent)
	assert.True(t, p.FullTrajectory)
	assert.Equal(t, int64(10), p.SpaceID)
	assert.Equal(t, []int64{20}, p.ExptIDs)
}

func TestExportColumnSelection_mgetExperimentResultParam_exportAll(t *testing.T) {
	sel := &exportColumnSelection{exportAll: true}
	p := sel.mgetExperimentResultParam(10, 20)
	assert.True(t, *p.LoadEvalTargetFullContent)
	assert.True(t, p.FullTrajectory)
}

func TestExportColumnSelection_mgetExperimentResultParam_specificKeys(t *testing.T) {
	sel := &exportColumnSelection{
		exportAll: false,
		keys: map[string]struct{}{
			exportColPrefixTarget + consts.ReportColumnNameEvalTargetActualOutput: {},
			exportColPrefixTarget + consts.ReportColumnNameEvalTargetTrajectory:   {},
			exportColPrefixTarget + consts.ReportColumnNameEvalTargetTotalLatency: {},
			exportColPrefixTarget + consts.ReportColumnNameEvalTargetInputTokens:  {},
		},
	}
	p := sel.mgetExperimentResultParam(5, 15)
	assert.False(t, *p.LoadEvalTargetFullContent)
	assert.True(t, p.FullTrajectory)
	assert.Contains(t, p.LoadEvalTargetOutputFieldKeys, consts.ReportColumnNameEvalTargetActualOutput)
	assert.Contains(t, p.LoadEvalTargetOutputFieldKeys, consts.ReportColumnNameEvalTargetTrajectory)
	assert.NotContains(t, p.LoadEvalTargetOutputFieldKeys, consts.ReportColumnNameEvalTargetTotalLatency)
	assert.NotContains(t, p.LoadEvalTargetOutputFieldKeys, consts.ReportColumnNameEvalTargetInputTokens)
}

func TestExportColumnSelection_hasTargetColumn(t *testing.T) {
	tests := []struct {
		name string
		sel  *exportColumnSelection
		col  string
		want bool
	}{
		{
			name: "nil selection",
			sel:  nil,
			col:  "any",
			want: true,
		},
		{
			name: "exportAll",
			sel:  &exportColumnSelection{exportAll: true},
			col:  "any",
			want: true,
		},
		{
			name: "found",
			sel: &exportColumnSelection{keys: map[string]struct{}{
				exportColPrefixTarget + "col_a": {},
			}},
			col:  "col_a",
			want: true,
		},
		{
			name: "not found",
			sel: &exportColumnSelection{keys: map[string]struct{}{
				exportColPrefixTarget + "col_a": {},
			}},
			col:  "col_b",
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.sel.hasTargetColumn(tt.col))
		})
	}
}

func TestExportColumnSelection_evalTargetOutputFieldKeysForLoad(t *testing.T) {
	tests := []struct {
		name    string
		sel     *exportColumnSelection
		wantNil bool
		want    []string
	}{
		{
			name:    "nil selection",
			sel:     nil,
			wantNil: true,
		},
		{
			name:    "exportAll",
			sel:     &exportColumnSelection{exportAll: true},
			wantNil: true,
		},
		{
			name: "mixed keys with metrics and targets",
			sel: &exportColumnSelection{keys: map[string]struct{}{
				exportColPrefixTarget + consts.ReportColumnNameEvalTargetActualOutput: {},
				exportColPrefixTarget + consts.ReportColumnNameEvalTargetTotalLatency: {},
				exportColPrefixEvalSet + "field1":                                     {},
				exportColKeyWeightedScore:                                             {},
			}},
			wantNil: false,
			want:    []string{consts.ReportColumnNameEvalTargetActualOutput},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.sel.evalTargetOutputFieldKeysForLoad()
			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestExportColumnSelection_includeEvalSetFieldKey(t *testing.T) {
	tests := []struct {
		name string
		sel  *exportColumnSelection
		key  string
		want bool
	}{
		{name: "nil", sel: nil, key: "any", want: true},
		{name: "exportAll", sel: &exportColumnSelection{exportAll: true}, key: "any", want: true},
		{
			name: "found",
			sel: &exportColumnSelection{keys: map[string]struct{}{
				exportColPrefixEvalSet + "input": {},
			}},
			key: "input", want: true,
		},
		{
			name: "not found",
			sel: &exportColumnSelection{keys: map[string]struct{}{
				exportColPrefixEvalSet + "input": {},
			}},
			key: "other", want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.sel.includeEvalSetFieldKey(tt.key))
		})
	}
}

func TestExportColumnSelection_includeTargetColumnName(t *testing.T) {
	tests := []struct {
		name string
		sel  *exportColumnSelection
		col  string
		want bool
	}{
		{name: "nil", sel: nil, col: "any", want: true},
		{name: "exportAll", sel: &exportColumnSelection{exportAll: true}, col: "any", want: true},
		{
			name: "found",
			sel: &exportColumnSelection{keys: map[string]struct{}{
				exportColPrefixTarget + "actual_output": {},
			}},
			col: "actual_output", want: true,
		},
		{
			name: "not found",
			sel: &exportColumnSelection{keys: map[string]struct{}{
				exportColPrefixTarget + "actual_output": {},
			}},
			col: "trajectory", want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.sel.includeTargetColumnName(tt.col))
		})
	}
}

func TestExportColumnSelection_includeEvaluatorScoreReason(t *testing.T) {
	sel42 := &exportColumnSelection{keys: map[string]struct{}{
		evaluatorColumnToken(42, "score"):  {},
		evaluatorColumnToken(42, "reason"): {},
	}}
	tests := []struct {
		name      string
		sel       *exportColumnSelection
		vid       int64
		wantScore bool
		wantRsn   bool
	}{
		{name: "nil", sel: nil, vid: 1, wantScore: true, wantRsn: true},
		{name: "exportAll", sel: &exportColumnSelection{exportAll: true}, vid: 1, wantScore: true, wantRsn: true},
		{name: "found", sel: sel42, vid: 42, wantScore: true, wantRsn: true},
		{name: "not found", sel: sel42, vid: 99, wantScore: false, wantRsn: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantScore, tt.sel.includeEvaluatorScore(tt.vid))
			assert.Equal(t, tt.wantRsn, tt.sel.includeEvaluatorReason(tt.vid))
		})
	}
}

func TestExportColumnSelection_includeWeightedScore(t *testing.T) {
	tests := []struct {
		name string
		sel  *exportColumnSelection
		want bool
	}{
		{name: "nil", sel: nil, want: true},
		{name: "exportAll", sel: &exportColumnSelection{exportAll: true}, want: true},
		{
			name: "true",
			sel: &exportColumnSelection{keys: map[string]struct{}{
				exportColKeyWeightedScore: {},
			}},
			want: true,
		},
		{
			name: "false",
			sel:  &exportColumnSelection{keys: map[string]struct{}{}},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.sel.includeWeightedScore())
		})
	}
}

func TestExportColumnSelection_includeAnnotationTag(t *testing.T) {
	tests := []struct {
		name string
		sel  *exportColumnSelection
		id   int64
		want bool
	}{
		{name: "nil", sel: nil, id: 1, want: true},
		{name: "exportAll", sel: &exportColumnSelection{exportAll: true}, id: 1, want: true},
		{
			name: "found",
			sel: &exportColumnSelection{keys: map[string]struct{}{
				exportColPrefixAnnotation + "100": {},
			}},
			id: 100, want: true,
		},
		{
			name: "not found",
			sel: &exportColumnSelection{keys: map[string]struct{}{
				exportColPrefixAnnotation + "100": {},
			}},
			id: 200, want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.sel.includeAnnotationTag(tt.id))
		})
	}
}

func TestEvaluatorColumnToken(t *testing.T) {
	assert.Equal(t, "evaluator:42:score", evaluatorColumnToken(42, "score"))
	assert.Equal(t, "evaluator:42:reason", evaluatorColumnToken(42, "reason"))
	assert.Equal(t, "evaluator:0:score", evaluatorColumnToken(0, "score"))
}

func TestFilterColumnEvalSetFieldsForExport(t *testing.T) {
	k1 := "input"
	k2 := "context"
	fields := []*entity.ColumnEvalSetField{
		{Key: &k1},
		{Key: &k2},
		nil,
		{Key: nil},
	}
	tests := []struct {
		name string
		sel  *exportColumnSelection
		want int
	}{
		{
			name: "nil sel returns all",
			sel:  nil,
			want: 4,
		},
		{
			name: "exportAll returns all",
			sel:  &exportColumnSelection{exportAll: true},
			want: 4,
		},
		{
			name: "whitelist filters",
			sel: &exportColumnSelection{keys: map[string]struct{}{
				exportColPrefixEvalSet + "input": {},
			}},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterColumnEvalSetFieldsForExport(fields, tt.sel)
			assert.Len(t, got, tt.want)
		})
	}
}

func TestFilterColumnEvalSetFieldsForExport_nilFields(t *testing.T) {
	sel := &exportColumnSelection{keys: map[string]struct{}{
		exportColPrefixEvalSet + "input": {},
	}}
	got := filterColumnEvalSetFieldsForExport(nil, sel)
	assert.Empty(t, got)
}

func TestFilterColumnsEvalTargetForExport(t *testing.T) {
	cols := []*entity.ColumnEvalTarget{
		{Name: "col_a"},
		{Name: "col_b"},
		nil,
	}
	tests := []struct {
		name string
		sel  *exportColumnSelection
		want int
	}{
		{
			name: "nil sel returns all",
			sel:  nil,
			want: 3,
		},
		{
			name: "exportAll returns all",
			sel:  &exportColumnSelection{exportAll: true},
			want: 3,
		},
		{
			name: "whitelist filters and skips nil",
			sel: &exportColumnSelection{keys: map[string]struct{}{
				exportColPrefixTarget + "col_a": {},
			}},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterColumnsEvalTargetForExport(cols, tt.sel)
			assert.Len(t, got, tt.want)
		})
	}
}

func TestFilterColumnsEvalTargetForExport_nilCols(t *testing.T) {
	sel := &exportColumnSelection{keys: map[string]struct{}{
		exportColPrefixTarget + "col_a": {},
	}}
	got := filterColumnsEvalTargetForExport(nil, sel)
	assert.Empty(t, got)
}

func TestEnsureTargetColumnsForExportWhitelist(t *testing.T) {
	tests := []struct {
		name     string
		spec     *entity.ExptResultExportColumnSpec
		filtered []*entity.ColumnEvalTarget
		sel      *exportColumnSelection
		wantLen  int
	}{
		{
			name:     "exportAll returns filtered as-is",
			spec:     &entity.ExptResultExportColumnSpec{},
			filtered: []*entity.ColumnEvalTarget{{Name: "a"}},
			sel:      &exportColumnSelection{exportAll: true},
			wantLen:  1,
		},
		{
			name:     "nil spec returns filtered as-is",
			spec:     nil,
			filtered: []*entity.ColumnEvalTarget{{Name: "a"}},
			sel:      &exportColumnSelection{keys: map[string]struct{}{}},
			wantLen:  1,
		},
		{
			name: "missing target names added",
			spec: &entity.ExptResultExportColumnSpec{
				EvalTargetOutputs: []string{consts.ReportColumnNameEvalTargetActualOutput, "custom_col"},
				Metrics:           []string{consts.ReportColumnNameEvalTargetTotalLatency},
			},
			filtered: []*entity.ColumnEvalTarget{
				{Name: consts.ReportColumnNameEvalTargetActualOutput},
			},
			sel: &exportColumnSelection{keys: map[string]struct{}{
				exportColPrefixTarget + consts.ReportColumnNameEvalTargetActualOutput: {},
				exportColPrefixTarget + "custom_col":                                  {},
				exportColPrefixTarget + consts.ReportColumnNameEvalTargetTotalLatency: {},
			}},
			wantLen: 3,
		},
		{
			name: "duplicate names not doubled",
			spec: &entity.ExptResultExportColumnSpec{
				EvalTargetOutputs: []string{"dup", "dup"},
				Metrics:           []string{"dup"},
			},
			filtered: []*entity.ColumnEvalTarget{},
			sel: &exportColumnSelection{keys: map[string]struct{}{
				exportColPrefixTarget + "dup": {},
			}},
			wantLen: 1,
		},
		{
			name: "trajectory in outputs is included when whitelisted",
			spec: &entity.ExptResultExportColumnSpec{
				EvalTargetOutputs: []string{consts.ReportColumnNameEvalTargetTrajectory, "real"},
			},
			filtered: []*entity.ColumnEvalTarget{},
			sel: &exportColumnSelection{keys: map[string]struct{}{
				exportColPrefixTarget + consts.ReportColumnNameEvalTargetTrajectory: {},
				exportColPrefixTarget + "real":                                      {},
			}},
			wantLen: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ensureTargetColumnsForExportWhitelist(tt.spec, tt.filtered, tt.sel)
			assert.Len(t, got, tt.wantLen)
		})
	}
}

func TestFilterColumnEvaluatorsForExport(t *testing.T) {
	evs := []*entity.ColumnEvaluator{
		{EvaluatorVersionID: 1},
		{EvaluatorVersionID: 2},
		nil,
	}
	tests := []struct {
		name string
		sel  *exportColumnSelection
		want int
	}{
		{name: "nil sel", sel: nil, want: 3},
		{name: "exportAll", sel: &exportColumnSelection{exportAll: true}, want: 3},
		{
			name: "mixed evaluators",
			sel: &exportColumnSelection{keys: map[string]struct{}{
				evaluatorColumnToken(1, "score"): {},
			}},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterColumnEvaluatorsForExport(evs, tt.sel)
			assert.Len(t, got, tt.want)
		})
	}
}

func TestFilterColumnEvaluatorsForExport_nilEvaluators(t *testing.T) {
	sel := &exportColumnSelection{keys: map[string]struct{}{
		evaluatorColumnToken(1, "score"): {},
	}}
	got := filterColumnEvaluatorsForExport(nil, sel)
	assert.Empty(t, got)
}

func TestFilterColumnAnnotationsForExport(t *testing.T) {
	ann := []*entity.ColumnAnnotation{
		{TagKeyID: 10},
		{TagKeyID: 20},
		nil,
	}
	tests := []struct {
		name string
		sel  *exportColumnSelection
		want int
	}{
		{name: "nil sel", sel: nil, want: 3},
		{name: "exportAll", sel: &exportColumnSelection{exportAll: true}, want: 3},
		{
			name: "mixed annotations",
			sel: &exportColumnSelection{keys: map[string]struct{}{
				exportColPrefixAnnotation + "10": {},
			}},
			want: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterColumnAnnotationsForExport(ann, tt.sel)
			assert.Len(t, got, tt.want)
		})
	}
}

func TestPickEvalTargetColsForExpt(t *testing.T) {
	tests := []struct {
		name    string
		report  *entity.MGetExperimentReportResult
		exptID  int64
		wantNil bool
		wantLen int
	}{
		{
			name:    "nil report",
			report:  nil,
			exptID:  1,
			wantNil: true,
		},
		{
			name: "matching exptID returns that experiment columns",
			report: &entity.MGetExperimentReportResult{
				ExptColumnsEvalTarget: []*entity.ExptColumnEvalTarget{
					{ExptID: 10, Columns: []*entity.ColumnEvalTarget{{Name: "wrong"}}},
					{ExptID: 20, Columns: []*entity.ColumnEvalTarget{{Name: "hit_a"}, {Name: "hit_b"}}},
				},
			},
			exptID:  20,
			wantNil: false,
			wantLen: 2,
		},
		{
			name: "no matching exptID falls back to first",
			report: &entity.MGetExperimentReportResult{
				ExptColumnsEvalTarget: []*entity.ExptColumnEvalTarget{
					{ExptID: 10, Columns: []*entity.ColumnEvalTarget{{Name: "fallback"}}},
					{ExptID: 20, Columns: []*entity.ColumnEvalTarget{{Name: "other"}}},
				},
			},
			exptID:  999,
			wantNil: false,
			wantLen: 1,
		},
		{
			name: "empty ExptColumnsEvalTarget",
			report: &entity.MGetExperimentReportResult{
				ExptColumnsEvalTarget: []*entity.ExptColumnEvalTarget{},
			},
			exptID:  1,
			wantNil: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pickEvalTargetColsForExpt(tt.report, tt.exptID)
			if tt.wantNil {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Len(t, got, tt.wantLen)
			}
		})
	}
}

func TestNewExportColumnSelectionFromSpec_evalSetFieldsOnly(t *testing.T) {
	sel := newExportColumnSelectionFromSpec(&entity.ExptResultExportColumnSpec{
		EvalSetFields: []string{"col_a", "col_b"},
	}, &entity.MGetExperimentReportResult{}, 1)
	require.False(t, sel.exportAll)
	_, okA := sel.keys[exportColPrefixEvalSet+"col_a"]
	_, okB := sel.keys[exportColPrefixEvalSet+"col_b"]
	assert.True(t, okA)
	assert.True(t, okB)
}

func TestExportColumnSelection_evalTargetOutputFieldKeysForLoad_skipsEmptyNameAndDedupes(t *testing.T) {
	sel := &exportColumnSelection{
		exportAll: false,
		keys: map[string]struct{}{
			exportColPrefixTarget + consts.ReportColumnNameEvalTargetActualOutput: {},
			exportColPrefixTarget + "": {}, // 键为 "target:"，TrimPrefix 后 name 为空，应跳过
		},
	}
	got := sel.evalTargetOutputFieldKeysForLoad()
	assert.Equal(t, []string{consts.ReportColumnNameEvalTargetActualOutput}, got)
}

func TestEnsureTargetColumnsForExportWhitelist_nilSelection(t *testing.T) {
	filtered := []*entity.ColumnEvalTarget{{Name: "keep"}}
	spec := &entity.ExptResultExportColumnSpec{EvalTargetOutputs: []string{"extra"}}
	got := ensureTargetColumnsForExportWhitelist(spec, filtered, nil)
	assert.Equal(t, filtered, got)
}

func TestFilterColumnEvaluatorsForExport_reasonOnly(t *testing.T) {
	evs := []*entity.ColumnEvaluator{
		{EvaluatorVersionID: 1},
		{EvaluatorVersionID: 2},
	}
	sel := &exportColumnSelection{
		exportAll: false,
		keys: map[string]struct{}{
			evaluatorColumnToken(2, "reason"): {},
		},
	}
	got := filterColumnEvaluatorsForExport(evs, sel)
	require.Len(t, got, 1)
	assert.Equal(t, int64(2), got[0].EvaluatorVersionID)
}
