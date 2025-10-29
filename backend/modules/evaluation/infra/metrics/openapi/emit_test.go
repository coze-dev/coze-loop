// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package openapi

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/metrics"
	"github.com/coze-dev/coze-loop/backend/infra/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func TestEvaluationSetMetricsImpl_EmitCreate(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetric := mocks.NewMockMetric(ctrl)
	metricsImpl := &OpenAPIEvaluationMetricsImpl{metric: mockMetric}

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
					gomock.Any(),
				).Times(1)
			},
		},
		{
			name:    "create with error",
			spaceID: 456,
			err:     errorx.NewByCode(1001),
			setup: func() {
				mockMetric.EXPECT().Emit(
					gomock.Any(),
					gomock.Any(),
				).Times(1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			metricsImpl.EmitOpenAPIMetric(context.Background(), tt.spaceID, 0, "", 0, tt.err)
		})
	}
}

func TestNewOpenAPIEvaluationMetrics(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name  string
		meter metrics.Meter
		want  *OpenAPIEvaluationMetricsImpl
	}{
		{
			name:  "nil meter",
			meter: nil,
			want:  nil,
		},
		{
			name:  "meter",
			meter: metrics.GetMeter(),
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewEvaluationOApiMetrics(tt.meter)
			if tt.want == nil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.IsType(t, &OpenAPIEvaluationMetricsImpl{}, got)
			}
		})
	}
}
