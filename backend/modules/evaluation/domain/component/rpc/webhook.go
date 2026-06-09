// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import "context"

type WebhookExptProgress struct {
	Total     int32 `json:"total"`
	Succeeded int32 `json:"succeeded"`
	Failed    int32 `json:"failed"`
}

type WebhookExptInfo struct {
	ID       int64                `json:"id"`
	Name     string               `json:"name"`
	Status   string               `json:"status"`
	Progress *WebhookExptProgress `json:"progress,omitempty"`
}

type WebhookPayload struct {
	DeliveryID string           `json:"delivery_id"`
	Event      string           `json:"event"`
	Timestamp  string           `json:"timestamp"`
	Experiment *WebhookExptInfo `json:"experiment"`
}

//go:generate mockgen -destination=mocks/webhook.go -package=mocks . IWebhookDeliveryAdapter
type IWebhookDeliveryAdapter interface {
	Deliver(ctx context.Context, url string, spaceID int64, payload *WebhookPayload) error
}
