// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

import (
	"context"
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

func TestTraceHubServiceImpl_CallBackSuccess(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockBenefit := benefit_mocks.NewMockIBenefitService(ctrl)
	mockTenant := tenant_mocks.NewMockITenantProvider(ctrl)
	mockTraceRepo := trace_repo_mocks.NewMockITraceRepo(ctrl)
	mockTaskRepo := repo_mocks.NewMockITaskRepo(ctrl)

	impl := &TraceHubServiceImpl{
		benefitSvc:     mockBenefit,
		tenantProvider: mockTenant,
		traceRepo:      mockTraceRepo,
		taskRepo:       mockTaskRepo,
	}

	mockTenant.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformType("loop_all")).Return([]string{"tenant"}, nil).AnyTimes()
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
	mockTaskRepo.EXPECT().IncrTaskRunSuccessCount(gomock.Any(), int64(101), int64(202)).Return(nil)
	mockTraceRepo.EXPECT().InsertAnnotations(gomock.Any(), gomock.AssignableToTypeOf(&repo.InsertAnnotationParam{})).DoAndReturn(
		func(_ context.Context, param *repo.InsertAnnotationParam) error {
			require.Len(t, param.Annotations, 1)
			require.Equal(t, loop_span.AnnotationTypeAutoEvaluate, param.Annotations[0].AnnotationType)
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

	require.NoError(t, impl.CallBack(context.Background(), event))
}

func TestTraceHubServiceImpl_CallBackSpanNotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	t.Cleanup(ctrl.Finish)

	mockBenefit := benefit_mocks.NewMockIBenefitService(ctrl)
	mockTenant := tenant_mocks.NewMockITenantProvider(ctrl)
	mockTraceRepo := trace_repo_mocks.NewMockITraceRepo(ctrl)

	impl := &TraceHubServiceImpl{
		benefitSvc:     mockBenefit,
		tenantProvider: mockTenant,
		traceRepo:      mockTraceRepo,
	}

	mockTenant.EXPECT().GetTenantsByPlatformType(gomock.Any(), loop_span.PlatformType("loop_all")).Return([]string{"tenant"}, nil).AnyTimes()
	mockBenefit.EXPECT().CheckTraceBenefit(gomock.Any(), gomock.Any()).Return(&benefit.CheckTraceBenefitResult{StorageDuration: 1}, nil).AnyTimes()
	mockTraceRepo.EXPECT().ListSpans(gomock.Any(), gomock.AssignableToTypeOf(&repo.ListSpansParam{})).Return(&repo.ListSpansResult{}, nil)

	event := &entity.AutoEvalEvent{
		TurnEvalResults: []*entity.OnlineExptTurnEvalResult{
			{
				Status: entity.EvaluatorRunStatus_Success,
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

	require.Error(t, impl.CallBack(context.Background(), event))
}
