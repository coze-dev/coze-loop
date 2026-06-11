// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	commondto "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	metricpb "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/metric"
	metricapi "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/metric"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc"
	rpcmock "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/rpc/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service"
	metricservicemock "github.com/coze-dev/coze-loop/backend/modules/observability/domain/metric/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

type safeMetricsRequests struct {
	mu   sync.Mutex
	list []*service.QueryMetricsReq
}

func (s *safeMetricsRequests) add(req *service.QueryMetricsReq) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.list = append(s.list, req)
}

func (s *safeMetricsRequests) snapshot() []*service.QueryMetricsReq {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*service.QueryMetricsReq, len(s.list))
	copy(out, s.list)
	return out
}

func TestMetricApplication_GetMetrics(t *testing.T) {
	t.Parallel()

	type fields struct {
		metricSvc service.IMetricsService
		auth      rpc.IAuthProvider
		captured  *safeMetricsRequests
	}

	type args struct {
		ctx context.Context
		req *metricapi.GetMetricsRequest
	}

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      bool
		postCheck    func(t *testing.T, f fields, got *metricapi.GetMetricsResponse)
	}{
		{
			name: "success without compare",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				metricMock := metricservicemock.NewMockIMetricsService(ctrl)
				authMock := rpcmock.NewMockIAuthProvider(ctrl)
				captured := &safeMetricsRequests{}
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceMetricRead, "1", false).Return(nil)
				metricMock.EXPECT().QueryMetrics(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *service.QueryMetricsReq) (*service.QueryMetricsResp, error) {
					captured.add(req)
					return &service.QueryMetricsResp{
						Metrics: map[string]*entity.Metric{
							"metric_a": {
								Summary: "10",
								Pie:     map[string]string{"foo": "1"},
								TimeSeries: entity.TimeSeries{
									"all": {{Timestamp: "1", Value: "2"}},
								},
							},
						},
					}, nil
				}).Times(1)
				return fields{
					metricSvc: metricMock,
					auth:      authMock,
					captured:  captured,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &metricapi.GetMetricsRequest{
					WorkspaceID: 1,
					StartTime:   1000,
					EndTime:     2000,
					MetricNames: []string{"metric_a"},
				},
			},
			wantErr: false,
			postCheck: func(t *testing.T, f fields, got *metricapi.GetMetricsResponse) {
				assert.NotNil(t, got)
				assert.Equal(t, "10", got.Metrics["metric_a"].GetSummary())
				assert.Equal(t, "1", got.Metrics["metric_a"].GetPie()["foo"])
				assert.Len(t, got.Metrics["metric_a"].GetTimeSeries()["all"], 1)
				if f.captured != nil {
					captured := f.captured.snapshot()
					if assert.Len(t, captured, 1) {
						assert.Equal(t, int64(1000), captured[0].StartTime)
						assert.Equal(t, int64(2000), captured[0].EndTime)
						assert.Equal(t, entity.MetricGranularity1Day, captured[0].Granularity)
					}
				}
			},
		},
		{
			name: "success with compare",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				metricMock := metricservicemock.NewMockIMetricsService(ctrl)
				authMock := rpcmock.NewMockIAuthProvider(ctrl)
				captured := &safeMetricsRequests{}
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceMetricRead, "2", false).Return(nil)
				metricMock.EXPECT().QueryMetrics(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *service.QueryMetricsReq) (*service.QueryMetricsResp, error) {
					captured.add(req)
					summary := "2"
					if req.StartTime == 2000 && req.EndTime == 4000 {
						summary = "1"
					}
					return &service.QueryMetricsResp{
						Metrics: map[string]*entity.Metric{
							"metric_a": {Summary: summary},
						},
					}, nil
				}).Times(2)
				return fields{
					metricSvc: metricMock,
					auth:      authMock,
					captured:  captured,
				}
			},
			args: args{
				ctx: context.Background(),
				req: func() *metricapi.GetMetricsRequest {
					compareType := metricpb.CompareTypeMoM
					return &metricapi.GetMetricsRequest{
						WorkspaceID: 2,
						StartTime:   2000,
						EndTime:     4000,
						MetricNames: []string{"metric_a"},
						Compare: &metricpb.Compare{
							CompareType: &compareType,
						},
					}
				}(),
			},
			wantErr: false,
			postCheck: func(t *testing.T, f fields, got *metricapi.GetMetricsResponse) {
				assert.NotNil(t, got)
				assert.Equal(t, "1", got.Metrics["metric_a"].GetSummary())
				assert.Equal(t, "2", got.ComparedMetrics["metric_a"].GetSummary())
				if f.captured != nil {
					captured := f.captured.snapshot()
					if assert.Len(t, captured, 2) {
						startEnds := map[string]bool{}
						for _, req := range captured {
							key := strconv.FormatInt(req.StartTime, 10) + ":" + strconv.FormatInt(req.EndTime, 10)
							startEnds[key] = true
						}
						assert.True(t, startEnds["2000:4000"])
						assert.True(t, startEnds["0:2000"])
					}
				}
			},
		},
		{
			name: "validate error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				req: &metricapi.GetMetricsRequest{
					WorkspaceID: 1,
					StartTime:   2000,
					EndTime:     1000,
					MetricNames: []string{"metric_a"},
				},
			},
			wantErr:   true,
			postCheck: nil,
		},
		{
			name: "auth error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				metricMock := metricservicemock.NewMockIMetricsService(ctrl)
				authMock := rpcmock.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceMetricRead, "3", false).Return(assert.AnError)
				return fields{
					metricSvc: metricMock,
					auth:      authMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &metricapi.GetMetricsRequest{
					WorkspaceID: 3,
					StartTime:   1000,
					EndTime:     2000,
					MetricNames: []string{"metric_a"},
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
			fieldVals := tt.fieldsGetter(ctrl)
			app := &MetricApplication{
				metricService: fieldVals.metricSvc,
				authSvc:       fieldVals.auth,
			}
			got, err := app.GetMetrics(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			if tt.wantErr {
				assert.Nil(t, got)
			} else if tt.postCheck != nil {
				ttFields := fieldVals
				ttPost := tt.postCheck
				ttPost(t, ttFields, got)
			}
		})
	}
}

func TestMetricApplication_buildGetMetricsReq(t *testing.T) {
	t.Parallel()
	app := &MetricApplication{}

	type testCase struct {
		name     string
		prepare  func() *metricapi.GetMetricsRequest
		assertFn func(t *testing.T, got *service.QueryMetricsReq)
	}

	tests := []testCase{
		{
			name: "with granularity",
			prepare: func() *metricapi.GetMetricsRequest {
				gran := string(entity.MetricGranularity1Hour)
				platform := commondto.PlatformType("bot")
				req := &metricapi.GetMetricsRequest{
					WorkspaceID: 10,
					StartTime:   100,
					EndTime:     200,
					MetricNames: []string{"a", "b"},
					Granularity: &gran,
				}
				req.SetPlatformType(&platform)
				return req
			},
			assertFn: func(t *testing.T, got *service.QueryMetricsReq) {
				assert.Equal(t, loop_span.PlatformType("bot"), got.PlatformType)
				assert.Equal(t, int64(10), got.WorkspaceID)
				assert.Equal(t, []string{"a", "b"}, got.MetricsNames)
				assert.Equal(t, entity.MetricGranularity1Hour, got.Granularity)
			},
		},
		{
			name: "default granularity",
			prepare: func() *metricapi.GetMetricsRequest {
				req := &metricapi.GetMetricsRequest{
					WorkspaceID: 11,
					StartTime:   100,
					EndTime:     200,
					MetricNames: []string{"x"},
				}
				return req
			},
			assertFn: func(t *testing.T, got *service.QueryMetricsReq) {
				assert.Equal(t, entity.MetricGranularity1Day, got.Granularity)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := tt.prepare()
			got := app.buildGetMetricsReq(req)
			tt := tt
			tt.assertFn(t, got)
		})
	}
}

func TestMetricApplication_shouldCompareWith(t *testing.T) {
	t.Parallel()
	app := &MetricApplication{}

	type testCase struct {
		name      string
		compare   *entity.Compare
		start     int64
		end       int64
		expectNil bool
		expect    func(t *testing.T, start, end int64, ok bool)
	}

	tests := []testCase{
		{
			name:      "nil compare",
			compare:   nil,
			start:     1000,
			end:       2000,
			expectNil: true,
		},
		{
			name: "mom compare",
			compare: &entity.Compare{
				Type: entity.MetricCompareTypeMoM,
			},
			start: 1000,
			end:   2000,
			expect: func(t *testing.T, newStart, newEnd int64, ok bool) {
				assert.True(t, ok)
				assert.Equal(t, int64(0), newStart)
				assert.Equal(t, int64(1000), newEnd)
			},
		},
		{
			name: "yoy compare",
			compare: &entity.Compare{
				Type:  entity.MetricCompareTypeYoY,
				Shift: 10,
			},
			start: 1000,
			end:   2000,
			expect: func(t *testing.T, newStart, newEnd int64, ok bool) {
				assert.True(t, ok)
				assert.Equal(t, int64(1000-10*1000), newStart)
				assert.Equal(t, int64(2000-10*1000), newEnd)
			},
		},
		{
			name: "unknown compare",
			compare: &entity.Compare{
				Type: "unknown",
			},
			start:     1000,
			end:       2000,
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			newStart, newEnd, ok := app.shouldCompareWith(tt.start, tt.end, tt.compare)
			if tt.expectNil {
				assert.False(t, ok)
				assert.Zero(t, newStart)
				assert.Zero(t, newEnd)
			} else {
				tt := tt
				tt.expect(t, newStart, newEnd, ok)
			}
		})
	}
}

func TestMetricApplication_validateGetMetricsReq(t *testing.T) {
	t.Parallel()
	app := &MetricApplication{}
	ctx := context.Background()

	type testCase struct {
		name    string
		req     *metricapi.GetMetricsRequest
		wantErr bool
	}

	tests := []testCase{
		{
			name: "start greater than end",
			req: &metricapi.GetMetricsRequest{
				StartTime:   2000,
				EndTime:     1000,
				MetricNames: []string{"metric_a"},
			},
			wantErr: true,
		},
		{
			name: "granularity 1min out of range",
			req: func() *metricapi.GetMetricsRequest {
				gran := string(entity.MetricGranularity1Min)
				return &metricapi.GetMetricsRequest{
					StartTime:   0,
					EndTime:     4 * time.Hour.Milliseconds(),
					Granularity: &gran,
					MetricNames: []string{"metric_a"},
				}
			}(),
			wantErr: true,
		},
		{
			name: "granularity 1hour out of range",
			req: func() *metricapi.GetMetricsRequest {
				gran := string(entity.MetricGranularity1Hour)
				return &metricapi.GetMetricsRequest{
					StartTime:   0,
					EndTime:     7 * 24 * time.Hour.Milliseconds(),
					Granularity: &gran,
					MetricNames: []string{"metric_a"},
				}
			}(),
			wantErr: true,
		},
		{
			name: "valid request",
			req: &metricapi.GetMetricsRequest{
				StartTime:   0,
				EndTime:     time.Hour.Milliseconds(),
				MetricNames: []string{"metric_a"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := app.validateGetMetricsReq(ctx, tt.req)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func TestMetricApplication_GetDrillDownValues(t *testing.T) {
	t.Parallel()

	type fields struct {
		metricSvc service.IMetricsService
		auth      rpc.IAuthProvider
		captured  *safeMetricsRequests
	}

	type args struct {
		ctx context.Context
		req *metricapi.GetDrillDownValuesRequest
	}

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      bool
		postCheck    func(t *testing.T, f fields, got *metricapi.GetDrillDownValuesResponse)
	}{
		{
			name: "success model name",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				metricMock := metricservicemock.NewMockIMetricsService(ctrl)
				authMock := rpcmock.NewMockIAuthProvider(ctrl)
				captured := &safeMetricsRequests{}
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceMetricRead, "5", false).Return(nil)
				metricMock.EXPECT().QueryMetrics(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *service.QueryMetricsReq) (*service.QueryMetricsResp, error) {
					captured.add(req)
					return &service.QueryMetricsResp{
						Metrics: map[string]*entity.Metric{
							entity.MetricNameModelTotalCountPie: {
								Pie: map[string]string{
									`{"name":"modelA"}`: "1",
									`{"name":"modelB"}`: "2",
								},
							},
						},
					}, nil
				}).Times(1)
				metricMock.EXPECT().GetMetricGroupBy(entity.MetricNameModelTotalCountPie).Return([]string{"name"}, nil).Times(1)
				return fields{
					metricSvc: metricMock,
					auth:      authMock,
					captured:  captured,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &metricapi.GetDrillDownValuesRequest{
					WorkspaceID:        5,
					StartTime:          0,
					EndTime:            10 * 24 * time.Hour.Milliseconds(),
					DrillDownValueType: metricpb.DrillDownValueTypeModelName,
				},
			},
			wantErr: false,
			postCheck: func(t *testing.T, f fields, got *metricapi.GetDrillDownValuesResponse) {
				assert.NotNil(t, got)
				if f.captured != nil {
					captured := f.captured.snapshot()
					if assert.Len(t, captured, 1) {
						assert.Equal(t, int64(0), captured[0].StartTime)
						assert.Equal(t, 10*24*time.Hour.Milliseconds(), captured[0].EndTime)
						assert.Equal(t, []string{entity.MetricNameModelTotalCountPie}, captured[0].MetricsNames)
					}
				}
				assert.Len(t, got.DrillDownValues, 2)
				assert.Equal(t, "modelB", got.DrillDownValues[0].Value) // sorted by count desc
				assert.Equal(t, "modelA", got.DrillDownValues[1].Value)
			},
		},
		{
			name: "success nested drill down",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				metricMock := metricservicemock.NewMockIMetricsService(ctrl)
				authMock := rpcmock.NewMockIAuthProvider(ctrl)
				captured := &safeMetricsRequests{}
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceMetricRead, "6", false).Return(nil)
				metricMock.EXPECT().QueryMetrics(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *service.QueryMetricsReq) (*service.QueryMetricsResp, error) {
					captured.add(req)
					return &service.QueryMetricsResp{
						Metrics: map[string]*entity.Metric{
							entity.MetricNameModelTotalCountPie: {
								Pie: map[string]string{
									`{"region":"us","zone":"east"}`:  "10",
									`{"region":"us","zone":"west"}`:  "20",
									`{"region":"cn","zone":"north"}`: "5",
								},
							},
						},
					}, nil
				}).Times(1)
				metricMock.EXPECT().GetMetricGroupBy(entity.MetricNameModelTotalCountPie).Return([]string{"region", "zone"}, nil).Times(1)
				return fields{
					metricSvc: metricMock,
					auth:      authMock,
					captured:  captured,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &metricapi.GetDrillDownValuesRequest{
					WorkspaceID:        6,
					StartTime:          0,
					EndTime:            10 * 24 * time.Hour.Milliseconds(),
					DrillDownValueType: metricpb.DrillDownValueTypeModelName,
				},
			},
			wantErr: false,
			postCheck: func(t *testing.T, f fields, got *metricapi.GetDrillDownValuesResponse) {
				assert.NotNil(t, got)
				assert.Len(t, got.DrillDownValues, 2)

				// 1. region: us (total 30)
				assert.Equal(t, "us", got.DrillDownValues[0].Value)
				assert.Len(t, got.DrillDownValues[0].SubDrillDownValues, 2)
				//    - zone: west (20)
				assert.Equal(t, "west", got.DrillDownValues[0].SubDrillDownValues[0].Value)
				//    - zone: east (10)
				assert.Equal(t, "east", got.DrillDownValues[0].SubDrillDownValues[1].Value)

				// 2. region: cn (total 5)
				assert.Equal(t, "cn", got.DrillDownValues[1].Value)
				assert.Len(t, got.DrillDownValues[1].SubDrillDownValues, 1)
				//    - zone: north (5)
				assert.Equal(t, "north", got.DrillDownValues[1].SubDrillDownValues[0].Value)
			},
		},
		{
			name: "invalid type",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				metricMock := metricservicemock.NewMockIMetricsService(ctrl)
				authMock := rpcmock.NewMockIAuthProvider(ctrl)
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceMetricRead, "5", false).Return(nil)
				return fields{
					metricSvc: metricMock,
					auth:      authMock,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &metricapi.GetDrillDownValuesRequest{
					WorkspaceID:        5,
					StartTime:          0,
					EndTime:            1,
					DrillDownValueType: "unknown",
				},
			},
			wantErr:   true,
			postCheck: nil,
		},
		{
			name: "validate error",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				return fields{}
			},
			args: args{
				ctx: context.Background(),
				req: &metricapi.GetDrillDownValuesRequest{
					WorkspaceID:        5,
					StartTime:          2,
					EndTime:            1,
					DrillDownValueType: metricpb.DrillDownValueTypeModelName,
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
			fieldVals := tt.fieldsGetter(ctrl)
			app := &MetricApplication{
				metricService: fieldVals.metricSvc,
				authSvc:       fieldVals.auth,
			}
			got, err := app.GetDrillDownValues(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			if tt.wantErr {
				assert.Nil(t, got)
			} else if tt.postCheck != nil {
				ttFields := fieldVals
				ttPost := tt.postCheck
				ttPost(t, ttFields, got)
			}
		})
	}
}

func TestMetricApplication_validateGetDrillDownValuesReq(t *testing.T) {
	t.Parallel()
	app := &MetricApplication{}
	ctx := context.Background()

	type testCase struct {
		name    string
		req     *metricapi.GetDrillDownValuesRequest
		wantErr bool
	}

	tests := []testCase{
		{
			name: "start greater than end",
			req: &metricapi.GetDrillDownValuesRequest{
				StartTime: 2,
				EndTime:   1,
			},
			wantErr: true,
		},
		{
			name: "valid request",
			req: &metricapi.GetDrillDownValuesRequest{
				StartTime: 1,
				EndTime:   2,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := app.validateGetDrillDownValuesReq(ctx, tt.req)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}

func ptr32(v int32) *int32    { return &v }
func ptr64(v int64) *int64    { return &v }
func ptrStr(v string) *string { return &v }

func TestMetricApplication_TraverseMetrics(t *testing.T) {
	t.Parallel()

	type args struct {
		ctx context.Context
		req *metricapi.TraverseMetricsRequest
	}

	tests := []struct {
		name      string
		setupMock func(ctrl *gomock.Controller) *metricservicemock.MockIMetricsService
		args      args
		wantErr   bool
		postCheck func(t *testing.T, got *metricapi.TraverseMetricsResponse)
	}{
		{
			name: "success",
			setupMock: func(ctrl *gomock.Controller) *metricservicemock.MockIMetricsService {
				metricMock := metricservicemock.NewMockIMetricsService(ctrl)
				metricMock.EXPECT().TraverseMetrics(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *service.TraverseMetricsReq) (*service.TraverseMetricsResp, error) {
					assert.Equal(t, []string{"metric_a", "metric_b"}, req.MetricsNames)
					assert.Equal(t, []loop_span.PlatformType{"bot", "agent"}, req.PlatformTypes)
					assert.Equal(t, int64(100), req.WorkspaceID)
					assert.Equal(t, 60*time.Second, req.QueryTimeout)
					return &service.TraverseMetricsResp{
						Statistic: service.TraverseMetricStatistic{
							Total:   10,
							Success: 8,
							Failure: 2,
						},
					}, nil
				}).Times(1)
				return metricMock
			},
			args: args{
				ctx: context.Background(),
				req: &metricapi.TraverseMetricsRequest{
					PlatformTypes: []commondto.PlatformType{"bot", "agent"},
					WorkspaceID:   ptr64(100),
					MetricNames:   []string{"metric_a", "metric_b"},
					StartDate:     ptrStr("2025-01-01"),
				},
			},
			wantErr: false,
			postCheck: func(t *testing.T, got *metricapi.TraverseMetricsResponse) {
				assert.NotNil(t, got)
				assert.NotNil(t, got.Statistic)
				assert.Equal(t, int32(10), got.Statistic.GetTotal())
				assert.Equal(t, int32(8), got.Statistic.GetSuccess())
				assert.Equal(t, int32(2), got.Statistic.GetFailure())
			},
		},
		{
			name: "default start_date when nil",
			setupMock: func(ctrl *gomock.Controller) *metricservicemock.MockIMetricsService {
				metricMock := metricservicemock.NewMockIMetricsService(ctrl)
				metricMock.EXPECT().TraverseMetrics(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *service.TraverseMetricsReq) (*service.TraverseMetricsResp, error) {
					// StartDate should be set to yesterday
					assert.NotEmpty(t, req.StartDate)
					parsed, err := time.Parse(time.DateOnly, req.StartDate)
					assert.NoError(t, err)
					// Should be approximately yesterday
					expected := time.Now().Add(-24 * time.Hour).Format(time.DateOnly)
					assert.Equal(t, expected, parsed.Format(time.DateOnly))
					return &service.TraverseMetricsResp{
						Statistic: service.TraverseMetricStatistic{
							Total: 1, Success: 1, Failure: 0,
						},
					}, nil
				}).Times(1)
				return metricMock
			},
			args: args{
				ctx: context.Background(),
				req: &metricapi.TraverseMetricsRequest{
					MetricNames: []string{"metric_a"},
					StartDate:   nil, // nil to trigger default
				},
			},
			wantErr: false,
			postCheck: func(t *testing.T, got *metricapi.TraverseMetricsResponse) {
				assert.NotNil(t, got)
				assert.Equal(t, int32(1), got.Statistic.GetTotal())
			},
		},
		{
			name: "with workspace_id",
			setupMock: func(ctrl *gomock.Controller) *metricservicemock.MockIMetricsService {
				metricMock := metricservicemock.NewMockIMetricsService(ctrl)
				metricMock.EXPECT().TraverseMetrics(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *service.TraverseMetricsReq) (*service.TraverseMetricsResp, error) {
					assert.Equal(t, int64(42), req.WorkspaceID)
					return &service.TraverseMetricsResp{
						Statistic: service.TraverseMetricStatistic{
							Total: 5, Success: 5, Failure: 0,
						},
					}, nil
				}).Times(1)
				return metricMock
			},
			args: args{
				ctx: context.Background(),
				req: &metricapi.TraverseMetricsRequest{
					MetricNames: []string{"metric_a"},
					WorkspaceID: ptr64(42),
					StartDate:   ptrStr("2025-06-01"),
				},
			},
			wantErr: false,
			postCheck: func(t *testing.T, got *metricapi.TraverseMetricsResponse) {
				assert.NotNil(t, got)
				assert.Equal(t, int32(5), got.Statistic.GetTotal())
			},
		},
		{
			name: "service error",
			setupMock: func(ctrl *gomock.Controller) *metricservicemock.MockIMetricsService {
				metricMock := metricservicemock.NewMockIMetricsService(ctrl)
				metricMock.EXPECT().TraverseMetrics(gomock.Any(), gomock.Any()).Return(nil, assert.AnError).Times(1)
				return metricMock
			},
			args: args{
				ctx: context.Background(),
				req: &metricapi.TraverseMetricsRequest{
					MetricNames: []string{"metric_a"},
					StartDate:   ptrStr("2025-06-01"),
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
			metricMock := tt.setupMock(ctrl)
			app := &MetricApplication{
				metricService: metricMock,
			}
			got, err := app.TraverseMetrics(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			if tt.wantErr {
				assert.Nil(t, got)
			} else if tt.postCheck != nil {
				tt.postCheck(t, got)
			}
		})
	}
}

func TestMetricApplication_GetDrillDownValues_FeedbackTypes(t *testing.T) {
	t.Parallel()

	type fields struct {
		metricSvc service.IMetricsService
		auth      rpc.IAuthProvider
		captured  *safeMetricsRequests
	}

	type args struct {
		ctx context.Context
		req *metricapi.GetDrillDownValuesRequest
	}

	tests := []struct {
		name         string
		fieldsGetter func(ctrl *gomock.Controller) fields
		args         args
		wantErr      bool
		postCheck    func(t *testing.T, f fields, got *metricapi.GetDrillDownValuesResponse)
	}{
		{
			name: "success annotation key",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				metricMock := metricservicemock.NewMockIMetricsService(ctrl)
				authMock := rpcmock.NewMockIAuthProvider(ctrl)
				captured := &safeMetricsRequests{}
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceMetricRead, "7", false).Return(nil)
				metricMock.EXPECT().QueryMetrics(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *service.QueryMetricsReq) (*service.QueryMetricsResp, error) {
					captured.add(req)
					return &service.QueryMetricsResp{
						Metrics: map[string]*entity.Metric{
							entity.MetricNameFeedbackCountByKeyPie: {
								Pie: map[string]string{
									`{"name":"keyA"}`: "3",
									`{"name":"keyB"}`: "7",
								},
							},
						},
					}, nil
				}).Times(1)
				metricMock.EXPECT().GetMetricGroupBy(entity.MetricNameFeedbackCountByKeyPie).Return([]string{"name"}, nil).Times(1)
				return fields{
					metricSvc: metricMock,
					auth:      authMock,
					captured:  captured,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &metricapi.GetDrillDownValuesRequest{
					WorkspaceID:        7,
					StartTime:          0,
					EndTime:            10 * 24 * time.Hour.Milliseconds(),
					DrillDownValueType: metricpb.DrillDownValueTypeAnnotationKey,
				},
			},
			wantErr: false,
			postCheck: func(t *testing.T, f fields, got *metricapi.GetDrillDownValuesResponse) {
				assert.NotNil(t, got)
				if f.captured != nil {
					captured := f.captured.snapshot()
					if assert.Len(t, captured, 1) {
						assert.Equal(t, []string{entity.MetricNameFeedbackCountByKeyPie}, captured[0].MetricsNames)
					}
				}
				assert.Len(t, got.DrillDownValues, 2)
				assert.Equal(t, "keyB", got.DrillDownValues[0].Value)
				assert.Equal(t, "keyA", got.DrillDownValues[1].Value)
			},
		},
		{
			name: "success feedback source",
			fieldsGetter: func(ctrl *gomock.Controller) fields {
				metricMock := metricservicemock.NewMockIMetricsService(ctrl)
				authMock := rpcmock.NewMockIAuthProvider(ctrl)
				captured := &safeMetricsRequests{}
				authMock.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceMetricRead, "8", false).Return(nil)
				metricMock.EXPECT().QueryMetrics(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *service.QueryMetricsReq) (*service.QueryMetricsResp, error) {
					captured.add(req)
					return &service.QueryMetricsResp{
						Metrics: map[string]*entity.Metric{
							entity.MetricNameFeedbackCountBySourcePie: {
								Pie: map[string]string{
									`{"name":"sourceA"}`: "5",
									`{"name":"sourceB"}`: "15",
								},
							},
						},
					}, nil
				}).Times(1)
				metricMock.EXPECT().GetMetricGroupBy(entity.MetricNameFeedbackCountBySourcePie).Return([]string{"name"}, nil).Times(1)
				return fields{
					metricSvc: metricMock,
					auth:      authMock,
					captured:  captured,
				}
			},
			args: args{
				ctx: context.Background(),
				req: &metricapi.GetDrillDownValuesRequest{
					WorkspaceID:        8,
					StartTime:          0,
					EndTime:            10 * 24 * time.Hour.Milliseconds(),
					DrillDownValueType: metricpb.DrillDownValueTypeFeedbackSource,
				},
			},
			wantErr: false,
			postCheck: func(t *testing.T, f fields, got *metricapi.GetDrillDownValuesResponse) {
				assert.NotNil(t, got)
				if f.captured != nil {
					captured := f.captured.snapshot()
					if assert.Len(t, captured, 1) {
						assert.Equal(t, []string{entity.MetricNameFeedbackCountBySourcePie}, captured[0].MetricsNames)
					}
				}
				assert.Len(t, got.DrillDownValues, 2)
				assert.Equal(t, "sourceB", got.DrillDownValues[0].Value)
				assert.Equal(t, "sourceA", got.DrillDownValues[1].Value)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			fieldVals := tt.fieldsGetter(ctrl)
			app := &MetricApplication{
				metricService: fieldVals.metricSvc,
				authSvc:       fieldVals.auth,
			}
			got, err := app.GetDrillDownValues(tt.args.ctx, tt.args.req)
			assert.Equal(t, tt.wantErr, err != nil)
			if tt.wantErr {
				assert.Nil(t, got)
			} else if tt.postCheck != nil {
				ttFields := fieldVals
				ttPost := tt.postCheck
				ttPost(t, ttFields, got)
			}
		})
	}
}
