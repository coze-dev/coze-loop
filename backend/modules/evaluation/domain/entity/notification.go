// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// NotificationConf 实验通知配置（顶层）
type NotificationConf struct {
	Rules []*NotificationRule
}

// NotificationRule 单条通知规则
type NotificationRule struct {
	Condition *NotificationFilterCondition
	Webhook   *WebhookChannelConf
	Feishu    *FeishuChannelConf
}

// NotificationFilterCondition 通知过滤条件（field + operator + values）
type NotificationFilterCondition struct {
	Field    string
	Operator NotificationOperator
	Values   []ExptStatus // 复用已有的 ExptStatus 枚举
}

// NotificationOperator 通知过滤运算符
type NotificationOperator int

const (
	NotificationOperatorUnknown  NotificationOperator = 0
	NotificationOperatorIncludes NotificationOperator = 1
	NotificationOperatorExcludes NotificationOperator = 2
)

// WebhookChannelConf Webhook 渠道配置
type WebhookChannelConf struct {
	Enabled bool
	URLs    []string
}

// FeishuChannelConf 飞书渠道配置
type FeishuChannelConf struct {
	Enabled bool
}

// WebhookDeliveryPayload Webhook 投递负载
type WebhookDeliveryPayload struct {
	DeliveryID string
	Event      string // started/succeeded/failed/terminated
	Timestamp  string // ISO 8601
	Experiment *WebhookExperimentInfo
}

// WebhookExperimentInfo Webhook 中的实验信息
type WebhookExperimentInfo struct {
	ID       int64
	Name     string
	Status   string
	Progress *WebhookProgress
}

// WebhookProgress 实验进度统计
type WebhookProgress struct {
	Total     int32
	Succeeded int32
	Failed    int32
}

// WebhookDeliveryMessage MQ 投递消息
type WebhookDeliveryMessage struct {
	DeliveryID  string
	URL         string
	Payload     *WebhookDeliveryPayload
	RetryCount  int
	SpaceID     int64
	WorkspaceID int64
}
