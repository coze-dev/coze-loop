// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package rpc

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

//go:generate mockgen -destination=mocks/prompt.go -package=mocks . IPromptRPCAdapter
type IPromptRPCAdapter interface {
	GetPrompt(ctx context.Context, spaceID, promptID int64, params GetPromptParams) (prompt *LoopPrompt, err error)
	MGetPrompt(ctx context.Context, spaceID int64, promptQueries []*MGetPromptQuery) (prompts []*LoopPrompt, err error)
	ListPrompt(ctx context.Context, param *ListPromptParam) (prompts []*LoopPrompt, total *int32, err error)
	ListPromptVersion(ctx context.Context, param *ListPromptVersionParam) (prompts []*CommitInfo, nextCursor string, err error)
	ExecutePrompt(ctx context.Context, spaceID int64, param *ExecutePromptParam) (result *ExecutePromptResult, err error)
}

type ExecutePromptParam struct {
	PromptID      int64
	PromptVersion string
	Variables     []*entity.VariableVal
	History       []*entity.Message
	RuntimeParam  *string
}

type ExecutePromptResult struct {
	Content    *string            `json:"content,omitempty"`
	ToolCalls  []*entity.ToolCall `json:"tool_calls,omitempty"`
	TokenUsage *entity.TokenUsage `json:"token_usage,omitempty"`
}

type GetPromptParams struct {
	CommitVersion *string
}

type LoopPrompt struct {
	ID           int64
	PromptKey    string
	PromptBasic  *PromptBasic
	PromptCommit *PromptCommit
}

type PromptBasic struct {
	DisplayName   *string
	Description   *string
	LatestVersion *string
}

type PromptCommit struct {
	Detail     *PromptDetail `thrift:"detail,1,optional" frugal:"1,optional,PromptDetail" form:"detail" json:"detail,omitempty" query:"detail"`
	CommitInfo *CommitInfo   `thrift:"commit_info,2,optional" frugal:"2,optional,CommitInfo" form:"commit_info" json:"commit_info,omitempty" query:"commit_info"`
}

type PromptDetail struct {
	PromptTemplate *PromptTemplate
}

type PromptTemplate struct {
	VariableDefs []*VariableDef `thrift:"variable_defs,3,optional" frugal:"3,optional,list<VariableDef>" form:"variable_defs" json:"variable_defs,omitempty" query:"variable_defs"`
}

type VariableDef struct {
	Key      *string
	Type     *VariableType `thrift:"type,3,optional" frugal:"3,optional,string" form:"type" json:"type,omitempty" query:"type"`
	TypeTags []string      `thrift:"type_tags,4,optional" frugal:"4,optional,list<string>" form:"type_tags" json:"type_tags,omitempty" query:"type_tags"`
}

type VariableType = string

const (
	VariableTypeString       = "string"
	VariableTypeBoolean      = "boolean"
	VariableTypeInteger      = "integer"
	VariableTypeFloat        = "float"
	VariableTypeObject       = "object"
	VariableTypeArrayString  = "array<string>"
	VariableTypeArrayBoolean = "array<boolean>"
	VariableTypeArrayInteger = "array<integer>"
	VariableTypeArrayFloat   = "array<float>"
	VariableTypeArrayObject  = "array<object>"

	VariableTypePlaceholder = "placeholder"
)

type CommitInfo struct {
	Version     *string
	BaseVersion *string
	Description *string
	CommittedBy *string
	CommittedAt *int64
}

type PublishStatus int64

const (
	PublishStatus_Undefined PublishStatus = 0
	// 未发布
	PublishStatus_UnPublish PublishStatus = 1
	// 已发布
	PublishStatus_Published PublishStatus = 2
)

type MGetPromptQuery struct {
	PromptID int64
	Version  *string
}

type PromptPublishInfo struct {
	Publisher          string
	PublishDescription string
	PublishTSMS        *int64
}

type ListPromptParam struct {
	SpaceID  *int64
	Page     *int32
	PageSize *int32
	// name/key前缀匹配
	KeyWord *string
}

type ListPromptVersionParam struct {
	PromptID int64
	SpaceID  *int64
	Cursor   *string
	PageSize *int32
}
