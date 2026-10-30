// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func TestManualRetryMySQLAbsentCurrentRun(t *testing.T) {
	f := newManualRetryFixture(t)
	oldRun := f.run
	newRun := f.base + 999
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("id=?", f.expt).Update("latest_run_id", newRun).Error)
	require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, 11).Update("status", 2).Error)
	require.NoError(t, f.conn.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", f.space, f.expt).Updates(map[string]any{"fail_cnt": 0, "success_cnt": 1}).Error)
	require.NoError(t, f.conn.Create(&model.ExptRunLog{ID: newRun, SpaceID: f.space, ExptID: f.expt, ExptRunID: newRun, Status: gptr.Of(int64(entity.ExptStatus_Processing)), Mode: gptr.Of(int32(entity.EvaluationModeRetryItems)), ItemIds: gptr.Of([]byte(`[{"ItemIDs":[12,13],"CreateAt":1700000000}]`))}).Error)
	require.NoError(t, f.conn.Exec("INSERT INTO expt_item_result_run_log (id,space_id,expt_id,expt_run_id,item_id,item_version_id,status) SELECT id+1000,space_id,expt_id,?,item_id,item_version_id,status FROM expt_item_result_run_log WHERE space_id=? AND expt_id=? AND item_id IN (12,13)", newRun, f.space, f.expt).Error)
	req := f.request(entity.EvaluationModeRetryItems)
	req.RunID = newRun
	req.Items[0].CurrentItemLog = nil
	for _, turn := range req.Items[0].Turns {
		turn.CurrentRunLog = nil
	}
	before := f.snapshot()
	receipt, err := f.port.RequeueTerminalItems(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, int32(1), receipt.AcceptedCount)
	var item model.ExptItemResult
	require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, 11).First(&item).Error)
	require.Equal(t, req.Items[0].SourceItem.ID, item.ID)
	require.Equal(t, newRun, item.ExptRunID)
	var itemLog model.ExptItemResultRunLog
	require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", f.space, f.expt, newRun, 11).First(&itemLog).Error)
	require.Equal(t, f.base+111, itemLog.ItemVersionID)
	require.Zero(t, itemLog.RetryTimes)
	require.Zero(t, *itemLog.ResultState)
	var logs []*model.ExptTurnResultRunLog
	require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, newRun).Order("turn_id").Find(&logs).Error)
	require.Len(t, logs, 2)
	for _, log := range logs {
		require.Equal(t, f.base+111, log.ItemVersionID)
		require.NotEqual(t, req.Items[0].Turns[0].SourceTurn.ID, log.ID)
		require.Zero(t, log.Status)
		require.Nil(t, log.ErrMsg)
		require.JSONEq(t, `{"other":"keep"}`, string(log.Ext), "new run logs retain unrelated frozen source Ext")
	}
	var run model.ExptRunLog
	require.NoError(t, f.conn.Where("id=?", newRun).First(&run).Error)
	var chunks []entity.ExptRunLogItems
	require.NoError(t, json.Unmarshal(*run.ItemIds, &chunks))
	require.Equal(t, []int64{12, 13, 11}, (&entity.ExptRunLog{ItemIds: chunks}).GetItemIDs())
	require.Equal(t, int32(2), run.PendingCnt)
	require.Equal(t, int32(1), run.ProcessingCnt)
	var oldItemLog model.ExptItemResultRunLog
	require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", f.space, f.expt, oldRun, 11).First(&oldItemLog).Error)
	require.Equal(t, int32(3), oldItemLog.Status)
	require.Equal(t, int32(3), oldItemLog.RetryTimes)
	require.Equal(t, []byte("old-run-error"), *oldItemLog.ErrMsg)
	var oldTurnLogs []*model.ExptTurnResultRunLog
	require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, oldRun).Find(&oldTurnLogs).Error)
	require.Len(t, oldTurnLogs, 2)
	for _, log := range oldTurnLogs {
		require.Equal(t, "turn-run-log", log.LogID)
	}
	require.Equal(t, before["experiment"], f.snapshot()["experiment"])
	first := f.snapshot()
	_, err = f.port.RequeueTerminalItems(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, first, f.snapshot())
	f.run = newRun
	require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("id=?", item.ID).Update("status", 2).Error)
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("id=?", itemLog.ID).Updates(map[string]any{"status": 2, "result_state": 1}).Error)
	require.NoError(t, f.conn.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", f.space, f.expt).Updates(map[string]any{"pending_cnt": 1, "success_cnt": 1}).Error)
	again := f.request(entity.EvaluationModeRetryItems)
	again.RequestID = "rq2"
	again.PayloadHash = "hash2"
	_, err = f.port.RequeueTerminalItems(context.Background(), again)
	require.NoError(t, err)
	require.NoError(t, f.conn.Where("id=?", newRun).First(&run).Error)
	require.NoError(t, json.Unmarshal(*run.ItemIds, &chunks))
	require.Equal(t, []int64{12, 13, 11}, (&entity.ExptRunLog{ItemIds: chunks}).GetItemIDs())
	require.Len(t, chunks, 4)
	t.Log("REAL_MYSQL_ABSENT_LOGS_VERIFIED all frozen turns/version R1 unchanged R2 total+1 repeated business IDs once")
}

func TestManualRetryMySQLAbsentInsertRollback(t *testing.T) {
	for _, table := range []string{"expt_item_result_run_log", "expt_turn_result_run_log"} {
		t.Run(table, func(t *testing.T) {
			f := newManualRetryFixture(t)
			req := f.request(entity.EvaluationModeRetryItems)
			newRun := f.base + 999
			require.NoError(t, f.conn.Model(&model.Experiment{}).Where("id=?", f.expt).Update("latest_run_id", newRun).Error)
			require.NoError(t, f.conn.Create(&model.ExptRunLog{ID: newRun, SpaceID: f.space, ExptID: f.expt, ExptRunID: newRun, Status: gptr.Of(int64(entity.ExptStatus_Processing)), ItemIds: gptr.Of([]byte(`[]`))}).Error)
			req.RunID = newRun
			req.Items[0].CurrentItemLog = nil
			for _, turn := range req.Items[0].Turns {
				turn.CurrentRunLog = nil
			}
			before := f.snapshot()
			hits := 0
			name := "fault_insert_retry"
			require.NoError(t, f.conn.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
				if tx.Statement.Table == table {
					hits++
					_ = tx.AddError(errors.New("injected insert failure"))
				}
			}))
			defer func() { require.NoError(t, f.conn.Callback().Create().Remove(name)) }()
			_, err := f.port.RequeueTerminalItems(context.Background(), req)
			require.ErrorContains(t, err, "injected insert failure")
			require.Positive(t, hits)
			require.Equal(t, before, f.snapshot())
			t.Logf("REAL_MYSQL_INSERT_ROLLBACK_VERIFIED table=%s snapshots=11", table)
		})
	}
}
