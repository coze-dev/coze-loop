// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"github.com/bytedance/gg/gptr"

	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// NotificationConfigsDTO2DO converts a slice of kitex_gen NotificationConfig DTOs to domain entity DOs.
func NotificationConfigsDTO2DO(dtos []*domain_expt.NotificationConfig) []*entity.NotificationConfig {
	if len(dtos) == 0 {
		return nil
	}
	result := make([]*entity.NotificationConfig, 0, len(dtos))
	for _, dto := range dtos {
		if dto == nil {
			continue
		}
		result = append(result, notificationConfigDTO2DO(dto))
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func notificationConfigDTO2DO(dto *domain_expt.NotificationConfig) *entity.NotificationConfig {
	if dto == nil {
		return nil
	}
	return &entity.NotificationConfig{
		Trigger: notificationTriggerDTO2DO(dto.GetTrigger()),
		Actions: notificationActionsDTO2DO(dto.GetActions()),
	}
}

func notificationTriggerDTO2DO(dto *domain_expt.NotificationTrigger) *entity.NotificationTrigger {
	if dto == nil {
		return nil
	}
	return &entity.NotificationTrigger{
		Field:    dto.GetField(),
		Operator: dto.GetOperator(),
		Values:   dto.GetValues(),
	}
}

func notificationActionsDTO2DO(dtos []*domain_expt.NotificationAction) []*entity.NotificationAction {
	if len(dtos) == 0 {
		return nil
	}
	result := make([]*entity.NotificationAction, 0, len(dtos))
	for _, dto := range dtos {
		if dto == nil {
			continue
		}
		result = append(result, notificationActionDTO2DO(dto))
	}
	return result
}

func notificationActionDTO2DO(dto *domain_expt.NotificationAction) *entity.NotificationAction {
	if dto == nil {
		return nil
	}
	action := &entity.NotificationAction{
		Type: dto.GetType(),
	}
	if dto.GetWebhook() != nil {
		action.Webhook = &entity.WebhookAction{
			URL:    dto.GetWebhook().GetURL(),
			Secret: dto.GetWebhook().GetSecret(),
		}
	}
	if dto.GetFeishu() != nil {
		action.Feishu = &entity.FeishuAction{
			WebhookURL:      dto.GetFeishu().GetWebhookURL(),
			MessageTemplate: dto.GetFeishu().GetMessageTemplate(),
		}
	}
	return action
}

// NotificationConfigsDO2DTO converts a slice of domain entity NotificationConfig DOs to kitex_gen DTOs.
func NotificationConfigsDO2DTO(dos []*entity.NotificationConfig) []*domain_expt.NotificationConfig {
	if len(dos) == 0 {
		return nil
	}
	result := make([]*domain_expt.NotificationConfig, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		result = append(result, notificationConfigDO2DTO(do))
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func notificationConfigDO2DTO(do *entity.NotificationConfig) *domain_expt.NotificationConfig {
	if do == nil {
		return nil
	}
	dto := domain_expt.NewNotificationConfig()
	dto.Trigger = notificationTriggerDO2DTO(do.Trigger)
	dto.Actions = notificationActionsDO2DTO(do.Actions)
	return dto
}

func notificationTriggerDO2DTO(do *entity.NotificationTrigger) *domain_expt.NotificationTrigger {
	if do == nil {
		return nil
	}
	dto := domain_expt.NewNotificationTrigger()
	dto.Field = gptr.Of(do.Field)
	dto.Operator = gptr.Of(do.Operator)
	dto.Values = do.Values
	return dto
}

func notificationActionsDO2DTO(dos []*entity.NotificationAction) []*domain_expt.NotificationAction {
	if len(dos) == 0 {
		return nil
	}
	result := make([]*domain_expt.NotificationAction, 0, len(dos))
	for _, do := range dos {
		if do == nil {
			continue
		}
		result = append(result, notificationActionDO2DTO(do))
	}
	return result
}

func notificationActionDO2DTO(do *entity.NotificationAction) *domain_expt.NotificationAction {
	if do == nil {
		return nil
	}
	dto := domain_expt.NewNotificationAction()
	dto.Type = gptr.Of(do.Type)
	if do.Webhook != nil {
		dto.Webhook = &domain_expt.WebhookAction{
			URL:    gptr.Of(do.Webhook.URL),
			Secret: gptr.Of(do.Webhook.Secret),
		}
	}
	if do.Feishu != nil {
		dto.Feishu = &domain_expt.FeishuAction{
			WebhookURL:      gptr.Of(do.Feishu.WebhookURL),
			MessageTemplate: gptr.Of(do.Feishu.MessageTemplate),
		}
	}
	return dto
}
