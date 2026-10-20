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
)

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
			return nil, fmt.Errorf("manual current source run mismatch")
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
	if item.ItemVersionID != log.ItemVersionID || log.RetryTimes < 0 || !validCurrentItemStatus(entity.ItemRunState(item.Status)) || !validCurrentItemStatus(entity.ItemRunState(log.Status)) || log.ResultState == nil || *log.ResultState < 0 || *log.ResultState > int32(entity.ExptItemResultStateLogged) {
		return nil, fmt.Errorf("damaged current item source")
	}
	source := &entity.ExptCurrentItemRunSource{Current: entity.ExptItemCurrentRequest{SpaceID: spaceID, ExptID: exptID, RunID: runID, ItemID: itemID, ItemVersionID: item.ItemVersionID, RequestID: request, Mode: mode, Status: entity.ItemRunState(log.Status), ResultState: entity.ExptItemResultState(*log.ResultState), RetryTimes: log.RetryTimes, UpdatedAt: gptr.Of(log.UpdatedAt)}, Turns: map[int64]*entity.ExptTurnResultRunLog{}}
	if entity.IsItemRunFinished(entity.ItemRunState(item.Status)) && item.Status != log.Status {
		source.Current.Status = entity.ItemRunState(item.Status)
	}
	var canonical []model.ExptTurnResult
	if err := tx.Clauses(locks...).Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", spaceID, exptID, runID, itemID).Order("id ASC").Find(&canonical).Error; err != nil {
		return nil, err
	}
	var logs []model.ExptTurnResultRunLog
	if err := tx.Clauses(locks...).Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", spaceID, exptID, runID, itemID).Order("id ASC").Find(&logs).Error; err != nil {
		return nil, err
	}
	canonicalIDs := make([]int64, 0, len(canonical))
	byTurn := map[int64]model.ExptTurnResult{}
	for _, turn := range canonical {
		if turn.ItemVersionID != item.ItemVersionID || turn.TurnID < 0 {
			return nil, fmt.Errorf("damaged canonical turn source")
		}
		byTurn[turn.TurnID] = turn
		canonicalIDs = append(canonicalIDs, turn.ID)
		source.CanonicalTurns = append(source.CanonicalTurns, convert.NewExptTurnResultConvertor().PO2DO(&turn, nil))
	}
	for index := range logs {
		log := &logs[index]
		turn, ok := byTurn[log.TurnID]
		if !ok || log.ItemVersionID != turn.ItemVersionID || log.TurnID < 0 || log.Status < int32(entity.TurnRunState_Queueing) || log.Status > int32(entity.TurnRunState_Terminal) {
			return nil, fmt.Errorf("damaged current turn source")
		}
		converted, err := convert.NewExptTurnResultRunLogConvertor().PO2DO(log)
		if err != nil {
			return nil, err
		}
		if err := validateCurrentEvaluatorRefs(converted.EvaluatorResultIds); err != nil {
			return nil, err
		}
		source.Turns[log.TurnID] = converted
	}
	// A failed first turn need not execute all later turns, but every frozen turn log exists.
	if manual != nil && (len(canonical) != len(logs) || len(canonical) == 0) {
		return nil, fmt.Errorf("current frozen turn logs incomplete")
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
		if turn == nil || other == nil || turn.ID != other.ID || turn.TargetResultID != other.TargetResultID || turn.ItemVersionID != other.ItemVersionID || !entity.SameCurrentEvaluatorRefs(turn.EvaluatorResultIds, other.EvaluatorResultIds) {
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
		if old == nil || result.ID != old.ID || result.SpaceID != source.Current.SpaceID || result.ExptID != source.Current.ExptID || result.ExptRunID != source.Current.RunID || result.ItemID != source.Current.ItemID || result.ItemVersionID != source.Current.ItemVersionID {
			return fmt.Errorf("foreign execution turn result")
		}
		if old.Status == entity.TurnRunState_Success || old.Status == entity.TurnRunState_Terminal {
			return nil
		}
		fields := map[string]any{"status": po.Status, "trace_id": po.TraceID, "log_id": po.LogID, "target_result_id": po.TargetResultID, "evaluator_result_ids": gptr.Indirect(po.EvaluatorResultIds), "err_msg": gptr.Indirect(po.ErrMsg), "ext": po.Ext}
		written := tx.Model(&model.ExptTurnResultRunLog{}).Where("id=? AND space_id=? AND expt_id=? AND expt_run_id=? AND item_id=? AND item_version_id=?", old.ID, source.Current.SpaceID, source.Current.ExptID, source.Current.RunID, source.Current.ItemID, source.Current.ItemVersionID).Updates(fields)
		if written.Error != nil {
			return written.Error
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
		c := actual.Current
		if entity.IsItemRunFinished(c.Status) && !(c.Status == entity.ItemRunState_Terminal && c.ResultState == entity.ExptItemResultStateDefault) {
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
		scope := "space_id=? AND expt_id=? AND expt_run_id=? AND item_id=? AND item_version_id=?"
		args := []any{c.SpaceID, c.ExptID, c.RunID, c.ItemID, c.ItemVersionID}
		if err := tx.Model(&model.ExptItemResultRunLog{}).Where(scope, args...).Updates(fields).Error; err != nil {
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
				if err := tx.Model(&model.ExptTurnResultRunLog{}).Where(scope, args...).Where("status NOT IN ?", []int32{int32(entity.TurnRunState_Success), int32(entity.TurnRunState_Terminal)}).Updates(turnFields).Error; err != nil {
					return err
				}
			}
			if write.Yield {
				if err := tx.Model(&model.ExptTurnResult{}).Where(scope, args...).Where("status NOT IN ?", []int32{int32(entity.TurnRunState_Success), int32(entity.TurnRunState_Terminal)}).Updates(map[string]any{"status": int32(entity.TurnRunState_Queueing)}).Error; err != nil {
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
			if err := tx.Model(&model.ExptItemResult{}).Where(scope, args...).Updates(canonicalFields).Error; err != nil {
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
				if err := tx.Model(&model.ExptStats{}).Where("id=? AND space_id=? AND expt_id=?", stats.ID, c.SpaceID, c.ExptID).Updates(map[string]any{from: gorm.Expr(from + "-1"), to: gorm.Expr(to + "+1")}).Error; err != nil {
					return err
				}
			}
		}
		applied = true
		return ctx.Err()
	}, db.WithMaster())
	return applied && err == nil, err
}
