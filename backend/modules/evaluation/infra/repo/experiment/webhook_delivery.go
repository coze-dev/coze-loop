// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
)

type WebhookDeliveryRepoImpl struct {
	webhookDeliveryDAO mysql.WebhookDeliveryDAO
}

func NewWebhookDeliveryRepo(webhookDeliveryDAO mysql.WebhookDeliveryDAO) repo.IWebhookDeliveryRepo {
	return &WebhookDeliveryRepoImpl{webhookDeliveryDAO: webhookDeliveryDAO}
}

func (r *WebhookDeliveryRepoImpl) Create(ctx context.Context, delivery *entity.WebhookDelivery, opts ...db.Option) error {
	return r.webhookDeliveryDAO.Create(ctx, convert.WebhookDeliveryDO2PO(delivery), opts...)
}

func (r *WebhookDeliveryRepoImpl) Update(ctx context.Context, delivery *entity.WebhookDelivery, opts ...db.Option) error {
	return r.webhookDeliveryDAO.Update(ctx, convert.WebhookDeliveryDO2PO(delivery), opts...)
}

func (r *WebhookDeliveryRepoImpl) GetByDeliveryID(ctx context.Context, deliveryID string, opts ...db.Option) (*entity.WebhookDelivery, error) {
	po, err := r.webhookDeliveryDAO.GetByDeliveryID(ctx, deliveryID, opts...)
	if err != nil {
		return nil, err
	}
	return convert.WebhookDeliveryPO2DO(po), nil
}

func (r *WebhookDeliveryRepoImpl) ListByExptID(ctx context.Context, params repo.ListDeliveryParams, opts ...db.Option) ([]*entity.WebhookDelivery, int64, error) {
	pos, total, err := r.webhookDeliveryDAO.ListByExptID(ctx, params, opts...)
	if err != nil {
		return nil, 0, err
	}
	deliveries := make([]*entity.WebhookDelivery, 0, len(pos))
	for _, po := range pos {
		deliveries = append(deliveries, convert.WebhookDeliveryPO2DO(po))
	}
	return deliveries, total, nil
}

func (r *WebhookDeliveryRepoImpl) ListRetryable(ctx context.Context, params repo.ListRetryableParams, opts ...db.Option) ([]*entity.WebhookDelivery, error) {
	pos, err := r.webhookDeliveryDAO.ListRetryable(ctx, params, opts...)
	if err != nil {
		return nil, err
	}
	deliveries := make([]*entity.WebhookDelivery, 0, len(pos))
	for _, po := range pos {
		deliveries = append(deliveries, convert.WebhookDeliveryPO2DO(po))
	}
	return deliveries, nil
}
