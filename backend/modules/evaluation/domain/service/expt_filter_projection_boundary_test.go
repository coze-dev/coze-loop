// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"math"
	"strings"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

func TestProjectionRejectsInvalidSourceRecordsAndScoreCalculation(t *testing.T) {
	for _, tc := range []struct {
		name   string
		stage  string
		change func(*projectionFixture, *ExptResultServiceImpl)
	}{
		{"nil_evaluator", "evaluators", func(f *projectionFixture, _ *ExptResultServiceImpl) { f.evaluators[0] = nil }},
		{"duplicate_evaluator", "evaluators", func(f *projectionFixture, _ *ExptResultServiceImpl) {
			f.evaluators = append(f.evaluators, f.evaluators[0])
		}},
		{"nil_target", "targets", func(f *projectionFixture, _ *ExptResultServiceImpl) { f.targets[0] = nil }},
		{"duplicate_target", "targets", func(f *projectionFixture, _ *ExptResultServiceImpl) { f.targets = append(f.targets, f.targets[0]) }},
		{"nil_annotation", "annotations", func(f *projectionFixture, _ *ExptResultServiceImpl) { f.annotations[0] = nil }},
		{"duplicate_annotation", "annotations", func(f *projectionFixture, _ *ExptResultServiceImpl) {
			f.annotations = append(f.annotations, f.annotations[0])
		}},
		{"duplicate_tag", "tag_refs", func(f *projectionFixture, _ *ExptResultServiceImpl) { f.tags = append(f.tags, f.tags[0]) }},
		{"foreign_annotation_ref", "annotation_refs", func(f *projectionFixture, _ *ExptResultServiceImpl) { f.annotationRefs[0].SpaceID = 99 }},
		{"missing_annotation_mapping", "annotation_refs", func(f *projectionFixture, _ *ExptResultServiceImpl) { f.mappings = f.mappings[:2] }},
		{"unknown_annotation_type", "annotation", func(f *projectionFixture, _ *ExptResultServiceImpl) {
			f.annotations[0].AnnotateData.TagContentType = "unsupported"
		}},
		{"unknown_experiment_type", "experiment", func(f *projectionFixture, _ *ExptResultServiceImpl) { f.expt.ExptType = 99 }},
		{"multiset_reader_missing", "item_refs", func(_ *projectionFixture, svc *ExptResultServiceImpl) { svc.exptItemRefRepo = nil }},
		{"pure_calculator_missing", "weighted_score", func(_ *projectionFixture, svc *ExptResultServiceImpl) { svc.scoreCalculator = nil }},
		{"nonfinite_persisted_weight", "weighted_score", func(f *projectionFixture, _ *ExptResultServiceImpl) {
			value := math.Inf(1)
			f.turns[0].WeightedScore = &value
		}},
		{"nonfinite_configured_weight", "weighted_score", func(f *projectionFixture, _ *ExptResultServiceImpl) {
			value := math.Inf(1)
			f.expt.EvalConf.ConnectorConf.EvaluatorsConf = &entity.EvaluatorsConf{EnableScoreWeight: true}
			f.expt.EvalConf.EvalSetConfigs[0].EvaluatorConfs = []*entity.ExptEvaluatorConf{{EvaluatorVersionID: 70, Alias: "a", ScoreWeight: &value}}
		}},
		{"finite_scores_overflow_average", "weighted_score", func(f *projectionFixture, _ *ExptResultServiceImpl) {
			value := math.MaxFloat64
			for _, record := range f.evaluators {
				record.EvaluatorOutputData.EvaluatorResult.Score = &value
			}
		}},
		{"ambiguous_dataset_source", "item_refs", func(f *projectionFixture, _ *ExptResultServiceImpl) {
			f.refs[0].EvalSetSourceSpaceID = 0
			f.expt.EvalConf.EvalSetConfigs = append(f.expt.EvalConf.EvalSetConfigs, &entity.EvalSetConfig{EvalSetID: 30, EvalSetVersionID: 31, SourceSpaceID: 99})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newProjectionFixture(t)
			svc := f.service()
			tc.change(f, &svc)
			rows, err := svc.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
			require.ErrorContains(t, err, "stage="+tc.stage)
			require.Empty(t, rows, "invalid source must not publish a partial projection")
		})
	}
}

func TestProjectionOversizeItemDoesNotReturnTruncatedTurns(t *testing.T) {
	f := newProjectionFixture(t)
	f.turns, f.targets = nil, nil
	f.evaluatorRefs, f.evaluators = nil, nil
	f.tags, f.annotationRefs, f.annotations = nil, nil, nil
	text := strings.Repeat("x", 1<<20)
	for i := int64(0); i < 70; i++ {
		f.turns = append(f.turns, &entity.ExptTurnResult{ID: 1000 + i, SpaceID: 1, ExptID: 2, ItemID: 3, TurnID: i, ExptRunID: 4, TargetResultID: 2000 + i})
		f.targets = append(f.targets, &entity.EvalTargetRecord{ID: 2000 + i, SpaceID: 9, ItemID: 3, TurnID: i, ExperimentRunID: 4, EvalTargetOutputData: &entity.EvalTargetOutputData{OutputFields: map[string]*entity.Content{"actual_output": {Text: &text}}}})
	}
	rows, err := f.service().BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
	require.ErrorContains(t, err, "oversize_item")
	require.Nil(t, rows)
	require.Len(t, f.turns, 70)
	require.Len(t, f.targets, 70)
}
