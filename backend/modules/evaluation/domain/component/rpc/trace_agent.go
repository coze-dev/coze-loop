// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate mockgen -destination=mocks/trace_agent.go -package=mocks . IAgentAdapter
type IAgentAdapter interface {
	CallTraceAgent(ctx context.Context, param *CallTraceAgentParam) (int64, error)
	GetReport(ctx context.Context, spaceID, reportID int64) (report string, list []*entity.InsightAnalysisReportIndex, status entity.ReportStatus, err error)
}

type CallTraceAgentParam struct {
	SpaceID int64
	ExptID  int64
	Url     string

	StartTime int64 // in ms
	EndTime   int64 // in ms

	EvalTargetType      entity.EvalTargetType // now support prompt only
	EvalTargetID        int64
	EvalTargetVersionID int64
}
