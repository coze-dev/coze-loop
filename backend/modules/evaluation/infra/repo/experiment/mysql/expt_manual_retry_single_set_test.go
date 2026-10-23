// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	lockmocks "github.com/coze-dev/coze-loop/backend/infra/lock/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	componentmocks "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

type manualRetryFixture struct {
	provider               db.Provider
	conn                   *gorm.DB
	port                   repo.IExptManualRetryRepo
	space, expt, run, base int64
	snapshot               func() map[string]string
}

func newManualRetryFixture(t *testing.T) *manualRetryFixture {
	provider, conn, port, space, expt, run, base, snapshot := mysql.NewManualRetryFixtureForTest(t)
	return &manualRetryFixture{provider, conn, port, space, expt, run, base, snapshot}
}

// Keep the manager and its SQL preparation/acceptance ports real. Only remote
// configuration, the renewable locks, and the central ledger are test doubles.
type singleSetRetryStorage struct {
	f *manualRetryFixture
}

type singleSetExperimentStorage struct {
	repo.IExperimentRepo
	*singleSetRetryStorage
}

type singleSetRunStorage struct {
	repo.IExptRunLogRepo
	*singleSetRetryStorage
}

type singleSetItemStorage struct {
	repo.IExptItemResultRepo
	*singleSetRetryStorage
}

type singleSetTurnStorage struct {
	repo.IExptTurnResultRepo
	*singleSetRetryStorage
}

type singleSetRefStorage struct {
	repo.IExptItemRefRepo
	*singleSetRetryStorage
}

type singleSetTargetStorage struct {
	service.IEvalTargetService
	*singleSetRetryStorage
}

type singleSetEvaluatorStorage struct {
	service.EvaluatorRecordService
	*singleSetRetryStorage
}

func (s *singleSetExperimentStorage) GetByID(ctx context.Context, expt, space int64) (*entity.Experiment, error) {
	row, err := mysql.NewExptDAO(s.f.provider).GetByID(ctx, expt)
	if err != nil {
		return nil, err
	}
	return convert.NewExptConverter().PO2DO(row, nil)
}

func (s *singleSetRunStorage) Get(ctx context.Context, expt, run int64) (*entity.ExptRunLog, error) {
	row, err := mysql.NewExptRunLogDAO(s.f.provider).Get(ctx, expt, run)
	if err != nil {
		return nil, err
	}
	return convert.NewExptRunLogConvertor().PO2DO(row)
}

func (s *singleSetItemStorage) BatchGet(ctx context.Context, space, expt int64, ids []int64) ([]*entity.ExptItemResult, error) {
	rows, err := mysql.NewExptItemResultDAO(s.f.provider).BatchGet(ctx, space, expt, ids)
	var result []*entity.ExptItemResult
	for _, row := range rows {
		item := convert.NewExptItemResultConvertor().PO2DO(row)
		item.Ext, err = entity.ParseExptItemResultExt(gptr.Indirect(row.Ext))
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, err
}

func (s *singleSetItemStorage) ScanItemResults(ctx context.Context, expt, cursor, limit int64, statuses []int32, space int64) ([]*entity.ExptItemResult, int64, error) {
	// The fixture has exactly one failed item, as in the captured A/B/C case.
	items, err := s.BatchGet(ctx, space, expt, []int64{11})
	return items, 0, err
}

func (s *singleSetItemStorage) MGetItemRunLog(ctx context.Context, expt, run int64, ids []int64, space int64) ([]*entity.ExptItemResultRunLog, error) {
	var result []*entity.ExptItemResultRunLog
	for _, id := range ids {
		row, err := mysql.NewExptItemResultDAO(s.f.provider).GetItemRunLog(ctx, expt, run, id, space)
		if err != nil {
			return nil, err
		}
		result = append(result, convert.NewExptItemResultRunLogConverter().PO2DO(row))
	}
	return result, nil
}

func (s *singleSetRefStorage) GetByExptIDAndItemID(ctx context.Context, space, expt, item int64) (*entity.ExptItemRef, error) {
	row, err := mysql.NewExptItemRefDAO(s.f.provider).GetByExptIDAndItemID(ctx, space, expt, item)
	return convert.NewExptItemRefConvertor().PO2DO(row), err
}

func (s *singleSetTurnStorage) GetItemTurnResults(ctx context.Context, expt, item, space int64) ([]*entity.ExptTurnResult, error) {
	rows, err := mysql.NewExptTurnResultDAO(s.f.provider).GetItemTurnResults(ctx, expt, item, space)
	var result []*entity.ExptTurnResult
	for _, row := range rows {
		result = append(result, convert.NewExptTurnResultConvertor().PO2DO(row, nil))
	}
	return result, err
}

func (s *singleSetTurnStorage) GetItemTurnRunLogs(ctx context.Context, expt, run, item, space int64) ([]*entity.ExptTurnResultRunLog, error) {
	rows, err := mysql.NewExptTurnResultDAO(s.f.provider).GetItemTurnRunLogs(ctx, expt, run, item, space)
	var result []*entity.ExptTurnResultRunLog
	for _, row := range rows {
		log, err := convert.NewExptTurnResultRunLogConvertor().PO2DO(row)
		if err != nil {
			return nil, err
		}
		result = append(result, log)
	}
	return result, err
}

func (s *singleSetTurnStorage) BatchGetTurnEvaluatorResultRef(ctx context.Context, space int64, ids []int64) ([]*entity.ExptTurnEvaluatorResultRef, error) {
	rows, err := mysql.NewExptTurnEvaluatorResultRefDAO(s.f.provider).BatchGet(ctx, space, ids)
	var result []*entity.ExptTurnEvaluatorResultRef
	for _, row := range rows {
		result = append(result, convert.NewExptTurnEvaluatorResultRefConvertor().PO2DO(row))
	}
	return result, err
}

func (s *singleSetTargetStorage) GetRecordByID(ctx context.Context, space, id int64) (*entity.EvalTargetRecord, error) {
	var row struct {
		ID, SpaceID, TargetVersionID, ExperimentRunID, ItemID, ItemVersionID, TurnID int64
		Status                                                                       entity.EvalTargetRunStatus
	}
	err := s.f.conn.WithContext(ctx).Raw("SELECT id, space_id, target_version_id, experiment_run_id, item_id, item_version_id, turn_id, status FROM eval_target_record WHERE space_id=? AND id=?", space, id).Scan(&row).Error
	return &entity.EvalTargetRecord{ID: row.ID, SpaceID: row.SpaceID, TargetVersionID: row.TargetVersionID, ExperimentRunID: row.ExperimentRunID, ItemID: row.ItemID, ItemVersionID: row.ItemVersionID, TurnID: row.TurnID, Status: &row.Status}, err
}

func (s *singleSetEvaluatorStorage) BatchGetEvaluatorRecord(ctx context.Context, ids []int64, include, storage bool) ([]*entity.EvaluatorRecord, error) {
	var rows []struct {
		ID, SpaceID, ExperimentID, ExperimentRunID, ItemID, ItemVersionID, TurnID, EvaluatorVersionID int64
		Status                                                                                        entity.EvaluatorRunStatus
	}
	err := s.f.conn.WithContext(ctx).Table("evaluator_record").Select("id, space_id, experiment_id, experiment_run_id, item_id, item_version_id, turn_id, evaluator_version_id, status").Where("space_id=? AND id IN ?", s.f.space, ids).Find(&rows).Error
	var records []*entity.EvaluatorRecord
	for _, row := range rows {
		records = append(records, &entity.EvaluatorRecord{ID: row.ID, SpaceID: row.SpaceID, ExperimentID: row.ExperimentID, ExperimentRunID: row.ExperimentRunID, ItemID: row.ItemID, ItemVersionID: row.ItemVersionID, TurnID: row.TurnID, EvaluatorVersionID: row.EvaluatorVersionID, Status: row.Status})
	}
	return records, err
}

type singleSetRetryPort struct {
	repo.IExptManualRetryRepo
	beforePreflight func()
	beforeCommit    func()
	prepared        *entity.RequeueTerminalItemsRequest
}

type singleSetRetryWaker struct{}

func (singleSetRetryWaker) WakeCurrentRun(context.Context, string) error { return nil }

func (p *singleSetRetryPort) PreflightTerminalItems(ctx context.Context, req *entity.RequeueTerminalItemsRequest) (*entity.ManualRetryPreparation, error) {
	p.prepared = req
	if p.beforePreflight != nil {
		p.beforePreflight()
	}
	return p.IExptManualRetryRepo.PreflightTerminalItems(ctx, req)
}

func (p *singleSetRetryPort) RequeueTerminalItems(ctx context.Context, req *entity.RequeueTerminalItemsRequest) (*entity.ManualRetryReceipt, error) {
	if p.beforeCommit != nil {
		p.beforeCommit()
	}
	return p.IExptManualRetryRepo.RequeueTerminalItems(ctx, req)
}

func singleSetRetryManager(t *testing.T, f *manualRetryFixture, port repo.IExptManualRetryRepo, enforce bool) service.IExptManager {
	t.Helper()
	ctrl := gomock.NewController(t)
	config := componentmocks.NewMockIConfiger(ctrl)
	config.EXPECT().GetExptExecConf(gomock.Any(), f.space).Return(&entity.ExptExecConf{RunningRetryEnabled: gptr.Of(true)}).AnyTimes()
	locker := lockmocks.NewMockILocker(ctrl)
	locker.EXPECT().LockWithRenew(gomock.Any(), gomock.Any(), 5*time.Second, time.Minute).DoAndReturn(func(ctx context.Context, key string, ttl, renewal time.Duration) (bool, context.Context, func(), error) {
		child, cancel := context.WithCancel(ctx)
		return true, child, cancel, nil
	}).AnyTimes()
	guard := componentmocks.NewMockICentralReservationGuard(ctrl)
	waker := singleSetRetryWaker{}
	if enforce {
		guard.EXPECT().ReleaseTerminalReservation(gomock.Any(), "single-set-test", f.run, int64(11), "").Return(component.TerminalReservationAbsent, nil).MaxTimes(1)
	}
	s := &singleSetRetryStorage{f: f}
	return service.NewExptManager(
		nil, &singleSetExperimentStorage{singleSetRetryStorage: s}, &singleSetRunStorage{singleSetRetryStorage: s}, nil,
		&singleSetItemStorage{singleSetRetryStorage: s}, &singleSetRefStorage{singleSetRetryStorage: s}, &singleSetTurnStorage{singleSetRetryStorage: s}, config, nil, locker, nil, nil,
		nil, nil, nil, nil, nil, nil, &singleSetTargetStorage{singleSetRetryStorage: s}, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, guard, port, &singleSetEvaluatorStorage{singleSetRetryStorage: s}, waker,
		func() lock.ILocker { return locker },
	)
}

func prepareSingleSetFixture(t *testing.T, f *manualRetryFixture, enforce, zeroVersion bool) {
	t.Helper()
	dispatch := entity.ExptDispatchModeLegacy
	if enforce {
		dispatch = entity.ExptDispatchModeEnforce
	}
	require.NoError(t, f.conn.Model(&model.Experiment{}).Where("id=?", f.expt).Updates(map[string]any{
		"eval_set_source_type": 1, "eval_set_id": f.base + 31, "eval_set_version_id": f.base + 32,
		"target_id": f.base + 41, "target_version_id": f.base + 42,
		"scheduler_mode": dispatch, "scheduler_scope": "single-set-test",
	}).Error)
	require.NoError(t, f.conn.Where("space_id=? AND expt_id=?", f.space, f.expt).Delete(&model.ExptItemRef{}).Error)
	// Turn zero is a real first turn, not an absence sentinel.
	for _, table := range []string{"expt_turn_result", "expt_turn_result_run_log", "eval_target_record", "evaluator_record"} {
		require.NoError(t, f.conn.Table(table).Where("space_id=? AND item_id=? AND turn_id=?", f.space, 11, 21).Update("turn_id", 0).Error)
	}
	if zeroVersion {
		for _, table := range []string{"expt_item_result", "expt_item_result_run_log", "expt_turn_result", "expt_turn_result_run_log", "eval_target_record", "evaluator_record"} {
			require.NoError(t, f.conn.Table(table).Where("space_id=? AND item_id=?", f.space, 11).Update("item_version_id", 0).Error)
		}
	}
}

func TestManualRetryMySQLSingleSetManagerSameRun(t *testing.T) {
	for _, enforce := range []bool{false, true} {
		for _, mode := range []entity.ExptRunMode{entity.EvaluationModeFailRetry, entity.EvaluationModeRetryItems} {
			for _, zeroVersion := range []bool{false, true} {
				name := "legacy"
				if enforce {
					name = "enforce"
				}
				t.Run(fmt.Sprintf("%s/mode=%d/zero-version=%t", name, mode, zeroVersion), func(t *testing.T) {
					f := newManualRetryFixture(t)
					prepareSingleSetFixture(t, f, enforce, zeroVersion)
					before := f.snapshot()
					var beforeItems []model.ExptItemResultRunLog
					var beforeTurns []model.ExptTurnResultRunLog
					require.NoError(t, f.conn.Where("space_id=? AND expt_id=?", f.space, f.expt).Order("id").Find(&beforeItems).Error)
					require.NoError(t, f.conn.Where("space_id=? AND expt_id=?", f.space, f.expt).Order("id").Find(&beforeTurns).Error)
					manager := singleSetRetryManager(t, f, f.port, enforce)
					req := &entity.RetryInCurrentRunRequest{SpaceID: f.space, ExptID: f.expt, ExpectedRunID: f.run, RequestID: "9ef59b9a-fbdb-4c9a-a027-8c969493aaf3", Mode: mode}
					if mode == entity.EvaluationModeRetryItems {
						req.ItemIDs = []int64{13, 11, 12, 11}
					}
					receipt, err := manager.RetryInCurrentRun(context.Background(), req)
					require.NoError(t, err, "single_set current items have frozen bindings without expt_item_ref")
					require.Equal(t, int32(1), receipt.AcceptedCount)
					if mode == entity.EvaluationModeRetryItems {
						require.Equal(t, []*entity.ManualRetryRejectedItem{{ItemID: 12, Reason: "processing"}, {ItemID: 13, Reason: "queueing"}}, receipt.RejectedItems)
					}
					after := f.snapshot()
					for _, table := range []string{"experiment", "expt_item_ref", "eval_target_record", "evaluator_record"} {
						require.Equal(t, before[table], after[table], table)
					}
					var items []model.ExptItemResultRunLog
					var turns []model.ExptTurnResultRunLog
					require.NoError(t, f.conn.Where("space_id=? AND expt_id=?", f.space, f.expt).Order("id").Find(&items).Error)
					require.NoError(t, f.conn.Where("space_id=? AND expt_id=?", f.space, f.expt).Order("id").Find(&turns).Error)
					require.Len(t, items, len(beforeItems))
					require.Len(t, turns, len(beforeTurns))
					for i := range items {
						require.Equal(t, beforeItems[i].ID, items[i].ID)
						require.Equal(t, beforeItems[i].RetryTimes, items[i].RetryTimes)
						if items[i].ItemID != 11 {
							require.Equal(t, beforeItems[i], items[i])
						} else {
							require.Zero(t, items[i].Status)
							require.Zero(t, gptr.Indirect(items[i].ResultState))
						}
					}
					for i := range turns {
						require.Equal(t, beforeTurns[i].ID, turns[i].ID)
						if mode == entity.EvaluationModeFailRetry && turns[i].TurnID == 22 {
							// The original writer rebinds expt_run_id and refreshes its
							// timestamp even when the run is unchanged. Payload stays intact.
							expected := beforeTurns[i]
							expected.UpdatedAt = turns[i].UpdatedAt
							require.Equal(t, expected, turns[i], "successful whole turn payload stays intact")
							continue
						}
						var kept entity.EvaluatorResults
						require.NoError(t, json.Unmarshal(gptr.Indirect(turns[i].EvaluatorResultIds), &kept))
						if mode == entity.EvaluationModeFailRetry {
							require.Equal(t, f.base+521, turns[i].TargetResultID)
							require.Equal(t, map[int64]int64{51: f.base + 1061}, kept.EvalVerIDToResID, "only E2 is eligible for a fresh call")
						} else {
							require.Zero(t, turns[i].TargetResultID)
							require.Empty(t, kept.EvalVerIDToResID)
						}
					}
					replay, err := manager.RetryInCurrentRun(context.Background(), req)
					require.NoError(t, err)
					expectedJSON, err := json.Marshal(receipt)
					require.NoError(t, err)
					replayedJSON, err := json.Marshal(replay)
					require.NoError(t, err)
					require.JSONEq(t, string(expectedJSON), string(replayedJSON))
					require.Equal(t, after, f.snapshot(), "receipt duplicate performs no second reset")
					t.Log("REAL_MYSQL_SINGLE_SET_MANAGER_VERIFIED sameRun rowIDs reused turn0 refsAbsent target/E1 preserved receipt replay")
				})
			}
		}
	}
}

func TestManualRetryMySQLSingleSetFrozenSourceDrift(t *testing.T) {
	for _, boundary := range []string{"preflight", "commit"} {
		for _, column := range []string{"eval_set_id", "eval_set_version_id", "eval_set_space_id", "eval_set_source_type", "target_id", "target_version_id", "target_space_id", "target_type"} {
			t.Run(boundary+"/"+column, func(t *testing.T) {
				f := newManualRetryFixture(t)
				if !f.conn.Migrator().HasColumn(&model.Experiment{}, column) {
					t.Skip("local fixture schema lacks " + column + "; source-field comparison is covered by the locked-source unit test")
				}
				prepareSingleSetFixture(t, f, false, false)
				port := &singleSetRetryPort{IExptManualRetryRepo: f.port}
				var before map[string]string
				drift := func() {
					require.NoError(t, f.conn.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumn(column, gorm.Expr(column+"+1")).Error)
					before = f.snapshot()
				}
				if boundary == "preflight" {
					port.beforePreflight = drift
				} else {
					port.beforeCommit = drift
				}
				manager := singleSetRetryManager(t, f, port, false)
				receipt, err := manager.RetryInCurrentRun(context.Background(), &entity.RetryInCurrentRunRequest{SpaceID: f.space, ExptID: f.expt, ExpectedRunID: f.run, RequestID: "9ef59b9a-fbdb-4c9a-a027-8c969493aaf3", Mode: entity.EvaluationModeRetryItems, ItemIDs: []int64{11}})
				require.NoError(t, err)
				require.Zero(t, receipt.AcceptedCount)
				require.Equal(t, []*entity.ManualRetryRejectedItem{{ItemID: 11, Reason: "source_changed"}}, receipt.RejectedItems)
				after := f.snapshot()
				for table, original := range before {
					if table != "expt_run_log" {
						require.Equal(t, original, after[table], table)
					}
				}
				t.Log("FROZEN_SOURCE_DRIFT_REJECTED boundary=" + boundary + " column=" + column)
			})
		}
	}
}

func TestManualRetryMySQLSingleSetPreparationFences(t *testing.T) {
	for _, kind := range []string{"multi_set_missing_ref", "draft_zero", "draft_set_id", "missing_set", "item_log_version", "turn_version", "turn_log_version", "not_resulted"} {
		t.Run(kind, func(t *testing.T) {
			f := newManualRetryFixture(t)
			prepareSingleSetFixture(t, f, false, true)
			switch kind {
			case "multi_set_missing_ref":
				require.NoError(t, f.conn.Model(&model.Experiment{}).Where("id=?", f.expt).Update("eval_set_source_type", 2).Error)
			case "draft_zero":
				require.NoError(t, f.conn.Model(&model.Experiment{}).Where("id=?", f.expt).Update("eval_set_version_id", 0).Error)
			case "draft_set_id":
				require.NoError(t, f.conn.Model(&model.Experiment{}).Where("id=?", f.expt).Update("eval_set_version_id", f.base+31).Error)
			case "missing_set":
				require.NoError(t, f.conn.Model(&model.Experiment{}).Where("id=?", f.expt).Update("eval_set_id", 0).Error)
			case "item_log_version":
				require.NoError(t, f.conn.Table("expt_item_result_run_log").Where("space_id=? AND item_id=?", f.space, 11).Update("item_version_id", 1).Error)
			case "turn_version":
				require.NoError(t, f.conn.Table("expt_turn_result").Where("space_id=? AND item_id=?", f.space, 11).Update("item_version_id", 1).Error)
			case "turn_log_version":
				require.NoError(t, f.conn.Table("expt_turn_result_run_log").Where("space_id=? AND item_id=?", f.space, 11).Update("item_version_id", 1).Error)
			case "not_resulted":
				require.NoError(t, f.conn.Table("expt_item_result_run_log").Where("space_id=? AND item_id=?", f.space, 11).Update("result_state", 0).Error)
			}
			before := f.snapshot()
			manager := singleSetRetryManager(t, f, f.port, false)
			receipt, err := manager.RetryInCurrentRun(context.Background(), &entity.RetryInCurrentRunRequest{SpaceID: f.space, ExptID: f.expt, ExpectedRunID: f.run, RequestID: "9ef59b9a-fbdb-4c9a-a027-8c969493aaf3", Mode: entity.EvaluationModeRetryItems, ItemIDs: []int64{11}})
			if kind == "not_resulted" {
				require.NoError(t, err)
				require.Zero(t, receipt.AcceptedCount)
				require.Equal(t, "result_not_ready", receipt.RejectedItems[0].Reason)
				before["expt_run_log"] = f.snapshot()["expt_run_log"]
			} else {
				require.Error(t, err)
				require.Nil(t, receipt)
				if kind == "multi_set_missing_ref" {
					require.ErrorContains(t, err, "record not found")
				}
			}
			require.Equal(t, before, f.snapshot())
		})
	}
}

func TestManualRetryMySQLSingleSetConcurrentReplay(t *testing.T) {
	f := newManualRetryFixture(t)
	prepareSingleSetFixture(t, f, false, true)
	port := &singleSetRetryPort{IExptManualRetryRepo: f.port}
	manager := singleSetRetryManager(t, f, port, false)
	// Capture the real manager-prepared source before acceptance. Concurrent SQL
	// acceptors then contend on the actual experiment/run/item locks and receipt.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	port.beforeCommit = cancel
	_, err := manager.RetryInCurrentRun(ctx, &entity.RetryInCurrentRunRequest{SpaceID: f.space, ExptID: f.expt, ExpectedRunID: f.run, RequestID: "9ef59b9a-fbdb-4c9a-a027-8c969493aaf3", Mode: entity.EvaluationModeFailRetry})
	require.ErrorIs(t, err, context.Canceled)
	require.NotNil(t, port.prepared)
	before := f.snapshot()
	var wg sync.WaitGroup
	var receipts [2]*entity.ManualRetryReceipt
	var errs [2]error
	start := make(chan struct{})
	for i := range receipts {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			receipts[i], errs[i] = f.port.RequeueTerminalItems(context.Background(), port.prepared)
		}(i)
	}
	close(start)
	wg.Wait()
	for i := range receipts {
		require.NoError(t, errs[i])
		require.Equal(t, int32(1), receipts[i].AcceptedCount)
	}
	a, err := json.Marshal(receipts[0])
	require.NoError(t, err)
	b, err := json.Marshal(receipts[1])
	require.NoError(t, err)
	require.JSONEq(t, string(a), string(b))
	after := f.snapshot()
	require.Equal(t, before["expt_item_ref"], after["expt_item_ref"])
	var stats model.ExptStats
	require.NoError(t, f.conn.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&stats).Error)
	require.Equal(t, int32(2), stats.PendingCnt)
	require.Zero(t, stats.FailCnt)
	t.Log("REAL_MYSQL_SINGLE_SET_CONCURRENT_REPLAY one atomic reset/stat delta/receipt")
}

func TestManualRetryMySQLSingleSetWriteRollback(t *testing.T) {
	for _, table := range []string{"expt_item_result", "expt_item_result_run_log", "expt_turn_result", "expt_turn_result_run_log", "expt_turn_evaluator_result_ref", "expt_stats", "expt_run_log"} {
		t.Run(table, func(t *testing.T) {
			f := newManualRetryFixture(t)
			prepareSingleSetFixture(t, f, false, false)
			before := f.snapshot()
			cause := errors.New("single-set atomic write failure")
			callback := func(tx *gorm.DB) {
				if tx.Statement.Table == table {
					_ = tx.AddError(cause)
				}
			}
			name := "single_set_write_failure"
			require.NoError(t, f.conn.Callback().Update().After("gorm:update").Register(name, callback))
			require.NoError(t, f.conn.Callback().Delete().After("gorm:delete").Register(name, callback))
			defer func() {
				require.NoError(t, f.conn.Callback().Update().Remove(name))
				require.NoError(t, f.conn.Callback().Delete().Remove(name))
			}()
			manager := singleSetRetryManager(t, f, f.port, false)
			receipt, err := manager.RetryInCurrentRun(context.Background(), &entity.RetryInCurrentRunRequest{SpaceID: f.space, ExptID: f.expt, ExpectedRunID: f.run, RequestID: "9ef59b9a-fbdb-4c9a-a027-8c969493aaf3", Mode: entity.EvaluationModeFailRetry})
			require.ErrorIs(t, err, cause)
			require.Nil(t, receipt)
			require.Equal(t, before, f.snapshot(), "four tables, refs, stats and receipt roll back together")
		})
	}
}
