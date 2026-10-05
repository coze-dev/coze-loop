// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
)

func (r *hookRuntimeRouter) onlineManager(ctx context.Context, key entity.HookRunKey) (*service.ExptMangerImpl, error) {
	if !r.admissionInstalled {
		return r.existingRunManager(ctx, key)
	}
	m := r.HookConfigBaseManager()
	if m == nil {
		return nil, entity.ErrHookExecutionUnsupported
	}
	return m, nil
}

func (r *hookRuntimeRouter) StartOnlineWithHookSchedule(ctx context.Context, expt, run, space int64, retries int, user *entity.Session, ext map[string]string) (bool, error) {
	key := entity.HookRunKey{WorkspaceID: space, ExperimentID: expt, RunID: run}
	if !r.admissionInstalled {
		initial, err := r.initialization.ReadRunInitialization(ctx, key)
		if err != nil {
			return true, err
		}
		if initial == nil {
			return true, entity.ErrHookStoreCorrupt
		}
		if !initial.Managed {
			if initial.HooksEnabled {
				return true, entity.ErrHookConfigStorage
			}
			return false, nil
		}
	}
	m, err := r.onlineManager(ctx, key)
	if err != nil {
		return true, err
	}
	return m.StartOnlineWithHookSchedule(ctx, expt, run, space, retries, user, ext)
}

func (r *hookRuntimeRouter) PrepareOnlinePlan(ctx context.Context, key entity.HookRunKey) error {
	m, err := r.onlineManager(ctx, key)
	if err != nil {
		return err
	}
	return m.PrepareOnlinePlan(ctx, key)
}
func (r *hookRuntimeRouter) PublishOnlineContinuation(ctx context.Context, key entity.HookRunKey) error {
	m, err := r.onlineManager(ctx, key)
	if err != nil {
		return err
	}
	return m.PublishOnlineContinuation(ctx, key)
}
func (r *hookRuntimeRouter) ValidateOnlineRun(ctx context.Context, key entity.HookRunKey) (bool, error) {
	if key.RunID <= 0 {
		return true, entity.ErrHookStoreConflict
	}
	execution, _, err := r.execution(ctx, key)
	if err != nil {
		return true, err
	}
	if execution == nil {
		return false, nil
	}
	return execution.Manager.ValidateOnlineRun(ctx, key)
}
func (r *hookRuntimeRouter) CheckOnlineRun(ctx context.Context, key entity.HookRunKey) (bool, error) {
	if key.RunID <= 0 {
		return true, entity.ErrHookStoreConflict
	}
	execution, _, err := r.execution(ctx, key)
	if err != nil {
		return true, err
	}
	if execution == nil {
		return false, nil
	}
	return execution.Manager.CheckOnlineRun(ctx, key)
}
func (r *hookRuntimeRouter) Invoke(ctx context.Context, in *entity.InvokeExptReq) error {
	if in == nil || in.RunID <= 0 {
		return entity.ErrHookStoreConflict
	}
	execution, _, err := r.execution(ctx, entity.HookRunKey{WorkspaceID: in.SpaceID, ExperimentID: in.ExptID, RunID: in.RunID})
	if err != nil {
		return err
	}
	if execution == nil {
		return r.IExptManager.Invoke(ctx, in)
	}
	return execution.Manager.Invoke(ctx, in)
}
func (r *hookRuntimeRouter) Finish(ctx context.Context, expt *entity.Experiment, run int64, user *entity.Session) error {
	if expt == nil || run <= 0 {
		return entity.ErrHookStoreConflict
	}
	execution, _, err := r.execution(ctx, entity.HookRunKey{WorkspaceID: expt.SpaceID, ExperimentID: expt.ID, RunID: run})
	if err != nil {
		return err
	}
	if execution == nil {
		return r.IExptManager.Finish(ctx, expt, run, user)
	}
	return execution.Manager.Finish(ctx, expt, run, user)
}
