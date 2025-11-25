// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type ITrajectoryAdapter interface {
	ListTrajectory(ctx context.Context, spaceID int64, traceID []string, startTimeMS *int64) ([]*entity.Trajectory, error)
}
