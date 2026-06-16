// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	metricsmock "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/metrics/mocks"
	tenantmock "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	repomock "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
	servicemock "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/mocks"
	filtermock "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_processor"
)

func TestTraceServiceImpl_ListMetadata(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	traceRepoMock := repomock.NewMockITraceRepo(ctrl)
	tenantProviderMock := tenantmock.NewMockITenantProvider(ctrl)
	metricsMock := metricsmock.NewMockITraceMetrics(ctrl)
	buildHelperMock := servicemock.NewMockTraceFilterProcessorBuilder(ctrl)
	filterMock := filtermock.NewMockFilter(ctrl)

	svc, err := service.NewTraceServiceImpl(
		traceRepoMock,
		nil,
		nil,
		nil,
		metricsMock,
		buildHelperMock,
		tenantProviderMock,
		nil,
		nil,
		nil,
	)
	assert.NoError(t, err)

	ctx := context.Background()
	req := &service.ListMetadataReq{
		WorkspaceID:  123,
		StartTime:    time.Now().Add(-time.Hour).UnixMilli(),
		EndTime:      time.Now().UnixMilli(),
		PlatformType: loop_span.PlatformCozeLoop,
	}

	t.Run("ListMetadata passes NotQueryAnnotation true to repo", func(t *testing.T) {
		buildHelperMock.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), req.PlatformType).Return(filterMock, nil)
		filterMock.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return(nil, true, nil)
		tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), req.PlatformType).Return([]string{"tenant1"}, nil)

		traceRepoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).DoAndReturn(
			func(ctx context.Context, param *repo.ListSpansParam) (*repo.ListSpansResult, error) {
				assert.True(t, param.NotQueryAnnotation)
				return &repo.ListSpansResult{
					Spans: loop_span.SpanList{{SpanID: "span1"}},
				}, nil
			})
		metricsMock.EXPECT().EmitListSpans(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
		buildHelperMock.EXPECT().BuildListSpansProcessors(gomock.Any(), gomock.Any()).Return([]span_processor.Processor{}, nil)

		resp, err := svc.ListMetadata(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})

	t.Run("dedup by key and sort by frequency desc", func(t *testing.T) {
		buildHelperMock.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), req.PlatformType).Return(filterMock, nil)
		filterMock.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return(nil, true, nil)
		tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), req.PlatformType).Return([]string{"tenant1"}, nil)

		spans := loop_span.SpanList{
			{
				SpanID: "span1",
				TagsString: map[string]string{
					"tag_a": "v1",
					"tag_b": "v2",
				},
			},
			{
				SpanID: "span2",
				TagsString: map[string]string{
					"tag_a": "v3",
					"tag_b": "v4",
					"tag_c": "v5",
				},
			},
			{
				SpanID: "span3",
				TagsString: map[string]string{
					"tag_a": "v1",
				},
			},
		}

		traceRepoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
			Spans: spans,
		}, nil)

		metricsMock.EXPECT().EmitListSpans(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
		buildHelperMock.EXPECT().BuildListSpansProcessors(gomock.Any(), gomock.Any()).Return([]span_processor.Processor{}, nil)

		resp, err := svc.ListMetadata(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Len(t, resp.MetadataItemList, 3)
		assert.Equal(t, "tag_a", resp.MetadataItemList[0].Key)
		assert.Equal(t, loop_span.MetadataValueTypeString, resp.MetadataItemList[0].ValueType)
		assert.Equal(t, "tag_b", resp.MetadataItemList[1].Key)
		assert.Equal(t, loop_span.MetadataValueTypeString, resp.MetadataItemList[1].ValueType)
		assert.Equal(t, "tag_c", resp.MetadataItemList[2].Key)
		assert.Equal(t, loop_span.MetadataValueTypeString, resp.MetadataItemList[2].ValueType)
	})

	t.Run("list spans error", func(t *testing.T) {
		buildHelperMock.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), req.PlatformType).Return(nil, errors.New("filter error"))

		resp, err := svc.ListMetadata(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("empty spans returns empty list", func(t *testing.T) {
		buildHelperMock.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), req.PlatformType).Return(filterMock, nil)
		filterMock.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return(nil, true, nil)
		tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), req.PlatformType).Return([]string{"tenant1"}, nil)
		traceRepoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
			Spans: loop_span.SpanList{},
		}, nil)
		metricsMock.EXPECT().EmitListSpans(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
		buildHelperMock.EXPECT().BuildListSpansProcessors(gomock.Any(), gomock.Any()).Return([]span_processor.Processor{}, nil)

		resp, err := svc.ListMetadata(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Len(t, resp.MetadataItemList, 0)
	})

	t.Run("mixed tag types return correct value_type", func(t *testing.T) {
		buildHelperMock.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), req.PlatformType).Return(filterMock, nil)
		filterMock.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return(nil, true, nil)
		tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), req.PlatformType).Return([]string{"tenant1"}, nil)

		spans := loop_span.SpanList{
			{
				SpanID:     "span1",
				TagsString: map[string]string{"str_tag": "val"},
				TagsLong:   map[string]int64{"long_tag": 42},
				TagsDouble: map[string]float64{"double_tag": 3.14},
				TagsBool:   map[string]bool{"bool_tag": true},
			},
		}

		traceRepoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
			Spans: spans,
		}, nil)
		metricsMock.EXPECT().EmitListSpans(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
		buildHelperMock.EXPECT().BuildListSpansProcessors(gomock.Any(), gomock.Any()).Return([]span_processor.Processor{}, nil)

		resp, err := svc.ListMetadata(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)

		typeMap := make(map[string]string)
		for _, item := range resp.MetadataItemList {
			typeMap[item.Key] = item.ValueType
		}
		assert.Equal(t, loop_span.MetadataValueTypeString, typeMap["str_tag"])
		assert.Equal(t, loop_span.MetadataValueTypeLong, typeMap["long_tag"])
		assert.Equal(t, loop_span.MetadataValueTypeDouble, typeMap["double_tag"])
		assert.Equal(t, loop_span.MetadataValueTypeBool, typeMap["bool_tag"])
	})

	t.Run("TagsByte returns string value type", func(t *testing.T) {
		buildHelperMock.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), req.PlatformType).Return(filterMock, nil)
		filterMock.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return(nil, true, nil)
		tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), req.PlatformType).Return([]string{"tenant1"}, nil)

		spans := loop_span.SpanList{
			{
				SpanID:   "span1",
				TagsByte: map[string]string{"byte_tag": "binary_data"},
			},
		}

		traceRepoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
			Spans: spans,
		}, nil)
		metricsMock.EXPECT().EmitListSpans(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
		buildHelperMock.EXPECT().BuildListSpansProcessors(gomock.Any(), gomock.Any()).Return([]span_processor.Processor{}, nil)

		resp, err := svc.ListMetadata(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)

		assert.Len(t, resp.MetadataItemList, 1)
		assert.Equal(t, "byte_tag", resp.MetadataItemList[0].Key)
		assert.Equal(t, loop_span.MetadataValueTypeString, resp.MetadataItemList[0].ValueType)
	})

	t.Run("same key across multiple spans increments count for all tag types", func(t *testing.T) {
		buildHelperMock.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), req.PlatformType).Return(filterMock, nil)
		filterMock.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return(nil, true, nil)
		tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), req.PlatformType).Return([]string{"tenant1"}, nil)

		spans := loop_span.SpanList{
			{
				SpanID:     "span1",
				TagsLong:   map[string]int64{"shared_long": 1},
				TagsDouble: map[string]float64{"shared_double": 1.1},
				TagsBool:   map[string]bool{"shared_bool": true},
				TagsByte:   map[string]string{"shared_byte": "x"},
			},
			{
				SpanID:     "span2",
				TagsLong:   map[string]int64{"shared_long": 2},
				TagsDouble: map[string]float64{"shared_double": 2.2},
				TagsBool:   map[string]bool{"shared_bool": false},
				TagsByte:   map[string]string{"shared_byte": "y"},
			},
		}

		traceRepoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
			Spans: spans,
		}, nil)
		metricsMock.EXPECT().EmitListSpans(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
		buildHelperMock.EXPECT().BuildListSpansProcessors(gomock.Any(), gomock.Any()).Return([]span_processor.Processor{}, nil)

		resp, err := svc.ListMetadata(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)

		assert.Len(t, resp.MetadataItemList, 4)
	})

	t.Run("all tag keys returned including those matching struct fields", func(t *testing.T) {
		buildHelperMock.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), req.PlatformType).Return(filterMock, nil)
		filterMock.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return(nil, true, nil)
		tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), req.PlatformType).Return([]string{"tenant1"}, nil)

		spans := loop_span.SpanList{
			{
				SpanID: "span1",
				TagsString: map[string]string{
					"trace_id":   "some-value",
					"custom_tag": "val",
				},
			},
		}

		traceRepoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
			Spans: spans,
		}, nil)
		metricsMock.EXPECT().EmitListSpans(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
		buildHelperMock.EXPECT().BuildListSpansProcessors(gomock.Any(), gomock.Any()).Return([]span_processor.Processor{}, nil)

		resp, err := svc.ListMetadata(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)

		assert.Len(t, resp.MetadataItemList, 2)
	})

	t.Run("data_extract scene includes system tags", func(t *testing.T) {
		reqWithScene := &service.ListMetadataReq{
			WorkspaceID:  123,
			StartTime:    req.StartTime,
			EndTime:      req.EndTime,
			PlatformType: req.PlatformType,
			Scene:        "data_extract",
		}
		buildHelperMock.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), reqWithScene.PlatformType).Return(filterMock, nil)
		filterMock.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return(nil, true, nil)
		tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), reqWithScene.PlatformType).Return([]string{"tenant1"}, nil)

		spans := loop_span.SpanList{
			{
				SpanID:     "span1",
				TagsString: map[string]string{"user_tag": "val"},
			},
		}

		traceRepoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
			Spans: spans,
		}, nil)
		metricsMock.EXPECT().EmitListSpans(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
		buildHelperMock.EXPECT().BuildListSpansProcessors(gomock.Any(), gomock.Any()).Return([]span_processor.Processor{}, nil)

		resp, err := svc.ListMetadata(ctx, reqWithScene)
		assert.NoError(t, err)
		assert.NotNil(t, resp)

		// Should contain SpanStructFieldKeys + user_tag (deduped)
		structFieldCount := len(loop_span.SpanStructFieldKeys)
		assert.True(t, len(resp.MetadataItemList) >= structFieldCount)
		// First items should be SpanStructFieldKeys
		for i, key := range loop_span.SpanStructFieldKeys {
			assert.Equal(t, key, resp.MetadataItemList[i].Key)
			assert.Equal(t, loop_span.MetadataValueTypeString, resp.MetadataItemList[i].ValueType)
		}
		// user_tag should follow after struct field keys
		assert.Equal(t, "user_tag", resp.MetadataItemList[structFieldCount].Key)
	})

	t.Run("default scene excludes system tags", func(t *testing.T) {
		buildHelperMock.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), req.PlatformType).Return(filterMock, nil)
		filterMock.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return(nil, true, nil)
		tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), req.PlatformType).Return([]string{"tenant1"}, nil)

		spans := loop_span.SpanList{
			{
				SpanID:           "span1",
				TagsString:       map[string]string{"user_tag": "val"},
				SystemTagsString: map[string]string{"sys_str": "v"},
				SystemTagsLong:   map[string]int64{"sys_long": 1},
			},
		}

		traceRepoMock.EXPECT().ListSpans(gomock.Any(), gomock.Any()).Return(&repo.ListSpansResult{
			Spans: spans,
		}, nil)
		metricsMock.EXPECT().EmitListSpans(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
		buildHelperMock.EXPECT().BuildListSpansProcessors(gomock.Any(), gomock.Any()).Return([]span_processor.Processor{}, nil)

		resp, err := svc.ListMetadata(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)

		assert.Len(t, resp.MetadataItemList, 1)
		assert.Equal(t, "user_tag", resp.MetadataItemList[0].Key)
	})
}

func TestTraceServiceImpl_ListWorkspaceAnnotations(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	traceRepoMock := repomock.NewMockITraceRepo(ctrl)
	tenantProviderMock := tenantmock.NewMockITenantProvider(ctrl)

	svc, err := service.NewTraceServiceImpl(
		traceRepoMock,
		nil,
		nil,
		nil,
		nil,
		nil,
		tenantProviderMock,
		nil,
		nil,
		nil,
	)
	assert.NoError(t, err)

	ctx := context.Background()
	req := &service.ListWorkspaceAnnotationsReq{
		WorkspaceID:    123,
		StartTime:      time.Now().Add(-time.Hour).UnixMilli(),
		AnnotationType: "test_type",
		PlatformType:   loop_span.PlatformCozeLoop,
	}

	t.Run("returns raw annotations", func(t *testing.T) {
		tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), req.PlatformType, gomock.Any()).Return([]string{"tenant1"}, nil)

		annotations := loop_span.AnnotationList{
			{ID: "a1", Key: "key_a", AnnotationType: loop_span.AnnotationType("test_type")},
			{ID: "a2", Key: "key_b", AnnotationType: loop_span.AnnotationType("test_type")},
			{ID: "a3", Key: "key_a", AnnotationType: loop_span.AnnotationType("test_type")},
		}

		traceRepoMock.EXPECT().ListWorkspaceAnnotations(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, param *repo.ListWorkspaceAnnotationsParam) (loop_span.AnnotationList, error) {
			assert.Equal(t, "123", param.WorkSpaceID)
			return annotations, nil
		})

		resp, err := svc.ListWorkspaceAnnotations(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Len(t, resp.Annotations, 3)
		assert.Equal(t, "key_a", resp.Annotations[0].Key)
		assert.Equal(t, "key_b", resp.Annotations[1].Key)
		assert.Equal(t, "key_a", resp.Annotations[2].Key)
	})

	t.Run("get tenants error", func(t *testing.T) {
		tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), req.PlatformType, gomock.Any()).Return(nil, errors.New("tenant error"))
		resp, err := svc.ListWorkspaceAnnotations(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("repo error", func(t *testing.T) {
		tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), req.PlatformType, gomock.Any()).Return([]string{"tenant1"}, nil)
		traceRepoMock.EXPECT().ListWorkspaceAnnotations(gomock.Any(), gomock.Any()).Return(nil, errors.New("repo error"))
		resp, err := svc.ListWorkspaceAnnotations(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})

	t.Run("empty annotations", func(t *testing.T) {
		tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), req.PlatformType, gomock.Any()).Return([]string{"tenant1"}, nil)
		traceRepoMock.EXPECT().ListWorkspaceAnnotations(gomock.Any(), gomock.Any()).Return(loop_span.AnnotationList{}, nil)

		resp, err := svc.ListWorkspaceAnnotations(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Len(t, resp.Annotations, 0)
	})
}
