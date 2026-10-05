// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"maps"
	"time"
)

type IHookRetryItemsScheduleStarter interface {
	StartRetryItemsWithHookSchedule(context.Context, int64, int64, int, []int64, *entity.Session, map[string]string) (bool, int64, bool, error)
}

func (e *ExptMangerImpl) StartRetryItemsWithHookSchedule(ctx context.Context, exptID, spaceID int64, retries int, items []int64, user *entity.Session, ext map[string]string) (bool, int64, bool, error) {
	if e == nil {
		return true, 0, false, entity.ErrHookStoreCorrupt
	}
	if e.hooks == nil {
		return false, 0, false, nil
	}
	if ctx == nil || user == nil {
		return true, 0, false, entity.ErrHookStoreCorrupt
	}
	seed := func(runID int64) *entity.HookScheduleSeed {
		return &entity.HookScheduleSeed{Version: 1, Key: entity.HookRunKey{WorkspaceID: spaceID, ExperimentID: exptID, RunID: runID}, ExecutionScope: e.hooks.ExecutionScope, Mode: entity.EvaluationModeRetryItems, CreatedAt: time.Now().Unix(), ItemRetryTimes: retries, Session: &entity.Session{UserID: user.UserID, AppID: user.AppID}, Ext: withRetryYieldExt(maps.Clone(ext), e.configer.GetRetryYieldEnabled(ctx, spaceID))}
	}
	runID, retried, err := e.logRetryItemsRun(ctx, exptID, entity.EvaluationModeRetryItems, spaceID, items, user, seed)
	if err != nil {
		return true, runID, retried, err
	}
	key := entity.HookRunKey{WorkspaceID: spaceID, ExperimentID: exptID, RunID: runID}
	initial, err := e.readHookRunInitialization(ctx, key)
	if err != nil {
		return true, runID, retried, err
	}
	if !initial.Managed {
		if !retried {
			err = e.RetryItems(ctx, exptID, runID, spaceID, retries, items, user, ext)
		}
		return true, runID, retried, err
	}
	if retried {
		err = e.PublishRetryItemsContinuation(ctx, key)
	} else {
		_, event, readErr := e.readHookSchedule(ctx, key)
		if readErr != nil {
			return true, runID, retried, readErr
		}
		_, err = e.publishHookSchedule(ctx, event)
	}
	return true, runID, retried, err
}

// RetryItems enters Processing at initialization commit, before the first item admission.
func (e *ExptMangerImpl) PublishRetryItemsContinuation(ctx context.Context, key entity.HookRunKey) error {
	if e == nil || e.hooks == nil || ctx == nil || missingManagerHookDependency(e.publisher) {
		return entity.ErrHookStoreCorrupt
	}
	ctx = contexts.WithCtxWriteDB(ctx)
	run, event, err := e.readHookSchedule(ctx, key)
	if err != nil {
		return err
	}
	if run.Mode != entity.EvaluationModeRetryItems || run.SourceRunID == nil || *run.SourceRunID <= 0 {
		return entity.ErrHookStoreConflict
	}
	initial, err := e.readHookRunInitialization(ctx, key)
	if err != nil {
		return err
	}
	if !initial.Managed || initial.RunLog == nil {
		return entity.ErrHookStoreCorrupt
	}
	if initial.LatestRunID != key.RunID || run.State.Finalize != entity.HookFinalizeNone || run.State.Gate == entity.HookGateClosed || entity.IsExptFinished(entity.ExptStatus(initial.RunLog.Status)) || entity.ExptStatus(initial.RunLog.Status) == entity.ExptStatus_Terminating {
		return nil
	}
	if run.State.Status != entity.ExptStatus_Processing {
		_, err = e.publishHookSchedule(ctx, event)
		return err
	}
	return e.publisher.PublishExptScheduleEvent(ctx, event, gptr.Of(3*time.Second))
}
