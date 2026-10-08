// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func hookSummaryRead(t *testing.T, f *hookTxFixture, keys ...entity.HookRunKey) map[entity.HookRunKey]*entity.LifecycleHookRunSummary {
	t.Helper()
	out, err := NewHookSummaryRepo(f.p).MGetSummaries(context.Background(), keys)
	require.NoError(t, err)
	return out
}

func hookSummaryQueries(t *testing.T, f *hookTxFixture) *[]string {
	t.Helper()
	var queries []string
	name := fmt.Sprintf("summary_queries_%d", f.expt)
	require.NoError(t, f.sql.Callback().Query().After("gorm:query").Register(name, func(tx *gorm.DB) {
		queries = append(queries, strings.ToLower(tx.Statement.SQL.String()))
	}))
	t.Cleanup(func() { require.NoError(t, f.sql.Callback().Query().Remove(name)) })
	return &queries
}

func TestHookSummaryPendingFixedRun(t *testing.T) {
	f := newHookTxFixture(t)
	old := f.input(true, 0)
	_, err := f.repo.CreateRunWithHooks(context.Background(), old)
	require.NoError(t, err)
	newer := f.input(false, old.Key.RunID)
	_, err = f.repo.CreateRunWithHooks(context.Background(), newer)
	require.NoError(t, err)
	queries := hookSummaryQueries(t, f)
	out := hookSummaryRead(t, f, old.Key, newer.Key, old.Key)
	require.Len(t, out, 2)
	require.Equal(t, old.Key.RunID, out[old.Key].RunID)
	require.Equal(t, old.Before.OperationID, out[old.Key].Before.OperationID)
	require.Equal(t, entity.HookOperationPending, out[old.Key].Before.Status)
	require.Equal(t, entity.HookOperationDisabled, out[newer.Key].Before.Status)
	require.Nil(t, out[newer.Key].Before.UpdatedAt)
	require.Equal(t, newer.After.OperationID, out[newer.Key].After.OperationID)
	require.NotNil(t, out[old.Key].Before.UpdatedAt)
	require.Nil(t, out[old.Key].Before.Response)
	require.Nil(t, out[old.Key].Before.Error)
	require.Len(t, *queries, 3)
	for _, query := range *queries {
		for _, forbidden := range []string{"snapshot", "lifecycle_hook_conf", "created_by", "lease_owner", "idempotency_key", "request_hash", "latest_run_id", "for update", "select *"} {
			require.NotContains(t, query, forbidden)
		}
	}
}

func TestHookSummaryOperationTimestampColumn(t *testing.T) {
	f := newHookTxFixture(t)
	var count int64
	require.NoError(t, f.sql.Raw("SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='expt_lifecycle_hook_run' AND column_name='updated_at' AND data_type='datetime' AND datetime_precision=3").Scan(&count).Error)
	require.Equal(t, int64(1), count, "operation update time must be persisted at millisecond precision")
}

func TestHookSummaryAllStatesAndResultVisibility(t *testing.T) {
	for _, status := range []string{"pending", "running", "retry_wait", "succeeded", "failed"} {
		t.Run(status, func(t *testing.T) {
			f := newHookTxFixture(t)
			in := f.input(true, 0)
			_, err := f.repo.CreateRunWithHooks(context.Background(), in)
			require.NoError(t, err)
			attempt := 1
			if status == "pending" {
				attempt = 0
			}
			now, err := hookDBNow(f.sql)
			require.NoError(t, err)
			require.NoError(t, f.sql.Model(&model.ExptLifecycleHookRun{}).Where("operation_id=?", in.Before.OperationID).UpdateColumns(map[string]any{
				"status": status, "attempt": attempt, "result_redacted": []byte(`{"action":"skipped","literal":"{\"x\":1}"}`), "error_code": "SAFE_ERROR", "error_message": "safe failure",
				"activated_at": now, "lease_generation": 1, "lease_until": now.Add(30 * time.Second), "attempt_deadline": now.Add(10 * time.Second), "operation_deadline": now.Add(time.Minute),
			}).Error)
			got := hookSummaryRead(t, f, in.Key)[in.Key].Before
			require.Equal(t, entity.HookOperationStatus(status), got.Status)
			switch status {
			case "succeeded":
				require.Nil(t, got.Error)
				require.NotNil(t, got.Response)
				require.Equal(t, "succeeded", got.Response.GetStatus())
				require.Equal(t, map[string]string{"action": "skipped", "literal": `{"x":1}`}, got.Response.GetResult_())
				require.Nil(t, got.Response.Error)
			case "failed":
				require.Nil(t, got.Response)
				require.Equal(t, "SAFE_ERROR", got.Error.GetCode())
				require.Equal(t, "safe failure", got.Error.GetMessage())
				require.False(t, got.Error.GetRetryable())
			default:
				require.Nil(t, got.Response)
				require.Nil(t, got.Error)
				// Corrupt leftovers are neither selected nor decoded while in flight.
				require.NoError(t, f.sql.Model(&model.ExptLifecycleHookRun{}).Where("operation_id=?", in.Before.OperationID).UpdateColumns(map[string]any{"result_redacted": []byte("private-not-json"), "error_message": strings.Repeat("私", 1000)}).Error)
				again := hookSummaryRead(t, f, in.Key)[in.Key].Before
				require.Nil(t, again.Response)
				require.Nil(t, again.Error)
			}
		})
	}
}

func TestHookSummaryOldAfterSurvivesSoftDelete(t *testing.T) {
	f := newHookTxFixture(t)
	in := hookAttemptInput(t, f, false)
	claim, err := f.repo.ClaimAttempt(context.Background(), in)
	require.NoError(t, err)
	_, err = f.repo.CompleteAttempt(context.Background(), hookCompletion(t, f, in, claim.Claim))
	require.NoError(t, err)
	newer := f.input(true, in.Key.RunID)
	_, err = f.repo.CreateRunWithHooks(context.Background(), newer)
	require.NoError(t, err)
	require.NoError(t, f.sql.Delete(&model.Experiment{}, "id=?", f.expt).Error)
	out := hookSummaryRead(t, f, in.Key, newer.Key)
	require.Equal(t, entity.HookOperationSucceeded, out[in.Key].After.Status)
	require.Equal(t, map[string]string{"ok": "redacted"}, out[in.Key].After.Response.GetResult_())
	require.Equal(t, entity.HookOperationPending, out[newer.Key].After.Status)
	require.Nil(t, out[newer.Key].After.Response)
}

func TestHookSummaryLegacySkipsHookTables(t *testing.T) {
	f := newHookTxFixture(t)
	var keys []entity.HookRunKey
	for _, marker := range []*int32{nil, gptr.Of(int32(0))} {
		in := f.input(false, 0)
		keys = append(keys, in.Key)
		require.NoError(t, f.sql.Create(&model.ExptRunLog{ID: in.Key.RunID, ExptRunID: in.Key.RunID, SpaceID: f.space, ExptID: f.expt, LifecycleHookVersion: marker}).Error)
	}
	queries := hookSummaryQueries(t, f)
	name := fmt.Sprint("summary_no_hook_", f.expt)
	require.NoError(t, f.sql.Callback().Query().Before("gorm:query").Register(name, func(tx *gorm.DB) {
		if strings.Contains(fmt.Sprint(tx.Statement.TableExpr), "expt_lifecycle") {
			tx.Statement.Selects = []string{"hook_summary_nonexistent_column"}
		}
	}))
	t.Cleanup(func() { require.NoError(t, f.sql.Callback().Query().Remove(name)) })
	out := hookSummaryRead(t, f, keys...)
	require.Len(t, out, 2)
	for _, key := range keys {
		require.Nil(t, out[key])
	}
	require.Len(t, *queries, 1)
}

func TestHookSummaryInputValidationBeforeDB(t *testing.T) {
	valid := entity.HookRunKey{WorkspaceID: 1, ExperimentID: 2, RunID: 3}
	for _, keys := range [][]entity.HookRunKey{
		{{WorkspaceID: 0, ExperimentID: 2, RunID: 3}},
		{{WorkspaceID: 1, ExperimentID: -2, RunID: 3}},
		{{WorkspaceID: 1, ExperimentID: 2, RunID: 0}},
		{valid, {WorkspaceID: 2, ExperimentID: 2, RunID: 3}},
		make([]entity.HookRunKey, 101),
	} {
		out, err := NewHookSummaryRepo(nil).MGetSummaries(context.Background(), keys)
		require.Error(t, err)
		require.Nil(t, out)
	}
	out, err := NewHookSummaryRepo(nil).MGetSummaries(context.Background(), nil)
	require.NoError(t, err)
	require.Empty(t, out)
}

func TestHookSummaryFailsClosed(t *testing.T) {
	for _, mutation := range []string{"wrong space", "wrong experiment", "missing run", "marker", "missing life", "missing before", "missing after", "extra phase", "disabled row", "illegal status", "negative attempt", "too many attempts", "operation owner", "lifecycle owner", "scope", "both disabled", "missing experiment"} {
		t.Run(mutation, func(t *testing.T) {
			f := newHookTxFixture(t)
			in := f.input(true, 0)
			_, err := f.repo.CreateRunWithHooks(context.Background(), in)
			require.NoError(t, err)
			key := in.Key
			op := f.sql.Model(&model.ExptLifecycleHookRun{}).Where("operation_id=?", in.Before.OperationID)
			life := f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, key.RunID)
			switch mutation {
			case "wrong space":
				key.WorkspaceID++
			case "wrong experiment":
				key.ExperimentID++
			case "missing run":
				key.RunID++
			case "marker":
				require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", key.RunID).UpdateColumn("lifecycle_hook_version", 2).Error)
			case "missing life":
				require.NoError(t, life.Delete(&model.ExptLifecycleRun{}).Error)
			case "missing before":
				require.NoError(t, op.Delete(&model.ExptLifecycleHookRun{}).Error)
			case "missing after":
				require.NoError(t, f.sql.Delete(&model.ExptLifecycleHookRun{}, "operation_id=?", in.After.OperationID).Error)
			case "extra phase":
				require.NoError(t, op.UpdateColumn("phase", "unknown").Error)
			case "disabled row":
				require.NoError(t, op.UpdateColumn("status", "disabled").Error)
			case "illegal status":
				require.NoError(t, op.UpdateColumn("status", "secret-status").Error)
			case "negative attempt":
				require.NoError(t, op.UpdateColumn("attempt", -1).Error)
			case "too many attempts":
				require.NoError(t, op.UpdateColumn("attempt", 12).Error)
			case "operation owner":
				require.NoError(t, op.UpdateColumn("expt_id", f.expt+1).Error)
				t.Cleanup(func() {
					require.NoError(t, f.sql.Model(&model.ExptLifecycleHookRun{}).Where("operation_id=?", in.Before.OperationID).UpdateColumn("expt_id", f.expt).Error)
				})
			case "lifecycle owner":
				require.NoError(t, life.UpdateColumn("expt_id", f.expt+1).Error)
				t.Cleanup(func() {
					require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, key.RunID).UpdateColumn("expt_id", f.expt).Error)
				})
			case "scope":
				require.NoError(t, op.UpdateColumn("execution_scope", "wrong").Error)
			case "both disabled":
				require.NoError(t, life.UpdateColumns(map[string]any{"before_enabled": false, "after_enabled": false}).Error)
			case "missing experiment":
				require.NoError(t, f.sql.Unscoped().Delete(&model.Experiment{}, "id=?", f.expt).Error)
			}
			out, err := NewHookSummaryRepo(f.p).MGetSummaries(context.Background(), []entity.HookRunKey{key})
			require.Error(t, err)
			require.Nil(t, out)
			require.NotContains(t, err.Error(), "secret-status")
		})
	}
}

func TestHookSummaryRejectsCorruptTerminalPayload(t *testing.T) {
	for _, payload := range []string{"private-not-json", `[]`, `{"x":null}`, `{"x":1}`, `{"x":"a","x":"b"}`, `{} {}`, "{\"x\":\"\xff\"}", `{"":"x"}`, `{"x":"` + strings.Repeat("x", 8193) + `"}`, strings.Repeat("s", 32769)} {
		f := newHookTxFixture(t)
		in := f.input(true, 0)
		_, err := f.repo.CreateRunWithHooks(context.Background(), in)
		require.NoError(t, err)
		require.NoError(t, f.sql.Model(&model.ExptLifecycleHookRun{}).Where("operation_id=?", in.Before.OperationID).UpdateColumns(map[string]any{"status": "succeeded", "attempt": 1, "result_redacted": []byte(payload)}).Error)
		out, err := NewHookSummaryRepo(f.p).MGetSummaries(context.Background(), []entity.HookRunKey{in.Key})
		require.ErrorIs(t, err, entity.ErrHookSummaryUnavailable)
		require.Nil(t, out)
		require.NotContains(t, err.Error(), payload)
	}
	f := newHookTxFixture(t)
	in := f.input(true, 0)
	_, err := f.repo.CreateRunWithHooks(context.Background(), in)
	require.NoError(t, err)
	require.NoError(t, f.sql.Model(&model.ExptLifecycleHookRun{}).Where("operation_id=?", in.Before.OperationID).UpdateColumns(map[string]any{"status": "failed", "error_message": strings.Repeat("密", 1000)}).Error)
	out, err := NewHookSummaryRepo(f.p).MGetSummaries(context.Background(), []entity.HookRunKey{in.Key})
	require.ErrorIs(t, err, entity.ErrHookSummaryUnavailable)
	require.Nil(t, out)
}

func TestHookSummaryBatchBoundedAndDBFailure(t *testing.T) {
	f := newHookTxFixture(t)
	var keys []entity.HookRunKey
	latest := int64(0)
	for i := 0; i < 100; i++ {
		in := f.input(true, latest)
		_, err := f.repo.CreateRunWithHooks(context.Background(), in)
		require.NoError(t, err)
		latest = in.Key.RunID
		keys = append(keys, in.Key)
	}
	queries := hookSummaryQueries(t, f)
	require.Len(t, hookSummaryRead(t, f, keys...), 100)
	require.Len(t, *queries, 3)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out, err := NewHookSummaryRepo(f.p).MGetSummaries(ctx, keys)
	require.ErrorIs(t, err, entity.ErrHookSummaryUnavailable)
	require.Nil(t, out)
}
