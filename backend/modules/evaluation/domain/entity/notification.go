// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// ExptNotificationConf 实验通知配置
type ExptNotificationConf struct {
	// 触发条件（复用 Filters 组件结构）
	Filter *NotificationFilter `json:"filter,omitempty"`
	// Webhook 渠道配置
	Webhook *WebhookNotificationConf `json:"webhook,omitempty"`
	// 飞书渠道配置
	FeishuNotification *FeishuNotificationConf `json:"feishu_notification,omitempty"`
}

// NotificationFilter 通知触发条件
type NotificationFilter struct {
	FilterConditions []*NotificationFilterCondition `json:"filter_conditions,omitempty"`
	LogicOp          *FilterLogicOp                 `json:"logic_op,omitempty"`
}

// NotificationFilterCondition 通知过滤条件项
type NotificationFilterCondition struct {
	Field    *NotificationFilterField `json:"field,omitempty"`
	Operator NotificationOperatorType `json:"operator"`
	Value    string                   `json:"value,omitempty"`
}

// NotificationFilterField 通知过滤字段
type NotificationFilterField struct {
	FieldType NotificationFieldType `json:"field_type"`
	FieldKey  *string               `json:"field_key,omitempty"`
}

// NotificationFieldType 通知过滤字段类型
type NotificationFieldType int64

const (
	NotificationFieldType_Unknown    NotificationFieldType = 0
	NotificationFieldType_ExptStatus NotificationFieldType = 3 // 实验状态
)

// NotificationOperatorType 通知过滤操作符
type NotificationOperatorType int64

const (
	NotificationOperatorType_Unknown  NotificationOperatorType = 0
	NotificationOperatorType_Equal    NotificationOperatorType = 1 // 等于
	NotificationOperatorType_NotEqual NotificationOperatorType = 2 // 不等于
	NotificationOperatorType_In       NotificationOperatorType = 7 // 包含于
	NotificationOperatorType_NotIn    NotificationOperatorType = 8 // 不包含于
)

// WebhookNotificationConf Webhook 通知配置
type WebhookNotificationConf struct {
	Enable bool    `json:"enable"`
	Urls   *string `json:"urls,omitempty"`
}

// FeishuNotificationConf 飞书通知配置
type FeishuNotificationConf struct {
	Enable bool    `json:"enable"`
	UserID *string `json:"user_id,omitempty"`
}
