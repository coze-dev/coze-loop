// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package openapi

import (
	"context"
	"strconv"
	"sync"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/metrics"
	eval_metrics "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
)

const (
	evaluationOApiMtrName = "evaluation_openapi"
	createSuffix          = "create"
	throughputSuffix      = ".throughput"
)

const (
	tagSpaceID  = "space_id"
	tagIsErr    = "is_error"
	tagCode     = "code"
	tagObjectID = "object_id"
	tagMethod   = "method"
)

func evaluationEvalMtrTags() []string {
	return []string{
		tagSpaceID,
		tagIsErr,
		tagCode,
		tagObjectID,
		tagMethod,
	}
}

var (
	evalOApiMetricsOnce = sync.Once{}
	evalOApiMetricsImpl eval_metrics.OpenAPIEvaluationMetrics
)

func NewEvaluationOApiMetrics(meter metrics.Meter) eval_metrics.OpenAPIEvaluationMetrics {
	evalOApiMetricsOnce.Do(func() {
		if meter == nil {
			return
		}
		metric, err := meter.NewMetric(evaluationOApiMtrName, []metrics.MetricType{metrics.MetricTypeCounter, metrics.MetricTypeTimer}, evaluationEvalMtrTags())
		if err != nil {
			return
		}
		evalOApiMetricsImpl = &OpenAPIEvaluationMetricsImpl{metric: metric}
	})
	return evalOApiMetricsImpl
}

type OpenAPIEvaluationMetricsImpl struct {
	metric metrics.Metric
}

func (m *OpenAPIEvaluationMetricsImpl) EmitOpenAPIMetric(ctx context.Context, spaceID, objectID int64, method string, startTime int64, err error) {
	if m == nil || m.metric == nil {
		return
	}

	code, isError := eval_metrics.GetCode(err)

	tags := []metrics.T{
		{Name: tagSpaceID, Value: strconv.FormatInt(spaceID, 10)},
		{Name: tagIsErr, Value: strconv.FormatInt(isError, 10)},
		{Name: tagCode, Value: strconv.FormatInt(code, 10)},
		{Name: tagObjectID, Value: strconv.FormatInt(objectID, 10)},
		{Name: tagMethod, Value: method},
	}

	// 记录调用次数
	m.metric.Emit(tags, metrics.Counter(1))

	// 记录响应时间
	if startTime > 0 {
		responseTime := time.Now().UnixNano()/int64(time.Millisecond) - startTime
		m.metric.Emit(tags, metrics.Timer(responseTime))
	}
}
