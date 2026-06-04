// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/mohae/deepcopy"
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

const (
	CtxKeyEnv = "K_ENV"
	XttEnv    = "x_tt_env"
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

func NewWebhookDeliveryEventPublisher(p events.ExptEventPublisher) events.WebhookDeliveryEventPublisher {
	return p.(events.WebhookDeliveryEventPublisher)
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
		rocket.ExptExportCSVEventRMQKey,
		rocket.ExptLifecycleEventRMQKey,
		rocket.WebhookDeliveryRMQKey,
	} {
		p := &producer{}

		if err := loader.UnmarshalKey(ctx, key, &p.cfg); err != nil {
			return nil, err
		}

		if gptr.Indirect(p.cfg.DisableProduce) {
			logs.CtxWarn(ctx, "[ExptProducer] producer skipped by DisableProduce=true, key=%s addr=%s topic=%s", key, p.cfg.Addr, p.cfg.Topic)
			continue
		}

		if !p.cfg.Valid() {
			return nil, fmt.Errorf("rmq config with invalid addr, key: %v, conf: %v", key, json.Jsonify(p.cfg))
		}

		if exist := publisher.getProducerWithAddr(p.cfg.Addr); exist != nil {
			p.p = exist.p
			publisher.producers[key] = p
			logs.CtxInfo(ctx, "[ExptProducer] producer reuse existing by addr, key=%s addr=%s topic=%s", key, p.cfg.Addr, p.cfg.Topic)
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
		logs.CtxInfo(ctx, "[ExptProducer] producer registered, key=%s addr=%s topic=%s producer_group=%s", key, p.cfg.Addr, p.cfg.Topic, p.cfg.ProducerGroup)
	}

	logs.CtxInfo(ctx, "[ExptProducer] publisher init complete, total_producers=%d lifecycle_registered=%v webhook_registered=%v",
		len(publisher.producers),
		publisher.producers[rocket.ExptLifecycleEventRMQKey] != nil,
		publisher.producers[rocket.WebhookDeliveryRMQKey] != nil)
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

func (e *exptEventPublisher) PublishExptRecordEvalEvent(ctx context.Context, event *entity.ExptItemEvalEvent, duration *time.Duration, modifyFunc func(event *entity.ExptItemEvalEvent)) error {
	if copied, ok := deepcopy.Copy(event).(*entity.ExptItemEvalEvent); ok {
		if modifyFunc != nil {
			modifyFunc(copied)
		}
		event = copied
	}
	return e.batchSend(ctx, rocket.ExptRecordEvalEventRMQKey, []any{event}, duration)
}

func (e *exptEventPublisher) BatchPublishExptRecordEvalEvent(ctx context.Context, events []*entity.ExptItemEvalEvent, duration *time.Duration) error {
	return e.batchSend(ctx, rocket.ExptRecordEvalEventRMQKey, lo.ToAnySlice(events), duration)
}

func (e *exptEventPublisher) PublishExptAggrCalculateEvent(ctx context.Context, events []*entity.AggrCalculateEvent, duration *time.Duration) error {
	return e.batchSend(ctx, rocket.ExptAggrCalculateEventRMQKey, lo.ToAnySlice(events), duration)
}

func (e *exptEventPublisher) PublishExptExportCSVEvent(ctx context.Context, event *entity.ExportCSVEvent, duration *time.Duration) error {
	return e.batchSend(ctx, rocket.ExptExportCSVEventRMQKey, []any{event}, duration)
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

func (e *exptEventPublisher) PublishExptLifecycleEvent(ctx context.Context, event *entity.ExptLifecycleEvent, duration *time.Duration) error {
	p, hit := e.producers[rocket.ExptLifecycleEventRMQKey]
	var topic, addr string
	if hit && p != nil {
		topic = p.cfg.Topic
		addr = p.cfg.Addr
	}
	logs.CtxInfo(ctx, "[ExptProducer] publish lifecycle event: key=%s producer_hit=%v topic=%s addr=%s expt_id=%d space_id=%d from_status=%d to_status=%d",
		rocket.ExptLifecycleEventRMQKey, hit, topic, addr, event.ExptID, event.SpaceID, event.FromStatus, event.ToStatus)
	err := e.batchSend(ctx, rocket.ExptLifecycleEventRMQKey, []any{event}, duration)
	if err != nil {
		logs.CtxError(ctx, "[ExptProducer] publish lifecycle event FAILED: expt_id=%d to_status=%d err=%v", event.ExptID, event.ToStatus, err)
	} else {
		logs.CtxInfo(ctx, "[ExptProducer] publish lifecycle event OK: expt_id=%d to_status=%d topic=%s", event.ExptID, event.ToStatus, topic)
	}
	return err
}

func (e *exptEventPublisher) PublishWebhookDeliveryEvent(ctx context.Context, event *entity.WebhookDeliveryMessage, duration *time.Duration) error {
	return e.batchSend(ctx, rocket.WebhookDeliveryRMQKey, []any{event}, duration)
}

func (e *exptEventPublisher) batchSend(ctx context.Context, pk string, events []any, duration *time.Duration) error {
	p, ok := e.producers[pk]
	if !ok {
		logs.CtxError(ctx, "[ExptProducer] batchSend producer NOT FOUND, producer_key=%s registered_keys=%v event_cnt=%d", pk, e.registeredKeys(), len(events))
		return fmt.Errorf("rmq producer not found %v", pk)
	}
	if p == nil || p.p == nil {
		logs.CtxError(ctx, "[ExptProducer] batchSend producer entry is nil, producer_key=%s p_nil=%v inner_nil=%v", pk, p == nil, p != nil && p.p == nil)
		return fmt.Errorf("rmq producer nil entry %v", pk)
	}

	msgs := make([]*mq.Message, 0, len(events))
	for _, e := range events {
		bytes, err := json.Marshal(e)
		if err != nil {
			logs.CtxError(ctx, "[ExptProducer] batchSend marshal fail, producer_key=%s err=%v", pk, err)
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
	if env := os.Getenv(XttEnv); env != "" {
		ctx = context.WithValue(ctx, CtxKeyEnv, env) //nolint:staticcheck
	}
	logs.CtxInfo(ctx, "[ExptProducer] batchSend dispatching to MQ, producer_key=%s topic=%s addr=%s msg_cnt=%d defer=%v", pk, p.cfg.Topic, p.cfg.Addr, len(msgs), duration != nil)
	resp, err := p.p.SendBatch(ctx, msgs)
	if err != nil {
		logs.CtxError(ctx, "[ExptProducer] batchSend SendBatch FAILED, producer_key=%s topic=%s addr=%s err=%v", pk, p.cfg.Topic, p.cfg.Addr, err)
		return errorx.Wrapf(err, "send batch message fail, producer_key: %v, msgs: %v", pk, json.Jsonify(msgs))
	}

	logs.CtxInfo(ctx, "expt event batch send success, producer_key: %v, message_id: %v, offset: %v", pk, resp.MessageID, resp.Offset)
	return nil
}

func (e *exptEventPublisher) registeredKeys() []string {
	keys := make([]string, 0, len(e.producers))
	for k := range e.producers {
		keys = append(keys, k)
	}
	return keys
}
