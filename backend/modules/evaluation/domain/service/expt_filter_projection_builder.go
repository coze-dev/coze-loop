// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// ExptFilterProjectionBuilder rebuilds complete items without changing source data.
type ExptFilterProjectionBuilder interface {
	BuildExptFilterProjection(context.Context, int64, int64, []int64) ([]*entity.ExptTurnResultFilterEntity, error)
}
