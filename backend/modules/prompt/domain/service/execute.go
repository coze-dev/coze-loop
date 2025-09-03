// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"time"

	"github.com/coze-dev/cozeloop-go"
	loopentity "github.com/coze-dev/cozeloop-go/entity"
	"github.com/coze-dev/cozeloop-go/spec/tracespec"
	"github.com/deatil/go-encoding/encoding"
	"github.com/google/uuid"

	"github.com/coze-dev/coze-loop/backend/infra/looptracer"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/trace"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/consts"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	loopslices "github.com/coze-dev/coze-loop/backend/pkg/lang/slices"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"github.com/coze-dev/coze-loop/backend/pkg/traceutil"
)

const (
	maxIterations = 8
	maxDuration   = 8 * time.Minute
)

type ExecuteParam struct {
	Prompt       *entity.Prompt
	Messages     []*entity.Message
	VariableVals []*entity.VariableVal

	MockTools     []*entity.MockTool
	SingleStep    bool
	DebugTraceKey string

	Scenario entity.Scenario
}

type ExecuteStreamingParam struct {
	ExecuteParam
	ResultStream chan<- *entity.Reply
}

func (p *PromptServiceImpl) FormatPrompt(ctx context.Context, prompt *entity.Prompt, messages []*entity.Message, variableVals []*entity.VariableVal) (formattedMessages []*entity.Message, err error) {
	if parentSpan := looptracer.GetTracer().GetSpanFromContext(ctx); parentSpan != nil {
		var span looptracer.Span
		ctx, span = looptracer.GetTracer().StartSpan(ctx, consts.SpanNamePromptTemplate, tracespec.VPromptTemplateSpanType, looptracer.WithSpanWorkspaceID(strconv.FormatInt(prompt.SpaceID, 10)))
		if span != nil {
			span.SetPrompt(ctx, loopentity.Prompt{PromptKey: prompt.PromptKey, Version: prompt.GetVersion()})
			span.SetInput(ctx, json.Jsonify(tracespec.PromptInput{
				Templates: trace.MessagesToSpanMessages(prompt.GetTemplateMessages(messages)),
				Arguments: trace.VariableValsToSpanPromptVariables(variableVals),
			}))
			defer func() {
				span.SetOutput(ctx, json.Jsonify(tracespec.PromptOutput{
					Prompts: trace.MessagesToSpanMessages(formattedMessages),
				}))
				if err != nil {
					span.SetStatusCode(ctx, int(traceutil.GetTraceStatusCode(err)))
					span.SetError(ctx, errors.New(errorx.ErrorWithoutStack(err)))
				}
				span.Finish(ctx)
			}()
		}
	}
	return prompt.FormatMessages(messages, variableVals)
}

func (p *PromptServiceImpl) ExecuteStreaming(ctx context.Context, param ExecuteStreamingParam) (aggregatedReply *entity.Reply, err error) {
	if param.Prompt == nil || param.ResultStream == nil {
		return nil, errorx.New("invalid param")
	}
	debugID, debugStep, err := p.getDebugIDAndStep(ctx, param.SingleStep, param.DebugTraceKey)
	if err != nil {
		return nil, err
	}
	defer func() {
		// 报错时，将debug_id返回
		if aggregatedReply == nil {
			aggregatedReply = &entity.Reply{
				DebugID: debugID,
			}
		}
	}()
	startTime := time.Now()
	// 统计多轮总token消耗
	// notice: 流式每轮的token单独在chunk中返回，没有总计
	tokenUsage := &entity.TokenUsage{
		InputTokens:  0,
		OutputTokens: 0,
	}
	for {
		replyItemWrapper, err := getReplyItemWrapper(debugID, debugStep)
		if err != nil {
			return nil, err
		}
		aggregatedReply, err = p.doStreamingIteration(ctx, param, replyItemWrapper)
		if err != nil {
			return nil, err
		}
		if aggregatedReply != nil && aggregatedReply.Item != nil && aggregatedReply.Item.TokenUsage != nil {
			tokenUsage.InputTokens += aggregatedReply.Item.TokenUsage.InputTokens
			tokenUsage.OutputTokens += aggregatedReply.Item.TokenUsage.OutputTokens
		}

		if !shouldContinue(param.SingleStep, startTime, debugStep, aggregatedReply) {
			break
		}
		debugStep++
		// 多轮执行需要重新编排上下文
		param.Messages, err = reorganizeContexts(param.Messages, param.MockTools, aggregatedReply)
		if err != nil {
			return nil, err
		}
	}
	if aggregatedReply != nil && aggregatedReply.Item != nil {
		aggregatedReply.Item.TokenUsage = tokenUsage
	}
	return aggregatedReply, nil
}

func (p *PromptServiceImpl) Execute(ctx context.Context, param ExecuteParam) (reply *entity.Reply, err error) {
	if param.Prompt == nil {
		return nil, errorx.New("invalid param")
	}
	debugID, debugStep, err := p.getDebugIDAndStep(ctx, param.SingleStep, param.DebugTraceKey)
	if err != nil {
		return nil, err
	}
	startTime := time.Now()
	// 统计多轮总token消耗
	tokenUsage := &entity.TokenUsage{
		InputTokens:  0,
		OutputTokens: 0,
	}
	for {
		replyItemWrapper, err := getReplyItemWrapper(debugID, debugStep)
		if err != nil {
			return nil, err
		}
		reply, err = p.doIteration(ctx, param, replyItemWrapper)
		if err != nil {
			return nil, err
		}
		if reply != nil && reply.Item != nil && reply.Item.TokenUsage != nil {
			tokenUsage.InputTokens += reply.Item.TokenUsage.InputTokens
			tokenUsage.OutputTokens += reply.Item.TokenUsage.OutputTokens
		}

		if !shouldContinue(param.SingleStep, startTime, debugStep, reply) {
			break
		}
		debugStep++
		// 多轮执行需要重新编排上下文
		param.Messages, err = reorganizeContexts(param.Messages, param.MockTools, reply)
		if err != nil {
			return nil, err
		}
	}
	if reply != nil && reply.Item != nil {
		reply.Item.TokenUsage = tokenUsage
	}
	return reply, nil
}

func (p *PromptServiceImpl) doStreamingIteration(ctx context.Context, param ExecuteStreamingParam, replyItemWrapper func(v *entity.ReplyItem) *entity.Reply) (aggregatedReply *entity.Reply, err error) {
	var span cozeloop.Span
	ctx, span = p.startSequenceSpan(ctx, param.Prompt, param.Messages, param.VariableVals)
	defer func() {
		p.finishSequenceSpan(ctx, span, aggregatedReply, err)
	}()
	var llmCallParam rpc.LLMCallParam
	llmCallParam, err = p.prepareLLMCallParam(ctx, param.ExecuteParam)
	if err != nil {
		return nil, err
	}
	var aggregatedResult *entity.ReplyItem

	resultStream := make(chan *entity.ReplyItem)
	errChan := make(chan error)
	go func() {
		var llmCallErr error
		defer func() {
			e := recover()
			if e != nil {
				llmCallErr = errorx.New("panic occurred, reason=%v", e)
			}
			// 确保errChan和resultStream被关闭
			close(resultStream)
			if llmCallErr != nil {
				errChan <- llmCallErr
			}
			close(errChan)
		}()
		aggregatedResult, llmCallErr = p.llm.StreamingCall(ctx, rpc.LLMStreamingCallParam{
			LLMCallParam: llmCallParam,
			ResultStream: resultStream,
		})
		if llmCallErr != nil {
			return
		}
	}()
	for v := range resultStream {
		param.ResultStream <- replyItemWrapper(v)
	}
	select { //nolint:staticcheck
	case err = <-errChan:
		if err != nil {
			return nil, err
		}
	}

	// report tool call span
	p.reportToolSpan(ctx, param.Prompt, param.MockTools, aggregatedResult)
	return replyItemWrapper(aggregatedResult), nil
}

func (p *PromptServiceImpl) doIteration(ctx context.Context, param ExecuteParam, replyItemWrapper func(v *entity.ReplyItem) *entity.Reply) (aggregatedReply *entity.Reply, err error) {
	var span cozeloop.Span
	ctx, span = p.startSequenceSpan(ctx, param.Prompt, param.Messages, param.VariableVals)
	defer func() {
		p.finishSequenceSpan(ctx, span, aggregatedReply, err)
	}()
	var llmCallParam rpc.LLMCallParam
	llmCallParam, err = p.prepareLLMCallParam(ctx, param)
	if err != nil {
		return nil, err
	}
	var aggregatedResult *entity.ReplyItem
	aggregatedResult, err = p.llm.Call(ctx, llmCallParam)
	if err != nil {
		return nil, err
	}
	// tool call处理
	p.reportToolSpan(ctx, param.Prompt, param.MockTools, aggregatedResult)
	return replyItemWrapper(aggregatedResult), nil
}

func getReplyItemWrapper(debugID int64, debugStep int32) (func(v *entity.ReplyItem) *entity.Reply, error) {
	nextDebugTraceKey, err := encodeDebugIDAndStep(debugID, debugStep+1)
	if err != nil {
		return nil, err
	}
	replyItemWrapper := func(v *entity.ReplyItem) *entity.Reply {
		if v == nil {
			return nil
		}
		return &entity.Reply{
			Item:          v,
			DebugID:       debugID,
			DebugStep:     debugStep,
			DebugTraceKey: nextDebugTraceKey,
		}
	}
	return replyItemWrapper, nil
}

func (p *PromptServiceImpl) getDebugIDAndStep(ctx context.Context, singleStepDebug bool, debugTraceKey string) (traceID int64, traceStep int32, err error) {
	// 非单步调试，传入的debugTraceKey无效
	if !singleStepDebug {
		debugTraceKey = ""
	}
	if debugTraceKey != "" {
		// 传递了则解析
		traceID, traceStep, err = decodeDebugIDAndStep(debugTraceKey)
		if err != nil {
			return traceID, traceStep, err
		}
	} else {
		// 第一次不传递，生成
		traceID, err = p.idgen.GenID(ctx)
		if err != nil {
			logs.CtxError(ctx, "GenID err=%v", err)
			traceID = int64(uuid.New().ID())
		}
		traceStep = 1
	}
	return traceID, traceStep, nil
}

func (p *PromptServiceImpl) reportToolSpan(ctx context.Context, prompt *entity.Prompt, mockTools []*entity.MockTool, result *entity.ReplyItem) {
	if result == nil || result.Message == nil || len(result.Message.ToolCalls) == 0 {
		return
	}
	var spaceID int64
	var promptKey, version string
	if prompt != nil {
		spaceID = prompt.SpaceID
		promptKey = prompt.PromptKey
		version = prompt.GetVersion()
	}
	mockToolResponseMap := loopslices.ToMap(mockTools, func(m *entity.MockTool) (string, string) {
		if m == nil {
			return "", ""
		}
		return m.Name, m.MockResponse
	})
	for _, toolCall := range result.Message.ToolCalls {
		if toolCall != nil && toolCall.FunctionCall != nil {
			var span looptracer.Span
			ctx, span = looptracer.GetTracer().StartSpan(ctx, toolCall.FunctionCall.Name, tracespec.VToolSpanType, looptracer.WithSpanWorkspaceID(strconv.FormatInt(spaceID, 10)))
			if span != nil {
				span.SetPrompt(ctx, loopentity.Prompt{PromptKey: promptKey, Version: version})
				span.SetInput(ctx, toolCall.FunctionCall.Arguments)
				span.SetOutput(ctx, mockToolResponseMap[toolCall.FunctionCall.Name])
				span.Finish(ctx)
			}
		}
	}
}

func reorganizeContexts(contexts []*entity.Message, mockTools []*entity.MockTool, reply *entity.Reply) ([]*entity.Message, error) {
	newContexts := slices.Clone(contexts)
	if reply == nil || reply.Item == nil || reply.Item.Message == nil {
		return newContexts, nil
	}
	newContexts = append(newContexts, reply.Item.Message)
	if len(reply.Item.Message.ToolCalls) > 0 {
		// 如果有工具调用，则需要mock response
		mockToolResponseMap := loopslices.ToMap(mockTools, func(m *entity.MockTool) (string, string) {
			if m == nil {
				return "", ""
			}
			return m.Name, m.MockResponse
		})
		for _, toolCall := range reply.Item.Message.ToolCalls {
			if toolCall.FunctionCall != nil {
				newContexts = append(newContexts, &entity.Message{
					Role:       entity.RoleTool,
					ToolCallID: ptr.Of(toolCall.ID),
					Content:    ptr.Of(mockToolResponseMap[toolCall.FunctionCall.Name]),
				})
			}
		}
	}
	return newContexts, nil
}

func (p *PromptServiceImpl) startSequenceSpan(ctx context.Context, prompt *entity.Prompt, messages []*entity.Message, variableVals []*entity.VariableVal) (context.Context, cozeloop.Span) {
	if prompt == nil {
		return ctx, nil
	}
	var span looptracer.Span
	ctx, span = looptracer.GetTracer().StartSpan(ctx, consts.SpanNameSequence, consts.SpanTypeSequence, looptracer.WithSpanWorkspaceID(strconv.FormatInt(prompt.SpaceID, 10)))
	if span != nil {
		var templateMessages []*entity.Message
		promptDetail := prompt.GetPromptDetail()
		if promptDetail != nil && promptDetail.PromptTemplate != nil {
			templateMessages = promptDetail.PromptTemplate.Messages
		}
		span.SetPrompt(ctx, loopentity.Prompt{PromptKey: prompt.PromptKey, Version: prompt.GetVersion()})
		span.SetInput(ctx, json.Jsonify(map[string]any{
			consts.SpanTagPromptTemplate:  trace.MessagesToSpanMessages(templateMessages),
			consts.SpanTagPromptVariables: trace.VariableValsToSpanPromptVariables(variableVals),
			consts.SpanTagMessages:        trace.MessagesToSpanMessages(messages),
		}))
	}
	return ctx, span
}

func (p *PromptServiceImpl) finishSequenceSpan(ctx context.Context, span cozeloop.Span, aggregatedReply *entity.Reply, err error) {
	if span == nil {
		return
	}
	var replyItem *entity.ReplyItem
	if aggregatedReply != nil {
		replyItem = aggregatedReply.Item
	}
	span.SetOutput(ctx, json.Jsonify(trace.ReplyItemToSpanOutput(replyItem)))
	if err != nil {
		span.SetStatusCode(ctx, int(traceutil.GetTraceStatusCode(err)))
		span.SetError(ctx, errors.New(errorx.ErrorWithoutStack(err)))
	}
	span.Finish(ctx)
}

func (p *PromptServiceImpl) prepareLLMCallParam(ctx context.Context, param ExecuteParam) (rpc.LLMCallParam, error) {
	// format messages
	messages, err := p.FormatPrompt(ctx, param.Prompt, param.Messages, param.VariableVals)
	if err != nil {
		return rpc.LLMCallParam{}, err
	}
	// call llm
	promptDetail := param.Prompt.GetPromptDetail()
	var tools []*entity.Tool
	if promptDetail != nil {
		if promptDetail.ToolCallConfig != nil && promptDetail.ToolCallConfig.ToolChoice != entity.ToolChoiceTypeNone {
			tools = promptDetail.Tools
		}
	}
	var modelConfig *entity.ModelConfig
	if promptDetail != nil {
		modelConfig = promptDetail.ModelConfig
	}
	var userID *string
	if userIDStr, ok := session.UserIDInCtx(ctx); ok {
		userID = ptr.Of(userIDStr)
	}
	return rpc.LLMCallParam{
		SpaceID:        param.Prompt.SpaceID,
		PromptID:       param.Prompt.ID,
		PromptKey:      param.Prompt.PromptKey,
		PromptVersion:  param.Prompt.GetVersion(),
		Scenario:       param.Scenario,
		UserID:         userID,
		Messages:       messages,
		Tools:          tools,
		ToolCallConfig: nil,
		ModelConfig:    modelConfig,
	}, nil
}

type debugTraceInfo struct {
	DebugID   int64
	DebugStep int32
}

func encodeDebugIDAndStep(debugID int64, debugStep int32) (string, error) {
	bytes, err := json.Marshal(&debugTraceInfo{
		DebugID:   debugID,
		DebugStep: debugStep,
	})
	if err != nil {
		return "", err
	}
	return encoding.FromBytes(bytes).Base32Encode().ToString(), nil
}

func decodeDebugIDAndStep(debugTraceKey string) (int64, int32, error) {
	bytes := encoding.FromString(debugTraceKey).Base32Decode().ToBytes()
	info := &debugTraceInfo{}
	err := json.Unmarshal(bytes, info)
	if err != nil {
		return 0, 0, err
	}
	return info.DebugID, info.DebugStep, nil
}

func shouldContinue(singleStep bool, startTime time.Time, currentStep int32, lastStepAggregatedReply *entity.Reply) bool {
	if singleStep {
		return false
	}
	if currentStep >= maxIterations {
		return false
	}
	if time.Since(startTime) > maxDuration {
		return false
	}
	if lastStepAggregatedReply == nil || lastStepAggregatedReply.Item == nil || lastStepAggregatedReply.Item.Message == nil {
		return false
	}
	return len(lastStepAggregatedReply.Item.Message.ToolCalls) > 0
}
