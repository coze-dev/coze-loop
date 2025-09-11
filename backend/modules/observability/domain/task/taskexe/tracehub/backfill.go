// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package tracehub

//import (
//	"context"
//	"sync"
//
//	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
//	"github.com/coze-dev/coze-loop/backend/pkg/logs"
//)
//
//func (h *TraceHubServiceImpl) BackFill(ctx context.Context, event *entity.AutoEvalEvent) error {
//	if h.isBackfillDone(ctx) {
//		logs.CtxInfo(ctx, "backfill is done, task_id=%d", c.task.ID)
//		return nil
//	}
//
//	if err := h.startTask(ctx); err != nil {
//		return err
//	}
//
//	subCtx, cancel := context.WithCancel(ctx)
//	wg := sync.WaitGroup{}
//	wg.Add(1)
//	goutil.GoWithRecover(ctx, func() {
//		defer wg.Done()
//		defer cancel()
//		h.flushSpans(subCtx)
//	})
//
//	listErr := h.listSpans(subCtx)
//	if listErr != nil {
//		logs.CtxError(ctx, "list spans failed, task_id=%d, err=%v", c.task.ID, listErr)
//		// continue on error
//	}
//	close(h.flushCh)
//	wg.Wait()
//
//	return h.onHandleDone(ctx, listErr)
//	return nil
//}
//
//func (h *TraceHubServiceImpl) isBackfillDone(ctx context.Context) bool {
//	task := h.task
//	return task.BackfillStat != nil && task.BackfillStat.BackfillStat.IsFinished()
//}
