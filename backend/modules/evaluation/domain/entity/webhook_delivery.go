// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import "time"

// DeliveryStatus Webhook 投递状态
type DeliveryStatus int32

const (
	DeliveryStatus_Pending  DeliveryStatus = 0
	DeliveryStatus_Success  DeliveryStatus = 1
	DeliveryStatus_Failed   DeliveryStatus = 2
	DeliveryStatus_Retrying DeliveryStatus = 3
)

// WebhookEventType Webhook 事件类型
type WebhookEventType string

const (
	WebhookEvent_Started    WebhookEventType = "started"
	WebhookEvent_Succeeded  WebhookEventType = "succeeded"
	WebhookEvent_Failed     WebhookEventType = "failed"
	WebhookEvent_Terminated WebhookEventType = "terminated"
)

// ExptStatusToWebhookEvent 将实验状态映射为 Webhook 事件类型
func ExptStatusToWebhookEvent(status ExptStatus) (WebhookEventType, bool) {
	switch status {
	case ExptStatus_Processing:
		return WebhookEvent_Started, true
	case ExptStatus_Success:
		return WebhookEvent_Succeeded, true
	case ExptStatus_Failed:
		return WebhookEvent_Failed, true
	case ExptStatus_Terminated, ExptStatus_SystemTerminated:
		return WebhookEvent_Terminated, true
	default:
		return "", false
	}
}

// WebhookDelivery Webhook 投递记录
type WebhookDelivery struct {
	ID             int64
	SpaceID        int64
	DeliveryID     string // UUID
	ExperimentID   int64
	EventType      WebhookEventType
	WebhookURL     string
	Status         DeliveryStatus
	RetryCount     int32
	LastStatusCode int32
	ErrorMessage   string
	RequestHeaders []byte // JSON
	NextRetryAt    *time.Time
	FirstSentAt    *time.Time
	LastSentAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// WebhookPayload is the JSON structure sent to the user's webhook URL
type WebhookPayload struct {
	DeliveryID string           `json:"delivery_id"`
	Event      WebhookEventType `json:"event"`
	Timestamp  string           `json:"timestamp"` // ISO 8601
	Experiment *WebhookExptInfo `json:"experiment"`
}

// WebhookExptInfo 实验摘要信息（包含在 Webhook payload 中）
type WebhookExptInfo struct {
	ID        int64            `json:"id"`
	Name      string           `json:"name"`
	Status    string           `json:"status"`
	Progress  *WebhookProgress `json:"progress"`
	Metrics   *WebhookMetrics  `json:"metrics,omitempty"`
	ResultURL string           `json:"result_url"`
}

// WebhookProgress 实验进度信息
type WebhookProgress struct {
	Total     int64 `json:"total"`
	Succeeded int64 `json:"succeeded"`
	Failed    int64 `json:"failed"`
}

// WebhookMetrics 实验指标汇总
type WebhookMetrics struct {
	OverallScore     *WebhookScoreMetric  `json:"overall_score,omitempty"`
	EvaluatorMetrics []*WebhookEvalMetric `json:"evaluator_metrics,omitempty"`
}

// WebhookScoreMetric 总分指标
type WebhookScoreMetric struct {
	Avg float64 `json:"avg"`
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

// WebhookEvalMetric 评估器维度指标
type WebhookEvalMetric struct {
	EvaluatorID   int64   `json:"evaluator_id"`
	EvaluatorName string  `json:"evaluator_name"`
	Avg           float64 `json:"avg"`
	Min           float64 `json:"min"`
	Max           float64 `json:"max"`
}

// WebhookRetryIntervals 重试间隔（秒）: 1min, 5min, 30min
var WebhookRetryIntervals = []int{60, 300, 1800}

// MaxWebhookRetries 最大重试次数
const MaxWebhookRetries = 3

// WebhookHTTPTimeout webhook HTTP 请求超时（秒）
const WebhookHTTPTimeout = 5

// MaxWebhookURLs 单个实验最大 Webhook URL 数量
const MaxWebhookURLs = 10
