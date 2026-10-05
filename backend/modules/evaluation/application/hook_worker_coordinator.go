// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
)

// Legacy snapshots without a seed cannot be reconstructed from current configuration.
var ErrHookWorkerScheduleRecoveryUnavailable = errors.New("original hook run schedule event recovery is unavailable")

type HookWorkerCoordinatorDependencies struct {
	ExecutionScope string
	Runs           repo.IHookRepo
	Preparer       hook.PlanPreparer
	Manager        interface {
		FinalizeRun(context.Context, entity.HookRunKey, entity.HookTerminalIntent) error
	}
	HookWake          hook.WakePublisher
	Gate              repo.IHookGateRepo
	Codec             hook.StorageCodec
	SchedulePublisher interface {
		PublishHookSchedule(context.Context, *entity.ExptScheduleEvent) error
	}
}

type HookWorkerCoordinator struct {
	deps HookWorkerCoordinatorDependencies
}

var _ hook.WorkerCoordinator = (*HookWorkerCoordinator)(nil)

func NewHookWorkerCoordinator(deps HookWorkerCoordinatorDependencies) (*HookWorkerCoordinator, error) {
	for _, dep := range []any{deps.Runs, deps.Preparer, deps.Manager, deps.HookWake, deps.Gate, deps.Codec, deps.SchedulePublisher} {
		if hookWorkerNil(dep) {
			return nil, ErrHookWorkerConfiguration
		}
	}
	if len(deps.ExecutionScope) == 0 || len(deps.ExecutionScope) > 128 {
		return nil, ErrHookWorkerConfiguration
	}
	for _, b := range []byte(deps.ExecutionScope) {
		if b < 33 || b > 126 {
			return nil, ErrHookWorkerConfiguration
		}
	}
	return &HookWorkerCoordinator{deps: deps}, nil
}

func (c *HookWorkerCoordinator) PreparePlan(ctx context.Context, in hook.WorkerRunInput) error {
	ctx, cancel, err := c.start(ctx, in.ExecutionScope, in.Candidate.Key, in.Candidate.Version)
	if err != nil {
		return err
	}
	defer cancel()
	run, err := c.read(ctx, in.Candidate.Key)
	if err != nil {
		return err
	}
	if !coordinatorRunOpen(run) {
		return nil
	}
	if run.Mode == entity.EvaluationModeAppend {
		publisher, ok := c.deps.SchedulePublisher.(interface {
			PrepareOnlinePlan(context.Context, entity.HookRunKey) error
			PublishOnlineContinuation(context.Context, entity.HookRunKey) error
		})
		if !ok {
			return entity.ErrHookExecutionUnsupported
		}
		if err := publisher.PrepareOnlinePlan(ctx, run.State.Key); err != nil {
			return err
		}
		run, err = c.read(ctx, in.Candidate.Key)
		if err != nil {
			return err
		}
		if !coordinatorRunOpen(run) || !run.PlanReady {
			return nil
		}
		if run.State.Gate == entity.HookGateReady {
			return publisher.PublishOnlineContinuation(ctx, run.State.Key)
		}
		return c.wakeOperation(ctx, run, run.State.Before)
	}
	if !run.PlanReady {
		// Candidate versions are scan hints; the preparer re-reads and fences its one page.
		if err := c.deps.Preparer.PreparePlan(ctx, in); err != nil {
			return err
		}
		run, err = c.read(ctx, in.Candidate.Key)
		if err != nil {
			return err
		}
	}
	if run.State.Finalize != entity.HookFinalizeNone {
		return c.finalize(ctx, run)
	}
	if !coordinatorRunOpen(run) || !run.PlanReady {
		return nil
	}
	if run.State.Gate == entity.HookGateReady {
		if run.Mode == entity.EvaluationModeRetryItems {
			if run.State.Status != entity.ExptStatus_Processing {
				return c.wakeEvaluation(ctx, run)
			}
			publisher, ok := c.deps.SchedulePublisher.(interface {
				PrepareRetryItemsTail(context.Context, entity.HookRunKey) (bool, error)
				PublishRetryItemsContinuation(context.Context, entity.HookRunKey) error
			})
			if !ok {
				return entity.ErrHookExecutionUnsupported
			}
			if _, err := publisher.PrepareRetryItemsTail(ctx, run.State.Key); err != nil {
				return err
			}
			return publisher.PublishRetryItemsContinuation(ctx, run.State.Key)
		}
		return c.wakeEvaluation(ctx, run)
	}
	return c.wakeOperation(ctx, run, run.State.Before)
}

func (c *HookWorkerCoordinator) FinalizeRun(ctx context.Context, in hook.WorkerRunInput) error {
	ctx, cancel, err := c.start(ctx, in.ExecutionScope, in.Candidate.Key, in.Candidate.Version)
	if err != nil {
		return err
	}
	defer cancel()
	run, err := c.read(ctx, in.Candidate.Key)
	if err != nil {
		return err
	}
	return c.finalize(ctx, run)
}

func (c *HookWorkerCoordinator) ApplyEffects(ctx context.Context, in hook.WorkerEffects) error {
	ctx, cancel, err := c.start(ctx, in.ExecutionScope, in.Key, 0)
	if err != nil {
		return err
	}
	defer cancel()
	if _, err := entity.EncodeHookWakeEvent(entity.HookWakeEvent{Run: in.Key, OperationID: in.OperationID, ExecutionScope: in.ExecutionScope}); err != nil {
		return err
	}
	run, err := c.read(ctx, in.Key)
	if err != nil {
		return err
	}
	before := run.State.Before.Status != entity.HookOperationDisabled && run.State.Before.ID == in.OperationID
	after := run.State.After.Status != entity.HookOperationDisabled && run.State.After.ID == in.OperationID
	if !before && !after {
		return entity.ErrHookStoreConflict
	}
	// Signals cannot create terminal intent; only persisted intent is recoverable.
	if in.BeginFinalize && run.State.Finalize != entity.HookFinalizeNone {
		return c.finalize(ctx, run)
	}
	if in.ActivateAfter && run.State.Finalize == entity.HookFinalizeCommitted {
		return c.wakeOperation(ctx, run, run.State.After)
	}
	if in.WakeEvaluation && before && coordinatorRunOpen(run) && run.PlanReady && run.State.Gate == entity.HookGateReady {
		if run.Mode == entity.EvaluationModeAppend {
			publisher, ok := c.deps.SchedulePublisher.(interface {
				PublishOnlineContinuation(context.Context, entity.HookRunKey) error
			})
			if !ok {
				return entity.ErrHookExecutionUnsupported
			}
			return publisher.PublishOnlineContinuation(ctx, run.State.Key)
		}
		return c.wakeEvaluation(ctx, run)
	}
	return nil
}

func (c *HookWorkerCoordinator) start(ctx context.Context, scope string, key entity.HookRunKey, version int64) (context.Context, context.CancelFunc, error) {
	if c == nil || ctx == nil {
		return nil, nil, ErrHookWorkerConfiguration
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	if scope != c.deps.ExecutionScope {
		return nil, nil, ErrHookWorkerScope
	}
	if err := (entity.HookStoreGuard{Key: key, ExpectedVersion: version}).Validate(); err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(contexts.WithCtxWriteDB(ctx), hookWorkerDependencyTimeout)
	return ctx, cancel, nil
}

func (c *HookWorkerCoordinator) read(ctx context.Context, key entity.HookRunKey) (*entity.HookStoredRun, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	run, err := c.deps.Runs.GetRun(ctx, key)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, err
	}
	if run == nil || run.State.Key != key || run.Snapshot.ExecutionScope != c.deps.ExecutionScope ||
		(entity.HookStoreGuard{Key: key, ExpectedVersion: run.Version}).Validate() != nil || entity.ValidateHookStorageState(&run.State) != nil {
		return nil, entity.ErrHookStoreCorrupt
	}
	return run, nil
}

func coordinatorRunOpen(run *entity.HookStoredRun) bool {
	return run.State.Finalize == entity.HookFinalizeNone && run.State.Gate != entity.HookGateClosed &&
		run.State.Status != entity.ExptStatus_Terminating && !entity.IsExptFinished(run.State.Status)
}

func (c *HookWorkerCoordinator) finalize(ctx context.Context, run *entity.HookStoredRun) error {
	if run.State.Finalize == entity.HookFinalizeNone {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	key, intent := run.State.Key, run.State.Intent
	if err := c.deps.Manager.FinalizeRun(ctx, key, intent); err != nil {
		return err
	}
	current, err := c.read(ctx, key)
	if err != nil {
		return err
	}
	if current.State.Finalize != entity.HookFinalizeCommitted || current.State.Intent != intent {
		return entity.ErrHookStoreConflict
	}
	return c.wakeOperation(ctx, current, current.State.After)
}

func (c *HookWorkerCoordinator) wakeOperation(ctx context.Context, run *entity.HookStoredRun, operation entity.HookOperation) error {
	if !operation.Activated || (operation.Status != entity.HookOperationPending && operation.Status != entity.HookOperationRetryWait) {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if operation.ID == run.State.Before.ID {
		decision, err := c.deps.Gate.CanDispatch(ctx, run.State.Key)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return err
		}
		switch decision.Gate {
		case entity.HookGateClosed:
			return nil
		case entity.HookGateWaiting, entity.HookGateReady:
		default:
			return entity.ErrHookStoreCorrupt
		}
	}
	event := entity.HookWakeEvent{Run: run.State.Key, OperationID: operation.ID, ExecutionScope: c.deps.ExecutionScope}
	if _, err := entity.EncodeHookWakeEvent(event); err != nil {
		return err
	}
	if err := c.deps.HookWake.PublishWake(ctx, event); err != nil {
		return err
	}
	return ctx.Err()
}

func (c *HookWorkerCoordinator) wakeEvaluation(ctx context.Context, run *entity.HookStoredRun) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	decision, err := c.deps.Gate.CanDispatch(ctx, run.State.Key)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return err
	}
	switch decision.Gate {
	case entity.HookGateClosed, entity.HookGateWaiting:
		return nil
	case entity.HookGateReady:
		snapshot, err := c.deps.Codec.DecodeSnapshot(ctx, run.State.Key, c.deps.ExecutionScope, run.Snapshot)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return err
		}
		if snapshot == nil {
			return entity.ErrHookStoreCorrupt
		}
		seed := snapshot.Input().Schedule
		if seed == nil {
			return ErrHookWorkerScheduleRecoveryUnavailable
		}
		current, err := c.read(ctx, run.State.Key)
		if err != nil {
			return err
		}
		if current.Snapshot.Hash != run.Snapshot.Hash {
			return entity.ErrHookStoreConflict
		}
		event, err := seed.Event(current.State.Key, c.deps.ExecutionScope, current.Mode, current.CreatedBy)
		if err != nil {
			return err
		}
		if !coordinatorRunOpen(current) || !current.PlanReady || current.State.Gate != entity.HookGateReady || current.ExecutionStarted || current.State.Status != entity.ExptStatus_Pending {
			return nil
		}
		decision, err := c.deps.Gate.CanDispatch(ctx, current.State.Key)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return err
		}
		if decision.Gate == entity.HookGateClosed || decision.Gate == entity.HookGateWaiting {
			return nil
		}
		if decision.Gate != entity.HookGateReady {
			return entity.ErrHookStoreCorrupt
		}
		if err := c.deps.SchedulePublisher.PublishHookSchedule(ctx, event); err != nil {
			return err
		}
		return ctx.Err()
	default:
		return entity.ErrHookStoreCorrupt
	}
}
