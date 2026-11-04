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

func stableItemsAfterObservation(before, after *itemReadObservation) (map[int64]bool, error) {
	if !reflect.DeepEqual(before.Experiment, after.Experiment) || !reflect.DeepEqual(before.Runs, after.Runs) {
		return nil, fmt.Errorf("current run observation changed; retry primary read")
	}
	turnItems := make(map[int64]int64, len(before.Turns)+len(after.Turns))
	for _, turn := range before.Turns {
		turnItems[turn.ID] = turn.ItemID
	}
	for _, turn := range after.Turns {
		turnItems[turn.ID] = turn.ItemID
	}
	beforeItems, err := splitItemReadObservation(before, turnItems)
	if err != nil {
		return nil, err
	}
	afterItems, err := splitItemReadObservation(after, turnItems)
	if err != nil {
		return nil, err
	}
	stable := make(map[int64]bool, len(beforeItems))
	for itemID, beforeItem := range beforeItems {
		afterItem, ok := afterItems[itemID]
		if ok && reflect.DeepEqual(beforeItem, afterItem) {
			stable[itemID] = true
		}
	}
	return stable, nil
}

func splitItemReadObservation(o *itemReadObservation, turnItems map[int64]int64) (map[int64]*itemReadObservation, error) {
	items := make(map[int64]*itemReadObservation)
	scoped := func(itemID int64) *itemReadObservation {
		item := items[itemID]
		if item == nil {
			item = &itemReadObservation{}
			items[itemID] = item
		}
		return item
	}
	for _, row := range o.Items {
		item := scoped(row.ItemID)
		item.Items = append(item.Items, row)
	}
	for _, row := range o.ItemLogs {
		item := scoped(row.ItemID)
		item.ItemLogs = append(item.ItemLogs, row)
	}
	for _, row := range o.Turns {
		item := scoped(row.ItemID)
		item.Turns = append(item.Turns, row)
	}
	for _, row := range o.TurnLogs {
		item := scoped(row.ItemID)
		item.TurnLogs = append(item.TurnLogs, row)
	}
	for _, row := range o.ItemRefs {
		item := scoped(row.ItemID)
		item.ItemRefs = append(item.ItemRefs, row)
	}
	for _, row := range o.Refs {
		itemID, ok := turnItems[row.ExptTurnResultID]
		if !ok {
			return nil, fmt.Errorf("current item observation ref turn missing")
		}
		item := scoped(itemID)
		item.Refs = append(item.Refs, row)
	}
	return items, nil
}
