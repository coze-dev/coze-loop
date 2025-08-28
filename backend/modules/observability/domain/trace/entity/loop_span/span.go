// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package loop_span

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/conv"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"github.com/pkg/errors"
	"github.com/samber/lo"
)

const (
	SpanFieldStartTime               = "start_time"
	SpanFieldSpanId                  = "span_id"
	SpanFieldSpanName                = "span_name"
	SpanFieldTraceId                 = "trace_id"
	SpanFieldSpaceId                 = "space_id"
	SpanFieldParentID                = "parent_id"
	SpanFieldSpanType                = "span_type"
	SpanFieldCallType                = "call_type"
	SpanFieldPSM                     = "psm"
	SpanFieldLogID                   = "logid"
	SpanFieldInput                   = "input"
	SpanFieldOutput                  = "output"
	SpanFieldMethod                  = "method"
	SpanFieldModelProvider           = "model_provider"
	SpanFieldInputTokens             = "input_tokens"
	SpanFieldOutputTokens            = "output_tokens"
	SpanFieldTokens                  = "tokens"
	SpanFieldStatus                  = "status"
	SpanFieldStatusCode              = "status_code"
	SpanFieldDuration                = "duration"
	SpanFieldObjectStorage           = "object_storage"
	SpanFieldStartTimeFirstResp      = "start_time_first_resp"
	SpanFieldLatencyFirstResp        = "latency_first_resp"
	SpanFieldStartTimeFirstTokenResp = "start_time_first_token_resp"
	SpanFieldLatencyFirstTokenResp   = "latency_first_token_resp"
	SpanFieldReasoningDuration       = "reasoning_duration"
	SpanFieldLogicDeleteDate         = "logic_delete_date"
	SpanFieldMessageID               = "message_id"
	SpanFieldUserID                  = "user_id"
	SpanFieldPromptKey               = "prompt_key"
	SpanFieldTenant                  = "tenant"

	SpanTypePrompt          = "prompt"
	SpanTypeModel           = "model"
	SpanTypeParser          = "parser"
	SpanTypeEmbedding       = "embedding"
	SpanTypeMemory          = "memory"
	SpanTypePlugin          = "plugin"
	SpanTypeFunction        = "function"
	SpanTypeGraph           = "graph"
	SpanTypeRemote          = "remote"
	SpanTypeLoader          = "loader"
	SpanTypeTransformer     = "transformer"
	SpanTypeVectorStore     = "vector_store"
	SpanTypeVectorRetriever = "vector_retriever"
	SpanTypeAgent           = "agent"
	SpanTypeLLMCall         = "LLMCall"

	SpanStatusSuccess = "success"
	SpanStatusError   = "error"

	MaxTagLength       = 50
	MaxKeySize         = 100
	MaxTextSize        = 1024 * 1024
	MaxCommonValueSize = 1024
)

type TTL string

const (
	TTL3d   TTL = "3d"
	TTL7d   TTL = "7d"
	TTL30d  TTL = "30d"
	TTL90d  TTL = "90d"
	TTL180d TTL = "180d"
	TTL365d TTL = "365d"
)

var TimeTagSlice = []string{
	SpanFieldStartTimeFirstResp,
	SpanFieldLatencyFirstResp,
	SpanFieldStartTimeFirstTokenResp,
	SpanFieldLatencyFirstTokenResp,
	SpanFieldReasoningDuration,
	SpanFieldLogicDeleteDate,
}

type SpanList []*Span

type Span struct {
	StartTime      int64  `json:"start_time"` // us
	SpanID         string `json:"span_id"`
	ParentID       string `json:"parent_id"`
	TraceID        string `json:"trace_id"`
	DurationMicros int64  `json:"duration_micros"` // us
	CallType       string `json:"call_type"`
	PSM            string `json:"psm"`
	LogID          string `json:"log_id"`
	WorkspaceID    string `json:"space_id"`
	SpanName       string `json:"span_name"`
	SpanType       string `json:"span_type"`
	Method         string `json:"method"`
	StatusCode     int32  `json:"status_code"`
	Input          string `json:"input"`
	Output         string `json:"output"`
	ObjectStorage  string `json:"object_storage"`

	SystemTagsString map[string]string  `json:"system_tags_string"`
	SystemTagsLong   map[string]int64   `json:"system_tags_long"`
	SystemTagsDouble map[string]float64 `json:"system_tags_double"`

	TagsString map[string]string  `json:"tags_string"`
	TagsLong   map[string]int64   `json:"tags_long"`
	TagsDouble map[string]float64 `json:"tags_double"`

	TagsBool map[string]bool   `json:"tags_bool"`
	TagsByte map[string]string `json:"tags_byte"`

	AttrTos         *AttrTos       `json:"-"`
	LogicDeleteTime int64          `json:"-"` // us
	Annotations     AnnotationList `json:"-"`
}

type ObjectStorage struct {
	InputTosKey  string        `json:"input_tos_key"`
	OutputTosKey string        `json:"output_tos_key"`
	Attachments  []*Attachment `json:"Attachments"`
}

type Attachment struct {
	Field  string `json:"field"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	TosKey string `json:"tos_key"`
}

type TraceAdvanceInfo struct {
	TraceId    string
	InputCost  int64
	OutputCost int64
}

type AttrTos struct {
	InputDataURL   string
	OutputDataURL  string
	MultimodalData map[string]string
}

func (s *Span) GetSystemTags() map[string]string {
	systemTags := make(map[string]string)
	for k, v := range s.SystemTagsString {
		systemTags[k] = v
	}
	for k, v := range s.SystemTagsLong {
		systemTags[k] = strconv.FormatInt(v, 10)
	}
	for k, v := range s.SystemTagsDouble {
		vStr := strconv.FormatFloat(v, 'f', -1, 64)
		systemTags[k] = vStr
	}
	return systemTags
}

func (s *Span) GetCustomTags() map[string]string {
	ret := make(map[string]string)
	tags := s.getTags()
	for _, tag := range tags {
		if tag == nil {
			continue
		}
		tagStr, err := tag.GetStringValue()
		if err != nil {
			continue
		}
		ret[tag.GetKey()] = tagStr
	}
	return ret
}

func (s *Span) getTags() []*Tag {
	tags := make([]*Tag, 0)
	for k, v := range s.TagsString {
		tags = append(tags, SetTag(k, TagValueTypeString, v))
	}
	for k, v := range s.TagsLong {
		tags = append(tags, SetTag(k, TagValueTypeInt64, v))
	}
	for k, v := range s.TagsDouble {
		tags = append(tags, SetTag(k, TagValueTypeFloat64, v))
	}
	for k, v := range s.TagsBool {
		tags = append(tags, SetTag(k, TagValueTypeBool, v))
	}
	for k, v := range s.TagsByte {
		tags = append(tags, SetTag(k, TagValueTypeString, v))
	}
	return tags
}

func (s *Span) getTokens(ctx context.Context) (inputTokens, outputTokens int64, err error) {
	inputToken := s.TagsLong[SpanFieldInputTokens]
	outputToken := s.TagsLong[SpanFieldOutputTokens]
	return inputToken, outputToken, nil
}

// filter使用, 当前只支持特定参数,后续有需要可拓展到其他参数
func (s *Span) GetFieldValue(fieldName string) any {
	switch fieldName {
	case SpanFieldStartTime:
		return s.StartTime
	case SpanFieldDuration:
		return s.DurationMicros
	case SpanFieldSpanId:
		return s.SpanID
	case SpanFieldParentID:
		return s.ParentID
	case SpanFieldCallType:
		return s.CallType
	case SpanFieldSpanType:
		return s.SpanType
	case SpanFieldInput:
		return s.Input
	case SpanFieldOutput:
		return s.Output
	case SpanFieldTraceId:
		return s.TraceID
	case SpanFieldSpanName:
		return s.SpanName
	case SpanFieldSpaceId:
		return s.WorkspaceID
	case SpanFieldPSM:
		return s.PSM
	case SpanFieldLogID:
		return s.LogID
	case SpanFieldStatusCode:
		return s.StatusCode
	case SpanFieldObjectStorage:
		return s.ObjectStorage
	case SpanFieldMethod:
		return s.Method
	}
	if val, ok := s.TagsString[fieldName]; ok {
		return val
	} else if val, ok := s.TagsLong[fieldName]; ok {
		return val
	} else if val, ok := s.TagsDouble[fieldName]; ok {
		return val
	} else if val, ok := s.TagsBool[fieldName]; ok {
		return val
	} else if val, ok := s.TagsByte[fieldName]; ok {
		return val
	}
	return nil
}

func (s *Span) IsValidSpan() error {
	if s == nil {
		return fmt.Errorf("nil span")
	}
	if len(s.TraceID) != 32 || s.TraceID == "00000000000000000000000000000000" {
		return fmt.Errorf("invalid trace_id: %s", s.TraceID)
	}
	for _, c := range s.TraceID {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') { //nolint:staticcheck,QF1001
			return fmt.Errorf("invalid trace_id: %s", s.TraceID)
		}
	}
	if len(s.SpanID) != 16 || s.SpanID == "0000000000000000" {
		return fmt.Errorf("invalid span_id: %s", s.SpanID)
	}
	for _, c := range s.SpanID {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') { //nolint:staticcheck,QF1001
			return fmt.Errorf("invalid span_id: %s", s.SpanID)
		}
	}
	if s.StartTime > time.Now().Add(time.Hour).UnixMicro() || s.StartTime < time.Now().Add(-time.Hour*24).UnixMicro() {
		return fmt.Errorf("invalid start time: %d", s.StartTime)
	}
	if s.DurationMicros < 0 {
		s.DurationMicros = 0
	}
	if s.TagsLong != nil {
		if s.TagsLong[SpanFieldTokens] < 0 {
			s.TagsLong[SpanFieldTokens] = 0
		}
		if s.TagsLong[SpanFieldStartTimeFirstResp] < 0 {
			s.TagsLong[SpanFieldStartTimeFirstResp] = 0
		}
		if s.TagsLong[SpanFieldLatencyFirstResp] < 0 {
			s.TagsLong[SpanFieldLatencyFirstResp] = 0
		}
		if s.TagsLong[SpanFieldStartTimeFirstTokenResp] < 0 {
			s.TagsLong[SpanFieldStartTimeFirstTokenResp] = 0
		}
		if s.TagsLong[SpanFieldLatencyFirstTokenResp] < 0 {
			s.TagsLong[SpanFieldLatencyFirstTokenResp] = 0
		}
		if s.TagsLong[SpanFieldReasoningDuration] < 0 {
			s.TagsLong[SpanFieldReasoningDuration] = 0
		}
	}
	s.ClipSpan()
	return nil
}

func (s *Span) GetTenant() string {
	return s.SystemTagsString[SpanFieldTenant]
}

func (s *Span) GetTTL(ctx context.Context) TTL {
	tStart := time.UnixMicro(s.StartTime)
	tEnd := time.UnixMicro(s.LogicDeleteTime)
	if s.DurationMicros > 0 {
		tStart = time.UnixMicro(s.StartTime + s.DurationMicros)
	}
	duration := tEnd.Sub(tStart)
	days := int64(duration.Hours() / 24)
	ttl := TTLFromInteger(days)
	logs.CtxInfo(ctx, "get ttl for span_id %s is %s", s.SpanID, ttl)
	return ttl
}

func (s *Span) BuildFeedback(t AnnotationType, key string, value AnnotationValue, reasoning, userID string, deleted bool) (*Annotation, error) {
	a := &Annotation{
		SpanID:         s.SpanID,
		TraceID:        s.TraceID,
		StartTime:      time.UnixMicro(s.StartTime),
		WorkspaceID:    s.WorkspaceID,
		AnnotationType: t,
		Key:            key,
		Value:          value,
		Reasoning:      reasoning,
		Status:         AnnotationStatusNormal,
		CreatedAt:      time.Now(),
		CreatedBy:      userID,
		UpdatedAt:      time.Now(),
		UpdatedBy:      userID,
		IsDeleted:      deleted,
	}
	if deleted {
		a.Status = AnnotationStatusDeleted
	}
	if err := a.GenID(); err != nil {
		return nil, fmt.Errorf("fail to generate annotation id: %v", err)
	}
	return a, nil
}

func (s *Span) AddAnnotation(annotation *Annotation) {
	if s.Annotations == nil {
		s.Annotations = make([]*Annotation, 0)
	}
	s.Annotations = append(s.Annotations, annotation)
}

func (s *Span) AddManualDatasetAnnotation(datasetID int64, userID string, annotationType AnnotationType) (*Annotation, error) {
	a := &Annotation{}
	a.SpanID = s.SpanID
	a.TraceID = s.TraceID
	a.StartTime = time.UnixMicro(s.StartTime)
	a.WorkspaceID = s.WorkspaceID
	a.AnnotationType = annotationType
	a.Key = strconv.FormatInt(datasetID, 10)
	a.Value = NewBoolValue(true)
	a.Metadata = &ManualDatasetMetadata{}
	a.Status = AnnotationStatusNormal
	a.CreatedAt = time.Now()
	a.CreatedBy = userID
	a.UpdatedAt = time.Now()
	a.UpdatedBy = userID

	if err := a.GenID(); err != nil {
		return nil, err
	}

	s.AddAnnotation(a)
	return a, nil
}

func (s *Span) ExtractByJsonpath(ctx context.Context, key string, jsonpath string) (string, error) {
	jsonpath = strings.TrimPrefix(jsonpath, key)
	jsonpath = strings.TrimPrefix(jsonpath, ".")
	data := ""
	if key == "Input" {
		data = s.Input
	} else if key == "Output" {
		data = s.Output
	} else if strings.HasPrefix(key, "Tags.") {
		key = strings.TrimPrefix(key, "Tags.")
		tag := s.GetFieldValue(key)
		data = conv.ToString(tag)
	} else {
		return "", errors.Errorf("unsupported mapping key: %s", key)
	}

	if data == "" {
		return "", nil
	}
	if jsonpath == "" {
		return data, nil
	}

	return json.GetStringByJSONPathRecursively(data, jsonpath)
}

func validField(clipFields *[]string, key, value string) string {
	if key == SpanFieldInput || key == SpanFieldOutput {
		if len(value) > MaxTextSize {
			*clipFields = append(*clipFields, key)
			return value[:MaxTextSize]
		}
	} else if len(value) > MaxCommonValueSize {
		*clipFields = append(*clipFields, key)
		return value[:MaxCommonValueSize]
	}
	return value
}

func validSystemTag[M any](clipFields *[]string, tag map[string]M) {
	toAddSystemTags := make(map[string]interface{})
	for k, v := range tag {
		if !SystemTagKeys[k] {
			delete(tag, k)
			continue
		}
		if VStr, ok := any(v).(string); ok {
			if len(VStr) > MaxCommonValueSize {
				tag[k] = any(VStr[:MaxCommonValueSize]).(M)
				*clipFields = append(*clipFields, "system_tag_string_"+k)
			}
		}
		if len(k) > MaxKeySize {
			toAddSystemTags[k[:MaxKeySize]] = tag[k]
			*clipFields = append(*clipFields, "system_tag_string_"+k)
			delete(tag, k)
		}
	}
	for key := range toAddSystemTags {
		tag[key] = toAddSystemTags[key].(M)
	}
}

func validTag[M any](clipFields *[]string, tag map[string]M, count int) int {
	toAddSystemTags := make(map[string]interface{})
	for k, v := range tag {
		if count > MaxTagLength {
			delete(tag, k)
			continue
		}
		count++
		if VStr, ok := any(v).(string); ok {
			if len(VStr) > MaxCommonValueSize {
				tag[k] = any(VStr[:MaxCommonValueSize]).(M)
				*clipFields = append(*clipFields, "system_tag_string_"+k)
			}
		}
		if len(k) > MaxKeySize {
			toAddSystemTags[k[:MaxKeySize]] = tag[k]
			*clipFields = append(*clipFields, "system_tag_string_"+k)
			delete(tag, k)
		}
	}
	for key := range toAddSystemTags {
		tag[key] = toAddSystemTags[key].(M)
	}
	return count
}

func (s *Span) ClipSpan() {
	clipFields := make([]string, 0)
	s.SpanID = validField(&clipFields, SpanFieldSpanId, s.SpanID)
	s.ParentID = validField(&clipFields, SpanFieldParentID, s.ParentID)
	s.TraceID = validField(&clipFields, SpanFieldTraceId, s.TraceID)
	s.CallType = validField(&clipFields, SpanFieldCallType, s.CallType)
	s.WorkspaceID = validField(&clipFields, SpanFieldSpaceId, s.WorkspaceID)
	s.SpanName = validField(&clipFields, SpanFieldSpanName, s.SpanName)
	s.SpanType = validField(&clipFields, SpanFieldSpanType, s.SpanType)
	s.Method = validField(&clipFields, SpanFieldMethod, s.Method)
	s.Input = validField(&clipFields, SpanFieldInput, s.Input)
	s.Output = validField(&clipFields, SpanFieldOutput, s.Output)
	s.ObjectStorage = validField(&clipFields, SpanFieldObjectStorage, s.ObjectStorage)

	validSystemTag(&clipFields, s.SystemTagsString)
	validSystemTag(&clipFields, s.SystemTagsDouble)
	validSystemTag(&clipFields, s.SystemTagsLong)

	totalCount := 0
	totalCount = validTag(&clipFields, s.TagsString, totalCount)
	totalCount = validTag(&clipFields, s.TagsByte, totalCount)
	totalCount = validTag(&clipFields, s.TagsDouble, totalCount)
	totalCount = validTag(&clipFields, s.TagsLong, totalCount)
	_ = validTag(&clipFields, s.TagsBool, totalCount)

	clipFieldsStr, _ := sonic.MarshalString(clipFields)
	if s.SystemTagsString == nil {
		s.SystemTagsString = map[string]string{
			"clip_fields": clipFieldsStr,
		}
	} else {
		s.SystemTagsString["clip_fields"] = clipFieldsStr
	}
}

func (s SpanList) Stat(ctx context.Context) (inputTokens, outputTokens int64, err error) {
	modelSpans := s.FilterModelSpans()
	for _, v := range modelSpans {
		in, out, err := v.getTokens(ctx)
		if err != nil {
			return -1, -1, err
		}
		inputTokens += in
		outputTokens += out
	}
	return
}

func (s SpanList) FilterSpans(f *FilterFields) SpanList {
	ret := make(SpanList, 0)
	for _, span := range s {
		if f.Satisfied(span) {
			ret = append(ret, span)
		}
	}
	return ret
}

func (s SpanList) FilterModelSpans() SpanList {
	if len(s) == 0 {
		return s
	}
	modelFilter := &FilterFields{
		QueryAndOr: ptr.Of(QueryAndOrEnumOr),
		FilterFields: []*FilterField{
			{
				FieldName: SpanFieldSpanType,
				FieldType: FieldTypeString,
				Values:    []string{"LLMCall"},
				QueryType: ptr.Of(QueryTypeEnumEq),
			},
			{
				FieldName:  SpanFieldSpanType,
				FieldType:  FieldTypeString,
				Values:     []string{"model"},
				QueryType:  ptr.Of(QueryTypeEnumEq),
				QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
				SubFilter: &FilterFields{
					QueryAndOr: ptr.Of(QueryAndOrEnumAnd),
					FilterFields: []*FilterField{
						{
							FieldName: SpanFieldModelProvider,
							FieldType: FieldTypeString,
							Values:    []string{"LLMGateway", "llm_gateway"},
							QueryType: ptr.Of(QueryTypeEnumNotIn),
						},
					},
				},
			},
		},
	}
	return s.FilterSpans(modelFilter)
}

func (s SpanList) SortByStartTime(desc bool) {
	if len(s) == 0 {
		return
	}
	sortByStartTime := func(i, j int) bool {
		if desc {
			return s[i].StartTime > s[j].StartTime
		}
		return s[i].StartTime < s[j].StartTime
	}
	sort.Slice(s, sortByStartTime)
}

func (s SpanList) SetAnnotations(annotations AnnotationList) {
	// spanId&traceId
	annotationMap := make(map[string]map[string]AnnotationList)
	for _, anno := range annotations {
		if annotationMap[anno.SpanID] == nil {
			annotationMap[anno.SpanID] = make(map[string]AnnotationList)
		}
		annotationMap[anno.SpanID][anno.TraceID] = append(annotationMap[anno.SpanID][anno.TraceID], anno)
	}
	for i := range s {
		s[i].Annotations = annotationMap[s[i].SpanID][s[i].TraceID]
	}
}

func (s SpanList) GetUserIDs() []string {
	ret := make([]string, 0)
	for _, span := range s {
		ret = append(ret, span.Annotations.GetUserIDs()...)
	}
	return lo.Uniq(ret)
}

func (s SpanList) GetAnnotationTagIDs() []string {
	ret := make([]string, 0)
	for _, span := range s {
		ret = append(ret, span.Annotations.GetAnnotationTagIDs()...)
	}
	return lo.Uniq(ret)
}

func (s SpanList) GetEvaluatorVersionIDs() []int64 {
	ret := make([]int64, 0)
	for _, span := range s {
		ret = append(ret, span.Annotations.GetEvaluatorVersionIDs()...)
	}
	return lo.Uniq(ret)
}

func (s SpanList) Uniq() SpanList {
	return lo.UniqBy(s, func(item *Span) string {
		return fmt.Sprintf("%s_%s", item.SpanID, item.TraceID)
	})
}

func TTLFromInteger(i int64) TTL {
	if i <= 4 {
		return TTL3d
	} else if i <= 8 {
		return TTL7d
	} else if i <= 31 {
		return TTL30d
	} else if i <= 91 {
		return TTL90d
	} else if i <= 181 {
		return TTL180d
	} else {
		return TTL365d
	}
}

var SystemTagKeys = map[string]bool{
	"dc":           true,
	"pod_name":     true,
	"cluster":      true,
	"deploy_stage": true,
	"env":          true,
	"language":     true,
	"runtime":      true,
	"cut_off":      true,
}
