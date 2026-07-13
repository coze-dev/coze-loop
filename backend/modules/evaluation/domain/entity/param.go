// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"time"
)

type CreateEvaluationSetParam struct {
	SpaceID             int64
	Name                string
	Description         *string
	EvaluationSetSchema *EvaluationSetSchema
	BizCategory         *BizCategory
	Session             *Session
	DatasetType         *string
	Tags                []*ResourceTagRef
	DatasetKey          *string
}

type CreateEvaluationSetWithImportParam struct {
	SpaceID             int64
	Name                string
	Description         *string
	EvaluationSetSchema *EvaluationSetSchema
	BizCategory         *BizCategory
	SourceType          *SetSourceType
	Source              *DatasetIOEndpoint
	FieldMappings       []*FieldMapping
	Session             *Session
	Option              *DatasetIOJobOption
	DatasetType         *string
}

type UpdateEvaluationSetParam struct {
	SpaceID         int64
	EvaluationSetID int64
	Name            *string
	Description     *string
	Tags            []*ResourceTagRef
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
	TagFilter        *TagFilter
	DatasetKeys      []string
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
	Filter          *Filter
	TagFilter       *TagFilter
}
type BatchGetEvaluationSetItemsParam struct {
	SpaceID            int64
	EvaluationSetID    int64
	ItemIDs            []int64
	ItemVersionQueries []*EvaluationItemVersionRef
	VersionID          *int64
	PageNumber         *int32
	PageSize           *int32
	PageToken          *string
	OrderBys           []*OrderBy
	Filter             *Filter
	TagFilter          *TagFilter
}

type GetEvaluationSetItemFieldParam struct {
	SpaceID         int64
	EvaluationSetID int64
	// item 的主键ID，即 item.ID 这一字段
	ItemPK int64
	// 列名
	FieldName string
	// 列的唯一键，用于精确查找
	FieldKey *string
	// 当 item 为多轮时，必须提供
	TurnID *int64
}

type BatchCreateEvaluationSetItemsParam struct {
	SpaceID         int64
	EvaluationSetID int64
	Items           []*EvaluationSetItem
	// items 中存在无效数据时，默认不会写入任何数据；设置 skipInvalidItems=true 会跳过无效数据，写入有效数据
	SkipInvalidItems *bool
	// 批量写入 items 如果超出数据集容量限制，默认不会写入任何数据；设置 partialAdd=true 会写入不超出容量限制的前 N 条
	AllowPartialAdd   *bool
	FieldWriteOptions []*FieldWriteOption
}

type BatchUpdateEvaluationSetItemsParam struct {
	SpaceID         int64
	EvaluationSetID int64
	Items           []*EvaluationSetItem
	// items 中存在无效数据时，默认不会写入任何数据；设置 skipInvalidItems=true 会跳过无效数据，写入有效数据
	SkipInvalidItems  *bool
	FieldWriteOptions []*FieldWriteOption
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
	Versions        []string // 精确查询
}

type BatchGetEvaluationSetVersionsResult struct {
	Version       *EvaluationSetVersion
	EvaluationSet *EvaluationSet
}

type Option func(option *Opt)

type Opt struct {
	PublishVersion       *string
	BotInfoType          CozeBotInfoType
	CustomEvalTarget     *CustomEvalTarget
	Region               *Region
	Env                  *string
	OperationInstruction *string
	Cluster              *string
	AgentConnection      *AgentConnection
	SandboxAgent         *SandboxAgent
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

func WithCustomEvalTarget(customTarget *CustomEvalTarget) Option {
	return func(option *Opt) {
		option.CustomEvalTarget = customTarget
	}
}

func WithRegion(region *Region) Option {
	return func(option *Opt) {
		option.Region = region
	}
}

func WithEnv(env *string) Option {
	return func(option *Opt) {
		option.Env = env
	}
}

func WithOperationInstruction(operationInstruction *string) Option {
	return func(option *Opt) {
		option.OperationInstruction = operationInstruction
	}
}

func WithCluster(cluster *string) Option {
	return func(option *Opt) {
		option.Cluster = cluster
	}
}

func WithAgentConnection(agentConnection *AgentConnection) Option {
	return func(option *Opt) {
		option.AgentConnection = agentConnection
	}
}

func WithSandboxAgent(sandboxAgent *SandboxAgent) Option {
	return func(option *Opt) {
		option.SandboxAgent = sandboxAgent
	}
}

type ExecuteEvalTargetParam struct {
	ExptID              int64
	TargetID            int64
	VersionID           int64
	SourceTargetID      string
	SourceTargetVersion string
	Input               *EvalTargetInputData
	TargetType          EvalTargetType
	EvalTarget          *EvalTarget // 透传，各个评测对象如需额外信息可以从这里消费
	EvalSetItemID       *int64
	EvalSetTurnID       *int64
	// LogID 当前调用链的 log id, 供评测对象 (如 SandboxAgent) 透传到外部执行侧做端到端定位。
	LogID string
	// ItemMeta 评测集/条目元数据快照 (评测集 id/名称/版本, item id/key/version 等), 供评测对象透传给外部执行侧。
	ItemMeta *EvalSetItemMeta
}

// EvalSetItemMeta 承载评测集与 item 层面的元数据快照, 用于评测对象 (如 SandboxAgent) 透传给外部执行侧。
// id 字段统一使用 string, 避免 JSON 序列化后被 JS 等消费方按 number 解析导致精度丢失。
// 字段可能为空 (旧数据集无 item 版本、调试场景无实验等), 使用方需容忍缺省值。
type EvalSetItemMeta struct {
	EvalSetID        string `json:"eval_set_id,omitempty"`
	EvalSetName      string `json:"eval_set_name,omitempty"`
	EvalSetVersionID string `json:"eval_set_version_id,omitempty"`
	EvalSetVersion   string `json:"eval_set_version,omitempty"`
	ItemID           string `json:"item_id,omitempty"`
	ItemKey          string `json:"item_key,omitempty"`
	ItemVersionID    string `json:"item_version_id,omitempty"`
	ItemVersion      string `json:"item_version,omitempty"`
}

type ListEvaluatorRequest struct {
	SpaceID       int64                  `json:"space_id"`
	SearchName    string                 `json:"search_name,omitempty"`
	CreatorIDs    []int64                `json:"creator_ids,omitempty"`
	EvaluatorType []EvaluatorType        `json:"evaluator_type,omitempty"`
	FilterOption  *EvaluatorFilterOption `json:"filter_option,omitempty"` // 标签筛选条件
	PageSize      int32                  `json:"page_size,omitempty"`
	PageNum       int32                  `json:"page_num,omitempty"`
	OrderBys      []*OrderBy             `json:"order_bys,omitempty"`
	WithVersion   bool                   `json:"with_version,omitempty"`
}

type ListBuiltinEvaluatorRequest struct {
	FilterOption *EvaluatorFilterOption `json:"filter_option,omitempty"` // 标签筛选条件
	PageSize     int32                  `json:"page_size,omitempty"`
	PageNum      int32                  `json:"page_num,omitempty"`
	WithVersion  bool                   `json:"with_version,omitempty"`
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
	DisableTracing     bool                `json:"disable_tracing,omitempty"`
	EvaluatorRunConf   *EvaluatorRunConfig `json:"evaluator_run_conf,omitempty"`
	// ★ alias 多实例: 同 evaluator_version 不同 alias 区分实例; 空串=默认实例
	Alias string `json:"alias,omitempty"`
	// ★ Builtin (含别名) / Inline 来源标记; 0/默认按 Builtin 处理
	SourceType EvaluatorRecordSourceType `json:"source_type,omitempty"`
}

type AsyncRunEvaluatorRequest struct {
	SpaceID            int64               `json:"space_id"`
	Name               string              `json:"name"`
	EvaluatorVersionID int64               `json:"evaluator_version_id"`
	InputData          *EvaluatorInputData `json:"input_data"`
	ExperimentID       int64               `json:"experiment_id,omitempty"`
	ExperimentRunID    int64               `json:"experiment_run_id,omitempty"`
	ItemID             int64               `json:"item_id,omitempty"`
	TurnID             int64               `json:"turn_id,omitempty"`
	Ext                map[string]string   `json:"ext,omitempty"`
	EvaluatorRunConf   *EvaluatorRunConfig `json:"evaluator_run_conf,omitempty"`
	// ★ alias 多实例: 同步与 RunEvaluatorRequest
	Alias      string                    `json:"alias,omitempty"`
	SourceType EvaluatorRecordSourceType `json:"source_type,omitempty"`
}

type AsyncRunEvaluatorResponse struct {
	InvokeID int64 `json:"invoke_id"`
}

type AsyncDebugEvaluatorRequest struct {
	SpaceID          int64               `json:"space_id"`
	EvaluatorDO      *Evaluator          `json:"evaluator_do"`
	InputData        *EvaluatorInputData `json:"input_data"`
	EvaluatorRunConf *EvaluatorRunConfig `json:"evaluator_run_conf,omitempty"`
}

type AsyncDebugEvaluatorResponse struct {
	InvokeID int64  `json:"invoke_id"`
	TraceID  string `json:"trace_id"`
}

type GetAsyncDebugEvaluatorInvokeResultRequest struct {
	SpaceID  int64 `json:"space_id"`
	InvokeID int64 `json:"invoke_id"`
}

type GetAsyncDebugEvaluatorInvokeResultResponse struct {
	SpaceID     int64                `json:"space_id"`
	Status      EvaluatorRunStatus   `json:"status"`
	OutputData  *EvaluatorOutputData `json:"output_data,omitempty"`
	EvaluatorDO *Evaluator           `json:"evaluator_do,omitempty"`
	InputData   *EvaluatorInputData  `json:"input_data,omitempty"`
}

type ReportEvaluatorRecordParam struct {
	SpaceID    int64                `json:"space_id"`
	RecordID   int64                `json:"record_id"`
	OutputData *EvaluatorOutputData `json:"output_data,omitempty"`
	Status     EvaluatorRunStatus   `json:"status"`
}

// UpdateRunConfParam 修改进行中实验运行配置的参数。
// ItemConcurNum / ItemRetryNum 为 nil 表示该字段不修改。
type UpdateRunConfParam struct {
	ExptID        int64
	SpaceID       int64
	ItemConcurNum *int
	ItemRetryNum  *int
	Session       *Session
}

type CreateExptParam struct {
	WorkspaceID           int64                    `json:"workspace_id"`
	EvalSetVersionID      int64                    `json:"eval_set_version_id"`
	TargetVersionID       int64                    `json:"target_version_id"`
	EvaluatorVersionIds   []int64                  `json:"evaluator_version_ids"`
	Name                  string                   `json:"name"`
	Desc                  string                   `json:"desc"`
	ExperimentGroupKey    string                   `json:"experiment_group_key,omitempty"`
	EvalSetID             int64                    `json:"eval_set_id"`
	TargetID              *int64                   `json:"target_id,omitempty"`
	CreateEvalTargetParam *CreateEvalTargetParam   `json:"create_eval_target_param,omitempty"`
	ExptType              ExptType                 `json:"expt_type"`
	MaxAliveTime          int64                    `json:"max_alive_time"`
	SourceType            SourceType               `json:"source_type"`
	SourceID              string                   `json:"source_id"`
	Visibility            *Visibility              `json:"visibility,omitempty"`
	ThreadID              *string                  `json:"thread_id,omitempty"`
	ExptTemplateID        int64                    `json:"expt_template_id"`
	ExptConf              *EvaluationConfiguration `json:"expt_conf"`
	ItemRetryNum          *int                     `json:"item_retry_num,omitempty"`
	TrialRunItemCount     int64                    `json:"trial_run_item_count"`
	TriggerType           string                   `json:"trigger_type,omitempty"`
	// ★ 新增: 多评测集配置 (MultiSetConfig 新路径权威源)
	EvalSetConfigs []*EvalSetConfig `json:"eval_set_configs,omitempty"`
	// ★ 新增: 分流依据 (== MultiSetConfig 走新路径), 由 request 透传, 不再从 len(EvalSetConfigs) 派生
	EvalSetSourceType ExptEvalSetSourceType `json:"eval_set_source_type"`
	NotificationConf  *ExptNotificationConf `json:"notification_conf,omitempty"`
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
	Status             ExptStatus
	StatusMessage      string
	CID                string
	CompleteInterval   time.Duration
	NoAggrCalculate    bool
	NoCompleteItemTurn bool
}

type CompleteExptOptionFn func(*CompleteExptOption)

func NoAggrCalculate() CompleteExptOptionFn {
	return func(c *CompleteExptOption) {
		c.NoAggrCalculate = true
	}
}

func NoCompleteItemTurn() CompleteExptOptionFn {
	return func(c *CompleteExptOption) {
		c.NoCompleteItemTurn = true
	}
}

func WithCompleteInterval(interval time.Duration) CompleteExptOptionFn {
	return func(c *CompleteExptOption) {
		c.CompleteInterval = interval
	}
}

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

type GetEvaluatorRecordOption struct {
	WithoutLoadStorageData bool
}

type GetEvaluatorRecordOptionFn func(*GetEvaluatorRecordOption)

func WithoutLoadStorageData() GetEvaluatorRecordOptionFn {
	return func(o *GetEvaluatorRecordOption) {
		o.WithoutLoadStorageData = true
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

type SearchCustomEvalTargetParam struct {
	WorkspaceID     *int64
	Keyword         *string
	ApplicationID   *int64
	CustomRPCServer *CustomRPCServer
	Region          *Region
	Env             *string
	PageSize        *int32
	PageToken       *string
}

type ReportTargetRecordParam struct {
	SpaceID    int64
	RecordID   int64
	Status     EvalTargetRunStatus
	OutputData *EvalTargetOutputData

	Session                 *Session
	EnableExtractTrajectory *bool
	// AsyncUnixMS 异步评测对象「请求发起」的时间(unix ms),即提交异步调用之前的时刻。
	// 用作抽取 trajectory 的时间下界:record.BaseInfo.CreatedAt 是异步返回后才 stamp 的,偏晚会漏掉
	// 请求发起到返回之间的 span。为 0 时回退到 record.BaseInfo.CreatedAt。
	AsyncUnixMS int64
}

type DebugTargetParam struct {
	SpaceID              int64
	PatchyTarget         *EvalTarget
	InputData            *EvalTargetInputData
	TruncateLargeContent *bool // 是否对大对象剪裁，nil 时默认剪裁
}

// CreateEvaluatorTemplateRequest 创建评估器模板请求
type CreateEvaluatorTemplateRequest struct {
	SpaceID            int64                                                 `json:"space_id" validate:"required,gt=0"`      // 空间ID
	Name               string                                                `json:"name" validate:"required,min=1,max=100"` // 模板名称
	Description        string                                                `json:"description" validate:"max=500"`         // 模板描述
	EvaluatorType      EvaluatorType                                         `json:"evaluator_type" validate:"required"`     // 评估器类型
	EvaluatorInfo      *EvaluatorInfo                                        `json:"evaluator_info,omitempty"`               // 评估器补充信息
	InputSchemas       []*ArgsSchema                                         `json:"input_schemas,omitempty"`                // 输入模式
	OutputSchemas      []*ArgsSchema                                         `json:"output_schemas,omitempty"`               // 输出模式
	ReceiveChatHistory *bool                                                 `json:"receive_chat_history,omitempty"`         // 是否接收聊天历史
	Tags               map[EvaluatorTagLangType]map[EvaluatorTagKey][]string `json:"tags,omitempty"`                         // 标签

	// 评估器内容
	PromptEvaluatorContent *PromptEvaluatorContent `json:"prompt_evaluator_content,omitempty"` // Prompt评估器内容
	CodeEvaluatorContent   *CodeEvaluatorContent   `json:"code_evaluator_content,omitempty"`   // Code评估器内容
}

// CreateEvaluatorTemplateResponse 创建评估器模板响应
type CreateEvaluatorTemplateResponse struct {
	Template *EvaluatorTemplate `json:"template"` // 创建的模板
}

// UpdateEvaluatorTemplateRequest 更新评估器模板请求
type UpdateEvaluatorTemplateRequest struct {
	ID                 int64                                                 `json:"id" validate:"required,gt=0"`                        // 模板ID
	Name               *string                                               `json:"name,omitempty" validate:"omitempty,min=1,max=100"`  // 模板名称
	Description        *string                                               `json:"description,omitempty" validate:"omitempty,max=500"` // 模板描述
	EvaluatorInfo      *EvaluatorInfo                                        `json:"evaluator_info,omitempty"`                           // 评估器补充信息
	InputSchemas       []*ArgsSchema                                         `json:"input_schemas,omitempty"`                            // 输入模式
	OutputSchemas      []*ArgsSchema                                         `json:"output_schemas,omitempty"`                           // 输出模式
	ReceiveChatHistory *bool                                                 `json:"receive_chat_history,omitempty"`                     // 是否接收聊天历史
	Tags               map[EvaluatorTagLangType]map[EvaluatorTagKey][]string `json:"tags,omitempty"`                                     // 标签

	// 评估器内容
	PromptEvaluatorContent *PromptEvaluatorContent `json:"prompt_evaluator_content,omitempty"` // Prompt评估器内容
	CodeEvaluatorContent   *CodeEvaluatorContent   `json:"code_evaluator_content,omitempty"`   // Code评估器内容
}

// UpdateEvaluatorTemplateResponse 更新评估器模板响应
type UpdateEvaluatorTemplateResponse struct {
	Template *EvaluatorTemplate `json:"template"` // 更新后的模板
}

// DeleteEvaluatorTemplateRequest 删除评估器模板请求
type DeleteEvaluatorTemplateRequest struct {
	ID int64 `json:"id" validate:"required,gt=0"` // 模板ID
}

// DeleteEvaluatorTemplateResponse 删除评估器模板响应
type DeleteEvaluatorTemplateResponse struct {
	Success bool `json:"success"` // 删除是否成功
}

// GetEvaluatorTemplateRequest 获取评估器模板请求
type GetEvaluatorTemplateRequest struct {
	ID             int64 `json:"id" validate:"required,gt=0"` // 模板ID
	IncludeDeleted bool  `json:"include_deleted,omitempty"`   // 是否包含已删除记录
}

// GetEvaluatorTemplateResponse 获取评估器模板响应
type GetEvaluatorTemplateResponse struct {
	Template *EvaluatorTemplate `json:"template"` // 模板详情
}

// ListEvaluatorTemplateRequest 查询评估器模板列表请求
type ListEvaluatorTemplateRequest struct {
	SpaceID        int64                  `json:"space_id" validate:"required,gt=0"`           // 空间ID
	FilterOption   *EvaluatorFilterOption `json:"filter_option,omitempty"`                     // 标签筛选条件
	PageSize       int32                  `json:"page_size" validate:"required,min=1,max=100"` // 分页大小
	PageNum        int32                  `json:"page_num" validate:"required,min=1"`          // 页码
	IncludeDeleted bool                   `json:"include_deleted,omitempty"`                   // 是否包含已删除记录
}

// ListEvaluatorTemplateResponse 查询评估器模板列表响应
type ListEvaluatorTemplateResponse struct {
	TotalCount int64                `json:"total_count"` // 总数量
	Templates  []*EvaluatorTemplate `json:"templates"`   // 模板列表
	PageSize   int32                `json:"page_size"`   // 分页大小
	PageNum    int32                `json:"page_num"`    // 页码
	TotalPages int32                `json:"total_pages"` // 总页数
}

type ListEvaluationSetItemDefsParam struct {
	SpaceID         int64
	EvaluationSetID int64
	PageNumber      *int32
	PageSize        *int32
	PageToken       *string
	OrderBys        []*OrderBy
}

type ListEvaluationSetItemVersionsParam struct {
	SpaceID         int64
	EvaluationSetID int64
	ItemID          int64
	PageNumber      *int32
	PageSize        *int32
	PageToken       *string
	OrderBys        []*OrderBy
}

type BatchAddExistEvaluationSetItemsParam struct {
	SpaceID         int64
	EvaluationSetID int64
	Items           []*EvaluationItemVersionRef
	AllowPartialAdd *bool
}

type BatchAddExistEvaluationSetItemsResult struct {
	SuccessCount *int32
	FailedCount  *int32
	FailedItems  []*EvaluationItemVersionRef
}
