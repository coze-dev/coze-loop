// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"testing"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

func TestSupplementRetryAuthorizationCASPreservesProcessingAndTerminal(t *testing.T) {
	ctx := context.Background()
	provider := db.NewTestDB(t, &model.ExptItemResultRunLog{})
	require.NotNil(t, provider)
	items := NewExptItemResultRepo(mysql.NewExptItemResultDAO(provider))
	retries := items.(repo.IExptItemRetryRepo)
	require.NoError(t, items.BatchCreateNXRunLogs(ctx, []*entity.ExptItemResultRunLog{{ID: 1, SpaceID: 3, ExptID: 1, ExptRunID: 2, ItemID: 4, Status: int32(entity.ItemRunState_Processing)}}))
	matched, err := retries.AuthorizeItemRetry(ctx, 3, 1, 2, 4, 0, []byte("retry failure"))
	require.NoError(t, err)
	require.True(t, matched)
	matched, err = retries.AuthorizeItemRetry(ctx, 3, 1, 2, 4, 0, []byte("duplicate failure"))
	require.NoError(t, err)
	require.False(t, matched)
	got, err := items.GetItemRunLog(ctx, 1, 2, 4, 3)
	require.NoError(t, err)
	require.Equal(t, int32(1), got.RetryTimes)
	require.Equal(t, int32(entity.ItemRunState_Processing), got.Status)
	require.Equal(t, []byte("retry failure"), got.ErrMsg)
	require.NoError(t, items.(repo.IExptItemCancellationRepo).UpdateItemRunLogIfNonterminal(ctx, 1, 2, []int64{4}, map[string]any{"status": int32(entity.ItemRunState_Terminal)}, 3))
	matched, err = retries.AuthorizeItemRetry(ctx, 3, 1, 2, 4, 1, []byte("late retry"))
	require.NoError(t, err)
	require.False(t, matched)
	require.NoError(t, items.UpdateItemRunLogIfNotTerminal(ctx, 1, 2, []int64{4}, map[string]any{"status": int32(entity.ItemRunState_Fail)}, 3))
	got, err = items.GetItemRunLog(ctx, 1, 2, 4, 3)
	require.NoError(t, err)
	require.Equal(t, int32(entity.ItemRunState_Terminal), got.Status)
	require.Equal(t, int32(1), got.RetryTimes)
}

func TestResourceCancellationRepositoryPreservesTerminalAndScansAllRuns(t *testing.T) {
	ctx := context.Background()
	provider := db.NewTestDB(t, &model.ExptItemResultRunLog{})
	require.NotNil(t, provider)
	items := NewExptItemResultRepo(mysql.NewExptItemResultDAO(provider))
	cancel := items.(repo.IExptItemCancellationRepo)
	rows := []*entity.ExptItemResultRunLog{
		{ID: 1, SpaceID: 3, ExptID: 1, ExptRunID: 10, ItemID: 1, Status: int32(entity.ItemRunState_Queueing)},
		{ID: 2, SpaceID: 3, ExptID: 1, ExptRunID: 20, ItemID: 2, Status: int32(entity.ItemRunState_Processing)},
		{ID: 3, SpaceID: 3, ExptID: 1, ExptRunID: 20, ItemID: 3, Status: int32(entity.ItemRunState_Success)},
		{ID: 4, SpaceID: 3, ExptID: 1, ExptRunID: 20, ItemID: 4, Status: int32(entity.ItemRunState_Fail)},
		{ID: 5, SpaceID: 3, ExptID: 1, ExptRunID: 20, ItemID: 5, Status: int32(entity.ItemRunState_Terminal)},
	}
	require.NoError(t, items.BatchCreateNXRunLogs(ctx, rows))
	selected, next, err := cancel.ScanItemRunLogsForCancellation(ctx, 3, 1, nil, nil, 0, 10)
	require.NoError(t, err)
	require.Equal(t, int64(5), next)
	require.Equal(t, []int64{1, 2, 5}, []int64{selected[0].ID, selected[1].ID, selected[2].ID})
	require.NoError(t, cancel.UpdateItemRunLogIfNonterminal(ctx, 1, 20, []int64{2, 3, 4, 5}, map[string]any{"status": int32(entity.ItemRunState_Terminal)}, 3))
	for itemID, want := range map[int64]entity.ItemRunState{2: entity.ItemRunState_Terminal, 3: entity.ItemRunState_Success, 4: entity.ItemRunState_Fail, 5: entity.ItemRunState_Terminal} {
		got, err := items.GetItemRunLog(ctx, 1, 20, itemID, 3)
		require.NoError(t, err)
		require.Equal(t, int32(want), got.Status)
	}
	// A delayed zombie must not overwrite the cancellation start barrier.
	require.NoError(t, cancel.UpdateItemRunLogIfNonterminal(ctx, 1, 20, []int64{2}, map[string]any{"status": int32(entity.ItemRunState_Fail)}, 3))
	got, err := items.GetItemRunLog(ctx, 1, 20, 2, 3)
	require.NoError(t, err)
	require.Equal(t, int32(entity.ItemRunState_Terminal), got.Status)
}
