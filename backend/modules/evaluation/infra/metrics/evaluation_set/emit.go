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

func (m *OpenAPIEvaluationSetMetricsImpl) EmitCreateEvaluationSet(spaceID int64, evaluationSetID *int64, err error) {
	if m == nil || m.meter == nil {
		return
	}
	
	metric, mErr := m.meter.NewMetric("openapi_evaluation_set_create", []metrics.MetricType{metrics.MetricTypeCounter}, []string{"space_id", "evaluation_set_id", "method", "status"})
	if mErr != nil {
		return
	}
	
	tags := []metrics.T{
		{Name: "space_id", Value: strconv.FormatInt(spaceID, 10)},
		{Name: "method", Value: "create_evaluation_set"},
	}
	
	if evaluationSetID != nil {
		tags = append(tags, metrics.T{Name: "evaluation_set_id", Value: strconv.FormatInt(*evaluationSetID, 10)})
	} else {
		tags = append(tags, metrics.T{Name: "evaluation_set_id", Value: ""})
	}
	
	if err != nil {
		tags = append(tags, metrics.T{Name: "status", Value: "error"})
	} else {
		tags = append(tags, metrics.T{Name: "status", Value: "success"})
	}
	
	metric.Emit(tags, metrics.Counter(1))
}

func (m *OpenAPIEvaluationSetMetricsImpl) EmitGetEvaluationSet(spaceID int64, evaluationSetID int64, err error) {
	if m == nil || m.meter == nil {
		return
	}
	
	metric, mErr := m.meter.NewMetric("openapi_evaluation_set_get", []metrics.MetricType{metrics.MetricTypeCounter}, []string{"space_id", "evaluation_set_id", "method", "status"})
	if mErr != nil {
		return
	}
	
	tags := []metrics.T{
		{Name: "space_id", Value: strconv.FormatInt(spaceID, 10)},
		{Name: "evaluation_set_id", Value: strconv.FormatInt(evaluationSetID, 10)},
		{Name: "method", Value: "get_evaluation_set"},
	}
	
	if err != nil {
		tags = append(tags, metrics.T{Name: "status", Value: "error"})
	} else {
		tags = append(tags, metrics.T{Name: "status", Value: "success"})
	}
	
	metric.Emit(tags, metrics.Counter(1))
}

func (m *OpenAPIEvaluationSetMetricsImpl) EmitListEvaluationSets(spaceID int64, err error) {
	if m == nil || m.meter == nil {
		return
	}
	
	metric, mErr := m.meter.NewMetric("openapi_evaluation_set_list", []metrics.MetricType{metrics.MetricTypeCounter}, []string{"space_id", "method", "status"})
	if mErr != nil {
		return
	}
	
	tags := []metrics.T{
		{Name: "space_id", Value: strconv.FormatInt(spaceID, 10)},
		{Name: "method", Value: "list_evaluation_sets"},
	}
	
	if err != nil {
		tags = append(tags, metrics.T{Name: "status", Value: "error"})
	} else {
		tags = append(tags, metrics.T{Name: "status", Value: "success"})
	}
	
	metric.Emit(tags, metrics.Counter(1))
}