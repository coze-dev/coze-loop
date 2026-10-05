// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

// The legacy RetryItems PreEval writes through this bound port, preserving its interface.
type hookRetryItemsTurnWriter struct {
	repo.IExptTurnResultRepo
	initializer repo.IHookTurnLogInitializer
	key         entity.HookRunKey
}

func (w *hookRetryItemsTurnWriter) BatchCreateNXRunLog(ctx context.Context, candidates []*entity.ExptTurnResultRunLog) error {
	if ctx == nil || hookExecutionNil(w.initializer) {
		return entity.ErrHookExecutionUnsupported
	}
	b, ok := ctx.Value(itemHookProgressContextKey{}).(itemHookProgressBinding)
	if !ok || b.key != w.key || len(candidates) == 0 || candidates[0] == nil {
		return entity.ErrHookAdmissionDenied
	}
	handled, committed, err := w.initializer.InitializeHookTurnRunLogs(ctx, w.key, b.itemID, b.itemVersion, candidates)
	if err != nil {
		return err
	}
	if !handled || len(committed) != len(candidates) {
		return entity.ErrHookStoreCorrupt
	}
	byTurn := make(map[int64]*entity.ExptTurnResultRunLog, len(committed))
	for _, row := range committed {
		byTurn[row.TurnID] = row
	}
	for _, row := range candidates {
		if row == nil || byTurn[row.TurnID] == nil {
			return entity.ErrHookStoreCorrupt
		}
		*row = *byTurn[row.TurnID]
	}
	return nil
}

func initializeBoundHookRetry(ctx context.Context, item *entity.ExptItemEvalCtx, ids idgen.IIDGenerator, target IEvalTargetService, records EvaluatorRecordService, turns repo.IExptTurnResultRepo) (bool, error) {
	if item.HookManifest == nil || !entity.HookBoundRetryMode(item.Event.ExptRunMode) {
		return false, nil
	}
	if item.HookManifest.Retry == nil || item.Event.ExptRunMode == entity.EvaluationModeRetryAll || item.Event.ExptRunMode == entity.EvaluationModeRetryItems {
		return initializeHookPreEval(ctx, item, ids)
	}
	b, ok := ctx.Value(itemHookProgressContextKey{}).(itemHookProgressBinding)
	if !ok {
		return true, itemHookControlError{wait: true}
	}
	writer, ok := b.repo.(repo.IHookRetryTurnLogInitializer)
	if !ok {
		return true, itemHookControlError{wait: true}
	}
	m, logs, err := writer.InitializeHookRetryTurnRunLogs(ctx, b.key, b.itemID, b.itemVersion, nil)
	if err != nil {
		return true, err
	}
	if len(logs) == 0 {
		allocated, err := ids.GenMultiIDs(ctx, len(m.Turns))
		if err != nil {
			return true, err
		}
		if len(allocated) != len(m.Turns) {
			return true, entity.ErrHookStoreCorrupt
		}
		sourceLogs := &retryEvaluatorSourceLogs{repo: turns, event: item.Event, logsByRun: make(map[int64]map[int64]*entity.ExptTurnResultRunLog)}
		candidates := make([]*entity.ExptTurnResultRunLog, 0, len(m.Turns))
		for i, source := range m.Retry.Turns {
			prior := &entity.ExptTurnResult{ID: source.ResultID, SpaceID: b.key.WorkspaceID, ExptID: b.key.ExperimentID, ExptRunID: source.RunID, ItemID: b.itemID, ItemVersionID: b.itemVersion, TurnID: source.TurnID, TargetResultID: source.TargetResultID}
			targetID, refs, err := failRetrySelectTurnRunLogRefs(ctx, resolveLoadSpaceID(b.key.WorkspaceID, item.TargetSourceSpaceID()), !shouldSkipTargetNode(item.Expt), prior, target, records, source.Refs, sourceLogs)
			if err != nil {
				return true, err
			}
			candidates = append(candidates, &entity.ExptTurnResultRunLog{ID: allocated[i], SpaceID: b.key.WorkspaceID, ExptID: b.key.ExperimentID, ExptRunID: b.key.RunID, ItemID: b.itemID, ItemVersionID: b.itemVersion, TurnID: source.TurnID, Status: entity.TurnRunState_Processing, TargetResultID: targetID, EvaluatorResultIds: refs})
		}
		m, logs, err = writer.InitializeHookRetryTurnRunLogs(ctx, b.key, b.itemID, b.itemVersion, candidates)
		if err != nil {
			return true, err
		}
	}
	item.HookManifest = m
	item.ExistItemEvalResult.TurnResultRunLogs = make(map[int64]*entity.ExptTurnResultRunLog, len(logs))
	for _, log := range logs {
		item.ExistItemEvalResult.TurnResultRunLogs[log.TurnID] = log
	}
	return true, nil
}
