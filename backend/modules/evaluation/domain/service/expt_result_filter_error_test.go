// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	metricsmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type unfilteredTurnReadSpy struct {
	repo.IExptTurnResultRepo
	calls int
}

func (r *unfilteredTurnReadSpy) ListTurnResultByItemIDs(context.Context, int64, int64, []int64, entity.Page, bool) ([]*entity.ExptTurnResult, int64, error) {
	r.calls++
	return nil, 8, nil
}

func TestResultFilterFailuresDoNotReturnUnfilteredRows(t *testing.T) {
	for _, tc := range []struct {
		name   string
		filter *entity.ExptTurnResultFilterAccelerator
	}{
		{"output_keyword", &entity.ExptTurnResultFilterAccelerator{KeywordSearch: &entity.KeywordFilter{Keyword: gptr.Of("hit"), EvalTargetDataFilters: []*entity.FieldFilter{{Key: "actual_output", Op: "LIKE", Values: []any{"hit"}}}}}},
		{"item_id", &entity.ExptTurnResultFilterAccelerator{ItemIDs: []*entity.FieldFilter{{Key: "item_id", Op: "=", Values: []any{"10"}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			filters := repomocks.NewMockIExptTurnResultFilterRepo(ctrl)
			metrics := metricsmocks.NewMockExptMetric(ctrl)
			failure := errors.New("CK filter query unavailable")
			filters.EXPECT().QueryItemIDStates(gomock.Any(), gomock.Any()).Return(nil, int64(0), failure)
			metrics.EXPECT().EmitExptTurnResultFilterQueryLatency(int64(100), gomock.Any(), true)
			turns := &unfilteredTurnReadSpy{}
			service := ExptResultServiceImpl{exptTurnResultFilterRepo: filters, Metric: metrics, ExptTurnResultRepo: turns}
			start := time.Unix(1793924017, 0)
			rows, _, total, err := service.ListTurnResult(context.Background(), &entity.MGetExperimentResultParam{SpaceID: 100, ExptIDs: []int64{1}, BaseExptID: gptr.Of(int64(1)), UseAccelerator: true, Page: entity.NewPage(1, 10), FilterAccelerators: map[int64]*entity.ExptTurnResultFilterAccelerator{1: tc.filter}}, &entity.Experiment{ID: 1, SpaceID: 100, ExptType: entity.ExptType_Offline, StartAt: &start})
			require.ErrorIs(t, err, failure)
			require.Empty(t, rows)
			require.Zero(t, total)
			require.Zero(t, turns.calls)
		})
	}
}
