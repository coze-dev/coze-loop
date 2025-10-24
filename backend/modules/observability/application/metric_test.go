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
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

type fakeMetricsService struct {
	mu        sync.Mutex
	requests  []*service.QueryMetricsReq
	responses []*service.QueryMetricsResp
	errs      []error
	handler   func(req *service.QueryMetricsReq) (*service.QueryMetricsResp, error)
}

func (f *fakeMetricsService) QueryMetrics(ctx context.Context, req *service.QueryMetricsReq) (*service.QueryMetricsResp, error) {
	f.mu.Lock()
	idx := len(f.requests)
	f.requests = append(f.requests, req)
	handler := f.handler
	var resp *service.QueryMetricsResp
	if idx < len(f.responses) {
		resp = f.responses[idx]
	}
	var err error
	if idx < len(f.errs) {
		err = f.errs[idx]
	}
	f.mu.Unlock()
	if handler != nil {
		return handler(req)
	}
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return &service.QueryMetricsResp{Metrics: map[string]*entity.Metric{}}, nil
	}
	return resp, nil
}

func (f *fakeMetricsService) Calls() []*service.QueryMetricsReq {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*service.QueryMetricsReq, len(f.requests))
	copy(out, f.requests)
	return out
}

func TestMetricApplication_GetMetrics(t *testing.T) {
	t.Parallel()

	t.Run("success without compare", func(t *testing.T) {
		t.Parallel()
		svc := &fakeMetricsService{
			responses: []*service.QueryMetricsResp{
				{
					Metrics: map[string]*entity.Metric{
						"metric_a": {
							Summary: "10",
							Pie: map[string]string{
								"foo": "1",
							},
							TimeSeries: entity.TimeSeries{
								"all": {
									{Timestamp: "1", Value: "2"},
								},
							},
						},
					},
				},
			},
		}
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		auth := rpcmock.NewMockIAuthProvider(ctrl)
		auth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceMetricRead, "1", false).Return(nil)
		app := &MetricApplication{metricService: svc, authSvc: auth}

		req := &metricapi.GetMetricsRequest{
			WorkspaceID: 1,
			StartTime:   1000,
			EndTime:     2000,
			MetricNames: []string{"metric_a"},
		}
		resp, err := app.GetMetrics(context.Background(), req)
		assert.NoError(t, err)
		assert.Equal(t, "10", resp.Metrics["metric_a"].GetSummary())
		assert.Equal(t, "1", resp.Metrics["metric_a"].GetPie()["foo"])
		assert.Len(t, resp.Metrics["metric_a"].GetTimeSeries()["all"], 1)

		calls := svc.Calls()
		if assert.Len(t, calls, 1) {
			assert.Equal(t, req.GetStartTime(), calls[0].StartTime)
			assert.Equal(t, req.GetEndTime(), calls[0].EndTime)
			assert.Equal(t, entity.MetricGranularity1Day, calls[0].Granularity)
		}
	})

	t.Run("success with compare", func(t *testing.T) {
		t.Parallel()
		svc := &fakeMetricsService{
			handler: func(req *service.QueryMetricsReq) (*service.QueryMetricsResp, error) {
				summary := "2"
				if req.StartTime == 2000 && req.EndTime == 4000 {
					summary = "1"
				}
				return &service.QueryMetricsResp{
					Metrics: map[string]*entity.Metric{
						"metric_a": {Summary: summary},
					},
				}, nil
			},
		}
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		auth := rpcmock.NewMockIAuthProvider(ctrl)
		auth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceMetricRead, "2", false).Return(nil)
		app := &MetricApplication{metricService: svc, authSvc: auth}

		compareType := metricpb.CompareTypeMoM
		req := &metricapi.GetMetricsRequest{
			WorkspaceID: 2,
			StartTime:   2000,
			EndTime:     4000,
			MetricNames: []string{"metric_a"},
			Compare: &metricpb.Compare{
				CompareType: &compareType,
			},
		}
		resp, err := app.GetMetrics(context.Background(), req)
		assert.NoError(t, err)
		assert.Equal(t, "1", resp.Metrics["metric_a"].GetSummary())
		assert.Equal(t, "2", resp.ComparedMetrics["metric_a"].GetSummary())

		calls := svc.Calls()
		if assert.Len(t, calls, 2) {
			startEnds := map[string]bool{}
			for _, call := range calls {
				key := strconv.FormatInt(call.StartTime, 10) + ":" + strconv.FormatInt(call.EndTime, 10)
				startEnds[key] = true
			}
			assert.True(t, startEnds["2000:4000"])
			assert.True(t, startEnds["0:2000"])
		}
	})

	t.Run("validate error", func(t *testing.T) {
		t.Parallel()
		app := &MetricApplication{}
		req := &metricapi.GetMetricsRequest{
			WorkspaceID: 1,
			StartTime:   2000,
			EndTime:     1000,
			MetricNames: []string{"metric_a"},
		}
		resp, err := app.GetMetrics(context.Background(), req)
		assert.Nil(t, resp)
		assert.Error(t, err)
	})

	t.Run("auth error", func(t *testing.T) {
		t.Parallel()
		svc := &fakeMetricsService{}
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		auth := rpcmock.NewMockIAuthProvider(ctrl)
		auth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceMetricRead, "3", false).Return(assert.AnError)
		app := &MetricApplication{metricService: svc, authSvc: auth}

		req := &metricapi.GetMetricsRequest{
			WorkspaceID: 3,
			StartTime:   1000,
			EndTime:     2000,
			MetricNames: []string{"metric_a"},
		}
		resp, err := app.GetMetrics(context.Background(), req)
		assert.Nil(t, resp)
		assert.Error(t, err)
		assert.Empty(t, svc.Calls())
	})
}

func TestMetricApplication_buildGetMetricsReq(t *testing.T) {
	app := &MetricApplication{}
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

	sReq := app.buildGetMetricsReq(req)
	assert.Equal(t, loop_span.PlatformType("bot"), sReq.PlatformType)
	assert.Equal(t, int64(10), sReq.WorkspaceID)
	assert.Equal(t, []string{"a", "b"}, sReq.MetricsNames)
	assert.Equal(t, entity.MetricGranularity1Hour, sReq.Granularity)
	req.Granularity = nil
	sReq = app.buildGetMetricsReq(req)
	assert.Equal(t, entity.MetricGranularity1Day, sReq.Granularity)
}

func TestMetricApplication_shouldCompareWith(t *testing.T) {
	app := &MetricApplication{}
	start, end := int64(1000), int64(2000)
	newStart, newEnd, ok := app.shouldCompareWith(start, end, nil)
	assert.Zero(t, newStart)
	assert.Zero(t, newEnd)
	assert.False(t, ok)

	cmp := &entity.Compare{Type: entity.MetricCompareTypeMoM}
	newStart, newEnd, ok = app.shouldCompareWith(start, end, cmp)
	assert.True(t, ok)
	assert.Equal(t, start-(end-start), newStart)
	assert.Equal(t, start, newEnd)

	cmp = &entity.Compare{Type: entity.MetricCompareTypeYoY, Shift: 10}
	newStart, newEnd, ok = app.shouldCompareWith(start, end, cmp)
	assert.True(t, ok)
	assert.Equal(t, start-10*1000, newStart)
	assert.Equal(t, end-10*1000, newEnd)

	cmp = &entity.Compare{Type: "unknown"}
	newStart, newEnd, ok = app.shouldCompareWith(start, end, cmp)
	assert.False(t, ok)
	assert.Zero(t, newStart)
	assert.Zero(t, newEnd)
}

func TestMetricApplication_validateGetMetricsReq(t *testing.T) {
	app := &MetricApplication{}
	ctx := context.Background()

	err := app.validateGetMetricsReq(ctx, &metricapi.GetMetricsRequest{
		StartTime:   2000,
		EndTime:     1000,
		MetricNames: []string{"metric_a"},
	})
	assert.Error(t, err)

	gran := string(entity.MetricGranularity1Min)
	err = app.validateGetMetricsReq(ctx, &metricapi.GetMetricsRequest{
		StartTime:   0,
		EndTime:     4 * time.Hour.Milliseconds(),
		MetricNames: []string{"metric_a"},
		Granularity: &gran,
	})
	assert.Error(t, err)

	gran = string(entity.MetricGranularity1Hour)
	err = app.validateGetMetricsReq(ctx, &metricapi.GetMetricsRequest{
		StartTime:   0,
		EndTime:     7 * 24 * time.Hour.Milliseconds(),
		MetricNames: []string{"metric_a"},
		Granularity: &gran,
	})
	assert.Error(t, err)

	err = app.validateGetMetricsReq(ctx, &metricapi.GetMetricsRequest{
		StartTime:   0,
		EndTime:     time.Hour.Milliseconds(),
		MetricNames: []string{"metric_a"},
	})
	assert.NoError(t, err)
}

func TestMetricApplication_GetDrillDownValues(t *testing.T) {
	t.Parallel()
	svc := &fakeMetricsService{
		responses: []*service.QueryMetricsResp{
			{
				Metrics: map[string]*entity.Metric{
					entity.MetricNameModelNamePie: {
						Pie: map[string]string{
							`{"name":"modelA"}`: "1",
							`{"name":"modelB"}`: "2",
						},
					},
				},
			},
		},
	}
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	auth := rpcmock.NewMockIAuthProvider(ctrl)
	auth.EXPECT().CheckWorkspacePermission(gomock.Any(), rpc.AuthActionTraceMetricRead, "5", false).Return(nil).Times(2)

	app := &MetricApplication{metricService: svc, authSvc: auth}
	req := &metricapi.GetDrillDownValuesRequest{
		WorkspaceID:        5,
		StartTime:          0,
		EndTime:            10 * 24 * time.Hour.Milliseconds(),
		DrillDownValueType: metricpb.DrillDownValueTypeModelName,
	}
	resp, err := app.GetDrillDownValues(context.Background(), req)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"modelA", "modelB"}, resp.Values)

	calls := svc.Calls()
	if assert.Len(t, calls, 1) {
		expectedStart := req.EndTime - 7*24*time.Hour.Milliseconds()
		assert.Equal(t, expectedStart, calls[0].StartTime)
		assert.Equal(t, req.EndTime, calls[0].EndTime)
		assert.Equal(t, []string{entity.MetricNameModelNamePie}, calls[0].MetricsNames)
	}

	// invalid type
	reqInvalid := &metricapi.GetDrillDownValuesRequest{
		WorkspaceID:        5,
		StartTime:          0,
		EndTime:            1,
		DrillDownValueType: "unknown",
	}
	r, err := app.GetDrillDownValues(context.Background(), reqInvalid)
	assert.Nil(t, r)
	assert.Error(t, err)

	// validation error
	reqInvalidTime := &metricapi.GetDrillDownValuesRequest{
		WorkspaceID:        5,
		StartTime:          2,
		EndTime:            1,
		DrillDownValueType: metricpb.DrillDownValueTypeModelName,
	}
	r, err = app.GetDrillDownValues(context.Background(), reqInvalidTime)
	assert.Nil(t, r)
	assert.Error(t, err)
}

func TestMetricApplication_validateGetDrillDownValuesReq(t *testing.T) {
	app := &MetricApplication{}
	err := app.validateGetDrillDownValuesReq(context.Background(), &metricapi.GetDrillDownValuesRequest{StartTime: 2, EndTime: 1})
	assert.Error(t, err)
	err = app.validateGetDrillDownValuesReq(context.Background(), &metricapi.GetDrillDownValuesRequest{StartTime: 1, EndTime: 2})
	assert.NoError(t, err)
}
