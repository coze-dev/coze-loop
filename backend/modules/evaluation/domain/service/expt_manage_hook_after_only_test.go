// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/lock"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	hookinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/hook"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type afterOnlyFixture struct {
	*finalizationManagerFixture
	preparer       hook.PlanPreparer
	initializer    hook.ExecutionInitializer
	initialization repo.IHookExecutionInitializationRepo
	gate           repo.IHookGateRepo
	mode           entity.ExptRunMode
	selector       *preparerSelector
}

type afterOnlyExperimentReader struct {
	repo.IExperimentRepo
	sql *gorm.DB
}

func (r afterOnlyExperimentReader) GetByID(ctx context.Context, id, space int64) (*entity.Experiment, error) {
	var row model.Experiment
	if err := r.sql.WithContext(ctx).Where("id=? AND space_id=?", id, space).First(&row).Error; err != nil {
		return nil, err
	}
	return convert.NewExptConverter().PO2DO(&row, nil)
}

// Only the external dataset boundary is substituted; creation, codec, plan,
// initialization, admission and finalization use their production repositories.
func newAfterOnlyFixture(t *testing.T, mode entity.ExptRunMode, boundary ...int32) *afterOnlyFixture {
	t.Helper()
	f := newFinalizationManagerFixture(t, "empty")
	ctx := context.Background()
	ids := &preparerIDs{next: finalizationTestIDs.Add(1000) - 1000}
	codec := hookinfra.NewStorageCodec(new(managerProtector))
	owner := hook.ConfigOwner{WorkspaceID: f.space, ObjectID: f.expt, Kind: hook.ConfigOwnerExperiment, ExecutionScope: "local"}
	raw, err := codec.EncodeConfig(ctx, "key", owner, managerEnabledConfig(false, true))
	require.NoError(t, err)
	exptType, source := int32(entity.ExptType_Offline), int32(entity.ExptEvalSetSourceType_SingleSet)
	if len(boundary) == 2 {
		exptType, source = boundary[0], boundary[1]
	}
	require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumns(map[string]any{"lifecycle_hook_conf": raw, "status": int32(entity.ExptStatus_Pending), "eval_set_id": 71, "eval_set_version_id": 72, "trial_run_item_count": 1, "expt_type": exptType, "eval_set_source_type": source}).Error)
	if source == int32(entity.ExptEvalSetSourceType_MultiSetConfig) {
		config, err := json.Marshal(&entity.EvaluationConfiguration{EvalSetConfigs: []*entity.EvalSetConfig{{EvalSetID: 71, EvalSetVersionID: 72}}})
		require.NoError(t, err)
		require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumn("eval_conf", config).Error)
	}
	versions := &loaderVersions{value: &entity.EvaluationSetVersion{ID: 72, SpaceID: f.space, EvaluationSetID: 71, EvaluationSetSchema: &entity.EvaluationSetSchema{ID: 73, SpaceID: f.space, EvaluationSetID: 71}}}
	f.manager.exptRepo = afterOnlyExperimentReader{sql: f.sql}
	_, err = f.manager.exptRepo.GetByID(ctx, f.expt, f.space)
	require.NoError(t, err)
	f.manager.idgenerator = ids
	f.manager.evaluationSetVersionService = versions
	identity, err := hookinfra.NewIdentityProvider(nil, 0)
	require.NoError(t, err)
	deps := ExptManagerHookDependencies{Initialization: exptinfra.NewHookRunInitializationRepo(f.p), Runs: f.repo, Configs: exptinfra.NewHookConfigRepo(f.p, codec), Codec: codec, Identity: identity, Runtime: &managerHookRuntime{admission: true}, Wake: new(managerHookWake), ExecutionScope: "local", SnapshotKeyID: "key"}
	m, err := NewExptManagerWithHooks(f.manager, deps)
	require.NoError(t, err)
	f.manager = m.(*ExptMangerImpl)
	f.manager.finalization.NewItemLocker = func() lock.ILocker { return lock.NewRedisLocker(f.redis) }
	initial, err := deps.Initialization.ReadRunInitialization(ctx, f.key)
	require.NoError(t, err)
	attempted := false
	selection := "[81,82]"
	require.NoError(t, f.manager.initializeHookRun(ctx, &entity.ExptRunLog{ID: f.key.RunID, ExptRunID: f.key.RunID, SpaceID: f.space, ExptID: f.expt, Mode: int32(mode), Status: int64(entity.ExptStatus_Pending), CreatedBy: "user"}, initial, &attempted, &selection))
	require.True(t, attempted)
	plans := exptinfra.NewHookPlanRepo(f.p)
	selector := &preparerSelector{selectFn: func(_ context.Context, in entity.HookSelectionInput) (entity.HookSelectionPage, error) {
		require.Equal(t, mode, in.Mode)
		if mode == entity.EvaluationModeTrialRun {
			require.True(t, in.HasExplicitItemIDs)
			require.Equal(t, []int64{81, 82}, in.ItemIDs)
		}
		return entity.HookSelectionPage{Items: []entity.HookPlanItem{{SourceSpaceID: f.space, EvalSetID: 71, EvalSetVersionID: 72, ItemID: 81}, {SourceSpaceID: f.space, EvalSetID: 71, EvalSetVersionID: 72, ItemID: 82}}, NextCursor: "done", Done: true}, nil
	}}
	preparer, err := NewHookPlanPreparer(HookPlanPreparerDependencies{Runs: f.repo, Plans: plans, Selector: selector, Codec: codec, Experiments: f.manager.exptRepo, Initialization: deps.Initialization, ResultReader: exptinfra.NewHookPlanResultReader(f.p), IDs: ids})
	require.NoError(t, err)
	items := &loaderItems{}
	for _, id := range []int64{81, 82} {
		items.batch = append(items.batch, &entity.EvaluationSetItem{ID: id + 100, ItemID: id, SpaceID: f.space, EvaluationSetID: 71, Turns: []*entity.Turn{{ID: 0, ItemID: id + 100, EvalSetID: 71}}})
	}
	loader, err := NewHookFrozenPlanLoader(HookFrozenPlanLoaderDependencies{Plans: plans, Items: items, Versions: versions, Sets: &loaderSets{}})
	require.NoError(t, err)
	initialization := exptinfra.NewHookExecutionInitializationRepo(f.p)
	initializer, err := NewHookFrozenExecutionInitializer(HookFrozenExecutionInitializerDependencies{Repository: initialization, Loader: loader, IDs: ids})
	require.NoError(t, err)
	return &afterOnlyFixture{finalizationManagerFixture: f, preparer: preparer, initializer: initializer, initialization: initialization, gate: exptinfra.NewHookExecutionGateRepo(f.p, func(context.Context) (string, error) { return "local", nil }), mode: mode, selector: selector}
}

func (f *afterOnlyFixture) prepare(t *testing.T) {
	t.Helper()
	for i := 0; i < 2; i++ {
		require.NoError(t, f.preparer.PreparePlan(context.Background(), hook.WorkerRunInput{ExecutionScope: "local", Candidate: entity.HookRunCandidate{Key: f.key}}))
	}
	require.True(t, finalizationRead(t, f.finalizationManagerFixture).PlanReady)
	require.Equal(t, 1, f.selector.calls)
}

func (f *afterOnlyFixture) initialize(t *testing.T) []entity.HookExecutionManifest {
	t.Helper()
	result, err := f.initializer.InitializeExecution(context.Background(), f.key, "local")
	require.NoError(t, err)
	require.True(t, result.Initialized)
	page, err := f.initialization.ReadExecutionInitializationPage(context.Background(), entity.HookPlanReadInput{Key: f.key, ExecutionScope: "local", Limit: 100})
	require.NoError(t, err)
	var manifests []entity.HookExecutionManifest
	for _, item := range page.Items {
		require.NotNil(t, item.Manifest)
		manifests = append(manifests, *item.Manifest)
	}
	return manifests
}

func (f *afterOnlyFixture) noBefore(t *testing.T) {
	t.Helper()
	run := finalizationRead(t, f.finalizationManagerFixture)
	require.Equal(t, entity.HookOperationDisabled, run.State.Before.Status)
	require.Len(t, run.Operations, 1)
	require.Equal(t, entity.HookPhaseAfter, run.Operations[0].Phase)
	var attempts int64
	require.NoError(t, f.sql.Model(&model.ExptLifecycleHookAttempt{}).Where("operation_id=?", run.State.After.ID).Count(&attempts).Error)
	require.Zero(t, attempts)
}

func TestHookAfterOnlySelectionAndReplayMySQL(t *testing.T) {
	f := newAfterOnlyFixture(t, entity.EvaluationModeTrialRun)
	ctx := context.Background()
	run := finalizationRead(t, f.finalizationManagerFixture)
	snapshot, err := f.manager.hooks.Codec.DecodeSnapshot(ctx, f.key, "local", run.Snapshot)
	require.NoError(t, err)
	require.NotNil(t, snapshot.Input().Selection)
	require.True(t, snapshot.Input().Selection.HasExplicitItemIDs)
	for _, tc := range []struct {
		raw string
		ok  bool
	}{{"[81,82]", true}, {"[82]", false}, {"[]", false}, {"", false}} {
		err := f.manager.LogRunWithPlanSeed(ctx, f.expt, f.key.RunID, f.mode, f.space, tc.raw, &entity.Session{UserID: "user"})
		if tc.ok {
			require.NoError(t, err)
		} else {
			require.Error(t, err)
		}
	}
	require.Equal(t, run, finalizationRead(t, f.finalizationManagerFixture))
	f.noBefore(t)
}

func TestHookAfterOnlyPreparerMySQL(t *testing.T) {
	f := newAfterOnlyFixture(t, entity.EvaluationModeSubmit)
	f.prepare(t)
	f.noBefore(t)
}

func TestHookAfterOnlyPlanAndInitializationGateMySQL(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeTrialRun} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			f := newAfterOnlyFixture(t, mode)
			ctx := context.Background()
			require.Equal(t, entity.HookGateReady, finalizationRead(t, f.finalizationManagerFixture).State.Gate)
			decision, err := f.gate.CanDispatch(ctx, f.key)
			require.NoError(t, err)
			require.Equal(t, entity.HookGateWaiting, decision.Gate, "logical before-ready must not bypass plan/initialization")
			f.prepare(t)
			decision, err = f.gate.CanDispatch(ctx, f.key)
			require.NoError(t, err)
			require.Equal(t, entity.HookGateWaiting, decision.Gate)
			run := finalizationRead(t, f.finalizationManagerFixture)
			_, err = f.repo.AdmitItem(ctx, entity.HookAdmitItemInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: run.Version}, ItemID: 81})
			require.ErrorIs(t, err, entity.ErrHookAdmissionDenied)
			manifests := f.initialize(t)
			require.Len(t, manifests, 2)
			decision, err = f.gate.CanDispatch(ctx, f.key)
			require.NoError(t, err)
			require.Equal(t, entity.HookGateReady, decision.Gate)
			var encoded []byte
			require.NoError(t, f.sql.Model(&model.ExptLifecycleRunItem{}).Select("execution_manifest").Where("id=?", manifests[0].Frozen.ID).Row().Scan(&encoded))
			var persisted entity.HookExecutionManifest
			require.NoError(t, json.Unmarshal(encoded, &persisted))
			require.Equal(t, manifests[0], persisted)
			f.noBefore(t)
		})
	}
}

func (f *afterOnlyFixture) partial(t *testing.T, all ...bool) {
	t.Helper()
	ctx := context.Background()
	page, err := f.initialization.ReadExecutionInitializationPage(ctx, entity.HookPlanReadInput{Key: f.key, ExecutionScope: "local", Limit: 1})
	require.NoError(t, err)
	m := entity.HookExecutionManifest{Version: 1, Key: f.key, Frozen: page.Items[0].Frozen, ItemResultID: finalizationTestIDs.Add(1), ItemRunLogID: finalizationTestIDs.Add(1), Turns: []entity.HookExecutionTurnManifest{{TurnID: 0, ResultID: finalizationTestIDs.Add(1)}}, TurnLogsInitialized: gptr.Of(false)}
	_, err = f.initialization.WriteExecutionInitializationPage(ctx, entity.HookExecutionInitializationWriteInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: page.RunVersion}, ExecutionScope: "local", PlanHash: page.Hash, Items: []entity.HookExecutionManifest{m}})
	require.NoError(t, err)
	if len(all) > 0 && all[0] {
		page, err = f.initialization.ReadExecutionInitializationPage(ctx, entity.HookPlanReadInput{Key: f.key, ExecutionScope: "local", StartOrdinal: 1, Limit: 1})
		require.NoError(t, err)
		m.Ordinal, m.Frozen = 1, page.Items[0].Frozen
		m.ItemResultID, m.ItemRunLogID, m.Turns[0].ResultID = finalizationTestIDs.Add(1), finalizationTestIDs.Add(1), finalizationTestIDs.Add(1)
		_, err = f.initialization.WriteExecutionInitializationPage(ctx, entity.HookExecutionInitializationWriteInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: page.RunVersion}, ExecutionScope: "local", PlanHash: page.Hash, StartOrdinal: 1, Items: []entity.HookExecutionManifest{m}})
		require.NoError(t, err)
	}
}

func TestHookAfterOnlyCancellationAndDeletionMySQL(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeSubmit, entity.EvaluationModeTrialRun} {
		for _, stage := range []string{"unprepared", "prepared", "partial", "materialized", "initialized", "active"} {
			for _, action := range []string{"user", "system", "delete"} {
				t.Run(fmt.Sprintf("%d/%s/%s", mode, stage, action), func(t *testing.T) {
					f := newAfterOnlyFixture(t, mode)
					ctx := context.Background()
					if stage != "unprepared" {
						f.prepare(t)
					}
					if stage == "partial" {
						f.partial(t)
					}
					if stage == "materialized" {
						f.partial(t, true)
					}
					if stage == "initialized" || stage == "active" {
						ms := f.initialize(t)
						if stage == "active" {
							_, err := f.deps.Repository.(repo.IHookSchedulerRepo).PersistHookDispatch(ctx, entity.HookSchedulerDispatchInput{Key: f.key, ExecutionScope: "local", ItemIDs: []int64{81}})
							require.NoError(t, err)
							itemCtx, item, pre := admitLazyTurn(t, f.finalizationManagerFixture, ms[0])
							require.NoError(t, pre.PreEval(itemCtx, item))
						}
					}
					before := finalizationRead(t, f.finalizationManagerFixture)
					if action == "delete" {
						m, err := NewExptManagerWithHookDeletion(f.manager, exptinfra.NewHookDeletionRepo(f.p))
						require.NoError(t, err)
						require.NoError(t, m.Delete(ctx, f.expt, f.space, nil))
						require.False(t, finalizationRead(t, f.finalizationManagerFixture).State.After.Activated)
						f.manager = m
						require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
					} else {
						status := entity.ExptStatus_Terminated
						if action == "system" {
							status = entity.ExptStatus_SystemTerminated
						}
						require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{Status: status}))
					}
					done := finalizationRead(t, f.finalizationManagerFixture)
					require.Equal(t, entity.HookFinalizeCommitted, done.State.Finalize)
					require.True(t, done.State.After.Activated)
					require.Equal(t, before.Snapshot, done.Snapshot)
					require.Equal(t, before.PlanHash, done.PlanHash)
					require.Equal(t, before.State.After.ID, done.State.After.ID)
					require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
					require.Equal(t, done, finalizationRead(t, f.finalizationManagerFixture))
					f.noBefore(t)
				})
			}
		}
	}
}

func TestHookAfterOnlyCorruptInitializationProofMySQL(t *testing.T) {
	f := newAfterOnlyFixture(t, entity.EvaluationModeSubmit)
	require.NoError(t, f.sql.Model(&model.ExptLifecycleRun{}).Where("space_id=? AND expt_run_id=?", f.space, f.key.RunID).UpdateColumn("execution_initialized", true).Error)
	decision, _ := f.gate.CanDispatch(context.Background(), f.key)
	require.Equal(t, entity.HookGateWaiting, decision.Gate, "an initialization flag cannot certify a missing frozen plan")
}

func TestHookAfterOnlyUnsupportedModesMySQL(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeAppend, entity.EvaluationModeRetryAll, entity.EvaluationModeFailRetry, entity.EvaluationModeRetryItems} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			f := newAfterOnlyFixture(t, mode)
			before := finalizationRead(t, f.finalizationManagerFixture)
			err := f.preparer.PreparePlan(context.Background(), hook.WorkerRunInput{ExecutionScope: "local", Candidate: entity.HookRunCandidate{Key: f.key}})
			if entity.HookBoundRetryMode(mode) {
				require.ErrorIs(t, err, entity.ErrHookStoreConflict)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, before, finalizationRead(t, f.finalizationManagerFixture))
			require.Zero(t, f.selector.calls)
			_, err = f.initializer.InitializeExecution(context.Background(), f.key, "local")
			require.ErrorIs(t, err, entity.ErrHookExecutionUnsupported)
		})
	}
	for _, boundary := range [][2]int32{{2, 1}} {
		t.Run(fmt.Sprint(boundary), func(t *testing.T) {
			f := newAfterOnlyFixture(t, entity.EvaluationModeSubmit, boundary[:]...)
			before := finalizationRead(t, f.finalizationManagerFixture)
			require.NoError(t, f.preparer.PreparePlan(context.Background(), hook.WorkerRunInput{ExecutionScope: "local", Candidate: entity.HookRunCandidate{Key: f.key}}))
			require.Equal(t, before, finalizationRead(t, f.finalizationManagerFixture))
			require.Zero(t, f.selector.calls)
			_, err := f.initializer.InitializeExecution(context.Background(), f.key, "local")
			require.ErrorIs(t, err, entity.ErrHookExecutionUnsupported)
		})
	}
}

func TestHookAfterOnlyMultiSetPlanKeepsLegacyInitializerClosedMySQL(t *testing.T) {
	f := newAfterOnlyFixture(t, entity.EvaluationModeSubmit, 1, 2)
	before := finalizationRead(t, f.finalizationManagerFixture)
	require.NoError(t, f.preparer.PreparePlan(context.Background(), hook.WorkerRunInput{ExecutionScope: "local", Candidate: entity.HookRunCandidate{Key: f.key}}))
	current := finalizationRead(t, f.finalizationManagerFixture)
	require.Equal(t, before.Snapshot, current.Snapshot)
	require.Equal(t, int64(2), current.PlanCount)
	require.False(t, current.ExecutionStarted)
	require.Equal(t, 1, f.selector.calls)
	_, err := f.initializer.InitializeExecution(context.Background(), f.key, "local")
	require.ErrorIs(t, err, entity.ErrHookExecutionUnsupported)
}

func TestHookAfterOnlySchedulerConsumerAndStartupMySQL(t *testing.T) {
	f := newAfterOnlyFixture(t, entity.EvaluationModeTrialRun)
	f.prepare(t)
	ms := f.initialize(t)
	ctx := context.Background()
	schedulerRepo := f.deps.Repository.(repo.IHookSchedulerRepo)
	scheduler := &ExptSchedulerImpl{hookGate: f.gate, hookRuns: f.repo, hookInitialization: f.initialization, hookScheduler: schedulerRepo, hookSchedulerScope: "local", ResultSvc: hookPersistenceFilters{}}
	initialized, stop, err := scheduler.resumeHookSchedulerInitialization(ctx, &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ExptRunMode: f.mode})
	require.NoError(t, err)
	require.True(t, initialized, "after-only startup must bypass legacy reinitialization")
	require.False(t, stop)
	changed, err := schedulerRepo.PersistHookDispatch(ctx, entity.HookSchedulerDispatchInput{Key: f.key, ExecutionScope: "local", ItemIDs: []int64{81}})
	require.NoError(t, err)
	require.Equal(t, []int64{81}, changed)
	ctx, item, pre := admitLazyTurn(t, f.finalizationManagerFixture, ms[0])
	require.NoError(t, pre.PreEval(ctx, item))
	var ledger model.ExptLifecycleRunItem
	require.NoError(t, f.sql.First(&ledger, ms[0].Frozen.ID).Error)
	var manifest entity.HookExecutionManifest
	require.NoError(t, json.Unmarshal(*ledger.ExecutionManifest, &manifest))
	require.True(t, gptr.Indirect(manifest.TurnLogsInitialized))
	control := exptinfra.NewHookTurnProgressRepo(f.p, func(context.Context) (string, error) { return "local", nil }).(repo.IHookConsumerControlRepo)
	result, err := control.ApplyHookConsumerControl(ctx, entity.HookConsumerControlInput{Key: f.key, ItemID: 81, Action: entity.HookConsumerYield, RetryTimes: 1})
	require.NoError(t, err)
	require.True(t, result.Handled)
	require.True(t, result.Changed)
	f.noBefore(t)
}
