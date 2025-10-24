// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/task"
	entity "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	repo_mocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	trace_repo_mocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo/mocks"
	"github.com/stretchr/testify/require"
)

func TestTraceHubServiceImpl_getObjListWithTaskFromCache_Fallback(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	impl := &TraceHubServiceImpl{}

	gotSpaces, gotBots, gotTasks := impl.getObjListWithTaskFromCache(ctx)
	require.Nil(t, gotSpaces)
	require.Nil(t, gotBots)
	require.Nil(t, gotTasks)
}

func TestTraceHubServiceImpl_getObjListWithTaskFromCache_FromCache(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
	impl := &TraceHubServiceImpl{taskRepo: mockRepo}

	expected := TaskCacheInfo{
		WorkspaceIDs: []string{"space-2"},
		BotIDs:       []string{"bot-2"},
		Tasks:        []*entity.ObservabilityTask{{}},
	}
	impl.taskCache.Store("ObjListWithTask", expected)

	gotSpaces, gotBots, gotTasks := impl.getObjListWithTaskFromCache(context.Background())
	require.Equal(t, expected.WorkspaceIDs, gotSpaces)
	require.Equal(t, expected.BotIDs, gotBots)
	require.Equal(t, expected.Tasks, gotTasks)
}

func TestTraceHubServiceImpl_getObjListWithTaskFromCache_TypeMismatch(t *testing.T) {
	t.Parallel()

	impl := &TraceHubServiceImpl{}

	impl.taskCache.Store("ObjListWithTask", "invalid")

	gotSpaces, gotBots, gotTasks := impl.getObjListWithTaskFromCache(context.Background())
	require.Nil(t, gotSpaces)
	require.Nil(t, gotBots)
	require.Nil(t, gotTasks)
}

func TestTraceHubServiceImpl_applySampling(t *testing.T) {
	t.Parallel()

	spans := []*loop_span.Span{{SpanID: "1"}, {SpanID: "2"}, {SpanID: "3"}}
	impl := &TraceHubServiceImpl{}

	fullRate := &spanSubscriber{
		t: &task.Task{
			Rule: &task.Rule{Sampler: &task.Sampler{SampleRate: floatPtr(1.0)}},
		},
	}
	zeroRate := &spanSubscriber{
		t: &task.Task{
			Rule: &task.Rule{Sampler: &task.Sampler{SampleRate: floatPtr(0.0)}},
		},
	}
	halfRate := &spanSubscriber{
		t: &task.Task{
			Rule: &task.Rule{Sampler: &task.Sampler{SampleRate: floatPtr(0.5)}},
		},
	}

	require.Len(t, impl.applySampling(spans, fullRate), len(spans))
	require.Nil(t, impl.applySampling(spans, zeroRate))
	require.Len(t, impl.applySampling(spans, halfRate), 1)
}

func TestTraceHubServiceImpl_updateTaskRunDetailsCount(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	taskID := int64(101)
	runIDStr := "202"
	runID := int64(202)

	tests := []struct {
		name          string
		status        entity.EvaluatorRunStatus
		expectSuccess bool
		expectFail    bool
		expectErr     bool
	}{
		{
			name:          "success_status",
			status:        entity.EvaluatorRunStatus_Success,
			expectSuccess: true,
		},
		{
			name:       "fail_status",
			status:     entity.EvaluatorRunStatus_Fail,
			expectFail: true,
		},
		{
			name:   "unknown_status",
			status: entity.EvaluatorRunStatus_Unknown,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			t.Cleanup(ctrl.Finish)

			mockRepo := repo_mocks.NewMockITaskRepo(ctrl)
			impl := &TraceHubServiceImpl{taskRepo: mockRepo}

			turn := &entity.OnlineExptTurnEvalResult{
				Status: tt.status,
				Ext: map[string]string{
					"run_id": runIDStr,
				},
			}

			if tt.expectSuccess {
				mockRepo.EXPECT().IncrTaskRunSuccessCount(ctx, taskID, runID, gomock.Any()).Return(nil)
			}
			if tt.expectFail {
				mockRepo.EXPECT().IncrTaskRunFailCount(ctx, taskID, runID, gomock.Any()).Return(nil)
			}

			err := impl.updateTaskRunDetailsCount(ctx, taskID, turn, 0)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}

	t.Run("missing_run_id", func(t *testing.T) {
		t.Parallel()
		impl := &TraceHubServiceImpl{}
		err := impl.updateTaskRunDetailsCount(ctx, taskID, &entity.OnlineExptTurnEvalResult{Ext: map[string]string{}}, 0)
		require.Error(t, err)
	})

	t.Run("invalid_run_id", func(t *testing.T) {
		t.Parallel()
		impl := &TraceHubServiceImpl{}
		err := impl.updateTaskRunDetailsCount(ctx, taskID, &entity.OnlineExptTurnEvalResult{Ext: map[string]string{"run_id": "abc"}}, 0)
		require.Error(t, err)
	})
}

func TestTraceHubServiceImpl_sendBackfillMessage(t *testing.T) {
	t.Parallel()

	impl := &TraceHubServiceImpl{}
	err := impl.sendBackfillMessage(context.Background(), &entity.BackFillEvent{})
	require.Error(t, err)

	fake := &fakeBackfillProducer{}
	impl.backfillProducer = fake

	evt := &entity.BackFillEvent{TaskID: 1, SpaceID: 2}
	require.NoError(t, impl.sendBackfillMessage(context.Background(), evt))
	require.Equal(t, evt, fake.event)
}

func TestTraceHubServiceImpl_getSpan(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tenants := []string{"tenant"}
	spanIDs := []string{"span-1"}
	traceID := "trace-1"
	workspaceID := "ws-1"
	start := int64(1000)
	end := int64(2000)

	t.Run("with_trace_id", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)
		mockTraceRepo := trace_repo_mocks.NewMockITraceRepo(ctrl)
		impl := &TraceHubServiceImpl{traceRepo: mockTraceRepo}
		expectedSpan := &loop_span.Span{SpanID: spanIDs[0], TraceID: traceID}

		mockTraceRepo.EXPECT().ListSpans(gomock.Any(), gomock.AssignableToTypeOf(&repo.ListSpansParam{})).DoAndReturn(
			func(_ context.Context, param *repo.ListSpansParam) (*repo.ListSpansResult, error) {
				require.Equal(t, tenants, param.Tenants)
				require.Equal(t, start, param.StartAt)
				require.Equal(t, end, param.EndAt)
				require.True(t, param.NotQueryAnnotation)
				require.Equal(t, int32(2), param.Limit)
				require.Len(t, param.Filters.FilterFields, 3)
				return &repo.ListSpansResult{Spans: loop_span.SpanList{expectedSpan}}, nil
			},
		)

		spans, err := impl.getSpan(ctx, tenants, spanIDs, traceID, workspaceID, start, end)
		require.NoError(t, err)
		require.Equal(t, []*loop_span.Span{expectedSpan}, spans)
	})

	t.Run("without_trace_id", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)
		mockTraceRepo := trace_repo_mocks.NewMockITraceRepo(ctrl)
		impl := &TraceHubServiceImpl{traceRepo: mockTraceRepo}
		expectedSpan := &loop_span.Span{SpanID: spanIDs[0]}

		mockTraceRepo.EXPECT().ListSpans(gomock.Any(), gomock.AssignableToTypeOf(&repo.ListSpansParam{})).DoAndReturn(
			func(_ context.Context, param *repo.ListSpansParam) (*repo.ListSpansResult, error) {
				require.Equal(t, tenants, param.Tenants)
				require.Len(t, param.Filters.FilterFields, 2)
				return &repo.ListSpansResult{Spans: loop_span.SpanList{expectedSpan}}, nil
			},
		)

		spans, err := impl.getSpan(ctx, tenants, spanIDs, "", workspaceID, start, end)
		require.NoError(t, err)
		require.Equal(t, []*loop_span.Span{expectedSpan}, spans)
	})

	t.Run("empty_span_ids", func(t *testing.T) {
		t.Parallel()
		impl := &TraceHubServiceImpl{}
		_, err := impl.getSpan(ctx, tenants, nil, traceID, workspaceID, start, end)
		require.Error(t, err)
	})

	t.Run("empty_workspace", func(t *testing.T) {
		t.Parallel()
		impl := &TraceHubServiceImpl{}
		_, err := impl.getSpan(ctx, tenants, spanIDs, traceID, "", start, end)
		require.Error(t, err)
	})

	t.Run("repo_error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)
		mockTraceRepo := trace_repo_mocks.NewMockITraceRepo(ctrl)
		impl := &TraceHubServiceImpl{traceRepo: mockTraceRepo}

		mockTraceRepo.EXPECT().ListSpans(gomock.Any(), gomock.AssignableToTypeOf(&repo.ListSpansParam{})).Return(nil, errors.New("list error"))

		_, err := impl.getSpan(ctx, tenants, spanIDs, traceID, workspaceID, start, end)
		require.Error(t, err)
	})

	t.Run("no_data", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)
		mockTraceRepo := trace_repo_mocks.NewMockITraceRepo(ctrl)
		impl := &TraceHubServiceImpl{traceRepo: mockTraceRepo}

		mockTraceRepo.EXPECT().ListSpans(gomock.Any(), gomock.AssignableToTypeOf(&repo.ListSpansParam{})).Return(&repo.ListSpansResult{}, nil)

		spans, err := impl.getSpan(ctx, tenants, spanIDs, traceID, workspaceID, start, end)
		require.NoError(t, err)
		require.Nil(t, spans)
	})
}

type fakeBackfillProducer struct {
	event *entity.BackFillEvent
}

func (f *fakeBackfillProducer) SendBackfill(_ context.Context, event *entity.BackFillEvent) error {
	f.event = event
	return nil
}
