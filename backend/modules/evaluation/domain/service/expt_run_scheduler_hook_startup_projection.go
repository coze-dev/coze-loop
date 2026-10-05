// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func (e *ExptSchedulerImpl) projectHookSchedulerStartup(ctx context.Context, key entity.HookRunKey) (stop bool, err error) {
	storage, ok := e.hookScheduler.(repo.IHookSchedulerStartupProjectionRepo)
	if !ok || hookExecutionNil(storage) || hookExecutionNil(e.ResultSvc) {
		return true, schedulerHookRetryError{}
	}
	idemKey := fmt.Sprintf("hook_startup_projection:%d:%d:%d", key.WorkspaceID, key.ExperimentID, key.RunID)
	if e.Idem != nil {
		complete, err := e.Idem.Exist(ctx, idemKey)
		if err != nil {
			logs.CtxWarn(ctx, "Hook startup projection receipt read failed: %v", err)
		} else if complete {
			return false, nil
		}
	}
	// Full indexing includes frozen Queueing items that have not yet been dispatched.
	indexErr := e.ResultSvc.UpsertExptTurnResultFilter(ctx, key.WorkspaceID, key.ExperimentID, nil)
	if indexErr != nil {
		logs.CtxWarn(ctx, "Hook startup full index publication failed: %v", indexErr)
	}
	active, err := storage.ProjectHookSchedulerStartup(ctx, key, e.hookSchedulerScope)
	if err != nil && !active {
		return true, err
	}
	if !active {
		// Index publication is eventually consistent; repair a cancellation/new-Latest race from current SQL.
		if err := e.ResultSvc.UpsertExptTurnResultFilter(ctx, key.WorkspaceID, key.ExperimentID, nil); err != nil {
			logs.CtxWarn(ctx, "Hook startup current index refresh failed: %v", err)
		}
		return true, nil
	}
	if err != nil {
		logs.CtxWarn(ctx, "Hook startup template projection failed: %v", err)
	}
	if indexErr != nil || err != nil {
		// Optional effects retain replay eligibility without blocking evaluation.
		return false, nil
	}
	if e.Idem != nil {
		ttl := 2 * time.Second * time.Duration(e.Configer.GetExptExecConf(ctx, key.WorkspaceID).GetZombieIntervalSecond())
		if err := e.Idem.Set(ctx, idemKey, ttl); err != nil {
			logs.CtxWarn(ctx, "Hook startup projection receipt write failed: %v", err)
		}
	}
	return false, nil
}
