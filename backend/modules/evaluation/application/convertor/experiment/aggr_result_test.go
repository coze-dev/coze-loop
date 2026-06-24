// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestExptAggregateResultDOToDTO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, ExptAggregateResultDOToDTO(nil))
	})

	t.Run("full input", func(t *testing.T) {
		updateTime := time.Unix(1000, 0)
		data := &entity.ExptAggregateResult{
			ExperimentID: 100,
			Status:       1,
			UpdateTime:   &updateTime,
			EvaluatorResults: map[int64]*entity.EvaluatorAggregateResult{
				10: {
					EvaluatorVersionID: 10,
					AggregatorResults: []*entity.AggregatorResult{
						{AggregatorType: entity.Average, Data: &entity.AggregateData{Value: gptr.Of(0.95)}},
					},
					Name:    gptr.Of("eval1"),
					Version: gptr.Of("v1"),
				},
			},
			AnnotationResults: map[int64]*entity.AnnotationAggregateResult{
				20: {
					TagKeyID: 20,
					AggregatorResults: []*entity.AggregatorResult{
						{AggregatorType: entity.Sum, Data: &entity.AggregateData{Value: gptr.Of(5.0)}},
					},
					Name: gptr.Of("tag1"),
				},
			},
			WeightedResults: []*entity.AggregatorResult{
				{AggregatorType: entity.Average, Data: &entity.AggregateData{Value: gptr.Of(0.88)}},
			},
		}

		dto := ExptAggregateResultDOToDTO(data)
		assert.NotNil(t, dto)
		assert.Equal(t, int64(100), dto.ExperimentID)
		assert.Equal(t, int64(1000), *dto.UpdateTime)
		assert.Len(t, dto.EvaluatorResults, 1)
		assert.NotNil(t, dto.EvaluatorResults[10])
		assert.Len(t, dto.AnnotationResults, 1)
		assert.NotNil(t, dto.AnnotationResults[20])
		assert.Len(t, dto.WeightedResults, 1)
	})

	t.Run("without update time", func(t *testing.T) {
		data := &entity.ExptAggregateResult{
			ExperimentID:      200,
			EvaluatorResults:  map[int64]*entity.EvaluatorAggregateResult{},
			AnnotationResults: map[int64]*entity.AnnotationAggregateResult{},
		}
		dto := ExptAggregateResultDOToDTO(data)
		assert.NotNil(t, dto)
		assert.Nil(t, dto.UpdateTime)
	})
}

func TestEvaluatorResultsDOToDTO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, EvaluatorResultsDOToDTO(nil))
	})

	t.Run("normal input", func(t *testing.T) {
		result := &entity.EvaluatorAggregateResult{
			EvaluatorVersionID: 10,
			AggregatorResults: []*entity.AggregatorResult{
				{AggregatorType: entity.Average, Data: &entity.AggregateData{Value: gptr.Of(0.5)}},
			},
			Name:    gptr.Of("eval"),
			Version: gptr.Of("v1"),
		}
		dto := EvaluatorResultsDOToDTO(result)
		assert.NotNil(t, dto)
		assert.Equal(t, int64(10), dto.EvaluatorVersionID)
		assert.Equal(t, "eval", *dto.Name)
		assert.Equal(t, "v1", *dto.Version)
		assert.Len(t, dto.AggregatorResults, 1)
	})
}

func TestAnnotationResultDOToDTO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, AnnotationResultDOToDTO(nil))
	})

	t.Run("normal input", func(t *testing.T) {
		result := &entity.AnnotationAggregateResult{
			TagKeyID: 20,
			AggregatorResults: []*entity.AggregatorResult{
				{AggregatorType: entity.Sum, Data: &entity.AggregateData{Value: gptr.Of(3.0)}},
			},
			Name: gptr.Of("tag"),
		}
		dto := AnnotationResultDOToDTO(result)
		assert.NotNil(t, dto)
		assert.Equal(t, int64(20), dto.TagKeyID)
		assert.Equal(t, "tag", *dto.Name)
	})
}

func TestAggregatorResultDOsToDTOs(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, AggregatorResultDOsToDTOs(nil))
	})

	t.Run("empty input", func(t *testing.T) {
		assert.Nil(t, AggregatorResultDOsToDTOs([]*entity.AggregatorResult{}))
	})

	t.Run("normal input", func(t *testing.T) {
		results := []*entity.AggregatorResult{
			{AggregatorType: entity.Average, Data: &entity.AggregateData{Value: gptr.Of(1.0)}},
			nil,
		}
		dtos := AggregatorResultDOsToDTOs(results)
		assert.Len(t, dtos, 2)
		assert.NotNil(t, dtos[0])
		assert.Nil(t, dtos[1])
	})
}

func TestAggregatorResultDOToDTO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, AggregatorResultDOToDTO(nil))
	})

	t.Run("normal input", func(t *testing.T) {
		result := &entity.AggregatorResult{
			AggregatorType: entity.Average,
			Data:           &entity.AggregateData{Value: gptr.Of(0.75)},
		}
		dto := AggregatorResultDOToDTO(result)
		assert.NotNil(t, dto)
		assert.Equal(t, 0.75, *dto.Data.Value)
	})
}

func TestAggregateDataDOToDTO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, AggregateDataDOToDTO(nil))
	})

	t.Run("with value only", func(t *testing.T) {
		data := &entity.AggregateData{
			Value: gptr.Of(0.123456),
		}
		dto := AggregateDataDOToDTO(data)
		assert.NotNil(t, dto)
		assert.Equal(t, 0.12, *dto.Value)
	})

	t.Run("with score distribution", func(t *testing.T) {
		data := &entity.AggregateData{
			ScoreDistribution: &entity.ScoreDistributionData{
				ScoreDistributionItems: []*entity.ScoreDistributionItem{
					{Score: "1.0", Count: 5, Percentage: 0.5},
					nil,
					{Score: "2.0", Count: 5, Percentage: 0.5},
				},
			},
		}
		dto := AggregateDataDOToDTO(data)
		assert.NotNil(t, dto)
		assert.NotNil(t, dto.ScoreDistribution)
		assert.Len(t, dto.ScoreDistribution.ScoreDistributionItems, 2)
	})

	t.Run("with option distribution", func(t *testing.T) {
		data := &entity.AggregateData{
			OptionDistribution: &entity.OptionDistributionData{
				OptionDistributionItems: []*entity.OptionDistributionItem{
					{Option: "A", Count: 3, Percentage: 0.3},
					nil,
					{Option: "B", Count: 7, Percentage: 0.7},
				},
			},
		}
		dto := AggregateDataDOToDTO(data)
		assert.NotNil(t, dto)
		assert.NotNil(t, dto.OptionDistribution)
		assert.Len(t, dto.OptionDistribution.OptionDistributionItems, 2)
	})

	t.Run("nil value", func(t *testing.T) {
		data := &entity.AggregateData{}
		dto := AggregateDataDOToDTO(data)
		assert.NotNil(t, dto)
		assert.Nil(t, dto.Value)
	})
}

func TestScoreDistributionItemsDOToDTO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, ScoreDistributionItemsDOToDTO(nil))
	})

	t.Run("empty input", func(t *testing.T) {
		assert.Nil(t, ScoreDistributionItemsDOToDTO([]*entity.ScoreDistributionItem{}))
	})

	t.Run("normal input with nil", func(t *testing.T) {
		items := []*entity.ScoreDistributionItem{
			{Score: "1.0", Count: 10, Percentage: 0.5},
			nil,
		}
		dtos := ScoreDistributionItemsDOToDTO(items)
		assert.Len(t, dtos, 1)
		assert.Equal(t, "1.0", dtos[0].Score)
	})
}

func TestOptionDistributionItemsDOToDTO(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		assert.Nil(t, OptionDistributionItemsDOToDTO(nil))
	})

	t.Run("empty input", func(t *testing.T) {
		assert.Nil(t, OptionDistributionItemsDOToDTO([]*entity.OptionDistributionItem{}))
	})

	t.Run("normal input with nil", func(t *testing.T) {
		items := []*entity.OptionDistributionItem{
			{Option: "yes", Count: 5, Percentage: 0.5},
			nil,
		}
		dtos := OptionDistributionItemsDOToDTO(items)
		assert.Len(t, dtos, 1)
		assert.Equal(t, "yes", dtos[0].Option)
	})
}
