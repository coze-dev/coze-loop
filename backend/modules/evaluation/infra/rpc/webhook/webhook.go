// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// NoopWebhookDeliveryService is a no-op implementation of IWebhookDeliveryService.
// Used as the default in the open-source build where no MQ-based delivery is available.
type NoopWebhookDeliveryService struct{}

func NewNoopWebhookDeliveryService() component.IWebhookDeliveryService {
	return &NoopWebhookDeliveryService{}
}

func (n *NoopWebhookDeliveryService) DeliverWebhook(ctx context.Context, msg *entity.WebhookDeliveryMessage) error {
	return nil
}

// NoopWebhookSecretProvider is a no-op implementation of IWebhookSecretProvider.
// Used as the default in the open-source build where no DKMS/workspace SK is available.
type NoopWebhookSecretProvider struct{}

func NewNoopWebhookSecretProvider() rpc.IWebhookSecretProvider {
	return &NoopWebhookSecretProvider{}
}

func (n *NoopWebhookSecretProvider) GetWorkspaceSK(ctx context.Context, spaceID int64) (string, error) {
	return "", nil
}
