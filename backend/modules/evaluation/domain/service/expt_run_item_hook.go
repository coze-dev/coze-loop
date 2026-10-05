// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"maps"
	"sync"
	"time"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
)

const itemHookDependencyTimeout = 5 * time.Second
const itemHookWaitDelay = 5 * time.Second

type itemHookManagedKey struct{}

type itemHookControlError struct{ wait, terminalAck bool }

func (e itemHookControlError) Error() string {
	if e.wait {
		return "hook item admission waiting"
	}
	return "hook item admission or continuation unavailable"
}

type itemHookAdmission struct {
	gate     repo.IHookGateRepo
	source   repo.IHookItemSourceRepo
	runs     repo.IHookRepo
	progress repo.IHookTurnProgressRepo
}

// NewHookAwareExptRecordEvalService opts in without changing legacy construction or wiring.
func NewHookAwareExptRecordEvalService(base ExptItemEvalEvent, gate repo.IHookGateRepo, source repo.IHookItemSourceRepo, runs repo.IHookRepo, progress repo.IHookTurnProgressRepo) (ExptItemEvalEvent, error) {
	legacy, ok := base.(*ExptItemEventEvalServiceImpl)
	if !ok || legacy == nil {
		return nil, errors.New("missing hook item consumer dependency")
	}
	for _, dep := range []any{gate, source, runs, progress, legacy.publisher, legacy.manager, legacy.mutex, legacy.exptItemResultRepo} {
		if hookExecutionNil(dep) {
			return nil, errors.New("missing hook item consumer dependency")
		}
	}
	aware := *legacy
	aware.hookAdmission = &itemHookAdmission{gate: gate, source: source, runs: runs, progress: progress}
	// Method values in the original chain still reference legacy, not this copy.
	aware.endpoints = RecordEvalChain(
		acknowledgeHookTerminal,
		aware.HandleEventErr,
		aware.handleHookEventCheck,
		aware.hookControlStage(aware.HandleCentralAdmission),
		aware.hookControlStage(aware.HandleEventLock),
		aware.handleHookItemAdmission,
		aware.hookControlStage(aware.HandleCentralReservation),
		aware.HandleEventExec,
	)(func(context.Context, *entity.ExptItemEvalEvent) error { return nil })
	return &aware, nil
}

func acknowledgeHookTerminal(next RecordEvalEndPoint) RecordEvalEndPoint {
	return func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		err := next(ctx, event)
		var control itemHookControlError
		if errors.As(err, &control) && control.terminalAck {
			return nil
		}
		return err
	}
}

func itemHookManaged(ctx context.Context) bool {
	managed, _ := ctx.Value(itemHookManagedKey{}).(bool)
	return managed
}

func itemHookKey(event *entity.ExptItemEvalEvent) entity.HookRunKey {
	return entity.HookRunKey{WorkspaceID: event.SpaceID, ExperimentID: event.ExptID, RunID: event.ExptRunID}
}

func (e *ExptItemEventEvalServiceImpl) readItemHookSource(ctx context.Context, key entity.HookRunKey) (*entity.HookRunInitialization, error) {
	readCtx, cancel := context.WithTimeout(ctx, itemHookDependencyTimeout)
	defer cancel()
	source, err := e.hookAdmission.source.ReadItemSource(readCtx, key)
	if err != nil || readCtx.Err() != nil || source == nil || source.RunLog == nil {
		return nil, itemHookControlError{wait: true}
	}
	log := source.RunLog
	if log.ID != key.RunID || log.SpaceID != key.WorkspaceID || log.ExptID != key.ExperimentID || log.ExptRunID != key.RunID {
		return nil, itemHookControlError{wait: true}
	}
	decision := entity.CheckHookAdmission(entity.HookAdmissionInput{Requested: key, Actual: key, LatestRunID: source.LatestRunID, Status: entity.ExptStatus(log.Status), Marker: entity.HookMarkerLegacy})
	if decision.Gate == entity.HookGateWaiting {
		return nil, itemHookControlError{wait: true}
	}
	if source.Managed && decision.Gate == entity.HookGateClosed && !entity.IsExptFinished(entity.ExptStatus(log.Status)) {
		return nil, itemHookControlError{}
	}
	return source, nil
}

func (e *ExptItemEventEvalServiceImpl) checkItemHookGate(ctx context.Context, key entity.HookRunKey) error {
	readCtx, cancel := context.WithTimeout(ctx, itemHookDependencyTimeout)
	defer cancel()
	decision, err := e.hookAdmission.gate.CanDispatch(readCtx, key)
	if err != nil || readCtx.Err() != nil {
		return itemHookControlError{wait: true}
	}
	switch decision.Gate {
	case entity.HookGateReady:
		return nil
	case entity.HookGateClosed:
		return itemHookControlError{}
	default:
		return itemHookControlError{wait: true}
	}
}

func (e *ExptItemEventEvalServiceImpl) handleHookEventCheck(next RecordEvalEndPoint) RecordEvalEndPoint {
	legacyCheck := e.HandleEventCheck(next)
	return func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		if event == nil || event.EvalSetItemID <= 0 || (entity.HookStoreGuard{Key: itemHookKey(event)}).Validate() != nil {
			return itemHookControlError{}
		}
		source, err := e.readItemHookSource(ctx, itemHookKey(event))
		if err != nil {
			return err
		}
		if !source.Managed {
			if e.boundContext != nil {
				return itemHookControlError{}
			}
			return legacyCheck(ctx, event)
		}
		// Reports persist business results before this payload-free resume notification.
		if entity.IsExptFinished(entity.ExptStatus(source.RunLog.Status)) {
			return itemHookControlError{terminalAck: true}
		}
		if err = e.checkItemHookGate(ctx, itemHookKey(event)); err != nil {
			return err
		}
		// Managed closure must not hit the legacy terminal-message acknowledgement.
		return next(context.WithValue(ctx, itemHookManagedKey{}, true), event)
	}
}

// Only failures before a middleware invokes its successor are admission control.
// Execution failures continue through the existing result/retry handling.
func (e *ExptItemEventEvalServiceImpl) hookControlStage(stage RecordEvalMiddleware) RecordEvalMiddleware {
	return func(next RecordEvalEndPoint) RecordEvalEndPoint {
		legacy := stage(next)
		return func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
			if !itemHookManaged(ctx) {
				return legacy(ctx, event)
			}
			entered := false
			err := stage(func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
				entered = true
				return next(ctx, event)
			})(ctx, event)
			if !entered {
				// A nil early return is not proof that a callback result was consumed.
				return itemHookControlError{wait: err != nil}
			}
			return err
		}
	}
}

func (e *ExptItemEventEvalServiceImpl) handleHookItemAdmission(next RecordEvalEndPoint) RecordEvalEndPoint {
	return func(ctx context.Context, event *entity.ExptItemEvalEvent) error {
		if !itemHookManaged(ctx) {
			return next(ctx, event)
		}
		key := itemHookKey(event)
		if err := e.checkItemHookGate(ctx, key); err != nil {
			return err
		}
		var itemVersion int64
		err := func() error {
			admitCtx, cancel := context.WithTimeout(ctx, itemHookDependencyTimeout)
			defer cancel()
			item, err := e.exptItemResultRepo.GetItemRunLog(contexts.WithCtxWriteDB(admitCtx), event.ExptID, event.ExptRunID, event.EvalSetItemID, event.SpaceID)
			if err != nil || admitCtx.Err() != nil || item == nil || item.SpaceID != key.WorkspaceID || item.ExptID != key.ExperimentID || item.ExptRunID != key.RunID || item.ItemID != event.EvalSetItemID {
				return itemHookControlError{wait: true}
			}
			if entity.ItemRunState(item.Status) == entity.ItemRunState_Terminal {
				return itemHookControlError{}
			}
			if item.ItemVersionID < 0 {
				return itemHookControlError{wait: true}
			}
			itemVersion = item.ItemVersionID
			run, err := e.hookAdmission.runs.GetRun(admitCtx, key)
			if err != nil || admitCtx.Err() != nil || run == nil || run.State.Key != key || run.Version < 0 || entity.ValidateHookStorageState(&run.State) != nil {
				return itemHookControlError{wait: true}
			}
			if run.State.Gate == entity.HookGateClosed || run.State.Finalize != entity.HookFinalizeNone {
				return itemHookControlError{}
			}
			if !run.PlanReady || run.State.Gate != entity.HookGateReady {
				return itemHookControlError{wait: true}
			}
			// The repository arbitrates cancellation and admission in one transaction.
			result, err := e.hookAdmission.runs.AdmitItem(admitCtx, entity.HookAdmitItemInput{HookStoreGuard: entity.HookStoreGuard{Key: key, ExpectedVersion: run.Version}, ItemID: event.EvalSetItemID})
			if err != nil || admitCtx.Err() != nil || !result.Admitted || result.AdmittedAt.IsZero() || result.Version < run.Version {
				return itemHookControlError{wait: true}
			}
			return nil
		}()
		if err != nil {
			return err
		}
		// Keep the existing callback/target/evaluator flow; flags never grant admission.
		check := itemHookExecutionCheck(func(callCtx context.Context) error { return e.checkItemHookGate(callCtx, key) })
		ctx = context.WithValue(ctx, itemHookProgressContextKey{}, itemHookProgressBinding{key: key, itemID: event.EvalSetItemID, itemVersion: itemVersion, repo: e.hookAdmission.progress, targets: new(sync.Map)})
		return next(context.WithValue(ctx, itemHookExecutionKey{}, check), event)
	}
}

func (e *ExptItemEventEvalServiceImpl) publishItemHookWait(ctx context.Context, event *entity.ExptItemEvalEvent) error {
	if ctx == nil || ctx.Err() != nil || event == nil {
		return itemHookControlError{}
	}
	clone := *event
	clone.HookControlContinuation = true
	clone.Ext = maps.Clone(event.Ext)
	if event.Session != nil {
		session := *event.Session
		clone.Session = &session
	}
	publishCtx, cancel := context.WithTimeout(ctx, itemHookDependencyTimeout)
	defer cancel()
	delay := itemHookWaitDelay
	if err := e.publisher.PublishExptRecordEvalEvent(publishCtx, &clone, &delay, nil); err != nil || publishCtx.Err() != nil {
		return itemHookControlError{}
	}
	return nil
}
