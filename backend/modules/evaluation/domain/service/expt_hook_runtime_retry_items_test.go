// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	em "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	exptmysql "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	tm "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

type retryItemsTestLoader struct{ boundConsumerLoaderMutation }

func (l retryItemsTestLoader) LoadCandidates(ctx context.Context, in entity.HookPlanReadInput, page *entity.HookPlanReadPage) (*entity.HookLoadedPlanPage, error) {
	loader, ok := l.PlanPageLoader.(interface {
		LoadCandidates(context.Context, entity.HookPlanReadInput, *entity.HookPlanReadPage) (*entity.HookLoadedPlanPage, error)
	})
	if !ok {
		return nil, entity.ErrHookExecutionUnsupported
	}
	p, err := loader.LoadCandidates(ctx, in, page)
	if err == nil {
		l.mutate(p)
	}
	return p, err
}

type RetryItemsAppTestEnvironment struct {
	DB              db.Provider
	Redis           redis.Cmdable
	Codec           hook.StorageCodec
	Identity        hook.IdentityProvider
	Manager         IExptManager
	Scheduler       ExptSchedulerEvent
	Consumer        ExptItemEvalEvent
	IDs             idgen.IIDGenerator
	Config          component.IConfiger
	Items           EvaluationSetItemService
	Versions        EvaluationSetVersionService
	Sets            IEvaluationSetService
	Refs            repo.IExptItemRefRepo
	Turns           repo.IExptTurnResultRepo
	Results         repo.IExptItemResultRepo
	Experiments     repo.IExperimentRepo
	SpaceID, ExptID int64
}
type RetryItemsAppTestDriver struct {
	Start     func(context.Context, []int64) (int64, error)
	Scheduler ExptSchedulerEvent
	Consumer  ExptItemEvalEvent
}
type retryItemsAppConfig struct{ *executionChainConfig }

func (retryItemsAppConfig) GetRetryYieldEnabled(context.Context, int64) bool { return false }

type retryItemsAppReader struct{ afterOnlyExperimentReader }

func (r retryItemsAppReader) MGetByID(ctx context.Context, ids []int64, space int64) ([]*entity.Experiment, error) {
	var out []*entity.Experiment
	for _, id := range ids {
		e, err := r.GetByID(ctx, id, space)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

type retryItemsIndexTurns struct{ repo.IExptTurnResultRepo }

func (retryItemsIndexTurns) ListTurnResultByItemIDs(context.Context, int64, int64, []int64, entity.Page, bool) ([]*entity.ExptTurnResult, int64, error) {
	return nil, 0, nil
}

type retryItemsAppDataset struct{ boundConsumerDataset }

func (d *retryItemsAppDataset) BatchGetEvaluationSetItems(ctx context.Context, in *entity.BatchGetEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, error) {
	items, err := d.boundConsumerDataset.BatchGetEvaluationSetItems(ctx, in)
	for _, it := range items {
		it.BaseInfo = &entity.BaseInfo{}
	}
	return items, err
}
func (d *retryItemsAppDataset) GetEvaluationSetItemVersion(ctx context.Context, space, set, item int64, version *int64, name *string) (*entity.EvaluationSetItemVersion, error) {
	it, err := d.boundConsumerDataset.GetEvaluationSetItemVersion(ctx, space, set, item, version, name)
	if err == nil {
		it.BaseInfo = &entity.BaseInfo{}
	}
	return it, err
}

func TestHookRetryItemsRuntimePrefixTailAfterMySQL(t *testing.T) {
	RetryItemsAppChainForTest(t, nil)
}

// Exported only in the test binary so the external test can drive the real application without an import cycle.
func RetryItemsAppChainForTest(t *testing.T, build func(RetryItemsAppTestEnvironment) RetryItemsAppTestDriver) {
	runRetryItemsChain(t, build, "")
}

func TestHookRetryItemsSameCountHashDriftMySQL(t *testing.T) {
	runRetryItemsChain(t, nil, "hash_drift")
}
func TestHookRetryItemsTailDuringContextReadMySQL(t *testing.T) {
	runRetryItemsChain(t, nil, "context_tail")
}
func TestHookRetryItemsCancelUnmaterializedTailMySQL(t *testing.T) {
	runRetryItemsChain(t, nil, "cancel_tail")
}
func TestHookRetryItemsBeforePendingTailMySQL(t *testing.T) {
	runRetryItemsChain(t, nil, "before_wait")
}
func TestHookRetryItemsTransientLoaderRecoversMySQL(t *testing.T) {
	runRetryItemsChain(t, nil, "source_io")
}
func TestHookRetryItemsLoaderExhaustionFinalizesMySQL(t *testing.T) {
	runRetryItemsChain(t, nil, "source_exhausted")
}

func runRetryItemsChain(t *testing.T, build func(RetryItemsAppTestEnvironment) RetryItemsAppTestDriver, scenario string) {
	var options []string
	if scenario == "before_wait" {
		options = []string{"before"}
	}
	f, d, pub, target := executionChainFixture(t, entity.EvaluationModeSubmit, options...)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	source := f.key
	runtime := retryExecutionAssemble(t, d)
	event := &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: source.RunID, ExptRunMode: entity.EvaluationModeSubmit, ExptType: entity.ExptType_Offline, CreatedAt: time.Now().Unix(), Session: &entity.Session{UserID: "original-user"}}
	if scenario == "before_wait" {
		completeExecutionChainBefore(t, f)
	}
	require.NoError(t, runtime.Scheduler.Schedule(ctx, event))
	for _, item := range pub.items {
		require.NoError(t, runtime.Consumer.Eval(ctx, item))
	}
	require.NoError(t, runtime.Scheduler.Schedule(ctx, event))
	require.Equal(t, 2, target.calls)
	f.key.RunID = finalizationTestIDs.Add(1)
	f.manager.configer = d.Scheduler.(*ExptSchedulerImpl).Configer
	f.manager.mtr = retryExecutionMetric{f.manager.mtr}
	f.manager.itemResultRepo = d.Consumer.(*ExptItemEventEvalServiceImpl).exptItemResultRepo
	t.Cleanup(func() {
		ids := []int64{source.RunID, f.key.RunID}
		require.NoError(t, f.sql.Unscoped().Where("experiment_run_id IN ?", ids).Delete(&tm.TargetRecord{}).Error)
		require.NoError(t, f.sql.Unscoped().Where("experiment_run_id IN ?", ids).Delete(&em.EvaluatorRecord{}).Error)
	})
	consumer := d.Consumer.(*ExptItemEventEvalServiceImpl)
	refs := exptinfra.NewExptItemRefRepo(exptmysql.NewExptItemRefDAO(f.p))
	d.Scheduler.(*ExptSchedulerImpl).schedulerModeFactory.(*DefaultSchedulerModeFactory).exptItemRefRepo = refs
	f.manager.itemRefRepo = refs
	var driver RetryItemsAppTestDriver
	if build != nil {
		d.Scheduler.(*ExptSchedulerImpl).ResultSvc.(*ExptResultServiceImpl).ExptTurnResultRepo = retryItemsIndexTurns{consumer.exptTurnResultRepo}
		f.manager.configer = retryItemsAppConfig{d.Scheduler.(*ExptSchedulerImpl).Configer.(*executionChainConfig)}
		f.manager.lwt = platestwrite.NewLatestWriteTracker(f.redis)
		f.manager.exptRepo = retryItemsAppReader{afterOnlyExperimentReader{sql: f.sql}}
		pub.ExptEventPublisher = f.manager.publisher
		f.manager.publisher = pub
		loader := d.Loader.(boundConsumerLoaderMutation).PlanPageLoader.(*hookFrozenPlanLoader)
		driver = build(RetryItemsAppTestEnvironment{DB: f.p, Redis: f.redis, Codec: f.manager.hooks.Codec, Identity: f.manager.hooks.Identity, Manager: f.manager, Scheduler: d.Scheduler, Consumer: d.Consumer, IDs: f.manager.idgenerator, Config: f.manager.configer, Items: new(retryItemsAppDataset), Versions: loader.deps.Versions, Sets: loader.deps.Sets, Refs: refs, Turns: consumer.exptTurnResultRepo, Results: consumer.exptItemResultRepo, Experiments: f.manager.exptRepo, SpaceID: f.space, ExptID: f.expt})
		rid, err := driver.Start(ctx, []int64{801})
		require.NoError(t, err)
		f.key.RunID = rid
	} else {
		require.NoError(t, f.manager.LogRun(ctx, f.expt, f.key.RunID, entity.EvaluationModeRetryItems, f.space, []int64{801}, &entity.Session{UserID: "original-user"}))
	}
	selector := NewHookPlanSelector(new(boundConsumerDataset), refs, consumer.exptTurnResultRepo, consumer.exptItemResultRepo)
	preparer, err := NewHookPlanPreparer(HookPlanPreparerDependencies{Runs: f.repo, Plans: exptinfra.NewHookPlanRepo(f.p), Selector: selector, Codec: f.manager.hooks.Codec, Experiments: f.manager.exptRepo, Initialization: f.manager.hooks.Initialization, ResultReader: exptinfra.NewHookPlanResultReader(f.p), IDs: activeReferenceIDs{}})
	require.NoError(t, err)
	for i := 0; i < 3 && !finalizationRead(t, f).PlanReady; i++ {
		require.NoError(t, preparer.PreparePlan(ctx, hook.WorkerRunInput{ExecutionScope: "local", Candidate: entity.HookRunCandidate{Key: f.key}}))
	}
	require.True(t, finalizationRead(t, f).PlanReady)
	d.Binding, err = LoadHookExecutionInitializationBinding(ctx, f.repo, f.manager.hooks.Codec, f.key, "local")
	require.NoError(t, err)
	d.Repositories, err = exptinfra.NewBoundHookRuntimeRepositories(f.p, d.Binding)
	require.NoError(t, err)
	d.Loader = retryItemsTestLoader{d.Loader.(boundConsumerLoaderMutation)}
	runtime = retryExecutionAssemble(t, d)
	var scheduler ExptSchedulerEvent = runtime.Scheduler
	var evaluator ExptItemEvalEvent = runtime.Consumer
	if build != nil {
		scheduler = driver.Scheduler
		evaluator = driver.Consumer
	}
	event.ExptRunID, event.ExptRunMode = f.key.RunID, entity.EvaluationModeRetryItems
	if build != nil {
		require.NotEmpty(t, pub.published)
		event = pub.published[len(pub.published)-1]
		require.Equal(t, f.key.RunID, event.ExptRunID)
		require.Equal(t, "original", event.Ext["route"])
	}
	pub.items = nil
	if scenario == "before_wait" {
		original := finalizationRead(t, f)
		require.NoError(t, scheduler.Schedule(ctx, event))
		require.Empty(t, pub.items)
		_, retried, err := f.manager.LogRetryItemsRun(ctx, f.expt, entity.EvaluationModeRetryItems, f.space, []int64{802}, &entity.Session{UserID: "different-user"})
		require.NoError(t, err)
		require.True(t, retried)
		require.Equal(t, int64(1), finalizationRead(t, f).PlanCount)
		completeExecutionChainBefore(t, f)
		require.NoError(t, scheduler.Schedule(ctx, event))
		require.Len(t, pub.items, 2)
		for _, item := range pub.items {
			require.NoError(t, evaluator.Eval(ctx, item))
		}
		require.NoError(t, scheduler.Schedule(ctx, event))
		finished := finalizationRead(t, f)
		require.True(t, finished.State.After.Activated)
		require.Equal(t, original.State.Before.ID, finished.State.Before.ID)
		require.Equal(t, original.Snapshot, finished.Snapshot)
		require.Equal(t, 4, target.calls)
		return
	}
	require.NoError(t, scheduler.Schedule(ctx, event))
	require.Len(t, pub.items, 1)
	original := finalizationRead(t, f)
	if build != nil {
		runID, err := driver.Start(ctx, []int64{802})
		require.NoError(t, err)
		require.Equal(t, f.key.RunID, runID)
	} else {
		runID, retried, err := f.manager.LogRetryItemsRun(ctx, f.expt, entity.EvaluationModeRetryItems, f.space, []int64{802}, &entity.Session{UserID: "different-user"})
		require.NoError(t, err)
		require.True(t, retried)
		require.Equal(t, f.key.RunID, runID)
	}
	_, _, duplicateErr := f.manager.LogRetryItemsRun(ctx, f.expt, entity.EvaluationModeRetryItems, f.space, []int64{802}, &entity.Session{UserID: "different-user"})
	require.Error(t, duplicateErr, "duplicate accepted IDs retain the existing rejection contract")
	if scenario == "hash_drift" {
		runtime.Consumer.boundContext.loader = boundConsumerLoaderMutation{runtime.Consumer.boundContext.loader, func(page *entity.HookLoadedPlanPage) { page.Hash = strings.Repeat("f", 64) }}
		_, err := runtime.Consumer.BuildExptRecordEvalCtx(ctx, pub.items[0])
		require.Error(t, err, "same-count hash replacement is not append growth")
		return
	}
	if scenario == "context_tail" {
		var once sync.Once
		runtime.Consumer.boundContext.loader = boundConsumerLoaderMutation{runtime.Consumer.boundContext.loader, func(page *entity.HookLoadedPlanPage) {
			once.Do(func() {
				changed, err := runtime.Scheduler.PrepareRetryItemsTail(ctx, f.key)
				require.NoError(t, err)
				require.True(t, changed)
			})
		}}
	}
	if scenario == "cancel_tail" {
		require.NoError(t, runtime.Manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{Status: entity.ExptStatus_Terminated, Reason: "user stop"}))
		finished := finalizationRead(t, f)
		require.True(t, finished.State.After.Activated)
		require.Equal(t, int64(1), finished.PlanCount)
		var c entity.HookRetryItemsCursor
		require.NoError(t, json.Unmarshal([]byte(finished.PlanCursor), &c))
		require.Equal(t, 2, c.Terminal.AcceptedBatches)
		require.Equal(t, 1, c.Terminal.Batch)
		require.NoError(t, evaluator.Eval(ctx, pub.items[0]))
		require.Equal(t, 2, target.calls)
		require.Equal(t, original.Snapshot, finished.Snapshot)
		return
	}
	require.NoError(t, evaluator.Eval(ctx, pub.items[0]))
	require.Equal(t, 3, target.calls, "RetryItems reruns the target even when the source succeeded")
	pub.items = nil
	if scenario == "source_io" || scenario == "source_exhausted" {
		loader := runtime.Scheduler.hookBoundInitializer.deps.Loader.(retryItemsTestLoader).PlanPageLoader.(*hookFrozenPlanLoader)
		dataset := loader.deps.Items.(*boundInitDataset)
		dataset.fail = true
		if scenario == "source_exhausted" {
			for i := 0; i < 11; i++ {
				_, err := runtime.Scheduler.PrepareRetryItemsTail(ctx, f.key)
				if i < 10 {
					require.ErrorIs(t, err, ErrHookPlanSourceRetry)
				} else {
					require.ErrorIs(t, err, ErrHookPlanPreparationFailed)
				}
			}
			require.NoError(t, runtime.Manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
			finished := finalizationRead(t, f)
			require.Equal(t, entity.ExptStatus_SystemTerminated, finished.State.Intent.Status)
			require.Equal(t, "HOOK_PLAN_SOURCE_EXHAUSTED", finished.State.Intent.Reason)
			require.True(t, finished.State.After.Activated)
			require.Equal(t, int64(1), finished.PlanCount)
			require.Equal(t, 3, target.calls)
			return
		}
		_, err := runtime.Scheduler.PrepareRetryItemsTail(ctx, f.key)
		require.ErrorIs(t, err, ErrHookPlanSourceRetry)
		waiting := finalizationRead(t, f)
		var c entity.HookRetryItemsCursor
		require.NoError(t, json.Unmarshal([]byte(waiting.PlanCursor), &c))
		require.Equal(t, 1, c.Retries)
		require.Equal(t, int64(1), waiting.PlanCount)
		require.Equal(t, entity.HookFinalizeNone, waiting.State.Finalize)
		require.NoError(t, scheduler.Schedule(ctx, event))
		require.Empty(t, pub.items)
		require.Equal(t, entity.HookFinalizeNone, finalizationRead(t, f).State.Finalize)
		dataset.fail = false
	}
	require.NoError(t, scheduler.Schedule(ctx, event))
	require.Equal(t, int64(2), finalizationRead(t, f).PlanCount, "accepted tail must be materialized before after")
	require.Len(t, pub.items, 1)
	require.Equal(t, int64(802), pub.items[0].EvalSetItemID)
	require.NoError(t, evaluator.Eval(ctx, pub.items[0]))
	require.NoError(t, scheduler.Schedule(ctx, event))
	finished := finalizationRead(t, f)
	require.Equal(t, entity.ExptStatus_Success, finished.State.Intent.Status)
	require.True(t, finished.State.After.Activated)
	require.Equal(t, 4, target.calls)
	require.Equal(t, original.Snapshot, finished.Snapshot)
	require.Equal(t, original.CreatedBy, finished.CreatedBy)
}
