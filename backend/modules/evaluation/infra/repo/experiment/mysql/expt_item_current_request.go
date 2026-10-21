// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bytedance/gg/gptr"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

type currentRequestRow struct {
	ID, ItemID, ItemVersionID, CanonicalID, CanonicalRunID, CanonicalItemVersionID int64
	Status, CanonicalStatus, RetryTimes                                            int32
	ResultState                                                                    *int32
	QuotaReservationState                                                          int32
	Ext                                                                            []byte
	Mode                                                                           *int32
	RunStatus                                                                      *int64
	UpdatedAt                                                                      time.Time
}

func (dao *exptItemResultDAOImpl) readCurrentRequests(ctx context.Context, spaceID, exptID, runID int64, itemIDs []int64, retryFirst bool, opts ...db.Option) ([]*entity.ExptItemCurrentRequest, error) {
	session := dao.provider.NewSession(ctx, append(opts, db.WithMaster())...).Table("expt_item_result_run_log AS l").
		Select("l.id,l.item_id,l.item_version_id,l.status,l.result_state,l.retry_times,l.quota_reservation_state,l.updated_at,c.id AS canonical_id,c.expt_run_id AS canonical_run_id,c.status AS canonical_status,c.item_version_id AS canonical_item_version_id,c.ext,r.mode,r.status AS run_status").
		Joins("LEFT JOIN expt_item_result AS c ON c.space_id=l.space_id AND c.expt_id=l.expt_id AND c.item_id=l.item_id AND c.deleted_at IS NULL").
		Joins("LEFT JOIN expt_run_log AS r ON r.space_id=l.space_id AND r.expt_id=l.expt_id AND r.expt_run_id=l.expt_run_id AND r.deleted_at IS NULL").
		Where("l.space_id=? AND l.expt_id=? AND l.expt_run_id=? AND l.deleted_at IS NULL", spaceID, exptID, runID)
	if itemIDs != nil {
		session = session.Where("l.item_id IN ?", itemIDs)
	}
	if retryFirst {
		session = session.Order("l.retry_times ASC")
	}
	var rows []currentRequestRow
	if err := session.Order("l.id ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]*entity.ExptItemCurrentRequest, 0, len(rows))
	for _, row := range rows {
		if row.CanonicalID == 0 || row.CanonicalRunID != runID {
			continue
		}
		ext, err := entity.ParseExptItemResultExt(row.Ext)
		if err != nil {
			return nil, err
		}
		manual, err := entity.ParseManualRetryContext(ext)
		if err != nil {
			return nil, err
		}
		status := entity.ItemRunState(row.Status)
		if !validCurrentItemStatus(status) || !validCurrentItemStatus(entity.ItemRunState(row.CanonicalStatus)) || row.RetryTimes < 0 {
			return nil, fmt.Errorf("invalid current item state")
		}
		if row.ItemVersionID != row.CanonicalItemVersionID {
			return nil, fmt.Errorf("canonical/run-log item version mismatch")
		}
		if row.RunStatus == nil {
			return nil, fmt.Errorf("source run missing")
		}
		runStatus := entity.ExptStatus(*row.RunStatus)
		if entity.IsExptFinished(runStatus) || entity.IsExptFinishing(runStatus) {
			continue
		}
		if runStatus != entity.ExptStatus_Processing && runStatus != entity.ExptStatus_Pending {
			return nil, fmt.Errorf("unknown source run state")
		}
		mode := entity.ExptRunMode(gptr.Indirect(row.Mode))
		if mode < entity.EvaluationModeSubmit || mode > entity.EvaluationModeTrialRun {
			return nil, fmt.Errorf("invalid source run mode")
		}
		request := ""
		if manual != nil {
			if manual.RunID != runID {
				return nil, fmt.Errorf("manual retry run mismatch")
			}
			mode = manual.Mode
			request = manual.RequestID
		}
		if mode < entity.EvaluationModeSubmit || mode > entity.EvaluationModeTrialRun {
			return nil, fmt.Errorf("invalid current run mode")
		}
		result := gptr.Indirect(row.ResultState)
		if result < 0 || result > int32(entity.ExptItemResultStateLogged) {
			return nil, fmt.Errorf("invalid item result state")
		}
		// The run log leads completion/yield/start; the canonical projection is synchronized later.
		// A finished canonical item remains absorbing even if a late run-log write disagrees.
		canonicalStatus := entity.ItemRunState(row.CanonicalStatus)
		if entity.IsItemRunFinished(canonicalStatus) && canonicalStatus != status {
			status = canonicalStatus
			result = int32(entity.ExptItemResultStateDefault)
		}

		out = append(out, &entity.ExptItemCurrentRequest{SpaceID: spaceID, ExptID: exptID, RunID: runID, ItemID: row.ItemID, ItemVersionID: row.ItemVersionID, RequestID: request, Mode: mode, Status: status, ResultState: entity.ExptItemResultState(result), RetryTimes: row.RetryTimes, UpdatedAt: gptr.Of(row.UpdatedAt), QuotaReservationState: entity.QuotaReservationState(row.QuotaReservationState)})
	}
	return out, nil
}

func validCurrentItemStatus(status entity.ItemRunState) bool {
	return status == entity.ItemRunState_Queueing || status == entity.ItemRunState_Processing || entity.IsItemRunFinished(status)
}

func (dao *exptItemResultDAOImpl) ScanCurrentRequests(ctx context.Context, spaceID, exptID, runID int64, retryFirst bool) ([]*entity.ExptItemCurrentRequest, error) {
	var rows []*entity.ExptItemCurrentRequest
	// NDB accepts ordinary Begin, without per-transaction isolation/readOnly SQL.
	// Complete raw source verification also makes default RC reads fail closed.
	err := dao.provider.NewSession(ctx, db.WithMaster()).Transaction(func(tx *gorm.DB) error {
		before, err := collectItemReadObservation(tx, spaceID, exptID, runID, nil, nil)
		if err != nil {
			return err
		}
		if len(before.Experiment) != 1 || before.Experiment[0].LatestRunID != runID {
			return nil
		}
		rows, err = dao.readCurrentRequests(ctx, spaceID, exptID, runID, nil, retryFirst, db.WithTransaction(tx))
		if err != nil {
			return err
		}
		for _, row := range rows {
			if row.ResultState == entity.ExptItemResultStateLogged {
				row.PublicationObserved = true
				publication, err := readItemResultPublishSourceWithLocks(tx, spaceID, exptID, runID, row.ItemID, false)
				if err != nil {
					return err
				}
				if publication != nil && publication.Source.Current.RequestID == row.RequestID && publication.ItemLog.Status == int32(row.Status) && publication.ItemLog.ResultState == int32(row.ResultState) && publication.ItemLog.RetryTimes == row.RetryTimes && publication.ItemLog.UpdatedAt != nil && row.UpdatedAt != nil && publication.ItemLog.UpdatedAt.Equal(*row.UpdatedAt) {
					row.Publication = publication
				}
			}
			// Both source forms are inside the same verified observation window;
			// mutation paths still recheck under their own row locks.
			if row.Status != entity.ItemRunState_Processing {
				continue
			}
			source, err := readItemRunSource(tx, spaceID, exptID, runID, row.ItemID, false, false)
			if err != nil {
				return err
			}
			if source == nil || !source.Current.Matches(runID, row.RequestID) || source.Current.ItemVersionID != row.ItemVersionID || source.Current.Status != row.Status || source.Current.RetryTimes != row.RetryTimes || source.Current.UpdatedAt == nil || row.UpdatedAt == nil || !source.Current.UpdatedAt.Equal(*row.UpdatedAt) {
				continue
			}
			row.Source = source
		}
		after, err := collectItemReadObservation(tx, spaceID, exptID, runID, nil, before)
		if err != nil {
			return err
		}
		if err := verifyItemReadObservation(before, after); err != nil {
			return err
		}
		return ctx.Err()
	})
	if err != nil {
		return nil, err
	}
	return rows, err
}

func (dao *exptItemResultDAOImpl) GetCurrentRequest(ctx context.Context, spaceID, exptID, runID, itemID int64) (*entity.ExptItemCurrentRequest, error) {
	rows, err := dao.readCurrentRequests(ctx, spaceID, exptID, runID, []int64{itemID}, false)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		current, err := dao.BatchGet(ctx, spaceID, exptID, []int64{itemID}, db.WithMaster())
		if err != nil {
			return nil, err
		}
		if len(current) > 0 && current[0].ExptRunID == runID && !entity.IsItemRunFinished(entity.ItemRunState(current[0].Status)) {
			log, err := dao.GetItemRunLog(ctx, exptID, runID, itemID, spaceID, db.WithMaster())
			if errors.Is(err, gorm.ErrRecordNotFound) || log == nil && err == nil {
				return nil, fmt.Errorf("current item source log missing")
			}
			if err != nil {
				return nil, err
			}
		}
		return nil, nil
	}
	return rows[0], nil
}

func (dao *exptItemResultDAOImpl) MarkProcessingIfCurrent(ctx context.Context, spaceID, exptID, runID, itemID int64, requestID string) (bool, error) {
	changed := false
	err := dao.provider.Transaction(ctx, func(tx *gorm.DB) error {
		var item model.ExptItemResult
		lock := clause.Locking{Strength: "UPDATE"}
		err := tx.Clauses(lock).Where("space_id=? AND expt_id=? AND item_id=?", spaceID, exptID, itemID).Take(&item).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if item.ExptRunID != runID || entity.IsItemRunFinished(entity.ItemRunState(item.Status)) {
			return nil
		}
		ext, err := entity.ParseExptItemResultExt(gptr.Indirect(item.Ext))
		if err != nil {
			return err
		}
		manual, err := entity.ParseManualRetryContext(ext)
		if err != nil {
			return err
		}
		currentRequest := ""
		if manual != nil {
			if manual.RunID != runID {
				return fmt.Errorf("manual retry run mismatch")
			}
			currentRequest = manual.RequestID
		}
		if currentRequest != requestID {
			return nil
		}
		var log model.ExptItemResultRunLog
		err = tx.Clauses(lock).Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", spaceID, exptID, runID, itemID).Take(&log).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if entity.IsItemRunFinished(entity.ItemRunState(log.Status)) {
			return nil
		}
		if log.RetryTimes < 0 || gptr.Indirect(log.ResultState) < 0 || gptr.Indirect(log.ResultState) > int32(entity.ExptItemResultStateLogged) {
			return fmt.Errorf("invalid item run counters or result state")
		}
		if !validCurrentItemStatus(entity.ItemRunState(item.Status)) || !validCurrentItemStatus(entity.ItemRunState(log.Status)) {
			return fmt.Errorf("invalid current item state")
		}
		if item.Status == int32(entity.ItemRunState_Processing) && log.Status == item.Status {
			return nil
		}
		if item.Status != int32(entity.ItemRunState_Queueing) || log.Status != item.Status {
			return fmt.Errorf("canonical/run-log status mismatch")
		}
		var run model.ExptRunLog
		if err := tx.Where("space_id=? AND expt_id=? AND expt_run_id=?", spaceID, exptID, runID).Take(&run).Error; err != nil {
			return err
		}
		if mode := entity.ExptRunMode(gptr.Indirect(run.Mode)); mode < entity.EvaluationModeSubmit || mode > entity.EvaluationModeTrialRun {
			return fmt.Errorf("invalid source run mode")
		}
		if entity.IsExptFinished(entity.ExptStatus(gptr.Indirect(run.Status))) || entity.IsExptFinishing(entity.ExptStatus(gptr.Indirect(run.Status))) {
			return nil
		}
		var turns []model.ExptTurnResult
		if err := tx.Clauses(lock).Where("space_id=? AND expt_id=? AND item_id=? AND expt_run_id=?", spaceID, exptID, itemID, runID).Order("id ASC").Find(&turns).Error; err != nil {
			return err
		}
		var logs []model.ExptTurnResultRunLog
		if err := tx.Clauses(lock).Where("space_id=? AND expt_id=? AND item_id=? AND expt_run_id=?", spaceID, exptID, itemID, runID).Order("id ASC").Find(&logs).Error; err != nil {
			return err
		}
		byTurn := make(map[int64]model.ExptTurnResultRunLog, len(logs))
		for _, l := range logs {
			if _, err := convert.NewExptTurnResultRunLogConvertor().PO2DO(&l); err != nil {
				return err
			}
			if err := validateManualRetryTurnExt(l.Ext); err != nil {
				return err
			}
			byTurn[l.TurnID] = l
		}
		pendingTurns, pendingLogs := []int64{}, []int64{}
		if manual != nil && len(turns) == 0 {
			return fmt.Errorf("manual frozen turns missing")
		}
		for _, turn := range turns {
			l, exists := byTurn[turn.TurnID]
			if manual != nil && !exists {
				return fmt.Errorf("manual frozen turn log missing")
			}
			if turn.Status == int32(entity.TurnRunState_Success) {
				if manual != nil && l.Status != int32(entity.TurnRunState_Success) {
					return fmt.Errorf("frozen success turn mismatch")
				}
				continue
			}
			if manual != nil && (turn.Status != int32(entity.TurnRunState_Queueing) || l.Status != int32(entity.TurnRunState_Queueing)) {
				return fmt.Errorf("invalid frozen pending turn")
			}
			if turn.Status == int32(entity.TurnRunState_Queueing) {
				pendingTurns = append(pendingTurns, turn.ID)
			}
			if exists && l.Status == int32(entity.TurnRunState_Queueing) {
				pendingLogs = append(pendingLogs, l.ID)
			}
		}
		var stats model.ExptStats
		if err := tx.Clauses(lock).Where("space_id=? AND expt_id=?", spaceID, exptID).Take(&stats).Error; err != nil {
			return err
		}
		if stats.PendingCnt <= 0 {
			return fmt.Errorf("queueing stats underflow")
		}
		for _, row := range []struct {
			value any
			id    int64
		}{{&model.ExptItemResult{}, item.ID}, {&model.ExptItemResultRunLog{}, log.ID}} {
			res := tx.Model(row.value).Where("id=? AND status=?", row.id, int32(entity.ItemRunState_Queueing)).Update("status", int32(entity.ItemRunState_Processing))
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected != 1 {
				return fmt.Errorf("processing CAS lost")
			}
		}
		if len(pendingTurns) > 0 {
			if err := tx.Model(&model.ExptTurnResult{}).Where("id IN ?", pendingTurns).Update("status", int32(entity.TurnRunState_Processing)).Error; err != nil {
				return err
			}
		}
		if len(pendingLogs) > 0 {
			if err := tx.Model(&model.ExptTurnResultRunLog{}).Where("id IN ?", pendingLogs).Update("status", int32(entity.TurnRunState_Processing)).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&model.ExptStats{}).Where("id=?", stats.ID).Updates(map[string]any{"pending_cnt": gorm.Expr("pending_cnt - 1"), "processing_cnt": gorm.Expr("processing_cnt + 1")}).Error; err != nil {
			return err
		}
		changed = true
		return nil
	}, db.WithMaster())
	return changed && err == nil, err
}
