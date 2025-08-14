// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/conv"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func newExptSchedulerConsumer(scheduler service.ExptSchedulerEvent) mq.IConsumerHandler {
	return &ExptSchedulerConsumer{
		scheduler: scheduler,
	}
}

type ExptSchedulerConsumer struct {
	scheduler service.ExptSchedulerEvent
}

func (e *ExptSchedulerConsumer) HandleMessage(ctx context.Context, msg *mq.MessageExt) error {
	rawLogID := logs.GetLogID(ctx)
	ctx = logs.SetLogID(ctx, logs.NewLogID())

	body := msg.Body
	event := &entity.ExptScheduleEvent{}
	if err := json.Unmarshal(body, event); err != nil {
		logs.CtxError(ctx, "ExptExecEvent json unmarshal fail, raw: %v, err: %s", conv.UnsafeBytesToString(body), err)
		return nil
	}

	if event.Session != nil {
		ctx = session.WithCtxUser(ctx, &session.User{
			ID: event.Session.UserID,
		})
	}

	logs.CtxInfo(ctx, "ExptSchedulerConsumer consume message, event: %v, msg_id: %v, rawlogid: %v", conv.UnsafeBytesToString(body), msg.MsgID, rawLogID)

	return e.scheduler.Schedule(ctx, event)
}
