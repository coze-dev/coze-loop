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

// 实验通知配置（notification_conf）DTO ↔ entity 转换。
//
// 触发条件复用既有 Filters / FilterCondition 通用模型（不另起枚举）：
//   - field.field_type = ExptStatus(3)（本期固定，不可切换）
//   - operator = In(7) / NotIn(8)
//   - value = 面向用户的条件值多选（NotificationStatusValue：开始执行/运行成功/运行失败/被终止），
//     编码为逗号分隔的整数字符串（如 "1,2,3"）。
//
// 设计依据：specs/experiment-webhook-notification-v6 backend/design.md D3/D6、
// capability spec experiment-notification-config。
//
// 全部转换 null-safe：nil 入参返回 nil（由 entity 层 GetNotificationConfOrDefault 兜底默认行为，
// 保证历史实验/模板零迁移、向后兼容）。

const (
	// notificationFieldKeyExptStatus OpenAPI filter field_type 的字符串值（与
	// openAPIExperimentFilterFieldTypeToDomain 中 "expt_status" 对齐）。
	notificationFieldKeyExptStatus = "expt_status"
	// notificationOperatorIn / notificationOperatorNotIn OpenAPI operator 字符串值
	// （与 openAPIExperimentFilterOperatorToDomain 中 "in"/"not_in" 对齐）。
	notificationOperatorIn    = "in"
	notificationOperatorNotIn = "not_in"
	// notificationLogicOpAnd OpenAPI filter logic_op 字符串值（本期单 condition，固定 and）。
	notificationLogicOpAnd = "and"
)

// encodeNotificationStatusValues 将面向用户的条件值集合编码为逗号分隔整数字符串。
func encodeNotificationStatusValues(values []entity.NotificationStatusValue) string {
	if len(values) == 0 {
		return ""
	}
	parts := make([]string, 0, len(values))
	for _, v := range values {
		parts = append(parts, strconv.FormatInt(int64(v), 10))
	}
	return strings.Join(parts, ",")
}

// decodeNotificationStatusValues 解析逗号分隔整数字符串为条件值集合，忽略空项/非法项。
func decodeNotificationStatusValues(s string) []entity.NotificationStatusValue {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	values := make([]entity.NotificationStatusValue, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			continue
		}
		values = append(values, entity.NotificationStatusValue(n))
	}
	if len(values) == 0 {
		return nil
	}
	return values
}

// notificationOperatorDO2DTO entity 运算符 → OpenAPI 字符串运算符。
func notificationOperatorDO2DTO(op entity.NotificationFilterOperatorType) string {
	switch op {
	case entity.NotificationFilterOperatorType_NotIn:
		return notificationOperatorNotIn
	default:
		// 默认 In（含未识别运算符兜底，避免输出非法 operator）。
		return notificationOperatorIn
	}
}

// notificationOperatorFromDomainEnum domain FilterOperatorType(int) → entity 通知运算符。
// 仅识别 In/NotIn，其余兜底为 In（本期通知条件仅这两种运算符）。
func notificationOperatorFromDomainEnum(op domain_expt.FilterOperatorType) entity.NotificationFilterOperatorType {
	if op == domain_expt.FilterOperatorType_NotIn {
		return entity.NotificationFilterOperatorType_NotIn
	}
	return entity.NotificationFilterOperatorType_In
}

// notificationOperatorFromOpenAPIString OpenAPI operator 字符串 → entity 通知运算符。
func notificationOperatorFromOpenAPIString(op string) entity.NotificationFilterOperatorType {
	if strings.ToLower(strings.TrimSpace(op)) == notificationOperatorNotIn {
		return entity.NotificationFilterOperatorType_NotIn
	}
	return entity.NotificationFilterOperatorType_In
}

// ---------- 内部 RPC（domain/expt）DTO ↔ entity ----------

// NotificationConfDTO2DO 将内部 RPC 的 domain_expt.ExptNotificationConf 转为 entity.NotificationConf。
// 用于 SubmitExperimentRequest / CreateExperimentRequest / CreateExperimentTemplateRequest 入参解析。
func NotificationConfDTO2DO(dto *domain_expt.ExptNotificationConf) *entity.NotificationConf {
	if dto == nil {
		return nil
	}
	conf := &entity.NotificationConf{
		Filter:  notificationFilterDTO2DO(dto.GetFilter()),
		Webhook: webhookNotificationConfDTO2DO(dto.GetWebhook()),
		Feishu:  feishuNotificationConfDTO2DO(dto.GetFeishu()),
	}
	return conf
}

// notificationFilterDTO2DO 取 Filters 的首条 ExptStatus condition 转为 entity 单条通知条件。
// 本期仅承载单 condition；忽略非 ExptStatus 字段的 condition。
func notificationFilterDTO2DO(filters *domain_expt.Filters) *entity.NotificationFilterCondition {
	if filters == nil {
		return nil
	}
	for _, cond := range filters.GetFilterConditions() {
		if cond == nil || cond.GetField() == nil {
			continue
		}
		if cond.GetField().GetFieldType() != domain_expt.FieldType_ExptStatus {
			continue
		}
		return &entity.NotificationFilterCondition{
			FieldType: entity.FieldType_ExptStatus,
			Operator:  notificationOperatorFromDomainEnum(cond.GetOperator()),
			Values:    decodeNotificationStatusValues(cond.GetValue()),
		}
	}
	return nil
}

func webhookNotificationConfDTO2DO(dto *domain_expt.WebhookNotificationConf) *entity.WebhookNotificationConf {
	if dto == nil {
		return nil
	}
	return &entity.WebhookNotificationConf{
		Enable: dto.GetEnable(),
		URLs:   dto.GetUrls(),
	}
}

func feishuNotificationConfDTO2DO(dto *domain_expt.FeishuNotificationConf) *entity.FeishuNotificationConf {
	if dto == nil {
		return nil
	}
	return &entity.FeishuNotificationConf{
		Enable: dto.GetEnable(),
	}
}

// NotificationConfDO2DTO 将 entity.NotificationConf 转为内部 RPC 的 domain_expt.ExptNotificationConf。
// 用于实验/模板查询出参（Experiment / ExptTemplate DTO）。
func NotificationConfDO2DTO(conf *entity.NotificationConf) *domain_expt.ExptNotificationConf {
	if conf == nil {
		return nil
	}
	dto := &domain_expt.ExptNotificationConf{
		Filter:  notificationFilterDO2DTO(conf.Filter),
		Webhook: webhookNotificationConfDO2DTO(conf.Webhook),
		Feishu:  feishuNotificationConfDO2DTO(conf.Feishu),
	}
	return dto
}

func notificationFilterDO2DTO(cond *entity.NotificationFilterCondition) *domain_expt.Filters {
	if cond == nil {
		return nil
	}
	var op domain_expt.FilterOperatorType
	if cond.Operator == entity.NotificationFilterOperatorType_NotIn {
		op = domain_expt.FilterOperatorType_NotIn
	} else {
		op = domain_expt.FilterOperatorType_In
	}
	return &domain_expt.Filters{
		LogicOp: gptr.Of(domain_expt.FilterLogicOp_And),
		FilterConditions: []*domain_expt.FilterCondition{
			{
				Field: &domain_expt.FilterField{
					FieldType: domain_expt.FieldType_ExptStatus,
				},
				Operator: op,
				Value:    encodeNotificationStatusValues(cond.Values),
			},
		},
	}
}

func webhookNotificationConfDO2DTO(conf *entity.WebhookNotificationConf) *domain_expt.WebhookNotificationConf {
	if conf == nil {
		return nil
	}
	return &domain_expt.WebhookNotificationConf{
		Enable: gptr.Of(conf.Enable),
		Urls:   conf.URLs,
	}
}

func feishuNotificationConfDO2DTO(conf *entity.FeishuNotificationConf) *domain_expt.FeishuNotificationConf {
	if conf == nil {
		return nil
	}
	return &domain_expt.FeishuNotificationConf{
		Enable: gptr.Of(conf.Enable),
	}
}

// ---------- OpenAPI（domain_openapi/experiment）DTO ↔ entity ----------

// OpenAPINotificationConfDTO2DO 将 OpenAPI 的 ExptNotificationConf 转为 entity.NotificationConf。
// 用于 SubmitExperimentOApiRequest / CreateExptTemplateOApiRequest 入参解析。
func OpenAPINotificationConfDTO2DO(dto *openapiExperiment.ExptNotificationConf) *entity.NotificationConf {
	if dto == nil {
		return nil
	}
	return &entity.NotificationConf{
		Filter:  openAPINotificationFilterDTO2DO(dto.GetFilter()),
		Webhook: openAPIWebhookNotificationConfDTO2DO(dto.GetWebhook()),
		Feishu:  openAPIFeishuNotificationConfDTO2DO(dto.GetFeishu()),
	}
}

func openAPINotificationFilterDTO2DO(filters *openapiExperiment.Filters) *entity.NotificationFilterCondition {
	if filters == nil {
		return nil
	}
	for _, cond := range filters.GetFilterConditions() {
		if cond == nil || cond.GetField() == nil {
			continue
		}
		if strings.ToLower(strings.TrimSpace(cond.GetField().GetFieldType())) != notificationFieldKeyExptStatus {
			continue
		}
		return &entity.NotificationFilterCondition{
			FieldType: entity.FieldType_ExptStatus,
			Operator:  notificationOperatorFromOpenAPIString(cond.GetOperator()),
			Values:    decodeNotificationStatusValues(cond.GetValue()),
		}
	}
	return nil
}

func openAPIWebhookNotificationConfDTO2DO(dto *openapiExperiment.WebhookNotificationConf) *entity.WebhookNotificationConf {
	if dto == nil {
		return nil
	}
	return &entity.WebhookNotificationConf{
		Enable: dto.GetEnable(),
		URLs:   dto.GetUrls(),
	}
}

func openAPIFeishuNotificationConfDTO2DO(dto *openapiExperiment.FeishuNotificationConf) *entity.FeishuNotificationConf {
	if dto == nil {
		return nil
	}
	return &entity.FeishuNotificationConf{
		Enable: dto.GetEnable(),
	}
}

// OpenAPINotificationConfDO2DTO 将 entity.NotificationConf 转为 OpenAPI 的 ExptNotificationConf。
// 用于 OpenAPI 实验/模板查询出参。
func OpenAPINotificationConfDO2DTO(conf *entity.NotificationConf) *openapiExperiment.ExptNotificationConf {
	if conf == nil {
		return nil
	}
	return &openapiExperiment.ExptNotificationConf{
		Filter:  openAPINotificationFilterDO2DTO(conf.Filter),
		Webhook: openAPIWebhookNotificationConfDO2DTO(conf.Webhook),
		Feishu:  openAPIFeishuNotificationConfDO2DTO(conf.Feishu),
	}
}

func openAPINotificationFilterDO2DTO(cond *entity.NotificationFilterCondition) *openapiExperiment.Filters {
	if cond == nil {
		return nil
	}
	return &openapiExperiment.Filters{
		LogicOp: gptr.Of(notificationLogicOpAnd),
		FilterConditions: []*openapiExperiment.FilterCondition{
			{
				Field: &openapiExperiment.FilterField{
					FieldType: gptr.Of(notificationFieldKeyExptStatus),
				},
				Operator: gptr.Of(notificationOperatorDO2DTO(cond.Operator)),
				Value:    gptr.Of(encodeNotificationStatusValues(cond.Values)),
			},
		},
	}
}

func openAPIWebhookNotificationConfDO2DTO(conf *entity.WebhookNotificationConf) *openapiExperiment.WebhookNotificationConf {
	if conf == nil {
		return nil
	}
	return &openapiExperiment.WebhookNotificationConf{
		Enable: gptr.Of(conf.Enable),
		Urls:   conf.URLs,
	}
}

func openAPIFeishuNotificationConfDO2DTO(conf *entity.FeishuNotificationConf) *openapiExperiment.FeishuNotificationConf {
	if conf == nil {
		return nil
	}
	return &openapiExperiment.FeishuNotificationConf{
		Enable: gptr.Of(conf.Enable),
	}
}

// ---------- OpenAPI → 内部 RPC（domain/expt）桥接 ----------

// OpenAPINotificationConfDTO2DomainDTO 将 OpenAPI 入参的 ExptNotificationConf 桥接为内部 RPC 的
// domain_expt.ExptNotificationConf（OpenAPI → entity → domain DTO）。
// 用于 SubmitExperimentOApi 把 OpenAPI 通知配置转交内部 SubmitExperiment 接口。null-safe。
func OpenAPINotificationConfDTO2DomainDTO(dto *openapiExperiment.ExptNotificationConf) *domain_expt.ExptNotificationConf {
	if dto == nil {
		return nil
	}
	return NotificationConfDO2DTO(OpenAPINotificationConfDTO2DO(dto))
}

