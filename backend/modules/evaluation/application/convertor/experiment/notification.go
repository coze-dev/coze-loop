// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"github.com/bytedance/gg/gptr"

	domainExpt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func NotificationConfDTO2DO(conf *domainExpt.ExptNotificationConf) (*entity.ExptNotificationConf, error) {
	if conf == nil {
		return nil, nil
	}
	res := &entity.ExptNotificationConf{
		Filter:             notificationFilterDTO2DO(conf.GetFilter()),
		Webhook:            webhookNotificationConfDTO2DO(conf.GetWebhook()),
		FeishuNotification: feishuNotificationConfDTO2DO(conf.GetFeishuNotification()),
	}
	if err := res.Validate(); err != nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg(err.Error()))
	}
	return res, nil
}

func NotificationConfDO2DTO(conf *entity.ExptNotificationConf) *domainExpt.ExptNotificationConf {
	if conf == nil {
		return nil
	}
	return &domainExpt.ExptNotificationConf{
		Filter:             notificationFilterDO2DTO(conf.Filter),
		Webhook:            webhookNotificationConfDO2DTO(conf.Webhook),
		FeishuNotification: feishuNotificationConfDO2DTO(conf.FeishuNotification),
	}
}

func notificationFilterDTO2DO(filter *domainExpt.Filters) *entity.NotificationFilter {
	if filter == nil {
		return nil
	}
	conditions := make([]*entity.NotificationFilterCondition, 0, len(filter.GetFilterConditions()))
	for _, cond := range filter.GetFilterConditions() {
		if cond == nil {
			continue
		}
		var field *entity.NotificationFilterField
		if cond.GetField() != nil {
			field = &entity.NotificationFilterField{
				FieldType: entity.FieldType(cond.GetField().GetFieldType()),
				FieldKey:  cond.GetField().GetFieldKey(),
			}
		}
		conditions = append(conditions, &entity.NotificationFilterCondition{
			Field:    field,
			Operator: entity.NotificationFilterOperatorType(cond.GetOperator()),
			Value:    cond.GetValue(),
		})
	}
	return &entity.NotificationFilter{FilterConditions: conditions}
}

func notificationFilterDO2DTO(filter *entity.NotificationFilter) *domainExpt.Filters {
	if filter == nil {
		return nil
	}
	conditions := make([]*domainExpt.FilterCondition, 0, len(filter.FilterConditions))
	for _, cond := range filter.FilterConditions {
		if cond == nil {
			continue
		}
		var field *domainExpt.FilterField
		if cond.Field != nil {
			field = &domainExpt.FilterField{
				FieldType: domainExpt.FieldType(cond.Field.FieldType),
				FieldKey:  gptr.Of(cond.Field.FieldKey),
			}
		}
		conditions = append(conditions, &domainExpt.FilterCondition{
			Field:    field,
			Operator: domainExpt.FilterOperatorType(cond.Operator),
			Value:    cond.Value,
		})
	}
	return &domainExpt.Filters{FilterConditions: conditions}
}

func webhookNotificationConfDTO2DO(conf *domainExpt.WebhookNotificationConf) *entity.WebhookNotificationConf {
	if conf == nil {
		return nil
	}
	return &entity.WebhookNotificationConf{
		Enable: conf.GetEnable(),
		URLs:   conf.GetUrls(),
	}
}

func webhookNotificationConfDO2DTO(conf *entity.WebhookNotificationConf) *domainExpt.WebhookNotificationConf {
	if conf == nil {
		return nil
	}
	return &domainExpt.WebhookNotificationConf{
		Enable: conf.Enable,
		Urls:   gptr.Of(conf.URLs),
	}
}

func feishuNotificationConfDTO2DO(conf *domainExpt.FeishuNotificationConf) *entity.FeishuNotificationConf {
	if conf == nil {
		return nil
	}
	return &entity.FeishuNotificationConf{
		Enable: conf.GetEnable(),
		UserID: conf.GetUserID(),
	}
}

func feishuNotificationConfDO2DTO(conf *entity.FeishuNotificationConf) *domainExpt.FeishuNotificationConf {
	if conf == nil {
		return nil
	}
	return &domainExpt.FeishuNotificationConf{
		Enable: conf.Enable,
		UserID: gptr.Of(conf.UserID),
	}
}
