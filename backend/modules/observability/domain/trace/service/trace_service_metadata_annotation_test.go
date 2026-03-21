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

	t.Run("success", func(t *testing.T) {
		buildHelperMock.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), req.PlatformType).Return(filterMock, nil)
		filterMock.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return(nil, true, nil)
		tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), req.PlatformType).Return([]string{"tenant1"}, nil)

		spans := loop_span.SpanList{
			{
				SpanID: "span1",
				TagsString: map[string]string{
					"custom_tag": "value1",
				},
			},
			{
				SpanID: "span2",
				TagsString: map[string]string{
					"custom_tag": "value2",
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

	t.Run("list spans error", func(t *testing.T) {
		buildHelperMock.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), req.PlatformType).Return(nil, errors.New("filter error"))

		resp, err := svc.ListMetadata(ctx, req)
		assert.Error(t, err)
		assert.Nil(t, resp)
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

	t.Run("success", func(t *testing.T) {
		tenantProviderMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), req.PlatformType, gomock.Any()).Return([]string{"tenant1"}, nil)

		annotations := loop_span.AnnotationList{
			{
				ID:             "anno1",
				Key:            "key1",
				AnnotationType: loop_span.AnnotationType("test_type"),
				Value: loop_span.AnnotationValue{
					ValueType:   loop_span.AnnotationValueTypeString,
					StringValue: "val1",
				},
			},
		}

		traceRepoMock.EXPECT().ListWorkspaceAnnotations(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, param *repo.ListWorkspaceAnnotationsParam) (loop_span.AnnotationList, error) {
			assert.Equal(t, "123", param.WorkSpaceID)
			return annotations, nil
		})

		resp, err := svc.ListWorkspaceAnnotations(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Len(t, resp.SimpleAnnotationList, 1)
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
}
