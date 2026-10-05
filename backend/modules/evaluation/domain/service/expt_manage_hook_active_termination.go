// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

func (e *ExptMangerImpl) prepareActiveHookTermination(ctx context.Context, source *entity.HookFinalizationSource, stored *entity.HookStoredRun) error {
	if !hookTerminationStatus(stored.State.Intent.Status) {
		return nil
	}
	if storage, ok := e.finalization.Repository.(repo.IHookPartialInitializationFinalizer); ok {
		if handled, err := storage.PreparePartialInitializationTermination(ctx, source.Key, e.finalization.ExecutionScope); handled || err != nil {
			return err
		}
	}
	// The never-initialized path retains its existing absence proof and zero-count semantics.
	stats, err := e.finalization.readStats(ctx, source.Key)
	if err == nil && stats.NeverAdmitted {
		return nil
	}
	storage, ok := e.finalization.Repository.(repo.IHookActiveTerminationRepo)
	if !ok {
		return entity.ErrHookFinalizationUnsettled
	}
	digest := entity.NewHookPlanDigest()
	for start := int64(0); ; {
		page, err := storage.ReadTerminationPage(ctx, entity.HookPlanReadInput{Key: source.Key, ExecutionScope: e.finalization.ExecutionScope, StartOrdinal: start, Limit: 100})
		if err != nil {
			return err
		}
		if page == nil || page.NextOrdinal != start+int64(len(page.Items)) || page.HasMore && len(page.Items) == 0 {
			return entity.ErrHookStoreCorrupt
		}
		for i, item := range page.Items {
			if item.Manifest == nil || item.Ordinal != start+int64(i) || item.Manifest.Key != source.Key || item.Manifest.Frozen != item.Frozen {
				return entity.ErrHookStoreCorrupt
			}
			digest, err = entity.AppendHookPlanDigest(digest, []entity.HookPlanItem{item.Frozen})
			if err != nil {
				return err
			}
			if err = e.prepareActiveHookItem(ctx, storage, source, item.Frozen.ItemID); err != nil {
				return err
			}
		}
		if !page.HasMore {
			if page.NextOrdinal != page.Count || digest.Hash != page.Hash {
				return entity.ErrHookStoreCorrupt
			}
			return nil
		}
		start = page.NextOrdinal
	}
}

func (e *ExptMangerImpl) prepareActiveHookItem(ctx context.Context, storage repo.IHookActiveTerminationRepo, source *entity.HookFinalizationSource, itemID int64) (resultErr error) {
	key := source.Key
	// Same narrow lock as the ordinary item consumer; a busy item leaves durable Pending.
	lockKey := fmt.Sprintf("expt_item_eval_run_lock:%d:%d", key.ExperimentID, itemID)
	if e.finalization.NewItemLocker == nil {
		return entity.ErrHookFinalizationUnsettled
	}
	locker := e.finalization.NewItemLocker()
	if hookExecutionNil(locker) {
		return entity.ErrHookFinalizationUnsettled
	}
	locked, lockedCtx, cancel, err := locker.LockWithRenew(ctx, lockKey, 5*time.Second, time.Hour)
	if err != nil {
		return err
	}
	if !locked {
		return entity.ErrHookFinalizationUnsettled
	}
	defer func() {
		cancel()
		if _, err := locker.Unlock(lockKey); resultErr == nil && err != nil {
			resultErr = err
		}
	}()
	item, err := storage.PrepareTerminationItem(lockedCtx, key, e.finalization.ExecutionScope, itemID)
	if err != nil {
		return err
	}
	if item == nil || item.Manifest.Key != key || item.Manifest.Frozen.ItemID != itemID {
		return entity.ErrHookStoreCorrupt
	}
	execution := source.Experiment
	if item.Execution != nil {
		execution = item.Execution
	}
	refs, err := hookArchiveReferences(item)
	if err != nil {
		return err
	}
	var scores map[int64]*float64
	if len(refs) > 0 {
		result, ok := e.exptResultService.(*ExptResultServiceImpl)
		if !ok {
			return entity.ErrHookFinalizationUnsettled
		}
		records, err := result.readHookArchiveRecords(lockedCtx, source.Experiment, item, refs)
		if err != nil {
			return err
		}
		for _, record := range records {
			item.Evaluators = append(item.Evaluators, record)
		}
		if item.Item.ResultState != int32(entity.ExptItemResultStateResulted) {
			scores, err = result.hookArchiveScores(lockedCtx, source.Experiment, item, refs, records)
			if err != nil {
				return err
			}
		}
	}
	for _, tr := range item.Turns {
		if tr.TargetResultID == 0 {
			continue
		}
		if hookExecutionNil(e.evalTargetService) {
			return entity.ErrHookFinalizationUnsettled
		}
		targetSpace := resolveLoadSpaceID(key.WorkspaceID, execution.TargetSpaceID)
		record, err := e.evalTargetService.GetRecordByID(lockedCtx, targetSpace, tr.TargetResultID)
		if err != nil {
			return err
		}
		if record == nil || record.ID != tr.TargetResultID || record.SpaceID != targetSpace || record.ExperimentRunID != item.Manifest.TargetRecordRun(record.ID) || record.ItemID != itemID || record.ItemVersionID != item.Manifest.Frozen.ItemVersionID || record.TurnID != tr.TurnID || record.TargetID != execution.TargetID || record.TargetVersionID != execution.TargetVersionID {
			return entity.ErrHookStoreCorrupt
		}
		item.Targets = append(item.Targets, record)
	}
	if len(item.Targets) > 0 || len(item.Evaluators) > 0 {
		closer, ok := e.finalization.Repository.(repo.IHookTerminationRecordRepo)
		if !ok {
			return entity.ErrHookFinalizationUnsettled
		}
		if err = closer.CloseHookExecutionRecords(lockedCtx, e.finalization.ExecutionScope, item); err != nil {
			return err
		}
	}
	var ownedTargets []*entity.EvalTargetRecord
	for _, record := range item.Targets {
		if record.ExperimentRunID == key.RunID {
			ownedTargets = append(ownedTargets, record)
		}
	}
	if len(ownedTargets) > 0 {
		cleaner, ok := e.evalTargetService.(hookTargetSandboxCleaner)
		if !ok {
			return entity.ErrHookFinalizationUnsettled
		}
		if err = cleaner.CleanupHookTargetSandboxes(lockedCtx, key, ownedTargets); err != nil {
			return err
		}
	}
	if err = lockedCtx.Err(); err != nil {
		return err
	}
	if len(refs) > 0 {
		if hookExecutionNil(e.idgenerator) {
			return entity.ErrHookFinalizationUnsettled
		}
		ids, err := e.idgenerator.GenMultiIDs(lockedCtx, len(refs))
		if err != nil {
			return err
		}
		if len(ids) != len(refs) {
			return entity.ErrHookStoreCorrupt
		}
		for i := range refs {
			refs[i].ID = ids[i]
		}
	}
	_, err = storage.ArchiveHookItem(lockedCtx, entity.HookItemArchiveInput{Key: key, ExecutionScope: e.finalization.ExecutionScope, ItemID: itemID, Cancellation: true, Prepared: item, Refs: refs, Scores: scores})
	return err
}
