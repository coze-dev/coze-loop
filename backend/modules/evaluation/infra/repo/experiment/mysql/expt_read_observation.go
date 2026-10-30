// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package mysql

import (
	"fmt"
	"reflect"

	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

// Keep raw persisted rows: converted sources intentionally omit fields and
// normalize JSON, so their execution/CAS equality is too weak for RC reads.
// This observes only SELECTs, never session settings or locking clauses.
type itemReadObservation struct {
	Items      []model.ExptItemResult
	ItemLogs   []model.ExptItemResultRunLog
	Turns      []model.ExptTurnResult
	TurnLogs   []model.ExptTurnResultRunLog
	Refs       []model.ExptTurnEvaluatorResultRef
	ItemRefs   []model.ExptItemRef
	Runs       []model.ExptRunLog
	Experiment []model.Experiment
}

// The scheduler observes the complete current run scan.
// Collection order is identical on both passes. The complete item/turn/ref
// membership is reread so a new row cannot silently enter the frozen source.
func collectItemReadObservation(tx *gorm.DB, space, expt, run int64, frozen *itemReadObservation) (*itemReadObservation, error) {
	o := &itemReadObservation{}
	items := tx.Where("space_id=? AND expt_id=? AND expt_run_id=?", space, expt, run)
	if err := items.Order("id ASC").Find(&o.Items).Error; err != nil {
		return nil, err
	}
	if err := tx.Where("space_id=? AND id=?", space, expt).Find(&o.Experiment).Error; err != nil {
		return nil, err
	}
	if run <= 0 {
		return o, nil
	}
	if err := tx.Where("space_id=? AND expt_id=? AND expt_run_id=?", space, expt, run).Order("id ASC").Find(&o.Runs).Error; err != nil {
		return nil, err
	}
	logs := tx.Where("space_id=? AND expt_id=? AND expt_run_id=?", space, expt, run)
	if err := logs.Order("id ASC").Find(&o.ItemLogs).Error; err != nil {
		return nil, err
	}
	itemLogs := o.ItemLogs
	if frozen != nil {
		itemLogs = frozen.ItemLogs
	}
	frozenItems := make([]int64, 0, len(itemLogs))
	for _, log := range itemLogs {
		frozenItems = append(frozenItems, log.ItemID)
	}
	if err := tx.Where("space_id=? AND expt_id=? AND item_id IN ?", space, expt, frozenItems).Order("id ASC").Find(&o.ItemRefs).Error; err != nil {
		return nil, err
	}
	if err := tx.Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id IN ?", space, expt, run, frozenItems).Order("id ASC").Find(&o.Turns).Error; err != nil {
		return nil, err
	}
	if err := tx.Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id IN ?", space, expt, run, frozenItems).Order("id ASC").Find(&o.TurnLogs).Error; err != nil {
		return nil, err
	}
	turns := o.Turns
	if frozen != nil {
		turns = frozen.Turns
	}
	turnIDs := make([]int64, 0, len(turns))
	for _, turn := range turns {
		turnIDs = append(turnIDs, turn.ID)
	}
	if err := tx.Where("space_id=? AND expt_id=? AND expt_turn_result_id IN ?", space, expt, turnIDs).Order("id ASC").Find(&o.Refs).Error; err != nil {
		return nil, err
	}
	return o, nil
}

func verifyItemReadObservation(before, after *itemReadObservation) error {
	if !reflect.DeepEqual(before, after) {
		return fmt.Errorf("current item observation changed; retry primary read")
	}
	return nil
}
