// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func NewWebhookDeliveryConverter() WebhookDeliveryConverter {
	return WebhookDeliveryConverter{}
}

type WebhookDeliveryConverter struct{}

func (WebhookDeliveryConverter) DO2PO(d *entity.WebhookDelivery) *model.WebhookDelivery {
	po := &model.WebhookDelivery{
		ID:             d.ID,
		SpaceID:        d.SpaceID,
		DeliveryID:     d.DeliveryID,
		ExperimentID:   d.ExperimentID,
		EventType:      string(d.EventType),
		WebhookURL:     d.WebhookURL,
		Status:         int32(d.Status),
		RetryCount:     d.RetryCount,
		LastStatusCode: d.LastStatusCode,
		ErrorMessage:   d.ErrorMessage,
		NextRetryAt:    d.NextRetryAt,
		FirstSentAt:    d.FirstSentAt,
		LastSentAt:     d.LastSentAt,
		CreatedAt:      d.CreatedAt,
		UpdatedAt:      d.UpdatedAt,
	}
	if len(d.RequestHeaders) > 0 {
		po.RequestHeaders = gptr.Of(d.RequestHeaders)
	}
	return po
}

func (WebhookDeliveryConverter) PO2DO(po *model.WebhookDelivery) *entity.WebhookDelivery {
	d := &entity.WebhookDelivery{
		ID:             po.ID,
		SpaceID:        po.SpaceID,
		DeliveryID:     po.DeliveryID,
		ExperimentID:   po.ExperimentID,
		EventType:      entity.WebhookEventType(po.EventType),
		WebhookURL:     po.WebhookURL,
		Status:         entity.DeliveryStatus(po.Status),
		RetryCount:     po.RetryCount,
		LastStatusCode: po.LastStatusCode,
		ErrorMessage:   po.ErrorMessage,
		NextRetryAt:    po.NextRetryAt,
		FirstSentAt:    po.FirstSentAt,
		LastSentAt:     po.LastSentAt,
		CreatedAt:      po.CreatedAt,
		UpdatedAt:      po.UpdatedAt,
	}
	if po.RequestHeaders != nil {
		d.RequestHeaders = *po.RequestHeaders
	}
	return d
}
