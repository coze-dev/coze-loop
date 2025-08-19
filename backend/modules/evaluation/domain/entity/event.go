// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type ExptScheduleEvent struct {
	SpaceID     int64
	ExptID      int64
	ExptRunID   int64
	ExptRunMode ExptRunMode
	ExptType    ExptType

	CreatedAt int64
	Ext       map[string]string
	Session   *Session

	RetryTimes int
}

type ExptItemEvalEvent struct {
	SpaceID     int64
	ExptID      int64
	ExptRunID   int64
	ExptRunMode ExptRunMode

	EvalSetItemID int64

	CreateAt   int64
	RetryTimes int
	Ext        map[string]string
	Session    *Session
}

type CalculateMode int

const (
	CreateAllFields     CalculateMode = 1
	UpdateSpecificField CalculateMode = 2
)

type AggrCalculateEvent struct {
	ExperimentID int64
	SpaceID      int64

	CalculateMode     CalculateMode
	SpecificFieldInfo *SpecificFieldInfo
}

type SpecificFieldInfo struct {
	FieldKey  string
	FieldType FieldType
}

func (e *AggrCalculateEvent) GetFieldKey() string {
	if e.SpecificFieldInfo == nil {
		return ""
	}

	return e.SpecificFieldInfo.FieldKey
}

func (e *AggrCalculateEvent) GetFieldType() FieldType {
	if e.SpecificFieldInfo == nil {
		return 0
	}

	return e.SpecificFieldInfo.FieldType
}

// OnlineExptTurnEvalResult 定义在线实验轮次评估结果结构体
type OnlineExptTurnEvalResult struct {
	EvaluatorVersionId int64              `json:"evaluator_version_id"`
	EvaluatorRecordId  int64              `json:"evaluator_record_id"`
	Score              float64            `json:"score"`
	Reasoning          string             `json:"reasoning"`
	Status             int32              `json:"status"`
	EvaluatorRunError  *EvaluatorRunError `json:"evaluator_run_error"`
	Ext                map[string]string  `json:"ext"`

	BaseInfo *BaseInfo `json:"base_info"`
}

// OnlineExptEvalResultEvent 定义在线实验评估结果事件结构体
type OnlineExptEvalResultEvent struct {
	ExptId          int64                       `json:"expt_id,omitempty"`
	TurnEvalResults []*OnlineExptTurnEvalResult `json:"turn_eval_results,omitempty"`
}

type EvaluatorRecordCorrectionEvent struct {
	EvaluatorResult    *EvaluatorResult  `json:"evaluator_result,omitempty"`
	EvaluatorRecordID  int64             `json:"evaluator_record_id"`
	EvaluatorVersionID int64             `json:"evaluator_version_id"`
	Ext                map[string]string `json:"ext,omitempty"`

	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

type UpsertExptTurnResultFilterType string

const (
	UpsertExptTurnResultFilterTypeAuto   UpsertExptTurnResultFilterType = "auto"
	UpsertExptTurnResultFilterTypeCheck  UpsertExptTurnResultFilterType = "check"
	UpsertExptTurnResultFilterTypeManual UpsertExptTurnResultFilterType = "manual"
)

type ExptTurnResultFilterEvent struct {
	ExperimentID int64
	SpaceID      int64
	ItemID       []int64

	RetryTimes *int32
	FilterType *UpsertExptTurnResultFilterType
}
