// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"fmt"
	"strings"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

// CodeEvaluatorVersion Code评估器版本实体
type CodeEvaluatorVersion struct {
	ID                 int64         `json:"id"`
	SpaceID            int64         `json:"space_id"`
	EvaluatorType      EvaluatorType `json:"evaluator_type"`
	EvaluatorID        int64         `json:"evaluator_id"`
	Description        string        `json:"description"`
	Version            string        `json:"version"`
	BaseInfo           *BaseInfo     `json:"base_info"`

	// Code评估器特有字段
	CodeTemplateKey  *string      `json:"code_template_key"`
	CodeTemplateName *string      `json:"code_template_name"`
	CodeContent      string       `json:"code_content"`
	LanguageType     LanguageType `json:"language_type"`
}

// LanguageType 编程语言类型
type LanguageType string

const (
	LanguageTypePython LanguageType = "python"
	LanguageTypeJS     LanguageType = "js"
)

var LanguageTypeSet = map[LanguageType]struct{}{
	LanguageTypePython: {},
	LanguageTypeJS:     {},
}

func (do *CodeEvaluatorVersion) SetID(id int64) {
	do.ID = id
}

func (do *CodeEvaluatorVersion) GetID() int64 {
	return do.ID
}

func (do *CodeEvaluatorVersion) SetEvaluatorID(evaluatorID int64) {
	do.EvaluatorID = evaluatorID
}

func (do *CodeEvaluatorVersion) GetEvaluatorID() int64 {
	return do.EvaluatorID
}

func (do *CodeEvaluatorVersion) SetSpaceID(spaceID int64) {
	do.SpaceID = spaceID
}

func (do *CodeEvaluatorVersion) GetSpaceID() int64 {
	return do.SpaceID
}

func (do *CodeEvaluatorVersion) GetVersion() string {
	return do.Version
}

func (do *CodeEvaluatorVersion) SetVersion(version string) {
	do.Version = version
}

func (do *CodeEvaluatorVersion) SetDescription(description string) {
	do.Description = description
}

func (do *CodeEvaluatorVersion) GetDescription() string {
	return do.Description
}

func (do *CodeEvaluatorVersion) SetBaseInfo(baseInfo *BaseInfo) {
	do.BaseInfo = baseInfo
}

func (do *CodeEvaluatorVersion) GetBaseInfo() *BaseInfo {
	return do.BaseInfo
}

func (do *CodeEvaluatorVersion) GetCodeTemplateKey() *string {
	return do.CodeTemplateKey
}

func (do *CodeEvaluatorVersion) SetCodeTemplateKey(key *string) {
	do.CodeTemplateKey = key
}

func (do *CodeEvaluatorVersion) GetCodeTemplateName() *string {
	return do.CodeTemplateName
}

func (do *CodeEvaluatorVersion) SetCodeTemplateName(name *string) {
	do.CodeTemplateName = name
}

func (do *CodeEvaluatorVersion) GetCodeContent() string {
	return do.CodeContent
}

func (do *CodeEvaluatorVersion) SetCodeContent(content string) {
	do.CodeContent = content
}

func (do *CodeEvaluatorVersion) GetLanguageType() LanguageType {
	return do.LanguageType
}

func (do *CodeEvaluatorVersion) SetLanguageType(languageType LanguageType) {
	do.LanguageType = languageType
}

// ValidateInput 验证输入数据
func (do *CodeEvaluatorVersion) ValidateInput(input *EvaluatorInputData) error {
	if input == nil {
		return errorx.NewByCode(errno.InvalidInputDataCode, errorx.WithExtraMsg("input data is nil"))
	}
	// Code评估器暂时不需要特殊的输入验证逻辑
	return nil
}

// ValidateBaseInfo 校验评估器基本信息（支持大小写不敏感）
func (do *CodeEvaluatorVersion) ValidateBaseInfo() error {
	if do == nil {
		return errorx.NewByCode(errno.EvaluatorNotExistCode, errorx.WithExtraMsg("evaluator_version is nil"))
	}
	if do.CodeContent == "" {
		return errorx.NewByCode(errno.InvalidCodeContentCode, errorx.WithExtraMsg("code content is empty"))
	}
	
	// 标准化语言类型（转换为小写）
	normalizedLangType := normalizeLanguageType(do.LanguageType)
	if _, ok := LanguageTypeSet[normalizedLangType]; !ok {
		return errorx.NewByCode(errno.InvalidLanguageTypeCode, errorx.WithExtraMsg(fmt.Sprintf("invalid language type: %s", do.LanguageType)))
	}
	
	// 将标准化后的语言类型设置回去
	do.LanguageType = normalizedLangType
	return nil
}

// normalizeLanguageType 标准化语言类型（转换为小写）
func normalizeLanguageType(langType LanguageType) LanguageType {
	switch strings.ToLower(string(langType)) {
	case "python":
		return LanguageTypePython  // "python"
	case "js", "javascript":
		return LanguageTypeJS      // "js"
	default:
		return LanguageType(strings.ToLower(string(langType)))
	}
}

// ExecutionRequest 代码执行请求


// ExecutionRequest 代码执行请求
type ExecutionRequest struct {
	Code         string                 `json:"code"`
	Language     string                 `json:"language"`
	InputData    map[string]interface{} `json:"input_data"`
	TimeoutMS    int64                  `json:"timeout_ms"`
}

// ExecutionResult 代码执行结果 - 匹配远程沙箱服务的响应格式
type ExecutionResult struct {
	Output       *ExecutionOutput       `json:"output"`
	WorkloadInfo *ExecutionWorkloadInfo `json:"workload_info"`
}

// ExecutionOutput 执行输出信息
type ExecutionOutput struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
	RetVal string `json:"ret_val"`
}

// ExecutionWorkloadInfo 工作负载信息
type ExecutionWorkloadInfo struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// ProcessedExecutionResult 处理后的代码执行结果
type ProcessedExecutionResult struct {
	Output   map[string]interface{} `json:"output"`
	Stdout   string                 `json:"stdout"`
	Stderr   string                 `json:"stderr"`
	RetVal   string                 `json:"ret_val"`
	Success  bool                   `json:"success"`
	ErrorMsg string                 `json:"error_msg"`
}