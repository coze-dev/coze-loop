// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/mem"
)

// PreserveModelConfigForSave 保护草稿遗漏的模型及思考字段，执行override仍整包替换。
func PreserveModelConfigForSave(incoming, previous *ModelConfig) (*ModelConfig, error) {
	if incoming == nil {
		incoming = previous
	}
	if incoming == nil {
		return nil, nil
	}
	result := &ModelConfig{}
	if err := mem.DeepCopy(incoming, result); err != nil {
		return nil, err
	}
	specificEffort := result.Thinking != nil && result.Thinking.ReasoningEffortValue != nil && *result.Thinking.ReasoningEffortValue != ""
	if previous != nil && result.ModelID == previous.ModelID && previous.Thinking != nil {
		if result.Thinking == nil {
			result.Thinking = &ThinkingConfig{}
			if err := mem.DeepCopy(previous.Thinking, result.Thinking); err != nil {
				return nil, err
			}
		} else {
			if result.Thinking.ReasoningEffortValue == nil && previous.Thinking.ReasoningEffortValue != nil {
				if result.Thinking.ReasoningEffort != nil && (previous.Thinking.ReasoningEffort == nil || *result.Thinking.ReasoningEffort != *previous.Thinking.ReasoningEffort) {
					return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("reasoning_effort_value missing while legacy reasoning_effort changed"))
				}
				value := *previous.Thinking.ReasoningEffortValue
				result.Thinking.ReasoningEffortValue = &value
			}
			if result.Thinking.ReasoningEffort == nil && previous.Thinking.ReasoningEffort != nil {
				value := *previous.Thinking.ReasoningEffort
				result.Thinking.ReasoningEffort = &value
			}
		}
		if result.Thinking.ThinkingOption == nil && previous.Thinking.ThinkingOption != nil {
			value := *previous.Thinking.ThinkingOption
			result.Thinking.ThinkingOption = &value
		}
		if result.Thinking.BudgetTokens == nil && previous.Thinking.BudgetTokens != nil && !specificEffort {
			value := *previous.Thinking.BudgetTokens
			result.Thinking.BudgetTokens = &value
		}
	}
	if result.Thinking != nil && result.Thinking.ThinkingOption != nil && *result.Thinking.ThinkingOption == ThinkingOptionDisabled {
		if result.Thinking.ReasoningEffortValue != nil {
			value := ""
			result.Thinking.ReasoningEffortValue = &value
			result.Thinking.BudgetTokens = nil
		}
	} else if result.Thinking != nil && result.Thinking.ReasoningEffortValue != nil && *result.Thinking.ReasoningEffortValue != "" && result.Thinking.BudgetTokens != nil {
		return nil, errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg("reasoning effort and thinking budget are mutually exclusive"))
	}
	return result, nil
}
