// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package mysql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ExptResultPublishDAOImpl struct{ provider db.Provider }

func NewExptResultPublishDAO(provider db.Provider) repo.IExptResultPublishRepo {
	return &ExptResultPublishDAOImpl{provider: provider}
}

func readItemResultPublishSource(tx *gorm.DB, space, expt, run, item int64) (*entity.ExptItemResultPublishSource, error) {
	return readItemResultPublishSourceWithLocks(tx, space, expt, run, item, true)
}

func readItemResultPublishSourceWithLocks(tx *gorm.DB, space, expt, run, item int64, lockRows bool) (*entity.ExptItemResultPublishSource, error) {
	var locks []clause.Expression
	if lockRows {
		locks = append(locks, clause.Locking{Strength: "UPDATE"})
	}
	source, err := readItemRunSource(tx, space, expt, run, item, true, lockRows)
	if err != nil || source == nil {
		return nil, err
	}
	var experiment model.Experiment
	if err := tx.Where("space_id=? AND id=?", space, expt).Take(&experiment).Error; err != nil {
		return nil, err
	}
	if experiment.LatestRunID != run {
		return nil, nil
	}
	var canonical model.ExptItemResult
	if err := tx.Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", space, expt, run, item).Take(&canonical).Error; err != nil {
		return nil, err
	}
	var log model.ExptItemResultRunLog
	if err := tx.Where("space_id=? AND expt_id=? AND expt_run_id=? AND item_id=?", space, expt, run, item).Take(&log).Error; err != nil {
		return nil, err
	}
	itemDO := convert.NewExptItemResultConvertor().PO2DO(&canonical)
	itemDO.Ext, err = entity.ParseExptItemResultExt(gptr.Indirect(canonical.Ext))
	if err != nil {
		return nil, err
	}
	result := &entity.ExptItemResultPublishSource{Source: source, Item: itemDO, ItemLog: convert.NewExptItemResultRunLogConverter().PO2DO(&log)}
	var ref model.ExptItemRef
	err = tx.Clauses(locks...).Where("space_id=? AND expt_id=? AND item_id=?", space, expt, item).Take(&ref).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err == nil {
		if ref.ItemVersionID != canonical.ItemVersionID {
			return nil, fmt.Errorf("publication frozen item version mismatch")
		}
		result.ItemRef = convert.NewExptItemRefConvertor().PO2DO(&ref)
		result.ItemConfigJSON = append([]byte(nil), gptr.Indirect(ref.ItemConfig)...)
		if len(result.ItemConfigJSON) > 0 {
			var config entity.ExptItemConfig
			if err := json.Unmarshal(result.ItemConfigJSON, &config); err != nil {
				return nil, err
			}
			if string(result.ItemConfigJSON) == "null" {
				return nil, fmt.Errorf("publication frozen item configuration null")
			}
			result.ItemRef.ItemConfig = &config
		}
	}
	return result, nil
}
func validPublicationSource(source *entity.ExptItemResultPublishSource) bool {
	return source != nil && source.Source != nil && source.Item != nil && source.ItemLog != nil && source.ItemLog.ResultState == int32(entity.ExptItemResultStateLogged) && entity.IsItemRunFinished(entity.ItemRunState(source.ItemLog.Status)) && (!entity.IsItemRunFinished(source.Item.Status) || source.Item.Status == entity.ItemRunState(source.ItemLog.Status))
}
func equalPublicationSource(a, b *entity.ExptItemResultPublishSource) bool {
	if !validPublicationSource(a) || !validPublicationSource(b) || !currentSourceMatches(a.Source, b.Source) {
		return false
	}
	if !reflect.DeepEqual(a.Item, b.Item) || !reflect.DeepEqual(a.ItemLog, b.ItemLog) || !reflect.DeepEqual(a.Source.CanonicalTurns, b.Source.CanonicalTurns) || !reflect.DeepEqual(a.ItemRef, b.ItemRef) || !reflect.DeepEqual(a.ItemConfigJSON, b.ItemConfigJSON) {
		return false
	}
	return reflect.DeepEqual(a.Source.Turns, b.Source.Turns)
}
func (r *ExptResultPublishDAOImpl) GetItemResultPublishSource(ctx context.Context, space, expt, run, item int64) (*entity.ExptItemResultPublishSource, error) {
	var result *entity.ExptItemResultPublishSource
	err := r.provider.Transaction(ctx, func(tx *gorm.DB) error {
		var err error
		result, err = readItemResultPublishSource(tx, space, expt, run, item)
		return err
	}, db.WithMaster())
	return result, err
}

func (r *ExptResultPublishDAOImpl) GetStandardEvalOutputSource(ctx context.Context, space, expt, run, item int64) (*entity.ExptItemResultPublishSource, error) {
	var result *entity.ExptItemResultPublishSource
	err := r.provider.NewSession(ctx, db.WithMaster()).Transaction(func(tx *gorm.DB) error {
		var experiment model.Experiment
		if err := tx.Where("space_id=? AND id=?", space, expt).Take(&experiment).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		if run > 0 && experiment.LatestRunID != run {
			return nil
		}
		if run == 0 {
			run = experiment.LatestRunID
		}
		if run <= 0 {
			return nil
		}
		var err error
		result, err = readItemResultPublishSourceWithLocks(tx, space, expt, run, item, false)
		return err
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	return result, err
}

func (r *ExptResultPublishDAOImpl) IsItemResultPublishSourceCurrent(ctx context.Context, source *entity.ExptItemResultPublishSource) (bool, error) {
	if source == nil || source.Source == nil {
		return false, fmt.Errorf("publication source missing")
	}
	c := source.Source.Current
	valid := false
	err := r.provider.Transaction(ctx, func(tx *gorm.DB) error {
		actual, err := readItemResultPublishSource(tx, c.SpaceID, c.ExptID, c.RunID, c.ItemID)
		if err != nil {
			return err
		}
		valid = equalPublicationSource(source, actual)
		return ctx.Err()
	}, db.WithMaster())
	return valid && err == nil, err
}
func (r *ExptResultPublishDAOImpl) CommitItemResult(ctx context.Context, publication *entity.ExptItemResultPublication) (bool, error) {
	if publication == nil || publication.Source == nil || publication.Source.Source == nil {
		return false, fmt.Errorf("publication source missing")
	}
	source := publication.Source
	c := source.Source.Current
	applied := false
	err := r.provider.Transaction(ctx, func(tx *gorm.DB) error {
		actual, err := readItemResultPublishSource(tx, c.SpaceID, c.ExptID, c.RunID, c.ItemID)
		if err != nil {
			return err
		}
		if !equalPublicationSource(source, actual) {
			return nil
		}
		if len(actual.Source.CanonicalTurns) == 0 || len(actual.Source.CanonicalTurns) != len(actual.Source.Turns) {
			return fmt.Errorf("publication frozen turn source incomplete")
		}
		if len(publication.Turns) != len(actual.Source.CanonicalTurns) {
			return fmt.Errorf("publication projected turns incomplete")
		}
		turns := map[int64]*entity.ExptTurnResult{}
		turnIDs := make([]int64, 0, len(publication.Turns))
		for _, turn := range publication.Turns {
			if turn == nil || turn.SpaceID != c.SpaceID || turn.ExptID != c.ExptID || turn.ExptRunID != c.RunID || turn.ItemID != c.ItemID || turn.ItemVersionID != c.ItemVersionID {
				return fmt.Errorf("foreign publication turn")
			}
			log := actual.Source.Turns[turn.TurnID]
			if log == nil || turn.Status != int32(log.Status) || turn.TargetResultID != log.TargetResultID || turn.ErrMsg != log.ErrMsg || turn.LogID != log.LogID || turn.TraceID != log.TraceID {
				return fmt.Errorf("publication turn/source mismatch")
			}
			if _, exists := turns[turn.ID]; exists {
				return fmt.Errorf("duplicate publication turn")
			}
			turns[turn.ID] = turn
			turnIDs = append(turnIDs, turn.ID)
		}
		expectedRefs := map[string]int64{}
		for _, turn := range actual.Source.CanonicalTurns {
			if turns[turn.ID] == nil || turns[turn.ID].TurnID != turn.TurnID {
				return fmt.Errorf("publication canonical turn mismatch")
			}
			log := actual.Source.Turns[turn.TurnID]
			refs := log.EvaluatorResultIds
			if refs == nil {
				continue
			}
			for _, ref := range refs.Registered {
				expectedRefs[publicationRefKey(turn.ID, ref.VersionID, ref.Alias, "")] = ref.RecordID
			}
			for _, ref := range refs.Inline {
				expectedRefs[publicationRefKey(turn.ID, 0, "", ref.InlineKey)] = ref.RecordID
			}
			if !refs.IsNewFormat() {
				for ver, id := range refs.EvalVerIDToResID {
					expectedRefs[publicationRefKey(turn.ID, ver, "", "")] = id
				}
			}
		}
		if len(expectedRefs) != len(publication.Refs) {
			return fmt.Errorf("publication refs incomplete")
		}
		priorRefs := map[string]*entity.ExptTurnEvaluatorResultRef{}
		for _, ref := range actual.Source.CurrentRefs {
			priorRefs[publicationRefKey(ref.ExptTurnResultID, ref.EvaluatorVersionID, ref.Alias, ref.InlineKey)] = ref
		}
		keepIDs := []int64{}
		inserts := []*model.ExptTurnEvaluatorResultRef{}
		used := map[string]bool{}
		for _, ref := range publication.Refs {
			if ref == nil || ref.ID <= 0 || ref.SpaceID != c.SpaceID || ref.ExptID != c.ExptID || turns[ref.ExptTurnResultID] == nil {
				return fmt.Errorf("foreign publication ref")
			}
			key := publicationRefKey(ref.ExptTurnResultID, ref.EvaluatorVersionID, ref.Alias, ref.InlineKey)
			if used[key] || expectedRefs[key] != ref.EvaluatorResultID {
				return fmt.Errorf("publication ref source mismatch")
			}
			used[key] = true
			if old := priorRefs[key]; old != nil && old.ID == ref.ID && old.EvaluatorResultID == ref.EvaluatorResultID && old.SourceType == ref.SourceType {
				keepIDs = append(keepIDs, old.ID)
			} else {
				inserts = append(inserts, convert.NewExptTurnEvaluatorResultRefConvertor().DO2PO(ref))
			}
		}
		deletion := tx.Where("space_id=? AND expt_id=? AND expt_turn_result_id IN ?", c.SpaceID, c.ExptID, turnIDs)
		if len(keepIDs) > 0 {
			deletion = deletion.Where("id NOT IN ?", keepIDs)
		}
		if err := deletion.Unscoped().Delete(&model.ExptTurnEvaluatorResultRef{}).Error; err != nil {
			return err
		}
		for _, ref := range inserts {
			if err := tx.Create(ref).Error; err != nil {
				return err
			}
		}
		for _, turn := range publication.Turns {
			fields := map[string]any{"status": turn.Status, "target_result_id": turn.TargetResultID, "trace_id": turn.TraceID, "log_id": turn.LogID, "err_msg": []byte(turn.ErrMsg), "weighted_score": turn.WeightedScore, "expt_run_id": c.RunID}
			if err := tx.Model(&model.ExptTurnResult{}).Where("id=? AND space_id=? AND expt_id=? AND expt_run_id=? AND item_id=? AND item_version_id=?", turn.ID, c.SpaceID, c.ExptID, c.RunID, c.ItemID, c.ItemVersionID).Updates(fields).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&model.ExptItemResult{}).Where("id=? AND space_id=? AND expt_id=? AND expt_run_id=? AND item_id=? AND item_version_id=?", actual.Item.ID, c.SpaceID, c.ExptID, c.RunID, c.ItemID, c.ItemVersionID).Updates(map[string]any{"status": actual.ItemLog.Status, "log_id": actual.ItemLog.LogID, "err_msg": actual.ItemLog.ErrMsg}).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.ExptItemResultRunLog{}).Where("id=? AND space_id=? AND expt_id=? AND expt_run_id=? AND item_id=? AND result_state=?", actual.ItemLog.ID, c.SpaceID, c.ExptID, c.RunID, c.ItemID, int32(entity.ExptItemResultStateLogged)).Update("result_state", int32(entity.ExptItemResultStateResulted)).Error; err != nil {
			return err
		}
		before, after := actual.Item.Status, entity.ItemRunState(actual.ItemLog.Status)
		if before != after {
			var stats model.ExptStats
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("space_id=? AND expt_id=?", c.SpaceID, c.ExptID).Take(&stats).Error; err != nil {
				return err
			}
			columns := map[entity.ItemRunState]string{entity.ItemRunState_Queueing: "pending_cnt", entity.ItemRunState_Processing: "processing_cnt", entity.ItemRunState_Success: "success_cnt", entity.ItemRunState_Fail: "fail_cnt", entity.ItemRunState_Terminal: "terminated_cnt"}
			from, to := columns[before], columns[after]
			if from == "" || to == "" {
				return fmt.Errorf("publication stats source invalid")
			}
			if err := tx.Model(&model.ExptStats{}).Where("id=? AND space_id=? AND expt_id=?", stats.ID, c.SpaceID, c.ExptID).Updates(map[string]any{from: gorm.Expr(from + "-1"), to: gorm.Expr(to + "+1")}).Error; err != nil {
				return err
			}
		}
		applied = true
		return ctx.Err()
	}, db.WithMaster())
	return applied && err == nil, err
}
func publicationRefKey(turn, version int64, alias, key string) string {
	return fmt.Sprintf("%d/%d/%s/%s", turn, version, alias, key)
}
