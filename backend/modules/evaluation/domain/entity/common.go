// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

// ContentType 定义内容类型
type ContentType string

const (
	ContentTypeText  ContentType = "Text"
	ContentTypeImage ContentType = "Image"
	ContentTypeAudio ContentType = "Audio"

	ContentTypeMultipart ContentType = "MultiPart"
)

// Image 图片结构体
type Image struct {
	Name     *string `json:"name,omitempty"`
	URL      *string `json:"url,omitempty"`
	URI      *string `json:"uri,omitempty"`
	ThumbURL *string `json:"thumb_url,omitempty"`
}

// Content 内容结构体
type Content struct {
	ContentType *ContentType        `json:"content_type,omitempty"`
	Format      *FieldDisplayFormat `json:"format,omitempty"` // 假设 datasetv2.FieldDisplayFormat 为 interface{}
	Text        *string             `json:"text,omitempty"`
	Image       *Image              `json:"image,omitempty"`
	MultiPart   []*Content          `json:"multi_part,omitempty"`
	Audio       *Audio              `json:"audio,omitempty"`
}

// GetText 获取内容文本
func (c *Content) GetText() string {
	if c == nil || c.Text == nil {
		return ""
	}
	return *c.Text
}

// SetText 设置内容文本
func (c *Content) SetText(text string) {
	if c != nil {
		c.Text = &text
	}
}

// GetContentType 获取内容类型
func (c *Content) GetContentType() ContentType {
	if c == nil || c.ContentType == nil {
		return ""
	}
	return *c.ContentType
}

// SetContentType 设置内容类型
func (c *Content) SetContentType(contentType ContentType) {
	if c != nil {
		c.ContentType = &contentType
	}
}

type Audio struct {
	Format *string `json:"format,omitempty"`
	URL    *string `json:"url,omitempty"`
}

// OrderBy 排序结构体
type OrderBy struct {
	Field *string `json:"field,omitempty"`
	IsAsc *bool   `json:"is_asc,omitempty"`
}

const (
	OrderByCreatedAt = "created_at"
	OrderByUpdatedAt = "updated_at"
)

var OrderBySet = map[string]struct{}{
	OrderByCreatedAt: {},
	OrderByUpdatedAt: {},
}

// Role 角色枚举
type Role int64

const (
	RoleUndefined Role = 0
	RoleSystem    Role = 1
	RoleUser      Role = 2
	RoleAssistant Role = 3
	RoleTool      Role = 4
)

// Message 消息结构体
type Message struct {
	Role    Role              `json:"role,omitempty"`
	Content *Content          `json:"content,omitempty"`
	Ext     map[string]string `json:"ext,omitempty"`
}

type VariableVal struct {
	Key                 *string    `json:"key,omitempty"`
	Value               *string    `json:"value,omitempty"`
	PlaceholderMessages []*Message `json:"placeholderMessages,omitempty"`
}

// ArgsSchema 参数模式结构体
type ArgsSchema struct {
	Key                 *string       `json:"key,omitempty"`
	SupportContentTypes []ContentType `json:"support_content_types,omitempty"`
	JsonSchema          *string       `json:"json_schema,omitempty"`
}

// UserInfo 用户信息结构体
type UserInfo struct {
	Name        *string `json:"name,omitempty"`
	EnName      *string `json:"en_name,omitempty"`
	AvatarURL   *string `json:"avatar_url,omitempty"`
	AvatarThumb *string `json:"avatar_thumb,omitempty"`
	OpenID      *string `json:"open_id,omitempty"`
	UnionID     *string `json:"union_id,omitempty"`
	UserID      *string `json:"user_id,omitempty"`
	Email       *string `json:"email,omitempty"`
}

// BaseInfo 基础信息结构体
type BaseInfo struct {
	CreatedBy *UserInfo `json:"created_by,omitempty"`
	UpdatedBy *UserInfo `json:"updated_by,omitempty"`
	CreatedAt *int64    `json:"created_at,omitempty"`
	UpdatedAt *int64    `json:"updated_at,omitempty"`
	DeletedAt *int64    `json:"deleted_at,omitempty"`
}

func (do *BaseInfo) GetCreatedBy() *UserInfo {
	return do.CreatedBy
}

func (do *BaseInfo) SetCreatedBy(createdBy *UserInfo) {
	do.CreatedBy = createdBy
}

func (do *BaseInfo) GetUpdatedBy() *UserInfo {
	return do.UpdatedBy
}

func (do *BaseInfo) SetUpdatedBy(updatedBy *UserInfo) {
	do.UpdatedBy = updatedBy
}

func (do *BaseInfo) SetUpdatedAt(updatedAt *int64) {
	do.UpdatedAt = updatedAt
}

// Provider 模型提供方枚举
type Provider int64

const (
	GPTOpenAPI Provider = 1
	Maas       Provider = 2
	BotEngine  Provider = 3
	Merlin     Provider = 4
	MerlinSeed Provider = 5
)

// ModelClass 模型系列枚举
type ModelClass int64

const (
	Undefined ModelClass = 0
	GPT       ModelClass = 1
	SEED      ModelClass = 2
	Gemini    ModelClass = 3
	Claude    ModelClass = 4
	Ernie     ModelClass = 5
	Baichuan  ModelClass = 6
	Qwen      ModelClass = 7
	GML       ModelClass = 8
	DeepSeek  ModelClass = 9
)

const (
	// PlainText 表示纯文本格式
	PlainText FieldDisplayFormat = iota + 1
	// Markdown 表示 Markdown 格式
	Markdown
	// JSON 表示 JSON 格式
	JSON
	// YAML 表示 YAML 格式
	YAML
	// Code 表示代码格式
	Code
)

type Tool struct {
	Type     ToolType  `json:"type"`
	Function *Function `json:"function,omitempty"`
}

type ToolType int64

const (
	ToolTypeFunction ToolType = 1
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
	ToolChoiceTypeNone     ToolChoiceType = "none"
	ToolChoiceTypeAuto     ToolChoiceType = "auto"
	ToolChoiceTypeRequired ToolChoiceType = "required"
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
	ModelID     int64          `json:"model_id"`
	ModelName   string         `json:"model_name"`
	MaxTokens   *int32         `json:"max_tokens,omitempty"`
	Temperature *float64       `json:"temperature,omitempty"`
	TopP        *float64       `json:"top_p,omitempty"`
	ToolChoice  ToolChoiceType `json:"tool_choice"`

	ProviderModelID *string `json:"provider_model_id,omitempty"`
}

type Reply struct {
	Item          *ReplyItem `json:"item,omitempty"`
	DebugID       int64      `json:"debug_id"`
	DebugStep     int32      `json:"debug_step"`
	DebugTraceKey string     `json:"debug_trace_key"`
}

type ReplyItem struct {
	Content          *string     `json:"content,omitempty"`
	ReasoningContent *string     `json:"reasoning_content,omitempty"`
	ToolCalls        []*ToolCall `json:"tool_calls,omitempty"`
	FinishReason     string      `json:"finish_reason"`
	TokenUsage       *TokenUsage `json:"token_usage,omitempty"`
}

type TokenUsage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

type Scenario string

const (
	ScenarioDefault    Scenario = "default"
	ScenarioEvalTarget Scenario = "eval_target"
	ScenarioEvaluator  Scenario = "evaluator"
)

type ParseType string

const (
	ParseTypeFunctionCall ParseType = "function_call"
	ParseTypeContent      ParseType = "content"
)

type ScoreType int64

const (
	ScoreTypeRange ScoreType = 1
	ScoreTypeEnum  ScoreType = 2
)
