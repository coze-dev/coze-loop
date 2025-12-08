// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"fmt"
	"strconv"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
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
	Tags          map[string]any    `json:"_tags"`        // value can be: [float64, int64, bool, string, []byte]
	SystemTags    map[string]any    `json:"_system_tags"` // value can be: [float64, int64, bool, string, []byte]
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
	systemTagsString := make(map[string]string)
	systemTagsLong := make(map[string]int64)
	systemTagsDouble := make(map[string]float64)
	tagsString := make(map[string]string)
	tagsLong := make(map[string]int64)
	tagsDouble := make(map[string]float64)
	tagsBool := make(map[string]bool)
	tagsByte := make(map[string]string)
	for k, v := range s.Tags {
		switch val := v.(type) {
		case string:
			tagsString[k] = val
		case int64:
			tagsLong[k] = val
		case float64:
			tagsDouble[k] = val
		case bool:
			tagsBool[k] = val
		case []byte:
			tagsByte[k] = string(val)
		default:
			tagsString[k] = ""
		}
	}
	for k, v := range s.SystemTags {
		switch val := v.(type) {
		case string:
			systemTagsString[k] = val
		case int64:
			systemTagsLong[k] = val
		case float64:
			systemTagsDouble[k] = val
		default:
			systemTagsString[k] = ""
		}
	}
	if s.SensitiveTags != nil {
		tagsLong["input_tokens"] = s.SensitiveTags.InputTokens
		tagsLong["output_tokens"] = s.SensitiveTags.OutputTokens
		tagsLong["tokens"] = s.SensitiveTags.Tokens
	}
	if s.Tags == nil {
		s.Tags = make(map[string]any)
	}
	spaceID := tagsString["fornax_space_id"]
	callType := tagsString["call_type"]
	spanType := tagsString["span_type"]

	result := &loop_span.Span{
		StartTime:        s.StartTimeInUs,
		SpanID:           s.SpanID,
		ParentID:         s.ParentID,
		LogID:            s.LogID,
		TraceID:          s.TraceID,
		DurationMicros:   s.DurationInUs,
		CallType:         callType,
		WorkspaceID:      spaceID,
		SpanName:         s.SpanName,
		SpanType:         spanType,
		Method:           s.Method,
		StatusCode:       s.StatusCode,
		SystemTagsString: systemTagsString,
		SystemTagsLong:   systemTagsLong,
		SystemTagsDouble: systemTagsDouble,
		TagsString:       tagsString,
		TagsLong:         tagsLong,
		TagsDouble:       tagsDouble,
		TagsBool:         tagsBool,
		TagsByte:         tagsByte,
	}
	if s.ServerEnv != nil {
		result.PSM = s.ServerEnv.PSM
	}
	if s.SensitiveTags != nil {
		result.Input = s.SensitiveTags.Input
		result.Output = s.SensitiveTags.Output
	}

	return result
}

type AutoEvalEvent struct {
	ExptID          int64                       `json:"expt_id"`
	TurnEvalResults []*OnlineExptTurnEvalResult `json:"turn_eval_results"`
}

func (e *AutoEvalEvent) Validate() error {
	if len(e.TurnEvalResults) == 0 {
		return fmt.Errorf("turn_eval_results is required")
	}
	return nil
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

func (s *OnlineExptTurnEvalResult) GetSpanIDFromExt() string {
	if s == nil {
		return ""
	}
	return s.Ext["span_id"]
}

func (s *OnlineExptTurnEvalResult) GetTraceIDFromExt() string {
	if s == nil {
		return ""
	}
	return s.Ext["trace_id"]
}

func (s *OnlineExptTurnEvalResult) GetStartTimeFromExt() int64 {
	if s == nil {
		return 0
	}
	timeStr := ""
	if spanStartTime, ok := s.Ext["span_start_time"]; ok {
		timeStr = spanStartTime
	} else {
		timeStr = s.Ext["start_time"]
	}
	startTime, err := strconv.ParseInt(timeStr, 10, 64)
	if err != nil {
		return 0
	}
	return startTime
}

func (s *OnlineExptTurnEvalResult) GetTaskIDFromExt() int64 {
	if s == nil {
		return 0
	}
	taskIDStr := s.Ext["task_id"]
	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil {
		return 0
	}
	return taskID
}

func (s *OnlineExptTurnEvalResult) GetWorkspaceIDFromExt() (string, int64) {
	if s == nil {
		return "", 0
	}
	workspaceIDStr := s.Ext["workspace_id"]
	workspaceID, err := strconv.ParseInt(workspaceIDStr, 10, 64)
	if err != nil {
		return "", 0
	}
	return workspaceIDStr, workspaceID
}

func (s *OnlineExptTurnEvalResult) GetRunID() (int64, error) {
	taskRunIDStr := s.Ext["run_id"]
	if taskRunIDStr == "" {
		return 0, fmt.Errorf("run_id not found in ext")
	}

	return strconv.ParseInt(taskRunIDStr, 10, 64)
}

func (s *OnlineExptTurnEvalResult) GetUserID() string {
	if s.BaseInfo == nil || s.BaseInfo.CreatedBy == nil {
		return ""
	}
	return s.BaseInfo.CreatedBy.UserID
}

func (s *OnlineExptTurnEvalResult) GetPlatformType() (loop_span.PlatformType, bool) {
	if platform, ok := s.Ext["platform_type"]; ok {
		return loop_span.PlatformType(platform), ok
	}
	return loop_span.PlatformCallbackAll, false
}

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

func (c *CorrectionEvent) Validate() error {
	if c.EvaluatorRecordID == 0 {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("evaluator_record_id is empty"))
	}
	if c.EvaluatorResult == nil || c.EvaluatorResult.Correction == nil {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("correction is empty"))
	}
	return nil
}

func (c *CorrectionEvent) GetSpanIDFromExt() string {
	if c == nil {
		return ""
	}
	return c.Ext["span_id"]
}

func (c *CorrectionEvent) GetTraceIDFromExt() string {
	if c == nil {
		return ""
	}
	return c.Ext["trace_id"]
}

func (c *CorrectionEvent) GetStartTimeFromExt() int64 {
	if c == nil {
		return 0
	}
	timeStr := ""
	if spanStartTime, ok := c.Ext["span_start_time"]; ok {
		timeStr = spanStartTime
	} else {
		timeStr = c.Ext["start_time"]
	}
	startTime, err := strconv.ParseInt(timeStr, 10, 64)
	if err != nil {
		return 0
	}
	return startTime
}

func (c *CorrectionEvent) GetTaskIDFromExt() int64 {
	if c == nil {
		return 0
	}
	taskIDStr := c.Ext["task_id"]
	taskID, err := strconv.ParseInt(taskIDStr, 10, 64)
	if err != nil {
		return 0
	}
	return taskID
}

func (c *CorrectionEvent) GetWorkspaceIDFromExt() (string, int64) {
	if c == nil {
		return "", 0
	}
	workspaceIDStr := c.Ext["workspace_id"]
	workspaceID, err := strconv.ParseInt(workspaceIDStr, 10, 64)
	if err != nil {
		return "", 0
	}
	return workspaceIDStr, workspaceID
}

func (c *CorrectionEvent) GetUpdateBy() string {
	if c == nil || c.EvaluatorResult == nil || c.EvaluatorResult.Correction == nil {
		return ""
	}
	return c.EvaluatorResult.Correction.UpdatedBy
}

func (c *CorrectionEvent) GetPlatfromType() (loop_span.PlatformType, bool) {
	if platform, ok := c.Ext["platform_type"]; ok {
		return loop_span.PlatformType(platform), ok
	}
	return loop_span.PlatformCallbackAll, false
}

type BackFillEvent struct {
	SpaceID int64 `json:"space_id"`
	TaskID  int64 `json:"task_id"`
}

func (b *BackFillEvent) Validate() error {
	if b.SpaceID == 0 {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("space_id is empty"))
	}
	if b.TaskID == 0 {
		return errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("task_id is empty"))
	}
	return nil
}
