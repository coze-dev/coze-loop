// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"strconv"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/middleware/session"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

type FieldType int64

const (
	FieldType_Unknown FieldType = 0
	// 评估器得分, FieldKey为evaluatorVersionID,value为score
	FieldType_EvaluatorScore     FieldType = 1
	FieldType_CreatorBy          FieldType = 2
	FieldType_ExptStatus         FieldType = 3
	FieldType_TurnRunState       FieldType = 4
	FieldType_TargetID           FieldType = 5
	FieldType_EvalSetID          FieldType = 6
	FieldType_EvaluatorID        FieldType = 7
	FieldType_TargetType         FieldType = 8
	FieldType_SourceTarget       FieldType = 9
	FieldType_EvaluatorVersionID FieldType = 20
	FieldType_TargetVersionID    FieldType = 21
	FieldType_EvalSetVersionID   FieldType = 22

	// 标注项, FieldKey为TagKeyID
	FieldType_Annotation FieldType = 23
)

// aggregate result
type UpdateExptAggrResultParam struct {
	SpaceID      int64
	ExperimentID int64
	FieldType    FieldType
	FieldKey     string
}

type CreateSpecificFieldAggrResultParam struct {
	SpaceID      int64
	ExperimentID int64
	FieldType    FieldType
	FieldKey     string
}

// AggregatorType 聚合器类型
type AggregatorType int

const (
	Average      AggregatorType = 1
	Sum          AggregatorType = 2
	Max          AggregatorType = 3
	Min          AggregatorType = 4
	Distribution AggregatorType = 5 // 得分的分布情况
)

type AggrResultDataType int

const (
	Double              AggrResultDataType = 0 // 默认，有小数的浮点数值类型
	ScoreDistribution   AggrResultDataType = 1 // 得分分布
	OptionDistribution  AggrResultDataType = 2 // 选项分布
	BooleanDistribution AggrResultDataType = 3 // 布尔分布
)

type ScoreDistributionData struct {
	ScoreDistributionItems []*ScoreDistributionItem
}

type ScoreDistributionItem struct {
	Score      string  // 得分,TOP5以外的聚合展示为“其他”
	Count      int64   // 此得分的数量
	Percentage float64 // 占总数的百分比
}

type OptionDistributionData struct {
	OptionDistributionItems []*OptionDistributionItem
}
type OptionDistributionItem struct {
	Option     string // 选项ID,TOP5以外的聚合展示为“其他”
	Count      int64
	Percentage float64
}

type BooleanDistributionData struct {
	TrueCount      int64
	FalseCount     int64
	TruePercentage float64
}

type AggregateData struct {
	DataType            AggrResultDataType
	Value               *float64
	ScoreDistribution   *ScoreDistributionData
	OptionDistribution  *OptionDistributionData
	BooleanDistribution *BooleanDistributionData
}

// AggregatorResult 一种聚合器类型的聚合结果
type AggregatorResult struct {
	AggregatorType AggregatorType
	Data           *AggregateData
}

// expt_aggr_result 表 aggr_result 字段blob结构
type AggregateResult struct {
	AggregatorResults []*AggregatorResult
}

func (a AggregatorResult) GetScore() float64 {
	if a.Data == nil {
		return 0
	}
	if a.Data.Value == nil {
		return 0
	}

	return *a.Data.Value
}

type ExptAggrResult struct {
	ID           int64
	SpaceID      int64
	ExperimentID int64
	FieldType    int32
	FieldKey     string
	Score        float64
	AggrResult   []byte
	Version      int64
	Status       int32
}

type ExptAggregateResult struct {
	ExperimentID      int64
	EvaluatorResults  map[int64]*EvaluatorAggregateResult
	Status            int64
	AnnotationResults map[int64]*AnnotationAggregateResult
}

type EvaluatorAggregateResult struct {
	EvaluatorVersionID int64
	AggregatorResults  []*AggregatorResult
	Name               *string
	Version            *string
}

// 人工标注项粒度聚合结果
type AnnotationAggregateResult struct {
	TagKeyID          int64
	AggregatorResults []*AggregatorResult
	Name              *string
}

// item result
type ExptItemResult struct {
	ID        int64
	SpaceID   int64
	ExptID    int64
	ExptRunID int64
	ItemID    int64
	Status    ItemRunState
	ErrMsg    string
	ItemIdx   int32
	LogID     string
}

type ExptItemResultRunLog struct {
	ID          int64
	SpaceID     int64
	ExptID      int64
	ExptRunID   int64
	ItemID      int64
	Status      int32
	ErrMsg      []byte
	LogID       string
	ResultState int32
	UpdatedAt   *time.Time
}

type ExptItemEvalResult struct {
	ItemResultRunLog  *ExptItemResultRunLog
	TurnResultRunLogs map[int64]*ExptTurnResultRunLog
}

type ExptEvalItem struct {
	ExptID           int64
	EvalSetVersionID int64
	ItemID           int64
	State            ItemRunState
	UpdatedAt        *time.Time
}

func (e *ExptEvalItem) SetState(state ItemRunState) *ExptEvalItem {
	e.State = state
	return e
}

type ExptEvalTurn struct {
	ExptID    int64
	ExptRunID int64
	ItemID    int64
	TurnID    int64
}

type ExptStats struct {
	ID                int64
	SpaceID           int64
	ExptID            int64
	PendingItemCnt    int32
	SuccessItemCnt    int32
	FailItemCnt       int32
	ProcessingItemCnt int32
	TerminatedItemCnt int32
	CreditCost        float64
	InputTokenCost    int64
	OutputTokenCost   int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type ExptTurnResult struct {
	ID               int64
	SpaceID          int64
	ExptID           int64
	ExptRunID        int64
	ItemID           int64
	TurnID           int64
	Status           int32
	TraceID          int64
	LogID            string
	TargetResultID   int64
	EvaluatorResults *EvaluatorResults
	ErrMsg           string
	TurnIdx          int32
}

func (tr *ExptTurnResult) ToRunLogDO() *ExptTurnResultRunLog {
	if tr == nil {
		return nil
	}
	return &ExptTurnResultRunLog{
		ID:                 tr.ID,
		SpaceID:            tr.SpaceID,
		ExptID:             tr.ExptID,
		ExptRunID:          tr.ExptRunID,
		ItemID:             tr.ItemID,
		TurnID:             tr.TurnID,
		Status:             TurnRunState(tr.Status),
		TraceID:            tr.TraceID,
		LogID:              tr.LogID,
		TargetResultID:     tr.TargetResultID,
		EvaluatorResultIds: tr.EvaluatorResults,
		ErrMsg:             tr.ErrMsg,
	}
}

type EvaluatorResults struct {
	EvalVerIDToResID map[int64]int64
}

func (e *EvaluatorResults) Serialize() ([]byte, error) {
	bytes, err := json.Marshal(e)
	if err != nil {
		return nil, errorx.Wrapf(err, "ExptTurnEvaluatorResultIDs json marshal fail")
	}
	return bytes, nil
}

type MGetExperimentResultParam struct {
	SpaceID            int64
	ExptIDs            []int64
	BaseExptID         *int64
	Filters            map[int64]*ExptTurnResultFilter
	FilterAccelerators map[int64]*ExptTurnResultFilterAccelerator
	UseAccelerator     bool
	Page               Page
}

type ExptTurnResultRunLog struct {
	ID                 int64
	SpaceID            int64
	ExptID             int64
	ExptRunID          int64
	ItemID             int64
	TurnID             int64
	Status             TurnRunState
	TraceID            int64
	LogID              string
	TargetResultID     int64
	EvaluatorResultIds *EvaluatorResults
	ErrMsg             string
}

type ExptTurnEvaluatorResultRef struct {
	ID                 int64
	SpaceID            int64
	ExptTurnResultID   int64
	EvaluatorVersionID int64
	EvaluatorResultID  int64
	ExptID             int64
}

type ExptEvaluatorRef struct {
	ID                 int64
	SpaceID            int64
	ExptID             int64
	EvaluatorID        int64
	EvaluatorVersionID int64
}

// filter
type ExptListFilter struct {
	FuzzyName string
	Includes  *ExptFilterFields
	Excludes  *ExptFilterFields
}

type ExptFilterFields struct {
	CreatedBy    []string
	Status       []int64
	EvalSetIDs   []int64
	TargetIDs    []int64
	EvaluatorIDs []int64
	TargetType   []int64
	ExptType     []int64
	SourceType   []int64
	SourceID     []string
}

func (e *ExptFilterFields) IsValid() bool {
	if e == nil {
		return true
	}
	for _, slice := range [][]int64{e.Status, e.EvalSetIDs, e.TargetIDs, e.EvaluatorIDs, e.TargetType} {
		for _, item := range slice {
			if item < 0 {
				return false
			}
		}
	}
	for _, item := range e.CreatedBy {
		if len(item) <= 0 {
			return false
		}
	}
	return true
}

type ExptItemRunLogFilter struct {
	Status      []ItemRunState
	ResultState *ExptItemResultState
}

func (e *ExptItemRunLogFilter) GetResultState() ExptItemResultState {
	return *e.ResultState
}

func (e *ExptItemRunLogFilter) GetStatus() []int32 {
	res := make([]int32, 0, len(e.Status))
	for _, status := range e.Status {
		res = append(res, int32(status))
	}
	return res
}

const (
	defaultPage     = 1 // 页数从 1 开始
	defaultLimit    = 20
	defaultMaxLimit = 200 // 分页最大限制
)

type Page struct {
	offset int
	limit  int
}

func NewPage(offset, limit int) Page {
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > defaultMaxLimit {
		limit = defaultMaxLimit
	}
	if offset <= 0 {
		offset = defaultPage
	}

	return Page{
		offset: offset,
		limit:  limit,
	}
}

func (p Page) Offset() int {
	return (p.offset - 1) * p.limit
}

func (p Page) Limit() int {
	return p.limit
}

type Session struct {
	UserID string
	AppID  int32
}

func NewSession(ctx context.Context) *Session {
	userIDInContext := session.UserIDInCtxOrEmpty(ctx)
	return &Session{
		UserID: userIDInContext,
	}
}

type ExptTurnResultFilterMapCond struct {
	EvalTargetDataFilters   []*FieldFilter
	EvaluatorScoreFilters   []*FieldFilter
	AnnotationFloatFilters  []*FieldFilter
	AnnotationBoolFilters   []*FieldFilter
	AnnotationStringFilters []*FieldFilter
}

type FieldFilter struct {
	Key    string
	Op     string // =, >, >=, <, <=, BETWEEN, LIKE
	Values []any
}

type ItemSnapshotFilter struct {
	BoolMapFilters   []*FieldFilter
	FloatMapFilters  []*FieldFilter
	IntMapFilters    []*FieldFilter
	StringMapFilters []*FieldFilter
}

type KeywordFilter struct {
	ItemSnapshotFilter    *ItemSnapshotFilter
	EvalTargetDataFilters []*FieldFilter
	Keyword               *string
}

type ExptTurnResultFilter struct {
	TrunRunStateFilters []*TurnRunStateFilter
	ScoreFilters        []*ScoreFilter
}

// ExptTurnResultFilterAccelerator 用于业务层组合主表字段和map字段的多条件查询
// 其中map字段支持等值、范围、模糊等多种组合
// 例如：EvalTargetDataFilters、EvaluatorScoreFilters等
// 具体用法参考DAO层QueryItemIDs的参数
type ExptTurnResultFilterAccelerator struct {
	// 必带字段
	SpaceID     int64     `json:"space_id"`
	ExptID      int64     `json:"expt_id"`
	CreatedDate time.Time `json:"created_date"`
	// 基础查询
	EvaluatorScoreCorrected *FieldFilter   `json:"evaluator_score_corrected"`
	ItemIDs                 []*FieldFilter `json:"item_id"`
	ItemRunStatus           []*FieldFilter `json:"item_status"`
	TurnRunStatus           []*FieldFilter `json:"turn_status"`
	// map类查询条件
	MapCond          *ExptTurnResultFilterMapCond `json:"map_cond,omitempty"`
	ItemSnapshotCond *ItemSnapshotFilter          `json:"item_snapshot_cond,omitempty"`
	// keyword search
	KeywordSearch     *KeywordFilter `json:"keyword_search"`
	Page              Page           `json:"page"`
	EvalSetSyncCkDate string
}

func (e *ExptTurnResultFilterAccelerator) HasFilters() bool {
	hasFilters := e.EvaluatorScoreCorrected != nil ||
		len(e.ItemIDs) > 0 ||
		len(e.ItemRunStatus) > 0 ||
		len(e.TurnRunStatus) > 0
	hasFilters = hasFilters || (e.MapCond != nil && (len(e.MapCond.EvalTargetDataFilters) > 0 ||
		len(e.MapCond.EvaluatorScoreFilters) > 0 ||
		len(e.MapCond.AnnotationFloatFilters) > 0 ||
		len(e.MapCond.AnnotationBoolFilters) > 0 ||
		len(e.MapCond.AnnotationStringFilters) > 0))
	hasFilters = hasFilters || (e.ItemSnapshotCond != nil && (len(e.ItemSnapshotCond.BoolMapFilters) > 0 ||
		len(e.ItemSnapshotCond.FloatMapFilters) > 0 ||
		len(e.ItemSnapshotCond.IntMapFilters) > 0 ||
		len(e.ItemSnapshotCond.StringMapFilters) > 0))
	hasFilters = hasFilters || (e.KeywordSearch != nil && ((e.KeywordSearch.ItemSnapshotFilter != nil && (len(e.KeywordSearch.ItemSnapshotFilter.BoolMapFilters) > 0 ||
		len(e.KeywordSearch.ItemSnapshotFilter.FloatMapFilters) > 0 ||
		len(e.KeywordSearch.ItemSnapshotFilter.IntMapFilters) > 0 ||
		len(e.KeywordSearch.ItemSnapshotFilter.StringMapFilters) > 0)) ||
		len(e.KeywordSearch.EvalTargetDataFilters) > 0))

	return hasFilters
}

// FieldTypeMapping 定义 ExptTurnResultFilterKeyMapping 中 FieldType 的常量
type FieldTypeMapping int32

const (
	// FieldTypeUnknown 未知类型
	FieldTypeUnknown FieldTypeMapping = 0
	// FieldTypeEvaluator 评估器类型
	FieldTypeEvaluator FieldTypeMapping = 1
	// FieldTypeManualAnnotation 人工标注类型
	FieldTypeManualAnnotation FieldTypeMapping = 2
	// FieldTypeManualAnnotationScore FieldTypeMapping = 2
	// FieldTypeManualAnnotationText FieldTypeMapping = 2
	// FieldTypeManualAnnotationCategorical FieldTypeMapping = 2

)

type ExptTurnResultFilterKeyMapping struct {
	SpaceID   int64            `json:"space_id"`   // 空间id
	ExptID    int64            `json:"expt_id"`    // 实验id
	FromField string           `json:"from_field"` // 筛选项唯一键，评估器: evaluator_version_id，人工标准：tag_key_id
	ToKey     string           `json:"to_key"`     // ck侧的map key，评估器：key1 ~ key10，人工标准：key1 ~ key100
	FieldType FieldTypeMapping `json:"field_type"` // 映射类型，Evaluator —— 1，人工标注—— 2
}

type ScoreFilter struct {
	Score              float64
	Operator           string
	EvaluatorVersionID int64
}

type TurnRunStateFilter struct {
	Status   []TurnRunState
	Operator string
}

type TurnTargetOutput struct {
	EvalTargetRecord *EvalTargetRecord
}

type TurnEvaluatorOutput struct {
	EvaluatorRecords map[int64]*EvaluatorRecord
}

type TurnAnnotateResult struct {
	AnnotateRecords map[int64]*AnnotateRecord
}

type TurnEvalSet struct {
	Turn *Turn
}

type TurnSystemInfo struct {
	TurnRunState TurnRunState
	LogID        *string
	Error        *RunError
}

type ItemSystemInfo struct {
	RunState ItemRunState
	LogID    *string
	Error    *RunError
}

type RunError struct {
	Code    int64
	Message *string
	Detail  *string
}

type ItemResult struct {
	ItemID int64
	// row粒度实验结果详情
	TurnResults []*TurnResult
	SystemInfo  *ItemSystemInfo
	ItemIndex   *int64
}

type ExperimentTurnPayload struct {
	TurnID int64
	// 评测数据集数据
	EvalSet *TurnEvalSet
	// 评测对象结果
	TargetOutput *TurnTargetOutput
	// 评测规则执行结果
	EvaluatorOutput *TurnEvaluatorOutput
	// 评测系统相关数据日志、error
	SystemInfo *TurnSystemInfo
	// 标注结果
	AnnotateResult *TurnAnnotateResult
}

type ExperimentResult struct {
	ExperimentID int64
	Payload      *ExperimentTurnPayload
}

type TurnResult struct {
	TurnID int64
	// 参与对比的实验序列，对于单报告序列长度为1
	ExperimentResults []*ExperimentResult
	TurnIndex         *int64
}

type ColumnEvalSetField struct {
	Key         *string
	Name        *string
	Description *string
	ContentType ContentType
	TextSchema  *string
}

type ColumnEvaluator struct {
	EvaluatorVersionID int64
	EvaluatorID        int64
	EvaluatorType      EvaluatorType
	Name               *string
	Version            *string
	Description        *string
}

type ExptColumnEvaluator struct {
	ExptID           int64
	ColumnEvaluators []*ColumnEvaluator
}

type ExptTurnResultFilterEntity struct {
	SpaceID                 int64              `json:"space_id"`
	ExptID                  int64              `json:"expt_id"`
	ItemID                  int64              `json:"item_id"`
	ItemIdx                 int32              `json:"item_idx"`
	TurnID                  int64              `json:"turn_id"`
	Status                  ItemRunState       `json:"status"`
	EvalTargetData          map[string]string  `json:"eval_target_data"`
	EvaluatorScore          map[string]float64 `json:"evaluator_score"`
	AnnotationFloat         map[string]float64 `json:"annotation_float"`
	AnnotationBool          map[string]bool    `json:"annotation_bool"`
	AnnotationString        map[string]string  `json:"annotation_string"`
	CreatedDate             time.Time          `json:"created_date"`
	EvaluatorScoreCorrected bool               `json:"evaluator_score_corrected"`
	EvalSetVersionID        int64              `json:"eval_set_version_id"`
	CreatedAt               time.Time          `json:"created_at"`
	UpdatedAt               time.Time          `json:"updated_at"`
}

type BmqProducerCfg struct {
	Topic   string `json:"topic"`
	Cluster string `json:"cluster"`
}

// IntersectInt64String 返回两个集合的交集（int64和string）
func IntersectInt64String(a []int64, b []string) []int64 {
	bSet := make(map[string]struct{}, len(b))
	for _, s := range b {
		bSet[s] = struct{}{}
	}
	var res []int64
	for _, v := range a {
		vs := strconv.FormatInt(v, 10)
		if _, ok := bSet[vs]; ok {
			res = append(res, v)
		}
	}
	return res
}

type ColumnAnnotation struct {
	TagKeyID       int64
	TagName        string
	Description    string
	TagValues      []*TagValue
	TagContentType TagContentType
	TagContentSpec *TagContentSpec
	TagStatus      TagStatus
}

type ExptColumnAnnotation struct {
	ExptID            int64
	ColumnAnnotations []*ColumnAnnotation
}
