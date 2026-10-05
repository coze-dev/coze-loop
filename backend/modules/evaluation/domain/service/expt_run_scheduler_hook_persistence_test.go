// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	mm "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/metrics/mocks"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/events"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"gorm.io/gorm"
)

type hookPersistencePublisher struct {
	events.ExptEventPublisher
	publish func(context.Context) error
}

func (p hookPersistencePublisher) BatchPublishExptRecordEvalEvent(ctx context.Context, _ []*entity.ExptItemEvalEvent, _ *time.Duration) error {
	return p.publish(ctx)
}

// Only the unrelated ClickHouse filter side effect is replaced; all state writes use MySQL.
type hookPersistenceFilters struct{ ExptResultService }

func (hookPersistenceFilters) UpsertExptTurnResultFilter(context.Context, int64, int64, []int64) error {
	return nil
}

func hookPersistenceScheduler(t *testing.T, f *finalizationManagerFixture, publish func(context.Context) error) *ExptSchedulerImpl {
	t.Helper()
	items := exptinfra.NewExptItemResultRepo(mysql.NewExptItemResultDAO(f.p))
	turns := exptinfra.NewExptTurnResultRepo(nil, mysql.NewExptTurnResultDAO(f.p), nil)
	metric := mm.NewMockExptMetric(gomock.NewController(t))
	metric.EXPECT().EmitItemExecEval(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	base := &ExptSchedulerImpl{Manager: f.manager, Publisher: hookPersistencePublisher{publish: publish}, Configer: &schedulerHookConfig{}, Metric: metric,
		ExptItemResultRepo: items, ExptTurnResultRepo: turns, ExptStatsRepo: exptinfra.NewExptStatsRepo(mysql.NewExptStatsDAO(f.p)), ResultSvc: &ExptResultServiceImpl{}}
	aware, err := NewHookAwareExptSchedulerSvc(base, exptinfra.NewHookExecutionGateRepo(f.p, func(context.Context) (string, error) { return "local", nil }))
	require.NoError(t, err)
	scheduler := aware.(*ExptSchedulerImpl)
	scheduler.ResultSvc = hookPersistenceFilters{scheduler.ResultSvc}
	return scheduler
}

type hookPersistenceSnapshot struct {
	Item  model.ExptItemResult
	Log   model.ExptItemResultRunLog
	Turn  model.ExptTurnResult
	Stats model.ExptStats
}

func hookPersistenceRead(t *testing.T, f *finalizationManagerFixture, m entity.HookExecutionManifest) hookPersistenceSnapshot {
	t.Helper()
	var out hookPersistenceSnapshot
	require.NoError(t, f.sql.First(&out.Item, m.ItemResultID).Error)
	require.NoError(t, f.sql.First(&out.Log, m.ItemRunLogID).Error)
	require.NoError(t, f.sql.First(&out.Turn, m.Turns[0].ResultID).Error)
	require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&out.Stats).Error)
	return out
}

// Removing the Hook persistence fence revives Terminal rows after a completed cancellation.
func TestHookSchedulerPersistencePausedPublishCancellation(t *testing.T) {
	f, manifests := activeTerminationFixture(t, entity.ItemRunState_Queueing)
	m := manifests[0]
	entered, resume := make(chan struct{}), make(chan struct{})
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	scheduler := hookPersistenceScheduler(t, f, func(ctx context.Context) error {
		close(entered)
		select {
		case <-resume:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	event := &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ExptRunMode: entity.EvaluationModeSubmit}
	done := make(chan error, 1)
	go func() {
		done <- scheduler.handleToSubmits(ctx, event, []*entity.ExptEvalItem{{ItemID: m.Frozen.ItemID, State: entity.ItemRunState_Queueing}})
	}()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("scheduler did not reach BatchPublish")
	}
	require.NoError(t, f.manager.Kill(ctx, f.expt, &f.key.RunID, f.space, "cancel paused dispatch", nil))
	require.True(t, finalizationRead(t, f).State.After.Activated)
	before := hookPersistenceRead(t, f, m)
	close(resume)
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("scheduler did not resume")
	}
	after := hookPersistenceRead(t, f, m)
	require.Equal(t, int32(entity.ItemRunState_Terminal), after.Log.Status, "late scheduler revived the original runlog")
	require.Equal(t, before, after, "late scheduler changed finalized rows or counters")
}

func hookPersistenceDispatch(ctx context.Context, scheduler *ExptSchedulerImpl, f *finalizationManagerFixture, manifests []entity.HookExecutionManifest) error {
	items := make([]*entity.ExptEvalItem, 0, len(manifests))
	for _, m := range manifests {
		items = append(items, &entity.ExptEvalItem{ItemID: m.Frozen.ItemID, State: entity.ItemRunState_Queueing})
	}
	return scheduler.handleToSubmits(ctx, &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ExptRunMode: entity.EvaluationModeSubmit}, items)
}

func TestHookSchedulerPersistenceActiveAndReplay(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeTrialRun} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			f, ms := activeTerminationFixture(t, entity.ItemRunState_Queueing)
			require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", f.key.RunID).UpdateColumn("mode", int32(mode)).Error)
			scheduler := hookPersistenceScheduler(t, f, func(context.Context) error { return nil })
			ctx := context.Background()
			require.NoError(t, hookPersistenceDispatch(ctx, scheduler, f, ms))
			first := hookPersistenceRead(t, f, ms[0])
			require.Equal(t, int32(entity.ItemRunState_Processing), first.Item.Status)
			require.Equal(t, int32(entity.ItemRunState_Processing), first.Log.Status)
			require.Equal(t, int32(entity.TurnRunState_Processing), first.Turn.Status)
			require.Zero(t, first.Stats.PendingCnt)
			require.Equal(t, int32(1), first.Stats.ProcessingCnt)
			require.Equal(t, int32(entity.ExptItemResultStateLogged), gptr.Indirect(first.Log.ResultState))
			require.NoError(t, hookPersistenceDispatch(ctx, scheduler, f, ms))
			require.Equal(t, first, hookPersistenceRead(t, f, ms[0]), "replay must not increment counters or rewrite rows")
		})
	}
}

func TestHookSchedulerPersistenceCompletedItem(t *testing.T) {
	f, ms := activeTerminationFixture(t, entity.ItemRunState_Success)
	before := hookPersistenceRead(t, f, ms[0])
	scheduler := hookPersistenceScheduler(t, f, func(context.Context) error { return nil })
	require.NoError(t, hookPersistenceDispatch(context.Background(), scheduler, f, ms))
	require.Equal(t, before, hookPersistenceRead(t, f, ms[0]))
}

func TestHookSchedulerPersistencePausedPublishNewLatest(t *testing.T) {
	f, ms := activeTerminationFixture(t, entity.ItemRunState_Queueing)
	ctx := context.Background()
	var before hookPersistenceSnapshot
	scheduler := hookPersistenceScheduler(t, f, func(context.Context) error {
		next := finalizationTestIDs.Add(1)
		_, err := f.repo.CreateRunWithHooks(ctx, entity.HookCreateRunInput{
			Key: entity.HookRunKey{WorkspaceID: f.space, ExperimentID: f.expt, RunID: next}, ExpectedLatestRunID: f.key.RunID,
			RunLog:   &entity.ExptRunLog{ID: next, SpaceID: f.space, ExptID: f.expt, ExptRunID: next, Mode: 1, Status: 3, CreatedBy: "user"},
			Snapshot: entity.HookProtectedSnapshot{Cipher: []byte{1}, Hash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", KeyID: "key", ExecutionScope: "local"},
			After:    &entity.HookOperationSeed{ID: finalizationTestIDs.Add(1), OperationID: fmt.Sprint(next), IdempotencyKey: fmt.Sprint(next)},
		})
		require.NoError(t, err)
		for _, table := range []any{&model.ExptItemResult{}, &model.ExptTurnResult{}} {
			require.NoError(t, f.sql.Model(table).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("expt_run_id", next).Error)
		}
		before = hookPersistenceRead(t, f, ms[0])
		return nil
	})
	require.NoError(t, hookPersistenceDispatch(ctx, scheduler, f, ms))
	require.Equal(t, before, hookPersistenceRead(t, f, ms[0]), "old tick must not touch the newer Run or original log")
}

func TestHookSchedulerPersistenceRollback(t *testing.T) {
	f, ms := activeTerminationFixture(t, entity.ItemRunState_Queueing, entity.ItemRunState_Queueing)
	require.NoError(t, f.sql.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", f.space, f.expt).UpdateColumn("pending_cnt", 2).Error)
	before := []hookPersistenceSnapshot{hookPersistenceRead(t, f, ms[0]), hookPersistenceRead(t, f, ms[1])}
	injected := errors.New("scheduler stats persistence failure")
	require.NoError(t, f.sql.Callback().Update().Before("gorm:update").Register("hook-scheduler-stats-failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "expt_stats" {
			tx.AddError(injected)
		}
	}))
	t.Cleanup(func() { _ = f.sql.Callback().Update().Remove("hook-scheduler-stats-failure") })
	scheduler := hookPersistenceScheduler(t, f, func(context.Context) error { return nil })
	require.ErrorIs(t, hookPersistenceDispatch(context.Background(), scheduler, f, ms), injected)
	for i, m := range ms {
		require.Equal(t, before[i], hookPersistenceRead(t, f, m), "all item and turn writes must roll back with stats")
	}
}

func TestHookSchedulerPersistenceRejectsWrongFrozenIdentity(t *testing.T) {
	for _, kind := range []string{"item-version", "item-run", "turn-id", "turn-index", "scope", "foreign-item", "duplicate-item"} {
		t.Run(kind, func(t *testing.T) {
			f, ms := activeTerminationFixture(t, entity.ItemRunState_Queueing)
			scheduler := hookPersistenceScheduler(t, f, func(context.Context) error { return nil })
			switch kind {
			case "item-version":
				require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("id=?", ms[0].ItemRunLogID).UpdateColumn("item_version_id", 777).Error)
			case "item-run":
				require.NoError(t, f.sql.Model(&model.ExptItemResult{}).Where("id=?", ms[0].ItemResultID).UpdateColumn("expt_run_id", 777).Error)
			case "turn-id":
				require.NoError(t, f.sql.Model(&model.ExptTurnResult{}).Where("id=?", ms[0].Turns[0].ResultID).UpdateColumn("turn_id", 777).Error)
			case "turn-index":
				require.NoError(t, f.sql.Model(&model.ExptTurnResult{}).Where("id=?", ms[0].Turns[0].ResultID).UpdateColumn("turn_idx", 777).Error)
			case "scope":
				scheduler.hookSchedulerScope = "other"
			}
			before := hookPersistenceRead(t, f, ms[0])
			input := append([]entity.HookExecutionManifest(nil), ms...)
			if kind == "foreign-item" {
				foreign := ms[0]
				foreign.Frozen.ItemID = 777
				input = append(input, foreign)
			}
			if kind == "duplicate-item" {
				input = append(input, ms[0])
			}
			require.Error(t, hookPersistenceDispatch(context.Background(), scheduler, f, input))
			require.Equal(t, before, hookPersistenceRead(t, f, ms[0]))
		})
	}
}

func TestHookSchedulerPersistenceLegacyBranchUnchanged(t *testing.T) {
	f, ms := activeTerminationFixture(t, entity.ItemRunState_Queueing)
	require.NoError(t, f.sql.Model(&model.ExptRunLog{}).Where("id=?", f.key.RunID).UpdateColumn("lifecycle_hook_version", 0).Error)
	scheduler := hookPersistenceScheduler(t, f, func(context.Context) error { return nil })
	scheduler.hookSchedulerScope = "unused-for-legacy"
	require.NoError(t, hookPersistenceDispatch(context.Background(), scheduler, f, ms))
	after := hookPersistenceRead(t, f, ms[0])
	require.Equal(t, int32(entity.ItemRunState_Processing), after.Log.Status)
	require.Equal(t, int32(entity.ItemRunState_Processing), after.Item.Status)
	require.Equal(t, int32(entity.TurnRunState_Processing), after.Turn.Status)
	require.Zero(t, after.Stats.PendingCnt)
	require.Equal(t, int32(1), after.Stats.ProcessingCnt)
}
