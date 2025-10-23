// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package evaluator

import (
	"strconv"
	"sync"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/metrics"
	eval_metrics "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
)

const (
	evaluatorMtrName = "evaluator"

	runSuffix    = "run"
	createSuffix = "create"

	throughputSuffix = ".throughput"
	latencySuffix    = ".latency"
)

const (
	tagSpaceID = "space_id"
	tagIsErr   = "is_error"
	tagCode    = "code"
	tagModelID = "model_id"
)

var (
	evaluatorMetricsOnce = sync.Once{}
	evaluatorMetricsImpl eval_metrics.EvaluatorExecMetrics
)

func evaluatorEvalMtrTags() []string {
	return []string{
		tagSpaceID,
		tagIsErr,
		tagCode,
		tagModelID,
	}
}

func NewEvaluatorMetrics(meter metrics.Meter) eval_metrics.EvaluatorExecMetrics {
	evaluatorMetricsOnce.Do(func() {
		if meter == nil {
			return
		}
		metric, err := meter.NewMetric(evaluatorMtrName, []metrics.MetricType{metrics.MetricTypeCounter, metrics.MetricTypeTimer}, evaluatorEvalMtrTags())
		if err != nil {
			return
		}
		evaluatorMetricsImpl = &EvaluatorExecMetricsImpl{metric: metric}
	})
	return evaluatorMetricsImpl
}

type EvaluatorExecMetricsImpl struct {
	metric metrics.Metric
}

func (e *EvaluatorExecMetricsImpl) EmitRun(spaceID int64, err error, start time.Time, modelID string) {
	if e == nil || e.metric == nil {
		return
	}
	code, isError := eval_metrics.GetCode(err)
	e.metric.Emit([]metrics.T{
		{Name: tagSpaceID, Value: strconv.FormatInt(spaceID, 10)},
		{Name: tagIsErr, Value: strconv.FormatInt(isError, 10)},
		{Name: tagCode, Value: strconv.FormatInt(code, 10)},
		{Name: tagModelID, Value: modelID},
	}, metrics.Counter(1, metrics.WithSuffix(runSuffix+throughputSuffix)),
		metrics.Timer(int64(time.Since(start).Seconds()), metrics.WithSuffix(runSuffix+latencySuffix)))
}

func (e *EvaluatorExecMetricsImpl) EmitCreate(spaceID int64, err error) {
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
