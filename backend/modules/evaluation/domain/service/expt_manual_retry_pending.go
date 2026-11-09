// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func (m *ExptMangerImpl) consumePendingCurrentRetry(ctx context.Context, event *entity.ExptScheduleEvent) error {
	port, ok := m.manualRetryRepo.(repo.IExptManualRetryPendingRepo)
	if !ok || event.ExptType != entity.ExptType_Offline || event.ExptRunMode == entity.EvaluationModeAppend {
		return nil
	}
	pending, err := port.GetPendingRetry(ctx, event.SpaceID, event.ExptID, event.ExptRunID)
	if err != nil {
		return errno.WrapMQRetryErr(err)
	}
	if pending == nil {
		return nil
	}
	receipt, err := port.ResolvePendingRetry(ctx, pending, false)
	if err != nil {
		if status, ok := errorx.FromStatusError(err); ok && (status.Code() == errno.ManualRetryBusyCode || status.Code() == errno.ManualRetryRunFinishedCode) {
			return nil
		}
		return errno.WrapMQRetryErr(err)
	}
	if receipt != nil {
		return nil
	}
	pending.Request.BeforeRequeue = func(ctx context.Context) error { return port.CheckPendingRetry(ctx, pending) }
	pending.Request.BeforeCommit = func(ctx context.Context, receipt *entity.ManualRetryReceipt, ids []int64) error {
		return port.PreparePendingRetryCommit(ctx, pending, receipt, ids)
	}
	receipt, err = m.applyPendingCurrentRetry(ctx, pending)
	if err != nil {
		if errno.NeedMQRetry(err) {
			return err
		}
		recovered, cancelErr := port.ResolvePendingRetry(ctx, pending, true)
		if recovered != nil {
			return nil
		}
		if cancelErr != nil {
			if _, business := errorx.FromStatusError(cancelErr); !business {
				return errno.WrapMQRetryErr(cancelErr)
			}
		}
		if resultErr := port.CompletePendingRetry(ctx, pending, nil, err); resultErr != nil {
			logs.CtxWarn(ctx, "pending manual retry rejection receipt unavailable, request_id=%s, err=%v", pending.Request.RequestID, resultErr)
		}
		logs.CtxWarn(ctx, "pending manual retry rejected, expt_id=%d, run_id=%d, request_id=%s, err=%v", event.ExptID, event.ExptRunID, pending.Request.RequestID, err)
		return nil
	}
	// SQL has committed; a failed receipt write must not interrupt this event's
	// ScheduleStart/cursor/NextTick chain. The API can recover the primary token.
	if err := port.CompletePendingRetry(ctx, pending, receipt, nil); err != nil {
		logs.CtxWarn(ctx, "pending manual retry committed; receipt recovery required, expt_id=%d, run_id=%d, request_id=%s, err=%v", event.ExptID, event.ExptRunID, pending.Request.RequestID, err)
	}
	return nil
}
