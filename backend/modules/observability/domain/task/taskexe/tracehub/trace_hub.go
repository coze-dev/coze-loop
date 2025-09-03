// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"time"

	config "github.com/coze-dev/coze-loop/backend/modules/data/domain/component/conf"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/collector/consumer"
	goredis "github.com/redis/go-redis/v9"
)

type TraceHub struct {
	c        consumer.Consumer
	cfg      *config.ConsumerConfig
	redis    *goredis.Client
	ticker   *time.Ticker
	stopChan chan struct{}
}

type ITaskEvent interface {
	TraceHub(ctx context.Context, event *entity.TaskEvent) error
}

func NewTraceHub(redisCli *goredis.Client, cfg *config.ConsumerConfig) (*TraceHub, error) {
	return nil, nil
}
