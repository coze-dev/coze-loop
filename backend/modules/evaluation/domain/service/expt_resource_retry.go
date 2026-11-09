// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"github.com/jinzhu/copier"
)

// Existing TargetRecord.Ext distinguishes a duplicate failed attempt from the next authorized retry.
const centralItemRetryRecordKey = "__central_item_retry_times"

type centralResourceExecutionKey struct{}
type centralResourceFirstStartKey struct{}
type centralResourceTurnStartKey struct{ turnID int64 }

func isCentralResourceExecution(ctx context.Context) bool {
	active, _ := ctx.Value(centralResourceExecutionKey{}).(bool)
	return active
}

var errCentralRetryPublication = errors.New("central retry publication failed")

func (e *ExptItemEventEvalServiceImpl) publishCentralRetry(ctx context.Context, event *entity.ExptItemEvalEvent, item *entity.ExptItemResultRunLog) error {
	clone := &entity.ExptItemEvalEvent{}
	if err := copier.CopyWithOption(clone, event, copier.Option{DeepCopy: true}); err != nil {
		return fmt.Errorf("%w: %w", errCentralRetryPublication, err)
	}
	clone.RetryTimes = int(item.RetryTimes)
	clone.AsyncReportTrigger, clone.AsyncEvaluatorReportTrigger = false, false
	clone.ResumeSource = nil
	interval := e.configer.GetErrRetryConf(ctx, event.SpaceID, errno.DeserializeErr(item.ErrMsg)).GetRetryInterval()
	if e.publisher == nil {
		return fmt.Errorf("%w: publisher not wired", errCentralRetryPublication)
	}
	if err := e.publisher.PublishExptRecordEvalEvent(ctx, clone, &interval, func(ne *entity.ExptItemEvalEvent) {
		ne.AsyncReportTrigger, ne.AsyncEvaluatorReportTrigger = false, false
		ne.ResumeSource = nil
	}); err != nil {
		return fmt.Errorf("%w: %w", errCentralRetryPublication, err)
	}
	return nil
}

func (e *ExptItemEventEvalServiceImpl) authorizeCentralRetry(ctx context.Context, event *entity.ExptItemEvalEvent, evalErr error) error {
	if e.mutex == nil {
		return fmt.Errorf("%w: retry mutex not wired", errCentralItemAdmission)
	}
	key := fmt.Sprintf("expt_item_eval_run_lock:%d:%d", event.ExptID, event.EvalSetItemID)
	locked, ctx, cancel, err := e.mutex.LockWithRenew(ctx, key, 5*time.Second, 5*time.Minute)
	if err != nil {
		return fmt.Errorf("%w: %w", errCentralItemAdmission, err)
	}
	if !locked {
		return fmt.Errorf("%w: retry item is locked", errCentralItemAdmission)
	}
	defer func() {
		cancel()
		if _, err := e.mutex.Unlock(key); err != nil {
			logs.CtxWarn(ctx, "[EvaluationResource] retry unlock failed, item_id=%d err=%v", event.EvalSetItemID, err)
		}
	}()
	store, ok := e.exptItemResultRepo.(repo.IExptItemRetryRepo)
	if !ok {
		return fmt.Errorf("%w: retry repository not wired", errCentralItemAdmission)
	}
	ctx = contexts.WithCtxWriteDB(ctx)
	_, err = store.AuthorizeItemRetry(ctx, event.SpaceID, event.ExptID, event.ExptRunID, event.EvalSetItemID, int32(event.RetryTimes), []byte(errno.SerializeErr(evalErr)))
	if err != nil {
		return fmt.Errorf("%w: %w", errCentralItemAdmission, err)
	}
	item, err := e.exptItemResultRepo.GetItemRunLog(ctx, event.ExptID, event.ExptRunID, event.EvalSetItemID, event.SpaceID)
	if err != nil {
		return fmt.Errorf("%w: %w", errCentralItemAdmission, err)
	}
	if item == nil || item.ID <= 0 || item.SpaceID != event.SpaceID || item.ExptID != event.ExptID || item.ExptRunID != event.ExptRunID || item.ItemID != event.EvalSetItemID {
		return fmt.Errorf("%w: retry identity missing", errCentralItemAdmission)
	}
	if entity.ItemRunState(item.Status) != entity.ItemRunState_Processing {
		return nil
	}
	if int(item.RetryTimes) <= event.RetryTimes {
		return fmt.Errorf("%w: retry authorization did not persist", errCentralItemAdmission)
	}
	return e.publishCentralRetry(ctx, event, item)
}
