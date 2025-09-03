// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package span_filter

import (
	"context"
	"strconv"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type CozeLoopFilter struct{}

func (c *CozeLoopFilter) BuildBasicSpanFilter(ctx context.Context, env *SpanEnv) ([]*loop_span.FilterField, bool, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpaceId,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{strconv.FormatInt(env.WorkspaceId, 10)},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
		{
			FieldName: loop_span.SpanFieldCallType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"Custom"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, false, nil
}

func (c *CozeLoopFilter) BuildRootSpanFilter(ctx context.Context, _ *SpanEnv) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldParentID,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"0", ""},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (c *CozeLoopFilter) BuildLLMSpanFilter(ctx context.Context, _ *SpanEnv) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"model"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (c *CozeLoopFilter) BuildALLSpanFilter(ctx context.Context, _ *SpanEnv) ([]*loop_span.FilterField, error) {
	return nil, nil
}

type CozeLoopFilterFactory struct{}

func (c *CozeLoopFilterFactory) PlatformType() loop_span.PlatformType {
	return loop_span.PlatformCozeLoop
}

func (c *CozeLoopFilterFactory) CreateFilter(ctx context.Context) (Filter, error) {
	return new(CozeLoopFilter), nil
}

func NewCozeLoopFilterFactory() Factory {
	return &CozeLoopFilterFactory{}
}
