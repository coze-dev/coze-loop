// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mq

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
)

//go:generate mockgen -destination=mocks/metric_producer.go -package=mocks . IMetricProducer
type IMetricProducer interface {
	EmitMetrics(ctx context.Context, events []*entity.MetricEvent) error
	Close() error
}
