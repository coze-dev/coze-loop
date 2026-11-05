// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
)

const projectionMaxBytes = 64 << 20

// BuildExptFilterProjection reads complete current items; it never publishes or repairs source records.
func (e ExptResultServiceImpl) BuildExptFilterProjection(ctx context.Context, spaceID, exptID int64, itemIDs []int64) ([]*entity.ExptTurnResultFilterEntity, error) {
	if spaceID <= 0 || exptID <= 0 || len(itemIDs) == 0 || len(itemIDs) > 100 {
		return nil, fmt.Errorf("invalid_projection: bounded scope and 1..100 item IDs required")
	}
	itemsWanted := make(map[int64]bool, len(itemIDs))
	for _, id := range itemIDs {
		if id <= 0 {
			return nil, fmt.Errorf("invalid_projection: item ID must be positive")
		}
		itemsWanted[id] = true
	}
	if e.ExperimentRepo == nil || e.ExptTurnResultRepo == nil || e.ExptItemResultRepo == nil || e.exptTurnResultFilterRepo == nil || e.ExptAnnotateRepo == nil {
		return nil, fmt.Errorf("invalid_projection: source repositories missing")
	}
	ctx = contexts.WithCtxWriteDB(ctx)
	expt, err := e.ExperimentRepo.GetByID(ctx, exptID, spaceID)
	if err != nil {
		return nil, projectionError(spaceID, exptID, 0, 0, "experiment", err)
	}
	if expt == nil || expt.ID != exptID || expt.SpaceID != spaceID {
		return nil, projectionError(spaceID, exptID, 0, 0, "experiment", fmt.Errorf("source missing or ownership mismatch"))
	}
	var turns []*entity.ExptTurnResult
	turnByID := make(map[int64]*entity.ExptTurnResult)
	keys := make(map[entity.ExptFilterKey]bool)
	for page := 1; ; page++ {
		rows, total, readErr := e.ExptTurnResultRepo.ListTurnResultByItemIDs(ctx, spaceID, exptID, itemIDs, entity.NewPage(page, 200), false)
		if readErr != nil {
			return nil, projectionError(spaceID, exptID, 0, 0, "turns", readErr)
		}
		if len(rows) == 0 {
			if int64(len(turns)) < total {
				return nil, projectionError(spaceID, exptID, 0, 0, "turns", fmt.Errorf("page ended before declared total"))
			}
			break
		}
		for _, tr := range rows {
			if tr == nil || tr.ID <= 0 || tr.SpaceID != spaceID || tr.ExptID != exptID || !itemsWanted[tr.ItemID] || tr.TurnID < 0 || tr.TargetResultID < 0 {
				return nil, projectionError(spaceID, exptID, 0, 0, "turns", fmt.Errorf("invalid row or ownership mismatch"))
			}
			key := entity.ExptFilterKey{SpaceID: spaceID, ExptID: exptID, ItemID: tr.ItemID, TurnID: tr.TurnID}
			if turnByID[tr.ID] != nil || keys[key] {
				return nil, projectionError(spaceID, exptID, tr.ItemID, tr.TurnID, "turns", fmt.Errorf("duplicate row or unstable page"))
			}
			turnByID[tr.ID] = tr
			keys[key] = true
			turns = append(turns, tr)
		}
		if len(turns) > 100000 {
			return nil, projectionError(spaceID, exptID, 0, 0, "oversize_item", fmt.Errorf("turn count exceeds bounded projection budget"))
		}
		if int64(len(turns)) >= total {
			break
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
	}
	if len(turns) == 0 {
		return nil, nil
	}
	if expt.StartAt == nil || expt.StartAt.IsZero() || expt.StartAt.In(time.Local).Format("2006-01-02") < "1970-01-01" || expt.StartAt.In(time.Local).Format("2006-01-02") > "2149-06-06" {
		return nil, projectionError(spaceID, exptID, 0, 0, "created_date", fmt.Errorf("valid business start date required"))
	}
	if expt.ExptType != entity.ExptType_Offline && expt.ExptType != entity.ExptType_Online {
		return nil, projectionError(spaceID, exptID, 0, 0, "experiment", fmt.Errorf("unknown experiment type"))
	}
	items, err := e.ExptItemResultRepo.BatchGet(ctx, spaceID, exptID, itemIDs)
	if err != nil {
		return nil, projectionError(spaceID, exptID, 0, 0, "items", err)
	}
	itemByID := make(map[int64]*entity.ExptItemResult, len(items))
	for _, item := range items {
		if item == nil || item.SpaceID != spaceID || item.ExptID != exptID || !itemsWanted[item.ItemID] || itemByID[item.ItemID] != nil || int64(item.Status) < math.MinInt32 || int64(item.Status) > math.MaxInt32 {
			return nil, projectionError(spaceID, exptID, 0, 0, "items", fmt.Errorf("invalid row, duplicate item or ownership mismatch"))
		}
		itemByID[item.ItemID] = item
	}
	refs, err := e.projectionItemRefs(ctx, expt, itemIDs, itemsWanted)
	if err != nil {
		return nil, err
	}
	mappings, err := e.exptTurnResultFilterRepo.GetExptTurnResultFilterKeyMappings(ctx, spaceID, exptID)
	if err != nil {
		return nil, projectionError(spaceID, exptID, 0, 0, "mappings", err)
	}
	evaluatorMapping, annotationMapping, err := projectionMappings(expt, mappings)
	if err != nil {
		return nil, err
	}
	turnIDs := make([]int64, 0, len(turns))
	for _, tr := range turns {
		turnIDs = append(turnIDs, tr.ID)
	}
	records, err := e.projectionEvaluators(ctx, expt, turnIDs, turnByID, evaluatorMapping)
	if err != nil {
		return nil, err
	}
	targets, err := e.projectionTargets(ctx, expt, turns, refs)
	if err != nil {
		return nil, err
	}
	annotations, err := e.projectionAnnotations(ctx, expt, turnIDs, turnByID, annotationMapping)
	if err != nil {
		return nil, err
	}
	result := make([]*entity.ExptTurnResultFilterEntity, 0, len(turns))
	var bytes int64
	for _, tr := range turns {
		item := itemByID[tr.ItemID]
		if item == nil {
			return nil, projectionError(spaceID, exptID, tr.ItemID, tr.TurnID, "items", fmt.Errorf("referenced item missing"))
		}
		row := &entity.ExptTurnResultFilterEntity{SpaceID: spaceID, ExptID: exptID, ItemID: tr.ItemID, TurnID: tr.TurnID, ItemVersionID: tr.ItemVersionID, ItemIdx: item.ItemIdx, Status: item.Status, CreatedDate: expt.StartAt.In(time.Local), EvalSetID: expt.EvalSetID, EvalSetVersionID: expt.EvalSetVersionID, EvalTargetData: map[string]string{}, EvaluatorScore: map[string]float64{}, AnnotationFloat: map[string]float64{}, AnnotationBool: map[string]bool{}, AnnotationString: map[string]string{}, EvalTargetMetrics: map[string]int64{}}
		if ref := refs[tr.ItemID]; ref != nil {
			row.EvalSetID = ref.EvalSetID
			row.EvalSetVersionID = ref.EvalSetVersionID
			row.ItemVersionID = ref.ItemVersionID
		}
		if tr.ItemVersionID != 0 && tr.ItemVersionID != row.ItemVersionID || item.ItemVersionID != 0 && item.ItemVersionID != row.ItemVersionID {
			return nil, projectionError(spaceID, exptID, tr.ItemID, tr.TurnID, "item_refs", fmt.Errorf("item version mismatch"))
		}
		if expt.ExptType == entity.ExptType_Online {
			row.EvalSetVersionID = 0
		}
		if row.EvalSetID <= 0 || row.EvalSetVersionID < 0 {
			return nil, projectionError(spaceID, exptID, tr.ItemID, tr.TurnID, "item_refs", fmt.Errorf("invalid dataset identity"))
		}
		if target := targets[tr.ID]; target != nil {
			if target.ItemVersionID != 0 && target.ItemVersionID != row.ItemVersionID {
				return nil, projectionError(spaceID, exptID, tr.ItemID, tr.TurnID, "targets", fmt.Errorf("item version mismatch"))
			}
			data := target.EvalTargetOutputData
			for key, value := range data.OutputFields {
				if value == nil {
					return nil, projectionError(spaceID, exptID, tr.ItemID, tr.TurnID, "target_output", fmt.Errorf("nil content at key %q", key))
				}
				row.EvalTargetData[key] = value.GetText()
			}
			if usage := data.EvalTargetUsage; usage != nil {
				row.EvalTargetMetrics["input_tokens"] = usage.InputTokens
				row.EvalTargetMetrics["output_tokens"] = usage.OutputTokens
				row.EvalTargetMetrics["total_tokens"] = usage.TotalTokens
			}
			if data.TimeConsumingMS != nil {
				row.EvalTargetMetrics["total_latency"] = *data.TimeConsumingMS
			}
		}
		for instance, record := range records[tr.ID] {
			if record.ItemVersionID != 0 && record.ItemVersionID != row.ItemVersionID {
				return nil, projectionError(spaceID, exptID, tr.ItemID, tr.TurnID, "evaluators", fmt.Errorf("item version mismatch"))
			}
			if score := record.GetScore(); score != nil {
				if !projectionFinite(*score) {
					return nil, projectionError(spaceID, exptID, tr.ItemID, tr.TurnID, "evaluator_score", fmt.Errorf("nonfinite score"))
				}
				row.EvaluatorScore[evaluatorMapping[instance]] = *score
			}
			row.EvaluatorScoreCorrected = row.EvaluatorScoreCorrected || record.GetCorrected()
		}
		for tag, record := range annotations[tr.ID] {
			if err := projectionAnnotation(row, annotationMapping[strconv.FormatInt(tag, 10)], record); err != nil {
				return nil, projectionError(spaceID, exptID, tr.ItemID, tr.TurnID, "annotation", err)
			}
		}
		row.EvaluatorWeightedScore, err = e.projectionWeightedScore(ctx, expt, tr, records[tr.ID])
		if err != nil {
			return nil, err
		}
		encodedSize, encodeErr := entity.ExptFilterProjectionJSONSize(row)
		if encodeErr != nil {
			return nil, projectionError(spaceID, exptID, tr.ItemID, tr.TurnID, "serialization", encodeErr)
		}
		bytes += encodedSize
		if bytes > projectionMaxBytes {
			return nil, projectionError(spaceID, exptID, tr.ItemID, tr.TurnID, "oversize_item", fmt.Errorf("projection exceeds 64 MiB"))
		}
		result = append(result, row)
	}
	return result, nil
}

func projectionError(space, expt, item, turn int64, stage string, cause error) error {
	return fmt.Errorf("invalid_projection: space=%d expt=%d item=%d turn=%d stage=%s: %w", space, expt, item, turn, stage, cause)
}
func projectionFinite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func (e ExptResultServiceImpl) projectionItemRefs(ctx context.Context, expt *entity.Experiment, itemIDs []int64, wanted map[int64]bool) (map[int64]*entity.ExptItemRef, error) {
	multi := expt.EvalSetSourceType == entity.ExptEvalSetSourceType_MultiSetConfig
	if e.exptItemRefRepo == nil {
		if multi {
			return nil, projectionError(expt.SpaceID, expt.ID, 0, 0, "item_refs", fmt.Errorf("multiset source repository missing"))
		}
		return nil, nil
	}
	refs, err := e.exptItemRefRepo.MGetByExptIDAndItemIDs(ctx, expt.SpaceID, expt.ID, itemIDs)
	if err != nil {
		return nil, projectionError(expt.SpaceID, expt.ID, 0, 0, "item_refs", err)
	}
	result := make(map[int64]*entity.ExptItemRef, len(refs))
	for _, ref := range refs {
		if ref == nil || ref.SpaceID != expt.SpaceID || ref.ExptID != expt.ID || !wanted[ref.ItemID] || result[ref.ItemID] != nil || ref.EvalSetID <= 0 || ref.EvalSetVersionID < 0 || ref.ItemVersionID < 0 {
			return nil, projectionError(expt.SpaceID, expt.ID, 0, 0, "item_refs", fmt.Errorf("invalid or duplicate item reference"))
		}
		copyRef := *ref
		if multi {
			matched := false
			var sourceSpace int64
			if expt.EvalConf != nil {
				for _, set := range expt.EvalConf.EvalSetConfigs {
					if set != nil && set.EvalSetID == ref.EvalSetID && (set.EvalSetVersionID == ref.EvalSetVersionID || expt.ExptType == entity.ExptType_Online) && (ref.EvalSetSourceSpaceID == 0 || resolveLoadSpaceID(expt.SpaceID, set.SourceSpaceID) == ref.EvalSetSourceSpaceID) {
						resolved := resolveLoadSpaceID(expt.SpaceID, set.SourceSpaceID)
						if matched && sourceSpace != resolved {
							return nil, projectionError(expt.SpaceID, expt.ID, ref.ItemID, 0, "item_refs", fmt.Errorf("ambiguous frozen dataset source"))
						}
						sourceSpace = resolved
						matched = true
					}
				}
			}
			if !matched {
				return nil, projectionError(expt.SpaceID, expt.ID, ref.ItemID, 0, "item_refs", fmt.Errorf("dataset reference is outside frozen experiment configuration"))
			}
			copyRef.EvalSetSourceSpaceID = sourceSpace
		}
		result[ref.ItemID] = &copyRef
	}
	if multi {
		for id := range wanted {
			if result[id] == nil {
				return nil, projectionError(expt.SpaceID, expt.ID, id, 0, "item_refs", fmt.Errorf("multiset reference missing"))
			}
		}
	}
	return result, nil
}

func projectionMappings(expt *entity.Experiment, mappings []*entity.ExptTurnResultFilterKeyMapping) (map[string]string, map[string]string, error) {
	evaluators, annotations := make(map[string]string), make(map[string]string)
	used := make(map[entity.FieldTypeMapping]map[string]bool)
	for _, m := range mappings {
		if m == nil || m.SpaceID != expt.SpaceID || m.ExptID != expt.ID {
			return nil, nil, projectionError(expt.SpaceID, expt.ID, 0, 0, "mappings", fmt.Errorf("nil mapping or ownership mismatch"))
		}
		var dst map[string]string
		switch m.FieldType {
		case entity.FieldTypeEvaluator:
			dst = evaluators
		case entity.FieldTypeManualAnnotation:
			dst = annotations
		default:
			continue
		}
		if used[m.FieldType] == nil {
			used[m.FieldType] = make(map[string]bool)
		}
		if m.FromField == "" || m.ToKey == "" || dst[m.FromField] != "" || used[m.FieldType][m.ToKey] {
			return nil, nil, projectionError(expt.SpaceID, expt.ID, 0, 0, "mappings", fmt.Errorf("empty or ambiguous mapping"))
		}
		dst[m.FromField] = m.ToKey
		used[m.FieldType][m.ToKey] = true
	}
	return evaluators, annotations, nil
}

func (e ExptResultServiceImpl) projectionEvaluators(ctx context.Context, expt *entity.Experiment, turnIDs []int64, turns map[int64]*entity.ExptTurnResult, mappings map[string]string) (map[int64]map[string]*entity.EvaluatorRecord, error) {
	refs, err := e.ExptTurnResultRepo.BatchGetTurnEvaluatorResultRef(ctx, expt.SpaceID, turnIDs)
	if err != nil {
		return nil, projectionError(expt.SpaceID, expt.ID, 0, 0, "evaluator_refs", err)
	}
	ids := make([]int64, 0, len(refs))
	byRecord := make(map[int64]*entity.ExptTurnEvaluatorResultRef)
	instances := make(map[int64]map[string]bool)
	for _, ref := range refs {
		if ref == nil || ref.SpaceID != expt.SpaceID || ref.ExptID != expt.ID || turns[ref.ExptTurnResultID] == nil || ref.EvaluatorResultID <= 0 || ref.EvaluatorVersionID < 0 || byRecord[ref.EvaluatorResultID] != nil {
			return nil, projectionError(expt.SpaceID, expt.ID, 0, 0, "evaluator_refs", fmt.Errorf("invalid or duplicate evaluator reference"))
		}
		key := entity.EncodeEvaluatorInstanceKey(ref.EvaluatorVersionID, ref.Alias)
		inline := ref.SourceType == int32(entity.EvaluatorRecordSourceTypeInline)
		if inline {
			if ref.InlineKey == "" {
				return nil, projectionError(expt.SpaceID, expt.ID, 0, 0, "evaluator_refs", fmt.Errorf("inline reference has no key"))
			}
			key = "inline:" + ref.InlineKey
		}
		if instances[ref.ExptTurnResultID] == nil {
			instances[ref.ExptTurnResultID] = make(map[string]bool)
		}
		if instances[ref.ExptTurnResultID][key] || !inline && mappings[key] == "" {
			return nil, projectionError(expt.SpaceID, expt.ID, turns[ref.ExptTurnResultID].ItemID, turns[ref.ExptTurnResultID].TurnID, "evaluator_refs", fmt.Errorf("duplicate instance or missing mapping for %s", key))
		}
		instances[ref.ExptTurnResultID][key] = true
		byRecord[ref.EvaluatorResultID] = ref
		ids = append(ids, ref.EvaluatorResultID)
	}
	result := make(map[int64]map[string]*entity.EvaluatorRecord)
	if len(ids) == 0 {
		return result, nil
	}
	if e.evaluatorRecordService == nil && e.filterEvaluatorRepo == nil {
		return nil, projectionError(expt.SpaceID, expt.ID, 0, 0, "evaluators", fmt.Errorf("evaluator source missing"))
	}
	var records []*entity.EvaluatorRecord
	if e.filterEvaluatorRepo != nil {
		records, err = e.filterEvaluatorRepo.BatchGetEvaluatorRecord(ctx, ids, false, false)
	} else {
		records, err = e.evaluatorRecordService.BatchGetEvaluatorRecord(ctx, ids, false, false)
	}
	if err != nil {
		return nil, projectionError(expt.SpaceID, expt.ID, 0, 0, "evaluators", err)
	}
	seen := make(map[int64]bool)
	for _, record := range records {
		if record == nil {
			return nil, projectionError(expt.SpaceID, expt.ID, 0, 0, "evaluators", fmt.Errorf("nil record"))
		}
		ref := byRecord[record.ID]
		if ref == nil || seen[record.ID] {
			return nil, projectionError(expt.SpaceID, expt.ID, record.ItemID, record.TurnID, "evaluators", fmt.Errorf("unexpected or duplicate record"))
		}
		tr := turns[ref.ExptTurnResultID]
		if record.SpaceID <= 0 || (record.SpaceID != expt.SpaceID && record.Ext[consts.EvaluatorRecordExtKeyCallerSpaceID] != strconv.FormatInt(expt.SpaceID, 10)) || record.ExperimentID != expt.ID || record.ItemID != tr.ItemID || record.TurnID != tr.TurnID || record.ExperimentRunID <= 0 || record.EvaluatorVersionID != ref.EvaluatorVersionID || record.Alias != ref.Alias || record.InlineKey != ref.InlineKey || (record.SourceType == entity.EvaluatorRecordSourceTypeInline) != (ref.SourceType == int32(entity.EvaluatorRecordSourceTypeInline)) || (record.TargetRecordID != 0 && record.TargetRecordID != tr.TargetResultID) {
			return nil, projectionError(expt.SpaceID, expt.ID, tr.ItemID, tr.TurnID, "evaluators", fmt.Errorf("record ownership or instance mismatch"))
		}
		seen[record.ID] = true
		if record.SourceType == entity.EvaluatorRecordSourceTypeInline {
			continue
		}
		if result[tr.ID] == nil {
			result[tr.ID] = make(map[string]*entity.EvaluatorRecord)
		}
		result[tr.ID][entity.EncodeEvaluatorInstanceKey(record.EvaluatorVersionID, record.Alias)] = record
		seen[record.ID] = true
	}
	if len(seen) != len(ids) {
		return nil, projectionError(expt.SpaceID, expt.ID, 0, 0, "evaluators", fmt.Errorf("referenced evaluator record missing"))
	}
	return result, nil
}

func (e ExptResultServiceImpl) projectionTargets(ctx context.Context, expt *entity.Experiment, turns []*entity.ExptTurnResult, refs map[int64]*entity.ExptItemRef) (map[int64]*entity.EvalTargetRecord, error) {
	result := make(map[int64]*entity.EvalTargetRecord)
	if expt.ExptType == entity.ExptType_Online {
		return result, nil
	}
	groups := make(map[int64][]*entity.ExptTurnResult)
	for _, tr := range turns {
		if tr.TargetResultID == 0 {
			continue
		}
		space := resolveLoadSpaceID(expt.SpaceID, expt.TargetSpaceID)
		if ref := refs[tr.ItemID]; ref != nil && ref.ItemConfig != nil && ref.ItemConfig.TargetSourceSpaceID > 0 {
			space = ref.ItemConfig.TargetSourceSpaceID
		}
		groups[space] = append(groups[space], tr)
	}
	if len(groups) > 0 && e.evalTargetService == nil && e.evalTargetRepo == nil {
		return nil, projectionError(expt.SpaceID, expt.ID, 0, 0, "targets", fmt.Errorf("target source missing"))
	}
	for space, group := range groups {
		ids := make([]int64, 0, len(group))
		byID := make(map[int64]*entity.ExptTurnResult)
		for _, tr := range group {
			if byID[tr.TargetResultID] != nil {
				return nil, projectionError(expt.SpaceID, expt.ID, tr.ItemID, tr.TurnID, "targets", fmt.Errorf("target record referenced by multiple turns"))
			}
			ids = append(ids, tr.TargetResultID)
			byID[tr.TargetResultID] = tr
		}
		var records []*entity.EvalTargetRecord
		var err error
		if e.evalTargetRepo != nil {
			records, err = e.evalTargetRepo.ListEvalTargetRecordByIDsAndSpaceID(ctx, space, ids)
		} else {
			records, err = e.evalTargetService.BatchGetRecordByIDs(ctx, space, ids)
		}
		if err != nil {
			return nil, projectionError(expt.SpaceID, expt.ID, 0, 0, "targets", err)
		}
		for _, record := range records {
			if record == nil {
				return nil, projectionError(expt.SpaceID, expt.ID, 0, 0, "targets", fmt.Errorf("nil target record"))
			}
			tr := byID[record.ID]
			if tr == nil || result[tr.ID] != nil || record.SpaceID != space || record.ItemID != tr.ItemID || record.TurnID != tr.TurnID || record.ExperimentRunID <= 0 || record.EvalTargetOutputData == nil {
				return nil, projectionError(expt.SpaceID, expt.ID, record.ItemID, record.TurnID, "targets", fmt.Errorf("target missing output or ownership mismatch"))
			}
			version := expt.TargetVersionID
			if ref := refs[tr.ItemID]; ref != nil && ref.ItemConfig != nil && ref.ItemConfig.EvalTargetConf != nil {
				version = ref.ItemConfig.EvalTargetConf.TargetVersionID
			}
			if version > 0 && record.TargetVersionID != version {
				return nil, projectionError(expt.SpaceID, expt.ID, tr.ItemID, tr.TurnID, "targets", fmt.Errorf("target version mismatch"))
			}
			result[tr.ID] = record
		}
		for _, tr := range group {
			if result[tr.ID] == nil {
				return nil, projectionError(expt.SpaceID, expt.ID, tr.ItemID, tr.TurnID, "targets", fmt.Errorf("referenced target record missing"))
			}
		}
	}
	return result, nil
}

func (e ExptResultServiceImpl) projectionAnnotations(ctx context.Context, expt *entity.Experiment, turnIDs []int64, turns map[int64]*entity.ExptTurnResult, mappings map[string]string) (map[int64]map[int64]*entity.AnnotateRecord, error) {
	tags, err := e.ExptAnnotateRepo.GetExptTurnResultTagRefs(ctx, expt.ID, expt.SpaceID)
	if err != nil {
		return nil, projectionError(expt.SpaceID, expt.ID, 0, 0, "tag_refs", err)
	}
	bound := make(map[int64]bool)
	for _, tag := range tags {
		if tag == nil || tag.SpaceID != expt.SpaceID || tag.ExptID != expt.ID || tag.TagKeyID <= 0 || bound[tag.TagKeyID] {
			return nil, projectionError(expt.SpaceID, expt.ID, 0, 0, "tag_refs", fmt.Errorf("invalid or duplicate tag binding"))
		}
		bound[tag.TagKeyID] = true
	}
	refs, err := e.ExptAnnotateRepo.GetExptTurnAnnotateRecordRefsByTurnResultIDs(ctx, expt.SpaceID, turnIDs)
	if err != nil {
		return nil, projectionError(expt.SpaceID, expt.ID, 0, 0, "annotation_refs", err)
	}
	byID := make(map[int64]*entity.ExptTurnAnnotateRecordRef)
	ids := make([]int64, 0, len(refs))
	result := make(map[int64]map[int64]*entity.AnnotateRecord)
	for _, ref := range refs {
		if ref == nil || ref.SpaceID != expt.SpaceID || ref.ExptID != expt.ID || turns[ref.ExptTurnResultID] == nil {
			return nil, projectionError(expt.SpaceID, expt.ID, 0, 0, "annotation_refs", fmt.Errorf("invalid annotation reference"))
		}
		if !bound[ref.TagKeyID] {
			continue
		}
		if ref.AnnotateRecordID <= 0 || byID[ref.AnnotateRecordID] != nil || mappings[strconv.FormatInt(ref.TagKeyID, 10)] == "" {
			return nil, projectionError(expt.SpaceID, expt.ID, 0, 0, "annotation_refs", fmt.Errorf("duplicate reference or missing mapping"))
		}
		byID[ref.AnnotateRecordID] = ref
		ids = append(ids, ref.AnnotateRecordID)
	}
	if len(ids) == 0 {
		return result, nil
	}
	records, err := e.ExptAnnotateRepo.GetAnnotateRecordsByIDs(ctx, expt.SpaceID, ids)
	if err != nil {
		return nil, projectionError(expt.SpaceID, expt.ID, 0, 0, "annotations", err)
	}
	seen := make(map[int64]bool)
	for _, record := range records {
		if record == nil {
			return nil, projectionError(expt.SpaceID, expt.ID, 0, 0, "annotations", fmt.Errorf("nil record"))
		}
		ref := byID[record.ID]
		if ref == nil {
			continue
		}
		if seen[record.ID] || record.SpaceID != expt.SpaceID || record.ExperimentID != expt.ID || record.TagKeyID != ref.TagKeyID {
			return nil, projectionError(expt.SpaceID, expt.ID, 0, 0, "annotations", fmt.Errorf("record ownership or tag mismatch"))
		}
		if result[ref.ExptTurnResultID] == nil {
			result[ref.ExptTurnResultID] = make(map[int64]*entity.AnnotateRecord)
		}
		if result[ref.ExptTurnResultID][ref.TagKeyID] != nil {
			return nil, projectionError(expt.SpaceID, expt.ID, 0, 0, "annotations", fmt.Errorf("ambiguous annotation value"))
		}
		result[ref.ExptTurnResultID][ref.TagKeyID] = record
		seen[record.ID] = true
	}
	if len(seen) != len(ids) {
		return nil, projectionError(expt.SpaceID, expt.ID, 0, 0, "annotations", fmt.Errorf("referenced annotation record missing"))
	}
	return result, nil
}

func projectionAnnotation(row *entity.ExptTurnResultFilterEntity, key string, record *entity.AnnotateRecord) error {
	data := record.AnnotateData
	if data == nil {
		return nil
	}
	switch data.TagContentType {
	case entity.TagContentTypeContinuousNumber:
		if data.Score == nil || !projectionFinite(*data.Score) {
			return fmt.Errorf("numeric annotation requires a finite score")
		}
		row.AnnotationFloat[key] = *data.Score
	case entity.TagContentTypeFreeText:
		if data.TextValue == nil {
			return fmt.Errorf("text annotation requires text value")
		}
		row.AnnotationString[key] = *data.TextValue
	case entity.TagContentTypeCategorical, entity.TagContentTypeBoolean:
		row.AnnotationString[key] = strconv.FormatInt(record.TagValueID, 10)
	default:
		return fmt.Errorf("unknown annotation content type %q", data.TagContentType)
	}
	return nil
}

func (e ExptResultServiceImpl) projectionWeightedScore(ctx context.Context, expt *entity.Experiment, tr *entity.ExptTurnResult, records map[string]*entity.EvaluatorRecord) (*float64, error) {
	fail := func(cause string) (*float64, error) {
		return nil, projectionError(expt.SpaceID, expt.ID, tr.ItemID, tr.TurnID, "weighted_score", fmt.Errorf("%s", cause))
	}
	if tr.WeightedScore != nil {
		if !projectionFinite(*tr.WeightedScore) {
			return fail("nonfinite persisted weighted score")
		}
		score := *tr.WeightedScore
		return &score, nil
	}
	hasScore := false
	for _, record := range records {
		if record.GetScore() != nil {
			hasScore = true
		}
	}
	if !hasScore {
		return nil, nil
	}
	calculator, ok := e.scoreCalculator.(*evaluatorScoreCalculator)
	if !ok || calculator == nil {
		return fail("pure score calculator dependency unavailable")
	}
	// A projection must not re-run the external score hook or disguise its failure as zero.
	if calculator.configer != nil {
		_, hit := calculator.configer.GetExptTurnScoreHookConf(ctx, expt.SpaceID, expt.ID, buildEvaluatorVersionRefs(expt, records))
		if hit {
			return fail("external score hook requires a persisted weighted score")
		}
	}
	weights := buildScoreWeights(expt)
	for _, weight := range weights {
		if !projectionFinite(weight) {
			return fail("nonfinite weight")
		}
	}
	score := calculateWeightedScore(records, weights)
	if score != nil && !projectionFinite(*score) {
		return fail("nonfinite calculated weighted score")
	}
	return score, nil
}

var _ ExptFilterProjectionBuilder = ExptResultServiceImpl{}
