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
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type schedulerInitFailureLoader struct {
	hook.PlanPageLoader
	err error
}

func (l *schedulerInitFailureLoader) LoadPage(ctx context.Context, in entity.HookPlanReadInput) (*entity.HookLoadedPlanPage, error) {
	if l.err != nil {
		return nil, l.err
	}
	return l.PlanPageLoader.LoadPage(ctx, in)
}

func TestHookSchedulerInitializationFailureSettlesMySQL(t *testing.T) {
	for _, tc := range []struct {
		name    string
		partial bool
		expired bool
		failure error
	}{
		{"missing-item", false, false, entity.ErrHookFrozenItemUnavailable},
		{"expired-missing-item", false, true, entity.ErrHookFrozenItemUnavailable},
		{"partial-content", true, false, entity.ErrHookFrozenContentUnavailable},
		{"expired-partial-storage", true, true, entity.ErrHookExecutionStorage},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, d, pub, target := executionChainFixture(t, entity.EvaluationModeSubmit)
			loader := &schedulerInitFailureLoader{PlanPageLoader: d.Loader}
			d.Loader = loader
			runtime := retryExecutionAssemble(t, d)
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if tc.partial {
				init := runtime.Scheduler.hookBoundInitializer
				in := entity.HookExecutionInitializationReadInput{Key: f.key, ExecutionScope: "local", Limit: 1}
				page, err := d.Repositories.Initialization.ReadExecutionInitializationPage(ctx, in)
				require.NoError(t, err)
				manifests, err := init.loadExecutionManifests(ctx, in, page)
				require.NoError(t, err)
				_, _, err = init.writeExecutionManifests(ctx, in, page, manifests)
				require.NoError(t, err)
				var count int64
				require.NoError(t, f.sql.Model(&model.ExptItemResultRunLog{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).Count(&count).Error)
				require.Equal(t, int64(1), count)
			}
			loader.err = tc.failure
			created := time.Now().Unix()
			if tc.expired {
				created = time.Now().Add(-time.Hour).Unix()
			}
			event := &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ExptRunMode: entity.EvaluationModeSubmit, CreatedAt: created, Session: &entity.Session{UserID: "original-user"}}
			require.NoError(t, runtime.Scheduler.Schedule(ctx, event))
			run := finalizationRead(t, f)
			require.Equal(t, entity.HookFinalizeCommitted, run.State.Finalize, "initialization failure must settle, not successfully publish another wait")
			require.Equal(t, entity.ExptStatus_SystemTerminated, run.State.Intent.Status)
			require.True(t, run.State.After.Activated)
			require.NotEmpty(t, gptr.Indirect(run.DisplayMessage))
			var logs []model.ExptItemResultRunLog
			require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).Find(&logs).Error)
			for _, log := range logs {
				require.Equal(t, int32(entity.ItemRunState_Terminal), log.Status)
				require.Equal(t, int32(entity.ExptItemResultStateResulted), gptr.Indirect(log.ResultState))
			}
			require.NoError(t, runtime.Scheduler.Schedule(ctx, event))
			require.Equal(t, run, finalizationRead(t, f))
			require.Empty(t, pub.published, "settled original event must not requeue")
			require.Empty(t, pub.items)
			require.Zero(t, target.calls)
			require.Equal(t, created, event.CreatedAt)
		})
	}
}

func TestHookSchedulerInitializationCleanupRetryIsolatesSuccessorMySQL(t *testing.T) {
	f, d, pub, _ := executionChainFixture(t, entity.EvaluationModeSubmit)
	loader := &schedulerInitFailureLoader{PlanPageLoader: d.Loader}
	d.Loader = loader
	runtime := retryExecutionAssemble(t, d)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	init := runtime.Scheduler.hookBoundInitializer
	in := entity.HookExecutionInitializationReadInput{Key: f.key, ExecutionScope: "local", Limit: 1}
	page, err := d.Repositories.Initialization.ReadExecutionInitializationPage(ctx, in)
	require.NoError(t, err)
	manifests, err := init.loadExecutionManifests(ctx, in, page)
	require.NoError(t, err)
	_, _, err = init.writeExecutionManifests(ctx, in, page, manifests)
	require.NoError(t, err)
	loader.err = entity.ErrHookFrozenItemUnavailable
	require.NoError(t, f.sql.Callback().Update().Before("gorm:update").Register("scheduler-init-cleanup-failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "expt_item_result_run_log" {
			tx.AddError(errors.New("cleanup storage unavailable"))
		}
	}))
	t.Cleanup(func() { _ = f.sql.Callback().Update().Remove("scheduler-init-cleanup-failure") })
	event := &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ExptRunMode: entity.EvaluationModeSubmit, CreatedAt: time.Now().Add(-time.Hour).Unix(), Session: &entity.Session{UserID: "original-user"}}
	require.ErrorIs(t, runtime.Scheduler.Schedule(ctx, event), schedulerHookRetryError{})
	pending := finalizationRead(t, f)
	require.Equal(t, entity.HookFinalizePending, pending.State.Finalize)
	require.False(t, pending.State.After.Activated)
	require.NoError(t, f.sql.Callback().Update().Remove("scheduler-init-cleanup-failure"))
	next := f.key
	next.RunID = finalizationTestIDs.Add(1)
	initial, err := f.manager.hooks.Initialization.ReadRunInitialization(ctx, next)
	require.NoError(t, err)
	created, err := f.repo.CreateRunWithHooks(ctx, entity.HookCreateRunInput{Key: next, ExpectedLatestRunID: f.key.RunID, ExpectedConfigRevision: initial.ConfigRevision,
		RunLog:   &entity.ExptRunLog{ID: next.RunID, SpaceID: f.space, ExptID: f.expt, ExptRunID: next.RunID, Mode: int32(entity.EvaluationModeSubmit), Status: int64(entity.ExptStatus_Processing), CreatedBy: "successor"},
		Snapshot: entity.HookProtectedSnapshot{Cipher: []byte{2}, KeyID: "key", Hash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", ExecutionScope: "local"},
		After:    &entity.HookOperationSeed{ID: finalizationTestIDs.Add(1), OperationID: fmt.Sprint(next.RunID), IdempotencyKey: fmt.Sprint(next.RunID)}})
	require.NoError(t, err)
	var parent model.Experiment
	require.NoError(t, f.sql.First(&parent, f.expt).Error)
	require.NoError(t, runtime.Scheduler.Schedule(ctx, event), "closed gate must resume durable pending cleanup")
	finished := finalizationRead(t, f)
	require.Equal(t, entity.HookFinalizeCommitted, finished.State.Finalize)
	require.True(t, finished.State.After.Activated)
	require.Equal(t, pending.DisplayMessage, finished.DisplayMessage)
	unchanged, err := f.repo.GetRun(ctx, next)
	require.NoError(t, err)
	require.Equal(t, created.Run, unchanged)
	var after model.Experiment
	require.NoError(t, f.sql.First(&after, f.expt).Error)
	require.Equal(t, parent, after)
	require.NoError(t, runtime.Scheduler.Schedule(ctx, event))
	require.Empty(t, pub.published)
	require.Empty(t, pub.items)
}

func TestHookSchedulerInitializationTransientResumesMySQL(t *testing.T) {
	for _, failure := range []error{entity.ErrHookStoreConflict, entity.ErrHookExecutionStorage} {
		t.Run(failure.Error(), func(t *testing.T) {
			f, d, pub, _ := executionChainFixture(t, entity.EvaluationModeSubmit)
			loader := &schedulerInitFailureLoader{PlanPageLoader: d.Loader, err: failure}
			d.Loader = loader
			runtime := retryExecutionAssemble(t, d)
			event := &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ExptRunMode: entity.EvaluationModeSubmit, CreatedAt: time.Now().Unix(), Session: &entity.Session{UserID: "original-user"}}
			require.NoError(t, runtime.Scheduler.Schedule(context.Background(), event))
			require.Equal(t, entity.HookFinalizeNone, finalizationRead(t, f).State.Finalize)
			require.Len(t, pub.published, 1)
			require.Equal(t, event.CreatedAt, pub.published[0].CreatedAt)
			loader.err = nil
			require.NoError(t, runtime.Scheduler.Schedule(context.Background(), pub.published[0]))
			require.Len(t, pub.items, 2)
		})
	}
}
