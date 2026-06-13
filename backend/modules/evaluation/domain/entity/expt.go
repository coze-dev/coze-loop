// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"fmt"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/mitchellh/mapstructure"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

type (
	ExptStatus                int64
	OfflineExptAnalysisStatus int32
	ExptType                  int64
	SourceType                = int64
	Visibility                = int64
)

const (
	Visibility_Hidden Visibility = 1
)

const (
	ExptStatus_Unknown ExptStatus = 0
	// Awaiting execution
	ExptStatus_Pending ExptStatus = 2
	// In progress
	ExptStatus_Processing ExptStatus = 3
	// Execution succeeded
	ExptStatus_Success ExptStatus = 11
	// Execution failed
	ExptStatus_Failed ExptStatus = 12
	// User terminated
	ExptStatus_Terminated ExptStatus = 13
	// System terminated
	ExptStatus_SystemTerminated ExptStatus = 14
	ExptStatus_Terminating      ExptStatus = 15

	// 流式执行完成，不再接收新的请求
	ExptStatus_Draining ExptStatus = 21
)

const (
	OfflineExptAnalysisStatus_NotStarted OfflineExptAnalysisStatus = 0 // 未开始
	OfflineExptAnalysisStatus_Processing OfflineExptAnalysisStatus = 1 // 进行中
	OfflineExptAnalysisStatus_Success    OfflineExptAnalysisStatus = 2 // 成功
	OfflineExptAnalysisStatus_Failed     OfflineExptAnalysisStatus = 3 // 失败
	OfflineExptAnalysisStatus_Superseded OfflineExptAnalysisStatus = 4 // 已被新版本/新分析取代
)

const (
	ExptType_Offline ExptType = 1
	ExptType_Online  ExptType = 2
)

const (
	SourceType_Evaluation SourceType = 1
	SourceType_Trace      SourceType = 2
	// SourceType_AutoTask 用于 ExptSource，与 IDL domain_expt.SourceType_AutoTask 一致
	SourceType_AutoTask SourceType = 2
	// SourceType_Workflow 与 IDL domain_expt.SourceType_Workflow 一致（Pipeline / 工作流来源，用于 enrichExptSourceFromPipeline 等）
	SourceType_Workflow       SourceType = 3
	SourceType_IntelligentGen SourceType = 4
)

type ExptRunLog struct {
	ID            int64
	SpaceID       int64
	CreatedBy     string
	ExptID        int64
	ExptRunID     int64
	ItemIds       []ExptRunLogItems
	Mode          int32
	Status        int64
	PendingCnt    int32
	SuccessCnt    int32
	FailCnt       int32
	CreditCost    float64
	TokenCost     int64
	StatusMessage []byte
	ProcessingCnt int32
	TerminatedCnt int32
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (e *ExptRunLog) GetItemIDs() []int64 {
	var itemIDs []int64
	for _, items := range e.ItemIds {
		itemIDs = append(itemIDs, items.ItemIDs...)
	}
	return itemIDs
}

func (e *ExptRunLog) AppendItemIDs(itemIDs []int64) error {
	if e == nil {
		return errorx.New("ExptRunLog AppendItemIDs must init first")
	}
	exists := make(map[int64]bool)
	for _, chunk := range e.ItemIds {
		for _, itemID := range chunk.ItemIDs {
			exists[itemID] = true
		}
	}
	rlItems := ExptRunLogItems{CreateAt: gptr.Of(time.Now().Unix())}
	for _, itemID := range itemIDs {
		if exists[itemID] {
			return errorx.NewByCode(errno.EvalItemAlreadyRetryingCode, errorx.WithExtraMsg(fmt.Sprintf("existed item_id: %v", itemID)))
		} else {
			rlItems.ItemIDs = append(rlItems.ItemIDs, itemID)
		}
	}
	e.ItemIds = append(e.ItemIds, rlItems)
	return nil
}

type Experiment struct {
	ID          int64
	SpaceID     int64
	CreatedBy   string
	Name        string
	Description string

	EvalSetVersionID    int64
	EvalSetID           int64
	TargetType          EvalTargetType
	TargetVersionID     int64
	TargetID            int64
	EvaluatorVersionRef []*ExptEvaluatorVersionRef
	EvalConf            *EvaluationConfiguration

	// ★ 新增: 评测集来源模式 (1=SingleSet老路径 / 2=MultiSetConfig新路径)
	EvalSetSourceType ExptEvalSetSourceType

	Target     *EvalTarget
	EvalSet    *EvaluationSet
	Evaluators []*Evaluator

	Status        ExptStatus
	StatusMessage string
	// OfflineExptAnalysisStatus 离线实验分析状态，与表字段 offline_expt_analysis_status 一致
	OfflineExptAnalysisStatus OfflineExptAnalysisStatus
	LatestRunID               int64

	CreditCost CreditCost

	StartAt *time.Time
	EndAt   *time.Time

	ExptType     ExptType
	MaxAliveTime int64
	SourceType   SourceType
	SourceID     string
	// TriggerType 实验触发方式，与表字段 trigger_type 一致：manual / openapi / schedule
	TriggerType string
	// ExptSource 查询时填充：与一级字段 source_type/source_id 一致；Workflow 时由 Pipeline 补充 span_filter / scheduler / sampler
	ExptSource        *ExptSource
	TrialRunItemCount int64

	Stats           *ExptStats
	AggregateResult *ExptAggregateResult

	ExptTemplateMeta *ExptTemplateMeta // 关联的实验模板基础信息（仅在查询时按需填充，包含模板 ID）

	Visibility Visibility // 实验模板可见性，默认为空，可见
	ThreadID   *string    // 关联的智能评测会话ID
}

func (e *Experiment) ToEvaluatorRefDO() []*ExptEvaluatorRef {
	if e == nil {
		return nil
	}
	// ★ 新路径 (MultiSetConfig): 从 EvalConf.EvalSetConfigs 构建带 alias/filter/binding_config 的 ref 行
	if e.EvalSetSourceType == ExptEvalSetSourceType_MultiSetConfig && e.EvalConf != nil && len(e.EvalConf.EvalSetConfigs) > 0 {
		refs := make([]*ExptEvaluatorRef, 0)
		for _, setConf := range e.EvalConf.EvalSetConfigs {
			for _, evConf := range setConf.EvaluatorConfs {
				ref := &ExptEvaluatorRef{
					SpaceID:            e.SpaceID,
					ExptID:             e.ID,
					EvalSetID:          setConf.EvalSetID,
					EvaluatorID:        evConf.EvaluatorID,
					EvaluatorVersionID: evConf.EvaluatorVersionID,
					Alias:              evConf.Alias,
				}
				// 序列化 filter + binding_config 快照（仅供查询）
				if evConf.Filter != nil {
					if b, err := json.Marshal(evConf.Filter); err == nil {
						ref.Filter = b
					}
				}
				// binding_config = {IngressConf, RunConf, ScoreWeight}
				bindingSnap := struct {
					FromEvalSet  []*FieldConf      `json:"from_eval_set,omitempty"`
					FromTarget   []*FieldConf      `json:"from_target,omitempty"`
					RuntimeParam map[string]string `json:"runtime_param,omitempty"`
					ScoreWeight  *float64          `json:"score_weight,omitempty"`
				}{
					FromEvalSet:  evConf.FromEvalSet,
					FromTarget:   evConf.FromTarget,
					RuntimeParam: evConf.RuntimeParam,
					ScoreWeight:  evConf.ScoreWeight,
				}
				if b, err := json.Marshal(bindingSnap); err == nil {
					ref.BindingConfig = b
				}
				refs = append(refs, ref)
			}
		}
		return refs
	}

	// 老路径 (SingleSet): 从 EvaluatorVersionRef 构建
	cnt := len(e.EvaluatorVersionRef)
	refs := make([]*ExptEvaluatorRef, 0, cnt)
	for _, evr := range e.EvaluatorVersionRef {
		refs = append(refs, &ExptEvaluatorRef{
			SpaceID:            e.SpaceID,
			ExptID:             e.ID,
			EvaluatorID:        evr.EvaluatorID,
			EvaluatorVersionID: evr.EvaluatorVersionID,
		})
	}
	return refs
}

func (e *Experiment) AsyncExec() bool {
	return e.AsyncCallTarget() || e.AsyncCallEvaluators()
}

func (e *Experiment) AsyncCallTarget() bool {
	if e == nil || e.Target == nil || e.Target.EvalTargetVersion == nil {
		return false
	}
	if e.Target.EvalTargetVersion.CustomRPCServer != nil && gptr.Indirect(e.Target.EvalTargetVersion.CustomRPCServer.IsAsync) {
		return true
	}
	if e.Target.EvalTargetVersion.WebAgent != nil {
		return true
	}
	return false
}

func (e *Experiment) AsyncCallEvaluators() bool {
	if e == nil || len(e.Evaluators) == 0 {
		return false
	}
	for _, ev := range e.Evaluators {
		if ev.IsAsync() {
			return true
		}
	}
	return false
}

func (e *Experiment) ContainsEvalTarget() bool {
	return e != nil && e.TargetVersionID > 0
}

type ExptEvaluatorVersionRef struct {
	EvaluatorID        int64
	EvaluatorVersionID int64
}

func (e *ExptEvaluatorVersionRef) String() string {
	return fmt.Sprintf("evaluator_id= %v, evaluator_version_id= %v", e.EvaluatorID, e.EvaluatorVersionID)
}

type EvaluationConfiguration struct {
	ConnectorConf           Connector
	ItemConcurNum           *int
	ItemRetryNum            *int
	TimeRange               *TaskTimeRangeDO  `json:"time_range,omitempty"`
	EnableExtractTrajectory *bool
	Ext                     map[string]string

	// ★ 新增: 多评测集配置 (MultiSetConfig 路径权威源)
	// 创建期序列化进 experiment.eval_conf; 调度期反序列化读取
	EvalSetConfigs []*EvalSetConfig `json:"eval_set_configs,omitempty"`
}

type Connector struct {
	TargetConf     *TargetConf
	EvaluatorsConf *EvaluatorsConf
}

type TargetConf struct {
	TargetVersionID int64
	IngressConf     *TargetIngressConf
}

func (t *TargetConf) Valid(ctx context.Context, targetType EvalTargetType) error {
	if t == nil || t.TargetVersionID == 0 {
		return fmt.Errorf("invalid TargetConf: %v", json.Jsonify(t))
	}
	// prompt/custom_rpc 可能无输入；仅记录型不需要执行，仅需记录对象类型和基本信息
	if targetType == EvalTargetTypeLoopPrompt || targetType == EvalTargetTypeCustomRPCServer || targetType == EvalTargetTypeWebAgent || targetType.IsRecordOnlyType() {
		return nil
	}
	if t.IngressConf != nil && t.IngressConf.EvalSetAdapter != nil && len(t.IngressConf.EvalSetAdapter.FieldConfs) > 0 {
		return nil
	}
	return fmt.Errorf("invalid TargetConf: %v", json.Jsonify(t))
}

type TargetIngressConf struct {
	EvalSetAdapter *FieldAdapter
	CustomConf     *FieldAdapter
}

type EvaluatorsConf struct {
	EvaluatorConcurNum *int
	EvaluatorConf      []*EvaluatorConf
	EnableScoreWeight  bool
}

func (e *EvaluatorsConf) Valid(ctx context.Context) error {
	if e == nil {
		return fmt.Errorf("nil EvaluatorConf")
	}
	for _, conf := range e.EvaluatorConf {
		if err := conf.Valid(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (e *EvaluatorsConf) GetEvaluatorConf(evalVerID int64) *EvaluatorConf {
	for _, conf := range e.EvaluatorConf {
		if conf.EvaluatorVersionID == evalVerID {
			return conf
		}
	}
	return nil
}

func (e *EvaluatorsConf) GetEvaluatorConcurNum() int {
	const defaultConcurNum = 3
	if e.EvaluatorConcurNum != nil && *e.EvaluatorConcurNum > 0 {
		return *e.EvaluatorConcurNum
	}
	return defaultConcurNum
}

type EvaluatorConf struct {
	EvaluatorVersionID int64
	EvaluatorID        int64  // 评估器ID（用于匹配回填 evaluator_version_id）
	Version            string // 评估器版本号（用于匹配回填 evaluator_version_id）
	IngressConf        *EvaluatorIngressConf
	RunConf            *EvaluatorRunConfig
	ScoreWeight        *float64
}

func (e *EvaluatorConf) Valid(ctx context.Context) error {
	if e == nil || e.EvaluatorVersionID == 0 || e.IngressConf == nil ||
		(e.IngressConf.TargetAdapter == nil && e.IngressConf.EvalSetAdapter == nil) {
		return fmt.Errorf("invalid EvaluatorConf: %v", json.Jsonify(e))
	}
	return nil
}

type EvaluatorIngressConf struct {
	EvalSetAdapter *FieldAdapter
	TargetAdapter  *FieldAdapter
	CustomConf     *FieldAdapter
}

type FieldAdapter struct {
	FieldConfs []*FieldConf
}

type FieldConf struct {
	FieldName string
	FromField string
	Value     string
}

type ExptUpdateFields struct {
	Name string `mapstructure:"name,omitempty"`
	Desc string `mapstructure:"description,omitempty"`
}

func (e *ExptUpdateFields) ToFieldMap() (map[string]any, error) {
	m := make(map[string]any)
	if err := mapstructure.Decode(e, &m); err != nil {
		return nil, errorx.Wrapf(err, "ExptUpdateFields decode to map fail: %v", e)
	}
	return m, nil
}

type ExptCalculateStats struct {
	PendingItemCnt    int
	FailItemCnt       int
	SuccessItemCnt    int
	ProcessingItemCnt int
	TerminatedItemCnt int
}

type ItemTurnID struct {
	ItemID int64
	TurnID int64
}

type StatsCntArithOp struct {
	OpStatusCnt map[ItemRunState]int
}

type TupleExpt struct {
	Expt *Experiment
	*ExptTuple
}

type ExptTuple struct {
	Target     *EvalTarget
	EvalSet    *EvaluationSet
	Evaluators []*Evaluator
}

type ExptTupleID struct {
	VersionedTargetID   *VersionedTargetID
	VersionedEvalSetID  *VersionedEvalSetID
	EvaluatorVersionIDs []int64
}

type VersionedTargetID struct {
	TargetID  int64
	VersionID int64
}

type VersionedEvalSetID struct {
	EvalSetID int64
	VersionID int64
}

type CreateEvalTargetParam struct {
	SourceTargetID       *string
	SourceTargetVersion  *string
	EvalTargetType       *EvalTargetType
	BotInfoType          *CozeBotInfoType
	BotPublishVersion    *string
	CustomEvalTarget     *CustomEvalTarget // 搜索对象返回的信息
	Region               *Region
	Env                  *string
	OperationInstruction *string
	Cluster              *string
	AgentConnection      *AgentConnection
}

func (c *CreateEvalTargetParam) IsNull() bool {
	if c == nil {
		return true
	}
	// 仅传 eval_target_type（如仅记录型 Online 评测对象）时也应走创建逻辑，不能仅依据 source 指针判断
	if c.EvalTargetType != nil {
		return false
	}
	return c.SourceTargetID == nil && c.SourceTargetVersion == nil
}

type InvokeExptReq struct {
	ExptID  int64
	RunID   int64
	SpaceID int64
	Session *Session

	Items []*EvaluationSetItem

	Ext map[string]string
}

type ExptRunLogItems struct {
	ItemIDs  []int64
	CreateAt *int64
}

// =====================================================================================
// ★ item-centric 实验改版新增类型 (2026-06)
// =====================================================================================

// ExptEvalSetSourceType 实验评测集来源模式: 读接口和执行链路分流依据
type ExptEvalSetSourceType int32

const (
	ExptEvalSetSourceType_SingleSet      ExptEvalSetSourceType = 1 // 老实验: 单评测集, 配置在平铺老字段
	ExptEvalSetSourceType_MultiSetConfig ExptEvalSetSourceType = 2 // 新实验: 多评测集+配置, 权威源 eval_conf.EvalSetConfigs
)

// ExptItemRef 实验绑定 item 的扁平集合 (首次调度 ExptStart 写入, 单行执行唯一配置源)
type ExptItemRef struct {
	ID               int64
	SpaceID          int64
	ExptID           int64
	ItemID           int64
	ItemVersionID    int64 // 0=无版本概念(DataSet暂不支持); 全链路真值源
	EvalSetID        int64 // 归属评测集标签 (前端分组/CK分桶/反查; 调度不读)
	EvalSetVersionID int64 // 调度键: 配合 item_id 定位 dataset_item_snapshot
	ItemConfig       *ExptItemConfig
	OrderIdx         int32
}

// ExptItemConfig per-item 行级配置 JSON (expt_item_ref.item_config)
// 单行执行的唯一配置源; 执行链路只读此结构, 不回读 eval_conf 或 expt_evaluator_ref
type ExptItemConfig struct {
	EvalTargetConf *ItemTargetConf      `json:"eval_target_conf,omitempty"`
	EvaluatorConfs []*ItemEvaluatorConf `json:"evaluator_conf,omitempty"`
	TurnIndexes    []int32              `json:"turn_indexes,omitempty"`
	Ext            map[string]string    `json:"ext,omitempty"`
}

// ItemTargetConf per-item target 运行配置
type ItemTargetConf struct {
	TargetVersionID int64             `json:"version_id"`
	FieldMapping    []*FieldConf      `json:"field_mapping,omitempty"`
	DynamicConf     map[string]string `json:"dynamic_conf,omitempty"`
	ScoreWeight     *float64          `json:"score_weight,omitempty"`
}

// ItemEvaluatorConf per-item 单个 evaluator binding 配置
// 消歧维度: (EvaluatorVersionID, Alias); Alias 为空 = 默认实例
type ItemEvaluatorConf struct {
	EvaluatorVersionID int64             `json:"version_id"`
	Alias              string            `json:"alias,omitempty"`
	FromEvalSet        []*FieldConf      `json:"from_eval_set,omitempty"`
	FromTarget         []*FieldConf      `json:"from_target,omitempty"`
	DynamicParam       map[string]string `json:"dynamic_param,omitempty"`
	Filter             *ExptItemFilter   `json:"filter,omitempty"`
	FilterMode         int32             `json:"filter_mode,omitempty"` // 0 None / 1 Include / 2 Exclude
	ScoreWeight        *float64          `json:"score_weight,omitempty"`
}

// ExptItemFilter item 圈选 / evaluator 行级过滤 (与 data/domain/filter.thrift Filter 同构)
type ExptItemFilter struct {
	QueryAndOr   string                `json:"query_and_or,omitempty"`
	FilterFields []*ExptItemFilterField `json:"filter_fields"`
}

// ExptItemFilterField 单个过滤字段
type ExptItemFilterField struct {
	FieldName  string   `json:"field_name"`
	FieldType  string   `json:"field_type"`
	Values     []string `json:"values,omitempty"`
	QueryType  string   `json:"query_type,omitempty"`
}

// EvalSetConfig 一个评测集 + 该集的完整配置包 (对应 IDL ExptDomain.EvalSetConfig)
type EvalSetConfig struct {
	EvalSetID        int64                `json:"eval_set_id"`
	EvalSetVersionID int64                `json:"eval_set_version_id"`
	ItemFilter       *ExptItemFilter      `json:"item_filter,omitempty"`
	TargetConfs      []*ExptTargetConf    `json:"target_confs,omitempty"`
	EvaluatorConfs   []*ExptEvaluatorConf `json:"evaluator_confs,omitempty"`
	Ext              map[string]string    `json:"ext,omitempty"`
}

// ExptTargetConf per-set target 运行配置 (本期 len<=1, alias 恒空)
type ExptTargetConf struct {
	TargetID        int64             `json:"target_id,omitempty"`
	TargetVersionID int64             `json:"target_version_id,omitempty"`
	FieldMapping    []*FieldConf      `json:"field_mapping,omitempty"`
	RuntimeParam    map[string]string `json:"runtime_param,omitempty"`
	Alias           string            `json:"alias,omitempty"` // 本期恒空串
	Ext             map[string]string `json:"ext,omitempty"`
}

// ExptEvaluatorConf per-set 一个 evaluator binding 配置 (对应 IDL ExptDomain.ExptEvaluatorConf)
type ExptEvaluatorConf struct {
	EvaluatorID        int64             `json:"evaluator_id"`
	EvaluatorVersionID int64             `json:"evaluator_version_id"`
	Alias              string            `json:"alias,omitempty"`
	FromEvalSet        []*FieldConf      `json:"from_eval_set,omitempty"`
	FromTarget         []*FieldConf      `json:"from_target,omitempty"`
	Filter             *ExptItemFilter   `json:"filter,omitempty"`
	FilterMode         int32             `json:"filter_mode,omitempty"`
	RuntimeParam       map[string]string `json:"runtime_param,omitempty"`
	ScoreWeight        *float64          `json:"score_weight,omitempty"`
	Ext                map[string]string `json:"ext,omitempty"`
}

