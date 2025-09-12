// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

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
func (c *EvaluatorSourceCodeServiceImpl) Run(ctx context.Context, evaluator *entity.Evaluator, input *entity.EvaluatorInputData) (output *entity.EvaluatorOutputData, runStatus entity.EvaluatorRunStatus, traceID string) {
	startTime := time.Now()

	// 验证评估器类型
	if evaluator.EvaluatorType != entity.EvaluatorTypeCode || evaluator.CodeEvaluatorVersion == nil {
		return &entity.EvaluatorOutputData{
			EvaluatorRunError: &entity.EvaluatorRunError{
				Code:    int32(errno.InvalidEvaluatorTypeCode),
				Message: "invalid evaluator type or code evaluator version is nil",
			},
			TimeConsumingMS: time.Since(startTime).Milliseconds(),
			Stdout:          "",
		}, entity.EvaluatorRunStatusFail, ""
	}

	codeVersion := evaluator.CodeEvaluatorVersion

	// 1. 获取代码构建器
	codeBuilder, err := c.codeBuilderFactory.CreateBuilder(codeVersion.LanguageType)
	if err != nil {
		return &entity.EvaluatorOutputData{
			EvaluatorRunError: &entity.EvaluatorRunError{
				Code:    int32(errno.InvalidInputDataCode),
				Message: fmt.Sprintf("failed to get code builder for language %s: %v", codeVersion.LanguageType, err),
			},
			TimeConsumingMS: time.Since(startTime).Milliseconds(),
			Stdout:          "",
		}, entity.EvaluatorRunStatusFail, ""
	}

	// 2. 构建代码
	code, err := codeBuilder.BuildCode(input, codeVersion)
	if err != nil {
		return &entity.EvaluatorOutputData{
			EvaluatorRunError: &entity.EvaluatorRunError{
				Code:    int32(errno.InvalidInputDataCode),
				Message: fmt.Sprintf("failed to build code: %v", err),
			},
			TimeConsumingMS: time.Since(startTime).Milliseconds(),
			Stdout:          "",
		}, entity.EvaluatorRunStatusFail, ""
	}

	// 3. 获取Runtime
	runtime, err := c.runtimeManager.GetRuntime(codeVersion.LanguageType)
	if err != nil {
		return &entity.EvaluatorOutputData{
			EvaluatorRunError: &entity.EvaluatorRunError{
				Code:    int32(errno.InvalidLanguageTypeCode),
				Message: fmt.Sprintf("failed to get runtime for language %s: %v", codeVersion.LanguageType, err),
			},
			TimeConsumingMS: time.Since(startTime).Milliseconds(),
			Stdout:          "",
		}, entity.EvaluatorRunStatusFail, ""
	}

	// 4. 执行代码
	result, err := runtime.RunCode(ctx, code, string(codeVersion.LanguageType), c.getTimeoutMS())
	if err != nil {
		return &entity.EvaluatorOutputData{
			EvaluatorRunError: &entity.EvaluatorRunError{
				Code:    int32(errno.CodeExecutionFailedCode),
				Message: fmt.Sprintf("code execution failed: %v", err),
			},
			TimeConsumingMS: time.Since(startTime).Milliseconds(),
			Stdout:          "",
		}, entity.EvaluatorRunStatusFail, ""
	}

	// 检查执行结果中的错误信息
	var evaluatorRunError *entity.EvaluatorRunError
	if result.Output != nil {
		// 优先从Stderr解析错误信息
		if result.Output.Stderr != "" {
			evaluatorRunError = &entity.EvaluatorRunError{
				Code:    int32(errno.CodeExecutionFailedCode),
				Message: result.Output.Stderr,
			}
		} else if result.Output.RetVal != "" {
			// 如果Stderr为空，尝试从RetVal中的err_msg字段解析
			if _, _, errMsg, parseErr := c.parseEvaluationRetVal(result.Output.RetVal); parseErr == nil && errMsg != "" {
				evaluatorRunError = &entity.EvaluatorRunError{
					Code:    int32(errno.CodeExecutionFailedCode),
					Message: errMsg,
				}
			}
		}
	}

	// 如果有错误信息，返回失败状态
	if evaluatorRunError != nil {
		return &entity.EvaluatorOutputData{
			EvaluatorRunError: evaluatorRunError,
			TimeConsumingMS:   time.Since(startTime).Milliseconds(),
			Stdout: func() string {
				if result.Output != nil {
					return result.Output.Stdout // 直接使用FaaS的stdout
				}
				return ""
			}(),
		}, entity.EvaluatorRunStatusFail, ""
	}

	// 解析执行结果
	evaluatorResult, err := c.parseEvaluationExecutionResult(result)
	if err != nil {
		return &entity.EvaluatorOutputData{
			EvaluatorRunError: &entity.EvaluatorRunError{
				Code:    int32(errno.ResultParseFailedCode),
				Message: fmt.Sprintf("failed to parse execution result: %v", err),
			},
			TimeConsumingMS: time.Since(startTime).Milliseconds(),
			Stdout: func() string {
				if result.Output != nil {
					return result.Output.Stdout // 直接使用FaaS的stdout
				}
				return ""
			}(),
		}, entity.EvaluatorRunStatusFail, ""
	}

		// 构造输出数据
	outputData := &entity.EvaluatorOutputData{
		EvaluatorResult: evaluatorResult,
		EvaluatorUsage: &entity.EvaluatorUsage{
			InputTokens:  0, // Code评估器暂不计算token
			OutputTokens: 0,
		},
		TimeConsumingMS: time.Since(startTime).Milliseconds(),
		Stdout: func() string {
			if result.Output != nil {
				return result.Output.Stdout // 直接使用FaaS的stdout
			}
			return ""
		}(),
	}

	return outputData, entity.EvaluatorRunStatusSuccess, ""
}

// Debug 调试Code评估器
func (c *EvaluatorSourceCodeServiceImpl) Debug(ctx context.Context, evaluator *entity.Evaluator, input *entity.EvaluatorInputData) (output *entity.EvaluatorOutputData, err error) {
	// 调试模式下直接调用Run方法
	output, runStatus, _ := c.Run(ctx, evaluator, input)
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
		return fmt.Errorf("invalid evaluator type or code evaluator version is nil")
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

	// 2. 再进行语法检查（现有逻辑）
	switch codeVersion.LanguageType {
	case entity.LanguageTypePython:
		return c.validatePythonCode(ctx, codeVersion)
	case entity.LanguageTypeJS:
		return c.validateJavaScriptCode(ctx, codeVersion)
	default:
		return fmt.Errorf("unsupported language type: %s", codeVersion.LanguageType)
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

// cleanStdoutForUser 清理stdout输出，直接使用FaaS返回的stdout字段
func (c *EvaluatorSourceCodeServiceImpl) cleanStdoutForUser(stdout string) string {
	// 直接返回FaaS服务的stdout，不再做复杂解析
	return stdout
}

// cleanStdoutForReasoning 清理stdout用作reasoning，移除冗余的系统信息
func (c *EvaluatorSourceCodeServiceImpl) cleanStdoutForReasoning(stdout string) string {
	if stdout == "" {
		return ""
	}
	
	// 首先尝试从stdout中提取评估结果JSON
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		// 尝试解析每一行，找到包含reason的JSON
		var testResult map[string]interface{}
		if err := json.Unmarshal([]byte(line), &testResult); err == nil {
			if reasonVal, hasReason := testResult["reason"]; hasReason {
				if reasonStr, ok := reasonVal.(string); ok && reasonStr != "" {
					return reasonStr
				}
			}
		}
	}
	
	// 如果没有找到JSON格式的reason，返回清理后的用户输出
	return c.cleanStdoutForUser(stdout)
}

// parseSyntaxValidationStdoutJSON 解析语法校验stdout中的JSON内容（语法校验链路）
func (c *EvaluatorSourceCodeServiceImpl) parseSyntaxValidationStdoutJSON(stdout string) (map[string]interface{}, error) {
	// 清理stdout，移除换行符和额外的空白字符
	stdout = strings.TrimSpace(stdout)
	if stdout == "" {
		return nil, fmt.Errorf("empty stdout")
	}

	// 尝试解析JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		return nil, fmt.Errorf("failed to parse stdout JSON: %v", err)
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
		return nil, fmt.Errorf("empty ret_val")
	}

	// 尝试解析JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(retVal), &result); err != nil {
		return nil, fmt.Errorf("failed to parse ret_val JSON: %v", err)
	}

	// 解码错误信息中的Unicode转义字符
	if errorVal, ok := result["error"]; ok {
		if errorStr, ok := errorVal.(string); ok {
			result["error"] = c.decodeUnicodeEscapes(errorStr)
		}
	}

	return result, nil
}

// processExecutionResult 处理执行结果，解码Unicode并提取有用信息
func (c *EvaluatorSourceCodeServiceImpl) processExecutionResult(result *entity.ExecutionResult) (*entity.ProcessedExecutionResult, error) {
	if result == nil {
		return nil, fmt.Errorf("execution result is nil")
	}

	processed := &entity.ProcessedExecutionResult{
		Success:  true,
		ErrorMsg: "",
		Output:   make(map[string]interface{}),
	}

	// 处理输出信息
	if result.Output != nil {
		// 解码stdout和stderr中的Unicode字符
		stdout := c.decodeUnicodeEscapes(result.Output.Stdout)
		stderr := c.decodeUnicodeEscapes(result.Output.Stderr)
		processed.RetVal = result.Output.RetVal

		// 如果有stderr输出，认为执行失败
		if stderr != "" {
			processed.Success = false
			processed.ErrorMsg = stderr
		}

		// 将基本信息添加到Output中
		processed.Output["stdout"] = stdout
		processed.Output["stderr"] = stderr
		if result.Output.RetVal != "" {
			processed.Output["ret_val"] = result.Output.RetVal
		}
	}

	// 记录工作负载信息用于调试
	if result.WorkloadInfo != nil {
		processed.Output["workload_id"] = result.WorkloadInfo.ID
		processed.Output["workload_status"] = result.WorkloadInfo.Status
	}

	return processed, nil
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
			if errorMsg, ok := errorVal.(string); ok {
				result.ErrorMsg = errorMsg
			}
		}
	}

	return result
}

// processSyntaxValidationExecutionResult 处理语法校验执行结果（语法校验链路：解析 valid 和 error）
// 作为统一的语法验证入口，负责所有的 valid 字段解析和验证
func (c *EvaluatorSourceCodeServiceImpl) processSyntaxValidationExecutionResult(result *entity.ExecutionResult) (*entity.ProcessedExecutionResult, error) {
	// 先进行基本处理
	processed, err := c.processExecutionResult(result)
	if err != nil {
		return nil, err
	}

	// 优先解析ret_val中的JSON内容
	if processed.RetVal != "" {
		if retValData, parseErr := c.parseSyntaxValidationRetValJSON(processed.RetVal); parseErr == nil {
			// 使用通用方法解析验证结果
			validationResult := c.parseSyntaxValidationResult(retValData)
			processed.Success = validationResult.Valid
			if !validationResult.Valid {
				processed.ErrorMsg = validationResult.ErrorMsg
			} else {
				processed.ErrorMsg = ""
			}

			// 将ret_val解析的内容合并到Output中
			for key, value := range retValData {
				processed.Output[key] = value
			}
			return processed, nil
		}
	}

	// 如果ret_val解析失败或为空，尝试解析stdout中的JSON内容作为备用
	if stdout, ok := processed.Output["stdout"].(string); ok && stdout != "" {
		if parsedOutput, parseErr := c.parseSyntaxValidationStdoutJSON(stdout); parseErr == nil {
			// 将解析的JSON内容合并到Output中
			for key, value := range parsedOutput {
				processed.Output[key] = value
			}

			// 使用通用方法解析验证结果
			validationResult := c.parseSyntaxValidationResult(parsedOutput)
			processed.Success = validationResult.Valid
			if !validationResult.Valid {
				processed.ErrorMsg = validationResult.ErrorMsg
			} else {
				processed.ErrorMsg = ""
			}
		}
	}

	return processed, nil
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

// parseEvaluationRetVal 解析评估结果RetVal字段中的JSON数据（评估结果链路：提取 score、reason、err_msg）
func (c *EvaluatorSourceCodeServiceImpl) parseEvaluationRetVal(retVal string) (score *float64, reason string, errMsg string, err error) {
	if strings.TrimSpace(retVal) == "" {
		return nil, "", "", nil
	}

	// 处理可能存在的嵌套JSON结构
	cleanedRetVal := c.cleanNestedJSON(retVal)
	
	var result map[string]interface{}
	
	// 首先尝试标准 JSON 解析
	if err := json.Unmarshal([]byte(cleanedRetVal), &result); err != nil {
		// 如果 JSON 解析失败，尝试 Python 字典格式
		jsonStr, convertErr := c.convertPythonDictToJSON(cleanedRetVal)
		if convertErr != nil {
			return nil, "", "", fmt.Errorf("failed to parse RetVal: %v", err)
		}
		
		if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
			return nil, "", "", fmt.Errorf("failed to parse converted RetVal JSON: %v", err)
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

	// 解析err_msg字段
	if errMsgVal, ok := result["err_msg"]; ok {
		if errMsgStr, ok := errMsgVal.(string); ok {
			errMsg = errMsgStr
		}
	}

	return score, reason, errMsg, nil
}

// parseEvaluationExecutionResult 解析评估器执行结果（评估结果链路：解析 score 和 reason）
func (c *EvaluatorSourceCodeServiceImpl) parseEvaluationExecutionResult(result *entity.ExecutionResult) (*entity.EvaluatorResult, error) {
	// 先处理原始执行结果
	processed, err := c.processExecutionResult(result)
	if err != nil {
		return nil, errorx.NewByCode(errno.ResultParseFailedCode, errorx.WithExtraMsg(fmt.Sprintf("failed to process execution result: %v", err)))
	}

	if !processed.Success {
		return nil, errorx.NewByCode(errno.CodeExecutionFailedCode, errorx.WithExtraMsg(processed.ErrorMsg))
	}

	evaluatorResult := &entity.EvaluatorResult{}

	// 直接从RetVal字段解析score和reason（简化后的逻辑）
	if result.Output != nil && result.Output.RetVal != "" {
		if score, reason, _, parseErr := c.parseEvaluationRetVal(result.Output.RetVal); parseErr == nil {
			if score != nil {
				evaluatorResult.Score = score
			}
			if reason != "" {
				evaluatorResult.Reasoning = reason
			}
			return evaluatorResult, nil
		}
	}

	// 如果RetVal解析失败，使用stdout作为推理过程
	if result.Output != nil && result.Output.Stdout != "" {
		evaluatorResult.Reasoning = result.Output.Stdout
	}
	
	return evaluatorResult, nil
}

// validatePythonCode 验证Python代码
func (c *EvaluatorSourceCodeServiceImpl) validatePythonCode(ctx context.Context, codeVersion *entity.CodeEvaluatorVersion) error {
	// 基础检查
	if codeVersion.CodeContent == "" {
		return fmt.Errorf("python code is empty")
	}

	// 额外的Python特定安全检查
	if err := c.validatePythonSpecificSecurity(codeVersion.CodeContent); err != nil {
		return err
	}

	// 获取Runtime
	runtime, err := c.runtimeManager.GetRuntime(entity.LanguageTypePython)
	if err != nil {
		return fmt.Errorf("failed to get python runtime for validation: %v", err)
	}

	// 构建Python语法检查代码，参考pyodide客户端的AST验证方式
	syntaxCheckCode := c.buildPythonSyntaxCheckCode(codeVersion.CodeContent)

	// 使用runtime执行语法检查，设置较短的超时时间
	result, err := runtime.RunCode(ctx, syntaxCheckCode, "python", 10000) // 10秒超时用于语法验证
	if err != nil {
		return fmt.Errorf("python syntax validation failed: %w", err)
	}

	// 处理执行结果并解析stdout中的JSON
	processed, err := c.processSyntaxValidationExecutionResult(result)
	if err != nil {
		return fmt.Errorf("failed to process syntax validation result: %w", err)
	}	// 直接使用 processSyntaxValidationExecutionResult 的验证结果
	// 该方法已经完成了所有的 valid 字段解析和验证
	if !processed.Success {
		return fmt.Errorf("python syntax error: %s", processed.ErrorMsg)
	}

	return nil
}

// validateJavaScriptCode 验证JavaScript代码
func (c *EvaluatorSourceCodeServiceImpl) validateJavaScriptCode(ctx context.Context, codeVersion *entity.CodeEvaluatorVersion) error {
	// 基础检查
	if codeVersion.CodeContent == "" {
		return fmt.Errorf("javascript code is empty")
	}

	// JavaScript特定安全检查
	if err := c.validateJavaScriptSpecificSecurity(codeVersion.CodeContent); err != nil {
		return err
	}

	// 获取Runtime
	runtime, err := c.runtimeManager.GetRuntime(entity.LanguageTypeJS)
	if err != nil {
		return fmt.Errorf("failed to get javascript runtime for validation: %v", err)
	}

	// 构建JavaScript语法检查代码 (使用Builder模式)
	syntaxCheckCode := c.buildJavaScriptSyntaxCheckCode(codeVersion.CodeContent)

	// 使用runtime执行语法检查，设置较短的超时时间
	result, err := runtime.RunCode(ctx, syntaxCheckCode, "js", 10000) // 与Python保持一致的10秒超时
	if err != nil {
		return fmt.Errorf("javascript syntax validation failed: %w", err)
	}

	// 使用统一的结果处理方法 (与Python保持一致)
	processed, err := c.processSyntaxValidationExecutionResult(result)
	if err != nil {
		return fmt.Errorf("failed to process syntax validation result: %w", err)
	}

	// 直接使用 processSyntaxValidationExecutionResult 的验证结果
	// 该方法已经完成了所有的 valid 字段解析和验证
	if !processed.Success {
		return fmt.Errorf("javascript syntax error: %s", processed.ErrorMsg)
	}

	return nil
}

// buildPythonSyntaxCheckCode 构建Python语法检查代码
func (c *EvaluatorSourceCodeServiceImpl) buildPythonSyntaxCheckCode(userCode string) string {
	// 获取Python代码构建器
	builder, err := c.codeBuilderFactory.CreateBuilder(entity.LanguageTypePython)
	if err != nil {
		// 如果无法获取构建器，使用简单的直接构建方式
		return c.buildSimplePythonSyntaxCheckCode(userCode)
	}

	pythonBuilder, ok := builder.(*PythonCodeBuilder)
	if !ok {
		// 如果类型断言失败，使用简单的直接构建方式
		return c.buildSimplePythonSyntaxCheckCode(userCode)
	}

	// 使用PythonCodeBuilder构建语法检查代码
	return pythonBuilder.BuildSyntaxCheckCode(userCode)
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
	// 获取JavaScript代码构建器
	builder, err := c.codeBuilderFactory.CreateBuilder(entity.LanguageTypeJS)
	if err != nil {
		// 如果无法获取构建器，使用简单的直接构建方式
		return c.buildSimpleJavaScriptSyntaxCheckCode(userCode)
	}

	jsBuilder, ok := builder.(*JavaScriptCodeBuilder)
	if !ok {
		// 如果类型断言失败，使用简单的直接构建方式
		return c.buildSimpleJavaScriptSyntaxCheckCode(userCode)
	}

	// 使用JavaScriptCodeBuilder构建语法检查代码
	return jsBuilder.BuildSyntaxCheckCode(userCode)
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
		return fmt.Errorf("代码不能为空")
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
			return fmt.Errorf("detected dangerous Python pattern")
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
			return fmt.Errorf("detected dangerous JavaScript pattern")
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
			return fmt.Errorf("安全违规: 检测到危险函数调用: %s", fn)
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
				return fmt.Errorf("安全违规: 检测到危险模块导入: %s", imp)
			}
		}
	}

	return nil
}

// checkMaliciousPatterns 检查恶意模式
func (c *EvaluatorSourceCodeServiceImpl) checkMaliciousPatterns(code, language string) error {
	// 通用恶意模式
	maliciousPatterns := []string{
		`while\s+True\s*:`,       // Python 无限循环
		`while\s*\(\s*true\s*\)`, // JS 无限循环
		`for\s*\(\s*;\s*;\s*\)`,  // JS 无限循环
		`setInterval\s*\(`,       // JS 定时器
		`setTimeout\s*\(`,        // JS 定时器
		`process\.exit`,          // 进程退出
		`System\.exit`,           // 系统退出
		`exit\s*\(`,              // 退出函数
		`quit\s*\(`,              // 退出函数
	}

	for _, pattern := range maliciousPatterns {
		regex := regexp.MustCompile(pattern)
		if regex.MatchString(code) {
			return fmt.Errorf("安全违规: 检测到潜在恶意代码模式")
		}
	}

	return nil
}

// getTimeoutMS 获取超时时间（毫秒）
func (c *EvaluatorSourceCodeServiceImpl) getTimeoutMS() int64 {
	// 默认5秒超时
	return 5000
}