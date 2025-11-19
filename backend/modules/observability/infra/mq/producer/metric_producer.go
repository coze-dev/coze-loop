// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"context"
	"sync/atomic"

	mq2 "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/mq"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func NewMetricProducer() mq2.IMetricProducer {
	return new(NullMetricProducer)
}

type NullMetricProducer struct {
	count atomic.Int64
}

func (m *NullMetricProducer) EmitMetrics(ctx context.Context, events []*entity.MetricEvent) error {
	m.count.Add(int64(len(events)))
	logs.CtxInfo(ctx, "event count: %d", m.count.Load())
	return nil
}
