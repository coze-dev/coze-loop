// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// NotificationConfig represents the notification configuration for an experiment or template.
// It is stored as a JSON BLOB in the database (notification_conf column).
type NotificationConfig struct {
	Rules []*NotificationRule `json:"rules,omitempty"`
}

// NotificationRule defines a single notification rule: a trigger condition + a list of actions.
type NotificationRule struct {
	Trigger *NotificationTrigger  `json:"trigger,omitempty"`
	Actions []*NotificationAction `json:"actions,omitempty"`
}

// NotificationTrigger defines the condition under which a notification is sent.
type NotificationTrigger struct {
	Field    NotificationField    `json:"field,omitempty"`
	Operator NotificationOperator `json:"operator,omitempty"`
	Values   []string             `json:"values,omitempty"`
}

// NotificationAction defines a single notification channel + its configuration.
type NotificationAction struct {
	Channel NotificationChannelType `json:"channel,omitempty"`
	Webhook *WebhookAction          `json:"webhook,omitempty"`
}

// WebhookAction holds Webhook-specific configuration.
type WebhookAction struct {
	URL string `json:"url,omitempty"`
}

// NotificationField enumerates the fields that can be used in a notification trigger.
type NotificationField int64

const (
	NotificationFieldUnknown    NotificationField = 0
	NotificationFieldExptStatus NotificationField = 1
)

// NotificationOperator enumerates the operators for notification trigger conditions.
type NotificationOperator int64

const (
	NotificationOperatorUnknown NotificationOperator = 0
	NotificationOperatorIn      NotificationOperator = 1
	NotificationOperatorNotIn   NotificationOperator = 2
)

// NotificationChannelType enumerates the notification channels.
type NotificationChannelType int64

const (
	NotificationChannelTypeUnknown NotificationChannelType = 0
	NotificationChannelTypeWebhook NotificationChannelType = 1
	NotificationChannelTypeFeishu  NotificationChannelType = 2
)

// WebhookDeliveryLog records one webhook delivery attempt.
type WebhookDeliveryLog struct {
	ID           int64
	SpaceID      int64
	ExptID       int64
	DeliveryID   string
	WebhookURL   string
	Status       WebhookDeliveryStatus
	HTTPCode     int
	ErrorMessage string
	RetryCount   int
	CreatedAt    int64
	UpdatedAt    int64
}

// WebhookDeliveryStatus enumerates delivery attempt outcomes.
type WebhookDeliveryStatus int32

const (
	WebhookDeliveryStatusPending WebhookDeliveryStatus = 0
	WebhookDeliveryStatusSuccess WebhookDeliveryStatus = 1
	WebhookDeliveryStatusFailed  WebhookDeliveryStatus = 2
)

// WebhookDeliveryEvent is the MQ message payload used for webhook retry queue.
type WebhookDeliveryEvent struct {
	DeliveryID string `json:"delivery_id"`
	SpaceID    int64  `json:"space_id"`
	ExptID     int64  `json:"expt_id"`
	WebhookURL string `json:"webhook_url"`
	Payload    []byte `json:"payload"`
	RetryCount int    `json:"retry_count"`
}

// --- Status mapping helpers ---

// ExptStatusToEventString maps ExptStatus to the event string used in webhook payloads.
func ExptStatusToEventString(s ExptStatus) string {
	switch s {
	case ExptStatus_Processing:
		return "started"
	case ExptStatus_Success:
		return "succeeded"
	case ExptStatus_Failed:
		return "failed"
	case ExptStatus_Terminated, ExptStatus_SystemTerminated:
		return "terminated"
	default:
		return ""
	}
}

// ExptStatusMatchesNotificationValue checks whether an ExptStatus matches a notification condition value string.
func ExptStatusMatchesNotificationValue(s ExptStatus, value string) bool {
	switch value {
	case "processing":
		return s == ExptStatus_Processing
	case "success":
		return s == ExptStatus_Success
	case "failed":
		return s == ExptStatus_Failed
	case "terminated":
		return s == ExptStatus_Terminated || s == ExptStatus_SystemTerminated
	default:
		return false
	}
}
