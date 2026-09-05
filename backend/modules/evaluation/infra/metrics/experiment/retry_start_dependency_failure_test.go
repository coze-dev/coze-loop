// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"testing"

	imetrics "github.com/coze-dev/coze-loop/backend/infra/metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type retryStartDependencyFailureCaptureMetric struct {
	tags   []imetrics.T
	values []*imetrics.Value
}

func (m *retryStartDependencyFailureCaptureMetric) Emit(tags []imetrics.T, values ...*imetrics.Value) {
	m.tags = append([]imetrics.T(nil), tags...)
	m.values = append([]*imetrics.Value(nil), values...)
}

func TestRetryStartDependencyFailureMetricDefinition(t *testing.T) {
	assert.Equal(t, "expt_retry_start_dependency_failure", retryStartDependencyFailureMtrName)
	assert.Equal(t, []string{"space_id", "dependency"}, retryStartDependencyFailureTags())
}

func TestExperimentMetricImpl_EmitRetryStartDependencyFailure(t *testing.T) {
	metric := &retryStartDependencyFailureCaptureMetric{}
	impl := ExperimentMetricImpl{retryStartDependencyFailureMtr: metric}

	impl.EmitRetryStartDependencyFailure(123, "eval_target_service")

	assert.Equal(t, []imetrics.T{
		{Name: "space_id", Value: "123"},
		{Name: "dependency", Value: "eval_target_service"},
	}, metric.tags)
	require.Len(t, metric.values, 1)
	assert.Equal(t, imetrics.MetricTypeCounter, metric.values[0].GetType())
	require.NotNil(t, metric.values[0].GetValue())
	assert.Equal(t, int64(1), *metric.values[0].GetValue())
	assert.Equal(t, ".throughput", metric.values[0].GetSuffix())
}
