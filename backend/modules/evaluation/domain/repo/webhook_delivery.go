// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate mockgen -destination ./mocks/webhook_delivery.go --package mocks . IWebhookDeliveryRepo
type IWebhookDeliveryRepo interface {
	Create(ctx context.Context, delivery *entity.WebhookDelivery, opts ...db.Option) error
	Update(ctx context.Context, delivery *entity.WebhookDelivery, opts ...db.Option) error
	GetByDeliveryID(ctx context.Context, deliveryID string) (*entity.WebhookDelivery, error)
	ListByExperimentID(ctx context.Context, spaceID, experimentID int64) ([]*entity.WebhookDelivery, error)
	ListPendingRetries(ctx context.Context, limit int) ([]*entity.WebhookDelivery, error)
}
