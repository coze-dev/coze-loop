// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package mysql

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCurrentRequestMySQLRollback(t *testing.T) {
	for _, table := range []string{"expt_item_result", "expt_item_result_run_log", "expt_turn_result", "expt_turn_result_run_log", "expt_stats"} {
		t.Run(table, func(t *testing.T) {
			f := newManualRetryFixture(t)
			_, err := f.port.RequeueTerminalItems(context.Background(), f.request(entity.EvaluationModeFailRetry))
			require.NoError(t, err)
			before := f.snapshot()
			hits := 0
			name := "processing_fault_" + table
			require.NoError(t, f.conn.Callback().Update().Before("gorm:update").Register(name, func(tx *gorm.DB) {
				if tx.Statement.Table == table {
					_, same := tx.Statement.ConnPool.(*sql.Tx)
					require.True(t, same)
					hits++
					_ = tx.AddError(errors.New("injected processing failure"))
				}
			}))
			defer func() { require.NoError(t, f.conn.Callback().Update().Remove(name)) }()
			changed, err := NewExptItemResultDAO(f.provider).MarkProcessingIfCurrent(context.Background(), f.space, f.expt, f.run, 11, "rq1")
			require.Error(t, err)
			require.False(t, changed)
			require.Positive(t, hits)
			require.Equal(t, before, f.snapshot())
			t.Logf("REAL_MYSQL_PROCESSING_ROLLBACK table=%s snapshots=11", table)
		})
	}
}

func TestCurrentRequestMySQLScanMix(t *testing.T) {
	f := newManualRetryFixture(t)
	// Convert the failed item to queueing through the actual admission transaction.
	_, err := f.port.RequeueTerminalItems(context.Background(), f.request(entity.EvaluationModeFailRetry))
	require.NoError(t, err)
	ext, err := entity.MergeManualRetryContext(nil, &entity.ManualRetryContext{RunID: f.run, RequestID: "rq2", Mode: entity.EvaluationModeRetryItems})
	require.NoError(t, err)
	require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, 13).Update("ext", ext).Error)
	rows, err := NewExptItemResultDAO(f.provider).ScanCurrentRequests(context.Background(), f.space, f.expt, f.run, false)
	require.NoError(t, err)
	require.Len(t, rows, 3)
	got := map[int64]*entity.ExptItemCurrentRequest{}
	for _, r := range rows {
		got[r.ItemID] = r
	}
	require.Equal(t, "rq1", got[11].RequestID)
	require.Equal(t, entity.EvaluationModeFailRetry, got[11].Mode)
	require.Equal(t, int32(3), got[11].RetryTimes)
	require.Empty(t, got[12].RequestID)
	require.Equal(t, entity.EvaluationModeSubmit, got[12].Mode)
	require.Equal(t, "rq2", got[13].RequestID)
	require.Equal(t, entity.EvaluationModeRetryItems, got[13].Mode)
	require.Equal(t, entity.ItemRunState_Processing, got[12].Status)
	t.Log("REAL_MYSQL_CURRENT_SOURCE_MIX empty/rq1/rq2 modes and retry_times verified")
}

func TestCurrentRequestMySQLZeroWrite(t *testing.T) {
	for _, kind := range []string{"old_token", "empty_token", "old_run", "missing", "soft_item", "soft_log", "terminal", "null_context", "bad_json", "foreign_run", "missing_turn", "unknown_item", "unknown_log", "bad_turn", "missing_stats"} {
		t.Run(kind, func(t *testing.T) {
			f := newManualRetryFixture(t)
			_, err := f.port.RequeueTerminalItems(context.Background(), f.request(entity.EvaluationModeFailRetry))
			require.NoError(t, err)
			run, token := f.run, "rq1"
			wantErr := false
			q := f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, 11)
			switch kind {
			case "old_token":
				token = "rq0"
			case "empty_token":
				token = ""
			case "old_run":
				run++
			case "missing":
				require.NoError(t, q.Unscoped().Delete(&model.ExptItemResult{}).Error)
			case "soft_item":
				require.NoError(t, q.Delete(&model.ExptItemResult{}).Error)
			case "soft_log":
				require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, 11).Delete(&model.ExptItemResultRunLog{}).Error)
			case "terminal":
				require.NoError(t, q.Update("status", int32(entity.ItemRunState_Success)).Error)
			case "null_context":
				require.NoError(t, q.Update("ext", []byte(`{"manual_retry":"null"}`)).Error)
				wantErr = true
			case "bad_json":
				require.NoError(t, q.Update("ext", []byte(`{"manual_retry":`)).Error)
				wantErr = true
			case "foreign_run":
				require.NoError(t, q.Update("expt_run_id", f.run+1).Error)
			case "missing_turn":
				require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND item_id=? AND turn_id=?", f.space, f.expt, 11, 21).Delete(&model.ExptTurnResultRunLog{}).Error)
				wantErr = true
			case "unknown_item":
				require.NoError(t, q.Update("status", 9).Error)
				wantErr = true
			case "unknown_log":
				require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, 11).Update("status", 9).Error)
				wantErr = true
			case "bad_turn":
				require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND expt_id=? AND item_id=? AND turn_id=?", f.space, f.expt, 11, 21).Update("evaluator_result_ids", []byte(`{"registered":`)).Error)
				wantErr = true
			case "missing_stats":
				require.NoError(t, f.conn.Where("space_id=? AND expt_id=?", f.space, f.expt).Delete(&model.ExptStats{}).Error)
				wantErr = true
			}
			before := f.snapshot()
			changed, err := NewExptItemResultDAO(f.provider).MarkProcessingIfCurrent(context.Background(), f.space, f.expt, run, 11, token)
			if wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.False(t, changed)
			require.Equal(t, before, f.snapshot())
		})
	}
}

func TestCurrentRequestMySQLSuccessZeroScore(t *testing.T) {
	f := newManualRetryFixture(t)
	_, err := f.port.RequeueTerminalItems(context.Background(), f.request(entity.EvaluationModeFailRetry))
	require.NoError(t, err)
	var before model.ExptTurnResult
	require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND turn_id=?", f.space, f.expt, 22).Take(&before).Error)
	require.Equal(t, 0.0, gptr.Indirect(before.WeightedScore))
	_, err = NewExptItemResultDAO(f.provider).MarkProcessingIfCurrent(context.Background(), f.space, f.expt, f.run, 11, "rq1")
	require.NoError(t, err)
	var after model.ExptTurnResult
	require.NoError(t, f.conn.Where("id=?", before.ID).Take(&after).Error)
	require.Equal(t, before, after)
}

func TestCurrentRequestMySQLConcurrentAndLateCAS(t *testing.T) {
	f := newManualRetryFixture(t)
	_, err := f.port.RequeueTerminalItems(context.Background(), f.request(entity.EvaluationModeFailRetry))
	require.NoError(t, err)
	port := NewExptItemResultDAO(f.provider)
	var wg sync.WaitGroup
	changes := make(chan bool, 2)
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			changed, err := port.MarkProcessingIfCurrent(context.Background(), f.space, f.expt, f.run, 11, "rq1")
			changes <- changed
			errs <- err
		}()
	}
	wg.Wait()
	close(changes)
	close(errs)
	count := 0
	for changed := range changes {
		if changed {
			count++
		}
	}
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, 1, count)
	var stats model.ExptStats
	require.NoError(t, f.conn.Where("space_id=? AND expt_id=?", f.space, f.expt).Take(&stats).Error)
	require.Equal(t, int32(1), stats.PendingCnt)
	require.Equal(t, int32(2), stats.ProcessingCnt)
	require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, 11).Update("status", int32(entity.ItemRunState_Success)).Error)
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, 11).Updates(map[string]any{"status": int32(entity.ItemRunState_Success), "result_state": int32(entity.ExptItemResultStateResulted)}).Error)
	before := f.snapshot()
	changed, err := port.MarkProcessingIfCurrent(context.Background(), f.space, f.expt, f.run, 11, "rq1")
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, before, f.snapshot())
	t.Log("REAL_MYSQL_CONCURRENT_CAS stats moved once; late producer after terminal zero writes")
}

func TestCurrentRequestMySQLOriginalEmptyToken(t *testing.T) {
	f := newManualRetryFixture(t)
	port := NewExptItemResultDAO(f.provider)
	before := f.snapshot()
	changed, err := port.MarkProcessingIfCurrent(context.Background(), f.space, f.expt, f.run, 13, "rq1")
	require.NoError(t, err)
	require.False(t, changed)
	require.Equal(t, before, f.snapshot())
	changed, err = port.MarkProcessingIfCurrent(context.Background(), f.space, f.expt, f.run, 13, "")
	require.NoError(t, err)
	require.True(t, changed)
	var log model.ExptItemResultRunLog
	require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, 13).Take(&log).Error)
	require.Equal(t, int32(3), log.RetryTimes)
}

func TestCurrentRequestMySQLScanRejectsBadSource(t *testing.T) {
	for _, kind := range []string{"bad_json", "null_context", "status", "mode", "run_deleted", "foreign_manual_run"} {
		t.Run(kind, func(t *testing.T) {
			f := newManualRetryFixture(t)
			q := f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, 13)
			switch kind {
			case "bad_json":
				require.NoError(t, q.Update("ext", []byte(`{`)).Error)
			case "null_context":
				require.NoError(t, q.Update("ext", []byte(`{"manual_retry":"null"}`)).Error)
			case "status":
				require.NoError(t, q.Update("status", 9).Error)
			case "mode":
				require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=? AND expt_id=?", f.space, f.expt).Update("mode", 0).Error)
			case "run_deleted":
				require.NoError(t, f.conn.Where("space_id=? AND expt_id=?", f.space, f.expt).Delete(&model.ExptRunLog{}).Error)
			case "foreign_manual_run":
				ext, err := entity.MergeManualRetryContext(nil, &entity.ManualRetryContext{RunID: f.run + 1, RequestID: "rq2", Mode: entity.EvaluationModeRetryItems})
				require.NoError(t, err)
				require.NoError(t, q.Update("ext", ext).Error)
			}
			before := f.snapshot()
			rows, err := NewExptItemResultDAO(f.provider).ScanCurrentRequests(context.Background(), f.space, f.expt, f.run, false)
			require.Error(t, err)
			require.Nil(t, rows)
			require.Equal(t, before, f.snapshot())
		})
	}
}

func TestCurrentRequestMySQLCASRejectsCorruptState(t *testing.T) {
	for _, kind := range []string{"retry_times", "result_state", "success_turn_mismatch", "unknown_turn", "manual_run_missing"} {
		t.Run(kind, func(t *testing.T) {
			f := newManualRetryFixture(t)
			_, err := f.port.RequeueTerminalItems(context.Background(), f.request(entity.EvaluationModeFailRetry))
			require.NoError(t, err)
			switch kind {
			case "retry_times":
				require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, 11).Update("retry_times", int64(2147483648)).Error)
			case "result_state":
				require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, 11).Update("result_state", 9).Error)
			case "success_turn_mismatch":
				require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND expt_id=? AND turn_id=?", f.space, f.expt, 22).Update("status", int32(entity.TurnRunState_Queueing)).Error)
			case "unknown_turn":
				require.NoError(t, f.conn.Model(&model.ExptTurnResult{}).Where("space_id=? AND expt_id=? AND turn_id=?", f.space, f.expt, 21).Update("status", 9).Error)
			case "manual_run_missing":
				require.NoError(t, f.conn.Where("space_id=? AND expt_id=?", f.space, f.expt).Delete(&model.ExptRunLog{}).Error)
			}
			before := f.snapshot()
			changed, err := NewExptItemResultDAO(f.provider).MarkProcessingIfCurrent(context.Background(), f.space, f.expt, f.run, 11, "rq1")
			require.Error(t, err)
			require.False(t, changed)
			require.Equal(t, before, f.snapshot())
		})
	}
}

func TestCurrentRequestMySQLMissingLogIsStorageError(t *testing.T) {
	f := newManualRetryFixture(t)
	_, err := f.port.RequeueTerminalItems(context.Background(), f.request(entity.EvaluationModeFailRetry))
	require.NoError(t, err)
	require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, 11).Delete(&model.ExptItemResultRunLog{}).Error)
	before := f.snapshot()
	source, err := NewExptItemResultDAO(f.provider).GetCurrentRequest(context.Background(), f.space, f.expt, f.run, 11)
	require.Error(t, err)
	require.Nil(t, source)
	require.Equal(t, before, f.snapshot())
}
