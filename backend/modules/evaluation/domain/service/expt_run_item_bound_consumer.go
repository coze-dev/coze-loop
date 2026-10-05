// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"reflect"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/jinzhu/copier"
)

type boundHookConsumerContext struct {
	binding *entity.HookExecutionInitializationBinding
	reader  repo.IHookBoundConsumerRepo
	loader  hook.PlanPageLoader
	execute bool
}

// This explicit constructor grants verified context reads, not execution or admission.
func NewBoundHookExptRecordContextService(base ExptItemEvalEvent, binding *entity.HookExecutionInitializationBinding, reader repo.IHookBoundConsumerRepo, loader hook.PlanPageLoader) (*ExptItemEventEvalServiceImpl, error) {
	legacy, ok := base.(*ExptItemEventEvalServiceImpl)
	source := binding.Input()
	if !ok || legacy == nil || legacy.hookAdmission == nil || legacy.boundContext != nil || source.Execution == nil || source.Execution.EvaluatorFallback == nil || hookExecutionNil(reader) || hookExecutionNil(loader) {
		return nil, entity.ErrHookExecutionUnsupported
	}
	if source.Execution.Target.ID > 0 && hookExecutionNil(legacy.evaTargetService) {
		return nil, entity.ErrHookExecutionUnsupported
	}
	if source.Execution.SingleSet && len(source.Execution.EvaluatorFallback.Confs) > 0 && hookExecutionNil(legacy.evaluatorService) {
		return nil, entity.ErrHookExecutionUnsupported
	}
	for _, set := range source.Execution.Sets {
		if len(set.ItemConfig.EvaluatorConfs) > 0 && hookExecutionNil(legacy.evaluatorService) {
			return nil, entity.ErrHookExecutionUnsupported
		}
	}
	copy := *legacy
	if source.Mode == entity.EvaluationModeRetryItems || source.Mode == entity.EvaluationModeAppend {
		initializer, _ := legacy.hookAdmission.progress.(repo.IHookTurnLogInitializer)
		copy.exptTurnResultRepo = &hookRetryItemsTurnWriter{IExptTurnResultRepo: legacy.exptTurnResultRepo, initializer: initializer, key: source.Key}
	}
	copy.boundContext = &boundHookConsumerContext{binding: binding, reader: reader, loader: loader}
	h := legacy.hookAdmission
	aware, err := NewHookAwareExptRecordEvalService(&copy, h.gate, h.source, h.runs, h.progress)
	if err != nil {
		return nil, err
	}
	return aware.(*ExptItemEventEvalServiceImpl), nil
}

func (e *ExptItemEventEvalServiceImpl) readBoundConsumer(ctx context.Context, event *entity.ExptItemEvalEvent, source entity.HookExecutionInitializationSource) (*entity.HookBoundConsumerItem, error) {
	p, err := e.boundContext.reader.ReadBoundConsumerItem(ctx, source.Key, source.ExecutionScope, event.EvalSetItemID)
	if err != nil {
		return nil, err
	}
	if p == nil || p.SnapshotHash != source.SnapshotHash || p.Manifest.Validate() != nil || p.Manifest.Key != source.Key || p.Manifest.Frozen.ItemID != event.EvalSetItemID || (!source.Execution.SingleSet && p.Manifest.ItemRef == nil) || p.ItemConfig == nil || p.Experiment == nil || p.Experiment.ID != source.Key.ExperimentID || p.Experiment.SpaceID != source.Key.WorkspaceID || p.Experiment.LatestRunID != source.Key.RunID || p.Result == nil || p.Result.ItemResultRunLog == nil {
		return nil, entity.ErrHookStoreCorrupt
	}
	if source.Execution.SingleSet {
		if p.Manifest.ItemRef != nil {
			return nil, entity.ErrHookStoreCorrupt
		}
		return p, nil
	}
	raw, err := json.Marshal(p.ItemConfig)
	if err != nil {
		return nil, entity.ErrHookStoreCorrupt
	}
	hash := sha256.Sum256(raw)
	if hex.EncodeToString(hash[:]) != p.Manifest.ItemRef.ConfigHash {
		return nil, entity.ErrHookStoreCorrupt
	}
	return p, nil
}

func (e *ExptItemEventEvalServiceImpl) buildBoundHookConsumerContext(ctx context.Context, event *entity.ExptItemEvalEvent) (*entity.ExptItemEvalCtx, error) {
	if ctx == nil || event == nil {
		return nil, entity.ErrHookStoreConflict
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	source := e.boundContext.binding.Input()
	if itemHookKey(event) != source.Key || event.ExptRunMode != source.Mode {
		return nil, entity.ErrHookStoreConflict
	}
	proof, err := e.readBoundConsumer(ctx, event, source)
	if err != nil {
		return nil, err
	}
	m := proof.Manifest
	loaded, err := e.boundContext.loader.LoadPage(ctx, entity.HookPlanReadInput{Key: source.Key, ExecutionScope: source.ExecutionScope, StartOrdinal: m.Ordinal, Limit: 1})
	if err != nil {
		return nil, err
	}
	stablePlan := loaded != nil && loaded.Hash == proof.PlanHash && loaded.Count == proof.PlanCount
	if (source.Mode == entity.EvaluationModeRetryItems || source.Mode == entity.EvaluationModeAppend) && loaded != nil {
		stablePlan = stablePlan || loaded.Count > proof.PlanCount
	}
	if loaded == nil || len(loaded.Items) != 1 || !stablePlan || loaded.NextOrdinal != m.Ordinal+1 || loaded.RunVersion < proof.RunVersion {
		return nil, entity.ErrHookStoreCorrupt
	}
	item := loaded.Items[0]
	if item.Ordinal != m.Ordinal || item.Frozen != m.Frozen || item.Item == nil || item.Item.ItemID != m.Frozen.ItemID || item.Item.SpaceID != m.Frozen.SourceSpaceID || item.Item.EvaluationSetID != m.Frozen.EvalSetID || (m.Frozen.ItemVersionID > 0 && gptr.Indirect(item.Item.ItemVersionID) != m.Frozen.ItemVersionID) || len(item.Item.Turns) != len(m.Turns) {
		return nil, entity.ErrHookStoreCorrupt
	}
	for i, turn := range item.Item.Turns {
		if turn == nil || turn.ID != m.Turns[i].TurnID || m.Turns[i].TurnIdx != int32(i) {
			return nil, entity.ErrHookStoreCorrupt
		}
	}
	d := source.Execution
	var target *entity.EvalTarget
	if d.Target.ID > 0 {
		if d.Target.VersionID <= 0 || d.Target.Config == nil {
			return nil, entity.ErrHookExecutionUnsupported
		}
		space := resolveLoadSpaceID(source.Key.WorkspaceID, d.Target.SourceSpaceID)
		itemScope := (&entity.ExptItemEvalCtx{Expt: &entity.Experiment{TargetSpaceID: d.Target.SourceSpaceID}, ItemConfig: proof.ItemConfig}).TargetSourceSpaceID()
		if resolveLoadSpaceID(source.Key.WorkspaceID, itemScope) != space {
			return nil, entity.ErrHookStoreCorrupt
		}
		got, err := e.evaTargetService.GetEvalTargetVersion(ctx, space, d.Target.VersionID, false)
		if err != nil {
			return nil, err
		}
		if got == nil || got.ID != d.Target.ID || got.SpaceID != space || got.EvalTargetType != d.Target.Type || got.EvalTargetVersion == nil || got.EvalTargetVersion.ID != d.Target.VersionID || got.EvalTargetVersion.TargetID != d.Target.ID || got.EvalTargetVersion.SpaceID != space {
			return nil, entity.ErrHookStoreCorrupt
		}
		target = new(entity.EvalTarget)
		if err := copier.CopyWithOption(target, got, copier.Option{DeepCopy: true}); err != nil {
			return nil, entity.ErrHookStoreCorrupt
		}
	}
	static := &entity.EvaluatorsConf{EvaluatorConf: d.EvaluatorFallback.Confs, EnableScoreWeight: d.EvaluatorFallback.EnableScoreWeight}
	if err := static.Valid(ctx); err != nil {
		return nil, entity.ErrHookStoreCorrupt
	}
	ids := []int64{}
	wanted := map[int64]bool{}
	itemConfs := proof.ItemConfig.EvaluatorConfs
	if d.SingleSet {
		for _, conf := range static.EvaluatorConf {
			itemConfs = append(itemConfs, &entity.ItemEvaluatorConf{EvaluatorVersionID: conf.EvaluatorVersionID})
		}
	}
	for _, conf := range itemConfs {
		if conf == nil || conf.EvaluatorVersionID <= 0 || static.GetEvaluatorConf(conf.EvaluatorVersionID) == nil {
			return nil, entity.ErrHookStoreCorrupt
		}
		if !wanted[conf.EvaluatorVersionID] {
			wanted[conf.EvaluatorVersionID] = true
			ids = append(ids, conf.EvaluatorVersionID)
		}
	}
	var evaluators []*entity.Evaluator
	if len(ids) > 0 {
		got, err := e.evaluatorService.BatchGetEvaluatorVersion(ctx, nil, ids, false)
		if err != nil {
			return nil, err
		}
		if len(got) != len(ids) {
			return nil, entity.ErrHookStoreCorrupt
		}
		for _, value := range got {
			if value == nil || !wanted[value.GetEvaluatorVersionID()] || value.GetSpaceID() <= 0 || value.SpaceID != value.GetSpaceID() {
				return nil, entity.ErrHookStoreCorrupt
			}
			delete(wanted, value.GetEvaluatorVersionID())
			owned := new(entity.Evaluator)
			if err := copier.CopyWithOption(owned, value, copier.Option{DeepCopy: true}); err != nil {
				return nil, entity.ErrHookStoreCorrupt
			}
			evaluators = append(evaluators, owned)
		}
	}
	// Recheck after all external reads; a cancellation or ref replacement must not escape as a usable context.
	current, err := e.readBoundConsumer(ctx, event, source)
	if err != nil {
		return nil, err
	}
	stablePlan = current.PlanHash == proof.PlanHash && current.PlanCount == proof.PlanCount
	if source.Mode == entity.EvaluationModeRetryItems || source.Mode == entity.EvaluationModeAppend {
		stablePlan = stablePlan || current.PlanCount > proof.PlanCount
	}
	if !reflect.DeepEqual(current.Manifest, m) || !stablePlan || current.RunVersion < proof.RunVersion {
		return nil, entity.ErrHookStoreConflict
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	view := current.Experiment
	if view.EvalConf == nil {
		view.EvalConf = &entity.EvaluationConfiguration{}
	}
	if live := view.EvalConf.ConnectorConf.EvaluatorsConf; live != nil {
		static.EvaluatorConcurNum = live.EvaluatorConcurNum
	}
	view.EvalConf.ConnectorConf.TargetConf = d.Target.Config
	view.EvalConf.ConnectorConf.EvaluatorsConf = static
	view.EvalConf.RunModeConfig = d.RunModeConfig
	view.EvalConf.VerificationConfig = d.VerificationConfig
	view.EvalConf.EnableExtractTrajectory = d.EnableExtractTrajectory
	view.EvalConf.SkillTOSKeys = d.SkillTOSKeys
	view.TargetID, view.TargetVersionID, view.TargetType, view.TargetSpaceID = d.Target.ID, d.Target.VersionID, d.Target.Type, d.Target.SourceSpaceID
	view.Target, view.Evaluators = target, evaluators
	view.EvalSetID, view.EvalSetVersionID, view.EvalSetSpaceID = m.Frozen.EvalSetID, m.Frozen.EvalSetVersionID, current.ItemConfig.EvalSetSourceSpaceID
	view.EvalSet = &entity.EvaluationSet{ID: m.Frozen.EvalSetID, SpaceID: m.Frozen.SourceSpaceID, EvaluationSetVersion: &entity.EvaluationSetVersion{ID: m.Frozen.EvalSetVersionID, SpaceID: m.Frozen.SourceSpaceID, EvaluationSetID: m.Frozen.EvalSetID}}
	ownedEvent := new(entity.ExptItemEvalEvent)
	if err := copier.CopyWithOption(ownedEvent, event, copier.Option{DeepCopy: true}); err != nil {
		return nil, entity.ErrHookStoreCorrupt
	}
	manifest := current.Manifest.Clone()
	itemConfig := current.ItemConfig
	if d.SingleSet {
		itemConfig = nil
	}
	return &entity.ExptItemEvalCtx{Event: ownedEvent, Expt: view, EvalSetItem: item.Item, ExistItemEvalResult: current.Result, ItemConfig: itemConfig, EvalSetVersionID: m.Frozen.EvalSetVersionID, HookManifest: &manifest}, nil
}
