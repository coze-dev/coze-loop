// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"context"
	"fmt"
	"time"

	"github.com/mitchellh/mapstructure"

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

// TODO
type ExptRunLog struct {
	ID            int64
	SpaceID       int64
	CreatedBy     string
	ExptID        int64
	ExptRunID     int64
	ItemIds       []byte
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
	if t != nil && t.TargetVersionID != 0 && t.IngressConf != nil && t.IngressConf.EvalSetAdapter != nil {
		if targetType == EvalTargetTypeLoopPrompt || len(t.IngressConf.EvalSetAdapter.FieldConfs) > 0 {
			return nil
		}
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
}

func (e *EvaluatorsConf) Valid(ctx context.Context) error {
	if e == nil || len(e.EvaluatorConf) == 0 {
		return fmt.Errorf("invalid EvaluatorConf: %v", json.Jsonify(e))
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
	IngressConf        *EvaluatorIngressConf
}

func (e *EvaluatorConf) Valid(ctx context.Context) error {
	if e == nil || e.EvaluatorVersionID == 0 || e.IngressConf == nil ||
		e.IngressConf.TargetAdapter == nil || e.IngressConf.EvalSetAdapter == nil {
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

	IncompleteTurnIDs []*ItemTurnID
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
}

type InvokeExptReq struct {
	ExptID  int64
	RunID   int64
	SpaceID int64
	Session *Session

	Items []*EvaluationSetItem

	Ext map[string]string
}
