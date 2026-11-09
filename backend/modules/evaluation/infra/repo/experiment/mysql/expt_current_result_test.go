// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

type currentResultFixture struct {
	t                                     *testing.T
	provider                              db.Provider
	conn                                  *gorm.DB
	port                                  repo.IExptCurrentResultRepo
	base, space, expt, run, item, version int64
}

func newCurrentResultFixture(t *testing.T, token string) *currentResultFixture {
	t.Helper()
	dsn := os.Getenv("EXPT_RETRY_MYSQL_DSN")
	if dsn == "" {
		t.Skip("EXPT_RETRY_MYSQL_DSN missing; REAL MYSQL NOT EXECUTED")
	}
	provider, err := db.NewDB(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	conn := provider.NewSession(context.Background(), db.WithMaster())
	var identity struct{ Version, Schema string }
	require.NoError(t, conn.Raw("SELECT VERSION() AS version,DATABASE() AS `schema`").Scan(&identity).Error)
	require.Equal(t, "expt_running_retry_test", identity.Schema)
	t.Logf("REAL_MYSQL_EXECUTED version=%s schema=%s", identity.Version, identity.Schema)
	base := int64(7384603622000000000) + time.Now().UnixNano()%1000000000000 + retryFixtureSeq.Add(1)*10000
	f := &currentResultFixture{t: t, provider: provider, conn: conn, base: base, space: base + 1, expt: base + 2, run: base + 3, item: base + 11, version: base + 31}
	t.Cleanup(func() {
		for _, table := range []string{"expt_turn_evaluator_result_ref", "expt_turn_result_run_log", "expt_turn_result", "expt_item_result_run_log", "expt_item_result", "expt_item_ref", "expt_run_log", "expt_stats"} {
			require.NoError(t, conn.Exec("DELETE FROM "+table+" WHERE space_id=? AND expt_id=?", f.space, f.expt).Error)
		}
		require.NoError(t, conn.Exec("DELETE FROM experiment WHERE space_id=? AND id=?", f.space, f.expt).Error)
		for _, table := range []string{"eval_target_record", "evaluator_record"} {
			require.NoError(t, conn.Exec("DELETE FROM "+table+" WHERE space_id=?", f.space).Error)
		}
		sqlDB, err := conn.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})
	require.NoError(t, conn.Exec("INSERT INTO experiment (id,space_id,latest_run_id,status,expt_type) VALUES (?,?,?,?,?)", f.expt, f.space, f.run, entity.ExptStatus_Processing, entity.ExptType_Offline).Error)
	require.NoError(t, conn.Create(&model.ExptRunLog{ID: f.run, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, Status: gptr.Of(int64(entity.ExptStatus_Processing)), Mode: gptr.Of(int32(entity.EvaluationModeSubmit)), ItemIds: gptr.Of([]byte(`[{"ItemIDs":[],"CreateAt":1700000000}]`))}).Error)
	require.NoError(t, conn.Create(&model.ExptStats{ID: base + 4, SpaceID: f.space, ExptID: f.expt, ProcessingCnt: 2, PendingCnt: 1}).Error)
	for index, status := range []entity.ItemRunState{entity.ItemRunState_Processing, entity.ItemRunState_Processing, entity.ItemRunState_Queueing} {
		item := f.item + int64(index)
		require.NoError(t, conn.Create(&model.ExptItemResult{ID: base + 100 + int64(index), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: item, ItemVersionID: f.version, Status: int32(status), Ext: gptr.Of([]byte(`{"other":"keep"}`))}).Error)
		require.NoError(t, conn.Create(&model.ExptItemResultRunLog{ID: base + 200 + int64(index), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: item, ItemVersionID: f.version, Status: int32(status), RetryTimes: 1, ResultState: gptr.Of(int32(entity.ExptItemResultStateDefault))}).Error)
		require.NoError(t, conn.Create(&model.ExptItemRef{ID: base + 300 + int64(index), SpaceID: f.space, ExptID: f.expt, ItemID: item, ItemVersionID: f.version}).Error)
	}
	f.token(token)
	refs, _ := json.Marshal(&entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: base + 51, Alias: "alias", RecordID: base + 61}}, Inline: []*entity.InlineEvalResult{{InlineKey: "inline", RecordID: base + 62}}})
	for index, turn := range []int64{0, base + 22} {
		require.NoError(t, conn.Create(&model.ExptTurnResult{ID: base + 400 + int64(index), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: f.item, ItemVersionID: f.version, TurnID: turn, Status: int32(entity.TurnRunState_Processing), TargetResultID: base + 41, WeightedScore: gptr.Of(0.0)}).Error)
		require.NoError(t, conn.Create(&model.ExptTurnResultRunLog{ID: base + 500 + int64(index), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: f.item, ItemVersionID: f.version, TurnID: turn, Status: int32(entity.TurnRunState_Processing), TargetResultID: base + 41, EvaluatorResultIds: &refs, Ext: []byte(`{"other":"keep"}`)}).Error)
	}
	require.NoError(t, conn.Create(&model.ExptTurnEvaluatorResultRef{ID: base + 600, SpaceID: f.space, ExptID: f.expt, ExptTurnResultID: base + 400, EvaluatorVersionID: base + 51, Alias_: "alias", EvaluatorResultID: base + 61}).Error)
	require.NoError(t, conn.Exec("INSERT INTO eval_target_record (id,space_id,target_id,target_version_id,experiment_run_id,item_id,item_version_id,turn_id,log_id,trace_id,status) VALUES (?,?,?,?,?,?,?,?,?,?,?)", base+41, f.space, base+42, base+43, f.run, f.item, f.version, 0, "target", "trace", 1).Error)
	require.NoError(t, conn.Exec("INSERT INTO evaluator_record (id,space_id,evaluator_version_id,experiment_id,experiment_run_id,item_id,item_version_id,turn_id,trace_id,status,target_record_id) VALUES (?,?,?,?,?,?,?,?,?,?,?)", base+61, f.space, base+51, f.expt, f.run, f.item, f.version, 0, "trace", 1, 0).Error)
	port, ok := any(NewExptItemResultDAO(provider)).(repo.IExptCurrentResultRepo)
	require.True(t, ok, "source conditional execution transaction missing")
	f.port = port
	return f
}

func (f *currentResultFixture) token(token string) {
	f.t.Helper()
	ext := map[string]string{"other": "keep"}
	if token != "" {
		raw, err := json.Marshal(&entity.ManualRetryContext{RunID: f.run, RequestID: token, Mode: entity.EvaluationModeFailRetry, Operator: "test", AcceptedAtMS: 1})
		require.NoError(f.t, err)
		ext[entity.ManualRetryExtKey] = string(raw)
	}
	raw, err := json.Marshal(ext)
	require.NoError(f.t, err)
	require.NoError(f.t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, f.item).Update("ext", raw).Error)
}

func (f *currentResultFixture) source() *entity.ExptCurrentItemRunSource {
	f.t.Helper()
	s, err := f.port.GetCurrentItemRunSource(context.Background(), f.space, f.expt, f.run, f.item)
	require.NoError(f.t, err)
	require.NotNil(f.t, s)
	return s
}

func (f *currentResultFixture) snapshot() map[string]string {
	f.t.Helper()
	out := map[string]string{}
	for _, table := range []string{"experiment", "expt_item_result", "expt_item_result_run_log", "expt_turn_result", "expt_turn_result_run_log", "expt_turn_evaluator_result_ref", "expt_stats", "expt_run_log", "expt_item_ref", "eval_target_record", "evaluator_record"} {
		var rows []map[string]any
		require.NoError(f.t, f.conn.Table(table).Where("space_id=?", f.space).Order("id").Find(&rows).Error)
		raw, err := json.Marshal(rows)
		require.NoError(f.t, err)
		out[table] = string(raw)
	}
	return out
}

func (f *currentResultFixture) result(source *entity.ExptCurrentItemRunSource) *entity.ExptTurnResultRunLog {
	r := *source.Turns[0]
	r.Status = entity.TurnRunState_Success
	r.TargetResultID = f.base + 71
	r.EvaluatorResultIds = &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: f.base + 51, Alias: "alias", RecordID: f.base + 81}}}
	return &r
}

func TestCurrentResultFailRetryReadsSuccessfulHistoryWithNewRunLogs(t *testing.T) {
	provider := db.NewTestDB(t, &model.ExptItemResult{}, &model.ExptItemResultRunLog{}, &model.ExptRunLog{}, &model.ExptTurnResult{}, &model.ExptTurnResultRunLog{}, &model.ExptTurnEvaluatorResultRef{})
	conn := provider.NewSession(context.Background(), db.WithMaster())
	const spaceID, exptID, oldRunID, runID, itemID, versionID = int64(1), int64(2), int64(3), int64(4), int64(5), int64(6)
	require.NoError(t, conn.Create(&model.ExptRunLog{ID: 10, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, Status: gptr.Of(int64(entity.ExptStatus_Processing)), Mode: gptr.Of(int32(entity.EvaluationModeFailRetry))}).Error)
	require.NoError(t, conn.Create(&model.ExptItemResult{ID: 11, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, ItemID: itemID, ItemVersionID: versionID, Status: int32(entity.ItemRunState_Processing)}).Error)
	require.NoError(t, conn.Create(&model.ExptItemResultRunLog{ID: 12, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, ItemID: itemID, ItemVersionID: versionID, Status: int32(entity.ItemRunState_Processing), ResultState: gptr.Of(int32(entity.ExptItemResultStateDefault))}).Error)
	for _, turn := range []model.ExptTurnResult{
		{ID: 21, SpaceID: spaceID, ExptID: exptID, ExptRunID: oldRunID, ItemID: itemID, ItemVersionID: versionID, TurnID: 31, Status: int32(entity.TurnRunState_Success)},
		{ID: 22, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, ItemID: itemID, ItemVersionID: versionID, TurnID: 32, Status: int32(entity.TurnRunState_Fail)},
	} {
		require.NoError(t, conn.Create(&turn).Error)
	}
	for _, log := range []model.ExptTurnResultRunLog{
		{ID: 41, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, ItemID: itemID, ItemVersionID: versionID, TurnID: 31, Status: int32(entity.TurnRunState_Processing)},
		{ID: 42, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, ItemID: itemID, ItemVersionID: versionID, TurnID: 32, Status: int32(entity.TurnRunState_Processing)},
	} {
		require.NoError(t, conn.Create(&log).Error)
	}

	source, err := loadCurrentRunSource(conn, spaceID, exptID, runID, itemID)
	require.NoError(t, err)
	require.NotNil(t, source)
	require.Len(t, source.CanonicalTurns, 2)
	require.Equal(t, oldRunID, source.CanonicalTurns[0].ExptRunID)
	require.Equal(t, runID, source.CanonicalTurns[1].ExptRunID)
	require.Len(t, source.Turns, 2)
	for _, log := range source.Turns {
		require.Equal(t, runID, log.ExptRunID)
	}
}

// Real SQL, not mocked Transaction: late sources must leave all eleven tables byte-for-byte unchanged.
func TestCurrentResultMySQLLateSource(t *testing.T) {
	for _, kind := range []string{"rq2", "same_request_invoke", "empty_to_manual", "old_run", "wrong_version", "alias", "inline", "resulted", "cancelled"} {
		t.Run(kind, func(t *testing.T) {
			token := "rq1"
			if kind == "empty_to_manual" {
				token = ""
			}
			f := newCurrentResultFixture(t, token)
			source := f.source()
			switch kind {
			case "rq2", "empty_to_manual":
				f.token("rq2")
			case "same_request_invoke":
				require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND turn_id=0", f.space).Update("target_result_id", f.base+42).Error)
			case "old_run":
				source.Current.RunID++
			case "wrong_version":
				source.Current.ItemVersionID++
			case "alias", "inline":
				refs := &entity.EvaluatorResults{Registered: []*entity.RegisteredEvalResult{{VersionID: f.base + 51, Alias: "newalias", RecordID: f.base + 61}}, Inline: []*entity.InlineEvalResult{{InlineKey: "newinline", RecordID: f.base + 62}}}
				raw, _ := json.Marshal(refs)
				require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND turn_id=0", f.space).Update("evaluator_result_ids", raw).Error)
			case "resulted":
				require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Updates(map[string]any{"status": int32(entity.ItemRunState_Success), "result_state": int32(entity.ExptItemResultStateResulted)}).Error)
			}
			ctx := context.Background()
			if kind == "cancelled" {
				c, cancel := context.WithCancel(ctx)
				cancel()
				ctx = c
			}
			before := f.snapshot()
			ok, err := f.port.StoreCurrentTurnRunResult(ctx, source, f.result(source))
			if kind == "cancelled" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.False(t, ok)
			require.Equal(t, before, f.snapshot())
			for _, write := range []*entity.ExptCurrentItemRunWrite{{Status: entity.ItemRunState_Fail, ErrMsg: []byte("late error"), FailPendingTurns: true}, {Yield: true, RetryTimes: 2}, {RetryOnly: true, RetryTimes: 3}} {
				ok, err = f.port.CompleteCurrentItemRun(ctx, source, write)
				if kind == "cancelled" {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
				require.False(t, ok)
				require.Equal(t, before, f.snapshot())
			}
		})
	}
}

func TestCurrentResultMySQLCurrentAtomic(t *testing.T) {
	for _, kind := range []string{"store", "complete", "yield", "budget", "first_turn_failed", "terminal", "oldmap", "builtin0"} {
		t.Run(kind, func(t *testing.T) {
			f := newCurrentResultFixture(t, "rq1")
			if kind == "terminal" {
				require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("status", int32(entity.ItemRunState_Terminal)).Error)
			}
			source := f.source()
			before := f.snapshot()
			var ok bool
			var err error
			switch kind {
			case "store", "oldmap", "builtin0":
				result := f.result(source)
				if kind == "oldmap" {
					result.EvaluatorResultIds = &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{f.base + 51: f.base + 81}}
				}
				if kind == "builtin0" {
					result.TargetResultID = 0
				}
				ok, err = f.port.StoreCurrentTurnRunResult(context.Background(), source, result)
			case "yield":
				ok, err = f.port.CompleteCurrentItemRun(context.Background(), source, &entity.ExptCurrentItemRunWrite{Yield: true, RetryTimes: 2, ErrMsg: []byte("retry")})
			case "budget":
				ok, err = f.port.CompleteCurrentItemRun(context.Background(), source, &entity.ExptCurrentItemRunWrite{RetryOnly: true, RetryTimes: 3})
			default:
				ok, err = f.port.CompleteCurrentItemRun(context.Background(), source, &entity.ExptCurrentItemRunWrite{Status: entity.ItemRunState_Fail, RetryTimes: 1, ErrMsg: []byte("failed"), FailPendingTurns: true})
			}
			require.NoError(t, err)
			require.True(t, ok)
			after := f.snapshot()
			require.NotEqual(t, before, after)
			for _, table := range []string{"experiment", "expt_item_ref", "expt_run_log", "eval_target_record", "evaluator_record", "expt_turn_evaluator_result_ref"} {
				require.Equal(t, before[table], after[table], table)
			}
			var log model.ExptItemResultRunLog
			require.NoError(t, f.conn.Where("space_id=? AND item_id=?", f.space, f.item).Take(&log).Error)
			if kind == "yield" {
				require.Equal(t, int32(2), log.RetryTimes)
				require.Equal(t, int32(entity.ItemRunState_Queueing), log.Status)
			}
			if kind == "budget" {
				require.Equal(t, int32(3), log.RetryTimes)
			}
			if kind == "complete" || kind == "first_turn_failed" || kind == "terminal" {
				require.Equal(t, int32(entity.ExptItemResultStateLogged), gptr.Indirect(log.ResultState))
				expected := entity.ItemRunState_Fail
				if kind == "terminal" {
					expected = entity.ItemRunState_Terminal
				}
				require.Equal(t, int32(expected), log.Status)
				source.Current.Status = expected
				before = f.snapshot()
				ok, err = f.port.CompleteCurrentItemRun(context.Background(), source, &entity.ExptCurrentItemRunWrite{Status: expected, RetryTimes: 1})
				require.NoError(t, err)
				require.False(t, ok)
				require.Equal(t, before, f.snapshot())
			}
		})
	}
}

func TestCurrentResultMySQLRollback(t *testing.T) {
	for _, kind := range []string{"store", "complete", "yield", "budget"} {
		for _, table := range []string{"expt_item_result_run_log", "expt_turn_result_run_log", "expt_turn_result", "expt_item_result", "expt_stats"} {
			if kind == "store" && table != "expt_turn_result_run_log" || kind == "budget" && table != "expt_item_result_run_log" || table == "expt_turn_result" && kind != "yield" {
				continue
			}
			t.Run(kind+"/"+table, func(t *testing.T) {
				f := newCurrentResultFixture(t, "rq1")
				source := f.source()
				before := f.snapshot()
				injected := errors.New("injected current write fault")
				hook := fmt.Sprintf("fault_%s_%s", kind, table)
				hits := 0
				require.NoError(t, f.conn.Callback().Update().Before("gorm:update").Register(hook, func(tx *gorm.DB) {
					if tx.Statement.Table == table {
						hits++
						_ = tx.AddError(injected)
					}
				}))
				var ok bool
				var err error
				if kind == "store" {
					ok, err = f.port.StoreCurrentTurnRunResult(context.Background(), source, f.result(source))
				} else {
					ok, err = f.port.CompleteCurrentItemRun(context.Background(), source, &entity.ExptCurrentItemRunWrite{Status: entity.ItemRunState_Fail, FailPendingTurns: true, Yield: kind == "yield", RetryOnly: kind == "budget", RetryTimes: 3})
				}
				require.NoError(t, f.conn.Callback().Update().Remove(hook))
				require.Positive(t, hits, "actual writepoint not exercised")
				require.ErrorIs(t, err, injected)
				require.False(t, ok)
				require.Equal(t, before, f.snapshot())
			})
		}
	}
}

func TestCurrentResultMySQLConcurrentAdmission(t *testing.T) {
	f := newCurrentResultFixture(t, "rq1")
	source := f.source()
	var wg sync.WaitGroup
	wg.Add(1)
	ready := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer wg.Done()
		require.NoError(t, f.provider.Transaction(context.Background(), func(tx *gorm.DB) error {
			var row model.ExptItemResult
			if err := tx.Set("gorm:query_option", "FOR UPDATE").Raw("SELECT * FROM expt_item_result WHERE space_id=? AND item_id=? FOR UPDATE", f.space, f.item).Scan(&row).Error; err != nil {
				return err
			}
			ext := []byte(fmt.Sprintf(`{"manual_retry":"{\"run_id\":%d,\"request_id\":\"rq2\",\"mode\":2,\"operator\":\"test\",\"accepted_at_ms\":1}"}`, f.run))
			if err := tx.Model(&row).Update("ext", ext).Error; err != nil {
				return err
			}
			close(ready)
			<-done
			return nil
		}, db.WithMaster()))
	}()
	<-ready
	result := make(chan error, 1)
	go func() {
		ok, err := f.port.StoreCurrentTurnRunResult(context.Background(), source, f.result(source))
		if ok {
			err = errors.New("late store wrote after admission")
		}
		result <- err
	}()
	time.Sleep(30 * time.Millisecond)
	close(done)
	wg.Wait()
	require.NoError(t, <-result)
}

// A raw pre-execution failure still needs a log for every canonical turn to archive.
func TestCurrentResultMySQLRawMissingTurnLogs(t *testing.T) {
	f := newCurrentResultFixture(t, "")
	require.NoError(t, f.conn.Unscoped().Where("space_id=?", f.space).Delete(&model.ExptTurnResultRunLog{}).Error)
	source := f.source()
	before := f.snapshot()
	applied, err := f.port.CompleteCurrentItemRun(context.Background(), source, &entity.ExptCurrentItemRunWrite{Status: entity.ItemRunState_Fail, ErrMsg: []byte("pre-exec error"), FailPendingTurns: true})
	require.Error(t, err, "incomplete raw failure cannot silently create Logged with no turns")
	require.False(t, applied)
	require.Equal(t, before, f.snapshot())
}

func TestCurrentResultMySQLRawPreparedMissingTurns(t *testing.T) {
	for _, fault := range []bool{false, true} {
		t.Run(fmt.Sprint(fault), func(t *testing.T) {
			f := newCurrentResultFixture(t, "")
			require.NoError(t, f.conn.Unscoped().Where("space_id=?", f.space).Delete(&model.ExptTurnResultRunLog{}).Error)
			source := f.source()
			write := &entity.ExptCurrentItemRunWrite{Status: entity.ItemRunState_Fail, ErrMsg: []byte("original pre-exec error"), FailPendingTurns: true}
			for index, turn := range source.CanonicalTurns {
				log := turn.ToRunLogDO()
				log.ID = f.base + 900 + int64(index)
				log.ExptRunID = f.run
				log.Status = entity.TurnRunState_Fail
				write.MissingTurnLogs = append(write.MissingTurnLogs, log)
			}
			before := f.snapshot()
			injected := errors.New("raw missing log write fault")
			if fault {
				require.NoError(t, f.conn.Callback().Create().Before("gorm:create").Register("missing_raw_log_fault", func(tx *gorm.DB) {
					if tx.Statement.Table == "expt_turn_result_run_log" {
						_ = tx.AddError(injected)
					}
				}))
			}
			applied, err := f.port.CompleteCurrentItemRun(context.Background(), source, write)
			if fault {
				require.NoError(t, f.conn.Callback().Create().Remove("missing_raw_log_fault"))
				require.ErrorIs(t, err, injected)
				require.False(t, applied)
				require.Equal(t, before, f.snapshot())
				return
			}
			require.NoError(t, err)
			require.True(t, applied)
			var logs []model.ExptTurnResultRunLog
			require.NoError(t, f.conn.Where("space_id=?", f.space).Find(&logs).Error)
			require.Len(t, logs, 2)
			for _, log := range logs {
				require.Equal(t, f.version, log.ItemVersionID)
				require.Equal(t, int32(entity.TurnRunState_Fail), log.Status)
			}
		})
	}
}

// FailRetry may hold the old run's non-successful canonical turn while the new
// run already owns the turn run log. That is a normal retry transition, not a
// damaged source, and must not block the whole run scan.
func TestCurrentResultFailRetryRetriedOldCanonicalTurn(t *testing.T) {
	provider := db.NewTestDB(t, &model.ExptItemResult{}, &model.ExptItemResultRunLog{}, &model.ExptRunLog{}, &model.ExptTurnResult{}, &model.ExptTurnResultRunLog{}, &model.ExptTurnEvaluatorResultRef{})
	conn := provider.NewSession(context.Background(), db.WithMaster())
	const spaceID, exptID, oldRunID, runID, itemID, versionID = int64(1), int64(2), int64(3), int64(4), int64(5), int64(6)
	require.NoError(t, conn.Create(&model.ExptRunLog{ID: 10, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, Status: gptr.Of(int64(entity.ExptStatus_Processing)), Mode: gptr.Of(int32(entity.EvaluationModeFailRetry))}).Error)
	require.NoError(t, conn.Create(&model.ExptItemResult{ID: 11, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, ItemID: itemID, ItemVersionID: versionID, Status: int32(entity.ItemRunState_Processing)}).Error)
	require.NoError(t, conn.Create(&model.ExptItemResultRunLog{ID: 12, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, ItemID: itemID, ItemVersionID: versionID, Status: int32(entity.ItemRunState_Processing), ResultState: gptr.Of(int32(entity.ExptItemResultStateDefault))}).Error)
	require.NoError(t, conn.Create(&model.ExptTurnResult{ID: 21, SpaceID: spaceID, ExptID: exptID, ExptRunID: oldRunID, ItemID: itemID, ItemVersionID: versionID, TurnID: 31, Status: int32(entity.TurnRunState_Fail)}).Error)
	require.NoError(t, conn.Create(&model.ExptTurnResultRunLog{ID: 41, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, ItemID: itemID, ItemVersionID: versionID, TurnID: 31, Status: int32(entity.TurnRunState_Processing)}).Error)

	source, err := loadCurrentRunSource(conn, spaceID, exptID, runID, itemID)
	require.NoError(t, err)
	require.NotNil(t, source)
	require.Len(t, source.CanonicalTurns, 1)
	require.Equal(t, oldRunID, source.CanonicalTurns[0].ExptRunID)
	require.Len(t, source.Turns, 1)
	require.Equal(t, int32(entity.TurnRunState_Processing), int32(source.Turns[31].Status))
}

// Legacy canonical turns may carry item_version_id=0. A non-zero mismatch stays
// fail-closed; zero is the only historical value that may disagree.
func TestCurrentResultLegacyZeroVersionSource(t *testing.T) {
	provider := db.NewTestDB(t, &model.ExptItemResult{}, &model.ExptItemResultRunLog{}, &model.ExptRunLog{}, &model.ExptTurnResult{}, &model.ExptTurnResultRunLog{}, &model.ExptTurnEvaluatorResultRef{})
	conn := provider.NewSession(context.Background(), db.WithMaster())
	const spaceID, exptID, oldRunID, runID, itemID, versionID = int64(1), int64(2), int64(3), int64(4), int64(5), int64(6)
	require.NoError(t, conn.Create(&model.ExptRunLog{ID: 10, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, Status: gptr.Of(int64(entity.ExptStatus_Processing)), Mode: gptr.Of(int32(entity.EvaluationModeFailRetry))}).Error)
	require.NoError(t, conn.Create(&model.ExptItemResult{ID: 11, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, ItemID: itemID, ItemVersionID: versionID, Status: int32(entity.ItemRunState_Processing)}).Error)
	require.NoError(t, conn.Create(&model.ExptItemResultRunLog{ID: 12, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, ItemID: itemID, ItemVersionID: versionID, Status: int32(entity.ItemRunState_Processing), ResultState: gptr.Of(int32(entity.ExptItemResultStateDefault))}).Error)
	require.NoError(t, conn.Create(&model.ExptTurnResult{ID: 21, SpaceID: spaceID, ExptID: exptID, ExptRunID: oldRunID, ItemID: itemID, ItemVersionID: 0, TurnID: 31, Status: int32(entity.TurnRunState_Success)}).Error)
	require.NoError(t, conn.Create(&model.ExptTurnResultRunLog{ID: 41, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, ItemID: itemID, ItemVersionID: versionID, TurnID: 31, Status: int32(entity.TurnRunState_Processing)}).Error)

	source, err := loadCurrentRunSource(conn, spaceID, exptID, runID, itemID)
	require.NoError(t, err)
	require.NotNil(t, source)
	require.Len(t, source.CanonicalTurns, 1)
	require.Zero(t, source.CanonicalTurns[0].ItemVersionID)
}

func TestCurrentResultRejectsNonZeroVersionMismatch(t *testing.T) {
	provider := db.NewTestDB(t, &model.ExptItemResult{}, &model.ExptItemResultRunLog{}, &model.ExptRunLog{}, &model.ExptTurnResult{}, &model.ExptTurnResultRunLog{}, &model.ExptTurnEvaluatorResultRef{})
	conn := provider.NewSession(context.Background(), db.WithMaster())
	const spaceID, exptID, oldRunID, runID, itemID, versionID = int64(1), int64(2), int64(3), int64(4), int64(5), int64(6)
	require.NoError(t, conn.Create(&model.ExptRunLog{ID: 10, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, Status: gptr.Of(int64(entity.ExptStatus_Processing)), Mode: gptr.Of(int32(entity.EvaluationModeFailRetry))}).Error)
	require.NoError(t, conn.Create(&model.ExptItemResult{ID: 11, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, ItemID: itemID, ItemVersionID: versionID, Status: int32(entity.ItemRunState_Processing)}).Error)
	require.NoError(t, conn.Create(&model.ExptItemResultRunLog{ID: 12, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, ItemID: itemID, ItemVersionID: versionID, Status: int32(entity.ItemRunState_Processing), ResultState: gptr.Of(int32(entity.ExptItemResultStateDefault))}).Error)
	require.NoError(t, conn.Create(&model.ExptTurnResult{ID: 21, SpaceID: spaceID, ExptID: exptID, ExptRunID: oldRunID, ItemID: itemID, ItemVersionID: versionID + 1, TurnID: 31, Status: int32(entity.TurnRunState_Success)}).Error)
	require.NoError(t, conn.Create(&model.ExptTurnResultRunLog{ID: 41, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, ItemID: itemID, ItemVersionID: versionID, TurnID: 31, Status: int32(entity.TurnRunState_Processing)}).Error)

	source, err := loadCurrentRunSource(conn, spaceID, exptID, runID, itemID)
	require.Error(t, err)
	require.Nil(t, source)
	require.True(t, isCurrentSourceInconsistent(err))
}

func TestCurrentResultRejectsUnknownTurnLog(t *testing.T) {
	provider := db.NewTestDB(t, &model.ExptItemResult{}, &model.ExptItemResultRunLog{}, &model.ExptRunLog{}, &model.ExptTurnResult{}, &model.ExptTurnResultRunLog{}, &model.ExptTurnEvaluatorResultRef{})
	conn := provider.NewSession(context.Background(), db.WithMaster())
	const spaceID, exptID, runID, itemID, versionID = int64(1), int64(2), int64(3), int64(5), int64(6)
	require.NoError(t, conn.Create(&model.ExptRunLog{ID: 10, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, Status: gptr.Of(int64(entity.ExptStatus_Processing)), Mode: gptr.Of(int32(entity.EvaluationModeSubmit))}).Error)
	require.NoError(t, conn.Create(&model.ExptItemResult{ID: 11, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, ItemID: itemID, ItemVersionID: versionID, Status: int32(entity.ItemRunState_Processing)}).Error)
	require.NoError(t, conn.Create(&model.ExptItemResultRunLog{ID: 12, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, ItemID: itemID, ItemVersionID: versionID, Status: int32(entity.ItemRunState_Processing), ResultState: gptr.Of(int32(entity.ExptItemResultStateDefault))}).Error)
	require.NoError(t, conn.Create(&model.ExptTurnResult{ID: 21, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, ItemID: itemID, ItemVersionID: versionID, TurnID: 31, Status: int32(entity.TurnRunState_Processing)}).Error)
	require.NoError(t, conn.Create(&model.ExptTurnResultRunLog{ID: 41, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, ItemID: itemID, ItemVersionID: versionID, TurnID: 32, Status: int32(entity.TurnRunState_Processing)}).Error)

	source, err := loadCurrentRunSource(conn, spaceID, exptID, runID, itemID)
	require.Error(t, err)
	require.Nil(t, source)
	require.True(t, isCurrentSourceInconsistent(err), "foreign/unknown turn must stay fail-closed but isolated by scan")
}

// One inconsistent item must be isolated with a warning; a healthy sibling item
// must still be returned so the scheduler can continue dispatching it.
func TestCurrentRequestScanIsolatesInconsistentItem(t *testing.T) {
	provider := db.NewTestDB(t,
		&model.Experiment{}, &model.ExptRunLog{}, &model.ExptItemResult{}, &model.ExptItemResultRunLog{},
		&model.ExptTurnResult{}, &model.ExptTurnResultRunLog{}, &model.ExptTurnEvaluatorResultRef{}, &model.ExptItemRef{}, &model.ExptStats{})
	conn := provider.NewSession(context.Background(), db.WithMaster())
	const spaceID, exptID, runID, versionID = int64(1), int64(2), int64(3), int64(6)
	require.NoError(t, conn.Create(&model.Experiment{ID: exptID, SpaceID: spaceID, LatestRunID: runID, Status: int32(entity.ExptStatus_Processing), ExptType: int32(entity.ExptType_Offline)}).Error)
	require.NoError(t, conn.Create(&model.ExptRunLog{ID: 10, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, Status: gptr.Of(int64(entity.ExptStatus_Processing)), Mode: gptr.Of(int32(entity.EvaluationModeSubmit))}).Error)
	const healthyItem, brokenItem = int64(21), int64(22)
	for index, itemID := range []int64{healthyItem, brokenItem} {
		require.NoError(t, conn.Create(&model.ExptItemResult{ID: int64(100 + index), SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, ItemID: itemID, ItemVersionID: versionID, Status: int32(entity.ItemRunState_Processing)}).Error)
		require.NoError(t, conn.Create(&model.ExptItemResultRunLog{ID: int64(200 + index), SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, ItemID: itemID, ItemVersionID: versionID, Status: int32(entity.ItemRunState_Processing), ResultState: gptr.Of(int32(entity.ExptItemResultStateDefault))}).Error)
	}
	require.NoError(t, conn.Create(&model.ExptTurnResult{ID: 300, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, ItemID: healthyItem, ItemVersionID: versionID, TurnID: 31, Status: int32(entity.TurnRunState_Processing)}).Error)
	require.NoError(t, conn.Create(&model.ExptTurnResultRunLog{ID: 400, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, ItemID: healthyItem, ItemVersionID: versionID, TurnID: 31, Status: int32(entity.TurnRunState_Processing)}).Error)
	// The broken item has a current-run turn log but no canonical turn.
	require.NoError(t, conn.Create(&model.ExptTurnResultRunLog{ID: 401, SpaceID: spaceID, ExptID: exptID, ExptRunID: runID, ItemID: brokenItem, ItemVersionID: versionID, TurnID: 31, Status: int32(entity.TurnRunState_Processing)}).Error)

	rows, err := NewExptItemResultDAO(provider).ScanCurrentRequests(context.Background(), spaceID, exptID, runID, false)
	require.NoError(t, err, "a single inconsistent item must not turn the whole scan into MQ retry")
	require.Len(t, rows, 2)
	byItem := map[int64]*entity.ExptItemCurrentRequest{}
	for _, row := range rows {
		byItem[row.ItemID] = row
	}
	require.NotNil(t, byItem[healthyItem].Source)
	require.False(t, byItem[healthyItem].ObservationUnstable)
	require.Nil(t, byItem[brokenItem].Source)
	require.True(t, byItem[brokenItem].ObservationUnstable)
	require.Equal(t, entity.ItemRunState_Processing, byItem[brokenItem].Status)
}

func TestCurrentResultStorageErrorIsNotConsistencyIsolated(t *testing.T) {
	provider := db.NewTestDB(t, &model.ExptItemResult{})
	conn := provider.NewSession(context.Background(), db.WithMaster())
	require.NoError(t, conn.Create(&model.ExptItemResult{ID: 11, SpaceID: 1, ExptID: 2, ExptRunID: 3, ItemID: 5, ItemVersionID: 6, Status: int32(entity.ItemRunState_Processing)}).Error)

	_, err := loadCurrentRunSource(conn, 1, 2, 3, 5)
	require.Error(t, err)
	require.False(t, isCurrentSourceInconsistent(err), "DB errors must keep propagating")
}

// Zero item versions are a legacy on-disk format: real CN rows were written
// with item_version_id=0 in all four tables (see workers/mq-rollout-baseline.md).
// This matrix pins the read policy on real MySQL: zero never asserts a version,
// an authoritative zero item version cannot vouch for a non-zero row version,
// and any two conflicting non-zero versions stay fail-closed.
func TestCurrentResultZeroVersionMatrixMySQL(t *testing.T) {
	cases := []struct {
		name                        string
		itemVersion, itemLogVersion int64
		turnVersion, turnLogVersion int64
		wantErr                     bool
	}{
		{name: "legacy_all_zero", itemVersion: 0, itemLogVersion: 0, turnVersion: 0, turnLogVersion: 0},
		{name: "item_known_item_log_zero", itemVersion: 1, itemLogVersion: 0, turnVersion: 1, turnLogVersion: 0},
		{name: "item_known_turn_and_log_zero", itemVersion: 1, itemLogVersion: 1, turnVersion: 0, turnLogVersion: 0},
		{name: "item_known_turn_zero_log_known", itemVersion: 1, itemLogVersion: 1, turnVersion: 0, turnLogVersion: 1},
		{name: "item_known_turn_known_log_zero", itemVersion: 1, itemLogVersion: 1, turnVersion: 1, turnLogVersion: 0},
		{name: "item_zero_turn_known", itemVersion: 0, itemLogVersion: 0, turnVersion: 1, turnLogVersion: 1, wantErr: true},
		{name: "item_zero_turn_log_known", itemVersion: 0, itemLogVersion: 0, turnVersion: 0, turnLogVersion: 1, wantErr: true},
		{name: "item_zero_item_log_known", itemVersion: 0, itemLogVersion: 1, turnVersion: 0, turnLogVersion: 0, wantErr: true},
		{name: "foreign_canonical_turn_version", itemVersion: 1, itemLogVersion: 1, turnVersion: 2, turnLogVersion: 1, wantErr: true},
		{name: "foreign_turn_log_version", itemVersion: 1, itemLogVersion: 1, turnVersion: 1, turnLogVersion: 2, wantErr: true},
		{name: "foreign_turn_log_pair", itemVersion: 1, itemLogVersion: 0, turnVersion: 1, turnLogVersion: 2, wantErr: true},
		{name: "foreign_item_run_log_version", itemVersion: 1, itemLogVersion: 2, turnVersion: 1, turnLogVersion: 1, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newCurrentResultFixture(t, "rq1")
			// Anchors are the real fixture version so the "1" and "2" below are
			// concrete non-zero versions relative to each other.
			item, itemLog, turn, turnLog := tc.itemVersion, tc.itemLogVersion, tc.turnVersion, tc.turnLogVersion
			version := f.version
			scale := func(v int64) int64 {
				if v == 0 {
					return 0
				}
				return version + v - 1
			}
			require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("item_version_id", scale(item)).Error)
			require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("item_version_id", scale(itemLog)).Error)
			require.NoError(t, f.conn.Model(&model.ExptTurnResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("item_version_id", scale(turn)).Error)
			require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("item_version_id", scale(turnLog)).Error)

			source, err := f.port.GetCurrentItemRunSource(context.Background(), f.space, f.expt, f.run, f.item)
			if tc.wantErr {
				require.Error(t, err)
				require.Nil(t, source)
				require.True(t, isCurrentSourceInconsistent(err), "snapshot damage must stay isolatable, err=%v", err)
				t.Logf("REAL_MYSQL_ZERO_MATRIX case=%s isolated=true", tc.name)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, source)
			require.Len(t, source.CanonicalTurns, 2)
			require.Len(t, source.Turns, 2)
			t.Logf("REAL_MYSQL_ZERO_MATRIX case=%s isolated=false", tc.name)
		})
	}
}

// Run lifecycle/configuration observations are not one item's snapshot: they
// must keep failing the whole scan instead of being isolated into a silently
// never-finishing run.
func TestCurrentResultRunLevelFailuresStayWholeScanMySQL(t *testing.T) {
	cases := []struct {
		name  string
		apply func(*currentResultFixture)
	}{
		{name: "missing_run_status", apply: func(f *currentResultFixture) {
			require.NoError(t, f.conn.Exec("UPDATE expt_run_log SET status=NULL WHERE space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, f.run).Error)
		}},
		{name: "invalid_run_status", apply: func(f *currentResultFixture) {
			require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, f.run).Update("status", int64(99)).Error)
		}},
		{name: "invalid_run_mode", apply: func(f *currentResultFixture) {
			require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=? AND expt_id=? AND expt_run_id=?", f.space, f.expt, f.run).Update("mode", int32(99)).Error)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// No manual token: manual mode overrides the run mode and would hide
			// an invalid run mode from the run-level check.
			f := newCurrentResultFixture(t, "")
			tc.apply(f)
			for _, load := range []struct {
				name string
				run  func() error
			}{
				{name: "execution", run: func() error {
					_, err := loadCurrentRunSource(f.conn, f.space, f.expt, f.run, f.item)
					return err
				}},
				{name: "publication", run: func() error {
					_, err := readItemResultPublishSourceWithLocks(f.conn, f.space, f.expt, f.run, f.item, false)
					return err
				}},
			} {
				err := load.run()
				require.Error(t, err, load.name)
				require.False(t, isCurrentSourceInconsistent(err), "%s/%s must not be isolated per item: %v", tc.name, load.name, err)
			}
			rows, err := NewExptItemResultDAO(f.provider).ScanCurrentRequests(context.Background(), f.space, f.expt, f.run, false)
			require.Error(t, err, "a run-level failure must fail the whole scan")
			require.Nil(t, rows)
			t.Logf("REAL_MYSQL_RUN_LEVEL case=%s whole_scan_failed=true isolated=false", tc.name)
		})
	}
}

// An isolated item must not stay isolated forever: once the snapshot is stable
// again the next scan returns its source and the frozen publication, and the
// publication can be validated for commit. Healthy siblings are unaffected.
func TestCurrentRequestScanIsolationRecoversMySQL(t *testing.T) {
	f := newCurrentResultFixture(t, "rq1")
	// f.item becomes a Logged publication candidate whose frozen item ref
	// disagrees with the canonical item version.
	require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("status", int32(entity.ItemRunState_Success)).Error)
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Updates(map[string]any{"status": int32(entity.ItemRunState_Success), "result_state": int32(entity.ExptItemResultStateLogged)}).Error)
	require.NoError(t, f.conn.Model(&model.ExptItemRef{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("item_version_id", f.version+1).Error)
	// f.item+1 (Processing) gets a turn run log with no canonical turn.
	require.NoError(t, f.conn.Create(&model.ExptTurnResultRunLog{ID: f.base + 700, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: f.item + 1, ItemVersionID: f.version, TurnID: 777, Status: int32(entity.TurnRunState_Processing)}).Error)

	dao := NewExptItemResultDAO(f.provider)
	tick := func() map[int64]*entity.ExptItemCurrentRequest {
		rows, err := dao.ScanCurrentRequests(context.Background(), f.space, f.expt, f.run, false)
		require.NoError(t, err)
		out := map[int64]*entity.ExptItemCurrentRequest{}
		for _, row := range rows {
			out[row.ItemID] = row
		}
		return out
	}

	first := tick()
	require.NotNil(t, first[f.item])
	require.True(t, first[f.item].PublicationObserved)
	require.Nil(t, first[f.item].Publication, "inconsistent frozen item ref must defer publication")
	require.True(t, first[f.item+1].ObservationUnstable)
	require.Nil(t, first[f.item+1].Source)
	require.Equal(t, entity.ItemRunState_Processing, first[f.item+1].Status)
	// Healthy queueing siblings are still returned for dispatch.
	require.Equal(t, entity.ItemRunState_Queueing, first[f.item+2].Status)
	require.False(t, first[f.item+2].ObservationUnstable)

	// Next stable tick: repair both snapshots.
	require.NoError(t, f.conn.Model(&model.ExptItemRef{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("item_version_id", f.version).Error)
	require.NoError(t, f.conn.Create(&model.ExptTurnResult{ID: f.base + 701, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: f.item + 1, ItemVersionID: f.version, TurnID: 777, Status: int32(entity.TurnRunState_Processing)}).Error)

	second := tick()
	require.NotNil(t, second[f.item])
	require.True(t, second[f.item].PublicationObserved)
	require.NotNil(t, second[f.item].Publication, "publication must be observed again after the snapshot is stable")
	valid, err := NewExptResultPublishDAO(f.provider).IsItemResultPublishSourceCurrent(context.Background(), second[f.item].Publication)
	require.NoError(t, err)
	require.True(t, valid, "recovered publication source must be committable")
	require.NotNil(t, second[f.item+1].Source)
	require.False(t, second[f.item+1].ObservationUnstable)
	require.Equal(t, entity.ItemRunState_Queueing, second[f.item+2].Status)
	t.Log("REAL_MYSQL_ISOLATION_RECOVERY tick1_publication_deferred=true tick1_item_isolated=true tick2_publication_committable=true tick2_item_recovered=true")
}

// Every frozen canonical turn needs exactly one current-run log. The previous
// filtered-length check could silently drop an old-run non-success turn whose
// log was gone; execution and publication must fail closed until it returns.
func TestCurrentResultFrozenTurnCoverageMySQL(t *testing.T) {
	t.Run("preserved_success_with_log", func(t *testing.T) {
		f := newCurrentResultFixture(t, "rq1")
		oldRun := f.run - 1
		require.NoError(t, f.conn.Model(&model.ExptTurnResult{}).Where("space_id=? AND item_id=? AND turn_id=0", f.space, f.item).Updates(map[string]any{"expt_run_id": oldRun, "status": int32(entity.TurnRunState_Success)}).Error)
		source, err := loadCurrentRunSource(f.conn, f.space, f.expt, f.run, f.item)
		require.NoError(t, err)
		require.NotNil(t, source)
		require.Len(t, source.CanonicalTurns, 2)
		require.Equal(t, oldRun, source.CanonicalTurns[0].ExptRunID, "preserved successful history stays in its old run")
		require.NotNil(t, source.Turns[0], "its current-run log is still required")
		t.Log("REAL_MYSQL_FROZEN_TURN case=preserved_success_with_log visible=true")
	})
	t.Run("preserved_success_without_log", func(t *testing.T) {
		f := newCurrentResultFixture(t, "rq1")
		oldRun := f.run - 1
		require.NoError(t, f.conn.Model(&model.ExptTurnResult{}).Where("space_id=? AND item_id=? AND turn_id=0", f.space, f.item).Updates(map[string]any{"expt_run_id": oldRun, "status": int32(entity.TurnRunState_Success)}).Error)
		require.NoError(t, f.conn.Unscoped().Where("space_id=? AND item_id=? AND turn_id=0", f.space, f.item).Delete(&model.ExptTurnResultRunLog{}).Error)
		_, err := loadCurrentRunSource(f.conn, f.space, f.expt, f.run, f.item)
		require.Error(t, err)
		require.True(t, isCurrentSourceInconsistent(err), "a preserved turn without its current log must not be published incomplete")
		t.Log("REAL_MYSQL_FROZEN_TURN case=preserved_success_without_log isolated=true")
	})
	t.Run("missing_turn_log", func(t *testing.T) {
		f := newCurrentResultFixture(t, "rq1")
		source := f.source()
		require.NoError(t, f.conn.Unscoped().Where("space_id=? AND item_id=? AND turn_id=0", f.space, f.item).Delete(&model.ExptTurnResultRunLog{}).Error)
		_, err := loadCurrentRunSource(f.conn, f.space, f.expt, f.run, f.item)
		require.Error(t, err)
		require.True(t, isCurrentSourceInconsistent(err))

		before := f.snapshot()
		applied, err := f.port.StoreCurrentTurnRunResult(context.Background(), source, f.result(source))
		require.Error(t, err)
		require.False(t, applied)
		require.Equal(t, before, f.snapshot(), "execution must not write from an incomplete frozen source")

		rows, err := NewExptItemResultDAO(f.provider).ScanCurrentRequests(context.Background(), f.space, f.expt, f.run, false)
		require.NoError(t, err)
		for _, row := range rows {
			if row.ItemID == f.item {
				require.True(t, row.ObservationUnstable)
				require.Nil(t, row.Source)
			}
		}
		t.Log("REAL_MYSQL_FROZEN_TURN case=missing_turn_log isolated=true execution_writes=0")
	})
	t.Run("damaged_evaluator_refs", func(t *testing.T) {
		f := newCurrentResultFixture(t, "rq1")
		require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND item_id=? AND turn_id=0", f.space, f.item).Update("evaluator_result_ids", []byte(`{"registered":[{"version_id":0,"alias":"alias","record_id":1}]}`)).Error)
		_, err := loadCurrentRunSource(f.conn, f.space, f.expt, f.run, f.item)
		require.Error(t, err)
		require.True(t, isCurrentSourceInconsistent(err), "stored evaluator refs are turn snapshot data: %v", err)
		rows, err := NewExptItemResultDAO(f.provider).ScanCurrentRequests(context.Background(), f.space, f.expt, f.run, false)
		require.NoError(t, err, "one item's damaged refs must not fail the whole scan")
		for _, row := range rows {
			if row.ItemID == f.item {
				require.True(t, row.ObservationUnstable)
				require.Nil(t, row.Source)
			}
		}
		t.Log("REAL_MYSQL_FROZEN_TURN case=damaged_evaluator_refs isolated=true")
	})
	t.Run("foreign_turn_log", func(t *testing.T) {
		f := newCurrentResultFixture(t, "rq1")
		require.NoError(t, f.conn.Create(&model.ExptTurnResultRunLog{ID: f.base + 710, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: f.item, ItemVersionID: f.version, TurnID: 999, Status: int32(entity.TurnRunState_Processing)}).Error)
		_, err := loadCurrentRunSource(f.conn, f.space, f.expt, f.run, f.item)
		require.Error(t, err)
		require.True(t, isCurrentSourceInconsistent(err), "a foreign turn log must stay fail-closed: %v", err)
		t.Log("REAL_MYSQL_FROZEN_TURN case=foreign_turn_log isolated=true")
	})
	t.Run("old_run_frozen_turn_without_log", func(t *testing.T) {
		f := newCurrentResultFixture(t, "rq1")
		oldRun := f.run - 1
		require.NoError(t, f.conn.Model(&model.ExptTurnResult{}).Where("space_id=? AND item_id=? AND turn_id=0", f.space, f.item).Updates(map[string]any{"expt_run_id": oldRun, "status": int32(entity.TurnRunState_Fail)}).Error)
		require.NoError(t, f.conn.Unscoped().Where("space_id=? AND item_id=? AND turn_id=0", f.space, f.item).Delete(&model.ExptTurnResultRunLog{}).Error)
		_, err := loadCurrentRunSource(f.conn, f.space, f.expt, f.run, f.item)
		require.ErrorContains(t, err, "frozen canonical turn has no current run log")
		require.True(t, isCurrentSourceInconsistent(err), "a skipped frozen turn must not disappear silently")
		t.Log("REAL_MYSQL_FROZEN_TURN case=old_run_without_log isolated=true silent_drop=false")
	})
	t.Run("publication_fail_closed", func(t *testing.T) {
		f := newCurrentResultFixture(t, "rq1")
		require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, f.item).Update("status", int32(entity.ItemRunState_Success)).Error)
		require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).Updates(map[string]any{"status": int32(entity.ItemRunState_Success), "result_state": int32(entity.ExptItemResultStateLogged)}).Error)
		pub, ok := any(NewExptResultPublishDAO(f.provider)).(repo.IExptResultPublishRepo)
		require.True(t, ok)
		source, err := pub.GetItemResultPublishSource(context.Background(), f.space, f.expt, f.run, f.item)
		require.NoError(t, err)
		require.NotNil(t, source)
		require.NoError(t, f.conn.Unscoped().Where("space_id=? AND item_id=? AND turn_id=0", f.space, f.item).Delete(&model.ExptTurnResultRunLog{}).Error)
		before := f.snapshot()
		applied, err := pub.CommitItemResult(context.Background(), &entity.ExptItemResultPublication{Source: source})
		require.Error(t, err)
		require.False(t, applied)
		require.Equal(t, before, f.snapshot(), "publication must not commit from an incomplete frozen source")
		t.Log("REAL_MYSQL_FROZEN_TURN case=publication_fail_closed writes=0")
	})
}

func TestCurrentRetryBudgetRefreshesZombieClockOnce(t *testing.T) {
	f := newCurrentResultFixture(t, "")
	old := time.Now().Add(-9 * time.Hour).Truncate(time.Second)
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).UpdateColumn("updated_at", old).Error)
	source := f.source()
	applied, err := f.port.CompleteCurrentItemRun(context.Background(), source, &entity.ExptCurrentItemRunWrite{RetryOnly: true, RetryTimes: source.Current.RetryTimes + 1})
	require.NoError(t, err)
	require.True(t, applied)
	current := f.source()
	require.True(t, current.Current.UpdatedAt.After(old))
	require.Equal(t, entity.ItemRunState_Processing, current.Current.Status)
	first := *current.Current.UpdatedAt
	// Fix the DB clock at an earlier value to detect even a duplicate same-second UPDATE.
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, f.item).UpdateColumn("updated_at", old).Error)
	current = f.source()
	applied, err = f.port.CompleteCurrentItemRun(context.Background(), current, &entity.ExptCurrentItemRunWrite{RetryOnly: true, RetryTimes: current.Current.RetryTimes})
	require.NoError(t, err)
	require.True(t, applied)
	require.True(t, f.source().Current.UpdatedAt.Equal(old), "duplicate retry must not extend the zombie clock")
	require.True(t, first.After(old))
}
