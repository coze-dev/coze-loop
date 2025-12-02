// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	json2 "encoding/json"
	"io"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/coze-dev/cozeloop-go/spec/tracespec"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/sonic"
	"github.com/kaptinlin/jsonrepair"
	"github.com/valyala/fasttemplate"

	"github.com/coze-dev/coze-loop/backend/infra/looptracer"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/evaluator"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/tracer"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/conf"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

const (
	TemplateStartTag = "{{"
	TemplateEndTag   = "}}"
)

var (
	evaluatorVersionServiceOnce      = sync.Once{}
	singletonEvaluatorVersionService EvaluatorSourceService
)

func NewEvaluatorSourcePromptServiceImpl(
	llmProvider rpc.ILLMProvider,
	metric metrics.EvaluatorExecMetrics,
	configer conf.IConfiger,
) EvaluatorSourceService {
	evaluatorVersionServiceOnce.Do(func() {
		singletonEvaluatorVersionService = &EvaluatorSourcePromptServiceImpl{
			llmProvider: llmProvider,
			metric:      metric,
			configer:    configer,
		}
	})
	return singletonEvaluatorVersionService
}

type EvaluatorSourcePromptServiceImpl struct {
	llmProvider rpc.ILLMProvider
	metric      metrics.EvaluatorExecMetrics
	configer    conf.IConfiger
}

func (p *EvaluatorSourcePromptServiceImpl) EvaluatorType() entity.EvaluatorType {
	return entity.EvaluatorTypePrompt
}

func (p *EvaluatorSourcePromptServiceImpl) Run(ctx context.Context, evaluator *entity.Evaluator, input *entity.EvaluatorInputData, exptSpaceID int64, disableTracing bool) (output *entity.EvaluatorOutputData, runStatus entity.EvaluatorRunStatus, traceID string) {
	var err error
	startTime := time.Now()
	var rootSpan *evaluatorSpan

	if !disableTracing {
		rootSpan, ctx = newEvaluatorSpan(ctx, evaluator.Name, "LoopEvaluation", strconv.FormatInt(exptSpaceID, 10), false)
		traceID = rootSpan.GetTraceID()
	} else {
		traceID = ""
	}

	defer func() {
		if output == nil {
			output = &entity.EvaluatorOutputData{
				EvaluatorRunError: &entity.EvaluatorRunError{},
			}
		}
		var errInfo error
		if err != nil {
			if output.EvaluatorRunError == nil {
				output.EvaluatorRunError = &entity.EvaluatorRunError{}
			}
			statusErr, ok := errorx.FromStatusError(err)
			if ok {
				output.EvaluatorRunError.Code = statusErr.Code()
				output.EvaluatorRunError.Message = statusErr.Error()
				errInfo = statusErr
			} else {
				output.EvaluatorRunError.Code = errno.RunEvaluatorFailCode
				output.EvaluatorRunError.Message = err.Error()
				errInfo = err
			}
		}
		if !disableTracing && rootSpan != nil {
			rootSpan.reportRootSpan(ctx, &ReportRootSpanRequest{
				input:            input,
				output:           output,
				runStatus:        runStatus,
				evaluatorVersion: evaluator.PromptEvaluatorVersion,
				errInfo:          errInfo,
			})
		}
	}()

	err = evaluator.ValidateBaseInfo()
	if err != nil {
		logs.CtxInfo(ctx, "[RunEvaluator] ValidateBaseInfo fail, err: %v", err)
		runStatus = entity.EvaluatorRunStatusFail
		return nil, runStatus, traceID
	}
	// 校验输入数据
	err = evaluator.ValidateInput(input)
	if err != nil {
		logs.CtxInfo(ctx, "[RunEvaluator] ValidateInput fail, err: %v", err)
		runStatus = entity.EvaluatorRunStatusFail
		return nil, runStatus, traceID
	}
	defer func() {
		var modelID string
		if evaluator.PromptEvaluatorVersion.ModelConfig.ModelID == 0 {
			modelID = ptr.From(evaluator.PromptEvaluatorVersion.ModelConfig.ProviderModelID)
		} else {
			modelID = strconv.FormatInt(evaluator.PromptEvaluatorVersion.ModelConfig.ModelID, 10)
		}

		p.metric.EmitRun(exptSpaceID, err, startTime, modelID)
	}()
	// 渲染变量
	err = renderTemplate(ctx, evaluator.PromptEvaluatorVersion, input, exptSpaceID, disableTracing)
	if err != nil {
		logs.CtxError(ctx, "[RunEvaluator] renderTemplate fail, err: %v", err)
		runStatus = entity.EvaluatorRunStatusFail
		return nil, runStatus, traceID
	}
	// 执行评估逻辑
	userIDInContext := session.UserIDInCtxOrEmpty(ctx)
	llmResp, err := p.chat(ctx, evaluator.PromptEvaluatorVersion, exptSpaceID, userIDInContext, disableTracing)
	if err != nil {
		logs.CtxError(ctx, "[RunEvaluator] chat fail, err: %v", err)
		runStatus = entity.EvaluatorRunStatusFail
		return nil, runStatus, traceID
	}
	output, err = parseOutput(ctx, evaluator.PromptEvaluatorVersion, llmResp, exptSpaceID, disableTracing)
	if err != nil {
		logs.CtxWarn(ctx, "[RunEvaluator] parseOutput fail, err: %v", err)
		runStatus = entity.EvaluatorRunStatusFail
		return nil, runStatus, traceID
	}
	return output, entity.EvaluatorRunStatusSuccess, traceID
}

func (p *EvaluatorSourcePromptServiceImpl) chat(ctx context.Context, evaluatorVersion *entity.PromptEvaluatorVersion, exptSpaceID int64, userIDInContext string, disableTracing bool) (resp *entity.ReplyItem, err error) {
	var modelSpan *evaluatorSpan
	modelCtx := ctx

	if !disableTracing {
		modelSpan, modelCtx = newEvaluatorSpan(ctx, evaluatorVersion.ModelConfig.ModelName, "model", strconv.FormatInt(exptSpaceID, 10), true)
		defer func() {
			modelSpan.reportModelSpan(modelCtx, evaluatorVersion, resp, err)
		}()
	}

	modelTraceCtx := modelCtx
	if !disableTracing {
		modelTraceCtx = looptracer.GetTracer().Inject(modelCtx)
		if err != nil {
			logs.CtxWarn(ctx, "[RunEvaluator] Inject fail, err: %v", err)
		}
	}

	llmCallParam := &entity.LLMCallParam{
		SpaceID:     exptSpaceID,
		EvaluatorID: strconv.FormatInt(evaluatorVersion.EvaluatorID, 10),
		UserID:      gptr.Of(userIDInContext),
		Scenario:    entity.ScenarioEvaluator,
		Messages:    evaluatorVersion.MessageList,
		ModelConfig: evaluatorVersion.ModelConfig,
	}
	if evaluatorVersion.ParseType == entity.ParseTypeFunctionCall {
		llmCallParam.Tools = evaluatorVersion.Tools
		llmCallParam.ToolCallConfig = &entity.ToolCallConfig{
			ToolChoice: entity.ToolChoiceTypeRequired,
		}
	}
	resp, err = p.llmProvider.Call(modelTraceCtx, llmCallParam)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

type evaluatorSpan struct {
	looptracer.Span
}

func newEvaluatorSpan(ctx context.Context, spanName, spanType, spaceID string, asyncChild bool) (*evaluatorSpan, context.Context) {
	var evalSpan looptracer.Span
	var nctx context.Context
	if asyncChild {
		nctx, evalSpan = looptracer.GetTracer().StartSpan(ctx, spanName, spanType, looptracer.WithSpanWorkspaceID(spaceID))
	} else {
		nctx, evalSpan = looptracer.GetTracer().StartSpan(ctx, spanName, spanType, looptracer.WithStartNewTrace(), looptracer.WithSpanWorkspaceID(spaceID))
	}

	return &evaluatorSpan{
		Span: evalSpan,
	}, nctx
}

type toolCallSpanContent struct {
	ToolCall *entity.ToolCall `json:"tool_call"`
}

type ReportRootSpanRequest struct {
	input            *entity.EvaluatorInputData
	output           *entity.EvaluatorOutputData
	runStatus        entity.EvaluatorRunStatus
	evaluatorVersion *entity.PromptEvaluatorVersion
	errInfo          error
}

func (e *evaluatorSpan) reportRootSpan(ctx context.Context, reportRootSpanRequest *ReportRootSpanRequest) {
	e.SetInput(ctx, tracer.Convert2TraceString(reportRootSpanRequest.input))
	if reportRootSpanRequest.output != nil {
		e.SetOutput(ctx, tracer.Convert2TraceString(reportRootSpanRequest.output.EvaluatorResult))
	}
	switch reportRootSpanRequest.runStatus {
	case entity.EvaluatorRunStatusSuccess:
		e.SetStatusCode(ctx, 0)
	case entity.EvaluatorRunStatusFail:
		e.SetStatusCode(ctx, int(entity.EvaluatorRunStatusFail))
		e.SetError(ctx, reportRootSpanRequest.errInfo)
	default:
		e.SetStatusCode(ctx, 0) // 默认为成功
	}
	tags := make(map[string]interface{}, 0)
	tags["evaluator_id"] = reportRootSpanRequest.evaluatorVersion.EvaluatorID
	tags["evaluator_version"] = reportRootSpanRequest.evaluatorVersion.Version
	e.SetCallType("Evaluator")
	userIDInContext := session.UserIDInCtxOrEmpty(ctx)
	if userIDInContext != "" {
		e.SetUserID(ctx, userIDInContext)
	}
	e.SetTags(ctx, tags)
	e.Finish(ctx)
}

func (e *evaluatorSpan) reportModelSpan(ctx context.Context, evaluatorVersion *entity.PromptEvaluatorVersion, replyItem *entity.ReplyItem, respErr error) {
	if respErr != nil {
		e.SetStatusCode(ctx, errno.InvalidOutputFromModelCode)
		e.SetError(ctx, respErr)
	}
	if evaluatorVersion.ParseType == entity.ParseTypeFunctionCall {
		if replyItem != nil && len(replyItem.ToolCalls) > 0 {
			e.SetOutput(ctx, tracer.Convert2TraceString(&toolCallSpanContent{
				ToolCall: replyItem.ToolCalls[0],
			}))
			if replyItem.TokenUsage != nil {
				e.SetInputTokens(ctx, int(replyItem.TokenUsage.InputTokens))
				e.SetOutputTokens(ctx, int(replyItem.TokenUsage.OutputTokens))
			}
		} else {
			e.SetStatusCode(ctx, errno.InvalidOutputFromModelCode)
			e.SetError(ctx, errorx.New("LLM response empty"))
		}
	} else {
		if replyItem != nil {
			e.SetOutput(ctx, replyItem.Content)
			if replyItem.TokenUsage != nil {
				e.SetInputTokens(ctx, int(replyItem.TokenUsage.InputTokens))
				e.SetOutputTokens(ctx, int(replyItem.TokenUsage.OutputTokens))
			}
		} else {
			e.SetStatusCode(ctx, errno.InvalidOutputFromModelCode)
			e.SetError(ctx, errorx.New("LLM response empty"))
		}
	}
	e.SetCallType("Evaluator")
	userIDInContext := session.UserIDInCtxOrEmpty(ctx)
	if userIDInContext != "" {
		e.SetUserID(ctx, userIDInContext)
	}
	tags := tracer.ConvertModel2Ob(evaluatorVersion.MessageList, evaluatorVersion.Tools)
	tags["model_config"] = tracer.Convert2TraceString(evaluatorVersion.ModelConfig)
	e.SetTags(ctx, tags)
	e.Finish(ctx)
}

func (e *evaluatorSpan) reportOutputParserSpan(ctx context.Context, replyItem *entity.ReplyItem, output *entity.EvaluatorOutputData, spaceID string, errInfo error) {
	if replyItem != nil && len(replyItem.ToolCalls) > 0 {
		e.SetInput(ctx, tracer.Convert2TraceString(&toolCallSpanContent{
			ToolCall: replyItem.ToolCalls[0],
		}))
	}
	if output != nil {
		e.SetOutput(ctx, tracer.Convert2TraceString(output.EvaluatorResult))
	}
	if errInfo != nil {
		e.SetStatusCode(ctx, int(entity.EvaluatorRunStatusFail))
		e.SetError(ctx, errInfo)
	} else {
		e.SetStatusCode(ctx, 0)
	}
	tags := make(map[string]interface{})
	e.SetCallType("Evaluator")
	userIDInContext := session.UserIDInCtxOrEmpty(ctx)
	if userIDInContext != "" {
		e.SetUserID(ctx, userIDInContext)
	}
	e.SetTags(ctx, tags)
	e.Finish(ctx)
}

func parseOutput(ctx context.Context, evaluatorVersion *entity.PromptEvaluatorVersion, replyItem *entity.ReplyItem, exptSpaceID int64, disableTracing bool) (output *entity.EvaluatorOutputData, err error) {
	// 输出数据全空直接返回
	var outputParserSpan *evaluatorSpan
	if !disableTracing {
		outputParserSpan, ctx = newEvaluatorSpan(ctx, "ParseOutput", "LoopEvaluation", strconv.FormatInt(exptSpaceID, 10), true)
		defer func() {
			outputParserSpan.reportOutputParserSpan(ctx, replyItem, output, strconv.FormatInt(exptSpaceID, 10), err)
		}()
	}
	output = &entity.EvaluatorOutputData{
		EvaluatorResult: &entity.EvaluatorResult{},
		EvaluatorUsage:  &entity.EvaluatorUsage{},
	}
	if replyItem == nil {
		logs.CtxWarn(ctx, "[RunEvaluator] parseOutput fail, err: resp is nil")
		return output, errorx.NewByCode(errno.LLMOutputEmptyCode, errorx.WithExtraMsg(" resp is nil"))
	}

	if evaluatorVersion.ParseType == entity.ParseTypeContent {
		err = parseContentOutput(ctx, evaluatorVersion, replyItem, output)
	} else {
		err = parseFunctionCallOutput(ctx, evaluatorVersion, replyItem, output)
	}

	if replyItem.TokenUsage != nil {
		output.EvaluatorUsage.InputTokens = replyItem.TokenUsage.InputTokens
		output.EvaluatorUsage.OutputTokens = replyItem.TokenUsage.OutputTokens
	}

	return output, err
}

type outputMsgFormat struct {
	Score  json2.Number `json:"score"`
	Reason string       `json:"reason"`
}

// 优化后的正则表达式，支持 score 和 reason 任意顺序，score 为 number 或 string 类型
var jsonRe = regexp.MustCompile(`\{(?s:[^{}]*(?:"score"\s*:\s*(?:"[\d.]+"|\d+(?:\.\d+)?)[^{}]*"reason"\s*:\s*"(?:[^"\\]|\\.)*"|"reason"\s*:\s*"(?:[^"\\]|\\.)*"[^{}]*"score"\s*:\s*(?:"[\d.]+"|\d+(?:\.\d+)?))[^{}]*)}`)

func parseContentOutput(ctx context.Context, evaluatorVersion *entity.PromptEvaluatorVersion, replyItem *entity.ReplyItem, output *entity.EvaluatorOutputData) error {
	content := gptr.Indirect(replyItem.Content)

	// 按优先级顺序执行解析策略
	strategies := []func(context.Context, string, *entity.EvaluatorOutputData) (bool, error){
		parseDirectJSON,         // 策略1：直接解析完整JSON
		parseRepairedJSON,       // 策略2：修复后解析完整JSON
		parseRegexExtractedJSON, // 策略3：正则提取JSON片段并解析
		parseScoreWithRegex,     // 策略4：正则提取score，优先尝试用正则提取reason字段作为reason，否则使用完整内容作为reason
	}

	for _, strategy := range strategies {
		success, err := strategy(ctx, content, output)
		if err != nil {
			return err
		}
		if success {
			return nil
		}
	}

	// 当所有解析策略都失败时，返回错误（Run方法的defer会处理错误并设置EvaluatorRunError）
	logs.CtxWarn(ctx, "[parseContentOutput] All parsing strategies failed, original content: %s", content)
	return errorx.NewByCode(errno.InvalidOutputFromModelCode, errorx.WithExtraMsg("All parsing strategies failed. Original content: "+content))
}

// parseDirectJSON 策略1：直接解析完整JSON内容
func parseDirectJSON(ctx context.Context, content string, output *entity.EvaluatorOutputData) (bool, error) {
	var outputMsg outputMsgFormat
	b := []byte(content)

	if err := sonic.Unmarshal(b, &outputMsg); err == nil {
		if outputMsg.Reason != "" {
			score, err := outputMsg.Score.Float64()
			if err != nil {
				return false, errorx.WrapByCode(err, errno.InvalidOutputFromModelCode)
			}
			output.EvaluatorResult.Score = &score
			output.EvaluatorResult.Reasoning = outputMsg.Reason
			return true, nil
		}
	}
	return false, nil
}

// parseRepairedJSON 策略2：使用jsonrepair修复后解析完整JSON内容
func parseRepairedJSON(ctx context.Context, content string, output *entity.EvaluatorOutputData) (bool, error) {
	var outputMsg outputMsgFormat

	repairedContent, repairErr := jsonrepair.JSONRepair(content)
	if repairErr == nil {
		if err := sonic.Unmarshal([]byte(repairedContent), &outputMsg); err == nil {
			if outputMsg.Reason != "" {
				score, err := outputMsg.Score.Float64()
				if err != nil {
					return false, errorx.WrapByCode(err, errno.InvalidOutputFromModelCode)
				}
				output.EvaluatorResult.Score = &score
				output.EvaluatorResult.Reasoning = outputMsg.Reason
				return true, nil
			}
		}
	}
	return false, nil
}

// parseRegexExtractedJSON 策略3：使用正则表达式提取JSON片段并解析
func parseRegexExtractedJSON(ctx context.Context, content string, output *entity.EvaluatorOutputData) (bool, error) {
	var outputMsg outputMsgFormat
	b := []byte(content)

	// 使用正则表达式查找JSON片段
	all := jsonRe.FindAll(b, -1)
	for _, bb := range all {
		// 首先尝试直接解析原始片段
		if err := sonic.Unmarshal(bb, &outputMsg); err == nil {
			if outputMsg.Reason != "" {
				score, err := outputMsg.Score.Float64()
				if err != nil {
					return false, errorx.WrapByCode(err, errno.InvalidOutputFromModelCode)
				}
				output.EvaluatorResult.Score = &score
				output.EvaluatorResult.Reasoning = outputMsg.Reason
				return true, nil
			}
		}

		// 如果直接解析失败，尝试修复后再解析
		repairedFragment, repairErr := jsonrepair.JSONRepair(string(bb))
		if repairErr == nil {
			if err := sonic.Unmarshal([]byte(repairedFragment), &outputMsg); err == nil {
				if outputMsg.Reason != "" {
					score, err := outputMsg.Score.Float64()
					if err != nil {
						return false, errorx.WrapByCode(err, errno.InvalidOutputFromModelCode)
					}
					output.EvaluatorResult.Score = &score
					output.EvaluatorResult.Reasoning = outputMsg.Reason
					return true, nil
				}
			}
		}
	}
	return false, nil
}

// parseScoreWithRegex 策略4：通过正则解析score字段，优先尝试用正则提取reason字段作为reason，否则使用完整内容作为reason
func parseScoreWithRegex(ctx context.Context, content string, output *entity.EvaluatorOutputData) (bool, error) {
	scoreRegex := regexp.MustCompile(`(?i)score[^0-9]*([0-9]+(?:\.[0-9]+)?)`)
	scoreMatches := scoreRegex.FindStringSubmatch(content)
	if len(scoreMatches) > 1 {
		scoreStr := scoreMatches[1]
		score, err := strconv.ParseFloat(scoreStr, 64)
		if err == nil {
			// 尝试提取reason字段，处理未转义双引号的情况
			// 方法：找到 "reason": " 后面的内容，提取到下一个字段或JSON对象结束之前
			reasonFieldRegex := regexp.MustCompile(`(?i)"reason"\s*:\s*"`)
			reasonStartMatches := reasonFieldRegex.FindStringIndex(content)
			if reasonStartMatches != nil {
				// 找到了reason字段的开始位置，reasonStartPos是reason值内容开始的位置（最后一个双引号之后）
				reasonStartPos := reasonStartMatches[1]
				reasonEndPos := -1

				// 首先检查reason值是否为空字符串（连续的两个双引号）
				if reasonStartPos < len(content) && content[reasonStartPos] == '"' {
					// reason值为空字符串，结束位置就是开始位置（不包含任何内容）
					reasonEndPos = reasonStartPos
				} else {
					// reason值不为空，需要找到结束位置
					// 查找下一个字段的开始位置（如 ", "score": 或其他字段）
					// 注意：需要查找reason之后的下一个字段
					nextFieldRegex := regexp.MustCompile(`(?i)",\s*"[^"]+"\s*:`)
					nextFieldMatches := nextFieldRegex.FindStringIndex(content[reasonStartPos:])
					if nextFieldMatches != nil {
						// 找到了下一个字段，且它在reason之后
						potentialEndPos := reasonStartPos + nextFieldMatches[0]
						// 从potentialEndPos向前查找最后一个双引号（reason值的结束双引号）
						for i := potentialEndPos - 1; i >= reasonStartPos; i-- {
							if content[i] == '"' {
								// 检查这是否是真正的结束双引号（前面不是转义符）
								if i == 0 || content[i-1] != '\\' {
									reasonEndPos = i
									break
								}
								// 如果是转义的双引号，继续向前查找
							}
						}
					} else {
						// 没找到下一个字段，尝试找到JSON对象的结束位置
						// 从reasonStartPos开始，向后查找第一个未转义的双引号
						for i := reasonStartPos; i < len(content); i++ {
							if content[i] == '"' {
								// 检查这是否是真正的结束双引号（前面不是转义符）
								if i == 0 || content[i-1] != '\\' {
									// 检查这个双引号后面是否是逗号、空格、}或其他字段
									if i+1 < len(content) {
										nextChar := content[i+1]
										if nextChar == ',' || nextChar == '}' || nextChar == ' ' || nextChar == '\n' || nextChar == '\r' {
											reasonEndPos = i
											break
										}
									} else {
										// 到达内容末尾
										reasonEndPos = i
										break
									}
								}
							}
						}
					}
				}

				if reasonEndPos >= reasonStartPos {
					// 提取reason值（从开始位置到结束位置，如果reason为空则extractedReason为空字符串）
					extractedReason := content[reasonStartPos:reasonEndPos]
					// 即使是空字符串也接受（reason可以为空）
					logs.CtxWarn(ctx, "[parseScoreWithRegex] Hit regex parsing strategy with reason extraction (handling unescaped quotes), original content: %s", content)
					output.EvaluatorResult.Score = &score
					output.EvaluatorResult.Reasoning = extractedReason
					return true, nil
				}
			}
			// 如果无法通过定位字段的方式提取reason，尝试传统方式（可能在无未转义双引号时有效）
			reasonRegex := regexp.MustCompile(`(?i)reason[^"]*"([^"]+)"`)
			reasonMatches := reasonRegex.FindStringSubmatch(content)
			if len(reasonMatches) > 1 && len(reasonMatches[1]) > 0 {
				// 成功提取到reason字段（传统方式，适用于无未转义双引号的情况）
				logs.CtxWarn(ctx, "[parseScoreWithRegex] Hit regex parsing strategy with reason extraction, original content: %s", content)
				output.EvaluatorResult.Score = &score
				output.EvaluatorResult.Reasoning = reasonMatches[1]
				return true, nil
			}
			// 如果无法提取reason字段，使用完整输出作为reason
			logs.CtxWarn(ctx, "[parseScoreWithRegex] Hit regex parsing strategy without reason extraction, original content: %s", content)
			output.EvaluatorResult.Score = &score
			output.EvaluatorResult.Reasoning = content // 使用完整输出作为reason
			return true, nil
		}
	}
	return false, nil
}

func parseFunctionCallOutput(ctx context.Context, evaluatorVersion *entity.PromptEvaluatorVersion, replyItem *entity.ReplyItem, output *entity.EvaluatorOutputData) error {
	if len(replyItem.ToolCalls) == 0 {
		logs.CtxWarn(ctx, "[RunEvaluator] parseOutput fail, err: tool call empty")
		return errorx.NewByCode(errno.LLMToolCallFailCode)
	}
	repairArgs, err := jsonrepair.JSONRepair(gptr.Indirect(replyItem.ToolCalls[0].FunctionCall.Arguments))
	if err != nil {
		logs.CtxWarn(ctx, "[RunEvaluator] parseOutput ToolCalls RepairJSON fail, origin content: %v, err: %v", gptr.Indirect(replyItem.ToolCalls[0].FunctionCall.Arguments), err)
		return errorx.NewByCode(errno.InvalidOutputFromModelCode)
	}
	// 解析输出数据
	params := evaluatorVersion.Tools[0].Function.Parameters
	var scoreFieldValue any
	scoreFieldValue, err = json.ExtractFieldValue(params, repairArgs, "score")
	if err != nil {
		logs.CtxWarn(ctx, "[RunEvaluator] parseOutput ExtractFieldValue score fail, repairArgs: %v, err: %v", repairArgs, err)
		return errorx.NewByCode(errno.InvalidOutputFromModelCode)
	}
	if score, ok := scoreFieldValue.(float64); ok {
		output.EvaluatorResult.Score = &score
	} else {
		logs.CtxWarn(ctx, "[RunEvaluator] parseOutput fail, repairArgs: %v, err: score not float64", repairArgs)
		return errorx.NewByCode(errno.InvalidOutputFromModelCode)
	}
	var reasonFieldValue any
	reasonFieldValue, err = json.ExtractFieldValue(params, repairArgs, "reason")
	if err != nil {
		logs.CtxWarn(ctx, "[RunEvaluator] parseOutput ReasonFieldValue reason fail, repairArgs: %v, err: %v", repairArgs, err)
		return errorx.NewByCode(errno.InvalidOutputFromModelCode)
	}
	if reason, ok := reasonFieldValue.(string); ok {
		output.EvaluatorResult.Reasoning = reason
	} else {
		logs.CtxWarn(ctx, "[RunEvaluator] parseOutput fail, repairArgs: %v, err: reason not string", repairArgs)
		return errorx.NewByCode(errno.InvalidOutputFromModelCode)
	}
	return nil
}

func renderTemplate(ctx context.Context, evaluatorVersion *entity.PromptEvaluatorVersion, input *entity.EvaluatorInputData, exptSpaceID int64, disableTracing bool) error {
	// 实现渲染模板的逻辑
	variables := make([]*tracespec.PromptArgument, 0)
	for k, v := range input.InputFields {
		if v == nil {
			variables = append(variables, &tracespec.PromptArgument{
				Key:    k,
				Source: "input",
			})
			continue
		}
		var value any
		var valueType tracespec.PromptArgumentValueType
		switch gptr.Indirect(v.ContentType) {
		case entity.ContentTypeText:
			value = v.Text
			valueType = tracespec.PromptArgumentValueTypeText
		case entity.ContentTypeMultipart:
			value = tracer.ContentToSpanParts(v.MultiPart)
			valueType = tracespec.PromptArgumentValueTypeMessagePart
		}
		variables = append(variables, &tracespec.PromptArgument{
			Key:       k,
			Value:     value,
			Source:    "input",
			ValueType: valueType,
		})
	}

	var renderTemplateSpan *evaluatorSpan
	if !disableTracing {
		renderTemplateSpan, ctx = newEvaluatorSpan(ctx, "RenderTemplate", "prompt", strconv.FormatInt(exptSpaceID, 10), true)
		renderTemplateSpan.SetInput(ctx, tracer.Convert2TraceString(tracer.ConvertPrompt2Ob(evaluatorVersion.MessageList, variables)))
	}
	for _, message := range evaluatorVersion.MessageList {
		if err := processMessageContent(message.Content, input.InputFields); err != nil {
			logs.CtxError(ctx, "[renderTemplate] process message content failed: %v", err)
			return err
		}
	}
	if len(evaluatorVersion.MessageList) > 0 {
		evaluatorVersion.MessageList[0].Content.Text = gptr.Of(gptr.Indirect(evaluatorVersion.MessageList[0].Content.Text) + evaluatorVersion.PromptSuffix)
	}

	if !disableTracing && renderTemplateSpan != nil {
		renderTemplateSpan.SetOutput(ctx, tracer.Convert2TraceString(tracer.ConvertPrompt2Ob(evaluatorVersion.MessageList, nil)))
		tags := make(map[string]interface{})
		renderTemplateSpan.SetTags(ctx, tags)
		renderTemplateSpan.SetCallType("Evaluator")
		userIDInContext := session.UserIDInCtxOrEmpty(ctx)
		if userIDInContext != "" {
			renderTemplateSpan.SetUserID(ctx, userIDInContext)
		}
		renderTemplateSpan.Finish(ctx)
	}
	return nil
}

func (p *EvaluatorSourcePromptServiceImpl) Debug(ctx context.Context, evaluator *entity.Evaluator, input *entity.EvaluatorInputData, exptSpaceID int64) (output *entity.EvaluatorOutputData, err error) {
	// 实现调试评估的逻辑
	output, _, _ = p.Run(ctx, evaluator, input, exptSpaceID, false)
	if output != nil && output.EvaluatorRunError != nil {
		return nil, errorx.NewByCode(output.EvaluatorRunError.Code, errorx.WithExtraMsg(output.EvaluatorRunError.Message))
	}
	return output, nil
}

func (p *EvaluatorSourcePromptServiceImpl) PreHandle(ctx context.Context, evaluator *entity.Evaluator) error {
	p.injectPromptTools(ctx, evaluator)
	p.injectParseType(ctx, evaluator)
	return nil
}

func (p *EvaluatorSourcePromptServiceImpl) injectPromptTools(ctx context.Context, evaluatorDO *entity.Evaluator) {
	// 注入默认工具
	tools := make([]*entity.Tool, 0, len(p.configer.GetEvaluatorToolConf(ctx)))

	if toolKey, ok := p.configer.GetEvaluatorToolMapping(ctx)[evaluatorDO.GetPromptTemplateKey()]; ok {
		tools = append(tools, evaluator.ConvertToolDTO2DO(p.configer.GetEvaluatorToolConf(ctx)[toolKey]))
	} else {
		tools = append(tools, evaluator.ConvertToolDTO2DO(p.configer.GetEvaluatorToolConf(ctx)[consts.DefaultEvaluatorToolKey]))
	}
	evaluatorDO.SetTools(tools)
}

func (p *EvaluatorSourcePromptServiceImpl) injectParseType(ctx context.Context, evaluatorDO *entity.Evaluator) {
	// 注入后缀
	if evaluatorDO.GetModelConfig() == nil {
		return
	}

	if suffixKey, ok := p.configer.GetEvaluatorPromptSuffixMapping(ctx)[strconv.FormatInt(evaluatorDO.GetModelConfig().ModelID, 10)]; ok {
		evaluatorDO.SetPromptSuffix(p.configer.GetEvaluatorPromptSuffix(ctx)[suffixKey])
		evaluatorDO.SetParseType(entity.ParseType(suffixKey))
	} else {
		evaluatorDO.SetPromptSuffix(p.configer.GetEvaluatorPromptSuffix(ctx)[consts.DefaultEvaluatorPromptSuffixKey])
		evaluatorDO.SetParseType(entity.ParseTypeContent)
	}
}

// processMessageContent 处理消息内容，支持Text和MultiPart类型
func processMessageContent(content *entity.Content, inputFields map[string]*entity.Content) error {
	if content == nil {
		return nil
	}

	switch gptr.Indirect(content.ContentType) {
	case entity.ContentTypeText:
		// 处理文本类型，保持现有逻辑
		content.Text = gptr.Of(fasttemplate.ExecuteFuncString(gptr.Indirect(content.Text), TemplateStartTag, TemplateEndTag, func(w io.Writer, tag string) (int, error) {
			// 输入变量里没有就不做替换直接返回
			if v, ok := inputFields[tag]; !ok || v == nil {
				return w.Write([]byte(""))
			}
			// 目前仅适用text替换
			return w.Write([]byte(gptr.Indirect(inputFields[tag].Text)))
		}))
	case entity.ContentTypeMultipart:
		// 处理多模态类型
		if err := processMultiPartContent(content, inputFields); err != nil {
			return err
		}
	}
	return nil
}

// processMultiPartContent 处理多模态内容
func processMultiPartContent(content *entity.Content, inputFields map[string]*entity.Content) error {
	if content == nil || content.MultiPart == nil {
		return nil
	}

	var newMultiPart []*entity.Content
	for _, part := range content.MultiPart {
		if part == nil {
			continue
		}

		switch gptr.Indirect(part.ContentType) {
		case entity.ContentTypeText:
			// 对文本部分执行模板替换
			part.Text = gptr.Of(fasttemplate.ExecuteFuncString(gptr.Indirect(part.Text), TemplateStartTag, TemplateEndTag, func(w io.Writer, tag string) (int, error) {
				// 输入变量里没有就不做替换直接返回
				if v, ok := inputFields[tag]; !ok || v == nil {
					return w.Write([]byte(""))
				}
				// 目前仅适用text替换
				return w.Write([]byte(gptr.Indirect(inputFields[tag].Text)))
			}))
			newMultiPart = append(newMultiPart, part)
		case entity.ContentTypeMultipartVariable:
			// 处理多模态变量，进行变量展开
			expandedParts, err := expandMultiPartVariable(part, inputFields)
			if err != nil {
				return err
			}
			newMultiPart = append(newMultiPart, expandedParts...)
		default:
			// 其他类型保持不变
			newMultiPart = append(newMultiPart, part)
		}
	}

	content.MultiPart = newMultiPart
	return nil
}

// expandMultiPartVariable 展开多模态变量
func expandMultiPartVariable(variablePart *entity.Content, inputFields map[string]*entity.Content) ([]*entity.Content, error) {
	if variablePart == nil || variablePart.Text == nil {
		return nil, nil
	}

	variableName := gptr.Indirect(variablePart.Text)
	if variableName == "" {
		return nil, nil
	}

	// 从输入字段中查找变量值
	variableValue, exists := inputFields[variableName]
	if !exists || variableValue == nil {
		// 变量不存在，返回空内容
		return nil, nil
	}
	res := make([]*entity.Content, 0)
	for _, part := range variableValue.MultiPart {
		if part == nil {
			continue
		}
		res = append(res, part)
	}
	return res, nil
}

// Validate 验证Prompt评估器（Prompt评估器暂时提供空实现）
func (p *EvaluatorSourcePromptServiceImpl) Validate(ctx context.Context, evaluator *entity.Evaluator) error {
	// Prompt评估器暂时提供空实现，返回nil表示验证通过
	return nil
}
