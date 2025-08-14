// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/status"
	"github.com/coze-dev/cozeloop-go"
	loopentity "github.com/coze-dev/cozeloop-go/entity"
	"github.com/coze-dev/cozeloop-go/spec/tracespec"

	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	"github.com/coze-dev/coze-loop/backend/infra/looptracer"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/debug"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/domain/prompt"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/application/convertor"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/trace"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/consts"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"github.com/coze-dev/coze-loop/backend/pkg/traceutil"
)

func NewPromptDebugApplication(
	debugLogRepo repo.IDebugLogRepo,
	debugContextRepo repo.IDebugContextRepo,
	promptService service.IPromptService,
	benefitService benefit.IBenefitService,
	auth rpc.IAuthProvider,
	file rpc.IFileProvider,
) debug.PromptDebugService {
	return &PromptDebugApplicationImpl{
		debugLogRepo:     debugLogRepo,
		debugContextRepo: debugContextRepo,
		promptService:    promptService,
		benefitService:   benefitService,
		auth:             auth,
		file:             file,
	}
}

type PromptDebugApplicationImpl struct {
	debugLogRepo     repo.IDebugLogRepo
	debugContextRepo repo.IDebugContextRepo
	promptService    service.IPromptService
	benefitService   benefit.IBenefitService
	auth             rpc.IAuthProvider
	file             rpc.IFileProvider
}

func (p *PromptDebugApplicationImpl) DebugStreaming(ctx context.Context, req *debug.DebugStreamingRequest, stream debug.PromptDebugService_DebugStreamingServer) (err error) {
	defer func() {
		if err != nil {
			logs.CtxError(ctx, "debug streaming failed, err=%v", err)
		}
	}()
	err = validateDebugStreamingRequest(req)
	if err != nil {
		return err
	}
	var callType string
	if req.Prompt.GetID() == 0 {
		callType = consts.SpanTagCallTypePromptPlayground
		req.Prompt.PromptKey = ptr.Of(fmt.Sprintf("playground-%s", session.UserIDInCtxOrEmpty(ctx)))
		err = p.auth.CheckSpacePermission(ctx, req.Prompt.GetWorkspaceID(), consts.ActionWorkspaceCreateLoopPrompt)
		if err != nil {
			return err
		}
	} else {
		callType = consts.SpanTagCallTypePromptDebug
		err = p.auth.MCheckPromptPermission(ctx, req.Prompt.GetWorkspaceID(), []int64{req.Prompt.GetID()}, consts.ActionLoopPromptDebug)
		if err != nil {
			return err
		}
	}
	var aggregatedReply *entity.Reply
	var span looptracer.Span
	ctx, span = looptracer.GetTracer().StartSpan(ctx, consts.SpanNamePromptExecutor, consts.SpanTypePromptExecutor, cozeloop.WithSpanWorkspaceID(strconv.FormatInt(req.Prompt.GetWorkspaceID(), 10)))
	if span != nil {
		span.SetCallType(callType)
		span.SetUserIDBaggage(ctx, session.UserIDInCtxOrEmpty(ctx))
		var version string
		if req.Prompt.PromptCommit != nil && req.Prompt.PromptCommit.CommitInfo != nil {
			version = req.Prompt.PromptCommit.CommitInfo.GetVersion()
		}
		span.SetPrompt(ctx, loopentity.Prompt{PromptKey: req.Prompt.GetPromptKey(), Version: version})
		span.SetInput(ctx, json.Jsonify(map[string]any{
			tracespec.PromptKey:           req.Prompt.GetPromptKey(),
			tracespec.PromptVersion:       version,
			consts.SpanTagPromptVariables: trace.VariableValsToSpanPromptVariables(convertor.BatchVariableValDTO2DO(req.VariableVals)),
			consts.SpanTagMessages:        trace.MessagesToSpanMessages(convertor.BatchMessageDTO2DO(req.Messages)),
		}))
		span.SetTags(ctx, map[string]any{
			tracespec.Stream: true,
		})
		defer func() {
			var debugID int64
			var replyItem *entity.ReplyItem
			if aggregatedReply != nil {
				debugID = aggregatedReply.DebugID
				replyItem = aggregatedReply.Item
			}
			var inputTokens, outputTokens int64
			if replyItem != nil && replyItem.TokenUsage != nil {
				inputTokens = replyItem.TokenUsage.InputTokens
				outputTokens = replyItem.TokenUsage.OutputTokens
			}
			span.SetOutput(ctx, json.Jsonify(trace.ReplyItemToSpanOutput(replyItem)))
			span.SetInputTokens(ctx, int(inputTokens))
			span.SetOutputTokens(ctx, int(outputTokens))
			span.SetTags(ctx, map[string]any{
				consts.SpanTagDebugID: debugID,
			})
			if err != nil {
				span.SetStatusCode(ctx, int(traceutil.GetTraceStatusCode(err)))
				span.SetError(ctx, errors.New(errorx.ErrorWithoutStack(err)))
				err = wrapErrorWithDebugID(err, debugID)
			}
			span.Finish(ctx)
		}()
	}
	aggregatedReply, err = p.doDebugStreaming(ctx, req, stream)
	return err
}

func validateDebugStreamingRequest(req *debug.DebugStreamingRequest) (err error) {
	defer func() {
		if err != nil {
			err = errorx.WrapByCode(err, prompterr.CommonInvalidParamCode)
		}
	}()
	// 基本参数校验(流式接口没有中间件，需要手动校验)
	err = req.IsValid()
	if err != nil {
		return err
	}
	var messages []*prompt.Message
	if req.Prompt.WorkspaceID == nil {
		return errorx.New("Prompt.WorkspaceID is nil")
	}
	if req.Prompt.PromptDraft == nil && req.Prompt.PromptCommit == nil {
		return errorx.New("Prompt.Draft and Prompt.Commit can not be nil at the same time")
	}
	var promptDetail *prompt.PromptDetail
	if req.Prompt.PromptDraft != nil {
		promptDetail = req.Prompt.PromptDraft.Detail
	} else {
		promptDetail = req.Prompt.PromptCommit.Detail
	}
	if promptDetail == nil {
		return errorx.New("PromptDetail is nil")
	}
	if promptDetail.PromptTemplate == nil {
		return errorx.New("PromptDetail.PromptTemplate is nil")
	}
	if promptDetail.PromptTemplate.TemplateType == nil {
		return errorx.New("PromptDetail.PromptTemplate.TemplateType is nil")
	}
	messages = append(messages, promptDetail.PromptTemplate.Messages...)
	if promptDetail.ModelConfig == nil {
		return errorx.New("PromptDetail.ModelConfig is nil")
	}
	messages = append(messages, req.Messages...)
	for _, message := range messages {
		if message == nil {
			return errorx.New("at least one Message is nil")
		}
		if message.Role == nil {
			return errorx.New("at least one Message.Role is nil")
		}
	}
	return nil
}

func wrapErrorWithDebugID(err error, debugID int64) error {
	if err == nil {
		return nil
	}
	if bizErr, ok := errorx.FromStatusError(err); ok {
		extra := make(map[string]string)
		if bizErr.Extra() != nil {
			extra = bizErr.Extra()
		}
		extra["debug_id"] = fmt.Sprint(debugID)
		bizErr.WithExtra(extra)
		return bizErr
	}
	return errorx.WrapByCode(err, prompterr.CommonInternalErrorCode, errorx.WithExtra(map[string]string{"debug_id": fmt.Sprint(debugID)}))
}

func (p *PromptDebugApplicationImpl) doDebugStreaming(ctx context.Context, req *debug.DebugStreamingRequest, stream debug.PromptDebugService_DebugStreamingServer) (aggregatedReply *entity.Reply, err error) {
	// 为了span的正确串联，该方法中ctx不能再从stream中获取
	startTime := time.Now()
	userID, ok := session.UserIDInCtx(ctx)
	if !ok {
		return nil, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("user id not found"))
	}
	result, err := p.benefitService.CheckPromptBenefit(ctx, &benefit.CheckPromptBenefitParams{
		ConnectorUID: userID,
		SpaceID:      req.Prompt.GetWorkspaceID(),
		PromptID:     req.Prompt.GetID(),
	})
	if err != nil {
		return nil, err
	}
	if result != nil && result.DenyReason != nil {
		// todo：错误码替换
		return nil, result.DenyReason.ToErr()
	}
	// construct prompt do
	prompt := convertor.PromptDTO2DO(req.Prompt)
	// prompt hub span report
	p.reportDebugPromptHubSpan(ctx, prompt)
	// execute
	resultStream := make(chan *entity.Reply)
	errChan := make(chan error)
	go func() {
		var executeErr error
		defer func() {
			e := recover()
			if e != nil {
				executeErr = errorx.New("panic occurred, reason=%v", e)
			}
			// 确保errChan和resultStream被关闭
			close(resultStream)
			if executeErr != nil {
				errChan <- executeErr
			}
			close(errChan)
		}()
		defer func() {
			// 写入调试记录
			logErr := p.saveDebugLog(context.WithoutCancel(ctx), saveDebugLogParam{
				prompt:          prompt,
				startTime:       startTime,
				result:          aggregatedReply,
				err:             executeErr,
				singleStepDebug: req.GetSingleStepDebug(),
			})
			if logErr != nil {
				logs.CtxError(ctx, "save debug log failed, err=%v", logErr)
			}
		}()
		messages := convertor.BatchMessageDTO2DO(req.Messages)
		mockVariables := convertor.BatchVariableValDTO2DO(req.VariableVals)
		mockTools := convertor.MockToolsDTO2DO(req.MockTools)
		// complete multi modal file uri to url
		executeErr = p.promptService.MCompleteMultiModalFileURL(ctx, messages)
		if executeErr != nil {
			return
		}
		aggregatedReply, executeErr = p.promptService.ExecuteStreaming(ctx, service.ExecuteStreamingParam{
			ExecuteParam: service.ExecuteParam{
				Prompt:        prompt,
				Messages:      messages,
				VariableVals:  mockVariables,
				MockTools:     mockTools,
				SingleStep:    req.GetSingleStepDebug(),
				DebugTraceKey: req.GetDebugTraceKey(),
				Scenario:      entity.ScenarioPromptDebug,
			},
			ResultStream: resultStream,
		})
		if executeErr != nil {
			return
		}
	}()
	// send result
	for reply := range resultStream {
		if reply == nil || reply.Item == nil {
			continue
		}
		chunk := &debug.DebugStreamingResponse{
			Delta:         convertor.MessageDO2DTO(reply.Item.Message),
			FinishReason:  ptr.Of(reply.Item.FinishReason),
			Usage:         convertor.TokenUsageDO2DTO(reply.Item.TokenUsage),
			DebugID:       ptr.Of(reply.DebugID),
			DebugTraceKey: ptr.Of(reply.DebugTraceKey),
		}
		err = stream.Send(ctx, chunk)
		if err != nil {
			if st, ok := status.FromError(err); ok && st.Code() == codes.Canceled {
				err = nil
				logs.CtxWarn(ctx, "debug streaming canceled")
			} else {
				logs.CtxError(ctx, "send chunk failed, err=%v", err)
			}
			return nil, err
		}
	}
	select { //nolint:staticcheck
	case err, ok = <-errChan:
		if !ok {
			logs.CtxInfo(ctx, "debug streaming finished")
		} else {
			if st, ok := status.FromError(err); ok && st.Code() == codes.Canceled {
				err = nil
				logs.CtxWarn(ctx, "debug streaming canceled")
			} else {
				logs.CtxError(ctx, "debug streaming failed, err=%v", err)
			}
		}
		return aggregatedReply, err
	}
}

func (p *PromptDebugApplicationImpl) reportDebugPromptHubSpan(ctx context.Context, prompt *entity.Prompt) {
	if prompt == nil {
		return
	}
	// 上报prompt hub span
	var span looptracer.Span
	ctx, span = looptracer.GetTracer().StartSpan(ctx, consts.SpanNamePromptHub, tracespec.VPromptHubSpanType, cozeloop.WithSpanWorkspaceID(strconv.FormatInt(prompt.SpaceID, 10)))
	if span != nil {
		span.SetPrompt(ctx, loopentity.Prompt{PromptKey: prompt.PromptKey, Version: prompt.GetVersion()})
		span.SetInput(ctx, json.Jsonify(map[string]any{
			tracespec.PromptKey:     prompt.PromptKey,
			tracespec.PromptVersion: prompt.GetVersion(),
		}))
		span.SetOutput(ctx, json.Jsonify(trace.PromptToSpanPrompt(prompt)))
		span.Finish(ctx)
	}
}

type saveDebugLogParam struct {
	prompt    *entity.Prompt
	startTime time.Time
	result    *entity.Reply
	err       error

	singleStepDebug bool
}

func (p *PromptDebugApplicationImpl) saveDebugLog(ctx context.Context, param saveDebugLogParam) error {
	var errCode int32
	if param.err != nil {
		errCode = prompterr.CommonInternalErrorCode
		bizErr, ok := errorx.FromStatusError(param.err)
		if ok {
			errCode = bizErr.Code()
		}
	}
	var inputTokens, outputTokens int64
	var debugID int64
	var debugStep int32
	if param.result != nil {
		if param.result.Item != nil && param.result.Item.TokenUsage != nil {
			inputTokens = param.result.Item.TokenUsage.InputTokens
			outputTokens = param.result.Item.TokenUsage.OutputTokens
		}
		debugID = param.result.DebugID
		debugStep = param.result.DebugStep
	}
	// 非单步调试，debug step记录为1，方便查询调试历史列表
	if !param.singleStepDebug {
		debugStep = 1
	}
	endTime := time.Now()
	costMS := endTime.Sub(param.startTime).Milliseconds()
	debugLog := &entity.DebugLog{
		PromptID:     param.prompt.ID,
		SpaceID:      param.prompt.SpaceID,
		PromptKey:    param.prompt.PromptKey,
		Version:      param.prompt.GetVersion(),
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		StartedAt:    param.startTime,
		EndedAt:      endTime,
		CostMS:       costMS,
		StatusCode:   errCode,
		DebuggedBy:   session.UserIDInCtxOrEmpty(ctx),
		DebugID:      debugID,
		DebugStep:    debugStep,
	}
	return p.debugLogRepo.SaveDebugLog(ctx, debugLog)
}

func (p *PromptDebugApplicationImpl) SaveDebugContext(ctx context.Context, req *debug.SaveDebugContextRequest) (r *debug.SaveDebugContextResponse, err error) {
	r = &debug.SaveDebugContextResponse{}
	err = p.auth.MCheckPromptPermission(ctx, req.GetWorkspaceID(), []int64{req.GetPromptID()}, consts.ActionLoopPromptDebug)
	if err != nil {
		return nil, err
	}
	userID, ok := session.UserIDInCtx(ctx)
	if !ok {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("user id not found"))
	}
	debugContext := convertor.DebugContextDTO2DO(req.GetPromptID(), userID, req.GetDebugContext())
	err = p.debugContextRepo.SaveDebugContext(ctx, debugContext)
	return r, err
}

func (p *PromptDebugApplicationImpl) GetDebugContext(ctx context.Context, req *debug.GetDebugContextRequest) (r *debug.GetDebugContextResponse, err error) {
	r = debug.NewGetDebugContextResponse()
	err = p.auth.MCheckPromptPermission(ctx, req.GetWorkspaceID(), []int64{req.GetPromptID()}, consts.ActionLoopPromptDebug)
	if err != nil {
		return nil, err
	}
	userID, ok := session.UserIDInCtx(ctx)
	if !ok {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("user id not found"))
	}
	debugContext, err := p.debugContextRepo.GetDebugContext(ctx, req.GetPromptID(), userID)
	if err != nil {
		return r, err
	}
	err = p.mCompleteDebugContextMultiModalFileURL(ctx, debugContext)
	if err != nil {
		return r, err
	}
	r.DebugContext = convertor.DebugContextDO2DTO(debugContext)
	return r, nil
}

func (p *PromptDebugApplicationImpl) mCompleteDebugContextMultiModalFileURL(ctx context.Context, debugContext *entity.DebugContext) error {
	if debugContext == nil {
		return nil
	}
	var fileKeys []string
	var messages []*entity.DebugMessage
	if debugContext.DebugCore != nil {
		messages = append(messages, debugContext.DebugCore.MockContexts...)
	}
	if debugContext.CompareConfig != nil {
		for _, group := range debugContext.CompareConfig.Groups {
			if group == nil || group.DebugCore == nil {
				continue
			}
			messages = append(messages, group.DebugCore.MockContexts...)
		}
	}
	for _, message := range messages {
		if message == nil || len(message.Parts) == 0 {
			continue
		}
		for _, part := range message.Parts {
			if part == nil || part.ImageURL == nil {
				continue
			}
			fileKeys = append(fileKeys, part.ImageURL.URI)
		}
	}
	if len(fileKeys) == 0 {
		return nil
	}
	urlMap, err := p.file.MGetFileURL(ctx, fileKeys)
	if err != nil {
		return err
	}
	// 回填url
	for _, message := range messages {
		if message == nil || len(message.Parts) == 0 {
			continue
		}
		for _, part := range message.Parts {
			if part == nil || part.ImageURL == nil {
				continue
			}
			part.ImageURL.URL = urlMap[part.ImageURL.URI]
		}
	}
	return nil
}

func (p *PromptDebugApplicationImpl) ListDebugHistory(ctx context.Context, req *debug.ListDebugHistoryRequest) (r *debug.ListDebugHistoryResponse, err error) {
	r = debug.NewListDebugHistoryResponse()
	err = p.auth.MCheckPromptPermission(ctx, req.GetWorkspaceID(), []int64{req.GetPromptID()}, consts.ActionLoopPromptDebug)
	if err != nil {
		return nil, err
	}
	userID, ok := session.UserIDInCtx(ctx)
	if !ok {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("user id not found"))
	}
	var pageToken *int64
	if req.PageToken != nil {
		pageTokenNum, err := strconv.ParseInt(req.GetPageToken(), 10, 64)
		if err != nil {
			return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("invalid page token"))
		}
		pageToken = ptr.Of(pageTokenNum)
	}

	result, err := p.debugLogRepo.ListDebugHistory(ctx, repo.ListDebugHistoryParam{
		PromptID:  req.GetPromptID(),
		UserID:    userID,
		DaysLimit: req.GetDaysLimit(),
		PageSize:  req.GetPageSize(),
		PageToken: pageToken,
	})
	if err != nil {
		return r, err
	}
	r.DebugHistory = convertor.BatchDebugLogDO2DTO(result.DebugHistory)
	r.HasMore = ptr.Of(result.HasMore)
	r.NextPageToken = ptr.Of(strconv.FormatInt(result.NextPageToken, 10))
	return r, nil
}
