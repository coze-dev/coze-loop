// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"context"

	"github.com/bytedance/sonic"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type ExptLifecycleConsumer struct {
	handler service.ExptLifecycleEventHandler
}

func NewExptLifecycleConsumer(handler service.ExptLifecycleEventHandler) mq.IConsumerHandler {
	return &ExptLifecycleConsumer{
		handler: handler,
	}
}

func (e *ExptLifecycleConsumer) HandleMessage(ctx context.Context, ext *mq.MessageExt) (err error) {
	defer func() {
		if err != nil {
			logs.CtxError(ctx, "ExptLifecycleConsumer HandleMessage fail, err: %v", err)
		}
	}()

	event := &entity.ExptLifecycleEvent{}
	body := ext.Body
	if err := sonic.Unmarshal(body, event); err != nil {
		logs.CtxError(ctx, "ExptLifecycleEvent json unmarshal fail, raw: %v, err: %s", string(body), err)
		return nil
	}

	logs.CtxInfo(ctx, "ExptLifecycleConsumer consume message, event: %v, msg_id: %v", string(body), ext.MsgID)

	return e.handler.HandleLifecycleEvent(ctx, event)
}
