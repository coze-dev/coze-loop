// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/stretchr/testify/require"
)

type deletedScoreProjectionRepo struct {
	repo.IEvaluatorRecordRepo
	t       *testing.T
	records []*entity.EvaluatorRecord
	readErr error
}

func (r *deletedScoreProjectionRepo) BatchGetEvaluatorRecord(_ context.Context, ids []int64, includeDeleted, withFullContent bool, _ ...entity.GetEvaluatorRecordOptionFn) ([]*entity.EvaluatorRecord, error) {
	require.False(r.t, withFullContent, "deletion checks must not load TOS content")
	if r.readErr != nil {
		return nil, r.readErr
	}
	wanted := make(map[int64]bool, len(ids))
	for _, id := range ids {
		wanted[id] = true
	}
	var records []*entity.EvaluatorRecord
	for _, record := range r.records {
		if !wanted[record.ID] || !includeDeleted && record.BaseInfo != nil && record.BaseInfo.DeletedAt != nil {
			continue
		}
		records = append(records, record)
	}
	return records, nil
}

type deletedScoreProjectionService struct {
	EvaluatorRecordService
	source *deletedScoreProjectionRepo
}

func (s deletedScoreProjectionService) BatchGetEvaluatorRecord(ctx context.Context, ids []int64, includeDeleted, withFullContent bool) ([]*entity.EvaluatorRecord, error) {
	return s.source.BatchGetEvaluatorRecord(ctx, ids, includeDeleted, withFullContent)
}

func markProjectionScoreDeleted(record *entity.EvaluatorRecord, timestamp int64) {
	record.BaseInfo = &entity.BaseInfo{DeletedAt: &timestamp}
}

func TestBuildExptFilterProjectionDeletedScoresKeepItem(t *testing.T) {
	tests := []struct {
		name      string
		change    func(*projectionFixture)
		want      map[string]float64
		weight    float64
		hasWeight bool
		corrected bool
	}{
		{name: "active scores unchanged", want: map[string]float64{"key11": 0, "key12": 1}, weight: 0.5, hasWeight: true},
		{name: "deleted correction does not restore score or weighted input", change: func(f *projectionFixture) {
			markProjectionScoreDeleted(f.evaluators[0], 1000)
			score := 9.0
			f.evaluators[0].EvaluatorOutputData.EvaluatorResult.Correction = &entity.Correction{Score: &score}
		}, want: map[string]float64{"key12": 1}, weight: 1, hasWeight: true},
		{name: "active zero remains present", change: func(f *projectionFixture) {
			markProjectionScoreDeleted(f.evaluators[1], 1000)
		}, want: map[string]float64{"key11": 0}, hasWeight: true},
		{name: "all deleted leaves score absent and keeps item", change: func(f *projectionFixture) {
			for _, record := range f.evaluators {
				markProjectionScoreDeleted(record, 1000)
			}
		}, want: map[string]float64{}},
		{name: "active absent score is not zero", change: func(f *projectionFixture) {
			f.evaluators[0].EvaluatorOutputData.EvaluatorResult.Score = nil
			markProjectionScoreDeleted(f.evaluators[1], 1000)
		}, want: map[string]float64{}},
		{name: "persisted weighted score keeps source precedence", change: func(f *projectionFixture) {
			for _, record := range f.evaluators {
				markProjectionScoreDeleted(record, 1000)
			}
			weight := 0.625
			f.turns[0].WeightedScore = &weight
		}, want: map[string]float64{}, weight: 0.625, hasWeight: true},
		{name: "persisted weighted zero stays present", change: func(f *projectionFixture) {
			for _, record := range f.evaluators {
				markProjectionScoreDeleted(record, 1000)
			}
			weight := 0.0
			f.turns[0].WeightedScore = &weight
		}, want: map[string]float64{}, hasWeight: true},
		{name: "active correction remains effective", change: func(f *projectionFixture) {
			markProjectionScoreDeleted(f.evaluators[1], 1000)
			score := 0.25
			f.evaluators[0].EvaluatorOutputData.EvaluatorResult.Correction = &entity.Correction{Score: &score}
		}, want: map[string]float64{"key11": 0.25}, weight: 0.25, hasWeight: true, corrected: true},
		{name: "present epoch deletion marker is still deleted", change: func(f *projectionFixture) {
			markProjectionScoreDeleted(f.evaluators[0], 0)
		}, want: map[string]float64{"key12": 1}, weight: 1, hasWeight: true},
	}
	for _, sourceKind := range []string{"repo", "service"} {
		for _, tt := range tests {
			t.Run(sourceKind+"/"+tt.name, func(t *testing.T) {
				f := newProjectionFixture(t)
				if tt.change != nil {
					tt.change(f)
				}
				before, err := json.Marshal(f.evaluators)
				require.NoError(t, err)
				source := &deletedScoreProjectionRepo{t: t, records: f.evaluators}
				s := f.service()
				if sourceKind == "repo" {
					s.filterEvaluatorRepo = source
				} else {
					s.evaluatorRecordService = deletedScoreProjectionService{source: source}
				}
				rows, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
				require.NoError(t, err)
				require.Len(t, rows, 1)
				row := rows[0]
				require.Equal(t, int64(3), row.ItemID)
				require.Equal(t, map[string]string{"actual_output": "", "custom": "preview"}, row.EvalTargetData)
				require.Equal(t, map[string]int64{"input_tokens": 0, "output_tokens": 0, "total_tokens": 0, "total_latency": 1501}, row.EvalTargetMetrics)
				require.Equal(t, map[string]float64{"key1": 0}, row.AnnotationFloat)
				require.Equal(t, map[string]string{"81": "", "82": "83"}, row.AnnotationString)
				require.Equal(t, tt.want, row.EvaluatorScore)
				require.Equal(t, tt.corrected, row.EvaluatorScoreCorrected)
				if tt.hasWeight {
					require.NotNil(t, row.EvaluatorWeightedScore)
					require.Equal(t, tt.weight, *row.EvaluatorWeightedScore)
				} else {
					require.Nil(t, row.EvaluatorWeightedScore)
				}
				after, err := json.Marshal(f.evaluators)
				require.NoError(t, err)
				require.Equal(t, before, after, "projection must not revive or mutate source records")
			})
		}
	}
}

func TestBuildExptFilterProjectionDeletedScoreGuards(t *testing.T) {
	tests := map[string]func(*projectionFixture, *deletedScoreProjectionRepo){
		"read error": func(_ *projectionFixture, r *deletedScoreProjectionRepo) {
			r.readErr = errors.New("record read unavailable")
		},
		"wrong record owner":         func(f *projectionFixture, _ *deletedScoreProjectionRepo) { f.evaluators[0].SpaceID = 99 },
		"wrong experiment":           func(f *projectionFixture, _ *deletedScoreProjectionRepo) { f.evaluators[0].ExperimentID = 99 },
		"wrong item":                 func(f *projectionFixture, _ *deletedScoreProjectionRepo) { f.evaluators[0].ItemID = 99 },
		"wrong turn":                 func(f *projectionFixture, _ *deletedScoreProjectionRepo) { f.evaluators[0].TurnID = 99 },
		"wrong evaluator version":    func(f *projectionFixture, _ *deletedScoreProjectionRepo) { f.evaluators[0].EvaluatorVersionID = 99 },
		"wrong item version":         func(f *projectionFixture, _ *deletedScoreProjectionRepo) { f.evaluators[0].ItemVersionID = 99 },
		"deleted record missing run": func(f *projectionFixture, _ *deletedScoreProjectionRepo) { f.evaluators[0].ExperimentRunID = 0 },
	}
	for _, sourceKind := range []string{"repo", "service"} {
		for name, change := range tests {
			t.Run(sourceKind+"/"+name, func(t *testing.T) {
				f := newProjectionFixture(t)
				markProjectionScoreDeleted(f.evaluators[0], 1000)
				source := &deletedScoreProjectionRepo{t: t, records: f.evaluators}
				change(f, source)
				s := f.service()
				if sourceKind == "repo" {
					s.filterEvaluatorRepo = source
				} else {
					s.evaluatorRecordService = deletedScoreProjectionService{source: source}
				}
				rows, err := s.BuildExptFilterProjection(context.Background(), 1, 2, []int64{3})
				require.Error(t, err)
				require.Empty(t, rows)
			})
		}
	}
}
