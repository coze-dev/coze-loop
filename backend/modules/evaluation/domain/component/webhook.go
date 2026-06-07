// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package component

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type IWebhookDispatcher interface {
	Dispatch(ctx context.Context, event *entity.ExptLifecycleEvent, expt *entity.Experiment) error
}
