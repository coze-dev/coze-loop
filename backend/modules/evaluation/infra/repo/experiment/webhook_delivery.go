// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
)

// NewWebhookDeliveryRepo wraps the DAO with entity ↔ PO conversion. The
// symbol is what commercial wire_gen already references
// (`experiment.NewWebhookDeliveryRepo(webhookDeliveryDAO)`).
func NewWebhookDeliveryRepo(dao mysql.IWebhookDeliveryDAO) repo.IWebhookDeliveryRepo {
	return &webhookDeliveryRepo{dao: dao}
}

type webhookDeliveryRepo struct {
	dao mysql.IWebhookDeliveryDAO
}

func (r *webhookDeliveryRepo) Create(ctx context.Context, delivery *entity.WebhookDelivery, opts ...db.Option) error {
	po := toWebhookDeliveryPO(delivery)
	if err := r.dao.Create(ctx, po, opts...); err != nil {
		return err
	}
	delivery.ID = po.ID
	delivery.CreatedAt = po.CreatedAt
	delivery.UpdatedAt = po.UpdatedAt
	return nil
}

func (r *webhookDeliveryRepo) Update(ctx context.Context, delivery *entity.WebhookDelivery, opts ...db.Option) error {
	return r.dao.Update(ctx, toWebhookDeliveryPO(delivery), opts...)
}

func (r *webhookDeliveryRepo) GetByDeliveryID(ctx context.Context, deliveryID string, opts ...db.Option) (*entity.WebhookDelivery, error) {
	po, err := r.dao.GetByDeliveryID(ctx, deliveryID, opts...)
	if err != nil {
		return nil, err
	}
	return fromWebhookDeliveryPO(po), nil
}

func (r *webhookDeliveryRepo) ListByExptID(ctx context.Context, params repo.ListDeliveryParams, opts ...db.Option) ([]*entity.WebhookDelivery, int64, error) {
	pos, total, err := r.dao.ListByExptID(ctx, params, opts...)
	if err != nil {
		return nil, 0, err
	}
	out := make([]*entity.WebhookDelivery, 0, len(pos))
	for _, po := range pos {
		out = append(out, fromWebhookDeliveryPO(po))
	}
	return out, total, nil
}

func (r *webhookDeliveryRepo) ListRetryable(ctx context.Context, params repo.ListRetryableParams, opts ...db.Option) ([]*entity.WebhookDelivery, error) {
	pos, err := r.dao.ListRetryable(ctx, params, opts...)
	if err != nil {
		return nil, err
	}
	out := make([]*entity.WebhookDelivery, 0, len(pos))
	for _, po := range pos {
		out = append(out, fromWebhookDeliveryPO(po))
	}
	return out, nil
}

func toWebhookDeliveryPO(d *entity.WebhookDelivery) *mysql.WebhookDeliveryPO {
	return &mysql.WebhookDeliveryPO{
		ID:               d.ID,
		DeliveryID:       d.DeliveryID,
		SpaceID:          d.SpaceID,
		ExperimentID:     d.ExperimentID,
		Event:            d.Event,
		URL:              d.URL,
		Payload:          d.Payload,
		Status:           d.Status,
		AttemptCount:     d.AttemptCount,
		FirstSentAt:      d.FirstSentAt,
		LastSentAt:       d.LastSentAt,
		LastResponseCode: d.LastResponseCode,
		LastError:        d.LastError,
		InternalSource:   d.InternalSource,
		CreatedAt:        d.CreatedAt,
		UpdatedAt:        d.UpdatedAt,
	}
}

func fromWebhookDeliveryPO(po *mysql.WebhookDeliveryPO) *entity.WebhookDelivery {
	if po == nil {
		return nil
	}
	return &entity.WebhookDelivery{
		ID:               po.ID,
		DeliveryID:       po.DeliveryID,
		SpaceID:          po.SpaceID,
		ExperimentID:     po.ExperimentID,
		Event:            po.Event,
		URL:              po.URL,
		Payload:          po.Payload,
		Status:           po.Status,
		AttemptCount:     po.AttemptCount,
		FirstSentAt:      po.FirstSentAt,
		LastSentAt:       po.LastSentAt,
		LastResponseCode: po.LastResponseCode,
		LastError:        po.LastError,
		InternalSource:   po.InternalSource,
		CreatedAt:        po.CreatedAt,
		UpdatedAt:        po.UpdatedAt,
	}
}
