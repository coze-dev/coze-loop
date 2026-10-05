// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"bytes"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// The caller decodes the original Run before opening any SQL transaction.
func NewBoundHookFinalizationRepo(p db.Provider, binding *entity.HookExecutionInitializationBinding) (repo.IHookFinalizationRepo, error) {
	r, err := NewBoundHookExecutionInitializationRepo(p, binding)
	if err != nil {
		return nil, err
	}
	return &hookFinalizationRepo{provider: p, binding: r.(*hookRunRepo).executionBinding}, nil
}

func checkBoundFinalizationRun(tx *gorm.DB, key entity.HookRunKey, scope string, b *boundHookExecution) error {
	if b == nil {
		return nil
	}
	v := b.source
	if key != v.Key || scope != v.ExecutionScope {
		return entity.ErrHookStoreConflict
	}
	var life model.ExptLifecycleRun
	if err := hookRunScope(tx, key).First(&life).Error; err != nil {
		return err
	}
	var log model.ExptRunLog
	if err := hookRunScope(tx.Unscoped(), key).First(&log).Error; err != nil {
		return err
	}
	if life.ExecutionScope != scope || life.SnapshotHash != v.SnapshotHash || life.BeforeEnabled != v.BeforeEnabled || life.AfterEnabled != v.AfterEnabled || log.DeletedAt.Valid || entity.ExptRunMode(gptr.Indirect(log.Mode)) != v.Mode || log.CreatedBy != v.CreatedBy || gptr.Indirect(life.SourceRunID) != v.SourceRunID {
		return entity.ErrHookStoreConflict
	}
	return nil
}

func boundFinalizationRequired(b *boundHookExecution, life *model.ExptLifecycleRun, mode entity.ExptRunMode) bool {
	return b != nil && b.source.Mode == mode && b.source.SnapshotHash == life.SnapshotHash && b.source.ExecutionScope == life.ExecutionScope && entity.HookBoundExecutionMode(mode)
}

// The original ID, not the current experiment/item lookup, is the ownership proof.
func checkBoundFinalizationRefs(tx *gorm.DB, key entity.HookRunKey, manifests []entity.HookExecutionManifest, b *boundHookExecution, locking bool) error {
	if b != nil && b.source.Execution.SingleSet {
		for _, m := range manifests {
			if m.Key != key || m.ItemRef != nil {
				return entity.ErrHookStoreCorrupt
			}
		}
		return nil
	}
	if b == nil {
		for _, m := range manifests {
			if m.ItemRef != nil {
				return entity.ErrHookExecutionUnsupported
			}
		}
		return nil
	}
	ids := make([]int64, 0, len(manifests))
	for _, m := range manifests {
		if m.Key != key || m.ItemRef == nil {
			return entity.ErrHookStoreCorrupt
		}
		ids = append(ids, m.ItemRef.ID)
	}
	if len(ids) == 0 {
		return nil
	}
	q := tx.Unscoped().Where("id IN ?", ids)
	if locking {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var rows []model.ExptItemRef
	if err := q.Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) != len(ids) {
		return entity.ErrHookStoreCorrupt
	}
	byID := make(map[int64]model.ExptItemRef, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}
	for _, m := range manifests {
		ref := byID[m.ItemRef.ID]
		cfg, err := b.manifestConfig(m)
		if err != nil {
			return err
		}
		if ref.DeletedAt.Valid || ref.SpaceID != key.WorkspaceID || ref.ExptID != key.ExperimentID || ref.ItemID != m.Frozen.ItemID || ref.ItemVersionID != m.Frozen.ItemVersionID || ref.EvalSetID != m.Frozen.EvalSetID || ref.EvalSetVersionID != m.Frozen.EvalSetVersionID || int64(ref.OrderIdx) != m.ProjectionOrdinal() || m.ItemRef.ConfigHash != cfg.hash || !bytes.Equal(gptr.Indirect(ref.ItemConfig), cfg.raw) {
			return entity.ErrHookStoreCorrupt
		}
	}
	return nil
}

func firstFinalizationBinding(bindings []*boundHookExecution) *boundHookExecution {
	if len(bindings) == 0 {
		return nil
	}
	return bindings[0]
}

func (r *hookFinalizationRepo) boundScope() string {
	if r.binding == nil {
		return ""
	}
	return r.binding.source.ExecutionScope
}

func (r *hookFinalizationRepo) readArchiveItem(tx *gorm.DB, key entity.HookRunKey, itemID int64) (*entity.HookTerminationItem, *model.ExptLifecycleRunItem, error) {
	item, row, err := readHookArchiveItem(tx, key, itemID)
	if err != nil {
		return nil, nil, err
	}
	if err := checkBoundFinalizationRefs(tx, key, []entity.HookExecutionManifest{item.Manifest}, r.binding, true); err != nil {
		return nil, nil, err
	}
	if r.binding != nil {
		view := &entity.Experiment{ID: key.ExperimentID, SpaceID: key.WorkspaceID}
		if err := r.restoreExecution(view, &item.Manifest.Frozen); err != nil {
			return nil, nil, err
		}
		item.Execution = view
	}
	return item, row, nil
}

func (r *hookFinalizationRepo) restoreExecution(view *entity.Experiment, item *entity.HookPlanItem) error {
	if r.binding == nil {
		return nil
	}
	source := r.binding.binding.Input()
	d := source.Execution
	if d == nil || d.EvaluatorFallback == nil {
		return entity.ErrHookExecutionUnsupported
	}
	if view.EvalConf == nil {
		view.EvalConf = &entity.EvaluationConfiguration{}
	}
	view.TargetID, view.TargetVersionID, view.TargetType, view.TargetSpaceID = d.Target.ID, d.Target.VersionID, d.Target.Type, d.Target.SourceSpaceID
	view.ExptType, view.EvalSetSourceType = entity.ExptType_Offline, entity.ExptEvalSetSourceType_MultiSetConfig
	if source.Mode == entity.EvaluationModeAppend {
		view.ExptType = entity.ExptType_Online
	}
	view.Target, view.Evaluators = nil, nil
	view.EvalConf.ConnectorConf.TargetConf = d.Target.Config
	static := &entity.EvaluatorsConf{EvaluatorConf: d.EvaluatorFallback.Confs, EnableScoreWeight: d.EvaluatorFallback.EnableScoreWeight}
	if live := view.EvalConf.ConnectorConf.EvaluatorsConf; live != nil {
		static.EvaluatorConcurNum = live.EvaluatorConcurNum
	}
	view.EvalConf.ConnectorConf.EvaluatorsConf = static
	view.EvalConf.RunModeConfig = d.RunModeConfig
	view.EvalConf.EvalSetConfigs = nil
	if d.SingleSet {
		view.EvalSetSourceType = entity.ExptEvalSetSourceType_SingleSet
		return nil
	}
	sets := d.Sets
	if item != nil {
		cfg, err := r.binding.config(*item)
		if err != nil {
			return err
		}
		sets = sets[cfg.index : cfg.index+1]
	}
	for _, set := range sets {
		conf := &entity.EvalSetConfig{EvalSetID: set.EvalSetID, EvalSetVersionID: set.EvalSetVersionID, SourceSpaceID: set.SourceSpaceID}
		for _, ec := range set.ItemConfig.EvaluatorConfs {
			conf.EvaluatorConfs = append(conf.EvaluatorConfs, &entity.ExptEvaluatorConf{EvaluatorVersionID: ec.EvaluatorVersionID, Alias: ec.Alias, ScoreWeight: ec.ScoreWeight})
		}
		view.EvalConf.EvalSetConfigs = append(view.EvalConf.EvalSetConfigs, conf)
	}
	return nil
}

func checkBoundFinalizationPage(tx *gorm.DB, key entity.HookRunKey, rows []model.ExptLifecycleRunItem, b *boundHookExecution, locking bool) error {
	var manifests []entity.HookExecutionManifest
	for _, row := range rows {
		if row.ExecutionManifest == nil {
			continue
		}
		m, err := hookTerminationManifest(key, row)
		if err != nil {
			return err
		}
		manifests = append(manifests, m)
	}
	return checkBoundFinalizationRefs(tx, key, manifests, b, locking)
}
