// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"strconv"

	"github.com/coze-dev/cozeloop-go"
	loopentity "github.com/coze-dev/cozeloop-go/entity"
	"github.com/coze-dev/cozeloop-go/spec/tracespec"

	"github.com/coze-dev/coze-loop/backend/infra/looptracer"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/prompt"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/execute"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/application/convertor"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/trace"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/consts"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/traceutil"
)

func NewPromptExecuteApplication(
	promptService service.IPromptService,
	promptManageRepo repo.IManageRepo,
) execute.PromptExecuteService {
	return &PromptExecuteApplicationImpl{
		promptService: promptService,
		manageRepo:    promptManageRepo,
	}
}

type PromptExecuteApplicationImpl struct {
	promptService service.IPromptService
	manageRepo    repo.IManageRepo
}

func (p *PromptExecuteApplicationImpl) ExecuteInternal(ctx context.Context, req *execute.ExecuteInternalRequest) (r *execute.ExecuteInternalResponse, err error) {
	r = execute.NewExecuteInternalResponse()
	var span cozeloop.Span
	ctx, span = p.startPromptExecutorSpan(ctx, startPromptExecutorSpanParam{
		workspaceID:      req.GetWorkspaceID(),
		bizScene:         req.Scenario,
		stream:           false,
		reqPromptID:      req.GetPromptID(),
		reqPromptVersion: req.GetVersion(),
		messages:         convertor.BatchMessageDTO2DO(req.Messages),
		variableVals:     convertor.BatchVariableValDTO2DO(req.VariableVals),
		overrideParams:   overrideParamsConvert(req.OverridePromptParams),
	})
	var promptDO *entity.Prompt
	var reply *entity.Reply
	defer func() {
		p.finishPromptExecutorSpan(ctx, span, promptDO, reply, err)
	}()
	// 内部接口不鉴权
	// retrieve prompt
	promptDO, err = p.getPromptByID(ctx, req.GetWorkspaceID(), req.GetPromptID(), req.GetVersion())
	if err != nil {
		return r, err
	}
	overridePromptParams(promptDO, req.OverridePromptParams)
	// execute
	reply, err = p.promptService.Execute(ctx, service.ExecuteParam{
		Prompt:       promptDO,
		Messages:     convertor.BatchMessageDTO2DO(req.Messages),
		VariableVals: convertor.BatchVariableValDTO2DO(req.VariableVals),
		SingleStep:   false, // 内部接口不支持单步调试
		Scenario:     convertor.ScenarioDTO2DO(req.GetScenario()),
	})
	if err != nil {
		return r, err
	}
	if reply != nil && reply.Item != nil {
		r.Message = convertor.MessageDO2DTO(reply.Item.Message)
		r.FinishReason = ptr.Of(reply.Item.FinishReason)
		r.Usage = convertor.TokenUsageDO2DTO(reply.Item.TokenUsage)
	}
	return r, nil
}

type startPromptExecutorSpanParam struct {
	workspaceID      int64
	bizScene         *prompt.Scenario
	stream           bool
	reqPromptID      int64
	reqPromptKey     string
	reqPromptVersion string
	messages         []*entity.Message
	variableVals     []*entity.VariableVal
	overrideParams   *overrideParams
}

type overrideParams struct {
	ModelConfig *entity.ModelConfig `json:"model_config"`
}

func overrideParamsConvert(dto *prompt.OverridePromptParams) *overrideParams {
	if dto == nil {
		return nil
	}
	return &overrideParams{
		ModelConfig: convertor.ModelConfigDTO2DO(dto.GetModelConfig()),
	}
}

func (p *PromptExecuteApplicationImpl) startPromptExecutorSpan(ctx context.Context, param startPromptExecutorSpanParam) (context.Context, cozeloop.Span) {
	// 上游已经设置call_type过则不再设置
	var hasSetCallType bool
	if parentSpan := cozeloop.GetSpanFromContext(ctx); parentSpan != nil && parentSpan.GetSpanID() != "" {
		for k, v := range parentSpan.GetBaggage() {
			if k == consts.SpanTagCallType && v != "" {
				hasSetCallType = true
				break
			}
		}
	}
	var span looptracer.Span
	ctx, span = looptracer.GetTracer().StartSpan(ctx, consts.SpanNamePromptExecutor, consts.SpanTypePromptExecutor,
		looptracer.WithSpanWorkspaceID(strconv.FormatInt(param.workspaceID, 10)))
	if span != nil {
		if !hasSetCallType {
			// todo: 目前只有评测，默认为评测
			span.SetCallType(consts.SpanTagCallTypeEvaluation)
		}
		intput := map[string]any{
			tracespec.PromptVersion:            param.reqPromptVersion,
			consts.SpanTagPromptVariables:      trace.VariableValsToSpanPromptVariables(param.variableVals),
			consts.SpanTagMessages:             trace.MessagesToSpanMessages(param.messages),
			consts.SpanTagOverridePromptParams: param.overrideParams,
		}
		if param.reqPromptKey != "" {
			intput[tracespec.PromptKey] = param.reqPromptKey
		} else {
			intput[consts.SpanTagPromptID] = strconv.FormatInt(param.reqPromptID, 10)
		}
		span.SetInput(ctx, json.Jsonify(intput))
		span.SetTags(ctx, map[string]any{
			tracespec.Stream: param.stream,
		})
	}
	return ctx, span
}

func (p *PromptExecuteApplicationImpl) finishPromptExecutorSpan(ctx context.Context, span cozeloop.Span, prompt *entity.Prompt, reply *entity.Reply, err error) {
	if span == nil || prompt == nil {
		return
	}
	var debugID int64
	var replyItem *entity.ReplyItem
	if reply != nil {
		debugID = reply.DebugID
		replyItem = reply.Item
	}
	var inputTokens, outputTokens int64
	if replyItem != nil && replyItem.TokenUsage != nil {
		inputTokens = replyItem.TokenUsage.InputTokens
		outputTokens = replyItem.TokenUsage.OutputTokens
	}
	span.SetPrompt(ctx, loopentity.Prompt{PromptKey: prompt.PromptKey, Version: prompt.GetVersion()})
	span.SetOutput(ctx, json.Jsonify(trace.ReplyItemToSpanOutput(replyItem)))
	span.SetInputTokens(ctx, int(inputTokens))
	span.SetOutputTokens(ctx, int(outputTokens))
	span.SetTags(ctx, map[string]any{
		consts.SpanTagDebugID: debugID,
	})
	if err != nil {
		span.SetStatusCode(ctx, int(traceutil.GetTraceStatusCode(err)))
		span.SetError(ctx, errors.New(errorx.ErrorWithoutStack(err)))
	}
	span.Finish(ctx)
}

func (p *PromptExecuteApplicationImpl) getPromptByID(ctx context.Context, spaceID int64, promptID int64, version string) (prompt *entity.Prompt, err error) {
	var span looptracer.Span
	ctx, span = looptracer.GetTracer().StartSpan(ctx, consts.SpanNamePromptHub, tracespec.VPromptHubSpanType, looptracer.WithSpanWorkspaceID(strconv.FormatInt(spaceID, 10)))
	if span != nil {
		span.SetInput(ctx, json.Jsonify(map[string]any{
			consts.SpanTagPromptID:  strconv.FormatInt(promptID, 10),
			tracespec.PromptVersion: version,
		}))
		defer func() {
			if prompt != nil {
				span.SetPrompt(ctx, loopentity.Prompt{PromptKey: prompt.PromptKey, Version: prompt.GetVersion()})
				span.SetOutput(ctx, json.Jsonify(trace.PromptToSpanPrompt(prompt)))
			}
			if err != nil {
				span.SetStatusCode(ctx, int(traceutil.GetTraceStatusCode(err)))
				span.SetError(ctx, errors.New(errorx.ErrorWithoutStack(err)))
			}
			span.Finish(ctx)
		}()
	}
	return p.manageRepo.GetPrompt(ctx, repo.GetPromptParam{
		PromptID:      promptID,
		WithCommit:    true,
		CommitVersion: version,
	})
}

func overridePromptParams(promptDO *entity.Prompt, overrideParams *prompt.OverridePromptParams) {
	if promptDO == nil || overrideParams == nil {
		return
	}
	if promptDO.GetPromptDetail() != nil && overrideParams.ModelConfig != nil {
		promptDO.PromptCommit.PromptDetail.ModelConfig = convertor.ModelConfigDTO2DO(overrideParams.ModelConfig)
	}
}
