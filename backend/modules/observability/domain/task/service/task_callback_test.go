// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/external/benefit"
	benefit_mocks "github.com/coze-dev/coze-loop/backend/infra/external/benefit/mocks"
	tenant_mocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/component/tenant/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	repo_mocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/repo/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	trace_repo_mocks "github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo/mocks"
	"github.com/stretchr/testify/require"
)

func TestTaskCallbackServiceImpl_CallBackSuccess(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockBenefit := benefit_mocks.NewMockIBenefitService(ctrl)
	mockTenant := tenant_mocks.NewMockITenantProvider(ctrl)
	mockTraceRepo := trace_repo_mocks.NewMockITraceRepo(ctrl)
	mockTaskRepo := repo_mocks.NewMockITaskRepo(ctrl)

	impl := &TaskCallbackServiceImpl{
		benefitSvc:     mockBenefit,
		tenantProvider: mockTenant,
		traceRepo:      mockTraceRepo,
		taskRepo:       mockTaskRepo,
	}

	mockTenant.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"tenant"}, nil).AnyTimes()
	mockBenefit.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{StorageDuration: 1}, nil).AnyTimes()

	now := time.Now()
	span := &loop_span.Span{
		SpanID:           "span-1",
		TraceID:          "trace-1",
		SystemTagsString: map[string]string{loop_span.SpanFieldTenant: "tenant"},
		LogicDeleteTime:  now.Add(24 * time.Hour).UnixMicro(),
		StartTime:        now.UnixMicro(),
	}

	mockTraceRepo.EXPECT().ListSpans(gomock.Any(), gomock.AssignableToTypeOf(&repo.ListSpansParam{})).Return(&repo.ListSpansResult{Spans: loop_span.SpanList{span}}, nil)
	mockTaskRepo.EXPECT().IncrTaskRunSuccessCount(gomock.Any(), int64(101), int64(202), gomock.Any()).Return(nil)
	mockTraceRepo.EXPECT().InsertAnnotations(gomock.Any(), gomock.AssignableToTypeOf(&repo.InsertAnnotationParam{})).DoAndReturn(
		func(_ context.Context, param *repo.InsertAnnotationParam) error {
			require.Equal(t, loop_span.AnnotationTypeAutoEvaluate, param.AnnotationType)
			return nil
		},
	)

	startTime := now.Add(-time.Minute).UnixMilli()
	event := &entity.AutoEvalEvent{
		TurnEvalResults: []*entity.OnlineExptTurnEvalResult{
			{
				EvaluatorVersionID: 1,
				Score:              0.9,
				Reasoning:          "ok",
				Status:             entity.EvaluatorRunStatus_Success,
				BaseInfo: &entity.BaseInfo{
					CreatedBy: &entity.UserInfo{UserID: "user-1"},
				},
				Ext: map[string]string{
					"workspace_id": strconv.FormatInt(1, 10),
					"span_id":      "span-1",
					"trace_id":     "trace-1",
					"start_time":   strconv.FormatInt(startTime*1000, 10),
					"task_id":      strconv.FormatInt(101, 10),
					"run_id":       strconv.FormatInt(202, 10),
				},
			},
		},
	}

	require.NoError(t, impl.AutoEvalCallback(context.Background(), event))
}

func TestTraceHubServiceImpl_CallBackSpanNotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockBenefit := benefit_mocks.NewMockIBenefitService(ctrl)
	mockTenant := tenant_mocks.NewMockITenantProvider(ctrl)
	mockTraceRepo := trace_repo_mocks.NewMockITraceRepo(ctrl)

	impl := &TaskCallbackServiceImpl{
		benefitSvc:     mockBenefit,
		tenantProvider: mockTenant,
		traceRepo:      mockTraceRepo,
	}

	mockTenant.EXPECT().GetTenantsByPlatformType(gomock.Any(), gomock.Any()).Return([]string{"tenant"}, nil).AnyTimes()
	mockBenefit.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{StorageDuration: 1}, nil).AnyTimes()
	mockTraceRepo.EXPECT().ListSpans(gomock.Any(), gomock.AssignableToTypeOf(&repo.ListSpansParam{})).Return(&repo.ListSpansResult{}, nil)

	event := &entity.AutoEvalEvent{
		TurnEvalResults: []*entity.OnlineExptTurnEvalResult{
			{
				Status: entity.EvaluatorRunStatus_Success,
				BaseInfo: &entity.BaseInfo{
					CreatedBy: &entity.UserInfo{UserID: "user-1"},
				},
				Ext: map[string]string{
					"workspace_id": "1",
					"span_id":      "span-1",
					"trace_id":     "trace-1",
					"start_time":   strconv.FormatInt(time.Now().UnixMilli()*1000, 10),
					"task_id":      "101",
					"run_id":       "202",
				},
			},
		},
	}

	require.Error(t, impl.AutoEvalCallback(context.Background(), event))
}

func TestTaskCallbackServiceImpl_getSpan(t *testing.T) {
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
		impl := &TaskCallbackServiceImpl{traceRepo: mockTraceRepo}
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
		impl := &TaskCallbackServiceImpl{traceRepo: mockTraceRepo}
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
		impl := &TaskCallbackServiceImpl{}
		_, err := impl.getSpan(ctx, tenants, nil, traceID, workspaceID, start, end)
		require.Error(t, err)
	})

	t.Run("empty_workspace", func(t *testing.T) {
		t.Parallel()
		impl := &TaskCallbackServiceImpl{}
		_, err := impl.getSpan(ctx, tenants, spanIDs, traceID, "", start, end)
		require.Error(t, err)
	})

	t.Run("repo_error", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)
		mockTraceRepo := trace_repo_mocks.NewMockITraceRepo(ctrl)
		impl := &TaskCallbackServiceImpl{traceRepo: mockTraceRepo}

		mockTraceRepo.EXPECT().ListSpans(gomock.Any(), gomock.AssignableToTypeOf(&repo.ListSpansParam{})).Return(nil, errors.New("list error"))

		_, err := impl.getSpan(ctx, tenants, spanIDs, traceID, workspaceID, start, end)
		require.Error(t, err)
	})

	t.Run("no_data", func(t *testing.T) {
		t.Parallel()
		ctrl := gomock.NewController(t)
		t.Cleanup(ctrl.Finish)
		mockTraceRepo := trace_repo_mocks.NewMockITraceRepo(ctrl)
		impl := &TaskCallbackServiceImpl{traceRepo: mockTraceRepo}

		mockTraceRepo.EXPECT().ListSpans(gomock.Any(), gomock.AssignableToTypeOf(&repo.ListSpansParam{})).Return(&repo.ListSpansResult{}, nil)

		spans, err := impl.getSpan(ctx, tenants, spanIDs, traceID, workspaceID, start, end)
		require.NoError(t, err)
		require.Nil(t, spans)
	})
}

func TestTaskCallbackServiceImpl_updateTaskRunDetailsCount(t *testing.T) {
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
			impl := &TaskCallbackServiceImpl{taskRepo: mockRepo}

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
		impl := &TaskCallbackServiceImpl{}
		err := impl.updateTaskRunDetailsCount(ctx, taskID, &entity.OnlineExptTurnEvalResult{Ext: map[string]string{}}, 0)
		require.Error(t, err)
	})

	t.Run("invalid_run_id", func(t *testing.T) {
		t.Parallel()
		impl := &TaskCallbackServiceImpl{}
		err := impl.updateTaskRunDetailsCount(ctx, taskID, &entity.OnlineExptTurnEvalResult{Ext: map[string]string{"run_id": "abc"}}, 0)
		require.Error(t, err)
	})
}
