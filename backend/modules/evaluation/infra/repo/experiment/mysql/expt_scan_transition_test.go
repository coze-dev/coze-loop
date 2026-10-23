// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func TestScanTransitionMySQLSingleWriterWindows(t *testing.T) {
	for _, kind := range []string{"yield", "central_start", "terminate_log_first", "terminate_canonical_first", "canonical_terminal_late_success"} {
		t.Run(kind, func(t *testing.T) {
			f := newManualRetryFixture(t)
			ctx := context.Background()
			dao := NewExptItemResultDAO(f.provider)
			item := int64(12)
			status := entity.ItemRunState_Queueing
			switch kind {
			case "yield":
				require.NoError(t, dao.UpdateItemRunLog(ctx, f.expt, f.run, []int64{item}, map[string]any{"status": int32(entity.ItemRunState_Queueing)}, f.space))
			case "central_start":
				item = 13
				status = entity.ItemRunState_Processing
				require.NoError(t, dao.UpdateItemRunLog(ctx, f.expt, f.run, []int64{item}, map[string]any{"quota_reservation_state": int32(entity.QuotaReservationStateReserved)}, f.space))
				changed, err := NewExptItemDispatchDAO(f.provider).StartReservedItem(ctx, f.space, f.expt, f.run, item, "")
				require.NoError(t, err)
				require.True(t, changed.Started)
			case "terminate_log_first":
				status = entity.ItemRunState_Terminal
				require.NoError(t, dao.UpdateItemRunLog(ctx, f.expt, f.run, []int64{item}, map[string]any{"status": int32(entity.ItemRunState_Terminal), "result_state": int32(entity.ExptItemResultStateLogged)}, f.space))
			case "terminate_canonical_first":
				status = entity.ItemRunState_Terminal
				require.NoError(t, dao.UpdateItemsResult(ctx, f.space, f.expt, []int64{item}, map[string]any{"status": int32(entity.ItemRunState_Terminal)}))
			case "canonical_terminal_late_success":
				status = entity.ItemRunState_Terminal
				require.NoError(t, dao.UpdateItemsResult(ctx, f.space, f.expt, []int64{item}, map[string]any{"status": int32(entity.ItemRunState_Terminal)}))
				require.NoError(t, dao.UpdateItemRunLog(ctx, f.expt, f.run, []int64{item}, map[string]any{"status": int32(entity.ItemRunState_Success), "result_state": int32(entity.ExptItemResultStateLogged)}, f.space))
			}
			before := f.snapshot()
			rows, err := dao.ScanCurrentRequests(ctx, f.space, f.expt, f.run, false)
			require.NoError(t, err)
			var got *entity.ExptItemCurrentRequest
			for _, row := range rows {
				if row.ItemID == item {
					got = row
				}
			}
			require.NotNil(t, got)
			require.Equal(t, status, got.Status)
			require.Equal(t, int32(3), got.RetryTimes)
			source, err := dao.GetCurrentRequest(ctx, f.space, f.expt, f.run, item)
			require.NoError(t, err)
			require.Equal(t, status, source.Status)
			if status == entity.ItemRunState_Terminal {
				require.False(t, source.Matches(f.run, ""))
			}
			require.Equal(t, before, f.snapshot(), "scan must never heal or overwrite projections")
			if kind == "canonical_terminal_late_success" {
				require.NotEqual(t, entity.ExptItemResultStateLogged, got.ResultState, "conflicting late terminal result must not be archived over canonical Terminal")
			}
			t.Logf("REAL_MYSQL_SCAN_TRANSITION kind=%s status=%d canonical/log divergence read-only", kind, status)
		})
	}
}

func TestScanTransitionMySQLIdentityAndCorruption(t *testing.T) {
	for _, kind := range []string{"soft_canonical", "soft_log", "old_run", "old_token", "version_mismatch", "bad_json", "unknown_canonical", "unknown_log", "null_context", "missing_run"} {
		t.Run(kind, func(t *testing.T) {
			f := newManualRetryFixture(t)
			ctx := context.Background()
			dao := NewExptItemResultDAO(f.provider)
			item := int64(13)
			bad := false
			switch kind {
			case "soft_canonical":
				require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, item).Delete(&model.ExptItemResult{}).Error)
			case "soft_log":
				require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, item).Delete(&model.ExptItemResultRunLog{}).Error)
			case "old_run":
				require.NoError(t, dao.UpdateItemsResult(ctx, f.space, f.expt, []int64{item}, map[string]any{"expt_run_id": f.run + 1}))
			case "old_token":
				_, err := f.port.RequeueTerminalItems(ctx, f.request(entity.EvaluationModeFailRetry))
				require.NoError(t, err)
				before := f.snapshot()
				changed, err := dao.MarkProcessingIfCurrent(ctx, f.space, f.expt, f.run, 11, "rq0")
				require.NoError(t, err)
				require.False(t, changed)
				require.Equal(t, before, f.snapshot())
			case "version_mismatch":
				bad = true
				require.NoError(t, dao.UpdateItemsResult(ctx, f.space, f.expt, []int64{item}, map[string]any{"item_version_id": f.base + 99}))
			case "bad_json":
				bad = true
				require.NoError(t, dao.UpdateItemsResult(ctx, f.space, f.expt, []int64{item}, map[string]any{"ext": []byte(`{`)}))
			case "unknown_canonical":
				bad = true
				require.NoError(t, dao.UpdateItemsResult(ctx, f.space, f.expt, []int64{item}, map[string]any{"status": 9}))
			case "unknown_log":
				bad = true
				require.NoError(t, dao.UpdateItemRunLog(ctx, f.expt, f.run, []int64{item}, map[string]any{"status": 9}, f.space))
			case "null_context":
				bad = true
				require.NoError(t, dao.UpdateItemsResult(ctx, f.space, f.expt, []int64{item}, map[string]any{"ext": []byte(`{"manual_retry":"null"}`)}))
			case "missing_run":
				bad = true
				require.NoError(t, f.conn.Where("space_id=? AND expt_id=?", f.space, f.expt).Delete(&model.ExptRunLog{}).Error)
			}
			before := f.snapshot()
			rows, err := dao.ScanCurrentRequests(ctx, f.space, f.expt, f.run, false)
			if bad {
				require.Error(t, err)
				require.Nil(t, rows)
			} else {
				require.NoError(t, err)
				if kind != "old_token" {
					for _, row := range rows {
						require.NotEqual(t, item, row.ItemID)
					}
				}
			}
			require.Equal(t, before, f.snapshot())
		})
	}
}
