// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestHookBoundInitializationParallelMySQL(t *testing.T) {
	f := newBoundInitFixture(t, 101, entity.EvaluationModeTrialRun, true, true)
	a := boundInitializationRepo(t, f)
	b, err := exptinfra.NewBoundHookExecutionInitializationRepo(f.p, f.binding)
	require.NoError(t, err)
	// Keep external fixture counters separate; both real repositories share the same Run.
	second := *f
	deps := f.loader.(*hookFrozenPlanLoader).deps
	deps.Versions = &loaderVersions{value: deps.Versions.(*loaderVersions).value}
	deps.Sets = &loaderSets{value: deps.Sets.(*loaderSets).value}
	second.loader, err = NewHookFrozenPlanLoader(deps)
	require.NoError(t, err)
	firstInit, err := NewBoundHookFrozenExecutionInitializer(HookFrozenExecutionInitializerDependencies{Repository: a, Loader: f.loader, IDs: activeReferenceIDs{}}, f.binding)
	require.NoError(t, err)
	secondInit, err := NewBoundHookFrozenExecutionInitializer(HookFrozenExecutionInitializerDependencies{Repository: b, Loader: second.loader, IDs: activeReferenceIDs{}}, f.binding)
	require.NoError(t, err)
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for _, fn := range []func(context.Context, entity.HookRunKey, string) (entity.HookExecutionInitializationCompletion, error){firstInit.InitializeExecution, secondInit.InitializeExecution} {
		wg.Add(1)
		go func(call func(context.Context, entity.HookRunKey, string) (entity.HookExecutionInitializationCompletion, error)) {
			defer wg.Done()
			<-start
			_, err := call(context.Background(), f.key, "local")
			errs <- err
		}(fn)
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.EqualValues(t, 101, boundCount(t, f, &model.ExptItemRef{}))
	_, err = f.initialize(t, a)
	require.NoError(t, err)
}

func TestHookBoundInitializationPreexistingOccupantsMySQL(t *testing.T) {
	for _, kind := range []string{"matching-item", "unrelated-item", "soft-deleted", "old-result"} {
		t.Run(kind, func(t *testing.T) {
			f := newBoundInitFixture(t, 2, entity.EvaluationModeSubmit, false, true)
			r := boundInitializationRepo(t, f)
			if kind == "old-result" {
				require.NoError(t, f.sql.Create(&model.ExptItemResult{ID: finalizationTestIDs.Add(1), SpaceID: f.space, ExptID: f.expt, ExptRunID: finalizationTestIDs.Add(1), ItemID: f.items[0].ItemID}).Error)
			} else {
				ref := model.ExptItemRef{ID: finalizationTestIDs.Add(1), SpaceID: f.space, ExptID: f.expt, ItemID: f.items[0].ItemID, EvalSetID: 71, ItemConfig: gptr.Of([]byte(`{}`))}
				if kind == "unrelated-item" {
					ref.ItemID = finalizationTestIDs.Add(1)
				}
				require.NoError(t, f.sql.Create(&ref).Error)
				if kind == "soft-deleted" {
					require.NoError(t, f.sql.Delete(&ref).Error)
				}
			}
			before := finalizationRead(t, f.finalizationManagerFixture)
			_, err := f.initialize(t, r)
			require.Error(t, err)
			require.Equal(t, before, finalizationRead(t, f.finalizationManagerFixture))
			require.Zero(t, boundCount(t, f, &model.ExptTurnResult{}))
		})
	}
}

func TestHookBoundInitializationForeignRefIDCannotBeTakenOverMySQL(t *testing.T) {
	f := newBoundInitFixture(t, 1, entity.EvaluationModeSubmit, false, true)
	r := boundInitializationRepo(t, f)
	ctx := context.Background()
	read := entity.HookPlanReadInput{Key: f.key, ExecutionScope: "local", Limit: 1}
	page, err := r.ReadExecutionInitializationPage(ctx, read)
	require.NoError(t, err)
	core, err := NewBoundHookFrozenExecutionInitializer(HookFrozenExecutionInitializerDependencies{Repository: r, Loader: f.loader, IDs: activeReferenceIDs{}}, f.binding)
	require.NoError(t, err)
	manifests, err := core.(*hookFrozenExecutionInitializer).loadExecutionManifests(ctx, read, page)
	require.NoError(t, err)
	foreign := model.ExptItemRef{ID: finalizationTestIDs.Add(1), SpaceID: f.space + 1, ExptID: f.expt + 1, ItemID: finalizationTestIDs.Add(1), EvalSetID: 999, ItemConfig: gptr.Of([]byte(`{"foreign":"retained"}`))}
	require.NoError(t, f.sql.Create(&foreign).Error)
	t.Cleanup(func() { require.NoError(t, f.sql.Unscoped().Delete(&model.ExptItemRef{}, foreign.ID).Error) })
	manifests[0].ItemRef.ID = foreign.ID
	_, err = r.WriteExecutionInitializationPage(ctx, entity.HookExecutionInitializationWriteInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: page.RunVersion}, ExecutionScope: "local", PlanHash: page.Hash, Items: manifests})
	require.Error(t, err)
	var after model.ExptItemRef
	require.NoError(t, f.sql.First(&after, foreign.ID).Error)
	require.Equal(t, foreign.SpaceID, after.SpaceID)
	require.Equal(t, foreign.ExptID, after.ExptID)
	require.Equal(t, foreign.ItemID, after.ItemID)
	require.Equal(t, foreign.ItemConfig, after.ItemConfig)
	require.Zero(t, boundCount(t, f, &model.ExptItemRef{}))
	require.Zero(t, boundCount(t, f, &model.ExptItemResult{}))
	_, err = f.initialize(t, r)
	require.NoError(t, err)
}

func TestHookBoundInitializationReferenceCountUsesCurrentReadMySQL(t *testing.T) {
	f := newBoundInitFixture(t, 0, entity.EvaluationModeSubmit, false, true)
	r := boundInitializationRepo(t, f)
	run := finalizationRead(t, f.finalizationManagerFixture)
	inserted := false
	require.NoError(t, f.sql.Callback().Query().Before("gorm:query").Register("bound-ref-count-snapshot", func(tx *gorm.DB) {
		if inserted || tx.Statement.Table != "expt_item_ref" {
			return
		}
		inserted = true
		var count int64
		// Establish the old RR snapshot, then commit an ordinary independent ItemRef insert.
		require.NoError(t, tx.Session(&gorm.Session{NewDB: true}).Raw("SELECT COUNT(*) FROM expt_item_ref WHERE space_id=? AND expt_id=?", f.space, f.expt).Scan(&count).Error)
		require.Zero(t, count)
		require.NoError(t, f.sql.Create(&model.ExptItemRef{ID: finalizationTestIDs.Add(1), SpaceID: f.space, ExptID: f.expt, ItemID: finalizationTestIDs.Add(1), EvalSetID: 71, ItemConfig: gptr.Of([]byte(`{}`))}).Error)
	}))
	_, err := r.CompleteExecutionInitialization(context.Background(), entity.HookExecutionInitializationCompleteInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: run.Version}, ExecutionScope: "local", PlanHash: run.PlanHash})
	require.NoError(t, f.sql.Callback().Query().Remove("bound-ref-count-snapshot"))
	require.True(t, inserted)
	require.Error(t, err, "a stale count must not certify an empty plan while a committed foreign ref exists")
	var life model.ExptLifecycleRun
	require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).First(&life).Error)
	require.False(t, life.ExecutionInitialized)
}

func TestHookBoundInitializationBindingMismatchMySQL(t *testing.T) {
	f := newBoundInitFixture(t, 2, entity.EvaluationModeSubmit, false, true)
	r := boundInitializationRepo(t, f)
	ctx := context.Background()
	for _, key := range []entity.HookRunKey{{WorkspaceID: f.space + 1, ExperimentID: f.expt, RunID: f.key.RunID}, {WorkspaceID: f.space, ExperimentID: f.expt, RunID: f.key.RunID + 1}} {
		_, err := r.ReadExecutionInitializationPage(ctx, entity.HookPlanReadInput{Key: key, ExecutionScope: "local", Limit: 1})
		require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	}
	_, err := r.ReadExecutionInitializationPage(ctx, entity.HookPlanReadInput{Key: f.key, ExecutionScope: "wrong", Limit: 1})
	require.ErrorIs(t, err, entity.ErrHookStoreConflict)
	source := f.binding.Input()
	source.Execution.Sets[0].ItemConfig.EvaluatorConfs[0].Alias = "mutated returned copy"
	_, err = f.initialize(t, r)
	require.NoError(t, err)
	var ref model.ExptItemRef
	require.NoError(t, f.sql.Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, f.items[0].ItemID).First(&ref).Error)
	require.Contains(t, string(*ref.ItemConfig), `"alias":"first"`)
	stored := finalizationRead(t, f.finalizationManagerFixture)
	decoded, err := f.manager.hooks.Codec.DecodeSnapshot(ctx, f.key, "local", stored.Snapshot)
	require.NoError(t, err)
	stored.CreatedBy = "wrong-user"
	_, err = entity.NewHookExecutionInitializationBinding(stored, decoded)
	require.Error(t, err)
	input := decoded.Input()
	input.Execution = nil
	old, err := entity.NewHookRunSnapshot(input)
	require.NoError(t, err)
	stored.CreatedBy = "original-user"
	_, err = entity.NewHookExecutionInitializationBinding(stored, old)
	require.ErrorIs(t, err, entity.ErrHookExecutionUnsupported)
}

func TestHookBoundInitializationFrozenPrecisionMySQL(t *testing.T) {
	f := newBoundInitFixture(t, 2, entity.EvaluationModeSubmit, false, true)
	ctx := context.Background()
	run := finalizationRead(t, f.finalizationManagerFixture)
	snapshot, err := f.manager.hooks.Codec.DecodeSnapshot(ctx, f.key, "local", run.Snapshot)
	require.NoError(t, err)
	input := snapshot.Input()
	input.Execution.Sets[0].ItemConfig.EvalTargetConf.RunConf.FixedQueryList = []*entity.FixedQuery{{Evaluators: map[string]interface{}{"opaque_id": json.Number("9007199254740993")}}}
	updated, err := entity.NewHookRunSnapshot(input)
	require.NoError(t, err)
	protected, err := f.manager.hooks.Codec.EncodeSnapshot(ctx, "key", updated)
	require.NoError(t, err)
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumns(map[string]any{"snapshot_cipher": protected.Cipher, "snapshot_hash": protected.Hash}).Error)
	r := boundInitializationRepo(t, f)
	_, err = f.initialize(t, r)
	require.NoError(t, err)
	var ref model.ExptItemRef
	require.NoError(t, f.sql.Where("space_id=? AND expt_id=? AND item_id=?", f.space, f.expt, f.items[0].ItemID).First(&ref).Error)
	require.Contains(t, string(*ref.ItemConfig), `"opaque_id":9007199254740993`)
	f.dataset.fail = true
	_, err = f.initialize(t, r)
	require.NoError(t, err)
}

type boundCancelBeforeComplete struct {
	repo.IHookExecutionInitializationRepo
	f *boundInitFixture
}

func (r boundCancelBeforeComplete) CompleteExecutionInitialization(ctx context.Context, in entity.HookExecutionInitializationCompleteInput) (entity.HookExecutionInitializationCompletion, error) {
	run, err := r.f.repo.GetRun(ctx, r.f.key)
	if err != nil {
		return entity.HookExecutionInitializationCompletion{}, err
	}
	_, err = r.f.repo.BeginFinalize(ctx, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: r.f.key, ExpectedVersion: run.Version}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated}})
	if err != nil {
		return entity.HookExecutionInitializationCompletion{}, err
	}
	return r.IHookExecutionInitializationRepo.CompleteExecutionInitialization(ctx, in)
}

func TestHookBoundInitializationCancelBetweenStagesMySQL(t *testing.T) {
	for _, stage := range []string{"partial-page", "before-completion", "after-completion"} {
		t.Run(stage, func(t *testing.T) {
			f := newBoundInitFixture(t, 101, entity.EvaluationModeTrialRun, false, true)
			r := boundInitializationRepo(t, f)
			if stage == "before-completion" {
				_, err := f.initialize(t, boundCancelBeforeComplete{r, f})
				require.ErrorIs(t, err, entity.ErrHookAdmissionDenied)
			} else {
				if stage == "partial-page" {
					_, err := f.initialize(t, &boundFailPageRepo{r, true})
					require.Error(t, err)
				} else {
					_, err := f.initialize(t, r)
					require.NoError(t, err)
				}
				run := finalizationRead(t, f.finalizationManagerFixture)
				_, err := f.repo.BeginFinalize(context.Background(), entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: run.Version}, Intent: entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated}})
				require.NoError(t, err)
			}
			n := boundCount(t, f, &model.ExptItemRef{})
			_, err := f.initialize(t, r)
			require.Error(t, err)
			require.Equal(t, n, boundCount(t, f, &model.ExptItemRef{}))
			require.False(t, finalizationRead(t, f.finalizationManagerFixture).State.After.Activated)
			var life model.ExptLifecycleRun
			require.NoError(t, f.sql.Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).First(&life).Error)
			require.Equal(t, stage == "after-completion", life.ExecutionInitialized, fmt.Sprint(stage))
		})
	}
}
