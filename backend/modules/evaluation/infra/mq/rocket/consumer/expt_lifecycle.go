// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"context"

	"github.com/bytedance/sonic"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/mq/rocket"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type ExptLifecycleConsumer struct {
	handler        service.ExptLifecycleEventHandler
	webhookHandler mq.IConsumerHandler
}

func NewExptLifecycleConsumer(handler service.ExptLifecycleEventHandler, webhookHandler mq.IConsumerHandler) mq.IConsumerHandler {
	return &ExptLifecycleConsumer{
		handler:        handler,
		webhookHandler: webhookHandler,
	}
}

func (e *ExptLifecycleConsumer) HandleMessage(ctx context.Context, ext *mq.MessageExt) (err error) {
	logs.CtxInfo(ctx, "ExptLifecycleConsumer received message, tag: %v, msg_id: %v, body: %v", ext.Tag, ext.MsgID, string(ext.Body))
	defer func() {
		if err != nil {
			logs.CtxError(ctx, "ExptLifecycleConsumer HandleMessage fail, err: %v", err)
		}
	}()

	// 根据 Tag 路由：webhook_retry 走重试逻辑
	if ext.Tag == rocket.TagWebhookRetry {
		return e.webhookHandler.HandleMessage(ctx, ext)
	}

	event := &entity.ExptLifecycleEvent{}
	body := ext.Body
	if err := sonic.Unmarshal(body, event); err != nil {
		logs.CtxError(ctx, "ExptLifecycleEvent json unmarshal fail, raw: %v, err: %s", string(body), err)
		return nil
	}

	logs.CtxInfo(ctx, "ExptLifecycleConsumer consume message, event: %v, msg_id: %v", string(body), ext.MsgID)

	return e.handler.HandleLifecycleEvent(ctx, event)
}
