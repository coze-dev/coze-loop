// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/bytedance/gg/gptr"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	looplogs "github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// currentSourceInconsistentError marks a deterministic inconsistency in one
// item's frozen snapshot (its item row, canonical turns, turn run logs or
// frozen publication references). Scans isolate just that item; execution and
// publication still fail closed on the same error.
//
// Run lifecycle/configuration observations (missing or invalid run status,
// invalid run mode) and execution-state observations (unknown run log status,
// invalid item result state) deliberately stay untyped: they describe the run
// or the caller, not one item's snapshot, and must keep failing the whole scan.
type currentSourceInconsistentError struct{ message string }

func (e *currentSourceInconsistentError) Error() string { return e.message }

func inconsistentCurrentSource(format string, args ...any) error {
	return &currentSourceInconsistentError{message: fmt.Sprintf(format, args...)}
}

func isCurrentSourceInconsistent(err error) bool {
	var target *currentSourceInconsistentError
	return errors.As(err, &target)
}

// itemVersionConflict reports a row version that contradicts the authoritative
// item version. Zero means "legacy/unversioned" in every result table and is
// only readable as a legacy item too: an unversioned item cannot vouch for a
// foreign non-zero turn/log version, so that still fails closed.
func itemVersionConflict(itemVersion, rowVersion int64) bool {
	return rowVersion != 0 && rowVersion != itemVersion
}

// versionPairConflict reports two optional versions that disagree. Both sides
// carry no authority over the other, so a legacy zero simply means "unknown".
func versionPairConflict(a, b int64) bool {
	return a != 0 && b != 0 && a != b
}

func (dao *exptItemResultDAOImpl) GetCurrentItemRunSource(ctx context.Context, spaceID, exptID, runID, itemID int64) (*entity.ExptCurrentItemRunSource, error) {
	var result *entity.ExptCurrentItemRunSource
	err := dao.provider.Transaction(ctx, func(tx *gorm.DB) error {
		var err error
		result, err = loadCurrentRunSource(tx, spaceID, exptID, runID, itemID)
		return err
	}, db.WithMaster())
	return result, err
}

func loadCurrentRunSource(tx *gorm.DB, spaceID, exptID, runID, itemID int64) (*entity.ExptCurrentItemRunSource, error) {
	return loadItemRunSource(tx, spaceID, exptID, runID, itemID, false)
}

// Publication uses the same primary row/source validation, with a separate run
// lifecycle rule. Execution remains active-only through loadCurrentRunSource.
func loadItemRunSource(tx *gorm.DB, spaceID, exptID, runID, itemID int64, publication bool) (*entity.ExptCurrentItemRunSource, error) {
	return readItemRunSource(tx, spaceID, exptID, runID, itemID, publication, true)
}

// Scan observations use only consistent nonlocking reads. Execution and
// publication validation/commit keep the canonical-first locking path above.
func readItemRunSource(tx *gorm.DB, spaceID, exptID, runID, itemID int64, publication, lockRows bool) (*entity.ExptCurrentItemRunSource, error) {
	var locks []clause.Expression
	if lockRows {
		locks = append(locks, clause.Locking{Strength: "UPDATE"})
	}
	var item model.ExptItemResult
	err := tx.Clauses(locks...).Where("space_id=? AND expt_id=? AND item_id=?", spaceID, exptID, itemID).Take(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if item.ExptRunID != runID {
		return nil, nil
	}
	ext, err := entity.ParseExptItemResultExt(gptr.Indirect(item.Ext))
	if err != nil {
		return nil, err
	}
	manual, err := entity.ParseManualRetryContext(ext)
	if err != nil {
		return nil, err
	}
	request := ""
	if manual != nil {
		if manual.RunID != runID {
			return nil, inconsistentCurrentSource("manual current source run mismatch")
		}
		request = manual.RequestID
	}
	var run model.ExptRunLog
	if err := tx.Where("space_id=? AND expt_id=? AND expt_run_id=?", spaceID, exptID, runID).Take(&run).Error; err != nil {
		return nil, err
	}
	if run.Status == nil {
		return nil, fmt.Errorf("source run status missing")
	}
	runStatus := entity.ExptStatus(*run.Status)
	if !publication {
		if entity.IsExptFinished(runStatus) || entity.IsExptFinishing(runStatus) {
			return nil, nil
		}
		if runStatus != entity.ExptStatus_Processing && runStatus != entity.ExptStatus_Pending {
			return nil, fmt.Errorf("invalid source run status")
		}
	} else if runStatus != entity.ExptStatus_Processing && runStatus != entity.ExptStatus_Pending && !entity.IsExptFinished(runStatus) && !entity.IsExptFinishing(runStatus) {
		return nil, fmt.Errorf("invalid publication run status")
	}
	mode := entity.ExptRunMode(gptr.Indirect(run.Mode))
	if manual != nil {
		mode = manual.Mode
	}
	if mode < entity.EvaluationModeSubmit || mode > entity.EvaluationModeTrialRun {
		return nil, fmt.Errorf("invalid current source mode")
	}
	var log model.ExptItemResultRunLog
	if err := tx.Clauses(locks...).Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", spaceID, exptID, runID, itemID).Take(&log).Error; err != nil {
		return nil, err
	}
	if itemVersionConflict(item.ItemVersionID, log.ItemVersionID) || log.RetryTimes < 0 || !validCurrentItemStatus(entity.ItemRunState(item.Status)) || !validCurrentItemStatus(entity.ItemRunState(log.Status)) || log.ResultState == nil || *log.ResultState < 0 || *log.ResultState > int32(entity.ExptItemResultStateLogged) {
		return nil, inconsistentCurrentSource("damaged current item source")
	}
	source := &entity.ExptCurrentItemRunSource{Current: entity.ExptItemCurrentRequest{SpaceID: spaceID, ExptID: exptID, RunID: runID, ItemID: itemID, ItemVersionID: item.ItemVersionID, RequestID: request, Mode: mode, Status: entity.ItemRunState(log.Status), ResultState: entity.ExptItemResultState(*log.ResultState), RetryTimes: log.RetryTimes, UpdatedAt: gptr.Of(log.UpdatedAt)}, Turns: map[int64]*entity.ExptTurnResultRunLog{}}
	if entity.IsItemRunFinished(entity.ItemRunState(item.Status)) && item.Status != log.Status {
		source.Current.Status = entity.ItemRunState(item.Status)
	}
	var logs []model.ExptTurnResultRunLog
	if err := tx.Clauses(locks...).Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", spaceID, exptID, runID, itemID).Order("id ASC").Find(&logs).Error; err != nil {
		return nil, err
	}
	logTurns := make(map[int64]struct{}, len(logs))
	for index := range logs {
		logTurns[logs[index].TurnID] = struct{}{}
	}
	var canonical []model.ExptTurnResult
	canonicalQuery := tx.Clauses(locks...).Where("space_id=? AND expt_id=? AND item_id=?", spaceID, exptID, itemID)
	if mode != entity.EvaluationModeFailRetry {
		canonicalQuery = canonicalQuery.Where("expt_run_id=?", runID)
	}
	// FailRetry keeps successful canonical turns in their original run until publication.
	if err := canonicalQuery.Order("id ASC").Find(&canonical).Error; err != nil {
		return nil, err
	}
	canonicalIDs := make([]int64, 0, len(canonical))
	byTurn := map[int64]model.ExptTurnResult{}
	for _, turn := range canonical {
		// Legacy rows are written with item_version_id=0 in every table. A
		// non-zero turn version must match the authoritative item version; an
		// unversioned item cannot vouch for one.
		if itemVersionConflict(item.ItemVersionID, turn.ItemVersionID) || turn.TurnID < 0 {
			return nil, inconsistentCurrentSource("damaged canonical turn source")
		}
		if turn.ExptRunID != runID {
			if mode != entity.EvaluationModeFailRetry {
				continue
			}
			_, retriedNow := logTurns[turn.TurnID]
			// Successful history legitimately stays in its old run. A retried
			// turn may still carry the old canonical row until the reset lands,
			// which is legal only when the current run already owns its turn log.
			// Without that log the frozen turn would silently disappear from
			// execution/publication, so isolate instead of dropping it.
			if entity.TurnRunState(turn.Status) != entity.TurnRunState_Success && !retriedNow {
				return nil, inconsistentCurrentSource("frozen canonical turn has no current run log")
			}
		}
		byTurn[turn.TurnID] = turn
		canonicalIDs = append(canonicalIDs, turn.ID)
		source.CanonicalTurns = append(source.CanonicalTurns, convert.NewExptTurnResultConvertor().PO2DO(&turn, nil))
	}
	for index := range logs {
		log := &logs[index]
		turn, ok := byTurn[log.TurnID]
		if !ok || log.TurnID < 0 || log.Status < int32(entity.TurnRunState_Queueing) || log.Status > int32(entity.TurnRunState_Terminal) {
			return nil, inconsistentCurrentSource("damaged current turn source")
		}
		if itemVersionConflict(item.ItemVersionID, log.ItemVersionID) {
			return nil, inconsistentCurrentSource("damaged current turn source")
		}
		if versionPairConflict(log.ItemVersionID, turn.ItemVersionID) {
			return nil, inconsistentCurrentSource("damaged current turn source")
		}
		converted, err := convert.NewExptTurnResultRunLogConvertor().PO2DO(log)
		if err != nil {
			return nil, err
		}
		if err := validateCurrentEvaluatorRefs(converted.EvaluatorResultIds); err != nil {
			// Stored evaluator references are part of this turn's frozen snapshot:
			// isolate the item instead of failing the whole run scan. The write
			// path still validates incoming references with the plain error.
			return nil, inconsistentCurrentSource("damaged current evaluator references: %v", err)
		}
		source.Turns[log.TurnID] = converted
	}
	// A failed first turn need not execute all later turns, but every frozen
	// canonical turn must own exactly one current-run log. Counting the filtered
	// snapshot alone would let a skipped frozen turn disappear silently, so check
	// both the row count and every canonical turn identity.
	if manual != nil {
		if len(canonical) == 0 || len(canonical) != len(logs) {
			return nil, inconsistentCurrentSource("current frozen turn logs incomplete")
		}
		for _, turn := range canonical {
			if _, ok := logTurns[turn.TurnID]; !ok {
				return nil, inconsistentCurrentSource("current frozen turn log missing")
			}
		}
	}
	var refs []model.ExptTurnEvaluatorResultRef
	if err := tx.Clauses(locks...).Where("space_id=? AND expt_id=? AND expt_turn_result_id IN ?", spaceID, exptID, canonicalIDs).Order("id ASC").Find(&refs).Error; err != nil {
		return nil, err
	}
	source.CurrentRefs = make([]*entity.ExptTurnEvaluatorResultRef, 0, len(refs))
	for index := range refs {
		source.CurrentRefs = append(source.CurrentRefs, convert.NewExptTurnEvaluatorResultRefConvertor().PO2DO(&refs[index]))
	}
	return source, nil
}

func validateCurrentEvaluatorRefs(refs *entity.EvaluatorResults) error {
	if refs == nil {
		return nil
	}
	seen := map[string]bool{}
	for _, ref := range refs.Registered {
		if ref == nil || ref.VersionID <= 0 || ref.RecordID <= 0 {
			return fmt.Errorf("incomplete registered source reference")
		}
		key := fmt.Sprintf("r:%d:%s", ref.VersionID, ref.Alias)
		if seen[key] {
			return fmt.Errorf("duplicate source reference")
		}
		seen[key] = true
	}
	for _, ref := range refs.Inline {
		if ref == nil || ref.InlineKey == "" || ref.RecordID <= 0 {
			return fmt.Errorf("incomplete inline source reference")
		}
		key := "i:" + ref.InlineKey
		if seen[key] {
			return fmt.Errorf("duplicate source reference")
		}
		seen[key] = true
	}
	for version, record := range refs.EvalVerIDToResID {
		if version <= 0 || record <= 0 {
			return fmt.Errorf("incomplete legacy source reference")
		}
	}
	return nil
}

func currentSourceMatches(expected, actual *entity.ExptCurrentItemRunSource) bool {
	if expected == nil || actual == nil {
		return false
	}
	a, b := expected.Current, actual.Current
	if a.SpaceID != b.SpaceID || a.ExptID != b.ExptID || a.RunID != b.RunID || a.ItemID != b.ItemID || a.ItemVersionID != b.ItemVersionID || a.RequestID != b.RequestID || a.Mode != b.Mode || len(expected.Turns) != len(actual.Turns) || !reflect.DeepEqual(expected.CurrentRefs, actual.CurrentRefs) {
		return false
	}
	if len(expected.CanonicalTurns) != len(actual.CanonicalTurns) {
		return false
	}
	for index, turn := range expected.CanonicalTurns {
		other := actual.CanonicalTurns[index]
		if turn == nil || other == nil || turn.ID != other.ID || turn.TurnID != other.TurnID || turn.ItemVersionID != other.ItemVersionID || turn.ExptRunID != other.ExptRunID || turn.TargetResultID != other.TargetResultID {
			return false
		}
	}
	for id, turn := range expected.Turns {
		other := actual.Turns[id]
		if turn == nil || other == nil || turn.ID != other.ID || turn.TargetResultID != other.TargetResultID || turn.ItemVersionID != other.ItemVersionID || !entity.SameCurrentEvaluatorRefs(turn.EvaluatorResultIds, other.EvaluatorResultIds) ||
			turn.Ext[entity.ResourceTurnStartExtKey] != other.Ext[entity.ResourceTurnStartExtKey] {
			return false
		}
	}
	return true
}

func (dao *exptItemResultDAOImpl) StoreCurrentTurnRunResult(ctx context.Context, source *entity.ExptCurrentItemRunSource, result *entity.ExptTurnResultRunLog) (bool, error) {
	if source == nil || result == nil {
		return false, fmt.Errorf("execution source or turn result missing")
	}
	po, err := convert.NewExptTurnResultRunLogConvertor().DO2PO(result)
	if err != nil {
		return false, err
	}
	if err := validateCurrentEvaluatorRefs(result.EvaluatorResultIds); err != nil {
		return false, err
	}
	applied := false
	err = dao.provider.Transaction(ctx, func(tx *gorm.DB) error {
		actual, err := loadCurrentRunSource(tx, source.Current.SpaceID, source.Current.ExptID, source.Current.RunID, source.Current.ItemID)
		if err != nil {
			return err
		}
		if !currentSourceMatches(source, actual) || entity.IsItemRunFinished(actual.Current.Status) || actual.Current.ResultState == entity.ExptItemResultStateResulted {
			return nil
		}
		old := actual.Turns[result.TurnID]
		if old == nil || result.ID != old.ID || result.SpaceID != source.Current.SpaceID || result.ExptID != source.Current.ExptID || result.ExptRunID != source.Current.RunID || result.ItemID != source.Current.ItemID || (result.ItemVersionID != source.Current.ItemVersionID && result.ItemVersionID != old.ItemVersionID) {
			return fmt.Errorf("foreign execution turn result")
		}
		if old.Status == entity.TurnRunState_Success || old.Status == entity.TurnRunState_Terminal {
			return nil
		}
		fields := map[string]any{"status": po.Status, "trace_id": po.TraceID, "log_id": po.LogID, "target_result_id": po.TargetResultID, "evaluator_result_ids": gptr.Indirect(po.EvaluatorResultIds), "err_msg": gptr.Indirect(po.ErrMsg), "ext": po.Ext}
		row := tx.Model(&model.ExptTurnResultRunLog{}).Where("id=? AND space_id=? AND expt_id=? AND expt_run_id=? AND item_id=? AND item_version_id=?", old.ID, source.Current.SpaceID, source.Current.ExptID, source.Current.RunID, source.Current.ItemID, old.ItemVersionID)
		written := row.Updates(fields)
		if written.Error != nil {
			return written.Error
		}
		if written.RowsAffected == 0 {
			// MySQL reports changed rows; verify a replay under the source/turn locks.
			var stored model.ExptTurnResultRunLog
			if err := row.Take(&stored).Error; err != nil {
				return err
			}
			got, err := convert.NewExptTurnResultRunLogConvertor().PO2DO(&stored)
			if err != nil {
				return err
			}
			if got.TurnID != old.TurnID || got.Status != result.Status || got.TraceID != result.TraceID || got.LogID != result.LogID || got.TargetResultID != result.TargetResultID || got.ErrMsg != result.ErrMsg ||
				!entity.SameCurrentEvaluatorRefs(got.EvaluatorResultIds, result.EvaluatorResultIds) || (len(got.Ext)+len(result.Ext) != 0 && !reflect.DeepEqual(got.Ext, result.Ext)) {
				return fmt.Errorf("current execution turn update not applied")
			}
			applied = true
			return ctx.Err()
		}
		if written.RowsAffected != 1 {
			return fmt.Errorf("current execution turn disappeared")
		}
		applied = true
		return ctx.Err()
	}, db.WithMaster())
	return applied && err == nil, err
}

func (dao *exptItemResultDAOImpl) CompleteCurrentItemRun(ctx context.Context, source *entity.ExptCurrentItemRunSource, write *entity.ExptCurrentItemRunWrite) (bool, error) {
	if source == nil || write == nil || write.RetryTimes < 0 {
		return false, fmt.Errorf("execution source or write missing")
	}
	applied := false
	err := dao.provider.Transaction(ctx, func(tx *gorm.DB) error {
		actual, err := loadCurrentRunSource(tx, source.Current.SpaceID, source.Current.ExptID, source.Current.RunID, source.Current.ItemID)
		if err != nil {
			return err
		}
		if !currentSourceMatches(source, actual) || actual.Current.ResultState == entity.ExptItemResultStateResulted {
			return nil
		}
		if write.OnlyIfQueueingUnreserved {
			// Reuse the canonical-first locked PO reader: the source DTO does not
			// carry the canonical status or a populated quota reservation state.
			locked, err := lockDispatchItem(tx, actual.Current.SpaceID, actual.Current.ExptID, actual.Current.RunID, actual.Current.ItemID, actual.Current.RequestID)
			if err != nil {
				return err
			}
			if locked == nil || locked.item.Status != int32(entity.ItemRunState_Queueing) || locked.log.Status != int32(entity.ItemRunState_Queueing) || locked.log.QuotaReservationState != int32(entity.QuotaReservationStateNone) {
				return nil
			}
		}
		c := actual.Current
		if entity.IsItemRunFinished(c.Status) && (c.Status != entity.ItemRunState_Terminal || c.ResultState != entity.ExptItemResultStateDefault) {
			return nil
		}
		if !write.RetryOnly && len(actual.Turns) != len(actual.CanonicalTurns) {
			if actual.Current.RequestID != "" {
				return fmt.Errorf("manual execution turn source incomplete")
			}
			prepared := map[int64]*entity.ExptTurnResultRunLog{}
			for _, log := range write.MissingTurnLogs {
				if log == nil || log.ID <= 0 || log.SpaceID != actual.Current.SpaceID || log.ExptID != actual.Current.ExptID || log.ExptRunID != actual.Current.RunID || log.ItemID != actual.Current.ItemID || log.ItemVersionID != actual.Current.ItemVersionID {
					return fmt.Errorf("missing raw turn log preparation invalid")
				}
				prepared[log.TurnID] = log
			}
			for _, turn := range actual.CanonicalTurns {
				if actual.Turns[turn.TurnID] != nil {
					continue
				}
				log := prepared[turn.TurnID]
				if log == nil {
					return fmt.Errorf("raw turn log preparation missing")
				}
				po, err := convert.NewExptTurnResultRunLogConvertor().DO2PO(log)
				if err != nil {
					return err
				}
				if err := tx.Create(po).Error; err != nil {
					return err
				}
			}
		}

		retry := max(c.RetryTimes, write.RetryTimes)
		fields := map[string]any{"retry_times": retry}
		next := write.Status
		if write.RetryOnly {
			if retry == c.RetryTimes {
				applied = true
				return nil
			}
		} else {
			if write.Yield {
				next = entity.ItemRunState_Queueing
				if c.Status != entity.ItemRunState_Processing {
					return nil
				}
			} else if next != entity.ItemRunState_Success && next != entity.ItemRunState_Fail && next != entity.ItemRunState_Terminal {
				return fmt.Errorf("invalid execution completion status")
			}
			if c.Status == entity.ItemRunState_Terminal {
				next = entity.ItemRunState_Terminal
			} else {
				fields["err_msg"] = write.ErrMsg
			}
			fields["status"] = int32(next)
			result := entity.ExptItemResultStateLogged
			if write.Yield {
				result = entity.ExptItemResultStateDefault
			}
			fields["result_state"] = int32(result)
		}
		var itemLog model.ExptItemResultRunLog
		if err := tx.Select("item_version_id").Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", c.SpaceID, c.ExptID, c.RunID, c.ItemID).Take(&itemLog).Error; err != nil {
			return err
		}
		scope := "space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?"
		args := []any{c.SpaceID, c.ExptID, c.RunID, c.ItemID}
		if err := tx.Model(&model.ExptItemResultRunLog{}).Where(scope, args...).Where("item_version_id=?", itemLog.ItemVersionID).Updates(fields).Error; err != nil {
			return err
		}
		if !write.RetryOnly {
			if write.Yield || write.FailPendingTurns || next == entity.ItemRunState_Terminal {
				turnState := entity.TurnRunState_Fail
				if write.Yield {
					turnState = entity.TurnRunState_Queueing
				}
				if next == entity.ItemRunState_Terminal {
					turnState = entity.TurnRunState_Terminal
				}
				turnFields := map[string]any{"status": int32(turnState)}
				if !write.Yield {
					turnFields["err_msg"] = write.TurnError(turnState)
				}
				if err := tx.Model(&model.ExptTurnResultRunLog{}).Where(scope, args...).Where("item_version_id IN ?", []int64{c.ItemVersionID, 0}).Where("status NOT IN ?", []int32{int32(entity.TurnRunState_Success), int32(entity.TurnRunState_Terminal)}).Updates(turnFields).Error; err != nil {
					return err
				}
			}
			if write.Yield {
				if err := tx.Model(&model.ExptTurnResult{}).Where(scope, args...).Where("item_version_id IN ?", []int64{c.ItemVersionID, 0}).Where("status NOT IN ?", []int32{int32(entity.TurnRunState_Success), int32(entity.TurnRunState_Terminal)}).Updates(map[string]any{"status": int32(entity.TurnRunState_Queueing)}).Error; err != nil {
					return err
				}
			}
			var canonical model.ExptItemResult
			if err := tx.Where("space_id=? AND expt_id=? AND item_id=?", c.SpaceID, c.ExptID, c.ItemID).Take(&canonical).Error; err != nil {
				return err
			}
			prior := entity.ItemRunState(canonical.Status)
			canonicalFields := map[string]any{"status": int32(next)}
			if next != entity.ItemRunState_Terminal {
				canonicalFields["err_msg"] = write.ErrMsg
			}
			if err := tx.Model(&model.ExptItemResult{}).Where(scope, args...).Where("item_version_id=?", c.ItemVersionID).Updates(canonicalFields).Error; err != nil {
				return err
			}
			if prior != next {
				var stats model.ExptStats
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("space_id=? AND expt_id=?", c.SpaceID, c.ExptID).Take(&stats).Error; err != nil {
					return err
				}
				columns := map[entity.ItemRunState]string{entity.ItemRunState_Queueing: "pending_cnt", entity.ItemRunState_Processing: "processing_cnt", entity.ItemRunState_Success: "success_cnt", entity.ItemRunState_Fail: "fail_cnt", entity.ItemRunState_Terminal: "terminated_cnt"}
				from, to := columns[prior], columns[next]
				if from == "" || to == "" {
					return fmt.Errorf("invalid completion stats source")
				}
				// 旧链路计数可能漂移（见 b0b19095）：条目状态与 expt_stats 不再一一对应时，
				// 这里的 -1 会把计数写成负数并外播到 progress/webhook。状态 CAS 仍是防双跑闸门，
				// 计数只 clamp + 告警，最终由 CompleteExpt 的全量重算兜底。
				if prior == entity.ItemRunState_Queueing && stats.PendingCnt <= 0 || prior == entity.ItemRunState_Processing && stats.ProcessingCnt <= 0 {
					looplogs.CtxWarn(ctx, "legacy completion stats drift: space_id=%d expt_id=%d item_id=%d prior=%d pending_cnt=%d processing_cnt=%d", c.SpaceID, c.ExptID, c.ItemID, prior, stats.PendingCnt, stats.ProcessingCnt)
				}
				if err := tx.Model(&model.ExptStats{}).Where("id=? AND space_id=? AND expt_id=?", stats.ID, c.SpaceID, c.ExptID).Updates(map[string]any{from: gorm.Expr(fmt.Sprintf("GREATEST(%s - 1, 0)", from)), to: gorm.Expr(fmt.Sprintf("GREATEST(%s + 1, 0)", to))}).Error; err != nil {
					return err
				}
			}
		}
		applied = true
		return ctx.Err()
	}, db.WithMaster())
	return applied && err == nil, err
}
