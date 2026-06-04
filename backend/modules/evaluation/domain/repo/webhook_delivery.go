// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type ListDeliveryParams struct {
	SpaceID int64
	ExptID  int64
	Page    entity.Page
}

type ListRetryableParams struct {
	Limit int
	Now   time.Time
}

type IWebhookDeliveryRepo interface {
	Create(ctx context.Context, delivery *entity.WebhookDelivery, opts ...db.Option) error
	Update(ctx context.Context, delivery *entity.WebhookDelivery, opts ...db.Option) error
	GetByDeliveryID(ctx context.Context, deliveryID string, opts ...db.Option) (*entity.WebhookDelivery, error)
	ListByExptID(ctx context.Context, params ListDeliveryParams, opts ...db.Option) ([]*entity.WebhookDelivery, int64, error)
	ListRetryable(ctx context.Context, params ListRetryableParams, opts ...db.Option) ([]*entity.WebhookDelivery, error)
}
