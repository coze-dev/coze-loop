// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	mysqlmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/mocks"
)

func TestApplyItemRunResults_PreservesEvaluatorIdentity(t *testing.T) {
	ctrl := gomock.NewController(t)
	dao := mysqlmocks.NewMockExptTurnResultDAO(ctrl)
	r := NewExptTurnResultRepo(nil, dao, nil)
	refs := []*entity.ExptTurnEvaluatorResultRef{
		{ID: 71, SpaceID: 3, ExptID: 1, ExptTurnResultID: 21, EvaluatorVersionID: 401, EvaluatorResultID: 301},
		{ID: 72, SpaceID: 3, ExptID: 1, ExptTurnResultID: 21, EvaluatorVersionID: 401, EvaluatorResultID: 302, SourceType: 1, Alias: "judge-a"},
		{ID: 73, SpaceID: 3, ExptID: 1, ExptTurnResultID: 21, EvaluatorResultID: 303, SourceType: 2, InlineKey: "quality"},
	}
	dao.EXPECT().ApplyItemRunResults(gomock.Any(), int64(1), int64(103), int64(10), int64(3), gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, _, _, _, _ int64, turns []*model.ExptTurnResult, got []*model.ExptTurnEvaluatorResultRef, _ ...db.Option) (bool, error) {
		require.Len(t, turns, 1)
		require.Equal(t, int64(103), turns[0].ExptRunID)
		require.Nil(t, turns[0].WeightedScore)
		require.Equal(t, []*model.ExptTurnEvaluatorResultRef{
			{ID: 71, SpaceID: 3, ExptID: 1, ExptTurnResultID: 21, EvaluatorVersionID: 401, EvaluatorResultID: 301},
			{ID: 72, SpaceID: 3, ExptID: 1, ExptTurnResultID: 21, EvaluatorVersionID: 401, EvaluatorResultID: 302, SourceType: 1, Alias_: "judge-a"},
			{ID: 73, SpaceID: 3, ExptID: 1, ExptTurnResultID: 21, EvaluatorResultID: 303, SourceType: 2, InlineKey: "quality"},
		}, got)
		return true, nil
	})
	applied, err := r.ApplyItemRunResults(context.Background(), 1, 103, 10, 3, []*entity.ExptTurnResult{{ID: 21, SpaceID: 3, ExptID: 1, ExptRunID: 103, ItemID: 10}}, refs)
	require.NoError(t, err)
	require.True(t, applied)
}
