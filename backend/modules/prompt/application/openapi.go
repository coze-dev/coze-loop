// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/asaskevich/govalidator"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/codes"
	"github.com/cloudwego/kitex/pkg/remote/trans/nphttp2/status"
	"github.com/coze-dev/cozeloop-go"
	loopentity "github.com/coze-dev/cozeloop-go/entity"
	"github.com/coze-dev/cozeloop-go/spec/tracespec"
	"github.com/vincent-petithory/dataurl"
	"golang.org/x/exp/maps"

	"github.com/coze-dev/coze-loop/backend/infra/limiter"
	"github.com/coze-dev/coze-loop/backend/infra/looptracer"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/prompt/openapi"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/application/convertor"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/conf"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/trace"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/collector"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/consts"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/goroutine"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"github.com/coze-dev/coze-loop/backend/pkg/traceutil"
)

func NewPromptOpenAPIApplication(
	promptService service.IPromptService,
	promptManageRepo repo.IManageRepo,
	config conf.IConfigProvider,
	auth rpc.IAuthProvider,
	factory limiter.IRateLimiterFactory,
	collector collector.ICollectorProvider,
) (openapi.PromptOpenAPIService, error) {
	return &PromptOpenAPIApplicationImpl{
		promptService:    promptService,
		promptManageRepo: promptManageRepo,
		config:           config,
		auth:             auth,
		rateLimiter:      factory.NewRateLimiter(),
		collector:        collector,
	}, nil
}

type PromptOpenAPIApplicationImpl struct {
	promptService    service.IPromptService
	promptManageRepo repo.IManageRepo
	config           conf.IConfigProvider
	auth             rpc.IAuthProvider
	rateLimiter      limiter.IRateLimiter
	collector        collector.ICollectorProvider
}

func (p *PromptOpenAPIApplicationImpl) BatchGetPromptByPromptKey(ctx context.Context, req *openapi.BatchGetPromptByPromptKeyRequest) (r *openapi.BatchGetPromptByPromptKeyResponse, err error) {
	r = openapi.NewBatchGetPromptByPromptKeyResponse()
	if req.GetWorkspaceID() == 0 {
		return r, errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtra(map[string]string{"invalid_param": "workspace_id参数为空"}))
	}
	defer func() {
		if err != nil {
			logs.CtxError(ctx, "openapi get prompts failed, err=%v", err)
		}
	}()

	// 限流检查
	if !p.promptHubAllowBySpace(ctx, req.GetWorkspaceID()) {
		return r, errorx.NewByCode(prompterr.PromptHubQPSLimitCode, errorx.WithExtraMsg("qps limit exceeded"))
	}

	// 查询prompt id并鉴权
	var promptKeys []string
	for _, q := range req.Queries {
		if q == nil {
			continue
		}
		promptKeys = append(promptKeys, q.GetPromptKey())
	}
	promptKeyIDMap, err := p.promptService.MGetPromptIDs(ctx, req.GetWorkspaceID(), promptKeys)
	if err != nil {
		return r, err
	}
	// 执行权限检查
	if err = p.auth.MCheckPromptPermissionForOpenAPI(ctx, req.GetWorkspaceID(), maps.Values(promptKeyIDMap), consts.ActionLoopPromptRead); err != nil {
		return nil, err
	}

	// 获取提示详细信息
	return p.fetchPromptResults(ctx, req, promptKeyIDMap)
}

// fetchPromptResults 构建返回结果
func (p *PromptOpenAPIApplicationImpl) fetchPromptResults(ctx context.Context, req *openapi.BatchGetPromptByPromptKeyRequest, promptKeyIDMap map[string]int64) (*openapi.BatchGetPromptByPromptKeyResponse, error) {
	// 准备查询参数
	var mgetParams []repo.GetPromptParam

	// 构建统一的查询参数
	var queryParams []service.PromptQueryParam
	for _, q := range req.Queries {
		if q == nil {
			continue
		}
		promptID, exists := promptKeyIDMap[q.GetPromptKey()]
		if !exists {
			continue // 如果找不到对应的 prompt ID，跳过该查询
		}
		queryParams = append(queryParams, service.PromptQueryParam{
			PromptID:  promptID,
			PromptKey: q.GetPromptKey(),
			Version:   q.GetVersion(),
			Label:     q.GetLabel(),
		})
	}

	// 使用统一的方法解析版本信息
	promptKeyCommitVersionMap, err := p.promptService.MParseCommitVersion(ctx, req.GetWorkspaceID(), queryParams)
	if err != nil {
		return nil, err
	}
	for _, query := range req.Queries {
		if query == nil {
			continue
		}

		// 构建查询参数以获取对应的版本
		promptID, exists := promptKeyIDMap[query.GetPromptKey()]
		if !exists {
			continue // 如果找不到对应的 prompt ID，跳过该查询
		}
		queryParam := service.PromptQueryParam{
			PromptID:  promptID,
			PromptKey: query.GetPromptKey(),
			Version:   query.GetVersion(),
			Label:     query.GetLabel(),
		}
		commitVersion := promptKeyCommitVersionMap[queryParam]

		mgetParams = append(mgetParams, repo.GetPromptParam{
			PromptID:      promptKeyIDMap[query.GetPromptKey()],
			WithCommit:    true,
			CommitVersion: commitVersion,
		})
	}

	// 获取prompt详细信息
	prompts, err := p.promptManageRepo.MGetPrompt(ctx, mgetParams, repo.WithPromptCacheEnable())
	if err != nil {
		if bizErr, ok := errorx.FromStatusError(err); ok && bizErr.Code() == prompterr.PromptVersionNotExistCode {
			extra := bizErr.Extra()
			for promptKey, promptID := range promptKeyIDMap {
				if extra["prompt_id"] == strconv.FormatInt(promptID, 10) {
					extra["prompt_key"] = promptKey
					break
				}
			}
			bizErr.WithExtra(extra)
		}
		return nil, err
	}

	// 构建版本映射
	promptMap := make(map[service.PromptKeyVersionPair]*entity.Prompt)
	for _, prompt := range maps.Values(prompts) {
		promptMap[service.PromptKeyVersionPair{
			PromptKey: prompt.PromptKey,
			Version:   prompt.GetVersion(),
		}] = prompt
	}

	// 构建响应
	r := openapi.NewBatchGetPromptByPromptKeyResponse()
	r.Data = openapi.NewPromptResultData()

	for _, q := range req.Queries {
		if q == nil {
			continue
		}
		// 找到具体的版本
		promptID, exists := promptKeyIDMap[q.GetPromptKey()]
		if !exists {
			return nil, errorx.NewByCode(prompterr.ResourceNotFoundCode,
				errorx.WithExtraMsg("prompt not exist"),
				errorx.WithExtra(map[string]string{"prompt_key": q.GetPromptKey()}))
		}
		queryParam := service.PromptQueryParam{
			PromptID:  promptID,
			PromptKey: q.GetPromptKey(),
			Version:   q.GetVersion(),
			Label:     q.GetLabel(),
		}
		commitVersion := promptKeyCommitVersionMap[queryParam]
		promptDTO := convertor.OpenAPIPromptDO2DTO(promptMap[service.PromptKeyVersionPair{PromptKey: q.GetPromptKey(), Version: commitVersion}])
		if promptDTO == nil {
			return nil, errorx.NewByCode(prompterr.PromptVersionNotExistCode,
				errorx.WithExtraMsg("prompt version not exist"),
				errorx.WithExtra(map[string]string{"prompt_key": q.GetPromptKey(), "version": q.GetVersion()}))
		}

		r.Data.Items = append(r.Data.Items, &openapi.PromptResult_{
			Query:  q,
			Prompt: promptDTO,
		})
	}

	if len(promptMap) > 0 {
		p.collector.CollectPromptHubEvent(ctx, req.GetWorkspaceID(), maps.Values(promptMap))
	}

	return r, nil
}

func (p *PromptOpenAPIApplicationImpl) promptHubAllowBySpace(ctx context.Context, workspaceID int64) bool {
	maxQPS, err := p.config.GetPromptHubMaxQPSBySpace(ctx, workspaceID)
	if err != nil {
		logs.CtxError(ctx, "get prompt hub max qps failed, err=%v, space_id=%d", err, workspaceID)
		return true
	}
	result, err := p.rateLimiter.AllowN(ctx, fmt.Sprintf("prompt_hub:qps:space_id:%d", workspaceID), 1,
		limiter.WithLimit(&limiter.Limit{
			Rate:   maxQPS,
			Burst:  maxQPS,
			Period: time.Second,
		}))
	if err != nil {
		logs.CtxError(ctx, "allow rate limit failed, err=%v", err)
		return true
	}
	if result == nil || result.Allowed {
		return true
	}
	return false
}

func (p *PromptOpenAPIApplicationImpl) Execute(ctx context.Context, req *openapi.ExecuteRequest) (r *openapi.ExecuteResponse, err error) {
	var promptDO *entity.Prompt
	var reply *entity.Reply
	startTime := time.Now()
	defer func() {
		var errCode int32
		if err != nil {
			logs.CtxError(ctx, "openapi execute prompt failed, err=%v", err)
			errCode = prompterr.CommonInternalErrorCode
			bizErr, ok := errorx.FromStatusError(err)
			if ok {
				errCode = bizErr.Code()
			}
		}
		var intputTokens, outputTokens int64
		var version string
		if promptDO != nil {
			version = promptDO.GetVersion()
		}
		if reply != nil && reply.Item != nil {
			intputTokens = reply.Item.TokenUsage.InputTokens
			outputTokens = reply.Item.TokenUsage.OutputTokens
		}
		p.collector.CollectPTaaSEvent(ctx, &collector.ExecuteLog{
			SpaceID:      req.GetWorkspaceID(),
			PromptKey:    req.GetPromptIdentifier().GetPromptKey(),
			Version:      version,
			Stream:       false,
			InputTokens:  intputTokens,
			OutputTokens: outputTokens,
			StartedAt:    startTime,
			EndedAt:      time.Now(),
			StatusCode:   errCode,
		})
	}()
	r = openapi.NewExecuteResponse()
	err = validateExecuteRequest(req)
	if err != nil {
		return r, err
	}
	var span cozeloop.Span
	ctx, span = p.startPromptExecutorSpan(ctx, ptaasStartPromptExecutorSpanParam{
		workspaceID:      req.GetWorkspaceID(),
		stream:           false,
		reqPromptKey:     req.GetPromptIdentifier().GetPromptKey(),
		reqPromptVersion: req.GetPromptIdentifier().GetVersion(),
		reqPromptLabel:   req.GetPromptIdentifier().GetLabel(),
		messages:         convertor.OpenAPIBatchMessageDTO2DO(req.Messages),
		variableVals:     convertor.OpenAPIBatchVariableValDTO2DO(req.VariableVals),
	})
	defer func() {
		p.finishPromptExecutorSpan(ctx, span, promptDO, reply, err)
	}()

	promptDO, reply, err = p.doExecute(ctx, req)
	if err != nil {
		return r, err
	}
	// 构建返回结果
	if reply != nil && reply.Item != nil {
		r.Data = &openapi.ExecuteData{
			Message:      convertor.OpenAPIMessageDO2DTO(reply.Item.Message),
			FinishReason: &reply.Item.FinishReason,
			Usage:        convertor.OpenAPITokenUsageDO2DTO(reply.Item.TokenUsage),
		}
	}

	// 记录使用数据
	return r, nil
}

func (p *PromptOpenAPIApplicationImpl) doExecute(ctx context.Context, req *openapi.ExecuteRequest) (promptDO *entity.Prompt, reply *entity.Reply, err error) {
	// 按prompt_key限流检查
	if !p.ptaasAllowByPromptKey(ctx, req.GetWorkspaceID(), req.GetPromptIdentifier().GetPromptKey()) {
		return promptDO, nil, errorx.NewByCode(prompterr.PTaaSQPSLimitCode, errorx.WithExtraMsg("qps limit exceeded"))
	}

	// 获取prompt并执行
	promptDO, err = p.getPromptByPromptKey(ctx, req.GetWorkspaceID(), req.GetPromptIdentifier())
	if err != nil {
		return promptDO, nil, err
	}

	// 执行权限检查
	if err = p.auth.MCheckPromptPermissionForOpenAPI(ctx, req.GetWorkspaceID(), []int64{promptDO.ID}, consts.ActionLoopPromptExecute); err != nil {
		return promptDO, nil, err
	}

	// 执行prompt
	reply, err = p.promptService.Execute(ctx, service.ExecuteParam{
		Prompt:       promptDO,
		Messages:     convertor.OpenAPIBatchMessageDTO2DO(req.Messages),
		VariableVals: convertor.OpenAPIBatchVariableValDTO2DO(req.VariableVals),
		SingleStep:   true,                 // PTaaS不支持非单步模式
		Scenario:     entity.ScenarioPTaaS, // PTaaS场景
	})
	if err != nil {
		return promptDO, nil, err
	}
	return promptDO, reply, nil
}

func (p *PromptOpenAPIApplicationImpl) ExecuteStreaming(ctx context.Context, req *openapi.ExecuteRequest, stream openapi.PromptOpenAPIService_ExecuteStreamingServer) (err error) {
	var promptDO *entity.Prompt
	var aggregatedReply *entity.Reply
	startTime := time.Now()
	defer func() {
		var errCode int32
		if err != nil {
			logs.CtxError(ctx, "openapi execute streaming prompt failed, err=%v", err)
			errCode = prompterr.CommonInternalErrorCode
			bizErr, ok := errorx.FromStatusError(err)
			if ok {
				errCode = bizErr.Code()
			}
		}
		var intputTokens, outputTokens int64
		var version string
		if promptDO != nil {
			version = promptDO.GetVersion()
		}
		if aggregatedReply != nil && aggregatedReply.Item != nil {
			intputTokens = aggregatedReply.Item.TokenUsage.InputTokens
			outputTokens = aggregatedReply.Item.TokenUsage.OutputTokens
		}
		p.collector.CollectPTaaSEvent(ctx, &collector.ExecuteLog{
			SpaceID:      req.GetWorkspaceID(),
			PromptKey:    req.GetPromptIdentifier().GetPromptKey(),
			Version:      version,
			Stream:       false,
			InputTokens:  intputTokens,
			OutputTokens: outputTokens,
			StartedAt:    startTime,
			EndedAt:      time.Now(),
			StatusCode:   errCode,
		})
	}()
	err = validateExecuteRequest(req)
	if err != nil {
		return err
	}
	var span cozeloop.Span
	ctx, span = p.startPromptExecutorSpan(ctx, ptaasStartPromptExecutorSpanParam{
		workspaceID:      req.GetWorkspaceID(),
		stream:           true,
		reqPromptKey:     req.GetPromptIdentifier().GetPromptKey(),
		reqPromptVersion: req.GetPromptIdentifier().GetVersion(),
		reqPromptLabel:   req.GetPromptIdentifier().GetLabel(),
		messages:         convertor.OpenAPIBatchMessageDTO2DO(req.Messages),
		variableVals:     convertor.OpenAPIBatchVariableValDTO2DO(req.VariableVals),
	})
	defer func() {
		p.finishPromptExecutorSpan(ctx, span, promptDO, aggregatedReply, err)
	}()
	promptDO, aggregatedReply, err = p.doExecuteStreaming(ctx, req, stream)
	// 记录使用数据
	return err
}

func (p *PromptOpenAPIApplicationImpl) doExecuteStreaming(ctx context.Context, req *openapi.ExecuteRequest, stream openapi.PromptOpenAPIService_ExecuteStreamingServer) (promptDO *entity.Prompt, aggregatedReply *entity.Reply, err error) {
	// 按prompt_key限流检查
	if !p.ptaasAllowByPromptKey(ctx, req.GetWorkspaceID(), req.GetPromptIdentifier().GetPromptKey()) {
		return promptDO, nil, errorx.NewByCode(prompterr.PTaaSQPSLimitCode, errorx.WithExtraMsg("qps limit exceeded"))
	}

	// 获取prompt并执行
	promptDO, err = p.getPromptByPromptKey(ctx, req.GetWorkspaceID(), req.GetPromptIdentifier())
	if err != nil {
		return promptDO, nil, err
	}

	// 执行权限检查
	if err = p.auth.MCheckPromptPermissionForOpenAPI(ctx, req.GetWorkspaceID(), []int64{promptDO.ID}, consts.ActionLoopPromptExecute); err != nil {
		return promptDO, nil, err
	}

	// 执行prompt流式调用
	resultStream := make(chan *entity.Reply)
	type replyResult struct {
		Reply *entity.Reply
		Err   error
	}
	replyResultChan := make(chan replyResult) // 用于接收aggregatedReply, error，避免数据竞争
	goroutine.GoSafe(ctx, func() {
		var executeErr error
		var localAggregatedReply *entity.Reply
		defer func() {
			e := recover()
			if e != nil {
				executeErr = errorx.New("panic occurred, reason=%v", e)
			}
			// 确保errChan和resultStream被关闭
			close(resultStream)
			replyResultChan <- replyResult{
				Reply: localAggregatedReply,
				Err:   executeErr,
			}
			close(replyResultChan)
		}()

		localAggregatedReply, executeErr = p.promptService.ExecuteStreaming(ctx, service.ExecuteStreamingParam{
			ExecuteParam: service.ExecuteParam{
				Prompt:       promptDO,
				Messages:     convertor.OpenAPIBatchMessageDTO2DO(req.Messages),
				VariableVals: convertor.OpenAPIBatchVariableValDTO2DO(req.VariableVals),
				SingleStep:   true,                 // PTaaS不支持非单步模式
				Scenario:     entity.ScenarioPTaaS, // PTaaS场景
			},
			ResultStream: resultStream,
		})
		if executeErr != nil {
			return
		}
	})
	// send result
	for reply := range resultStream {
		if reply == nil || reply.Item == nil {
			continue
		}
		chunk := &openapi.ExecuteStreamingResponse{
			Data: &openapi.ExecuteStreamingData{
				Message:      convertor.OpenAPIMessageDO2DTO(reply.Item.Message),
				FinishReason: ptr.Of(reply.Item.FinishReason),
				Usage:        convertor.OpenAPITokenUsageDO2DTO(reply.Item.TokenUsage),
			},
		}
		err = stream.Send(ctx, chunk)
		if err != nil {
			if st, ok := status.FromError(err); (ok && st.Code() == codes.Canceled) || errors.Is(err, context.Canceled) {
				err = nil
				logs.CtxWarn(ctx, "execute streaming canceled")
			} else if errors.Is(err, context.DeadlineExceeded) {
				err = nil
				logs.CtxWarn(ctx, "execute streaming ctx deadline exceeded")
			} else {
				logs.CtxError(ctx, "send chunk failed, err=%v", err)
			}
			return promptDO, nil, err
		}
	}
	select { //nolint:staticcheck
	case result := <-replyResultChan:
		if result.Err == nil {
			logs.CtxInfo(ctx, "execute streaming finished")
			return promptDO, result.Reply, nil
		} else {
			if st, ok := status.FromError(result.Err); (ok && st.Code() == codes.Canceled) || errors.Is(result.Err, context.Canceled) {
				logs.CtxWarn(ctx, "execute streaming canceled")
			} else if errors.Is(result.Err, context.DeadlineExceeded) {
				logs.CtxWarn(ctx, "execute streaming ctx deadline exceeded")
			} else {
				logs.CtxError(ctx, "execute streaming failed, err=%v", result.Err)
			}
			return promptDO, nil, result.Err
		}
	}
}

// ptaasAllowByPromptKey 按prompt_key维度的限流检查
func (p *PromptOpenAPIApplicationImpl) ptaasAllowByPromptKey(ctx context.Context, workspaceID int64, promptKey string) bool {
	maxQPS, err := p.config.GetPTaaSMaxQPSByPromptKey(ctx, workspaceID, promptKey)
	if err != nil {
		logs.CtxError(ctx, "get ptaas max qps failed, err=%v, prompt_key=%s", err, promptKey)
		return true
	}
	result, err := p.rateLimiter.AllowN(ctx, fmt.Sprintf("ptaas:qps:space_id:%d:prompt_key:%s", workspaceID, promptKey), 1,
		limiter.WithLimit(&limiter.Limit{
			Rate:   maxQPS,
			Burst:  maxQPS,
			Period: time.Second,
		}))
	if err != nil {
		logs.CtxError(ctx, "allow rate limit failed, err=%v", err)
		return true
	}
	if result == nil || result.Allowed {
		return true
	}
	return false
}

// getPromptByPromptKey 根据prompt_key获取prompt
func (p *PromptOpenAPIApplicationImpl) getPromptByPromptKey(ctx context.Context, spaceID int64, promptIdentifier *openapi.PromptQuery) (prompt *entity.Prompt, err error) {
	if promptIdentifier == nil {
		return nil, errors.New("prompt identifier is nil")
	}
	var span looptracer.Span
	ctx, span = looptracer.GetTracer().StartSpan(ctx, consts.SpanNamePromptHub, tracespec.VPromptHubSpanType, looptracer.WithSpanWorkspaceID(strconv.FormatInt(spaceID, 10)))
	if span != nil {
		span.SetInput(ctx, json.Jsonify(map[string]any{
			tracespec.PromptKey:     promptIdentifier.GetPromptKey(),
			tracespec.PromptVersion: promptIdentifier.GetVersion(),
			tracespec.PromptLabel:   promptIdentifier.GetLabel(),
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

	// 根据prompt_key获取prompt_id
	promptKeyIDMap, err := p.promptService.MGetPromptIDs(ctx, spaceID, []string{promptIdentifier.GetPromptKey()})
	if err != nil {
		return nil, err
	}
	promptID := promptKeyIDMap[promptIdentifier.GetPromptKey()]
	// 解析具体的提交版本
	queryParam := service.PromptQueryParam{
		PromptID:  promptID,
		PromptKey: promptIdentifier.GetPromptKey(),
		Version:   promptIdentifier.GetVersion(),
		Label:     promptIdentifier.GetLabel(),
	}
	promptKeyCommitVersionMap, err := p.promptService.MParseCommitVersion(ctx, spaceID, []service.PromptQueryParam{queryParam})
	if err != nil {
		return nil, err
	}
	commitVersion := promptKeyCommitVersionMap[queryParam]

	// 根据prompt_id、version获取prompt DO
	param := repo.GetPromptParam{
		PromptID:      promptID,
		WithCommit:    true,
		CommitVersion: commitVersion,
	}
	promptDOs, err := p.promptManageRepo.MGetPrompt(ctx, []repo.GetPromptParam{param}, repo.WithPromptCacheEnable())
	if err != nil {
		if bizErr, ok := errorx.FromStatusError(err); ok && bizErr.Code() == prompterr.PromptVersionNotExistCode {
			extra := bizErr.Extra()
			extra["prompt_key"] = promptIdentifier.GetPromptKey()
			bizErr.WithExtra(extra)
		}
		return nil, err
	}

	return promptDOs[param], nil
}

type ptaasStartPromptExecutorSpanParam struct {
	workspaceID      int64
	stream           bool
	reqPromptKey     string
	reqPromptVersion string
	reqPromptLabel   string
	messages         []*entity.Message
	variableVals     []*entity.VariableVal
}

func (p *PromptOpenAPIApplicationImpl) startPromptExecutorSpan(ctx context.Context, param ptaasStartPromptExecutorSpanParam) (context.Context, cozeloop.Span) {
	var span looptracer.Span
	ctx, span = looptracer.GetTracer().StartSpan(ctx, consts.SpanNamePromptExecutor, consts.SpanTypePromptExecutor,
		looptracer.WithSpanWorkspaceID(strconv.FormatInt(param.workspaceID, 10)))
	if span != nil {
		span.SetCallType(consts.SpanTagCallTypePTaaS)
		intput := map[string]any{
			tracespec.PromptKey:           param.reqPromptKey,
			tracespec.PromptVersion:       param.reqPromptVersion,
			tracespec.PromptLabel:         param.reqPromptLabel,
			consts.SpanTagPromptVariables: trace.VariableValsToSpanPromptVariables(param.variableVals),
			consts.SpanTagMessages:        trace.MessagesToSpanMessages(param.messages),
		}
		span.SetInput(ctx, json.Jsonify(intput))
		span.SetTags(ctx, map[string]any{
			tracespec.Stream: param.stream,
		})
	}
	return ctx, span
}

func (p *PromptOpenAPIApplicationImpl) finishPromptExecutorSpan(ctx context.Context, span cozeloop.Span, prompt *entity.Prompt, reply *entity.Reply, err error) {
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

func validateExecuteRequest(req *openapi.ExecuteRequest) error {
	err := req.IsValid()
	if err != nil {
		return err
	}
	if req.GetWorkspaceID() == 0 {
		return errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtra(map[string]string{"invalid_param": "workspace_id参数为空"}))
	}
	if req.GetPromptIdentifier() == nil || req.GetPromptIdentifier().GetPromptKey() == "" {
		return errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtra(map[string]string{"invalid_param": "prompt_key参数为空"}))
	}
	validateParts := func(parts []*openapi.ContentPart) error {
		for _, part := range parts {
			switch part.GetType() {
			case openapi.ContentTypeImageURL:
				if !govalidator.IsURL(part.GetImageURL()) {
					return errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtra(map[string]string{"invalid_param": fmt.Sprintf("%s不是有效的URL", part.GetImageURL())}))
				}
			case openapi.ContentTypeBase64Data:
				if _, err = dataurl.DecodeString(part.GetBase64Data()); err != nil {
					return errorx.NewByCode(prompterr.CommonInvalidParamCode, errorx.WithExtra(map[string]string{"invalid_param": "存在无效的base64数据，数据格式应该符合data:[<mediatype>][;base64],<data>"}))
				}
			}
		}
		return nil
	}
	for _, message := range req.Messages {
		err = validateParts(message.Parts)
		if err != nil {
			return err
		}
	}
	for _, val := range req.VariableVals {
		err = validateParts(val.MultiPartValues)
		if err != nil {
			return err
		}
	}
	return nil
}
