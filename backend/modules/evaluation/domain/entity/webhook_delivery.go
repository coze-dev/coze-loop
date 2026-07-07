// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import "time"

// DeliveryStatus webhook 投递状态机：pending → retrying → succeeded / failed。
type DeliveryStatus int32

const (
	DeliveryStatus_Unknown   DeliveryStatus = 0
	DeliveryStatus_Pending   DeliveryStatus = 1 // 首投前入库
	DeliveryStatus_Retrying  DeliveryStatus = 2 // 首投/中途失败等待重试
	DeliveryStatus_Succeeded DeliveryStatus = 3 // 终态：2xx
	DeliveryStatus_Failed    DeliveryStatus = 4 // 终态：4 次投递耗尽
)

// WebhookDelivery 一次 webhook 投递记录（含 14 字段业务列，对齐 test_case 26 IDL）。
type WebhookDelivery struct {
	ID               int64               `json:"id"`
	DeliveryID       string              `json:"delivery_id"` // UUID v4, uk_delivery_id
	WorkspaceID      int64               `json:"workspace_id"`
	ExperimentID     int64               `json:"experiment_id"`
	Event            NotificationTrigger `json:"event"`
	URL              string              `json:"url"`
	Status           DeliveryStatus      `json:"status"`
	AttemptCount     int32               `json:"attempt_count"`
	LastResponseCode int32               `json:"last_response_code"`
	LastError        string              `json:"last_error"`
	RequestBody      string              `json:"request_body"` // canonical JSON, 供重放
	FirstSentAt      *time.Time          `json:"first_sent_at"`
	LastSentAt       *time.Time          `json:"last_sent_at"`
	NextRetryAt      *time.Time          `json:"next_retry_at"`
}
