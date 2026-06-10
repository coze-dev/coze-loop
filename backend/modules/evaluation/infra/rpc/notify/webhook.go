// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package notify

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// WebhookSender 开源 noop 实现：不实际投递，仅记录日志。
// 商业仓通过 Wire 注入真实实现（RocketMQ 延迟消息 Producer + HTTP Consumer）。
type WebhookSender struct{}

func NewWebhookSender() rpc.IWebhookSender {
	return WebhookSender{}
}

func (WebhookSender) SendWebhookDelivery(ctx context.Context, event *entity.WebhookDeliveryEvent) error {
	if event == nil {
		return nil
	}
	logs.CtxInfo(ctx, "[Webhook] noop sender skip delivery, delivery_id: %v, url: %v, attempt: %v",
		event.DeliveryID, event.URL, event.Attempt)
	return nil
}

// WebhookSecretProvider 开源 noop 实现：返回空 SK（开源无字节内部密钥服务）。
// 商业仓通过 Wire 注入真实实现（读 foundation 空间 AK/SK）。
type WebhookSecretProvider struct{}

func NewWebhookSecretProvider() rpc.IWebhookSecretProvider {
	return WebhookSecretProvider{}
}

func (WebhookSecretProvider) GetSpaceSecretKey(ctx context.Context, spaceID int64) (string, error) {
	return "", nil
}
