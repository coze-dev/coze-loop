// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func notifyCentralItemTerminal(ctx context.Context, guard component.ICentralReservationGuard, items repo.IExptItemResultRepo, spaceID, exptID, runID, itemID int64) {
	lifecycle, ok := guard.(component.ICentralItemLifecycle)
	if !ok || items == nil {
		return
	}
	callbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
	defer cancel()
	item, err := items.GetItemRunLog(contexts.WithCtxWriteDB(callbackCtx), exptID, runID, itemID, spaceID)
	if err != nil {
		logs.CtxWarn(callbackCtx, "[EvaluationResource] terminal read failed, expt_id=%d run_id=%d item_id=%d err=%v", exptID, runID, itemID, err)
		return
	}
	if item == nil || !entity.IsItemRunFinished(entity.ItemRunState(item.Status)) {
		return
	}
	if item.ID <= 0 || item.SpaceID != spaceID || item.ExptID != exptID || item.ExptRunID != runID || item.ItemID != itemID {
		return
	}
	if err := lifecycle.OnItemTerminal(callbackCtx, item); err != nil {
		logs.CtxWarn(callbackCtx, "[EvaluationResource] finalization failed, item_run_log_id=%d status=%d err=%v", item.ID, item.Status, err)
	}
}
