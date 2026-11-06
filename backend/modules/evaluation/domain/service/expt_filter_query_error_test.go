// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	lwtmocks "github.com/coze-dev/coze-loop/backend/infra/platestwrite/mocks"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestV2QueryErrorsDoNotFallbackToUnfilteredRDS(t *testing.T) {
	for _, target := range []entity.ExptFilterTarget{entity.ExptFilterTargetV2, entity.ExptFilterTargetLegacy} {
		for _, kind := range []string{"table_missing", "kv_expression_error", "complexity_limit", "invalid_filter"} {
			t.Run(string(target)+"/"+kind, func(t *testing.T) {
				ctrl := gomock.NewController(t)
				experiments := repomocks.NewMockIExperimentRepo(ctrl)
				items := repomocks.NewMockIExptItemResultRepo(ctrl)
				turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
				filters := repomocks.NewMockIExptTurnResultFilterRepo(ctrl)
				annotations := repomocks.NewMockIExptAnnotateRepo(ctrl)
				metrics := metricsmocks.NewMockExptMetric(ctrl)
				writes := lwtmocks.NewMockILatestWriteTracker(ctrl)
				versions := svcmocks.NewMockEvaluationSetVersionService(ctrl)
				now := time.Unix(1767312000, 0)
				expt := &entity.Experiment{ID: 1, SpaceID: 100, EvalSetID: 10, EvalSetVersionID: 20, ExptType: entity.ExptType_Offline, StartAt: &now}
				experiments.EXPECT().MGetByID(gomock.Any(), []int64{1}, int64(100)).Return([]*entity.Experiment{expt}, nil)
				experiments.EXPECT().GetEvaluatorRefByExptIDs(gomock.Any(), []int64{1}, int64(100)).Return(nil, nil)
				annotations.EXPECT().BatchGetExptTurnResultTagRefs(gomock.Any(), []int64{1}, int64(100)).Return(nil, nil)
				versions.EXPECT().GetEvaluationSetVersion(gomock.Any(), int64(100), int64(20), gomock.Any(), gomock.Nil()).Return(&entity.EvaluationSetVersion{}, nil, nil)
				writes.EXPECT().CheckWriteFlagByID(gomock.Any(), gomock.Any(), int64(1)).Return(false)
				metrics.EXPECT().EmitGetExptResult(int64(100), gomock.Any())
				metrics.EXPECT().EmitExptTurnResultFilterQueryLatency(int64(100), gomock.Any(), true)
				fault := errors.New(kind)
				queryCalls, fallbackCalls := 0, 0
				var fallbackIDs []int64
				filters.EXPECT().QueryItemIDStates(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, filter *entity.ExptTurnResultFilterAccelerator) (map[int64]entity.ItemRunState, int64, error) {
					queryCalls++
					require.Equal(t, target, contexts.ExptFilterPlanFromContext(ctx).ReadTarget)
					require.True(t, filter.HasFilters())
					return nil, 0, fault
				})
				turns.EXPECT().ListTurnResultByItemIDs(gomock.Any(), int64(100), int64(1), gomock.Any(), gomock.Any(), false).DoAndReturn(func(_ context.Context, _, _ int64, ids []int64, _ entity.Page, _ bool) ([]*entity.ExptTurnResult, int64, error) {
					fallbackCalls++
					fallbackIDs = append([]int64(nil), ids...)
					return nil, 6, nil
				}).AnyTimes()
				param := &entity.MGetExperimentResultParam{SpaceID: 100, ExptIDs: []int64{1}, Page: entity.NewPage(9, 2), UseAccelerator: true, FilterAccelerators: map[int64]*entity.ExptTurnResultFilterAccelerator{1: {ItemIDs: []*entity.FieldFilter{{Key: "item_id", Op: "=", Values: []any{"42"}}}}}}
				svc := ExptResultServiceImpl{ExperimentRepo: experiments, ExptItemResultRepo: items, ExptTurnResultRepo: turns, exptTurnResultFilterRepo: filters, ExptAnnotateRepo: annotations, Metric: metrics, lwt: writes, evaluationSetVersionService: versions}
				ctx := contexts.WithExptFilterPlan(context.Background(), &entity.ExptFilterPlan{WriteMode: "dual", ReadTarget: target, RequiredTargets: []entity.ExptFilterTarget{entity.ExptFilterTargetLegacy, entity.ExptFilterTargetV2}, ExptIDs: []int64{1}, ConfigHash: "frozen-query"})
				result, err := svc.MGetExperimentResult(ctx, param)
				record := map[string]any{"read_target": target, "injected_query_failure": kind, "error": fmt.Sprint(err), "cause_preserved": errors.Is(err, fault), "result_nil": result == nil, "query_calls": queryCalls, "unfiltered_rds_calls": fallbackCalls, "fallback_item_ids": fallbackIDs}
				if result != nil {
					record["response_total"] = result.Total
					record["response_items"] = len(result.ItemResults)
				}
				t.Logf("FILTER_FALLBACK_EVIDENCE %+v", record)
				require.Equal(t, 1, queryCalls)
				if target == entity.ExptFilterTargetV2 {
					require.ErrorIs(t, err, fault)
					require.Nil(t, result)
					require.Zero(t, fallbackCalls)
				} else {
					require.NoError(t, err)
					require.NotNil(t, result)
					require.Equal(t, 1, fallbackCalls)
					require.Empty(t, fallbackIDs)
					require.Equal(t, int64(6), result.Total)
					require.Empty(t, result.ItemResults)
				}
			})
		}
	}
}
