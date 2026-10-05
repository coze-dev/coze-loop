// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	driver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

const hookScanSocket = "/Users/bytedance/fornax_workspace/_local/scratch/hook-mysql-20260922/m.sock"

var hookScanSequence = func() *atomic.Int64 { v := new(atomic.Int64); v.Store(time.Now().UnixMicro()); return v }()

type hookScanFixture struct {
	sql   *gorm.DB
	repo  repo.IHookScanRepo
	base  int64
	scope string
	now   time.Time
}

func newHookScanFixture(t *testing.T) *hookScanFixture {
	t.Helper()
	dsn := os.Getenv("HOOK_MYSQL_SCAN_DSN")
	if dsn == "" {
		t.Skip("requires HOOK_MYSQL_SCAN_DSN with the isolated local scan database")
	}
	cfg, err := driver.ParseDSN(dsn)
	require.NoError(t, err)
	require.Equal(t, "unix", cfg.Net)
	require.Equal(t, hookScanSocket, cfg.Addr)
	require.Equal(t, "hook_7378265404_scan", cfg.DBName)
	require.True(t, cfg.ParseTime)
	require.Equal(t, time.UTC, cfg.Loc)
	p, err := db.NewDB(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	s := p.NewSession(context.Background())
	conn, err := s.DB()
	require.NoError(t, err)
	conn.SetMaxOpenConns(2)
	base := hookScanSequence.Add(100000)
	f := &hookScanFixture{sql: s, repo: NewHookScanRepo(p), base: base, scope: fmt.Sprintf("scan_%d", base), now: time.Date(2026, 9, 22, 12, 0, 0, 0, time.UTC)}
	t.Cleanup(func() {
		for _, table := range []string{"expt_lifecycle_hook_run", "expt_lifecycle_run"} {
			require.NoError(t, s.Table(table).Where("execution_scope IN ?", []string{f.scope, f.scope + "_other"}).Delete(nil).Error)
		}
		require.NoError(t, conn.Close())
	})
	return f
}

func (f *hookScanFixture) input(status entity.HookOperationStatus, limit int) entity.HookScanInput {
	return entity.HookScanInput{ExecutionScope: f.scope, Status: status, Now: f.now, Limit: limit}
}

func (f *hookScanFixture) operation(n int64, status string, at *time.Time) model.ExptLifecycleHookRun {
	id := f.base + n
	return model.ExptLifecycleHookRun{ID: id, SpaceID: f.base, ExptID: f.base + 1, ExptRunID: id,
		Phase: "after", OperationID: fmt.Sprint(id), IdempotencyKey: fmt.Sprintf("key_%d", id),
		Status: status, ActivatedAt: at, NextAttemptAt: at, LeaseUntil: at, ExecutionScope: f.scope, Version: 7}
}

func (f *hookScanFixture) run(space, run int64) model.ExptLifecycleRun {
	return model.ExptLifecycleRun{SpaceID: f.base + space, ExptID: f.base + space, ExptRunID: f.base + run,
		ExecutionScope: f.scope, SnapshotCipher: []byte("do-not-return-cipher"), SnapshotKeyID: "do-not-return-key",
		SnapshotHash: strings.Repeat("a", 64), Version: 9}
}

func TestHookScanMySQLDueBoundariesAndRawTail(t *testing.T) {
	f := newHookScanFixture(t)
	for _, status := range []entity.HookOperationStatus{entity.HookOperationPending, entity.HookOperationRetryWait} {
		t.Run(string(status), func(t *testing.T) {
			// A time-only cursor would skip 100 tied rows; an ID-only cursor would reorder row 110.
			offset := int64(0)
			if status == entity.HookOperationRetryWait {
				offset = 1000
			}
			rows := make([]model.ExptLifecycleHookRun, 0, 110)
			for i := int64(1); i <= 102; i++ {
				rows = append(rows, f.operation(offset+i, string(status), &f.now))
			}
			past, future := f.now.Add(-time.Millisecond), f.now.Add(time.Millisecond)
			rows = append(rows, f.operation(offset+110, string(status), &past), f.operation(offset+111, string(status), &future), f.operation(offset+112, string(status), nil))
			other := f.operation(offset+113, string(status), &past)
			other.ExecutionScope = f.scope + "_other"
			rows = append(rows, other, f.operation(offset+114, "succeeded", &past))
			inactive := f.operation(offset+115, string(status), &past)
			inactive.ActivatedAt = nil
			rows = append(rows, inactive)
			require.NoError(t, f.sql.Create(&rows).Error)
			in := f.input(status, 100)
			page, err := f.repo.ScanDueOperations(context.Background(), in)
			require.NoError(t, err)
			require.Len(t, page.Candidates, 100)
			require.Equal(t, f.base+offset+110, page.Candidates[0].ID)
			require.Equal(t, f.base+offset+99, page.NextCursor.ID)
			require.True(t, page.HasMore)
			// Discard the whole first page; continuation comes from the raw page, not retained rows.
			page.Candidates = nil
			in.Cursor = page.NextCursor
			next, err := f.repo.ScanDueOperations(context.Background(), in)
			require.NoError(t, err)
			require.Len(t, next.Candidates, 3)
			require.False(t, next.HasMore)
			for i, candidate := range next.Candidates {
				require.Equal(t, f.base+offset+100+int64(i), candidate.ID)
				require.Equal(t, entity.HookRunKey{WorkspaceID: f.base, ExperimentID: f.base + 1, RunID: candidate.ID}, candidate.Key)
				require.Equal(t, fmt.Sprint(candidate.ID), candidate.OperationID)
				require.Equal(t, entity.HookPhaseAfter, candidate.Phase)
				require.Equal(t, int64(7), candidate.Version)
				require.True(t, f.now.Equal(candidate.DueAt))
			}
			in.Cursor = next.NextCursor
			empty, err := f.repo.ScanDueOperations(context.Background(), in)
			require.NoError(t, err)
			require.Empty(t, empty.Candidates)
			require.Nil(t, empty.NextCursor)
			require.False(t, empty.HasMore)
		})
	}
}

func TestHookScanMySQLExpiredIncludesOverdueRunning(t *testing.T) {
	f := newHookScanFixture(t)
	old, future := f.now.Add(-365*24*time.Hour), f.now.Add(time.Millisecond)
	rows := []model.ExptLifecycleHookRun{f.operation(1, "running", &old), f.operation(2, "running", &f.now), f.operation(3, "running", &f.now), f.operation(4, "running", &future), f.operation(5, "running", nil), f.operation(6, "pending", &old)}
	rows[0].OperationDeadline, rows[0].AttemptDeadline = &old, &old
	other := f.operation(7, "running", &old)
	other.ExecutionScope = f.scope + "_other"
	rows = append(rows, other)
	require.NoError(t, f.sql.Create(&rows).Error)
	in := f.input("", 1)
	for _, n := range []int64{1, 2, 3} {
		page, err := f.repo.ScanExpiredOperations(context.Background(), in)
		require.NoError(t, err)
		require.Len(t, page.Candidates, 1)
		require.Equal(t, f.base+n, page.Candidates[0].ID)
		require.Equal(t, n != 3, page.HasMore)
		in.Cursor = page.NextCursor
	}
}

func TestHookScanMySQLPreparingIncludesTerminalAndOrdersCompositeKey(t *testing.T) {
	f := newHookScanFixture(t)
	rows := []model.ExptLifecycleRun{f.run(2, 1), f.run(1, 3), f.run(1, 2), f.run(1, 1), f.run(3, 1)}
	terminal := int32(4)
	rows[3].TerminalStatus, rows[3].Gate = &terminal, 2
	rows[4].PlanState = 1
	other := f.run(4, 1)
	other.ExecutionScope = f.scope + "_other"
	rows = append(rows, other)
	require.NoError(t, f.sql.Create(&rows).Error)
	in := f.input("", 2)
	first, err := f.repo.ScanPreparingPlans(context.Background(), in)
	require.NoError(t, err)
	require.Len(t, first.Candidates, 2)
	require.Equal(t, f.base+1, first.Candidates[0].Key.RunID) // Already terminal still advances the raw cursor.
	require.True(t, first.HasMore)
	in.Cursor = first.NextCursor
	next, err := f.repo.ScanPreparingPlans(context.Background(), in)
	require.NoError(t, err)
	require.Len(t, next.Candidates, 2)
	require.Equal(t, entity.HookRunKey{WorkspaceID: f.base + 1, ExperimentID: f.base + 1, RunID: f.base + 3}, next.Candidates[0].Key)
	require.Equal(t, entity.HookRunKey{WorkspaceID: f.base + 2, ExperimentID: f.base + 2, RunID: f.base + 1}, next.Candidates[1].Key)
	require.Equal(t, int64(9), next.Candidates[1].Version)
	require.True(t, next.Candidates[1].ReconcileAt.IsZero())
	require.False(t, next.HasMore) // Exactly a full last page, not a guessed has-more.
	encoded, err := json.Marshal(next)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "do-not-return")
}

func TestHookScanMySQLFinalizeTimeAndPrimaryKeyTies(t *testing.T) {
	f := newHookScanFixture(t)
	old, future := f.now.Add(-time.Millisecond), f.now.Add(time.Millisecond)
	rows := []model.ExptLifecycleRun{f.run(1, 1), f.run(1, 2), f.run(2, 1), f.run(3, 1), f.run(4, 1), f.run(5, 1), f.run(6, 1)}
	for i := range rows {
		rows[i].FinalizeState, rows[i].NextReconcileAt = 1, &f.now
	}
	rows[3].NextReconcileAt = &old
	rows[4].NextReconcileAt = &future
	rows[5].NextReconcileAt = nil
	rows[6].FinalizeState = 2
	other := f.run(7, 1)
	other.ExecutionScope, other.FinalizeState, other.NextReconcileAt = f.scope+"_other", 1, &old
	rows = append(rows, other)
	require.NoError(t, f.sql.Create(&rows).Error)
	in := f.input("", 1)
	for i, pair := range [][2]int64{{3, 1}, {1, 1}, {1, 2}, {2, 1}} {
		page, err := f.repo.ScanPendingFinalizations(context.Background(), in)
		require.NoError(t, err)
		require.Len(t, page.Candidates, 1)
		require.Equal(t, entity.HookRunKey{WorkspaceID: f.base + pair[0], ExperimentID: f.base + pair[0], RunID: f.base + pair[1]}, page.Candidates[0].Key)
		require.Equal(t, i < 3, page.HasMore)
		in.Cursor = page.NextCursor
	}
	var after []model.ExptLifecycleRun
	require.NoError(t, f.sql.Where("execution_scope IN ?", []string{f.scope, f.scope + "_other"}).Order("space_id,expt_run_id").Find(&after).Error)
	for i := range after {
		require.Equal(t, rows[i].FinalizeState, after[i].FinalizeState)
		require.Equal(t, int64(9), after[i].Version)
	}
}

func TestHookScanRejectsInvalidInputBeforeDB(t *testing.T) {
	r := NewHookScanRepo(nil)
	ctx := context.Background()
	_, err := r.ScanDueOperations(ctx, entity.HookScanInput{})
	require.Error(t, err)
	_, err = r.ScanExpiredOperations(ctx, entity.HookScanInput{})
	require.Error(t, err)
	_, err = r.ScanPreparingPlans(ctx, entity.HookScanInput{})
	require.Error(t, err)
	_, err = r.ScanPendingFinalizations(ctx, entity.HookScanInput{})
	require.Error(t, err)
}

func TestHookScanMySQLCancellation(t *testing.T) {
	f := newHookScanFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.repo.ScanDueOperations(ctx, f.input(entity.HookOperationPending, 1))
	require.ErrorIs(t, err, context.Canceled)
	_, err = f.repo.ScanExpiredOperations(ctx, f.input("", 1))
	require.ErrorIs(t, err, context.Canceled)
	_, err = f.repo.ScanPreparingPlans(ctx, f.input("", 1))
	require.ErrorIs(t, err, context.Canceled)
	_, err = f.repo.ScanPendingFinalizations(ctx, f.input("", 1))
	require.ErrorIs(t, err, context.Canceled)
}

func TestHookScanMySQLExplicitClockMatchesStorageLocation(t *testing.T) {
	f := newHookScanFixture(t)
	cfg, err := driver.ParseDSN(os.Getenv("HOOK_MYSQL_SCAN_DSN"))
	require.NoError(t, err)
	cfg.Loc, err = time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	p, err := db.NewDB(mysql.Open(cfg.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	s := p.NewSession(context.Background())
	conn, err := s.DB()
	require.NoError(t, err)
	defer conn.Close()
	localNow := f.now.In(cfg.Loc)
	rows := []model.ExptLifecycleHookRun{f.operation(1, "pending", &localNow), f.operation(2, "pending", &localNow)}
	require.NoError(t, s.Create(&rows).Error)
	in := f.input(entity.HookOperationPending, 1)
	in.Now = localNow
	r := NewHookScanRepo(p)
	for _, n := range []int64{1, 2} {
		page, err := r.ScanDueOperations(context.Background(), in)
		require.NoError(t, err)
		require.Len(t, page.Candidates, 1)
		require.Equal(t, f.base+n, page.Candidates[0].ID)
		require.True(t, localNow.Equal(page.Candidates[0].DueAt))
		in.Cursor = page.NextCursor
	}
}

func TestHookScanMySQLReadOnlyConnection(t *testing.T) {
	f := newHookScanFixture(t)
	ops := []model.ExptLifecycleHookRun{f.operation(1, "pending", &f.now), f.operation(2, "running", &f.now)}
	run := f.run(1, 1)
	run.FinalizeState, run.NextReconcileAt = 1, &f.now
	require.NoError(t, f.sql.Create(&ops).Error)
	require.NoError(t, f.sql.Create(&run).Error)
	var before []model.ExptLifecycleHookRun
	require.NoError(t, f.sql.Where("execution_scope = ?", f.scope).Order("id").Find(&before).Error)
	cfg, err := driver.ParseDSN(os.Getenv("HOOK_MYSQL_SCAN_DSN"))
	require.NoError(t, err)
	if cfg.Params == nil {
		cfg.Params = make(map[string]string)
	}
	cfg.Params["transaction_read_only"] = "1"
	p, err := db.NewDB(mysql.Open(cfg.FormatDSN()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	s := p.NewSession(context.Background())
	conn, err := s.DB()
	require.NoError(t, err)
	defer conn.Close()
	var readOnly int
	require.NoError(t, s.Raw("SELECT @@transaction_read_only").Scan(&readOnly).Error)
	require.Equal(t, 1, readOnly)
	r := NewHookScanRepo(p)
	due, err := r.ScanDueOperations(context.Background(), f.input(entity.HookOperationPending, 1))
	require.NoError(t, err)
	require.Len(t, due.Candidates, 1)
	expired, err := r.ScanExpiredOperations(context.Background(), f.input("", 1))
	require.NoError(t, err)
	require.Len(t, expired.Candidates, 1)
	preparing, err := r.ScanPreparingPlans(context.Background(), f.input("", 1))
	require.NoError(t, err)
	require.Len(t, preparing.Candidates, 1)
	finalize, err := r.ScanPendingFinalizations(context.Background(), f.input("", 1))
	require.NoError(t, err)
	require.Len(t, finalize.Candidates, 1)
	var unchanged []model.ExptLifecycleHookRun
	require.NoError(t, f.sql.Where("execution_scope = ?", f.scope).Order("id").Find(&unchanged).Error)
	require.Equal(t, before, unchanged)
}

func TestHookScanMySQLExplainExistingIndexes(t *testing.T) {
	f := newHookScanFixture(t)
	const count = 6000
	ops := make([]model.ExptLifecycleHookRun, 0, count)
	runs := make([]model.ExptLifecycleRun, 0, count)
	statuses := []string{"pending", "retry_wait", "running", "succeeded"}
	for i := 1; i <= count; i++ {
		at := f.now.Add(time.Duration(i/4-1600) * time.Millisecond)
		op := f.operation(int64(i), statuses[i%4], &at)
		run := f.run(int64(i/4+1), int64(i))
		run.PlanState, run.FinalizeState, run.NextReconcileAt = int32(i%2), int32(i%3), &at
		if i%5 == 0 {
			op.ActivatedAt, op.NextAttemptAt, op.LeaseUntil, run.NextReconcileAt = nil, nil, nil, nil
		} else if i%5 == 1 {
			future := f.now.Add(time.Duration(i) * time.Millisecond)
			op.NextAttemptAt, op.LeaseUntil, run.NextReconcileAt = &future, &future, &future
		}
		if i%7 == 0 {
			op.ExecutionScope, run.ExecutionScope = f.scope+"_other", f.scope+"_other"
		}
		ops, runs = append(ops, op), append(runs, run)
	}
	require.NoError(t, f.sql.CreateInBatches(ops, 250).Error)
	require.NoError(t, f.sql.CreateInBatches(runs, 250).Error)
	require.NoError(t, f.sql.Exec("ANALYZE TABLE expt_lifecycle_hook_run, expt_lifecycle_run").Error)
	var version string
	require.NoError(t, f.sql.Raw("SELECT VERSION()").Scan(&version).Error)
	t.Logf("MySQL=%s; fixture rows per table=%d; four statuses, two scopes, tied millisecond keys, 20%% NULL and 20%% future times", version, count)
	for _, tc := range []struct {
		kind               entity.HookScanKind
		status             entity.HookOperationStatus
		index, boundStatus string
	}{
		{entity.HookScanDue, entity.HookOperationPending, "idx_scope_status_next", "pending"},
		{entity.HookScanDue, entity.HookOperationRetryWait, "idx_scope_status_next", "retry_wait"},
		{entity.HookScanExpired, "", "idx_scope_status_lease", "running"},
		{entity.HookScanPreparing, "", "idx_scope_plan_run", "preparing"},
		{entity.HookScanFinalize, "", "idx_scope_finalize_reconcile", "pending"},
	} {
		for _, tail := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s_%s_tail_%t", tc.kind, tc.status, tail), func(t *testing.T) {
				in := f.input(tc.status, 100)
				if tail {
					c := &entity.HookScanCursor{Kind: tc.kind, ExecutionScope: f.scope, Status: tc.boundStatus, Now: f.now}
					if tc.kind == entity.HookScanDue || tc.kind == entity.HookScanExpired {
						c.ID = f.base + 5800
					} else {
						c.WorkspaceID, c.RunID = f.base+1451, f.base+5800
					}
					if tc.kind != entity.HookScanPreparing {
						c.At = f.now.Add(-150 * time.Millisecond)
					}
					in.Cursor = c
				}
				require.NoError(t, in.Validate(tc.kind))
				stmt := hookScanQuery(f.sql.Session(&gorm.Session{DryRun: true}), in, tc.kind).Find(&[]hookScanRow{}).Statement
				sql := stmt.SQL.String()
				t.Logf("SQL=%s; args=%v", sql, stmt.Vars)
				var plans []struct {
					ID                                                             int
					SelectType, Table, Type, PossibleKeys, Key, KeyLen, Ref, Extra string
					Rows                                                           int64
					Filtered                                                       float64
				}
				require.NoError(t, f.sql.Raw("EXPLAIN "+sql, stmt.Vars...).Scan(&plans).Error)
				encoded, err := json.Marshal(plans)
				require.NoError(t, err)
				t.Logf("EXPLAIN=%s", encoded)
				var raw string
				require.NoError(t, f.sql.Raw("EXPLAIN FORMAT=JSON "+sql, stmt.Vars...).Scan(&raw).Error)
				t.Logf("EXPLAIN_JSON=%s", raw)
				require.Len(t, plans, 1)
				require.Equal(t, tc.index, plans[0].Key)
				require.Contains(t, []string{"range", "ref"}, plans[0].Type)
				require.NotContains(t, plans[0].Extra, "filesort")
				require.NotContains(t, raw, `"using_filesort": true`)
				if tail {
					require.Equal(t, "range", plans[0].Type)
					require.Less(t, plans[0].Rows, int64(500))
				}
				var got []hookScanRow
				require.NoError(t, hookScanQuery(f.sql, in, tc.kind).Find(&got).Error)
				require.NotEmpty(t, got)
				require.LessOrEqual(t, len(got), 101)
			})
		}
	}
}
