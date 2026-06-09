// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"github.com/bytedance/gg/gptr"

	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// notificationConfigDTOToEntity converts IDL NotificationConfig to domain entity.
func notificationConfigDTOToEntity(dto *domain_expt.NotificationConfig) *entity.NotificationConfig {
	if dto == nil {
		return nil
	}
	nc := &entity.NotificationConfig{}

	if dto.IsSetCondition() {
		cond := dto.GetCondition()
		nc.Condition = &entity.NotificationCondition{
			Field: cond.GetField(),
			Values: cond.GetValues(),
		}
		// Map FilterOperatorType to string
		switch cond.GetOperator() {
		case domain_expt.FilterOperatorType_In:
			nc.Condition.Operator = "in"
		case domain_expt.FilterOperatorType_NotIn:
			nc.Condition.Operator = "not_in"
		default:
			nc.Condition.Operator = "in"
		}
	}

	if dto.IsSetWebhookChannel() {
		wc := dto.GetWebhookChannel()
		nc.WebhookChannel = &entity.WebhookChannelConfig{
			Enabled: wc.GetEnabled(),
			URLs:    wc.GetUrls(),
		}
	}

	if dto.IsSetLarkChannel() {
		lc := dto.GetLarkChannel()
		nc.LarkChannel = &entity.LarkChannelConfig{
			Enabled: lc.GetEnabled(),
		}
	}

	return nc
}

// notificationConfigEntityToDTO converts domain entity NotificationConfig to IDL DTO.
func notificationConfigEntityToDTO(nc *entity.NotificationConfig) *domain_expt.NotificationConfig {
	if nc == nil {
		return nil
	}
	dto := domain_expt.NewNotificationConfig()

	if nc.Condition != nil {
		cond := domain_expt.NewNotificationCondition()
		cond.Field = gptr.Of(nc.Condition.Field)
		cond.Values = nc.Condition.Values
		switch nc.Condition.Operator {
		case "in":
			cond.Operator = gptr.Of(domain_expt.FilterOperatorType_In)
		case "not_in":
			cond.Operator = gptr.Of(domain_expt.FilterOperatorType_NotIn)
		default:
			cond.Operator = gptr.Of(domain_expt.FilterOperatorType_In)
		}
		dto.Condition = cond
	}

	if nc.WebhookChannel != nil {
		wc := domain_expt.NewWebhookChannelConfig()
		wc.Enabled = gptr.Of(nc.WebhookChannel.Enabled)
		wc.Urls = nc.WebhookChannel.URLs
		dto.WebhookChannel = wc
	}

	if nc.LarkChannel != nil {
		lc := domain_expt.NewLarkChannelConfig()
		lc.Enabled = gptr.Of(nc.LarkChannel.Enabled)
		dto.LarkChannel = lc
	}

	return dto
}
