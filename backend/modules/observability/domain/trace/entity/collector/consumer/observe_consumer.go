package consumer

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/metrics"
	obmetrics "github.com/coze-dev/coze-loop/backend/modules/observability/infra/metrics"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type elapsedCtxKey struct{}

type ObserveConsumer struct {
	name      string
	inner     Consumer
	trackSelf bool
	metric    metrics.Metric
}

func NewObserveConsumer(name string, inner Consumer, trackSelf bool, metric metrics.Metric) Consumer {
	return &ObserveConsumer{
		name:      name,
		inner:     inner,
		trackSelf: trackSelf,
		metric:    metric,
	}
}

func (t *ObserveConsumer) ConsumeTraces(ctx context.Context, tds Traces) error {
	var elapsed *atomic.Int64
	if t.trackSelf {
		elapsed = &atomic.Int64{}
		ctx = context.WithValue(ctx, elapsedCtxKey{}, elapsed)
	}

	start := time.Now()
	err := t.inner.ConsumeTraces(ctx, tds)
	total := time.Since(start)

	var selfDuration time.Duration
	if elapsed != nil {
		selfDuration = total - time.Duration(elapsed.Load())
	} else {
		selfDuration = total
	}

	isErr := err != nil
	if t.metric != nil {
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
	inner Consumer
}

func NewStopwatchConsumer(inner Consumer) Consumer {
	return &stopwatchConsumer{
		inner: inner,
	}
}

func (s *stopwatchConsumer) ConsumeTraces(ctx context.Context, tds Traces) error {
	start := time.Now()
	err := s.inner.ConsumeTraces(ctx, tds)
	if elapsed, ok := ctx.Value(elapsedCtxKey{}).(*atomic.Int64); ok && elapsed != nil {
		elapsed.Add(time.Since(start).Nanoseconds())
	}
	return err
}

func boolToStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}
