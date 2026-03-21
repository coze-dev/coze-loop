// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"strconv"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	timeutil "github.com/coze-dev/coze-loop/backend/pkg/time"
	"github.com/samber/lo"
)

func BuildBuiltinFilters(ctx context.Context, f span_filter.Filter, env *span_filter.SpanEnv, spanListType loop_span.SpanListType) (*loop_span.FilterFields, error) {
	filters := make([]*loop_span.FilterField, 0)
	basicFilter, forceQuery, err := f.BuildBasicSpanFilter(ctx, env)
	if err != nil {
		return nil, err
	} else if len(basicFilter) == 0 && !forceQuery { // if it's null, no need to query from ck
		return nil, nil
	}
	filters = append(filters, basicFilter...)
	switch spanListType {
	case loop_span.SpanListTypeRootSpan:
		subFilter, err := f.BuildRootSpanFilter(ctx, env)
		if err != nil {
			return nil, err
		}
		filters = append(filters, subFilter...)
	case loop_span.SpanListTypeLLMSpan:
		subFilter, err := f.BuildLLMSpanFilter(ctx, env)
		if err != nil {
			return nil, err
		}
		filters = append(filters, subFilter...)
	case loop_span.SpanListTypeAllSpan:
		subFilter, err := f.BuildALLSpanFilter(ctx, env)
		if err != nil {
			return nil, err
		}
		filters = append(filters, subFilter...)
	case "":
		// do nothing
	default:
		return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg(fmt.Sprintf("invalid span list type: %s", spanListType)))
	}
	filterAggr := &loop_span.FilterFields{
		QueryAndOr:   lo.ToPtr(loop_span.QueryAndOrEnumAnd),
		FilterFields: filters,
	}
	return filterAggr, nil
}

func CombineFilters(filters ...*loop_span.FilterFields) *loop_span.FilterFields {
	filterAggr := &loop_span.FilterFields{
		QueryAndOr: lo.ToPtr(loop_span.QueryAndOrEnumAnd),
	}
	for _, f := range filters {
		if f == nil {
			continue
		}
		filterAggr.FilterFields = append(filterAggr.FilterFields, &loop_span.FilterField{
			QueryAndOr: lo.ToPtr(loop_span.QueryAndOrEnumAnd),
			SubFilter:  f,
		})
	}
	return filterAggr
}

func processSpecificFilter(f *loop_span.FilterField) error {
	switch f.FieldName {
	case loop_span.SpanFieldStatus:
		if err := processStatusFilter(f); err != nil {
			return err
		}
	case loop_span.SpanFieldDuration,
		loop_span.SpanFieldLatencyFirstResp,
		loop_span.SpanFieldStartTimeFirstResp,
		loop_span.SpanFieldStartTimeFirstTokenResp,
		loop_span.SpanFieldLatencyFirstTokenResp,
		loop_span.SpanFieldReasoningDuration:
		if err := processLatencyFilter(f); err != nil {
			return err
		}
	}
	return nil
}

func processStatusFilter(f *loop_span.FilterField) error {
	if f.QueryType == nil || *f.QueryType != loop_span.QueryTypeEnumIn {
		return fmt.Errorf("status filter should use in operator")
	}
	f.FieldName = loop_span.SpanFieldStatusCode
	f.FieldType = loop_span.FieldTypeLong
	checkSuccess, checkError := false, false
	for _, val := range f.Values {
		switch val {
		case loop_span.SpanStatusSuccess:
			checkSuccess = true
		case loop_span.SpanStatusError:
			checkError = true
		default:
			return fmt.Errorf("invalid status code field value")
		}
	}
	if checkSuccess && checkError {
		f.QueryType = gptr.Of(loop_span.QueryTypeEnumAlwaysTrue)
		f.Values = nil
	} else if checkSuccess {
		f.Values = []string{"0"}
	} else if checkError {
		f.QueryType = gptr.Of(loop_span.QueryTypeEnumNotIn)
		f.Values = []string{"0"}
	} else {
		return fmt.Errorf("invalid status code query")
	}
	return nil
}

// ms -> us
func processLatencyFilter(f *loop_span.FilterField) error {
	if f.FieldType != loop_span.FieldTypeLong {
		return fmt.Errorf("latency field type should be long ")
	}
	micros := make([]string, 0)
	for _, val := range f.Values {
		integer, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return fmt.Errorf("fail to parse long value %s, %v", val, err)
		}
		integer = timeutil.MillSec2MicroSec(integer)
		micros = append(micros, strconv.FormatInt(integer, 10))
	}
	f.Values = micros
	return nil
}
