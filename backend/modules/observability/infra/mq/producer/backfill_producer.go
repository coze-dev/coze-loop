// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	mq2 "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/mq"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

var (
	backfillProducerOnce      sync.Once
	singletonBackfillProducer mq2.IBackfillProducer
)

type BackfillProducerImpl struct {
	topic      string
	mqProducer mq.IProducer
}

func NewBackfillProducerImpl(traceConfig config.ITraceConfig, mqFactory mq.IFactory) (mq2.IBackfillProducer, error) {
	var err error
	backfillProducerOnce.Do(func() {
		singletonBackfillProducer, err = newBackfillProducerImpl(traceConfig, mqFactory)
	})
	if err != nil {
		return nil, err
	} else {
		return singletonBackfillProducer, nil
	}
}

func newBackfillProducerImpl(traceConfig config.ITraceConfig, mqFactory mq.IFactory) (mq2.IBackfillProducer, error) {
	mqCfg, err := traceConfig.GetBackfillMqProducerCfg(context.Background())
	if err != nil {
		return nil, err
	}
	if mqCfg.Topic == "" {
		return nil, fmt.Errorf("trace topic required")
	}
	mqProducer, err := mqFactory.NewProducer(mq.ProducerConfig{
		Addr:           mqCfg.Addr,
		ProduceTimeout: time.Duration(mqCfg.Timeout) * time.Millisecond,
		RetryTimes:     mqCfg.RetryTimes,
		ProducerGroup:  ptr.Of(mqCfg.ProducerGroup),
	})
	if err != nil {
		return nil, err
	}
	if err := mqProducer.Start(); err != nil {
		return nil, fmt.Errorf("fail to start producer, %v", err)
	}
	return &BackfillProducerImpl{
		topic:      mqCfg.Topic,
		mqProducer: mqProducer,
	}, nil
}
