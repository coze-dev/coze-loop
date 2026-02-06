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

type AgentEvaluatorVersion struct {
	// standard EvaluatorVersion layer attributes
	ID            int64         `json:"id"`
	SpaceID       int64         `json:"space_id"`
	EvaluatorType EvaluatorType `json:"evaluator_type"`
	EvaluatorID   int64         `json:"evaluator_id"`
	Description   string        `json:"description"`
	Version       string        `json:"version"`
	BaseInfo      *BaseInfo     `json:"base_info"`

	// standard EvaluatorContent layer attributes
	InputSchemas  []*ArgsSchema `json:"input_schemas"`
	OutputSchemas []*ArgsSchema `json:"output_schemas"`

	// specific AgentEvaluatorVersion layer attributes, refer to AgentEvaluator DTO
	// an agent implementation, based on a model, equipped with some skills, running some prompt
	AgentConfig  *AgentConfig                `json:"agent_config,omitempty"`  // agent config
	ModelConfig  *ModelConfig                `json:"model_config,omitempty"`  // model config for agent
	SkillConfigs []*SkillConfig              `json:"skill_configs,omitempty"` // skill configs for agent
	PromptConfig *AgentEvaluatorPromptConfig `json:"prompt_config,omitempty"` // agent prompt config for agent
}

type AgentEvaluatorPromptConfig struct {
	MessageList []*Message                             `json:"message_list,omitempty"`
	OutputRules *AgentEvaluatorPromptConfigOutputRules `json:"output_rules,omitempty"`
}

type AgentEvaluatorPromptConfigOutputRules struct {
	ScorePrompt       *Message `json:"score_prompt,omitempty"`        // 分值
	ReasoningPrompt   *Message `json:"reasoning_prompt,omitempty"`    // 原因
	ExtraOutputPrompt *Message `json:"extra_output_prompt,omitempty"` // 附加输出
}

func (do *AgentEvaluatorVersion) SetID(id int64) {
	do.ID = id
}

func (do *AgentEvaluatorVersion) GetID() int64 {
	return do.ID
}

func (do *AgentEvaluatorVersion) SetEvaluatorID(evaluatorID int64) {
	do.EvaluatorID = evaluatorID
}

func (do *AgentEvaluatorVersion) GetEvaluatorID() int64 {
	return do.EvaluatorID
}

func (do *AgentEvaluatorVersion) SetSpaceID(spaceID int64) {
	do.SpaceID = spaceID
}

func (do *AgentEvaluatorVersion) GetSpaceID() int64 {
	return do.SpaceID
}

func (do *AgentEvaluatorVersion) GetVersion() string {
	return do.Version
}

func (do *AgentEvaluatorVersion) SetVersion(version string) {
	do.Version = version
}

func (do *AgentEvaluatorVersion) SetDescription(description string) {
	do.Description = description
}

func (do *AgentEvaluatorVersion) GetDescription() string {
	return do.Description
}

func (do *AgentEvaluatorVersion) SetBaseInfo(baseInfo *BaseInfo) {
	do.BaseInfo = baseInfo
}

func (do *AgentEvaluatorVersion) GetBaseInfo() *BaseInfo {
	return do.BaseInfo
}

func (do *AgentEvaluatorVersion) ValidateInput(input *EvaluatorInputData) error {
	inputSchemaMap := make(map[string]*ArgsSchema)
	for _, argsSchema := range do.InputSchemas {
		inputSchemaMap[gptr.Indirect(argsSchema.Key)] = argsSchema
	}
	for fieldKey, content := range input.InputFields {
		if content == nil {
			continue
		}
		// no need to validate schema for fields not defined in input schemas
		if argsSchema, ok := inputSchemaMap[fieldKey]; ok {
			if !gslice.Contains(argsSchema.SupportContentTypes, gptr.Indirect(content.ContentType)) {
				return errorx.NewByCode(errno.ContentTypeNotSupportedCode, errorx.WithExtraMsg(fmt.Sprintf("content type %v not supported", gptr.Indirect(content.ContentType))))
			}
			if gptr.Indirect(content.ContentType) == ContentTypeText {
				valid, err := json.ValidateJSONSchema(gptr.Indirect(argsSchema.JsonSchema), gptr.Indirect(content.Text))
				if err != nil || !valid {
					return errorx.NewByCode(errno.ContentSchemaInvalidCode, errorx.WithExtraMsg(fmt.Sprintf("content %v does not validate with expected schema: %v", gptr.Indirect(content.Text), gptr.Indirect(argsSchema.JsonSchema))))
				}
			}
		}
	}
	return nil
}

func (do *AgentEvaluatorVersion) ValidateBaseInfo() error {
	if do == nil {
		return errorx.NewByCode(errno.EvaluatorNotExistCode, errorx.WithExtraMsg("evaluator_version is nil"))
	}
	return nil
}
