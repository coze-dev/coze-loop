// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"io"
	"slices"

	"github.com/google/go-cmp/cmp"
	"github.com/valyala/fasttemplate"

	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

const (
	PromptNormalTemplateStartTag = "{{"
	PromptNormalTemplateEndTag   = "}}"
)

type PromptDetail struct {
	PromptTemplate *PromptTemplate `json:"prompt_template,omitempty"`
	Tools          []*Tool         `json:"tools,omitempty"`
	ToolCallConfig *ToolCallConfig `json:"tool_call_config,omitempty"`
	ModelConfig    *ModelConfig    `json:"model_config,omitempty"`
}

type PromptTemplate struct {
	TemplateType TemplateType   `json:"template_type"`
	Messages     []*Message     `json:"messages,omitempty"`
	VariableDefs []*VariableDef `json:"variable_defs,omitempty"`
}

type TemplateType string

const (
	TemplateTypeNormal TemplateType = "normal"
	TemplateTypeJinja2 TemplateType = "jinja2"  // 新增Jinja2模板类型
)

type Message struct {
	Role             Role           `json:"role"`
	ReasoningContent *string        `json:"reasoning_content,omitempty"`
	Content          *string        `json:"content,omitempty"`
	Parts            []*ContentPart `json:"parts,omitempty"`
	ToolCallID       *string        `json:"tool_call_id,omitempty"`
	ToolCalls        []*ToolCall    `json:"tool_calls,omitempty"`
}

type Role string

const (
	RoleSystem      Role = "system"
	RoleUser        Role = "user"
	RoleAssistant   Role = "assistant"
	RoleTool        Role = "tool"
	RolePlaceholder Role = "placeholder"
)

type ContentPart struct {
	Type     ContentType `json:"type"`
	Text     *string     `json:"text,omitempty"`
	ImageURL *ImageURL   `json:"image_url,omitempty"`
}

type ContentType string

const (
	ContentTypeText     ContentType = "text"
	ContentTypeImageURL ContentType = "image_url"
)

type ImageURL struct {
	URI string `json:"uri"`
	URL string `json:"url"`
}

type VariableDef struct {
	Key  string       `json:"key"`
	Desc string       `json:"desc"`
	Type VariableType `json:"type"`
}

type VariableType string

const (
	VariableTypeString      VariableType = "string"
	VariableTypePlaceholder VariableType = "placeholder"
)

type VariableVal struct {
	Key                 string     `json:"key"`
	Value               *string    `json:"value,omitempty"`
	PlaceholderMessages []*Message `json:"placeholder_messages,omitempty"`
}

type Tool struct {
	Type     ToolType  `json:"type"`
	Function *Function `json:"function,omitempty"`
}

type ToolType string

const (
	ToolTypeFunction ToolType = "function"
)

type Function struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  string `json:"parameters"`
}

type ToolCallConfig struct {
	ToolChoice ToolChoiceType `json:"tool_choice"`
}

type ToolChoiceType string

const (
	ToolChoiceTypeNone ToolChoiceType = "none"
	ToolChoiceTypeAuto ToolChoiceType = "auto"
)

type ToolCall struct {
	Index        int64         `json:"index"`
	ID           string        `json:"id"`
	Type         ToolType      `json:"type"`
	FunctionCall *FunctionCall `json:"function_call,omitempty"`
}

type FunctionCall struct {
	Name      string  `json:"name"`
	Arguments *string `json:"arguments,omitempty"`
}

type ModelConfig struct {
	ModelID          int64    `json:"model_id"`
	MaxTokens        *int32   `json:"max_tokens,omitempty"`
	Temperature      *float64 `json:"temperature,omitempty"`
	TopK             *int32   `json:"top_k,omitempty"`
	TopP             *float64 `json:"top_p,omitempty"`
	PresencePenalty  *float64 `json:"presence_penalty,omitempty"`
	FrequencyPenalty *float64 `json:"frequency_penalty,omitempty"`
	JSONMode         *bool    `json:"json_mode,omitempty"`
}

func (pt *PromptTemplate) formatMessages(messages []*Message, variableVals []*VariableVal) ([]*Message, error) {
	if pt == nil {
		return nil, nil
	}
	messagesToFormat := pt.getTemplateMessages(messages)

	defMap := make(map[string]*VariableDef)
	for _, variableDef := range pt.VariableDefs {
		if variableDef != nil {
			defMap[variableDef.Key] = variableDef
		}
	}
	valMap := make(map[string]*VariableVal)
	for _, variableVal := range variableVals {
		if variableVal != nil {
			valMap[variableVal.Key] = variableVal
		}
	}

	var formattedMessages []*Message
	for _, message := range messagesToFormat {
		if message == nil {
			continue
		}
		switch message.Role {
		case RolePlaceholder:
			if placeholderVal, ok := valMap[ptr.From(message.Content)]; ok && placeholderVal != nil {
				for _, placeholderMessage := range placeholderVal.PlaceholderMessages {
					if placeholderMessage == nil {
						continue
					}
					if !slices.Contains([]Role{RoleSystem, RoleUser, RoleAssistant, RoleTool}, placeholderMessage.Role) {
						return nil, errorx.NewByCode(prompterr.CommonInvalidParamCode)
					}
					formattedMessages = append(formattedMessages, placeholderMessage)
				}
			}
		default:
			if templateStr := ptr.From(message.Content); templateStr != "" {
				formattedStr, err := formatText(pt.TemplateType, templateStr, defMap, valMap)
				if err != nil {
					return nil, err
				}
				message.Content = ptr.Of(formattedStr)
			}
			for _, part := range message.Parts {
				if part.Type == ContentTypeText && ptr.From(part.Text) != "" {
					formattedStr, err := formatText(pt.TemplateType, ptr.From(part.Text), defMap, valMap)
					if err != nil {
						return nil, err
					}
					part.Text = ptr.Of(formattedStr)
				}
			}
			formattedMessages = append(formattedMessages, message)
		}
	}
	return formattedMessages, nil
}

func (pt *PromptTemplate) getTemplateMessages(messages []*Message) []*Message {
	if pt == nil {
		return nil
	}
	var messagesToFormat []*Message
	messagesToFormat = append(messagesToFormat, pt.Messages...)
	messagesToFormat = append(messagesToFormat, messages...)
	return messagesToFormat
}

func formatText(templateType TemplateType, templateStr string, defMap map[string]*VariableDef, valMap map[string]*VariableVal) (string, error) {
	switch templateType {
	case TemplateTypeNormal:
		return fasttemplate.ExecuteFuncString(templateStr, PromptNormalTemplateStartTag, PromptNormalTemplateEndTag,
			func(w io.Writer, tag string) (int, error) {
				// If not in variable definition, don't replace and return directly
				if defMap[tag] == nil {
					return w.Write([]byte(PromptNormalTemplateStartTag + tag + PromptNormalTemplateEndTag))
				}
				// Otherwise replace
				if val, ok := valMap[tag]; ok {
					return w.Write([]byte(ptr.From(val.Value)))
				}
				return 0, nil
			}), nil
	case TemplateTypeJinja2:
		return formatJinja2Text(templateStr, defMap, valMap)
	default:
		return "", errorx.New("unknown template type")
	}
}

// formatJinja2Text 处理Jinja2模板格式化
func formatJinja2Text(templateStr string, defMap map[string]*VariableDef, valMap map[string]*VariableVal) (string, error) {
	engine := NewJinja2Engine()

	// 构建变量映射
	variables := make(map[string]interface{})
	for key, val := range valMap {
		if defMap[key] != nil && val != nil {
			variables[key] = ptr.From(val.Value)
		}
	}

	return engine.Execute(templateStr, variables)
}

func (pd *PromptDetail) DeepEqual(other *PromptDetail) bool {
	return cmp.Equal(pd, other)
}
