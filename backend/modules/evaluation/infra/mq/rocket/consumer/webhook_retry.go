// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"context"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/sonic"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/mq/rocket"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// WebhookRetryConsumer 消费 webhook_delivery_event_rmq Topic，执行投递与失败重试。
type WebhookRetryConsumer struct {
	delivery *service.WebhookDeliveryService
}

func NewWebhookRetryConsumer(delivery *service.WebhookDeliveryService) mq.IConsumerHandler {
	return &WebhookRetryConsumer{delivery: delivery}
}

func (c *WebhookRetryConsumer) HandleMessage(ctx context.Context, ext *mq.MessageExt) (err error) {
	defer func() {
		if err != nil {
			logs.CtxError(ctx, "WebhookRetryConsumer HandleMessage fail, err: %v", err)
		}
	}()

	event := &entity.WebhookRetryEvent{}
	body := ext.Body
	if err := sonic.Unmarshal(body, event); err != nil {
		logs.CtxError(ctx, "WebhookRetryEvent json unmarshal fail, raw: %v, err: %s", string(body), err)
		// 解析失败的脏消息直接 ack 丢弃，避免无限重试
		return nil
	}

	logs.CtxInfo(ctx, "WebhookRetryConsumer consume message, delivery_id: %s, expt_id: %d, event: %s, retry: %d, msg_id: %v",
		event.DeliveryID, event.ExptID, event.Event, event.RetryCount, ext.MsgID)

	c.delivery.DeliverRetry(ctx, event)
	return nil
}

// WebhookDeliveryEventConsumer ConsumerWorker，与其他 evaluation consumer 风格保持一致。
type WebhookDeliveryEventConsumer struct {
	mq.IConsumerHandler
	conf.IConfigLoader
}

func NewWebhookDeliveryEventConsumer(handler mq.IConsumerHandler, loader conf.IConfigLoader) mq.IConsumerWorker {
	return &WebhookDeliveryEventConsumer{
		IConsumerHandler: handler,
		IConfigLoader:    loader,
	}
}

func (e *WebhookDeliveryEventConsumer) ConsumerCfg(ctx context.Context) (*mq.ConsumerConfig, error) {
	rmqCfg := &rocket.RMQConf{}
	if err := e.UnmarshalKey(ctx, rocket.WebhookDeliveryEventRMQKey, rmqCfg); err != nil {
		return nil, err
	}
	return gptr.Of(rmqCfg.ToConsumerCfg()), nil
}
