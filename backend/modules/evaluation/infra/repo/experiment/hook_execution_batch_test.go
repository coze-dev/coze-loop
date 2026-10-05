// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

var executionBatchKey = entity.HookRunKey{WorkspaceID: 10, ExperimentID: 20, RunID: 30}

type executionBatchFixture struct {
	ledger  []model.ExptLifecycleRunItem
	items   []entity.HookExecutionInitializationItem
	results []model.ExptItemResult
	logs    []model.ExptItemResultRunLog
	turns   []model.ExptTurnResult
}

func executionBatchData(t *testing.T, start, count int) *executionBatchFixture {
	t.Helper()
	f := new(executionBatchFixture)
	k := executionBatchKey
	for i := start; i < start+count; i++ {
		frozen := entity.HookPlanItem{ID: int64(10000 + i), SourceSpaceID: 40, EvalSetID: 50, EvalSetVersionID: 60, ItemID: int64(20000 - i), ItemVersionID: int64(i % 2)}
		m := entity.HookExecutionManifest{Version: 1, Key: k, Ordinal: int64(i), Frozen: frozen, ItemResultID: int64(30000 + i), ItemRunLogID: int64(40000 + i), Turns: []entity.HookExecutionTurnManifest{{TurnID: 0, TurnIdx: 0, ResultID: int64(50000 + 2*i)}, {TurnID: 9, TurnIdx: 1, ResultID: int64(50001 + 2*i)}}, TurnLogsInitialized: gptr.Of(false)}
		require.NoError(t, m.Validate())
		raw, err := json.Marshal(m)
		require.NoError(t, err)
		f.ledger = append(f.ledger, model.ExptLifecycleRunItem{ID: frozen.ID, SpaceID: k.WorkspaceID, ExptID: k.ExperimentID, ExptRunID: k.RunID, Ordinal: m.Ordinal, SourceSpaceID: frozen.SourceSpaceID, EvalSetID: frozen.EvalSetID, EvalSetVersionID: frozen.EvalSetVersionID, ItemID: frozen.ItemID, ItemVersionID: frozen.ItemVersionID, ExecutionManifest: &raw})
		f.items = append(f.items, entity.HookExecutionInitializationItem{Ordinal: m.Ordinal, Frozen: frozen, Manifest: &m})
		f.results = append(f.results, model.ExptItemResult{ID: m.ItemResultID, SpaceID: k.WorkspaceID, ExptID: k.ExperimentID, ExptRunID: k.RunID, ItemID: frozen.ItemID, ItemVersionID: frozen.ItemVersionID, ItemIdx: gptr.Of(int32(i)), Status: int32(entity.ItemRunState_Queueing)})
		f.logs = append(f.logs, model.ExptItemResultRunLog{ID: m.ItemRunLogID, SpaceID: k.WorkspaceID, ExptID: k.ExperimentID, ExptRunID: k.RunID, ItemID: frozen.ItemID, ItemVersionID: frozen.ItemVersionID, Status: int32(entity.ItemRunState_Queueing)})
		for _, turn := range m.Turns {
			f.turns = append(f.turns, model.ExptTurnResult{ID: turn.ResultID, SpaceID: k.WorkspaceID, ExptID: k.ExperimentID, ExptRunID: k.RunID, ItemID: frozen.ItemID, ItemVersionID: frozen.ItemVersionID, TurnID: turn.TurnID, TurnIdx: gptr.Of(turn.TurnIdx), Status: int32(entity.TurnRunState_Queueing)})
		}
	}
	return f
}

func executionBatchDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	c, mock, err := sqlmock.New()
	require.NoError(t, err)
	tx, err := gorm.Open(mysql.New(mysql.Config{Conn: c, SkipInitializeWithVersion: true}), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, mock.ExpectationsWereMet()); _ = c.Close() })
	return tx, mock
}

func executionBatchState(count int, initialized bool) *lockedHookRun {
	return &lockedHookRun{life: model.ExptLifecycleRun{PlanCount: int64(count), Version: 7, ExecutionInitialized: initialized}, view: &entity.HookStoredRun{State: entity.HookRunState{Key: executionBatchKey}, PlanHash: strings.Repeat("a", 64)}}
}

func executionBatchExpectLedger(mock sqlmock.Sqlmock, f *executionBatchFixture, start, end, limit int) {
	rows := sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "ordinal", "source_space_id", "eval_set_id", "eval_set_version_id", "item_id", "item_version_id", "execution_manifest"})
	for _, r := range f.ledger {
		var raw driver.Value
		if r.ExecutionManifest != nil {
			raw = *r.ExecutionManifest
		}
		rows.AddRow(r.ID, r.SpaceID, r.ExptID, r.ExptRunID, r.Ordinal, r.SourceSpaceID, r.EvalSetID, r.EvalSetVersionID, r.ItemID, r.ItemVersionID, raw)
	}
	query := "SELECT * FROM `expt_lifecycle_run_item` WHERE (space_id=? AND expt_id=? AND expt_run_id=?) AND (ordinal>=? AND ordinal<?) ORDER BY ordinal ASC,id ASC LIMIT ? FOR UPDATE"
	mock.ExpectQuery("^"+regexp.QuoteMeta(query)+"$").WithArgs(executionBatchKey.WorkspaceID, executionBatchKey.ExperimentID, executionBatchKey.RunID, int64(start), int64(end), limit+1).WillReturnRows(rows)
}

func executionBatchRecordRows(f *executionBatchFixture) []*sqlmock.Rows {
	items := sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "item_id", "item_version_id", "item_idx", "status", "deleted_at", "err_msg", "log_id", "ext"})
	for _, r := range f.results {
		items.AddRow(r.ID, r.SpaceID, r.ExptID, r.ExptRunID, r.ItemID, r.ItemVersionID, r.ItemIdx, r.Status, r.DeletedAt, r.ErrMsg, r.LogID, r.Ext)
	}
	logs := sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "item_id", "item_version_id", "status", "deleted_at", "quota_reservation_state", "retry_times", "result_state", "err_msg", "log_id"})
	for _, r := range f.logs {
		logs.AddRow(r.ID, r.SpaceID, r.ExptID, r.ExptRunID, r.ItemID, r.ItemVersionID, r.Status, r.DeletedAt, r.QuotaReservationState, r.RetryTimes, r.ResultState, r.ErrMsg, r.LogID)
	}
	turns := sqlmock.NewRows([]string{"id", "space_id", "expt_id", "expt_run_id", "item_id", "item_version_id", "turn_id", "turn_idx", "status", "deleted_at", "target_result_id", "trace_id", "log_id", "weighted_score", "err_msg"})
	for _, r := range f.turns {
		turns.AddRow(r.ID, r.SpaceID, r.ExptID, r.ExptRunID, r.ItemID, r.ItemVersionID, r.TurnID, r.TurnIdx, r.Status, r.DeletedAt, r.TargetResultID, r.TraceID, r.LogID, r.WeightedScore, r.ErrMsg)
	}
	return []*sqlmock.Rows{items, logs, turns}
}

func executionBatchExpectRecords(mock sqlmock.Sqlmock, f *executionBatchFixture, single bool, failAt int, dbErr error) {
	args := []driver.Value{executionBatchKey.WorkspaceID, executionBatchKey.ExperimentID}
	for _, item := range f.items {
		args = append(args, item.Frozen.ItemID)
	}
	condition := regexp.QuoteMeta("space_id=? AND expt_id=? AND item_id IN (" + strings.TrimSuffix(strings.Repeat("?,", len(f.items)), ",") + ")")
	if single {
		// The single-item oracle accepts both equivalent SQL forms, including the pre-batch implementation.
		condition = regexp.QuoteMeta("space_id=? AND expt_id=? AND item_id") + `(?:=\?| IN \(\?\))`
	}
	rows := executionBatchRecordRows(f)
	for i, table := range []string{"expt_item_result", "expt_item_result_run_log", "expt_turn_result"} {
		query := regexp.QuoteMeta("SELECT * FROM `"+table+"` WHERE ") + condition
		queryArgs := append([]driver.Value(nil), args...)
		if i == 1 {
			query = regexp.QuoteMeta("SELECT * FROM `"+table+"` WHERE (") + condition + regexp.QuoteMeta(") AND expt_run_id=?")
			queryArgs = append(queryArgs, executionBatchKey.RunID)
		}
		if i == 2 {
			if single {
				query += ` ORDER BY (?:item_id ASC,)?turn_idx ASC,turn_id ASC`
			} else {
				query += regexp.QuoteMeta(" ORDER BY item_id ASC,turn_idx ASC,turn_id ASC")
			}
		}
		expect := mock.ExpectQuery("^" + query + " FOR UPDATE$").WithArgs(queryArgs...)
		if i == failAt {
			expect.WillReturnError(dbErr)
			return
		}
		expect.WillReturnRows(rows[i])
	}
}

func TestHookExecutionBatchQueriesPerPage(t *testing.T) {
	for _, count := range []int{100, 101, 1000} {
		t.Run(fmt.Sprint(count), func(t *testing.T) {
			tx, mock := executionBatchDB(t)
			s := executionBatchState(count, false)
			pages := 0
			for start := 0; start < count; start += 100 {
				end := min(start+100, count)
				f := executionBatchData(t, start, end-start)
				// DB result order need not match frozen ordinal order.
				slices.Reverse(f.results)
				slices.Reverse(f.logs)
				slices.SortStableFunc(f.turns, func(a, b model.ExptTurnResult) int { return int(a.ItemID - b.ItemID) })
				executionBatchExpectLedger(mock, f, start, end, 100)
				executionBatchExpectRecords(mock, f, false, -1, nil)
				page, rows, err := readExecutionPage(tx, s, int64(start), 100)
				require.NoError(t, err)
				require.Equal(t, f.items, page.Items)
				require.Equal(t, f.ledger, rows)
				require.Equal(t, int64(count), page.Count)
				require.Equal(t, s.view.PlanHash, page.Hash)
				require.Equal(t, int64(7), page.RunVersion)
				require.False(t, page.Initialized)
				require.Equal(t, int64(end), page.NextOrdinal)
				require.Equal(t, end < count, page.HasMore)
				pages++
			}
			require.NoError(t, mock.ExpectationsWereMet())
			t.Logf("items=%d pages=%d exact_selects=%d (ledger + three result tables per page)", count, pages, 4*pages)
		})
	}
}

func TestHookExecutionBatchEquivalentRecords(t *testing.T) {
	deleted := gorm.DeletedAt{Time: time.Unix(1, 0), Valid: true}
	cases := []struct {
		name string
		edit func(*executionBatchFixture)
	}{
		{"item_pk", func(f *executionBatchFixture) { f.results[0].ID++ }},
		{"item_foreign_run", func(f *executionBatchFixture) { f.results[0].ExptRunID++ }},
		{"item_version", func(f *executionBatchFixture) { f.results[0].ItemVersionID++ }},
		{"item_ordinal", func(f *executionBatchFixture) { f.results[0].ItemIdx = gptr.Of(int32(2)) }},
		{"item_nil_ordinal", func(f *executionBatchFixture) { f.results[0].ItemIdx = nil }},
		{"item_deleted", func(f *executionBatchFixture) { f.results[0].DeletedAt = deleted }},
		{"item_missing", func(f *executionBatchFixture) { f.results = nil }},
		{"item_extra", func(f *executionBatchFixture) { f.results = append(f.results, f.results[0]) }},
		{"log_pk", func(f *executionBatchFixture) { f.logs[0].ID++ }},
		{"log_version", func(f *executionBatchFixture) { f.logs[0].ItemVersionID++ }},
		{"log_deleted", func(f *executionBatchFixture) { f.logs[0].DeletedAt = deleted }},
		{"log_missing", func(f *executionBatchFixture) { f.logs = nil }},
		{"log_extra", func(f *executionBatchFixture) { f.logs = append(f.logs, f.logs[0]) }},
		{"turn_pk", func(f *executionBatchFixture) { f.turns[0].ID++ }},
		{"turn_foreign_run", func(f *executionBatchFixture) { f.turns[0].ExptRunID++ }},
		{"turn_version", func(f *executionBatchFixture) { f.turns[0].ItemVersionID++ }},
		{"turn_id", func(f *executionBatchFixture) { f.turns[0].TurnID++ }},
		{"turn_idx", func(f *executionBatchFixture) { f.turns[0].TurnIdx = gptr.Of(int32(2)) }},
		{"turn_nil_idx", func(f *executionBatchFixture) { f.turns[0].TurnIdx = nil }},
		{"turn_deleted", func(f *executionBatchFixture) { f.turns[0].DeletedAt = deleted }},
		{"turn_missing", func(f *executionBatchFixture) { f.turns = f.turns[:1] }},
		{"turn_extra", func(f *executionBatchFixture) { f.turns = append(f.turns, f.turns[0]) }},
	}
	initialOnly := []struct {
		name string
		edit func(*executionBatchFixture)
	}{
		{"item_status", func(f *executionBatchFixture) { f.results[0].Status++ }},
		{"item_error", func(f *executionBatchFixture) { f.results[0].ErrMsg = gptr.Of([]byte("error")) }},
		{"item_log", func(f *executionBatchFixture) { f.results[0].LogID = "log" }},
		{"item_ext", func(f *executionBatchFixture) { f.results[0].Ext = gptr.Of([]byte("ext")) }},
		{"log_status", func(f *executionBatchFixture) { f.logs[0].Status++ }},
		{"log_quota", func(f *executionBatchFixture) { f.logs[0].QuotaReservationState = 1 }},
		{"log_retry", func(f *executionBatchFixture) { f.logs[0].RetryTimes = 1 }},
		{"log_result", func(f *executionBatchFixture) { f.logs[0].ResultState = gptr.Of(int32(1)) }},
		{"log_error", func(f *executionBatchFixture) { f.logs[0].ErrMsg = gptr.Of([]byte("error")) }},
		{"log_log", func(f *executionBatchFixture) { f.logs[0].LogID = "log" }},
		{"turn_status", func(f *executionBatchFixture) { f.turns[0].Status++ }},
		{"turn_target", func(f *executionBatchFixture) { f.turns[0].TargetResultID = 1 }},
		{"turn_trace", func(f *executionBatchFixture) { f.turns[0].TraceID = 1 }},
		{"turn_log", func(f *executionBatchFixture) { f.turns[0].LogID = "log" }},
		{"turn_score", func(f *executionBatchFixture) { f.turns[0].WeightedScore = gptr.Of(float64(0)) }},
		{"turn_error", func(f *executionBatchFixture) { f.turns[0].ErrMsg = gptr.Of([]byte("error")) }},
	}
	for _, initial := range []bool{true, false} {
		t.Run(fmt.Sprintf("initial=%v", initial), func(t *testing.T) {
			for i, tc := range append(cases, initialOnly...) {
				t.Run(tc.name, func(t *testing.T) {
					tx, mock := executionBatchDB(t)
					f := executionBatchData(t, 0, 1)
					tc.edit(f)
					executionBatchExpectRecords(mock, f, true, -1, nil)
					err := verifyExecutionRecords(tx, executionBatchKey, f.items[0], initial)
					if i >= len(cases) && !initial {
						require.NoError(t, err)
					} else {
						require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
					}
				})
			}
			t.Run("valid_zero_turn_id_and_zero_item_version", func(t *testing.T) {
				tx, mock := executionBatchDB(t)
				f := executionBatchData(t, 0, 1)
				executionBatchExpectRecords(mock, f, true, -1, nil)
				require.NoError(t, verifyExecutionRecords(tx, executionBatchKey, f.items[0], initial))
			})
		})
	}
}

func TestHookExecutionBatchUncommittedRows(t *testing.T) {
	for _, row := range []string{"none", "item", "log", "turn"} {
		t.Run(row, func(t *testing.T) {
			tx, mock := executionBatchDB(t)
			f := executionBatchData(t, 0, 1)
			f.items[0].Manifest = nil
			if row != "item" {
				f.results = nil
			}
			if row != "log" {
				f.logs = nil
			}
			if row != "turn" {
				f.turns = nil
			}
			executionBatchExpectRecords(mock, f, true, -1, nil)
			err := verifyExecutionRecords(tx, executionBatchKey, f.items[0], true)
			if row == "none" {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
			}
		})
	}
}

func TestHookExecutionBatchPageFailureReturnsNoPartialData(t *testing.T) {
	for _, stage := range []string{"item_read", "log_read", "turn_read", "last_item_corrupt", "uncommitted_with_row"} {
		t.Run(stage, func(t *testing.T) {
			tx, mock := executionBatchDB(t)
			f := executionBatchData(t, 0, 2)
			failAt := slices.Index([]string{"item_read", "log_read", "turn_read"}, stage)
			dbErr := errors.New("read failed")
			if stage == "last_item_corrupt" {
				f.results[1].ID++
			}
			if stage == "uncommitted_with_row" {
				f.ledger[1].ExecutionManifest = nil
				f.items[1].Manifest = nil
			}
			executionBatchExpectLedger(mock, f, 0, 2, 100)
			executionBatchExpectRecords(mock, f, false, failAt, dbErr)
			page, rows, err := readExecutionPage(tx, executionBatchState(2, false), 0, 100)
			if failAt >= 0 {
				require.ErrorIs(t, err, dbErr)
			} else {
				require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
			}
			require.Nil(t, page)
			require.Nil(t, rows)
		})
	}
}

func TestHookExecutionBatchEmptyAndCancelled(t *testing.T) {
	t.Run("empty_plan", func(t *testing.T) {
		tx, mock := executionBatchDB(t)
		f := executionBatchData(t, 0, 0)
		executionBatchExpectLedger(mock, f, 0, 0, 100)
		page, _, err := readExecutionPage(tx, executionBatchState(0, false), 0, 100)
		require.NoError(t, err)
		require.Empty(t, page.Items)
		require.Zero(t, page.NextOrdinal)
		require.False(t, page.HasMore)
	})
	t.Run("cancelled", func(t *testing.T) {
		tx, _ := executionBatchDB(t)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		page, rows, err := readExecutionPage(tx.WithContext(ctx), executionBatchState(1, false), 0, 100)
		require.ErrorIs(t, err, context.Canceled)
		require.Nil(t, page)
		require.Nil(t, rows)
	})
}

func TestHookExecutionBatchMixedAndInitializedPages(t *testing.T) {
	for _, mode := range []string{"mixed", "uncommitted", "initialized", "initialized_corrupt", "item_wrong_group", "log_wrong_group", "turn_wrong_group"} {
		t.Run(mode, func(t *testing.T) {
			tx, mock := executionBatchDB(t)
			f := executionBatchData(t, 0, 2)
			initialized := strings.HasPrefix(mode, "initialized")
			bad := strings.HasSuffix(mode, "corrupt") || strings.HasSuffix(mode, "group")
			switch mode {
			case "mixed":
				f.ledger[1].ExecutionManifest, f.items[1].Manifest = nil, nil
				f.results, f.logs, f.turns = f.results[:1], f.logs[:1], f.turns[:2]
			case "uncommitted":
				for i := range f.items {
					f.ledger[i].ExecutionManifest, f.items[i].Manifest = nil, nil
				}
				f.results, f.logs, f.turns = nil, nil, nil
			case "initialized", "initialized_corrupt":
				f.results[1].Status++
				f.logs[1].QuotaReservationState = 1
				f.turns[2].TargetResultID = 123
				if bad {
					f.turns[2].ID++
				}
			case "item_wrong_group":
				f.results[1].ItemID = f.results[0].ItemID
			case "log_wrong_group":
				f.logs[1].ItemID = f.logs[0].ItemID
			case "turn_wrong_group":
				f.turns[2].ItemID = f.turns[0].ItemID
			}
			slices.SortStableFunc(f.turns, func(a, b model.ExptTurnResult) int { return int(a.ItemID - b.ItemID) })
			executionBatchExpectLedger(mock, f, 0, 2, 100)
			executionBatchExpectRecords(mock, f, false, -1, nil)
			page, rows, err := readExecutionPage(tx, executionBatchState(2, initialized), 0, 100)
			if bad {
				require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
				require.Nil(t, page)
				require.Nil(t, rows)
			} else {
				require.NoError(t, err)
				require.Equal(t, f.items, page.Items)
				require.Equal(t, initialized, page.Initialized)
			}
		})
	}
}

func TestHookExecutionBatchManifestValidation(t *testing.T) {
	for _, defect := range []string{"empty", "unknown_field", "trailing_json", "malformed", "version", "key", "ordinal", "frozen", "turn_order", "missing_initialized", "ledger_gap", "ledger_missing", "ledger_extra"} {
		t.Run(defect, func(t *testing.T) {
			tx, mock := executionBatchDB(t)
			f := executionBatchData(t, 0, 2)
			m := f.items[1].Manifest.Clone()
			switch defect {
			case "version":
				m.Version++
			case "key":
				m.Key.RunID++
			case "ordinal":
				m.Ordinal++
			case "frozen":
				m.Frozen.EvalSetVersionID++
			case "turn_order":
				m.Turns[0].TurnIdx++
			}
			raw, err := json.Marshal(m)
			require.NoError(t, err)
			switch defect {
			case "empty":
				raw = nil
			case "unknown_field":
				raw = append([]byte(`{"unknown":true,`), raw[1:]...)
			case "trailing_json":
				raw = append(raw, []byte(` {}`)...)
			case "malformed":
				raw = []byte(`{`)
			}
			f.ledger[1].ExecutionManifest = &raw
			if defect == "missing_initialized" {
				f.ledger[1].ExecutionManifest = nil
			}
			if defect == "ledger_gap" {
				f.ledger[1].Ordinal++
			}
			if defect == "ledger_missing" {
				f.ledger = f.ledger[:1]
			}
			if defect == "ledger_extra" {
				f.ledger = append(f.ledger, f.ledger[1])
			}
			executionBatchExpectLedger(mock, f, 0, 2, 100)
			page, rows, err := readExecutionPage(tx, executionBatchState(2, defect == "missing_initialized"), 0, 100)
			require.ErrorIs(t, err, entity.ErrHookStoreCorrupt)
			require.Nil(t, page)
			require.Nil(t, rows)
		})
	}
}
