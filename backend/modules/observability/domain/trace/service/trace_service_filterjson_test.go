// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service_test

import (
	"context"
	"testing"

	metricmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/metrics/mocks"
	tenantmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/mocks"
	spanfiltermocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_processor"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func combineFilters(filters ...*loop_span.FilterFields) *loop_span.FilterFields {
	filterAggr := &loop_span.FilterFields{
		QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
	}
	for _, f := range filters {
		if f == nil {
			continue
		}
		filterAggr.FilterFields = append(filterAggr.FilterFields, &loop_span.FilterField{
			QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
			SubFilter:  f,
		})
	}
	return filterAggr
}

func TestTraceServiceImpl_ListSpans_QueryFilterJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repoMock := repomocks.NewMockITraceRepo(ctrl)
	tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
	metricsMock := metricmocks.NewMockITraceMetrics(ctrl)
	buildHelperMock := servicemocks.NewMockTraceFilterProcessorBuilder(ctrl)
	platformFilterMock := spanfiltermocks.NewMockFilter(ctrl)

	basicField := &loop_span.FilterField{
		FieldName: "k1",
		FieldType: loop_span.FieldTypeString,
		Values:    []string{"v1"},
		QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
	}
	allSpanField := &loop_span.FilterField{
		FieldName: "k2",
		FieldType: loop_span.FieldTypeLong,
		Values:    []string{"1"},
		QueryType: ptr.Of(loop_span.QueryTypeEnumGte),
	}
	reqFilters := &loop_span.FilterFields{
		QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumOr),
		FilterFields: []*loop_span.FilterField{
			{
				FieldName: "k3",
				FieldType: loop_span.FieldTypeString,
				Values:    []string{"v3"},
				QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
			},
		},
	}

	req := &service.ListSpansReq{
		WorkspaceID:     1,
		StartTime:       100,
		EndTime:         200,
		Filters:         reqFilters,
		Limit:           10,
		PlatformType:    loop_span.PlatformCozeLoop,
		SpanListType:    loop_span.SpanListTypeAllSpan,
		DescByStartTime: true,
		PageToken:       "pt",
	}

	tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), req.PlatformType).Return([]string{"t1"}, nil).AnyTimes()
	buildHelperMock.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), req.PlatformType).Return(platformFilterMock, nil)
	platformFilterMock.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{basicField}, false, nil)
	platformFilterMock.EXPECT().BuildALLSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{allSpanField}, nil)

	repoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{Spans: loop_span.SpanList{}}, nil)
	metricsMock.EXPECT().EmitListSpans(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())

	rI, err := service.NewTraceServiceImpl(repoMock, nil, nil, nil, metricsMock, buildHelperMock, tenantProviderMock, nil, nil, nil)
	require.NoError(t, err)

	expectedBuiltin := &loop_span.FilterFields{
		QueryAndOr:   ptr.Of(loop_span.QueryAndOrEnumAnd),
		FilterFields: []*loop_span.FilterField{basicField, allSpanField},
	}
	expectedCombined := combineFilters(expectedBuiltin, reqFilters)

	buildHelperMock.EXPECT().BuildListSpansProcessors(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, set span_processor.Settings) ([]span_processor.Processor, error) {
			require.NotNil(t, set.QueryFilter)
			assert.Equal(t, expectedCombined, set.QueryFilter)
			return []span_processor.Processor{}, nil
		},
	)

	_, err = rI.ListSpans(context.Background(), req)
	require.NoError(t, err)
}

func TestTraceServiceImpl_GetTrace_QueryFilterJSON(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repoMock := repomocks.NewMockITraceRepo(ctrl)
	tenantProviderMock := tenantmocks.NewMockITenantProvider(ctrl)
	metricsMock := metricmocks.NewMockITraceMetrics(ctrl)
	buildHelperMock := servicemocks.NewMockTraceFilterProcessorBuilder(ctrl)

	reqFilters := &loop_span.FilterFields{
		QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
		FilterFields: []*loop_span.FilterField{
			{
				FieldName: "status",
				FieldType: loop_span.FieldTypeString,
				Values:    []string{"success"},
				QueryType: ptr.Of(loop_span.QueryTypeEnumIn),
			},
		},
	}
	req := &service.GetTraceReq{
		WorkspaceID:  1,
		LogID:        "lid",
		TraceID:      "tid",
		StartTime:    100,
		EndTime:      200,
		PlatformType: loop_span.PlatformCozeLoop,
		Filters:      reqFilters,
		WithDetail:   true,
	}

	tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), req.PlatformType, gomock.Any()).Return([]string{"t1"}, nil).AnyTimes()
	repoMock.EXPECT().GetTrace(gomock.Any(), gomock.Any()).Return(loop_span.SpanList{}, nil)
	metricsMock.EXPECT().EmitGetTrace(gomock.Any(), gomock.Any(), gomock.Any())

	rI, err := service.NewTraceServiceImpl(repoMock, nil, nil, nil, metricsMock, buildHelperMock, tenantProviderMock, nil, nil, nil)
	require.NoError(t, err)

	logTraceFilter := &loop_span.FilterFields{
		QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
		FilterFields: []*loop_span.FilterField{
			{
				FieldName: loop_span.SpanFieldTraceId,
				FieldType: loop_span.FieldTypeString,
				Values:    []string{req.TraceID},
				QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
			},
			{
				FieldName: loop_span.SpanFieldLogID,
				FieldType: loop_span.FieldTypeString,
				Values:    []string{req.LogID},
				QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
			},
		},
	}
	expectedCombined := combineFilters(logTraceFilter, reqFilters)

	buildHelperMock.EXPECT().BuildGetTraceProcessors(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, set span_processor.Settings) ([]span_processor.Processor, error) {
			require.NotNil(t, set.QueryFilter)
			assert.Equal(t, expectedCombined, set.QueryFilter)
			return []span_processor.Processor{}, nil
		},
	)

	_, err = rI.GetTrace(context.Background(), req)
	require.NoError(t, err)
}
