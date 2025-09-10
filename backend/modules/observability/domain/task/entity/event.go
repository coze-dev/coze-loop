// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

type RawSpan struct {
	TraceID       string            `json:"_trace_id"`
	LogID         string            `json:"__logid"`
	Method        string            `json:"_method"`
	SpanID        string            `json:"_span_id"`
	ParentID      string            `json:"_parent_id"`
	Events        []*EventInRawSpan `json:"_events"`
	DurationInUs  int64             `json:"_duration"`   // unit: microsecond
	StartTimeInUs int64             `json:"_start_time"` // unix microsecond
	StatusCode    int32             `json:"_status_code"`
	SpanName      string            `json:"_span_name"`
	SpanType      string            `json:"_span_type"`
	ServerEnv     *ServerInRawSpan  `json:"_server_env"`
	Tags          map[string]any    `json:"_tags"` // value can be: [float64, int64, bool, string, []byte]
	Tenant        string            `json:"tenant"`
	SensitiveTags *SensitiveTags    `json:"sensitive_tags"`
}
type EventInRawSpan struct {
	Type      string        `json:"_type,omitempty"`
	Name      string        `json:"_name,omitempty"`
	Tags      []*RawSpanTag `json:"_tags,omitempty"`
	StartTime int64         `json:"_start_time,omitempty"`
	Data      []byte        `json:"_data,omitempty"`
}
type RawSpanTag struct {
	Key   string
	Value any // value can be: [float64, int64, bool, string, []byte]
}
type SensitiveTags struct {
	Input        string `json:"input"`
	Output       string `json:"output"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	Tokens       int64  `json:"tokens"`
}

type ServerInRawSpan struct {
	PSM     string `json:"psm,omitempty"`
	Cluster string `json:"cluster,omitempty"`
	DC      string `json:"dc,omitempty"`
	Env     string `json:"env,omitempty"`
	PodName string `json:"pod_name,omitempty"`
	Stage   string `json:"stage,omitempty"`
	Region  string `json:"_region,omitempty"`
}

var MockRawSpan = &RawSpan{
	TraceID:       "1",
	LogID:         "2",
	Method:        "3",
	SpanID:        "4",
	ParentID:      "0",
	DurationInUs:  0,
	StartTimeInUs: 0,
	StatusCode:    0,
	SpanName:      "xun_test",
	Tags: map[string]any{
		"span_type": "root",
		"tokens":    3,
		"input":     "世界上最美的火山",
		"output":    "富士山",
	},
	Tenant: "fornax_saas",
}

func (s *RawSpan) GetSensitiveTags() *SensitiveTags {
	if s == nil {
		return nil
	}
	return s.SensitiveTags
}
func (s *RawSpan) GetServerEnv() *ServerInRawSpan {
	if s == nil {
		return nil
	}
	return s.ServerEnv
}

func (s *RawSpan) RawSpanConvertToLoopSpan() *loop_span.Span {
	if s == nil {
		return nil
	}

	result := &loop_span.Span{
		StartTime:        s.StartTimeInUs / 1000,
		SpanID:           s.SpanID,
		ParentID:         s.ParentID,
		LogID:            s.LogID,
		TraceID:          s.TraceID,
		DurationMicros:   s.DurationInUs / 1000,
		PSM:              s.ServerEnv.PSM,
		CallType:         "",
		WorkspaceID:      s.Tags["fornax_space_id"].(string),
		SpanName:         s.SpanName,
		SpanType:         s.SpanType,
		Method:           s.Method,
		StatusCode:       s.StatusCode,
		Input:            s.SensitiveTags.Input,
		Output:           s.SensitiveTags.Output,
		ObjectStorage:    "",
		SystemTagsString: nil,
		SystemTagsLong:   nil,
		SystemTagsDouble: nil,
		TagsString:       nil,
		TagsLong:         nil,
		TagsDouble:       nil,
		TagsBool:         nil,
		TagsByte:         nil,
	}

	return result
}

type AutoEvalEvent struct {
	ExptID          int64                       `json:"expt_id"`
	TurnEvalResults []*OnlineExptTurnEvalResult `json:"turn_eval_results"`
}
type OnlineExptTurnEvalResult struct {
	EvaluatorVersionID int64              `json:"evaluator_version_id"`
	EvaluatorRecordID  int64              `json:"evaluator_record_id"`
	Score              float64            `json:"score"`
	Reasoning          string             `json:"reasoning"`
	Status             EvaluatorRunStatus `json:"status"`
	EvaluatorRunError  *EvaluatorRunError `json:"evaluator_run_error"`
	Ext                map[string]string  `json:"ext"`
	BaseInfo           *BaseInfo          `json:"base_info"`
}
type BaseInfo struct {
	UpdatedBy *UserInfo `json:"updated_by"`
	UpdatedAt int64     `json:"updated_at"`
	CreatedBy *UserInfo `json:"created_by"`
	CreatedAt int64     `json:"created_at"`
}
type UserInfo struct {
	UserID string `json:"user_id"`
}
type EvaluatorRunStatus int

const (
	EvaluatorRunStatus_Unknown = 0
	EvaluatorRunStatus_Success = 1
	EvaluatorRunStatus_Fail    = 2
)

type EvaluatorRunError struct {
	Code    int32  `json:"code"`
	Message string `json:"message"`
}

type Correction struct {
	Score     float64 `json:"score"`
	Explain   string  `json:"explain"`
	UpdatedBy string  `json:"updated_by"`
}

type EvaluatorResult struct {
	Score      float64     `json:"score"`
	Correction *Correction `json:"correction"`
	Reasoning  string      `json:"reasoning"`
}

type CorrectionEvent struct {
	EvaluatorResult    *EvaluatorResult  `json:"evaluator_result"`
	EvaluatorRecordID  int64             `json:"evaluator_record_id"`
	EvaluatorVersionID int64             `json:"evaluator_version_id"`
	Ext                map[string]string `json:"ext"`
	CreatedAt          int64             `json:"created_at"`
	UpdatedAt          int64             `json:"updated_at"`
}
