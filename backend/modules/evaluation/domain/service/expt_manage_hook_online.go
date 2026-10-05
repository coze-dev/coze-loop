// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"maps"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

type IHookOnlineScheduleStarter interface {
	StartOnlineWithHookSchedule(context.Context, int64, int64, int64, int, *entity.Session, map[string]string) (bool, error)
}

type IHookOnlineRunAccess interface {
	CheckOnlineRun(context.Context, entity.HookRunKey) (bool, error)
	ValidateOnlineRun(context.Context, entity.HookRunKey) (bool, error)
}

func (e *ExptMangerImpl) ValidateOnlineRun(ctx context.Context, key entity.HookRunKey) (bool, error) {
	if e == nil || ctx == nil {
		return true, entity.ErrHookStoreCorrupt
	}
	if e.hooks == nil {
		return false, nil
	}
	initial, err := e.readHookRunInitialization(ctx, key)
	if err != nil {
		return true, err
	}
	if err := e.checkOnlineRequestedRun(ctx, key, initial); err != nil {
		return true, err
	}
	if !initial.Managed {
		return false, nil
	}
	_, _, err = e.readOnlineRun(ctx, key)
	return true, err
}

func (e *ExptMangerImpl) StartOnlineWithHookSchedule(ctx context.Context, exptID, runID, spaceID int64, retries int, user *entity.Session, ext map[string]string) (bool, error) {
	if e == nil {
		return true, entity.ErrHookStoreCorrupt
	}
	if e.hooks == nil {
		return false, nil
	}
	if ctx == nil || user == nil {
		return true, entity.ErrHookStoreCorrupt
	}
	ctx = contexts.WithCtxWriteDB(ctx)
	key := entity.HookRunKey{WorkspaceID: spaceID, ExperimentID: exptID, RunID: runID}
	initial, err := e.readHookRunInitialization(ctx, key)
	if err != nil {
		return true, err
	}
	if !initial.Managed && !initial.HooksEnabled {
		return false, nil
	}
	expt, err := e.exptRepo.GetByID(ctx, exptID, spaceID)
	if err != nil {
		return true, err
	}
	if expt == nil || expt.ID != exptID || expt.SpaceID != spaceID || expt.ExptType != entity.ExptType_Online {
		return true, entity.ErrHookStoreConflict
	}
	if initial.RunLog == nil {
		seed := &entity.HookScheduleSeed{Version: 1, Key: key, ExecutionScope: e.hooks.ExecutionScope, Mode: entity.EvaluationModeAppend, CreatedAt: time.Now().Unix(), ItemRetryTimes: retries, Session: &entity.Session{UserID: user.UserID, AppID: user.AppID}, Ext: withRetryYieldExt(maps.Clone(ext), e.configer.GetRetryYieldEnabled(ctx, spaceID))}
		if err := e.logHookRun(ctx, exptID, runID, entity.EvaluationModeAppend, spaceID, nil, user, nil, seed); err != nil {
			return true, err
		}
	}
	if err := e.PrepareOnlinePlan(ctx, key); err != nil {
		return true, err
	}
	return true, e.PublishOnlineContinuation(ctx, key)
}

func (e *ExptMangerImpl) readOnlineRun(ctx context.Context, key entity.HookRunKey) (*entity.HookStoredRun, *entity.ExptScheduleEvent, error) {
	if ctx == nil || e == nil || e.hooks == nil {
		return nil, nil, entity.ErrHookStoreCorrupt
	}
	initial, err := e.readHookRunInitialization(contexts.WithCtxWriteDB(ctx), key)
	if err != nil {
		return nil, nil, err
	}
	if !initial.Managed || initial.RunLog == nil || initial.LatestRunID != key.RunID {
		return nil, nil, entity.ErrHookStoreConflict
	}
	run, event, err := e.readHookSchedule(ctx, key)
	if err != nil {
		return nil, nil, err
	}
	if run.Mode != entity.EvaluationModeAppend || event.ExptType != entity.ExptType_Online {
		return nil, nil, entity.ErrHookStoreConflict
	}
	return run, event, nil
}

func (e *ExptMangerImpl) CheckOnlineRun(ctx context.Context, key entity.HookRunKey) (bool, error) {
	if e == nil || ctx == nil {
		return true, entity.ErrHookStoreCorrupt
	}
	if e.hooks == nil {
		return false, nil
	}
	initial, err := e.readHookRunInitialization(ctx, key)
	if err != nil {
		return true, err
	}
	if err := e.checkOnlineRequestedRun(ctx, key, initial); err != nil {
		return true, err
	}
	if !initial.Managed {
		return false, nil
	}
	if err := e.drainOnlineDeadline(ctx, key); err != nil {
		return true, err
	}
	run, _, err := e.readOnlineRun(ctx, key)
	if err != nil {
		return true, err
	}
	if run.State.Finalize != entity.HookFinalizeNone || run.State.Gate == entity.HookGateClosed || (run.State.Status != entity.ExptStatus_Pending && run.State.Status != entity.ExptStatus_Processing) {
		return true, errorx.NewByCode(errno.ExperimentStatusNotAllowedToInvokeCode)
	}
	return true, nil
}

func (e *ExptMangerImpl) checkOnlineRequestedRun(ctx context.Context, key entity.HookRunKey, initial *entity.HookRunInitialization) error {
	if initial.Managed {
		return nil
	}
	if initial.RunLog == nil && initial.HooksEnabled {
		return entity.ErrHookStoreConflict
	}
	if initial.LatestRunID > 0 && initial.LatestRunID != key.RunID {
		latest := key
		latest.RunID = initial.LatestRunID
		current, err := e.readHookRunInitialization(ctx, latest)
		if err != nil {
			return err
		}
		if current.Managed {
			return entity.ErrHookStoreConflict
		}
	}
	return nil
}

func (e *ExptMangerImpl) PrepareOnlinePlan(ctx context.Context, key entity.HookRunKey) error {
	if _, _, err := e.readOnlineRun(ctx, key); err != nil {
		return err
	}
	p, ok := e.hooks.Runs.(repo.IHookOnlineRepo)
	if !ok {
		return entity.ErrHookExecutionUnsupported
	}
	if err := p.PrepareOnlinePlan(ctx, key, e.hooks.ExecutionScope); err != nil {
		return err
	}
	return e.drainOnlineDeadline(ctx, key)
}

func (e *ExptMangerImpl) PublishOnlineContinuation(ctx context.Context, key entity.HookRunKey) error {
	run, event, err := e.readOnlineRun(ctx, key)
	if err != nil {
		return err
	}
	if run.State.Finalize != entity.HookFinalizeNone || run.State.Gate == entity.HookGateClosed || entity.IsExptFinished(run.State.Status) || run.State.Status == entity.ExptStatus_Terminating {
		return nil
	}
	if err := e.drainOnlineDeadline(ctx, key); err != nil {
		return err
	}
	run, event, err = e.readOnlineRun(ctx, key)
	if err != nil {
		return err
	}
	if !run.ExecutionInitialized {
		_, err = e.publishHookSchedule(ctx, event)
		return err
	}
	return e.publisher.PublishExptScheduleEvent(ctx, event, gptr.Of(3*time.Second))
}

func (e *ExptMangerImpl) invokeOnline(ctx context.Context, in *entity.InvokeExptReq) error {
	key := entity.HookRunKey{WorkspaceID: in.SpaceID, ExperimentID: in.ExptID, RunID: in.RunID}
	if _, err := e.CheckOnlineRun(ctx, key); err != nil {
		return err
	}
	if len(in.Items) == 0 {
		return nil
	}
	p, ok := e.hooks.Runs.(repo.IHookOnlineRepo)
	if !ok || e.onlineItems == nil {
		return entity.ErrHookExecutionUnsupported
	}
	plans, ok := e.hooks.Runs.(repo.IHookPlanRepo)
	if !ok {
		return entity.ErrHookExecutionUnsupported
	}
	changed := false
	groups := map[int64][]int64{}
	seen := map[int64]int64{}
	for _, item := range in.Items {
		if item == nil || item.ItemID <= 0 || item.EvaluationSetID <= 0 || (item.SpaceID != 0 && item.SpaceID != in.SpaceID) {
			return entity.ErrHookStoreCorrupt
		}
		if set, ok := seen[item.ItemID]; ok {
			if set != item.EvaluationSetID {
				return entity.ErrHookStoreConflict
			}
			continue
		}
		seen[item.ItemID] = item.EvaluationSetID
		groups[item.EvaluationSetID] = append(groups[item.EvaluationSetID], item.ItemID)
	}
	for set, ids := range groups {
		for start := 0; start < len(ids); start += 100 {
			page := ids[start:min(start+100, len(ids))]
			known, err := plans.MGetPlanItems(ctx, entity.HookPlanLookupInput{Key: key, ExecutionScope: e.hooks.ExecutionScope, ItemIDs: page})
			if err != nil {
				return err
			}
			if known == nil {
				return entity.ErrHookStoreCorrupt
			}
			exists := map[int64]bool{}
			for _, item := range known.Items {
				exists[item.ItemID] = true
			}
			fresh := make([]int64, 0, len(page))
			for _, id := range page {
				if !exists[id] {
					fresh = append(fresh, id)
				}
			}
			page = fresh
			if len(page) == 0 {
				continue
			}
			items, err := e.onlineItems.BatchGetEvaluationSetItems(ctx, &entity.BatchGetEvaluationSetItemsParam{SpaceID: in.SpaceID, EvaluationSetID: set, ItemIDs: page})
			if err != nil {
				return err
			}
			if len(items) != len(page) {
				return entity.ErrHookFrozenItemUnavailable
			}
			byID := map[int64]*entity.EvaluationSetItem{}
			count := 3 * len(items)
			for _, item := range items {
				if item == nil || item.SpaceID != in.SpaceID || item.EvaluationSetID != set || byID[item.ItemID] != nil {
					return entity.ErrHookFrozenItemUnavailable
				}
				byID[item.ItemID] = item
				count += len(item.Turns)
			}
			allocated, err := e.idgenerator.GenMultiIDs(ctx, count)
			if err != nil {
				return err
			}
			if len(allocated) != count {
				return entity.ErrHookStoreCorrupt
			}
			i := 0
			var manifests []entity.HookExecutionManifest
			for _, id := range page {
				item := byID[id]
				if item == nil {
					return entity.ErrHookFrozenItemUnavailable
				}
				m := entity.HookExecutionManifest{Version: 1, Key: key, Frozen: entity.HookPlanItem{ID: allocated[i], SourceSpaceID: in.SpaceID, EvalSetID: set, ItemID: id, ItemVersionID: gptr.Indirect(item.ItemVersionID)}, ItemResultID: allocated[i+1], ItemRunLogID: allocated[i+2], TurnLogsInitialized: gptr.Of(false)}
				i += 3
				for idx, turn := range item.Turns {
					if turn == nil {
						return entity.ErrHookFrozenItemUnavailable
					}
					m.Turns = append(m.Turns, entity.HookExecutionTurnManifest{TurnID: turn.ID, TurnIdx: int32(idx), ResultID: allocated[i]})
					i++
				}
				manifests = append(manifests, m)
			}
			wrote, err := p.AppendOnlinePage(ctx, key, e.hooks.ExecutionScope, manifests, in.Ext)
			if err != nil {
				return err
			}
			changed = changed || wrote
		}
	}
	if !changed {
		return nil
	}
	return e.PublishOnlineContinuation(ctx, key)
}

func (e *ExptMangerImpl) drainOnlineDeadline(ctx context.Context, key entity.HookRunKey) error {
	expt, err := e.exptRepo.GetByID(contexts.WithCtxWriteDB(ctx), key.ExperimentID, key.WorkspaceID)
	if err != nil {
		return err
	}
	if expt == nil || expt.ID != key.ExperimentID || expt.SpaceID != key.WorkspaceID || expt.ExptType != entity.ExptType_Online {
		return entity.ErrHookStoreConflict
	}
	if (expt.Status == entity.ExptStatus_Pending || expt.Status == entity.ExptStatus_Processing) && expt.MaxAliveTime > 0 && expt.StartAt != nil && time.Now().After(expt.StartAt.Add(time.Duration(expt.MaxAliveTime)*time.Millisecond)) {
		p, ok := e.hooks.Runs.(repo.IHookOnlineRepo)
		if !ok {
			return entity.ErrHookExecutionUnsupported
		}
		return p.DrainOnlineRun(ctx, key, e.hooks.ExecutionScope)
	}
	return nil
}

func (e *ExptMangerImpl) finishOnline(ctx context.Context, key entity.HookRunKey) error {
	run, _, err := e.readOnlineRun(ctx, key)
	if err != nil {
		return err
	}
	if run.State.Finalize != entity.HookFinalizeNone || entity.IsExptFinished(run.State.Status) {
		return nil
	}
	p, ok := e.hooks.Runs.(repo.IHookOnlineRepo)
	if !ok {
		return entity.ErrHookExecutionUnsupported
	}
	if err := p.DrainOnlineRun(ctx, key, e.hooks.ExecutionScope); err != nil {
		return err
	}
	return e.PublishOnlineContinuation(ctx, key)
}
