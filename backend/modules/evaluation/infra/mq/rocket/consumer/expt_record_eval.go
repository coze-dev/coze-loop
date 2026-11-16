// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"context"

	"github.com/bytedance/sonic"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/conv"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func NewExptRecordEvalConsumer(recordEval service.ExptItemEvalEvent) mq.IConsumerHandler {
	return &ExptRecordEvalConsumer{
		ExptItemEvalEvent: recordEval,
	}
}

// NewCentralExptRecordEvalConsumer 中心事件的专用 consumer：只消费本 env 的 central 事件。
func NewCentralExptRecordEvalConsumer(recordEval service.ExptItemEvalEvent, env component.IEnvironmentProvider) mq.IConsumerHandler {
	return &ExptRecordEvalConsumer{
		ExptItemEvalEvent: recordEval,
		central:           true,
		env:               env,
	}
}

type ExptRecordEvalConsumer struct {
	service.ExptItemEvalEvent
	central bool
	env     component.IEnvironmentProvider
}

func (e *ExptRecordEvalConsumer) HandleMessage(ctx context.Context, ext *mq.MessageExt) error {
	rawLogID := logs.GetLogID(ctx)
	ctx = logs.SetLogID(ctx, logs.NewLogID())

	body := ext.Body
	event := &entity.ExptItemEvalEvent{}
	if err := sonic.Unmarshal(body, event); err != nil {
		logs.CtxError(ctx, "ExptItemEvalEvent json unmarshal fail, raw: %v, err: %s", conv.UnsafeBytesToString(body), err)
		return nil
	}

	if e.central {
		// central 组只处理中心事件，且必须属于本进程 env，避免跨泳道串消息。
		if event.CentralReservationID == "" {
			return nil
		}
		current, err := e.env.CurrentEnvID(ctx)
		if err != nil || event.CentralEnv == "" || event.CentralEnv != current {
			logs.CtxWarn(ctx, "[CentralConsumer] skip cross-env event resv=%s event_env=%q current=%q err=%v",
				event.CentralReservationID, event.CentralEnv, current, err)
			return nil
		}
	} else if event.CentralReservationID != "" {
		// 旧 consumer group 必须跳过中心事件，保证同一 item 不被两条链路消费。
		logs.CtxInfo(ctx, "[CentralConsumer] legacy group skip central event resv=%s", event.CentralReservationID)
		return nil
	}

	if event.Session != nil {
		ctx = session.WithCtxUser(ctx, &session.User{
			ID: event.Session.UserID,
		})
	}

	logs.CtxInfo(ctx, "ExptRecordEvalConsumer consume message, event: %v, msg_id: %v, rawlogid: %v", conv.UnsafeBytesToString(body), ext.MsgID, rawLogID)

	return e.Eval(ctx, event)
}
