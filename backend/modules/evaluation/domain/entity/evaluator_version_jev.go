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

// JevQuestionType Jev 问题原语类型，与 kitex_gen JevQuestionType 对齐
type JevQuestionType int64

const (
	JevQuestionTypeNoul   JevQuestionType = 1 // 是非题，返回 yes 概率
	JevQuestionTypeChoice JevQuestionType = 2 // 单选（≤255 项）
	JevQuestionTypeScore  JevQuestionType = 3 // 有序档位打分（2-10 档）
)

type JevQuestion struct {
	Key            *string           `json:"key,omitempty"` // 适配层生成，页面不暴露
	Type           JevQuestionType   `json:"type,omitempty"`
	Instructions   *string           `json:"instructions,omitempty"`
	ChoiceCriteria map[string]string `json:"choice_criteria,omitempty"` // 仅 Choice
	ScoreLevels    []string          `json:"score_levels,omitempty"`    // 仅 Score
}

type JevEvaluatorVersion struct {
	// standard EvaluatorVersion layer attributes
	ID            int64         `json:"id"`
	SpaceID       int64         `json:"space_id"`
	EvaluatorType EvaluatorType `json:"evaluator_type"`
	EvaluatorID   int64         `json:"evaluator_id"`
	Description   string        `json:"description"`
	Version       string        `json:"version"`
	BaseInfo      *BaseInfo     `json:"base_info"`

	// standard EvaluatorContent layer attributes；state 复用 InputSchemas 承载评测字段
	InputSchemas  []*ArgsSchema `json:"input_schemas"`
	OutputSchemas []*ArgsSchema `json:"output_schemas"`

	// specific JevEvaluator layer attributes, refer to JevEvaluator DTO
	Model     *string        `json:"model,omitempty"`     // jev 模型版本，默认 jev-latest
	Questions []*JevQuestion `json:"questions,omitempty"` // MVP 限定长度 1
	APIKey    string         `json:"api_key,omitempty"`   // 敏感字段：加密存储、不明文回显
}

func (do *JevEvaluatorVersion) SetID(id int64) {
	do.ID = id
}

func (do *JevEvaluatorVersion) GetID() int64 {
	return do.ID
}

func (do *JevEvaluatorVersion) SetEvaluatorID(evaluatorID int64) {
	do.EvaluatorID = evaluatorID
}

func (do *JevEvaluatorVersion) GetEvaluatorID() int64 {
	return do.EvaluatorID
}

func (do *JevEvaluatorVersion) SetSpaceID(spaceID int64) {
	do.SpaceID = spaceID
}

func (do *JevEvaluatorVersion) GetSpaceID() int64 {
	return do.SpaceID
}

func (do *JevEvaluatorVersion) GetVersion() string {
	return do.Version
}

func (do *JevEvaluatorVersion) SetVersion(version string) {
	do.Version = version
}

func (do *JevEvaluatorVersion) SetDescription(description string) {
	do.Description = description
}

func (do *JevEvaluatorVersion) GetDescription() string {
	return do.Description
}

func (do *JevEvaluatorVersion) SetBaseInfo(baseInfo *BaseInfo) {
	do.BaseInfo = baseInfo
}

func (do *JevEvaluatorVersion) GetBaseInfo() *BaseInfo {
	return do.BaseInfo
}

func (do *JevEvaluatorVersion) ValidateInput(input *EvaluatorInputData) error {
	if input == nil {
		return errorx.NewByCode(errno.InvalidInputDataCode, errorx.WithExtraMsg("input data is nil"))
	}
	inputSchemaMap := make(map[string]*ArgsSchema)
	for _, argsSchema := range do.InputSchemas {
		inputSchemaMap[gptr.Indirect(argsSchema.Key)] = argsSchema
	}
	for fieldKey, content := range input.InputFields {
		if content == nil {
			continue
		}
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

func (do *JevEvaluatorVersion) ValidateBaseInfo() error {
	if do == nil {
		return errorx.NewByCode(errno.EvaluatorNotExistCode, errorx.WithExtraMsg("evaluator_version is nil"))
	}
	return nil
}

// MaskJevAPIKey 对 api_key 做出参脱敏：保留前 4 后 2 位，其余以 * 代替；过短则整体脱敏。
// 空串返回空串（未配置场景），避免把占位符误当成真实值回传。
func MaskJevAPIKey(apiKey string) string {
	if apiKey == "" {
		return ""
	}
	runes := []rune(apiKey)
	if len(runes) <= 6 {
		return "****"
	}
	return string(runes[:4]) + "****" + string(runes[len(runes)-2:])
}
