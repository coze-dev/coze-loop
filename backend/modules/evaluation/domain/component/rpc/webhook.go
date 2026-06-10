// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// IWebhookSender webhook 投递端口：领域服务命中 webhook 规则后调用，将单个 URL 的投递入队（异步）。
// 开源仓提供 noop 默认实现；商业仓注入真实实现（RocketMQ 延迟消息 Producer），由 Consumer 执行 HTTP POST。
//
//go:generate mockgen -destination=mocks/webhook.go -package=mocks . IWebhookSender,IWebhookSecretProvider
type IWebhookSender interface {
	// SendWebhookDelivery 将一条 webhook 投递事件入队。attempt=0 为首发。
	// delivery_id 重试时保持不变（at-least-once + 业务幂等）。
	SendWebhookDelivery(ctx context.Context, event *entity.WebhookDeliveryEvent) error
}

// IWebhookSecretProvider 取空间 SK 用于 HMAC-SHA256 签名（复用 foundation 空间 AK/SK，不新建 secret）。
// 开源仓提供 noop 默认实现（返回空）；商业仓注入真实实现（读空间凭据）。
type IWebhookSecretProvider interface {
	// GetSpaceSecretKey 返回指定空间用于 webhook 签名的 SK；无可用 SK 时返回空字符串。
	GetSpaceSecretKey(ctx context.Context, spaceID int64) (string, error)
}
