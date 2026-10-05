// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	driver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	evalconvert "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/convertor"
	evalmodel "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
	store "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

var hookRecoveryIDs = func() *atomic.Int64 { n := new(atomic.Int64); n.Store(time.Now().UnixNano()); return n }()

// Real controlled-progress storage; all rows are owned by this test's unique scope.
func hookRecoveryStorage(t *testing.T, f *executionHookFixture) (repo.IHookTurnProgressRepo, *gorm.DB) {
	t.Helper()
	dsn := os.Getenv("HOOK_MYSQL_TX_DSN")
	if dsn == "" {
		t.Skip("requires existing isolated HOOK_MYSQL_TX_DSN")
	}
	cfg, err := driver.ParseDSN(dsn)
	require.NoError(t, err)
	require.Equal(t, "unix", cfg.Net)
	require.Equal(t, "/Users/bytedance/fornax_workspace/_local/scratch/hook-mysql-20260922/m.sock", cfg.Addr)
	require.Equal(t, "hook_7378265404_tx", cfg.DBName)
	p, err := db.NewDB(mysql.Open(dsn), &gorm.Config{Logger: logger.Discard})
	require.NoError(t, err)
	sql := p.NewSession(context.Background(), db.WithMaster())
	pool, err := sql.DB()
	require.NoError(t, err)
	key := entity.HookRunKey{WorkspaceID: hookRecoveryIDs.Add(1), ExperimentID: hookRecoveryIDs.Add(1), RunID: hookRecoveryIDs.Add(1)}
	itemID := hookRecoveryIDs.Add(1)
	f.etec.Event.SpaceID, f.etec.Event.ExptID, f.etec.Event.ExptRunID, f.etec.Event.EvalSetItemID = key.WorkspaceID, key.ExperimentID, key.RunID, itemID
	f.etec.Expt.SpaceID, f.etec.Expt.ID = key.WorkspaceID, key.ExperimentID
	f.etec.EvalSetItem.ItemID = itemID
	row := f.etec.GetExistTurnResultRunLog(f.etec.Turn.ID)
	row.ID, row.SpaceID, row.ExptID, row.ExptRunID, row.ItemID = hookRecoveryIDs.Add(1), key.WorkspaceID, key.ExperimentID, key.RunID, itemID
	row.ItemVersionID, row.Status = 7, entity.TurnRunState_Processing
	for _, ev := range f.etec.Expt.Evaluators {
		ev.SpaceID = key.WorkspaceID
	}
	if tr := f.etec.ExptTurnRunResult.TargetResult; tr != nil {
		tr.SpaceID, tr.ExperimentRunID, tr.ItemID, tr.ItemVersionID, tr.TurnID = key.WorkspaceID, key.RunID, itemID, 7, row.TurnID
	}
	for _, er := range f.etec.ExptTurnRunResult.EvaluatorResults {
		er.SpaceID, er.ExperimentID, er.ExperimentRunID, er.ItemID, er.ItemVersionID, er.TurnID = key.WorkspaceID, key.ExperimentID, key.RunID, itemID, 7, row.TurnID
	}
	t.Cleanup(func() {
		for _, table := range []string{"expt_turn_result_run_log", "expt_lifecycle_run_item", "expt_lifecycle_run"} {
			require.NoError(t, sql.Table(table).Where("space_id=? AND expt_id=?", key.WorkspaceID, key.ExperimentID).Delete(nil).Error)
		}
		require.NoError(t, pool.Close())
	})
	require.NoError(t, sql.Create(&model.ExptLifecycleRun{SpaceID: key.WorkspaceID, ExptID: key.ExperimentID, ExptRunID: key.RunID, ExecutionScope: "local", SnapshotCipher: []byte{1}, SnapshotKeyID: "k", SnapshotHash: "hash", Gate: 1, PlanState: 1}).Error)
	require.NoError(t, sql.Create(&model.ExptLifecycleRunItem{ID: hookRecoveryIDs.Add(1), SpaceID: key.WorkspaceID, ExptID: key.ExperimentID, ExptRunID: key.RunID, ItemID: itemID, ItemVersionID: 7, AdmittedAt: gptr.Of(time.Now())}).Error)
	po, err := convert.NewExptTurnResultRunLogConvertor().DO2PO(row)
	require.NoError(t, err)
	require.NoError(t, sql.Create(po).Error)
	r := store.NewHookTurnProgressRepo(p, func(context.Context) (string, error) { return "local", nil })
	b := f.ctx.Value(itemHookProgressContextKey{}).(itemHookProgressBinding)
	b.key, b.itemID, b.itemVersion, b.repo = key, itemID, 7, r
	f.ctx = context.WithValue(f.ctx, itemHookProgressContextKey{}, b)
	f.ctx = context.WithValue(f.ctx, itemHookExecutionKey{}, itemHookExecutionCheck(func(context.Context) error {
		if f.gate.waiting.Load() {
			return itemHookControlError{wait: true}
		}
		return nil
	}))
	return r, sql
}

func TestHookRecoveryFreshTargetPausePersistsThenContinues(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeRetryAll, entity.EvaluationModeRetryItems} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			f := newExecutionHookFixture(t, true, false, false)
			prepareHookContinuationRecords(f, false, false)
			r, _ := hookRecoveryStorage(t, f)
			base := f.etec.GetExistTurnResultRunLog(1)
			oldRecords := append([]*entity.EvaluatorRecord(nil), f.etec.ExptTurnRunResult.EvaluatorResults...)
			oldSnapshot, err := json.Marshal(oldRecords)
			require.NoError(t, err)
			persisted := make(map[int64]*evalmodel.EvaluatorRecord)
			for _, record := range oldRecords {
				persisted[record.ID] = evalconvert.ConvertEvaluatorRecordDO2PO(record)
			}
			fresh := *f.etec.ExptTurnRunResult.TargetResult
			fresh.ID = 101
			f.etec.Event.ExptRunMode, f.etec.Event.RetryTimes = mode, 0
			f.targets.EXPECT().ExecuteTarget(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(context.Context, int64, int64, int64, *entity.ExecuteTargetCtx, *entity.EvalTargetInputData) (*entity.EvalTargetRecord, error) {
				f.gate.waiting.Store(true)
				return &fresh, nil
			}).Times(1)
			paused := f.turnEval.Eval(f.ctx, f.etec)
			require.True(t, itemHookControlOnly(paused.EvalErr))
			assert.Empty(t, paused.EvaluatorResults, "fresh T2 must not return T1's E1 at early gate")
			require.NoError(t, (&ExptItemEvalCtxExecutor{}).storeTurnRunResult(f.ctx, f.etec, paused))
			stored, err := r.ReadTurnProgress(f.ctx, entity.HookTurnProgressIdentity(base))
			require.NoError(t, err)
			assert.Equal(t, int64(101), stored.TargetResultID)
			assert.Empty(t, stored.EvaluatorResultIds.Registered)
			assert.Empty(t, stored.EvaluatorResultIds.EvalVerIDToResID)
			afterSnapshot, err := json.Marshal(f.etec.ExptTurnRunResult.EvaluatorResults)
			require.NoError(t, err)
			assert.Equal(t, oldSnapshot, afterSnapshot, "old records are retained unchanged")
			f.etec.ExistItemEvalResult.TurnResultRunLogs[1] = stored
			f.etec.ExptTurnRunResult = &entity.ExptTurnRunResult{TargetResult: &fresh}
			f.etec.Event.HookControlContinuation = true
			f.gate.waiting.Store(false)
			b := f.ctx.Value(itemHookProgressContextKey{}).(itemHookProgressBinding)
			b.targets = new(sync.Map)
			f.ctx = context.WithValue(f.ctx, itemHookProgressContextKey{}, b)
			f.etec = hookRecoveryReload(t, f, persisted)
			assert.Empty(t, f.etec.ExptTurnRunResult.EvaluatorResults)
			f.evaluators.EXPECT().RunEvaluator(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, req *entity.RunEvaluatorRequest) (*entity.EvaluatorRecord, error) {
				return &entity.EvaluatorRecord{ID: req.EvaluatorVersionID + 2000, EvaluatorVersionID: req.EvaluatorVersionID, Status: entity.EvaluatorRunStatusSuccess}, nil
			}).Times(2)
			resumed := f.turnEval.Eval(f.ctx, f.etec)
			require.NoError(t, resumed.EvalErr)
			require.Len(t, resumed.EvaluatorResults, 2)
			for _, record := range resumed.EvaluatorResults {
				assert.Greater(t, record.ID, int64(2000))
			}
		})
	}
}
