// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package webhook

import "github.com/google/wire"

var WebhookSet = wire.NewSet(NewWebhookSender)
