// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"bytes"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
)

func readHookActiveTerminationStats(tx *gorm.DB, key entity.HookRunKey, life *model.ExptLifecycleRun, bindings ...*boundHookExecution) (*entity.HookFinalizationStats, error) {
	expt, err := readHookRecoveryExperiment(tx, key)
	if err != nil {
		return nil, err
	}
	var run model.ExptRunLog
	if err := hookRunScope(tx, key).First(&run).Error; err != nil {
		return nil, err
	}
	if err := validateActiveTermination(life, expt, entity.ExptRunMode(gptr.Indirect(run.Mode)), bindings...); err != nil {
		return nil, err
	}
	out := &entity.HookFinalizationStats{Key: key, ExecutionScope: life.ExecutionScope, ActiveTermination: true}
	digest := entity.NewHookPlanDigest()
	var turnCount, projectedTurns int64
	for start := int64(0); start < life.PlanCount; {
		var page []model.ExptLifecycleRunItem
		if err := hookRunScope(tx, key).Where("ordinal>=?", start).Order("ordinal").Limit(100).Find(&page).Error; err != nil {
			return nil, err
		}
		if len(page) == 0 || start+int64(len(page)) > life.PlanCount {
			return nil, entity.ErrHookStoreCorrupt
		}
		if err := checkBoundFinalizationPage(tx, key, page, firstFinalizationBinding(bindings), false); err != nil {
			return nil, err
		}
		for i, row := range page {
			if row.Ordinal != start+int64(i) {
				return nil, entity.ErrHookStoreCorrupt
			}
			// Stats is read-only; no row locks are taken in its repeatable-read snapshot.
			current, err := readHookTerminationStatsItem(tx, key, row)
			if err != nil {
				return nil, err
			}
			digest, err = entity.AppendHookPlanDigest(digest, []entity.HookPlanItem{current.Manifest.Frozen})
			if err != nil {
				return nil, err
			}
			if current.Item.ResultState != int32(entity.ExptItemResultStateResulted) {
				return nil, entity.ErrHookFinalizationUnsettled
			}
			if expt.LatestRunID == key.RunID {
				if err := validateHookTerminationProjection(tx, key, current); err != nil {
					return nil, err
				}
			}
			if err := addHookFinalizationCount(&out.Items, current.Item.Status); err != nil {
				return nil, err
			}
			if len(current.Turns) == 0 {
				status := int32(entity.ItemRunState_Terminal)
				if hookNoExecutionFailure(current) {
					status = int32(entity.ItemRunState_Fail)
				}
				for range current.Manifest.Turns {
					if err := addHookFinalizationCount(&out.Turns, status); err != nil {
						return nil, err
					}
				}
			}
			for _, tr := range current.Turns {
				var status entity.ItemRunState
				switch tr.Status {
				case entity.TurnRunState_Success:
					status = entity.ItemRunState_Success
				case entity.TurnRunState_Fail:
					status = entity.ItemRunState_Fail
				case entity.TurnRunState_Terminal:
					status = entity.ItemRunState_Terminal
				default:
					return nil, entity.ErrHookFinalizationUnsettled
				}
				if err := addHookFinalizationCount(&out.Turns, int32(status)); err != nil {
					return nil, err
				}
			}
			turnCount += int64(len(current.Turns))
			projectedTurns += int64(len(current.Manifest.Turns))
			out.ItemIDs = append(out.ItemIDs, row.ItemID)
		}
		start += int64(len(page))
	}
	if digest.Hash != gptr.Indirect(life.PlanHash) {
		return nil, entity.ErrHookStoreCorrupt
	}
	for _, c := range []struct {
		table any
		n     int64
	}{{&model.ExptLifecycleRunItem{}, life.PlanCount}, {&model.ExptItemResultRunLog{}, life.PlanCount}, {&model.ExptTurnResultRunLog{}, turnCount}} {
		var n int64
		if err := hookRunScope(tx.Unscoped().Model(c.table), key).Count(&n).Error; err != nil {
			return nil, err
		}
		if n != c.n {
			return nil, entity.ErrHookStoreCorrupt
		}
	}
	if expt.LatestRunID == key.RunID {
		for _, c := range []struct {
			table any
			n     int64
		}{{&model.ExptItemResult{}, life.PlanCount}, {&model.ExptTurnResult{}, projectedTurns}} {
			var n int64
			if err := hookRunScope(tx.Unscoped().Model(c.table), key).Count(&n).Error; err != nil {
				return nil, err
			}
			if n != c.n {
				return nil, entity.ErrHookStoreCorrupt
			}
		}
	}
	return out, nil
}

func validateHookTerminationProjection(tx *gorm.DB, key entity.HookRunKey, current *entity.HookTerminationItem) error {
	m := current.Manifest
	noExecutionFailure := hookNoExecutionFailure(current)
	var item model.ExptItemResult
	if err := tx.Unscoped().Where("id=?", m.ItemResultID).First(&item).Error; err != nil {
		return err
	}
	if item.DeletedAt.Valid || item.SpaceID != key.WorkspaceID || item.ExptID != key.ExperimentID || item.ExptRunID != key.RunID || item.ItemID != m.Frozen.ItemID || item.ItemVersionID != m.Frozen.ItemVersionID || item.ItemIdx == nil || *item.ItemIdx != int32(m.ProjectionOrdinal()) {
		return entity.ErrHookStoreCorrupt
	}
	if item.Status != current.Item.Status {
		return entity.ErrHookFinalizationUnsettled
	}
	if noExecutionFailure && !bytes.Equal(gptr.Indirect(item.ErrMsg), current.Item.ErrMsg) {
		return entity.ErrHookStoreCorrupt
	}
	byTurn := map[int64]*entity.ExptTurnResultRunLog{}
	for _, tr := range current.Turns {
		byTurn[tr.TurnID] = tr
	}
	for _, mt := range m.Turns {
		var tr model.ExptTurnResult
		if err := tx.Unscoped().Where("id=?", mt.ResultID).First(&tr).Error; err != nil {
			return err
		}
		if tr.DeletedAt.Valid || tr.SpaceID != key.WorkspaceID || tr.ExptID != key.ExperimentID || tr.ExptRunID != key.RunID || tr.ItemID != m.Frozen.ItemID || tr.ItemVersionID != m.Frozen.ItemVersionID || tr.TurnID != mt.TurnID || tr.TurnIdx == nil || *tr.TurnIdx != mt.TurnIdx {
			return entity.ErrHookStoreCorrupt
		}
		log := byTurn[mt.TurnID]
		if log == nil {
			status := int32(entity.TurnRunState_Terminal)
			if noExecutionFailure {
				status = int32(entity.TurnRunState_Fail)
				if tr.WeightedScore != nil || !bytes.Equal(gptr.Indirect(tr.ErrMsg), current.Item.ErrMsg) {
					return entity.ErrHookStoreCorrupt
				}
			}
			if tr.Status != status || tr.TargetResultID != 0 {
				return entity.ErrHookFinalizationUnsettled
			}
		} else if tr.Status != int32(log.Status) || tr.TargetResultID != log.TargetResultID {
			return entity.ErrHookFinalizationUnsettled
		}
	}
	if noExecutionFailure {
		for _, c := range []struct {
			table any
			count int64
		}{{&model.ExptItemResult{}, 1}, {&model.ExptTurnResult{}, int64(len(m.Turns))}} {
			var n int64
			if err := hookRunScope(tx.Unscoped().Model(c.table), key).Where("item_id=?", m.Frozen.ItemID).Count(&n).Error; err != nil {
				return err
			}
			if n != c.count {
				return entity.ErrHookStoreCorrupt
			}
		}
		ids := make([]int64, 0, len(m.Turns))
		for _, mt := range m.Turns {
			ids = append(ids, mt.ResultID)
		}
		if err := checkHookNoExecutionReferences(tx, key, ids); err != nil {
			return err
		}
	}
	return validateHookArchiveRefs(tx, key, current)
}

func readHookTerminationStatsItem(tx *gorm.DB, key entity.HookRunKey, row model.ExptLifecycleRunItem) (*entity.HookTerminationItem, error) {
	current, _, err := readHookArchiveItemRows(tx, key, row.ItemID, false)
	return current, err
}
