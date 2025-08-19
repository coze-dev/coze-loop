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
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

var (
	annotationProducerOnce      sync.Once
	singletonAnnotationProducer mq2.IAnnotationProducer
)

type AnnotationProducerImpl struct {
	topic      string
	mqProducer mq.IProducer
}

func (a *AnnotationProducerImpl) SendAnnotation(ctx context.Context, message *entity.AnnotationEvent) error {
	bytes, err := json.Marshal(message)
	if err != nil {
		return errorx.WrapByCode(err, obErrorx.CommercialCommonInternalErrorCodeCode)
	}
	msg := mq.NewDeferMessage(a.topic, 10*time.Second, bytes)
	_, err = a.mqProducer.Send(ctx, msg)
	if err != nil {
		logs.CtxWarn(ctx, "send annotation msg err: %v", err)
		return errorx.WrapByCode(err, obErrorx.CommercialCommonRPCErrorCodeCode)
	}
	logs.CtxInfo(ctx, "send annotation msg %s successfully", string(bytes))
	return nil
}

func NewAnnotationProducerImpl(traceConfig config.ITraceConfig, mqFactory mq.IFactory) (mq2.IAnnotationProducer, error) {
	var err error
	annotationProducerOnce.Do(func() {
		singletonAnnotationProducer, err = newAnnotationProducerImpl(traceConfig, mqFactory)
	})
	if err != nil {
		return nil, err
	} else {
		return singletonAnnotationProducer, nil
	}
}

func newAnnotationProducerImpl(traceConfig config.ITraceConfig, mqFactory mq.IFactory) (mq2.IAnnotationProducer, error) {
	mqCfg, err := traceConfig.GetAnnotationMqProducerCfg(context.Background())
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
	return &AnnotationProducerImpl{
		topic:      mqCfg.Topic,
		mqProducer: mqProducer,
	}, nil
}
