// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"fmt"

	"github.com/bytedance/gg/gptr"
	"github.com/bytedance/gg/gslice"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

type PromptEvaluatorVersion struct {
	ID                 int64            `json:"id"`
	SpaceID            int64            `json:"space_id"`
	EvaluatorType      EvaluatorType    `json:"evaluator_type"`
	EvaluatorID        int64            `json:"evaluator_id"`
	Description        string           `json:"description"`
	Version            string           `json:"version"`
	InputSchemas       []*ArgsSchema    `json:"input_schemas"`
	PromptSourceType   PromptSourceType `json:"prompt_source_type"`
	PromptTemplateKey  string           `json:"prompt_template_key"`
	MessageList        []*Message       `json:"message_list"`
	ModelConfig        *ModelConfig     `json:"model_config"`
	Tools              []*Tool          `json:"tools"`
	ReceiveChatHistory *bool            `json:"receive_chat_history"`
	BaseInfo           *BaseInfo        `json:"base_info"`
	ParseType          ParseType        `json:"parse_type"`
	PromptSuffix       string           `json:"prompt_suffix"`
}

type PromptSourceType int64

const (
	PromptSourceTypeBuiltinTemplate PromptSourceType = 1
	PromptSourceTypeLoopPrompt      PromptSourceType = 2
	PromptSourceTypeCustom          PromptSourceType = 3
)

func (do *PromptEvaluatorVersion) SetID(id int64) {
	do.ID = id
}

func (do *PromptEvaluatorVersion) GetID() int64 {
	return do.ID
}

func (do *PromptEvaluatorVersion) SetEvaluatorID(evaluatorID int64) {
	do.EvaluatorID = evaluatorID
}

func (do *PromptEvaluatorVersion) GetEvaluatorID() int64 {
	return do.EvaluatorID
}

func (do *PromptEvaluatorVersion) SetSpaceID(spaceID int64) {
	do.SpaceID = spaceID
}

func (do *PromptEvaluatorVersion) GetSpaceID() int64 {
	return do.SpaceID
}

func (do *PromptEvaluatorVersion) GetVersion() string {
	return do.Version
}

func (do *PromptEvaluatorVersion) SetVersion(version string) {
	do.Version = version
}

func (do *PromptEvaluatorVersion) SetDescription(description string) {
	do.Description = description
}

func (do *PromptEvaluatorVersion) GetDescription() string {
	return do.Description
}

func (do *PromptEvaluatorVersion) SetBaseInfo(baseInfo *BaseInfo) {
	do.BaseInfo = baseInfo
}

func (do *PromptEvaluatorVersion) GetBaseInfo() *BaseInfo {
	return do.BaseInfo
}

func (do *PromptEvaluatorVersion) SetTools(tools []*Tool) {
	do.Tools = tools
}

func (do *PromptEvaluatorVersion) GetPromptTemplateKey() string {
	return do.PromptTemplateKey
}

func (do *PromptEvaluatorVersion) SetPromptSuffix(promptSuffix string) {
	do.PromptSuffix = promptSuffix
}

func (do *PromptEvaluatorVersion) SetParseType(parseType ParseType) {
	do.ParseType = parseType
}

func (do *PromptEvaluatorVersion) GetModelConfig() *ModelConfig {
	return do.ModelConfig
}

// ValidateInput 验证输入数据
func (do *PromptEvaluatorVersion) ValidateInput(input *EvaluatorInputData) error {
	// 实现验证逻辑，
	inputSchemaMap := make(map[string]*ArgsSchema)
	for _, argsSchema := range do.InputSchemas {
		inputSchemaMap[gptr.Indirect(argsSchema.Key)] = argsSchema
	}
	for fieldKey, content := range input.InputFields {
		if content == nil {
			continue
		}
		// schema中不存在的字段无需校验
		if argsSchema, ok := inputSchemaMap[fieldKey]; ok {
			if !gslice.Contains(argsSchema.SupportContentTypes, gptr.Indirect(content.ContentType)) {
				return errorx.NewByCode(errno.ContentTypeNotSupportedCode, errorx.WithExtraMsg(fmt.Sprintf("content type %v not supported", content.ContentType)))
			}
			if gptr.Indirect(content.ContentType) == ContentTypeText {
				valid, err := json.ValidateJSONSchema(gptr.Indirect(argsSchema.JsonSchema), gptr.Indirect(content.Text))
				if err != nil || !valid {
					return errorx.NewByCode(errno.ContentSchemaInvalidCode, errorx.WithExtraMsg(fmt.Sprintf("content %v does not validate with expected schema: %v", content.Text, argsSchema.JsonSchema)))
				}
			}
		}
	}
	return nil
}

// ValidateBaseInfo 校验评估器基本信息
func (do *PromptEvaluatorVersion) ValidateBaseInfo() error {
	if do == nil {
		return errorx.NewByCode(errno.EvaluatorNotExistCode, errorx.WithExtraMsg("evaluator_version is nil"))
	}
	if len(do.MessageList) == 0 {
		return errorx.NewByCode(errno.InvalidMessageListCode, errorx.WithExtraMsg("message list is empty"))
	}
	if do.ModelConfig == nil {
		return errorx.NewByCode(errno.InvalidModelConfigCode, errorx.WithExtraMsg("model config is nil"))
	}
	if do.ModelConfig.ModelID == nil && do.ModelConfig.ProviderModelID == nil {
		return errorx.NewByCode(errno.InvalidModelConfigCode, errorx.WithExtraMsg("model id is empty"))
	}
	return nil
}
