// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/repo"
	metricmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	tracemocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	filtermocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter/mocks"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
	"go.uber.org/mock/gomock"
)

// 测试常量
const (
	testWorkspaceID = int64(12345)
	testStartTime   = int64(1640995200) // 2022-01-01 00:00:00 UTC
	testEndTime     = int64(1641081600) // 2022-01-02 00:00:00 UTC
)

// MockMetricDefinition 实现 IMetricDefinition 接口用于测试
type MockMetricDefinition struct {
	name       entity.MetricName
	metricType entity.MetricType
	source     entity.MetricSource
	expression string
	where      []*loop_span.FilterField
	whereError error
	groupBy    []*entity.Dimension
}

func (m *MockMetricDefinition) Name() entity.MetricName {
	return m.name
}

func (m *MockMetricDefinition) Type() entity.MetricType {
	return m.metricType
}

func (m *MockMetricDefinition) Source() entity.MetricSource {
	return m.source
}

func (m *MockMetricDefinition) Expression(granularity entity.MetricGranularity) string {
	return m.expression
}

func (m *MockMetricDefinition) Where(ctx context.Context, filter span_filter.Filter, env *span_filter.SpanEnv) ([]*loop_span.FilterField, error) {
	if m.whereError != nil {
		return nil, m.whereError
	}
	return m.where, nil
}

func (m *MockMetricDefinition) GroupBy() []*entity.Dimension {
	return m.groupBy
}

// 创建测试用的 MetricsService
func setupQueryMetricsTest(t *testing.T) (*MetricsService, *gomock.Controller, *metricmocks.MockIMetricRepo, *mocks.MockITenantProvider, *tracemocks.MockTraceFilterProcessorBuilder, *filtermocks.MockFilter) {
	ctrl := gomock.NewController(t)

	mockRepo := metricmocks.NewMockIMetricRepo(ctrl)
	mockTenantProvider := mocks.NewMockITenantProvider(ctrl)
	mockBuildHelper := tracemocks.NewMockTraceFilterProcessorBuilder(ctrl)
	mockFilter := filtermocks.NewMockFilter(ctrl)

	// 创建测试用的指标定义
	metricDefs := []entity.IMetricDefinition{
		&MockMetricDefinition{
			name:       entity.MetricNameModelFailRatio,
			metricType: entity.MetricTypeTimeSeries,
			source:     entity.MetricSourceCK,
			expression: "count(*)",
			where:      []*loop_span.FilterField{},
			groupBy:    []*entity.Dimension{},
		},
		&MockMetricDefinition{
			name:       entity.MetricNameTotalCount,
			metricType: entity.MetricTypeSummary,
			source:     entity.MetricSourceCK,
			expression: "sum(count)",
			where:      []*loop_span.FilterField{},
			groupBy:    []*entity.Dimension{},
		},
		&MockMetricDefinition{
			name:       entity.MetricNameToolFailRatio,
			metricType: entity.MetricTypePie,
			source:     entity.MetricSourceCK,
			expression: "avg(ratio)",
			where:      []*loop_span.FilterField{},
			groupBy:    []*entity.Dimension{{Expression: "region", Alias: "region"}},
		},
	}

	service, err := NewMetricsService(mockRepo, metricDefs, mockTenantProvider, mockBuildHelper)
	if err != nil {
		t.Fatalf("Failed to create MetricsService: %v", err)
	}

	return service.(*MetricsService), ctrl, mockRepo, mockTenantProvider, mockBuildHelper, mockFilter
}

// 创建有效的查询请求
func createValidQueryMetricsReq() *QueryMetricsReq {
	return &QueryMetricsReq{
		PlatformType: loop_span.PlatformOpenAPI,
		WorkspaceID:  testWorkspaceID,
		MetricsNames: []entity.MetricName{entity.MetricNameModelFailRatio},
		Granularity:  entity.MetricsGranularity5Min,
		FilterFields: &loop_span.FilterFields{
			QueryAndOr: ptr.Of(loop_span.QueryAndOrEnumAnd),
			FilterFields: []*loop_span.FilterField{
				{
					FieldName: "status",
					FieldType: loop_span.FieldTypeString,
					QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
					Values:    []string{"success"},
				},
			},
		},
		StartTime: testStartTime,
		EndTime:   testEndTime,
	}
}

// TestMetricsService_QueryMetrics 完整的 QueryMetrics 测试套件
func TestMetricsService_QueryMetrics(t *testing.T) {
	tests := []struct {
		name          string
		setupMocks    func(*metricmocks.MockIMetricRepo, *mocks.MockITenantProvider, *tracemocks.MockTraceFilterProcessorBuilder, *filtermocks.MockFilter)
		req           *QueryMetricsReq
		expectedError string
		checkResult   func(*testing.T, *QueryMetricsResp)
	}{
		// 正常场景测试
		{
			name: "Success_SingleMetric_TimeSeries",
			setupMocks: func(mockRepo *metricmocks.MockIMetricRepo, mockTenantProvider *mocks.MockITenantProvider, mockBuildHelper *tracemocks.MockTraceFilterProcessorBuilder, mockFilter *filtermocks.MockFilter) {
				mockBuildHelper.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), loop_span.PlatformOpenAPI).Return(mockFilter, nil)
				mockTenantProvider.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformOpenAPI).Return([]string{"tenant1", "tenant2"}, nil)
				mockFilter.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{}, true, nil)

				mockRepo.EXPECT().GetMetrics(gomock.Any(), gomock.Any()).Return(&repo.GetMetricsResult{
					Data: []map[string]any{
						{
							"time_bucket":                           "2022-01-01T00:00:00Z",
							string(entity.MetricNameModelFailRatio): 100,
						},
						{
							"time_bucket":                           "2022-01-01T01:00:00Z",
							string(entity.MetricNameModelFailRatio): 150,
						},
					},
				}, nil)
			},
			req: createValidQueryMetricsReq(),
			checkResult: func(t *testing.T, result *QueryMetricsResp) {
				if result == nil {
					t.Error("Expected result, but got nil")
					return
				}
				if len(result.Metrics) != 1 {
					t.Errorf("Expected 1 metric, but got %d", len(result.Metrics))
				}
				metric := result.Metrics[string(entity.MetricNameModelFailRatio)]
				if metric == nil || metric.TimeSeries == nil {
					t.Error("Expected TimeSeries data")
				}
			},
		},
		{
			name: "Success_MultipleMetrics_DifferentTypes",
			setupMocks: func(mockRepo *metricmocks.MockIMetricRepo, mockTenantProvider *mocks.MockITenantProvider, mockBuildHelper *tracemocks.MockTraceFilterProcessorBuilder, mockFilter *filtermocks.MockFilter) {
				mockBuildHelper.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), loop_span.PlatformOpenAPI).Return(mockFilter, nil)
				mockTenantProvider.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformOpenAPI).Return([]string{"tenant1"}, nil)
				mockFilter.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{}, true, nil)

				mockRepo.EXPECT().GetMetrics(gomock.Any(), gomock.Any()).Return(&repo.GetMetricsResult{
					Data: []map[string]any{
						{
							string(entity.MetricNameToolFailRatio): 75,
							"region":                               "us-east",
						},
						{
							string(entity.MetricNameToolFailRatio): 25,
							"region":                               "us-west",
						},
					},
				}, nil)
			},
			req: &QueryMetricsReq{
				PlatformType: loop_span.PlatformOpenAPI,
				WorkspaceID:  testWorkspaceID,
				MetricsNames: []entity.MetricName{entity.MetricNameToolFailRatio},
				Granularity:  entity.MetricsGranularity5Min,
				StartTime:    testStartTime,
				EndTime:      testEndTime,
			},
			checkResult: func(t *testing.T, result *QueryMetricsResp) {
				if result == nil {
					t.Error("Expected result, but got nil")
					return
				}
				metric := result.Metrics[string(entity.MetricNameToolFailRatio)]
				if metric == nil || metric.Pie == nil {
					t.Error("Expected Pie data")
					return
				}
				if len(metric.Pie) != 2 {
					t.Errorf("Expected 2 pie segments, got %d", len(metric.Pie))
				}
			},
		},
		{
			name: "Success_DifferentMetricTypes",
			setupMocks: func(mockRepo *metricmocks.MockIMetricRepo, mockTenantProvider *mocks.MockITenantProvider, mockBuildHelper *tracemocks.MockTraceFilterProcessorBuilder, mockFilter *filtermocks.MockFilter) {
				mockBuildHelper.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), loop_span.PlatformOpenAPI).Return(mockFilter, nil)
				mockTenantProvider.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformOpenAPI).Return([]string{"tenant1"}, nil)
				mockFilter.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{}, true, nil)

				mockRepo.EXPECT().GetMetrics(gomock.Any(), gomock.Any()).Return(&repo.GetMetricsResult{
					Data: []map[string]any{
						{
							string(entity.MetricNameTotalCount): 100,
						},
					},
				}, nil)
			},
			req: &QueryMetricsReq{
				PlatformType: loop_span.PlatformOpenAPI,
				WorkspaceID:  testWorkspaceID,
				MetricsNames: []entity.MetricName{entity.MetricNameTotalCount},
				Granularity:  entity.MetricsGranularity5Min,
				StartTime:    testStartTime,
				EndTime:      testEndTime,
			},
			checkResult: func(t *testing.T, result *QueryMetricsResp) {
				if result == nil {
					t.Error("Expected result, but got nil")
					return
				}
				metric := result.Metrics[string(entity.MetricNameTotalCount)]
				if metric == nil || metric.Summary == "" {
					t.Error("Expected Summary data")
				}
			},
		},

		// 边界情况测试
		{
			name: "EmptyMetricsNames",
			setupMocks: func(mockRepo *metricmocks.MockIMetricRepo, mockTenantProvider *mocks.MockITenantProvider, mockBuildHelper *tracemocks.MockTraceFilterProcessorBuilder, mockFilter *filtermocks.MockFilter) {
				mockBuildHelper.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), loop_span.PlatformOpenAPI).Return(mockFilter, nil)
				mockTenantProvider.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformOpenAPI).Return([]string{"tenant1"}, nil)
			},
			req: &QueryMetricsReq{
				PlatformType: loop_span.PlatformOpenAPI,
				WorkspaceID:  testWorkspaceID,
				MetricsNames: []entity.MetricName{},
				Granularity:  entity.MetricsGranularity5Min,
				StartTime:    testStartTime,
				EndTime:      testEndTime,
			},
			expectedError: "invalid param",
		},
		{
			name: "TimeRangeBoundary",
			setupMocks: func(mockRepo *metricmocks.MockIMetricRepo, mockTenantProvider *mocks.MockITenantProvider, mockBuildHelper *tracemocks.MockTraceFilterProcessorBuilder, mockFilter *filtermocks.MockFilter) {
				mockBuildHelper.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), loop_span.PlatformOpenAPI).Return(mockFilter, nil)
				mockTenantProvider.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformOpenAPI).Return([]string{"tenant1"}, nil)
				mockFilter.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{}, true, nil)

				mockRepo.EXPECT().GetMetrics(gomock.Any(), gomock.Any()).Return(&repo.GetMetricsResult{
					Data: []map[string]any{},
				}, nil)
			},
			req: &QueryMetricsReq{
				PlatformType: loop_span.PlatformOpenAPI,
				WorkspaceID:  testWorkspaceID,
				MetricsNames: []entity.MetricName{entity.MetricNameModelFailRatio},
				Granularity:  entity.MetricsGranularity5Min,
				StartTime:    testEndTime, // StartTime >= EndTime
				EndTime:      testStartTime,
			},
			checkResult: func(t *testing.T, result *QueryMetricsResp) {
				// 应该返回空结果，但不报错
				if result == nil {
					t.Error("Expected empty result, but got nil")
				}
			},
		},
		{
			name: "EmptyFilterResult",
			setupMocks: func(mockRepo *metricmocks.MockIMetricRepo, mockTenantProvider *mocks.MockITenantProvider, mockBuildHelper *tracemocks.MockTraceFilterProcessorBuilder, mockFilter *filtermocks.MockFilter) {
				mockBuildHelper.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), loop_span.PlatformOpenAPI).Return(mockFilter, nil)
				mockTenantProvider.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformOpenAPI).Return([]string{"tenant1"}, nil)
				mockFilter.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{}, false, nil)
			},
			req: createValidQueryMetricsReq(),
			checkResult: func(t *testing.T, result *QueryMetricsResp) {
				if result == nil {
					t.Error("Expected empty result, but got nil")
					return
				}
				if len(result.Metrics) != 0 {
					t.Errorf("Expected empty metrics, but got %d", len(result.Metrics))
				}
			},
		},

		// 异常场景测试
		{
			name: "BuildPlatformFilterError",
			setupMocks: func(mockRepo *metricmocks.MockIMetricRepo, mockTenantProvider *mocks.MockITenantProvider, mockBuildHelper *tracemocks.MockTraceFilterProcessorBuilder, mockFilter *filtermocks.MockFilter) {
				mockBuildHelper.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), loop_span.PlatformOpenAPI).Return(nil, errors.New("platform filter error"))
			},
			req:           createValidQueryMetricsReq(),
			expectedError: "platform filter error",
		},
		{
			name: "GetTenantsError",
			setupMocks: func(mockRepo *metricmocks.MockIMetricRepo, mockTenantProvider *mocks.MockITenantProvider, mockBuildHelper *tracemocks.MockTraceFilterProcessorBuilder, mockFilter *filtermocks.MockFilter) {
				mockBuildHelper.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), loop_span.PlatformOpenAPI).Return(mockFilter, nil)
				mockTenantProvider.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformOpenAPI).Return(nil, errors.New("tenant error"))
			},
			req:           createValidQueryMetricsReq(),
			expectedError: "tenant error",
		},
		{
			name: "MetricDefinitionNotFound",
			setupMocks: func(mockRepo *metricmocks.MockIMetricRepo, mockTenantProvider *mocks.MockITenantProvider, mockBuildHelper *tracemocks.MockTraceFilterProcessorBuilder, mockFilter *filtermocks.MockFilter) {
				mockBuildHelper.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), loop_span.PlatformOpenAPI).Return(mockFilter, nil)
				mockTenantProvider.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformOpenAPI).Return([]string{"tenant1"}, nil)
			},
			req: &QueryMetricsReq{
				PlatformType: loop_span.PlatformOpenAPI,
				WorkspaceID:  testWorkspaceID,
				MetricsNames: []entity.MetricName{"nonexistent_metric"},
				Granularity:  entity.MetricsGranularity5Min,
				StartTime:    testStartTime,
				EndTime:      testEndTime,
			},
			expectedError: "metric definition nonexistent_metric not found",
		},
		{
			name: "BuildMetricFilterError",
			setupMocks: func(mockRepo *metricmocks.MockIMetricRepo, mockTenantProvider *mocks.MockITenantProvider, mockBuildHelper *tracemocks.MockTraceFilterProcessorBuilder, mockFilter *filtermocks.MockFilter) {
				mockBuildHelper.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), loop_span.PlatformOpenAPI).Return(mockFilter, nil)
				mockTenantProvider.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformOpenAPI).Return([]string{"tenant1"}, nil)
				mockFilter.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return(nil, false, errors.New("filter error"))
			},
			req:           createValidQueryMetricsReq(),
			expectedError: "filter error",
		},
		{
			name: "MetricRepoError",
			setupMocks: func(mockRepo *metricmocks.MockIMetricRepo, mockTenantProvider *mocks.MockITenantProvider, mockBuildHelper *tracemocks.MockTraceFilterProcessorBuilder, mockFilter *filtermocks.MockFilter) {
				mockBuildHelper.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), loop_span.PlatformOpenAPI).Return(mockFilter, nil)
				mockTenantProvider.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformOpenAPI).Return([]string{"tenant1"}, nil)
				mockFilter.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{}, true, nil)

				mockRepo.EXPECT().GetMetrics(gomock.Any(), gomock.Any()).Return(nil, errors.New("repo error"))
			},
			req:           createValidQueryMetricsReq(),
			expectedError: "repo error",
		},

		// 参数验证测试
		{
			name: "InvalidPlatformType",
			setupMocks: func(mockRepo *metricmocks.MockIMetricRepo, mockTenantProvider *mocks.MockITenantProvider, mockBuildHelper *tracemocks.MockTraceFilterProcessorBuilder, mockFilter *filtermocks.MockFilter) {
				mockBuildHelper.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), loop_span.PlatformType("invalid")).Return(nil, errors.New("invalid platform type"))
			},
			req: &QueryMetricsReq{
				PlatformType: loop_span.PlatformType("invalid"),
				WorkspaceID:  testWorkspaceID,
				MetricsNames: []entity.MetricName{entity.MetricNameModelFailRatio},
				Granularity:  entity.MetricsGranularity5Min,
				StartTime:    testStartTime,
				EndTime:      testEndTime,
			},
			expectedError: "invalid platform type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, ctrl, mockRepo, mockTenantProvider, mockBuildHelper, mockFilter := setupQueryMetricsTest(t)
			defer ctrl.Finish()

			if tt.setupMocks != nil {
				tt.setupMocks(mockRepo, mockTenantProvider, mockBuildHelper, mockFilter)
			}

			result, err := service.QueryMetrics(context.Background(), tt.req)

			// 检查错误
			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("Expected error containing '%s', but got nil", tt.expectedError)
					return
				}
				if !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error containing '%s', but got '%s'", tt.expectedError, err.Error())
					return
				}
			} else if err != nil {
				t.Errorf("Expected no error, but got: %v", err)
				return
			}

			// 检查结果
			if tt.checkResult != nil {
				tt.checkResult(t, result)
			}
		})
	}
}

// 测试 MetricDefinition Where 方法失败的情况
func TestMetricsService_QueryMetrics_MetricDefinitionWhereError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := metricmocks.NewMockIMetricRepo(ctrl)
	mockTenantProvider := mocks.NewMockITenantProvider(ctrl)
	mockBuildHelper := tracemocks.NewMockTraceFilterProcessorBuilder(ctrl)
	mockFilter := filtermocks.NewMockFilter(ctrl)

	// 创建一个会返回错误的指标定义
	metricDefs := []entity.IMetricDefinition{
		&MockMetricDefinition{
			name:       entity.MetricNameModelFailRatio,
			metricType: entity.MetricTypeTimeSeries,
			source:     entity.MetricSourceCK,
			expression: "count(*)",
			whereError: errors.New("where method error"),
		},
	}

	service, err := NewMetricsService(mockRepo, metricDefs, mockTenantProvider, mockBuildHelper)
	if err != nil {
		t.Fatalf("Failed to create MetricsService: %v", err)
	}

	// 设置 mock 期望
	mockBuildHelper.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), loop_span.PlatformOpenAPI).Return(mockFilter, nil)
	mockTenantProvider.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformOpenAPI).Return([]string{"tenant1"}, nil)

	req := createValidQueryMetricsReq()
	result, err := service.QueryMetrics(context.Background(), req)

	if err == nil {
		t.Error("Expected error from Where method, but got nil")
		return
	}

	if result != nil {
		t.Errorf("Expected nil result, but got: %+v", result)
	}

	// 检查错误是否被正确包装
	if !strings.Contains(err.Error(), "where method error") {
		t.Errorf("Expected error containing 'where method error', but got: %s", err.Error())
	}
}

// 测试指标信息合并失败的情况
func TestMetricsService_QueryMetrics_CombineMetricInfosError(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockRepo := metricmocks.NewMockIMetricRepo(ctrl)
	mockTenantProvider := mocks.NewMockITenantProvider(ctrl)
	mockBuildHelper := tracemocks.NewMockTraceFilterProcessorBuilder(ctrl)
	mockFilter := filtermocks.NewMockFilter(ctrl)

	// 创建两个类型不同的指标定义，会导致合并失败
	metricDefs := []entity.IMetricDefinition{
		&MockMetricDefinition{
			name:       entity.MetricNameModelFailRatio,
			metricType: entity.MetricTypeTimeSeries,
			source:     entity.MetricSourceCK,
			expression: "count(*)",
			where:      []*loop_span.FilterField{},
			groupBy:    []*entity.Dimension{},
		},
		&MockMetricDefinition{
			name:       entity.MetricNameTotalCount,
			metricType: entity.MetricTypeSummary, // 不同的类型
			source:     entity.MetricSourceCK,
			expression: "sum(count)",
			where:      []*loop_span.FilterField{},
			groupBy:    []*entity.Dimension{},
		},
	}

	service, err := NewMetricsService(mockRepo, metricDefs, mockTenantProvider, mockBuildHelper)
	if err != nil {
		t.Fatalf("Failed to create MetricsService: %v", err)
	}

	// 设置 mock 期望
	mockBuildHelper.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), loop_span.PlatformOpenAPI).Return(mockFilter, nil)
	mockTenantProvider.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformOpenAPI).Return([]string{"tenant1"}, nil)

	req := &QueryMetricsReq{
		PlatformType: loop_span.PlatformOpenAPI,
		WorkspaceID:  testWorkspaceID,
		MetricsNames: []entity.MetricName{entity.MetricNameModelFailRatio, entity.MetricNameTotalCount}, // 不同类型的指标
		Granularity:  entity.MetricsGranularity5Min,
		StartTime:    testStartTime,
		EndTime:      testEndTime,
	}

	result, err := service.QueryMetrics(context.Background(), req)

	if err == nil {
		t.Error("Expected error from combineMetricInfos, but got nil")
		return
	}

	if result != nil {
		t.Errorf("Expected nil result, but got: %+v", result)
	}

	// 检查错误信息
	if !strings.Contains(err.Error(), "metric types not the same") {
		t.Errorf("Expected error containing 'metric types not the same', but got: %s", err.Error())
	}
}

// 测试 combineMetricInfos 方法的边界情况
func TestMetricsService_combineMetricInfos_EdgeCases(t *testing.T) {
	service := &MetricsService{}

	tests := []struct {
		name          string
		mInfos        []*metricInfo
		expectedError string
	}{
		{
			name:          "EmptyMetricInfos",
			mInfos:        []*metricInfo{},
			expectedError: "invalid param",
		},
		{
			name: "DifferentWhere",
			mInfos: []*metricInfo{
				{
					mType:        entity.MetricTypeTimeSeries,
					mAggregation: []*entity.Dimension{{Expression: "count(*)", Alias: "metric1"}},
					mGroupBy:     []*entity.Dimension{},
					mWhere: []*loop_span.FilterField{{
						FieldName: "status",
						FieldType: loop_span.FieldTypeString,
						QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
						Values:    []string{"success"},
					}},
				},
				{
					mType:        entity.MetricTypeTimeSeries,
					mAggregation: []*entity.Dimension{{Expression: "sum(value)", Alias: "metric2"}},
					mGroupBy:     []*entity.Dimension{},
					mWhere: []*loop_span.FilterField{{
						FieldName: "status",
						FieldType: loop_span.FieldTypeString,
						QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
						Values:    []string{"failed"},
					}}, // 不同的 Where 条件
				},
			},
			expectedError: "metric condition not the same",
		},
		{
			name: "DifferentGroupBy",
			mInfos: []*metricInfo{
				{
					mType:        entity.MetricTypeTimeSeries,
					mAggregation: []*entity.Dimension{{Expression: "count(*)", Alias: "metric1"}},
					mGroupBy:     []*entity.Dimension{{Expression: "region", Alias: "region"}},
					mWhere:       []*loop_span.FilterField{},
				},
				{
					mType:        entity.MetricTypeTimeSeries,
					mAggregation: []*entity.Dimension{{Expression: "sum(value)", Alias: "metric2"}},
					mGroupBy:     []*entity.Dimension{{Expression: "env", Alias: "env"}}, // 不同的 GroupBy
					mWhere:       []*loop_span.FilterField{},
				},
			},
			expectedError: "metric groupby not the same",
		},
		{
			name: "SuccessfulCombination",
			mInfos: []*metricInfo{
				{
					mType:        entity.MetricTypeTimeSeries,
					mAggregation: []*entity.Dimension{{Expression: "count(*)", Alias: "metric1"}},
					mGroupBy:     []*entity.Dimension{{Expression: "region", Alias: "region"}},
					mWhere:       []*loop_span.FilterField{},
				},
				{
					mType:        entity.MetricTypeTimeSeries,
					mAggregation: []*entity.Dimension{{Expression: "sum(value)", Alias: "metric2"}},
					mGroupBy:     []*entity.Dimension{{Expression: "region", Alias: "region"}}, // 相同的 GroupBy
					mWhere:       []*loop_span.FilterField{},
				},
			},
			expectedError: "", // 期望成功
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.combineMetricInfos(tt.mInfos)

			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("Expected error containing '%s', but got nil", tt.expectedError)
					return
				}
				if !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error containing '%s', but got '%s'", tt.expectedError, err.Error())
				}
			} else if err != nil {
				t.Errorf("Expected no error, but got: %v", err)
			}

			if tt.expectedError != "" && result != nil {
				t.Errorf("Expected nil result when error occurs, but got: %+v", result)
			} else if tt.expectedError == "" && result == nil {
				t.Error("Expected valid result when no error, but got nil")
			}
		})
	}
}

// 测试 buildMetricFilter 方法的边界情况
func TestMetricsService_buildMetricFilter_EdgeCases(t *testing.T) {
	service := &MetricsService{}
	ctx := context.Background()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockFilter := filtermocks.NewMockFilter(ctrl)
	spanEnv := &span_filter.SpanEnv{WorkspaceID: testWorkspaceID}
	metricFilters := []*loop_span.FilterField{{
		FieldName: "metric_field",
		FieldType: loop_span.FieldTypeString,
		QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
		Values:    []string{"metric_value"},
	}}
	requestFilter := &loop_span.FilterFields{
		FilterFields: []*loop_span.FilterField{{
			FieldName: "request_field",
			FieldType: loop_span.FieldTypeString,
			QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
			Values:    []string{"request_value"},
		}},
	}

	tests := []struct {
		name          string
		setupMock     func()
		expectedError string
		expectNil     bool
	}{
		{
			name: "BuildBasicSpanFilterError",
			setupMock: func() {
				mockFilter.EXPECT().BuildBasicSpanFilter(ctx, spanEnv).Return(nil, false, errors.New("basic filter error"))
			},
			expectedError: "basic filter error",
		},
		{
			name: "EmptyBasicFilterNoForceQuery",
			setupMock: func() {
				mockFilter.EXPECT().BuildBasicSpanFilter(ctx, spanEnv).Return([]*loop_span.FilterField{}, false, nil)
			},
			expectNil: true,
		},
		{
			name: "EmptyBasicFilterWithForceQuery",
			setupMock: func() {
				mockFilter.EXPECT().BuildBasicSpanFilter(ctx, spanEnv).Return([]*loop_span.FilterField{}, true, nil)
			},
			expectedError: "",
		},
		{
			name: "SuccessfulBuild",
			setupMock: func() {
				mockFilter.EXPECT().BuildBasicSpanFilter(ctx, spanEnv).Return([]*loop_span.FilterField{{
					FieldName: "basic_field",
					FieldType: loop_span.FieldTypeString,
					QueryType: ptr.Of(loop_span.QueryTypeEnumEq),
					Values:    []string{"basic_value"},
				}}, true, nil)
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()

			result, err := service.buildMetricFilter(ctx, mockFilter, spanEnv, metricFilters, requestFilter)

			if tt.expectedError != "" {
				if err == nil {
					t.Errorf("Expected error containing '%s', but got nil", tt.expectedError)
					return
				}
				if !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("Expected error containing '%s', but got '%s'", tt.expectedError, err.Error())
				}
			} else if err != nil {
				t.Errorf("Expected no error, but got: %v", err)
			}

			if tt.expectNil && result != nil {
				t.Errorf("Expected nil result, but got: %+v", result)
			} else if !tt.expectNil && tt.expectedError == "" && result == nil {
				t.Error("Expected non-nil result, but got nil")
			}
		})
	}
}

// 测试错误代码是否正确
func TestMetricsService_QueryMetrics_ErrorCodes(t *testing.T) {
	service, ctrl, _, mockTenantProvider, mockBuildHelper, mockFilter := setupQueryMetricsTest(t)
	defer ctrl.Finish()

	mockBuildHelper.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), loop_span.PlatformOpenAPI).Return(mockFilter, nil)
	mockTenantProvider.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformOpenAPI).Return([]string{"tenant1"}, nil)

	req := &QueryMetricsReq{
		PlatformType: loop_span.PlatformOpenAPI,
		WorkspaceID:  testWorkspaceID,
		MetricsNames: []entity.MetricName{"nonexistent_metric"},
		Granularity:  entity.MetricsGranularity5Min,
		StartTime:    testStartTime,
		EndTime:      testEndTime,
	}

	_, err := service.QueryMetrics(context.Background(), req)

	if err == nil {
		t.Error("Expected error for nonexistent metric, but got nil")
		return
	}

	// 检查错误是否包含正确的错误代码信息
	if !strings.Contains(err.Error(), "600904002") {
		t.Errorf("Expected error containing error code 600904002, but got: %s", err.Error())
	}
}

// 辅助函数：检查错误信息是否包含预期内容
func containsError(actual, expected string) bool {
	if actual == "" || expected == "" {
		return false
	}
	// 检查是否包含预期的错误信息
	return len(actual) >= len(expected) &&
		(actual == expected ||
			strings.Contains(actual, expected))
}
