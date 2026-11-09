// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"math"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

func unconfiguredProjectionFixture(t *testing.T) *projectionFixture {
	f := newProjectionFixture(t)
	f.refs[0].ItemConfig = &entity.ExptItemConfig{}
	f.expt.EvalConf.EvalSetConfigs[0].EvaluatorConfs = []*entity.ExptEvaluatorConf{{EvaluatorVersionID: 70, Alias: "a"}, {EvaluatorVersionID: 71, Alias: "b"}}
	f.evaluators[0].Alias = ""
	f.evaluators[1].Alias = ""
	f.evaluators[1].EvaluatorVersionID = 71
	f.evaluatorRefs[0].Alias = ""
	f.evaluatorRefs[1].Alias = ""
	f.evaluatorRefs[1].EvaluatorVersionID = 71
	f.mappings[1].FromField = "71:b"
	return f
}

func TestBuildExptFilterProjectionUnconfiguredLegacyEvaluators(t *testing.T) {
	zero, half, corrected := 0.0, 0.5, 0.8
	for _, tc := range []struct {
		name      string
		change    func(*projectionFixture)
		weighted  *float64
		corrected bool
	}{
		{name: "unmapped legacy scores retain equal weight calculation", weighted: &half},
		{name: "named weights do not alias bare records", change: func(f *projectionFixture) {
			one := 1.0
			f.expt.EvalConf.ConnectorConf.EvaluatorsConf = &entity.EvaluatorsConf{EnableScoreWeight: true}
			for _, c := range f.expt.EvalConf.EvalSetConfigs[0].EvaluatorConfs {
				c.ScoreWeight = &one
			}
		}},
		{name: "persisted zero is retained", change: func(f *projectionFixture) { f.turns[0].WeightedScore = &zero }, weighted: &zero},
		{name: "corrections retain aggregate and flag", change: func(f *projectionFixture) {
			value := 0.6
			f.evaluators[0].EvaluatorOutputData.EvaluatorResult.Correction = &entity.Correction{Score: &value}
		}, weighted: &corrected, corrected: true},
		{name: "absent scores remain absent", change: func(f *projectionFixture) {
			for _, r := range f.evaluators {
				r.EvaluatorOutputData = nil
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := unconfiguredProjectionFixture(t)
			if tc.change != nil {
				tc.change(f)
			}
			s := f.service()
			rows, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
			require.NoError(t, err)
			require.Len(t, rows, 1)
			require.Empty(t, rows[0].EvaluatorScore)
			require.Equal(t, tc.weighted, rows[0].EvaluatorWeightedScore)
			require.Equal(t, tc.corrected, rows[0].EvaluatorScoreCorrected)
			require.Equal(t, "preview", rows[0].EvalTargetData["custom"])
			require.Len(t, f.evaluators, 2)
			require.Empty(t, f.evaluators[0].Alias)
			require.Empty(t, f.evaluatorRefs[0].Alias)
		})
	}
}

func TestBuildExptFilterProjectionUnconfiguredLegacyGuards(t *testing.T) {
	for name, change := range map[string]func(*projectionFixture){
		"single set":                 func(f *projectionFixture) { f.expt.EvalSetSourceType = entity.ExptEvalSetSourceType_SingleSet },
		"missing frozen item config": func(f *projectionFixture) { f.refs[0].ItemConfig = nil },
		"configured item": func(f *projectionFixture) {
			f.refs[0].ItemConfig.EvaluatorConfs = []*entity.ItemEvaluatorConf{{EvaluatorVersionID: 70, Alias: "a"}}
		},
		"unmapped named alias": func(f *projectionFixture) { f.evaluatorRefs[0].Alias = "other"; f.evaluators[0].Alias = "other" },
		"unknown version": func(f *projectionFixture) {
			f.evaluatorRefs[0].EvaluatorVersionID = 999
			f.evaluators[0].EvaluatorVersionID = 999
		},
		"missing named mapping": func(f *projectionFixture) { f.mappings = f.mappings[1:] },
		"duplicate bare instance": func(f *projectionFixture) {
			f.evaluatorRefs[1].EvaluatorVersionID = 70
			f.evaluators[1].EvaluatorVersionID = 70
		},
		"wrong record owner":       func(f *projectionFixture) { f.evaluators[0].ExperimentID = 99 },
		"mismatched record alias":  func(f *projectionFixture) { f.evaluators[0].Alias = "a" },
		"nonfinite unmapped score": func(f *projectionFixture) { *f.evaluators[0].EvaluatorOutputData.EvaluatorResult.Score = math.NaN() },
	} {
		t.Run(name, func(t *testing.T) {
			f := unconfiguredProjectionFixture(t)
			change(f)
			s := f.service()
			rows, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
			require.Error(t, err)
			require.Empty(t, rows)
		})
	}
}
