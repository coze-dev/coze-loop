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
	evaluatorPublisherSingleton events.EvaluatorEventPublisher
	evaluatorPublisherOnce      sync.Once
)

func NewEvaluatorEventPublisher(ctx context.Context, cfgFactory conf.IConfigLoaderFactory, mqFactory mq.IFactory) (p events.EvaluatorEventPublisher, err error) {
	evaluatorPublisherOnce.Do(func() {
		evaluatorPublisherSingleton, err = newEvaluatorEventPublisher(ctx, cfgFactory, mqFactory)
	})
	return evaluatorPublisherSingleton, err
}

func newEvaluatorEventPublisher(ctx context.Context, cfgFactory conf.IConfigLoaderFactory, mqFactory mq.IFactory) (events.EvaluatorEventPublisher, error) {
	loader, err := cfgFactory.NewConfigLoader(consts.EvaluationConfigFileName)
	if err != nil {
		return nil, err
	}

	publisher := &evaluatorEventPublisher{producers: make(map[string]*producer)}

	for _, key := range []string{
		// 这里假设 evaluator 相关的 RMQ 键，需要根据实际情况修改
		rocket.EvaluatorRecordCorrectionRMQKey,
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

type evaluatorEventPublisher struct {
	producers map[string]*producer
}

func (e *evaluatorEventPublisher) getProducerWithAddr(addr string) *producer {
	for _, p := range e.producers {
		if p.cfg.Addr == addr {
			return p
		}
	}
	return nil
}

func (e *evaluatorEventPublisher) PublishEvaluatorRecordCorrection(ctx context.Context, evaluatorRecordCorrectionEvent *entity.EvaluatorRecordCorrectionEvent, duration *time.Duration) error {
	logs.CtxInfo(ctx, "Publishing EvaluatorRecordCorrection event, evaluator_record_id: %v", evaluatorRecordCorrectionEvent.EvaluatorRecordID)
	return e.batchSend(ctx, rocket.EvaluatorRecordCorrectionRMQKey, lo.ToAnySlice([]*entity.EvaluatorRecordCorrectionEvent{evaluatorRecordCorrectionEvent}), duration)
}

func (e *evaluatorEventPublisher) batchSend(ctx context.Context, pk string, events []any, duration *time.Duration) error {
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
	if env := os.Getenv(XttEnv); env != "" {
		ctx = context.WithValue(ctx, CtxKeyEnv, env) //nolint:staticcheck
	}
	resp, err := p.p.SendBatch(ctx, msgs)
	if err != nil {
		return errorx.Wrapf(err, "send batch message fail, msgs: %v", json.Jsonify(msgs))
	}

	logs.CtxInfo(ctx, "evaluator event batch send success, message_id: %v, offset: %v", resp.MessageID, resp.Offset)
	return nil
}
