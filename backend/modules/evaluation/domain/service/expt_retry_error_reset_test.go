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
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
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
	deps.exptItemResultRepo.(*repomocks.MockIExptItemResultRepo).EXPECT().UpdateItemsResult(
		gomock.Any(), int64(1), int64(2), []int64{10}, gomock.Any(),
	).DoAndReturn(func(_ context.Context, _, _ int64, _ []int64, fields map[string]any) error {
		item.Status = entity.ItemRunState(fields["status"].(int32))
		item.ExptRunID = fields["expt_run_id"].(int64)
		if value, exists := fields["err_msg"]; exists && value == nil {
			item.ErrMsg = ""
		}
		return nil
	})
	deps.exptTurnResultRepo.(*repomocks.MockIExptTurnResultRepo).EXPECT().UpdateTurnResults(
		gomock.Any(), int64(2), gomock.Any(), int64(1), gomock.Any(),
	).DoAndReturn(func(_ context.Context, _ int64, _ []*entity.ItemTurnID, _ int64, fields map[string]any) error {
		turn.Status = fields["status"].(int32)
		turn.ExptRunID = fields["expt_run_id"].(int64)
		if value, exists := fields["err_msg"]; exists && value == nil {
			turn.ErrMsg = ""
		}
		return nil
	})
	deps.exptTurnResultRepo.(*repomocks.MockIExptTurnResultRepo).EXPECT().UpdateTurnRunLogWithItemIDs(
		gomock.Any(), int64(1), int64(2), int64(101), []int64{10}, gomock.Any(),
	).Return(nil)
	deps.exptItemResultRepo.(*repomocks.MockIExptItemResultRepo).EXPECT().BatchCreateNXRunLogs(
		gomock.Any(), gomock.Any(),
	).DoAndReturn(func(_ context.Context, rows []*entity.ExptItemResultRunLog) error {
		require.Len(t, rows, 1)
		require.Equal(t, int64(101), rows[0].ExptRunID)
		require.Empty(t, rows[0].ErrMsg)
		return nil
	})

	_, err := resetRetryRunLogsForItems(ctx, deps,
		&entity.ExptScheduleEvent{SpaceID: 1, ExptID: 2, ExptRunID: 101},
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
