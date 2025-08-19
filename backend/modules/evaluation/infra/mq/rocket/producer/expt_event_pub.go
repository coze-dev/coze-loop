// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/samber/lo"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/mq/rocket"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

var (
	publisherSingleton events.ExptEventPublisher
	publisherOnce      sync.Once
)

func NewExptEventPublisher(ctx context.Context, cfgFactory conf.IConfigLoaderFactory, mqFactory mq.IFactory) (p events.ExptEventPublisher, err error) {
	publisherOnce.Do(func() {
		publisherSingleton, err = newExptEventPublisher(ctx, cfgFactory, mqFactory)
	})
	return publisherSingleton, err
}

func newExptEventPublisher(ctx context.Context, cfgFactory conf.IConfigLoaderFactory, mqFactory mq.IFactory) (events.ExptEventPublisher, error) {
	loader, err := cfgFactory.NewConfigLoader(consts.EvaluationConfigFileName)
	if err != nil {
		return nil, err
	}

	publisher := &exptEventPublisher{producers: make(map[string]*producer)}

	// return publisher, nil

	for _, key := range []string{
		rocket.ExptScheduleEventRMQKey,
		rocket.ExptRecordEvalEventRMQKey,
		rocket.ExptAggrCalculateEventRMQKey,
		rocket.ExptOnlineEvalResultRMQKey,
		rocket.ExptTurnResultFilterRMQKey,
	} {
		p := &producer{}

		if err := loader.UnmarshalKey(ctx, key, &p.cfg); err != nil {
			return nil, err
		}

		if !p.cfg.Valid() {
			return nil, fmt.Errorf("rmq config with invalid addr, key: %v, conf: %v", key, json.Jsonify(p.cfg))
		}

		if exist := publisher.getProducerWithAddr(p.cfg.Addr); exist != nil {
			p.p = exist.p
			publisher.producers[key] = p
			continue
		}

		pcfg := p.cfg.ToProducerCfg()
		p.p, err = mqFactory.NewProducer(pcfg)
		if err != nil {
			return nil, errorx.Wrapf(err, "new mq producer fail, cfg: %v", pcfg)
		}

		if err := p.p.Start(); err != nil {
			return nil, errorx.Wrapf(err, "start mq producer fail, cfg: %v", pcfg)
		}

		publisher.producers[key] = p
	}

	return publisher, nil
}

type producer struct {
	cfg rocket.RMQConf
	p   mq.IProducer
}

type exptEventPublisher struct {
	producers map[string]*producer
}

func (e *exptEventPublisher) getProducerWithAddr(addr string) *producer {
	for _, p := range e.producers {
		if p.cfg.Addr == addr {
			return p
		}
	}
	return nil
}

func (e *exptEventPublisher) PublishExptScheduleEvent(ctx context.Context, event *entity.ExptScheduleEvent, duration *time.Duration) error {
	return e.batchSend(ctx, rocket.ExptScheduleEventRMQKey, []any{event}, duration)
}

func (e *exptEventPublisher) PublishExptRecordEvalEvent(ctx context.Context, event *entity.ExptItemEvalEvent, duration *time.Duration) error {
	return e.batchSend(ctx, rocket.ExptRecordEvalEventRMQKey, []any{event}, duration)
}

func (e *exptEventPublisher) BatchPublishExptRecordEvalEvent(ctx context.Context, events []*entity.ExptItemEvalEvent, duration *time.Duration) error {
	return e.batchSend(ctx, rocket.ExptRecordEvalEventRMQKey, lo.ToAnySlice(events), duration)
}

func (e *exptEventPublisher) PublishExptAggrCalculateEvent(ctx context.Context, events []*entity.AggrCalculateEvent, duration *time.Duration) error {
	return e.batchSend(ctx, rocket.ExptAggrCalculateEventRMQKey, lo.ToAnySlice(events), duration)
}

func (e *exptEventPublisher) PublishExptOnlineEvalResult(ctx context.Context, event *entity.OnlineExptEvalResultEvent, duration *time.Duration) error {
	if len(event.TurnEvalResults) == 0 {
		return nil
	}
	evaluatorRecordIDs := make([]int64, 0, len(event.TurnEvalResults))
	for _, r := range event.TurnEvalResults {
		evaluatorRecordIDs = append(evaluatorRecordIDs, r.EvaluatorRecordId)
	}
	logs.CtxInfo(ctx, "Publishing ExptOnlineEvalResult event, expt_id: %v, evaluator_record_ids: %v", event.ExptId, evaluatorRecordIDs)
	return e.batchSend(ctx, rocket.ExptOnlineEvalResultRMQKey, []any{event}, duration)
}

func (e *exptEventPublisher) PublishExptTurnResultFilterEvent(ctx context.Context, event *entity.ExptTurnResultFilterEvent, duration *time.Duration) error {
	return e.batchSend(ctx, rocket.ExptTurnResultFilterRMQKey, []any{event}, duration)
}

func (e *exptEventPublisher) batchSend(ctx context.Context, pk string, events []any, duration *time.Duration) error {
	p, ok := e.producers[pk]
	if !ok {
		return fmt.Errorf("rmq producer not found %v", pk)
	}

	msgs := make([]*mq.Message, 0, len(events))
	for _, e := range events {
		bytes, err := json.Marshal(e)
		if err != nil {
			return errorx.Wrapf(err, "json marshal fail")
		}

		var msg *mq.Message
		if duration == nil {
			msg = mq.NewMessage(p.cfg.Topic, bytes)
		} else {
			msg = mq.NewDeferMessage(p.cfg.Topic, gptr.Indirect(duration), bytes)
		}
		msgs = append(msgs, msg)
	}

	resp, err := p.p.SendBatch(ctx, msgs)
	if err != nil {
		return errorx.Wrapf(err, "send batch message fail, msgs: %v", json.Jsonify(msgs))
	}

	logs.CtxInfo(ctx, "expt event batch send success, message_id: %v, offset: %v", resp.MessageID, resp.Offset)
	return nil
}
