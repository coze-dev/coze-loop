// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"math"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func (e *ExptItemEventEvalServiceImpl) applyHookConsumerControl(ctx context.Context, event *entity.ExptItemEvalEvent, action entity.HookConsumerAction, failure error) (entity.HookConsumerControlResult, error) {
	if e.hookAdmission == nil {
		return entity.HookConsumerControlResult{}, nil
	}
	closed := entity.HookConsumerControlResult{Handled: true}
	if ctx == nil || event == nil || hookExecutionNil(e.hookAdmission.source) {
		return closed, itemHookControlError{wait: true}
	}
	writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), itemHookDependencyTimeout)
	defer cancel()
	if !itemHookManaged(ctx) {
		source, err := e.readItemHookSource(writeCtx, itemHookKey(event))
		if err != nil {
			return closed, err
		}
		if !source.Managed {
			return entity.HookConsumerControlResult{}, nil
		}
	}
	writer, ok := e.hookAdmission.progress.(repo.IHookConsumerControlRepo)
	if !ok || hookExecutionNil(writer) {
		return closed, itemHookControlError{wait: true}
	}
	in := entity.HookConsumerControlInput{Key: itemHookKey(event), ItemID: event.EvalSetItemID, Action: action}
	if failure != nil {
		in.ErrorMessage = errno.SerializeErr(failure)
	}
	if action == entity.HookConsumerYield {
		if event.RetryTimes < 0 || event.RetryTimes >= math.MaxInt32 {
			return closed, itemHookControlError{wait: true}
		}
		in.RetryTimes = int32(event.RetryTimes + 1)
	}
	out, err := writer.ApplyHookConsumerControl(writeCtx, in)
	if err != nil {
		logs.CtxWarn(ctx, "Hook consumer control write failed, run=%d item=%d action=%s: %v", event.ExptRunID, event.EvalSetItemID, action, err)
		return closed, itemHookControlError{wait: true}
	}
	if out.Handled && out.ProjectionChanged && (action == entity.HookConsumerStartReserved || action == entity.HookConsumerReservationAbsent) {
		e.refreshHookConsumerFilter(ctx, event)
	}
	return out, nil
}

// Indexing reads committed results; it is not part of the SQL transaction or its success condition.
func (e *ExptItemEventEvalServiceImpl) refreshHookConsumerFilter(ctx context.Context, event *entity.ExptItemEvalEvent) {
	if hookExecutionNil(e.resultSvc) || hookExecutionNil(e.hookAdmission.gate) {
		return
	}
	decision, err := e.hookAdmission.gate.CanDispatch(ctx, itemHookKey(event))
	if err != nil {
		logs.CtxWarn(ctx, "Hook consumer filter refresh gate unavailable, run=%d item=%d: %v", event.ExptRunID, event.EvalSetItemID, err)
		return
	}
	if decision.Gate != entity.HookGateReady {
		return
	}
	if err := e.resultSvc.UpsertExptTurnResultFilter(ctx, event.SpaceID, event.ExptID, []int64{event.EvalSetItemID}); err != nil {
		logs.CtxWarn(ctx, "Hook consumer filter refresh failed (display only), run=%d item=%d: %v", event.ExptRunID, event.EvalSetItemID, err)
	}
}
