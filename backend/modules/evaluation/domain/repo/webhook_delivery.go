// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// UpdateWebhookDeliveryStatusRequest webhook_delivery 状态机推进入参。
type UpdateWebhookDeliveryStatusRequest struct {
	DeliveryID       string
	Status           entity.DeliveryStatus
	AttemptCount     int32
	LastResponseCode int32
	LastError        string
	LastSentAt       *time.Time
	NextRetryAt      *time.Time
}

// IWebhookDeliveryRepo webhook_delivery 表 Repo 接口。Create 依赖 uk_delivery_id
// 保证幂等（重复 delivery_id 返 duplicate 语义供上层短路）；GetByDeliveryID 供 MQ
// retry 消费者判断是否短路；ListByExperimentID 供 ListWebhookDeliveriesOApi 分页倒序回显。
//
//go:generate mockgen -destination mocks/webhook_delivery_mock.go -package mocks . IWebhookDeliveryRepo
type IWebhookDeliveryRepo interface {
	Create(ctx context.Context, delivery *entity.WebhookDelivery) error
	GetByDeliveryID(ctx context.Context, deliveryID string) (*entity.WebhookDelivery, error)
	UpdateStatus(ctx context.Context, req *UpdateWebhookDeliveryStatusRequest) error
	ListByExperimentID(ctx context.Context, experimentID int64, pageSize int32, pageToken string) (deliveries []*entity.WebhookDelivery, nextPageToken string, err error)
}
