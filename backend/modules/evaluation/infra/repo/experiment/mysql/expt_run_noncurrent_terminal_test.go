// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package mysql

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

func withoutOwnRunCompletion(t *testing.T, snapshot map[string]string, f *manualRetryFixture) map[string]string {
	t.Helper()
	result := make(map[string]string, len(snapshot))
	for table, data := range snapshot {
		result[table] = data
	}
	rows, err := f.conn.Raw("SELECT * FROM expt_run_log WHERE 1=0").Rows()
	require.NoError(t, err)
	columns, err := rows.Columns()
	require.NoError(t, err)
	require.NoError(t, rows.Close())
	indexes := make(map[string]int, len(columns))
	for i, name := range columns {
		indexes[name] = i
	}
	idIndex, ok := indexes["expt_run_id"]
	require.True(t, ok)
	var runs [][]string
	require.NoError(t, json.Unmarshal([]byte(result["expt_run_log"]), &runs))
	for _, run := range runs {
		if run[idIndex] == strconv.FormatInt(f.run, 10) {
			for _, name := range []string{"status", "pending_cnt", "processing_cnt", "success_cnt", "fail_cnt", "terminated_cnt", "updated_at"} {
				index, ok := indexes[name]
				require.True(t, ok, name)
				run[index] = "<own completion field>"
			}
		}
	}
	data, err := json.Marshal(runs)
	require.NoError(t, err)
	result["expt_run_log"] = string(data)
	return result
}

func TestNonCurrentRunMySQLRequiresActualReplacement(t *testing.T) {
	for _, state := range []string{"actual", "zero_latest", "missing_latest", "foreign_latest", "missing_status", "already_terminal"} {
		t.Run(state, func(t *testing.T) {
			f := terminalFinishFixture(t)
			next := f.run + 100
			replacement := &model.ExptRunLog{ID: next, SpaceID: f.space, ExptID: f.expt, ExptRunID: next, Status: gptr.Of(int64(entity.ExptStatus_Pending)), Mode: gptr.Of(int32(entity.EvaluationModeRetryAll))}
			if state != "missing_latest" && state != "zero_latest" {
				if state == "foreign_latest" {
					replacement.ExptID = f.expt + 100
					t.Cleanup(func() { f.conn.Where("space_id=? AND expt_id=?", f.space, f.expt+100).Delete(&model.ExptRunLog{}) })
				}
				require.NoError(t, f.conn.Create(replacement).Error)
			}
			latest := next
			if state == "zero_latest" {
				latest = 0
			}
			require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).Update("latest_run_id", latest).Error)
			if state == "missing_status" {
				require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=? AND expt_run_id=?", f.space, f.run).Update("status", nil).Error)
			}
			if state == "already_terminal" {
				require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=? AND expt_run_id=?", f.space, f.run).Updates(map[string]any{"status": int64(entity.ExptStatus_Terminated), "status_message": []byte("preserve original terminal")}).Error)
			}
			before := f.snapshot()
			result, err := finishFixture(f)
			if state == "actual" {
				require.NoError(t, err)
				require.Equal(t, entity.ExptRunFinishNonCurrent, result.Disposition)
				var old, current model.ExptRunLog
				require.NoError(t, f.conn.Where("space_id=? AND expt_run_id=?", f.space, f.run).Take(&old).Error)
				require.Equal(t, int64(entity.ExptStatus_Failed), gptr.Indirect(old.Status))
				require.Equal(t, int32(1), old.FailCnt)
				require.Equal(t, int32(1), old.SuccessCnt)
				require.Zero(t, old.PendingCnt)
				require.Zero(t, old.ProcessingCnt)
				require.Zero(t, old.TerminatedCnt)
				require.NoError(t, f.conn.Where("space_id=? AND expt_run_id=?", f.space, next).Take(&current).Error)
				require.Equal(t, int64(entity.ExptStatus_Pending), gptr.Indirect(current.Status))
				require.Equal(t, withoutOwnRunCompletion(t, before, f), withoutOwnRunCompletion(t, f.snapshot(), f))
			} else if state == "already_terminal" {
				require.NoError(t, err)
				require.Equal(t, entity.ExptRunFinishNonCurrent, result.Disposition)
				require.Equal(t, before, f.snapshot())
			} else {
				require.Error(t, err)
				require.Nil(t, result)
				require.Equal(t, before, f.snapshot())
			}
		})
	}
}
