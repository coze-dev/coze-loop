// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package span_filter

import (
	"context"
	"strconv"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type EvalTargetFilter struct{}

func (e *EvalTargetFilter) BuildBasicSpanFilter(ctx context.Context, env *SpanEnv) ([]*loop_span.FilterField, bool, error) {
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
			Values:    []string{"EvalTarget"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, false, nil
}

func (e *EvalTargetFilter) BuildRootSpanFilter(ctx context.Context, _ *SpanEnv) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldParentID,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"0", ""},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (e *EvalTargetFilter) BuildLLMSpanFilter(ctx context.Context, _ *SpanEnv) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"model"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (e *EvalTargetFilter) BuildALLSpanFilter(ctx context.Context, _ *SpanEnv) ([]*loop_span.FilterField, error) {
	return nil, nil
}

type EvalTargetFilterFactory struct{}

func (c *EvalTargetFilterFactory) PlatformType() loop_span.PlatformType {
	return loop_span.PlatformEvalTarget
}

func (c *EvalTargetFilterFactory) CreateFilter(ctx context.Context) (Filter, error) {
	return new(EvalTargetFilter), nil
}

func NewEvalTargetFilterFactory() Factory {
	return &EvalTargetFilterFactory{}
}
