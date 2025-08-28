// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package span_filter

import (
	"context"
	"strconv"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type EvaluatorFilter struct{}

func (e *EvaluatorFilter) BuildBasicSpanFilter(ctx context.Context, env *SpanEnv) ([]*loop_span.FilterField, bool, error) {
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
			Values:    []string{"Evaluator"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, false, nil
}

func (e *EvaluatorFilter) BuildRootSpanFilter(ctx context.Context, _ *SpanEnv) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldParentID,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"0", ""},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (e *EvaluatorFilter) BuildLLMSpanFilter(ctx context.Context, _ *SpanEnv) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"model"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (e *EvaluatorFilter) BuildALLSpanFilter(ctx context.Context, _ *SpanEnv) ([]*loop_span.FilterField, error) {
	return nil, nil
}

type EvaluatorFilterFactory struct{}

func (c *EvaluatorFilterFactory) PlatformType() loop_span.PlatformType {
	return loop_span.PlatformEvaluator
}

func (c *EvaluatorFilterFactory) CreateFilter(ctx context.Context) (Filter, error) {
	return new(EvaluatorFilter), nil
}

func NewEvaluatorFilterFactory() Factory {
	return &EvaluatorFilterFactory{}
}
