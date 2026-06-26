// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// NotificationActionType 通知动作类型
type NotificationActionType = string

const (
	NotificationActionTypeWebhook NotificationActionType = "webhook"
	NotificationActionTypeFeishu  NotificationActionType = "feishu"
)

// WebhookAction Webhook 动作配置
type WebhookAction struct {
	URL    string `json:"url,omitempty"`
	Secret string `json:"secret,omitempty"`
}

// FeishuAction 飞书动作配置
type FeishuAction struct {
	WebhookURL      string `json:"webhook_url,omitempty"`
	MessageTemplate string `json:"message_template,omitempty"`
}

// NotificationTrigger 通知触发条件
type NotificationTrigger struct {
	Field    string   `json:"field,omitempty"`
	Operator string   `json:"operator,omitempty"`
	Values   []string `json:"values,omitempty"`
}

// NotificationAction 通知动作
type NotificationAction struct {
	Type    NotificationActionType `json:"type,omitempty"`
	Webhook *WebhookAction         `json:"webhook,omitempty"`
	Feishu  *FeishuAction          `json:"feishu,omitempty"`
}

// NotificationConfig 通知配置
type NotificationConfig struct {
	Trigger *NotificationTrigger `json:"trigger,omitempty"`
	Actions []*NotificationAction `json:"actions,omitempty"`
}

// WebhookDeliveryStatus Webhook 投递状态
type WebhookDeliveryStatus int64

const (
	WebhookDeliveryStatusPending    WebhookDeliveryStatus = 0
	WebhookDeliveryStatusDelivering WebhookDeliveryStatus = 1
	WebhookDeliveryStatusSuccess    WebhookDeliveryStatus = 2
	WebhookDeliveryStatusFailed     WebhookDeliveryStatus = 3
	WebhookDeliveryStatusRetrying   WebhookDeliveryStatus = 4
)

// WebhookDeliveryEvent Webhook 投递事件（MQ 消息体）
type WebhookDeliveryEvent struct {
	DeliveryID       string `json:"delivery_id,omitempty"`
	WorkspaceID      int64  `json:"workspace_id"`
	ExperimentID     int64  `json:"experiment_id"`
	ExperimentName   string `json:"experiment_name,omitempty"`
	FromStatus       string `json:"from_status,omitempty"`
	ToStatus         string `json:"to_status,omitempty"`
	WebhookURL       string `json:"webhook_url,omitempty"`
	Secret           string `json:"secret,omitempty"`
	RetryCount       int32  `json:"retry_count,omitempty"`
	MaxRetries       int32  `json:"max_retries,omitempty"`
	CreatedAt        int64  `json:"created_at"`
	ActionType       string `json:"action_type,omitempty"`
	FeishuWebhookURL string `json:"feishu_webhook_url,omitempty"`
	MessageTemplate  string `json:"message_template,omitempty"`
}
