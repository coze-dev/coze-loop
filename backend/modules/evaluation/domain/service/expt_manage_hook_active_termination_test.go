// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	mm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	cm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	sm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service/mocks"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
)

func activeTerminationFixture(t *testing.T, states ...entity.ItemRunState) (*finalizationManagerFixture, []entity.HookExecutionManifest) {
	t.Helper()
	return activeTerminationFixtureDB(t, "tx", states...)
}
func activeTerminationFixtureDB(t *testing.T, database string, states ...entity.ItemRunState) (*finalizationManagerFixture, []entity.HookExecutionManifest) {
	t.Helper()
	f := neverAdmittedFinalizationFixture(t, true, true, database)
	f.deps.NewItemLocker = func() lock.ILocker { return lock.NewRedisLocker(f.redis) }
	finalizationRecreate(t, f)
	c := claimTerminationBefore(t, f)
	_, err := f.repo.CompleteAttempt(context.Background(), c)
	require.NoError(t, err)
	require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).Delete(&model.ExptLifecycleRunItem{}).Error)
	digest := entity.NewHookPlanDigest()
	var manifests []entity.HookExecutionManifest
	for i, state := range states {
		item := entity.HookPlanItem{ID: finalizationTestIDs.Add(1), SourceSpaceID: f.space, EvalSetID: 71, ItemID: finalizationTestIDs.Add(1), ItemVersionID: int64(i)}
		m := entity.HookExecutionManifest{Version: 1, Key: f.key, Ordinal: int64(i), Frozen: item, ItemResultID: finalizationTestIDs.Add(1), ItemRunLogID: finalizationTestIDs.Add(1), Turns: []entity.HookExecutionTurnManifest{{TurnID: 0, ResultID: finalizationTestIDs.Add(1)}}, TurnLogsInitialized: gptr.Of(false)}
		raw, err := json.Marshal(m)
		require.NoError(t, err)
		row := model.ExptLifecycleRunItem{ID: item.ID, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, Ordinal: int64(i), SourceSpaceID: f.space, EvalSetID: 71, ItemID: item.ItemID, ItemVersionID: item.ItemVersionID, ExecutionManifest: &raw}
		turnStatus := entity.TurnRunState_Queueing
		resulted := int32(entity.ExptItemResultStateLogged)
		if state != entity.ItemRunState_Queueing {
			row.AdmittedAt = gptr.Of(time.Now().UTC())
			switch state {
			case entity.ItemRunState_Processing:
				turnStatus = entity.TurnRunState_Processing
			case entity.ItemRunState_Success:
				turnStatus = entity.TurnRunState_Success
			case entity.ItemRunState_Fail:
				turnStatus = entity.TurnRunState_Fail
			case entity.ItemRunState_Terminal:
				turnStatus = entity.TurnRunState_Terminal
			}
			m.Turns[0].RunLogID = finalizationTestIDs.Add(1)
			m.TurnLogsInitialized = gptr.Of(true)
			raw, err = json.Marshal(m)
			require.NoError(t, err)
			require.NoError(t, f.sql.Create(&model.ExptTurnResultRunLog{ID: m.Turns[0].RunLogID, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: item.ItemID, ItemVersionID: item.ItemVersionID, TurnID: 0, Status: int32(turnStatus), LogID: "original-log"}).Error)
		}
		if state == entity.ItemRunState_Success || state == entity.ItemRunState_Fail {
			resulted = int32(entity.ExptItemResultStateResulted)
		}
		require.NoError(t, f.sql.Create(&row).Error)
		require.NoError(t, f.sql.Create(&model.ExptItemResultRunLog{ID: m.ItemRunLogID, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: item.ItemID, ItemVersionID: item.ItemVersionID, Status: int32(state), ResultState: &resulted}).Error)
		require.NoError(t, f.sql.Create(&model.ExptItemResult{ID: m.ItemResultID, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: item.ItemID, ItemVersionID: item.ItemVersionID, ItemIdx: gptr.Of(int32(i)), Status: int32(state)}).Error)
		require.NoError(t, f.sql.Create(&model.ExptTurnResult{ID: m.Turns[0].ResultID, SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: item.ItemID, ItemVersionID: item.ItemVersionID, TurnIdx: gptr.Of(int32(0)), Status: int32(turnStatus), LogID: "original-log"}).Error)
		digest, err = entity.AppendHookPlanDigest(digest, []entity.HookPlanItem{item})
		require.NoError(t, err)
		manifests = append(manifests, m)
	}
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumns(map[string]any{"plan_count": len(states), "plan_hash": digest.Hash, "execution_initialized": true, "execution_started": true}).Error)
	return f, manifests
}

type activeTerminationTarget struct {
	IEvalTargetService
	record *entity.EvalTargetRecord
	fail   bool
	calls  int
	keys   []entity.HookRunKey
}

func (s *activeTerminationTarget) GetRecordByID(_ context.Context, space, id int64) (*entity.EvalTargetRecord, error) {
	if s.record.SpaceID != space || s.record.ID != id {
		return nil, errors.New("wrong target lookup")
	}
	return s.record, nil
}
func (s *activeTerminationTarget) CleanupHookTargetSandboxes(_ context.Context, key entity.HookRunKey, records []*entity.EvalTargetRecord) error {
	s.calls++
	s.keys = append(s.keys, key)
	if len(records) != 1 || records[0].ID != s.record.ID {
		return errors.New("missing original reference")
	}
	if s.fail {
		return errors.New("cleanup uncertain")
	}
	return nil
}

// Dropping target references (or treating terminal logs as already cleaned) must fail this test.
func TestHookActiveTerminationCleanupRetryRetainsRefs(t *testing.T) {
	f, ms := activeTerminationFixtureDB(t, "tx", entity.ItemRunState_Processing)
	m := ms[0]
	ctx := context.Background()
	target := &activeTerminationTarget{fail: true, record: &entity.EvalTargetRecord{ID: finalizationTestIDs.Add(1), SpaceID: f.space + 500, TargetID: 91, TargetVersionID: 92, ExperimentRunID: f.key.RunID, ItemID: m.Frozen.ItemID, ItemVersionID: m.Frozen.ItemVersionID, TurnID: 0, Status: gptr.Of(entity.EvalTargetRunStatusAsyncInvoking), Ext: map[string]string{"sandbox_execute_ids": "[\"owned-execute\"]"}}}
	persistActiveTerminationTarget(t, f, target.record)
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumns(map[string]any{"target_id": 91, "target_version_id": 92, "target_space_id": target.record.SpaceID}).Error)
	require.NoError(t, f.sql.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("target_result_id", target.record.ID).Error)
	f.base.(*ExptMangerImpl).evalTargetService = target
	finalizationRecreate(t, f)
	require.ErrorContains(t, f.manager.Kill(ctx, f.expt, &f.key.RunID, f.space, "quota exhausted", nil), "cleanup uncertain")
	pending := finalizationRead(t, f)
	require.False(t, pending.State.After.Activated)
	require.Equal(t, entity.HookFinalizePending, pending.State.Finalize)
	var item model.ExptItemResultRunLog
	require.NoError(t, f.sql.First(&item, m.ItemRunLogID).Error)
	require.Equal(t, int32(entity.ItemRunState_Terminal), item.Status)
	require.NotEqual(t, int32(entity.ExptItemResultStateResulted), gptr.Indirect(item.ResultState))
	target.fail = false
	require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
	require.Equal(t, 2, target.calls)
	require.Equal(t, []entity.HookRunKey{f.key, f.key}, target.keys)
	var turn model.ExptTurnResult
	require.NoError(t, f.sql.First(&turn, m.Turns[0].ResultID).Error)
	require.Equal(t, target.record.ID, turn.TargetResultID)
	require.Equal(t, int32(entity.TurnRunState_Terminal), turn.Status)
	require.Equal(t, "quota exhausted", gptr.Indirect(finalizationRead(t, f).DisplayMessage))
}

// Success-like data captured before cancellation must not revive the read side or counters.
func TestHookActiveTerminationLateArchiveRejected(t *testing.T) {
	f, ms := activeTerminationFixture(t, entity.ItemRunState_Processing)
	storage, ok := f.deps.Repository.(repo.IHookItemArchiveRepo)
	require.True(t, ok)
	ctx := context.Background()
	before, err := storage.ReadHookArchiveItem(ctx, f.key, "local", ms[0].Frozen.ItemID)
	require.NoError(t, err)
	require.NoError(t, f.manager.Kill(ctx, f.expt, &f.key.RunID, f.space, "cancel", nil))
	_, err = storage.ArchiveHookItem(ctx, entity.HookItemArchiveInput{Key: f.key, ExecutionScope: "local", ItemID: ms[0].Frozen.ItemID, Prepared: before})
	require.Error(t, err)
	var tr model.ExptTurnResult
	require.NoError(t, f.sql.First(&tr, ms[0].Turns[0].ResultID).Error)
	require.Equal(t, int32(entity.TurnRunState_Terminal), tr.Status)
}

func TestHookActiveTerminationDebtMiddleware(t *testing.T) {
	f, ms := activeTerminationFixture(t, entity.ItemRunState_Processing)
	ctrl := gomock.NewController(t)
	config := cm.NewMockIConfiger(ctrl)
	metric := mm.NewMockExptMetric(ctrl)
	debt := errors.New("insufficient credit")
	config.EXPECT().GetErrRetryConf(gomock.Any(), f.space, debt).Return(&entity.RetryConf{IsInDebt: true})
	metric.EXPECT().EmitItemExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
	svc := &ExptItemEventEvalServiceImpl{manager: f.manager, mutex: f.manager.mutex, configer: config, metric: metric, hookAdmission: &itemHookAdmission{source: exptinfra.NewHookItemSourceRepo(f.p)}}
	event := &entity.ExptItemEvalEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, EvalSetItemID: ms[0].Frozen.ItemID, ExptRunMode: entity.EvaluationModeSubmit}
	endpoint := svc.HandleEventErr(svc.HandleEventLock(func(context.Context, *entity.ExptItemEvalEvent) error { return debt }))
	require.NoError(t, endpoint(context.Background(), event))
	done := finalizationRead(t, f)
	require.Equal(t, entity.HookFinalizeCommitted, done.State.Finalize)
	require.True(t, done.State.After.Activated)
	require.Equal(t, "insufficient credit", gptr.Indirect(done.DisplayMessage))
}

func TestHookActiveTerminationLegacyDebtSequence(t *testing.T) {
	ctrl := gomock.NewController(t)
	config := cm.NewMockIConfiger(ctrl)
	metric := mm.NewMockExptMetric(ctrl)
	manager := sm.NewMockIExptManager(ctrl)
	debt := errors.New("legacy debt")
	config.EXPECT().GetErrRetryConf(gomock.Any(), int64(1), debt).Return(&entity.RetryConf{IsInDebt: true})
	metric.EXPECT().EmitItemExecResult(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any())
	first := manager.EXPECT().CompleteRun(gomock.Any(), int64(2), int64(3), int64(1), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)
	manager.EXPECT().CompleteExpt(gomock.Any(), int64(2), gomock.Any(), int64(1), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).After(first).Return(nil)
	svc := &ExptItemEventEvalServiceImpl{manager: manager, configer: config, metric: metric}
	require.NoError(t, svc.HandleEventErr(func(context.Context, *entity.ExptItemEvalEvent) error { return debt })(context.Background(), &entity.ExptItemEvalEvent{SpaceID: 1, ExptID: 2, ExptRunID: 3}))
}

type activeFinalizationReadOnly struct{ repo.IHookFinalizationRepo }

func TestHookActiveTerminationMissingCleanupCapabilityStaysPending(t *testing.T) {
	f, _ := activeTerminationFixture(t, entity.ItemRunState_Success)
	f.deps.Repository = activeFinalizationReadOnly{f.deps.Repository}
	finalizationRecreate(t, f)
	require.Error(t, f.manager.Kill(context.Background(), f.expt, &f.key.RunID, f.space, "cancel", nil))
	require.False(t, finalizationRead(t, f).State.After.Activated)
}

func TestHookActiveTerminationMissingLockerFactoryStaysPending(t *testing.T) {
	f, _ := activeTerminationFixture(t, entity.ItemRunState_Success)
	f.deps.NewItemLocker = nil
	finalizationRecreate(t, f)
	require.Error(t, f.manager.Kill(context.Background(), f.expt, &f.key.RunID, f.space, "cancel", nil))
	require.False(t, finalizationRead(t, f).State.After.Activated)
}

func TestHookActiveTerminationPublicArchiveBoundary(t *testing.T) {
	f, ms := activeTerminationFixture(t, entity.ItemRunState_Success)
	type archiveOptIn interface {
		WithHookArchive(repo.IHookItemArchiveRepo, string) (ExptResultService, error)
	}
	opt, ok := any(&ExptResultServiceImpl{}).(archiveOptIn)
	require.True(t, ok, "real result service must expose a private opt-in archival boundary")
	svc, err := opt.WithHookArchive(f.deps.Repository.(repo.IHookItemArchiveRepo), "local")
	require.NoError(t, err)
	require.NoError(t, f.manager.SetExptTerminating(context.Background(), f.expt, f.key.RunID, f.space, nil))
	_, err = svc.RecordItemRunLogs(context.Background(), f.expt, f.key.RunID, ms[0].Frozen.ItemID, f.space, &entity.Experiment{ID: f.expt, SpaceID: f.space})
	require.ErrorIs(t, err, entity.ErrHookAdmissionDenied)
	require.False(t, finalizationRead(t, f).State.After.Activated)
}

func TestHookActiveTerminationMultipageRecovery(t *testing.T) {
	states := make([]entity.ItemRunState, 101)
	for i := range states {
		states[i] = entity.ItemRunState_Queueing
	}
	f, ms := activeTerminationFixture(t, states...)
	var writes atomic.Int32
	fail := errors.New("second page archive unavailable")
	require.NoError(t, f.sql.Callback().Update().Before("gorm:update").Register("active-second-page", func(tx *gorm.DB) {
		if tx.Statement.Table == "expt_item_result" && writes.Add(1) == 101 {
			tx.AddError(fail)
		}
	}))
	ctx := context.Background()
	require.ErrorIs(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{Status: entity.ExptStatus_SystemTerminated}), fail)
	require.NoError(t, f.sql.Callback().Update().Remove("active-second-page"))
	pending := finalizationRead(t, f)
	require.Equal(t, entity.HookFinalizePending, pending.State.Finalize)
	require.False(t, pending.State.After.Activated)
	var n int64
	require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND expt_run_id=? AND result_state=?", f.space, f.key.RunID, int32(entity.ExptItemResultStateResulted)).Count(&n).Error)
	require.Equal(t, int64(100), n)
	var last model.ExptItemResultRunLog
	require.NoError(t, f.sql.First(&last, ms[100].ItemRunLogID).Error)
	require.Equal(t, int32(entity.ItemRunState_Terminal), last.Status)
	require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
	var run model.ExptRunLog
	require.NoError(t, f.sql.First(&run, f.key.RunID).Error)
	require.Equal(t, int32(101), run.TerminatedCnt)
	require.Equal(t, int64(entity.ExptStatus_SystemTerminated), gptr.Indirect(run.Status))
	require.True(t, finalizationRead(t, f).State.After.Activated)
}

func TestHookActiveTerminationNewRunIsolation(t *testing.T) {
	f, ms := activeTerminationFixture(t, entity.ItemRunState_Processing, entity.ItemRunState_Queueing)
	ctx := context.Background()
	next := finalizationTestIDs.Add(1)
	key := entity.HookRunKey{WorkspaceID: f.space, ExperimentID: f.expt, RunID: next}
	created, err := f.repo.CreateRunWithHooks(ctx, entity.HookCreateRunInput{Key: key, ExpectedLatestRunID: f.key.RunID, RunLog: &entity.ExptRunLog{ID: next, ExptRunID: next, SpaceID: f.space, ExptID: f.expt, Mode: 1, Status: 3, CreatedBy: "user"}, Snapshot: entity.HookProtectedSnapshot{Cipher: []byte{1}, Hash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", KeyID: "key", ExecutionScope: "local"}, After: &entity.HookOperationSeed{ID: finalizationTestIDs.Add(1), OperationID: fmt.Sprint(next), IdempotencyKey: fmt.Sprint(next)}})
	require.NoError(t, err)
	for _, table := range []any{&model.ExptItemResult{}, &model.ExptTurnResult{}} {
		require.NoError(t, f.sql.Model(table).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("expt_run_id", next).Error)
	}
	var before model.ExptStats
	require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&before).Error)
	owner := fmt.Sprintf("hook_run:%d:abcdef0123456789abcdef0123456789", next)
	lockKey := fmt.Sprintf("expt_run_mutex_lock:%d", f.expt)
	require.NoError(t, f.redis.Set(ctx, lockKey, owner, time.Hour).Err())
	require.NoError(t, f.manager.Kill(ctx, f.expt, &f.key.RunID, f.space, "old run", nil))
	after, err := f.repo.GetRun(ctx, key)
	require.NoError(t, err)
	require.Equal(t, created.Run, after)
	var stats model.ExptStats
	require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&stats).Error)
	require.Equal(t, before, stats)
	var item model.ExptItemResult
	require.NoError(t, f.sql.First(&item, ms[0].ItemResultID).Error)
	require.Equal(t, next, item.ExptRunID)
	require.Equal(t, int32(entity.ItemRunState_Processing), item.Status)
	require.Equal(t, owner, f.redis.Get(ctx, lockKey).Val())
	require.Zero(t, f.notifications)
}

func TestHookActiveTerminationRejectsCorruptAdmittedRecords(t *testing.T) {
	for _, kind := range []string{"missing-turn", "wrong-version", "extra-turn", "wrong-result-id", "missing-item", "extra-item", "wrong-turn-index"} {
		t.Run(kind, func(t *testing.T) {
			f, ms := activeTerminationFixture(t, entity.ItemRunState_Processing)
			m := ms[0]
			switch kind {
			case "missing-turn":
				require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).Delete(&model.ExptTurnResultRunLog{}).Error)
			case "wrong-version":
				require.NoError(t, f.sql.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("item_version_id", 77).Error)
			case "extra-turn":
				require.NoError(t, f.sql.Create(&model.ExptTurnResultRunLog{ID: finalizationTestIDs.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: m.Frozen.ItemID, TurnID: 5, Status: 1}).Error)
			case "wrong-result-id":
				require.NoError(t, f.sql.Model(&model.ExptTurnResult{}).Where("id=?", m.Turns[0].ResultID).UpdateColumn("item_id", 88).Error)
			case "missing-item":
				require.NoError(t, f.sql.Where("id=?", m.ItemRunLogID).Delete(&model.ExptItemResultRunLog{}).Error)
			case "extra-item":
				require.NoError(t, f.sql.Create(&model.ExptItemResultRunLog{ID: finalizationTestIDs.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ItemID: finalizationTestIDs.Add(1), Status: 1}).Error)
			case "wrong-turn-index":
				require.NoError(t, f.sql.Model(&model.ExptTurnResult{}).Where("id=?", m.Turns[0].ResultID).UpdateColumn("turn_idx", 4).Error)
			}
			require.Error(t, f.manager.Kill(context.Background(), f.expt, &f.key.RunID, f.space, "cancel", nil))
			require.False(t, finalizationRead(t, f).State.After.Activated)
			require.Zero(t, f.notifications)
		})
	}
}

func TestHookActiveTerminationLateWritesAndBusyItem(t *testing.T) {
	f, ms := activeTerminationFixture(t, entity.ItemRunState_Processing)
	ctx := context.Background()
	itemID := ms[0].Frozen.ItemID
	writer := exptinfra.NewHookTurnProgressRepo(f.p, func(context.Context) (string, error) { return "local", nil })
	var tr model.ExptTurnResultRunLog
	require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).First(&tr).Error)
	k := entity.HookTurnProgressKey{HookRunKey: f.key, ItemID: itemID, ItemVersionID: 0, TurnID: 0, LogID: tr.ID}
	base, err := writer.ReadTurnProgress(ctx, k)
	require.NoError(t, err)
	lockKey := fmt.Sprintf("expt_item_eval_run_lock:%d:%d", f.expt, itemID)
	ok, _, cancel, err := f.manager.mutex.LockWithRenew(ctx, lockKey, 5*time.Second, time.Hour)
	require.NoError(t, err)
	require.True(t, ok)
	require.ErrorIs(t, f.manager.Kill(ctx, f.expt, &f.key.RunID, f.space, "cancel", nil), entity.ErrHookFinalizationUnsettled)
	cancel()
	_, err = f.manager.mutex.Unlock(lockKey)
	require.NoError(t, err)
	require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
	sealedTurn, err := writer.ReadTurnProgress(ctx, k)
	require.NoError(t, err)
	var sealedItem model.ExptItemResultRunLog
	require.NoError(t, f.sql.First(&sealedItem, ms[0].ItemRunLogID).Error)
	sealedState := finalizationRead(t, f)
	late := *base
	late.Status = entity.TurnRunState_Success
	got, err := writer.(repo.IHookTurnResultWriteRepo).WriteTurnResult(ctx, entity.HookTurnProgressInput{Base: base, Progress: &late})
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	require.Nil(t, got)
	_, err = writer.(repo.IHookItemRunWriteRepo).WriteItemRun(ctx, entity.HookItemRunWriteInput{HookRunKey: f.key, ItemID: itemID, Status: entity.ItemRunState_Success})
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	currentTurn, err := writer.ReadTurnProgress(ctx, k)
	require.NoError(t, err)
	require.Equal(t, sealedTurn, currentTurn)
	require.Equal(t, entity.TurnRunState_Terminal, currentTurn.Status)
	var item model.ExptItemResultRunLog
	require.NoError(t, f.sql.First(&item, ms[0].ItemRunLogID).Error)
	require.Equal(t, sealedItem, item)
	require.Equal(t, int32(entity.ExptItemResultStateResulted), gptr.Indirect(item.ResultState))
	require.Equal(t, sealedState, finalizationRead(t, f))
}

// The old coordinator rejects initialized cancellations instead of preparing real records.
func TestHookActiveTerminationPublicSequence(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeTrialRun} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			f, ms := activeTerminationFixture(t, entity.ItemRunState_Queueing, entity.ItemRunState_Processing, entity.ItemRunState_Success, entity.ItemRunState_Fail, entity.ItemRunState_Terminal)
			require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", f.key.RunID).UpdateColumn("mode", int32(mode)).Error)
			ctx := context.Background()
			require.NoError(t, f.manager.SetExptTerminating(ctx, f.expt, f.key.RunID, f.space, nil))
			require.NoError(t, f.manager.CompleteRun(ctx, f.expt, f.key.RunID, f.space, nil, entity.WithStatus(entity.ExptStatus_Terminated)))
			require.False(t, finalizationRead(t, f).State.After.Activated)
			require.Zero(t, f.notifications)
			require.NoError(t, f.manager.CompleteExpt(ctx, f.expt, &f.key.RunID, f.space, nil, entity.WithStatus(entity.ExptStatus_Terminated), entity.NoAggrCalculate()))
			done := finalizationRead(t, f)
			require.Equal(t, entity.HookFinalizeCommitted, done.State.Finalize)
			require.True(t, done.State.After.Activated)
			var stats model.ExptStats
			require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&stats).Error)
			require.Equal(t, int32(3), stats.TerminatedCnt)
			require.Equal(t, int32(1), stats.SuccessCnt)
			require.Equal(t, int32(1), stats.FailCnt)
			require.Zero(t, stats.PendingCnt+stats.ProcessingCnt)
			var run model.ExptRunLog
			require.NoError(t, f.sql.First(&run, f.key.RunID).Error)
			require.Equal(t, int32(3), run.TerminatedCnt)
			require.Equal(t, int32(1), run.SuccessCnt)
			require.Equal(t, int32(1), run.FailCnt)
			for i, m := range ms {
				var item model.ExptItemResultRunLog
				require.NoError(t, f.sql.First(&item, m.ItemRunLogID).Error)
				want := entity.ItemRunState_Terminal
				if i == 2 {
					want = entity.ItemRunState_Success
				}
				if i == 3 {
					want = entity.ItemRunState_Fail
				}
				require.Equal(t, int32(want), item.Status)
				require.Equal(t, int32(entity.ExptItemResultStateResulted), gptr.Indirect(item.ResultState))
			}
			var n int64
			require.NoError(t, f.sql.Model(&model.ExptTurnResultRunLog{}).Where("space_id=? AND expt_run_id=? AND item_id=?", f.space, f.key.RunID, ms[0].Frozen.ItemID).Count(&n).Error)
			require.Zero(t, n, "queued cancellation does not invent execution logs")
			require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
			require.Equal(t, done, finalizationRead(t, f))
			require.Equal(t, 1, f.notifications)
		})
	}
}
