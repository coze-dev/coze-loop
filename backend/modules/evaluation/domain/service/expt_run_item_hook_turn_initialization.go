// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func initializeHookPreEval(ctx context.Context, eiec *entity.ExptItemEvalCtx, ids idgen.IIDGenerator) (bool, error) {
	binding, managed := ctx.Value(itemHookProgressContextKey{}).(itemHookProgressBinding)
	if !managed {
		return false, nil
	}
	initializer, ok := binding.repo.(repo.IHookTurnLogInitializer)
	if !ok || hookExecutionNil(initializer) {
		return true, itemHookControlError{wait: true}
	}
	handled, committed, err := initializer.InitializeHookTurnRunLogs(ctx, binding.key, binding.itemID, binding.itemVersion, nil)
	if err != nil {
		return true, itemHookControlError{wait: !errors.Is(err, entity.ErrHookAdmissionDenied)}
	}
	if !handled {
		return false, nil
	}
	if len(committed) == 0 {
		if eiec.EvalSetItem == nil || len(eiec.EvalSetItem.Turns) == 0 || hookExecutionNil(ids) {
			return true, itemHookControlError{wait: true}
		}
		allocated, err := ids.GenMultiIDs(ctx, len(eiec.EvalSetItem.Turns))
		if err != nil {
			logs.CtxWarn(ctx, "Hook turn ID allocation failed, run=%d item=%d: %v", binding.key.RunID, binding.itemID, err)
			return true, itemHookControlError{wait: true}
		}
		if len(allocated) != len(eiec.EvalSetItem.Turns) {
			return true, itemHookControlError{wait: true}
		}
		candidates := make([]*entity.ExptTurnResultRunLog, 0, len(allocated))
		for i, turn := range eiec.EvalSetItem.Turns {
			if turn == nil {
				return true, itemHookControlError{wait: true}
			}
			candidates = append(candidates, &entity.ExptTurnResultRunLog{ID: allocated[i], SpaceID: binding.key.WorkspaceID, ExptID: binding.key.ExperimentID, ExptRunID: binding.key.RunID, ItemID: binding.itemID, ItemVersionID: binding.itemVersion, TurnID: turn.ID, Status: entity.TurnRunState_Processing, LogID: logs.GetLogID(ctx)})
		}
		handled, committed, err = initializer.InitializeHookTurnRunLogs(ctx, binding.key, binding.itemID, binding.itemVersion, candidates)
		if err != nil || !handled || len(committed) == 0 {
			return true, itemHookControlError{wait: !errors.Is(err, entity.ErrHookAdmissionDenied)}
		}
	}
	eiec.ExistItemEvalResult.TurnResultRunLogs = make(map[int64]*entity.ExptTurnResultRunLog, len(committed))
	for _, row := range committed {
		eiec.ExistItemEvalResult.TurnResultRunLogs[row.TurnID] = row
	}
	return true, nil
}
