// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"maps"
	"reflect"
	"time"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

// IHookRunScheduleStarter is additive; handled=false preserves the legacy two-call path.
type IHookRunScheduleStarter interface {
	StartRunWithHookSchedule(context.Context, int64, int64, int64, int, *entity.Session, entity.ExptRunMode, map[string]string) (bool, error)
}

var _ IHookRunScheduleStarter = (*ExptMangerImpl)(nil)

var ErrHookScheduleRetrySourceMissing = errors.New("hook retry requires a persisted source Run")

func (e *ExptMangerImpl) StartRunWithHookSchedule(ctx context.Context, exptID, runID, spaceID int64, itemRetryNum int, session *entity.Session, mode entity.ExptRunMode, ext map[string]string) (bool, error) {
	if e == nil {
		return true, entity.ErrHookStoreCorrupt
	}
	if e.hooks == nil {
		return false, nil
	}
	key := entity.HookRunKey{WorkspaceID: spaceID, ExperimentID: exptID, RunID: runID}
	if ctx == nil || session == nil || (entity.HookStoreGuard{Key: key}).Validate() != nil {
		return true, entity.ErrHookStoreCorrupt
	}
	if err := ctx.Err(); err != nil {
		return true, err
	}
	ctx = contexts.WithCtxWriteDB(ctx)
	initial, err := e.readHookRunInitialization(ctx, key)
	if err != nil {
		return true, err
	}
	if initial.RunLog != nil && !initial.Managed || initial.RunLog == nil && !initial.HooksEnabled {
		return false, nil
	}
	if mode != entity.EvaluationModeSubmit && mode != entity.EvaluationModeTrialRun && mode != entity.EvaluationModeFailRetry && mode != entity.EvaluationModeRetryAll {
		return true, entity.ErrHookFinalizationUnsupported
	}
	if initial.RunLog == nil {
		expt, err := e.exptRepo.GetByID(ctx, exptID, spaceID)
		if err != nil {
			return true, err
		}
		if expt == nil || expt.ID != exptID || expt.SpaceID != spaceID || expt.LatestRunID != initial.LatestRunID || expt.ExptType != entity.ExptType_Offline {
			return true, entity.ErrHookStoreConflict
		}
		retry := mode == entity.EvaluationModeFailRetry || mode == entity.EvaluationModeRetryAll
		if retry {
			if initial.LatestRunID <= 0 {
				return true, ErrHookScheduleRetrySourceMissing
			}
			if initial.LatestRunID == runID {
				return true, entity.ErrHookStoreConflict
			}
			sourceKey := key
			sourceKey.RunID = initial.LatestRunID
			source, err := e.readHookRunInitialization(ctx, sourceKey)
			if err != nil {
				return true, err
			}
			old := source.RunLog
			if old == nil {
				return true, ErrHookScheduleRetrySourceMissing
			}
			if source.LatestRunID != sourceKey.RunID || old.ID != sourceKey.RunID || old.ExptRunID != sourceKey.RunID || old.ExptID != exptID || old.SpaceID != spaceID {
				return true, entity.ErrHookStoreConflict
			}
		}
		seed := &entity.HookScheduleSeed{Version: 1, Key: key, ExecutionScope: e.hooks.ExecutionScope, Mode: mode, CreatedAt: time.Now().Unix(), ItemRetryTimes: itemRetryNum,
			Session: &entity.Session{UserID: session.UserID, AppID: session.AppID}, Ext: withRetryYieldExt(maps.Clone(ext), e.configer.GetRetryYieldEnabled(ctx, spaceID))}
		if _, err := seed.Event(key, e.hooks.ExecutionScope, mode, session.UserID); err != nil {
			return true, err
		}
		raw := ext["__item_ids"]
		if retry {
			// Keep the verified source pinned through the existing owner lease and creation CAS.
			err = e.logHookRunFromInitialization(ctx, exptID, runID, mode, spaceID, nil, session, &raw, initial, seed)
		} else {
			err = e.logHookRun(ctx, exptID, runID, mode, spaceID, nil, session, &raw, seed)
		}
		if err != nil {
			return true, err
		}
	}
	_, event, err := e.readHookSchedule(ctx, key)
	if err != nil {
		return true, err
	}
	requested, frozen := maps.Clone(ext), maps.Clone(event.Ext)
	delete(requested, entity.RetryYieldExtKey)
	delete(frozen, entity.RetryYieldExtKey)
	if event.ExptRunMode != mode || *event.Session != *session || event.ItemRetryTimes != itemRetryNum || !maps.Equal(requested, frozen) {
		return true, entity.ErrHookStoreConflict
	}
	published, err := e.publishHookSchedule(ctx, event)
	if err != nil {
		return true, err
	}
	if !published {
		return true, nil
	}
	// Start notifications stay outside worker recovery; recovery never sends another card.
	expt, err := e.exptRepo.GetByID(ctx, exptID, spaceID)
	if (mode == entity.EvaluationModeSubmit || mode == entity.EvaluationModeTrialRun) && err == nil && expt != nil && expt.LatestRunID == runID && expt.NotificationConf == nil && !isFeishuNotifySuppressedByTrigger(expt) {
		if err := e.sendNotifyCard(ctx, expt); err != nil {
			logs.CtxWarn(ctx, "Hook start notification failed")
		}
	}
	return true, nil
}

func (e *ExptMangerImpl) readHookSchedule(ctx context.Context, key entity.HookRunKey) (*entity.HookStoredRun, *entity.ExptScheduleEvent, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	run, err := e.hooks.Runs.GetRun(ctx, key)
	if err != nil {
		return nil, nil, err
	}
	if run == nil || run.State.Key != key || run.Snapshot.ExecutionScope != e.hooks.ExecutionScope || entity.ValidateHookStorageState(&run.State) != nil {
		return nil, nil, entity.ErrHookStoreCorrupt
	}
	if (run.Mode == entity.EvaluationModeFailRetry || run.Mode == entity.EvaluationModeRetryAll) && (run.SourceRunID == nil || *run.SourceRunID <= 0) {
		return nil, nil, ErrHookScheduleRetrySourceMissing
	}
	if run.SourceRunID != nil && *run.SourceRunID == key.RunID {
		return nil, nil, entity.ErrHookStoreConflict
	}
	snapshot, err := e.hooks.Codec.DecodeSnapshot(ctx, key, e.hooks.ExecutionScope, run.Snapshot)
	if err != nil {
		return nil, nil, err
	}
	if snapshot == nil {
		return nil, nil, entity.ErrHookStoreCorrupt
	}
	event, err := snapshot.Input().Schedule.Event(key, e.hooks.ExecutionScope, run.Mode, run.CreatedBy)
	return run, event, err
}

var errHookScheduleClosed = errors.New("hook schedule run is closed or superseded")

// PublishHookSchedule keeps the existing MQ protocol and is safe to repeat after a lost receipt.
func (e *ExptMangerImpl) PublishHookSchedule(ctx context.Context, event *entity.ExptScheduleEvent) error {
	_, err := e.publishHookSchedule(ctx, event)
	return err
}

func (e *ExptMangerImpl) publishHookSchedule(ctx context.Context, event *entity.ExptScheduleEvent) (bool, error) {
	if e == nil || e.hooks == nil || ctx == nil || event == nil {
		return false, entity.ErrHookStoreCorrupt
	}
	for _, dep := range []any{e.hooks.Runs, e.hooks.Initialization, e.hooks.Codec, e.quotaRepo, e.configer, e.publisher} {
		if missingManagerHookDependency(dep) {
			return false, entity.ErrHookStoreCorrupt
		}
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	ctx = contexts.WithCtxWriteDB(ctx)
	key := entity.HookRunKey{WorkspaceID: event.SpaceID, ExperimentID: event.ExptID, RunID: event.ExptRunID}
	_, original, err := e.readHookSchedule(ctx, key)
	if err != nil {
		return false, err
	}
	if !reflect.DeepEqual(original, event) {
		return false, entity.ErrHookStoreConflict
	}
	conf := e.configer.GetExptExecConf(ctx, key.WorkspaceID)
	err = e.quotaRepo.CreateOrUpdate(ctx, key.WorkspaceID, func(cur *entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error) {
		if err := e.checkHookScheduleCurrent(ctx, key, original); err != nil {
			return nil, false, err
		}
		if cur != nil {
			if at, ok := cur.ExptID2RunTime[key.ExperimentID]; ok && at == original.CreatedAt {
				return cur, false, nil
			}
			if _, ok := cur.ExptID2RunTime[key.ExperimentID]; !ok && len(cur.ExptID2RunTime) >= conf.GetSpaceExptConcurLimit() {
				return nil, false, errorx.NewByCode(errno.ExperimentRunningCountLimitCode)
			}
		}
		next := cur.Clone()
		next.ExptID2RunTime[key.ExperimentID] = original.CreatedAt
		// Quota expiry remains live policy; neither the event nor its own slot is refreshed.
		now := time.Now().Unix()
		for id, at := range next.ExptID2RunTime {
			if id != key.ExperimentID && int(now-at) > conf.GetZombieIntervalSecond() {
				delete(next.ExptID2RunTime, id)
			}
		}
		return next, true, nil
	}, original.Session)
	if errors.Is(err, errHookScheduleClosed) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if err := e.checkHookScheduleCurrent(ctx, key, original); err != nil {
		if errors.Is(err, errHookScheduleClosed) {
			return false, nil
		}
		return false, err
	}
	if err := e.publisher.PublishExptScheduleEvent(ctx, original, gptr.Of(3*time.Second)); err != nil {
		return false, err
	}
	return true, ctx.Err()
}

func (e *ExptMangerImpl) checkHookScheduleCurrent(ctx context.Context, key entity.HookRunKey, event *entity.ExptScheduleEvent) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	initial, err := e.readHookRunInitialization(ctx, key)
	if err != nil {
		return err
	}
	log := initial.RunLog
	if !initial.Managed || log == nil || log.ID != key.RunID || log.ExptRunID != key.RunID || log.SpaceID != key.WorkspaceID || log.ExptID != key.ExperimentID || log.CreatedBy != event.Session.UserID || entity.ExptRunMode(log.Mode) != event.ExptRunMode {
		return entity.ErrHookStoreCorrupt
	}
	if initial.LatestRunID != key.RunID || entity.IsExptFinished(entity.ExptStatus(log.Status)) || entity.ExptStatus(log.Status) == entity.ExptStatus_Terminating {
		return errHookScheduleClosed
	}
	run, original, err := e.readHookSchedule(ctx, key)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(original, event) {
		return entity.ErrHookStoreConflict
	}
	onlineDraining := run.Mode == entity.EvaluationModeAppend && run.State.Status == entity.ExptStatus_Draining && !run.ExecutionInitialized
	if run.State.Finalize != entity.HookFinalizeNone || run.State.Gate == entity.HookGateClosed || run.ExecutionStarted || run.State.Status != entity.ExptStatus_Pending && !onlineDraining {
		return errHookScheduleClosed
	}
	return ctx.Err()
}
