// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func TestHookSummaryDatabaseErrorDoesNotBecomeLegacy(t *testing.T) {
	for _, table := range []string{"expt_run_log", "expt_lifecycle_run"} {
		t.Run(table, func(t *testing.T) {
			f := newHookTxFixture(t)
			in := f.input(true, 0)
			_, err := f.repo.CreateRunWithHooks(context.Background(), in)
			require.NoError(t, err)
			name := fmt.Sprint("summary_db_failure_", f.expt)
			require.NoError(t, f.sql.Callback().Query().Before("gorm:query").Register(name, func(tx *gorm.DB) {
				if tx.Statement.Table == "l" && strings.Contains(fmt.Sprint(tx.Statement.TableExpr), table) {
					tx.Statement.Selects = []string{"secret_missing_summary_column"}
				}
			}))
			t.Cleanup(func() { require.NoError(t, f.sql.Callback().Query().Remove(name)) })
			out, err := NewHookSummaryRepo(f.p).MGetSummaries(context.Background(), []entity.HookRunKey{in.Key})
			require.ErrorIs(t, err, entity.ErrHookSummaryUnavailable)
			require.Nil(t, out)
			require.NotContains(t, err.Error(), "secret_missing_summary_column")
		})
	}
}

func TestHookSummaryReadsOneSnapshotWithoutRowLocks(t *testing.T) {
	f := newHookTxFixture(t)
	in := f.input(true, 0)
	_, err := f.repo.CreateRunWithHooks(context.Background(), in)
	require.NoError(t, err)
	lock := f.sql.Begin()
	require.NoError(t, lock.Error)
	defer lock.Rollback()
	require.NoError(t, lock.Exec("SELECT id FROM expt_lifecycle_hook_run WHERE operation_id=? FOR UPDATE", in.Before.OperationID).Error)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	out, err := NewHookSummaryRepo(f.p).MGetSummaries(ctx, []entity.HookRunKey{in.Key})
	require.NoError(t, err)
	require.Equal(t, entity.HookOperationPending, out[in.Key].Before.Status)
	require.NoError(t, lock.Rollback().Error)
	const callback = "summary_concurrent_commit"
	changed := false
	require.NoError(t, f.sql.Callback().Query().After("gorm:query").Register(callback, func(tx *gorm.DB) {
		if !changed && strings.Contains(tx.Statement.SQL.String(), "expt_run_log") {
			changed = true
			require.NoError(t, f.sql.Model(&model.ExptLifecycleHookRun{}).Where("operation_id=?", in.Before.OperationID).UpdateColumns(map[string]any{"status": "succeeded", "attempt": 1, "result_redacted": []byte(`{"v":"new"}`)}).Error)
		}
	}))
	t.Cleanup(func() { require.NoError(t, f.sql.Callback().Query().Remove(callback)) })
	out = hookSummaryRead(t, f, in.Key)
	require.True(t, changed)
	require.Equal(t, entity.HookOperationPending, out[in.Key].Before.Status)
	next := hookSummaryRead(t, f, in.Key)
	require.Equal(t, entity.HookOperationSucceeded, next[in.Key].Before.Status)
}

func TestHookSummaryMissingRunFailsEntireMixedBatch(t *testing.T) {
	f := newHookTxFixture(t)
	in := f.input(true, 0)
	_, err := f.repo.CreateRunWithHooks(context.Background(), in)
	require.NoError(t, err)
	legacy := f.input(false, 0)
	require.NoError(t, f.sql.Create(&model.ExptRunLog{ID: legacy.Key.RunID, ExptRunID: legacy.Key.RunID, SpaceID: f.space, ExptID: f.expt}).Error)
	out := hookSummaryRead(t, f, in.Key, legacy.Key)
	require.NotNil(t, out[in.Key])
	require.Nil(t, out[legacy.Key])
	missing := entity.HookRunKey{WorkspaceID: f.space, ExperimentID: f.expt, RunID: hookTxSequence.Add(1)}
	got, err := NewHookSummaryRepo(f.p).MGetSummaries(context.Background(), []entity.HookRunKey{in.Key, legacy.Key, missing})
	require.ErrorIs(t, err, entity.ErrHookSummaryUnavailable)
	require.Nil(t, got)
}

func TestHookSummaryFrozenDisabledRejectsExtraOperation(t *testing.T) {
	f := newHookTxFixture(t)
	in := f.input(false, 0)
	_, err := f.repo.CreateRunWithHooks(context.Background(), in)
	require.NoError(t, err)
	before := hookSummaryRead(t, f, in.Key)[in.Key].Before
	require.Equal(t, entity.HookOperationDisabled, before.Status)
	row := hookAttemptRow(t, f, in.After.OperationID)
	row.ID = hookTxSequence.Add(1)
	row.Phase = "before"
	row.OperationID = fmt.Sprint("extra_", row.ID)
	row.IdempotencyKey = row.OperationID
	require.NoError(t, f.sql.Create(&row).Error)
	out, err := NewHookSummaryRepo(f.p).MGetSummaries(context.Background(), []entity.HookRunKey{in.Key})
	require.ErrorIs(t, err, entity.ErrHookSummaryUnavailable)
	require.Nil(t, out)
}

func TestHookSummaryRejectsInvalidLifecycleStates(t *testing.T) {
	for _, field := range []string{"gate", "plan_state", "finalize_state"} {
		t.Run(field, func(t *testing.T) {
			f := newHookTxFixture(t)
			in := f.input(true, 0)
			_, err := f.repo.CreateRunWithHooks(context.Background(), in)
			require.NoError(t, err)
			require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, in.Key.RunID).UpdateColumn(field, 9).Error)
			out, err := NewHookSummaryRepo(f.p).MGetSummaries(context.Background(), []entity.HookRunKey{in.Key})
			require.ErrorIs(t, err, entity.ErrHookSummaryUnavailable)
			require.Nil(t, out)
		})
	}
}

func TestHookSummaryRejectsInconsistentLifecycle(t *testing.T) {
	for _, name := range []string{"running without lease", "ready with pending before", "premature after", "invalid run status", "missing terminal time"} {
		t.Run(name, func(t *testing.T) {
			f := newHookTxFixture(t)
			in := f.input(true, 0)
			_, err := f.repo.CreateRunWithHooks(context.Background(), in)
			require.NoError(t, err)
			switch name {
			case "running without lease":
				require.NoError(t, f.sql.Model(&model.ExptLifecycleHookRun{}).Where("operation_id=?", in.Before.OperationID).UpdateColumns(map[string]any{"status": "running", "attempt": 1}).Error)
			case "ready with pending before":
				require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, in.Key.RunID).UpdateColumn("gate", 1).Error)
			case "premature after":
				require.NoError(t, f.sql.Model(&model.ExptLifecycleHookRun{}).Where("operation_id=?", in.After.OperationID).UpdateColumn("activated_at", time.Now()).Error)
			case "invalid run status":
				require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", in.Key.RunID).UpdateColumn("status", 99).Error)
			case "missing terminal time":
				require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, in.Key.RunID).UpdateColumn("finalize_state", 1).Error)
			}
			out, err := NewHookSummaryRepo(f.p).MGetSummaries(context.Background(), []entity.HookRunKey{in.Key})
			require.ErrorIs(t, err, entity.ErrHookSummaryUnavailable)
			require.Nil(t, out)
		})
	}
}
