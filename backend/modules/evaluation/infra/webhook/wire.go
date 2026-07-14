// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"

	"github.com/google/wire"

	componentwebhook "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/webhook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	domaincomponent "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
)

// WebhookInfraSet provides the OSS-side webhook subsystem: default configer
// (returns baked-in Default*Conf) + a sender pinned to those defaults. Commercial
// substitutes both providers with TCC-backed variants in its own wire graph.
var WebhookInfraSet = wire.NewSet(
	NewDefaultWebhookConfiger,
	ProvideWebhookSender,
)

// ProvideWebhookSender resolves the retry + security config off the configer
// and hands them to `NewWebhookSenderWithConf`. Kept as a wire-facing wrapper
// so `WebhookInfraSet` stays self-contained (no free-standing *Conf inputs).
func ProvideWebhookSender(configer domaincomponent.IWebhookConfiger) componentwebhook.IWebhookSender {
	ctx := context.Background()
	var retry *entity.WebhookRetryConf
	var security *entity.WebhookSecurityConf
	if configer != nil {
		retry = configer.GetWebhookRetryConf(ctx)
		security = configer.GetWebhookSecurityConf(ctx)
	}
	return NewWebhookSenderWithConf(retry, security)
}
