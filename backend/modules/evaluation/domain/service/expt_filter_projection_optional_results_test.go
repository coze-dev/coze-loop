// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/stretchr/testify/require"
)

func TestBuildExptFilterProjectionOptionalResultsKeepItem(t *testing.T) {
	tests := []struct {
		name      string
		change    func(*projectionFixture)
		data      map[string]string
		metrics   map[string]int64
		scores    map[string]float64
		weight    float64
		hasWeight bool
	}{
		{
			name: "null output fields retain their keys",
			change: func(f *projectionFixture) {
				text := "answer"
				f.targets[0].EvalTargetOutputData.OutputFields = map[string]*entity.Content{"actual_output": {Text: &text}, "trajectory": nil, "run_error": nil}
			},
			data:    map[string]string{"actual_output": "answer", "trajectory": "", "run_error": ""},
			metrics: map[string]int64{"input_tokens": 0, "output_tokens": 0, "total_tokens": 0, "total_latency": 1501},
			scores:  map[string]float64{"key11": 0, "key12": 1}, weight: 0.5, hasWeight: true,
		},
		{
			name: "missing target retains item and scores", change: func(f *projectionFixture) { f.targets = nil },
			data: map[string]string{}, metrics: map[string]int64{}, scores: map[string]float64{"key11": 0, "key12": 1}, weight: 0.5, hasWeight: true,
		},
		{
			name: "target without output leaves data absent", change: func(f *projectionFixture) { f.targets[0].EvalTargetOutputData = nil },
			data: map[string]string{}, metrics: map[string]int64{}, scores: map[string]float64{"key11": 0, "key12": 1}, weight: 0.5, hasWeight: true,
		},
		{
			name: "missing score retains present zero", change: func(f *projectionFixture) { f.evaluators = f.evaluators[:1] },
			data:    map[string]string{"actual_output": "", "custom": "preview"},
			metrics: map[string]int64{"input_tokens": 0, "output_tokens": 0, "total_tokens": 0, "total_latency": 1501},
			scores:  map[string]float64{"key11": 0}, hasWeight: true,
		},
		{
			name: "all scores missing keeps unscored item", change: func(f *projectionFixture) { f.evaluators = nil },
			data:    map[string]string{"actual_output": "", "custom": "preview"},
			metrics: map[string]int64{"input_tokens": 0, "output_tokens": 0, "total_tokens": 0, "total_latency": 1501}, scores: map[string]float64{},
		},
		{
			name: "deleted reused score contributes no score or correction",
			change: func(f *projectionFixture) {
				f.evaluators[0].ExperimentRunID = 99
				markProjectionScoreDeleted(f.evaluators[0], 1000)
				value := 7.0
				f.evaluators[0].EvaluatorOutputData.EvaluatorResult.Correction = &entity.Correction{Score: &value}
			},
			data:    map[string]string{"actual_output": "", "custom": "preview"},
			metrics: map[string]int64{"input_tokens": 0, "output_tokens": 0, "total_tokens": 0, "total_latency": 1501},
			scores:  map[string]float64{"key12": 1}, weight: 1, hasWeight: true,
		},
		{
			name: "missing results preserve persisted weighted score",
			change: func(f *projectionFixture) {
				f.targets, f.evaluators = nil, nil
				value := 0.625
				f.turns[0].WeightedScore = &value
			},
			data: map[string]string{}, metrics: map[string]int64{}, scores: map[string]float64{}, weight: 0.625, hasWeight: true,
		},
	}
	for _, sourceKind := range []string{"service", "repo"} {
		for _, tt := range tests {
			t.Run(sourceKind+"/"+tt.name, func(t *testing.T) {
				f := newProjectionFixture(t)
				f.items[0].ItemIdx, f.items[0].Status = 17, entity.ItemRunState(3)
				tt.change(f)
				before := projectionSourceDigest(t, f)
				s := f.service()
				if sourceKind == "repo" {
					s.evalTargetRepo = projectionSourceTargetRepo{f: f}
					s.filterEvaluatorRepo = projectionSourceEvaluatorRepo{f: f}
				}
				rows, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
				require.NoError(t, err)
				require.Len(t, rows, 1)
				row := rows[0]
				require.Equal(t, int64(3), row.ItemID)
				require.Equal(t, int32(17), row.ItemIdx)
				require.Equal(t, entity.ItemRunState(3), row.Status)
				require.Equal(t, int64(32), row.ItemVersionID)
				require.Equal(t, int64(30), row.EvalSetID)
				require.Equal(t, int64(31), row.EvalSetVersionID)
				require.Equal(t, tt.data, row.EvalTargetData)
				require.Equal(t, tt.metrics, row.EvalTargetMetrics)
				require.Equal(t, tt.scores, row.EvaluatorScore)
				require.False(t, row.EvaluatorScoreCorrected)
				require.Equal(t, map[string]float64{"key1": 0}, row.AnnotationFloat)
				require.Equal(t, map[string]string{"81": "", "82": "83"}, row.AnnotationString)
				if tt.hasWeight {
					require.NotNil(t, row.EvaluatorWeightedScore)
					require.Equal(t, tt.weight, *row.EvaluatorWeightedScore)
				} else {
					require.Nil(t, row.EvaluatorWeightedScore)
				}
				after := projectionSourceDigest(t, f)
				require.Equal(t, before, after)
			})
		}
	}
}

func TestProjectionOrdinaryBuilderOptionalResultSemantics(t *testing.T) {
	for _, mode := range []string{"missing results", "null output fields", "deleted reused score"} {
		t.Run(mode, func(t *testing.T) {
			f := newProjectionFixture(t)
			f.items[0].ItemIdx, f.items[0].Status = 17, entity.ItemRunState(3)
			var records []*entity.EvaluatorRecord
			wantData := map[string]string{}
			switch mode {
			case "null output fields":
				text := "answer"
				f.targets[0].EvalTargetOutputData.OutputFields = map[string]*entity.Content{"actual_output": {Text: &text}, "trajectory": nil, "run_error": nil}
				wantData = map[string]string{"actual_output": "answer", "trajectory": "", "run_error": ""}
			case "deleted reused score":
				f.evaluators[0].ExperimentRunID = 99
				markProjectionScoreDeleted(f.evaluators[0], 1000)
				records = f.evaluators[:1]
				f.targets = nil
			default:
				f.targets = nil
			}
			reader := &ExptResultBuilder{
				SpaceID: 1, exptDO: f.expt, turnResultDO: f.turns,
				ExptTurnResultRepo: projectionTurnRepo{f: f}, evalTargetService: projectionTargetService{f: f},
				evaluatorRecordService: deletedScoreProjectionService{source: &deletedScoreProjectionRepo{t: t, records: records}},
			}
			ctx := contexts.WithCtxWriteDB(context.Background())
			require.NoError(t, reader.buildTargetOutput(ctx))
			require.NoError(t, reader.buildEvaluatorResult(ctx))
			if f.targets == nil {
				stub := reader.turnResultID2TargetOutput[10].EvalTargetRecord
				require.Equal(t, entity.EvalTargetRunStatusAsyncInvoking, *stub.Status)
				require.Nil(t, stub.EvalTargetOutputData)
			}
			builder := &PayloadBuilder{SpaceID: 1, BaselineExptID: 2, BaseExptTurnResultDO: f.turns, BaseExptItemResultDO: f.items, ExptResultBuilders: []*ExptResultBuilder{reader}}
			require.NoError(t, builder.fillExptTurnResultFilters(ctx, f.expt.StartAt, 20, 21, false))
			require.Len(t, builder.ExptTurnResultFilters, 1)
			row := builder.ExptTurnResultFilters[0]
			require.Equal(t, int32(17), row.ItemIdx)
			require.Equal(t, entity.ItemRunState(3), row.Status)
			require.Equal(t, wantData, row.EvalTargetData)
			require.Empty(t, row.EvaluatorScore)
			require.False(t, row.EvaluatorScoreCorrected)
			require.Nil(t, row.EvaluatorWeightedScore)
			if f.targets == nil {
				require.Empty(t, row.EvalTargetMetrics)
			}
		})
	}
}

func TestBuildExptFilterProjectionOptionalResultsKeepRequiredGuards(t *testing.T) {
	cases := map[string]func(*projectionFixture){
		"missing target with read error":        func(f *projectionFixture) { f.targets, f.fail = nil, "targets" },
		"missing scores with read error":        func(f *projectionFixture) { f.evaluators, f.fail = nil, "evaluators" },
		"missing scores with missing item":      func(f *projectionFixture) { f.evaluators, f.items = nil, nil },
		"missing results with missing item ref": func(f *projectionFixture) { f.targets, f.evaluators, f.refs = nil, nil, nil },
		"missing scores with foreign reference": func(f *projectionFixture) { f.evaluators = nil; f.evaluatorRefs[0].SpaceID = 99 },
		"missing scores with duplicate reference": func(f *projectionFixture) {
			f.evaluators = nil
			f.evaluatorRefs = append(f.evaluatorRefs, f.evaluatorRefs[0])
		},
		"empty output with wrong item version": func(f *projectionFixture) { f.targets[0].EvalTargetOutputData = nil; f.targets[0].ItemVersionID = 99 },
		"empty output with wrong owner":        func(f *projectionFixture) { f.targets[0].EvalTargetOutputData = nil; f.targets[0].SpaceID = 99 },
		"deleted reused score with wrong owner": func(f *projectionFixture) {
			markProjectionScoreDeleted(f.evaluators[0], 1000)
			f.evaluators[0].ExperimentRunID, f.evaluators[0].SpaceID = 99, 99
		},
		"deleted reused score with wrong version": func(f *projectionFixture) {
			markProjectionScoreDeleted(f.evaluators[0], 1000)
			f.evaluators[0].ExperimentRunID, f.evaluators[0].ItemVersionID = 99, 99
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			f := newProjectionFixture(t)
			change(f)
			rows, err := f.service().BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
			require.Error(t, err)
			require.Empty(t, rows)
		})
	}
}
