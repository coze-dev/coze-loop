// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	servicemocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestExperimentTargetRecordOwnership(t *testing.T) {
	ctx := context.Background()
	t.Run("consumer record does not read source", func(t *testing.T) {
		svc := servicemocks.NewMockIEvalTargetService(gomock.NewController(t))
		record := &entity.EvalTargetRecord{ID: 30, SpaceID: 10}
		svc.EXPECT().GetRecordByID(ctx, int64(10), int64(30)).Return(record, nil)
		got, err := GetExperimentTargetRecord(ctx, svc, 10, 20, 30)
		require.NoError(t, err)
		require.Same(t, record, got)
	})
	t.Run("legacy record remains readable", func(t *testing.T) {
		svc := servicemocks.NewMockIEvalTargetService(gomock.NewController(t))
		record := &entity.EvalTargetRecord{ID: 30, SpaceID: 20}
		gomock.InOrder(svc.EXPECT().GetRecordByID(ctx, int64(10), int64(30)).Return(nil, nil), svc.EXPECT().GetRecordByID(ctx, int64(20), int64(30)).Return(record, nil))
		got, err := GetExperimentTargetRecord(ctx, svc, 10, 20, 30)
		require.NoError(t, err)
		require.Same(t, record, got)
	})
	t.Run("query failures do not trigger cross-space fallback", func(t *testing.T) {
		svc := servicemocks.NewMockIEvalTargetService(gomock.NewController(t))
		failure := errors.New("read unavailable")
		svc.EXPECT().GetRecordByID(ctx, int64(10), int64(30)).Return(nil, failure)
		_, err := GetExperimentTargetRecord(ctx, svc, 10, 20, 30)
		require.ErrorIs(t, err, failure)
	})
	t.Run("mixed generations fetch only missing source records", func(t *testing.T) {
		svc := servicemocks.NewMockIEvalTargetService(gomock.NewController(t))
		current := &entity.EvalTargetRecord{ID: 30, SpaceID: 10}
		legacy := &entity.EvalTargetRecord{ID: 31, SpaceID: 20}
		gomock.InOrder(svc.EXPECT().BatchGetRecordByIDs(ctx, int64(10), []int64{30, 31}).Return([]*entity.EvalTargetRecord{current}, nil), svc.EXPECT().BatchGetRecordByIDs(ctx, int64(20), []int64{31}).Return([]*entity.EvalTargetRecord{legacy}, nil))
		got, err := BatchGetExperimentTargetRecords(ctx, svc, 10, 20, []int64{30, 31})
		require.NoError(t, err)
		require.Equal(t, []*entity.EvalTargetRecord{current, legacy}, got)
	})
}

func TestSandboxRecordDefinitionRemainsInSourceWorkspace(t *testing.T) {
	ctrl := gomock.NewController(t)
	store := repomocks.NewMockIEvalTargetRepo(ctrl)
	svc := &EvalTargetServiceImpl{evalTargetRepo: store}
	record := &entity.EvalTargetRecord{ID: 30, SpaceID: 10, TargetID: 40, TargetVersionID: 50}
	target := &entity.EvalTarget{ID: 40, SpaceID: 20, EvalTargetType: entity.EvalTargetTypeSandboxAgent, EvalTargetVersion: &entity.EvalTargetVersion{ID: 50}}
	gomock.InOrder(
		store.EXPECT().GetEvalTargetVersion(gomock.Any(), int64(10), int64(50)).Return(nil, errorx.NewByCode(errno.ResourceNotFoundCode)),
		store.EXPECT().GetEvalTarget(gomock.Any(), int64(40)).Return(target, nil),
		store.EXPECT().GetEvalTargetVersion(gomock.Any(), int64(20), int64(50)).Return(target, nil),
	)
	got, err := svc.recordTargetVersion(context.Background(), record)
	require.NoError(t, err)
	require.Same(t, target, got)
}

func TestSandboxAsyncRecordUsesExperimentWorkspace(t *testing.T) {
	for _, tc := range []struct {
		name                           string
		experimentSpace, expectedSpace int64
	}{{"shared", 10, 10}, {"same-space", 20, 20}, {"debug", 0, 20}} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			store := repomocks.NewMockIEvalTargetRepo(ctrl)
			operator := servicemocks.NewMockISourceEvalTargetOperateService(ctrl)
			metric := metricsmocks.NewMockEvalTargetMetrics(ctrl)
			config := componentmocks.NewMockIConfiger(ctrl)
			target := &entity.EvalTarget{ID: 40, SpaceID: 20, SourceTargetID: "shared-agent", EvalTargetType: entity.EvalTargetTypeSandboxAgent, EvalTargetVersion: &entity.EvalTargetVersion{ID: 50, SourceTargetVersion: "v1"}}
			input := &entity.EvalTargetInputData{}
			operator.EXPECT().ValidateInput(gomock.Any(), int64(20), gomock.Any(), input).Return(nil)
			operator.EXPECT().AsyncExecute(gomock.Any(), int64(20), gomock.Any()).DoAndReturn(func(_ context.Context, _ int64, p *entity.ExecuteEvalTargetParam) (int64, string, map[string]string, error) {
				require.Equal(t, tc.experimentSpace, p.ExptSpaceID)
				require.Same(t, target, p.EvalTarget)
				return 30, "sandbox_agent", nil, nil
			})
			metric.EXPECT().EmitRun(tc.expectedSpace, gomock.Any(), gomock.Any())
			config.EXPECT().BuildEvalExt(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes().Return(nil)
			if tc.expectedSpace != 20 {
				store.EXPECT().GetEvalTargetVersion(gomock.Any(), tc.expectedSpace, int64(50)).Return(nil, errorx.NewByCode(errno.ResourceNotFoundCode)).AnyTimes()
				store.EXPECT().GetEvalTarget(gomock.Any(), int64(40)).Return(target, nil).AnyTimes()
			}
			store.EXPECT().GetEvalTargetVersion(gomock.Any(), int64(20), int64(50)).Return(target, nil).AnyTimes()
			store.EXPECT().CreateEvalTargetRecord(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, r *entity.EvalTargetRecord, _ *bool) (int64, error) {
				require.Equal(t, tc.expectedSpace, r.SpaceID)
				return r.ID, nil
			})
			svc := &EvalTargetServiceImpl{evalTargetRepo: store, typedOperators: map[entity.EvalTargetType]ISourceEvalTargetOperateService{entity.EvalTargetTypeSandboxAgent: operator}, metric: metric, configer: config}
			record, _, err := svc.asyncExecuteTarget(context.Background(), 20, target, &entity.ExecuteTargetCtx{ExptSpaceID: tc.experimentSpace, ExperimentID: gptr.Of(int64(60))}, input)
			require.NoError(t, err)
			require.Equal(t, tc.expectedSpace, record.SpaceID)
		})
	}
}

func TestExperimentSandboxCleanupIncludesLegacyRecords(t *testing.T) {
	ctx := context.Background()
	svc := servicemocks.NewMockIEvalTargetService(gomock.NewController(t))
	failure := errors.New("consumer cleanup unavailable")
	gomock.InOrder(
		svc.EXPECT().TerminateAsyncRecordsAndDestroySandbox(ctx, int64(10), []int64{30, 31}, int32(1), "cancelled", false, true).Return(failure),
		svc.EXPECT().TerminateAsyncRecordsAndDestroySandbox(ctx, int64(20), []int64{30, 31}, int32(1), "cancelled", false, true).Return(nil),
	)
	require.ErrorIs(t, terminateExperimentTargetRecords(ctx, svc, 10, 20, []int64{30, 31}, 1, "cancelled", false, true), failure)
}

func TestExperimentSandboxStatusIncludesBothGenerations(t *testing.T) {
	ctx := context.Background()
	svc := servicemocks.NewMockIEvalTargetService(gomock.NewController(t))
	svc.EXPECT().CheckSandboxTerminated(ctx, int64(10), []int64{30, 31}).Return([]int64{30}, map[int64]string{30: "Failed"})
	svc.EXPECT().CheckSandboxTerminated(ctx, int64(20), []int64{30, 31}).Return([]int64{31}, map[int64]string{31: "Canceled"})
	ids, status := checkExperimentSandboxTerminated(ctx, svc, 10, 20, []int64{30, 31})
	require.Equal(t, []int64{30, 31}, ids)
	require.Equal(t, map[int64]string{30: "Failed", 31: "Canceled"}, status)
}
