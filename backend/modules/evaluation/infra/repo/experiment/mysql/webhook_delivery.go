// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/query"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func NewWebhookDeliveryRepo(provider db.Provider) repo.IWebhookDeliveryRepo {
	return &webhookDeliveryRepoImpl{
		provider:  provider,
		converter: convert.NewWebhookDeliveryConverter(),
	}
}

type webhookDeliveryRepoImpl struct {
	provider  db.Provider
	converter convert.WebhookDeliveryConverter
}

func (r *webhookDeliveryRepoImpl) Create(ctx context.Context, delivery *entity.WebhookDelivery, opts ...db.Option) error {
	po := r.converter.DO2PO(delivery)
	session := r.provider.NewSession(ctx, opts...)
	if err := session.Create(po).Error; err != nil {
		return errorx.Wrapf(err, "create webhook_delivery fail, delivery_id: %s", delivery.DeliveryID)
	}
	delivery.ID = po.ID
	return nil
}

func (r *webhookDeliveryRepoImpl) Update(ctx context.Context, delivery *entity.WebhookDelivery, opts ...db.Option) error {
	po := r.converter.DO2PO(delivery)
	po.UpdatedAt = time.Now()
	session := r.provider.NewSession(ctx, opts...)
	q := query.Use(session).WebhookDelivery
	_, err := q.WithContext(ctx).
		Where(q.ID.Eq(po.ID)).
		UpdateColumns(map[string]any{
			"status":           po.Status,
			"retry_count":      po.RetryCount,
			"last_status_code": po.LastStatusCode,
			"error_message":    po.ErrorMessage,
			"request_headers":  po.RequestHeaders,
			"next_retry_at":    po.NextRetryAt,
			"first_sent_at":    po.FirstSentAt,
			"last_sent_at":     po.LastSentAt,
			"updated_at":       po.UpdatedAt,
		})
	if err != nil {
		return errorx.Wrapf(err, "update webhook_delivery fail, id: %d", po.ID)
	}
	return nil
}

func (r *webhookDeliveryRepoImpl) GetByDeliveryID(ctx context.Context, deliveryID string) (*entity.WebhookDelivery, error) {
	session := r.provider.NewSession(ctx)
	q := query.Use(session).WebhookDelivery
	po, err := q.WithContext(ctx).
		Where(q.DeliveryID.Eq(deliveryID)).
		First()
	if err != nil {
		return nil, errorx.Wrapf(err, "get webhook_delivery fail, delivery_id: %s", deliveryID)
	}
	return r.converter.PO2DO(po), nil
}

func (r *webhookDeliveryRepoImpl) ListByExperimentID(ctx context.Context, spaceID, experimentID int64) ([]*entity.WebhookDelivery, error) {
	session := r.provider.NewSession(ctx)
	q := query.Use(session).WebhookDelivery
	pos, err := q.WithContext(ctx).
		Where(q.SpaceID.Eq(spaceID)).
		Where(q.ExperimentID.Eq(experimentID)).
		Order(q.CreatedAt.Desc()).
		Find()
	if err != nil {
		return nil, errorx.Wrapf(err, "list webhook_delivery fail, space_id: %d, experiment_id: %d", spaceID, experimentID)
	}
	results := make([]*entity.WebhookDelivery, 0, len(pos))
	for _, po := range pos {
		results = append(results, r.converter.PO2DO(po))
	}
	return results, nil
}

func (r *webhookDeliveryRepoImpl) ListPendingRetries(ctx context.Context, limit int) ([]*entity.WebhookDelivery, error) {
	session := r.provider.NewSession(ctx)
	q := query.Use(session).WebhookDelivery
	now := time.Now()
	pos, err := q.WithContext(ctx).
		Where(q.Status.Eq(int32(entity.DeliveryStatus_Retrying))).
		Where(q.NextRetryAt.Lte(now)).
		Order(q.NextRetryAt.Asc()).
		Limit(limit).
		Find()
	if err != nil {
		return nil, errorx.Wrapf(err, "list pending retries fail, limit: %d", limit)
	}
	results := make([]*entity.WebhookDelivery, 0, len(pos))
	for _, po := range pos {
		results = append(results, r.converter.PO2DO(po))
	}
	return results, nil
}
