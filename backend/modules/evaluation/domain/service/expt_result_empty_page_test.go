// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	lwtmocks "github.com/coze-dev/coze-loop/backend/infra/platestwrite/mocks"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	svcmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
)

func TestEmptyExperimentResultPageKeepsTotal(t *testing.T) {
	for _, tc := range []struct {
		name                  string
		target                entity.ExptFilterTarget
		accelerator, filtered bool
	}{
		{"legacy_filter", entity.ExptFilterTargetLegacy, true, true},
		{"v2_filter", entity.ExptFilterTargetV2, true, true},
		{"unfiltered_accelerator", entity.ExptFilterTargetLegacy, true, false},
		{"source_page", entity.ExptFilterTargetLegacy, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			experiment := repomocks.NewMockIExperimentRepo(ctrl)
			items := repomocks.NewMockIExptItemResultRepo(ctrl)
			turns := repomocks.NewMockIExptTurnResultRepo(ctrl)
			filters := repomocks.NewMockIExptTurnResultFilterRepo(ctrl)
			annotations := repomocks.NewMockIExptAnnotateRepo(ctrl)
			metrics := metricsmocks.NewMockExptMetric(ctrl)
			writes := lwtmocks.NewMockILatestWriteTracker(ctrl)
			versions := svcmocks.NewMockEvaluationSetVersionService(ctrl)
			now := time.Unix(1793924017, 0)
			expt := &entity.Experiment{ID: 1, SpaceID: 100, EvalSetID: 10, EvalSetVersionID: 20, ExptType: entity.ExptType_Offline, StartAt: &now}
			experiment.EXPECT().MGetByID(gomock.Any(), []int64{1}, int64(100)).Return([]*entity.Experiment{expt}, nil)
			experiment.EXPECT().GetEvaluatorRefByExptIDs(gomock.Any(), []int64{1}, int64(100)).Return(nil, nil)
			annotations.EXPECT().BatchGetExptTurnResultTagRefs(gomock.Any(), []int64{1}, int64(100)).Return(nil, nil)
			versions.EXPECT().GetEvaluationSetVersion(gomock.Any(), int64(100), int64(20), gomock.Any(), gomock.Nil()).Return(&entity.EvaluationSetVersion{}, nil, nil)
			writes.EXPECT().CheckWriteFlagByID(gomock.Any(), gomock.Any(), int64(1)).Return(false)
			metrics.EXPECT().EmitGetExptResult(int64(100), false)

			page := entity.NewPage(3, 2)
			param := &entity.MGetExperimentResultParam{SpaceID: 100, ExptIDs: []int64{1}, Page: page, UseAccelerator: tc.accelerator}
			switch {
			case tc.filtered:
				param.FilterAccelerators = map[int64]*entity.ExptTurnResultFilterAccelerator{
					1: {ItemIDs: []*entity.FieldFilter{{Key: "item_id", Op: "=", Values: []any{"42"}}}},
				}
				filters.EXPECT().QueryItemIDStates(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, filter *entity.ExptTurnResultFilterAccelerator) (map[int64]entity.ItemRunState, int64, error) {
					require.Equal(t, int64(100), filter.SpaceID)
					require.Equal(t, int64(1), filter.ExptID)
					require.Equal(t, page, filter.Page)
					return map[int64]entity.ItemRunState{}, 4, nil
				})
				metrics.EXPECT().EmitExptTurnResultFilterQueryLatency(int64(100), gomock.Any(), false)
			case tc.accelerator:
				items.EXPECT().ListItemResultsByExptID(gomock.Any(), int64(1), int64(100), page, false).Return(nil, int64(4), nil)
			default:
				turns.EXPECT().ListTurnResult(gomock.Any(), int64(100), int64(1), nil, page, false).Return(nil, int64(4), nil)
			}
			svc := ExptResultServiceImpl{
				ExperimentRepo: experiment, ExptItemResultRepo: items, ExptTurnResultRepo: turns,
				exptTurnResultFilterRepo: filters, ExptAnnotateRepo: annotations, Metric: metrics,
				lwt: writes, evaluationSetVersionService: versions,
			}
			ctx := contexts.WithExptFilterPlan(context.Background(), &entity.ExptFilterPlan{ReadTarget: tc.target})
			result, err := svc.MGetExperimentResult(ctx, param)
			require.NoError(t, err)
			require.Empty(t, result.ItemResults)
			require.Equal(t, int64(4), result.Total)
		})
	}
}
