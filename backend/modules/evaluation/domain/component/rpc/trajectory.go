// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate mockgen -destination=mocks/trajectory.go -package=mocks . ITrajectoryAdapter
type ITrajectoryAdapter interface {
	ListTrajectory(ctx context.Context, spaceID int64, traceID []string, startTimeMS *int64) ([]*entity.Trajectory, error)
	// SearchTraceSpans returns lightweight span info for a single trace, used for computing trace metrics (span_count, tool_count).
	// startTimeMS and endTimeMS are in milliseconds.
	SearchTraceSpans(ctx context.Context, spaceID int64, traceID string, startTimeMS, endTimeMS int64) ([]*entity.TraceSpanInfo, error)
}
