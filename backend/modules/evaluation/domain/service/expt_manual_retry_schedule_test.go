// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	repomocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo/mocks"
)

func TestManualRetryScheduleDoesNotResetMaterializedItems(t *testing.T) {
	ctrl := gomock.NewController(t)
	runs := repomocks.NewMockIExptRunLogRepo(ctrl)
	items := repomocks.NewMockIExptItemResultRepo(ctrl)
	stats := repomocks.NewMockIExptStatsRepo(ctrl)
	stats.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, int64, int64) (*entity.ExptStats, error) {
		require.FailNow(t, "ScheduleStart must not count/reset manually materialized items again")
		return nil, nil
	}).AnyTimes()
	runs.EXPECT().Get(gomock.Any(), int64(7), int64(8)).Return(&entity.ExptRunLog{SpaceID: 9, ExptID: 7, ExptRunID: 8, ItemIds: []entity.ExptRunLogItems{{ItemIDs: []int64{12, 13}}, {ItemIDs: []int64{11}}, {ManualRetryReceipt: &entity.ManualRetryReceipt{RequestID: "rq1", AcceptedCount: 1}}}}, nil).AnyTimes()
	items.EXPECT().BatchGet(gomock.Any(), int64(9), int64(7), []int64{11}).Return([]*entity.ExptItemResult{{SpaceID: 9, ExptID: 7, ExptRunID: 8, ItemID: 11, Ext: map[string]string{"manual_retry": `{"run_id":8,"request_id":"rq1","mode":5}`}}}, nil).AnyTimes()
	exec := &ExptRetryItemsExec{exptRunLogRepo: runs, exptItemResultRepo: items, exptStatsRepo: stats}
	event := &entity.ExptScheduleEvent{SpaceID: 9, ExptID: 7, ExptRunID: 8, ExecEvalSetItemIDs: []int64{12, 13}}
	require.NoError(t, exec.ScheduleStart(context.Background(), event, &entity.Experiment{ID: 7, SpaceID: 9}))
	require.Equal(t, []int64{12, 13, 11}, event.ExecEvalSetItemIDs)
	require.NoError(t, exec.ScheduleStart(context.Background(), event, &entity.Experiment{ID: 7, SpaceID: 9}))
	require.Equal(t, []int64{12, 13, 11}, event.ExecEvalSetItemIDs)
}
