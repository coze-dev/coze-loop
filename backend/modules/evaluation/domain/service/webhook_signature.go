// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
)

// IWebhookSignatureProvider defines the interface for webhook payload signing
type IWebhookSignatureProvider interface {
	// Sign computes an HMAC-SHA256 signature for the given payload
	Sign(ctx context.Context, spaceID int64, timestamp string, body []byte) (string, error)
}

// NoopWebhookSignatureProvider is a no-op implementation that returns empty signature
type NoopWebhookSignatureProvider struct{}

// NewNoopWebhookSignatureProvider creates a new NoopWebhookSignatureProvider
func NewNoopWebhookSignatureProvider() IWebhookSignatureProvider {
	return &NoopWebhookSignatureProvider{}
}

// Sign returns an empty signature (noop implementation)
func (n *NoopWebhookSignatureProvider) Sign(_ context.Context, _ int64, _ string, _ []byte) (string, error) {
	return "", nil
}
