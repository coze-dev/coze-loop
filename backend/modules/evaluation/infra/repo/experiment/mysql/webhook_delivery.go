// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/query"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

//go:generate mockgen -destination=mocks/webhook_delivery.go -package=mocks . WebhookDeliveryDAO
type WebhookDeliveryDAO interface {
	Create(ctx context.Context, delivery *model.WebhookDelivery, opts ...db.Option) error
	Update(ctx context.Context, delivery *model.WebhookDelivery, opts ...db.Option) error
	GetByDeliveryID(ctx context.Context, deliveryID string, opts ...db.Option) (*model.WebhookDelivery, error)
	ListByExptID(ctx context.Context, params repo.ListDeliveryParams, opts ...db.Option) ([]*model.WebhookDelivery, int64, error)
	ListRetryable(ctx context.Context, params repo.ListRetryableParams, opts ...db.Option) ([]*model.WebhookDelivery, error)
}

func NewWebhookDeliveryDAO(db db.Provider) WebhookDeliveryDAO {
	return &webhookDeliveryDAO{
		db:    db,
		query: query.Use(db.NewSession(context.Background())),
	}
}

type webhookDeliveryDAO struct {
	db    db.Provider
	query *query.Query
}

func (d *webhookDeliveryDAO) Create(ctx context.Context, delivery *model.WebhookDelivery, opts ...db.Option) error {
	if err := d.db.NewSession(ctx, opts...).Create(delivery).Error; err != nil {
		return errorx.Wrapf(err, "webhookDeliveryDAO create fail, model: %v", json.Jsonify(delivery))
	}
	return nil
}

func (d *webhookDeliveryDAO) Update(ctx context.Context, delivery *model.WebhookDelivery, opts ...db.Option) error {
	if err := d.db.NewSession(ctx, opts...).Model(&model.WebhookDelivery{}).Where("delivery_id = ?", delivery.DeliveryID).Updates(delivery).Error; err != nil {
		return errorx.Wrapf(err, "webhookDeliveryDAO update fail, model: %v", json.Jsonify(delivery))
	}
	return nil
}

func (d *webhookDeliveryDAO) GetByDeliveryID(ctx context.Context, deliveryID string, opts ...db.Option) (*model.WebhookDelivery, error) {
	q := query.Use(d.db.NewSession(ctx, opts...)).WebhookDelivery
	delivery, err := q.WithContext(ctx).Where(q.DeliveryID.Eq(deliveryID)).First()
	if err != nil {
		return nil, errorx.Wrapf(err, "webhookDeliveryDAO get fail, delivery_id: %v", deliveryID)
	}
	return delivery, nil
}

func (d *webhookDeliveryDAO) ListByExptID(ctx context.Context, params repo.ListDeliveryParams, opts ...db.Option) ([]*model.WebhookDelivery, int64, error) {
	var (
		deliveries []*model.WebhookDelivery
		total      int64
	)
	dbSession := d.db.NewSession(ctx, opts...).Model(&model.WebhookDelivery{}).
		Where("space_id = ?", params.SpaceID).
		Where("expt_id = ?", params.ExptID).
		Where("deleted_at = 0")
	if err := dbSession.Count(&total).Error; err != nil {
		return nil, 0, errorx.Wrapf(err, "webhookDeliveryDAO count fail, params: %v", json.Jsonify(params))
	}
	if err := dbSession.Order("created_at desc").Offset(params.Page.Offset()).Limit(params.Page.Limit()).Find(&deliveries).Error; err != nil {
		return nil, 0, errorx.Wrapf(err, "webhookDeliveryDAO list fail, params: %v", json.Jsonify(params))
	}
	return deliveries, total, nil
}

func (d *webhookDeliveryDAO) ListRetryable(ctx context.Context, params repo.ListRetryableParams, opts ...db.Option) ([]*model.WebhookDelivery, error) {
	limit := params.Limit
	if limit <= 0 {
		limit = 100
	}
	q := query.Use(d.db.NewSession(ctx, opts...)).WebhookDelivery
	deliveries, err := q.WithContext(ctx).Where(
		q.Status.Eq(string(entity.DeliveryStatusRetrying)),
		q.NextRetryAt.IsNotNull(),
		q.NextRetryAt.Lte(params.Now),
		q.DeletedAt.Eq(0),
	).Order(q.NextRetryAt.Asc()).Limit(limit).Find()
	if err != nil {
		return nil, errorx.Wrapf(err, "webhookDeliveryDAO list retryable fail, params: %v", json.Jsonify(params))
	}
	return deliveries, nil
}
