// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/infra/lock"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

type ExptManagerFinalizationDependencies struct {
	Runs           repo.IHookRepo
	Repository     repo.IHookFinalizationRepo
	Owners         repo.IHookFinalizationOwnerReader
	ExecutionScope string
	// Each cleanup acquisition must have independent renewal/unlock ownership.
	NewItemLocker func() lock.ILocker
}

// This opt-in is independent of Hook initialization and does not extend IExptManager.
func NewExptManagerWithHookFinalization(base IExptManager, deps ExptManagerFinalizationDependencies) (*ExptMangerImpl, error) {
	m, ok := base.(*ExptMangerImpl)
	if !ok || m == nil || m.finalization != nil {
		return nil, errors.New("invalid hook finalization manager")
	}
	for _, v := range []any{deps.Runs, deps.Repository, deps.Owners, m.quotaRepo, m.mutex, m.publisher, m.mtr, m.exptAggrResultService} {
		if missingManagerHookDependency(v) {
			return nil, errors.New("missing hook finalization dependency")
		}
	}
	if len(deps.ExecutionScope) == 0 || len(deps.ExecutionScope) > 128 {
		return nil, errors.New("invalid hook finalization scope")
	}
	for _, c := range deps.ExecutionScope {
		if c < 33 || c > 126 {
			return nil, errors.New("invalid hook finalization scope")
		}
	}
	copy := *m
	copy.finalization = &deps
	return &copy, nil
}

func (e *ExptMangerImpl) completeHookNormalRun(ctx context.Context, key entity.HookRunKey, opts ...entity.CompleteExptOptionFn) (bool, error) {
	source, opt, err := e.hookNormalCompletionInput(ctx, key, opts...)
	if err != nil {
		return true, err
	}
	if source == nil || !source.Managed {
		return false, nil
	}
	var display *string
	if opt.StatusMessage != "" {
		message, err := normalizeHookStatusMessage(opt.StatusMessage)
		if err != nil {
			return true, err
		}
		display = &message
	}
	intent := entity.HookTerminalIntent{Status: opt.Status}
	if hookTerminationStatus(opt.Status) {
		accepted, err := e.acceptHookTermination(ctx, source.Key, opt.Status, display)
		if err != nil {
			return true, err
		}
		intent = accepted.State.Intent
		if err := waitHookCompletion(ctx, opt.CompleteInterval); err != nil {
			return true, err
		}
	}
	return true, e.finalizeHookRun(ctx, source.Key, intent, opt, display)
}

// CompleteRun precedes the caller's terminal decision in CompleteExpt.
func (e *ExptMangerImpl) prepareHookNormalRun(ctx context.Context, key entity.HookRunKey, opts ...entity.CompleteExptOptionFn) (bool, error) {
	source, opt, err := e.hookNormalCompletionInput(ctx, key, opts...)
	if err != nil {
		return true, err
	}
	if source == nil || !source.Managed {
		return false, nil
	}
	if hookTerminationStatus(opt.Status) {
		return true, e.prepareHookTermination(ctx, source, opt)
	}
	if opt.Status != 0 || opt.StatusMessage != "" {
		return true, entity.ErrHookFinalizationUnsupported
	}
	stored, err := e.finalization.loadNormalRun(ctx, source)
	if err != nil {
		return true, err
	}
	if stored.State.Finalize != entity.HookFinalizeNone {
		if stored.State.Intent.Status != entity.ExptStatus_Success && stored.State.Intent.Status != entity.ExptStatus_Failed {
			return true, entity.ErrHookFinalizationUnsupported
		}
		if stored.State.Finalize == entity.HookFinalizeCommitted {
			return true, e.unlockFinalizedHookRun(ctx, source.Key)
		}
	} else if stored.State.Gate != entity.HookGateReady {
		return true, entity.ErrHookFinalizationUnsettled
	}
	_, err = e.finalization.readStats(ctx, source.Key)
	return true, err
}

func (e *ExptMangerImpl) hookNormalCompletionInput(ctx context.Context, key entity.HookRunKey, opts ...entity.CompleteExptOptionFn) (*entity.HookFinalizationSource, *entity.CompleteExptOption, error) {
	if e.finalization == nil {
		return nil, nil, nil
	}
	if ctx == nil {
		return nil, nil, entity.ErrHookStoreCorrupt
	}
	source, err := e.finalization.Repository.ReadFinalizationSource(ctx, key)
	if err != nil {
		return nil, nil, err
	}
	if source == nil || source.RunLog == nil || source.Experiment == nil {
		return nil, nil, entity.ErrHookStoreCorrupt
	}
	if source.Key.WorkspaceID != key.WorkspaceID || source.Key.ExperimentID != key.ExperimentID || source.Key.RunID <= 0 ||
		(key.RunID != 0 && source.Key.RunID != key.RunID) || source.RunLog.ID != source.Key.RunID ||
		source.RunLog.ExptRunID != source.Key.RunID || source.RunLog.ExptID != key.ExperimentID || source.RunLog.SpaceID != key.WorkspaceID ||
		source.Experiment.ID != key.ExperimentID || source.Experiment.SpaceID != key.WorkspaceID {
		return nil, nil, entity.ErrHookStoreCorrupt
	}
	if !source.Managed {
		return source, nil, nil
	}
	// Resolve optional Latest before any delay, and never pass an optional key below.
	opt := &entity.CompleteExptOption{}
	for _, fn := range opts {
		fn(opt)
	}
	if opt.CompleteInterval > 0 && !hookTerminationStatus(opt.Status) {
		timer := time.NewTimer(opt.CompleteInterval)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-timer.C:
		}
	}
	return source, opt, nil
}

// FinalizeRun is an additive recovery entry; the legacy IExptManager is unchanged.
func (e *ExptMangerImpl) FinalizeRun(ctx context.Context, key entity.HookRunKey, intent entity.HookTerminalIntent) error {
	return e.finalizeHookRun(ctx, key, intent, &entity.CompleteExptOption{}, nil)
}

func (e *ExptMangerImpl) finalizeHookRun(ctx context.Context, key entity.HookRunKey, requested entity.HookTerminalIntent, opt *entity.CompleteExptOption, requestedDisplay *string) error {
	if e.finalization == nil || ctx == nil || (entity.HookStoreGuard{Key: key}).Validate() != nil {
		return entity.ErrHookStoreCorrupt
	}
	terminationRequested := hookTerminationStatus(requested.Status)
	if requested.Status != 0 && !entity.IsExptFinished(requested.Status) {
		return entity.ErrHookFinalizationUnsupported
	}
	ctx = contexts.WithCtxWriteDB(ctx)
	deps := e.finalization
	for attempt := 0; attempt < 5; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		source, err := deps.readSource(ctx, key)
		if err != nil {
			return err
		}
		stored, err := deps.loadNormalRun(ctx, source)
		if err != nil {
			return err
		}
		intent := stored.State.Intent
		display := requestedDisplay
		if stored.State.Finalize != entity.HookFinalizeNone {
			if !entity.IsExptFinished(intent.Status) {
				return entity.ErrHookFinalizationUnsupported
			}
			if requested.Status != 0 && requested.Status != intent.Status || requested.Reason != "" && requested.Reason != intent.Reason {
				return entity.ErrHookStoreConflict
			}
			if requestedDisplay != nil && (stored.DisplayMessage == nil || *requestedDisplay != *stored.DisplayMessage) {
				return entity.ErrHookStoreConflict
			}
			display = stored.DisplayMessage
			if stored.State.Finalize == entity.HookFinalizeCommitted {
				return e.unlockFinalizedHookRun(ctx, key)
			}
		}
		// Explicit cancellation fences admission before reading execution or doing cleanup.
		// The public two-call completion path cannot freeze its later display decision here.
		if stored.State.Finalize == entity.HookFinalizeNone && terminationRequested {
			intent = requested
			display = stored.DisplayMessage
			begun, err := deps.Runs.BeginFinalize(ctx, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: key, ExpectedVersion: stored.Version}, Intent: intent, DisplayMessage: display})
			if errors.Is(err, entity.ErrHookStoreConflict) {
				continue
			}
			if err != nil {
				return err
			}
			if begun.Run == nil || begun.Run.State.Key != key || begun.Run.State.Intent != intent || begun.Run.State.Finalize == entity.HookFinalizeNone {
				return entity.ErrHookStoreCorrupt
			}
			stored, display = begun.Run, begun.Run.DisplayMessage
			if stored.State.Finalize == entity.HookFinalizeCommitted {
				return e.unlockFinalizedHookRun(ctx, key)
			}
		}
		if err := e.prepareActiveHookTermination(ctx, source, stored); err != nil {
			return err
		}
		stats, err := deps.readStats(ctx, key)
		if err != nil {
			return err
		}
		if stored.State.Finalize == entity.HookFinalizeNone {
			if stored.State.Gate != entity.HookGateReady {
				return entity.ErrHookFinalizationUnsettled
			}
			intent = entity.HookTerminalIntent{Status: stats.NormalStatus(), Reason: requested.Reason}
			if requested.Status != 0 {
				if !stats.AllowsTerminalStatus(requested.Status) {
					return entity.ErrHookStoreConflict
				}
				intent.Status = requested.Status
			}
			begun, err := deps.Runs.BeginFinalize(ctx, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: key, ExpectedVersion: stored.Version}, Intent: intent, DisplayMessage: display})
			if errors.Is(err, entity.ErrHookStoreConflict) {
				continue
			}
			if err != nil {
				return err
			}
			if begun.Run == nil || begun.Run.State.Key != key || begun.Run.State.Intent != intent {
				return entity.ErrHookStoreCorrupt
			}
			stored = begun.Run
			if !begun.Changed {
				display = stored.DisplayMessage
			}
		}
		if !stats.AllowsTerminalStatus(intent.Status) {
			return entity.ErrHookStoreConflict
		}
		if err := e.cleanupHookNormalRun(ctx, source, stats); err != nil {
			return err
		}
		result, err := deps.Runs.CommitFinalize(ctx, entity.HookFinalizeInput{HookStoreGuard: entity.HookStoreGuard{Key: key, ExpectedVersion: stored.Version}, Intent: intent, Stats: stats, DisplayMessage: display})
		if errors.Is(err, entity.ErrHookStoreConflict) {
			continue
		}
		if err != nil {
			return err
		}
		if result.Run == nil || result.Run.State.Key != key || result.Run.State.Finalize != entity.HookFinalizeCommitted || result.Run.State.Intent != intent {
			return entity.ErrHookStoreCorrupt
		}
		if result.Changed && result.LatestProjected {
			if stats.ActiveTermination && !hookExecutionNil(e.publisher) {
				for start := 0; start < len(stats.ItemIDs); start += 100 {
					items := append([]int64(nil), stats.ItemIDs[start:min(start+100, len(stats.ItemIDs))]...)
					if err := e.publisher.PublishExptTurnResultFilterEvent(ctx, &entity.ExptTurnResultFilterEvent{
						ExperimentID: key.ExperimentID, SpaceID: key.WorkspaceID, ItemID: items,
						RetryTimes: gptr.Of(int32(0)), FilterType: gptr.Of(entity.UpsertExptTurnResultFilterTypeAuto),
					}, gptr.Of(10*time.Second)); err != nil {
						logs.CtxWarn(ctx, "Hook termination index refresh publication failed: %v", err)
					}
				}
			}
			message := intent.Reason
			if stats.NeverAdmitted {
				message = ""
			}
			if result.Run.DisplayMessage != nil {
				message = *result.Run.DisplayMessage
			}
			e.publishHookNormalCompletion(ctx, source, intent, opt, message)
		}
		return e.unlockFinalizedHookRun(ctx, key)
	}
	return entity.ErrHookStoreConflict
}

func (d *ExptManagerFinalizationDependencies) loadNormalRun(ctx context.Context, source *entity.HookFinalizationSource) (*entity.HookStoredRun, error) {
	online := source.Experiment.ExptType == entity.ExptType_Online && entity.ExptRunMode(source.RunLog.Mode) == entity.EvaluationModeAppend
	if !online && entity.ExptRunMode(source.RunLog.Mode) == entity.EvaluationModeAppend || (!online && source.Experiment.ExptType != entity.ExptType_Offline) ||
		!entity.HookBoundExecutionMode(entity.ExptRunMode(source.RunLog.Mode)) {
		return nil, entity.ErrHookFinalizationUnsupported
	}
	if online || entity.HookBoundRetryMode(entity.ExptRunMode(source.RunLog.Mode)) {
		owner, ok := d.Repository.(repo.IHookBoundExecutionOwner)
		if !ok {
			return nil, entity.ErrHookFinalizationUnsupported
		}
		key, scope, hash := owner.HookExecutionBinding()
		if key != source.Key || scope != d.ExecutionScope || hash == "" {
			return nil, entity.ErrHookFinalizationUnsupported
		}
	}
	stored, err := d.Runs.GetRun(ctx, source.Key)
	if err != nil {
		return nil, err
	}
	if stored == nil || stored.State.Key != source.Key || stored.Snapshot.ExecutionScope != d.ExecutionScope || entity.ValidateHookStorageState(&stored.State) != nil {
		return nil, entity.ErrHookStoreCorrupt
	}
	return stored, nil
}

func (d *ExptManagerFinalizationDependencies) readSource(ctx context.Context, key entity.HookRunKey) (*entity.HookFinalizationSource, error) {
	source, err := d.Repository.ReadFinalizationSource(ctx, key)
	if err != nil {
		return nil, err
	}
	if source == nil || !source.Managed || source.Key != key || source.RunLog == nil || source.Experiment == nil ||
		source.RunLog.ExptRunID != key.RunID || source.RunLog.ExptID != key.ExperimentID || source.RunLog.SpaceID != key.WorkspaceID ||
		source.Experiment.ID != key.ExperimentID || source.Experiment.SpaceID != key.WorkspaceID {
		return nil, entity.ErrHookStoreCorrupt
	}
	return source, nil
}

func (d *ExptManagerFinalizationDependencies) readStats(ctx context.Context, key entity.HookRunKey) (*entity.HookFinalizationStats, error) {
	s, err := d.Repository.ReadFinalizationStats(ctx, key, d.ExecutionScope)
	if err != nil {
		return nil, err
	}
	if s == nil || s.Key != key || s.ExecutionScope != d.ExecutionScope {
		return nil, entity.ErrHookStoreCorrupt
	}
	return s, nil
}

func (e *ExptMangerImpl) cleanupHookNormalRun(ctx context.Context, source *entity.HookFinalizationSource, stats *entity.HookFinalizationStats) error {
	key := source.Key
	if entity.IsCentralDispatch(source.Experiment.ExptDispatchMode) {
		if missingManagerHookDependency(e.centralGuard) || source.Experiment.SchedulerScope == "" {
			return entity.ErrHookStoreCorrupt
		}
		for _, itemID := range stats.ItemIDs {
			if err := e.centralGuard.Release(ctx, source.Experiment.SchedulerScope, key.RunID, itemID, "hook normal run finalized"); err != nil {
				return err
			}
		}
	}
	// AllowExptRun acquires this same workspace lock only after LogRun publishes Latest.
	// A successor published after this read cannot acquire its quota until Set completes.
	return e.quotaRepo.CreateOrUpdate(ctx, key.WorkspaceID, func(cur *entity.QuotaSpaceExpt) (*entity.QuotaSpaceExpt, bool, error) {
		current, err := e.finalization.readSource(ctx, key)
		if err != nil {
			return nil, false, err
		}
		if current.Experiment.LatestRunID != key.RunID || cur == nil || cur.ExptID2RunTime == nil {
			return cur, false, nil
		}
		if _, exists := cur.ExptID2RunTime[key.ExperimentID]; !exists {
			return cur, false, nil
		}
		delete(cur.ExptID2RunTime, key.ExperimentID)
		return cur, true, nil
	}, &entity.Session{UserID: source.RunLog.CreatedBy})
}

func (e *ExptMangerImpl) unlockFinalizedHookRun(ctx context.Context, key entity.HookRunKey) error {
	lockKey := e.makeExptMutexLockKey(key.ExperimentID)
	owner, err := e.finalization.Owners.ReadFinalizationOwner(ctx, lockKey)
	if err != nil {
		return err
	}
	if owner == "" {
		return nil
	}
	runID, err := managerHookLockRunID(owner)
	if err != nil {
		return err
	}
	if runID != key.RunID {
		return nil
	}
	if !strings.HasPrefix(owner, "hook_run:") {
		return entity.ErrHookStoreConflict
	}
	_, err = e.mutex.UnlockWithValue(ctx, lockKey, owner)
	return err
}

func (e *ExptMangerImpl) publishHookNormalCompletion(ctx context.Context, source *entity.HookFinalizationSource, intent entity.HookTerminalIntent, opt *entity.CompleteExptOption, display string) {
	expt := *source.Experiment
	from := entity.ExptStatus(source.RunLog.Status)
	expt.Status = intent.Status
	expt.StatusMessage = display
	expt.EndAt = gptr.Of(time.Now())
	e.notifyWorkflowPipelineOnExptFinished(ctx, &expt, expt.SpaceID, intent.Status)
	if expt.ExptTemplateMeta != nil && expt.ExptTemplateMeta.ID > 0 && e.templateManager != nil {
		if err := e.templateManager.UpdateExptInfo(ctx, expt.ExptTemplateMeta.ID, expt.SpaceID, expt.ID, intent.Status, 0, nil); err != nil {
			logs.CtxWarn(ctx, "Hook finalization template notification failed: %v", err)
		}
	}
	if !opt.NoAggrCalculate {
		if err := e.exptAggrResultService.PublishExptAggrResultEvent(ctx, &entity.AggrCalculateEvent{ExperimentID: expt.ID, SpaceID: expt.SpaceID, CalculateMode: entity.CreateAllFields}, gptr.Of(3*time.Second)); err != nil {
			logs.CtxWarn(ctx, "Hook finalization aggregation publication failed: %v", err)
		}
	}
	if err := e.sendExptCompleteEvent(ctx, &expt, gptr.Of(source.Key.RunID), from); err != nil {
		logs.CtxWarn(ctx, "Hook finalization notification failed: %v", err)
	}
	e.mtr.EmitExptExecResult(expt.SpaceID, int64(expt.ExptType), int64(intent.Status), gptr.Indirect(expt.StartAt))
	e.emitSandboxAgentExperimentFinished(ctx, &expt, intent.Status, *expt.EndAt)
}

// Legacy WithStatusMessage can cut a valid UTF-8 sequence at its byte limit.
// Drop only that incomplete tail; retain valid text verbatim and reject other invalid bytes.
func normalizeHookStatusMessage(message string) (string, error) {
	for offset := 0; offset < len(message); {
		r, size := utf8.DecodeRuneInString(message[offset:])
		if r == utf8.RuneError && size == 1 {
			if !utf8.FullRuneInString(message[offset:]) {
				return message[:offset], nil
			}
			return "", errors.New("invalid hook display message UTF-8")
		}
		offset += size
	}
	return message, nil
}
