// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package prompt

import (
	"context"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/apis/promptexecuteservice"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/prompt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/execute"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/manage"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/promptmanageservice"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/js_conv"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type PromptRPCAdapter struct {
	client        promptmanageservice.Client
	executeClient promptexecuteservice.Client
}

func NewPromptRPCAdapter(client promptmanageservice.Client, executeClient promptexecuteservice.Client) rpc.IPromptRPCAdapter {
	return &PromptRPCAdapter{
		client:        client,
		executeClient: executeClient,
	}
}

func (p PromptRPCAdapter) ExecutePrompt(ctx context.Context, spaceID int64, param *rpc.ExecutePromptParam) (result *rpc.ExecutePromptResult, err error) {
	req := &execute.ExecuteInternalRequest{
		PromptID:     gptr.Of(param.PromptID),
		WorkspaceID:  gptr.Of(spaceID),
		Version:      gptr.Of(param.PromptVersion),
		Messages:     nil,
		VariableVals: nil,
		Scenario:     gptr.Of(prompt.ScenarioEvalTarget),
	}
	// logs.CtxInfo(ctx, "ExecutePrompt History=%v, Variables=%v", json.Jsonify(param.History), json.Jsonify(param.Variables))
	req.VariableVals = ConvertVariables2Prompt(param.Variables)

	req.Messages = ConvertMessages2Prompt(param.History)

	if runtimeParam, err := p.parseRuntimeParam(ctx, gptr.Indirect(param.RuntimeParam)); err != nil {
		logs.CtxError(ctx, "prompt execute parse runtime param fail, err=%v", err)
	} else {
		if runtimeParam != nil && runtimeParam.ModelConfig != nil {
			req.OverridePromptParams = &prompt.OverridePromptParams{
				ModelConfig: &prompt.ModelConfig{
					ModelID:     gptr.Of(runtimeParam.ModelConfig.ModelID),
					MaxTokens:   runtimeParam.ModelConfig.MaxTokens,
					Temperature: runtimeParam.ModelConfig.Temperature,
					TopP:        runtimeParam.ModelConfig.TopP,
				},
			}
		}
	}

	resp, err := p.executeClient.ExecuteInternal(ctx, req)
	if err != nil {
		return nil, err
	}

	if resp == nil {
		return nil, errorx.NewByCode(errno.CommonRPCErrorCode)
	}
	if resp.BaseResp != nil && resp.BaseResp.StatusCode != 0 {
		return nil, errorx.NewByCode(resp.BaseResp.StatusCode, errorx.WithExtraMsg(resp.BaseResp.StatusMessage))
	}
	if resp.Message == nil {
		return nil, nil
	}

	result = &rpc.ExecutePromptResult{}
	result.Content = resp.Message.Content
	result.ToolCalls = ConvertPromptToolCalls2Eval(resp.GetMessage().GetToolCalls())
	result.TokenUsage = &entity.TokenUsage{
		InputTokens:  resp.GetUsage().GetInputTokens(),
		OutputTokens: resp.GetUsage().GetOutputTokens(),
	}
	return result, nil
}

func (p PromptRPCAdapter) parseRuntimeParam(ctx context.Context, rtp string) (*entity.PromptRuntimeParam, error) {
	if len(rtp) == 0 {
		return &entity.PromptRuntimeParam{}, nil
	}
	runtimeParam := new(entity.PromptRuntimeParam)
	if err := js_conv.GetUnmarshaler()([]byte(rtp), runtimeParam); err != nil {
		return runtimeParam, errorx.Wrapf(err, "PromptRuntimeParam json unmarshal fail, raw: %s", rtp)
	}
	return runtimeParam, nil
}

func (p PromptRPCAdapter) GetPrompt(ctx context.Context, spaceID, promptID int64, params rpc.GetPromptParams) (prompt *rpc.LoopPrompt, err error) {
	req := &manage.GetPromptRequest{
		PromptID: gptr.Of(promptID),
	}
	if params.CommitVersion != nil {
		req.CommitVersion = params.CommitVersion
		req.WithCommit = gptr.Of(true)
	}
	resp, err := p.client.GetPrompt(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errorx.NewByCode(errno.CommonRPCErrorCode)
	}
	if resp.BaseResp != nil && resp.BaseResp.StatusCode != 0 {
		return nil, errorx.NewByCode(resp.BaseResp.StatusCode, errorx.WithExtraMsg(resp.BaseResp.StatusMessage))
	}
	if resp.Prompt == nil {
		return nil, nil
	}
	res := ConvertToLoopPrompt(resp.Prompt)
	return res, nil
}

func (p PromptRPCAdapter) MGetPrompt(ctx context.Context, spaceID int64, promptQueries []*rpc.MGetPromptQuery) (prompts []*rpc.LoopPrompt, err error) {
	queries := make([]*manage.PromptQuery, 0, len(promptQueries))
	for _, query := range promptQueries {
		promptQuery := &manage.PromptQuery{
			PromptID: &query.PromptID,
		}
		if query.Version != nil {
			promptQuery.WithCommit = gptr.Of(true)
			promptQuery.CommitVersion = query.Version
		}
		queries = append(queries, promptQuery)
	}
	resp, err := p.client.BatchGetPrompt(ctx, &manage.BatchGetPromptRequest{
		Queries: queries,
	})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, errorx.NewByCode(errno.CommonRPCErrorCode)
	}
	if resp.BaseResp != nil && resp.BaseResp.StatusCode != 0 {
		return nil, errorx.NewByCode(resp.BaseResp.StatusCode, errorx.WithExtraMsg(resp.BaseResp.StatusMessage))
	}
	var promptDTOs []*prompt.Prompt
	for _, promptDTO := range resp.Results {
		if promptDTO.Prompt == nil {
			continue
		}
		promptDTOs = append(promptDTOs, promptDTO.Prompt)
	}
	return ConvertToLoopPrompts(promptDTOs), nil
}

func (p PromptRPCAdapter) ListPrompt(ctx context.Context, param *rpc.ListPromptParam) (prompts []*rpc.LoopPrompt, total *int32, err error) {
	resp, err := p.client.ListPrompt(ctx, &manage.ListPromptRequest{
		WorkspaceID: param.SpaceID,
		PageNum:     param.Page,
		PageSize:    param.PageSize,
		KeyWord:     param.KeyWord,
	})
	if err != nil {
		return nil, nil, err
	}
	if resp == nil {
		return nil, nil, errorx.NewByCode(errno.CommonRPCErrorCode)
	}
	if resp.BaseResp != nil && resp.BaseResp.StatusCode != 0 {
		return nil, nil, errorx.NewByCode(resp.BaseResp.StatusCode, errorx.WithExtraMsg(resp.BaseResp.StatusMessage))
	}
	return ConvertToLoopPrompts(resp.Prompts), resp.Total, nil
}

func (p PromptRPCAdapter) ListPromptVersion(ctx context.Context, param *rpc.ListPromptVersionParam) (prompts []*rpc.CommitInfo, nextCursor string, err error) {
	resp, err := p.client.ListCommit(ctx, &manage.ListCommitRequest{
		PromptID:  &param.PromptID,
		PageToken: param.Cursor,
		PageSize:  param.PageSize,
	})
	if err != nil {
		return nil, "", err
	}
	if resp == nil {
		return nil, "", errorx.NewByCode(errno.CommonRPCErrorCode)
	}
	if resp.BaseResp != nil && resp.BaseResp.StatusCode != 0 {
		return nil, "", errorx.NewByCode(resp.BaseResp.StatusCode, errorx.WithExtraMsg(resp.BaseResp.StatusMessage))
	}
	res := make([]*rpc.CommitInfo, 0)
	for _, c := range resp.GetPromptCommitInfos() {
		res = append(res, &rpc.CommitInfo{
			Version:     gptr.Of(c.GetVersion()),
			BaseVersion: gptr.Of(c.GetBaseVersion()),
			Description: gptr.Of(c.GetDescription()),
			CommittedAt: gptr.Of(c.GetCommittedAt()),
			CommittedBy: gptr.Of(c.GetCommittedBy()),
		})
	}
	if resp.NextPageToken == nil {
		return res, "", nil
	}
	return res, *resp.NextPageToken, nil
}
