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

type ExptExportConsumer struct {
	exptResultExportService service.IExptResultExportService
}

func NewExptExportConsumer(exptResultExportService service.IExptResultExportService) mq.IConsumerHandler {
	return &ExptExportConsumer{
		exptResultExportService: exptResultExportService,
	}
}

func (e *ExptExportConsumer) HandleMessage(ctx context.Context, ext *mq.MessageExt) (err error) {
	defer func() {
		if err != nil {
			logs.CtxError(ctx, "AggrCalculateHandler HandleMessage fail, err: %v", err)
		}
	}()

	event := &entity.ExportCSVEvent{}
	body := ext.Body
	if err := sonic.Unmarshal(body, event); err != nil {
		logs.CtxError(ctx, "ExportCSVEvent json unmarshal fail, raw: %v, err: %s", string(body), err)
		return nil
	}

	logs.CtxInfo(ctx, "ExptExportConsumer consume message, event: %v, msg_id: %v", string(body), ext.MsgID)

	return e.handleEvent(ctx, event)
}

func (e *ExptExportConsumer) handleEvent(ctx context.Context, event *entity.ExportCSVEvent) (err error) {
	err = e.exptResultExportService.DoExportCSV(ctx, event.SpaceID, event.ExperimentID, event.ExportID)
	if err != nil {
		// 不进行重试
		logs.CtxError(ctx, "ExptExportConsumer DoExportCSV fail, expt_id:%v, err: %v", event.ExperimentID, err)
		return nil
	}

	return nil
}
