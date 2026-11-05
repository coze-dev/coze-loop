// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package consumer

import (
	"context"
	"fmt"

	"github.com/bytedance/sonic"

	"github.com/coze-dev/coze-loop/backend/infra/mq"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type ExptTurnResultFilterConsumer struct {
	exptManager evaluation.ExperimentService
}

func NewExptTurnResultFilterConsumer(exptManager evaluation.ExperimentService) mq.IConsumerHandler {
	return &ExptTurnResultFilterConsumer{
		exptManager: exptManager,
	}
}

func (c *ExptTurnResultFilterConsumer) HandleMessage(ctx context.Context, ext *mq.MessageExt) (err error) {
	defer func() {
		if err != nil {
			logs.CtxError(ctx, "ExptTurnResultFilterConsumer HandleMessage fail, err: %v", err)
		}
	}()

	if ext == nil {
		return fmt.Errorf("filter event is nil")
	}
	event := &entity.ExptTurnResultFilterEvent{}
	body := ext.Body
	if err := sonic.Unmarshal(body, event); err != nil {
		return fmt.Errorf("decode filter event: %w", err)
	}
	if event.SpaceID <= 0 || event.ExperimentID <= 0 || event.RetryTimes != nil && *event.RetryTimes < 0 {
		return fmt.Errorf("invalid filter event scope or retry count")
	}
	for _, itemID := range event.ItemID {
		if itemID <= 0 {
			return fmt.Errorf("invalid filter event item_id: %d", itemID)
		}
	}
	if event.FilterType != nil {
		switch *event.FilterType {
		case entity.UpsertExptTurnResultFilterTypeAuto, entity.UpsertExptTurnResultFilterTypeCheck, entity.UpsertExptTurnResultFilterTypeManual:
		default:
			return fmt.Errorf("unknown filter event type: %q", *event.FilterType)
		}
	}

	logs.CtxInfo(ctx, "ExptTurnResultFilterConsumer consume message, event: %v, msg_id: %v", string(body), ext.MsgID)

	upsertExptTurnResultFilterRequest := &expt.UpsertExptTurnResultFilterRequest{
		WorkspaceID:  ptr.Of(event.SpaceID),
		ExperimentID: ptr.Of(event.ExperimentID),
		ItemIds:      event.ItemID,
	}
	if event.FilterType != nil {
		upsertExptTurnResultFilterRequest.FilterType = (*expt.UpsertExptTurnResultFilterType)(event.FilterType)
	}
	if event.RetryTimes != nil {
		upsertExptTurnResultFilterRequest.RetryTimes = ptr.Of(*event.RetryTimes)
	}
	// 调用 ExptMangerImpl.UpsertExptTurnResultFilter
	resp, err := c.exptManager.UpsertExptTurnResultFilter(ctx, upsertExptTurnResultFilterRequest)
	if err != nil {
		return err
	}
	if resp == nil {
		return fmt.Errorf("filter application returned nil response")
	}
	if resp.BaseResp != nil && resp.BaseResp.StatusCode != 0 {
		return errorx.NewByCode(resp.BaseResp.StatusCode, errorx.WithExtraMsg(resp.BaseResp.StatusMessage))
	}
	return nil
}
