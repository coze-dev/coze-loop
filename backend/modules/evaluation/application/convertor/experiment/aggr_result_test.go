// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestAggregateDataDOToDTO(t *testing.T) {
	t.Parallel()

	assert.Nil(t, AggregateDataDOToDTO(nil))

	tests := []struct {
		name   string
		input  *entity.AggregateData
		verify func(t *testing.T, got *domain_expt.AggregateData)
	}{
		{
			name: "value rounded to two decimals",
			input: &entity.AggregateData{
				DataType: entity.Double,
				Value:    gptr.Of(0.12345),
			},
			verify: func(t *testing.T, got *domain_expt.AggregateData) {
				assert.Equal(t, domain_expt.DataType(entity.Double), got.DataType)
				if assert.NotNil(t, got.Value) {
					assert.Equal(t, 0.12, *got.Value)
				}
			},
		},
		{
			name: "value nil left nil",
			input: &entity.AggregateData{
				DataType: entity.Double,
			},
			verify: func(t *testing.T, got *domain_expt.AggregateData) {
				assert.Nil(t, got.Value)
			},
		},
		{
			name: "score distribution converted",
			input: &entity.AggregateData{
				DataType: entity.ScoreDistribution,
				ScoreDistribution: &entity.ScoreDistributionData{
					ScoreDistributionItems: []*entity.ScoreDistributionItem{
						{Score: "1", Count: 3, Percentage: 0.3},
						nil,
						{Score: "2", Count: 7, Percentage: 0.7},
					},
				},
			},
			verify: func(t *testing.T, got *domain_expt.AggregateData) {
				if assert.NotNil(t, got.ScoreDistribution) {
					assert.Len(t, got.ScoreDistribution.ScoreDistributionItems, 2)
					assert.Equal(t, "1", got.ScoreDistribution.ScoreDistributionItems[0].Score)
					assert.Equal(t, int64(7), got.ScoreDistribution.ScoreDistributionItems[1].Count)
				}
				assert.Nil(t, got.OptionDistribution)
			},
		},
		{
			name: "option distribution converted",
			input: &entity.AggregateData{
				DataType: entity.OptionDistribution,
				OptionDistribution: &entity.OptionDistributionData{
					OptionDistributionItems: []*entity.OptionDistributionItem{
						{Option: "A", Count: 1, Percentage: 0.5},
						{Option: "B", Count: 1, Percentage: 0.5},
					},
				},
			},
			verify: func(t *testing.T, got *domain_expt.AggregateData) {
				if assert.NotNil(t, got.OptionDistribution) {
					assert.Len(t, got.OptionDistribution.OptionDistributionItems, 2)
				}
				assert.Nil(t, got.ScoreDistribution)
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := AggregateDataDOToDTO(tt.input)
			if assert.NotNil(t, got) {
				tt.verify(t, got)
			}
		})
	}
}

func TestScoreDistributionItemsDOToDTO(t *testing.T) {
	t.Parallel()

	assert.Nil(t, ScoreDistributionItemsDOToDTO(nil))
	assert.Nil(t, ScoreDistributionItemsDOToDTO([]*entity.ScoreDistributionItem{}))

	got := ScoreDistributionItemsDOToDTO([]*entity.ScoreDistributionItem{
		nil,
		{Score: "1", Count: 2, Percentage: 0.5},
	})
	if assert.Len(t, got, 1) {
		assert.Equal(t, "1", got[0].Score)
		assert.Equal(t, int64(2), got[0].Count)
		assert.Equal(t, 0.5, got[0].Percentage)
	}
}

func TestOptionDistributionItemsDOToDTO(t *testing.T) {
	t.Parallel()

	assert.Nil(t, OptionDistributionItemsDOToDTO(nil))
	assert.Nil(t, OptionDistributionItemsDOToDTO([]*entity.OptionDistributionItem{}))

	got := OptionDistributionItemsDOToDTO([]*entity.OptionDistributionItem{
		nil,
		{Option: "A", Count: 4, Percentage: 0.4},
	})
	if assert.Len(t, got, 1) {
		assert.Equal(t, "A", got[0].Option)
		assert.Equal(t, int64(4), got[0].Count)
		assert.Equal(t, 0.4, got[0].Percentage)
	}
}

func TestAggregatorResultDOToDTO(t *testing.T) {
	t.Parallel()

	assert.Nil(t, AggregatorResultDOToDTO(nil))

	got := AggregatorResultDOToDTO(&entity.AggregatorResult{
		AggregatorType: entity.Average,
		Data:           &entity.AggregateData{DataType: entity.Double, Value: gptr.Of(1.129)},
	})
	if assert.NotNil(t, got) {
		assert.Equal(t, domain_expt.AggregatorType(entity.Average), got.AggregatorType)
		if assert.NotNil(t, got.Data) {
			assert.Equal(t, 1.13, *got.Data.Value) // rounded to 2 decimals
		}
	}

	got = AggregatorResultDOToDTO(&entity.AggregatorResult{AggregatorType: entity.Sum})
	if assert.NotNil(t, got) {
		assert.Nil(t, got.Data)
	}
}

func TestAggregatorResultDOsToDTOs(t *testing.T) {
	t.Parallel()

	assert.Nil(t, AggregatorResultDOsToDTOs(nil))
	assert.Nil(t, AggregatorResultDOsToDTOs([]*entity.AggregatorResult{}))

	in := []*entity.AggregatorResult{
		{AggregatorType: entity.Average, Data: &entity.AggregateData{Value: gptr.Of(1.0)}},
		{AggregatorType: entity.Max, Data: &entity.AggregateData{Value: gptr.Of(2.0)}},
	}
	got := AggregatorResultDOsToDTOs(in)
	assert.Len(t, got, 2)
}

func TestEvaluatorResultsDOToDTO(t *testing.T) {
	t.Parallel()

	assert.Nil(t, EvaluatorResultsDOToDTO(nil))

	got := EvaluatorResultsDOToDTO(&entity.EvaluatorAggregateResult{
		EvaluatorVersionID: 42,
		Name:               gptr.Of("acc"),
		Version:            gptr.Of("v1"),
		Alias:              "judge_b",
		AggregatorResults: []*entity.AggregatorResult{
			{AggregatorType: entity.Average, Data: &entity.AggregateData{Value: gptr.Of(0.9)}},
		},
	})
	if assert.NotNil(t, got) {
		assert.Equal(t, int64(42), got.EvaluatorVersionID)
		assert.Equal(t, "acc", *got.Name)
		assert.Equal(t, "v1", *got.Version)
		if assert.NotNil(t, got.Alias) {
			assert.Equal(t, "judge_b", *got.Alias)
		}
		assert.Len(t, got.AggregatorResults, 1)
	}

	// empty alias should still marshal (empty string pointer)
	got = EvaluatorResultsDOToDTO(&entity.EvaluatorAggregateResult{EvaluatorVersionID: 1})
	if assert.NotNil(t, got) && assert.NotNil(t, got.Alias) {
		assert.Equal(t, "", *got.Alias)
	}
}

func TestAnnotationResultDOToDTO(t *testing.T) {
	t.Parallel()

	assert.Nil(t, AnnotationResultDOToDTO(nil))

	got := AnnotationResultDOToDTO(&entity.AnnotationAggregateResult{
		TagKeyID: 7,
		Name:     gptr.Of("quality"),
		AggregatorResults: []*entity.AggregatorResult{
			{AggregatorType: entity.Average, Data: &entity.AggregateData{Value: gptr.Of(0.5)}},
		},
	})
	if assert.NotNil(t, got) {
		assert.Equal(t, int64(7), got.TagKeyID)
		assert.Equal(t, "quality", *got.Name)
		assert.Len(t, got.AggregatorResults, 1)
	}
}

func TestExptAggregateResultDOToDTO(t *testing.T) {
	t.Parallel()

	assert.Nil(t, ExptAggregateResultDOToDTO(nil))

	ts := time.Unix(1_700_000_000, 0)
	do := &entity.ExptAggregateResult{
		ExperimentID: 123,
		Status:       1,
		UpdateTime:   &ts,
		EvaluatorResults: map[string]*entity.EvaluatorAggregateResult{
			// same versionID, different alias — thrift map keyed by versionID drops one; test intentionally uses different versionIDs to keep both.
			"11":             {EvaluatorVersionID: 11, Alias: "default"},
			"12:judge_alias": {EvaluatorVersionID: 12, Alias: "judge_alias"},
		},
		AnnotationResults: map[int64]*entity.AnnotationAggregateResult{
			99: {TagKeyID: 99, Name: gptr.Of("tag")},
		},
		TargetResults: &entity.EvalTargetMtrAggrResult{TargetID: 55},
		WeightedResults: []*entity.AggregatorResult{
			{AggregatorType: entity.Average, Data: &entity.AggregateData{Value: gptr.Of(0.8)}},
		},
	}

	got := ExptAggregateResultDOToDTO(do)
	if !assert.NotNil(t, got) {
		return
	}
	assert.Equal(t, int64(123), got.ExperimentID)
	assert.Len(t, got.EvaluatorResults, 2)
	assert.Contains(t, got.EvaluatorResults, int64(11))
	assert.Contains(t, got.EvaluatorResults, int64(12))
	assert.Len(t, got.AnnotationResults, 1)
	assert.Contains(t, got.AnnotationResults, int64(99))
	if assert.NotNil(t, got.EvalTargetAggrResult_) {
		assert.Equal(t, int64(55), *got.EvalTargetAggrResult_.TargetID)
	}
	if assert.NotNil(t, got.UpdateTime) {
		assert.Equal(t, ts.Unix(), *got.UpdateTime)
	}
	assert.Len(t, got.WeightedResults, 1)
	if assert.NotNil(t, got.Status) {
		assert.Equal(t, domain_expt.ExptAggregateCalculateStatus(1), *got.Status)
	}
}

func TestExptAggregateResultDOToDTO_NoOptionalFields(t *testing.T) {
	t.Parallel()

	got := ExptAggregateResultDOToDTO(&entity.ExptAggregateResult{ExperimentID: 1})
	if !assert.NotNil(t, got) {
		return
	}
	assert.Nil(t, got.UpdateTime)
	assert.Empty(t, got.WeightedResults)
	assert.Empty(t, got.EvaluatorResults)
	assert.Empty(t, got.AnnotationResults)
	assert.Nil(t, got.EvalTargetAggrResult_)
}
