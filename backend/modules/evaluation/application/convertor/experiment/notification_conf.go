// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"strconv"
	"strings"

	"github.com/bytedance/gg/gptr"

	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	openapiExperiment "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// NotificationConfDTO2DO converts Thrift ExptNotificationConf DTO to entity NotificationConf DO.
func NotificationConfDTO2DO(dto *domain_expt.ExptNotificationConf) *entity.NotificationConf {
	if dto == nil {
		return nil
	}
	conf := &entity.NotificationConf{}

	if dto.Filter != nil {
		conf.Filter = notificationFilterDTO2DO(dto.Filter)
	}
	if dto.Webhook != nil {
		conf.Webhook = &entity.WebhookConf{
			Enable: dto.Webhook.GetEnable(),
			URLs:   dto.Webhook.GetUrls(),
			Secret: dto.Webhook.GetSecret(),
		}
	}
	if dto.FeishuNotification != nil {
		conf.FeishuNotification = &entity.FeishuNotificationConf{
			Enable: dto.FeishuNotification.GetEnable(),
		}
	}
	return conf
}

// NotificationConfDO2DTO converts entity NotificationConf DO to Thrift ExptNotificationConf DTO.
func NotificationConfDO2DTO(do *entity.NotificationConf) *domain_expt.ExptNotificationConf {
	if do == nil {
		return nil
	}
	dto := domain_expt.NewExptNotificationConf()

	if do.Filter != nil {
		dto.Filter = notificationFilterDO2DTO(do.Filter)
	}
	if do.Webhook != nil {
		dto.Webhook = &domain_expt.WebhookNotificationConf{
			Enable: do.Webhook.Enable,
			Urls:   gptr.Of(do.Webhook.URLs),
			Secret: gptr.Of(do.Webhook.Secret),
		}
	}
	if do.FeishuNotification != nil {
		dto.FeishuNotification = &domain_expt.FeishuNotificationConf{
			Enable: do.FeishuNotification.Enable,
		}
	}
	return dto
}

// notificationFilterDTO2DO converts Thrift Filters to entity ExptListFilter for notification context.
// In notification context, only ExptStatus filter conditions are relevant.
func notificationFilterDTO2DO(dto *domain_expt.Filters) *entity.ExptListFilter {
	if dto == nil {
		return nil
	}
	filter := &entity.ExptListFilter{
		Includes: &entity.ExptFilterFields{},
		Excludes: &entity.ExptFilterFields{},
	}
	for _, cond := range dto.GetFilterConditions() {
		if cond == nil || cond.GetField() == nil {
			continue
		}
		if cond.GetField().GetFieldType() != domain_expt.FieldType_ExptStatus {
			continue
		}
		if len(cond.GetValue()) == 0 {
			continue
		}
		statusValues := entity.ParseFilterConditionStatusValues(cond.GetValue())
		if len(statusValues) == 0 {
			continue
		}
		switch cond.GetOperator() {
		case domain_expt.FilterOperatorType_In, domain_expt.FilterOperatorType_Equal:
			filter.Includes.Status = append(filter.Includes.Status, statusValues...)
		case domain_expt.FilterOperatorType_NotIn, domain_expt.FilterOperatorType_NotEqual:
			filter.Excludes.Status = append(filter.Excludes.Status, statusValues...)
		}
	}
	return filter
}

// notificationFilterDO2DTO converts entity ExptListFilter to Thrift Filters for notification context.
// Only status fields are relevant for notification filters.
func notificationFilterDO2DTO(do *entity.ExptListFilter) *domain_expt.Filters {
	if do == nil {
		return nil
	}
	conditions := make([]*domain_expt.FilterCondition, 0)

	if do.Includes != nil && len(do.Includes.Status) > 0 {
		conditions = append(conditions, &domain_expt.FilterCondition{
			Field:    &domain_expt.FilterField{FieldType: domain_expt.FieldType_ExptStatus},
			Operator: domain_expt.FilterOperatorType_In,
			Value:    joinInt64Slice(do.Includes.Status),
		})
	}
	if do.Excludes != nil && len(do.Excludes.Status) > 0 {
		conditions = append(conditions, &domain_expt.FilterCondition{
			Field:    &domain_expt.FilterField{FieldType: domain_expt.FieldType_ExptStatus},
			Operator: domain_expt.FilterOperatorType_NotIn,
			Value:    joinInt64Slice(do.Excludes.Status),
		})
	}
	if len(conditions) == 0 {
		return nil
	}
	logicOp := domain_expt.FilterLogicOp_And
	return &domain_expt.Filters{
		FilterConditions: conditions,
		LogicOp:          &logicOp,
	}
}

// joinInt64Slice converts []int64 to a comma-separated string.
func joinInt64Slice(vals []int64) string {
	parts := make([]string, 0, len(vals))
	for _, v := range vals {
		parts = append(parts, strconv.FormatInt(v, 10))
	}
	return strings.Join(parts, ",")
}

// OpenAPINotificationConfDTO2Domain converts OpenAPI ExptNotificationConf to domain DTO ExptNotificationConf.
func OpenAPINotificationConfDTO2Domain(dto *openapiExperiment.ExptNotificationConf) *domain_expt.ExptNotificationConf {
	if dto == nil {
		return nil
	}
	result := domain_expt.NewExptNotificationConf()

	if dto.Filter != nil {
		result.Filter = openAPIFilterDTO2Domain(dto.Filter)
	}
	if dto.Webhook != nil {
		result.Webhook = &domain_expt.WebhookNotificationConf{
			Enable: dto.Webhook.GetEnable(),
			Urls:   gptr.Of(dto.Webhook.GetUrls()),
			Secret: gptr.Of(dto.Webhook.GetSecret()),
		}
	}
	if dto.FeishuNotification != nil {
		result.FeishuNotification = &domain_expt.FeishuNotificationConf{
			Enable: dto.FeishuNotification.GetEnable(),
		}
	}
	return result
}

// openAPIFilterDTO2Domain converts OpenAPI Filters to domain DTO Filters for notification context.
func openAPIFilterDTO2Domain(dto *openapiExperiment.Filters) *domain_expt.Filters {
	if dto == nil {
		return nil
	}
	conditions := make([]*domain_expt.FilterCondition, 0)
	for _, cond := range dto.GetFilterConditions() {
		if cond == nil || cond.GetField() == nil {
			continue
		}
		if cond.GetField().GetFieldType() != openapiExperiment.FilterFieldTypeExptStatus {
			continue
		}
		if cond.GetValue() == "" {
			continue
		}
		domainCond := &domain_expt.FilterCondition{
			Field: &domain_expt.FilterField{FieldType: domain_expt.FieldType_ExptStatus},
			Value: cond.GetValue(),
		}
		switch cond.GetOperator() {
		case openapiExperiment.FilterOperatorTypeIn:
			domainCond.Operator = domain_expt.FilterOperatorType_In
		case openapiExperiment.FilterOperatorTypeNotIn:
			domainCond.Operator = domain_expt.FilterOperatorType_NotIn
		case openapiExperiment.FilterOperatorTypeEqual:
			domainCond.Operator = domain_expt.FilterOperatorType_Equal
		case openapiExperiment.FilterOperatorTypeNotEqual:
			domainCond.Operator = domain_expt.FilterOperatorType_NotEqual
		default:
			continue
		}
		conditions = append(conditions, domainCond)
	}
	if len(conditions) == 0 {
		return nil
	}
	logicOp := domain_expt.FilterLogicOp_And
	return &domain_expt.Filters{
		FilterConditions: conditions,
		LogicOp:          &logicOp,
	}
}
