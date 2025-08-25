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
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

const (
	maxBatchSize = 1024 * 1024 * 10
)

var (
	traceProducerOnce      sync.Once
	singletonTraceProducer mq2.ITraceProducer
)

type producerProxy struct {
	traceTopic string
	mqProducer mq.IProducer
}

type TraceProducerImpl struct {
	producerProxy map[string]*producerProxy
}

func (t *TraceProducerImpl) IngestSpans(ctx context.Context, td *entity.TraceData) error {
	if t.producerProxy == nil || t.producerProxy[td.Tenant] == nil {
		return errorx.NewByCode(obErrorx.CommercialCommonInternalErrorCodeCode, errorx.WithExtraMsg("tenant producer not exist"))
	}
	producer := t.producerProxy[td.Tenant]
	payload, err := json.Marshal(td)
	if err != nil {
		return errorx.WrapByCode(err, obErrorx.CommercialCommonInternalErrorCodeCode, errorx.WithExtraMsg("trace data marshal failed"))
	}
	if len(payload) > maxBatchSize {
		if len(td.SpanList) == 1 {
			return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("span size too large"))
		}
		for _, span := range td.SpanList {
			if err := t.IngestSpans(ctx, &entity.TraceData{
				Tenant:     td.Tenant,
				TenantInfo: td.TenantInfo,
				SpanList:   []*loop_span.Span{span},
			}); err != nil {
				return err
			}
		}
	} else {
		msg := mq.NewMessage(producer.traceTopic, payload)
		if err := producer.mqProducer.SendAsync(ctx, func(ctx context.Context, sendResponse mq.SendResponse, err error) {
			if err != nil {
				logs.CtxWarn(ctx, "mq send error: %v", err)
			}
		}, msg); err != nil {
			return errorx.WrapByCode(err, obErrorx.CommercialCommonRPCErrorCodeCode)
		}
	}
	return nil
}

func NewTraceProducerImpl(traceConfig config.ITraceConfig, mqFactory mq.IFactory) (mq2.ITraceProducer, error) {
	var err error
	traceProducerOnce.Do(func() {
		singletonTraceProducer, err = newTraceProducerImpl(traceConfig, mqFactory)
	})
	if err != nil {
		return nil, err
	} else {
		return singletonTraceProducer, nil
	}
}

func newTraceProducerImpl(traceConfig config.ITraceConfig, mqFactory mq.IFactory) (mq2.ITraceProducer, error) {
	ingestTenantCfg, err := traceConfig.GetTraceIngestTenantProducerCfg(context.Background())
	if err != nil {
		return nil, err
	}
	impl := &TraceProducerImpl{
		producerProxy: make(map[string]*producerProxy),
	}
	for tenant, ingestCfg := range ingestTenantCfg {
		if ingestCfg == nil {
			continue
		}
		mqCfg := ingestCfg.MqProducer
		if mqCfg.Topic == "" {
			return nil, fmt.Errorf("trace topic required")
		}
		mqProducer, e := mqFactory.NewProducer(mq.ProducerConfig{
			Addr:           mqCfg.Addr,
			ProduceTimeout: time.Duration(mqCfg.Timeout) * time.Millisecond,
			RetryTimes:     mqCfg.RetryTimes,
			ProducerGroup:  ptr.Of(mqCfg.ProducerGroup),
		})
		if e != nil {
			return nil, e
		}
		if e = mqProducer.Start(); e != nil {
			return nil, fmt.Errorf("fail to start producer, %v", e)
		}
		impl.producerProxy[tenant] = &producerProxy{
			traceTopic: mqCfg.Topic,
			mqProducer: mqProducer,
		}
	}
	return impl, nil
}
