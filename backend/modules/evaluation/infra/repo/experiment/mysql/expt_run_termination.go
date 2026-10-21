// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package mysql

import (
	"context"
	"fmt"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (d *ExptRunFinishDAOImpl) GetTerminationItemSource(ctx context.Context, space, expt, run, item int64) (*entity.ExptCurrentItemRunSource, error) {
	var source *entity.ExptCurrentItemRunSource
	err := d.provider.Transaction(ctx, func(tx *gorm.DB) error {
		var err error
		source, err = readItemRunSource(tx, space, expt, run, item, true, false)
		if err != nil {
			return err
		}
		if source == nil {
			return fmt.Errorf("termination item source missing")
		}
		return ctx.Err()
	}, db.WithMaster())
	return source, err
}

func (d *ExptRunFinishDAOImpl) TerminateCurrentRun(ctx context.Context, r *entity.ExptRunTerminationRequest) (*entity.ExptRunTerminationResult, error) {
	if r == nil || r.SpaceID <= 0 || r.ExptID <= 0 || r.RunID <= 0 {
		return nil, fmt.Errorf("invalid termination identities")
	}
	result := &entity.ExptRunTerminationResult{}
	err := d.provider.Transaction(ctx, func(tx *gorm.DB) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if r.Begin {
			// Drain transactions that already passed the active-run fence before
			// atomically publishing Terminating to subsequent execution writers.
			var items []model.ExptItemResult
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("space_id=? AND expt_id=? AND expt_run_id=?", r.SpaceID, r.ExptID, r.RunID).Order("item_id").Find(&items).Error; err != nil {
				return err
			}
		}
		// Lock the canonical sources before their logs, as execution/publication do.
		actual := make([]*entity.ExptCurrentItemRunSource, 0, len(r.Items))
		prior := map[int64]entity.ItemRunState{}
		changed := map[int64]bool{}
		var lastItem int64
		for _, source := range r.Items {
			if source == nil || source.Current.ItemID <= lastItem {
				return fmt.Errorf("termination sources must be unique and ascending")
			}
			lastItem = source.Current.ItemID
			var canonical model.ExptItemResult
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("space_id=? AND expt_id=? AND item_id=?", r.SpaceID, r.ExptID, source.Current.ItemID).Take(&canonical).Error; err != nil {
				return err
			}
			prior[source.Current.ItemID] = entity.ItemRunState(canonical.Status)
			if source.Current.SpaceID != r.SpaceID || source.Current.ExptID != r.ExptID || source.Current.RunID != r.RunID {
				return fmt.Errorf("foreign termination source")
			}
			s, err := readItemRunSource(tx, r.SpaceID, r.ExptID, r.RunID, source.Current.ItemID, true, true)
			if err != nil {
				return err
			}
			if s == nil {
				return fmt.Errorf("termination source disappeared")
			}
			if !currentSourceMatches(source, s) {
				if source.Current.RequestID == s.Current.RequestID {
					return fmt.Errorf("termination source references advanced")
				}
				result.NonCurrent = true
				return nil
			}
			actual = append(actual, s)
		}
		if r.ExpectedSource != nil {
			c := r.ExpectedSource.Current
			s, err := readItemRunSource(tx, r.SpaceID, r.ExptID, r.RunID, c.ItemID, true, true)
			if err != nil {
				return err
			}
			if s == nil {
				return fmt.Errorf("termination execution source disappeared")
			}
			if !currentSourceMatches(r.ExpectedSource, s) {
				result.NonCurrent = true
				return nil
			}
		}
		var expt model.Experiment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("space_id=? AND id=?", r.SpaceID, r.ExptID).Take(&expt).Error; err != nil {
			return err
		}
		var run model.ExptRunLog
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("space_id=? AND expt_id=? AND expt_run_id=?", r.SpaceID, r.ExptID, r.RunID).Take(&run).Error; err != nil {
			return err
		}
		if expt.LatestRunID != r.RunID {
			result.NonCurrent = true
			return nil
		}
		if expt.ExptType != int32(entity.ExptType_Offline) || run.Status == nil {
			return fmt.Errorf("invalid offline termination binding")
		}
		if r.SetupAbort {
			// This admission cleanup precedes ExptStart. It never terminates the
			// experiment or permits a user cancellation of a Pending experiment.
			if r.Begin || r.Finalize || len(r.Items) != 0 || r.ExpectedSource != nil ||
				expt.Status != int32(entity.ExptStatus_Pending) || *run.Status != int64(entity.ExptStatus_Pending) {
				return fmt.Errorf("setup abort requires pending experiment and pending current run")
			}
			fields := map[string]any{"status": int64(entity.ExptStatus_Failed), "status_message": []byte(r.StatusMessage)}
			if err := tx.Model(&model.ExptRunLog{}).Where("id=?", run.ID).UpdateColumns(fields).Error; err != nil {
				return err
			}
			result.Applied = true
			return ctx.Err()
		}
		if entity.IsExptFinished(entity.ExptStatus(expt.Status)) || entity.IsExptFinished(entity.ExptStatus(*run.Status)) {
			if !r.Finalize || !entity.IsExptFinished(entity.ExptStatus(expt.Status)) || !entity.IsExptFinished(entity.ExptStatus(*run.Status)) || expt.EndAt == nil {
				return fmt.Errorf("termination requires active current run")
			}
			var err error
			result.Experiment, err = convert.NewExptConverter().PO2DO(&expt, nil)
			return err
		}
		active := expt.Status == int32(entity.ExptStatus_Processing) && (*run.Status == int64(entity.ExptStatus_Pending) || *run.Status == int64(entity.ExptStatus_Processing))
		terminating := expt.Status == int32(entity.ExptStatus_Terminating) && *run.Status == int64(entity.ExptStatus_Terminating)
		allowed := active || terminating && (r.Begin || r.Finalize)
		if !allowed {
			return fmt.Errorf("termination requires processing experiment and active run")
		}
		if r.Begin {
			if active {
				if err := tx.Model(&model.ExptRunLog{}).Where("id=?", run.ID).UpdateColumns(map[string]any{"status": int64(entity.ExptStatus_Terminating)}).Error; err != nil {
					return err
				}
				if err := tx.Model(&model.Experiment{}).Where("id=? AND latest_run_id=?", expt.ID, r.RunID).UpdateColumns(map[string]any{"status": int32(entity.ExptStatus_Terminating)}).Error; err != nil {
					return err
				}
			}
			result.Applied = true
			return ctx.Err()
		}
		for _, source := range actual {
			c := source.Current
			if c.Status == entity.ItemRunState_Success || c.Status == entity.ItemRunState_Fail {
				continue
			}
			if c.Status == entity.ItemRunState_Terminal && len(source.Turns) == len(source.CanonicalTurns) && c.ResultState != entity.ExptItemResultStateDefault {
				continue
			}
			for _, turn := range source.CanonicalTurns {
				if source.Turns[turn.TurnID] != nil {
					continue
				}
				var prepared *entity.ExptTurnResultRunLog
				for _, log := range r.MissingTurnLogs {
					if log != nil && log.ItemID == c.ItemID && log.TurnID == turn.TurnID {
						prepared = log
						break
					}
				}
				if c.RequestID != "" || prepared == nil || prepared.ID <= 0 || prepared.SpaceID != c.SpaceID || prepared.ExptID != c.ExptID || prepared.ExptRunID != c.RunID || prepared.ItemVersionID != c.ItemVersionID {
					return fmt.Errorf("raw termination turn preparation missing")
				}
				po, err := convert.NewExptTurnResultRunLogConvertor().DO2PO(prepared)
				if err != nil {
					return err
				}
				if err := tx.Create(po).Error; err != nil {
					return err
				}
			}
			errBytes := []byte(errno.SerializeErr(errno.NewItemManuallyTerminatedErr()))
			scope := "space_id=? AND expt_id=? AND expt_run_id=? AND item_id=? AND item_version_id=?"
			args := []any{c.SpaceID, c.ExptID, c.RunID, c.ItemID, c.ItemVersionID}
			if !r.NoCompleteItemTurn {
				if err := tx.Model(&model.ExptTurnResultRunLog{}).Where(scope, args...).Updates(map[string]any{"status": int32(entity.TurnRunState_Terminal), "err_msg": errBytes}).Error; err != nil {
					return err
				}
				if err := tx.Model(&model.ExptTurnResult{}).Where(scope, args...).Updates(map[string]any{"status": int32(entity.TurnRunState_Terminal)}).Error; err != nil {
					return err
				}
			}
			if err := tx.Model(&model.ExptItemResultRunLog{}).Where(scope, args...).Updates(map[string]any{"status": int32(entity.ItemRunState_Terminal), "result_state": int32(entity.ExptItemResultStateLogged), "err_msg": errBytes}).Error; err != nil {
				return err
			}
			if !r.NoCompleteItemTurn {
				if err := tx.Model(&model.ExptItemResult{}).Where(scope, args...).Updates(map[string]any{"status": int32(entity.ItemRunState_Terminal), "err_msg": errBytes}).Error; err != nil {
					return err
				}
			}
			result.Applied = true
			changed[c.ItemID] = true
		}
		// Row cancellation changes only its own buckets; publication sees the
		// same canonical status and contributes no second delta.

		if result.Applied || r.Finalize {
			var stats model.ExptStats
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("space_id=? AND expt_id=?", r.SpaceID, r.ExptID).Take(&stats).Error; err != nil {
				return err
			}
			var fields map[string]any
			if r.Finalize {
				var items []model.ExptItemResult
				if err := tx.Where("space_id=? AND expt_id=?", r.SpaceID, r.ExptID).Find(&items).Error; err != nil {
					return err
				}
				var counts finishCounts
				for _, item := range items {
					counts.add(item.Status)
				}
				fields = counts.fields("", nil)
			} else {
				columns := map[entity.ItemRunState]string{entity.ItemRunState_Queueing: "pending_cnt", entity.ItemRunState_Processing: "processing_cnt", entity.ItemRunState_Success: "success_cnt", entity.ItemRunState_Fail: "fail_cnt", entity.ItemRunState_Terminal: "terminated_cnt"}
				deltas := map[string]int{}
				for _, source := range actual {
					if !changed[source.Current.ItemID] || prior[source.Current.ItemID] == entity.ItemRunState_Terminal {
						continue
					}
					from := columns[prior[source.Current.ItemID]]
					if from == "" {
						return fmt.Errorf("termination stats source invalid")
					}
					deltas[from]--
					deltas["terminated_cnt"]++
				}
				fields = map[string]any{}
				for column, delta := range deltas {
					if delta != 0 {
						fields[column] = gorm.Expr(column+" + ?", delta)
					}
				}
			}
			if len(fields) > 0 {
				if err := tx.Model(&stats).UpdateColumns(fields).Error; err != nil {
					return err
				}
			}
		}

		if r.Finalize {
			if r.Status != entity.ExptStatus_Terminated && r.Status != entity.ExptStatus_Failed {
				return fmt.Errorf("invalid explicit termination status")
			}
			var turns []model.ExptTurnResult
			if err := tx.Where("space_id=? AND expt_id=?", r.SpaceID, r.ExptID).Find(&turns).Error; err != nil {
				return err
			}
			var turnCounts finishCounts
			for _, turn := range turns {
				turnCounts.addTurn(entity.TurnRunState(turn.Status))
			}
			fields := turnCounts.fields("status", int64(r.Status))
			if r.StatusMessage != "" {
				fields["status_message"] = []byte(r.StatusMessage)
			}
			if err := tx.Model(&model.ExptRunLog{}).Where("id=?", run.ID).UpdateColumns(fields).Error; err != nil {
				return err
			}
			now := time.Now()
			fields = map[string]any{"status": int32(r.Status), "end_at": now}
			if r.StatusMessage != "" {
				fields["status_message"] = []byte(r.StatusMessage)
			}
			if err := tx.Model(&model.Experiment{}).Where("id=? AND latest_run_id=?", expt.ID, r.RunID).UpdateColumns(fields).Error; err != nil {
				return err
			}
			expt.Status = int32(r.Status)
			expt.EndAt = &now
			result.Applied = true
			var err error
			result.Experiment, err = convert.NewExptConverter().PO2DO(&expt, nil)
			if err != nil {
				return err
			}
		}
		return ctx.Err()
	}, db.WithMaster())
	if err != nil {
		return nil, err
	}
	return result, nil
}
