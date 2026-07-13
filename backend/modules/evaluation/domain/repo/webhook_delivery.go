// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// ListDeliveryParams is the paginated read filter for webhook_delivery.
type ListDeliveryParams struct {
	SpaceID      int64
	ExperimentID int64
	Event        string
	Status       string
	Page         int
	Size         int
}

// ListRetryableParams selects rows that are eligible to retry: status ∈
// {retrying, failed} with last_sent_at older than `NotAfter`.
type ListRetryableParams struct {
	NotAfter time.Time
	Limit    int
}

//go:generate mockgen -destination mocks/webhook_delivery.go -package mocks . IWebhookDeliveryRepo
type IWebhookDeliveryRepo interface {
	Create(ctx context.Context, delivery *entity.WebhookDelivery, opts ...db.Option) error
	Update(ctx context.Context, delivery *entity.WebhookDelivery, opts ...db.Option) error
	GetByDeliveryID(ctx context.Context, deliveryID string, opts ...db.Option) (*entity.WebhookDelivery, error)
	ListByExptID(ctx context.Context, params ListDeliveryParams, opts ...db.Option) ([]*entity.WebhookDelivery, int64, error)
	ListRetryable(ctx context.Context, params ListRetryableParams, opts ...db.Option) ([]*entity.WebhookDelivery, error)
}
