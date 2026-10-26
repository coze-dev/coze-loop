// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0
package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/bytedance/gg/gptr"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// ExptResultPublicationService keeps external record reads and publication outside SQL.
// The callback is invoked after an item lock and source recheck, immediately before commit.
//
//go:generate mockgen -destination=mocks/expt_result_publish.go -package=mocks . ExptResultPublicationService
type ExptResultPublicationService interface {
	GetItemResultPublishSource(context.Context, int64, int64, int64, int64) (*entity.ExptItemResultPublishSource, error)
	PrepareItemResultPublication(context.Context, *entity.ExptItemResultPublishSource, *entity.Experiment) (*entity.ExptItemResultPublication, error)
	PublishPreparedItemResult(context.Context, *entity.ExptItemResultPublication, func(context.Context, *entity.ExptItemResultPublishSource) error) (bool, error)
}

func (e *ExptResultServiceImpl) GetItemResultPublishSource(ctx context.Context, space, expt, run, item int64) (*entity.ExptItemResultPublishSource, error) {
	if err := ctx.Err(); err != nil {
		return nil, errno.WrapMQRetryErr(err)
	}
	if e.resultPublishRepo == nil {
		return nil, errno.NewMQRetryErr("result publication repository missing")
	}
	source, err := e.resultPublishRepo.GetItemResultPublishSource(ctx, space, expt, run, item)
	if err != nil {
		return nil, errno.WrapMQRetryErr(err)
	}
	return source, nil
}

func publicationReady(source *entity.ExptItemResultPublishSource) bool {
	return source != nil && source.Source != nil && source.ItemLog != nil && source.ItemLog.ResultState == int32(entity.ExptItemResultStateLogged) && entity.IsItemRunFinished(entity.ItemRunState(source.ItemLog.Status))
}

func (e *ExptResultServiceImpl) PrepareItemResultPublication(ctx context.Context, source *entity.ExptItemResultPublishSource, expt *entity.Experiment) (*entity.ExptItemResultPublication, error) {
	if err := ctx.Err(); err != nil {
		return nil, errno.WrapMQRetryErr(err)
	}
	if !publicationReady(source) {
		return nil, nil
	}
	if source.Item == nil || len(source.Source.CanonicalTurns) == 0 || len(source.Source.CanonicalTurns) != len(source.Source.Turns) {
		return nil, errno.NewMQRetryErr("publication source incomplete")
	}
	ctx = contexts.WithCtxWriteDB(ctx)
	c := source.Source.Current
	if expt == nil || expt.ID != c.ExptID || expt.SpaceID != c.SpaceID {
		return nil, errno.NewMQRetryErr("publication experiment source mismatch")
	}
	publication := &entity.ExptItemResultPublication{Source: source}
	weights := buildScoreWeights(expt)
	for _, old := range source.Source.CanonicalTurns {
		if err := ctx.Err(); err != nil {
			return nil, errno.WrapMQRetryErr(err)
		}
		if old == nil {
			return nil, errno.NewMQRetryErr("publication canonical turn missing")
		}
		log := source.Source.Turns[old.TurnID]
		if log == nil {
			return nil, errno.NewMQRetryErr("publication frozen turn missing")
		}
		turn := *old
		turn.Status = int32(log.Status)
		turn.TargetResultID = log.TargetResultID
		turn.TraceID = log.TraceID
		turn.LogID = log.LogID
		turn.ErrMsg = log.ErrMsg
		turn.ExptRunID = c.RunID
		turn.WeightedScore = nil
		if log.TargetResultID > 0 {
			targetSpace := resolveLoadSpaceID(c.SpaceID, expt.TargetSpaceID)
			if expt.EvalSetSourceType == entity.ExptEvalSetSourceType_MultiSetConfig {
				if source.ItemRef == nil || source.ItemRef.ItemConfig == nil {
					return nil, errno.NewMQRetryErr("publication frozen item configuration missing")
				}
				if source.ItemRef.ItemConfig.TargetSourceSpaceID > 0 {
					targetSpace = source.ItemRef.ItemConfig.TargetSourceSpaceID
				}
			}
			if e.evalTargetService == nil {
				return nil, errno.NewMQRetryErr("publication target reader missing")
			}
			record, err := e.evalTargetService.GetRecordByID(ctx, targetSpace, log.TargetResultID)
			if err != nil {
				return nil, errno.WrapMQRetryErr(err)
			}
			if record == nil || record.ID != log.TargetResultID || record.ItemID != c.ItemID || record.TurnID != log.TurnID || record.SpaceID != targetSpace || record.ExperimentRunID <= 0 || record.Status == nil || *record.Status < entity.EvalTargetRunStatusSuccess || *record.Status > entity.EvalTargetRunStatusAsyncInvoking || record.ItemVersionID != 0 && record.ItemVersionID != c.ItemVersionID {
				return nil, errno.NewMQRetryErr("publication target source record incomplete or foreign")
			}
		}
		if err := ctx.Err(); err != nil {
			return nil, errno.WrapMQRetryErr(err)
		}
		refs := NewTurnEvaluatorResultRefs(0, c.ExptID, old.ID, c.SpaceID, log.EvaluatorResultIds)
		ids := make([]int64, 0, len(refs))
		seen := map[int64]bool{}
		for _, ref := range refs {
			if ref == nil || ref.EvaluatorResultID <= 0 {
				return nil, errno.NewMQRetryErr("publication evaluator ref incomplete")
			}
			if !seen[ref.EvaluatorResultID] {
				ids = append(ids, ref.EvaluatorResultID)
				seen[ref.EvaluatorResultID] = true
			}
		}
		scoreRecords := map[string]*entity.EvaluatorRecord{}
		if len(ids) > 0 {
			if e.evaluatorRecordService == nil {
				return nil, errno.NewMQRetryErr("publication evaluator reader missing")
			}
			records, err := e.evaluatorRecordService.BatchGetEvaluatorRecord(ctx, ids, false, false)
			if err != nil {
				return nil, errno.WrapMQRetryErr(err)
			}
			if err := ctx.Err(); err != nil {
				return nil, errno.WrapMQRetryErr(err)
			}
			byID := map[int64]*entity.EvaluatorRecord{}
			for _, record := range records {
				if record == nil || byID[record.ID] != nil {
					return nil, errno.NewMQRetryErr("publication evaluator records incomplete or duplicated")
				}
				byID[record.ID] = record
			}
			for _, ref := range refs {
				record := byID[ref.EvaluatorResultID]
				if record == nil || record.ItemID != c.ItemID || record.TurnID != log.TurnID || record.ExperimentID != c.ExptID || record.ExperimentRunID <= 0 || record.SpaceID <= 0 || record.Status < entity.EvaluatorRunStatusSuccess || record.Status > entity.EvaluatorRunStatusSkipped || record.ItemVersionID != 0 && record.ItemVersionID != c.ItemVersionID || record.TargetRecordID != 0 && record.TargetRecordID != log.TargetResultID {
					return nil, errno.NewMQRetryErr("publication evaluator source record incomplete or foreign")
				}
				if ref.SourceType == int32(entity.EvaluatorRecordSourceTypeInline) {
					if record.SourceType != entity.EvaluatorRecordSourceTypeInline || record.InlineKey != ref.InlineKey {
						return nil, errno.NewMQRetryErr("publication inline source mismatch")
					}
				} else {
					if record.SourceType == entity.EvaluatorRecordSourceTypeInline || record.EvaluatorVersionID != ref.EvaluatorVersionID || record.Alias != ref.Alias {
						return nil, errno.NewMQRetryErr("publication registered source mismatch")
					}
					if record.Status == entity.EvaluatorRunStatusSuccess {
						scoreRecords[entity.EncodeEvaluatorInstanceKey(record.EvaluatorVersionID, record.Alias)] = record
					}
				}
			}
		}
		if len(scoreRecords) > 0 {
			if e.scoreCalculator != nil {
				turn.WeightedScore = e.scoreCalculator.CalculateWeightedScore(ctx, expt, scoreRecords, weights)
			} else {
				turn.WeightedScore = calculateWeightedScore(scoreRecords, weights)
			}
		}
		publication.Turns = append(publication.Turns, &turn)
		publication.Refs = append(publication.Refs, refs...)
	}
	sort.Slice(publication.Refs, func(i, j int) bool {
		a, b := publication.Refs[i], publication.Refs[j]
		if a.ExptTurnResultID != b.ExptTurnResultID {
			return a.ExptTurnResultID < b.ExptTurnResultID
		}
		if a.EvaluatorVersionID != b.EvaluatorVersionID {
			return a.EvaluatorVersionID < b.EvaluatorVersionID
		}
		if a.Alias != b.Alias {
			return a.Alias < b.Alias
		}
		return a.InlineKey < b.InlineKey
	})
	var missing []*entity.ExptTurnEvaluatorResultRef
	for _, ref := range publication.Refs {
		for _, old := range source.Source.CurrentRefs {
			if old.ExptTurnResultID == ref.ExptTurnResultID && old.EvaluatorVersionID == ref.EvaluatorVersionID && old.EvaluatorResultID == ref.EvaluatorResultID && old.Alias == ref.Alias && old.InlineKey == ref.InlineKey && old.SourceType == ref.SourceType {
				ref.ID = old.ID
				break
			}
		}
		if ref.ID == 0 {
			missing = append(missing, ref)
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, errno.WrapMQRetryErr(err)
	}
	if len(missing) > 0 {
		if e.idgen == nil {
			return nil, errno.NewMQRetryErr("publication reference ID generator missing")
		}
		ids, err := e.idgen.GenMultiIDs(ctx, len(missing))
		if err != nil {
			return nil, errno.WrapMQRetryErr(err)
		}
		if len(ids) != len(missing) {
			return nil, errno.NewMQRetryErr("publication ref IDs incomplete")
		}
		for i, ref := range missing {
			if ids[i] <= 0 {
				return nil, errno.NewMQRetryErr("publication ref ID invalid")
			}
			ref.ID = ids[i]
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, errno.WrapMQRetryErr(err)
	}
	return publication, nil
}

func (e *ExptResultServiceImpl) PublishPreparedItemResult(ctx context.Context, publication *entity.ExptItemResultPublication, beforeCommit func(context.Context, *entity.ExptItemResultPublishSource) error) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, errno.WrapMQRetryErr(err)
	}
	if publication == nil {
		return false, nil
	}
	if !publicationReady(publication.Source) {
		return false, errno.NewMQRetryErr("publication prepared source invalid")
	}
	if e.resultPublishRepo == nil || e.resultPublishLocker == nil {
		return false, errno.NewMQRetryErr("publication repository or independent item locker missing")
	}
	c := publication.Source.Source.Current
	locker := e.resultPublishLocker()
	if locker == nil {
		return false, errno.NewMQRetryErr("publication item locker missing")
	}
	key := fmt.Sprintf("expt_item_eval_run_lock:%d:%d", c.ExptID, c.ItemID)
	locked, lockCtx, cancel, err := locker.LockWithRenew(ctx, key, 5*time.Second, time.Minute)
	if err != nil {
		return false, errno.WrapMQRetryErr(err)
	}
	if !locked {
		return false, errno.NewMQRetryErr("publication item lock busy")
	}
	defer cancel()
	defer func() {
		if _, err := locker.Unlock(key); err != nil {
			logs.CtxWarn(ctx, "unlock publication item: %v", err)
		}
	}()
	valid, err := e.resultPublishRepo.IsItemResultPublishSourceCurrent(lockCtx, publication.Source)
	if err != nil {
		return false, errno.WrapMQRetryErr(err)
	}
	if !valid {
		return false, nil
	}
	if err := lockCtx.Err(); err != nil {
		return false, errno.WrapMQRetryErr(err)
	}
	if beforeCommit != nil {
		if err := beforeCommit(lockCtx, publication.Source); err != nil {
			return false, err
		}
	}
	if err := lockCtx.Err(); err != nil {
		return false, errno.WrapMQRetryErr(err)
	}
	applied, err := e.resultPublishRepo.CommitItemResult(lockCtx, publication)
	if err != nil {
		return false, errno.WrapMQRetryErr(err)
	}
	return applied, nil
}

func (e *ExptSchedulerImpl) recordCurrentItemResults(ctx context.Context, event *entity.ExptScheduleEvent, items []*entity.ExptEvalItem, mode entity.ExptSchedulerMode, expt *entity.Experiment, service ExptResultPublicationService) error {
	if err := ctx.Err(); err != nil {
		return errno.WrapMQRetryErr(err)
	}
	var itemMeta map[int64]*entity.EvaluationSetItem
	var itemVer map[int64]int64
	if e.itemCompletePublisher != nil && len(items) > 0 {
		itemMeta, itemVer = e.resolveItemCompleteMeta(ctx, event, items, expt)
	}
	appliedItems := []int64{}
	for _, item := range items {
		if item == nil {
			return errno.NewMQRetryErr("nil scanned publication item")
		}
		source := item.Publication
		if source == nil && item.PublicationObserved {
			continue
		}
		if source == nil {
			// Legacy callers do not carry the new snapshot. Freeze before preparation,
			// matching the original scanned token/status; never attach a newer request.
			var err error
			source, err = service.GetItemResultPublishSource(ctx, event.SpaceID, event.ExptID, event.ExptRunID, item.ItemID)
			if err != nil {
				return err
			}
			if source == nil || source.Source.Current.RequestID != item.RequestID || source.ItemLog.Status != int32(item.State) {
				continue
			}
			if item.Source != nil && !sameExecutionIdentity(item.Source, source.Source) {
				continue
			}
		}
		if !publicationReady(source) {
			continue
		}
		current := source.Source.Current
		if current.SpaceID != event.SpaceID || current.ExptID != event.ExptID || current.RunID != event.ExptRunID || current.ItemID != item.ItemID || current.RequestID != item.RequestID {
			return errno.NewMQRetryErr("publication execution scope mismatch")
		}
		publication, err := service.PrepareItemResultPublication(ctx, source, expt)
		if err != nil {
			return err
		}
		applied, err := service.PublishPreparedItemResult(ctx, publication, func(lockCtx context.Context, source *entity.ExptItemResultPublishSource) error {
			if e.itemCompletePublisher == nil || entity.ItemRunState(source.ItemLog.Status) != entity.ItemRunState_Success {
				return nil
			}
			current := *item
			current.State = entity.ItemRunState(source.ItemLog.Status)
			current.RequestID = source.Source.Current.RequestID
			return e.sendItemComplete(lockCtx, event, expt, &current, itemMeta[item.ItemID], itemVer[item.ItemID])
		})
		if err != nil {
			return err
		}
		if !applied {
			continue
		}
		appliedItems = append(appliedItems, item.ItemID)
		// Existing PublishResult failure remains best effort after commit. Task4.3
		// handles the final projection repair; Resulted is not scanned as Logged.
		if err := ctx.Err(); err != nil {
			return errno.WrapMQRetryErr(err)
		}
		if err := mode.PublishResult(ctx, publication.Refs, event); err != nil {
			logs.CtxError(ctx, "publish online result after commit: %v", err)
		}
	}
	if len(appliedItems) == 0 {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return errno.WrapMQRetryErr(err)
	}
	if err := e.ResultSvc.UpsertExptTurnResultFilter(ctx, event.SpaceID, event.ExptID, appliedItems); err != nil {
		logs.CtxError(ctx, "UpsertExptTurnResultFilter after commit: %v", err)
	}
	if e.Publisher == nil {
		return nil
	}
	return e.Publisher.PublishExptTurnResultFilterEvent(ctx, &entity.ExptTurnResultFilterEvent{ExperimentID: event.ExptID, SpaceID: event.SpaceID, ItemID: appliedItems, RetryTimes: gptr.Of(int32(0)), FilterType: gptr.Of(entity.UpsertExptTurnResultFilterTypeCheck)}, gptr.Of(10*time.Second))
}
