// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"maps"
	"slices"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

const schedulerHookWaitDelay = 5 * time.Second
const schedulerHookDependencyTimeout = 5 * time.Second

type schedulerHookRetryError struct{}

func (schedulerHookRetryError) Error() string { return "hook scheduler continuation unavailable" }

// NewHookAwareExptSchedulerSvc builds an isolated scheduler with the mandatory Gate.
// The old constructor and its generated wiring remain unchanged; this is not a setter.
func NewHookAwareExptSchedulerSvc(base ExptSchedulerEvent, gate repo.IHookGateRepo, initializers ...hook.ExecutionInitializer) (ExptSchedulerEvent, error) {
	legacy, ok := base.(*ExptSchedulerImpl)
	if !ok || legacy == nil || hookExecutionNil(gate) || hookExecutionNil(legacy.Publisher) {
		return nil, schedulerHookRetryError{}
	}
	aware := *legacy
	aware.hookGate = gate
	if len(initializers) > 1 || len(initializers) == 1 && hookExecutionNil(initializers[0]) {
		return nil, schedulerHookRetryError{}
	}
	if len(initializers) == 1 {
		aware.hookFrozenInitializer = initializers[0]
	}
	manager, ok := legacy.Manager.(*ExptMangerImpl)
	if !ok || manager == nil || manager.finalization == nil {
		return nil, schedulerHookRetryError{}
	}
	aware.hookRuns = manager.finalization.Runs
	aware.hookInitialization, ok = manager.finalization.Runs.(repo.IHookExecutionInitializationRepo)
	if !ok || hookExecutionNil(aware.hookInitialization) {
		return nil, schedulerHookRetryError{}
	}
	storage, ok := manager.finalization.Repository.(repo.IHookItemArchiveRepo)
	result, valid := legacy.ResultSvc.(*ExptResultServiceImpl)
	if !ok || !valid {
		return nil, schedulerHookRetryError{}
	}
	aware.hookScheduler, ok = storage.(repo.IHookSchedulerRepo)
	if !ok {
		return nil, schedulerHookRetryError{}
	}
	if _, ok := storage.(repo.IHookSchedulerFailureRepo); !ok {
		return nil, schedulerHookRetryError{}
	}
	aware.hookSchedulerScope = manager.finalization.ExecutionScope
	var err error
	aware.ResultSvc, err = result.WithHookArchive(storage, manager.finalization.ExecutionScope)
	if err != nil {
		return nil, err
	}
	// Rebind method values to the copy; the original chain closes over legacy.
	aware.Endpoints = aware.schedulerEndpoints()
	return &aware, nil
}

func (e *ExptSchedulerImpl) persistHookSubmits(ctx context.Context, event *entity.ExptScheduleEvent, itemIDs []int64) (bool, error) {
	if e.hookScheduler == nil {
		if e.hookGate != nil {
			return true, schedulerHookRetryError{}
		}
		return false, nil
	}
	key := entity.HookRunKey{WorkspaceID: event.SpaceID, ExperimentID: event.ExptID, RunID: event.ExptRunID}
	source, err := e.hookScheduler.ReadFinalizationSource(ctx, key)
	if err != nil {
		return true, err
	}
	if source == nil || source.Key != key {
		return true, entity.ErrHookStoreCorrupt
	}
	if !source.Managed {
		return false, nil
	}
	changed, err := e.hookScheduler.PersistHookDispatch(ctx, entity.HookSchedulerDispatchInput{Key: key, ExecutionScope: e.hookSchedulerScope, ItemIDs: itemIDs})
	if err != nil {
		return true, err
	}
	if len(changed) > 0 {
		// Filter indexing is best-effort, as in the legacy branch, and reads committed results.
		if err := e.ResultSvc.UpsertExptTurnResultFilter(ctx, event.SpaceID, event.ExptID, changed); err != nil {
			logs.CtxError(ctx, "hook scheduler filter publication failed: %v", err)
		}
	}
	return true, nil
}

func (e *ExptSchedulerImpl) schedulerEndpoints() SchedulerEndPoint {
	return SchedulerChain(e.HandleEventErr, e.SysOps, e.HandleEventCheck, e.HandleEventLock,
		e.HandleEventEndpoint, e.SandboxAgentHourlyNotify)(func(context.Context, *entity.ExptScheduleEvent) error { return nil })
}

func (e *ExptSchedulerImpl) waitForHookAdmission(ctx context.Context, event *entity.ExptScheduleEvent) (bool, error) {
	if e.hookGate == nil {
		return false, nil
	}
	if ctx == nil || ctx.Err() != nil || event == nil || event.SpaceID <= 0 || event.ExptID <= 0 || event.ExptRunID <= 0 {
		return true, schedulerHookRetryError{}
	}
	gateCtx, cancel := context.WithTimeout(ctx, schedulerHookDependencyTimeout)
	decision, err := e.hookGate.CanDispatch(gateCtx, entity.HookRunKey{WorkspaceID: event.SpaceID, ExperimentID: event.ExptID, RunID: event.ExptRunID})
	if gateCtx.Err() != nil {
		err = gateCtx.Err()
	}
	cancel()
	if e.hookBoundInitializer == nil && e.hookFrozenInitializer != nil && err == nil && decision.Reason == "HOOK_EXECUTION_PENDING" {
		key := entity.HookRunKey{WorkspaceID: event.SpaceID, ExperimentID: event.ExptID, RunID: event.ExptRunID}
		if _, initErr := e.hookFrozenInitializer.InitializeExecution(ctx, key, e.hookSchedulerScope); initErr != nil {
			if permanentHookInitializationError(initErr) {
				return true, e.finishHookSchedulerRun(ctx, event, initErr)
			}
			return true, e.publishHookWait(ctx, event)
		}
		gateCtx, cancel = context.WithTimeout(ctx, schedulerHookDependencyTimeout)
		decision, err = e.hookGate.CanDispatch(gateCtx, key)
		cancel()
	}
	if init := e.hookBoundInitializer; init != nil {
		if init.boundKey != (entity.HookRunKey{WorkspaceID: event.SpaceID, ExperimentID: event.ExptID, RunID: event.ExptRunID}) {
			return true, schedulerHookRetryError{}
		}
		if err == nil && decision.Gate == entity.HookGateClosed {
			run, readErr := e.hookRuns.GetRun(ctx, init.boundKey)
			if readErr != nil || run == nil {
				return true, schedulerHookRetryError{}
			}
			if run.State.Finalize == entity.HookFinalizePending {
				return true, e.finishHookSchedulerRun(ctx, event, nil)
			}
			return true, nil
		}
		if deadlineErr := e.boundHookSchedulerDeadline(ctx, event); deadlineErr != nil {
			return true, e.finishHookSchedulerRun(ctx, event, deadlineErr)
		}
		if err == nil && decision.Reason == "HOOK_EXECUTION_PENDING" {
			if _, initErr := init.InitializeExecution(ctx, init.boundKey, init.boundScope); initErr != nil {
				if permanentHookInitializationError(initErr) {
					return true, e.finishHookSchedulerRun(ctx, event, initErr)
				}
				return true, e.publishHookWait(ctx, event)
			}
			gateCtx, cancel = context.WithTimeout(ctx, schedulerHookDependencyTimeout)
			decision, err = e.hookGate.CanDispatch(gateCtx, init.boundKey)
			cancel()
		}
		if deadlineErr := e.boundHookSchedulerDeadline(ctx, event); deadlineErr != nil {
			return true, e.finishHookSchedulerRun(ctx, event, deadlineErr)
		}
	}
	if err == nil {
		switch decision.Gate {
		case entity.HookGateReady:
			return false, nil
		case entity.HookGateClosed:
			return true, nil
		}
	}
	// Waiting, unknown state and read errors share a bounded independent continuation.
	return true, e.publishHookWait(ctx, event)
}

func (e *ExptSchedulerImpl) publishHookWait(ctx context.Context, event *entity.ExptScheduleEvent) error {
	if ctx.Err() != nil {
		return schedulerHookRetryError{}
	}
	if err := e.boundHookSchedulerDeadline(ctx, event); err != nil {
		return e.finishHookSchedulerRun(ctx, event, err)
	}
	next := *event
	next.Ext = maps.Clone(event.Ext)
	next.ExecEvalSetItemIDs = slices.Clone(event.ExecEvalSetItemIDs)
	if event.Session != nil {
		session := *event.Session
		next.Session = &session
	}
	// Continuations retain the original watchdog deadline.
	publishCtx, cancel := context.WithTimeout(ctx, schedulerHookDependencyTimeout)
	defer cancel()
	delay := schedulerHookWaitDelay
	if err := e.Publisher.PublishExptScheduleEvent(publishCtx, &next, &delay); err != nil {
		return schedulerHookRetryError{}
	}
	if publishCtx.Err() != nil {
		return schedulerHookRetryError{}
	}
	return nil
}

func (e *ExptSchedulerImpl) boundHookSchedulerDeadline(ctx context.Context, event *entity.ExptScheduleEvent) error {
	if e.boundOnline(event) {
		return nil
	}
	if e.hookBoundInitializer != nil {
		interval := int64(e.Configer.GetExptExecConf(ctx, event.SpaceID).GetZombieIntervalSecond())
		if time.Now().Unix()-event.CreatedAt >= interval {
			return errno.NewExptZombieTimeoutErr(interval, event.ExptID, event.ExptRunID)
		}
	}
	return nil
}

func permanentHookInitializationError(err error) bool {
	for _, permanent := range []error{entity.ErrHookFrozenItemUnavailable, entity.ErrHookFrozenContentUnavailable,
		entity.ErrHookFrozenPlanInvalid, entity.ErrHookFrozenSchemaInvalid, entity.ErrHookStoreCorrupt,
		entity.ErrHookStoreMissing, entity.ErrHookExecutionUnsupported} {
		if errors.Is(err, permanent) {
			return true
		}
	}
	return false
}

func (e *ExptSchedulerImpl) finishHookSchedulerRun(ctx context.Context, event *entity.ExptScheduleEvent, cause error) error {
	var opts []entity.CompleteExptOptionFn
	if cause != nil {
		// System termination fences admission and proves partial initialization cleanup.
		opts = append(opts, entity.WithStatus(entity.ExptStatus_SystemTerminated), entity.WithStatusMessage(userVisibleErrMsg(cause)))
	}
	if err := e.Manager.CompleteExpt(ctx, event.ExptID, &event.ExptRunID, event.SpaceID, event.Session, opts...); err != nil {
		logs.CtxError(ctx, "hook scheduler finalization failed, run=%d: %v", event.ExptRunID, err)
		return schedulerHookRetryError{}
	}
	return nil
}
