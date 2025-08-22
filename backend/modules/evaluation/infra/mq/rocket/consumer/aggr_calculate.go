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

type AggrCalculateConsumer struct {
	exptAggrResultService service.ExptAggrResultService
}

func NewAggrCalculateConsumer(exptAggrResultService service.ExptAggrResultService) mq.IConsumerHandler {
	return &AggrCalculateConsumer{
		exptAggrResultService: exptAggrResultService,
	}
}

func (a *AggrCalculateConsumer) HandleMessage(ctx context.Context, ext *mq.MessageExt) (err error) {
	defer func() {
		if err != nil {
			logs.CtxError(ctx, "AggrCalculateHandler HandleMessage fail, err: %v", err)
		}
	}()

	event := &entity.AggrCalculateEvent{}
	body := ext.Body
	if err := sonic.Unmarshal(body, event); err != nil {
		logs.CtxError(ctx, "AggrCalculateEvent json unmarshal fail, raw: %v, err: %s", string(body), err)
		return nil
	}

	logs.CtxInfo(ctx, "AggrCalculateHandler consume message, event: %v, msg_id: %v", string(body), ext.MsgID)

	return a.handleEvent(ctx, event)
}

func (a *AggrCalculateConsumer) handleEvent(ctx context.Context, event *entity.AggrCalculateEvent) (err error) {
	switch event.CalculateMode {
	case entity.CreateAllFields:
		err = a.exptAggrResultService.CreateExptAggrResult(ctx, event.SpaceID, event.ExperimentID)
		return err
	case entity.UpdateSpecificField:
		param := &entity.UpdateExptAggrResultParam{
			SpaceID:      event.SpaceID,
			ExperimentID: event.ExperimentID,
			FieldType:    event.GetFieldType(),
			FieldKey:     event.GetFieldKey(),
		}
		err = a.exptAggrResultService.UpdateExptAggrResult(ctx, param)
		return err
	case entity.CreateAnnotationFields:
		param := &entity.CreateSpecificFieldAggrResultParam{
			SpaceID:      event.SpaceID,
			ExperimentID: event.ExperimentID,
			FieldType:    event.GetFieldType(),
			FieldKey:     event.GetFieldKey(),
		}
		err = a.exptAggrResultService.CreateAnnotationAggrResult(ctx, param)
		return err
	case entity.UpdateAnnotationFields:
		param := &entity.UpdateExptAggrResultParam{
			SpaceID:      event.SpaceID,
			ExperimentID: event.ExperimentID,
			FieldType:    event.GetFieldType(),
			FieldKey:     event.GetFieldKey(),
		}
		err = a.exptAggrResultService.UpdateAnnotationAggrResult(ctx, param)
		return err
	}

	return nil
}
