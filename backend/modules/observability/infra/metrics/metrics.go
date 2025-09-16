// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"strconv"
	"sync"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/metrics"
	metrics2 "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

const (
	traceSpansMetricsName = "trace_spans"

	getTraceSuffix  = "get_trace"
	listSpansSuffix = "list_spans"
	traceOApiSuffix = "trace_oapi"

	throughputSuffix = ".throughput"
	latencySuffix    = ".latency"
	sizeSuffix       = ".size"
)

const (
	tagMethod       = "method"
	tagSpaceID      = "workspace_id"
	tagPlatformType = "platform_type"
	tagSpanType     = "span_list_type"
	tagIsErr        = "is_err"
	tagErrCode      = "err_code"
)

func traceQueryTagNames() []string {
	return []string{
		tagMethod,
		tagSpaceID,
		tagPlatformType,
		tagSpanType,
		tagIsErr,
		tagErrCode,
	}
}

var (
	traceMetricsOnce      sync.Once
	singletonTraceMetrics metrics2.ITraceMetrics
)

func NewTraceMetricsImpl(meter metrics.Meter) metrics2.ITraceMetrics {
	traceMetricsOnce.Do(func() {
		if meter == nil {
			return
		}
		spansMetrics, err := meter.NewMetric(traceSpansMetricsName, []metrics.MetricType{metrics.MetricTypeCounter, metrics.MetricTypeTimer}, traceQueryTagNames())
		if err != nil {
			logs.Error("Failed to create trace metrics: %v", err)
			return
		}
		singletonTraceMetrics = &TraceMetricsImpl{
			spansMetrics: spansMetrics,
		}
	})
	if singletonTraceMetrics != nil {
		return singletonTraceMetrics
	} else {
		return &TraceMetricsImpl{} // not expected to be here
	}
}

type TraceMetricsImpl struct {
	spansMetrics metrics.Metric
}

func (t *TraceMetricsImpl) EmitListSpans(workspaceId int64, spanType string, start time.Time, isError bool) {
	if t.spansMetrics == nil {
		return
	}
	t.spansMetrics.Emit(
		[]metrics.T{
			{Name: tagSpaceID, Value: strconv.FormatInt(workspaceId, 10)},
			{Name: tagIsErr, Value: strconv.FormatBool(isError)},
			{Name: tagSpanType, Value: spanType},
		},
		metrics.Counter(1, metrics.WithSuffix(listSpansSuffix+throughputSuffix)),
		metrics.Timer(time.Since(start).Microseconds(), metrics.WithSuffix(listSpansSuffix+latencySuffix)))
}

func (t *TraceMetricsImpl) EmitGetTrace(workspaceId int64, start time.Time, isError bool) {
	if t.spansMetrics == nil {
		return
	}
	t.spansMetrics.Emit(
		[]metrics.T{
			{Name: tagSpaceID, Value: strconv.FormatInt(workspaceId, 10)},
			{Name: tagIsErr, Value: strconv.FormatBool(isError)},
		},
		metrics.Counter(1, metrics.WithSuffix(getTraceSuffix+throughputSuffix)),
		metrics.Timer(time.Since(start).Microseconds(), metrics.WithSuffix(getTraceSuffix+latencySuffix)))
}

func (t *TraceMetricsImpl) EmitTraceOapi(method string, workspaceId int64, platformType, spanListType string, spanSize int64, errorCode int, start time.Time, isError bool) {
	if t.spansMetrics == nil {
		return
	}
	t.spansMetrics.Emit(
		[]metrics.T{
			{Name: tagMethod, Value: method},
			{Name: tagSpaceID, Value: strconv.FormatInt(workspaceId, 10)},
			{Name: tagIsErr, Value: strconv.FormatBool(isError)},
			{Name: tagPlatformType, Value: platformType},
			{Name: tagSpanType, Value: spanListType},
			{Name: tagErrCode, Value: strconv.Itoa(errorCode)},
		},
		metrics.Counter(1, metrics.WithSuffix(traceOApiSuffix+throughputSuffix)),
		metrics.Counter(spanSize, metrics.WithSuffix(traceOApiSuffix+sizeSuffix)),
		metrics.Timer(time.Since(start).Microseconds(), metrics.WithSuffix(traceOApiSuffix+latencySuffix)))
}
