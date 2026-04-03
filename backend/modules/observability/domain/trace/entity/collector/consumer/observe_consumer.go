package consumer

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/metrics"
	obmetrics "github.com/coze-dev/coze-loop/backend/modules/observability/infra/metrics"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type ObserveConsumer struct {
	name        string
	inner       Consumer
	nextElapsed *atomic.Int64
	metric      metrics.Metric
}

func NewObserveConsumer(name string, inner Consumer, nextElapsed *atomic.Int64, metric metrics.Metric) Consumer {
	return &ObserveConsumer{
		name:        name,
		inner:       inner,
		nextElapsed: nextElapsed,
		metric:      metric,
	}
}

func (t *ObserveConsumer) ConsumeTraces(ctx context.Context, tds Traces) error {
	if t.nextElapsed != nil {
		t.nextElapsed.Store(0)
	}

	start := time.Now()
	err := t.inner.ConsumeTraces(ctx, tds)
	total := time.Since(start)

	var selfDuration time.Duration
	if t.nextElapsed != nil {
		selfDuration = total - time.Duration(t.nextElapsed.Load())
	} else {
		selfDuration = total
	}

	isErr := err != nil
	if t.metric != nil {
		logs.CtxInfo(ctx, "ObserveConsumer[%s] ConsumeTraces, self_duration=%s, is_err=%s, spans_count=%d", t.name, selfDuration, boolToStr(isErr), tds.SpansCount())
		psmCounts := tds.SpansCountByPSM()
		for psm, count := range psmCounts {
			t.metric.Emit(
				[]metrics.T{
					{Name: obmetrics.ConsumeTagNode, Value: t.name},
					{Name: obmetrics.ConsumeTagIsErr, Value: boolToStr(isErr)},
					{Name: obmetrics.ConsumeTagPSM, Value: psm},
					{Name: obmetrics.ConsumeTagTenant, Value: tds.Tenant},
				},
				metrics.Counter(1, metrics.WithSuffix(obmetrics.ConsumeSuffixThroughput)),
				metrics.Timer(selfDuration.Microseconds(), metrics.WithSuffix(obmetrics.ConsumeSuffixLatency)),
				metrics.Counter(int64(count), metrics.WithSuffix(obmetrics.ConsumeSuffixSpans)),
			)
		}
	}

	if err != nil {
		logs.CtxWarn(ctx, "ObserveConsumer[%s] ConsumeTraces failed, self_duration=%s, err=%v", t.name, selfDuration, err)
	}
	return err
}

type stopwatchConsumer struct {
	inner   Consumer
	elapsed *atomic.Int64
}

func NewStopwatchConsumer(inner Consumer, elapsed *atomic.Int64) Consumer {
	return &stopwatchConsumer{
		inner:   inner,
		elapsed: elapsed,
	}
}

func (s *stopwatchConsumer) ConsumeTraces(ctx context.Context, tds Traces) error {
	start := time.Now()
	err := s.inner.ConsumeTraces(ctx, tds)
	s.elapsed.Add(time.Since(start).Nanoseconds())
	return err
}

func boolToStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
