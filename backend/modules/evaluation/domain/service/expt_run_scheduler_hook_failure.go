// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"time"
)

func (e *ExptSchedulerImpl) handleHookSchedulerFailures(ctx context.Context, event *entity.ExptScheduleEvent, items []*entity.ExptEvalItem, expt *entity.Experiment, zombie bool) (handled bool, alive, failed []*entity.ExptEvalItem, err error) {
	if e.hookScheduler == nil {
		if e.hookGate != nil {
			return true, nil, nil, schedulerHookRetryError{}
		}
		return false, nil, nil, nil
	}
	key := entity.HookRunKey{WorkspaceID: event.SpaceID, ExperimentID: event.ExptID, RunID: event.ExptRunID}
	source, err := e.hookScheduler.ReadFinalizationSource(ctx, key)
	if err != nil {
		return true, nil, nil, err
	}
	if source == nil || source.Key != key {
		return true, nil, nil, entity.ErrHookStoreCorrupt
	}
	if !source.Managed {
		return false, nil, nil, nil
	}
	storage, ok := e.hookScheduler.(repo.IHookSchedulerFailureRepo)
	if !ok {
		return true, nil, nil, entity.ErrHookFinalizationUnsupported
	}
	if !zombie && (e.evalTargetService == nil || !isSandboxAgentExpt(expt)) {
		return true, items, nil, nil
	}
	async := expt != nil && expt.AsyncExec()
	seconds := 0
	if zombie {
		seconds = e.Configer.GetConsumerConf(ctx).GetExptExecConf(event.SpaceID).GetExptItemEvalConf().GetItemZombieSecond(async)
	}
	cutoff := time.Now().Add(-time.Duration(seconds) * time.Second)
	for _, candidate := range items {
		if candidate == nil {
			continue
		}
		if candidate.State != entity.ItemRunState_Processing || zombie && (candidate.UpdatedAt == nil || candidate.UpdatedAt.IsZero() || !candidate.UpdatedAt.Before(cutoff)) {
			alive = append(alive, candidate)
			continue
		}
		item, err := storage.ReadHookSchedulerFailureItem(ctx, key, e.hookSchedulerScope, candidate.ItemID)
		if err != nil {
			return true, nil, nil, err
		}
		if item == nil {
			alive = append(alive, candidate)
			continue
		}
		if zombie && !item.Item.UpdatedAt.Before(cutoff) {
			alive = append(alive, candidate)
			continue
		}
		in := entity.HookSchedulerFailureInput{ExecutionScope: e.hookSchedulerScope, Item: item, Zombie: zombie, ZombieSeconds: seconds, Async: async, ExpiredBefore: cutoff}
		var recordIDs []int64
		for _, turn := range item.Turns {
			if turn.TargetResultID > 0 {
				recordIDs = append(recordIDs, turn.TargetResultID)
			}
		}
		if !zombie {
			ids, status := e.evalTargetService.CheckSandboxTerminated(ctx, resolveLoadSpaceID(key.WorkspaceID, source.Experiment.TargetSpaceID), recordIDs)
			if len(ids) == 0 {
				alive = append(alive, candidate)
				continue
			}
			in.ObservedTargetIDs = ids
			in.SandboxStatus = status[ids[0]]
			if in.SandboxStatus == "" {
				in.SandboxStatus = "Terminated"
			}
		}
		if err := e.loadHookSchedulerFailureRecords(ctx, source, item); err != nil {
			return true, nil, nil, err
		}
		var cleanup []*entity.EvalTargetRecord
		for _, record := range item.Targets {
			if s := gptr.Indirect(record.Status); s == entity.EvalTargetRunStatusAsyncInvoking || s == entity.EvalTargetRunStatusUnknown {
				cleanup = append(cleanup, record)
			}
		}
		if len(cleanup) > 0 {
			cleaner, ok := e.evalTargetService.(hookSchedulerTargetCleaner)
			if !ok {
				return true, nil, nil, entity.ErrHookFinalizationUnsupported
			}
			if err := cleaner.CleanupHookSchedulerTargets(ctx, key, cleanup, zombie); err != nil {
				return true, nil, nil, err
			}
		}
		changed, err := storage.ApplyHookSchedulerFailure(ctx, in)
		if err != nil {
			return true, nil, nil, err
		}
		if !changed {
			alive = append(alive, candidate)
			continue
		}
		copy := *candidate
		copy.State = entity.ItemRunState_Fail
		failed = append(failed, &copy)
		e.releaseCentralQuotaForItems(ctx, expt, key.RunID, []int64{candidate.ItemID}, "hook scheduler item failure")
		mapping := make(map[int64][]int64, len(recordIDs))
		for _, id := range recordIDs {
			mapping[id] = []int64{candidate.ItemID}
		}
		if zombie {
			if isSandboxAgentExpt(expt) {
				e.emitSandboxZombieInvokeFinished(ctx, event, expt, recordIDs, mapping)
			}
			if e.sandboxAgentNotifier != nil {
				if err := e.sandboxAgentNotifier.NotifyItemFail(ctx, expt, candidate.ItemID, errno.NewItemZombieTimeoutErr(seconds, async)); err != nil {
					logs.CtxWarn(ctx, "hook zombie notification failed: %v", err)
				}
			}
		} else {
			e.emitSandboxSweptInvokeFinished(ctx, event, expt, in.ObservedTargetIDs, mapping)
		}
	}
	return true, alive, failed, nil
}

func (e *ExptSchedulerImpl) loadHookSchedulerFailureRecords(ctx context.Context, source *entity.HookFinalizationSource, item *entity.HookTerminationItem) error {
	refs, err := hookArchiveReferences(item)
	if err != nil {
		return err
	}
	if len(refs) > 0 {
		result, ok := e.ResultSvc.(*ExptResultServiceImpl)
		if !ok {
			return entity.ErrHookFinalizationUnsupported
		}
		records, err := result.readHookArchiveRecords(ctx, source.Experiment, item, refs)
		if err != nil {
			return err
		}
		for _, record := range records {
			item.Evaluators = append(item.Evaluators, record)
		}
	}
	for _, turn := range item.Turns {
		if turn.TargetResultID == 0 {
			continue
		}
		if hookExecutionNil(e.evalTargetService) {
			return entity.ErrHookFinalizationUnsupported
		}
		record, err := e.evalTargetService.GetRecordByID(ctx, resolveLoadSpaceID(source.Key.WorkspaceID, source.Experiment.TargetSpaceID), turn.TargetResultID)
		if err != nil {
			return err
		}
		if record == nil || record.ID != turn.TargetResultID || record.ExperimentRunID != item.Manifest.TargetRecordRun(record.ID) || record.ItemID != item.Manifest.Frozen.ItemID || record.ItemVersionID != item.Manifest.Frozen.ItemVersionID || record.TurnID != turn.TurnID {
			return entity.ErrHookStoreCorrupt
		}
		item.Targets = append(item.Targets, record)
	}
	return nil
}
