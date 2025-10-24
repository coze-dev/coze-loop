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
