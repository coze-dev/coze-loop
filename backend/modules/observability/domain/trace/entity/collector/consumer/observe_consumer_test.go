package consumer

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"

	metricsmocks "github.com/coze-dev/coze-loop/backend/infra/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

func TestObserveConsumer_ConsumeTraces_Success(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetric := metricsmocks.NewMockMetric(ctrl)

	inner := &mockConsumer{}
	timed := NewObserveConsumer("test_node", inner, nil, mockMetric)

	err := timed.ConsumeTraces(context.Background(), Traces{})
	assert.NoError(t, err)
}

func TestObserveConsumer_ConsumeTraces_Error(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetric := metricsmocks.NewMockMetric(ctrl)

	expectedErr := errors.New("consume failed")
	inner := &errConsumer{err: expectedErr}
	timed := NewObserveConsumer("test_node", inner, nil, mockMetric)

	err := timed.ConsumeTraces(context.Background(), Traces{})
	assert.ErrorIs(t, err, expectedErr)
}

func TestObserveConsumer_ConsumeTraces_NilMetric(t *testing.T) {
	inner := &mockConsumer{}
	timed := NewObserveConsumer("test_node", inner, nil, nil)

	err := timed.ConsumeTraces(context.Background(), Traces{})
	assert.NoError(t, err)
}

func TestObserveConsumer_SubtractsNextElapsed(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetric := metricsmocks.NewMockMetric(ctrl)

	nextElapsed := &atomic.Int64{}
	sleepDuration := 50 * time.Millisecond

	inner := &sleepConsumer{
		duration: sleepDuration,
		afterSleep: func() {
			nextElapsed.Store((100 * time.Millisecond).Nanoseconds())
		},
	}
	timed := NewObserveConsumer("test_node", inner, nextElapsed, mockMetric)

	err := timed.ConsumeTraces(context.Background(), Traces{})
	assert.NoError(t, err)
}

func TestObserveConsumer_GroupByPSM(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockMetric := metricsmocks.NewMockMetric(ctrl)
	mockMetric.EXPECT().Emit(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Times(2)

	inner := &mockConsumer{}
	timed := NewObserveConsumer("test_node", inner, nil, mockMetric)

	traces := Traces{
		Tenant: "test_tenant",
		TraceData: []*entity.TraceData{
			{
				SpanList: loop_span.SpanList{
					{PSM: "svc-a"},
					{PSM: "svc-a"},
					{PSM: "svc-b"},
				},
			},
		},
	}
	err := timed.ConsumeTraces(context.Background(), traces)
	assert.NoError(t, err)
}

func TestStopwatchConsumer_RecordsElapsed(t *testing.T) {
	elapsed := &atomic.Int64{}
	inner := &sleepConsumer{duration: 10 * time.Millisecond}
	sw := NewStopwatchConsumer(inner, elapsed)

	err := sw.ConsumeTraces(context.Background(), Traces{})
	assert.NoError(t, err)
	assert.Greater(t, elapsed.Load(), int64(0))
}

func TestStopwatchConsumer_AccumulatesElapsed(t *testing.T) {
	elapsed := &atomic.Int64{}
	inner := &sleepConsumer{duration: 5 * time.Millisecond}
	sw := NewStopwatchConsumer(inner, elapsed)

	_ = sw.ConsumeTraces(context.Background(), Traces{})
	first := elapsed.Load()
	_ = sw.ConsumeTraces(context.Background(), Traces{})
	second := elapsed.Load()
	assert.Greater(t, second, first)
}

type errConsumer struct {
	err error
}

func (e *errConsumer) ConsumeTraces(ctx context.Context, tds Traces) error {
	return e.err
}

type sleepConsumer struct {
	duration   time.Duration
	afterSleep func()
}

func (s *sleepConsumer) ConsumeTraces(ctx context.Context, tds Traces) error {
	time.Sleep(s.duration)
	if s.afterSleep != nil {
		s.afterSleep()
	}
	return nil
}
