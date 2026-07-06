// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"github.com/bytedance/gg/gptr"

	domainExpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func WebhookDeliveryDO2DTO(d *entity.WebhookDelivery) *domainExpt.WebhookDelivery {
	if d == nil {
		return nil
	}
	dto := &domainExpt.WebhookDelivery{
		DeliveryID:   gptr.Of(d.DeliveryID),
		EventType:    gptr.Of(string(d.EventType)),
		ChannelType:  gptr.Of(d.ChannelType),
		WebhookURL:   gptr.Of(d.WebhookURL),
		Status:       gptr.Of(string(d.Status)),
		AttemptCount: gptr.Of(int32(d.AttemptCount)),
		MaxAttempts:  gptr.Of(int32(d.MaxAttempts)),
		ErrorMessage: gptr.Of(d.ErrorMessage),
	}
	if d.FirstSentAt != nil {
		dto.FirstSentAtMs = gptr.Of(d.FirstSentAt.UnixMilli())
	}
	if d.LastSentAt != nil {
		dto.LastSentAtMs = gptr.Of(d.LastSentAt.UnixMilli())
	}
	if d.ResponseCode != nil {
		dto.ResponseCode = gptr.Of(int32(*d.ResponseCode))
	}
	return dto
}

func WebhookDeliveryDO2DTOs(ds []*entity.WebhookDelivery) []*domainExpt.WebhookDelivery {
	if len(ds) == 0 {
		return nil
	}
	out := make([]*domainExpt.WebhookDelivery, 0, len(ds))
	for _, d := range ds {
		out = append(out, WebhookDeliveryDO2DTO(d))
	}
	return out
}
