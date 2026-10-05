// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"errors"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/hook"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
)

type hookRunExecutionFactory interface {
	ForRun(context.Context, entity.HookRunKey) (*service.HookRuntimeExecution, error)
}

type hookDeletionExecutionFactory interface {
	ForDeletion(context.Context, []int64, int64) (*service.ExptMangerImpl, error)
}

func (r *hookRuntimeRouter) StartRetryItemsWithHookSchedule(ctx context.Context, expt, space int64, retries int, items []int64, user *entity.Session, ext map[string]string) (bool, int64, bool, error) {
	manager := r.HookConfigBaseManager()
	if manager == nil {
		return true, 0, false, entity.ErrHookExecutionUnsupported
	}
	if !r.admissionInstalled {
		if ctx == nil {
			return true, 0, false, entity.ErrHookStoreCorrupt
		}
		got, err := manager.Get(contexts.WithCtxWriteDB(ctx), expt, space, user)
		if err != nil {
			return true, 0, false, err
		}
		if got == nil {
			return true, 0, false, entity.ErrHookStoreMissing
		}
		if got.LatestRunID <= 0 {
			return false, 0, false, nil
		}
		key := entity.HookRunKey{WorkspaceID: space, ExperimentID: expt, RunID: got.LatestRunID}
		initial, err := r.initialization.ReadRunInitialization(ctx, key)
		if err != nil {
			return true, 0, false, err
		}
		if initial == nil {
			return true, 0, false, entity.ErrHookStoreCorrupt
		}
		if !initial.Managed {
			return false, 0, false, nil
		}
		manager, err = r.existingRunManager(ctx, key)
		if err != nil {
			return true, 0, false, err
		}
	}
	return manager.StartRetryItemsWithHookSchedule(ctx, expt, space, retries, items, user, ext)
}

func (r *hookRuntimeRouter) PrepareRetryItemsTail(ctx context.Context, key entity.HookRunKey) (bool, error) {
	execution, _, err := r.execution(ctx, key)
	if err != nil {
		return false, err
	}
	if execution == nil {
		return false, entity.ErrHookExecutionUnsupported
	}
	return execution.Scheduler.PrepareRetryItemsTail(ctx, key)
}

func (r *hookRuntimeRouter) PublishRetryItemsContinuation(ctx context.Context, key entity.HookRunKey) error {
	manager := r.HookConfigBaseManager()
	if !r.admissionInstalled {
		var err error
		manager, err = r.existingRunManager(ctx, key)
		if err != nil {
			return err
		}
	}
	if manager == nil {
		return entity.ErrHookExecutionUnsupported
	}
	return manager.PublishRetryItemsContinuation(ctx, key)
}

var _ hookDeletionExecutionFactory = (*HookRuntimeExecutionFactory)(nil)

// The shared services retain the single-set path; only authenticated snapshots
// without a bound execution may use it after the per-Run factory rejects them.
type hookRuntimeRouter struct {
	service.IExptManager
	source             repo.IHookFinalizationRepo
	initialization     repo.IHookRunInitializationRepo
	runs               repo.IHookRepo
	codec              hook.StorageCodec
	scope              string
	factory            hookRunExecutionFactory
	scheduler          service.ExptSchedulerEvent
	consumer           service.ExptItemEvalEvent
	runtime            *HookRuntimeServices
	admissionInstalled bool
	stores             *HookRuntimeStores
}

func (r *hookRuntimeRouter) execution(ctx context.Context, key entity.HookRunKey) (*service.HookRuntimeExecution, entity.HookRunKey, error) {
	if ctx == nil || ctx.Err() != nil || r == nil || hookWorkerNil(r.source) || key.WorkspaceID <= 0 || key.ExperimentID <= 0 || key.RunID < 0 {
		return nil, key, entity.ErrHookStoreCorrupt
	}
	source, err := r.source.ReadFinalizationSource(ctx, key)
	if err != nil {
		return nil, key, err
	}
	if source == nil || source.Key.WorkspaceID != key.WorkspaceID || source.Key.ExperimentID != key.ExperimentID || source.Key.RunID <= 0 || key.RunID != 0 && source.Key.RunID != key.RunID {
		return nil, key, entity.ErrHookStoreCorrupt
	}
	key = source.Key
	if !source.Managed {
		return nil, key, nil
	}
	if hookWorkerNil(r.factory) {
		return nil, key, entity.ErrHookExecutionUnsupported
	}
	factory := r.factory
	if !r.admissionInstalled && source.RunLog != nil && entity.ExptRunMode(source.RunLog.Mode) == entity.EvaluationModeAppend {
		manager, err := r.existingRunManager(ctx, key)
		if err != nil {
			return nil, key, err
		}
		configured, ok := factory.(*HookRuntimeExecutionFactory)
		if !ok {
			return nil, key, entity.ErrHookExecutionUnsupported
		}
		copy := *configured
		copy.deps.Manager = manager
		factory = &copy
	}
	execution, err := factory.ForRun(ctx, key)
	if err == nil {
		if execution == nil || execution.Manager == nil || execution.Scheduler == nil || execution.Consumer == nil {
			return nil, key, entity.ErrHookExecutionUnsupported
		}
		return execution, key, nil
	}
	if !errors.Is(err, entity.ErrHookExecutionUnsupported) {
		return nil, key, err
	}
	if hookWorkerNil(r.runs) || hookWorkerNil(r.codec) {
		return nil, key, err
	}
	stored, readErr := r.runs.GetRun(ctx, key)
	if readErr != nil {
		return nil, key, readErr
	}
	if stored == nil || stored.State.Key != key || stored.Snapshot.ExecutionScope != r.scope {
		return nil, key, entity.ErrHookStoreCorrupt
	}
	snapshot, readErr := r.codec.DecodeSnapshot(ctx, key, r.scope, stored.Snapshot)
	if readErr != nil {
		return nil, key, readErr
	}
	if snapshot == nil {
		return nil, key, entity.ErrHookStoreCorrupt
	}
	input := snapshot.Input()
	if stored.Mode == entity.EvaluationModeRetryItems || stored.Mode == entity.EvaluationModeAppend || input.Key != key || input.ExecutionScope != r.scope || input.Execution != nil || input.Context == nil || len(input.Context.EvalSets) != 1 ||
		source.Experiment == nil || source.Experiment.EvalSetSourceType == entity.ExptEvalSetSourceType_MultiSetConfig {
		return nil, key, entity.ErrHookExecutionUnsupported
	}
	return nil, key, nil
}

func (r *hookRuntimeRouter) Schedule(ctx context.Context, event *entity.ExptScheduleEvent) error {
	if event == nil {
		return entity.ErrHookStoreCorrupt
	}
	execution, _, err := r.execution(ctx, entity.HookRunKey{WorkspaceID: event.SpaceID, ExperimentID: event.ExptID, RunID: event.ExptRunID})
	if err != nil {
		return err
	}
	if execution != nil {
		return execution.Scheduler.Schedule(ctx, event)
	}
	return r.scheduler.Schedule(ctx, event)
}

func (r *hookRuntimeRouter) Eval(ctx context.Context, event *entity.ExptItemEvalEvent) error {
	if event == nil {
		return entity.ErrHookStoreCorrupt
	}
	execution, _, err := r.execution(ctx, entity.HookRunKey{WorkspaceID: event.SpaceID, ExperimentID: event.ExptID, RunID: event.ExptRunID})
	if err != nil {
		return err
	}
	if execution != nil {
		return execution.Consumer.Eval(ctx, event)
	}
	return r.consumer.Eval(ctx, event)
}

func (r *hookRuntimeRouter) CompleteRun(ctx context.Context, exptID, runID, spaceID int64, session *entity.Session, opts ...entity.CompleteExptOptionFn) error {
	execution, key, err := r.execution(ctx, entity.HookRunKey{WorkspaceID: spaceID, ExperimentID: exptID, RunID: runID})
	if err != nil {
		return err
	}
	if execution != nil {
		return execution.Manager.CompleteRun(ctx, exptID, key.RunID, spaceID, session, opts...)
	}
	return r.IExptManager.CompleteRun(ctx, exptID, key.RunID, spaceID, session, opts...)
}

func (r *hookRuntimeRouter) CompleteExpt(ctx context.Context, exptID int64, runID *int64, spaceID int64, session *entity.Session, opts ...entity.CompleteExptOptionFn) error {
	key := entity.HookRunKey{WorkspaceID: spaceID, ExperimentID: exptID}
	if runID != nil {
		key.RunID = *runID
	}
	execution, key, err := r.execution(ctx, key)
	if err != nil {
		return err
	}
	if execution != nil {
		return execution.Manager.CompleteExpt(ctx, exptID, &key.RunID, spaceID, session, opts...)
	}
	return r.IExptManager.CompleteExpt(ctx, exptID, &key.RunID, spaceID, session, opts...)
}

func (r *hookRuntimeRouter) Kill(ctx context.Context, exptID int64, runID *int64, spaceID int64, msg string, session *entity.Session) error {
	return r.CompleteExpt(ctx, exptID, runID, spaceID, session, entity.WithStatus(entity.ExptStatus_Terminated), entity.WithStatusMessage(msg))
}

func (r *hookRuntimeRouter) Delete(ctx context.Context, exptID, spaceID int64, session *entity.Session) error {
	return r.MDelete(ctx, []int64{exptID}, spaceID, session)
}

func (r *hookRuntimeRouter) MDelete(ctx context.Context, ids []int64, spaceID int64, session *entity.Session) error {
	if r == nil {
		return entity.ErrHookExecutionUnsupported
	}
	factory, ok := r.factory.(hookDeletionExecutionFactory)
	if !ok || hookWorkerNil(factory) {
		return entity.ErrHookExecutionUnsupported
	}
	manager, err := factory.ForDeletion(ctx, ids, spaceID)
	if err != nil {
		return err
	}
	if manager == nil {
		return entity.ErrHookExecutionUnsupported
	}
	return manager.MDelete(ctx, ids, spaceID, session)
}

func (r *hookRuntimeRouter) FinalizeRun(ctx context.Context, key entity.HookRunKey, intent entity.HookTerminalIntent) error {
	execution, key, err := r.execution(ctx, key)
	if err != nil {
		return err
	}
	if execution != nil {
		return execution.Manager.FinalizeRun(ctx, key, intent)
	}
	manager := r.HookConfigBaseManager()
	if manager == nil {
		return entity.ErrHookExecutionUnsupported
	}
	return manager.FinalizeRun(ctx, key, intent)
}

func (r *hookRuntimeRouter) StartRunWithHookSchedule(ctx context.Context, exptID, runID, spaceID int64, retries int, session *entity.Session, mode entity.ExptRunMode, ext map[string]string) (bool, error) {
	if !r.admissionInstalled {
		if hookWorkerNil(r.initialization) {
			return true, entity.ErrHookConfigStorage
		}
		initial, err := r.initialization.ReadRunInitialization(ctx, entity.HookRunKey{WorkspaceID: spaceID, ExperimentID: exptID, RunID: runID})
		if err != nil {
			return true, err
		}
		if initial == nil || initial.Managed || initial.HooksEnabled {
			if initial != nil && initial.Managed {
				manager, err := r.existingRunManager(ctx, entity.HookRunKey{WorkspaceID: spaceID, ExperimentID: exptID, RunID: runID})
				if err != nil {
					return true, err
				}
				return manager.StartRunWithHookSchedule(ctx, exptID, runID, spaceID, retries, session, mode, ext)
			}
			return true, entity.ErrHookConfigStorage
		}
		return false, nil
	}
	manager := r.HookConfigBaseManager()
	if manager == nil {
		return true, entity.ErrHookExecutionUnsupported
	}
	return manager.StartRunWithHookSchedule(ctx, exptID, runID, spaceID, retries, session, mode, ext)
}

func (r *hookRuntimeRouter) LogRunWithPlanSeed(ctx context.Context, exptID, runID int64, mode entity.ExptRunMode, spaceID int64, raw string, session *entity.Session) error {
	manager := r.HookConfigBaseManager()
	if manager == nil {
		return entity.ErrHookExecutionUnsupported
	}
	return manager.LogRunWithLegacyHookGuard(ctx, r.initialization, exptID, runID, mode, spaceID, nil, &raw, session)
}

func (r *hookRuntimeRouter) LogRun(ctx context.Context, exptID, runID int64, mode entity.ExptRunMode, spaceID int64, itemIDs []int64, session *entity.Session) error {
	manager := r.HookConfigBaseManager()
	if manager == nil {
		return entity.ErrHookExecutionUnsupported
	}
	return manager.LogRunWithLegacyHookGuard(ctx, r.initialization, exptID, runID, mode, spaceID, itemIDs, nil, session)
}

func (r *hookRuntimeRouter) PublishHookSchedule(ctx context.Context, event *entity.ExptScheduleEvent) error {
	if event == nil {
		return entity.ErrHookStoreCorrupt
	}
	manager := r.HookConfigBaseManager()
	if manager == nil {
		return entity.ErrHookExecutionUnsupported
	}
	if !r.admissionInstalled {
		var err error
		manager, err = r.existingRunManager(ctx, entity.HookRunKey{WorkspaceID: event.SpaceID, ExperimentID: event.ExptID, RunID: event.ExptRunID})
		if err != nil {
			return err
		}
	}
	return manager.PublishHookSchedule(ctx, event)
}

// Recovery uses the original envelope's authenticated key, never a replacement
// for the missing current write key or a new admission.
func (r *hookRuntimeRouter) existingRunManager(ctx context.Context, key entity.HookRunKey) (*service.ExptMangerImpl, error) {
	if r.stores == nil || r.HookConfigBaseManager() == nil || hookWorkerNil(r.runs) || hookWorkerNil(r.codec) {
		return nil, entity.ErrHookConfigStorage
	}
	run, err := r.runs.GetRun(ctx, key)
	if err != nil {
		return nil, err
	}
	if run == nil || run.State.Key != key || run.Snapshot.ExecutionScope != r.scope || run.Snapshot.KeyID == "" {
		return nil, entity.ErrHookStoreCorrupt
	}
	snapshot, err := r.codec.DecodeSnapshot(ctx, key, r.scope, run.Snapshot)
	if err != nil {
		return nil, err
	}
	if snapshot == nil || snapshot.Input().Key != key || snapshot.Input().ExecutionScope != r.scope {
		return nil, entity.ErrHookStoreCorrupt
	}
	p := r.stores.Platform
	manager, err := service.NewExptManagerWithHooks(r.HookConfigBaseManager(), service.ExptManagerHookDependencies{
		Initialization: r.initialization, Runs: r.runs, Configs: r.stores.Configs, Codec: r.codec, Identity: p.Identity,
		Runtime: hookRuntimeAdmissionConfig{p}, Wake: p.Wake, ExecutionScope: r.scope, SnapshotKeyID: run.Snapshot.KeyID})
	if err != nil {
		return nil, err
	}
	return manager.(*service.ExptMangerImpl), nil
}

func (r *hookRuntimeRouter) HookConfigBaseManager() *service.ExptMangerImpl {
	base, _ := r.IExptManager.(*service.ExptMangerImpl)
	return base
}

var (
	_ service.IExptManager            = (*hookRuntimeRouter)(nil)
	_ service.IHookRunScheduleStarter = (*hookRuntimeRouter)(nil)
	_ service.ExptSchedulerEvent      = (*hookRuntimeRouter)(nil)
	_ service.ExptItemEvalEvent       = (*hookRuntimeRouter)(nil)
)
