// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package mysql

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/callbacks"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

// Simulate the source SQL restriction while executing accepted statements on MySQL.
func rejectCurrentRequestJoins(t *testing.T, conn *gorm.DB) {
	t.Helper()
	require.NoError(t, conn.Callback().Query().Before("gorm:query").Register("reject_current_request_join", func(tx *gorm.DB) {
		callbacks.BuildQuerySQL(tx)
		if strings.Contains(strings.ToUpper(tx.Statement.SQL.String()), " JOIN ") {
			_ = tx.AddError(errors.New("Error 1105 (HY000): statement not supported"))
		}
	}))
	t.Cleanup(func() {
		require.NoError(t, conn.Callback().Query().Remove("reject_current_request_join"))
	})
}

func TestCurrentRequestMySQLWithoutJoins(t *testing.T) {
	for _, operation := range []string{"scan", "retry_first", "get"} {
		t.Run(operation, func(t *testing.T) {
			f := newCurrentResultFixture(t, "rq1")
			require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=?", f.space).Update("status", int32(entity.ItemRunState_Queueing)).Error)
			require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=?", f.space).Update("status", int32(entity.ItemRunState_Queueing)).Error)
			require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("retry_times", 2).Error)
			require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item+1).Update("retry_times", 0).Error)
			before := f.snapshot()
			rejectCurrentRequestJoins(t, f.conn)
			dao := NewExptItemResultDAO(f.provider)
			var rows []*entity.ExptItemCurrentRequest
			var err error
			if operation == "get" {
				var row *entity.ExptItemCurrentRequest
				row, err = dao.GetCurrentRequest(context.Background(), f.space, f.expt, f.run, f.item)
				rows = []*entity.ExptItemCurrentRequest{row}
			} else {
				rows, err = dao.ScanCurrentRequests(context.Background(), f.space, f.expt, f.run, operation == "retry_first")
			}
			require.NoError(t, err, "queueing current requests must be readable without JOIN support")
			wantIDs := []int64{f.item, f.item + 1, f.item + 2}
			switch operation {
			case "retry_first":
				wantIDs = []int64{f.item + 1, f.item + 2, f.item}
			case "get":
				wantIDs = []int64{f.item}
			}
			require.Len(t, rows, len(wantIDs))
			for i, row := range rows {
				require.NotNil(t, row)
				require.Equal(t, wantIDs[i], row.ItemID)
				require.Equal(t, f.version, row.ItemVersionID)
				require.Equal(t, entity.ItemRunState_Queueing, row.Status)
				if row.ItemID == f.item {
					require.Equal(t, "rq1", row.RequestID)
					require.Equal(t, entity.EvaluationModeFailRetry, row.Mode)
				} else {
					require.Empty(t, row.RequestID)
					require.Equal(t, entity.EvaluationModeSubmit, row.Mode)
				}
			}
			require.Equal(t, before, f.snapshot(), "current request reads must not change source or result tables")
		})
	}
}

func TestCurrentRequestMySQLSingleTableBindings(t *testing.T) {
	for _, change := range []string{"missing_item", "deleted_item", "other_run", "missing_run", "deleted_run", "version_mismatch"} {
		t.Run(change, func(t *testing.T) {
			f := newCurrentResultFixture(t, "")
			item := f.conn.Where("space_id=? AND item_id=?", f.space, f.item)
			switch change {
			case "missing_item":
				require.NoError(t, item.Unscoped().Delete(&model.ExptItemResult{}).Error)
			case "deleted_item":
				require.NoError(t, item.Delete(&model.ExptItemResult{}).Error)
			case "other_run":
				require.NoError(t, item.Model(&model.ExptItemResult{}).Update("expt_run_id", f.run+1).Error)
			case "missing_run":
				require.NoError(t, f.conn.Where("space_id=?", f.space).Unscoped().Delete(&model.ExptRunLog{}).Error)
			case "deleted_run":
				require.NoError(t, f.conn.Where("space_id=?", f.space).Delete(&model.ExptRunLog{}).Error)
			case "version_mismatch":
				require.NoError(t, item.Model(&model.ExptItemResult{}).Update("item_version_id", f.version+1).Error)
			}
			rejectCurrentRequestJoins(t, f.conn)
			row, err := NewExptItemResultDAO(f.provider).GetCurrentRequest(context.Background(), f.space, f.expt, f.run, f.item)
			require.Nil(t, row)
			switch change {
			case "missing_run", "deleted_run":
				require.ErrorContains(t, err, "source run missing")
			case "version_mismatch":
				require.ErrorContains(t, err, "item version mismatch")
			default:
				require.NoError(t, err)
			}
		})
	}
}

func TestCurrentRequestMySQLSplitReadRejectsConcurrentChange(t *testing.T) {
	f := newCurrentResultFixture(t, "rq1")
	provider := newObservationProvider(t, "READ-COMMITTED")
	conn := provider.NewSession(context.Background(), db.WithMaster())
	rejectCurrentRequestJoins(t, conn)
	var once sync.Once
	var readerTransaction *sql.Tx
	changed := false
	require.NoError(t, conn.Callback().Query().After("gorm:query").Register("change_current_request_between_reads", func(tx *gorm.DB) {
		transaction, ok := tx.Statement.ConnPool.(*sql.Tx)
		require.True(t, ok, "split reads must use one SQL transaction")
		if readerTransaction == nil {
			readerTransaction = transaction
		}
		require.Same(t, readerTransaction, transaction)
		if tx.Statement.Table != "expt_item_result_run_log" {
			return
		}
		once.Do(func() {
			raw, err := entity.MergeManualRetryContext(nil, &entity.ManualRetryContext{RunID: f.run, RequestID: "rq2", Mode: entity.EvaluationModeFailRetry})
			require.NoError(t, err)
			require.NoError(t, f.provider.Transaction(context.Background(), func(writer *gorm.DB) error {
				if err := writer.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("retry_times", 2).Error; err != nil {
					return err
				}
				return writer.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("ext", raw).Error
			}))
			changed = true
		})
	}))
	t.Cleanup(func() { require.NoError(t, conn.Callback().Query().Remove("change_current_request_between_reads")) })
	row, err := NewExptItemResultDAO(provider).GetCurrentRequest(context.Background(), f.space, f.expt, f.run, f.item)
	require.True(t, changed)
	require.Nil(t, row)
	require.ErrorContains(t, err, "current item observation changed")
}
