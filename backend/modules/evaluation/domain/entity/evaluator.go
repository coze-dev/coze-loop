// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type Evaluator struct {
	ID             int64
	SpaceID        int64
	Name           string
	Description    string
	DraftSubmitted bool
	EvaluatorType  EvaluatorType
	LatestVersion  string
	BaseInfo       *BaseInfo

	Builtin   bool
	Benchmark string
	Vendor    string
	Tags      map[EvaluatorTagKey][]string `json:"tags"`

	PromptEvaluatorVersion *PromptEvaluatorVersion
	CodeEvaluatorVersion   *CodeEvaluatorVersion
	CustomRPCEvaluatorVersion *CustomRPCEvaluatorVersion
}

type EvaluatorType int64

const (
	EvaluatorTypePrompt    EvaluatorType = 1
	EvaluatorTypeCode      EvaluatorType = 2
	EvaluatorTypeCustomRPC EvaluatorType = 3
)

var EvaluatorTypeSet = map[EvaluatorType]struct{}{
	EvaluatorTypePrompt:    {},
	EvaluatorTypeCode:      {},
	EvaluatorTypeCustomRPC: {},
}

// GetEvaluatorVersionID 获取评估器版本ID
func (e *Evaluator) GetEvaluatorVersionID() int64 {
	switch e.EvaluatorType {
	case EvaluatorTypePrompt:
		if e.PromptEvaluatorVersion != nil {
			return e.PromptEvaluatorVersion.GetID()
		}
	case EvaluatorTypeCode:
		if e.CodeEvaluatorVersion != nil {
			return e.CodeEvaluatorVersion.GetID()
		}
	case EvaluatorTypeCustomRPC:
		if e.CustomRPCEvaluatorVersion != nil {
			return e.CustomRPCEvaluatorVersion.GetID()
		}
	default:
		return 0
	}
	return 0
}

// GetVersion 获取评估器版本号
func (e *Evaluator) GetVersion() string {
	switch e.EvaluatorType {
	case EvaluatorTypePrompt:
		if e.PromptEvaluatorVersion != nil {
			return e.PromptEvaluatorVersion.GetVersion()
		}
	case EvaluatorTypeCode:
		if e.CodeEvaluatorVersion != nil {
			return e.CodeEvaluatorVersion.GetVersion()
		}
	case EvaluatorTypeCustomRPC:
		if e.CustomRPCEvaluatorVersion != nil {
			return e.CustomRPCEvaluatorVersion.GetVersion()
		}
	default:
		return ""
	}
	return ""
}

// GetEvaluatorID 获取评估器ID
func (e *Evaluator) GetEvaluatorID() int64 {
	switch e.EvaluatorType {
	case EvaluatorTypePrompt:
		if e.PromptEvaluatorVersion != nil {
			return e.PromptEvaluatorVersion.GetEvaluatorID()
		}
	case EvaluatorTypeCode:
		if e.CodeEvaluatorVersion != nil {
			return e.CodeEvaluatorVersion.GetEvaluatorID()
		}
	case EvaluatorTypeCustomRPC:
		if e.CustomRPCEvaluatorVersion != nil {
			return e.CustomRPCEvaluatorVersion.GetEvaluatorID()
		}
	default:
		return 0
	}
	return 0
}

// GetSpaceID 获取空间ID
func (e *Evaluator) GetSpaceID() int64 {
	switch e.EvaluatorType {
	case EvaluatorTypePrompt:
		if e.PromptEvaluatorVersion != nil {
			return e.PromptEvaluatorVersion.GetSpaceID()
		}
	case EvaluatorTypeCode:
		if e.CodeEvaluatorVersion != nil {
			return e.CodeEvaluatorVersion.GetSpaceID()
		}
	case EvaluatorTypeCustomRPC:
		if e.CustomRPCEvaluatorVersion != nil {
			return e.CustomRPCEvaluatorVersion.GetSpaceID()
		}
	default:
		return 0
	}
	return 0
}

// GetEvaluatorDescription 获取评估器描述
func (e *Evaluator) GetEvaluatorDescription() string {
	return e.Description
}

// GetEvaluatorVersionDescription 获取评估器版本描述
func (e *Evaluator) GetEvaluatorVersionDescription() string {
	switch e.EvaluatorType {
	case EvaluatorTypePrompt:
		if e.PromptEvaluatorVersion != nil {
			return e.PromptEvaluatorVersion.GetDescription()
		}
	case EvaluatorTypeCode:
		if e.CodeEvaluatorVersion != nil {
			return e.CodeEvaluatorVersion.GetDescription()
		}
	case EvaluatorTypeCustomRPC:
		if e.CustomRPCEvaluatorVersion != nil {
			return e.CustomRPCEvaluatorVersion.GetDescription()
		}
	default:
		return ""
	}
	return ""
}

// GetBaseInfo 获取基础信息
func (e *Evaluator) GetBaseInfo() *BaseInfo {
	switch e.EvaluatorType {
	case EvaluatorTypePrompt:
		if e.PromptEvaluatorVersion != nil {
			return e.PromptEvaluatorVersion.GetBaseInfo()
		}
	case EvaluatorTypeCode:
		if e.CodeEvaluatorVersion != nil {
			return e.CodeEvaluatorVersion.GetBaseInfo()
		}
	case EvaluatorTypeCustomRPC:
		if e.CustomRPCEvaluatorVersion != nil {
			return e.CustomRPCEvaluatorVersion.GetBaseInfo()
		}
	default:
		return nil
	}
	return nil
}

// GetPromptTemplateKey 获取提示模板键
func (e *Evaluator) GetPromptTemplateKey() string {
	switch e.EvaluatorType {
	case EvaluatorTypePrompt:
		if e.PromptEvaluatorVersion != nil {
			return e.PromptEvaluatorVersion.GetPromptTemplateKey()
		}
	default:
		return ""
	}
	return ""
}

// GetModelConfig 获取模型配置
func (e *Evaluator) GetModelConfig() *ModelConfig {
	switch e.EvaluatorType {
	case EvaluatorTypePrompt:
		if e.PromptEvaluatorVersion != nil {
			return e.PromptEvaluatorVersion.GetModelConfig()
		}
	default:
		return nil
	}
	return nil
}

// ValidateInput 验证输入数据
func (e *Evaluator) ValidateInput(input *EvaluatorInputData) error {
	switch e.EvaluatorType {
	case EvaluatorTypePrompt:
		if e.PromptEvaluatorVersion != nil {
			return e.PromptEvaluatorVersion.ValidateInput(input)
		}
	case EvaluatorTypeCode:
		if e.CodeEvaluatorVersion != nil {
			return e.CodeEvaluatorVersion.ValidateInput(input)
		}
	case EvaluatorTypeCustomRPC:
		if e.CustomRPCEvaluatorVersion != nil {
			return e.CustomRPCEvaluatorVersion.ValidateInput(input)
		}
	default:
		return nil
	}
	return nil
}

// ValidateBaseInfo 校验评估器基本信息
func (e *Evaluator) ValidateBaseInfo() error {
	switch e.EvaluatorType {
	case EvaluatorTypePrompt:
		if e.PromptEvaluatorVersion != nil {
			return e.PromptEvaluatorVersion.ValidateBaseInfo()
		}
	case EvaluatorTypeCode:
		if e.CodeEvaluatorVersion != nil {
			return e.CodeEvaluatorVersion.ValidateBaseInfo()
		}
	case EvaluatorTypeCustomRPC:
		if e.CustomRPCEvaluatorVersion != nil {
			return e.CustomRPCEvaluatorVersion.ValidateBaseInfo()
		}
	default:
		return nil
	}
	return nil
}

// SetEvaluatorVersionID 设置评估器版本ID
func (e *Evaluator) SetEvaluatorVersionID(id int64) {
	switch e.EvaluatorType {
	case EvaluatorTypePrompt:
		if e.PromptEvaluatorVersion != nil {
			e.PromptEvaluatorVersion.SetID(id)
		}
	case EvaluatorTypeCode:
		if e.CodeEvaluatorVersion != nil {
			e.CodeEvaluatorVersion.SetID(id)
		}
	case EvaluatorTypeCustomRPC:
		if e.CustomRPCEvaluatorVersion != nil {
			e.CustomRPCEvaluatorVersion.SetID(id)
		}
	default:
		return
	}
}

// SetVersion 设置版本号
func (e *Evaluator) SetVersion(version string) {
	switch e.EvaluatorType {
	case EvaluatorTypePrompt:
		if e.PromptEvaluatorVersion != nil {
			e.PromptEvaluatorVersion.SetVersion(version)
		}
	case EvaluatorTypeCode:
		if e.CodeEvaluatorVersion != nil {
			e.CodeEvaluatorVersion.SetVersion(version)
		}
	case EvaluatorTypeCustomRPC:
		if e.CustomRPCEvaluatorVersion != nil {
			e.CustomRPCEvaluatorVersion.SetVersion(version)
		}
	default:
		return
	}
}

// SetEvaluatorDescription 设置评估器描述
func (e *Evaluator) SetEvaluatorDescription(description string) {
	e.Description = description
}

// SetEvaluatorVersionDescription 设置评估器版本描述
func (e *Evaluator) SetEvaluatorVersionDescription(description string) {
	switch e.EvaluatorType {
	case EvaluatorTypePrompt:
		if e.PromptEvaluatorVersion != nil {
			e.PromptEvaluatorVersion.SetDescription(description)
		}
	case EvaluatorTypeCode:
		if e.CodeEvaluatorVersion != nil {
			e.CodeEvaluatorVersion.SetDescription(description)
		}
	case EvaluatorTypeCustomRPC:
		if e.CustomRPCEvaluatorVersion != nil {
			e.CustomRPCEvaluatorVersion.SetDescription(description)
		}
	default:
		return
	}
}

// SetBaseInfo 设置基础信息
func (e *Evaluator) SetBaseInfo(baseInfo *BaseInfo) {
	switch e.EvaluatorType {
	case EvaluatorTypePrompt:
		if e.PromptEvaluatorVersion != nil {
			e.PromptEvaluatorVersion.SetBaseInfo(baseInfo)
		}
	case EvaluatorTypeCode:
		if e.CodeEvaluatorVersion != nil {
			e.CodeEvaluatorVersion.SetBaseInfo(baseInfo)
		}
	case EvaluatorTypeCustomRPC:
		if e.CustomRPCEvaluatorVersion != nil {
			e.CustomRPCEvaluatorVersion.SetBaseInfo(baseInfo)
		}
	default:
		return
	}
}

// SetTools 设置工具
func (e *Evaluator) SetTools(tools []*Tool) {
	switch e.EvaluatorType {
	case EvaluatorTypePrompt:
		if e.PromptEvaluatorVersion != nil {
			e.PromptEvaluatorVersion.SetTools(tools)
		}
	default:
		return
	}
}

// SetPromptSuffix 设置提示后缀
func (e *Evaluator) SetPromptSuffix(promptSuffix string) {
	switch e.EvaluatorType {
	case EvaluatorTypePrompt:
		if e.PromptEvaluatorVersion != nil {
			e.PromptEvaluatorVersion.SetPromptSuffix(promptSuffix)
		}
	default:
		return
	}
}

// SetParseType 设置解析类型
func (e *Evaluator) SetParseType(parseType ParseType) {
	switch e.EvaluatorType {
	case EvaluatorTypePrompt:
		if e.PromptEvaluatorVersion != nil {
			e.PromptEvaluatorVersion.SetParseType(parseType)
		}
	default:
		return
	}
}

// SetEvaluatorID 设置评估器ID
func (e *Evaluator) SetEvaluatorID(evaluatorID int64) {
	switch e.EvaluatorType {
	case EvaluatorTypePrompt:
		if e.PromptEvaluatorVersion != nil {
			e.PromptEvaluatorVersion.SetEvaluatorID(evaluatorID)
		}
	case EvaluatorTypeCode:
		if e.CodeEvaluatorVersion != nil {
			e.CodeEvaluatorVersion.SetEvaluatorID(evaluatorID)
		}
	case EvaluatorTypeCustomRPC:
		if e.CustomRPCEvaluatorVersion != nil {
			e.CustomRPCEvaluatorVersion.SetEvaluatorID(evaluatorID)
		}
	default:
		return
	}
}

// SetSpaceID 设置空间ID
func (e *Evaluator) SetSpaceID(spaceID int64) {
	switch e.EvaluatorType {
	case EvaluatorTypePrompt:
		if e.PromptEvaluatorVersion != nil {
			e.PromptEvaluatorVersion.SetSpaceID(spaceID)
		}
	case EvaluatorTypeCode:
		if e.CodeEvaluatorVersion != nil {
			e.CodeEvaluatorVersion.SetSpaceID(spaceID)
		}
	case EvaluatorTypeCustomRPC:
		if e.CustomRPCEvaluatorVersion != nil {
			e.CustomRPCEvaluatorVersion.SetSpaceID(spaceID)
		}
	default:
		return
	}
}

func (e *Evaluator) SetEvaluatorVersion(version *Evaluator) {
	switch e.EvaluatorType {
	case EvaluatorTypePrompt:
		e.PromptEvaluatorVersion = version.PromptEvaluatorVersion
	case EvaluatorTypeCode:
		e.CodeEvaluatorVersion = version.CodeEvaluatorVersion
	case EvaluatorTypeCustomRPC:
		e.CustomRPCEvaluatorVersion = version.CustomRPCEvaluatorVersion
	default:
		return
	}
}
