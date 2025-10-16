// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0	return errorx.NewByCode(errno.RequiredFunctionNotFoundCode, errorx.WithExtraMsg("代码中必须定义 exec_evaluation 或 execEvaluation 函数。JavaScript 函数定义格式：function execEvaluation(turn, userInput, modelOutput, modelConfig, evaluatorConfig) { ... } 或 function exec_evaluation(turn, user_input, model_output, model_config, evaluator_config) { ... }"))
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/looptracer"
	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/tracer"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// MaliciousPatternCategory 恶意模式类别
type MaliciousPatternCategory string

const (
	CategoryInfiniteLoop   MaliciousPatternCategory = "infinite_loop"   // 无限循环
	CategoryProcessControl MaliciousPatternCategory = "process_control" // 进程控制
	CategoryAsyncOperation MaliciousPatternCategory = "async_operation" // 异步操作
	CategoryResourceAccess MaliciousPatternCategory = "resource_access" // 资源访问
)

// MaliciousPattern 恶意模式定义
type MaliciousPattern struct {
	Pattern     string                   // 正则表达式
	Category    MaliciousPatternCategory // 威胁类型
	Description string                   // 模式描述
	Languages   []string                 // 适用语言
	Severity    string                   // 严重程度
	Risk        string                   // 风险说明
	Suggestion  string                   // 修复建议
}

// 恶意模式定义映射表
var maliciousPatternsMap = map[string][]MaliciousPattern{
	"python": {
		{
			Pattern:     `while\s+True\s*:`,
			Category:    CategoryInfiniteLoop,
			Description: "Python while True 无限循环",
			Languages:   []string{"python"},
			Severity:    "high",
			Risk:        "此模式可能导致程序无限运行，消耗系统资源，造成系统卡死",
			Suggestion:  "请使用有明确终止条件的循环，如 for i in range(10): 或添加 break 条件",
		},
		{
			Pattern:     `exit\s*\(`,
			Category:    CategoryProcessControl,
			Description: "Python exit() 进程退出",
			Languages:   []string{"python"},
			Severity:    "medium",
			Risk:        "此函数会强制退出程序，可能导致评估过程异常终止",
			Suggestion:  "请使用 return 语句正常返回结果，避免使用 exit() 函数",
		},
		{
			Pattern:     `quit\s*\(`,
			Category:    CategoryProcessControl,
			Description: "Python quit() 进程退出",
			Languages:   []string{"python"},
			Severity:    "medium",
			Risk:        "此函数会强制退出程序，可能导致评估过程异常终止",
			Suggestion:  "请使用 return 语句正常返回结果，避免使用 quit() 函数",
		},
	},
	"javascript": {
		{
			Pattern:     `while\s*\(\s*true\s*\)`,
			Category:    CategoryInfiniteLoop,
			Description: "JavaScript while(true) 无限循环",
			Languages:   []string{"javascript", "typescript"},
			Severity:    "high",
			Risk:        "此模式可能导致程序无限运行，消耗系统资源，造成浏览器或系统卡死",
			Suggestion:  "请使用有明确终止条件的循环，如 for(let i=0; i<10; i++) 或添加 break 条件",
		},
		{
			Pattern:     `for\s*\(\s*;\s*;\s*\)`,
			Category:    CategoryInfiniteLoop,
			Description: "JavaScript for(;;) 无限循环",
			Languages:   []string{"javascript", "typescript"},
			Severity:    "high",
			Risk:        "此模式会创建无限循环，消耗系统资源，可能导致程序无响应",
			Suggestion:  "请为循环添加明确的初始化、条件和递增语句，如 for(let i=0; i<10; i++)",
		},
		{
			Pattern:     `setInterval\s*\(`,
			Category:    CategoryAsyncOperation,
			Description: "JavaScript setInterval 定时器",
			Languages:   []string{"javascript", "typescript"},
			Severity:    "medium",
			Risk:        "定时器可能导致异步操作持续执行，影响系统性能和资源管理",
			Suggestion:  "请避免使用定时器，或确保在适当时机使用 clearInterval() 清理定时器",
		},
		{
			Pattern:     `setTimeout\s*\(`,
			Category:    CategoryAsyncOperation,
			Description: "JavaScript setTimeout 延时器",
			Languages:   []string{"javascript", "typescript"},
			Severity:    "low",
			Risk:        "延时器可能导致异步操作，影响评估结果的及时性",
			Suggestion:  "请避免使用延时器，直接执行相关逻辑或使用同步方式",
		},
		{
			Pattern:     `process\.exit`,
			Category:    CategoryProcessControl,
			Description: "Node.js process.exit 进程退出",
			Languages:   []string{"javascript", "typescript"},
			Severity:    "high",
			Risk:        "此函数会强制退出 Node.js 进程，导致评估过程异常终止",
			Suggestion:  "请使用 return 语句正常返回结果，避免使用 process.exit",
		},
	},
	"java": {
		{
			Pattern:     `while\s*\(\s*true\s*\)`,
			Category:    CategoryInfiniteLoop,
			Description: "Java while(true) 无限循环",
			Languages:   []string{"java"},
			Severity:    "high",
			Risk:        "此模式可能导致程序无限运行，消耗系统资源，造成 JVM 卡死",
			Suggestion:  "请使用有明确终止条件的循环，如 for(int i=0; i<10; i++) 或添加 break 条件",
		},
		{
			Pattern:     `System\.exit`,
			Category:    CategoryProcessControl,
			Description: "Java System.exit 系统退出",
			Languages:   []string{"java"},
			Severity:    "high",
			Risk:        "此函数会强制退出 JVM，导致整个应用程序终止",
			Suggestion:  "请使用 return 语句正常返回结果，避免使用 System.exit",
		},
	},
	"go": {
		{
			Pattern:     `for\s*\{\s*\}`,
			Category:    CategoryInfiniteLoop,
			Description: "Go for{} 无限循环",
			Languages:   []string{"go"},
			Severity:    "high",
			Risk:        "此模式会创建无限循环，消耗系统资源，可能导致程序无响应",
			Suggestion:  "请为循环添加明确的终止条件，如 for i := 0; i < 10; i++ 或添加 break 条件",
		},
		{
			Pattern:     `for\s*;\s*;\s*\{`,
			Category:    CategoryInfiniteLoop,
			Description: "Go for;;{} 无限循环",
			Languages:   []string{"go"},
			Severity:    "high",
			Risk:        "此模式会创建无限循环，消耗系统资源，可能导致程序无响应",
			Suggestion:  "请为循环添加明确的初始化、条件和递增语句，如 for i := 0; i < 10; i++",
		},
	},
}

// SecurityViolationDetails 安全违规详细信息
type SecurityViolationDetails struct {
	Category    MaliciousPatternCategory // 威胁类型
	Pattern     string                   // 匹配的模式
	Description string                   // 详细描述
	Language    string                   // 检测语言
	Risk        string                   // 风险说明
	Suggestion  string                   // 修复建议
}

// EvaluatorSourceCodeServiceImpl Code评估器服务实现
type EvaluatorSourceCodeServiceImpl struct {
	runtimeManager     component.IRuntimeManager
	codeBuilderFactory CodeBuilderFactory
	metric             metrics.EvaluatorExecMetrics
}

// NewEvaluatorSourceCodeServiceImpl 创建Code评估器服务实例
func NewEvaluatorSourceCodeServiceImpl(
	runtimeManager component.IRuntimeManager,
	codeBuilderFactory CodeBuilderFactory,
	metric metrics.EvaluatorExecMetrics,
) *EvaluatorSourceCodeServiceImpl {
	return &EvaluatorSourceCodeServiceImpl{
		runtimeManager:     runtimeManager,
		codeBuilderFactory: codeBuilderFactory,
		metric:             metric,
	}
}

// EvaluatorType 返回评估器类型
func (c *EvaluatorSourceCodeServiceImpl) EvaluatorType() entity.EvaluatorType {
	return entity.EvaluatorTypeCode
}

// Run 执行Code评估器
func (c *EvaluatorSourceCodeServiceImpl) Run(ctx context.Context, evaluator *entity.Evaluator, input *entity.EvaluatorInputData, disableTracing bool) (output *entity.EvaluatorOutputData, runStatus entity.EvaluatorRunStatus, traceID string) {
	var err error
	var code string
	startTime := time.Now()
	// 创建trace span
	rootSpan, ctx := c.newEvaluatorSpan(ctx, evaluator.Name, "LoopEvaluation", strconv.FormatInt(evaluator.SpaceID, 10), disableTracing)
	traceID = rootSpan.GetTraceID()

	defer func() {
		var errInfo error
		if err != nil {
			errInfo = err
		}
		c.handleRunDefer(ctx, rootSpan, &output, &errInfo, input, evaluator, code, runStatus)
	}()

	// 1. 验证评估器
	if err = c.validateEvaluator(evaluator, startTime); err != nil {
		output, runStatus = c.createErrorOutput(err, errno.InvalidEvaluatorTypeCode, "invalid evaluator type or code evaluator version is nil", startTime)
		return
	}

	// 2. 准备代码执行环境
	code, result, err := c.prepareAndExecuteCode(ctx, evaluator, input, startTime)
	if err != nil {
		output, runStatus = c.createErrorOutputFromError(err, startTime)
		return
	}

	// 3. 处理执行结果
	output, runStatus, err = c.processCodeExecutionResult(result, startTime)
	return
}

// handleRunDefer 处理Run方法的defer逻辑
func (c *EvaluatorSourceCodeServiceImpl) handleRunDefer(ctx context.Context, rootSpan *codeEvaluatorSpan, output **entity.EvaluatorOutputData, errInfo *error, input *entity.EvaluatorInputData, evaluator *entity.Evaluator, code string, runStatus entity.EvaluatorRunStatus) {
	if *output == nil {
		*output = &entity.EvaluatorOutputData{
			EvaluatorRunError: &entity.EvaluatorRunError{},
		}
	}

	if *errInfo != nil {
		// 处理错误信息
		if (*output).EvaluatorRunError == nil {
			(*output).EvaluatorRunError = &entity.EvaluatorRunError{}
		}
		statusErr, ok := errorx.FromStatusError(*errInfo)
		if ok {
			(*output).EvaluatorRunError.Code = statusErr.Code()
			(*output).EvaluatorRunError.Message = statusErr.Error()
		} else {
			(*output).EvaluatorRunError.Code = errno.CodeExecutionFailedCode
			(*output).EvaluatorRunError.Message = (*errInfo).Error()
		}
	}

	// 上报trace
	var finalErr error
	if errInfo != nil {
		finalErr = *errInfo
	}
	rootSpan.reportCodeRootSpan(ctx, &ReportCodeRootSpanRequest{
		input:            input,
		output:           *output,
		runStatus:        runStatus,
		evaluatorVersion: evaluator.CodeEvaluatorVersion,
		errInfo:          finalErr,
		code:             code, // 构建后的完整代码
	})
}

// validateEvaluator 验证评估器类型和版本
func (c *EvaluatorSourceCodeServiceImpl) validateEvaluator(evaluator *entity.Evaluator, startTime time.Time) error {
	if evaluator.EvaluatorType != entity.EvaluatorTypeCode || evaluator.CodeEvaluatorVersion == nil {
		return errorx.NewByCode(errno.InvalidEvaluatorTypeCode, errorx.WithExtraMsg("invalid evaluator type or code evaluator version is nil"))
	}
	return nil
}

// prepareAndExecuteCode 准备并执行代码
func (c *EvaluatorSourceCodeServiceImpl) prepareAndExecuteCode(ctx context.Context, evaluator *entity.Evaluator, input *entity.EvaluatorInputData, startTime time.Time) (string, *entity.ExecutionResult, error) {
	codeVersion := evaluator.CodeEvaluatorVersion

	// 1. 获取代码构建器
	codeBuilder, err := c.codeBuilderFactory.CreateBuilder(codeVersion.LanguageType)
	if err != nil {
		return "", nil, errorx.NewByCode(errno.CodeBuilderGetFailedCode, errorx.WithExtraMsg(fmt.Sprintf("language: %s, error: %v", codeVersion.LanguageType, err)))
	}

	// 2. 构建代码
	code, err := codeBuilder.BuildCode(input, codeVersion)
	if err != nil {
		return "", nil, errorx.NewByCode(errno.CodeBuildFailedCode, errorx.WithExtraMsg(err.Error()))
	}

	// 3. 获取Runtime
	runtime, err := c.runtimeManager.GetRuntime(codeVersion.LanguageType)
	if err != nil {
		return code, nil, errorx.NewByCode(errno.RuntimeGetFailedCode, errorx.WithExtraMsg(fmt.Sprintf("language: %s, error: %v", codeVersion.LanguageType, err)))
	}

	// 4. 执行代码
	ext := c.buildExtParams(evaluator)
	result, err := runtime.RunCode(ctx, code, string(codeVersion.LanguageType), c.getTimeoutMS(), ext)
	if err != nil {
		return code, nil, errorx.NewByCode(errno.CodeExecutionFailedCode, errorx.WithExtraMsg(err.Error()))
	}

	return code, result, nil
}

// processCodeExecutionResult 处理代码执行结果
func (c *EvaluatorSourceCodeServiceImpl) processCodeExecutionResult(result *entity.ExecutionResult, startTime time.Time) (*entity.EvaluatorOutputData, entity.EvaluatorRunStatus, error) {
	// 解析评估结果
	evaluatorResult, retValErrorMsg := c.parseEvaluationExecutionResult(result)

	// 处理stdout和stderr
	processedStdout, canIgnoreStderr := c.processStdoutAndStderr(result, evaluatorResult)

	// 检查是否有错误
	if evaluatorRunError := c.checkExecutionErrors(result, retValErrorMsg, canIgnoreStderr); evaluatorRunError != nil {
		return &entity.EvaluatorOutputData{
			EvaluatorRunError: evaluatorRunError,
			TimeConsumingMS:   time.Since(startTime).Milliseconds(),
			Stdout: func() string {
				if result.Output != nil {
					return result.Output.Stdout
				}
				return ""
			}(),
		}, entity.EvaluatorRunStatusFail, errorx.NewByCode(errno.CodeExecutionFailedCode, errorx.WithExtraMsg(evaluatorRunError.Message))
	}

	// 构造成功输出
	output := &entity.EvaluatorOutputData{
		EvaluatorResult: evaluatorResult,
		EvaluatorUsage: &entity.EvaluatorUsage{
			InputTokens:  0, // Code评估器暂不计算token
			OutputTokens: 0,
		},
		TimeConsumingMS: time.Since(startTime).Milliseconds(),
		Stdout:          c.getFinalStdout(result, processedStdout, canIgnoreStderr),
	}

	return output, entity.EvaluatorRunStatusSuccess, nil
}

// processStdoutAndStderr 处理stdout和stderr，返回处理后的stdout和是否可以忽略stderr
func (c *EvaluatorSourceCodeServiceImpl) processStdoutAndStderr(result *entity.ExecutionResult, evaluatorResult *entity.EvaluatorResult) (string, bool) {
	var processedStdout string
	var canIgnoreStderr bool

	// 检查是否成功解析出有效的 score 和 reason
	if evaluatorResult != nil && evaluatorResult.Score != nil && evaluatorResult.Reasoning != "" {
		canIgnoreStderr = true

		// 如果成功解析，将 stderr 作为警告信息拼接到 stdout 后面
		processedStdout = result.Output.Stdout
		if result.Output != nil && result.Output.Stderr != "" {
			if processedStdout != "" {
				processedStdout += "\n"
			}
			// 为 stderr 的每一行添加 [warning] 标志
			stderrLines := strings.Split(result.Output.Stderr, "\n")
			for _, line := range stderrLines {
				if strings.TrimSpace(line) != "" {
					processedStdout += "[warning] " + line + "\n"
				}
			}
			// 移除最后一个多余的换行符
			processedStdout = strings.TrimSuffix(processedStdout, "\n")
		}
	}

	return processedStdout, canIgnoreStderr
}

// checkExecutionErrors 检查执行结果中的错误信息
func (c *EvaluatorSourceCodeServiceImpl) checkExecutionErrors(result *entity.ExecutionResult, retValErrorMsg string, canIgnoreStderr bool) *entity.EvaluatorRunError {
	if result.Output != nil && !canIgnoreStderr {
		// 构造错误消息：RetVal错误信息 + stderr
		var errorMessage string

		// 优先添加 RetVal 中的错误信息
		if retValErrorMsg != "" {
			errorMessage = retValErrorMsg
		}

		// 然后添加 stderr（如果存在）
		if result.Output.Stderr != "" {
			if errorMessage != "" {
				errorMessage += "\n" + result.Output.Stderr
			} else {
				errorMessage = result.Output.Stderr
			}
		}

		// 只有在有错误信息时才创建 EvaluatorRunError
		if errorMessage != "" {
			return &entity.EvaluatorRunError{
				Code:    int32(errno.CodeExecutionFailedCode),
				Message: errorMessage,
			}
		}
	}
	return nil
}

// getFinalStdout 获取最终的stdout输出
func (c *EvaluatorSourceCodeServiceImpl) getFinalStdout(result *entity.ExecutionResult, processedStdout string, canIgnoreStderr bool) string {
	if canIgnoreStderr && processedStdout != "" {
		return processedStdout // 使用包含警告信息的处理后 stdout
	}
	if result.Output != nil {
		return result.Output.Stdout // 原始 stdout
	}
	return ""
}

// createErrorOutput 创建错误输出
func (c *EvaluatorSourceCodeServiceImpl) createErrorOutput(err error, code int32, message string, startTime time.Time) (*entity.EvaluatorOutputData, entity.EvaluatorRunStatus) {
	return &entity.EvaluatorOutputData{
		EvaluatorRunError: &entity.EvaluatorRunError{
			Code:    code,
			Message: message,
		},
		TimeConsumingMS: time.Since(startTime).Milliseconds(),
		Stdout:          "",
	}, entity.EvaluatorRunStatusFail
}

// createErrorOutputFromError 从错误创建错误输出
func (c *EvaluatorSourceCodeServiceImpl) createErrorOutputFromError(err error, startTime time.Time) (*entity.EvaluatorOutputData, entity.EvaluatorRunStatus) {
	var code int32
	var message string

	if strings.Contains(err.Error(), "failed to get code builder") {
		code = int32(errno.InvalidInputDataCode)
	} else if strings.Contains(err.Error(), "failed to build code") {
		code = int32(errno.InvalidInputDataCode)
	} else if strings.Contains(err.Error(), "failed to get runtime") {
		code = int32(errno.InvalidLanguageTypeCode)
	} else {
		code = int32(errno.CodeExecutionFailedCode)
	}
	message = err.Error()

	return &entity.EvaluatorOutputData{
		EvaluatorRunError: &entity.EvaluatorRunError{
			Code:    code,
			Message: message,
		},
		TimeConsumingMS: time.Since(startTime).Milliseconds(),
		Stdout:          "",
	}, entity.EvaluatorRunStatusFail
}

// Debug 调试Code评估器
func (c *EvaluatorSourceCodeServiceImpl) Debug(ctx context.Context, evaluator *entity.Evaluator, input *entity.EvaluatorInputData) (output *entity.EvaluatorOutputData, err error) {
	// 调试模式下直接调用Run方法
	output, runStatus, _ := c.Run(ctx, evaluator, input, true)
	if runStatus == entity.EvaluatorRunStatusFail {
		if output.EvaluatorRunError != nil {
			return output, errorx.NewByCode(errno.CodeExecutionFailedCode, errorx.WithExtraMsg(output.EvaluatorRunError.Message))
		}
		return output, errorx.NewByCode(errno.CodeExecutionFailedCode, errorx.WithExtraMsg("unknown error"))
	}
	return output, nil
}

// PreHandle 预处理Code评估器（语法检查等）
func (c *EvaluatorSourceCodeServiceImpl) PreHandle(ctx context.Context, evaluator *entity.Evaluator) error {
	if evaluator.EvaluatorType != entity.EvaluatorTypeCode || evaluator.CodeEvaluatorVersion == nil {
		return errorx.NewByCode(errno.InvalidEvaluatorTypeCode, errorx.WithExtraMsg("invalid evaluator type or code evaluator version is nil"))
	}

	return nil
}

// Validate 验证代码评估器
func (c *EvaluatorSourceCodeServiceImpl) Validate(ctx context.Context, evaluator *entity.Evaluator) error {
	// 基础验证
	if evaluator.EvaluatorType != entity.EvaluatorTypeCode || evaluator.CodeEvaluatorVersion == nil {
		return errorx.NewByCode(errno.InvalidEvaluatorConfigurationCode, errorx.WithExtraMsg("invalid evaluator type or code evaluator version is nil"))
	}

	codeVersion := evaluator.CodeEvaluatorVersion

	// 调用ValidateBaseInfo进行基础信息验证和language_type标准化
	if err := codeVersion.ValidateBaseInfo(); err != nil {
		return err
	}

	// 1. 先进行安全检查
	if err := c.validateCodeSecurity(codeVersion); err != nil {
		return err
	}

	// 2. 检查是否定义了 exec_evaluation 函数
	if err := c.validateExecEvaluationFunction(codeVersion); err != nil {
		return err
	}

	// 3. 再进行语法检查（现有逻辑）
	switch codeVersion.LanguageType {
	case entity.LanguageTypePython:
		return c.validatePythonCode(ctx, evaluator, codeVersion)
	case entity.LanguageTypeJS:
		return c.validateJavaScriptCode(ctx, evaluator, codeVersion)
	default:
		return errorx.NewByCode(errno.UnsupportedLanguageTypeCode, errorx.WithExtraMsg(fmt.Sprintf("language type: %s", codeVersion.LanguageType)))
	}
}

// decodeUnicodeEscapes 解码Unicode转义字符
func (c *EvaluatorSourceCodeServiceImpl) decodeUnicodeEscapes(s string) string {
	var result strings.Builder
	for i := 0; i < len(s); i++ {
		if i < len(s)-5 && s[i] == '\\' && s[i+1] == 'u' {
			// 解析Unicode转义序列 \uXXXX
			if hexStr := s[i+2 : i+6]; len(hexStr) == 4 {
				if code, err := strconv.ParseInt(hexStr, 16, 32); err == nil {
					result.WriteRune(rune(code))
					i += 5 // 跳过 \uXXXX
					continue
				}
			}
		}
		result.WriteByte(s[i])
	}
	return result.String()
}

// cleanNestedJSON 清理嵌套的JSON结构，提取最内层的JSON
func (c *EvaluatorSourceCodeServiceImpl) cleanNestedJSON(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return input
	}

	// 检查是否包含嵌套的JSON结构（如问题中的情况）
	// 寻找形如 {"score":0,"reason":"..."}\n{"stdout":"...", "stderr":""}的结构
	lines := strings.Split(input, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// 尝试解析每一行，找到包含score和reason的JSON
		var testResult map[string]interface{}
		if err := json.Unmarshal([]byte(line), &testResult); err == nil {
			// 检查是否包含score和reason字段（评估结果的标志）
			if _, hasScore := testResult["score"]; hasScore {
				if _, hasReason := testResult["reason"]; hasReason {
					return line
				}
			}
		}
	}

	return input
}

// parseSyntaxValidationStdoutJSON 解析语法校验stdout中的JSON内容（语法校验链路）
func (c *EvaluatorSourceCodeServiceImpl) parseSyntaxValidationStdoutJSON(stdout string) (map[string]interface{}, error) {
	// 清理stdout，移除换行符和额外的空白字符
	stdout = strings.TrimSpace(stdout)
	if stdout == "" {
		return nil, errorx.NewByCode(errno.ExecutionResultEmptyCode, errorx.WithExtraMsg("stdout is empty"))
	}

	// 尝试解析JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		return nil, errorx.NewByCode(errno.ExecutionResultParseFailedCode, errorx.WithExtraMsg(fmt.Sprintf("failed to parse stdout JSON: %v", err)))
	}

	// 解码错误信息中的Unicode转义字符
	if errorVal, ok := result["error"]; ok {
		if errorStr, ok := errorVal.(string); ok {
			result["error"] = c.decodeUnicodeEscapes(errorStr)
		}
	}

	return result, nil
}

// parseSyntaxValidationRetValJSON 解析语法校验ret_val中的JSON数据（语法校验链路）
func (c *EvaluatorSourceCodeServiceImpl) parseSyntaxValidationRetValJSON(retVal string) (map[string]interface{}, error) {
	// 清理retVal，移除换行符和额外的空白字符
	retVal = strings.TrimSpace(retVal)
	if retVal == "" {
		return nil, errorx.NewByCode(errno.ExecutionResultEmptyCode, errorx.WithExtraMsg("ret_val is empty"))
	}

	// 尝试解析JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(retVal), &result); err != nil {
		return nil, errorx.NewByCode(errno.ExecutionResultParseFailedCode, errorx.WithExtraMsg(fmt.Sprintf("failed to parse ret_val JSON: %v", err)))
	}

	// 解码错误信息中的Unicode转义字符
	if errorVal, ok := result["error"]; ok {
		if errorStr, ok := errorVal.(string); ok {
			result["error"] = c.decodeUnicodeEscapes(errorStr)
		}
	}

	return result, nil
}

// processExecutionResult 处理执行结果，仅保留decodeUnicodeEscapes的处理
func (c *EvaluatorSourceCodeServiceImpl) processExecutionResult(result *entity.ExecutionResult) (*entity.ExecutionResult, error) {
	if result == nil {
		return nil, errorx.NewByCode(errno.ExecutionResultNilCode, errorx.WithExtraMsg("execution result is nil"))
	}

	// 处理输出信息
	if result.Output != nil {
		// 解码stdout和stderr中的Unicode字符
		result.Output.Stdout = c.decodeUnicodeEscapes(result.Output.Stdout)
		result.Output.Stderr = c.decodeUnicodeEscapes(result.Output.Stderr)
	}

	return result, nil
}

// ValidationResult 验证结果结构体
type ValidationResult struct {
	Valid    bool
	ErrorMsg string
}

// parseSyntaxValidationResult 解析语法校验结果中的 valid 和 error 字段（语法校验链路）
// 这是一个通用方法，用于处理 ret_val 或 stdout 中的语法验证结果
func (c *EvaluatorSourceCodeServiceImpl) parseSyntaxValidationResult(data map[string]interface{}) *ValidationResult {
	result := &ValidationResult{
		Valid:    true, // 默认为有效
		ErrorMsg: "",
	}

	// 检查 valid 字段
	if validVal, ok := data["valid"]; ok {
		if valid, ok := validVal.(bool); ok {
			result.Valid = valid
		}
	}

	// 如果无效，获取错误信息
	if !result.Valid {
		if errorVal, ok := data["error"]; ok {
			switch errorInfo := errorVal.(type) {
			case string:
				// 兼容旧格式：简单字符串错误信息
				result.ErrorMsg = errorInfo
			case map[string]interface{}:
				// 新格式：包含详细行列号信息的对象
				if fullMsg, ok := errorInfo["full_message"].(string); ok {
					result.ErrorMsg = fullMsg
				} else if msg, ok := errorInfo["message"].(string); ok {
					result.ErrorMsg = msg
				}
			}
		}
	}

	return result
}

// processSyntaxValidationExecutionResult 处理语法校验执行结果（语法校验链路：解析 valid 和 error）
// 作为统一的语法验证入口，负责所有的 valid 字段解析和验证
func (c *EvaluatorSourceCodeServiceImpl) processSyntaxValidationExecutionResult(result *entity.ExecutionResult) (bool, string, error) {
	// 先进行基本处理
	processed, err := c.processExecutionResult(result)
	if err != nil {
		return false, "", err
	}

	// 优先解析ret_val中的JSON内容
	if processed.Output != nil && processed.Output.RetVal != "" {
		if retValData, parseErr := c.parseSyntaxValidationRetValJSON(processed.Output.RetVal); parseErr == nil {
			// 使用通用方法解析验证结果
			validationResult := c.parseSyntaxValidationResult(retValData)
			return validationResult.Valid, validationResult.ErrorMsg, nil
		} else {
			return false, processed.Output.RetVal, nil
		}
	}

	// 如果ret_val解析失败或为空，尝试解析stdout中的JSON内容作为备用
	if processed.Output != nil && processed.Output.Stdout != "" {
		if parsedOutput, parseErr := c.parseSyntaxValidationStdoutJSON(processed.Output.Stdout); parseErr == nil {
			// 使用通用方法解析验证结果
			validationResult := c.parseSyntaxValidationResult(parsedOutput)
			return validationResult.Valid, validationResult.ErrorMsg, nil
		} else {
			return false, processed.Output.Stdout, nil
		}
	}

	// 如果都解析失败，检查stderr
	if processed.Output != nil && processed.Output.Stderr != "" {
		return false, processed.Output.Stderr, nil
	}

	return true, "", nil
}

// convertPythonDictToJSON 将 Python 字典格式转换为标准 JSON 格式
func (c *EvaluatorSourceCodeServiceImpl) convertPythonDictToJSON(pythonDict string) (string, error) {
	result := make([]rune, 0, len(pythonDict))
	runes := []rune(pythonDict)
	inString := false
	stringDelimiter := '\000'

	for i := 0; i < len(runes); i++ {
		char := runes[i]

		if !inString {
			// 在字符串外部
			if char == '\'' || char == '"' {
				// 开始字符串，记录分隔符并统一使用双引号
				result = append(result, '"')
				inString = true
				stringDelimiter = char
			} else {
				result = append(result, char)
			}
		} else {
			// 在字符串内部
			if char == '\\' && i+1 < len(runes) {
				// 处理转义字符
				nextChar := runes[i+1]
				if nextChar == '\'' || nextChar == '"' || nextChar == '\\' || nextChar == 'n' || nextChar == 't' || nextChar == 'r' {
					result = append(result, '\\')
					result = append(result, nextChar)
					i++ // 跳过下一个字符
				} else {
					result = append(result, char)
				}
			} else if char == stringDelimiter {
				// 字符串结束
				result = append(result, '"')
				inString = false
				stringDelimiter = '\000'
			} else if char == '"' && stringDelimiter == '\'' {
				// 在单引号字符串内部遇到双引号，需要转义
				result = append(result, '\\')
				result = append(result, '"')
			} else if char == '\'' && stringDelimiter == '"' {
				// 在双引号字符串内部遇到单引号，直接保留
				result = append(result, '\'')
			} else {
				result = append(result, char)
			}
		}
	}

	return string(result), nil
}

// parseEvaluationRetVal 解析评估结果RetVal字段中的JSON数据（评估结果链路：提取 score、reason）
func (c *EvaluatorSourceCodeServiceImpl) parseEvaluationRetVal(retVal string) (score *float64, reason string, err error) {
	if strings.TrimSpace(retVal) == "" {
		return nil, "", nil
	}

	// 处理可能存在的嵌套JSON结构
	cleanedRetVal := c.cleanNestedJSON(retVal)

	var result map[string]interface{}

	// 首先尝试标准 JSON 解析
	if err := json.Unmarshal([]byte(cleanedRetVal), &result); err != nil {
		// 如果 JSON 解析失败，尝试 Python 字典格式
		jsonStr, convertErr := c.convertPythonDictToJSON(cleanedRetVal)
		if convertErr != nil {
			return nil, "", errorx.NewByCode(errno.ExecutionResultParseFailedCode, errorx.WithExtraMsg(fmt.Sprintf("failed to parse RetVal: %v, jsonStr: %s", err, jsonStr)))
		}

		if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
			return nil, "", errorx.NewByCode(errno.ExecutionResultParseFailedCode, errorx.WithExtraMsg(fmt.Sprintf("failed to parse converted RetVal JSON: %v, jsonStr: %s", err, jsonStr)))
		}
	}

	// 解析score字段
	if scoreVal, ok := result["score"]; ok {
		switch v := scoreVal.(type) {
		case float64:
			score = &v
		case int:
			f := float64(v)
			score = &f
		case string:
			if f, parseErr := strconv.ParseFloat(v, 64); parseErr == nil {
				score = &f
			}
		}
	}

	// 解析reason字段
	if reasonVal, ok := result["reason"]; ok {
		if reasonStr, ok := reasonVal.(string); ok {
			reason = reasonStr
		}
	}

	return score, reason, nil
}

// parseEvaluationExecutionResult 解析评估器执行结果（评估结果链路：解析 score 和 reason）
func (c *EvaluatorSourceCodeServiceImpl) parseEvaluationExecutionResult(result *entity.ExecutionResult) (*entity.EvaluatorResult, string) {
	evaluatorResult := &entity.EvaluatorResult{}

	// 直接从RetVal字段解析score和reason
	if result.Output != nil && result.Output.RetVal != "" {
		score, reason, parseErr := c.parseEvaluationRetVal(result.Output.RetVal)
		if parseErr != nil {
			logs.Error("failed to parse RetVal: %v", parseErr)
			// 解析失败时，将 RetVal 内容作为错误信息返回
			return nil, result.Output.RetVal
		}

		if score != nil {
			evaluatorResult.Score = score
		}
		if reason != "" {
			evaluatorResult.Reasoning = reason
		}
		return evaluatorResult, ""
	}
	return nil, ""
}

// validatePythonCode 验证Python代码
func (c *EvaluatorSourceCodeServiceImpl) validatePythonCode(ctx context.Context, evaluator *entity.Evaluator, codeVersion *entity.CodeEvaluatorVersion) error {
	// 基础检查
	if codeVersion.CodeContent == "" {
		return errorx.NewByCode(errno.EmptyCodeContentCode, errorx.WithExtraMsg("python code is empty"))
	}

	// 额外的Python特定安全检查
	if err := c.validatePythonSpecificSecurity(codeVersion.CodeContent); err != nil {
		return err
	}

	// 获取Runtime
	runtime, err := c.runtimeManager.GetRuntime(entity.LanguageTypePython)
	if err != nil {
		return errorx.NewByCode(errno.RuntimeGetFailedCode, errorx.WithExtraMsg(fmt.Sprintf("failed to get python runtime for validation: %v", err)))
	}

	// 构建Python语法检查代码，参考pyodide客户端的AST验证方式
	syntaxCheckCode := c.buildPythonSyntaxCheckCode(codeVersion.CodeContent)

	// 使用runtime执行语法检查，设置较短的超时时间
	ext := c.buildExtParams(evaluator)
	result, err := runtime.RunCode(ctx, syntaxCheckCode, "python", 10000, ext) // 10秒超时用于语法验证
	if err != nil {
		return errorx.NewByCode(errno.SyntaxValidationFailedCode, errorx.WithExtraMsg(fmt.Sprintf("python syntax validation failed: %v", err)))
	}

	// 处理执行结果并解析stdout中的JSON
	valid, errorMsg, err := c.processSyntaxValidationExecutionResult(result)
	if err != nil {
		return errorx.NewByCode(errno.SyntaxValidationResultParseFailedCode, errorx.WithExtraMsg(err.Error()))
	}
	// 直接使用 processSyntaxValidationExecutionResult 的验证结果
	// 该方法已经完成了所有的 valid 字段解析和验证
	if !valid {
		return errorx.NewByCode(errno.SyntaxValidationFailedCode, errorx.WithExtraMsg(fmt.Sprintf("python syntax error: %s", errorMsg)))
	}

	return nil
}

// validateJavaScriptCode 验证JavaScript代码
func (c *EvaluatorSourceCodeServiceImpl) validateJavaScriptCode(ctx context.Context, evaluator *entity.Evaluator, codeVersion *entity.CodeEvaluatorVersion) error {
	// 基础检查
	if codeVersion.CodeContent == "" {
		return errorx.NewByCode(errno.EmptyCodeContentCode, errorx.WithExtraMsg("javascript code is empty"))
	}

	// JavaScript特定安全检查
	if err := c.validateJavaScriptSpecificSecurity(codeVersion.CodeContent); err != nil {
		return err
	}

	// 获取Runtime
	runtime, err := c.runtimeManager.GetRuntime(entity.LanguageTypeJS)
	if err != nil {
		return errorx.NewByCode(errno.RuntimeGetFailedCode, errorx.WithExtraMsg(fmt.Sprintf("failed to get javascript runtime for validation: %v", err)))
	}

	// 构建JavaScript语法检查代码 (使用Builder模式)
	syntaxCheckCode := c.buildJavaScriptSyntaxCheckCode(codeVersion.CodeContent)

	// 使用runtime执行语法检查，设置较短的超时时间
	ext := c.buildExtParams(evaluator)
	result, err := runtime.RunCode(ctx, syntaxCheckCode, "js", 10000, ext) // 与Python保持一致的10秒超时
	if err != nil {
		return errorx.NewByCode(errno.SyntaxValidationFailedCode, errorx.WithExtraMsg(fmt.Sprintf("javascript syntax validation failed: %v", err)))
	}

	// 使用统一的结果处理方法 (与Python保持一致)
	valid, errorMsg, err := c.processSyntaxValidationExecutionResult(result)
	if err != nil {
		return errorx.NewByCode(errno.SyntaxValidationResultParseFailedCode, errorx.WithExtraMsg(err.Error()))
	}

	// 直接使用 processSyntaxValidationExecutionResult 的验证结果
	// 该方法已经完成了所有的 valid 字段解析和验证
	if !valid {
		return errorx.NewByCode(errno.SyntaxValidationFailedCode, errorx.WithExtraMsg(fmt.Sprintf("javascript syntax error: %s", errorMsg)))
	}

	return nil
}

// buildPythonSyntaxCheckCode 构建Python语法检查代码
func (c *EvaluatorSourceCodeServiceImpl) buildPythonSyntaxCheckCode(userCode string) string {
	// 使用CodeBuilderFactory创建PythonCodeBuilder
	codeBuilder, err := c.codeBuilderFactory.CreateBuilder(entity.LanguageTypePython)
	if err != nil {
		// 如果创建失败，回退到简单构建方式以保持向后兼容
		return c.buildSimplePythonSyntaxCheckCode(userCode)
	}

	// 使用Builder的BuildSyntaxCheckCode方法
	return codeBuilder.BuildSyntaxCheckCode(userCode)
}

// buildSimplePythonSyntaxCheckCode 构建简单的Python语法检查代码（备用方案）
func (c *EvaluatorSourceCodeServiceImpl) buildSimplePythonSyntaxCheckCode(userCode string) string {
	// 转义用户代码中的特殊字符
	escapedCode := strings.ReplaceAll(userCode, "\\", "\\\\")
	escapedCode = strings.ReplaceAll(escapedCode, `"""`, `\"\"\"`)
	escapedCode = strings.ReplaceAll(escapedCode, `"`, `\"`)

	// 构建Python AST语法检查代码，参考提供的Python ast校验代码
	syntaxCheckCode := fmt.Sprintf(`
import ast
import json

def check_syntax(code):
    """
    检查Python代码是否有语法错误
    返回 (是否有错误, 错误信息或None)
    """
    try:
        # 尝试解析代码
        ast.parse(code)
        return (False, None)  # 没有语法错误
    except SyntaxError as e:
        # 捕获语法错误并返回错误信息
        error_msg = f"语法错误: {e.msg} (行号: {e.lineno}, 列号: {e.offset})"
        return (True, error_msg)

# 用户代码
user_code = """%s"""

# 检查语法
has_error, msg = check_syntax(user_code)
if has_error:
    result = {"valid": False, "error": msg}
else:
    result = {"valid": True, "error": None}

# 输出结果
print(json.dumps(result))
`, escapedCode)

	return syntaxCheckCode
}

// buildJavaScriptSyntaxCheckCode 构建JavaScript语法检查代码 (优化版本)
func (c *EvaluatorSourceCodeServiceImpl) buildJavaScriptSyntaxCheckCode(userCode string) string {
	// 使用CodeBuilderFactory创建JavaScriptCodeBuilder
	codeBuilder, err := c.codeBuilderFactory.CreateBuilder(entity.LanguageTypeJS)
	if err != nil {
		// 如果创建失败，回退到简单构建方式以保持向后兼容
		return c.buildSimpleJavaScriptSyntaxCheckCode(userCode)
	}

	// 使用Builder的BuildSyntaxCheckCode方法
	return codeBuilder.BuildSyntaxCheckCode(userCode)
}

// buildSimpleJavaScriptSyntaxCheckCode 构建简单的JavaScript语法检查代码（备用方案）
func (c *EvaluatorSourceCodeServiceImpl) buildSimpleJavaScriptSyntaxCheckCode(userCode string) string {
	// 转义用户代码中的特殊字符
	escapedCode := strings.ReplaceAll(userCode, "\\", "\\\\")
	escapedCode = strings.ReplaceAll(escapedCode, "`", "\\`")
	escapedCode = strings.ReplaceAll(escapedCode, "$", "\\$")

	// 构建JavaScript语法检查代码，输出JSON格式结果
	syntaxCheckCode := fmt.Sprintf(`
// JavaScript语法检查
const userCode = %s;

try {
    // 使用Function构造函数进行语法检查
    new Function(userCode);
    
    // 语法正确，输出JSON结果
    const result = {"valid": true, "error": null};
    console.log(JSON.stringify(result));
} catch (error) {
    // 捕获语法错误，输出JSON结果
    const result = {"valid": false, "error": "语法错误: " + error.message};
    console.log(JSON.stringify(result));
}
`, "`"+escapedCode+"`")

	return syntaxCheckCode
}

// validateCodeSecurity 验证代码安全性
func (c *EvaluatorSourceCodeServiceImpl) validateCodeSecurity(codeVersion *entity.CodeEvaluatorVersion) error {
	if strings.TrimSpace(codeVersion.CodeContent) == "" {
		return errorx.NewByCode(errno.EmptyCodeContentCode, errorx.WithExtraMsg("代码不能为空"))
	}

	// 转换语言类型
	language := c.convertLanguageType(codeVersion.LanguageType)

	// 检查危险函数调用
	if err := c.checkDangerousFunctions(codeVersion.CodeContent, language); err != nil {
		return err
	}

	// 检查危险模块导入
	if err := c.checkDangerousImports(codeVersion.CodeContent, language); err != nil {
		return err
	}

	// 检查恶意模式
	if err := c.checkMaliciousPatterns(codeVersion.CodeContent, language); err != nil {
		return err
	}

	return nil
}

// convertLanguageType 转换语言类型
func (c *EvaluatorSourceCodeServiceImpl) convertLanguageType(langType entity.LanguageType) string {
	switch langType {
	case entity.LanguageTypePython:
		return "python"
	case entity.LanguageTypeJS:
		return "javascript"
	default:
		return string(langType)
	}
}

// validatePythonSpecificSecurity Python特定安全检查
func (c *EvaluatorSourceCodeServiceImpl) validatePythonSpecificSecurity(code string) error {
	// 检查Python特有的危险模式
	dangerousPatterns := []string{
		`__import__\s*\(\s*["']os["']`,   // 动态导入os模块
		`getattr\s*\(.*,\s*["']__.*["']`, // 访问私有属性
		`setattr\s*\(.*,\s*["']__.*["']`, // 设置私有属性
		`hasattr\s*\(.*,\s*["']__.*["']`, // 检查私有属性
	}

	for _, pattern := range dangerousPatterns {
		if matched, _ := regexp.MatchString(pattern, code); matched {
			return errorx.NewByCode(errno.DangerousImportDetectedCode, errorx.WithExtraMsg("detected dangerous Python pattern"))
		}
	}

	return nil
}

// validateJavaScriptSpecificSecurity JavaScript特定安全检查
func (c *EvaluatorSourceCodeServiceImpl) validateJavaScriptSpecificSecurity(code string) error {
	// 检查JavaScript特有的危险模式
	dangerousPatterns := []string{
		`document\..*`,  // DOM操作
		`window\..*`,    // 窗口对象访问
		`location\..*`,  // 位置对象访问
		`navigator\..*`, // 导航器对象访问
	}

	for _, pattern := range dangerousPatterns {
		if matched, _ := regexp.MatchString(pattern, code); matched {
			return errorx.NewByCode(errno.DangerousImportDetectedCode, errorx.WithExtraMsg("detected dangerous JavaScript pattern"))
		}
	}

	return nil
}

// checkDangerousFunctions 检查危险函数调用
func (c *EvaluatorSourceCodeServiceImpl) checkDangerousFunctions(code, language string) error {
	dangerousFunctions := map[string][]string{
		"javascript": {"eval", "Function", "setTimeout", "setInterval", "XMLHttpRequest", "fetch"},
		"typescript": {"eval", "Function", "setTimeout", "setInterval", "XMLHttpRequest", "fetch"},
		"python":     {"exec", "eval", "__import__", "open", "input", "compile", "globals", "locals"},
	}

	normalizedLang := strings.ToLower(strings.TrimSpace(language))
	functions, exists := dangerousFunctions[normalizedLang]
	if !exists {
		return nil
	}

	for _, fn := range functions {
		// 创建正则表达式匹配函数调用
		pattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(fn) + `\s*\(`)
		if pattern.MatchString(code) {
			return errorx.NewByCode(errno.DangerousFunctionDetectedCode, errorx.WithExtraMsg(fmt.Sprintf("detected function: %s", fn)))
		}
	}

	return nil
}

// checkDangerousImports 检查危险模块导入
func (c *EvaluatorSourceCodeServiceImpl) checkDangerousImports(code, language string) error {
	dangerousImports := map[string][]string{
		"javascript": {"fs", "child_process", "os", "path", "net", "http", "https"},
		"typescript": {"fs", "child_process", "os", "path", "net", "http", "https"},
		"python":     {"os", "sys", "subprocess", "socket", "urllib", "requests", "__builtin__", "builtins"},
	}

	normalizedLang := strings.ToLower(strings.TrimSpace(language))
	imports, exists := dangerousImports[normalizedLang]
	if !exists {
		return nil
	}

	for _, imp := range imports {
		var patterns []string

		switch normalizedLang {
		case "python":
			patterns = []string{
				`import\s+` + regexp.QuoteMeta(imp),
				`from\s+` + regexp.QuoteMeta(imp) + `\s+import`,
				`__import__\s*\(\s*['"` + regexp.QuoteMeta(imp) + `'"]`,
			}
		case "javascript", "typescript":
			patterns = []string{
				`import\s+.*from\s+['"]` + regexp.QuoteMeta(imp) + `['"]`,
				`require\s*\(\s*['"]` + regexp.QuoteMeta(imp) + `['"]`,
			}
		}

		for _, pattern := range patterns {
			regex := regexp.MustCompile(pattern)
			if regex.MatchString(code) {
				return errorx.NewByCode(errno.DangerousImportDetectedCode, errorx.WithExtraMsg(fmt.Sprintf("detected import: %s", imp)))
			}
		}
	}

	return nil
}

// checkMaliciousPatterns 检查恶意模式
func (c *EvaluatorSourceCodeServiceImpl) checkMaliciousPatterns(code, language string) error {
	patterns := c.getMaliciousPatternsForLanguage(language)

	for _, pattern := range patterns {
		regex := regexp.MustCompile(pattern.Pattern)
		if regex.MatchString(code) {
			return c.createSecurityViolationError(pattern, language)
		}
	}

	return nil
}

// getMaliciousPatternsForLanguage 根据语言获取相应的恶意模式列表
func (c *EvaluatorSourceCodeServiceImpl) getMaliciousPatternsForLanguage(language string) []MaliciousPattern {
	// 标准化语言名称
	normalizedLang := strings.ToLower(strings.TrimSpace(language))

	// 语言别名映射
	langAliases := map[string]string{
		"js":         "javascript",
		"ts":         "javascript", // TypeScript 使用 JavaScript 的模式
		"typescript": "javascript",
		"py":         "python",
		"golang":     "go",
	}

	// 如果有别名，使用标准名称
	if alias, exists := langAliases[normalizedLang]; exists {
		normalizedLang = alias
	}

	// 获取语言特定的模式
	if patterns, exists := maliciousPatternsMap[normalizedLang]; exists {
		return patterns
	}

	// 如果没有找到特定语言的模式，返回通用模式（主要是进程控制相关）
	return []MaliciousPattern{
		{
			Pattern:     `exit\s*\(`,
			Category:    CategoryProcessControl,
			Description: "进程退出函数调用",
			Languages:   []string{"general"},
			Severity:    "medium",
			Risk:        "此函数会强制退出程序，可能导致评估过程异常终止",
			Suggestion:  "请使用 return 语句正常返回结果，避免使用 exit() 函数",
		},
		{
			Pattern:     `quit\s*\(`,
			Category:    CategoryProcessControl,
			Description: "程序退出函数调用",
			Languages:   []string{"general"},
			Severity:    "medium",
			Risk:        "此函数会强制退出程序，可能导致评估过程异常终止",
			Suggestion:  "请使用 return 语句正常返回结果，避免使用 quit() 函数",
		},
	}
}

// createSecurityViolationError 创建详细的安全违规错误信息
func (c *EvaluatorSourceCodeServiceImpl) createSecurityViolationError(pattern MaliciousPattern, language string) error {
	details := SecurityViolationDetails{
		Category:    pattern.Category,
		Pattern:     pattern.Pattern,
		Description: pattern.Description,
		Language:    language,
		Risk:        pattern.Risk,
		Suggestion:  pattern.Suggestion,
	}

	// 构建详细的错误信息
	errorMsg := c.formatSecurityViolationMessage(details)

	return errorx.NewByCode(errno.MaliciousCodePatternDetectedCode, errorx.WithExtraMsg(errorMsg))
}

// formatSecurityViolationMessage 格式化安全违规错误信息
func (c *EvaluatorSourceCodeServiceImpl) formatSecurityViolationMessage(details SecurityViolationDetails) string {
	// 威胁类型的中文映射
	categoryNames := map[MaliciousPatternCategory]string{
		CategoryInfiniteLoop:   "无限循环",
		CategoryProcessControl: "进程控制",
		CategoryAsyncOperation: "异步操作",
		CategoryResourceAccess: "资源访问",
	}

	categoryName := categoryNames[details.Category]
	if categoryName == "" {
		categoryName = string(details.Category)
	}

	return fmt.Sprintf(`安全违规：检测到恶意代码模式
- 威胁类型：%s (%s)
- 检测到的模式：%s
- 编程语言：%s
- 风险说明：%s
- 修复建议：%s`,
		categoryName,
		details.Category,
		details.Description,
		details.Language,
		details.Risk,
		details.Suggestion,
	)
}

// buildExtParams 构建扩展参数，包含 space_id
func (c *EvaluatorSourceCodeServiceImpl) buildExtParams(evaluator *entity.Evaluator) map[string]string {
	ext := make(map[string]string)

	// 添加 space_id
	if evaluator != nil {
		spaceID := evaluator.GetSpaceID()
		if spaceID > 0 {
			ext["space_id"] = strconv.FormatInt(spaceID, 10)
		}
	}

	return ext
}

// getTimeoutMS 获取超时时间（毫秒）
func (c *EvaluatorSourceCodeServiceImpl) getTimeoutMS() int64 {
	// 默认5秒超时
	return 5000
}

// validateExecEvaluationFunction 验证代码中是否定义了 exec_evaluation 函数
func (c *EvaluatorSourceCodeServiceImpl) validateExecEvaluationFunction(codeVersion *entity.CodeEvaluatorVersion) error {
	switch codeVersion.LanguageType {
	case entity.LanguageTypePython:
		return c.validatePythonExecEvaluationFunction(codeVersion.CodeContent)
	case entity.LanguageTypeJS:
		return c.validateJavaScriptExecEvaluationFunction(codeVersion.CodeContent)
	default:
		return errorx.NewByCode(errno.UnsupportedLanguageTypeCode, errorx.WithExtraMsg(fmt.Sprintf("unsupported language type for exec_evaluation validation: %s", codeVersion.LanguageType)))
	}
}

// validatePythonExecEvaluationFunction 验证Python代码中是否定义了 exec_evaluation 函数
func (c *EvaluatorSourceCodeServiceImpl) validatePythonExecEvaluationFunction(code string) error {
	// 使用正则表达式匹配 Python 函数定义
	// 支持多行匹配，匹配 def exec_evaluation( 格式
	pattern := `(?m)^\s*def\s+exec_evaluation\s*\(`
	regex := regexp.MustCompile(pattern)
	if !regex.MatchString(code) {
		return errorx.NewByCode(errno.RequiredFunctionNotFoundCode, errorx.WithExtraMsg("代码中必须定义 exec_evaluation 函数。Python 函数定义格式：def exec_evaluation(turn_data):"))
	}
	return nil
}

// validateJavaScriptExecEvaluationFunction 验证JavaScript代码中是否定义了 exec_evaluation 函数
func (c *EvaluatorSourceCodeServiceImpl) validateJavaScriptExecEvaluationFunction(code string) error {
	// JavaScript 支持多种函数定义方式
	patterns := []string{
		`(?m)^\s*function\s+exec_evaluation\s*\(`,             // function exec_evaluation(
		`(?m)^\s*function\s+execEvaluation\s*\(`,              // function execEvaluation(
		`(?m)^\s*const\s+exec_evaluation\s*=\s*function\s*\(`, // const exec_evaluation = function(
		`(?m)^\s*const\s+execEvaluation\s*=\s*function\s*\(`,  // const execEvaluation = function(
		`(?m)^\s*const\s+exec_evaluation\s*=\s*\(`,            // const exec_evaluation = (
		`(?m)^\s*const\s+execEvaluation\s*=\s*\(`,             // const execEvaluation = (
		`(?m)^\s*let\s+exec_evaluation\s*=\s*function\s*\(`,   // let exec_evaluation = function(
		`(?m)^\s*let\s+execEvaluation\s*=\s*function\s*\(`,    // let execEvaluation = function(
		`(?m)^\s*let\s+exec_evaluation\s*=\s*\(`,              // let exec_evaluation = (
		`(?m)^\s*let\s+execEvaluation\s*=\s*\(`,               // let execEvaluation = (
		`(?m)^\s*var\s+exec_evaluation\s*=\s*function\s*\(`,   // var exec_evaluation = function(
		`(?m)^\s*var\s+execEvaluation\s*=\s*function\s*\(`,    // var execEvaluation = function(
		`(?m)^\s*var\s+exec_evaluation\s*=\s*\(`,              // var exec_evaluation = (
		`(?m)^\s*var\s+execEvaluation\s*=\s*\(`,               // var execEvaluation = (
	}

	for _, pattern := range patterns {
		regex := regexp.MustCompile(pattern)
		if regex.MatchString(code) {
			return nil
		}
	}

	return errorx.NewByCode(errno.RequiredFunctionNotFoundCode, errorx.WithExtraMsg("代码中必须定义 exec_evaluation 或 execEvaluation 函数。JavaScript 函数定义格式：function exec_evaluation(turn_data) { ... }"))
}

// ReportCodeRootSpanRequest Code评估器专用的上报请求结构
type ReportCodeRootSpanRequest struct {
	input            *entity.EvaluatorInputData
	output           *entity.EvaluatorOutputData
	runStatus        entity.EvaluatorRunStatus
	evaluatorVersion *entity.CodeEvaluatorVersion
	errInfo          error
	code             string // 评估器代码内容
}

// reportCodeRootSpan 上报Code评估器的根节点trace
func (e *codeEvaluatorSpan) reportCodeRootSpan(ctx context.Context, request *ReportCodeRootSpanRequest) {
	e.SetInput(ctx, tracer.Convert2TraceString(request.input))
	if request.output != nil {
		e.SetOutput(ctx, tracer.Convert2TraceString(request.output.EvaluatorResult))
	}
	switch request.runStatus {
	case entity.EvaluatorRunStatusSuccess:
		e.SetStatusCode(ctx, 0)
	case entity.EvaluatorRunStatusFail:
		e.SetStatusCode(ctx, int(entity.EvaluatorRunStatusFail))
		e.SetError(ctx, request.errInfo)
	default:
		e.SetStatusCode(ctx, 0) // 默认为成功
	}
	tags := make(map[string]interface{}, 0)
	// 防止空指针引用
	if request.evaluatorVersion != nil {
		tags["evaluator_id"] = request.evaluatorVersion.EvaluatorID
		tags["evaluator_version"] = request.evaluatorVersion.Version
	}
	tags["code_content"] = request.code // 添加代码内容到trace
	e.SetCallType("Evaluator")
	userIDInContext := session.UserIDInCtxOrEmpty(ctx)
	if userIDInContext != "" {
		e.SetUserID(ctx, userIDInContext)
	}
	e.SetTags(ctx, tags)
	e.Finish(ctx)
}

// codeEvaluatorSpan Code评估器专用span结构体
type codeEvaluatorSpan struct {
	looptracer.Span
}

// newEvaluatorSpan 创建评估器span
func (c *EvaluatorSourceCodeServiceImpl) newEvaluatorSpan(ctx context.Context, spanName, spanType, spaceID string, asyncChild bool) (*codeEvaluatorSpan, context.Context) {
	var evalSpan looptracer.Span
	var nctx context.Context
	if asyncChild {
		nctx, evalSpan = looptracer.GetTracer().StartSpan(ctx, spanName, spanType, looptracer.WithSpanWorkspaceID(spaceID))
	} else {
		nctx, evalSpan = looptracer.GetTracer().StartSpan(ctx, spanName, spanType, looptracer.WithStartNewTrace(), looptracer.WithSpanWorkspaceID(spaceID))
	}

	return &codeEvaluatorSpan{
		Span: evalSpan,
	}, nctx
}
