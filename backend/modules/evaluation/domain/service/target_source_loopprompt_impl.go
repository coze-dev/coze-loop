// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/bytedance/gg/gcond"
	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func NewPromptSourceEvalTargetServiceImpl(promptRPCAdapter rpc.IPromptRPCAdapter) ISourceEvalTargetOperateService {
	singletonPromptSourceEvalTargetService := &PromptSourceEvalTargetServiceImpl{
		promptRPCAdapter: promptRPCAdapter,
	}
	return singletonPromptSourceEvalTargetService
}

type PromptSourceEvalTargetServiceImpl struct {
	promptRPCAdapter rpc.IPromptRPCAdapter
}

func (t *PromptSourceEvalTargetServiceImpl) EvalType() entity.EvalTargetType {
	return entity.EvalTargetTypeLoopPrompt
}

func (t *PromptSourceEvalTargetServiceImpl) ValidateInput(ctx context.Context, spaceID int64, inputSchema []*entity.ArgsSchema, input *entity.EvalTargetInputData) error {
	return input.ValidateInputSchema(inputSchema)
}

func (t *PromptSourceEvalTargetServiceImpl) Execute(ctx context.Context, spaceID int64, param *entity.ExecuteEvalTargetParam) (evaluatorOutputData *entity.EvalTargetOutputData, status entity.EvalTargetRunStatus, err error) {
	start := time.Now()

	evaluatorOutputData = &entity.EvalTargetOutputData{}
	defer func() {
		timeCostMS := time.Since(start).Milliseconds()
		evaluatorOutputData.TimeConsumingMS = gptr.Of(timeCostMS)
		if err != nil {
			evaluatorOutputData.EvalTargetRunError = &entity.EvalTargetRunError{}
			statusErr, ok := errorx.FromStatusError(err)
			if ok {
				evaluatorOutputData.EvalTargetRunError.Code = statusErr.Code()
				evaluatorOutputData.EvalTargetRunError.Message = statusErr.Error()
			} else {
				evaluatorOutputData.EvalTargetRunError.Code = errno.CommonInternalErrorCode
				evaluatorOutputData.EvalTargetRunError.Message = err.Error()
			}
		}
	}()

	promptID, err := strconv.ParseInt(param.SourceTargetID, 10, 64)
	if err != nil {
		return evaluatorOutputData, entity.EvalTargetRunStatusFail, errorx.WrapByCode(err, errno.CommonInvalidParamCode)
	}
	exePromptParam := &rpc.ExecutePromptParam{
		PromptID:      promptID,
		PromptVersion: param.SourceTargetVersion,
		Variables:     nil,
		History:       param.Input.HistoryMessages,
	}
	vals := make([]*entity.VariableVal, 0)
	for key, content := range param.Input.InputFields {
		if content != nil {
			variable := &entity.VariableVal{
				Key:                 gptr.Of(key),
				Value:               content.Text,
				PlaceholderMessages: nil,
			}
			// placeholder
			placeholder := make([]*entity.Message, 0)
			if content.Text != nil {
				err = json.Unmarshal([]byte(*content.Text), &placeholder)
				if err == nil {
					variable.PlaceholderMessages = placeholder
				}
			}
			vals = append(vals, variable)
		}
	}
	exePromptParam.Variables = vals

	// ExecutePrompt
	executePromptResult, err := t.promptRPCAdapter.ExecutePrompt(ctx, spaceID, exePromptParam)
	if err != nil {
		return evaluatorOutputData, entity.EvalTargetRunStatusFail, err
	}

	var outputStr string

	if executePromptResult == nil {
		outputStr = ""
	} else if executePromptResult.Content != nil {
		outputStr = *executePromptResult.Content
	} else if executePromptResult.ToolCalls != nil {
		outputStr, err = json.MarshalString(executePromptResult.ToolCalls)
	} else {
		outputStr = ""
	}

	evaluatorOutputData.OutputFields = map[string]*entity.Content{
		consts.OutputSchemaKey: {
			ContentType: gptr.Of(entity.ContentTypeText),
			Format:      gptr.Of(entity.Markdown),
			Text:        &outputStr,
		},
	}

	if executePromptResult != nil && executePromptResult.TokenUsage != nil {
		evaluatorOutputData.EvalTargetUsage = &entity.EvalTargetUsage{
			InputTokens:  executePromptResult.TokenUsage.InputTokens,
			OutputTokens: executePromptResult.TokenUsage.OutputTokens,
		}
	}

	return evaluatorOutputData, entity.EvalTargetRunStatusSuccess, nil
}

func (t *PromptSourceEvalTargetServiceImpl) BuildBySource(ctx context.Context, spaceID int64, sourceTargetID, sourceTargetVersion string, opts ...entity.Option) (*entity.EvalTarget, error) {
	promptID, err := strconv.ParseInt(sourceTargetID, 10, 64)
	if err != nil {
		return nil, err
	}
	prompt, err := t.promptRPCAdapter.GetPrompt(ctx, spaceID, promptID, rpc.GetPromptParams{
		CommitVersion: &sourceTargetVersion,
	})
	if err != nil {
		return nil, err
	}
	if prompt == nil {
		return nil, errorx.NewByCode(errno.ResourceNotFoundCode)
	}
	var inputSchema []*entity.ArgsSchema
	if prompt.PromptCommit != nil && prompt.PromptCommit.Detail != nil && prompt.PromptCommit.Detail.PromptTemplate != nil {
		inputSchema = make([]*entity.ArgsSchema, 0)
		for _, p := range prompt.PromptCommit.Detail.PromptTemplate.VariableDefs {
			inputSchema = append(inputSchema, &entity.ArgsSchema{
				Key: p.Key,
				// 目前prompt变量只支持text string类型，后续可以拓展其他类型
				SupportContentTypes: []entity.ContentType{entity.ContentTypeText},
				JsonSchema:          gptr.Of(consts.StringJsonSchema),
			})
		}
	}
	userIDInContext := session.UserIDInCtxOrEmpty(ctx)
	do := &entity.EvalTarget{
		SpaceID:        spaceID,
		SourceTargetID: sourceTargetID,
		EvalTargetType: entity.EvalTargetTypeLoopPrompt,
		EvalTargetVersion: &entity.EvalTargetVersion{
			SpaceID:             spaceID,
			SourceTargetVersion: sourceTargetVersion,
			EvalTargetType:      entity.EvalTargetTypeLoopPrompt,
			Prompt: &entity.LoopPrompt{
				PromptID: promptID,
				Version:  sourceTargetVersion,
			},
			InputSchema: inputSchema,
			OutputSchema: []*entity.ArgsSchema{
				{
					Key: gptr.Of(consts.OutputSchemaKey),
					// 目前prompt输出只支持text string类型，后续可以拓展其他类型
					SupportContentTypes: []entity.ContentType{entity.ContentTypeText, entity.ContentTypeMultipart},
					JsonSchema:          gptr.Of(consts.StringJsonSchema),
				},
			},
			BaseInfo: &entity.BaseInfo{
				CreatedBy: &entity.UserInfo{
					UserID: gptr.Of(userIDInContext),
				},
				UpdatedBy: &entity.UserInfo{
					UserID: gptr.Of(userIDInContext),
				},
			},
		},
		BaseInfo: &entity.BaseInfo{
			CreatedBy: &entity.UserInfo{
				UserID: gptr.Of(userIDInContext),
			},
			UpdatedBy: &entity.UserInfo{
				UserID: gptr.Of(userIDInContext),
			},
		},
	}
	return do, nil
}

func (t *PromptSourceEvalTargetServiceImpl) ListSource(ctx context.Context, param *entity.ListSourceParam) (targets []*entity.EvalTarget, nextCursor string, hasMore bool, err error) {
	// prompt没有滚动分页接口，需要自己适配一下
	page, err := buildPageByCursor(param.Cursor)
	if err != nil {
		return nil, "", false, err
	}
	// 请求prompt列表
	prompts, _, err := t.promptRPCAdapter.ListPrompt(ctx, &rpc.ListPromptParam{
		SpaceID:  param.SpaceID,
		PageSize: param.PageSize,
		Page:     &page,
		KeyWord:  param.KeyWord,
	})
	if err != nil {
		return nil, "", false, err
	}
	// 结果构建
	targets = make([]*entity.EvalTarget, 0)
	for _, p := range prompts {
		var name, desc string
		var status entity.SubmitStatus
		if p.PromptBasic != nil {
			name = gptr.Indirect(p.PromptBasic.DisplayName)
			desc = gptr.Indirect(p.PromptBasic.Description)
			status = gcond.If(p.PromptBasic.LatestVersion == nil, entity.SubmitStatus_UnSubmit, entity.SubmitStatus_Submitted)
		}
		targets = append(targets, &entity.EvalTarget{
			SpaceID:        gptr.Indirect(param.SpaceID),
			SourceTargetID: strconv.FormatInt(p.ID, 10),
			EvalTargetType: entity.EvalTargetTypeLoopPrompt,
			EvalTargetVersion: &entity.EvalTargetVersion{
				SpaceID: gptr.Indirect(param.SpaceID),
				Prompt: &entity.LoopPrompt{
					PromptID:     p.ID,
					PromptKey:    p.PromptKey,
					Name:         name,
					Description:  desc,
					SubmitStatus: status,
				},
			},
		})
	}
	return targets, strconv.FormatInt(int64(page+1), 10), len(prompts) == int(gptr.Indirect(param.PageSize)), nil
}

func (t *PromptSourceEvalTargetServiceImpl) ListSourceVersion(ctx context.Context, param *entity.ListSourceVersionParam) (versions []*entity.EvalTargetVersion, nextCursor string, hasMore bool, err error) {
	parseInt, err := strconv.ParseInt(param.SourceTargetID, 10, 64)
	if err != nil {
		return nil, "", false, err
	}
	prompt, err := t.promptRPCAdapter.GetPrompt(ctx, gptr.Indirect(param.SpaceID), parseInt, rpc.GetPromptParams{})
	if err != nil {
		return nil, "", false, err
	}
	if prompt == nil {
		return nil, "", false, errorx.NewByCode(errno.ResourceNotFoundCode)
	}
	var name string
	var status entity.SubmitStatus
	if prompt.PromptBasic != nil {
		name = gptr.Indirect(prompt.PromptBasic.DisplayName)
		status = gcond.If(prompt.PromptBasic.LatestVersion == nil, entity.SubmitStatus_UnSubmit, entity.SubmitStatus_Submitted)
	}
	info, nextCursor, err := t.promptRPCAdapter.ListPromptVersion(ctx, &rpc.ListPromptVersionParam{
		PromptID: parseInt,
		SpaceID:  param.SpaceID,
		PageSize: param.PageSize,
		Cursor:   param.Cursor,
	})
	if err != nil {
		return nil, "", false, err
	}
	versions = make([]*entity.EvalTargetVersion, 0)
	for _, p := range info {
		desc := p.Description
		versions = append(versions, &entity.EvalTargetVersion{
			SpaceID:             gptr.Indirect(param.SpaceID),
			SourceTargetVersion: gptr.Indirect(p.Version),
			EvalTargetType:      entity.EvalTargetTypeLoopPrompt,
			Prompt: &entity.LoopPrompt{
				PromptID:     prompt.ID,
				Version:      gptr.Indirect(p.Version),
				Name:         name,
				PromptKey:    prompt.PromptKey,
				SubmitStatus: status,
				Description:  gptr.Indirect(desc),
			},
		})
	}
	return versions, nextCursor, len(info) == int(gptr.Indirect(param.PageSize)), nil
}

func (t *PromptSourceEvalTargetServiceImpl) PackSourceInfo(ctx context.Context, spaceID int64, dos []*entity.EvalTarget) (err error) {
	sourcePromptMap := make(map[string]*rpc.LoopPrompt)
	promptQueries := make([]*rpc.MGetPromptQuery, 0)
	for _, do := range dos {
		if do.EvalTargetType != entity.EvalTargetTypeLoopPrompt {
			continue
		}
		id, err := strconv.ParseInt(do.SourceTargetID, 10, 64)
		if err != nil {
			logs.CtxError(ctx, "buildQueries ParseInt err=%v", err)
			continue
		}
		promptQueries = append(promptQueries, &rpc.MGetPromptQuery{
			PromptID: id,
			Version:  nil,
		})
	}
	if len(promptQueries) == 0 {
		return nil
	}
	prompts, err := t.promptRPCAdapter.MGetPrompt(ctx, spaceID, promptQueries)
	if err != nil {
		logs.CtxError(ctx, "packSourceInfo MGetPrompt err=%v", err)
	}
	for _, p := range prompts {
		sourcePromptMap[fmt.Sprintf("%v", p.ID)] = p
	}
	for _, do := range dos {
		if do.EvalTargetType != entity.EvalTargetTypeLoopPrompt {
			continue
		}
		if p, ok := sourcePromptMap[fmt.Sprintf("%v", do.SourceTargetID)]; ok {
			var name string
			if p.PromptBasic != nil {
				name = gptr.Indirect(p.PromptBasic.DisplayName)
			}
			do.EvalTargetVersion = &entity.EvalTargetVersion{
				Prompt: &entity.LoopPrompt{
					Name: name,
				},
			}
		}
	}
	return nil
}

func (t *PromptSourceEvalTargetServiceImpl) PackSourceVersionInfo(ctx context.Context, spaceID int64, dos []*entity.EvalTarget) (err error) {
	sourcePromptMap := make(map[string]*rpc.LoopPrompt)
	promptQueries := make([]*rpc.MGetPromptQuery, 0)
	for _, do := range dos {
		if do.EvalTargetType != entity.EvalTargetTypeLoopPrompt {
			continue
		}
		if do.EvalTargetVersion == nil || do.EvalTargetVersion.Prompt == nil {
			continue
		}
		promptQueries = append(promptQueries, &rpc.MGetPromptQuery{
			PromptID: do.EvalTargetVersion.Prompt.PromptID,
			Version:  &do.EvalTargetVersion.SourceTargetVersion,
		})
	}
	if len(promptQueries) == 0 {
		return nil
	}
	prompts, err := t.promptRPCAdapter.MGetPrompt(ctx, spaceID, promptQueries)
	if err != nil {
		logs.CtxError(ctx, "packSourceInfoWithVersion MGetPrompt err=%v", err)
	}
	for _, p := range prompts {
		if p.PromptCommit == nil || p.PromptCommit.CommitInfo == nil {
			continue
		}
		sourcePromptMap[fmt.Sprintf("%v_%v", p.ID, gptr.Indirect(p.PromptCommit.CommitInfo.Version))] = p
	}

	for _, do := range dos {
		if do.EvalTargetType != entity.EvalTargetTypeLoopPrompt {
			continue
		}
		if do.EvalTargetVersion == nil || do.EvalTargetVersion.Prompt == nil {
			continue
		}
		if p, ok := sourcePromptMap[fmt.Sprintf("%v_%v", do.SourceTargetID, do.EvalTargetVersion.SourceTargetVersion)]; ok {
			var name string
			if p.PromptBasic != nil {
				name = gptr.Indirect(p.PromptBasic.DisplayName)
			}
			do.EvalTargetVersion.Prompt.Name = name
			if p.PromptCommit != nil && p.PromptCommit.CommitInfo != nil {
				do.EvalTargetVersion.Prompt.Description = gptr.Indirect(p.PromptCommit.CommitInfo.Description)
			}
		} else {
			do.BaseInfo.DeletedAt = gptr.Of(int64(1)) // 说明源数据已删除
		}
	}
	return nil
}

func (t *PromptSourceEvalTargetServiceImpl) BatchGetSource(ctx context.Context, spaceID int64, ids []string) (targets []*entity.EvalTarget, err error) {
	promptQueries := make([]*rpc.MGetPromptQuery, 0)
	for _, id := range ids {
		promptID, err := strconv.ParseInt(id, 10, 64)
		if err != nil {
			logs.CtxError(ctx, "buildQueries ParseInt err=%v", err)
			continue
		}
		promptQueries = append(promptQueries, &rpc.MGetPromptQuery{
			PromptID: promptID,
			Version:  nil,
		})
	}
	if len(promptQueries) == 0 {
		return nil, nil
	}
	prompts, err := t.promptRPCAdapter.MGetPrompt(ctx, spaceID, promptQueries)
	if err != nil {
		return nil, err
	}
	targets = make([]*entity.EvalTarget, 0)
	for _, p := range prompts {
		targets = append(targets, &entity.EvalTarget{
			SpaceID:        spaceID,
			SourceTargetID: fmt.Sprintf("%v", p.ID),
			EvalTargetType: entity.EvalTargetTypeLoopPrompt,
			EvalTargetVersion: &entity.EvalTargetVersion{
				SpaceID:        spaceID,
				EvalTargetType: entity.EvalTargetTypeLoopPrompt,
				Prompt: &entity.LoopPrompt{
					PromptID:    p.ID,
					Name:        gptr.Indirect(p.PromptBasic.DisplayName),
					PromptKey:   p.PromptKey,
					Description: gptr.Indirect(p.PromptBasic.Description),
				},
			},
		})
	}
	return targets, nil
}
