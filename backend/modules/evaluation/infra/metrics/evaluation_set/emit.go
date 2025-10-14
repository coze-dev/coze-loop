// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"strconv"
	"time"

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

func NewOpenAPIEvaluationSetMetrics(meter metrics.Meter) eval_metrics.OpenAPIEvaluationSetMetrics {
	return &OpenAPIEvaluationSetMetricsImpl{
		meter: meter,
	}
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

type OpenAPIEvaluationSetMetricsImpl struct {
	meter metrics.Meter
}

func (m *OpenAPIEvaluationSetMetricsImpl) EmitOpenAPIMetric(ctx context.Context, spaceID, evaluationSetID int64, method string, startTime int64, err error) {
	if m == nil || m.meter == nil {
		return
	}
	
	metric, mErr := m.meter.NewMetric("openapi_evaluation_set", []metrics.MetricType{metrics.MetricTypeCounter, metrics.MetricTypeTimer}, []string{"space_id", "evaluation_set_id", "method", "is_error", "code"})
	if mErr != nil {
		return
	}
	
	code, isError := eval_metrics.GetCode(err)
	
	tags := []metrics.T{
		{Name: "space_id", Value: strconv.FormatInt(spaceID, 10)},
		{Name: "evaluation_set_id", Value: strconv.FormatInt(evaluationSetID, 10)},
		{Name: "method", Value: method},
		{Name: "is_error", Value: strconv.FormatInt(isError, 10)},
		{Name: "code", Value: strconv.FormatInt(code, 10)},
	}
	
	// 记录调用次数
	metric.Emit(tags, metrics.Counter(1))
	
	// 记录响应时间
	if startTime > 0 {
		responseTime := time.Now().UnixNano()/int64(time.Millisecond) - startTime
		metric.Emit(tags, metrics.Timer(responseTime))
	}
}