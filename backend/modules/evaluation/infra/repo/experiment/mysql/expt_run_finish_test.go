// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package mysql

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Removing the primary pending recheck would close a run with newly queued work.
func TestRunFinishMySQLPendingDoesNotWrite(t *testing.T) {
	for _, pending := range []struct {
		name     string
		status   entity.ItemRunState
		state    int32
		reserved int32
	}{
		{"queueing", entity.ItemRunState_Queueing, 0, 0}, {"reserved", entity.ItemRunState_Queueing, 0, 1},
		{"processing", entity.ItemRunState_Processing, 0, 0}, {"logged", entity.ItemRunState_Fail, 2, 0},
	} {
		t.Run(pending.name, func(t *testing.T) {
			f := newManualRetryFixture(t)
			require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND expt_id=?", f.space, f.expt).Updates(map[string]any{"status": int32(entity.ItemRunState_Success), "result_state": 1}).Error)
			require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, 11).Updates(map[string]any{"status": int32(pending.status), "result_state": pending.state, "quota_reservation_state": pending.reserved}).Error)
			before := f.snapshot()
			result, err := NewExptRunFinishDAO(f.provider).FinishCurrentRun(context.Background(), &entity.ExptRunFinishRequest{SpaceID: f.space, ExptID: f.expt, RunID: f.run})
			require.NoError(t, err)
			require.Equal(t, entity.ExptRunFinishPending, result.Disposition)
			require.Equal(t, before, f.snapshot())
		})
	}
}

func terminalFinishFixture(t *testing.T) *manualRetryFixture {
	f := newManualRetryFixture(t)
	require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND expt_id=?", f.space, f.expt).Updates(map[string]any{"status": int32(entity.ItemRunState_Success)}).Error)
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND expt_id=?", f.space, f.expt).Updates(map[string]any{"status": int32(entity.ItemRunState_Success), "result_state": 1}).Error)
	return f
}
func finishFixture(f *manualRetryFixture) (*entity.ExptRunFinishResult, error) {
	return NewExptRunFinishDAO(f.provider).FinishCurrentRun(context.Background(), &entity.ExptRunFinishRequest{SpaceID: f.space, ExptID: f.expt, RunID: f.run})
}

// Canonical turn counts span historical success, while item counts retain the
// experiment projection; credit/token and opaque receipt bytes are untouched.
func TestRunFinishMySQLFinalStatsAndIdempotency(t *testing.T) {
	f := terminalFinishFixture(t)
	require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, 11).Update("status", int32(entity.ItemRunState_Fail)).Error)
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, 11).Update("status", int32(entity.ItemRunState_Fail)).Error)
	require.NoError(t, f.conn.Model(&model.ExptTurnResult{}).Where("space_id=? AND turn_id=?", f.space, 22).Update("status", int32(entity.TurnRunState_Queueing)).Error)
	require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, 12).Updates(map[string]any{"expt_run_id": f.run - 1}).Error)
	require.NoError(t, f.conn.Where("space_id=? AND item_id=?", f.space, 12).Delete(&model.ExptItemResultRunLog{}).Error)
	require.NoError(t, f.conn.Create(&model.ExptTurnResult{ID: f.base + 9501, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run - 1, ItemID: 12, ItemVersionID: f.base + 112, TurnID: 0, Status: int32(entity.TurnRunState_Success), WeightedScore: nil}).Error)
	require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, 13).Update("status", int32(entity.ItemRunState_Terminal)).Error)
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, 13).Update("status", int32(entity.ItemRunState_Terminal)).Error)
	require.NoError(t, f.conn.Create(&model.ExptTurnResult{ID: f.base + 9502, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: 13, ItemVersionID: f.base + 113, TurnID: 0, Status: int32(entity.TurnRunState_Terminal)}).Error)
	before := f.snapshot()
	var originalRun model.ExptRunLog
	require.NoError(t, f.conn.Where("space_id=? AND expt_run_id=?", f.space, f.run).Take(&originalRun).Error)
	result, err := finishFixture(f)
	require.NoError(t, err)
	require.Equal(t, entity.ExptRunFinishFinished, result.Disposition)
	var run model.ExptRunLog
	require.NoError(t, f.conn.Where("space_id=? AND expt_run_id=?", f.space, f.run).Take(&run).Error)
	require.Equal(t, int64(entity.ExptStatus_Failed), gptr.Indirect(run.Status))
	require.Equal(t, int32(1), run.PendingCnt)
	require.Equal(t, int32(1), run.SuccessCnt)
	require.Equal(t, int32(1), run.FailCnt)
	require.Equal(t, int32(1), run.TerminatedCnt)
	require.Equal(t, float64(42), run.CreditCost)
	require.Equal(t, int64(123), gptr.Indirect(run.TokenCost))
	require.Equal(t, gptr.Indirect(originalRun.ItemIds), gptr.Indirect(run.ItemIds))
	require.Contains(t, string(gptr.Indirect(run.ItemIds)), `"future":{"id":9223372036854775806}`)
	var stats model.ExptStats
	require.NoError(t, f.conn.Where("space_id=? AND expt_id=?", f.space, f.expt).Take(&stats).Error)
	require.Equal(t, int32(1), stats.SuccessCnt)
	require.Equal(t, int32(1), stats.FailCnt)
	require.Equal(t, int32(1), stats.TerminatedCnt)
	require.Zero(t, stats.PendingCnt)
	require.Equal(t, float64(42), stats.CreditCost)
	require.Equal(t, int64(100), gptr.Indirect(stats.InputTokenCost))
	after := f.snapshot()
	for _, table := range []string{"expt_item_result", "expt_item_result_run_log", "expt_turn_result", "expt_turn_result_run_log", "expt_turn_evaluator_result_ref"} {
		require.Equal(t, before[table], after[table], table)
	}
	var writes atomic.Int32
	require.NoError(t, f.conn.Callback().Update().Before("gorm:update").Register("finish_second_write", func(*gorm.DB) { writes.Add(1) }))
	result, err = finishFixture(f)
	require.NoError(t, err)
	require.Equal(t, entity.ExptRunFinishAlreadyFinished, result.Disposition)
	require.Zero(t, writes.Load())
	require.Equal(t, after, f.snapshot())
}

func TestRunFinishMySQLTerminalItemsAreNotExperimentFailures(t *testing.T) {
	f := terminalFinishFixture(t)
	require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=?", f.space).Update("status", int32(entity.ItemRunState_Terminal)).Error)
	require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=?", f.space).Update("status", int32(entity.ItemRunState_Terminal)).Error)
	require.NoError(t, f.conn.Model(&model.ExptTurnResult{}).Where("space_id=?", f.space).Update("status", int32(entity.TurnRunState_Terminal)).Error)
	result, err := finishFixture(f)
	require.NoError(t, err)
	require.Equal(t, entity.ExptStatus_Success, result.Experiment.Status)
}

type finishFaultProvider struct{ db.Provider }

func (p finishFaultProvider) Transaction(ctx context.Context, fn func(*gorm.DB) error, opts ...db.Option) error {
	return p.Provider.Transaction(ctx, func(tx *gorm.DB) error {
		committer := tx.Statement.ConnPool.(gorm.TxCommitter)
		tx.Statement.ConnPool = &finishCommitFaultConnection{ConnPool: tx.Statement.ConnPool, TxCommitter: committer}
		return fn(tx)
	}, opts...)
}
func TestRunFinishMySQLRollbackEveryWriteAndCommit(t *testing.T) {
	for _, table := range []string{"expt_run_log", "experiment", "expt_stats", "commit"} {
		t.Run(table, func(t *testing.T) {
			f := terminalFinishFixture(t)
			before := f.snapshot()
			var provider db.Provider = f.provider
			if table == "commit" {
				provider = finishFaultProvider{provider}
			} else {
				require.NoError(t, f.conn.Callback().Update().Before("gorm:update").Register("finish_fault", func(tx *gorm.DB) {
					if tx.Statement.Table == table {
						tx.AddError(errors.New("finish write fault"))
					}
				}))
			}
			result, err := NewExptRunFinishDAO(provider).FinishCurrentRun(context.Background(), &entity.ExptRunFinishRequest{SpaceID: f.space, ExptID: f.expt, RunID: f.run})
			require.Error(t, err)
			require.Nil(t, result)
			require.Equal(t, before, f.snapshot())
		})
	}
}
func TestRunFinishMySQLMissingBindingAndHalfTerminalFailClosed(t *testing.T) {
	for _, which := range []string{"missing_run", "missing_experiment", "wrong_space", "noncurrent", "half_run", "half_expt", "missing_stats"} {
		t.Run(which, func(t *testing.T) {
			f := terminalFinishFixture(t)
			switch which {
			case "missing_run":
				require.NoError(t, f.conn.Where("space_id=?", f.space).Delete(&model.ExptRunLog{}).Error)
			case "missing_experiment":
				require.NoError(t, f.conn.Where("space_id=?", f.space).Delete(&model.Experiment{}).Error)
			case "wrong_space":
				require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=?", f.space).Update("space_id", f.space+100).Error)
				t.Cleanup(func() { f.conn.Exec("DELETE FROM expt_run_log WHERE space_id=? AND expt_id=?", f.space+100, f.expt) })
			case "noncurrent":
				require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).Update("latest_run_id", f.run+100).Error)
			case "half_run":
				require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("space_id=?", f.space).Update("status", int64(entity.ExptStatus_Success)).Error)
			case "half_expt":
				require.NoError(t, f.conn.Model(&model.Experiment{}).Where("space_id=?", f.space).Updates(map[string]any{"status": int32(entity.ExptStatus_Success), "end_at": time.Now()}).Error)
			case "missing_stats":
				require.NoError(t, f.conn.Where("space_id=?", f.space).Delete(&model.ExptStats{}).Error)
			}
			before := f.snapshot()
			r, err := finishFixture(f)
			if which == "noncurrent" {
				require.NoError(t, err)
				require.Equal(t, entity.ExptRunFinishNonCurrent, r.Disposition)
			} else {
				require.Error(t, err)
				require.Nil(t, r)
			}
			require.Equal(t, before, f.snapshot())
		})
	}
}

// Both participants execute real provider transactions. The first holds its
// experiment/run locks until the second has attempted admission or finish.
type finishBarrierProvider struct {
	db.Provider
	written chan struct{}
	release chan struct{}
}

func (p finishBarrierProvider) Transaction(ctx context.Context, fn func(*gorm.DB) error, opts ...db.Option) error {
	return p.Provider.Transaction(ctx, func(tx *gorm.DB) error {
		if err := fn(tx); err != nil {
			return err
		}
		close(p.written)
		<-p.release
		return nil
	}, opts...)
}
func TestRunFinishMySQLConcurrentAdmissionAndReceiptReplay(t *testing.T) {
	for _, first := range []string{"admission", "finish"} {
		t.Run(first, func(t *testing.T) {
			f := terminalFinishFixture(t)
			require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, 11).Update("status", int32(entity.ItemRunState_Fail)).Error)
			require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, 11).Update("status", int32(entity.ItemRunState_Fail)).Error)
			request := f.request(entity.EvaluationModeRetryItems)
			written, release := make(chan struct{}), make(chan struct{})
			barrier := finishBarrierProvider{Provider: f.provider, written: written, release: release}
			done1, done2 := make(chan error, 1), make(chan error, 1)
			var finished *entity.ExptRunFinishResult
			var receipt *entity.ManualRetryReceipt
			if first == "admission" {
				port := f.port.(*ExptManualRetryDAOImpl)
				copyPort := *port
				copyPort.provider = barrier
				go func() {
					var err error
					receipt, err = copyPort.RequeueTerminalItems(context.Background(), request)
					done1 <- err
				}()
				<-written
				go func() { var err error; finished, err = finishFixture(f); done2 <- err }()
			} else {
				go func() {
					var err error
					finished, err = NewExptRunFinishDAO(barrier).FinishCurrentRun(context.Background(), &entity.ExptRunFinishRequest{SpaceID: f.space, ExptID: f.expt, RunID: f.run})
					done1 <- err
				}()
				<-written
				go func() {
					var err error
					receipt, err = f.port.RequeueTerminalItems(context.Background(), request)
					done2 <- err
				}()
			}
			select {
			case err := <-done2:
				close(release)
				t.Fatalf("second transaction ignored first experiment/run locks: %v", err)
			case <-time.After(40 * time.Millisecond):
			}
			close(release)
			require.NoError(t, <-done1)
			err2 := <-done2
			if first == "admission" {
				require.NoError(t, err2)
				require.Equal(t, int32(1), receipt.AcceptedCount)
				require.Equal(t, entity.ExptRunFinishPending, finished.Disposition)
				after := f.snapshot()
				again, err := f.port.RequeueTerminalItems(context.Background(), request)
				require.NoError(t, err)
				originalRaw, _ := json.Marshal(receipt)
				replayRaw, _ := json.Marshal(again)
				require.Equal(t, string(originalRaw), string(replayRaw))
				require.Equal(t, after, f.snapshot())
				require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, 11).Update("status", int32(entity.ItemRunState_Success)).Error)
				require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, 11).Updates(map[string]any{"status": int32(entity.ItemRunState_Success), "result_state": 1}).Error)
				finished, err = finishFixture(f)
				require.NoError(t, err)
				require.Equal(t, entity.ExptRunFinishFinished, finished.Disposition)
				after = f.snapshot()
				again, err = f.port.RequeueTerminalItems(context.Background(), request)
				require.NoError(t, err)
				originalRaw, _ = json.Marshal(receipt)
				replayRaw, _ = json.Marshal(again)
				require.Equal(t, string(originalRaw), string(replayRaw))
				require.Equal(t, after, f.snapshot())
			} else {
				require.Error(t, err2)
				require.Nil(t, receipt)
				require.Equal(t, entity.ExptRunFinishFinished, finished.Disposition)
			}
		})
	}
}

// Real SQL runs on the real transaction; the final Commit boundary fails after
// rolling it back, just as a server that rejects COMMIT before durable commit.
type finishCommitFaultConnection struct {
	gorm.ConnPool
	gorm.TxCommitter
}

func (c *finishCommitFaultConnection) Commit() error {
	_ = c.TxCommitter.Rollback()
	return errors.New("commit fault at transaction commit boundary")
}

func TestRunFinishMySQLCanonicalBindingCorruptDoesNotFinish(t *testing.T) {
	for _, kind := range []string{"missing_item", "wrong_run", "wrong_version", "unpublished_terminal"} {
		t.Run(kind, func(t *testing.T) {
			f := terminalFinishFixture(t)
			switch kind {
			case "missing_item":
				require.NoError(t, f.conn.Where("space_id=? AND item_id=?", f.space, 11).Delete(&model.ExptItemResult{}).Error)
			case "wrong_run":
				require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND item_id=?", f.space, 11).Update("expt_run_id", f.run-1).Error)
			case "wrong_version":
				require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, 11).Update("item_version_id", f.base+999).Error)
			case "unpublished_terminal":
				require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND item_id=?", f.space, 11).Update("result_state", 0).Error)
			}
			before := f.snapshot()
			result, err := finishFixture(f)
			require.Error(t, err)
			require.Nil(t, result)
			require.Equal(t, before, f.snapshot())
		})
	}
}

func TestRunFinishMySQLLocksExperimentThenRunAndStats(t *testing.T) {
	f := terminalFinishFixture(t)
	var queries []string
	require.NoError(t, f.conn.Callback().Query().After("gorm:query").Register("finish_lock_sequence", func(tx *gorm.DB) {
		sql := tx.Statement.SQL.String()
		if strings.Contains(sql, "FOR UPDATE") {
			queries = append(queries, sql)
		}
	}))
	result, err := finishFixture(f)
	require.NoError(t, err)
	require.Equal(t, entity.ExptRunFinishFinished, result.Disposition)
	require.GreaterOrEqual(t, len(queries), 5)
	require.Contains(t, queries[0], "`experiment`")
	require.Contains(t, queries[1], "`expt_run_log`")
	require.Contains(t, queries[len(queries)-1], "`expt_stats`")
	for _, sql := range queries {
		require.NotContains(t, sql, "SLEEP")
		require.Contains(t, sql, "FOR UPDATE")
	}
}
