// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func TestCurrentItemTransitionMySQLAllowsLegacyStatsDrift(t *testing.T) {
	for _, path := range []string{"legacy_start", "central_start", "central_requeue"} {
		for _, sourceCount := range []int32{0, -1} {
			t.Run(fmt.Sprintf("%s/%d", path, sourceCount), func(t *testing.T) {
				f := newManualRetryFixture(t)
				ctx := context.Background()
				const itemID = int64(13)
				counts := map[string]any{"pending_cnt": sourceCount, "processing_cnt": 0}
				expectedState := entity.ItemRunState_Processing
				if path == "central_start" {
					require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, itemID).Update("quota_reservation_state", entity.QuotaReservationStateReserved).Error)
				}
				if path == "central_requeue" {
					counts = map[string]any{"pending_cnt": 0, "processing_cnt": sourceCount}
					expectedState = entity.ItemRunState_Queueing
					for _, row := range []any{&model.ExptItemResult{}, &model.ExptItemResultRunLog{}} {
						require.NoError(t, f.conn.Model(row).Where("space_id=? AND item_id=?", f.space, itemID).Update("status", entity.ItemRunState_Processing).Error)
					}
				}
				require.NoError(t, f.conn.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", f.space, f.expt).Updates(counts).Error)
				transition := func(request string) (bool, error) {
					switch path {
					case "legacy_start":
						return NewExptItemResultDAO(f.provider).MarkProcessingIfCurrent(ctx, f.space, f.expt, f.run, itemID, request)
					case "central_requeue":
						return NewExptItemDispatchDAO(f.provider).RequeueProcessingItem(ctx, f.space, f.expt, f.run, itemID, request)
					default:
						result, err := NewExptItemDispatchDAO(f.provider).StartReservedItem(ctx, f.space, f.expt, f.run, itemID, request)
						if err != nil {
							return false, err
						}
						require.NotNil(t, result)
						return result.Started, nil
					}
				}
				before := f.snapshot()
				changed, err := transition("other-request")
				require.NoError(t, err)
				require.False(t, changed)
				require.Equal(t, before, f.snapshot(), "stats drift cannot bypass current-request identity")

				changed, err = transition("")
				require.NoError(t, err, "legacy stats drift must not prevent a valid state transition")
				require.True(t, changed)
				var stats model.ExptStats
				require.NoError(t, f.conn.Where("space_id=? AND expt_id=?", f.space, f.expt).Take(&stats).Error)
				if path == "central_requeue" {
					require.Equal(t, int32(1), stats.PendingCnt)
					require.Zero(t, stats.ProcessingCnt)
				} else {
					require.Zero(t, stats.PendingCnt)
					require.Equal(t, int32(1), stats.ProcessingCnt)
				}
				require.Equal(t, int32(1), stats.FailCnt)
				require.EqualValues(t, 42, stats.CreditCost)
				var item model.ExptItemResult
				var log model.ExptItemResultRunLog
				require.NoError(t, f.conn.Where("space_id=? AND item_id=?", f.space, itemID).Take(&item).Error)
				require.NoError(t, f.conn.Where("space_id=? AND item_id=?", f.space, itemID).Take(&log).Error)
				require.Equal(t, int32(expectedState), item.Status)
				require.Equal(t, item.Status, log.Status)
				require.Equal(t, int32(entity.QuotaReservationStateNone), log.QuotaReservationState)

				completed := f.snapshot()
				changed, err = transition("")
				require.NoError(t, err)
				require.False(t, changed, "a duplicate transition must not count twice")
				require.Equal(t, completed, f.snapshot())
			})
		}
	}
}
