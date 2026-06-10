// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import "context"

// IWebhookSecretProvider 获取 Webhook 签名密钥
//
//go:generate mockgen -destination=mocks/webhook_secret_provider.go -package=mocks . IWebhookSecretProvider
type IWebhookSecretProvider interface {
	GetSpaceSK(ctx context.Context, spaceID int64) (string, error)
}

// NoopWebhookSecretProvider 默认空实现，返回空字符串
type NoopWebhookSecretProvider struct{}

func NewNoopWebhookSecretProvider() IWebhookSecretProvider {
	return &NoopWebhookSecretProvider{}
}

func (n *NoopWebhookSecretProvider) GetSpaceSK(_ context.Context, _ int64) (string, error) {
	return "", nil
}
