// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import "time"

// WebhookDelivery status enum (T1.1)
const (
	WebhookDeliveryStatusPending     = "pending"
	WebhookDeliveryStatusRetrying    = "retrying"
	WebhookDeliveryStatusSucceeded   = "succeeded"
	WebhookDeliveryStatusFailed      = "failed"
	WebhookDeliveryStatusFinalFailed = "final_failed"
	WebhookDeliveryStatusRateLimited = "rate_limited"
)

// WebhookDelivery event enum
const (
	WebhookDeliveryEventStarted    = "started"
	WebhookDeliveryEventSucceeded  = "succeeded"
	WebhookDeliveryEventFailed     = "failed"
	WebhookDeliveryEventTerminated = "terminated"
)

// WebhookInternalSourceBITs marks internal delivery injected by BITs node.
const WebhookInternalSourceBITs = "bits"

// WebhookDelivery mirrors the `webhook_delivery` MySQL row.
type WebhookDelivery struct {
	ID               int64
	DeliveryID       string
	SpaceID          int64
	ExperimentID     int64
	Event            string
	URL              string
	Payload          []byte
	Status           string
	AttemptCount     int
	FirstSentAt      *time.Time
	LastSentAt       *time.Time
	LastResponseCode int
	LastError        string
	InternalSource   string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// IsFinal returns true if the delivery has reached a terminal status.
func (d *WebhookDelivery) IsFinal() bool {
	switch d.Status {
	case WebhookDeliveryStatusSucceeded, WebhookDeliveryStatusFinalFailed, WebhookDeliveryStatusRateLimited:
		return true
	default:
		return false
	}
}

// EventToStatusAlias normalises system-terminated to the user-facing
// `terminated` bucket so filter values=[terminated] still catch the
// system-triggered termination path.
func EventToStatusAlias(event string) string {
	switch event {
	case "system_terminated":
		return "terminated"
	default:
		return event
	}
}
