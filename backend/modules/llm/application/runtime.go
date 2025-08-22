// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/coze-dev/cozeloop-go/spec/tracespec"
	"github.com/pkg/errors"

	"github.com/coze-dev/coze-loop/backend/infra/limiter"
	"github.com/coze-dev/coze-loop/backend/infra/looptracer"
	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/domain/common"
	druntime "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/domain/runtime"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/runtime"
	"github.com/coze-dev/coze-loop/backend/modules/llm/application/convertor"
	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/llm/pkg/consts"
	llm_errorx "github.com/coze-dev/coze-loop/backend/modules/llm/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/modules/llm/pkg/goroutineutil"
	"github.com/coze-dev/coze-loop/backend/modules/llm/pkg/traceutil"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type runtimeApp struct {
	manageSrv   service.IManage
	runtimeSrv  service.IRuntime
	redis       redis.Cmdable
	rateLimiter limiter.IRateLimiter
}

func NewRuntimeApplication(
	manageSrv service.IManage,
	runtimeSrv service.IRuntime,
	redis redis.Cmdable,
	factory limiter.IRateLimiterFactory,
) runtime.LLMRuntimeService {
	return &runtimeApp{
		manageSrv:   manageSrv,
		runtimeSrv:  runtimeSrv,
		redis:       redis,
		rateLimiter: factory.NewRateLimiter(),
	}
}

func (r *runtimeApp) Chat(ctx context.Context, req *runtime.ChatRequest) (resp *runtime.ChatResponse, err error) {
	resp = runtime.NewChatResponse()
	if err = r.validateChatReq(ctx, req); err != nil {
		return resp, errorx.NewByCode(llm_errorx.RequestNotValidCode, errorx.WithExtraMsg(err.Error()))
	}
	// 1. 模型信息获取
	model, err := r.manageSrv.GetModelByID(ctx, req.GetModelConfig().GetModelID())
	if err != nil {
		return resp, err
	}
	// 2. model参数校验
	if err = model.Valid(); err != nil {
		return resp, errorx.NewByCode(llm_errorx.ModelInvalidCode, errorx.WithExtraMsg(err.Error()))
	}
	// 3. 限流
	if err = r.rateLimitAllow(ctx, req, model); err != nil {
		return resp, err
	}
	// 4. 格式转换
	msgs := convertor.MessagesDTO2DO(req.GetMessages())
	msgs, err = r.runtimeSrv.HandleMsgsPreCallModel(ctx, model, msgs)
	if err != nil {
		return resp, errorx.NewByCode(llm_errorx.RequestNotValidCode, errorx.WithExtraMsg(err.Error()))
	}
	options := convertor.ModelAndTools2OptionDOs(req.GetModelConfig(), req.GetTools())
	var respMsg *entity.Message
	// 5. start span
	var span looptracer.Span
	ctx, span = looptracer.GetTracer().StartSpan(ctx, model.Name, tracespec.VModelSpanType, looptracer.WithSpanWorkspaceID(strconv.FormatInt(req.GetBizParam().GetWorkspaceID(), 10)))
	// 6. 调用llm.generate or llm.stream方法, 并解析流式返回
	defer func() {
		// 上报span
		r.setAndFinishSpan(ctx, span, setSpanParam{
			stream:     false,
			inputMsgs:  msgs,
			toolInfos:  convertor.ToolsDTO2DO(req.GetTools()),
			toolChoice: convertor.ToolChoiceDTO2DO(req.GetModelConfig().ToolChoice),
			options:    options,
			model:      model,
			err:        err,
			respMsgs:   []*entity.Message{respMsg},
		})
		// 异步记录本次模型请求
		r.recordModelRequest(ctx, &recordModelRequestParam{
			bizParam: req.BizParam,
			model:    model,
			input:    msgs,
			lastMsg:  respMsg,
			err:      err,
		})
	}()
	respMsg, err = r.runtimeSrv.Generate(ctx, model, msgs, options...)
	if err != nil {
		return resp, err
	}
	msgDTO := convertor.MessageDO2DTO(respMsg)
	resp.SetMessage(msgDTO)
	return resp, nil
}

func (r *runtimeApp) ChatStream(ctx context.Context, req *runtime.ChatRequest, stream runtime.LLMRuntimeService_ChatStreamServer) (err error) {
	// 参数校验
	if err = r.validateChatReq(ctx, req); err != nil {
		return errorx.NewByCode(llm_errorx.RequestNotValidCode, errorx.WithExtraMsg(err.Error()))
	}
	// 1. 模型信息获取
	model, err := r.manageSrv.GetModelByID(ctx, req.GetModelConfig().GetModelID())
	if err != nil {
		return err
	}
	// 对model参数做校验
	if err = model.Valid(); err != nil {
		return errorx.NewByCode(llm_errorx.ModelInvalidCode, errorx.WithExtraMsg(err.Error()))
	}
	// 2. 限流
	if err = r.rateLimitAllow(ctx, req, model); err != nil {
		return err
	}
	// 3. 格式转换
	msgs := convertor.MessagesDTO2DO(req.GetMessages())
	msgs, err = r.runtimeSrv.HandleMsgsPreCallModel(ctx, model, msgs)
	if err != nil {
		return errorx.NewByCode(llm_errorx.RequestNotValidCode, errorx.WithExtraMsg(err.Error()))
	}
	options := convertor.ModelAndTools2OptionDOs(req.GetModelConfig(), req.GetTools())
	// 4. start trace
	var span looptracer.Span
	ctx, span = looptracer.GetTracer().StartSpan(ctx, model.Name, tracespec.VModelSpanType, looptracer.WithSpanWorkspaceID(strconv.FormatInt(req.GetBizParam().GetWorkspaceID(), 10)))
	// 5. 调用llm.generate or llm.stream方法, 并解析流式返回
	var parseResult entity.StreamRespParseResult
	beginTime := time.Now()
	defer func() {
		// 上报span
		r.setAndFinishSpan(ctx, span, setSpanParam{
			stream:            true,
			inputMsgs:         msgs,
			toolInfos:         convertor.ToolsDTO2DO(req.GetTools()),
			toolChoice:        convertor.ToolChoiceDTO2DO(req.GetModelConfig().ToolChoice),
			options:           options,
			reasoningDuration: parseResult.ReasoningDuration,
			model:             model,
			err:               err,
			respMsgs:          parseResult.RespMsgs,
			firstTokenLatency: parseResult.FirstTokenLatency,
		})
		// 异步记录本次模型请求
		r.recordModelRequest(ctx, &recordModelRequestParam{
			bizParam: req.BizParam,
			model:    model,
			input:    msgs,
			lastMsg:  parseResult.LastRespMsg,
			err:      err,
		})
	}()
	sr, err := r.runtimeSrv.Stream(ctx, model, msgs, options...)
	if err != nil {
		return err
	}
	if parseResult, err = r.parseChatStreamResp(ctx, sr, stream, beginTime); err != nil {
		return errorx.NewByCode(llm_errorx.ParseModelRespFailedCode, errorx.WithExtraMsg(err.Error()))
	}
	return nil
}

func (r *runtimeApp) parseChatStreamResp(ctx context.Context, streamDO entity.IStreamReader, streamDTO runtime.LLMRuntimeService_ChatStreamServer,
	beginTime time.Time,
) (parseResult entity.StreamRespParseResult, err error) {
	var hasReasoningContent bool
	for {
		msgDO, err := streamDO.Recv()
		if int64(parseResult.FirstTokenLatency) <= int64(0) {
			parseResult.FirstTokenLatency = time.Since(beginTime)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return parseResult, err
		}
		if msgDO.ReasoningContent != "" {
			hasReasoningContent = true
		}
		// 计算reasoning duration
		if hasReasoningContent && msgDO.ReasoningContent == "" && parseResult.ReasoningDuration == 0 {
			parseResult.ReasoningDuration = time.Since(beginTime)
		}
		parseResult.LastRespMsg = msgDO
		parseResult.RespMsgs = append(parseResult.RespMsgs, msgDO)
		msgDTO := convertor.MessageDO2DTO(msgDO)
		respDTO := &runtime.ChatResponse{
			Message: msgDTO,
		}
		if err := streamDTO.Send(ctx, respDTO); err != nil {
			return parseResult, err
		}
	}
	return parseResult, nil
}

func (r *runtimeApp) rateLimitAllow(ctx context.Context, req *runtime.ChatRequest, model *entity.Model) error {
	var scenario *entity.Scenario
	if req.GetBizParam() != nil && req.GetBizParam().Scenario != nil {
		scenario = convertor.ScenarioPtrDTO2DTO(req.GetBizParam().Scenario)
	} else {
		scenario = ptr.Of(entity.ScenarioDefault)
	}
	// 获得模型在此场景下的qpm tpm
	sceneCfg := model.GetScenarioConfig(scenario)
	if sceneCfg == nil || sceneCfg.Quota == nil {
		return nil
	}
	qpm := sceneCfg.Quota.Qpm
	tpm := sceneCfg.Quota.Tpm
	// qpm
	if qpm >= 0 {
		qpmKey := fmt.Sprintf("%s:%d:%s", "qpm", model.ID, *scenario)
		result, err := r.rateLimiter.AllowN(ctx, qpmKey, 1, limiter.WithLimit(&limiter.Limit{
			Rate:   int(qpm),
			Burst:  int(qpm),
			Period: time.Minute,
		}))
		if err == nil && result != nil && !result.Allowed {
			return errorx.NewByCode(llm_errorx.ModelQPMLimitCode)
		}
	}
	// tpm
	if tpm >= 0 {
		tpmKey := fmt.Sprintf("%s:%d:%s", "tpm", model.ID, *scenario)
		result, err := r.rateLimiter.AllowN(ctx, tpmKey, int(req.GetModelConfig().GetMaxTokens()), limiter.WithLimit(&limiter.Limit{
			Rate:   int(tpm),
			Burst:  int(tpm),
			Period: time.Minute,
		}))
		if err == nil && result != nil && !result.Allowed {
			return errorx.NewByCode(llm_errorx.ModelTPMLimitCode)
		}
	}
	return nil
}

type recordModelRequestParam struct {
	bizParam *druntime.BizParam
	model    *entity.Model
	input    []*entity.Message
	lastMsg  *entity.Message
	err      error
}

func (r *runtimeApp) recordModelRequest(ctx context.Context, param *recordModelRequestParam) {
	goroutineutil.GoWithDefaultRecovery(ctx, func() {
		record := &entity.ModelRequestRecord{
			SpaceID:             param.bizParam.GetWorkspaceID(),
			UserID:              param.bizParam.GetUserID(),
			UsageScene:          entity.Scenario(param.bizParam.GetScenario()),
			UsageSceneEntityID:  param.bizParam.GetScenarioEntityID(),
			Frame:               param.model.Frame,
			Protocol:            param.model.Protocol,
			ModelIdentification: param.model.ProtocolConfig.Model,
			ModelAk:             param.model.ProtocolConfig.APIKey,
			ModelID:             strconv.FormatInt(param.model.ID, 10),
			ModelName:           param.model.Name,
			InputToken:          int64(param.lastMsg.GetInputToken()),
			OutputToken:         int64(param.lastMsg.GetOutputToken()),
			Logid:               logs.GetLogID(ctx),
		}
		if param.err != nil {
			record.ErrorCode = strconv.FormatInt(int64(traceutil.GetTraceStatusCode(param.err)), 10)
			record.ErrorMsg = ptr.Of(param.err.Error())
		}
		if err := r.runtimeSrv.CreateModelRequestRecord(ctx, record); err != nil {
			logs.CtxWarn(ctx, "[recordModelRequest] failed, err:%v", err)
		}
	})
}

type setSpanParam struct {
	stream     bool
	inputMsgs  []*entity.Message
	toolInfos  []*entity.ToolInfo
	toolChoice *entity.ToolChoice
	options    []entity.Option
	model      *entity.Model
	bizParam   *druntime.BizParam

	firstTokenLatency time.Duration
	reasoningDuration time.Duration
	err               error
	respMsgs          []*entity.Message
}

func (r *runtimeApp) setAndFinishSpan(ctx context.Context, span looptracer.Span, param setSpanParam) {
	if span == nil {
		return
	}
	tags := make(map[string]any)
	if param.err != nil {
		span.SetStatusCode(ctx, int(traceutil.GetTraceStatusCode(param.err)))
		tags[tracespec.Error] = errorx.ErrorWithoutStack(param.err)
	}
	// set model request to span
	tags[tracespec.Stream] = param.stream
	tags[tracespec.Input] = json.Jsonify(entity.ToTraceModelInput(param.inputMsgs, param.toolInfos, param.toolChoice))
	tags[tracespec.CallOptions] = json.Jsonify(entity.OptionsToTrace(param.options))
	tags[consts.SpanTagModelID] = param.model.ID
	tags[tracespec.ModelIdentification] = param.model.GetModel()
	tags[tracespec.ModelName] = param.model.Name
	if param.bizParam.GetScenario() == common.ScenarioPromptDebug {
		tags[tracespec.PromptKey] = param.bizParam.GetScenarioEntityID()
		tags[tracespec.PromptVersion] = param.bizParam.GetScenarioEntityVersion()
	}
	// set model response to span
	if param.reasoningDuration > 0 {
		tags[tracespec.ReasoningDuration] = param.reasoningDuration.Milliseconds()
	}
	if len(param.respMsgs) > 0 {
		// response msg
		tags[tracespec.Output] = json.Jsonify(entity.StreamMsgsToTraceModelChoices(param.respMsgs))
		// token usage
		lastMsg := param.respMsgs[len(param.respMsgs)-1]
		if lastMsg != nil && lastMsg.ResponseMeta != nil && lastMsg.ResponseMeta.Usage != nil {
			tags[tracespec.InputTokens] = lastMsg.ResponseMeta.Usage.PromptTokens
			tags[tracespec.OutputTokens] = lastMsg.ResponseMeta.Usage.CompletionTokens
			tags[tracespec.Tokens] = lastMsg.ResponseMeta.Usage.TotalTokens
		}
	}
	if param.stream {
		tags[tracespec.LatencyFirstResp] = param.firstTokenLatency.Microseconds()
	}
	span.SetTags(ctx, tags)
	span.Finish(ctx)
}

func (r *runtimeApp) validateChatReq(ctx context.Context, req *runtime.ChatRequest) (err error) {
	if req.GetModelConfig() == nil {
		return errors.Errorf("model config is required")
	}
	if len(req.GetMessages()) == 0 {
		return errors.Errorf("messages is required")
	}
	if req.GetBizParam() == nil {
		return errors.Errorf("bizParam is required")
	}
	if !req.GetBizParam().IsSetScenario() {
		return errors.Errorf("bizParam.scenario is required")
	}
	if !req.GetBizParam().IsSetScenarioEntityID() {
		return errors.Errorf("bizParam.scenario_entity_id is required")
	}
	// if !req.GetBizParam().IsSetUserID() {
	// 	return errors.Errorf("bizParam.user_id is required")
	// }
	if !req.GetBizParam().IsSetWorkspaceID() {
		return errors.Errorf("bizParam.workspace_id is required")
	}
	return nil
}
