// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type boundExecutionSetKey struct{ space, set, version int64 }

// Shared admission must fence partial initialization even before per-Run assembly.
func hookMultiSetInitializationRequired(enabled bool, mode entity.ExptRunMode, typ entity.ExptType, source entity.ExptEvalSetSourceType) bool {
	return enabled && entity.HookBoundExecutionMode(mode) && typ == entity.ExptType_Offline && (source == entity.ExptEvalSetSourceType_MultiSetConfig || entity.HookBoundRetryMode(mode) && (source == 0 || source == entity.ExptEvalSetSourceType_SingleSet))
}

type boundExecutionConfig struct {
	raw   []byte
	hash  string
	index int
}
type boundHookExecution struct {
	source  entity.HookExecutionInitializationSource
	sets    map[boundExecutionSetKey]boundExecutionConfig
	binding *entity.HookExecutionInitializationBinding
}

func NewBoundHookExecutionInitializationRepo(p db.Provider, binding *entity.HookExecutionInitializationBinding) (repo.IHookExecutionInitializationRepo, error) {
	source := binding.Input()
	if source.Execution == nil || p == nil {
		return nil, entity.ErrHookExecutionUnsupported
	}
	b := &boundHookExecution{source: source, sets: map[boundExecutionSetKey]boundExecutionConfig{}, binding: binding}
	for index, set := range source.Execution.Sets {
		space, version := set.SourceSpaceID, set.EvalSetVersionID
		if space == 0 {
			space = source.Key.WorkspaceID
		}
		if version == set.EvalSetID {
			version = 0
		}
		key := boundExecutionSetKey{space, set.EvalSetID, version}
		if _, exists := b.sets[key]; exists {
			return nil, entity.ErrHookStoreCorrupt
		}
		raw, err := json.Marshal(set.ItemConfig)
		if err != nil {
			return nil, entity.ErrHookStoreCorrupt
		}
		sum := sha256.Sum256(raw)
		b.sets[key] = boundExecutionConfig{raw: raw, hash: hex.EncodeToString(sum[:]), index: index}
	}
	return &hookRunRepo{provider: p, executionBinding: b}, nil
}

func (b *boundHookExecution) checkRun(tx *gorm.DB, e *model.Experiment, s *lockedHookRun) error {
	v := b.source
	if s.view.State.Key != v.Key || s.life.ExecutionScope != v.ExecutionScope || s.life.SnapshotHash != v.SnapshotHash || s.view.Mode != v.Mode || s.view.CreatedBy != v.CreatedBy || s.life.BeforeEnabled != v.BeforeEnabled || s.life.AfterEnabled != v.AfterEnabled || gptr.Indirect(s.life.SourceRunID) != v.SourceRunID {
		return entity.ErrHookStoreConflict
	}
	single := e.EvalSetSourceType == 0 || e.EvalSetSourceType == int32(entity.ExptEvalSetSourceType_SingleSet)
	if v.Mode == entity.EvaluationModeAppend {
		if entity.ExptType(e.ExptType) != entity.ExptType_Online || !v.Execution.SingleSet {
			return entity.ErrHookExecutionUnsupported
		}
		return nil
	}
	if entity.ExptType(e.ExptType) != entity.ExptType_Offline || (v.Execution.SingleSet && !single) || (!v.Execution.SingleSet && entity.ExptEvalSetSourceType(e.EvalSetSourceType) != entity.ExptEvalSetSourceType_MultiSetConfig) {
		return entity.ErrHookExecutionUnsupported
	}
	return b.checkReferenceCount(tx, false)
}

func (b *boundHookExecution) checkReferenceCount(tx *gorm.DB, current bool) error {
	if entity.HookBoundRetryMode(b.source.Mode) || b.source.Mode == entity.EvaluationModeAppend {
		return nil
	}
	v := b.source
	var refs, manifests int64
	q := tx.Unscoped().Model(&model.ExptItemRef{}).Where("space_id=? AND expt_id=?", v.Key.WorkspaceID, v.Key.ExperimentID)
	// Fence the prefix only for completion; page writers must not hold shared empty gaps before inserting.
	if current {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := q.Count(&refs).Error; err != nil {
		return err
	}
	if err := hookRunScope(tx.Model(&model.ExptLifecycleRunItem{}), v.Key).Where("execution_manifest IS NOT NULL").Count(&manifests).Error; err != nil {
		return err
	}
	if refs != manifests {
		return entity.ErrHookStoreCorrupt
	}
	return nil
}

func (b *boundHookExecution) config(item entity.HookPlanItem) (boundExecutionConfig, error) {
	if b.source.Mode == entity.EvaluationModeAppend {
		raw, err := json.Marshal(&entity.ExptItemConfig{EvalSetSourceSpaceID: item.SourceSpaceID})
		if err != nil {
			return boundExecutionConfig{}, entity.ErrHookStoreCorrupt
		}
		sum := sha256.Sum256(raw)
		return boundExecutionConfig{raw: raw, hash: hex.EncodeToString(sum[:]), index: -1}, nil
	}
	config, ok := b.sets[boundExecutionSetKey{item.SourceSpaceID, item.EvalSetID, item.EvalSetVersionID}]
	if !ok {
		return boundExecutionConfig{}, entity.ErrHookStoreCorrupt
	}
	return config, nil
}

func (r *hookRunRepo) readExecutionPage(tx *gorm.DB, s *lockedHookRun, start int64, limit int) (*entity.HookExecutionInitializationPage, []model.ExptLifecycleRunItem, error) {
	if r.executionBinding != nil && r.executionBinding.source.Mode == entity.EvaluationModeAppend {
		page, rows, err := readExecutionPage(tx, s, start, limit, true)
		if err != nil {
			return nil, nil, err
		}
		for _, item := range page.Items {
			if item.Manifest == nil || item.Manifest.ItemRef != nil || item.Manifest.Retry != nil {
				return nil, nil, entity.ErrHookStoreCorrupt
			}
		}
		page.BoundSnapshotHash = r.executionBinding.source.SnapshotHash
		return page, rows, nil
	}
	if r.executionBinding != nil && entity.HookBoundRetryMode(r.executionBinding.source.Mode) {
		return r.readRetryExecutionPage(tx, s, start, limit)
	}
	page, rows, err := readExecutionPage(tx, s, start, limit)
	if err != nil {
		return nil, nil, err
	}
	b := r.executionBinding
	if b == nil {
		for _, item := range page.Items {
			if item.Manifest != nil && item.Manifest.ItemRef != nil {
				return nil, nil, entity.ErrHookExecutionUnsupported
			}
		}
		return page, rows, nil
	}
	page.BoundSnapshotHash = b.source.SnapshotHash
	itemIDs := make([]int64, 0, len(page.Items))
	refIDs := make([]int64, 0, len(page.Items))
	for _, item := range page.Items {
		if item.Manifest == nil {
			itemIDs = append(itemIDs, item.Frozen.ItemID)
		} else {
			if item.Manifest.ItemRef == nil {
				return nil, nil, entity.ErrHookStoreCorrupt
			}
			refIDs = append(refIDs, item.Manifest.ItemRef.ID)
		}
	}
	var refs []model.ExptItemRef
	if len(itemIDs) > 0 {
		// Strict inserts reject competing occupants; do not lock absent ref ranges before creating them.
		if err := tx.Unscoped().Where("space_id=? AND expt_id=? AND item_id IN ?", b.source.Key.WorkspaceID, b.source.Key.ExperimentID, itemIDs).Find(&refs).Error; err != nil {
			return nil, nil, err
		}
	}
	if len(refIDs) > 0 {
		var committed []model.ExptItemRef
		if err := tx.Unscoped().Where("id IN ?", refIDs).Clauses(clause.Locking{Strength: "UPDATE"}).Find(&committed).Error; err != nil {
			return nil, nil, err
		}
		if len(committed) != len(refIDs) {
			return nil, nil, entity.ErrHookStoreCorrupt
		}
		refs = append(refs, committed...)
	}
	byItem := make(map[int64]model.ExptItemRef, len(refs))
	for _, ref := range refs {
		if _, ok := byItem[ref.ItemID]; ok {
			return nil, nil, entity.ErrHookStoreCorrupt
		}
		byItem[ref.ItemID] = ref
	}
	for i := range page.Items {
		item := &page.Items[i]
		config, err := b.config(item.Frozen)
		if err != nil {
			return nil, nil, err
		}
		item.ItemRefConfigHash = config.hash
		ref, exists := byItem[item.Frozen.ItemID]
		if item.Manifest == nil {
			if exists {
				return nil, nil, entity.ErrHookStoreCorrupt
			}
			continue
		}
		proof := item.Manifest.ItemRef
		if !exists || proof == nil || proof.ID != ref.ID || proof.ConfigHash != config.hash || ref.DeletedAt.Valid || ref.SpaceID != b.source.Key.WorkspaceID || ref.ExptID != b.source.Key.ExperimentID || ref.ItemID != item.Frozen.ItemID || ref.ItemVersionID != item.Frozen.ItemVersionID || ref.EvalSetID != item.Frozen.EvalSetID || ref.EvalSetVersionID != item.Frozen.EvalSetVersionID || int64(ref.OrderIdx) != item.Manifest.ProjectionOrdinal() || !bytes.Equal(gptr.Indirect(ref.ItemConfig), config.raw) {
			return nil, nil, entity.ErrHookStoreCorrupt
		}
	}
	return page, rows, nil
}

func (r *hookRunRepo) writeExecutionItemRef(tx *gorm.DB, m entity.HookExecutionManifest) error {
	if r.executionBinding != nil && r.executionBinding.source.Execution.SingleSet {
		if m.ItemRef != nil {
			return entity.ErrHookStoreCorrupt
		}
		return nil
	}
	if r.executionBinding == nil {
		if m.ItemRef != nil {
			return entity.ErrHookExecutionUnsupported
		}
		return nil
	}
	config, err := r.executionBinding.config(m.Frozen)
	if err != nil {
		return err
	}
	if m.ItemRef == nil || m.ItemRef.ConfigHash != config.hash {
		return entity.ErrHookStoreCorrupt
	}
	ref := model.ExptItemRef{ID: m.ItemRef.ID, SpaceID: m.Key.WorkspaceID, ExptID: m.Key.ExperimentID, ItemID: m.Frozen.ItemID, ItemVersionID: m.Frozen.ItemVersionID, EvalSetID: m.Frozen.EvalSetID, EvalSetVersionID: m.Frozen.EvalSetVersionID, OrderIdx: int32(m.Ordinal), ItemConfig: gptr.Of(config.raw)}
	return tx.Create(&ref).Error
}
