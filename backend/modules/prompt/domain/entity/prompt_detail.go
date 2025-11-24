// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/google/go-cmp/cmp"
	"github.com/valyala/fasttemplate"

	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/template"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

const (
	PromptNormalTemplateStartTag = "{{"
	PromptNormalTemplateEndTag   = "}}"
)

type PromptDetail struct {
	PromptTemplate *PromptTemplate   `json:"prompt_template,omitempty"`
	Tools          []*Tool           `json:"tools,omitempty"`
	ToolCallConfig *ToolCallConfig   `json:"tool_call_config,omitempty"`
	ModelConfig    *ModelConfig      `json:"model_config,omitempty"`
	ExtInfos       map[string]string `json:"ext_infos,omitempty"`
}

type PromptTemplate struct {
	TemplateType TemplateType   `json:"template_type"`
	Messages     []*Message     `json:"messages,omitempty"`
	VariableDefs []*VariableDef `json:"variable_defs,omitempty"`

	HasSnippets bool              `json:"has_snippets"`
	Snippets    []*Prompt         `json:"snippets,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type TemplateType string

const (
	TemplateTypeNormal          TemplateType = "normal"
	TemplateTypeJinja2          TemplateType = "jinja2"
	TemplateTypeGoTemplate      TemplateType = "go_template"
	TemplateTYpeCustomTemplateM TemplateType = "custom_template_m"
)

type Message struct {
	Role             Role           `json:"role"`
	ReasoningContent *string        `json:"reasoning_content,omitempty"`
	Content          *string        `json:"content,omitempty"`
	Parts            []*ContentPart `json:"parts,omitempty"`
	ToolCallID       *string        `json:"tool_call_id,omitempty"`
	ToolCalls        []*ToolCall    `json:"tool_calls,omitempty"`

	Metadata map[string]string `json:"metadata,omitempty"`
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
	Type        ContentType  `json:"type"`
	Text        *string      `json:"text,omitempty"`
	ImageURL    *ImageURL    `json:"image_url,omitempty"`
	VideoURL    *VideoURL    `json:"video_url,omitempty"`
	Base64Data  *string      `json:"base64_data,omitempty"`
	MediaConfig *MediaConfig `json:"media_config,omitempty"`
}

type ContentType string

const (
	ContentTypeText              ContentType = "text"
	ContentTypeImageURL          ContentType = "image_url"
	ContentTypeVideoURL          ContentType = "video_url"
	ContentTypeBase64Data        ContentType = "base64_data"
	ContentTypeMultiPartVariable ContentType = "multi_part_variable"
)

type ImageURL struct {
	URI string `json:"uri"`
	URL string `json:"url"`
}

type VideoURL struct {
	URI string `json:"uri"`
	URL string `json:"url"`
}

type MediaConfig struct {
	Fps *float64 `json:"fps,omitempty"`
}

type VariableDef struct {
	Key      string       `json:"key"`
	Desc     string       `json:"desc"`
	Type     VariableType `json:"type"`
	TypeTags []string     `json:"type_tags,omitempty"`
}

type VariableType string

const (
	VariableTypeString       VariableType = "string"
	VariableTypePlaceholder  VariableType = "placeholder"
	VariableTypeBoolean      VariableType = "boolean"
	VariableTypeInteger      VariableType = "integer"
	VariableTypeFloat        VariableType = "float"
	VariableTypeObject       VariableType = "object"
	VariableTypeArrayString  VariableType = "array<string>"
	VariableTypeArrayBoolean VariableType = "array<boolean>"
	VariableTypeArrayInteger VariableType = "array<integer>"
	VariableTypeArrayFloat   VariableType = "array<float>"
	VariableTypeArrayObject  VariableType = "array<object>"
	VariableTypeMultiPart    VariableType = "multi_part"
)

type VariableVal struct {
	Key                 string         `json:"key"`
	Value               *string        `json:"value,omitempty"`
	PlaceholderMessages []*Message     `json:"placeholder_messages,omitempty"`
	MultiPartValues     []*ContentPart `json:"multi_part_values,omitempty"`
}

type Tool struct {
	Type     ToolType  `json:"type"`
	Function *Function `json:"function,omitempty"`
}

type ToolType string

const (
	ToolTypeFunction     ToolType = "function"
	ToolTypeGoogleSearch ToolType = "google_search"
)

type Function struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  string `json:"parameters"`
}

type ToolCallConfig struct {
	ToolChoice              ToolChoiceType           `json:"tool_choice"`
	ToolChoiceSpecification *ToolChoiceSpecification `json:"tool_choice_specification,omitempty"`
}

type ToolChoiceType string

const (
	ToolChoiceTypeNone     ToolChoiceType = "none"
	ToolChoiceTypeAuto     ToolChoiceType = "auto"
	ToolChoiceTypeSpecific ToolChoiceType = "specific"
)

type ToolChoiceSpecification struct {
	Type ToolType `json:"type"`
	Name string   `json:"name"`
}

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
	ModelID           int64               `json:"model_id"`
	MaxTokens         *int32              `json:"max_tokens,omitempty"`
	Temperature       *float64            `json:"temperature,omitempty"`
	TopK              *int32              `json:"top_k,omitempty"`
	TopP              *float64            `json:"top_p,omitempty"`
	PresencePenalty   *float64            `json:"presence_penalty,omitempty"`
	FrequencyPenalty  *float64            `json:"frequency_penalty,omitempty"`
	JSONMode          *bool               `json:"json_mode,omitempty"`
	Extra            *string  `json:"extra,omitempty"`
	ParamConfigValues []*ParamConfigValue `json:"param_config_values,omitempty"`
}

type ParamConfigValue struct {
	Name  string       `json:"name"`
	Label string       `json:"label"`
	Value *ParamOption `json:"value,omitempty"`
}

type ParamOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
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
			message.Parts = formatMultiPart(message.Parts, defMap, valMap)
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

func formatMultiPart(parts []*ContentPart, defMap map[string]*VariableDef, valMap map[string]*VariableVal) []*ContentPart {
	var formatedParts []*ContentPart
	for _, part := range parts {
		if part.Type == ContentTypeMultiPartVariable && ptr.From(part.Text) != "" {
			multiPartVariableKey := ptr.From(part.Text)
			if vardef, ok := defMap[multiPartVariableKey]; ok {
				if value, ok := valMap[multiPartVariableKey]; ok {
					if vardef != nil && value != nil && vardef.Type == VariableTypeMultiPart {
						formatedParts = append(formatedParts, value.MultiPartValues...)
					}
				}
			}
		} else {
			formatedParts = append(formatedParts, part)
		}
	}
	var filtered []*ContentPart
	for _, pt := range formatedParts {
		if pt == nil {
			continue
		}
		if ptr.From(pt.Text) != "" || pt.ImageURL != nil || pt.VideoURL != nil || ptr.From(pt.Base64Data) != "" {
			filtered = append(filtered, pt)
		}
	}
	return filtered
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
		return renderJinja2Template(templateStr, defMap, valMap)
	case TemplateTypeGoTemplate:
		return renderGoTemplate(templateStr, defMap, valMap)
	default:
		return "", errorx.NewByCode(prompterr.UnsupportedTemplateTypeCode, errorx.WithExtraMsg("unknown template type: "+string(templateType)))
	}
}

// renderJinja2Template 渲染 Jinja2 模板
func renderJinja2Template(templateStr string, defMap map[string]*VariableDef, valMap map[string]*VariableVal) (string, error) {
	// 转换变量为 map[string]any 格式
	variables, err := convertVariablesToMap(defMap, valMap)
	if err != nil {
		return "", err
	}

	return template.InterpolateJinja2(templateStr, variables)
}

// renderGoTemplate 渲染 Go Template 模板
func renderGoTemplate(templateStr string, defMap map[string]*VariableDef, valMap map[string]*VariableVal) (string, error) {
	// 转换变量为 map[string]any 格式
	variables, err := convertVariablesToMap(defMap, valMap)
	if err != nil {
		return "", err
	}

	return template.InterpolateGoTemplate(templateStr, variables)
}

// convertVariablesToMap 将变量定义和变量值转换为模板引擎可用的 map
func convertVariablesToMap(defMap map[string]*VariableDef, valMap map[string]*VariableVal) (map[string]any, error) {
	if len(defMap) == 0 || len(valMap) == 0 {
		return nil, nil
	}

	result := make(map[string]any)

	// 遍历变量值
	for key, v := range valMap {
		if v == nil || v.Value == nil || ptr.From(v.Value) == "" {
			continue
		}

		// 查找对应的变量定义
		if def, ok := defMap[key]; ok {
			switch def.Type {
			case VariableTypeBoolean:
				result[key] = ptr.From(v.Value) == "true"

			case VariableTypeInteger:
				valueStr := ptr.From(v.Value)
				vInt64, err := strconv.ParseInt(valueStr, 10, 64) // 解析为 int64
				if err != nil {
					return nil, errorx.NewByCode(prompterr.CommonInvalidParamCode,
						errorx.WithExtraMsg(fmt.Sprintf("parse variable %s error with type:%s, value:%s",
							v.Key, def.Type, json.Jsonify(v))))
				}
				result[key] = vInt64

			case VariableTypeFloat:
				valueStr := ptr.From(v.Value)
				vFloat64, err := strconv.ParseFloat(valueStr, 64) // 解析为 float64
				if err != nil {
					return nil, errorx.NewByCode(prompterr.CommonInvalidParamCode,
						errorx.WithExtraMsg(fmt.Sprintf("parse variable %s error with type:%s, value:%s",
							v.Key, def.Type, json.Jsonify(v))))
				}
				result[key] = vFloat64

			case VariableTypeArrayString:
				var vArray []string
				err := Decode(&vArray, def, v)
				if err != nil {
					return nil, err
				}
				result[key] = vArray

			case VariableTypeArrayBoolean:
				var vArray []bool
				err := Decode(&vArray, def, v)
				if err != nil {
					return nil, err
				}
				result[key] = vArray

			case VariableTypeArrayInteger:
				var vArray []int64
				err := Decode(&vArray, def, v)
				if err != nil {
					return nil, err
				}
				result[key] = vArray

			case VariableTypeArrayFloat:
				var vArray []float64
				err := Decode(&vArray, def, v)
				if err != nil {
					return nil, err
				}
				result[key] = vArray

			case VariableTypeObject, VariableTypeArrayObject:
				var vAny interface{}
				err := Decode(&vAny, def, v)
				if err != nil {
					return nil, err
				}
				result[key] = vAny

			default:
				result[key] = ptr.From(v.Value)
			}
		}
	}

	return result, nil
}

func (pd *PromptDetail) DeepEqual(other *PromptDetail) bool {
	return cmp.Equal(pd, other)
}

func Decode(vAny interface{}, def *VariableDef, v *VariableVal) error {
	decoder := json.NewDecoder(strings.NewReader(ptr.From(v.Value)))
	if err := decoder.Decode(&vAny); err != nil {
		return errorx.WrapByCode(err, prompterr.CommonInvalidParamCode,
			errorx.WithExtraMsg(fmt.Sprintf("parse variable %s error with type:%s, value:%s",
				v.Key, def.Type, json.Jsonify(v))))
	}
	return nil
}
