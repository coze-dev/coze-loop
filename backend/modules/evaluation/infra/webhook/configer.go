// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// NewDefaultWebhookConfiger returns a stand-alone OSS configer that always
// returns `Default*Conf`. Commercial overrides this with the TCC-backed
// `exptConfiger` in its wire graph.
func NewDefaultWebhookConfiger() component.IWebhookConfiger {
	return defaultWebhookConfiger{}
}

type defaultWebhookConfiger struct{}

func (defaultWebhookConfiger) GetWebhookConf(_ context.Context) *entity.WebhookGlobalConf {
	return entity.DefaultWebhookGlobalConf()
}

func (defaultWebhookConfiger) GetWebhookRetryConf(_ context.Context) *entity.WebhookRetryConf {
	return entity.DefaultWebhookRetryConf()
}

func (defaultWebhookConfiger) GetWebhookRateLimitConf(_ context.Context) *entity.WebhookRateLimitConf {
	return entity.DefaultWebhookRateLimitConf()
}

func (defaultWebhookConfiger) GetWebhookURLLimitConf(_ context.Context) *entity.WebhookURLLimitConf {
	return entity.DefaultWebhookURLLimitConf()
}

func (defaultWebhookConfiger) GetWebhookSecurityConf(_ context.Context) *entity.WebhookSecurityConf {
	return entity.DefaultWebhookSecurityConf()
}
