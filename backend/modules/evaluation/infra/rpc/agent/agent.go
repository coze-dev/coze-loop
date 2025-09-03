// Copyright (c) 2025 Bytedance Ltd. and/or its affiliates
// SPDX-License-Identifier: Apache-2.0

package agent

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

type AgentAdapter struct {
	//traceagentservice.Client
}

func NewAgentAdapter() rpc.IAgentAdapter {
	return &AgentAdapter{}
}

func (a AgentAdapter) CallTraceAgent(ctx context.Context, spaceID int64, url string) (int64, error) {
	//req := &trace_agent.CallTraceAgentRequest{
	//	SpaceID: ptr.Of(spaceID),
	//	CsvURL:  ptr.Of(url),
	//}
	//resp, err := a.Client.CallTraceAgent(ctx, req)
	//if err != nil {
	//	return 0, err
	//}
	//
	//if resp.ReportID == nil {
	//	return 0, fmt.Errorf("empty report id")
	//}
	//
	//return ptr.From(resp.ReportID), nil
	return 0, nil
}

func (a AgentAdapter) GetReport(ctx context.Context, spaceID, reportID int64) (report string, status entity.ReportStatus, err error) {
	//req := &trace_agent.GetReportRequest{
	//	SpaceID:  ptr.Of(spaceID),
	//	ReportID: ptr.Of(reportID),
	//}
	//resp, err := a.Client.GetReport(ctx, req)
	//if err != nil {
	//	return "", 0, err
	//}
	//
	//return ptr.From(resp.Report), entity.ReportStatus(ptr.From(resp.Status)), nil

	return "", 0, nil
}
