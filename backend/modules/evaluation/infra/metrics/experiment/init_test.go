// Copyright (c) 2025 Bytedance Ltd. and/or its affiliates
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"

	"github.com/coze-dev/coze-loop/backend/infra/metrics"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestNewEvaluationSetMetrics(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		name  string
		meter metrics.Meter
		want  *ExperimentMetricImpl
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
			got := NewExperimentMetric(tt.meter)
			if tt.want == nil {
				assert.Nil(t, got)
			} else {
				assert.NotNil(t, got)
				assert.IsType(t, &ExperimentMetricImpl{}, got)
			}
		})
	}
}
