// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"strconv"
	"strings"

	"github.com/bytedance/gg/gptr"

	domain_expt "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// 通知 trigger 简写 → ExptStatus 集合映射（裁决①/②）。
// 对外 SDK/CLI 使用 {"trigger":"succeeded","actions":[...]} 形式；
// 内部统一映射成 FilterCondition（FieldType=ExptStatus，value=逗号分隔状态整数）。
//   - started   = 开始执行 = Processing(3)
//   - succeeded = 运行成功 = Success(11)
//   - failed    = 运行失败 = Failed(12)
//   - terminated= 被终止   = Terminated(13) + SystemTerminated(14)
var notificationTriggerStatusMap = map[string][]entity.ExptStatus{
	"started":    {entity.ExptStatus_Processing},
	"succeeded":  {entity.ExptStatus_Success},
	"failed":     {entity.ExptStatus_Failed},
	"terminated": {entity.ExptStatus_Terminated, entity.ExptStatus_SystemTerminated},
}

// NotificationTriggerToStatusValues 将 trigger 简写映射为 ExptStatus 整数集合（裁决①映射层）。
// 未识别的 trigger 返回 nil。
func NotificationTriggerToStatusValues(trigger string) []entity.ExptStatus {
	statuses, ok := notificationTriggerStatusMap[strings.ToLower(strings.TrimSpace(trigger))]
	if !ok {
		return nil
	}
	return statuses
}

// EncodeExptStatusValues 将 ExptStatus 集合编码为逗号分隔整数串（裁决②，复用既有 ExptStatus 过滤编码约定）。
func EncodeExptStatusValues(statuses []entity.ExptStatus) string {
	if len(statuses) == 0 {
		return ""
	}
	parts := make([]string, 0, len(statuses))
	for _, s := range statuses {
		parts = append(parts, strconv.FormatInt(int64(s), 10))
	}
	return strings.Join(parts, ",")
}

// NotificationConfDTO2DO 将 IDL 的 ExptNotificationConf（FilterCondition 模型）转换为领域 DO。
// 解析 FilterCondition：仅取 FieldType=ExptStatus 的条件，value 走 parseIntList（裁决②）。
// 为 nil 返回 nil（向前兼容，走默认终态飞书）。
func NotificationConfDTO2DO(dto *domain_expt.ExptNotificationConf) (*entity.NotificationConf, error) {
	if dto == nil {
		return nil, nil
	}
	conf := &entity.NotificationConf{}
	for _, rule := range dto.GetRules() {
		if rule == nil {
			continue
		}
		doRule := &entity.NotificationRule{}

		if filters := rule.GetFilters(); filters != nil {
			for _, cond := range filters.GetFilterConditions() {
				if cond == nil {
					continue
				}
				// 本期通知规则筛选字段固定为实验状态
				if cond.GetField() == nil || cond.GetField().GetFieldType() != domain_expt.FieldType_ExptStatus {
					continue
				}
				op := mapFilterOperatorDTO2DO(cond.GetOperator())
				if op == entity.NotificationFilterOperator_Unknown {
					continue
				}
				var statusValues []entity.ExptStatus
				if v := strings.TrimSpace(cond.GetValue()); v != "" {
					ints, err := parseIntList(v)
					if err != nil {
						return nil, err
					}
					for _, i := range ints {
						statusValues = append(statusValues, entity.ExptStatus(i))
					}
				}
				doRule.Conditions = append(doRule.Conditions, &entity.NotificationFilterCondition{
					Operator:     op,
					StatusValues: statusValues,
				})
			}
		}

		for _, action := range rule.GetActions() {
			if action == nil {
				continue
			}
			doAction := &entity.NotificationAction{
				Type: entity.NotificationActionType(action.GetType()),
			}
			switch action.GetType() {
			case domain_expt.NotificationActionType_Webhook:
				if wh := action.GetWebhook(); wh != nil {
					doAction.Webhook = &entity.NotificationWebhookConf{URLs: wh.GetUrls()}
				} else {
					doAction.Webhook = &entity.NotificationWebhookConf{}
				}
			case domain_expt.NotificationActionType_Feishu:
				doAction.Feishu = &entity.NotificationFeishuConf{}
			}
			doRule.Actions = append(doRule.Actions, doAction)
		}

		conf.Rules = append(conf.Rules, doRule)
	}
	return conf, nil
}

// NotificationConfDO2DTO 将领域 DO 转换回 IDL ExptNotificationConf（用于响应回显）。为 nil 返回 nil。
func NotificationConfDO2DTO(do *entity.NotificationConf) *domain_expt.ExptNotificationConf {
	if do == nil {
		return nil
	}
	dto := &domain_expt.ExptNotificationConf{}
	for _, rule := range do.Rules {
		if rule == nil {
			continue
		}
		dtoRule := &domain_expt.NotificationRule{}

		if len(rule.Conditions) > 0 {
			filters := &domain_expt.Filters{LogicOp: gptr.Of(domain_expt.FilterLogicOp_And)}
			for _, cond := range rule.Conditions {
				if cond == nil {
					continue
				}
				filters.FilterConditions = append(filters.FilterConditions, &domain_expt.FilterCondition{
					Field:    &domain_expt.FilterField{FieldType: domain_expt.FieldType_ExptStatus},
					Operator: mapFilterOperatorDO2DTO(cond.Operator),
					Value:    EncodeExptStatusValues(cond.StatusValues),
				})
			}
			dtoRule.Filters = filters
		}

		for _, action := range rule.Actions {
			if action == nil {
				continue
			}
			dtoAction := &domain_expt.NotificationAction{
				Type: domain_expt.NotificationActionType(action.Type),
			}
			switch action.Type {
			case entity.NotificationActionType_Webhook:
				wh := &domain_expt.WebhookNotificationConf{}
				if action.Webhook != nil {
					wh.Urls = action.Webhook.URLs
				}
				dtoAction.Webhook = wh
			case entity.NotificationActionType_Feishu:
				dtoAction.Feishu = &domain_expt.FeishuNotificationConf{}
			}
			dtoRule.Actions = append(dtoRule.Actions, dtoAction)
		}

		dto.Rules = append(dto.Rules, dtoRule)
	}
	return dto
}

func mapFilterOperatorDTO2DO(op domain_expt.FilterOperatorType) entity.NotificationFilterOperator {
	switch op {
	case domain_expt.FilterOperatorType_In:
		return entity.NotificationFilterOperator_In
	case domain_expt.FilterOperatorType_NotIn:
		return entity.NotificationFilterOperator_NotIn
	default:
		return entity.NotificationFilterOperator_Unknown
	}
}

func mapFilterOperatorDO2DTO(op entity.NotificationFilterOperator) domain_expt.FilterOperatorType {
	switch op {
	case entity.NotificationFilterOperator_In:
		return domain_expt.FilterOperatorType_In
	case entity.NotificationFilterOperator_NotIn:
		return domain_expt.FilterOperatorType_NotIn
	default:
		return domain_expt.FilterOperatorType_Unknown
	}
}
