// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import "time"

// NotificationTrigger 通知触发条件
type NotificationTrigger = string

const (
	NotificationTriggerStarted    NotificationTrigger = "started"
	NotificationTriggerSucceeded  NotificationTrigger = "succeeded"
	NotificationTriggerFailed     NotificationTrigger = "failed"
	NotificationTriggerTerminated NotificationTrigger = "terminated"
)

// NotificationActionType 通知动作类型
type NotificationActionType = string

const (
	NotificationActionTypeWebhook NotificationActionType = "webhook"
	NotificationActionTypeFeishu  NotificationActionType = "feishu"
)

// NotificationRule 通知规则领域实体
type NotificationRule struct {
	Trigger NotificationTrigger  `json:"trigger,omitempty"`
	Actions []*NotificationAction `json:"actions,omitempty"`
}

// NotificationAction 通知动作
type NotificationAction struct {
	Type NotificationActionType `json:"type,omitempty"`
	URL  string                 `json:"url,omitempty"` // webhook 类型时必填
}

// WebhookDelivery 表示一次 Webhook 投递
type WebhookDelivery struct {
	DeliveryID string `json:"delivery_id"`
	ExptID     int64  `json:"expt_id"`
	SpaceID    int64  `json:"space_id"`
	Event      string `json:"event"`
	URL        string `json:"url"`
	Payload    string `json:"payload"`
	Signature  string `json:"signature"`
	Timestamp  int64  `json:"timestamp"`
	RetryCount int    `json:"retry_count"`
	MaxRetries int    `json:"max_retries"`
}

const (
	// MaxWebhookRetries 最大重试次数
	MaxWebhookRetries = 3
	// WebhookTimeout HTTP 超时
	WebhookTimeout = 5 * time.Second
)

// StatusToTrigger 将 ExptStatus 映射为 NotificationTrigger
func StatusToTrigger(status ExptStatus) (NotificationTrigger, bool) {
	switch status {
	case ExptStatus_Processing:
		return NotificationTriggerStarted, true
	case ExptStatus_Success:
		return NotificationTriggerSucceeded, true
	case ExptStatus_Failed:
		return NotificationTriggerFailed, true
	case ExptStatus_Terminated, ExptStatus_SystemTerminated:
		return NotificationTriggerTerminated, true
	default:
		return "", false
	}
}
