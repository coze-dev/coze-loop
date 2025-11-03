// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate mockgen -destination=mocks/trace_agent.go -package=mocks . IAgentAdapter
type IAgentAdapter interface {
	CallTraceAgent(ctx context.Context, spaceID int64, url string, startTime, endTime int64) (int64, error)
	GetReport(ctx context.Context, spaceID, reportID int64) (report string, status entity.ReportStatus, err error)
}
