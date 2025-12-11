// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/ck/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/dao"
)

func SpanListPO2CKModels(spans []*dao.Span) []*model.ObservabilitySpan {
	ret := make([]*model.ObservabilitySpan, len(spans))
	for i, span := range spans {
		ret[i] = SpanPO2CKModel(span)
	}
	return ret
}

func SpanListCKModels2PO(spans []*model.ObservabilitySpan) []*dao.Span {
	ret := make([]*dao.Span, len(spans))
	for i, span := range spans {
		ret[i] = SpanCKModel2PO(span)
	}
	return ret
}

func SpanPO2CKModel(span *dao.Span) *model.ObservabilitySpan {
	if span == nil {
		return nil
	}
	return &model.ObservabilitySpan{
		TraceID:           span.TraceID,
		SpanID:            span.SpanID,
		SpaceID:           span.SpaceID,
		SpanType:          span.SpanType,
		SpanName:          span.SpanName,
		ParentID:          span.ParentID,
		Method:            span.Method,
		Psm:               span.Psm,
		Logid:             span.Logid,
		StartTime:         span.StartTime, // us
		CallType:          span.CallType,
		Duration:          span.Duration,
		StatusCode:        span.StatusCode,
		ObjectStorage:     span.ObjectStorage,
		Input:             span.Input,
		Output:            span.Output,
		LogicDeleteDate:   span.LogicDeleteDate,
		ReserveCreateTime: span.ReserveCreateTime,
		TagsBool:          span.TagsBool,
		TagsFloat:         span.TagsFloat,
		TagsString:        span.TagsString,
		TagsLong:          span.TagsLong,
		TagsByte:          span.TagsByte,
		SystemTagsFloat:   span.SystemTagsFloat,
		SystemTagsLong:    span.SystemTagsLong,
		SystemTagsString:  span.SystemTagsString,
	}
}

func SpanCKModel2PO(span *model.ObservabilitySpan) *dao.Span {
	if span == nil {
		return nil
	}
	return &dao.Span{
		TraceID:           span.TraceID,
		SpanID:            span.SpanID,
		SpaceID:           span.SpaceID,
		SpanType:          span.SpanType,
		SpanName:          span.SpanName,
		ParentID:          span.ParentID,
		Method:            span.Method,
		Psm:               span.Psm,
		Logid:             span.Logid,
		StartTime:         span.StartTime, // us
		CallType:          span.CallType,
		Duration:          span.Duration,
		StatusCode:        span.StatusCode,
		ObjectStorage:     span.ObjectStorage,
		Input:             span.Input,
		Output:            span.Output,
		LogicDeleteDate:   span.LogicDeleteDate,
		ReserveCreateTime: span.ReserveCreateTime,
		TagsBool:          span.TagsBool,
		TagsFloat:         span.TagsFloat,
		TagsString:        span.TagsString,
		TagsLong:          span.TagsLong,
		TagsByte:          span.TagsByte,
		SystemTagsFloat:   span.SystemTagsFloat,
		SystemTagsLong:    span.SystemTagsLong,
		SystemTagsString:  span.SystemTagsString,
	}
}
