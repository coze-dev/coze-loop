// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	mysqldriver "gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	idgenmocks "github.com/coze-dev/coze-loop/backend/infra/idgen/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

var retryFixtureSeq atomic.Int64

type manualRetryFixture struct {
	t                      *testing.T
	provider               db.Provider
	conn                   *gorm.DB
	port                   repo.IExptManualRetryRepo
	space, expt, run, base int64
}

func newManualRetryFixture(t *testing.T) *manualRetryFixture {
	t.Helper()
	dsn := os.Getenv("EXPT_RETRY_MYSQL_DSN")
	if dsn == "" {
		t.Skip("EXPT_RETRY_MYSQL_DSN not configured; real MySQL integration not executed")
	}
	provider, err := db.NewDB(mysqldriver.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	conn := provider.NewSession(context.Background(), db.WithMaster())
	var identity struct {
		Version string
		Schema  string
	}
	require.NoError(t, conn.Raw("SELECT VERSION() AS version, DATABASE() AS `schema`").Scan(&identity).Error)
	require.Equal(t, "expt_running_retry_test", identity.Schema)
	t.Logf("REAL_MYSQL_EXECUTED version=%s schema=%s", identity.Version, identity.Schema)
	base := int64(860000000000000000) + time.Now().UnixNano()%1000000000000 + retryFixtureSeq.Add(1)*10000
	f := &manualRetryFixture{t: t, provider: provider, conn: conn, space: base + 1, expt: base + 2, run: base + 3, base: base}
	t.Cleanup(func() {
		for _, table := range []string{"expt_turn_evaluator_result_ref", "expt_turn_result_run_log", "expt_turn_result", "expt_item_result_run_log", "expt_item_result", "expt_item_ref", "expt_run_log", "expt_stats"} {
			require.NoError(t, conn.Exec("DELETE FROM "+table+" WHERE space_id = ? AND expt_id = ?", f.space, f.expt).Error)
		}
		require.NoError(t, conn.Exec("DELETE FROM experiment WHERE space_id = ? AND id = ?", f.space, f.expt).Error)
		for _, table := range []string{"eval_target_record", "evaluator_record"} {
			require.NoError(t, conn.Exec("DELETE FROM "+table+" WHERE space_id = ? AND item_id = ?", f.space, 11).Error)
		}
		sqlDB, err := conn.DB()
		require.NoError(t, err)
		require.NoError(t, sqlDB.Close())
	})
	gen := idgenmocks.NewMockIIDGenerator(gomock.NewController(t))
	var inTransaction atomic.Bool
	require.NoError(t, conn.Callback().Query().After("gorm:query").Register("track_manual_retry_transaction", func(tx *gorm.DB) {
		if _, ok := tx.Statement.ConnPool.(*sql.Tx); ok {
			inTransaction.Store(true)
		}
	}))
	gen.EXPECT().GenID(gomock.Any()).DoAndReturn(func(context.Context) (int64, error) {
		require.False(t, inTransaction.Load(), "IDGen may use Redis and must run before SQL transaction")
		return base + 9000 + retryFixtureSeq.Add(1), nil
	}).AnyTimes()
	port, ok := any(&ExptManualRetryDAOImpl{provider: provider, idgen: gen}).(repo.IExptManualRetryRepo)
	require.True(t, ok, "same-run four-table transaction is missing")
	f.port = port
	require.NoError(t, conn.Exec("INSERT INTO experiment (id,space_id,latest_run_id,status,expt_type) VALUES (?,?,?,?,?)", f.expt, f.space, f.run, entity.ExptStatus_Processing, entity.ExptType_Offline).Error)
	raw := `[{"ItemIDs":[11,12,13],"CreateAt":1700000000,"future":{"id":9223372036854775806}}]`
	require.NoError(t, conn.Create(&model.ExptRunLog{ID: f.run, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, Status: gptr.Of(int64(entity.ExptStatus_Processing)), Mode: gptr.Of(int32(entity.EvaluationModeSubmit)), CreatedBy: "original", ItemIds: gptr.Of([]byte(raw)), PendingCnt: 1, ProcessingCnt: 1, FailCnt: 1, CreditCost: 42, TokenCost: gptr.Of(int64(123))}).Error)
	require.NoError(t, conn.Create(&model.ExptStats{ID: base + 4, SpaceID: f.space, ExptID: f.expt, PendingCnt: 1, ProcessingCnt: 1, FailCnt: 1, CreditCost: 42, InputTokenCost: gptr.Of(int64(100)), OutputTokenCost: gptr.Of(int64(23))}).Error)
	for i, status := range []int32{3, 1, 0} {
		item := int64(11 + i)
		version := base + 100 + item
		require.NoError(t, conn.Create(&model.ExptItemResult{ID: base + 200 + item, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: item, ItemVersionID: version, Status: status, ErrMsg: gptr.Of([]byte("old-error")), LogID: "old-item-log", Ext: gptr.Of([]byte(`{"other":"keep","future_id":"9223372036854775806"}`))}).Error)
		result := int32(0)
		if status == 3 {
			result = 1
		}
		require.NoError(t, conn.Create(&model.ExptItemResultRunLog{ID: base + 300 + item, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: item, ItemVersionID: version, Status: status, ResultState: gptr.Of(result), RetryTimes: 3, ErrMsg: gptr.Of([]byte("old-run-error")), LogID: "old-item-run"}).Error)
		require.NoError(t, conn.Create(&model.ExptItemRef{ID: base + 400 + item, SpaceID: f.space, ExptID: f.expt, ItemID: item, ItemVersionID: version, ItemConfig: gptr.Of([]byte(`{"target_source_space_id":19}`))}).Error)
	}
	for i, status := range []int32{2, 1} {
		turn := int64(21 + i)
		target := base + 500 + turn
		require.NoError(t, conn.Create(&model.ExptTurnResult{ID: base + 600 + turn, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: 11, ItemVersionID: base + 111, TurnID: turn, Status: status, TargetResultID: target, WeightedScore: gptr.Of(0.0), ErrMsg: gptr.Of([]byte("turn-error")), TraceID: 55, LogID: "turn-log"}).Error)
		eval, err := json.Marshal(&entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{51: base + 1061, 52: base + 1062}})
		require.NoError(t, err)
		if status == 1 {
			eval, err = json.Marshal(&entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{51: base + 1063}})
			require.NoError(t, err)
		}
		require.NoError(t, conn.Create(&model.ExptTurnResultRunLog{ID: base + 700 + turn, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.run, ItemID: 11, ItemVersionID: base + 111, TurnID: turn, Status: status, TargetResultID: target, EvaluatorResultIds: &eval, ErrMsg: gptr.Of([]byte("turn-run-error")), TraceID: 55, LogID: "turn-run-log", Ext: []byte(`{"other":"keep"}`)}).Error)
		for j, record := range []int64{base + 1061, base + 1062} {
			if status == 1 && j == 1 {
				continue
			}
			if status == 1 {
				record = base + 1063
			}
			require.NoError(t, conn.Create(&model.ExptTurnEvaluatorResultRef{ID: base + 800 + turn*10 + int64(j), SpaceID: f.space, ExptID: f.expt, ExptTurnResultID: base + 600 + turn, EvaluatorVersionID: 51 + int64(j), EvaluatorResultID: record}).Error)
		}
	}
	for _, turn := range []int64{21, 22} {
		require.NoError(t, conn.Exec("INSERT INTO eval_target_record (id,space_id,target_id,target_version_id,experiment_run_id,item_id,item_version_id,turn_id,log_id,trace_id,status) VALUES (?,?,?,?,?,?,?,?,?,?,?)", base+500+turn, f.space, base+41, base+42, f.run, 11, base+111, turn, "target-log", "trace", 1).Error)
	}
	for i, record := range []int64{base + 1061, base + 1062, base + 1063} {
		turn := int64(21)
		if i == 2 {
			turn = 22
		}
		status := 1
		if i == 1 {
			status = 2
		}
		require.NoError(t, conn.Exec("INSERT INTO evaluator_record (id,space_id,evaluator_version_id,experiment_id,experiment_run_id,item_id,item_version_id,turn_id,trace_id,status,score,target_record_id) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)", record, f.space, 51+int64(i%2), f.expt, f.run, 11, base+111, turn, "evaluator-trace", status, nil, 0).Error)
	}
	return f
}

func (f *manualRetryFixture) snapshot() map[string]string {
	f.t.Helper()
	result := make(map[string]string)
	for _, table := range []string{"experiment", "expt_item_result", "expt_item_result_run_log", "expt_turn_result", "expt_turn_result_run_log", "expt_turn_evaluator_result_ref", "expt_stats", "expt_run_log", "expt_item_ref"} {
		idField := "expt_id"
		if table == "experiment" {
			idField = "id"
		}
		rows, err := f.conn.Raw("SELECT * FROM "+table+" WHERE space_id = ? AND "+idField+" = ? ORDER BY id", f.space, f.expt).Rows()
		require.NoError(f.t, err)
		columns, err := rows.Columns()
		require.NoError(f.t, err)
		var values [][]string
		for rows.Next() {
			cells := make([]any, len(columns))
			ptr := make([]any, len(cells))
			for i := range cells {
				ptr[i] = &cells[i]
			}
			require.NoError(f.t, rows.Scan(ptr...))
			converted := make([]string, len(cells))
			for i, cell := range cells {
				if b, ok := cell.([]byte); ok {
					converted[i] = string(b)
				} else {
					b, err := json.Marshal(cell)
					require.NoError(f.t, err)
					converted[i] = string(b)
				}
			}
			values = append(values, converted)
		}
		require.NoError(f.t, rows.Err())
		require.NoError(f.t, rows.Close())
		encoded, err := json.Marshal(values)
		require.NoError(f.t, err)
		result[table] = string(encoded)
	}
	for _, table := range []string{"eval_target_record", "evaluator_record"} {
		var values []map[string]any
		require.NoError(f.t, f.conn.Table(table).Where("space_id=? AND item_id=?", f.space, 11).Order("id").Find(&values).Error)
		encoded, err := json.Marshal(values)
		require.NoError(f.t, err)
		result[table] = string(encoded)
	}
	return result
}

func (f *manualRetryFixture) request(mode entity.ExptRunMode) *entity.RequeueTerminalItemsRequest {
	f.t.Helper()
	ctx := context.Background()
	itemRows, err := NewExptItemResultDAO(f.provider).BatchGet(ctx, f.space, f.expt, []int64{11})
	require.NoError(f.t, err)
	source := convert.NewExptItemResultConvertor().PO2DO(itemRows[0])
	source.Ext, err = entity.ParseExptItemResultExt(gptr.Indirect(itemRows[0].Ext))
	require.NoError(f.t, err)
	itemLog, err := NewExptItemResultDAO(f.provider).GetItemRunLog(ctx, f.expt, f.run, 11, f.space)
	require.NoError(f.t, err)
	ref, err := NewExptItemRefDAO(f.provider).GetByExptIDAndItemID(ctx, f.space, f.expt, 11)
	require.NoError(f.t, err)
	prepared := &entity.PreparedRetryItem{PreviousRunID: source.ExptRunID, SourceItem: source, ItemVersionID: source.ItemVersionID, CurrentItemLog: convert.NewExptItemResultRunLogConverter().PO2DO(itemLog), FrozenItemRef: convert.NewExptItemRefConvertor().PO2DO(ref)}
	previous, err := entity.ParseManualRetryContext(source.Ext)
	require.NoError(f.t, err)
	if previous != nil {
		prepared.PreviousRequestID = previous.RequestID
	}
	turns, err := NewExptTurnResultDAO(f.provider).GetItemTurnResults(ctx, f.expt, 11, f.space)
	require.NoError(f.t, err)
	logs, err := NewExptTurnResultDAO(f.provider).GetItemTurnRunLogs(ctx, f.expt, f.run, 11, f.space)
	require.NoError(f.t, err)
	for _, turn := range turns {
		plan := &entity.PreparedRetryTurn{SourceTurn: convert.NewExptTurnResultConvertor().PO2DO(turn, nil)}
		for _, log := range logs {
			if log.TurnID == turn.TurnID {
				plan.CurrentRunLog, err = convert.NewExptTurnResultRunLogConvertor().PO2DO(log)
				require.NoError(f.t, err)
			}
		}
		refs, err := NewExptTurnEvaluatorResultRefDAO(f.provider).BatchGet(ctx, f.space, []int64{turn.ID})
		require.NoError(f.t, err)
		for _, ref := range refs {
			plan.SourceRefs = append(plan.SourceRefs, convert.NewExptTurnEvaluatorResultRefConvertor().PO2DO(ref))
		}
		if mode == entity.EvaluationModeFailRetry {
			plan.TargetResultID = turn.TargetResultID
			plan.PreserveSuccess = turn.Status == 1
			if plan.PreserveSuccess {
				plan.EvaluatorResults = &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{51: f.base + 1063}}
			} else {
				plan.EvaluatorResults = &entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{51: f.base + 1061}}
			}
			for _, ref := range plan.SourceRefs {
				if ref.EvaluatorResultID == f.base+1061 || plan.PreserveSuccess {
					plan.KeepRefIDs = append(plan.KeepRefIDs, ref.ID)
				}
			}
		}
		prepared.Turns = append(prepared.Turns, plan)
	}
	return &entity.RequeueTerminalItemsRequest{SpaceID: f.space, ExptID: f.expt, RunID: f.run, RequestID: "rq1", PayloadHash: "hash1", Operator: "operator", Mode: mode, SnapshotAtMS: 1700000000123, Items: []*entity.PreparedRetryItem{prepared}}
}

func TestManualRetryMySQLSameRun(t *testing.T) {
	f := newManualRetryFixture(t)
	before := f.snapshot()
	req := f.request(entity.EvaluationModeFailRetry)
	receipt, err := f.port.RequeueTerminalItems(context.Background(), req)
	require.NoError(t, err)
	require.Equal(t, int32(1), receipt.AcceptedCount)
	after := f.snapshot()
	require.NotEqual(t, before["expt_item_result"], after["expt_item_result"])
	require.Equal(t, before["experiment"], after["experiment"])
	require.Equal(t, before["eval_target_record"], after["eval_target_record"])
	require.Equal(t, before["evaluator_record"], after["evaluator_record"])
	var item model.ExptItemResult
	require.NoError(t, f.conn.Where("space_id = ? AND expt_id = ? AND item_id = ?", f.space, f.expt, 11).First(&item).Error)
	require.Zero(t, item.Status)
	require.Nil(t, item.ErrMsg)
	require.Empty(t, item.LogID)
	require.Equal(t, f.base+111, item.ItemVersionID)
	ext, err := entity.ParseExptItemResultExt(gptr.Indirect(item.Ext))
	require.NoError(t, err)
	require.Equal(t, "keep", ext["other"])
	require.Equal(t, "9223372036854775806", ext["future_id"])
	retry, err := entity.ParseManualRetryContext(ext)
	require.NoError(t, err)
	require.Equal(t, entity.EvaluationModeFailRetry, retry.Mode)
	var log model.ExptItemResultRunLog
	require.NoError(t, f.conn.Where("space_id = ? AND expt_id = ? AND item_id = ?", f.space, f.expt, 11).First(&log).Error)
	require.Zero(t, log.Status)
	require.Zero(t, *log.ResultState)
	require.Zero(t, log.QuotaReservationState)
	require.Equal(t, int32(3), log.RetryTimes)
	require.Nil(t, log.ErrMsg)
	require.Empty(t, log.LogID)
	var turns []*model.ExptTurnResult
	require.NoError(t, f.conn.Where("space_id = ? AND expt_id = ?", f.space, f.expt).Order("turn_id").Find(&turns).Error)
	require.Nil(t, turns[0].WeightedScore)
	require.Nil(t, turns[0].ErrMsg)
	require.Zero(t, turns[0].TraceID)
	require.Empty(t, turns[0].LogID)
	require.Equal(t, f.base+521, turns[0].TargetResultID)
	require.Equal(t, 0.0, *turns[1].WeightedScore)
	require.Equal(t, int32(1), turns[1].Status)
	require.Equal(t, "turn-log", turns[1].LogID)
	var turnLogs []*model.ExptTurnResultRunLog
	require.NoError(t, f.conn.Where("space_id = ? AND expt_id = ?", f.space, f.expt).Order("turn_id").Find(&turnLogs).Error)
	require.Zero(t, turnLogs[0].Status)
	require.Nil(t, turnLogs[0].ErrMsg)
	require.Empty(t, turnLogs[0].LogID)
	require.Zero(t, turnLogs[0].TraceID)
	var kept entity.EvaluatorResults
	require.NoError(t, json.Unmarshal(*turnLogs[0].EvaluatorResultIds, &kept))
	require.Equal(t, map[int64]int64{51: f.base + 1061}, kept.EvalVerIDToResID)
	require.JSONEq(t, `{"other":"keep"}`, string(turnLogs[0].Ext))
	require.Equal(t, "turn-run-log", turnLogs[1].LogID)
	require.Equal(t, int64(55), turnLogs[1].TraceID)
	var refs []*model.ExptTurnEvaluatorResultRef
	require.NoError(t, f.conn.Where("space_id = ? AND expt_id = ?", f.space, f.expt).Order("id").Find(&refs).Error)
	require.Len(t, refs, 2)
	require.Equal(t, f.base+1061, refs[0].EvaluatorResultID)
	require.Equal(t, f.base+1063, refs[1].EvaluatorResultID)
	var stats model.ExptStats
	require.NoError(t, f.conn.Where("space_id = ? AND expt_id = ?", f.space, f.expt).First(&stats).Error)
	require.Equal(t, int32(2), stats.PendingCnt)
	require.Equal(t, int32(1), stats.ProcessingCnt)
	require.Zero(t, stats.FailCnt)
	require.Equal(t, float64(42), stats.CreditCost)
	require.Equal(t, int64(100), *stats.InputTokenCost)
	var run model.ExptRunLog
	require.NoError(t, f.conn.Where("id = ?", f.run).First(&run).Error)
	require.Equal(t, int32(2), run.PendingCnt)
	require.Equal(t, int32(1), run.ProcessingCnt)
	require.Zero(t, run.FailCnt)
	require.Equal(t, "original", run.CreatedBy)
	require.Equal(t, int32(1), *run.Mode)
	var chunks []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(*run.ItemIds, &chunks))
	require.JSONEq(t, `{"id":9223372036854775806}`, string(chunks[0]["future"]))
	for _, id := range []int64{12, 13} {
		var got model.ExptItemResult
		require.NoError(t, f.conn.Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, id).First(&got).Error)
		require.Equal(t, "old-item-log", got.LogID)
		require.Equal(t, []byte("old-error"), *got.ErrMsg)
	}
	replay, err := f.port.RequeueTerminalItems(context.Background(), req)
	require.NoError(t, err)
	expectedReceipt, err := json.Marshal(receipt)
	require.NoError(t, err)
	replayReceipt, err := json.Marshal(replay)
	require.NoError(t, err)
	require.JSONEq(t, string(expectedReceipt), string(replayReceipt))
	require.Equal(t, after, f.snapshot())
	req.PayloadHash = "different"
	_, err = f.port.RequeueTerminalItems(context.Background(), req)
	require.ErrorContains(t, err, "request_conflict")
	require.Equal(t, after, f.snapshot())
}

func TestManualRetryMySQLWriteRollback(t *testing.T) {
	for _, table := range []string{"expt_item_result", "expt_item_result_run_log", "expt_turn_result", "expt_turn_result_run_log", "expt_turn_evaluator_result_ref", "expt_stats", "expt_run_log"} {
		t.Run(table, func(t *testing.T) {
			f := newManualRetryFixture(t)
			req := f.request(entity.EvaluationModeFailRetry)
			before := f.snapshot()
			hits := 0
			callback := func(tx *gorm.DB) {
				if tx.Statement.Table == table {
					_, sameTx := tx.Statement.ConnPool.(*sql.Tx)
					require.True(t, sameTx, "write must use the Provider transaction connection")
					hits++
					_ = tx.AddError(errors.New("injected write failure " + table))
				}
			}
			name := "manual_retry_fault_" + table
			if table == "expt_turn_evaluator_result_ref" {
				require.NoError(t, f.conn.Callback().Delete().Before("gorm:delete").Register(name, callback))
				defer func() { require.NoError(t, f.conn.Callback().Delete().Remove(name)) }()
			} else {
				require.NoError(t, f.conn.Callback().Update().Before("gorm:update").Register(name, callback))
				defer func() { require.NoError(t, f.conn.Callback().Update().Remove(name)) }()
			}
			_, err := f.port.RequeueTerminalItems(context.Background(), req)
			require.ErrorContains(t, err, "injected write failure")
			require.Positive(t, hits)
			require.True(t, reflect.DeepEqual(before, f.snapshot()), "all four tables + refs + stats + receipt must roll back")
			t.Logf("REAL_MYSQL_ROLLBACK_VERIFIED table=%s snapshots=11", table)
		})
	}
}

func TestManualRetryMySQLBoundaries(t *testing.T) {
	for _, kind := range []string{"logged", "source_changed", "capacity", "manual_whole_row", "zero_accepted", "finished_replay"} {
		t.Run(kind, func(t *testing.T) {
			f := newManualRetryFixture(t)
			mode := entity.EvaluationModeFailRetry
			if kind == "manual_whole_row" {
				mode = entity.EvaluationModeRetryItems
			}
			req := f.request(mode)
			switch kind {
			case "logged":
				require.NoError(t, f.conn.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, 11).Update("result_state", 2).Error)
			case "source_changed":
				require.NoError(t, f.conn.Model(&model.ExptTurnResult{}).Where("space_id=? AND expt_id=? AND turn_id=?", f.space, f.expt, 21).Update("target_result_id", 991).Error)
			case "capacity":
				raw := []byte(`[{"ItemIDs":[11,12,13],"future":"` + strings.Repeat("x", 65450) + `"}]`)
				require.Less(t, len(raw), 65535)
				require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("id=?", f.run).Update("item_ids", raw).Error)
			case "zero_accepted":
				req.Items = nil
				req.RejectedItems = []*entity.ManualRetryRejectedItem{{ItemID: 12, Reason: "processing"}, {ItemID: 13, Reason: "queueing"}}
			}
			before := f.snapshot()
			receipt, err := f.port.RequeueTerminalItems(context.Background(), req)
			if kind == "capacity" {
				require.ErrorContains(t, err, "capacity")
				require.Equal(t, before, f.snapshot())
				return
			}
			require.NoError(t, err)
			switch kind {
			case "logged", "source_changed", "zero_accepted":
				require.Zero(t, receipt.AcceptedCount)
				if kind != "zero_accepted" {
					want := "source_changed"
					if kind == "logged" {
						want = "result_not_ready"
					}
					require.Equal(t, want, receipt.RejectedItems[0].Reason)
				}
				after := f.snapshot()
				for table, snapshot := range before {
					if table != "expt_run_log" {
						require.Equal(t, snapshot, after[table], table)
					}
				}
				_, err = f.port.RequeueTerminalItems(context.Background(), req)
				require.NoError(t, err)
				require.Equal(t, after, f.snapshot())
			case "manual_whole_row":
				var turns []*model.ExptTurnResult
				require.NoError(t, f.conn.Where("space_id=? AND expt_id=?", f.space, f.expt).Find(&turns).Error)
				for _, turn := range turns {
					require.Zero(t, turn.Status)
					require.Zero(t, turn.TargetResultID)
					require.Nil(t, turn.WeightedScore)
				}
				var logs []*model.ExptTurnResultRunLog
				require.NoError(t, f.conn.Where("space_id=? AND expt_id=?", f.space, f.expt).Find(&logs).Error)
				for _, log := range logs {
					require.Zero(t, log.TargetResultID)
					require.JSONEq(t, `null`, string(*log.EvaluatorResultIds))
					require.JSONEq(t, `{"other":"keep"}`, string(log.Ext))
				}
				var refs int64
				require.NoError(t, f.conn.Model(&model.ExptTurnEvaluatorResultRef{}).Where("space_id=? AND expt_id=?", f.space, f.expt).Count(&refs).Error)
				require.Zero(t, refs)
			case "finished_replay":
				require.NoError(t, f.conn.Model(&model.Experiment{}).Where("id=?", f.expt).Update("status", int32(entity.ExptStatus_Success)).Error)
				require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("id=?", f.run).Update("status", int64(entity.ExptStatus_Success)).Error)
				after := f.snapshot()
				replay, err := f.port.RequeueTerminalItems(context.Background(), req)
				require.NoError(t, err)
				require.Equal(t, receipt.RequestID, replay.RequestID)
				require.Equal(t, after, f.snapshot())
				found, err := f.port.FindReceipt(context.Background(), f.space, f.expt, f.run, req.RequestID, req.PayloadHash)
				require.NoError(t, err)
				require.Equal(t, int32(1), found.AcceptedCount)
			}
		})
	}
}

func TestManualRetryMySQLConcurrentReplay(t *testing.T) {
	f := newManualRetryFixture(t)
	req := f.request(entity.EvaluationModeFailRetry)
	results := make(chan *entity.ManualRetryReceipt, 2)
	failures := make(chan error, 2)
	var group sync.WaitGroup
	for i := 0; i < 2; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			r, err := f.port.RequeueTerminalItems(context.Background(), req)
			results <- r
			failures <- err
		}()
	}
	group.Wait()
	close(results)
	close(failures)
	for err := range failures {
		require.NoError(t, err)
	}
	for r := range results {
		require.Equal(t, int32(1), r.AcceptedCount)
	}
	var stats model.ExptStats
	require.NoError(t, f.conn.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&stats).Error)
	require.Equal(t, int32(2), stats.PendingCnt)
	require.Zero(t, stats.FailCnt)
	var run model.ExptRunLog
	require.NoError(t, f.conn.Where("id=?", f.run).First(&run).Error)
	var chunks []entity.ExptRunLogItems
	require.NoError(t, json.Unmarshal(*run.ItemIds, &chunks))
	require.Len(t, chunks, 2)
	t.Log("REAL_MYSQL_CONCURRENT_REPLAY_VERIFIED same request one write set")
}

func TestManualRetryMySQLCorruptionNoReceipt(t *testing.T) {
	for _, kind := range []string{"item_ext", "run_metadata", "turn_ext", "turn_results", "frozen_config", "source_trace"} {
		t.Run(kind, func(t *testing.T) {
			f := newManualRetryFixture(t)
			req := f.request(entity.EvaluationModeFailRetry)
			switch kind {
			case "item_ext":
				require.NoError(t, f.conn.Model(&model.ExptItemResult{}).Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, 11).Update("ext", []byte(`{"manual_retry":"broken"}`)).Error)
			case "run_metadata":
				require.NoError(t, f.conn.Model(&model.ExptRunLog{}).Where("id=?", f.run).Update("item_ids", []byte(`{`)).Error)
			case "turn_ext":
				require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND expt_id=? AND turn_id=?", f.space, f.expt, 21).Update("ext", []byte(`{"other":42}`)).Error)
			case "turn_results":
				require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND expt_id=? AND turn_id=?", f.space, f.expt, 21).Update("evaluator_result_ids", []byte(`{`)).Error)
			case "source_trace":
				require.NoError(t, f.conn.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND expt_id=? AND turn_id=?", f.space, f.expt, 21).Update("trace_id", 99).Error)
				before := f.snapshot()
				receipt, err := f.port.RequeueTerminalItems(context.Background(), req)
				require.NoError(t, err)
				require.Zero(t, receipt.AcceptedCount)
				require.Equal(t, "source_changed", receipt.RejectedItems[0].Reason)
				after := f.snapshot()
				for table, raw := range before {
					if table != "expt_run_log" {
						require.Equal(t, raw, after[table], table)
					}
				}
				return
			case "frozen_config":
				require.NoError(t, f.conn.Model(&model.ExptItemRef{}).Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, 11).Update("item_config", []byte(`{`)).Error)
			}
			before := f.snapshot()
			_, err := f.port.RequeueTerminalItems(context.Background(), req)
			require.Error(t, err)
			require.Equal(t, before, f.snapshot())
		})
	}
}
