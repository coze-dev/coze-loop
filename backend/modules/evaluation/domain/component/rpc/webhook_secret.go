// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import "context"

// IWebhookSecretProvider 获取 Webhook 签名密钥，用于 HMAC-SHA256 签名
type IWebhookSecretProvider interface {
	GetWorkspaceSK(ctx context.Context, spaceID int64) (string, error)
}
