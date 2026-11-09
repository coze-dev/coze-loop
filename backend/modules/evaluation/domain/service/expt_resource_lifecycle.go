// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func (e *ExptItemEventEvalServiceImpl) notifyCompletedResourceItem(ctx context.Context, event *entity.ExptItemEvalEvent, execution *currentItemExecution) {
	if _, ok := e.centralGuard.(component.ICentralItemLifecycle); !ok || execution == nil ||
		!execution.Completed || !execution.SourceValidated || execution.LockLost || execution.ItemRunLogID <= 0 {
		return
	}
	expected := execution.Request
	if execution.Source != nil {
		expected = &execution.Source.Current
	}
	if expected == nil || event == nil || expected.RequestID != event.RequestID || expected.RunID != event.ExptRunID ||
		expected.ItemID != event.EvalSetItemID || expected.ExptID != event.ExptID || expected.SpaceID != event.SpaceID {
		return
	}
	mutex := e.mutex
	if e.itemLocker != nil {
		mutex = e.itemLocker()
	}
	if mutex == nil {
		return
	}
	callbackCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer stop()
	key := fmt.Sprintf("expt_item_eval_run_lock:%d:%d", expected.ExptID, expected.ItemID)
	locked, lockCtx, cancel, err := mutex.LockWithRenew(callbackCtx, key, 5*time.Second, time.Minute)
	if err != nil || !locked {
		if cancel != nil {
			cancel()
		}
		logs.CtxWarn(callbackCtx, "[EvaluationResource] source-owned finalization lock unavailable: %v", err)
		return
	}
	defer cancel()
	defer func() { _, _ = mutex.Unlock(key) }()
	if err := notifyCentralItemTerminalForSource(lockCtx, e.centralGuard, e.exptItemResultRepo, expected, execution.ItemRunLogID); err != nil {
		logs.CtxWarn(lockCtx, "[EvaluationResource] source-owned finalization deferred: %v", err)
	}
}

// Callers hold the item lease; canonical request ownership is rechecked before using the immutable log receipt.
func notifyCentralItemTerminalForSource(ctx context.Context, guard component.ICentralReservationGuard, items repo.IExptItemResultRepo, expected *entity.ExptItemCurrentRequest, logID int64, stoppedAndStartPrevented ...bool) error {
	if expected == nil || items == nil {
		return errExptNonCurrentSource
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	readCtx := contexts.WithCtxWriteDB(ctx)
	if _, current := items.(repo.IExptCurrentResultRepo); current {
		rows, err := items.BatchGet(readCtx, expected.SpaceID, expected.ExptID, []int64{expected.ItemID})
		if err != nil {
			return err
		}
		if len(rows) != 1 || rows[0] == nil || rows[0].ExptRunID != expected.RunID || rows[0].ItemVersionID != expected.ItemVersionID {
			return errExptNonCurrentSource
		}
		manual, err := entity.ParseManualRetryContext(rows[0].Ext)
		if err != nil {
			return err
		}
		request := ""
		if manual != nil {
			if manual.RunID != expected.RunID {
				return errExptNonCurrentSource
			}
			request = manual.RequestID
		}
		if request != expected.RequestID {
			return errExptNonCurrentSource
		}
	} else {
		current, err := items.GetCurrentRequest(readCtx, expected.SpaceID, expected.ExptID, expected.RunID, expected.ItemID)
		if err != nil {
			return err
		}
		if current == nil || current.RunID != expected.RunID || current.RequestID != expected.RequestID {
			return errExptNonCurrentSource
		}
	}
	item, err := items.GetItemRunLog(readCtx, expected.ExptID, expected.RunID, expected.ItemID, expected.SpaceID)
	if err != nil {
		return err
	}
	if item == nil || logID > 0 && item.ID != logID {
		return errExptNonCurrentSource
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	ctx = component.WithEvaluationResourceRequestID(ctx, expected.RequestID)
	return notifyCentralItemTerminal(ctx, guard, items, expected.SpaceID, expected.ExptID, expected.RunID, expected.ItemID, stoppedAndStartPrevented...)
}

func resourceRequestForItem(ctx context.Context, items repo.IExptItemResultRepo, item *entity.ExptItemResultRunLog) (*entity.ExptItemCurrentRequest, error) {
	readCtx := contexts.WithCtxWriteDB(ctx)
	if _, current := items.(repo.IExptCurrentResultRepo); !current {
		return items.GetCurrentRequest(readCtx, item.SpaceID, item.ExptID, item.ExptRunID, item.ItemID)
	}
	rows, err := items.BatchGet(readCtx, item.SpaceID, item.ExptID, []int64{item.ItemID})
	if err != nil {
		return nil, err
	}
	if len(rows) != 1 || rows[0] == nil || rows[0].ExptRunID != item.ExptRunID {
		return nil, errExptNonCurrentSource
	}
	manual, err := entity.ParseManualRetryContext(rows[0].Ext)
	if err != nil {
		return nil, err
	}
	request := &entity.ExptItemCurrentRequest{SpaceID: item.SpaceID, ExptID: item.ExptID, RunID: item.ExptRunID,
		ItemID: item.ItemID, ItemVersionID: rows[0].ItemVersionID}
	if manual != nil {
		if manual.RunID != item.ExptRunID {
			return nil, errExptNonCurrentSource
		}
		request.RequestID = manual.RequestID
	}
	return request, nil
}

func notifyCentralItemTerminal(ctx context.Context, guard component.ICentralReservationGuard, items repo.IExptItemResultRepo, spaceID, exptID, runID, itemID int64, stoppedAndStartPrevented ...bool) error {
	lifecycle, ok := guard.(component.ICentralItemLifecycle)
	if !ok || items == nil {
		return nil
	}
	callbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	item, err := items.GetItemRunLog(contexts.WithCtxWriteDB(callbackCtx), exptID, runID, itemID, spaceID)
	if err != nil {
		logs.CtxWarn(callbackCtx, "[EvaluationResource] terminal read failed, expt_id=%d run_id=%d item_id=%d err=%v", exptID, runID, itemID, err)
		return err
	}
	if item == nil || !entity.IsItemRunFinished(entity.ItemRunState(item.Status)) {
		return nil
	}
	if item.ID <= 0 || item.SpaceID != spaceID || item.ExptID != exptID || item.ExptRunID != runID || item.ItemID != itemID {
		return fmt.Errorf("central terminal item identity mismatch")
	}
	// Terminal is a start barrier, not evidence that a running Sandbox has stopped.
	if entity.ItemRunState(item.Status) == entity.ItemRunState_Terminal && (len(stoppedAndStartPrevented) == 0 || !stoppedAndStartPrevented[0]) {
		return nil
	}
	if err := lifecycle.OnItemTerminal(callbackCtx, item); err != nil {
		logs.CtxWarn(callbackCtx, "[EvaluationResource] finalization failed, item_run_log_id=%d status=%d err=%v", item.ID, item.Status, err)
		return err
	}
	return callbackCtx.Err()
}
