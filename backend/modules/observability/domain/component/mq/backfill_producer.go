// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mq

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
)

type IBackfillProducer interface {
	SendBackfill(ctx context.Context, message *entity.BackFillEvent) error
}
