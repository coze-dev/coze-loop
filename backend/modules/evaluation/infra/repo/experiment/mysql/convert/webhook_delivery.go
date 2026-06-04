// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convert

import (
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func WebhookDeliveryPO2DO(po *model.WebhookDelivery) *entity.WebhookDelivery {
	if po == nil {
		return nil
	}
	delivery := &entity.WebhookDelivery{
		ID:           po.ID,
		SpaceID:      po.SpaceID,
		ExptID:       po.ExptID,
		DeliveryID:   po.DeliveryID,
		EventType:    entity.WebhookEventType(po.EventType),
		ChannelType:  po.ChannelType,
		WebhookURL:   po.WebhookURL,
		Status:       entity.DeliveryStatus(po.Status),
		AttemptCount: int(po.AttemptCount),
		MaxAttempts:  int(po.MaxAttempts),
		FirstSentAt:  po.FirstSentAt,
		LastSentAt:   po.LastSentAt,
		NextRetryAt:  po.NextRetryAt,
		ErrorMessage: po.ErrorMessage,
		CreatedBy:    po.CreatedBy,
		UpdatedBy:    po.UpdatedBy,
		CreatedAt:    po.CreatedAt,
		UpdatedAt:    po.UpdatedAt,
	}
	if po.ResponseCode != nil {
		responseCode := int(*po.ResponseCode)
		delivery.ResponseCode = &responseCode
	}
	return delivery
}

func WebhookDeliveryDO2PO(delivery *entity.WebhookDelivery) *model.WebhookDelivery {
	if delivery == nil {
		return nil
	}
	po := &model.WebhookDelivery{
		ID:           delivery.ID,
		SpaceID:      delivery.SpaceID,
		ExptID:       delivery.ExptID,
		DeliveryID:   delivery.DeliveryID,
		EventType:    string(delivery.EventType),
		ChannelType:  delivery.ChannelType,
		WebhookURL:   delivery.WebhookURL,
		Status:       string(delivery.Status),
		AttemptCount: int32(delivery.AttemptCount),
		MaxAttempts:  int32(delivery.MaxAttempts),
		FirstSentAt:  delivery.FirstSentAt,
		LastSentAt:   delivery.LastSentAt,
		NextRetryAt:  delivery.NextRetryAt,
		ErrorMessage: delivery.ErrorMessage,
		CreatedBy:    delivery.CreatedBy,
		UpdatedBy:    delivery.UpdatedBy,
		CreatedAt:    delivery.CreatedAt,
		UpdatedAt:    delivery.UpdatedAt,
	}
	if delivery.ResponseCode != nil {
		responseCode := int32(*delivery.ResponseCode)
		po.ResponseCode = &responseCode
	}
	return po
}
