// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

// WithHookArchive copies the service; the legacy instance and interface stay unchanged.
func (e *ExptResultServiceImpl) WithHookArchive(storage repo.IHookItemArchiveRepo, scope string) (ExptResultService, error) {
	if e == nil || hookExecutionNil(storage) || scope == "" {
		return nil, entity.ErrHookStoreCorrupt
	}
	copy := *e
	copy.hookArchive, copy.hookArchiveScope = storage, scope
	return &copy, nil
}

func (e ExptResultServiceImpl) hookArchiveScores(ctx context.Context, expt *entity.Experiment, item *entity.HookTerminationItem, refs []*entity.ExptTurnEvaluatorResultRef, records map[int64]*entity.EvaluatorRecord) (map[int64]*float64, error) {
	if item.Execution != nil {
		expt = item.Execution
	}
	if hookExecutionNil(e.scoreCalculator) {
		return nil, entity.ErrHookFinalizationUnsettled
	}
	scores := map[int64]*float64{}
	for _, turn := range item.Manifest.Turns {
		byVersion := map[string]*entity.EvaluatorRecord{}
		for _, ref := range refs {
			if ref.ExptTurnResultID == turn.ResultID && ref.InlineKey == "" {
				byVersion[entity.EncodeEvaluatorInstanceKey(ref.EvaluatorVersionID, ref.Alias)] = records[ref.EvaluatorResultID]
			}
		}
		if len(byVersion) > 0 {
			if score := e.scoreCalculator.CalculateWeightedScore(ctx, expt, byVersion, buildScoreWeights(expt)); score != nil {
				scores[turn.ResultID] = score
			}
		}
	}
	return scores, nil
}

func hookArchiveReferences(item *entity.HookTerminationItem) ([]*entity.ExptTurnEvaluatorResultRef, error) {
	if item == nil || item.Manifest.Validate() != nil {
		return nil, entity.ErrHookStoreCorrupt
	}
	type refKey struct {
		turn, version int64
		alias, inline string
	}
	unique := map[refKey]*entity.ExptTurnEvaluatorResultRef{}
	var out []*entity.ExptTurnEvaluatorResultRef
	resultIDs := map[int64]int64{}
	for _, t := range item.Manifest.Turns {
		resultIDs[t.TurnID] = t.ResultID
	}
	for _, log := range item.Turns {
		resultID, ok := resultIDs[log.TurnID]
		if !ok {
			return nil, entity.ErrHookStoreCorrupt
		}
		refs := NewTurnEvaluatorResultRefs(0, item.Manifest.Key.ExperimentID, resultID, item.Manifest.Key.WorkspaceID, log.EvaluatorResultIds)
		if log.EvaluatorResultIds != nil && log.EvaluatorResultIds.IsNewFormat() {
			refs = append(refs, NewTurnEvaluatorResultRefs(0, item.Manifest.Key.ExperimentID, resultID, item.Manifest.Key.WorkspaceID, &entity.EvaluatorResults{EvalVerIDToResID: log.EvaluatorResultIds.EvalVerIDToResID})...)
		}
		for _, r := range refs {
			if r.EvaluatorResultID <= 0 || r.InlineKey == "" && r.EvaluatorVersionID <= 0 {
				return nil, entity.ErrHookStoreCorrupt
			}
			k := refKey{resultID, r.EvaluatorVersionID, r.Alias, r.InlineKey}
			if old, ok := unique[k]; ok {
				if old.EvaluatorResultID != r.EvaluatorResultID {
					return nil, entity.ErrHookStoreCorrupt
				}
				continue
			}
			unique[k] = r
			out = append(out, r)
		}
	}
	return out, nil
}

func (e ExptResultServiceImpl) recordHookItemRunLogs(ctx context.Context, key entity.HookRunKey, itemID int64, expt *entity.Experiment) ([]*entity.ExptTurnEvaluatorResultRef, error) {
	if expt == nil || expt.ID != key.ExperimentID || expt.SpaceID != key.WorkspaceID {
		return nil, entity.ErrHookStoreCorrupt
	}
	item, err := e.hookArchive.ReadHookArchiveItem(ctx, key, e.hookArchiveScope, itemID)
	if err != nil {
		return nil, err
	}
	if item.Item.ResultState == int32(entity.ExptItemResultStateResulted) {
		return nil, nil
	}
	refs, err := hookArchiveReferences(item)
	if err != nil {
		return nil, err
	}
	scores := map[int64]*float64{}
	if len(refs) > 0 {
		if hookExecutionNil(e.idgen) || hookExecutionNil(e.evaluatorRecordService) || hookExecutionNil(e.scoreCalculator) {
			return nil, entity.ErrHookFinalizationUnsettled
		}
		verified, err := e.readHookArchiveRecords(ctx, expt, item, refs)
		if err != nil {
			return nil, err
		}
		ids, err := e.idgen.GenMultiIDs(ctx, len(refs))
		if err != nil {
			return nil, err
		}
		if len(ids) != len(refs) {
			return nil, entity.ErrHookStoreCorrupt
		}
		for i := range refs {
			refs[i].ID = ids[i]
		}
		scores, err = e.hookArchiveScores(ctx, expt, item, refs, verified)
		if err != nil {
			return nil, err
		}
	}
	// No pre-read statuses are trusted at commit: the repository rechecks gate and snapshot.
	return e.hookArchive.ArchiveHookItem(ctx, entity.HookItemArchiveInput{Key: key, ExecutionScope: e.hookArchiveScope, ItemID: itemID, Prepared: item, Refs: refs, Scores: scores})
}

func (e ExptResultServiceImpl) readHookArchiveRecords(ctx context.Context, expt *entity.Experiment, item *entity.HookTerminationItem, refs []*entity.ExptTurnEvaluatorResultRef) (map[int64]*entity.EvaluatorRecord, error) {
	if item.Execution != nil {
		expt = item.Execution
	}
	if hookExecutionNil(e.evaluatorRecordService) {
		return nil, entity.ErrHookFinalizationUnsettled
	}
	key := item.Manifest.Key
	byID := map[int64]*entity.ExptTurnEvaluatorResultRef{}
	var ids []int64
	turns := map[int64]int64{}
	targets := map[int64]int64{}
	for _, mt := range item.Manifest.Turns {
		turns[mt.ResultID] = mt.TurnID
	}
	for _, tr := range item.Turns {
		targets[tr.TurnID] = tr.TargetResultID
	}
	for _, ref := range refs {
		if item.Execution != nil && ref.InlineKey == "" {
			matched := false
			if item.Execution.EvalSetSourceType != entity.ExptEvalSetSourceType_MultiSetConfig {
				matched = ref.Alias == "" && item.Execution.EvalConf.ConnectorConf.EvaluatorsConf.GetEvaluatorConf(ref.EvaluatorVersionID) != nil
			}
			for _, set := range item.Execution.EvalConf.EvalSetConfigs {
				for _, conf := range set.EvaluatorConfs {
					if conf.EvaluatorVersionID == ref.EvaluatorVersionID && conf.Alias == ref.Alias {
						matched = true
					}
				}
			}
			if !matched {
				return nil, entity.ErrHookStoreCorrupt
			}
		}
		if byID[ref.EvaluatorResultID] != nil {
			return nil, entity.ErrHookStoreCorrupt
		}
		byID[ref.EvaluatorResultID] = ref
		ids = append(ids, ref.EvaluatorResultID)
	}
	records, err := e.evaluatorRecordService.BatchGetEvaluatorRecord(ctx, ids, false, false)
	if err != nil {
		return nil, err
	}
	if len(records) != len(ids) {
		return nil, entity.ErrHookStoreCorrupt
	}
	out := map[int64]*entity.EvaluatorRecord{}
	for _, record := range records {
		if record == nil {
			return nil, entity.ErrHookStoreCorrupt
		}
		ref := byID[record.ID]
		if record.ExperimentRunID != key.RunID && record.Status != entity.EvaluatorRunStatusSuccess {
			return nil, entity.ErrHookStoreCorrupt
		}
		if ref == nil || out[record.ID] != nil || record.ExperimentID != key.ExperimentID || record.ExperimentRunID != item.Manifest.EvaluatorRecordRun(record.ID) || record.ItemID != item.Manifest.Frozen.ItemID || record.ItemVersionID != item.Manifest.Frozen.ItemVersionID || record.TurnID != turns[ref.ExptTurnResultID] || record.EvaluatorVersionID != ref.EvaluatorVersionID || record.Alias != ref.Alias || record.InlineKey != ref.InlineKey {
			return nil, entity.ErrHookStoreCorrupt
		}
		if ref.InlineKey != "" {
			if record.SourceType != entity.EvaluatorRecordSourceTypeInline || record.SpaceID != key.WorkspaceID || record.TargetRecordID <= 0 || record.TargetRecordID != targets[record.TurnID] {
				return nil, entity.ErrHookStoreCorrupt
			}
		} else {
			if record.SourceType != entity.EvaluatorRecordSourceTypeUnknown && record.SourceType != entity.EvaluatorRecordSourceTypeBuiltin || record.TargetRecordID != 0 {
				return nil, entity.ErrHookStoreCorrupt
			}
			if record.SpaceID != key.WorkspaceID {
				owner := int64(0)
				for _, ev := range expt.Evaluators {
					if ev != nil && ev.GetEvaluatorVersionID() == record.EvaluatorVersionID {
						owner = resolveEvaluatorSpaceID(ev, key.WorkspaceID)
						break
					}
				}
				if owner == 0 && !hookExecutionNil(e.evaluatorService) {
					ev, err := e.evaluatorService.GetEvaluatorVersion(ctx, nil, record.EvaluatorVersionID, false, false)
					if err != nil {
						return nil, err
					}
					if ev != nil && ev.GetEvaluatorVersionID() == record.EvaluatorVersionID {
						owner = resolveEvaluatorSpaceID(ev, key.WorkspaceID)
					}
				}
				if owner <= 0 || record.SpaceID != owner {
					return nil, entity.ErrHookStoreCorrupt
				}
			}
		}
		if record.Status != entity.EvaluatorRunStatusUnknown && record.Status != entity.EvaluatorRunStatusSuccess && record.Status != entity.EvaluatorRunStatusFail && record.Status != entity.EvaluatorRunStatusAsyncInvoking && record.Status != entity.EvaluatorRunStatusSkipped {
			return nil, entity.ErrHookStoreCorrupt
		}
		out[record.ID] = record
	}
	return out, nil
}
