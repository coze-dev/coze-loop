// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

type EvaluatorRecord struct {
	ID                  int64                `json:"id"`
	SpaceID             int64                `json:"space_id"`
	ExperimentID        int64                `json:"experiment_id"`
	ExperimentRunID     int64                `json:"experiment_run_id"`
	ItemID              int64                `json:"item_id"`
	TurnID              int64                `json:"turn_id"`
	EvaluatorVersionID  int64                `json:"evaluator_version_id"`
	TraceID             string               `json:"trace_id"`
	LogID               string               `json:"log_id"`
	EvaluatorInputData  *EvaluatorInputData  `json:"evaluator_input_data"`
	EvaluatorOutputData *EvaluatorOutputData `json:"evaluator_output_data"`
	Status              EvaluatorRunStatus   `json:"status"`
	BaseInfo            *BaseInfo            `json:"base_info"`

	Ext map[string]string `json:"ext,omitempty"`
}

type EvaluatorInputData struct {
	HistoryMessages []*Message          `json:"history_messages,omitempty"`
	InputFields     map[string]*Content `json:"input_fields,omitempty"`
}

type EvaluatorOutputData struct {
	EvaluatorResult   *EvaluatorResult   `json:"evaluator_result,omitempty"`
	EvaluatorUsage    *EvaluatorUsage    `json:"evaluator_usage,omitempty"`
	EvaluatorRunError *EvaluatorRunError `json:"evaluator_run_error,omitempty"`
	TimeConsumingMS   int64              `json:"time_consuming_ms,omitempty"`
}

type Correction struct {
	Score     *float64 `json:"score,omitempty"`
	Explain   string   `json:"explain,omitempty"`
	UpdatedBy string   `json:"updated_by,omitempty"`
}

type EvaluatorResult struct {
	Score      *float64    `json:"score,omitempty"`
	Correction *Correction `json:"correction,omitempty"`
	Reasoning  string      `json:"reasoning,omitempty"`
}

type EvaluatorUsage struct {
	InputTokens  int64 `json:"input_tokens,omitempty"`
	OutputTokens int64 `json:"output_tokens,omitempty"`
}

type EvaluatorRunError struct {
	Code    int32  `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type EvaluatorRunStatus int64

const (
	EvaluatorRunStatusUnknown EvaluatorRunStatus = 0
	EvaluatorRunStatusSuccess EvaluatorRunStatus = 1
	EvaluatorRunStatusFail    EvaluatorRunStatus = 2
)

func (e *EvaluatorRecord) GetBaseInfo() *BaseInfo {
	return e.BaseInfo
}

func (e *EvaluatorRecord) SetBaseInfo(info *BaseInfo) {
	e.BaseInfo = info
}

func (e *EvaluatorRecord) GetScore() *float64 {
	if e.EvaluatorOutputData == nil || e.EvaluatorOutputData.EvaluatorResult == nil {
		return nil
	}
	if e.EvaluatorOutputData.EvaluatorResult.Correction != nil {
		return e.EvaluatorOutputData.EvaluatorResult.Correction.Score
	}
	return e.EvaluatorOutputData.EvaluatorResult.Score
}

func (e *EvaluatorRecord) GetReasoning() string {
	if e.EvaluatorOutputData == nil || e.EvaluatorOutputData.EvaluatorResult == nil {
		return ""
	}
	if e.EvaluatorOutputData.EvaluatorResult.Correction != nil {
		return e.EvaluatorOutputData.EvaluatorResult.Correction.Explain
	}
	return e.EvaluatorOutputData.EvaluatorResult.Reasoning
}

func (e *EvaluatorRecord) GetCorrected() bool {
	if e.EvaluatorOutputData == nil || e.EvaluatorOutputData.EvaluatorResult == nil {
		return false
	}
	return e.EvaluatorOutputData.EvaluatorResult.Correction != nil
}
