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
		assert.Equal(t, "tag_b", resp.MetadataItemList[1].Key)
		assert.Equal(t, "tag_c", resp.MetadataItemList[2].Key)
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
