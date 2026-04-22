// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type ExptLifecycleEventHandler interface {
	HandleLifecycleEvent(ctx context.Context, event *entity.ExptLifecycleEvent) error
}
