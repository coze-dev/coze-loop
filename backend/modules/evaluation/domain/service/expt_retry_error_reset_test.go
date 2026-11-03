// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
)

func TestResetRetryRunLogsForItems_ClearsCurrentError(t *testing.T) {
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	deps := newRetryHelperDeps(ctrl)
	oldError := errno.SerializeErr(errno.NewTurnOtherErr("previous attempt failed", nil))
	item := &entity.ExptItemResult{ItemID: 10, ExptRunID: 100, Status: entity.ItemRunState_Fail, ErrMsg: oldError}
	turn := &entity.ExptTurnResult{ID: 20, ItemID: 10, TurnID: 30, ExptRunID: 100, Status: int32(entity.TurnRunState_Fail), ErrMsg: oldError}
	builder := &ExptResultBuilder{
		turnResultDO:              []*entity.ExptTurnResult{turn},
		ItemIDTurnID2TurnResultID: map[int64]map[int64]int64{10: {30: 20}},
	}
	require.Contains(t, *builder.getTurnSystemInfo(ctx, 10, 30).Error.Detail, "previous attempt failed")

	deps.idgenerator.(*idgenmocks.MockIIDGenerator).EXPECT().GenMultiIDs(gomock.Any(), 2).Return([]int64{40, 50}, nil)
	deps.exptItemResultRepo.(*retryTestItemResultRepo).reset.EXPECT().ResetItemForRetry(
		gomock.Any(), int64(1), int64(2), int64(101), gomock.Any(), gomock.Any(),
	).DoAndReturn(func(_ context.Context, _, _, _ int64, runLog *entity.ExptItemResultRunLog, turnIDs []int64) (bool, error) {
		require.Equal(t, []int64{30}, turnIDs)
		require.Equal(t, int64(101), runLog.ExptRunID)
		require.Empty(t, runLog.ErrMsg)
		item.Status = entity.ItemRunState_Queueing
		item.ExptRunID = 101
		item.ErrMsg = ""
		turn.Status = int32(entity.TurnRunState_Queueing)
		turn.ExptRunID = 101
		turn.ErrMsg = ""
		return true, nil
	})

	err := resetRetryItemsForNewRun(ctx, deps,
		&entity.ExptScheduleEvent{SpaceID: 1, ExptID: 2, ExptRunID: 101},
		&entity.Experiment{Status: entity.ExptStatus_Processing},
		[]*entity.EvaluationSetItem{{ItemID: 10, Turns: []*entity.Turn{{ID: 30}}}}, nil)
	require.NoError(t, err)
	require.Empty(t, item.ErrMsg)
	require.Empty(t, turn.ErrMsg)
	require.Equal(t, entity.TurnRunState_Queueing, builder.getTurnSystemInfo(ctx, 10, 30).TurnRunState)
	require.Nil(t, builder.getTurnSystemInfo(ctx, 10, 30).Error)
	turn.Status = int32(entity.TurnRunState_Processing)
	require.Nil(t, builder.getTurnSystemInfo(ctx, 10, 30).Error)
	turn.Status = int32(entity.TurnRunState_Fail)
	turn.ErrMsg = errno.SerializeErr(errno.NewTurnOtherErr("current attempt failed", nil))
	require.Contains(t, *builder.getTurnSystemInfo(ctx, 10, 30).Error.Detail, "current attempt failed")
}
