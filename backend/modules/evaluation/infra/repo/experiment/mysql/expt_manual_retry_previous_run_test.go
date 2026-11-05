// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

// A normal failed Submit item remains in R1 while R2 runs other items. There
// is one failed canonical turn in R1; no historical Success exception is used.
func previousRunRetryFixture(t *testing.T, mode entity.ExptRunMode) (*manualRetryFixture, *entity.RequeueTerminalItemsRequest) {
	t.Helper()
	f := newManualRetryFixture(t)
	require.NoError(t, f.conn.Where("space_id=? AND expt_turn_result_id=?", f.space, f.base+622).Delete(&model.ExptTurnEvaluatorResultRef{}).Error)
	require.NoError(t, f.conn.Where("space_id=? AND item_id=11 AND turn_id=22", f.space).Delete(&model.ExptTurnResultRunLog{}).Error)
	require.NoError(t, f.conn.Where("space_id=? AND item_id=11 AND turn_id=22", f.space).Delete(&model.ExptTurnResult{}).Error)
	request := f.request(mode)
	require.Len(t, request.Items[0].Turns, 1)
	request.RunID = f.base + 999
	request.Items[0].CurrentItemLog = nil
	for _, turn := range request.Items[0].Turns {
		turn.CurrentRunLog = nil
	}
	require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("id=?", f.run).Update("status", int64(entity.ExptStatus_Terminated)).Error)
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("id=?", f.expt).Update("latest_run_id", request.RunID).Error)
	require.NoError(t, f.conn.Create(&model.ExptRunLog{ID: request.RunID, SpaceID: f.space, ExptID: f.expt, ExptRunID: request.RunID, Status: gptr.Of(int64(entity.ExptStatus_Processing)), Mode: gptr.Of(int32(entity.EvaluationModeSubmit)), ItemIds: gptr.Of([]byte(`[{"ItemIDs":[12,13],"CreateAt":1700000000}]`))}).Error)
	require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id IN ?", f.space, []int64{12, 13}).Update("expt_run_id", request.RunID).Error)
	require.NoError(t, f.conn.Exec("INSERT INTO expt_item_result_run_log (id,space_id,expt_id,expt_run_id,item_id,item_version_id,status) SELECT id+1000,space_id,expt_id,?,item_id,item_version_id,status FROM expt_item_result_run_log WHERE space_id=? AND item_id IN (12,13)", request.RunID, f.space).Error)
	return f, request
}

func TestManualRetryPreviousRunMySQLAdmission(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeFailRetry, entity.EvaluationModeRetryItems} {
		for _, state := range []string{"logged", "default", "missing", "wrong_version"} {
			for _, boundary := range []string{"preflight", "commit"} {
				t.Run(fmt.Sprintf("mode=%d/%s/%s", mode, state, boundary), func(t *testing.T) {
					f, request := previousRunRetryFixture(t, mode)
					query := f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND expt_run_id=? AND item_id=11", f.space, f.run)
					reason := "result_not_ready"
					switch state {
					case "logged":
						require.NoError(t, query.Update("result_state", int32(entity.ExptItemResultStateLogged)).Error)
					case "default":
						require.NoError(t, query.Update("result_state", int32(entity.ExptItemResultStateDefault)).Error)
					case "missing":
						require.NoError(t, query.Delete(&model.ExptItemResultRunLog{}).Error)
					case "wrong_version":
						require.NoError(t, query.Update("item_version_id", f.base+112).Error)
						reason = "source_changed"
					}
					before := f.snapshot()
					if boundary == "preflight" {
						plan, err := f.port.PreflightTerminalItems(context.Background(), request)
						require.NoError(t, err)
						require.Empty(t, plan.Items)
						require.Equal(t, []*entity.ManualRetryRejectedItem{{ItemID: 11, Reason: reason}}, plan.RejectedItems)
					} else {
						receipt, err := f.port.RequeueTerminalItems(context.Background(), request)
						require.NoError(t, err)
						require.Zero(t, receipt.AcceptedCount)
						require.Equal(t, []*entity.ManualRetryRejectedItem{{ItemID: 11, Reason: reason}}, receipt.RejectedItems)
					}
					require.Equal(t, before, f.snapshot(), "reject before reservation release or reset; no receipt, new logs, counters or refs may change")
				})
			}
		}
	}
}

func TestManualRetryPreviousRunMySQLPublicationDrift(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeFailRetry, entity.EvaluationModeRetryItems} {
		for _, boundary := range []string{"after_prepare", "after_preflight"} {
			for _, state := range []entity.ExptItemResultState{entity.ExptItemResultStateLogged, entity.ExptItemResultStateDefault} {
				t.Run(fmt.Sprintf("mode=%d/%s/state=%d", mode, boundary, state), func(t *testing.T) {
					f, request := previousRunRetryFixture(t, mode)
					if boundary == "after_preflight" {
						plan, err := f.port.PreflightTerminalItems(context.Background(), request)
						require.NoError(t, err)
						require.Len(t, plan.Items, 1)
					}
					require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND expt_run_id=? AND item_id=11", f.space, f.run).Update("result_state", int32(state)).Error)
					before := f.snapshot()
					if boundary == "after_prepare" {
						plan, err := f.port.PreflightTerminalItems(context.Background(), request)
						require.NoError(t, err)
						require.Empty(t, plan.Items)
						require.Equal(t, "result_not_ready", plan.RejectedItems[0].Reason)
					}
					receipt, err := f.port.RequeueTerminalItems(context.Background(), request)
					require.NoError(t, err)
					require.Zero(t, receipt.AcceptedCount)
					require.Equal(t, "result_not_ready", receipt.RejectedItems[0].Reason)
					require.Equal(t, before, f.snapshot())
				})
			}
		}
	}
}

func TestManualRetryPreviousRunMySQLPublishedRebuild(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeFailRetry, entity.EvaluationModeRetryItems} {
		for _, zero := range []bool{false, true} {
			t.Run(fmt.Sprintf("mode=%d/zero=%t", mode, zero), func(t *testing.T) {
				f, request := previousRunRetryFixture(t, mode)
				if zero {
					require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND item_id=11", f.space).Update("item_version_id", 0).Error)
				}
				plan, err := f.port.PreflightTerminalItems(context.Background(), request)
				require.NoError(t, err)
				require.Len(t, plan.Items, 1)
				require.Nil(t, plan.Items[0].CurrentItemLog)
				// Unused log metadata is not a new frozen source snapshot.
				require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND expt_run_id=? AND item_id=11", f.space, f.run).Update("log_id", "published-metadata-change").Error)
				var oldItem model.ExptItemResultRunLog
				var oldTurns []model.ExptTurnResultRunLog
				require.NoError(t, f.conn.Where("space_id=? AND expt_run_id=? AND item_id=11", f.space, f.run).Take(&oldItem).Error)
				require.NoError(t, f.conn.Where("space_id=? AND expt_run_id=? AND item_id=11", f.space, f.run).Order("id").Find(&oldTurns).Error)
				receipt, err := f.port.RequeueTerminalItems(context.Background(), request)
				require.NoError(t, err)
				require.Equal(t, int32(1), receipt.AcceptedCount)
				source, err := NewExptItemResultDAO(f.provider).(repo.IExptCurrentResultRepo).GetCurrentItemRunSource(context.Background(), f.space, f.expt, request.RunID, 11)
				require.NoError(t, err)
				require.Equal(t, request.RequestID, source.Current.RequestID)
				require.Equal(t, f.base+111, source.Turns[21].ItemVersionID)
				var afterItem model.ExptItemResultRunLog
				var afterTurns []model.ExptTurnResultRunLog
				require.NoError(t, f.conn.Where("space_id=? AND expt_run_id=? AND item_id=11", f.space, f.run).Take(&afterItem).Error)
				require.NoError(t, f.conn.Where("space_id=? AND expt_run_id=? AND item_id=11", f.space, f.run).Order("id").Find(&afterTurns).Error)
				require.Equal(t, oldItem, afterItem)
				require.Equal(t, oldTurns, afterTurns)
			})
		}
	}
}

func TestManualRetryPreviousRunMySQLQueryFailure(t *testing.T) {
	for _, boundary := range []string{"preflight", "commit"} {
		t.Run(boundary, func(t *testing.T) {
			f, request := previousRunRetryFixture(t, entity.EvaluationModeRetryItems)
			fault := errors.New("previous item log query failed")
			hit := false
			name := "previous_log_failure"
			require.NoError(t, f.conn.Callback().Query().After("gorm:query").Register(name, func(tx *gorm.DB) {
				if tx.Statement.Table != "expt_item_result_run_log" {
					return
				}
				for _, value := range tx.Statement.Vars {
					if run, ok := value.(int64); ok && run == f.run {
						require.IsType(t, &sql.Tx{}, tx.Statement.ConnPool)
						require.Contains(t, strings.ToUpper(tx.Statement.SQL.String()), "FOR UPDATE")
						hit = true
						_ = tx.AddError(fault)
					}
				}
			}))
			defer func() { require.NoError(t, f.conn.Callback().Query().Remove(name)) }()
			before := f.snapshot()
			if boundary == "preflight" {
				plan, err := f.port.PreflightTerminalItems(context.Background(), request)
				require.ErrorIs(t, err, fault)
				require.Nil(t, plan)
			} else {
				receipt, err := f.port.RequeueTerminalItems(context.Background(), request)
				require.ErrorIs(t, err, fault)
				require.Nil(t, receipt)
			}
			require.True(t, hit)
			require.Equal(t, before, f.snapshot())
		})
	}
}
