// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func zeroTimeoutFixture(t *testing.T) (*finalizationManagerFixture, entity.HookExecutionManifest, *ExptSchedulerImpl) {
	t.Helper()
	f, ms := activeTerminationFixture(t, entity.ItemRunState_Queueing)
	m := lazyTurnManifest(t, f, ms[0], 2)
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("execution_started", false).Error)
	s := hookPersistenceScheduler(t, f, func(context.Context) error { return nil })
	s.Configer = &failureConfig{}
	archive, err := (&ExptResultServiceImpl{}).WithHookArchive(f.deps.Repository.(repo.IHookItemArchiveRepo), "local")
	require.NoError(t, err)
	require.NoError(t, hookPersistenceDispatch(context.Background(), s, f, []entity.HookExecutionManifest{m}))
	s.ResultSvc = archive
	assertLazyTurnCount(t, f, 0)
	require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("id=?", m.ItemRunLogID).UpdateColumn("updated_at", time.Now().UTC().Add(-4*time.Hour)).Error)
	return f, m, s
}

func expireZeroTimeout(ctx context.Context, s *ExptSchedulerImpl, f *finalizationManagerFixture, m entity.HookExecutionManifest) error {
	return callSchedulerFailure(ctx, s, f, m, &entity.Experiment{ID: f.expt, SpaceID: f.space}, "zombie")
}

func archiveZeroTimeout(t *testing.T, f *finalizationManagerFixture, m entity.HookExecutionManifest, s *ExptSchedulerImpl) {
	t.Helper()
	refs, err := s.ResultSvc.RecordItemRunLogs(context.Background(), f.expt, f.key.RunID, m.Frozen.ItemID, f.space, &entity.Experiment{ID: f.expt, SpaceID: f.space})
	require.NoError(t, err)
	require.Empty(t, refs)
	assertLazyTurnCount(t, f, 0)
	var item model.ExptItemResultRunLog
	require.NoError(t, f.sql.First(&item, m.ItemRunLogID).Error)
	require.Equal(t, int32(entity.ItemRunState_Fail), item.Status)
	require.Equal(t, int32(entity.ExptItemResultStateResulted), gptr.Indirect(item.ResultState))
	for _, mt := range m.Turns {
		var tr model.ExptTurnResult
		require.NoError(t, f.sql.First(&tr, mt.ResultID).Error)
		require.Equal(t, int32(entity.TurnRunState_Fail), tr.Status)
		require.Equal(t, gptr.Indirect(item.ErrMsg), gptr.Indirect(tr.ErrMsg))
		require.Zero(t, tr.TargetResultID)
		require.Nil(t, tr.WeightedScore)
	}
}

func TestHookZeroTurnTimeoutDispatchToFinalization(t *testing.T) {
	for _, admitted := range []bool{false, true} {
		t.Run(fmt.Sprint(admitted), func(t *testing.T) {
			f, m, s := zeroTimeoutFixture(t)
			if admitted {
				ctx, eiec, pre := admitLazyTurn(t, f, m)
				require.NoError(t, f.sql.Callback().Create().Before("gorm:create").Register("zero-timeout-preeval-fail", func(tx *gorm.DB) {
					if tx.Statement.Table == "expt_turn_result_run_log" {
						tx.AddError(errors.New("PreEval unavailable"))
					}
				}))
				err := pre.PreEval(ctx, eiec)
				require.NoError(t, f.sql.Callback().Create().Remove("zero-timeout-preeval-fail"))
				require.Error(t, err)
			}
			var ledgerBefore model.ExptLifecycleRunItem
			require.NoError(t, f.sql.First(&ledgerBefore, m.Frozen.ID).Error)
			require.NoError(t, expireZeroTimeout(context.Background(), s, f, m), "genuine dispatched no-PreEval timeout must use normal failure path")
			var marked model.ExptLifecycleRunItem
			require.NoError(t, f.sql.First(&marked, m.Frozen.ID).Error)
			var blob map[string]any
			require.NoError(t, json.Unmarshal(gptr.Indirect(marked.ExecutionManifest), &blob))
			require.Equal(t, true, blob["no_execution_failure"], "platform failure must have explicit durable no-execution proof")
			var item model.ExptItemResultRunLog
			require.NoError(t, f.sql.First(&item, m.ItemRunLogID).Error)
			require.Equal(t, int32(entity.ItemRunState_Fail), item.Status)
			require.Equal(t, int32(entity.ExptItemResultStateLogged), gptr.Indirect(item.ResultState))
			ok, _ := errno.ParseItemZombieTimeoutErr(errno.DeserializeErr(gptr.Indirect(item.ErrMsg)))
			require.True(t, ok)
			archiveZeroTimeout(t, f, m, s)
			require.NoError(t, f.manager.FinalizeRun(context.Background(), f.key, entity.HookTerminalIntent{}))
			done := finalizationRead(t, f)
			require.Equal(t, entity.ExptStatus_Failed, done.State.Intent.Status)
			require.True(t, done.State.After.Activated)
			var run model.ExptRunLog
			require.NoError(t, f.sql.First(&run, f.key.RunID).Error)
			require.Equal(t, int32(2), run.FailCnt)
			require.Zero(t, run.SuccessCnt+run.TerminatedCnt+run.ProcessingCnt+run.PendingCnt)
			var ledgerAfter model.ExptLifecycleRunItem
			require.NoError(t, f.sql.First(&ledgerAfter, m.Frozen.ID).Error)
			require.Equal(t, ledgerBefore.AdmittedAt, ledgerAfter.AdmittedAt)
			var life model.ExptLifecycleRun
			require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).First(&life).Error)
			require.Equal(t, admitted, life.ExecutionStarted)
			assertLazyTurnCount(t, f, 0)
		})
	}
}

func TestHookZeroTurnTimeoutRejectsLatePreEval(t *testing.T) {
	f, m, s := zeroTimeoutFixture(t)
	ctx, eiec, pre := admitLazyTurn(t, f, m)
	entered, release := make(chan struct{}), make(chan struct{})
	pre.idgen = lazyTurnBarrierIDs{entered: entered, release: release}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- pre.PreEval(ctx, eiec) }()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("PreEval barrier not reached")
	}
	err := expireZeroTimeout(context.Background(), s, f, m)
	close(release)
	preErr := <-done
	require.NoError(t, err)
	require.Error(t, preErr, "late PreEval must reject failed item under lifecycle lock")
	assertLazyTurnCount(t, f, 0)
	archiveZeroTimeout(t, f, m, s)
}

func TestHookZeroTurnTimeoutCancellationOrdering(t *testing.T) {
	for _, first := range []string{"timeout", "cancel"} {
		t.Run(first, func(t *testing.T) {
			f, m, s := zeroTimeoutFixture(t)
			if first == "timeout" {
				require.NoError(t, expireZeroTimeout(context.Background(), s, f, m))
			}
			require.NoError(t, f.manager.Kill(context.Background(), f.expt, &f.key.RunID, f.space, "cancel", nil))
			if first == "cancel" {
				require.NoError(t, expireZeroTimeout(context.Background(), s, f, m))
			}
			var run model.ExptRunLog
			require.NoError(t, f.sql.First(&run, f.key.RunID).Error)
			var item model.ExptItemResultRunLog
			require.NoError(t, f.sql.First(&item, m.ItemRunLogID).Error)
			if first == "timeout" {
				require.Equal(t, int32(entity.ItemRunState_Fail), item.Status)
				require.Equal(t, int32(2), run.FailCnt)
				require.Zero(t, run.TerminatedCnt)
			} else {
				require.Equal(t, int32(entity.ItemRunState_Terminal), item.Status)
				require.Equal(t, int32(2), run.TerminatedCnt)
				require.Zero(t, run.FailCnt)
			}
			require.Equal(t, int32(entity.ExptItemResultStateResulted), gptr.Indirect(item.ResultState))
			require.True(t, finalizationRead(t, f).State.After.Activated)
			assertLazyTurnCount(t, f, 0)
		})
	}
}

func TestHookZeroTurnTimeoutCorruptEvidenceFailsClosed(t *testing.T) {
	for _, kind := range []string{"unexplained", "wrong-code", "deleted-log", "foreign-space", "foreign-expt", "partial", "extra-turn", "score", "target", "foreign-ref", "foreign-expt-ref", "extra-projection"} {
		t.Run(kind, func(t *testing.T) {
			f, m, s := zeroTimeoutFixture(t)
			if kind == "foreign-space" || kind == "foreign-expt" {
				ctx, eiec, pre := admitLazyTurn(t, f, m)
				require.NoError(t, pre.PreEval(ctx, eiec))
				var row model.ExptTurnResultRunLog
				require.NoError(t, f.sql.Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", f.space, f.expt, f.key.RunID, m.Frozen.ItemID).Order("turn_id").First(&row).Error)
				if kind == "foreign-space" {
					row.SpaceID++
				} else {
					row.ExptID++
				}
				require.NoError(t, f.sql.Save(&row).Error)
				t.Cleanup(func() { require.NoError(t, f.sql.Unscoped().Delete(&row).Error) })
				require.Error(t, expireZeroTimeout(context.Background(), s, f, m))
				require.False(t, finalizationRead(t, f).State.After.Activated)
				return
			}
			require.NoError(t, expireZeroTimeout(context.Background(), s, f, m))
			switch kind {
			case "unexplained", "wrong-code":
				// Error text alone never proves that execution did not happen.
				raw, err := json.Marshal(m)
				require.NoError(t, err)
				require.NoError(t, f.sql.Model(&model.ExptLifecycleRunItem{}).Where("id=?", m.Frozen.ID).UpdateColumn("execution_manifest", raw).Error)
				message := "unexplained failure"
				if kind == "wrong-code" {
					message = errno.SerializeErr(errno.NewItemManuallyTerminatedErr())
				}
				require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("id=?", m.ItemRunLogID).UpdateColumn("err_msg", []byte(message)).Error)
			case "score":
				require.NoError(t, f.sql.Model(&model.ExptTurnResult{}).Where("id=?", m.Turns[0].ResultID).UpdateColumn("weighted_score", 1).Error)
			case "target":
				require.NoError(t, f.sql.Model(&model.ExptTurnResult{}).Where("id=?", m.Turns[0].ResultID).UpdateColumn("target_result_id", 777).Error)
			case "foreign-ref", "foreign-expt-ref":
				row := model.ExptTurnEvaluatorResultRef{ID: finalizationTestIDs.Add(1), SpaceID: f.space + 1, ExptID: f.expt, ExptTurnResultID: m.Turns[0].ResultID, EvaluatorVersionID: 93, EvaluatorResultID: 777}
				if kind == "foreign-expt-ref" {
					row.SpaceID = f.space
					row.ExptID = f.expt + 1
				}
				require.NoError(t, f.sql.Create(&row).Error)
				t.Cleanup(func() { require.NoError(t, f.sql.Unscoped().Delete(&row).Error) })
			case "extra-projection":
				require.NoError(t, f.sql.Create(&model.ExptTurnResult{ID: finalizationTestIDs.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: m.Frozen.ItemID, TurnID: 999, Status: int32(entity.TurnRunState_Processing)}).Error)
			default:
				row := model.ExptTurnResultRunLog{ID: finalizationTestIDs.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: m.Frozen.ItemID, ItemVersionID: m.Frozen.ItemVersionID, TurnID: 0, Status: int32(entity.TurnRunState_Fail)}
				switch kind {
				case "deleted-log":
					row.DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}
				case "foreign-space":
					row.SpaceID++
				case "foreign-expt":
					row.ExptID++
				case "extra-turn":
					row.TurnID = 999
				}
				require.NoError(t, f.sql.Create(&row).Error)
				t.Cleanup(func() { require.NoError(t, f.sql.Unscoped().Delete(&row).Error) })
			}
			_, err := s.ResultSvc.RecordItemRunLogs(context.Background(), f.expt, f.key.RunID, m.Frozen.ItemID, f.space, &entity.Experiment{ID: f.expt, SpaceID: f.space})
			require.Error(t, err)
			var item model.ExptItemResultRunLog
			require.NoError(t, f.sql.First(&item, m.ItemRunLogID).Error)
			require.NotEqual(t, int32(entity.ExptItemResultStateResulted), gptr.Indirect(item.ResultState))
			require.False(t, finalizationRead(t, f).State.After.Activated)
		})
	}
}

func TestHookZeroTurnTimeoutFinalProofRejectsUnexpectedOwnedLog(t *testing.T) {
	f, m, s := zeroTimeoutFixture(t)
	require.NoError(t, expireZeroTimeout(context.Background(), s, f, m))
	archiveZeroTimeout(t, f, m, s)
	row := model.ExptTurnResultRunLog{ID: finalizationTestIDs.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: m.Frozen.ItemID, ItemVersionID: m.Frozen.ItemVersionID, TurnID: 0, Status: int32(entity.TurnRunState_Fail)}
	require.NoError(t, f.sql.Create(&row).Error)
	t.Cleanup(func() { require.NoError(t, f.sql.Unscoped().Delete(&row).Error) })
	require.Error(t, f.manager.FinalizeRun(context.Background(), f.key, entity.HookTerminalIntent{}))
	require.False(t, finalizationRead(t, f).State.After.Activated)
}

func TestHookZeroTurnTimeoutTransactionPrecedesPreEval(t *testing.T) {
	f, m, s := zeroTimeoutFixture(t)
	ctx, eiec, pre := admitLazyTurn(t, f, m)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	entered, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	require.NoError(t, f.sql.Callback().Update().Before("gorm:update").Register("zero-timeout-lock", func(tx *gorm.DB) {
		if tx.Statement.Table == "expt_item_result_run_log" {
			close(entered)
			select {
			case <-release:
			case <-ctx.Done():
				tx.AddError(ctx.Err())
			}
		}
	}))
	expired, created := make(chan error, 1), make(chan error, 1)
	go func() { expired <- expireZeroTimeout(ctx, s, f, m) }()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("timeout transaction not reached")
	}
	go func() { created <- pre.PreEval(ctx, eiec) }()
	require.Eventually(t, func() bool {
		var n int64
		err := f.sql.Raw("SELECT COUNT(*) FROM performance_schema.data_lock_waits w JOIN performance_schema.data_locks l ON w.BLOCKING_ENGINE_LOCK_ID=l.ENGINE_LOCK_ID WHERE l.OBJECT_SCHEMA=DATABASE() AND l.OBJECT_NAME='experiment' AND l.LOCK_DATA=?", fmt.Sprint(f.expt)).Scan(&n).Error
		return err == nil && n > 0
	}, 3*time.Second, 10*time.Millisecond)
	unblock()
	require.NoError(t, <-expired)
	require.Error(t, <-created)
	require.NoError(t, f.sql.Callback().Update().Remove("zero-timeout-lock"))
	assertLazyTurnCount(t, f, 0)
	archiveZeroTimeout(t, f, m, s)
}
