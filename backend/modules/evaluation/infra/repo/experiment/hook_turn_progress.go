// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"reflect"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

type hookTurnProgressRepo struct {
	provider       db.Provider
	executionScope func(context.Context) (string, error)
	binding        *boundHookExecution
}

// Scope is injected by platform construction, never read from the event hint.
func NewHookTurnProgressRepo(provider db.Provider, scope func(context.Context) (string, error)) repo.IHookTurnProgressRepo {
	return &hookTurnProgressRepo{provider: provider, executionScope: scope}
}

func (r *hookTurnProgressRepo) transaction(ctx context.Context, k entity.HookTurnProgressKey, fn func(*gorm.DB, *model.ExptLifecycleRun) error) error {
	if k.Validate() != nil {
		return entity.ErrHookStoreCorrupt
	}
	return r.itemTransaction(ctx, k.HookRunKey, k.ItemID, k.ItemVersionID, fn)
}

func (r *hookTurnProgressRepo) itemTransaction(ctx context.Context, k entity.HookRunKey, itemID, version int64, fn func(*gorm.DB, *model.ExptLifecycleRun) error) error {
	if (entity.HookStoreGuard{Key: k}).Validate() != nil || itemID <= 0 || version < 0 || r == nil || r.provider == nil || r.executionScope == nil {
		return entity.ErrHookStoreCorrupt
	}
	if v := reflect.ValueOf(r.provider); v.Kind() == reflect.Ptr && v.IsNil() {
		return entity.ErrHookExecutionStorage
	}
	scope, err := r.executionScope(ctx)
	if err != nil || !hookGateASCII(scope) {
		return entity.ErrHookExecutionStorage
	}
	err = r.provider.NewSession(ctx, db.WithMaster()).Session(&gorm.Session{Logger: logger.Discard}).Transaction(func(tx *gorm.DB) error {
		// Lock before any snapshot read: finalization and archival hold this same original-Run row.
		var life model.ExptLifecycleRun
		if err := hookRunScope(tx, k).Clauses(clause.Locking{Strength: "UPDATE"}).First(&life).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return entity.ErrHookStoreConflict
			}
			return err
		}
		if life.ExecutionScope != scope {
			return entity.ErrHookStoreConflict
		}
		if r.binding != nil {
			if err := checkBoundFinalizationRun(tx, k, scope, r.binding); err != nil {
				return err
			}
			var row model.ExptLifecycleRunItem
			if err := hookRunScope(tx, k).Where("item_id=?", itemID).First(&row).Error; err != nil {
				return err
			}
			if err := checkBoundFinalizationPage(tx, k, []model.ExptLifecycleRunItem{row}, r.binding, true); err != nil {
				return err
			}
		}
		if life.FinalizeState < 0 || life.FinalizeState > 2 {
			return entity.ErrHookStoreCorrupt
		}
		var count int64
		if err := hookRunScope(tx.Model(&model.ExptLifecycleRunItem{}), k).Where("item_id=? AND item_version_id=? AND admitted_at IS NOT NULL", itemID, version).Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return entity.ErrHookStoreConflict
		}
		return fn(tx, &life)
	})
	for _, safe := range []error{entity.ErrHookStoreMissing, entity.ErrHookStoreConflict, entity.ErrHookStoreCorrupt, context.Canceled, context.DeadlineExceeded} {
		if errors.Is(err, safe) {
			return safe
		}
	}
	if err != nil {
		return entity.ErrHookExecutionStorage
	}
	return nil
}

func hookProgressRow(tx *gorm.DB, k entity.HookTurnProgressKey) *gorm.DB {
	return tx.Model(&model.ExptTurnResultRunLog{}).Where("id=? AND space_id=? AND expt_id=? AND expt_run_id=? AND item_id=? AND item_version_id=? AND turn_id=?", k.LogID, k.WorkspaceID, k.ExperimentID, k.RunID, k.ItemID, k.ItemVersionID, k.TurnID)
}

func readHookProgress(tx *gorm.DB, k entity.HookTurnProgressKey, lock bool) (*entity.ExptTurnResultRunLog, error) {
	q := hookProgressRow(tx, k)
	if lock {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var po model.ExptTurnResultRunLog
	if err := q.First(&po).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, entity.ErrHookStoreMissing
		}
		return nil, err
	}
	row, err := convert.NewExptTurnResultRunLogConvertor().PO2DO(&po)
	if err != nil {
		return nil, entity.ErrHookStoreCorrupt
	}
	return row, nil
}

func (r *hookTurnProgressRepo) ReadTurnProgress(ctx context.Context, k entity.HookTurnProgressKey) (out *entity.ExptTurnResultRunLog, err error) {
	err = r.transaction(ctx, k, func(tx *gorm.DB, _ *model.ExptLifecycleRun) error {
		var e error
		out, e = readHookProgress(tx, k, false)
		return e
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (r *hookTurnProgressRepo) WriteTurnProgress(ctx context.Context, in entity.HookTurnProgressInput) (out *entity.ExptTurnResultRunLog, err error) {
	return r.writeTurn(ctx, in, false)
}

func (r *hookTurnProgressRepo) WriteTurnResult(ctx context.Context, in entity.HookTurnProgressInput) (*entity.ExptTurnResultRunLog, error) {
	return r.writeTurn(ctx, in, true)
}

func (r *hookTurnProgressRepo) writeTurn(ctx context.Context, in entity.HookTurnProgressInput, fullResult bool) (out *entity.ExptTurnResultRunLog, err error) {
	k := entity.HookTurnProgressIdentity(in.Base)
	if k.Validate() != nil || k != entity.HookTurnProgressIdentity(in.Progress) {
		return nil, entity.ErrHookStoreCorrupt
	}
	status := in.Progress.Status
	if status != entity.TurnRunState_Processing && status != entity.TurnRunState_Fail && !(fullResult && (status == entity.TurnRunState_Success || (status == entity.TurnRunState_Terminal && in.Base.Status == entity.TurnRunState_Terminal))) {
		return nil, entity.ErrHookStoreCorrupt
	}
	err = r.transaction(ctx, k, func(tx *gorm.DB, life *model.ExptLifecycleRun) error {
		if life.FinalizeState == 2 {
			return entity.ErrHookStoreConflict
		}
		item, e := lockHookProgressItem(tx, k.HookRunKey, k.ItemID, k.ItemVersionID)
		if e != nil && (life.FinalizeState != 0 || !errors.Is(e, entity.ErrHookStoreMissing)) {
			return e
		}
		current, e := readHookProgress(tx, k, true)
		if e != nil {
			return e
		}
		originalRefs, e := hookProgressRefs(current.EvaluatorResultIds)
		if e != nil {
			return e
		}
		baseRefs, currentRefs, nextRefs := in.Base.EvaluatorResultIds, current.EvaluatorResultIds, in.Progress.EvaluatorResultIds
		if in.Progress.TargetResultID > 0 && in.Progress.TargetResultID != in.Base.TargetResultID {
			oldRefs := currentRefs
			if current.TargetResultID == in.Progress.TargetResultID {
				oldRefs = nil // A replay may merge other results already saved for the new target.
			} else {
				currentRefs = nil
			}
			nextRefs, e = freshHookProgressRefs(baseRefs, oldRefs, nextRefs)
			if e != nil {
				return e
			}
			baseRefs = nil
		}
		refs, changedRefs, e := mergeHookProgressRefs(baseRefs, currentRefs, nextRefs)
		if e != nil {
			return e
		}
		if (changedRefs || fullResult) && current.TargetResultID != in.Base.TargetResultID && in.Progress.TargetResultID != current.TargetResultID {
			return entity.ErrHookStoreConflict
		}
		target := current.TargetResultID
		if next := in.Progress.TargetResultID; next > 0 && next != in.Base.TargetResultID {
			if target != in.Base.TargetResultID && target != next {
				return entity.ErrHookStoreConflict
			}
			target = next
		}
		mergedRefs, e := hookProgressRefs(refs)
		if e != nil {
			return e
		}
		effective := target != current.TargetResultID || !maps.Equal(originalRefs, mergedRefs)
		raw, e := json.Marshal(refs)
		if e != nil {
			return entity.ErrHookStoreCorrupt
		}
		fields := map[string]any{"target_result_id": target, "evaluator_result_ids": raw, "updated_at": time.Now()}
		if fullResult {
			ext := maps.Clone(current.Ext)
			if ext == nil {
				ext = make(map[string]string)
			}
			for key, value := range in.Progress.Ext {
				baseValue, baseHas := in.Base.Ext[key]
				if baseHas && value == baseValue {
					continue
				}
				currentValue, currentHas := current.Ext[key]
				if (currentHas != baseHas || currentValue != baseValue) && (!currentHas || currentValue != value) {
					return entity.ErrHookStoreConflict
				}
				ext[key] = value
			}
			rawExt, e := json.Marshal(ext)
			if e != nil {
				return entity.ErrHookStoreCorrupt
			}
			effective = effective || !maps.Equal(current.Ext, ext)
			fields["ext"], current.Ext = rawExt, ext
		}
		if current.Status != entity.TurnRunState_Terminal {
			effective = effective || current.Status != in.Progress.Status || current.ErrMsg != in.Progress.ErrMsg
			current.Status, current.ErrMsg = in.Progress.Status, in.Progress.ErrMsg
			fields["status"], fields["err_msg"] = int32(current.Status), []byte(current.ErrMsg)
		}
		if hookProgressNormalFrozen(life, item) && effective {
			return entity.ErrHookStoreConflict
		}
		if (life.FinalizeState == 1 || hookProgressNormalFrozen(life, item)) && !effective {
			out = current
			return nil
		}
		if e = hookProgressRow(tx, k).Updates(fields).Error; e != nil {
			return e
		}
		if hookCancellation(life) && hookProgressItemResulted(item) {
			if e = hookProgressItemRow(tx, k.HookRunKey, k.ItemID, k.ItemVersionID).Where("id=?", item.ID).UpdateColumn("result_state", int32(entity.ExptItemResultStateLogged)).Error; e != nil {
				return e
			}
		}
		current.TargetResultID, current.EvaluatorResultIds = target, refs
		out = current
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

type hookProgressRefKey struct {
	version       int64
	alias, inline string
}

// Replacing a target severs old score associations, not the score records themselves.
func freshHookProgressRefs(base, current, next *entity.EvaluatorResults) (*entity.EvaluatorResults, error) {
	oldIDs := make(map[int64]bool)
	for _, refs := range []*entity.EvaluatorResults{base, current} {
		values, err := hookProgressRefs(refs)
		if err != nil {
			return nil, err
		}
		for _, id := range values {
			oldIDs[id] = true
		}
	}
	if _, err := hookProgressRefs(next); err != nil {
		return nil, err
	}
	out := &entity.EvaluatorResults{}
	if next == nil {
		return out, nil
	}
	if next.EvalVerIDToResID != nil {
		out.EvalVerIDToResID = make(map[int64]int64)
		for version, id := range next.EvalVerIDToResID {
			if !oldIDs[id] {
				out.EvalVerIDToResID[version] = id
			}
		}
	}
	for _, ref := range next.Registered {
		if !oldIDs[ref.RecordID] {
			out.Registered = append(out.Registered, ref)
		}
	}
	for _, ref := range next.Inline {
		if !oldIDs[ref.RecordID] {
			out.Inline = append(out.Inline, ref)
		}
	}
	return out, nil
}

func hookProgressRefs(refs *entity.EvaluatorResults) (map[hookProgressRefKey]int64, error) {
	out := make(map[hookProgressRefKey]int64)
	add := func(k hookProgressRefKey, id int64) error {
		if id <= 0 || (k.inline == "" && k.version <= 0) {
			return entity.ErrHookStoreCorrupt
		}
		if old, ok := out[k]; ok && old != id {
			return entity.ErrHookStoreCorrupt
		}
		out[k] = id
		return nil
	}
	if refs == nil {
		return out, nil
	}
	for version, id := range refs.EvalVerIDToResID {
		if err := add(hookProgressRefKey{version: version}, id); err != nil {
			return nil, err
		}
	}
	for _, ref := range refs.Registered {
		if ref == nil {
			return nil, entity.ErrHookStoreCorrupt
		}
		if err := add(hookProgressRefKey{version: ref.VersionID, alias: ref.Alias}, ref.RecordID); err != nil {
			return nil, err
		}
	}
	for _, ref := range refs.Inline {
		if ref == nil || ref.InlineKey == "" {
			return nil, entity.ErrHookStoreCorrupt
		}
		if err := add(hookProgressRefKey{inline: ref.InlineKey}, ref.RecordID); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func mergeHookProgressRefs(base, current, next *entity.EvaluatorResults) (*entity.EvaluatorResults, bool, error) {
	b, err := hookProgressRefs(base)
	if err != nil {
		return nil, false, err
	}
	c, err := hookProgressRefs(current)
	if err != nil {
		return nil, false, err
	}
	n, err := hookProgressRefs(next)
	if err != nil {
		return nil, false, err
	}
	changed := false
	for k, id := range n {
		if id == b[k] {
			continue
		}
		if c[k] != b[k] && c[k] != id {
			return nil, false, entity.ErrHookStoreConflict
		}
		changed = true
		c[k] = id
	}
	out := &entity.EvaluatorResults{}
	if (current != nil && current.EvalVerIDToResID != nil) || (next != nil && next.EvalVerIDToResID != nil) {
		out.EvalVerIDToResID = make(map[int64]int64)
	}
	for k, id := range c {
		if k.inline != "" {
			out.Inline = append(out.Inline, &entity.InlineEvalResult{InlineKey: k.inline, RecordID: id})
			continue
		}
		out.Registered = append(out.Registered, &entity.RegisteredEvalResult{VersionID: k.version, Alias: k.alias, RecordID: id})
		if out.EvalVerIDToResID != nil && k.alias == "" {
			out.EvalVerIDToResID[k.version] = id
		}
	}
	return out, changed, nil
}
