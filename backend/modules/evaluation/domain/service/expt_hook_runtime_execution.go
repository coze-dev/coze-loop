// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/jinzhu/copier"
)

type HookRuntimeExecutionDependencies struct {
	Manager      IExptManager
	Scheduler    ExptSchedulerEvent
	Consumer     ExptItemEvalEvent
	Binding      *entity.HookExecutionInitializationBinding
	Repositories repo.HookBoundRuntimeRepositories
	Loader       hook.PlanPageLoader
	IDs          idgen.IIDGenerator
}

type HookRuntimeExecution struct {
	Manager   *ExptMangerImpl
	Scheduler *ExptSchedulerImpl
	Consumer  *ExptItemEventEvalServiceImpl
}

// The readonly constructor stays readonly. Only a complete, same-Run bundle can execute.
func NewBoundHookRuntimeExecution(d HookRuntimeExecutionDependencies) (*HookRuntimeExecution, error) {
	m, mok := d.Manager.(*ExptMangerImpl)
	s, sok := d.Scheduler.(*ExptSchedulerImpl)
	c, cok := d.Consumer.(*ExptItemEventEvalServiceImpl)
	source := d.Binding.Input()
	if !mok || !sok || !cok || m == nil || s == nil || c == nil || m.finalization == nil || c.boundContext != nil || source.Execution == nil || source.Execution.EvaluatorFallback == nil {
		return nil, entity.ErrHookExecutionUnsupported
	}
	r := d.Repositories
	for _, dep := range []any{r.Runs, r.Initialization, r.Finalization, r.Consumer, r.Progress, r.Gate, r.Source, d.Loader, d.IDs, m.finalization.NewItemLocker, s.ExptRepo, s.ExptItemResultRepo, s.ExptTurnResultRepo, s.ExptStatsRepo, s.Idem, s.Configer, s.Mutex, s.Publisher, s.Metric, c.configer, c.metric, c.experimentRepo, c.exptItemResultRepo, c.exptTurnResultRepo, c.mutex, c.publisher, c.benefitService, c.evaluationSetItemService, c.evaluatorRecordService, c.evalAsyncRepo} {
		if hookExecutionNil(dep) {
			return nil, entity.ErrHookExecutionUnsupported
		}
	}
	for _, dep := range []any{r.Runs, r.Initialization, r.Finalization, r.Consumer, r.Progress, r.Gate} {
		owner, ok := dep.(repo.IHookBoundExecutionOwner)
		if !ok {
			return nil, entity.ErrHookExecutionUnsupported
		}
		key, scope, hash := owner.HookExecutionBinding()
		if key != source.Key || scope != source.ExecutionScope || hash != source.SnapshotHash {
			return nil, entity.ErrHookStoreConflict
		}
	}
	if _, ok := r.Progress.(interface {
		repo.IHookTurnLogInitializer
		repo.IHookTurnResultWriteRepo
		repo.IHookItemRunWriteRepo
		repo.IHookConsumerControlRepo
	}); !ok {
		return nil, entity.ErrHookExecutionUnsupported
	}
	if m.finalization.ExecutionScope != source.ExecutionScope {
		return nil, entity.ErrHookStoreConflict
	}
	factory, ok := s.schedulerModeFactory.(*DefaultSchedulerModeFactory)
	result, rok := s.ResultSvc.(*ExptResultServiceImpl)
	archive, aok := r.Finalization.(repo.IHookItemArchiveRepo)
	if !ok || factory == nil || !rok || result == nil || !aok {
		return nil, entity.ErrHookExecutionUnsupported
	}
	if source.Execution.Target.ID > 0 && hookExecutionNil(c.evaTargetService) {
		return nil, entity.ErrHookExecutionUnsupported
	}
	if source.Execution.SingleSet && len(source.Execution.EvaluatorFallback.Confs) > 0 && hookExecutionNil(c.evaluatorService) {
		return nil, entity.ErrHookExecutionUnsupported
	}
	if entity.HookBoundRetryMode(source.Mode) {
		if _, ok := r.Progress.(repo.IHookRetryTurnLogInitializer); !ok {
			return nil, entity.ErrHookExecutionUnsupported
		}
	}
	for _, set := range source.Execution.Sets {
		if len(set.ItemConfig.EvaluatorConfs) > 0 && hookExecutionNil(c.evaluatorService) {
			return nil, entity.ErrHookExecutionUnsupported
		}
	}
	resultCopy := *result
	resultCopy.idgen, resultCopy.evaluatorRecordService = d.IDs, c.evaluatorRecordService
	if hookExecutionNil(resultCopy.scoreCalculator) {
		return nil, entity.ErrHookExecutionUnsupported
	}
	resultSvc, err := resultCopy.WithHookArchive(archive, source.ExecutionScope)
	if err != nil {
		return nil, err
	}
	manager := *m
	if source.Mode == entity.EvaluationModeAppend {
		if m.hooks == nil {
			return nil, entity.ErrHookExecutionUnsupported
		}
		hooks := *m.hooks
		hooks.Runs = r.Runs
		manager.hooks = &hooks
		manager.onlineItems = c.evaluationSetItemService
	}
	finalization := *m.finalization
	finalization.Runs, finalization.Repository = r.Runs, r.Finalization
	manager.finalization = &finalization
	deletion, ok := r.Runs.(repo.IHookDeletionRepo)
	if !ok {
		return nil, entity.ErrHookExecutionUnsupported
	}
	manager.deletion = deletion
	manager.exptResultService, manager.evalTargetService, manager.idgenerator = resultSvc, c.evaTargetService, d.IDs
	view := &boundRuntimeManager{IExptManager: &manager, key: source.Key, source: r.Finalization, target: c.evaTargetService}
	consumer := *c
	consumer.manager, consumer.resultSvc, consumer.idgen = view, resultSvc, d.IDs
	aware, err := NewHookAwareExptRecordEvalService(&consumer, r.Gate, r.Source, r.Runs, r.Progress)
	if err != nil {
		return nil, err
	}
	bound, err := NewBoundHookExptRecordContextService(aware, d.Binding, r.Consumer, d.Loader)
	if err != nil {
		return nil, err
	}
	scheduler := *s
	scheduler.Manager, scheduler.ResultSvc = &manager, resultSvc
	awareScheduler, err := NewHookAwareExptSchedulerSvc(&scheduler, r.Gate)
	if err != nil {
		return nil, err
	}
	scheduled := awareScheduler.(*ExptSchedulerImpl)
	initializer, err := NewBoundHookFrozenExecutionInitializer(HookFrozenExecutionInitializerDependencies{Repository: r.Initialization, Loader: d.Loader, IDs: d.IDs}, d.Binding)
	if err != nil {
		return nil, err
	}
	scheduled.hookBoundInitializer = initializer.(*hookFrozenExecutionInitializer)
	scheduled.hookBoundMode = source.Mode
	scheduled.Manager = view
	modeFactory := *factory
	modeFactory.manager, modeFactory.resultSvc, modeFactory.evalTargetService = view, scheduled.ResultSvc, c.evaTargetService
	scheduled.schedulerModeFactory = &modeFactory
	bound.boundContext.execute = true
	return &HookRuntimeExecution{Manager: &manager, Scheduler: scheduled, Consumer: bound}, nil
}

type boundRuntimeManager struct {
	IExptManager
	key    entity.HookRunKey
	source repo.IHookFinalizationRepo
	target IEvalTargetService
}

func (m *boundRuntimeManager) GetRunLog(ctx context.Context, expt, run, space int64, _ *entity.Session) (*entity.ExptRunLog, error) {
	if m.key != (entity.HookRunKey{WorkspaceID: space, ExperimentID: expt, RunID: run}) {
		return nil, entity.ErrHookStoreConflict
	}
	source, err := m.source.ReadFinalizationSource(ctx, m.key)
	if err != nil {
		return nil, err
	}
	return source.RunLog, nil
}

func (m *boundRuntimeManager) GetDetail(ctx context.Context, expt, space int64, _ *entity.Session, _ ...entity.GetExptTupleOptionFn) (*entity.Experiment, error) {
	if expt != m.key.ExperimentID || space != m.key.WorkspaceID {
		return nil, entity.ErrHookStoreConflict
	}
	source, err := m.source.ReadFinalizationSource(ctx, m.key)
	if err != nil {
		return nil, err
	}
	view := source.Experiment
	if view.TargetID > 0 {
		owner := resolveLoadSpaceID(space, view.TargetSpaceID)
		target, err := m.target.GetEvalTargetVersion(ctx, owner, view.TargetVersionID, false)
		if err != nil {
			return nil, err
		}
		if target == nil || target.ID != view.TargetID || target.SpaceID != owner || target.EvalTargetVersion == nil || target.EvalTargetVersion.ID != view.TargetVersionID || target.EvalTargetVersion.TargetID != view.TargetID || target.EvalTargetVersion.SpaceID != owner || target.EvalTargetType != view.TargetType {
			return nil, entity.ErrHookStoreCorrupt
		}
		view.Target = new(entity.EvalTarget)
		if err := copier.CopyWithOption(view.Target, target, copier.Option{DeepCopy: true}); err != nil {
			return nil, err
		}
	}
	view.EvalSet = &entity.EvaluationSet{ID: view.EvalSetID, SpaceID: resolveLoadSpaceID(space, view.EvalSetSpaceID), EvaluationSetVersion: &entity.EvaluationSetVersion{ID: view.EvalSetVersionID, EvaluationSetID: view.EvalSetID}}
	return view, nil
}
