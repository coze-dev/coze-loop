// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/stretchr/testify/require"
)

func legacyEmptyEvaluatorFixture(t *testing.T) *projectionFixture {
	f := newProjectionFixture(t)
	f.expt.EvalSetSourceType = entity.ExptEvalSetSourceType_SingleSet
	f.expt.EvalConf.ConnectorConf.EvaluatorsConf = &entity.EvaluatorsConf{
		EvaluatorConf: []*entity.EvaluatorConf{{EvaluatorVersionID: 70}},
	}
	f.evaluatorRefs = []*entity.ExptTurnEvaluatorResultRef{{ID: 100, SpaceID: 1, ExptID: 2, ExptTurnResultID: 10}}
	f.evaluators = nil
	f.mappings[0].FromField = "70"
	f.mappings = append(f.mappings[:1], f.mappings[2:]...)
	return f
}

func TestProjectionLegacyEmptyEvaluatorReferenceKeepsItem(t *testing.T) {
	f := legacyEmptyEvaluatorFixture(t)
	f.fail = "evaluators"
	before := projectionSourceDigest(t, f)

	rows, err := f.service().BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})

	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, int64(3), rows[0].ItemID)
	require.Equal(t, entity.ItemRunState_Success, rows[0].Status)
	require.Equal(t, map[string]string{"actual_output": "", "custom": "preview"}, rows[0].EvalTargetData)
	require.Equal(t, map[string]float64{"key1": 0}, rows[0].AnnotationFloat)
	require.Empty(t, rows[0].EvaluatorScore)
	require.False(t, rows[0].EvaluatorScoreCorrected)
	require.Nil(t, rows[0].EvaluatorWeightedScore)
	require.Equal(t, before, projectionSourceDigest(t, f))
}

func TestProjectionLegacyEmptyEvaluatorReferenceKeepsFiveItems(t *testing.T) {
	f := legacyEmptyEvaluatorFixture(t)
	turn, item, itemRef, target, evaluatorRef := *f.turns[0], *f.items[0], *f.refs[0], *f.targets[0], *f.evaluatorRefs[0]
	f.turns, f.items, f.refs, f.targets, f.evaluatorRefs = nil, nil, nil, nil, nil
	f.tags, f.annotations, f.annotationRefs = nil, nil, nil
	f.mappings = f.mappings[:1]
	ids := []int64{3, 4, 5, 6, 7}
	for i, id := range ids {
		tr, it, ir, ta, er := turn, item, itemRef, target, evaluatorRef
		tr.ID, tr.ItemID, tr.TargetResultID = int64(10+i), id, int64(50+i)
		it.ID, it.ItemID = int64(20+i), id
		ir.ItemID = id
		ta.ID, ta.ItemID = tr.TargetResultID, id
		er.ID, er.ExptTurnResultID = int64(100+i), tr.ID
		f.turns, f.items, f.refs = append(f.turns, &tr), append(f.items, &it), append(f.refs, &ir)
		f.targets, f.evaluatorRefs = append(f.targets, &ta), append(f.evaluatorRefs, &er)
	}
	before := projectionSourceDigest(t, f)

	rows, err := f.service().BuildExptFilterProjection(context.Background(), 1, 2, ids)

	require.NoError(t, err)
	require.Len(t, rows, len(ids))
	for i, row := range rows {
		require.Equal(t, ids[i], row.ItemID)
		require.Equal(t, map[string]string{"actual_output": "", "custom": "preview"}, row.EvalTargetData)
		require.Empty(t, row.EvaluatorScore)
		require.Nil(t, row.EvaluatorWeightedScore)
	}
	require.Equal(t, before, projectionSourceDigest(t, f))
}

func TestProjectionLegacyEmptyEvaluatorReferenceGuards(t *testing.T) {
	cases := map[string]func(*projectionFixture){
		"nil_reference":      func(f *projectionFixture) { f.evaluatorRefs[0] = nil },
		"foreign_space":      func(f *projectionFixture) { f.evaluatorRefs[0].SpaceID = 99 },
		"foreign_experiment": func(f *projectionFixture) { f.evaluatorRefs[0].ExptID = 99 },
		"missing_turn":       func(f *projectionFixture) { f.evaluatorRefs[0].ExptTurnResultID = 99 },
		"multiset":           func(f *projectionFixture) { f.expt.EvalSetSourceType = entity.ExptEvalSetSourceType_MultiSetConfig },
		"unknown_set_source": func(f *projectionFixture) { f.expt.EvalSetSourceType = 0 },
		"builtin_source": func(f *projectionFixture) {
			f.evaluatorRefs[0].SourceType = int32(entity.EvaluatorRecordSourceTypeBuiltin)
		},
		"inline_source": func(f *projectionFixture) {
			f.evaluatorRefs[0].SourceType = int32(entity.EvaluatorRecordSourceTypeInline)
		},
		"unknown_record_source": func(f *projectionFixture) { f.evaluatorRefs[0].SourceType = 99 },
		"positive_version":      func(f *projectionFixture) { f.evaluatorRefs[0].EvaluatorVersionID = 70 },
		"negative_version":      func(f *projectionFixture) { f.evaluatorRefs[0].EvaluatorVersionID = -1 },
		"positive_record":       func(f *projectionFixture) { f.evaluatorRefs[0].EvaluatorResultID = 60 },
		"negative_record":       func(f *projectionFixture) { f.evaluatorRefs[0].EvaluatorResultID = -1 },
		"alias_present":         func(f *projectionFixture) { f.evaluatorRefs[0].Alias = "a" },
		"inline_key_present":    func(f *projectionFixture) { f.evaluatorRefs[0].InlineKey = "score" },
		"repeated_empty_reference": func(f *projectionFixture) {
			other := *f.evaluatorRefs[0]
			other.ID++
			f.evaluatorRefs = append(f.evaluatorRefs, &other)
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			f := legacyEmptyEvaluatorFixture(t)
			change(f)
			before := projectionSourceDigest(t, f)

			rows, err := f.service().BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})

			require.ErrorContains(t, err, "stage=evaluator_refs")
			require.Empty(t, rows)
			require.Equal(t, before, projectionSourceDigest(t, f))
		})
	}
}

func TestProjectionLegacyEmptyEvaluatorReferenceKeepsRealScores(t *testing.T) {
	f := newProjectionFixture(t)
	f.expt.EvalSetSourceType = entity.ExptEvalSetSourceType_SingleSet
	f.evaluatorRefs = append([]*entity.ExptTurnEvaluatorResultRef{{SpaceID: 1, ExptID: 2, ExptTurnResultID: 10}}, f.evaluatorRefs...)
	before := projectionSourceDigest(t, f)

	rows, err := f.service().BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})

	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, map[string]float64{"key11": 0, "key12": 1}, rows[0].EvaluatorScore)
	require.Equal(t, 0.5, *rows[0].EvaluatorWeightedScore)
	require.Equal(t, before, projectionSourceDigest(t, f))
}

func TestProjectionLegacyEmptyEvaluatorReferenceKeepsPositiveGuards(t *testing.T) {
	for _, tc := range []struct {
		name   string
		stage  string
		change func(*projectionFixture)
	}{
		{"duplicate_record", "evaluator_refs", func(f *projectionFixture) {
			f.evaluatorRefs = append(f.evaluatorRefs, f.evaluatorRefs[0])
		}},
		{"duplicate_instance", "evaluator_refs", func(f *projectionFixture) {
			other := *f.evaluatorRefs[0]
			other.EvaluatorResultID = 62
			f.evaluatorRefs = append(f.evaluatorRefs, &other)
		}},
		{"record_read_error", "evaluators", func(f *projectionFixture) { f.fail = "evaluators" }},
		{"foreign_record", "evaluators", func(f *projectionFixture) { f.evaluators[0].SpaceID = 99 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newProjectionFixture(t)
			f.expt.EvalSetSourceType = entity.ExptEvalSetSourceType_SingleSet
			tc.change(f)
			f.evaluatorRefs = append([]*entity.ExptTurnEvaluatorResultRef{{SpaceID: 1, ExptID: 2, ExptTurnResultID: 10}}, f.evaluatorRefs...)
			before := projectionSourceDigest(t, f)

			rows, err := f.service().BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})

			require.ErrorContains(t, err, "stage="+tc.stage)
			require.Empty(t, rows)
			require.Equal(t, before, projectionSourceDigest(t, f))
		})
	}
}

func TestProjectionLegacyEmptyEvaluatorReferenceKeepsAbsentScores(t *testing.T) {
	f := newProjectionFixture(t)
	f.expt.EvalSetSourceType = entity.ExptEvalSetSourceType_SingleSet
	f.evaluators = nil
	f.evaluatorRefs = append([]*entity.ExptTurnEvaluatorResultRef{{SpaceID: 1, ExptID: 2, ExptTurnResultID: 10}}, f.evaluatorRefs...)
	before := projectionSourceDigest(t, f)

	rows, err := f.service().BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})

	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Equal(t, "preview", rows[0].EvalTargetData["custom"])
	require.Empty(t, rows[0].EvaluatorScore)
	require.Nil(t, rows[0].EvaluatorWeightedScore)
	require.Equal(t, before, projectionSourceDigest(t, f))
}
