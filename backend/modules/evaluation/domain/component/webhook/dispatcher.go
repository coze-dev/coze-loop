// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package webhook

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate mockgen -destination=mocks/dispatcher.go -package=mocks . IWebhookDispatcher
type IWebhookDispatcher interface {
	Dispatch(ctx context.Context, expt *entity.Experiment, eventType entity.WebhookEventType) error
}
