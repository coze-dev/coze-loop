// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func TestCurrentItemTransitionAllowsLegacyStatsDrift(t *testing.T) {
	for _, path := range []string{"legacy", "central"} {
		t.Run(path, func(t *testing.T) {
			provider := db.NewTestDB(t, &model.Experiment{}, &model.ExptItemResult{}, &model.ExptItemResultRunLog{}, &model.ExptRunLog{}, &model.ExptTurnResult{}, &model.ExptTurnResultRunLog{}, &model.ExptStats{})
			conn := provider.NewSession(context.Background(), db.WithMaster())
			const spaceID, exptID, runID, itemID, versionID = int64(1), int64(2), int64(3), int64(4), int64(5)
			quota := entity.QuotaReservationStateNone
			if path == "central" {
				quota = entity.QuotaReservationStateReserved
			}
			for _, row := range []any{
				&model.Experiment{ID: exptID, SpaceID: spaceID, LatestRunID: runID, Status: int32(entity.ExptStatus_Processing), ExptType: int32(entity.ExptType_Offline)},
				&model.ExptRunLog{ID: runID, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, Status: gptr.Of(int64(entity.ExptStatus_Processing)), Mode: gptr.Of(int32(entity.EvaluationModeSubmit))},
				&model.ExptItemResult{ID: 10, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, ItemID: itemID, ItemVersionID: versionID, Status: int32(entity.ItemRunState_Queueing)},
				&model.ExptItemResultRunLog{ID: 20, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, ItemID: itemID, ItemVersionID: versionID, Status: int32(entity.ItemRunState_Queueing), ResultState: gptr.Of(int32(entity.ExptItemResultStateDefault)), QuotaReservationState: int32(quota)},
				&model.ExptStats{ID: 30, SpaceID: spaceID, ExptID: exptID, PendingCnt: 0, ProcessingCnt: 0},
			} {
				require.NoError(t, conn.Create(row).Error)
			}

			if path == "legacy" {
				changed, err := NewExptItemResultDAO(provider).MarkProcessingIfCurrent(context.Background(), spaceID, exptID, runID, itemID, "")
				require.NoError(t, err)
				require.True(t, changed)
			} else {
				result, err := NewExptItemDispatchDAO(provider).StartReservedItem(context.Background(), spaceID, exptID, runID, itemID, "")
				require.NoError(t, err)
				require.True(t, result.Started)
			}

			var stats model.ExptStats
			require.NoError(t, conn.Take(&stats, 30).Error)
			require.Zero(t, stats.PendingCnt)
			require.Equal(t, int32(1), stats.ProcessingCnt)
			var item model.ExptItemResult
			require.NoError(t, conn.Take(&item, 10).Error)
			require.Equal(t, int32(entity.ItemRunState_Processing), item.Status)
		})
	}
}
