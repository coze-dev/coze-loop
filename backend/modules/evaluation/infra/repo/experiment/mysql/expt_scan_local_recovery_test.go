// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestScanLocalSourceDamageRemainsPending(t *testing.T) {
	for _, logged := range []bool{false, true} {
		name := "processing"
		if logged {
			name = "logged"
		}
		t.Run(name, func(t *testing.T) {
			for _, damage := range []string{"turn_version", "manual_zero_version", "turn_json", "turn_status", "item_version", "item_status", "item_ext", "frozen_missing_turn"} {
				t.Run(damage, func(t *testing.T) {
					f := newCurrentResultFixture(t, "rq1")
					if logged {
						require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Updates(map[string]any{"status": int32(entity.ItemRunState_Success), "result_state": int32(entity.ExptItemResultStateLogged)}).Error)
					}
					turn := f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND item_id=? AND turn_id=0", f.space, f.item)
					item := f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item)
					switch damage {
					case "turn_version":
						require.NoError(t, turn.Update("item_version_id", f.version+1).Error)
					case "manual_zero_version":
						require.NoError(t, turn.Update("item_version_id", 0).Error)
					case "turn_json":
						require.NoError(t, turn.Update("evaluator_result_ids", []byte(`{"registered":`)).Error)
					case "turn_status":
						require.NoError(t, turn.Update("status", 99).Error)
					case "item_version":
						require.NoError(t, item.Update("item_version_id", f.version+1).Error)
					case "item_status":
						require.NoError(t, item.Update("status", 99).Error)
					case "item_ext":
						require.NoError(t, item.Update("ext", []byte(`{"manual_retry":"null"}`)).Error)
					case "frozen_missing_turn":
						require.NoError(t, turn.Unscoped().Delete(&model.ExptTurnResultRunLog{}).Error)
					}
					before := f.snapshot()
					rows, err := NewExptItemResultDAO(f.provider).ScanCurrentRequests(context.Background(), f.space, f.expt, f.run, false)
					require.NoError(t, err, "one damaged source must not block the other items")
					require.Len(t, rows, 3)
					byItem := make(map[int64]*entity.ExptItemCurrentRequest)
					for _, row := range rows {
						byItem[row.ItemID] = row
					}
					require.True(t, byItem[f.item].ObservationUnstable)
					require.Nil(t, byItem[f.item].Source)
					require.Nil(t, byItem[f.item].Publication)
					require.False(t, byItem[f.item+1].ObservationUnstable)
					require.NotNil(t, byItem[f.item+1].Source)
					require.Equal(t, entity.ItemRunState_Queueing, byItem[f.item+2].Status)
					_, err = f.port.GetCurrentItemRunSource(context.Background(), f.space, f.expt, f.run, f.item)
					require.Error(t, err, "execution remains fail closed on the damaged identity/source")
					finish, err := NewExptRunFinishDAO(f.provider).FinishCurrentRun(context.Background(), &entity.ExptRunFinishRequest{SpaceID: f.space, ExptID: f.expt, RunID: f.run})
					require.NoError(t, err)
					require.Equal(t, entity.ExptRunFinishPending, finish.Disposition)
					require.Equal(t, before, f.snapshot(), "scan, strict rejection and pending finish cannot mutate or release the request")
				})
			}
		})
	}
}

func TestScanLocalRecoveryPreservesGlobalErrors(t *testing.T) {
	for _, damage := range []string{"run_status", "run_mode", "missing_run", "database"} {
		t.Run(damage, func(t *testing.T) {
			f := newCurrentResultFixture(t, "rq1")
			run := f.conn.Model(&model.ExptRunLog{}).Where("space_id=?", f.space)
			sentinel := errors.New("source SELECT failed")
			switch damage {
			case "run_status":
				require.NoError(t, run.Update("status", 999).Error)
			case "run_mode":
				require.NoError(t, run.Update("mode", 999).Error)
			case "missing_run":
				require.NoError(t, run.Unscoped().Delete(&model.ExptRunLog{}).Error)
			case "database":
				hits := 0
				require.NoError(t, f.conn.Callback().Query().Before("gorm:query").Register("source_select_failure", func(tx *gorm.DB) {
					if tx.Statement.Table == "expt_turn_result_run_log" {
						hits++
						if hits == 2 {
							_ = tx.AddError(sentinel)
						}
					}
				}))
				t.Cleanup(func() { require.NoError(t, f.conn.Callback().Query().Remove("source_select_failure")) })
			}
			_, err := NewExptItemResultDAO(f.provider).ScanCurrentRequests(context.Background(), f.space, f.expt, f.run, false)
			require.Error(t, err, "global run and database failures still abort the scan")
			if damage == "database" {
				require.ErrorIs(t, err, sentinel)
			}
		})
	}
}
