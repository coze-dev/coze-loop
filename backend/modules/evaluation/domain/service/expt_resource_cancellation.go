// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func (e *ExptMangerImpl) usesCentralResourceProtocol(expt *entity.Experiment) bool {
	_, ok := e.centralGuard.(component.ICentralItemExecution)
	return ok && expt != nil && entity.IsCentralDispatch(expt.ExptDispatchMode)
}

func (e *ExptMangerImpl) cancelCentralItems(ctx context.Context, expt *entity.Experiment, runID *int64, itemIDs []int64) error {
	items, ok := e.itemResultRepo.(repo.IExptItemCancellationRepo)
	if !ok || e.mutex == nil {
		return fmt.Errorf("central item cancellation not wired")
	}
	if _, ok := e.centralGuard.(component.ICentralItemLifecycle); !ok {
		return fmt.Errorf("central item lifecycle not wired")
	}
	var cursor int64
	for {
		rows, next, err := items.ScanItemRunLogsForCancellation(contexts.WithCtxWriteDB(ctx), expt.SpaceID, expt.ID, runID, itemIDs, cursor, 100)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if row == nil || row.ID <= 0 || row.SpaceID != expt.SpaceID || row.ExptID != expt.ID || (runID != nil && row.ExptRunID != *runID) {
				return fmt.Errorf("central cancellation item identity mismatch")
			}
			if err := e.cancelCentralItem(ctx, expt, row, items); err != nil {
				return err
			}
		}
		if next == 0 {
			return nil
		}
		if next <= cursor {
			return fmt.Errorf("central cancellation cursor did not advance")
		}
		cursor = next
	}
}

func (e *ExptMangerImpl) cancelCentralItem(ctx context.Context, expt *entity.Experiment, row *entity.ExptItemResultRunLog, items repo.IExptItemCancellationRepo) error {
	key := fmt.Sprintf("expt_item_eval_run_lock:%d:%d", expt.ID, row.ItemID)
	locked, ctx, cancel, err := e.mutex.LockWithRenew(ctx, key, 5*time.Second, 5*time.Minute)
	if err != nil {
		return err
	}
	if !locked {
		return fmt.Errorf("central cancellation item is locked")
	}
	defer func() {
		cancel()
		if _, err := e.mutex.Unlock(key); err != nil {
			logs.CtxWarn(ctx, "[EvaluationResource] cancellation unlock failed, item_run_log_id=%d err=%v", row.ID, err)
		}
	}()
	ctx = contexts.WithCtxWriteDB(ctx)
	current, err := e.itemResultRepo.GetItemRunLog(ctx, expt.ID, row.ExptRunID, row.ItemID, expt.SpaceID)
	if err != nil {
		return err
	}
	if current == nil || current.ID != row.ID {
		return fmt.Errorf("central cancellation immutable item identity mismatch")
	}
	state := entity.ItemRunState(current.Status)
	if state == entity.ItemRunState_Success || state == entity.ItemRunState_Fail {
		return nil
	}
	if state != entity.ItemRunState_Terminal {
		if e.turnResultRepo == nil {
			return fmt.Errorf("central cancellation turn repository not wired")
		}
		if err := e.turnResultRepo.CreateOrUpdateItemsTurnRunLogStatus(ctx, expt.SpaceID, expt.ID, row.ExptRunID, []int64{row.ItemID}, entity.TurnRunState_Terminal); err != nil {
			return err
		}
		if err := items.UpdateItemRunLogIfNonterminal(ctx, expt.ID, row.ExptRunID, []int64{row.ItemID}, map[string]any{
			"status": int32(entity.ItemRunState_Terminal), "result_state": int32(entity.ExptItemResultStateLogged), "err_msg": errno.SerializeErr(errno.NewItemManuallyTerminatedErr()),
		}, expt.SpaceID); err != nil {
			return err
		}
		current, err = e.itemResultRepo.GetItemRunLog(ctx, expt.ID, row.ExptRunID, row.ItemID, expt.SpaceID)
		if err != nil {
			return err
		}
		if current == nil || current.ID != row.ID {
			return fmt.Errorf("central cancellation item disappeared")
		}
		state = entity.ItemRunState(current.Status)
		if state == entity.ItemRunState_Success || state == entity.ItemRunState_Fail {
			return nil
		}
		if state != entity.ItemRunState_Terminal {
			return fmt.Errorf("central cancellation Terminal did not persist")
		}
	}
	// Retrying Terminal items must retry physical stop; Terminal alone cannot release running budget.
	request, err := resourceRequestForItem(ctx, e.itemResultRepo, current)
	if errors.Is(err, errExptNonCurrentSource) {
		rows, readErr := e.itemResultRepo.BatchGet(ctx, current.SpaceID, current.ExptID, []int64{current.ItemID})
		if readErr != nil {
			return readErr
		}
		if len(rows) != 1 || rows[0] == nil || rows[0].ExptRunID == current.ExptRunID {
			return err
		}
		historical, ok := e.centralGuard.(component.ICentralHistoricalItemLifecycle)
		if !ok {
			return fmt.Errorf("central historical item lifecycle not wired: %w", err)
		}
		if err := e.stopCentralItemResources(ctx, expt, current.ExptRunID, current.ItemID); err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		return historical.OnHistoricalItemTerminal(ctx, current)
	}
	if err != nil || request == nil {
		if err != nil {
			return err
		}
		return fmt.Errorf("central cancellation source generation unavailable")
	}
	ctx = component.WithEvaluationResourceRequestID(ctx, request.RequestID)
	if err := e.stopCentralItemResources(ctx, expt, row.ExptRunID, row.ItemID); err != nil {
		return err
	}
	notifyCentralItemTerminalForSource(ctx, e.centralGuard, e.itemResultRepo, request, row.ID, true)
	return nil
}

func (e *ExptMangerImpl) stopCentralItemResources(ctx context.Context, expt *entity.Experiment, runID, itemID int64) error {
	if _, ok := e.centralGuard.(component.ICentralItemLifecycle); !ok {
		return fmt.Errorf("central item lifecycle not wired")
	}
	if _, ok := e.evalTargetService.(interface {
		TerminateSandboxAttemptsForRunItems(context.Context, int64, int64, []int64, ...bool) error
	}); !ok {
		return fmt.Errorf("central sandbox attempt discovery not wired")
	}
	if err := e.terminateSandboxExecutesForCancelledItems(ctx, expt.SpaceID, expt.TargetSpaceID, expt.ID, &runID, []int64{itemID}, true, true); err != nil {
		return err
	}
	return nil
}
