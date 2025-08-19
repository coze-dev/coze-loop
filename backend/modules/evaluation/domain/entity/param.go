// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type CreateEvaluationSetParam struct {
	SpaceID             int64
	Name                string
	Description         *string
	EvaluationSetSchema *EvaluationSetSchema
	BizCategory         *BizCategory
	Session             *Session
}

type UpdateEvaluationSetParam struct {
	SpaceID         int64
	EvaluationSetID int64
	Name            *string
	Description     *string
}

type ListEvaluationSetsParam struct {
	SpaceID          int64
	EvaluationSetIDs []int64
	Name             *string
	Creators         []string
	PageNumber       *int32
	PageSize         *int32
	PageToken        *string
	OrderBys         []*OrderBy
}

type ListEvaluationSetItemsParam struct {
	SpaceID         int64
	EvaluationSetID int64
	VersionID       *int64
	PageNumber      *int32
	PageSize        *int32
	PageToken       *string
	OrderBys        []*OrderBy
	ItemIDsNotIn    []int64
}
type BatchGetEvaluationSetItemsParam struct {
	SpaceID         int64
	EvaluationSetID int64
	ItemIDs         []int64
	VersionID       *int64
	PageNumber      *int32
	PageSize        *int32
	PageToken       *string
	OrderBys        []*OrderBy
}

type BatchCreateEvaluationSetItemsParam struct {
	SpaceID         int64
	EvaluationSetID int64
	Items           []*EvaluationSetItem
	// items 中存在无效数据时，默认不会写入任何数据；设置 skipInvalidItems=true 会跳过无效数据，写入有效数据
	SkipInvalidItems *bool
	// 批量写入 items 如果超出数据集容量限制，默认不会写入任何数据；设置 partialAdd=true 会写入不超出容量限制的前 N 条
	AllowPartialAdd *bool
}

type CreateEvaluationSetVersionParam struct {
	SpaceID         int64
	EvaluationSetID int64
	Version         string
	Description     *string
}

type ListEvaluationSetVersionsParam struct {
	SpaceID         int64
	EvaluationSetID int64
	PageToken       *string
	PageSize        *int32
	PageNumber      *int32
	VersionLike     *string
}

type BatchGetEvaluationSetVersionsResult struct {
	Version       *EvaluationSetVersion
	EvaluationSet *EvaluationSet
}

type Option func(option *Opt)

type Opt struct {
	PublishVersion *string
	BotInfoType    CozeBotInfoType
}

func WithCozeBotPublishVersion(publishVersion *string) Option {
	return func(option *Opt) {
		option.PublishVersion = publishVersion
	}
}

func WithCozeBotInfoType(botInfoType CozeBotInfoType) Option {
	return func(option *Opt) {
		option.BotInfoType = botInfoType
	}
}

type ExecuteEvalTargetParam struct {
	TargetID            int64
	VersionID           int64
	SourceTargetID      string
	SourceTargetVersion string
	Input               *EvalTargetInputData
	TargetType          EvalTargetType
}

type ListEvaluatorRequest struct {
	SpaceID       int64           `json:"space_id"`
	SearchName    string          `json:"search_name,omitempty"`
	CreatorIDs    []int64         `json:"creator_ids,omitempty"`
	EvaluatorType []EvaluatorType `json:"evaluator_type,omitempty"`
	PageSize      int32           `json:"page_size,omitempty"`
	PageNum       int32           `json:"page_num,omitempty"`
	OrderBys      []*OrderBy      `json:"order_bys,omitempty"`
	WithVersion   bool            `json:"with_version,omitempty"`
}

type ListEvaluatorVersionRequest struct {
	SpaceID       int64      `json:"space_id"`
	EvaluatorID   int64      `json:"evaluator_id,omitempty"`
	QueryVersions []string   `json:"query_versions,omitempty"`
	PageSize      int32      `json:"page_size,omitempty"`
	PageNum       int32      `json:"page_num,omitempty"`
	OrderBys      []*OrderBy `json:"order_bys,omitempty"`
}

type ListEvaluatorVersionResponse struct {
	EvaluatorVersions []*Evaluator `json:"evaluator_versions,omitempty"`
	Total             int64        `json:"total,omitempty"`
}

type RunEvaluatorRequest struct {
	SpaceID            int64               `json:"space_id"`
	Name               string              `json:"name"`
	EvaluatorVersionID int64               `json:"evaluator_version_id"`
	InputData          *EvaluatorInputData `json:"input_data"`
	ExperimentID       int64               `json:"experiment_id,omitempty"`
	ExperimentRunID    int64               `json:"experiment_run_id,omitempty"`
	ItemID             int64               `json:"item_id,omitempty"`
	TurnID             int64               `json:"turn_id,omitempty"`
	Ext                map[string]string   `json:"ext,omitempty"`
}

type CreateExptParam struct {
	WorkspaceID         int64   `thrift:"workspace_id,1,required" frugal:"1,required,i64" json:"workspace_id" form:"workspace_id,required" `
	EvalSetVersionID    int64   `thrift:"eval_set_version_id,2,optional" frugal:"2,optional,i64" json:"eval_set_version_id" form:"eval_set_version_id" `
	TargetVersionID     int64   `thrift:"target_version_id,3,optional" frugal:"3,optional,i64" json:"target_version_id" form:"target_version_id" `
	EvaluatorVersionIds []int64 `thrift:"evaluator_version_ids,4,optional" frugal:"4,optional,list<i64>" json:"evaluator_version_ids" form:"evaluator_version_ids" `
	Name                string  `thrift:"name,5,optional" frugal:"5,optional,string" form:"name" json:"name,omitempty"`
	Desc                string  `thrift:"desc,6,optional" frugal:"6,optional,string" form:"desc" json:"desc,omitempty"`
	EvalSetID           int64   `thrift:"eval_set_id,7,optional" frugal:"7,optional,i64" json:"eval_set_id" form:"eval_set_id" `
	TargetID            *int64  `thrift:"target_id,8,optional" frugal:"8,optional,i64" json:"target_id" form:"target_id" `
	// TargetFieldMapping    *TargetFieldMapping                `thrift:"target_field_mapping,20,optional" frugal:"20,optional,TargetFieldMapping" form:"target_field_mapping" json:"target_field_mapping,omitempty"`
	// EvaluatorFieldMapping []*EvaluatorFieldMapping           `thrift:"evaluator_field_mapping,21,optional" frugal:"21,optional,list<EvaluatorFieldMapping>" form:"evaluator_field_mapping" json:"evaluator_field_mapping,omitempty"`
	// ItemConcurNum         int32                        `thrift:"item_concur_num,22,optional" frugal:"22,optional,i32" form:"item_concur_num" json:"item_concur_num,omitempty"`
	// EvaluatorsConcurNum   int32                        `thrift:"evaluators_concur_num,23,optional" frugal:"23,optional,i32" form:"evaluators_concur_num" json:"evaluators_concur_num,omitempty"`
	CreateEvalTargetParam *CreateEvalTargetParam `thrift:"create_eval_target_param,24,optional" frugal:"24,optional,eval_target.CreateEvalTargetParam" form:"create_eval_target_param" json:"create_eval_target_param,omitempty"`
	ExptType              ExptType               `thrift:"expt_type,30,optional" frugal:"30,optional,ExptType" form:"expt_type" json:"expt_type,omitempty"`
	MaxAliveTime          int64                  `thrift:"max_alive_time,31,optional" frugal:"31,optional,i64" form:"max_alive_time" json:"max_alive_time,omitempty"`
	SourceType            SourceType             `thrift:"source_type,32,optional" frugal:"32,optional,SourceType" form:"source_type" json:"source_type,omitempty"`
	SourceID              string                 `thrift:"source_id,33,optional" frugal:"33,optional,string" form:"source_id" json:"source_id,omitempty"`

	ExptConf *EvaluationConfiguration
}

type ExptRunCheckOption struct {
	CheckBenefit bool
}

type ExptRunCheckOptionFn func(*ExptRunCheckOption)

func WithCheckBenefit() ExptRunCheckOptionFn {
	return func(e *ExptRunCheckOption) {
		e.CheckBenefit = true
	}
}

type CompleteExptOption struct {
	Status        ExptStatus
	StatusMessage string
	CID           string
}

type CompleteExptOptionFn func(*CompleteExptOption)

func WithStatus(status ExptStatus) CompleteExptOptionFn {
	return func(c *CompleteExptOption) {
		c.Status = status
	}
}

func WithStatusMessage(msg string) CompleteExptOptionFn {
	return func(c *CompleteExptOption) {
		const maxLen = 200
		if len(msg) > maxLen {
			msg = msg[:maxLen]
		}
		c.StatusMessage = msg
	}
}

func WithCID(cid string) CompleteExptOptionFn {
	return func(c *CompleteExptOption) {
		c.CID = cid
	}
}

type GetExptTupleOption struct {
	WithoutDeleted bool
}

type GetExptTupleOptionFn func(*GetExptTupleOption)

func WithoutTupleDeleted() GetExptTupleOptionFn {
	return func(c *GetExptTupleOption) {
		c.WithoutDeleted = true
	}
}

type BatchGetEvalTargetBySourceParam struct {
	SpaceID        int64
	SourceTargetID []string
	TargetType     EvalTargetType
}

type ListSourceParam struct {
	TargetType EvalTargetType
	SpaceID    *int64
	PageSize   *int32
	Cursor     *string
	KeyWord    *string
}

type ListSourceVersionParam struct {
	TargetType     EvalTargetType
	SpaceID        *int64
	PageSize       *int32
	Cursor         *string
	SourceTargetID string
}

type LLMCallParam struct {
	SpaceID     int64
	EvaluatorID string
	UserID      *string

	Scenario       Scenario
	Messages       []*Message
	Tools          []*Tool
	ToolCallConfig *ToolCallConfig
	ModelConfig    *ModelConfig
}
