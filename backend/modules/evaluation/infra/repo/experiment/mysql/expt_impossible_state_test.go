// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	drivermysql "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

func newImpossibleStateFixture(t *testing.T, token string) *currentResultFixture {
	t.Helper()
	f := newCurrentResultFixture(t, token)
	for _, table := range []string{"expt_item_result", "expt_item_result_run_log", "expt_turn_result", "expt_turn_result_run_log"} {
		require.NoError(t, f.conn.Table(table).Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, f.item).Update("status", 0).Error)
	}
	require.NoError(t, f.conn.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", f.space, f.expt).Updates(map[string]any{"pending_cnt": 2, "processing_cnt": 1}).Error)
	return f
}

func impossibleStateWrite() *entity.ExptCurrentItemRunWrite {
	return &entity.ExptCurrentItemRunWrite{OnlyIfQueueingUnreserved: true, Status: entity.ItemRunState_Fail, ErrMsg: []byte("impossible quota"), RetryTimes: 1, FailPendingTurns: true}
}

func requireImpossibleStats(t *testing.T, f *currentResultFixture, pending, processing, failed int32) {
	t.Helper()
	var stats model.ExptStats
	require.NoError(t, f.conn.Where("space_id=? AND expt_id=?", f.space, f.expt).Take(&stats).Error)
	require.Equal(t, pending, stats.PendingCnt)
	require.Equal(t, processing, stats.ProcessingCnt)
	require.Equal(t, failed, stats.FailCnt)
	require.Zero(t, stats.SuccessCnt)
	require.Zero(t, stats.TerminatedCnt)
}

func prepareImpossibleMissingTurns(t *testing.T, f *currentResultFixture, source *entity.ExptCurrentItemRunSource, write *entity.ExptCurrentItemRunWrite) {
	t.Helper()
	for index, turn := range source.CanonicalTurns {
		log := turn.ToRunLogDO()
		log.ID = f.base + 900 + int64(index)
		log.ExptRunID = f.run
		log.Status = entity.TurnRunState_Fail
		write.MissingTurnLogs = append(write.MissingTurnLogs, log)
	}
}

// Guarded Q/None completion keeps the existing Logged, turn and stats contract.
func TestImpossibleStateMySQLQueueingUnreserved(t *testing.T) {
	for _, kind := range []string{"raw", "rq1", "raw_zero_ids", "raw_missing_turns", "successful_turn"} {
		t.Run(kind, func(t *testing.T) {
			token := ""
			if kind == "rq1" || kind == "successful_turn" {
				token = "rq1"
			}
			f := newImpossibleStateFixture(t, token)
			if kind == "raw_zero_ids" {
				f.version = 0
				for _, table := range []string{"expt_item_result", "expt_item_result_run_log", "expt_turn_result", "expt_turn_result_run_log", "expt_item_ref", "eval_target_record", "evaluator_record"} {
					require.NoError(t, f.conn.Table(table).Where("space_id=? AND item_id=?", f.space, f.item).Update("item_version_id", 0).Error)
				}
				for _, table := range []string{"expt_turn_result", "expt_turn_result_run_log"} {
					require.NoError(t, f.conn.Table(table).Where("space_id=? AND item_id=?", f.space, f.item).Update("target_result_id", 0).Error)
				}
			}
			if kind == "raw_missing_turns" {
				require.NoError(t, f.conn.Unscoped().Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, f.item).Delete(&model.ExptTurnResultRunLog{}).Error)
			}
			if kind == "successful_turn" {
				for _, table := range []string{"expt_turn_result", "expt_turn_result_run_log"} {
					require.NoError(t, f.conn.Table(table).Where("space_id=? AND item_id=? AND turn_id=0", f.space, f.item).Update("status", int32(entity.TurnRunState_Success)).Error)
				}
			}
			source := f.source()
			require.Equal(t, token, source.Current.RequestID)
			if kind == "raw_zero_ids" {
				require.Zero(t, source.Current.ItemVersionID)
				require.NotNil(t, source.Turns[0])
				require.Zero(t, source.Turns[0].TargetResultID)
			}
			write := impossibleStateWrite()
			if kind == "raw_missing_turns" {
				prepareImpossibleMissingTurns(t, f, source, write)
			}
			before := f.snapshot()
			applied, err := f.port.CompleteCurrentItemRun(context.Background(), source, write)
			require.NoError(t, err)
			require.True(t, applied)
			var item model.ExptItemResult
			var log model.ExptItemResultRunLog
			require.NoError(t, f.conn.Where("space_id=? AND item_id=?", f.space, f.item).Take(&item).Error)
			require.NoError(t, f.conn.Where("space_id=? AND item_id=?", f.space, f.item).Take(&log).Error)
			require.Equal(t, int32(entity.ItemRunState_Fail), item.Status)
			require.Equal(t, int32(entity.ItemRunState_Fail), log.Status)
			require.Equal(t, int32(entity.ExptItemResultStateLogged), gptr.Indirect(log.ResultState))
			require.Zero(t, log.QuotaReservationState)
			require.Equal(t, int32(1), log.RetryTimes)
			require.Equal(t, write.ErrMsg, gptr.Indirect(item.ErrMsg))
			require.Equal(t, write.ErrMsg, gptr.Indirect(log.ErrMsg))
			var turns []model.ExptTurnResultRunLog
			require.NoError(t, f.conn.Where("space_id=? AND item_id=?", f.space, f.item).Find(&turns).Error)
			require.Len(t, turns, 2)
			for _, turn := range turns {
				want := entity.TurnRunState_Fail
				if kind == "successful_turn" && turn.TurnID == 0 {
					want = entity.TurnRunState_Success
				}
				require.Equal(t, int32(want), turn.Status)
				require.Equal(t, f.version, turn.ItemVersionID)
			}
			requireImpossibleStats(t, f, 1, 1, 1)
			after := f.snapshot()
			for _, table := range []string{"experiment", "expt_turn_result", "expt_turn_evaluator_result_ref", "expt_item_ref", "expt_run_log", "eval_target_record", "evaluator_record"} {
				require.Equal(t, before[table], after[table], table)
			}
			applied, err = f.port.CompleteCurrentItemRun(context.Background(), source, write)
			require.NoError(t, err)
			require.False(t, applied)
			require.Equal(t, after, f.snapshot(), "repeat failure must leave all eleven tables unchanged")
			t.Logf("REAL_MYSQL_IMPOSSIBLE_QNONE kind=%s Logged=true stats_once=true snapshots=11", kind)
		})
	}
}

// Every part of Q/None is required even if the observed token and refs match.
func TestImpossibleStateMySQLFreshPredicateZeroWrites(t *testing.T) {
	for _, kind := range []string{"canonical_processing", "log_processing", "reserved", "unknown_quota", "wrong_token", "wrong_version", "changed_target", "changed_ref", "canonical_success", "canonical_fail", "canonical_terminal", "log_success", "log_fail", "log_terminal", "raw_missing_claimed"} {
		t.Run(kind, func(t *testing.T) {
			token := "rq1"
			if kind == "raw_missing_claimed" {
				token = ""
			}
			f := newImpossibleStateFixture(t, token)
			if kind == "raw_missing_claimed" {
				require.NoError(t, f.conn.Unscoped().Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, f.item).Delete(&model.ExptTurnResultRunLog{}).Error)
			}
			source := f.source()
			write := impossibleStateWrite()
			switch kind {
			case "canonical_processing", "canonical_success", "canonical_fail", "canonical_terminal":
				states := map[string]entity.ItemRunState{"canonical_processing": entity.ItemRunState_Processing, "canonical_success": entity.ItemRunState_Success, "canonical_fail": entity.ItemRunState_Fail, "canonical_terminal": entity.ItemRunState_Terminal}
				require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("status", int32(states[kind])).Error)
			case "log_processing", "log_success", "log_fail", "log_terminal":
				states := map[string]entity.ItemRunState{"log_processing": entity.ItemRunState_Processing, "log_success": entity.ItemRunState_Success, "log_fail": entity.ItemRunState_Fail, "log_terminal": entity.ItemRunState_Terminal}
				require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("status", int32(states[kind])).Error)
			case "reserved", "unknown_quota":
				quota := int32(entity.QuotaReservationStateReserved)
				if kind == "unknown_quota" {
					quota = 99
				}
				require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("quota_reservation_state", quota).Error)
			case "wrong_token":
				f.token("rq2")
			case "wrong_version":
				source.Current.ItemVersionID++
			case "changed_target":
				require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND turn_id=0", f.space).Update("target_result_id", f.base+42).Error)
			case "changed_ref":
				require.NoError(t, f.conn.Model(&model.ExptTurnEvaluatorResultRef{}).Where("space_id=?", f.space).Update("evaluator_result_id", f.base+62).Error)
			case "raw_missing_claimed":
				claimed, err := NewExptItemDispatchDAO(f.provider).ClaimQuotaReserved(context.Background(), f.space, f.expt, f.run, []int64{f.item}, map[int64]string{f.item: token})
				require.NoError(t, err)
				require.Len(t, claimed, 1)
				// Invalid preparation must not even be inspected after the predicate rejects.
				write.MissingTurnLogs = []*entity.ExptTurnResultRunLog{nil}
			}
			before := f.snapshot()
			applied, err := f.port.CompleteCurrentItemRun(context.Background(), source, write)
			require.NoError(t, err)
			require.False(t, applied)
			require.Equal(t, before, f.snapshot(), "predicate rejection must make zero writes across all eleven tables")
			t.Logf("REAL_MYSQL_IMPOSSIBLE_REJECT kind=%s snapshots=11", kind)
		})
	}
}

// The optional flag must not narrow ordinary Processing completion or retries.
func TestImpossibleStateMySQLDefaultFalseCompatibility(t *testing.T) {
	for _, kind := range []string{"success", "fail", "yield", "retry_only", "retry_only_unchanged"} {
		t.Run(kind, func(t *testing.T) {
			f := newImpossibleStateFixture(t, "rq1")
			source := f.source()
			dispatch := NewExptItemDispatchDAO(f.provider)
			claimed, err := dispatch.ClaimQuotaReserved(context.Background(), f.space, f.expt, f.run, []int64{f.item}, map[int64]string{f.item: "rq1"})
			require.NoError(t, err)
			require.Len(t, claimed, 1)
			started, err := dispatch.StartReservedItem(context.Background(), f.space, f.expt, f.run, f.item, "rq1")
			require.NoError(t, err)
			require.True(t, started.Started)
			write := &entity.ExptCurrentItemRunWrite{Status: entity.ItemRunState_Fail, RetryTimes: 2, FailPendingTurns: true}
			wantStatus, wantResult, wantRetry := entity.ItemRunState_Fail, entity.ExptItemResultStateLogged, int32(2)
			switch kind {
			case "success":
				write.Status, write.FailPendingTurns = entity.ItemRunState_Success, false
				wantStatus = entity.ItemRunState_Success
			case "yield":
				write.Yield = true
				wantStatus, wantResult = entity.ItemRunState_Queueing, entity.ExptItemResultStateDefault
			case "retry_only", "retry_only_unchanged":
				write.RetryOnly = true
				wantStatus, wantResult = entity.ItemRunState_Processing, entity.ExptItemResultStateDefault
				if kind == "retry_only_unchanged" {
					write.RetryTimes, wantRetry = 1, 1
				}
			}
			before := f.snapshot()
			applied, err := f.port.CompleteCurrentItemRun(context.Background(), source, write)
			require.NoError(t, err)
			require.True(t, applied)
			var log model.ExptItemResultRunLog
			require.NoError(t, f.conn.Where("space_id=? AND item_id=?", f.space, f.item).Take(&log).Error)
			require.Equal(t, int32(wantStatus), log.Status)
			require.Equal(t, int32(wantResult), gptr.Indirect(log.ResultState))
			require.Equal(t, wantRetry, log.RetryTimes)
			if kind == "retry_only_unchanged" {
				require.Equal(t, before, f.snapshot())
			}
			t.Logf("REAL_MYSQL_IMPOSSIBLE_DEFAULT_FALSE kind=%s same_token_started_source=true", kind)
		})
	}
}

func TestImpossibleStateMySQLRollback(t *testing.T) {
	for _, table := range []string{"expt_turn_result_run_log_create", "expt_turn_result_run_log_create_second", "expt_item_result_run_log", "expt_turn_result_run_log", "expt_item_result", "expt_stats"} {
		t.Run(table, func(t *testing.T) {
			f := newImpossibleStateFixture(t, "")
			createFault := table == "expt_turn_result_run_log_create" || table == "expt_turn_result_run_log_create_second"
			if createFault {
				require.NoError(t, f.conn.Unscoped().Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, f.item).Delete(&model.ExptTurnResultRunLog{}).Error)
			}
			source := f.source()
			write := impossibleStateWrite()
			if createFault {
				prepareImpossibleMissingTurns(t, f, source, write)
			}
			before := f.snapshot()
			injected := errors.New("impossible completion write fault")
			hits := 0
			hook := "impossible_fault_" + table
			callback := func(tx *gorm.DB) {
				if tx.Statement.Table == table || createFault && tx.Statement.Table == "expt_turn_result_run_log" {
					hits++
					_, transactional := tx.Statement.ConnPool.(*sql.Tx)
					require.True(t, transactional, "fault must hit the real SQL transaction")
					if table != "expt_turn_result_run_log_create_second" || hits == 2 {
						_ = tx.AddError(injected)
					}
				}
			}
			if createFault {
				require.NoError(t, f.conn.Callback().Create().Before("gorm:create").Register(hook, callback))
			} else {
				require.NoError(t, f.conn.Callback().Update().Before("gorm:update").Register(hook, callback))
			}
			applied, err := f.port.CompleteCurrentItemRun(context.Background(), source, write)
			if createFault {
				require.NoError(t, f.conn.Callback().Create().Remove(hook))
			} else {
				require.NoError(t, f.conn.Callback().Update().Remove(hook))
			}
			require.Positive(t, hits)
			require.ErrorIs(t, err, injected)
			require.False(t, applied)
			require.Equal(t, before, f.snapshot())
			t.Logf("REAL_MYSQL_IMPOSSIBLE_ROLLBACK point=%s snapshots=11", table)
		})
	}
}

// Hold the canonical lock in the winning operation's actual transaction, then
// run Claim/Start and guarded Fail in two goroutines. Both serializations matter.
func TestImpossibleStateMySQLCanonicalLockRace(t *testing.T) {
	for _, token := range []string{"", "rq1"} {
		for _, first := range []string{"fail", "claim"} {
			t.Run(token+"/"+first, func(t *testing.T) {
				f := newImpossibleStateFixture(t, token)
				source := f.source()
				before := f.snapshot()
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				ready, release := make(chan struct{}), make(chan struct{})
				var releaseOnce sync.Once
				defer releaseOnce.Do(func() { close(release) })
				hook := "impossible_lock_" + first
				var once sync.Once
				require.NoError(t, f.conn.Callback().Query().After("gorm:query").Register(hook, func(tx *gorm.DB) {
					if tx.Statement.Table != "expt_item_result" || tx.Error != nil {
						return
					}
					if _, locked := tx.Statement.Clauses["FOR"]; !locked {
						return
					}
					_, transactional := tx.Statement.ConnPool.(*sql.Tx)
					if !transactional {
						return
					}
					once.Do(func() {
						close(ready)
						select {
						case <-release:
						case <-ctx.Done():
							_ = tx.AddError(ctx.Err())
						}
					})
				}))
				defer func() { require.NoError(t, f.conn.Callback().Query().Remove(hook)) }()
				type failResult struct {
					applied bool
					err     error
				}
				type claimResult struct {
					claimed []repo.ClaimedDispatchItem
					started repo.StartReservedItemResult
					err     error
				}
				failed, claimed := make(chan failResult, 1), make(chan claimResult, 1)
				fail := func() {
					applied, err := f.port.CompleteCurrentItemRun(ctx, source, impossibleStateWrite())
					failed <- failResult{applied, err}
				}
				claim := func() {
					dispatch := NewExptItemDispatchDAO(f.provider)
					result := claimResult{}
					result.claimed, result.err = dispatch.ClaimQuotaReserved(ctx, f.space, f.expt, f.run, []int64{f.item}, map[int64]string{f.item: token})
					if result.err == nil {
						result.started, result.err = dispatch.StartReservedItem(ctx, f.space, f.expt, f.run, f.item, token)
					}
					claimed <- result
				}
				if first == "fail" {
					go fail()
				} else {
					go claim()
				}
				select {
				case <-ready:
				case <-ctx.Done():
					t.Fatal("winning operation did not lock the canonical MySQL row")
				}
				// A real NOWAIT read proves the winner currently holds the row.
				lockedErr := f.provider.Transaction(ctx, func(tx *gorm.DB) error {
					var item model.ExptItemResult
					return tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "NOWAIT"}).Where("space_id=? AND item_id=?", f.space, f.item).Take(&item).Error
				}, db.WithMaster())
				require.Error(t, lockedErr, "canonical row lock must be held by the first goroutine")
				var mysqlErr *drivermysql.MySQLError
				require.ErrorAs(t, lockedErr, &mysqlErr)
				require.Equal(t, uint16(3572), mysqlErr.Number, "NOWAIT must report the real held row lock")
				if first == "fail" {
					go claim()
				} else {
					go fail()
				}
				releaseOnce.Do(func() { close(release) })
				var failure failResult
				var takeover claimResult
				select {
				case failure = <-failed:
				case <-ctx.Done():
					t.Fatal("guarded failure did not finish")
				}
				select {
				case takeover = <-claimed:
				case <-ctx.Done():
					t.Fatal("Claim/Start did not finish")
				}
				require.NoError(t, failure.err)
				require.NoError(t, takeover.err)
				if first == "fail" {
					require.True(t, failure.applied)
					require.Empty(t, takeover.claimed)
					require.False(t, takeover.started.CurrentRequest)
					require.False(t, takeover.started.Started)
					requireImpossibleStats(t, f, 1, 1, 1)
				} else {
					require.False(t, failure.applied)
					require.Len(t, takeover.claimed, 1)
					require.True(t, takeover.started.CurrentRequest)
					require.True(t, takeover.started.Started)
					requireImpossibleStats(t, f, 1, 2, 0)
				}
				after := f.snapshot()
				for _, table := range []string{"experiment", "expt_turn_evaluator_result_ref", "expt_item_ref", "expt_run_log", "eval_target_record", "evaluator_record"} {
					require.Equal(t, before[table], after[table], table)
				}
				if first == "fail" {
					require.Equal(t, before["expt_turn_result"], after["expt_turn_result"])
				}
				var item model.ExptItemResult
				var log model.ExptItemResultRunLog
				require.NoError(t, f.conn.Where("space_id=? AND item_id=?", f.space, f.item).Take(&item).Error)
				require.NoError(t, f.conn.Where("space_id=? AND item_id=?", f.space, f.item).Take(&log).Error)
				wantStatus, wantResult := entity.ItemRunState_Processing, entity.ExptItemResultStateDefault
				if first == "fail" {
					wantStatus, wantResult = entity.ItemRunState_Fail, entity.ExptItemResultStateLogged
				}
				require.Equal(t, int32(wantStatus), item.Status)
				require.Equal(t, int32(wantStatus), log.Status)
				require.Equal(t, int32(wantResult), gptr.Indirect(log.ResultState))
				require.Zero(t, log.QuotaReservationState)
				t.Logf("REAL_MYSQL_IMPOSSIBLE_LOCK_RACE first=%s token=%q canonical_NOWAIT_blocked=true snapshots=11 stats_once=true", first, token)
			})
		}
	}
}

// A same-token Claim/Start must supersede an earlier impossible observation.
// Removing the transaction's fresh Q/None predicate must make this test fail.
func TestImpossibleStateMySQLSameTokenTakeover(t *testing.T) {
	for _, token := range []string{"", "rq1"} {
		for _, start := range []bool{false, true} {
			name := "claimed"
			if start {
				name = "started"
			}
			t.Run(token+"/"+name, func(t *testing.T) {
				f := newImpossibleStateFixture(t, token)
				source := f.source()
				require.Equal(t, entity.ItemRunState_Queueing, source.Current.Status)
				dispatch := NewExptItemDispatchDAO(f.provider)
				claimed, err := dispatch.ClaimQuotaReserved(context.Background(), f.space, f.expt, f.run, []int64{f.item}, map[int64]string{f.item: token})
				require.NoError(t, err)
				require.Len(t, claimed, 1)
				require.Equal(t, token, claimed[0].RequestID)
				if start {
					started, err := dispatch.StartReservedItem(context.Background(), f.space, f.expt, f.run, f.item, token)
					require.NoError(t, err)
					require.True(t, started.CurrentRequest)
					require.True(t, started.Started)
				}
				before := f.snapshot()
				applied, err := f.port.CompleteCurrentItemRun(context.Background(), source, &entity.ExptCurrentItemRunWrite{OnlyIfQueueingUnreserved: true, Status: entity.ItemRunState_Fail, ErrMsg: []byte("impossible quota"), RetryTimes: 1, FailPendingTurns: true})
				require.NoError(t, err)
				assert.False(t, applied, "same-token execution took over the earlier Q/None observation")
				after := f.snapshot()
				for table, snapshot := range before {
					assert.True(t, snapshot == after[table], "table %s changed after same-token takeover", table)
				}
			})
		}
	}
}
