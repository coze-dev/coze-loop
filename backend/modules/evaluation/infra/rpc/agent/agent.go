// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type AgentAdapter struct{}

func NewAgentAdapter() rpc.IAgentAdapter {
	return &AgentAdapter{}
}

func (a AgentAdapter) CallTraceAgent(ctx context.Context, spaceID int64, url string) (int64, error) {
	return 0, nil
}

func (a AgentAdapter) GetReport(ctx context.Context, spaceID, reportID int64) (report string, status entity.ReportStatus, err error) {
	return "", 0, nil
}
