// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

// IExptRetryItemResetDAO is the storage port behind repo.IExptRetryItemResetRepo.
type IExptRetryItemResetDAO interface {
	ResetItemForRetry(ctx context.Context, spaceID, exptID, runID int64, runLog *entity.ExptItemResultRunLog, turnIDs []int64) (bool, error)
}

func (dao *exptItemResultDAOImpl) ResetItemForRetry(ctx context.Context, spaceID, exptID, runID int64, runLog *entity.ExptItemResultRunLog, turnIDs []int64) (applied bool, err error) {
	if runLog == nil || runLog.SpaceID != spaceID || runLog.ExptID != exptID || runLog.ExptRunID != runID || runLog.ItemID <= 0 {
		return false, fmt.Errorf("invalid retry reset run log")
	}
	err = dao.provider.Transaction(ctx, func(tx *gorm.DB) error {
		var expt model.Experiment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("space_id=? AND id=?", spaceID, exptID).
			Take(&expt).Error; err != nil {
			return fmt.Errorf("retry reset experiment: %w", err)
		}
		if expt.LatestRunID != runID {
			return fmt.Errorf("retry reset experiment run replaced")
		}
		if expt.Status != int32(entity.ExptStatus_Processing) {
			if err := tx.Model(&model.Experiment{}).
				Where("id=? AND space_id=? AND latest_run_id=?", expt.ID, spaceID, runID).
				Update("status", int32(entity.ExptStatus_Processing)).Error; err != nil {
				return err
			}
		}

		var canonical model.ExptItemResult
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("space_id=? AND expt_id=? AND item_id=?", spaceID, exptID, runLog.ItemID).
			Take(&canonical).Error; err != nil {
			return fmt.Errorf("retry reset canonical item: %w", err)
		}

		var current model.ExptItemResultRunLog
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", spaceID, exptID, runID, runLog.ItemID).
			Take(&current).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		applied = true
		fields, err := newRunItemResultFields(&canonical, runID, map[string]any{
			"err_msg":     nil,
			"status":      int32(entity.ItemRunState_Queueing),
			"expt_run_id": runID,
		})
		if err != nil {
			return err
		}
		if err := tx.Model(&model.ExptItemResult{}).Where("id=? AND space_id=? AND expt_id=?", canonical.ID, spaceID, exptID).Updates(fields).Error; err != nil {
			return err
		}
		if len(turnIDs) > 0 {
			if err := tx.Model(&model.ExptTurnResult{}).
				Where("space_id=? AND expt_id=? AND item_id=? AND turn_id IN ?", spaceID, exptID, runLog.ItemID, turnIDs).
				Updates(map[string]any{
					"err_msg":          nil,
					"status":           int32(entity.TurnRunState_Queueing),
					"target_result_id": int64(0),
					"expt_run_id":      runID,
				}).Error; err != nil {
				return err
			}
		}

		emptyEvaluatorResults, err := (&entity.EvaluatorResults{EvalVerIDToResID: map[int64]int64{}}).Serialize()
		if err != nil {
			return err
		}
		opts := []db.Option{db.WithTransaction(tx), db.WithMaster()}
		if err := NewExptTurnResultDAO(dao.provider).UpdateTurnRunLogWithItemIDs(ctx, spaceID, exptID, runID, []int64{runLog.ItemID}, map[string]any{
			"target_result_id":     int64(0),
			"evaluator_result_ids": emptyEvaluatorResults,
		}, opts...); err != nil {
			return err
		}

		if runLog.ItemVersionID == 0 {
			runLog.ItemVersionID = canonical.ItemVersionID
		}
		if err := tx.Create(convert.NewExptItemResultRunLogConverter().DO2PO(runLog)).Error; err != nil {
			return err
		}

		delta := make(map[entity.ItemRunState]int)
		switch entity.ItemRunState(canonical.Status) {
		case entity.ItemRunState_Processing, entity.ItemRunState_Success, entity.ItemRunState_Fail, entity.ItemRunState_Terminal:
			delta[entity.ItemRunState(canonical.Status)] = -1
			delta[entity.ItemRunState_Queueing] = 1
		}
		if err := NewExptStatsDAO(dao.provider).ArithOperateCount(ctx, exptID, spaceID, &entity.StatsCntArithOp{OpStatusCnt: delta}, opts...); err != nil {
			return err
		}
		return ctx.Err()
	}, db.WithMaster())
	if err != nil {
		return false, err
	}
	return applied, nil
}
