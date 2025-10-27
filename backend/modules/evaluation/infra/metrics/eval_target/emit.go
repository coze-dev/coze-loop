// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"strconv"
	"sync"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/metrics"
	eval_metrics "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
)

const (
	evalTargetMtrName = "evaluation_target"

	runSuffix    = "run"
	createSuffix = "create"

	throughputSuffix = ".throughput"
	latencySuffix    = ".latency"
)

const (
	tagSpaceID = "space_id"
	tagIsErr   = "is_error"
	tagCode    = "code"
)

func evalTargetEvalMtrTags() []string {
	return []string{
		tagSpaceID,
		tagIsErr,
		tagCode,
	}
}

var (
	evalTargetMetricsOnce = sync.Once{}
	evalTargetMetricsImpl eval_metrics.EvalTargetMetrics
)

func NewEvalTargetMetrics(meter metrics.Meter) eval_metrics.EvalTargetMetrics {
	evalTargetMetricsOnce.Do(func() {
		if meter == nil {
			return
		}
		metric, err := meter.NewMetric(evalTargetMtrName, []metrics.MetricType{metrics.MetricTypeCounter, metrics.MetricTypeTimer}, evalTargetEvalMtrTags())
		if err != nil {
			return
		}
		evalTargetMetricsImpl = &EvalTargetMetricsImpl{metric: metric}
	})
	return evalTargetMetricsImpl
}

type EvalTargetMetricsImpl struct {
	metric metrics.Metric
}

func (e *EvalTargetMetricsImpl) EmitRun(spaceID int64, err error, start time.Time) {
	if e == nil || e.metric == nil {
		return
	}
	code, isError := eval_metrics.GetCode(err)
	e.metric.Emit([]metrics.T{
		{Name: tagSpaceID, Value: strconv.FormatInt(spaceID, 10)},
		{Name: tagIsErr, Value: strconv.FormatInt(isError, 10)},
		{Name: tagCode, Value: strconv.FormatInt(code, 10)},
	}, metrics.Counter(1, metrics.WithSuffix(runSuffix+throughputSuffix)),
		metrics.Timer(int64(time.Since(start).Seconds()), metrics.WithSuffix(runSuffix+latencySuffix)))
}

func (e *EvalTargetMetricsImpl) EmitCreate(spaceID int64, err error) {
	if e.metric == nil {
		return
	}
	code, isError := eval_metrics.GetCode(err)
	e.metric.Emit([]metrics.T{
		{Name: tagSpaceID, Value: strconv.FormatInt(spaceID, 10)},
		{Name: tagIsErr, Value: strconv.FormatInt(isError, 10)},
		{Name: tagCode, Value: strconv.FormatInt(code, 10)},
	}, metrics.Counter(1, metrics.WithSuffix(createSuffix+throughputSuffix)))
}
