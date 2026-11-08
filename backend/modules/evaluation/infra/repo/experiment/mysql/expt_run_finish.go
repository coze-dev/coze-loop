// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package mysql

import (
	"context"
	"fmt"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
)

type ExptRunFinishDAOImpl struct{ provider db.Provider }

func NewExptRunFinishDAO(provider db.Provider) repo.IExptRunFinishRepo {
	return &ExptRunFinishDAOImpl{provider: provider}
}

func (d *ExptRunFinishDAOImpl) FinishCurrentRun(ctx context.Context, r *entity.ExptRunFinishRequest) (*entity.ExptRunFinishResult, error) {
	if r == nil || r.SpaceID <= 0 || r.ExptID <= 0 || r.RunID <= 0 {
		return nil, fmt.Errorf("invalid finish identities")
	}
	result := &entity.ExptRunFinishResult{}
	err := d.provider.Transaction(ctx, func(tx *gorm.DB) error {
		opts := []db.Option{db.WithTransaction(tx), db.WithMaster(), db.WithSelectForUpdate()}
		expt, err := NewExptDAO(d.provider).GetByID(ctx, r.ExptID, opts...)
		if err != nil {
			return err
		}
		run, err := NewExptRunLogDAO(d.provider).Get(ctx, r.ExptID, r.RunID, opts...)
		if err != nil {
			return err
		}
		if expt.SpaceID != r.SpaceID || run.SpaceID != r.SpaceID || run.ExptID != r.ExptID || run.ExptRunID != r.RunID {
			return fmt.Errorf("finish run binding corrupt")
		}
		if expt.LatestRunID != r.RunID {
			result.Disposition = entity.ExptRunFinishNonCurrent
			return d.completeHistoricalRun(ctx, tx, expt, run, 0, "")
		}
		if expt.ExptType != int32(entity.ExptType_Offline) {
			return fmt.Errorf("natural current-run finish requires offline experiment")
		}
		runStatus := entity.ExptStatus(gptr.Indirect(run.Status))
		runFinished := entity.IsExptFinished(runStatus)
		exptFinished := entity.IsExptFinished(entity.ExptStatus(expt.Status))
		// Legacy CompleteRun commits before CompleteExpt; revalidate its result below.
		legacyCompleted := expt.Status == int32(entity.ExptStatus_Processing) && (runStatus == entity.ExptStatus_Success || runStatus == entity.ExptStatus_Failed)
		if (runFinished || exptFinished) && !legacyCompleted {
			if !runFinished || !exptFinished || expt.EndAt == nil {
				return fmt.Errorf("inconsistent partially finished current run")
			}
			result.Disposition = entity.ExptRunFinishAlreadyFinished
			result.Experiment, err = convert.NewExptConverter().PO2DO(expt, nil)
			return err
		}
		if expt.Status != int32(entity.ExptStatus_Processing) || !legacyCompleted && (runStatus != entity.ExptStatus_Pending && runStatus != entity.ExptStatus_Processing) {
			return fmt.Errorf("finish requires active experiment and run")
		}
		session := d.provider.NewSession(ctx, opts...)
		var items []model.ExptItemResult
		if err := session.Select("id,item_id,expt_run_id,item_version_id,status").Where("space_id=? AND expt_id=?", r.SpaceID, r.ExptID).Order("item_id").Find(&items).Error; err != nil {
			return err
		}
		var itemLogs []model.ExptItemResultRunLog
		if err := session.Select("id,item_id,item_version_id,status,result_state").Where("space_id=? AND expt_id=? AND expt_run_id=?", r.SpaceID, r.ExptID, r.RunID).Order("item_id").Find(&itemLogs).Error; err != nil {
			return err
		}
		for _, log := range itemLogs {
			if log.Status == int32(entity.ItemRunState_Queueing) || log.Status == int32(entity.ItemRunState_Processing) || gptr.Indirect(log.ResultState) == int32(entity.ExptItemResultStateLogged) {
				result.Disposition = entity.ExptRunFinishPending
				return nil
			}
		}
		canonical := make(map[int64]model.ExptItemResult, len(items))
		var itemCounts finishCounts
		for _, item := range items {
			canonical[item.ItemID] = item
			itemCounts.add(item.Status)
		}
		for _, log := range itemLogs {
			item, ok := canonical[log.ItemID]
			if !ok || item.ExptRunID != r.RunID || item.Status != log.Status || item.ItemVersionID != log.ItemVersionID || gptr.Indirect(log.ResultState) != int32(entity.ExptItemResultStateResulted) || !entity.IsItemRunFinished(entity.ItemRunState(log.Status)) {
				return fmt.Errorf("finish canonical item/run log binding corrupt")
			}
		}
		loggedItems := make(map[int64]bool, len(itemLogs))
		for _, log := range itemLogs {
			loggedItems[log.ItemID] = true
		}
		for _, item := range items {
			if item.ExptRunID == r.RunID && !loggedItems[item.ItemID] {
				return fmt.Errorf("finish current item run log missing")
			}
		}
		var turns []model.ExptTurnResult
		if err := session.Select("id,item_id,turn_id,status").Where("space_id=? AND expt_id=?", r.SpaceID, r.ExptID).Order("item_id,turn_id").Find(&turns).Error; err != nil {
			return err
		}
		var turnCounts finishCounts
		for _, turn := range turns {
			turnCounts.addTurn(entity.TurnRunState(turn.Status))
		}
		var stats model.ExptStats
		if err := session.Where("space_id=? AND expt_id=?", r.SpaceID, r.ExptID).Take(&stats).Error; err != nil {
			return err
		}
		runStatus = entity.ExptStatus_Success
		// Preserve calculateRunLogStats: canonical turns across all runs, including
		// unexecuted turns after an item failure. Those counts do not block finish.
		if turnCounts.pending > 0 || turnCounts.fail > 0 {
			runStatus = entity.ExptStatus_Failed
		}
		if legacyCompleted && gptr.Indirect(run.Status) != int64(runStatus) {
			return fmt.Errorf("legacy finished run does not match canonical turn stats")
		}
		exptStatus := entity.ExptStatus_Success
		// User-terminated items are not evaluation failures (CalculateStats contract).
		if itemCounts.fail > 0 || itemCounts.pending > 0 || itemCounts.processing > 0 {
			exptStatus = entity.ExptStatus_Failed
		}
		if err := session.Model(&model.ExptRunLog{}).Where("space_id=? AND expt_id=? AND expt_run_id=?", r.SpaceID, r.ExptID, r.RunID).UpdateColumns(turnCounts.fields("status", int64(runStatus))).Error; err != nil {
			return err
		}
		now := time.Now()
		if err := session.Model(&model.Experiment{}).Where("space_id=? AND id=? AND latest_run_id=?", r.SpaceID, r.ExptID, r.RunID).UpdateColumns(map[string]any{"status": int32(exptStatus), "end_at": now}).Error; err != nil {
			return err
		}
		if err := session.Model(&model.ExptStats{}).Where("space_id=? AND expt_id=?", r.SpaceID, r.ExptID).UpdateColumns(itemCounts.fields("", nil)).Error; err != nil {
			return err
		}
		result.Disposition = entity.ExptRunFinishFinished
		expt.Status = int32(exptStatus)
		expt.EndAt = &now
		result.Experiment, err = convert.NewExptConverter().PO2DO(expt, nil)
		if err != nil {
			return err
		}
		// e6 already released admission before saving a legacy terminal run.
		if !runFinished && r.BeforeCommit != nil {
			return r.BeforeCommit(ctx, entity.ExptRunMode(gptr.Indirect(run.Mode)))
		}
		return nil
	}, db.WithMaster())
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (d *ExptRunFinishDAOImpl) completeHistoricalRun(ctx context.Context, tx *gorm.DB, expt *model.Experiment, run *model.ExptRunLog, status entity.ExptStatus, message string) error {
	if run.Status == nil {
		return fmt.Errorf("historical run status missing")
	}
	if entity.IsExptFinished(entity.ExptStatus(*run.Status)) {
		return nil
	}
	if expt.ExptType != int32(entity.ExptType_Offline) || expt.LatestRunID <= 0 || expt.LatestRunID == run.ExptRunID {
		return fmt.Errorf("historical completion requires another current offline run")
	}
	var latest model.ExptRunLog
	if err := tx.WithContext(ctx).Where("space_id=? AND expt_id=? AND expt_run_id=?", expt.SpaceID, expt.ID, expt.LatestRunID).Take(&latest).Error; err != nil {
		return err
	}
	active := []int64{int64(entity.ExptStatus_Pending), int64(entity.ExptStatus_Processing), int64(entity.ExptStatus_Terminating)}
	if *run.Status != active[0] && *run.Status != active[1] && *run.Status != active[2] {
		return fmt.Errorf("invalid historical run status")
	}
	var turns []model.ExptTurnResultRunLog
	if err := tx.WithContext(ctx).Select("status").Where("space_id=? AND expt_id=? AND expt_run_id=?", run.SpaceID, run.ExptID, run.ExptRunID).Find(&turns).Error; err != nil {
		return err
	}
	var counts finishCounts
	for _, turn := range turns {
		counts.addTurn(entity.TurnRunState(turn.Status))
	}
	if status == 0 {
		status = entity.ExptStatus_Success
		if counts.pending > 0 || counts.fail > 0 {
			status = entity.ExptStatus_Failed
		}
	} else if status != entity.ExptStatus_Failed && status != entity.ExptStatus_Terminated {
		return fmt.Errorf("invalid historical explicit status")
	}
	// R2 owns canonical projections; only R1's historical log may be updated.
	fields := counts.fields("status", int64(status))
	fields["updated_at"] = time.Now()
	if message != "" {
		fields["status_message"] = []byte(message)
	}
	return tx.WithContext(ctx).Model(&model.ExptRunLog{}).
		Where("space_id=? AND expt_id=? AND expt_run_id=? AND status IN ?", run.SpaceID, run.ExptID, run.ExptRunID, active).
		UpdateColumns(fields).Error
}

// Counts follow the existing canonical projection readers. Item and turn
// status enums differ; keep their mappings separate.
type finishCounts struct{ pending, processing, success, fail, terminal int32 }

func (c *finishCounts) add(status int32) {
	switch status {
	case int32(entity.ItemRunState_Queueing):
		c.pending++
	case int32(entity.ItemRunState_Processing):
		c.processing++
	case int32(entity.ItemRunState_Success):
		c.success++
	case int32(entity.ItemRunState_Fail):
		c.fail++
	case int32(entity.ItemRunState_Terminal):
		c.terminal++
	}
}

func (c finishCounts) fields(statusKey string, status any) map[string]any {
	result := map[string]any{"pending_cnt": c.pending, "processing_cnt": c.processing, "success_cnt": c.success, "fail_cnt": c.fail, "terminated_cnt": c.terminal}
	if statusKey != "" {
		result[statusKey] = status
	}
	return result
}

func (c *finishCounts) addTurn(status entity.TurnRunState) {
	switch status {
	case entity.TurnRunState_Queueing:
		c.pending++
	case entity.TurnRunState_Processing:
		c.processing++
	case entity.TurnRunState_Success:
		c.success++
	case entity.TurnRunState_Fail:
		c.fail++
	case entity.TurnRunState_Terminal:
		c.terminal++
	}
}
