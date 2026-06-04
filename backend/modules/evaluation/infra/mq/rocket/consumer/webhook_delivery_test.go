// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func TestBuildWebhookPayloadIncludesProgressMetricsAndResultURL(t *testing.T) {
	avg := 0.8
	min := 0.5
	max := 0.95
	evaluatorName := "quality"
	expt := &entity.Experiment{
		ID:      123,
		SpaceID: 456,
		Name:    "webhook experiment",
		Status:  entity.ExptStatus_Success,
		Stats: &entity.ExptStats{
			PendingItemCnt:    1,
			SuccessItemCnt:    6,
			FailItemCnt:       2,
			ProcessingItemCnt: 3,
			TerminatedItemCnt: 4,
		},
		AggregateResult: &entity.ExptAggregateResult{
			WeightedResults: []*entity.AggregatorResult{
				{AggregatorType: entity.Average, Data: &entity.AggregateData{Value: &avg}},
				{AggregatorType: entity.Min, Data: &entity.AggregateData{Value: &min}},
				{AggregatorType: entity.Max, Data: &entity.AggregateData{Value: &max}},
			},
			EvaluatorResults: map[int64]*entity.EvaluatorAggregateResult{
				9: {
					EvaluatorID: 9,
					Name:        &evaluatorName,
					AggregatorResults: []*entity.AggregatorResult{
						{AggregatorType: entity.Average, Data: &entity.AggregateData{Value: &avg}},
					},
				},
			},
		},
	}
	event := &entity.WebhookDeliveryMessage{
		DeliveryID: "delivery-123",
		SpaceID:    456,
		EventType:  entity.WebhookEventSucceeded,
	}
	conf := &entity.WebhookGlobalConf{
		ResultURLTemplate: "https://loop.example.com/evaluation/experiments/{experiment_id}?workspace_id={space_id}",
	}

	payload := buildWebhookPayload(event, expt, conf)
	require.NotNil(t, payload)
	require.NotNil(t, payload.Experiment)
	assert.Equal(t, "delivery-123", payload.DeliveryID)
	assert.Equal(t, entity.WebhookEventSucceeded, payload.Event)
	assert.Equal(t, "123", payload.Experiment.ID)
	assert.Equal(t, "webhook experiment", payload.Experiment.Name)
	assert.Equal(t, "success", payload.Experiment.Status)
	assert.Equal(t, 16, payload.Experiment.Progress.Total)
	assert.Equal(t, 6, payload.Experiment.Progress.Succeeded)
	assert.Equal(t, 2, payload.Experiment.Progress.Failed)
	assert.Equal(t, 3, payload.Experiment.Progress.Processing)
	require.NotNil(t, payload.Experiment.ResultURL)
	assert.Equal(t, "https://loop.example.com/evaluation/experiments/123?workspace_id=456", *payload.Experiment.ResultURL)
	require.NotNil(t, payload.Experiment.Metrics)
	require.NotNil(t, payload.Experiment.Metrics.OverallScore)
	assert.InDelta(t, avg, *payload.Experiment.Metrics.OverallScore.Avg, 0.0001)
	assert.InDelta(t, min, *payload.Experiment.Metrics.OverallScore.Min, 0.0001)
	assert.InDelta(t, max, *payload.Experiment.Metrics.OverallScore.Max, 0.0001)
	require.Len(t, payload.Experiment.Metrics.EvaluatorMetrics, 1)
	assert.Equal(t, "9", payload.Experiment.Metrics.EvaluatorMetrics[0].EvaluatorID)
	assert.Equal(t, evaluatorName, payload.Experiment.Metrics.EvaluatorMetrics[0].EvaluatorName)
}
