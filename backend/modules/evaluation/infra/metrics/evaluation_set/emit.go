// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"strconv"

	"github.com/coze-dev/coze-loop/backend/infra/metrics"
	eval_metrics "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
)

const (
	evaluationSetMtrName = "evaluation_set"
	createSuffix         = "create"
	throughputSuffix     = ".throughput"
)

const (
	tagSpaceID = "space_id"
	tagIsErr   = "is_error"
	tagCode    = "code"
)

func evaluationSetEvalMtrTags() []string {
	return []string{
		tagSpaceID,
		tagIsErr,
		tagCode,
	}
}

func NewEvaluationSetMetrics(meter metrics.Meter) eval_metrics.EvaluationSetMetrics {
	if meter == nil {
		return nil
	}
	metric, err := meter.NewMetric(evaluationSetMtrName, []metrics.MetricType{metrics.MetricTypeCounter, metrics.MetricTypeTimer}, evaluationSetEvalMtrTags())
	if err != nil {
		return nil
	}
	return &EvaluationSetMetricsImpl{metric: metric}
}

type EvaluationSetMetricsImpl struct {
	metric metrics.Metric
}

func (e *EvaluationSetMetricsImpl) EmitCreate(spaceID int64, err error) {
	if e == nil || e.metric == nil {
		return
	}
	code, isError := eval_metrics.GetCode(err)
	e.metric.Emit([]metrics.T{
		{Name: tagSpaceID, Value: strconv.FormatInt(spaceID, 10)},
		{Name: tagIsErr, Value: strconv.FormatInt(isError, 10)},
		{Name: tagCode, Value: strconv.FormatInt(code, 10)},
	}, metrics.Counter(1, metrics.WithSuffix(createSuffix+throughputSuffix)))
}
