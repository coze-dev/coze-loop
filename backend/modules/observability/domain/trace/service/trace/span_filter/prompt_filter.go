// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package span_filter

import (
	"context"
	"fmt"
	"strconv"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/config"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type PromptFilter struct {
	transCfg loop_span.SpanTransCfgList
}

func (p *PromptFilter) BuildBasicSpanFilter(ctx context.Context, env *SpanEnv) ([]*loop_span.FilterField, bool, error) {
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
			Values:    []string{"PromptPlayground", "PromptDebug"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, false, nil
}

func (p *PromptFilter) BuildRootSpanFilter(ctx context.Context, _ *SpanEnv) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldParentID,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"0", ""},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (p *PromptFilter) BuildLLMSpanFilter(ctx context.Context, _ *SpanEnv) ([]*loop_span.FilterField, error) {
	return []*loop_span.FilterField{
		{
			FieldName: loop_span.SpanFieldSpanType,
			FieldType: loop_span.FieldTypeString,
			Values:    []string{"model"},
			QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
		},
	}, nil
}

func (p *PromptFilter) BuildALLSpanFilter(ctx context.Context, _ *SpanEnv) ([]*loop_span.FilterField, error) {
	if len(p.transCfg) == 0 {
		return nil, nil
	}
	filter := &loop_span.FilterField{
		SubFilter: &loop_span.FilterFields{
			QueryAndOr:   ptr.Of(loop_span.QueryAndOrEnumOr),
			FilterFields: nil,
		},
	}
	for _, cfg := range p.transCfg {
		if cfg.SpanFilter != nil {
			filterField := &loop_span.FilterField{
				SubFilter: cfg.SpanFilter,
			}
			filter.SubFilter.FilterFields = append(filter.SubFilter.FilterFields, filterField)
		}
	}
	return []*loop_span.FilterField{filter}, nil
}

type PromptFilterFactory struct {
	traceConfig config.ITraceConfig
}

func (c *PromptFilterFactory) PlatformType() loop_span.PlatformType {
	return loop_span.PlatformPrompt
}

func (c *PromptFilterFactory) CreateFilter(ctx context.Context) (Filter, error) {
	transCfg, err := c.traceConfig.GetPlatformSpansTrans(ctx)
	if err != nil {
		return nil, err
	}
	cfg, ok := transCfg.PlatformCfg[string(c.PlatformType())]
	if !ok {
		return nil, fmt.Errorf("trans config not configured for platform type %s", c.PlatformType())
	}
	return &PromptFilter{
		transCfg: cfg,
	}, nil
}

func NewPromptFilterFactory(traceConfig config.ITraceConfig) Factory {
	return &PromptFilterFactory{
		traceConfig: traceConfig,
	}
}
