// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/ck/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func SpanListDO2PO(spans loop_span.SpanList, TTL loop_span.TTL) []*model.ObservabilitySpan {
	ret := make([]*model.ObservabilitySpan, len(spans))
	for i, span := range spans {
		ret[i] = SpanDO2PO(span, TTL)
	}
	return ret
}

func SpanListPO2DO(spans []*model.ObservabilitySpan) loop_span.SpanList {
	ret := make(loop_span.SpanList, len(spans))
	for i, span := range spans {
		ret[i] = SpanPO2DO(span)
	}
	return ret
}

func SpanDO2PO(span *loop_span.Span, TTL loop_span.TTL) *model.ObservabilitySpan {
	ret := &model.ObservabilitySpan{
		TraceID:          span.TraceID,
		SpanID:           span.SpanID,
		SpaceID:          span.WorkspaceID,
		SpanType:         span.SpanType,
		SpanName:         span.SpanName,
		ParentID:         span.ParentID,
		StartTime:        span.StartTime, // us
		Duration:         span.DurationMicros,
		Psm:              ptr.Of(span.PSM),
		Logid:            ptr.Of(span.LogID),
		StatusCode:       span.StatusCode,
		Input:            span.Input,
		Output:           span.Output,
		TagsFloat:        CopyMap(span.TagsDouble),
		TagsString:       CopyMap(span.TagsString),
		TagsLong:         CopyMap(span.TagsLong),
		TagsByte:         CopyMap(span.TagsByte),
		SystemTagsFloat:  CopyMap(span.SystemTagsDouble),
		SystemTagsLong:   CopyMap(span.SystemTagsLong),
		SystemTagsString: CopyMap(span.SystemTagsString),
	}
	ret.TagsBool = make(map[string]uint8)
	for k, v := range span.TagsBool {
		if v {
			ret.TagsBool[k] = 1
		} else {
			ret.TagsBool[k] = 0
		}
	}
	if span.Method != "" {
		ret.Method = ptr.Of(span.Method)
	}
	if span.CallType != "" {
		ret.CallType = ptr.Of(span.CallType)
	}
	if span.ObjectStorage != "" {
		ret.ObjectStorage = ptr.Of(span.ObjectStorage)
	}
	switch TTL {
	case loop_span.TTL3d:
		ret.LogicDeleteDate = time.Now().Add(3 * 24 * time.Hour).UnixMicro()
	case loop_span.TTL7d:
		ret.LogicDeleteDate = time.Now().Add(7 * 24 * time.Hour).UnixMicro()
	case loop_span.TTL30d:
		ret.LogicDeleteDate = time.Now().Add(30 * 24 * time.Hour).UnixMicro()
	case loop_span.TTL90d:
		ret.LogicDeleteDate = time.Now().Add(90 * 24 * time.Hour).UnixMicro()
	case loop_span.TTL180d:
		ret.LogicDeleteDate = time.Now().Add(180 * 24 * time.Hour).UnixMicro()
	case loop_span.TTL365d:
		ret.LogicDeleteDate = time.Now().Add(365 * 24 * time.Hour).UnixMicro()
	default:
		ret.LogicDeleteDate = time.Now().Add(3 * 24 * time.Hour).UnixMicro()
	}
	return ret
}

func SpanPO2DO(span *model.ObservabilitySpan) *loop_span.Span {
	if span == nil {
		return nil
	}
	ret := &loop_span.Span{
		TraceID:          span.TraceID,
		SpanID:           span.SpanID,
		WorkspaceID:      span.SpaceID,
		SpanType:         span.SpanType,
		SpanName:         span.SpanName,
		ParentID:         span.ParentID,
		StartTime:        span.StartTime, // us
		DurationMicros:   span.Duration,
		StatusCode:       span.StatusCode,
		Input:            span.Input,
		Output:           span.Output,
		TagsDouble:       CopyMap(span.TagsFloat),
		TagsString:       CopyMap(span.TagsString),
		TagsLong:         CopyMap(span.TagsLong),
		TagsByte:         CopyMap(span.TagsByte),
		SystemTagsDouble: CopyMap(span.SystemTagsFloat),
		SystemTagsLong:   CopyMap(span.SystemTagsLong),
		SystemTagsString: CopyMap(span.SystemTagsString),
		LogicDeleteTime:  span.LogicDeleteDate,
	}
	ret.TagsBool = make(map[string]bool)
	for k, v := range span.TagsBool {
		if v > 0 {
			ret.TagsBool[k] = true
		} else {
			ret.TagsBool[k] = false
		}
	}
	if span.Method != nil {
		ret.Method = *span.Method
	}
	if span.CallType != nil {
		ret.CallType = *span.CallType
	}
	if span.ObjectStorage != nil {
		ret.ObjectStorage = *span.ObjectStorage
	}
	if span.Psm != nil {
		ret.PSM = *span.Psm
	}
	if span.Logid != nil {
		ret.LogID = *span.Logid
	}
	return ret
}

func CopyMap[T any](in map[string]T) map[string]T {
	ret := make(map[string]T)
	for k, v := range in {
		ret[k] = v
	}
	return ret
}
