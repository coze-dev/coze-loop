// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

type AgentAdapter struct{}

func NewAgentAdapter() rpc.IAgentAdapter {
	return &AgentAdapter{}
}

func (a AgentAdapter) CallTraceAgent(ctx context.Context, spaceID int64, url string, exptId int64, startTime, endTime int64) (int64, error) {
	return 0, errorx.NewByCode(errno.CommonInternalErrorCode, errorx.WithExtraMsg("CallTraceAgent not implement"))
}

func (a AgentAdapter) GetReport(ctx context.Context, spaceID, reportID int64) (report string, list []*entity.InsightAnalysisReportIndex, status entity.ReportStatus, err error) {
	return "", nil, entity.ReportStatus_Failed, errorx.NewByCode(errno.CommonInternalErrorCode, errorx.WithExtraMsg("GetReport not implement"))
}
