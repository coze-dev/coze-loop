// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package producer

import (
	"context"
	"fmt"
	"strings"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

// CentralItemDispatcher 复用既有 item eval 事件通道，只额外标记“确定失败”与“结果不明”。
type CentralItemDispatcher struct {
	publisher events.ExptEventPublisher
}

func NewCentralItemDispatcher(publisher events.ExptEventPublisher) repo.ICentralItemDispatcher {
	return &CentralItemDispatcher{publisher: publisher}
}

func (d *CentralItemDispatcher) Dispatch(ctx context.Context, event *entity.ExptItemEvalEvent) error {
	if event == nil {
		return fmt.Errorf("%w: nil item event", repo.ErrDefiniteDispatchFailure)
	}
	if err := d.publisher.PublishExptRecordEvalEvent(ctx, event, nil, nil); err != nil {
		// 发送前可判定的失败：无 producer / 序列化失败，消息一定没发出去。
		msg := err.Error()
		if strings.Contains(msg, "rmq producer not found") || strings.Contains(msg, "json marshal fail") {
			return fmt.Errorf("%w: %v", repo.ErrDefiniteDispatchFailure, err)
		}
		// SendBatch 超时/网络错误可能已投递，按结果不明处理，进入待补偿。
		return err
	}
	return nil
}
