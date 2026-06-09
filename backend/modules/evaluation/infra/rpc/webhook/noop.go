// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// NoopWebhookDeliveryService is the default noop implementation for OSS deployments.
type NoopWebhookDeliveryService struct{}

func NewNoopWebhookDeliveryService() rpc.IWebhookDeliveryService {
	return &NoopWebhookDeliveryService{}
}

func (n *NoopWebhookDeliveryService) PublishWebhookDelivery(_ context.Context, _ []*entity.WebhookDeliveryEvent) error {
	return nil
}

// NoopWebhookSigner is the default noop implementation for OSS deployments.
type NoopWebhookSigner struct{}

func NewNoopWebhookSigner() rpc.IWebhookSigner {
	return &NoopWebhookSigner{}
}

func (n *NoopWebhookSigner) Sign(_ context.Context, _ int64, _ int64, _ []byte) (string, error) {
	return "", nil
}
