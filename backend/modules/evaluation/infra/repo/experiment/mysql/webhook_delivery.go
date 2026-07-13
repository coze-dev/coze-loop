// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"time"

	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// WebhookDeliveryPO mirrors the `webhook_delivery` MySQL row 1:1. Columns are
// pinned via tags so the struct owns the schema until gorm_gen catches up.
type WebhookDeliveryPO struct {
	ID               int64      `gorm:"column:id;primaryKey;autoIncrement"`
	DeliveryID       string     `gorm:"column:delivery_id;uniqueIndex:uk_delivery_id;size:64"`
	SpaceID          int64      `gorm:"column:space_id;index:idx_space_time,priority:1"`
	ExperimentID     int64      `gorm:"column:experiment_id;index:idx_experiment,priority:1"`
	Event            string     `gorm:"column:event;index:idx_experiment,priority:2;size:32"`
	URL              string     `gorm:"column:url;size:1024"`
	Payload          []byte     `gorm:"column:payload;type:json"`
	Status           string     `gorm:"column:status;index:idx_retry,priority:1;size:32"`
	AttemptCount     int        `gorm:"column:attempt_count;default:0"`
	FirstSentAt      *time.Time `gorm:"column:first_sent_at"`
	LastSentAt       *time.Time `gorm:"column:last_sent_at;index:idx_retry,priority:2"`
	LastResponseCode int        `gorm:"column:last_response_code;default:0"`
	LastError        string     `gorm:"column:last_error;size:2048"`
	InternalSource   string     `gorm:"column:internal_source;size:32"`
	CreatedAt        time.Time  `gorm:"column:created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at"`
}

// TableName pins the table name (mandatory for GORM without a naming strategy
// convention override).
func (WebhookDeliveryPO) TableName() string { return "webhook_delivery" }

//go:generate mockgen -destination=mocks/webhook_delivery.go -package mocks . IWebhookDeliveryDAO
type IWebhookDeliveryDAO interface {
	Create(ctx context.Context, po *WebhookDeliveryPO, opts ...db.Option) error
	Update(ctx context.Context, po *WebhookDeliveryPO, opts ...db.Option) error
	GetByDeliveryID(ctx context.Context, deliveryID string, opts ...db.Option) (*WebhookDeliveryPO, error)
	ListByExptID(ctx context.Context, params repo.ListDeliveryParams, opts ...db.Option) ([]*WebhookDeliveryPO, int64, error)
	ListRetryable(ctx context.Context, params repo.ListRetryableParams, opts ...db.Option) ([]*WebhookDeliveryPO, error)
}

// NewWebhookDeliveryDAO is the constructor name commercial wire_gen already
// references (`mysql.NewWebhookDeliveryDAO(db2)`), so its signature is
// frozen: `db.Provider` in, `IWebhookDeliveryDAO` out.
func NewWebhookDeliveryDAO(provider db.Provider) IWebhookDeliveryDAO {
	return &webhookDeliveryDAO{provider: provider}
}

type webhookDeliveryDAO struct {
	provider db.Provider
}

func (d *webhookDeliveryDAO) session(ctx context.Context, opts ...db.Option) *gorm.DB {
	return d.provider.NewSession(ctx, opts...)
}

func (d *webhookDeliveryDAO) Create(ctx context.Context, po *WebhookDeliveryPO, opts ...db.Option) error {
	if err := d.session(ctx, opts...).Create(po).Error; err != nil {
		return errorx.Wrapf(err, "create webhook_delivery fail")
	}
	return nil
}

func (d *webhookDeliveryDAO) Update(ctx context.Context, po *WebhookDeliveryPO, opts ...db.Option) error {
	if err := d.session(ctx, opts...).
		Where("delivery_id = ?", po.DeliveryID).
		Updates(po).Error; err != nil {
		return errorx.Wrapf(err, "update webhook_delivery fail")
	}
	return nil
}

func (d *webhookDeliveryDAO) GetByDeliveryID(ctx context.Context, deliveryID string, opts ...db.Option) (*WebhookDeliveryPO, error) {
	var po WebhookDeliveryPO
	err := d.session(ctx, opts...).Where("delivery_id = ?", deliveryID).First(&po).Error
	if err != nil {
		return nil, errorx.Wrapf(err, "get webhook_delivery fail, id: %s", deliveryID)
	}
	return &po, nil
}

func (d *webhookDeliveryDAO) ListByExptID(ctx context.Context, params repo.ListDeliveryParams, opts ...db.Option) ([]*WebhookDeliveryPO, int64, error) {
	q := d.session(ctx, opts...).Model(&WebhookDeliveryPO{}).Where("experiment_id = ?", params.ExperimentID)
	if params.Event != "" {
		q = q.Where("event = ?", params.Event)
	}
	if params.Status != "" {
		q = q.Where("status = ?", params.Status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, errorx.Wrapf(err, "count webhook_delivery fail")
	}
	page, size := params.Page, params.Size
	if size <= 0 {
		size = 20
	}
	if page <= 0 {
		page = 1
	}
	var out []*WebhookDeliveryPO
	if err := q.Order("id DESC").Offset((page - 1) * size).Limit(size).Find(&out).Error; err != nil {
		return nil, 0, errorx.Wrapf(err, "list webhook_delivery fail")
	}
	return out, total, nil
}

func (d *webhookDeliveryDAO) ListRetryable(ctx context.Context, params repo.ListRetryableParams, opts ...db.Option) ([]*WebhookDeliveryPO, error) {
	q := d.session(ctx, opts...).Model(&WebhookDeliveryPO{}).
		Where("status IN ?", []string{entity.WebhookDeliveryStatusRetrying, entity.WebhookDeliveryStatusFailed}).
		Where("last_sent_at < ?", params.NotAfter)
	limit := params.Limit
	if limit <= 0 {
		limit = 100
	}
	var out []*WebhookDeliveryPO
	if err := q.Order("last_sent_at ASC").Limit(limit).Find(&out).Error; err != nil {
		return nil, errorx.Wrapf(err, "list retryable webhook_delivery fail")
	}
	return out, nil
}
