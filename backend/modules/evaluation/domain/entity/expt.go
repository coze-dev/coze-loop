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
	ExptStatus int64
	ExptType   int64
	SourceType = int64
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
	ExptType_Offline ExptType = 1
	ExptType_Online  ExptType = 2
)

const (
	SourceType_Evaluation SourceType = 1
	SourceType_Trace      SourceType = 2
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
		for _, itemID := range items.ItemIDs {
			itemIDs = append(itemIDs, itemID)
		}
	}
	return itemIDs
}

func (e *ExptRunLog) AppendItemIDs(itemIDs []int64) error {
	if e == nil {
		return errorx.New("ExptRunLog AppendItemIDs must init first")
	}
	var exists map[int64]bool
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

	Target     *EvalTarget
	EvalSet    *EvaluationSet
	Evaluators []*Evaluator

	Status        ExptStatus
	StatusMessage string
	LatestRunID   int64

	CreditCost CreditCost

	StartAt *time.Time
	EndAt   *time.Time

	ExptType     ExptType
	MaxAliveTime int64
	SourceType   SourceType
	SourceID     string

	Stats           *ExptStats
	AggregateResult *ExptAggregateResult

	ExptTemplateMeta *ExptTemplateMeta // 关联的实验模板基础信息（仅在查询时按需填充，包含模板 ID）
}

func (e *Experiment) ToEvaluatorRefDO() []*ExptEvaluatorRef {
	if e == nil {
		return nil
	}
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
	if e == nil || e.Target == nil || e.Target.EvalTargetVersion == nil || e.Target.EvalTargetVersion.CustomRPCServer == nil {
		return false
	}
	return gptr.Indirect(e.Target.EvalTargetVersion.CustomRPCServer.IsAsync)
}

func (e *Experiment) AsyncCallEvaluators() bool {
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
	ConnectorConf Connector
	ItemConcurNum *int
	ItemRetryNum  *int
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
	if targetType == EvalTargetTypeLoopPrompt || targetType == EvalTargetTypeCustomRPCServer { // prompt target might receive no input
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
	SourceTargetID      *string
	SourceTargetVersion *string
	EvalTargetType      *EvalTargetType
	BotInfoType         *CozeBotInfoType
	BotPublishVersion   *string
	CustomEvalTarget    *CustomEvalTarget // 搜索对象返回的信息
	Region              *Region
	Env                 *string
}

func (c *CreateEvalTargetParam) IsNull() bool {
	return c == nil || (c.SourceTargetID == nil && c.SourceTargetVersion == nil)
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
