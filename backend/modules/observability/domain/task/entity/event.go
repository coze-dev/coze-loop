// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

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
