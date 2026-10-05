// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"errors"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

// A committed frozen initialization replaces legacy startup, not the remaining mode duties.
func (e *ExptSchedulerImpl) resumeHookSchedulerInitialization(ctx context.Context, event *entity.ExptScheduleEvent) (initialized, stop bool, err error) {
	if e.hookGate == nil {
		return false, false, nil
	}
	if hookExecutionNil(e.hookScheduler) || hookExecutionNil(e.hookRuns) || hookExecutionNil(e.hookInitialization) {
		return false, true, schedulerHookRetryError{}
	}
	key := entity.HookRunKey{WorkspaceID: event.SpaceID, ExperimentID: event.ExptID, RunID: event.ExptRunID}
	source, err := e.hookScheduler.ReadFinalizationSource(ctx, key)
	if err != nil || source == nil || source.Key != key {
		return false, true, schedulerHookRetryError{}
	}
	if !source.Managed {
		return false, false, nil
	}
	run, err := e.hookRuns.GetRun(ctx, key)
	if err != nil || run == nil || run.State.Key != key || run.Snapshot.ExecutionScope != e.hookSchedulerScope || entity.ValidateHookStorageState(&run.State) != nil || source.RunLog == nil || source.Experiment == nil || entity.ExptRunMode(source.RunLog.Mode) != event.ExptRunMode {
		return false, true, schedulerHookRetryError{}
	}
	if !entity.HookExecutionInitializationRequired(run.State.Before.Status != entity.HookOperationDisabled || run.State.After.Status != entity.HookOperationDisabled, entity.ExptRunMode(source.RunLog.Mode), source.Experiment.ExptType, source.Experiment.EvalSetSourceType) && e.hookBoundInitializer == nil {
		return false, false, nil
	}
	if run.State.Gate == entity.HookGateClosed || run.State.Finalize != entity.HookFinalizeNone || source.Experiment.LatestRunID != key.RunID {
		return true, true, nil
	}
	page, err := e.hookInitialization.ReadExecutionInitializationPage(ctx, entity.HookExecutionInitializationReadInput{Key: key, ExecutionScope: e.hookSchedulerScope, Limit: 100})
	if errors.Is(err, entity.ErrHookAdmissionDenied) {
		return true, true, nil
	}
	if err != nil || page == nil || !page.Initialized || page.RunVersion < run.Version || page.Count != run.PlanCount || page.Hash != run.PlanHash || page.NextOrdinal != int64(len(page.Items)) {
		return false, true, schedulerHookRetryError{}
	}
	if init := e.hookBoundInitializer; init != nil && (key != init.boundKey || run.Snapshot.Hash != init.boundHash || page.BoundSnapshotHash != init.boundHash) {
		return false, true, schedulerHookRetryError{}
	}
	stop, err = e.projectHookSchedulerStartup(ctx, key)
	return true, stop, err
}
