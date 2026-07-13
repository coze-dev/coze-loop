// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package component

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// IWebhookConfiger exposes typed accessors for the 5 webhook conf entities.
// Commercial impls (`exptConfiger.Get*`) read from TCC and fall back to the
// `Default*Conf` factories on parse errors; OSS ships a `defaultWebhookConfiger`
// that always returns the defaults so `go build ./...` works stand-alone.
//
//go:generate mockgen -destination=mocks/webhook_configer.go -package=mocks . IWebhookConfiger
type IWebhookConfiger interface {
	GetWebhookConf(ctx context.Context) *entity.WebhookGlobalConf
	GetWebhookRetryConf(ctx context.Context) *entity.WebhookRetryConf
	GetWebhookRateLimitConf(ctx context.Context) *entity.WebhookRateLimitConf
	GetWebhookURLLimitConf(ctx context.Context) *entity.WebhookURLLimitConf
	GetWebhookSecurityConf(ctx context.Context) *entity.WebhookSecurityConf
}
