// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"github.com/google/wire"
)

// WebhookDomainSet exposes the domain-side dispatcher constructor. Combine
// with `infra/webhook.WebhookInfraSet` + `infra/repo/experiment` +
// `infra/mq/rocket/producer` providers to complete the graph.
var WebhookDomainSet = wire.NewSet(
	NewWebhookDispatcher,
)
