// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"strconv"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/common"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/slices"
	time_util "github.com/coze-dev/coze-loop/backend/pkg/time"
	"github.com/samber/lo"
)

func SpanDO2DTO(
	s *loop_span.Span,
	userMap map[string]*common.UserInfo,
	evalMap map[int64]*rpc.Evaluator,
	tagMap map[int64]*rpc.TagInfo,
) *span.OutputSpan {
	outSpan := &span.OutputSpan{
		TraceID:         s.TraceID,
		SpanID:          s.SpanID,
		ParentID:        s.ParentID,
		SpanName:        s.SpanName,
		SpanType:        s.SpanType,
		StartedAt:       time_util.MicroSec2MillSec(s.StartTime),      // to ms
		Duration:        time_util.MicroSec2MillSec(s.DurationMicros), // to ms
		StatusCode:      s.StatusCode,
		Input:           s.Input,
		Output:          s.Output,
		LogicDeleteDate: ptr.Of(time_util.MicroSec2MillSec(s.LogicDeleteTime)), // to ms
	}
	if s.PSM != "" {
		outSpan.ServiceName = ptr.Of(s.PSM)
	}
	if s.LogID != "" {
		outSpan.Logid = ptr.Of(s.LogID)
	}
	switch s.SpanType {
	case loop_span.SpanTypePrompt:
		outSpan.SetType(span.SpanTypePrompt)
	case loop_span.SpanTypeModel:
		outSpan.SetType(span.SpanTypeModel)
	case loop_span.SpanTypeParser:
		outSpan.SetType(span.SpanTypeParser)
	case loop_span.SpanTypeEmbedding:
		outSpan.SetType(span.SpanTypeEmbedding)
	case loop_span.SpanTypeMemory:
		outSpan.SetType(span.SpanTypeMemory)
	case loop_span.SpanTypePlugin:
		outSpan.SetType(span.SpanTypePlugin)
	case loop_span.SpanTypeFunction:
		outSpan.SetType(span.SpanTypeFunction)
	case loop_span.SpanTypeGraph:
		outSpan.SetType(span.SpanTypeGraph)
	case loop_span.SpanTypeRemote:
		outSpan.SetType(span.SpanTypeRemote)
	case loop_span.SpanTypeLoader:
		outSpan.SetType(span.SpanTypeLoader)
	case loop_span.SpanTypeTransformer:
		outSpan.SetType(span.SpanTypeTransformer)
	case loop_span.SpanTypeVectorStore:
		outSpan.SetType(span.SpanTypeVectorStore)
	case loop_span.SpanTypeVectorRetriever:
		outSpan.SetType(span.SpanTypeVectorRetriever)
	case loop_span.SpanTypeAgent:
		outSpan.SetType(span.SpanTypeAgent)
	case loop_span.SpanTypeLLMCall:
		outSpan.SetType(span.SpanTypeLLMCall)
	default:
		outSpan.SetType(span.SpanTypeUnknown)
	}
	outSpan.SetStatus(lo.Ternary[string](s.StatusCode == 0, span.SpanStatusSuccess, span.SpanStatusError))
	systemTags := s.GetSystemTags()
	customTags := s.GetCustomTags()
	if s.AttrTos != nil {
		outSpan.SetAttrTos(&span.AttrTos{
			InputDataURL:   ptr.Of(s.AttrTos.InputDataURL),
			OutputDataURL:  ptr.Of(s.AttrTos.OutputDataURL),
			MultimodalData: s.AttrTos.MultimodalData,
		})
	}
	for k, v := range systemTags {
		if slices.Contains(loop_span.TimeTagSlice, k) { // to ms
			integer, err := strconv.ParseInt(v, 10, 64)
			if err == nil {
				integer = time_util.MicroSec2MillSec(integer)
				systemTags[k] = strconv.FormatInt(integer, 10)
			}
		}
	}
	for k, v := range customTags {
		if slices.Contains(loop_span.TimeTagSlice, k) { // to ms
			integer, err := strconv.ParseInt(v, 10, 64)
			if err == nil {
				integer = time_util.MicroSec2MillSec(integer)
				customTags[k] = strconv.FormatInt(integer, 10)
			}
		}
	}
	outSpan.SetSystemTags(systemTags)
	outSpan.SetCustomTags(customTags)
	if s.Annotations != nil {
		annotationDTOList := AnnotationListDO2DTO(s.Annotations, userMap, evalMap, tagMap)
		if len(annotationDTOList) > 0 {
			outSpan.Annotations = annotationDTOList
		}
	}
	return outSpan
}

func SpanDTO2DO(span *span.InputSpan) *loop_span.Span {
	outSpan := &loop_span.Span{
		StartTime:        span.StartedAtMicros,
		SpanID:           span.SpanID,
		ParentID:         span.ParentID,
		TraceID:          span.TraceID,
		DurationMicros:   span.Duration,
		CallType:         ptr.From(span.CallType),
		WorkspaceID:      span.WorkspaceID,
		SpanName:         span.SpanName,
		SpanType:         span.SpanType,
		Method:           span.Method,
		StatusCode:       span.StatusCode,
		Input:            span.Input,
		Output:           span.Output,
		ObjectStorage:    ptr.From(span.ObjectStorage),
		SystemTagsString: span.SystemTagsString,
		SystemTagsLong:   span.SystemTagsLong,
		SystemTagsDouble: span.SystemTagsDouble,
		TagsString:       span.TagsString,
		TagsLong:         span.TagsLong,
		TagsDouble:       span.TagsDouble,
		TagsBool:         span.TagsBool,
		TagsByte:         span.TagsBytes,
	}
	if span.DurationMicros != nil {
		outSpan.DurationMicros = *span.DurationMicros
	}
	if span.LogID != nil {
		outSpan.LogID = *span.LogID
	}
	if span.ServiceName != nil {
		outSpan.PSM = *span.ServiceName
	}
	return outSpan
}

func SpanListDO2DTO(
	spans loop_span.SpanList,
	userMap map[string]*common.UserInfo,
	evalMap map[int64]*rpc.Evaluator,
	tagMap map[int64]*rpc.TagInfo,
) []*span.OutputSpan {
	ret := make([]*span.OutputSpan, len(spans))
	for i, s := range spans {
		ret[i] = SpanDO2DTO(s, userMap, evalMap, tagMap)
	}
	return ret
}

func SpanListDTO2DO(spans []*span.InputSpan) loop_span.SpanList {
	ret := make(loop_span.SpanList, len(spans))
	for i, s := range spans {
		ret[i] = SpanDTO2DO(s)
	}
	return ret
}
