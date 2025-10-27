// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package evaluator

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/metrics"
	"github.com/coze-dev/coze-loop/backend/infra/metrics/mocks"
)

func TestEvaluatorExecMetricsImpl_EmitRun(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetric := mocks.NewMockMetric(ctrl)
	metricsImpl := &EvaluatorExecMetricsImpl{metric: mockMetric}

	tests := []struct {
		name    string
		spaceID int64
		err     error
		start   time.Time
		modelID string
		setup   func()
	}{
		{
			name:    "successful run",
			spaceID: 123,
			err:     nil,
			start:   time.Now().Add(-time.Second),
			modelID: "modelA",
			setup: func() {
				mockMetric.EXPECT().Emit(
					gomock.Any(),
					metrics.Counter(1, metrics.WithSuffix(runSuffix+throughputSuffix)),
					gomock.Any(),
				).Times(1)
			},
		},
		{
			name:    "run with error",
			spaceID: 456,
			err:     assert.AnError,
			start:   time.Now().Add(-time.Second),
			modelID: "modelB",
			setup: func() {
				mockMetric.EXPECT().Emit(
					gomock.Any(),
					metrics.Counter(1, metrics.WithSuffix(runSuffix+throughputSuffix)),
					gomock.Any(),
				).Times(1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			metricsImpl.EmitRun(tt.spaceID, tt.err, tt.start, tt.modelID)
		})
	}
}

func TestEvaluatorExecMetricsImpl_EmitCreate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetric := mocks.NewMockMetric(ctrl)
	metricsImpl := &EvaluatorExecMetricsImpl{metric: mockMetric}

	tests := []struct {
		name    string
		spaceID int64
		err     error
		setup   func()
	}{
		{
			name:    "successful create",
			spaceID: 123,
			err:     nil,
			setup: func() {
				mockMetric.EXPECT().Emit(
					gomock.Any(),
					metrics.Counter(1, metrics.WithSuffix(createSuffix+throughputSuffix)),
				).Times(1)
			},
		},
		{
			name:    "create with error",
			spaceID: 456,
			err:     assert.AnError,
			setup: func() {
				mockMetric.EXPECT().Emit(
					gomock.Any(),
					metrics.Counter(1, metrics.WithSuffix(createSuffix+throughputSuffix)),
				).Times(1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			metricsImpl.EmitCreate(tt.spaceID, tt.err)
		})
	}
}

func TestEvaluatorExecMetricsImpl_EmitRun_NilMetric(t *testing.T) {
	metricsImpl := &EvaluatorExecMetricsImpl{metric: nil}
	metricsImpl.EmitRun(123, nil, time.Now(), "modelX")
	// Should not panic
}

func TestEvaluatorExecMetricsImpl_EmitCreate_NilMetric(t *testing.T) {
	metricsImpl := &EvaluatorExecMetricsImpl{metric: nil}
	metricsImpl.EmitCreate(123, nil)
	// Should not panic
}

func TestNewEvaluatorMetrics(t *testing.T) {
	tests := []struct {
		name  string
		meter metrics.Meter
		want  interface{}
	}{
		{
			name:  "nil meter",
			meter: nil,
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewEvaluatorMetrics(tt.meter)
			if tt.want == nil {
				assert.Nil(t, got)
			} else {
				assert.IsType(t, tt.want, got)
			}
		})
	}
}

func TestEvaluatorMetrics_EvaluatorEvalMtrTags(t *testing.T) {
	got := evaluatorEvalMtrTags()
	assert.Equal(t, []string{
		tagSpaceID,
		tagIsErr,
		tagCode,
		tagModelID,
	}, got)
}
