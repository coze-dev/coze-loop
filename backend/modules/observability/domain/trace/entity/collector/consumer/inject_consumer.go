package consumer

import "context"

type injectConsumer struct {
	inner Consumer
}

func NewInjectConsumer(inner Consumer) Consumer {
	return &injectConsumer{inner: inner}
}

func (c *injectConsumer) ConsumeTraces(ctx context.Context, tds Traces) error {
	ctx = NewSpanStatsContext(ctx)
	InjectSpanCounts(ctx, tds)
	return c.inner.ConsumeTraces(ctx, tds)
}
