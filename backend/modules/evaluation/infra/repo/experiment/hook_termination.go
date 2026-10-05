// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"reflect"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

func hookCancellation(life *model.ExptLifecycleRun) bool {
	return life.FinalizeState != 0 && (gptr.Indirect(life.TerminalStatus) == int32(entity.ExptStatus_Terminated) || gptr.Indirect(life.TerminalStatus) == int32(entity.ExptStatus_SystemTerminated))
}

func validateActiveTermination(life *model.ExptLifecycleRun, expt *model.Experiment, mode entity.ExptRunMode, bindings ...*boundHookExecution) error {
	if !hookCancellation(life) || life.Gate != 2 || !life.ExecutionInitialized || !(entity.HookExecutionInitializationRequired(life.BeforeEnabled || life.AfterEnabled, mode, entity.ExptType(expt.ExptType), entity.ExptEvalSetSourceType(expt.EvalSetSourceType)) || boundFinalizationRequired(firstFinalizationBinding(bindings), life, mode)) {
		return entity.ErrHookFinalizationUnsettled
	}
	if life.PlanState != 1 || life.PlanCount < 0 || life.PlanCount > math.MaxInt32 {
		return entity.ErrHookStoreCorrupt
	}
	return nil
}

func hookTerminationManifest(key entity.HookRunKey, row model.ExptLifecycleRunItem) (entity.HookExecutionManifest, error) {
	var m entity.HookExecutionManifest
	if row.ExecutionManifest == nil {
		return m, entity.ErrHookStoreCorrupt
	}
	d := json.NewDecoder(bytes.NewReader(*row.ExecutionManifest))
	d.DisallowUnknownFields()
	if d.Decode(&m) != nil || d.Decode(new(any)) != io.EOF || m.Validate() != nil || m.Key != key || m.Ordinal != row.Ordinal || m.Frozen != executionTuple(row) {
		return m, entity.ErrHookStoreCorrupt
	}
	return m, nil
}

func (r *hookFinalizationRepo) ReadTerminationPage(ctx context.Context, in entity.HookPlanReadInput) (out *entity.HookExecutionInitializationPage, err error) {
	if (entity.HookStoreGuard{Key: in.Key}).Validate() != nil || in.StartOrdinal < 0 || in.Limit < 1 || in.Limit > 100 || !hookGateASCII(in.ExecutionScope) {
		return nil, entity.ErrHookStoreCorrupt
	}
	err = r.read(ctx, func(tx *gorm.DB) error {
		if err := checkBoundFinalizationRun(tx, in.Key, in.ExecutionScope, r.binding); err != nil {
			return err
		}
		var life model.ExptLifecycleRun
		if err := hookRunScope(tx, in.Key).First(&life).Error; err != nil {
			return err
		}
		expt, err := readHookRecoveryExperiment(tx, in.Key)
		if err != nil {
			return err
		}
		var log model.ExptRunLog
		if err := hookRunScope(tx, in.Key).First(&log).Error; err != nil {
			return err
		}
		if life.ExecutionScope != in.ExecutionScope {
			return entity.ErrHookStoreConflict
		}
		if err := validateActiveTermination(&life, expt, entity.ExptRunMode(gptr.Indirect(log.Mode)), r.binding); err != nil {
			return err
		}
		if in.StartOrdinal > life.PlanCount {
			return entity.ErrHookStoreCorrupt
		}
		var rows []model.ExptLifecycleRunItem
		if err := hookRunScope(tx, in.Key).Where("ordinal>=?", in.StartOrdinal).Order("ordinal").Limit(int(in.Limit)).Find(&rows).Error; err != nil {
			return err
		}
		if err := checkBoundFinalizationPage(tx, in.Key, rows, r.binding, false); err != nil {
			return err
		}
		out = &entity.HookExecutionInitializationPage{Count: life.PlanCount, Hash: gptr.Indirect(life.PlanHash), RunVersion: life.Version, Initialized: true, NextOrdinal: in.StartOrdinal + int64(len(rows))}
		if out.NextOrdinal > out.Count || len(rows) == 0 && in.StartOrdinal < out.Count {
			return entity.ErrHookStoreCorrupt
		}
		for i, row := range rows {
			m, err := hookTerminationManifest(in.Key, row)
			if err != nil {
				return err
			}
			if row.Ordinal != in.StartOrdinal+int64(i) {
				return entity.ErrHookStoreCorrupt
			}
			out.Items = append(out.Items, entity.HookExecutionInitializationItem{Ordinal: row.Ordinal, Frozen: m.Frozen, Manifest: &m})
		}
		out.HasMore = out.NextOrdinal < out.Count
		return nil
	})
	return out, err
}

func (r *hookFinalizationRepo) itemArchiveTransaction(ctx context.Context, key entity.HookRunKey, scope string, fn func(*gorm.DB, *model.Experiment, *model.ExptLifecycleRun) error) error {
	return r.itemArchiveRecoveryTransaction(ctx, key, scope, false, fn)
}

func (r *hookFinalizationRepo) itemArchiveRecoveryTransaction(ctx context.Context, key entity.HookRunKey, scope string, recovery bool, fn func(*gorm.DB, *model.Experiment, *model.ExptLifecycleRun) error) error {
	if ctx == nil || r == nil || r.provider == nil || (entity.HookStoreGuard{Key: key}).Validate() != nil || !hookGateASCII(scope) {
		return entity.ErrHookStoreCorrupt
	}
	return r.provider.NewSession(ctx, db.WithMaster()).Session(&gorm.Session{Logger: logger.Discard}).Transaction(func(tx *gorm.DB) error {
		expt, err := lockHookExperiment(tx, key)
		if err != nil {
			return err
		}
		if expt.DeletedAt.Valid && !recovery {
			return entity.ErrHookStoreMissing
		}
		run, err := loadHookRun(tx, key)
		if err != nil {
			return err
		}
		if run.life.ExecutionScope != scope {
			return entity.ErrHookStoreConflict
		}
		if err := checkBoundFinalizationRun(tx, key, scope, r.binding); err != nil {
			return err
		}
		if expt.DeletedAt.Valid && (run.life.FinalizeState != 1 || run.life.Gate != 2) {
			return entity.ErrHookStoreMissing
		}
		return fn(tx, expt, &run.life)
	})
}

// Read and lock the exact frozen item; dispatch/admission can precede lazy turnlog creation.
func readHookArchiveItem(tx *gorm.DB, key entity.HookRunKey, itemID int64) (*entity.HookTerminationItem, *model.ExptLifecycleRunItem, error) {
	return readHookArchiveItemRows(tx, key, itemID, true)
}

func readHookArchiveItemRows(tx *gorm.DB, key entity.HookRunKey, itemID int64, locking bool) (*entity.HookTerminationItem, *model.ExptLifecycleRunItem, error) {
	var row model.ExptLifecycleRunItem
	ledgerQuery := hookRunScope(tx, key).Where("item_id=?", itemID)
	if locking {
		ledgerQuery = ledgerQuery.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := ledgerQuery.First(&row).Error; err != nil {
		return nil, nil, err
	}
	m, err := hookTerminationManifest(key, row)
	if err != nil {
		return nil, nil, err
	}
	storedItem, turns, err := readIndexedHookExecutionRows(tx, key, m, locking)
	if err != nil {
		return nil, nil, err
	}
	item := convert.NewExptItemResultRunLogConverter().PO2DO(storedItem)
	out := &entity.HookTerminationItem{Manifest: m, Item: item}
	expected := map[int64]bool{}
	for _, t := range m.Turns {
		expected[t.TurnID] = true
	}
	for _, t := range turns {
		if !expected[t.TurnID] || t.ID <= 0 || t.SpaceID != key.WorkspaceID || t.ExptID != key.ExperimentID || t.ItemVersionID != m.Frozen.ItemVersionID || t.DeletedAt.Valid {
			return nil, nil, entity.ErrHookStoreCorrupt
		}
		delete(expected, t.TurnID)
		tr, err := convert.NewExptTurnResultRunLogConvertor().PO2DO(&t)
		if err != nil {
			return nil, nil, err
		}
		if _, err = hookProgressRefs(tr.EvaluatorResultIds); err != nil {
			return nil, nil, err
		}
		out.Turns = append(out.Turns, tr)
	}
	if m.NoExecutionFailure && !hookNoExecutionFailure(out) {
		return nil, nil, entity.ErrHookStoreCorrupt
	}
	if len(turns) == 0 {
		switch entity.ItemRunState(item.Status) {
		case entity.ItemRunState_Queueing, entity.ItemRunState_Processing, entity.ItemRunState_Terminal:
		case entity.ItemRunState_Fail:
			if !hookNoExecutionFailure(out) {
				return nil, nil, entity.ErrHookStoreCorrupt
			}
		default:
			return nil, nil, entity.ErrHookStoreCorrupt
		}
	} else if row.AdmittedAt == nil || len(expected) != 0 {
		return nil, nil, entity.ErrHookStoreCorrupt
	}
	return out, &row, nil
}

func (r *hookFinalizationRepo) PrepareTerminationItem(ctx context.Context, key entity.HookRunKey, scope string, itemID int64) (out *entity.HookTerminationItem, err error) {
	err = r.itemArchiveRecoveryTransaction(ctx, key, scope, true, func(tx *gorm.DB, expt *model.Experiment, life *model.ExptLifecycleRun) error {
		var log model.ExptRunLog
		if err := hookRunScope(tx, key).First(&log).Error; err != nil {
			return err
		}
		if err := validateActiveTermination(life, expt, entity.ExptRunMode(gptr.Indirect(log.Mode)), r.binding); err != nil {
			return err
		}
		current, _, err := r.readArchiveItem(tx, key, itemID)
		if err != nil {
			return err
		}
		message := errno.SerializeErr(errno.NewItemManuallyTerminatedErr())
		for _, tr := range current.Turns {
			switch tr.Status {
			case entity.TurnRunState_Queueing, entity.TurnRunState_Processing:
				if err := hookRunScope(tx.Model(&model.ExptTurnResultRunLog{}), key).Where("id=?", tr.ID).UpdateColumns(map[string]any{"status": int32(entity.TurnRunState_Terminal), "err_msg": []byte(message)}).Error; err != nil {
					return err
				}
				tr.Status, tr.ErrMsg = entity.TurnRunState_Terminal, message
			case entity.TurnRunState_Success, entity.TurnRunState_Fail, entity.TurnRunState_Terminal:
			default:
				return entity.ErrHookStoreCorrupt
			}
		}
		switch entity.ItemRunState(current.Item.Status) {
		case entity.ItemRunState_Queueing, entity.ItemRunState_Processing:
			if err := hookRunScope(tx.Model(&model.ExptItemResultRunLog{}), key).Where("id=?", current.Item.ID).UpdateColumns(map[string]any{"status": int32(entity.ItemRunState_Terminal), "err_msg": []byte(message)}).Error; err != nil {
				return err
			}
			current.Item.Status, current.Item.ErrMsg = int32(entity.ItemRunState_Terminal), []byte(message)
		case entity.ItemRunState_Success, entity.ItemRunState_Fail, entity.ItemRunState_Terminal:
		default:
			return entity.ErrHookStoreCorrupt
		}
		out, _, err = r.readArchiveItem(tx, key, itemID)
		return err
	})
	return out, err
}

func (r *hookFinalizationRepo) ReadHookArchiveItem(ctx context.Context, key entity.HookRunKey, scope string, itemID int64) (out *entity.HookTerminationItem, err error) {
	err = r.itemArchiveTransaction(ctx, key, scope, func(tx *gorm.DB, expt *model.Experiment, life *model.ExptLifecycleRun) error {
		if life.Gate != 1 || life.FinalizeState != 0 || expt.LatestRunID != key.RunID {
			return entity.ErrHookAdmissionDenied
		}
		var err error
		out, _, err = r.readArchiveItem(tx, key, itemID)
		return err
	})
	return out, err
}

func (r *hookFinalizationRepo) ArchiveHookItem(ctx context.Context, in entity.HookItemArchiveInput) (refs []*entity.ExptTurnEvaluatorResultRef, err error) {
	err = r.itemArchiveRecoveryTransaction(ctx, in.Key, in.ExecutionScope, in.Cancellation, func(tx *gorm.DB, expt *model.Experiment, life *model.ExptLifecycleRun) error {
		cancellation := hookCancellation(life)
		if cancellation != in.Cancellation || cancellation && in.Prepared == nil || !cancellation && (life.Gate != 1 || life.FinalizeState != 0 || expt.LatestRunID != in.Key.RunID) {
			return entity.ErrHookAdmissionDenied
		}
		current, _, err := r.readArchiveItem(tx, in.Key, in.ItemID)
		if err != nil {
			return err
		}
		if in.Prepared != nil && (!reflect.DeepEqual(current.Manifest, in.Prepared.Manifest) || !reflect.DeepEqual(current.Turns, in.Prepared.Turns) || !reflect.DeepEqual(current.Item, in.Prepared.Item)) {
			return entity.ErrHookStoreConflict
		}
		if !entity.IsItemRunFinished(entity.ItemRunState(current.Item.Status)) {
			return entity.ErrHookFinalizationUnsettled
		}
		for _, tr := range current.Turns {
			if tr.Status != entity.TurnRunState_Success && tr.Status != entity.TurnRunState_Fail && tr.Status != entity.TurnRunState_Terminal {
				return entity.ErrHookFinalizationUnsettled
			}
		}
		if expt.LatestRunID == in.Key.RunID {
			if err := materializeHookItem(tx, in, current, cancellation); err != nil {
				return err
			}
		}
		if err := hookRunScope(tx.Model(&model.ExptItemResultRunLog{}), in.Key).Where("id=?", current.Item.ID).UpdateColumn("result_state", int32(entity.ExptItemResultStateResulted)).Error; err != nil {
			return err
		}
		refs = in.Refs
		return nil
	})
	return refs, err
}

func materializeHookItem(tx *gorm.DB, in entity.HookItemArchiveInput, current *entity.HookTerminationItem, cancellation bool) error {
	m := current.Manifest
	noExecutionFailure := hookNoExecutionFailure(current)
	if noExecutionFailure && (len(in.Refs) != 0 || len(in.Scores) != 0) {
		return entity.ErrHookStoreCorrupt
	}
	var item model.ExptItemResult
	if err := tx.Unscoped().Where("id=?", m.ItemResultID).Clauses(clause.Locking{Strength: "UPDATE"}).First(&item).Error; err != nil {
		return err
	}
	if item.DeletedAt.Valid || item.SpaceID != in.Key.WorkspaceID || item.ExptID != in.Key.ExperimentID || item.ExptRunID != in.Key.RunID || item.ItemID != m.Frozen.ItemID || item.ItemVersionID != m.Frozen.ItemVersionID || item.ItemIdx == nil || *item.ItemIdx != int32(m.ProjectionOrdinal()) {
		return entity.ErrHookStoreCorrupt
	}
	if noExecutionFailure && item.Status != int32(entity.ItemRunState_Queueing) && item.Status != int32(entity.ItemRunState_Processing) && !(item.Status == int32(entity.ItemRunState_Fail) && bytes.Equal(gptr.Indirect(item.ErrMsg), current.Item.ErrMsg)) {
		return entity.ErrHookStoreCorrupt
	}
	byTurn := map[int64]*entity.ExptTurnResultRunLog{}
	for _, tr := range current.Turns {
		byTurn[tr.TurnID] = tr
	}
	for _, mt := range m.Turns {
		var tr model.ExptTurnResult
		if err := tx.Unscoped().Where("id=?", mt.ResultID).Clauses(clause.Locking{Strength: "UPDATE"}).First(&tr).Error; err != nil {
			return err
		}
		if tr.DeletedAt.Valid || tr.SpaceID != in.Key.WorkspaceID || tr.ExptID != in.Key.ExperimentID || tr.ExptRunID != in.Key.RunID || tr.ItemID != m.Frozen.ItemID || tr.ItemVersionID != m.Frozen.ItemVersionID || tr.TurnID != mt.TurnID || tr.TurnIdx == nil || *tr.TurnIdx != mt.TurnIdx {
			return entity.ErrHookStoreCorrupt
		}
		fields := map[string]any{}
		if log := byTurn[mt.TurnID]; log != nil {
			fields["status"], fields["target_result_id"], fields["log_id"], fields["err_msg"] = int32(log.Status), log.TargetResultID, log.LogID, []byte(log.ErrMsg)
		} else if noExecutionFailure {
			if tr.TargetResultID != 0 || tr.WeightedScore != nil || (tr.Status != int32(entity.TurnRunState_Queueing) && tr.Status != int32(entity.TurnRunState_Processing) && !(tr.Status == int32(entity.TurnRunState_Fail) && bytes.Equal(gptr.Indirect(tr.ErrMsg), current.Item.ErrMsg))) {
				return entity.ErrHookStoreCorrupt
			}
			fields["status"], fields["err_msg"] = int32(entity.TurnRunState_Fail), []byte(current.Item.ErrMsg)
		} else if cancellation {
			fields["status"], fields["err_msg"] = int32(entity.TurnRunState_Terminal), []byte(current.Item.ErrMsg)
		} else {
			return entity.ErrHookStoreCorrupt
		}
		if score, ok := in.Scores[mt.ResultID]; ok {
			fields["weighted_score"] = score
		}
		if err := hookRunScope(tx.Model(&model.ExptTurnResult{}), in.Key).Where("id=?", mt.ResultID).UpdateColumns(fields).Error; err != nil {
			return err
		}
	}
	if err := materializeHookReferences(tx, in, current); err != nil {
		return err
	}
	if err := hookRunScope(tx.Model(&model.ExptItemResult{}), in.Key).Where("id=?", m.ItemResultID).UpdateColumns(map[string]any{"status": current.Item.Status, "log_id": current.Item.LogID, "err_msg": []byte(current.Item.ErrMsg)}).Error; err != nil {
		return err
	}
	if !cancellation && item.Status != current.Item.Status {
		columns := map[int32]string{int32(entity.ItemRunState_Queueing): "pending_cnt", int32(entity.ItemRunState_Processing): "processing_cnt", int32(entity.ItemRunState_Success): "success_cnt", int32(entity.ItemRunState_Fail): "fail_cnt", int32(entity.ItemRunState_Terminal): "terminated_cnt"}
		old, next := columns[item.Status], columns[current.Item.Status]
		if old == "" || next == "" {
			return entity.ErrHookStoreCorrupt
		}
		if err := tx.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", in.Key.WorkspaceID, in.Key.ExperimentID).UpdateColumns(map[string]any{old: gorm.Expr(old + " - 1"), next: gorm.Expr(next + " + 1")}).Error; err != nil {
			return err
		}
	}
	if noExecutionFailure {
		return validateHookTerminationProjection(tx, in.Key, current)
	}
	return nil
}

// Only the transactional manifest proof permits a failed plan without executed turns.
func hookNoExecutionFailure(item *entity.HookTerminationItem) bool {
	return item != nil && item.Item != nil && item.Manifest.NoExecutionFailure && len(item.Turns) == 0 && item.Item.Status == int32(entity.ItemRunState_Fail) && len(item.Item.ErrMsg) > 0 && (item.Item.ResultState == int32(entity.ExptItemResultStateLogged) || item.Item.ResultState == int32(entity.ExptItemResultStateResulted))
}

func readIndexedHookExecutionRows(tx *gorm.DB, key entity.HookRunKey, m entity.HookExecutionManifest, locking bool) (*model.ExptItemResultRunLog, []model.ExptTurnResultRunLog, error) {
	lockRow := func(q *gorm.DB) *gorm.DB {
		if locking {
			return q.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		return q
	}
	var item model.ExptItemResultRunLog
	if err := lockRow(tx.Unscoped().Where("id=?", m.ItemRunLogID)).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, entity.ErrHookStoreCorrupt
		}
		return nil, nil, err
	}
	if item.SpaceID != key.WorkspaceID || item.ExptID != key.ExperimentID || item.ExptRunID != key.RunID || item.ItemID != m.Frozen.ItemID || item.ItemVersionID != m.Frozen.ItemVersionID || item.DeletedAt.Valid {
		return nil, nil, entity.ErrHookStoreCorrupt
	}
	// Current reads use the original item index range, never an unbounded cross-tenant lock.
	var scoped []model.ExptTurnResultRunLog
	if err := lockRow(hookRunScope(tx.Unscoped(), key).Where("item_id=?", m.Frozen.ItemID).Order("turn_id").Limit(len(m.Turns) + 1)).Find(&scoped).Error; err != nil {
		return nil, nil, err
	}
	if m.TurnLogsInitialized != nil && !*m.TurnLogsInitialized {
		if len(scoped) != 0 {
			return nil, nil, entity.ErrHookStoreCorrupt
		}
		return &item, nil, nil
	}
	if m.TurnLogsInitialized == nil && len(scoped) == 0 && m.NoExecutionFailure {
		return &item, nil, nil
	}
	// Pre-pin snapshots need a complete actual set; absence cannot establish never-executed.
	if m.TurnLogsInitialized == nil && len(scoped) != len(m.Turns) {
		return nil, nil, entity.ErrHookStoreCorrupt
	}
	ids := make([]int64, 0, len(m.Turns))
	if m.TurnLogsInitialized != nil {
		for _, mt := range m.Turns {
			ids = append(ids, mt.RunLogID)
		}
	} else {
		for _, row := range scoped {
			ids = append(ids, row.ID)
		}
	}
	var rows []model.ExptTurnResultRunLog
	if err := lockRow(tx.Unscoped().Where("id IN ?", ids).Order("turn_id")).Find(&rows).Error; err != nil {
		return nil, nil, err
	}
	if len(rows) != len(m.Turns) || len(scoped) != len(m.Turns) {
		return nil, nil, entity.ErrHookStoreCorrupt
	}
	expected := map[int64]entity.HookExecutionTurnManifest{}
	set := map[int64]bool{}
	for _, mt := range m.Turns {
		expected[mt.TurnID] = mt
	}
	for _, row := range scoped {
		set[row.ID] = true
	}
	for _, row := range rows {
		mt, ok := expected[row.TurnID]
		if !ok || !set[row.ID] || row.SpaceID != key.WorkspaceID || row.ExptID != key.ExperimentID || row.ExptRunID != key.RunID || row.ItemID != m.Frozen.ItemID || row.ItemVersionID != m.Frozen.ItemVersionID || row.DeletedAt.Valid || m.TurnLogsInitialized != nil && mt.RunLogID != row.ID {
			return nil, nil, entity.ErrHookStoreCorrupt
		}
		delete(expected, row.TurnID)
	}
	return &item, rows, nil
}
