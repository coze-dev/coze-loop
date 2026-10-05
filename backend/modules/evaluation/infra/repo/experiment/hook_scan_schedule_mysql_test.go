// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"testing"
)

// Requires the parent's isolated scan DB with the production expt_run_log schema.
func TestHookScheduleScanMySQLRecoveryAndStop(t *testing.T) {
	f := newHookScanFixture(t)
	r := NewHookScheduleScanRepo(f.repo.(*hookRunRepo).provider)
	var lives []model.ExptLifecycleRun
	var logs []model.ExptRunLog
	for i := int64(1); i <= 105; i++ {
		row := f.run(1, i)
		row.PlanState = 1
		row.Gate = 1
		row.ExecutionStarted = i <= 101
		lives = append(lives, row)
		logs = append(logs, model.ExptRunLog{ID: row.ExptRunID, SpaceID: row.SpaceID, ExptID: row.ExptID, ExptRunID: row.ExptRunID, CreatedBy: "schedule-user", Mode: gptr.Of(int32(entity.EvaluationModeSubmit)), Status: gptr.Of(int64(entity.ExptStatus_Pending)), LifecycleHookVersion: gptr.Of(int32(1))})
	}
	lives[102].Gate = 2
	logs[103].Status = gptr.Of(int64(entity.ExptStatus_Processing))
	logs[104].Mode = gptr.Of(int32(entity.EvaluationModeTrialRun))
	t.Cleanup(func() {
		require.NoError(t, f.sql.Unscoped().Where("space_id = ?", f.base+1).Delete(&model.ExptRunLog{}).Error)
	})
	require.NoError(t, f.sql.Create(&lives).Error)
	require.NoError(t, f.sql.Create(&logs).Error)
	in := f.input("", 100)
	first, err := r.ScanPreparingPlans(context.Background(), in)
	require.NoError(t, err)
	require.Empty(t, first.Candidates)
	require.True(t, first.HasMore)
	require.Equal(t, f.base+100, first.NextCursor.RunID)
	in.Cursor = first.NextCursor
	second, err := r.ScanPreparingPlans(context.Background(), in)
	require.NoError(t, err)
	require.Len(t, second.Candidates, 2)
	require.False(t, second.HasMore)
	require.Equal(t, f.base+102, second.Candidates[0].Key.RunID)
	require.Equal(t, f.base+105, second.Candidates[1].Key.RunID)
	// A successful MQ send does not alter eligibility; actual startup does.
	again, err := r.ScanPreparingPlans(context.Background(), in)
	require.NoError(t, err)
	require.Equal(t, second.Candidates, again.Candidates)
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id = ? AND expt_run_id = ?", f.base+1, f.base+102).Update("execution_started", true).Error)
	after, err := r.ScanPreparingPlans(context.Background(), in)
	require.NoError(t, err)
	require.Len(t, after.Candidates, 1)
	require.Equal(t, f.base+105, after.Candidates[0].Key.RunID)

	dry := hookScheduleReadyScanQuery(f.sql.Session(&gorm.Session{DryRun: true}), in).Find(&[]hookScheduleScanRow{})
	var plan []struct{ Table, Key, Type, Extra string }
	require.NoError(t, f.sql.Raw("EXPLAIN "+dry.Statement.SQL.String(), dry.Statement.Vars...).Scan(&plan).Error)
	found := false
	for _, row := range plan {
		t.Logf("schedule scan EXPLAIN: %+v", row)
		if row.Table == "life" {
			found = true
			require.Equal(t, "idx_scope_plan_run", row.Key)
			require.NotContains(t, row.Extra, "filesort")
		}
		if row.Table == "log" {
			require.Equal(t, "PRIMARY", row.Key)
		}
	}
	require.True(t, found)
}
