// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"strings"
)

// NotificationFilterOperatorType operator type for notification filter condition
type NotificationFilterOperatorType int32

const (
	NotificationFilterOperatorType_Unknown NotificationFilterOperatorType = 0
	NotificationFilterOperatorType_In      NotificationFilterOperatorType = 1
	NotificationFilterOperatorType_NotIn   NotificationFilterOperatorType = 2
)

// NotificationFilterCondition defines the filter condition for notifications
type NotificationFilterCondition struct {
	Operator     NotificationFilterOperatorType `json:"operator"`
	StatusValues []string                       `json:"status_values"`
}

// WebhookConfig defines webhook channel configuration
type WebhookConfig struct {
	URLs []string `json:"urls"`
}

// LarkNotifyConfig defines lark (feishu) notification configuration
type LarkNotifyConfig struct {
	Enabled *bool `json:"enabled,omitempty"`
}

// NotificationChannelConfig defines notification channel configurations
type NotificationChannelConfig struct {
	Webhook *WebhookConfig   `json:"webhook,omitempty"`
	Lark    *LarkNotifyConfig `json:"lark,omitempty"`
}

// NotificationConfig defines the notification configuration for an experiment
type NotificationConfig struct {
	FilterCondition *NotificationFilterCondition `json:"filter_condition,omitempty"`
	Channels        *NotificationChannelConfig   `json:"channels,omitempty"`
}

// WebhookDeliveryEvent represents a webhook delivery event for MQ
type WebhookDeliveryEvent struct {
	DeliveryID string `json:"delivery_id"`
	ExptID     int64  `json:"expt_id"`
	SpaceID    int64  `json:"space_id"`
	WebhookURL string `json:"webhook_url"`
	Payload    string `json:"payload"`
	RetryCount int    `json:"retry_count"`
	MaxRetry   int    `json:"max_retry"`
	Timestamp  string `json:"timestamp"`
}

// exptStatusToUserStatus maps internal ExptStatus to user-visible status strings
var exptStatusToUserStatus = map[ExptStatus]string{
	ExptStatus_Processing:       "processing",
	ExptStatus_Success:          "success",
	ExptStatus_Failed:           "failed",
	ExptStatus_Terminated:       "terminated",
	ExptStatus_SystemTerminated: "terminated",
	ExptStatus_Terminating:      "processing",
	ExptStatus_Draining:         "processing",
	ExptStatus_Pending:          "processing",
}

// MapExptStatusToNotificationEvent maps ExptStatus to user-visible status string
func MapExptStatusToNotificationEvent(status ExptStatus) string {
	if s, ok := exptStatusToUserStatus[status]; ok {
		return s
	}
	return ""
}

// MatchNotificationCondition checks if a notification should be sent based on the config and target status
func MatchNotificationCondition(config *NotificationConfig, toStatus ExptStatus) bool {
	if config == nil {
		return false
	}

	// No filter condition means match all terminal statuses
	if config.FilterCondition == nil {
		userStatus := MapExptStatusToNotificationEvent(toStatus)
		// Only match terminal statuses by default
		return userStatus == "success" || userStatus == "failed" || userStatus == "terminated"
	}

	userStatus := MapExptStatusToNotificationEvent(toStatus)
	if userStatus == "" {
		return false
	}

	fc := config.FilterCondition
	switch fc.Operator {
	case NotificationFilterOperatorType_In:
		for _, sv := range fc.StatusValues {
			if sv == userStatus {
				return true
			}
		}
		return false
	case NotificationFilterOperatorType_NotIn:
		for _, sv := range fc.StatusValues {
			if sv == userStatus {
				return false
			}
		}
		return true
	default:
		// Unknown operator: match terminal statuses by default
		return userStatus == "success" || userStatus == "failed" || userStatus == "terminated"
	}
}

// HasWebhookURLs checks if the notification config has any webhook URLs configured
func (c *NotificationConfig) HasWebhookURLs() bool {
	return c != nil && c.Channels != nil && c.Channels.Webhook != nil && len(c.Channels.Webhook.URLs) > 0
}

// IsLarkEnabled checks if lark notification is enabled
func (c *NotificationConfig) IsLarkEnabled() bool {
	if c == nil || c.Channels == nil || c.Channels.Lark == nil || c.Channels.Lark.Enabled == nil {
		return false
	}
	return *c.Channels.Lark.Enabled
}

// ValidateNotificationRules validates the notification configuration.
// Returns an error if webhook channel is configured but any URL is empty or whitespace-only.
// Returns nil if config is nil (backward compatible — no notifications means no validation needed).
func ValidateNotificationRules(config *NotificationConfig) error {
	if config == nil {
		return nil
	}
	if config.Channels == nil {
		return nil
	}
	wh := config.Channels.Webhook
	if wh == nil {
		return nil
	}
	for _, u := range wh.URLs {
		if strings.TrimSpace(u) == "" {
			return ErrWebhookURLEmpty
		}
	}
	return nil
}

// ErrWebhookURLEmpty is a sentinel error for empty webhook URL validation.
var ErrWebhookURLEmpty = &WebhookValidationError{Msg: "webhook url cannot be empty"}

// WebhookValidationError represents a webhook validation error.
type WebhookValidationError struct {
	Msg string
}

func (e *WebhookValidationError) Error() string {
	return e.Msg
}
