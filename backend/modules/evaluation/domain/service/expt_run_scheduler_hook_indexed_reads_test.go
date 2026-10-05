// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestHookIndexedPreEvalPersistsTurnLogPins(t *testing.T) {
	f, ms := activeTerminationFixture(t, entity.ItemRunState_Queueing)
	m := lazyTurnManifest(t, f, ms[0], 2)
	ctx, eiec, pre := admitLazyTurn(t, f, m)
	require.NoError(t, pre.PreEval(ctx, eiec))
	var row model.ExptLifecycleRunItem
	require.NoError(t, f.sql.First(&row, m.Frozen.ID).Error)
	var manifest map[string]any
	require.NoError(t, json.Unmarshal(gptr.Indirect(row.ExecutionManifest), &manifest))
	require.Equal(t, true, manifest["turn_logs_initialized"])
	var logs []model.ExptTurnResultRunLog
	require.NoError(t, f.sql.Where("space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, f.key.RunID).Order("turn_id").Find(&logs).Error)
	turns := manifest["turns"].([]any)
	require.Len(t, turns, len(logs))
	for i, tr := range turns {
		require.Equal(t, float64(logs[i].ID), tr.(map[string]any)["run_log_id"])
	}
}

func TestHookIndexedArchiveExplain(t *testing.T) {
	for _, empty := range []bool{false, true} {
		t.Run(map[bool]string{false: "executed", true: "no_execution"}[empty], func(t *testing.T) {
			var f *finalizationManagerFixture
			var m entity.HookExecutionManifest
			var scheduler *ExptSchedulerImpl
			if empty {
				f, m, scheduler = zeroTimeoutFixture(t)
			} else {
				var ms []entity.HookExecutionManifest
				f, ms = activeTerminationFixture(t, entity.ItemRunState_Queueing)
				m = ms[0]
				ctx, eiec, pre := admitLazyTurn(t, f, m)
				require.NoError(t, pre.PreEval(ctx, eiec))
			}
			type query struct {
				sql  string
				vars []any
			}
			var captured []query
			require.NoError(t, f.sql.Callback().Query().After("gorm:query").Register("hook-index-explain", func(tx *gorm.DB) {
				switch tx.Statement.Table {
				case "expt_item_result_run_log", "expt_turn_result_run_log", "expt_item_result", "expt_turn_result", "expt_turn_evaluator_result_ref":
					captured = append(captured, query{tx.Statement.SQL.String(), append([]any(nil), tx.Statement.Vars...)})
				}
			}))
			if empty {
				require.NoError(t, expireZeroTimeout(context.Background(), scheduler, f, m))
				archiveZeroTimeout(t, f, m, scheduler)
			} else {
				_, err := f.deps.Repository.(repo.IHookItemArchiveRepo).ReadHookArchiveItem(context.Background(), f.key, "local", m.Frozen.ItemID)
				require.NoError(t, err)
			}
			require.NoError(t, f.sql.Callback().Query().Remove("hook-index-explain"))
			require.NotEmpty(t, captured)
			seen := map[string]bool{}
			for _, q := range captured {
				var plans []struct {
					Table    string  `gorm:"column:table"`
					Access   string  `gorm:"column:type"`
					Possible *string `gorm:"column:possible_keys"`
					Key      *string `gorm:"column:key"`
				}
				require.NoError(t, f.sql.Raw("EXPLAIN "+q.sql, q.vars...).Scan(&plans).Error)
				for _, p := range plans {
					t.Logf("SQL=%s PLAN table=%s access=%s possible=%v key=%v", q.sql, p.Table, p.Access, gptr.Indirect(p.Possible), gptr.Indirect(p.Key))
					if p.Table == "" {
						continue
					}
					seen[p.Table] = true
					require.NotEqual(t, "ALL", strings.ToUpper(p.Access), "archive must not scan/lock the global table")
					require.NotEmpty(t, gptr.Indirect(p.Possible))
				}
			}
			if empty {
				for _, table := range []string{"expt_item_result", "expt_turn_result", "expt_turn_evaluator_result_ref"} {
					require.True(t, seen[table], "must explain actual no-execution table %s", table)
				}
			}
		})
	}
}

type indexedReplayProbeKey struct{}

func TestHookIndexedConcurrentInitializerSeesLatestPins(t *testing.T) {
	f, ms := activeTerminationFixture(t, entity.ItemRunState_Queueing)
	m := lazyTurnManifest(t, f, ms[0], 2)
	ctx, eiec, pre := admitLazyTurn(t, f, m)
	initializer := ctx.Value(itemHookProgressContextKey{}).(itemHookProgressBinding).repo.(repo.IHookTurnLogInitializer)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	entered, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	var paused atomic.Bool
	require.NoError(t, f.sql.Callback().Query().After("gorm:query").Register("indexed-old-probe", func(tx *gorm.DB) {
		if tx.Statement.Context.Value(indexedReplayProbeKey{}) != nil && strings.Contains(tx.Statement.SQL.String(), "`before_enabled`") && paused.CompareAndSwap(false, true) {
			close(entered)
			select {
			case <-release:
			case <-ctx.Done():
				tx.AddError(ctx.Err())
			}
		}
	}))
	type result struct {
		rows []*entity.ExptTurnResultRunLog
		err  error
	}
	done := make(chan result, 1)
	go func() {
		_, rows, err := initializer.InitializeHookTurnRunLogs(context.WithValue(ctx, indexedReplayProbeKey{}, true), f.key, m.Frozen.ItemID, m.Frozen.ItemVersionID, nil)
		done <- result{rows, err}
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("immutable probe barrier not reached")
	}
	createErr := pre.PreEval(ctx, eiec)
	unblock()
	var replay result
	select {
	case replay = <-done:
	case <-ctx.Done():
		t.Fatal("initializer replay did not finish")
	}
	require.NoError(t, f.sql.Callback().Query().Remove("indexed-old-probe"))
	require.NoError(t, createErr)
	require.NoError(t, replay.err, "old consistent view must not hide committed pins")
	require.Len(t, replay.rows, 2)
	for _, row := range replay.rows {
		require.Equal(t, eiec.ExistItemEvalResult.TurnResultRunLogs[row.TurnID].ID, row.ID)
	}
	assertLazyTurnCount(t, f, 2)
}

func TestHookIndexedPinWriteRollsBackWithCreation(t *testing.T) {
	f, ms := activeTerminationFixture(t, entity.ItemRunState_Queueing)
	ctx, eiec, pre := admitLazyTurn(t, f, ms[0])
	require.NoError(t, f.sql.Callback().Update().Before("gorm:update").Register("indexed-pin-failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "expt_lifecycle_run_item" {
			tx.AddError(errors.New("pin update unavailable"))
		}
	}))
	err := pre.PreEval(ctx, eiec)
	require.NoError(t, f.sql.Callback().Update().Remove("indexed-pin-failure"))
	require.Error(t, err)
	assertLazyTurnCount(t, f, 0)
	var ledger model.ExptLifecycleRunItem
	require.NoError(t, f.sql.First(&ledger, ms[0].Frozen.ID).Error)
	var m entity.HookExecutionManifest
	require.NoError(t, json.Unmarshal(gptr.Indirect(ledger.ExecutionManifest), &m))
	require.NotNil(t, m.TurnLogsInitialized)
	require.False(t, *m.TurnLogsInitialized)
	require.Zero(t, m.Turns[0].RunLogID)
}

func TestHookIndexedLegacyNilCompatibility(t *testing.T) {
	for _, kind := range []string{"complete", "empty", "missing", "foreign", "deleted"} {
		t.Run(kind, func(t *testing.T) {
			f, ms := activeTerminationFixture(t, entity.ItemRunState_Queueing)
			m := lazyTurnManifest(t, f, ms[0], 2)
			ctx, eiec, pre := admitLazyTurn(t, f, m)
			if kind != "empty" {
				require.NoError(t, pre.PreEval(ctx, eiec))
			}
			var logs []model.ExptTurnResultRunLog
			require.NoError(t, f.sql.Where("space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, f.key.RunID).Order("turn_id").Find(&logs).Error)
			if kind == "missing" {
				require.NoError(t, f.sql.Unscoped().Delete(&logs[0]).Error)
			}
			if kind == "foreign" {
				logs[0].SpaceID++
				require.NoError(t, f.sql.Save(&logs[0]).Error)
				t.Cleanup(func() { require.NoError(t, f.sql.Unscoped().Delete(&logs[0]).Error) })
			}
			if kind == "deleted" {
				require.NoError(t, f.sql.Delete(&logs[0]).Error)
			}
			m.TurnLogsInitialized = nil
			for i := range m.Turns {
				m.Turns[i].RunLogID = 0
			}
			raw, err := json.Marshal(m)
			require.NoError(t, err)
			require.NoError(t, f.sql.Model(&model.ExptLifecycleRunItem{}).Where("id=?", m.Frozen.ID).UpdateColumn("execution_manifest", raw).Error)
			_, err = f.deps.Repository.(repo.IHookItemArchiveRepo).ReadHookArchiveItem(ctx, f.key, "local", m.Frozen.ItemID)
			if kind != "complete" {
				require.Error(t, err, "unknown legacy absence cannot prove no execution")
				return
			}
			require.NoError(t, err)
			initializer := ctx.Value(itemHookProgressContextKey{}).(itemHookProgressBinding).repo.(repo.IHookTurnLogInitializer)
			_, rows, err := initializer.InitializeHookTurnRunLogs(ctx, f.key, m.Frozen.ItemID, m.Frozen.ItemVersionID, nil)
			require.NoError(t, err)
			require.Len(t, rows, 2)
			var ledger model.ExptLifecycleRunItem
			require.NoError(t, f.sql.First(&ledger, m.Frozen.ID).Error)
			var pinned entity.HookExecutionManifest
			require.NoError(t, json.Unmarshal(gptr.Indirect(ledger.ExecutionManifest), &pinned))
			require.NotNil(t, pinned.TurnLogsInitialized)
			require.True(t, *pinned.TurnLogsInitialized)
			for i, tr := range pinned.Turns {
				require.Equal(t, logs[i].ID, tr.RunLogID)
			}
		})
	}
}
