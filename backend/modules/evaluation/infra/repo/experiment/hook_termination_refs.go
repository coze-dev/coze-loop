// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type hookArchiveRefKey struct {
	turn int64
	ref  hookProgressRefKey
}

func expectedHookArchiveRefs(current *entity.HookTerminationItem) (map[hookArchiveRefKey]int64, error) {
	ids := map[int64]int64{}
	for _, mt := range current.Manifest.Turns {
		ids[mt.TurnID] = mt.ResultID
	}
	expected := map[hookArchiveRefKey]int64{}
	for _, tr := range current.Turns {
		refs, err := hookProgressRefs(tr.EvaluatorResultIds)
		if err != nil {
			return nil, err
		}
		for key, id := range refs {
			expected[hookArchiveRefKey{ids[tr.TurnID], key}] = id
		}
	}
	return expected, nil
}

func materializeHookReferences(tx *gorm.DB, in entity.HookItemArchiveInput, current *entity.HookTerminationItem) error {
	expected, err := expectedHookArchiveRefs(current)
	if err != nil {
		return err
	}
	if len(in.Refs) != len(expected) {
		return entity.ErrHookFinalizationUnsettled
	}
	if len(expected) == 0 {
		return validateHookArchiveRefs(tx, in.Key, current)
	}
	seen := map[hookArchiveRefKey]bool{}
	ids := map[int64]bool{}
	for _, ref := range in.Refs {
		if ref == nil || ref.ID <= 0 || ids[ref.ID] || ref.SpaceID != in.Key.WorkspaceID || ref.ExptID != in.Key.ExperimentID {
			return entity.ErrHookStoreCorrupt
		}
		key := hookArchiveRefKey{ref.ExptTurnResultID, hookProgressRefKey{ref.EvaluatorVersionID, ref.Alias, ref.InlineKey}}
		if seen[key] || expected[key] != ref.EvaluatorResultID || ref.InlineKey != "" && (ref.SourceType != int32(entity.EvaluatorRecordSourceTypeInline) || ref.EvaluatorVersionID != 0 || ref.Alias != "") {
			return entity.ErrHookStoreCorrupt
		}
		seen[key], ids[ref.ID] = true, true
		po := convert.NewExptTurnEvaluatorResultRefConvertor().DO2PO(ref)
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(po).Error; err != nil {
			return err
		}
	}
	return validateHookArchiveRefs(tx, in.Key, current)
}

func validateHookArchiveRefs(tx *gorm.DB, key entity.HookRunKey, current *entity.HookTerminationItem) error {
	expected, err := expectedHookArchiveRefs(current)
	if err != nil {
		return err
	}
	ids := make([]int64, 0, len(current.Manifest.Turns))
	for _, mt := range current.Manifest.Turns {
		ids = append(ids, mt.ResultID)
	}
	var rows []model.ExptTurnEvaluatorResultRef
	if err := tx.Unscoped().Where("space_id=? AND expt_id=? AND expt_turn_result_id IN ?", key.WorkspaceID, key.ExperimentID, ids).Find(&rows).Error; err != nil {
		return err
	}
	for _, row := range rows {
		k := hookArchiveRefKey{row.ExptTurnResultID, hookProgressRefKey{row.EvaluatorVersionID, row.Alias_, row.InlineKey}}
		if row.DeletedAt.Valid || expected[k] != row.EvaluatorResultID {
			return entity.ErrHookStoreCorrupt
		}
		delete(expected, k)
	}
	if len(expected) != 0 {
		return entity.ErrHookFinalizationUnsettled
	}
	return nil
}
