// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// NotificationConf 通知配置（公共触发条件 + 各渠道独立开关/参数）
// 存储在 experiment.notification_conf BLOB 字段，JSON 序列化
type NotificationConf struct {
	Filter             *ExptListFilter         `json:"filter,omitempty"`
	Webhook            *WebhookConf            `json:"webhook,omitempty"`
	FeishuNotification *FeishuNotificationConf `json:"feishu_notification,omitempty"`
}

// WebhookConf Webhook 渠道配置
type WebhookConf struct {
	Enable bool   `json:"enable"`
	URL    string `json:"url,omitempty"`
}

// FeishuNotificationConf 飞书通知渠道配置
type FeishuNotificationConf struct {
	Enable bool   `json:"enable"`
	UserID string `json:"user_id,omitempty"`
}

// DeliveryStatus 投递状态
type DeliveryStatus string

const (
	DeliveryPending  DeliveryStatus = "pending"
	DeliverySuccess  DeliveryStatus = "success"
	DeliveryFailed   DeliveryStatus = "failed"
	DeliveryRetrying DeliveryStatus = "retrying"
)

// WebhookEventType Webhook 事件类型
type WebhookEventType string

const (
	WebhookEventStarted    WebhookEventType = "experiment.started"
	WebhookEventSucceeded  WebhookEventType = "experiment.succeeded"
	WebhookEventFailed     WebhookEventType = "experiment.failed"
	WebhookEventTerminated WebhookEventType = "experiment.terminated"
)

// ExptStatusToWebhookEvent 将实验状态映射为 Webhook 事件类型
func ExptStatusToWebhookEvent(status ExptStatus) WebhookEventType {
	switch status {
	case ExptStatus_Processing:
		return WebhookEventStarted
	case ExptStatus_Success:
		return WebhookEventSucceeded
	case ExptStatus_Failed:
		return WebhookEventFailed
	case ExptStatus_Terminated, ExptStatus_SystemTerminated:
		return WebhookEventTerminated
	default:
		return ""
	}
}

// WebhookRetryEvent Webhook 重试 MQ 消息体
type WebhookRetryEvent struct {
	LogID      int64  `json:"log_id"`
	DeliveryID string `json:"delivery_id"`
	AttemptNum int    `json:"attempt_num"` // 当前第几次重试 (1/2/3)
}

// WebhookPayload Webhook 请求 Body 结构（信封 + 业务数据）
type WebhookPayload struct {
	DeliveryID   string              `json:"delivery_id"`
	CreateTime   string              `json:"create_time"`
	EventType    WebhookEventType    `json:"event_type"`
	ResourceType string              `json:"resource_type"`
	Summary      string              `json:"summary"`
	Data         *WebhookPayloadData `json:"data"`
}

// WebhookPayloadData Webhook Body 中的业务数据
type WebhookPayloadData struct {
	ExperimentID   string                  `json:"experiment_id"`
	ExperimentName string                  `json:"experiment_name"`
	Status         string                  `json:"status"`
	Progress       *WebhookPayloadProgress `json:"progress"`
}

// WebhookPayloadProgress 实验执行进度
type WebhookPayloadProgress struct {
	Total     int64 `json:"total"`
	Succeeded int64 `json:"succeeded"`
	Failed    int64 `json:"failed"`
}

// NotificationLog 投递日志实体
type NotificationLog struct {
	ID           int64          `json:"id"`
	SpaceID      int64          `json:"space_id"`
	ExptID       int64          `json:"expt_id"`
	DeliveryID   string         `json:"delivery_id"`
	EventType    string         `json:"event_type"`
	ChannelType  string         `json:"channel_type"`
	WebhookURL   string         `json:"webhook_url"`
	Status       DeliveryStatus `json:"status"`
	AttemptCount int            `json:"attempt_count"`
	MaxAttempts  int            `json:"max_attempts"`
	FirstSentAt  *int64         `json:"first_sent_at"`
	LastSentAt   *int64         `json:"last_sent_at"`
	ResponseCode *int           `json:"response_code"`
	ErrorMessage string         `json:"error_message"`
	RequestBody  string         `json:"request_body"`
}

// ShouldNotify 判断公共触发条件是否匹配
func (conf *NotificationConf) ShouldNotify(status ExptStatus) bool {
	if conf == nil {
		return false
	}
	if conf.Filter == nil {
		return true // 无 filter 时默认全部触发
	}
	// 检查 Includes.Status
	if conf.Filter.Includes != nil && len(conf.Filter.Includes.Status) > 0 {
		for _, s := range conf.Filter.Includes.Status {
			if ExptStatus(s) == status {
				return true
			}
		}
		return false
	}
	// 检查 Excludes.Status
	if conf.Filter.Excludes != nil && len(conf.Filter.Excludes.Status) > 0 {
		for _, s := range conf.Filter.Excludes.Status {
			if ExptStatus(s) == status {
				return false
			}
		}
	}
	return true
}

// ShouldWebhook 判断是否应该发送 Webhook
func (conf *NotificationConf) ShouldWebhook(status ExptStatus) bool {
	if conf == nil || conf.Webhook == nil || !conf.Webhook.Enable || conf.Webhook.URL == "" {
		return false
	}
	return conf.ShouldNotify(status)
}

// ShouldFeishu 判断是否应该发送飞书通知
func (conf *NotificationConf) ShouldFeishu(status ExptStatus) bool {
	if conf == nil || conf.FeishuNotification == nil || !conf.FeishuNotification.Enable {
		return false
	}
	return conf.ShouldNotify(status)
}
