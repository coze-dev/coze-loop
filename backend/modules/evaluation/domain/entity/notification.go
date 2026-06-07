// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import "fmt"

type ExptNotificationConf struct {
	Filter             *NotificationFilter
	Webhook            *WebhookNotificationConf
	FeishuNotification *FeishuNotificationConf
}

type WebhookNotificationConf struct {
	Enable bool
	URLs   []string
	Secret string
}

type FeishuNotificationConf struct {
	Enable bool
}

type NotificationFilter struct {
	Conditions []*NotificationFilterCondition
	LogicOp    int // 0=Unknown, 1=And, 2=Or
}

type NotificationFilterCondition struct {
	FieldType int    // 3 = ExptStatus
	Operator  int    // 7 = In, 8 = NotIn
	Value     string // JSON array string, e.g. "[\"11\",\"12\"]"
}

func (c *ExptNotificationConf) ShouldWebhook(status ExptStatus) bool {
	if c == nil || c.Webhook == nil || !c.Webhook.Enable || len(c.Webhook.URLs) == 0 {
		return false
	}
	return c.matchFilter(status)
}

func (c *ExptNotificationConf) ShouldFeishu(status ExptStatus) bool {
	if c == nil || c.FeishuNotification == nil || !c.FeishuNotification.Enable {
		return false
	}
	return c.matchFilter(status)
}

func (c *ExptNotificationConf) matchFilter(status ExptStatus) bool {
	if c.Filter == nil || len(c.Filter.Conditions) == 0 {
		return true
	}
	statusStr := fmt.Sprintf("%d", int64(status))
	for _, cond := range c.Filter.Conditions {
		if cond.FieldType != 3 { // Only ExptStatus
			continue
		}
		matched := containsStatus(cond.Value, statusStr)
		if cond.Operator == 7 { // In
			if matched {
				return true
			}
		} else if cond.Operator == 8 { // NotIn
			if !matched {
				return true
			}
		}
	}
	return false
}

func containsStatus(valueJSON string, status string) bool {
	// value is a JSON array string like "[\"3\",\"11\"]" or comma-separated
	// Simple string contains check for status values
	return len(valueJSON) > 0 && (valueJSON == status ||
		len(valueJSON) > 2 && (valueJSON[1:len(valueJSON)-1] == status ||
			// Check comma-separated within brackets
			containsInList(valueJSON, status)))
}

func containsInList(s, target string) bool {
	// Parse simple JSON array or comma-separated values
	inQuote := false
	current := ""
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == '"' {
			inQuote = !inQuote
			continue
		}
		if ch == '[' || ch == ']' || ch == ' ' {
			continue
		}
		if ch == ',' && !inQuote {
			if current == target {
				return true
			}
			current = ""
			continue
		}
		current += string(ch)
	}
	return current == target
}

// WebhookEnvelope is the top-level payload sent to webhook URLs.
type WebhookEnvelope struct {
	DeliveryID   string      `json:"delivery_id"`
	CreateTime   string      `json:"create_time"`
	EventType    string      `json:"event_type"`
	ResourceType string      `json:"resource_type"`
	Summary      string      `json:"summary"`
	Data         WebhookData `json:"data"`
}

// WebhookData contains the experiment-specific data within a webhook envelope.
type WebhookData struct {
	ExperimentID   int64            `json:"experiment_id"`
	ExperimentName string           `json:"experiment_name"`
	Status         string           `json:"status"`
	Progress       *WebhookProgress `json:"progress,omitempty"`
}

// WebhookProgress contains experiment execution progress counters.
type WebhookProgress struct {
	Total     int64 `json:"total"`
	Succeeded int64 `json:"succeeded"`
	Failed    int64 `json:"failed"`
}

// ExptStatusToEventType converts an ExptStatus to a webhook event type string.
func ExptStatusToEventType(status ExptStatus) string {
	switch status {
	case ExptStatus_Processing:
		return "experiment.started"
	case ExptStatus_Success:
		return "experiment.succeeded"
	case ExptStatus_Failed:
		return "experiment.failed"
	case ExptStatus_Terminated, ExptStatus_SystemTerminated:
		return "experiment.terminated"
	default:
		return ""
	}
}

// ExptStatusToString converts an ExptStatus to a human-readable string.
func ExptStatusToString(status ExptStatus) string {
	switch status {
	case ExptStatus_Processing:
		return "processing"
	case ExptStatus_Success:
		return "success"
	case ExptStatus_Failed:
		return "failed"
	case ExptStatus_Terminated:
		return "terminated"
	case ExptStatus_SystemTerminated:
		return "system_terminated"
	default:
		return "unknown"
	}
}
