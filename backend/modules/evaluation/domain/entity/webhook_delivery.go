// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import "time"

type WebhookEventType string

const (
	WebhookEventStarted    WebhookEventType = "started"
	WebhookEventSucceeded  WebhookEventType = "succeeded"
	WebhookEventFailed     WebhookEventType = "failed"
	WebhookEventTerminated WebhookEventType = "terminated"
)

type DeliveryStatus string

const (
	DeliveryStatusPending  DeliveryStatus = "pending"
	DeliveryStatusSuccess  DeliveryStatus = "success"
	DeliveryStatusRetrying DeliveryStatus = "retrying"
	DeliveryStatusFailed   DeliveryStatus = "failed"
)

type WebhookDelivery struct {
	ID           int64
	SpaceID      int64
	ExptID       int64
	DeliveryID   string
	EventType    WebhookEventType
	ChannelType  string
	WebhookURL   string
	Status       DeliveryStatus
	AttemptCount int
	MaxAttempts  int
	FirstSentAt  *time.Time
	LastSentAt   *time.Time
	NextRetryAt  *time.Time
	ResponseCode *int
	ErrorMessage string
	CreatedBy    string
	UpdatedBy    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type WebhookDeliveryMessage struct {
	DeliveryID string           `json:"delivery_id"`
	ExptID     int64            `json:"expt_id"`
	SpaceID    int64            `json:"space_id"`
	EventType  WebhookEventType `json:"event_type"`
	WebhookURL string           `json:"webhook_url"`
	Attempt    int              `json:"attempt"`
	CreatedAt  int64            `json:"created_at"`
	SourceType string           `json:"source_type,omitempty"`
}

type WebhookPayload struct {
	DeliveryID string           `json:"delivery_id"`
	Event      WebhookEventType `json:"event"`
	Timestamp  int64            `json:"timestamp"`
	Experiment *WebhookExptInfo `json:"experiment"`
}

type WebhookExptInfo struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Status    string           `json:"status"`
	Progress  *WebhookProgress `json:"progress"`
	Metrics   *WebhookMetrics  `json:"metrics"`
	ResultURL *string          `json:"result_url"`
}

type WebhookProgress struct {
	Total      int `json:"total"`
	Succeeded  int `json:"succeeded"`
	Failed     int `json:"failed"`
	Processing int `json:"processing"`
}

type WebhookMetrics struct {
	OverallScore     *WebhookScoreAgg       `json:"overall_score"`
	EvaluatorMetrics []*WebhookEvaluatorAgg `json:"evaluator_metrics"`
}

type WebhookScoreAgg struct {
	Avg *float64 `json:"avg"`
	Min *float64 `json:"min"`
	Max *float64 `json:"max"`
}

type WebhookEvaluatorAgg struct {
	EvaluatorID   string           `json:"evaluator_id"`
	EvaluatorName string           `json:"evaluator_name"`
	Score         *WebhookScoreAgg `json:"score"`
}

func ExptStatusToWebhookEvent(status ExptStatus) (WebhookEventType, bool) {
	switch status {
	case ExptStatus_Processing:
		return WebhookEventStarted, true
	case ExptStatus_Success:
		return WebhookEventSucceeded, true
	case ExptStatus_Failed:
		return WebhookEventFailed, true
	case ExptStatus_Terminated, ExptStatus_SystemTerminated:
		return WebhookEventTerminated, true
	default:
		return "", false
	}
}
