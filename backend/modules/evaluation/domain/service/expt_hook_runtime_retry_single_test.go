// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	em "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
	exptinfra "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/stretchr/testify/require"
)

type singleRetryDataset struct {
	boundConsumerDataset
	extra bool
}

func (s *singleRetryDataset) BatchGetEvaluationSetItems(ctx context.Context, in *entity.BatchGetEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, error) {
	items, err := s.boundConsumerDataset.BatchGetEvaluationSetItems(ctx, in)
	for _, item := range items {
		item.BaseInfo = &entity.BaseInfo{}
	}
	return items, err
}
func (s *singleRetryDataset) ListEvaluationSetItems(ctx context.Context, in *entity.ListEvaluationSetItemsParam) ([]*entity.EvaluationSetItem, *int64, *int64, *string, error) {
	ids := []int64{901, 902}
	if s.extra {
		ids = append(ids, 903)
	}
	items, err := s.BatchGetEvaluationSetItems(ctx, &entity.BatchGetEvaluationSetItemsParam{SpaceID: in.SpaceID, EvaluationSetID: in.EvaluationSetID, ItemIDs: ids})
	return items, gptr.Of(int64(len(ids))), gptr.Of(int64(len(ids))), nil, err
}

type singleRetryManager struct {
	IExptManager
	f          *finalizationManagerFixture
	evaluators *retryExecutionEvaluators
}

func (m singleRetryManager) GetDetail(ctx context.Context, id, space int64, _ *entity.Session, _ ...entity.GetExptTupleOptionFn) (*entity.Experiment, error) {
	view, err := m.f.manager.exptRepo.GetByID(ctx, id, space)
	if err != nil {
		return nil, err
	}
	view.EvalSet = &entity.EvaluationSet{ID: 71, SpaceID: space, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: 71, EvaluationSetID: 71, SpaceID: space}}
	view.Evaluators, err = m.evaluators.BatchGetEvaluatorVersion(ctx, nil, []int64{111}, false)
	return view, err
}

func TestHookRetrySingleSetUsesLegacyGlobalConfigurationMySQL(t *testing.T) {
	for _, mode := range []entity.ExptRunMode{entity.EvaluationModeFailRetry, entity.EvaluationModeRetryAll} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
			defer cancel()
			f := foundationManagerFixture(t, false, true)
			conf := &entity.EvaluationConfiguration{ItemConcurNum: gptr.Of(2), ConnectorConf: entity.Connector{EvaluatorsConf: &entity.EvaluatorsConf{EvaluatorConcurNum: gptr.Of(2), EvaluatorConf: []*entity.EvaluatorConf{{EvaluatorVersionID: 111, IngressConf: &entity.EvaluatorIngressConf{EvalSetAdapter: &entity.FieldAdapter{}}, RunConf: &entity.EvaluatorRunConfig{Env: gptr.Of("original-env")}}}}}}
			raw, err := json.Marshal(conf)
			require.NoError(t, err)
			require.NoError(t, f.sql.Model(&model.Experiment{}).Where("id=?", f.expt).UpdateColumns(map[string]any{"eval_set_source_type": 1, "eval_set_id": 71, "eval_set_version_id": 71, "target_id": 0, "target_version_id": 0, "target_type": 0, "target_space_id": 0, "eval_conf": raw}).Error)
			foundationCreate(t, f, entity.EvaluationModeSubmit)
			plan := []entity.HookPlanItem{{ID: finalizationTestIDs.Add(1), SourceSpaceID: f.space, EvalSetID: 71, ItemID: 901}, {ID: finalizationTestIDs.Add(1), SourceSpaceID: f.space, EvalSetID: 71, ItemID: 902}}
			run := finalizationRead(t, f)
			page, err := f.repo.AppendPlanPage(ctx, entity.HookPlanPageInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: run.Version}, Items: plan})
			require.NoError(t, err)
			digest, err := entity.AppendHookPlanDigest(entity.NewHookPlanDigest(), plan)
			require.NoError(t, err)
			_, err = f.repo.FinishPlan(ctx, entity.HookFinishPlanInput{HookStoreGuard: entity.HookStoreGuard{Key: f.key, ExpectedVersion: page.Run.Version}, Count: 2, Hash: digest.Hash})
			require.NoError(t, err)
			dataset := new(singleRetryDataset)
			loader, err := NewHookFrozenPlanLoader(HookFrozenPlanLoaderDependencies{Plans: exptinfra.NewHookPlanRepo(f.p), Items: dataset, Versions: &loaderVersions{}, Sets: &loaderSets{value: &entity.EvaluationSet{ID: 71, SpaceID: f.space, EvaluationSetVersion: &entity.EvaluationSetVersion{EvaluationSetSchema: &entity.EvaluationSetSchema{ID: 73, SpaceID: f.space, EvaluationSetID: 71}}}}})
			require.NoError(t, err)
			initializer, err := NewHookFrozenExecutionInitializer(HookFrozenExecutionInitializerDependencies{Repository: exptinfra.NewHookExecutionInitializationRepo(f.p), Loader: loader, IDs: activeReferenceIDs{}})
			require.NoError(t, err)
			_, err = initializer.InitializeExecution(ctx, f.key, "local")
			require.NoError(t, err)
			d, pub, _ := executionChainDependencies(t, f, nil, repo.HookBoundRuntimeRepositories{}, loader, entity.EvalTargetTypeLoopPrompt)
			base := d.Consumer.(*ExptItemEventEvalServiceImpl)
			base.idgen = activeReferenceIDs{}
			evals := &retryExecutionEvaluators{executionChainEvaluators: &executionChainEvaluators{f: f}, calls: map[int64]int{}, fail: true, failItem: 902}
			base.evaluatorService = evals
			base.evaluationSetItemService = dataset
			base.manager = singleRetryManager{IExptManager: f.manager, f: f, evaluators: evals}
			gate := exptinfra.NewHookExecutionGateRepo(f.p, func(context.Context) (string, error) { return "local", nil })
			consumer, err := NewHookAwareExptRecordEvalService(base, gate, exptinfra.NewHookItemSourceRepo(f.p), f.repo, exptinfra.NewHookTurnProgressRepo(f.p, func(context.Context) (string, error) { return "local", nil }))
			require.NoError(t, err)
			result, err := d.Scheduler.(*ExptSchedulerImpl).ResultSvc.(*ExptResultServiceImpl).WithHookArchive(f.manager.finalization.Repository.(repo.IHookItemArchiveRepo), "local")
			require.NoError(t, err)
			_, err = f.manager.finalization.Repository.(repo.IHookSchedulerRepo).PersistHookDispatch(ctx, entity.HookSchedulerDispatchInput{Key: f.key, ExecutionScope: "local", ItemIDs: []int64{901, 902}})
			require.NoError(t, err)
			for _, id := range []int64{901, 902} {
				event := &entity.ExptItemEvalEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ExptRunMode: entity.EvaluationModeSubmit, EvalSetItemID: id, CreateAt: time.Now().Unix(), Session: &entity.Session{UserID: "original-user"}}
				require.NoError(t, consumer.Eval(ctx, event))
				view, err := base.manager.GetDetail(ctx, f.expt, f.space, event.Session)
				require.NoError(t, err)
				_, err = result.RecordItemRunLogs(ctx, f.expt, f.key.RunID, id, f.space, view)
				require.NoError(t, err)
			}
			require.NoError(t, f.manager.FinalizeRun(ctx, f.key, entity.HookTerminalIntent{}))
			require.Equal(t, entity.ExptStatus_Failed, finalizationRead(t, f).State.Intent.Status)
			original := f.key
			t.Cleanup(func() {
				require.NoError(t, f.sql.Unscoped().Where("experiment_run_id=?", original.RunID).Delete(&em.EvaluatorRecord{}).Error)
			})
			f.key.RunID = finalizationTestIDs.Add(1)
			foundationCreate(t, f, mode)
			if mode == entity.EvaluationModeRetryAll {
				dataset.extra = true
			}
			selector := NewHookPlanSelector(dataset, nil, base.exptTurnResultRepo, base.exptItemResultRepo)
			preparer, err := NewHookPlanPreparer(HookPlanPreparerDependencies{Runs: f.repo, Plans: exptinfra.NewHookPlanRepo(f.p), Selector: selector, Codec: f.manager.hooks.Codec, Experiments: f.manager.exptRepo, Initialization: f.manager.hooks.Initialization, ResultReader: exptinfra.NewHookPlanResultReader(f.p), IDs: activeReferenceIDs{}})
			require.NoError(t, err)
			steps := 3
			if mode == entity.EvaluationModeRetryAll {
				steps = 2
			}
			for i := 0; i < steps; i++ {
				require.NoError(t, preparer.PreparePlan(ctx, hook.WorkerRunInput{ExecutionScope: "local", Candidate: entity.HookRunCandidate{Key: f.key}}))
			}
			require.True(t, finalizationRead(t, f).PlanReady)
			d.Binding, err = LoadHookExecutionInitializationBinding(ctx, f.repo, f.manager.hooks.Codec, f.key, "local")
			require.NoError(t, err)
			require.True(t, d.Binding.Input().Execution.SingleSet)
			d.Repositories, err = exptinfra.NewBoundHookRuntimeRepositories(f.p, d.Binding)
			require.NoError(t, err)
			runtime := retryExecutionAssemble(t, d)
			evals.fail = false
			pub.items = nil
			event := &entity.ExptScheduleEvent{SpaceID: f.space, ExptID: f.expt, ExptRunID: f.key.RunID, ExptRunMode: mode, ExptType: entity.ExptType_Offline, CreatedAt: time.Now().Unix(), Session: &entity.Session{UserID: "original-user"}}
			require.NoError(t, runtime.Scheduler.Schedule(ctx, event))
			want := 1
			if mode == entity.EvaluationModeRetryAll {
				want = 3
			}
			firstBatch := want
			if firstBatch > 2 {
				firstBatch = 2
			}
			require.Len(t, pub.items, firstBatch)
			for _, item := range pub.items[:firstBatch] {
				require.NoError(t, runtime.Consumer.Eval(ctx, item))
			}
			require.NoError(t, runtime.Scheduler.Schedule(ctx, event))
			if want == 3 {
				require.Len(t, pub.items, 3, "live concurrency of two must dispatch the third item only after the first batch completes")
				require.NoError(t, runtime.Consumer.Eval(ctx, pub.items[2]))
				require.NoError(t, runtime.Scheduler.Schedule(ctx, event))
			}
			finished := finalizationRead(t, f)
			require.Equal(t, entity.ExptStatus_Success, finished.State.Intent.Status)
			require.True(t, finished.State.After.Activated)
			require.Equal(t, 2+want, evals.calls[111])
			var stats model.ExptStats
			require.NoError(t, f.sql.Where("space_id=? AND expt_id=?", f.space, f.expt).First(&stats).Error)
			expectedTotal := int32(2)
			if mode == entity.EvaluationModeRetryAll {
				expectedTotal = 3
			}
			require.Equal(t, expectedTotal, stats.SuccessCnt)
			var refs int64
			require.NoError(t, f.sql.Model(&model.ExptItemRef{}).Where("space_id=? AND expt_id=?", f.space, f.expt).Count(&refs).Error)
			require.Zero(t, refs, "single-set retry must not become a multi-set ItemRef flow")
		})
	}
}
