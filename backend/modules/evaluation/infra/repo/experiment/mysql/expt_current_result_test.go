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
