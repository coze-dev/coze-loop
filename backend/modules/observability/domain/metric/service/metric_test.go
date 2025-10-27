// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant"
	tenantmocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	trace_service "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service"
	traceServicemocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/mocks"
	spanfilter "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter"
	spanfiltermocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/service/trace/span_filter/mocks"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestNewMetricsService(t *testing.T) {
	t.Parallel()

	t.Run("success with unique metrics", func(t *testing.T) {
		t.Parallel()
		defs := []entity.IMetricDefinition{
			&testMetricDefinition{name: "metric_a", metricType: entity.MetricTypeSummary},
		}
		svc, err := NewMetricsService(nil, defs, nil, nil)
		assert.NoError(t, err)
		assert.NotNil(t, svc)
	})

	t.Run("duplicate metric name", func(t *testing.T) {
		t.Parallel()
		defs := []entity.IMetricDefinition{
			&testMetricDefinition{name: "metric_a", metricType: entity.MetricTypeSummary},
			&testMetricDefinition{name: "metric_a", metricType: entity.MetricTypeSummary},
		}
		svc, err := NewMetricsService(nil, defs, nil, nil)
		assert.Error(t, err)
		assert.Nil(t, svc)
	})
}

func TestMetricsService_QueryMetrics(t *testing.T) {
	t.Parallel()

	type fields struct {
		metricRepo     repo.IMetricRepo
		tenantProvider tenant.ITenantProvider
		builder        trace_service.TraceFilterProcessorBuilder
		metricDefs     []entity.IMetricDefinition
		capturedParams *[]*repo.GetMetricsParam
	}

	type args struct {
		ctx context.Context
		req *QueryMetricsReq
	}

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      bool
		postCheck    func(t *testing.T, f fields, resp *QueryMetricsResp)
	}{
		{
			name: "time series success",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockIMetricRepo(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				builderMock := traceServicemocks.NewMockTraceFilterProcessorBuilder(ctrl)
				filterMock := spanfiltermocks.NewMockFilter(ctrl)
				captured := make([]*repo.GetMetricsParam, 0)

				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"tenant-1"}, nil)
				builderMock.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), gomock.Any()).Return(filterMock, nil)
				filterMock.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, env *spanfilter.SpanEnv) ([]*loop_span.FilterField, bool, error) {
						assert.Equal(t, int64(1), env.WorkspaceID)
						return []*loop_span.FilterField{{FieldName: "workspace"}}, true, nil
					},
				)
				repoMock.EXPECT().GetMetrics(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, param *repo.GetMetricsParam) (*repo.GetMetricsResult, error) {
						captured = append(captured, param)
						assert.Equal(t, []string{"tenant-1"}, param.Tenants)
						assert.NotNil(t, param.Filters)
						return &repo.GetMetricsResult{
							Data: []map[string]any{{
								"time_bucket": "0",
								"metric_a":    "3",
							}},
						}, nil
					},
				)
				metricDef := &testMetricDefinition{
					name:       "metric_a",
					metricType: entity.MetricTypeTimeSeries,
				}
				return fields{
					metricRepo:     repoMock,
					tenantProvider: tenantMock,
					builder:        builderMock,
					metricDefs:     []entity.IMetricDefinition{metricDef},
					capturedParams: &captured,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &QueryMetricsReq{
					PlatformType: loop_span.PlatformType("loop"),
					WorkspaceID:  1,
					MetricsNames: []string{"metric_a"},
					Granularity:  entity.MetricGranularity1Hour,
					StartTime:    0,
					EndTime:      0,
				},
			},
			wantErr: false,
			postCheck: func(t *testing.T, f fields, resp *QueryMetricsResp) {
				assert.NotNil(t, resp)
				assert.Equal(t, "3", resp.Metrics["metric_a"].TimeSeries["all"][0].Value)
				if f.capturedParams != nil {
					captured := *f.capturedParams
					if assert.Len(t, captured, 1) {
						assert.Equal(t, entity.MetricGranularity1Hour, captured[0].Granularity)
						assert.Equal(t, int64(0), captured[0].StartAt)
						assert.Equal(t, int64(0), captured[0].EndAt)
					}
				}
			},
		},
		{
			name: "filter returns nil",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockIMetricRepo(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				builderMock := traceServicemocks.NewMockTraceFilterProcessorBuilder(ctrl)
				filterMock := spanfiltermocks.NewMockFilter(ctrl)

				repoMock.EXPECT().GetMetrics(gomock.Any(), gomock.Any()).Times(0)
				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"tenant-1"}, nil)
				builderMock.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), gomock.Any()).Return(filterMock, nil)
				filterMock.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return(nil, false, nil)

				metricDef := &testMetricDefinition{
					name:       "metric_a",
					metricType: entity.MetricTypeTimeSeries,
				}
				return fields{
					metricRepo:     repoMock,
					tenantProvider: tenantMock,
					builder:        builderMock,
					metricDefs:     []entity.IMetricDefinition{metricDef},
				}
			},
			args: args{
				ctx: context.Background(),
				req: &QueryMetricsReq{
					PlatformType: loop_span.PlatformCozeLoop,
					WorkspaceID:  2,
					MetricsNames: []string{"metric_a"},
					Granularity:  entity.MetricGranularity1Hour,
					StartTime:    0,
					EndTime:      0,
				},
			},
			wantErr: false,
			postCheck: func(t *testing.T, _ fields, resp *QueryMetricsResp) {
				assert.NotNil(t, resp)
				assert.Empty(t, resp.Metrics)
			},
		},
		{
			name: "metric definition not found",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{
					metricRepo:     repomocks.NewMockIMetricRepo(ctrl),
					tenantProvider: tenantmocks.NewMockITenantProvider(ctrl),
					builder:        traceServicemocks.NewMockTraceFilterProcessorBuilder(ctrl),
					metricDefs:     []entity.IMetricDefinition{},
				}
			},
			args: args{
				ctx: context.Background(),
				req: &QueryMetricsReq{
					PlatformType: loop_span.PlatformCozeLoop,
					WorkspaceID:  1,
					MetricsNames: []string{"unknown"},
					Granularity:  entity.MetricGranularity1Hour,
					StartTime:    0,
					EndTime:      0,
				},
			},
			wantErr:   true,
			postCheck: nil,
		},
		{
			name: "compound divide metric time series",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockIMetricRepo(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				builderMock := traceServicemocks.NewMockTraceFilterProcessorBuilder(ctrl)
				filterMock := spanfiltermocks.NewMockFilter(ctrl)

				numerator := &testMetricDefinition{
					name:       "metric_numerator",
					metricType: entity.MetricTypeTimeSeries,
				}
				denominator := &testMetricDefinition{
					name:       "metric_denominator",
					metricType: entity.MetricTypeTimeSeries,
				}
				compound := &testCompoundMetricDefinition{
					testMetricDefinition: &testMetricDefinition{
						name:       "metric_ratio",
						metricType: entity.MetricTypeTimeSeries,
					},
					metrics:  []entity.IMetricDefinition{numerator, denominator},
					operator: entity.MetricOperatorDivide,
				}

				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"tenant-1"}, nil).Times(2)
				builderMock.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), gomock.Any()).Return(filterMock, nil).Times(2)
				filterMock.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, env *spanfilter.SpanEnv) ([]*loop_span.FilterField, bool, error) {
						assert.Equal(t, int64(3), env.WorkspaceID)
						return []*loop_span.FilterField{{FieldName: "workspace"}}, true, nil
					},
				).Times(2)
				repoMock.EXPECT().GetMetrics(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, param *repo.GetMetricsParam) (*repo.GetMetricsResult, error) {
						assert.Equal(t, []string{"tenant-1"}, param.Tenants)
						alias := ""
						if len(param.Aggregations) > 0 && param.Aggregations[0] != nil {
							alias = param.Aggregations[0].Alias
						}
						switch alias {
						case "metric_numerator":
							return &repo.GetMetricsResult{
								Data: []map[string]any{{
									"time_bucket":      "0",
									"metric_numerator": "10",
								}},
							}, nil
						case "metric_denominator":
							return &repo.GetMetricsResult{
								Data: []map[string]any{{
									"time_bucket":        "0",
									"metric_denominator": "2",
								}},
							}, nil
						default:
							return nil, assert.AnError
						}
					},
				).Times(2)

				return fields{
					metricRepo:     repoMock,
					tenantProvider: tenantMock,
					builder:        builderMock,
					metricDefs:     []entity.IMetricDefinition{numerator, denominator, compound},
				}
			},
			args: args{
				ctx: context.Background(),
				req: &QueryMetricsReq{
					PlatformType: loop_span.PlatformType("loop"),
					WorkspaceID:  3,
					MetricsNames: []string{"metric_ratio"},
					Granularity:  entity.MetricGranularity1Hour,
					StartTime:    0,
					EndTime:      0,
				},
			},
			wantErr: false,
			postCheck: func(t *testing.T, _ fields, resp *QueryMetricsResp) {
				assert.NotNil(t, resp)
				metric := resp.Metrics["metric_ratio"]
				if assert.NotNil(t, metric) {
					assert.Equal(t, "5", metric.TimeSeries["all"][0].Value)
				}
			},
		},
		{
			name: "compound pie metric summary",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				repoMock := repomocks.NewMockIMetricRepo(ctrl)
				tenantMock := tenantmocks.NewMockITenantProvider(ctrl)
				builderMock := traceServicemocks.NewMockTraceFilterProcessorBuilder(ctrl)
				filterMock := spanfiltermocks.NewMockFilter(ctrl)

				metricA := &testMetricDefinition{
					name:       "metric_a_summary",
					metricType: entity.MetricTypeSummary,
				}
				metricB := &testMetricDefinition{
					name:       "metric_b_summary",
					metricType: entity.MetricTypeSummary,
				}
				compound := &testCompoundMetricDefinition{
					testMetricDefinition: &testMetricDefinition{
						name:       "metric_pie",
						metricType: entity.MetricTypePie,
					},
					metrics:  []entity.IMetricDefinition{metricA, metricB},
					operator: entity.MetricOperatorPie,
				}

				tenantMock.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"tenant-2"}, nil).Times(2)
				builderMock.EXPECT().BuildPlatformRelatedFilter(gomock.Any(), gomock.Any()).Return(filterMock, nil).Times(2)
				filterMock.EXPECT().BuildBasicSpanFilter(gomock.Any(), gomock.Any()).Return([]*loop_span.FilterField{{FieldName: "workspace"}}, true, nil).Times(2)
				repoMock.EXPECT().GetMetrics(gomock.Any(), gomock.Any()).DoAndReturn(
					func(_ context.Context, param *repo.GetMetricsParam) (*repo.GetMetricsResult, error) {
						alias := ""
						if len(param.Aggregations) > 0 && param.Aggregations[0] != nil {
							alias = param.Aggregations[0].Alias
						}
						switch alias {
						case "metric_a_summary":
							return &repo.GetMetricsResult{Data: []map[string]any{{"metric_a_summary": "4"}}}, nil
						case "metric_b_summary":
							return &repo.GetMetricsResult{Data: []map[string]any{{"metric_b_summary": "1"}}}, nil
						default:
							return nil, assert.AnError
						}
					},
				).Times(2)

				return fields{
					metricRepo:     repoMock,
					tenantProvider: tenantMock,
					builder:        builderMock,
					metricDefs:     []entity.IMetricDefinition{metricA, metricB, compound},
				}
			},
			args: args{
				ctx: context.Background(),
				req: &QueryMetricsReq{
					PlatformType: loop_span.PlatformType("loop"),
					WorkspaceID:  4,
					MetricsNames: []string{"metric_pie"},
					Granularity:  entity.MetricGranularity1Hour,
					StartTime:    0,
					EndTime:      0,
				},
			},
			wantErr: false,
			postCheck: func(t *testing.T, _ fields, resp *QueryMetricsResp) {
				assert.NotNil(t, resp)
				metric := resp.Metrics["metric_pie"]
				if assert.NotNil(t, metric) {
					assert.Equal(t, map[string]string{
						"metric_a_summary": "4",
						"metric_b_summary": "1",
					}, metric.Pie)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			f := tt.fieldsGetter(ctrl)
			svc, err := NewMetricsService(f.metricRepo, f.metricDefs, f.tenantProvider, f.builder)
			assert.NoError(t, err)
			resp, err := svc.QueryMetrics(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			if tt.wantErr {
				assert.Nil(t, resp)
				return
			}
			if tt.postCheck != nil {
				ttPost := tt.postCheck
				ttPost(t, f, resp)
			}
		})
	}
}

type testMetricDefinition struct {
	name       string
	metricType entity.MetricType
	groupBy    []*entity.Dimension
	where      []*loop_span.FilterField
}

func (d *testMetricDefinition) Name() string {
	return d.name
}

func (d *testMetricDefinition) Type() entity.MetricType {
	return d.metricType
}

func (d *testMetricDefinition) Source() entity.MetricSource {
	return entity.MetricSourceCK
}

func (d *testMetricDefinition) Expression(entity.MetricGranularity) *entity.Expression {
	return &entity.Expression{Expression: "count()"}
}

func (d *testMetricDefinition) Where(context.Context, spanfilter.Filter, *spanfilter.SpanEnv) ([]*loop_span.FilterField, error) {
	return d.where, nil
}

func (d *testMetricDefinition) GroupBy() []*entity.Dimension {
	return d.groupBy
}

type testCompoundMetricDefinition struct {
	*testMetricDefinition
	metrics  []entity.IMetricDefinition
	operator entity.MetricOperator
}

func (d *testCompoundMetricDefinition) GetMetrics() []entity.IMetricDefinition {
	return d.metrics
}

func (d *testCompoundMetricDefinition) Operator() entity.MetricOperator {
	return d.operator
}

func TestDivideNumber(t *testing.T) {
	t.Parallel()
	t.Run("valid division", func(t *testing.T) {
		assert.Equal(t, "2.5", divideNumber("5", "2"))
	})
	t.Run("invalid inputs", func(t *testing.T) {
		assert.Equal(t, "", divideNumber("NaN", "2"))
		assert.Equal(t, "", divideNumber("5", "0"))
		assert.Equal(t, "", divideNumber("-1", "1"))
	})
}

func TestDivideTimeSeries(t *testing.T) {
	t.Parallel()
	seriesA := entity.TimeSeries{
		"group": {
			{Timestamp: "2", Value: "9"},
			{Timestamp: "1", Value: "10"},
		},
		"mismatch": {
			{Timestamp: "1", Value: "1"},
		},
	}
	seriesB := entity.TimeSeries{
		"group": {
			{Timestamp: "1", Value: "2"},
			{Timestamp: "2", Value: "0"},
		},
		"mismatch": {
			{Timestamp: "1", Value: "1"},
			{Timestamp: "2", Value: "1"},
		},
	}
	ret := divideTimeSeries(context.Background(), seriesA, seriesB)
	assert.Len(t, ret, 1)
	points := ret["group"]
	if assert.Len(t, points, 2) {
		assert.Equal(t, "1", points[0].Timestamp)
		assert.Equal(t, "5", points[0].Value)
		assert.Equal(t, "2", points[1].Timestamp)
		assert.Equal(t, "null", points[1].Value)
	}
}

func TestMetricsService_PieMetrics(t *testing.T) {
	t.Parallel()
	svc := &MetricsService{}
	resp, err := svc.pieMetrics(context.Background(), []*QueryMetricsResp{
		{Metrics: map[string]*entity.Metric{
			"metric_a": {Summary: "1"},
		}},
		{Metrics: map[string]*entity.Metric{
			"metric_b": {Summary: "3"},
		}},
	}, "metric_pie")
	assert.NoError(t, err)
	metric := resp.Metrics["metric_pie"]
	if assert.NotNil(t, metric) {
		assert.Equal(t, map[string]string{
			"metric_a": "1",
			"metric_b": "3",
		}, metric.Pie)
	}
}
