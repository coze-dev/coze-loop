// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"reflect"

	"github.com/bytedance/gg/gptr"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

type hookGateRepo struct {
	provider       db.Provider
	executionScope func(context.Context) (string, error)
	execution      bool
	binding        *boundHookExecution
}

// executionScope must come from platform wiring, never from a user Hook field.
// It is evaluated only for managed Runs; legacy has no dependency on it.
func NewHookGateRepo(provider db.Provider, executionScope func(context.Context) (string, error)) repo.IHookGateRepo {
	return &hookGateRepo{provider: provider, executionScope: executionScope}
}

func NewHookExecutionGateRepo(provider db.Provider, executionScope func(context.Context) (string, error)) repo.IHookGateRepo {
	return &hookGateRepo{provider: provider, executionScope: executionScope, execution: true}
}

func (r *hookGateRepo) CanDispatch(ctx context.Context, key entity.HookRunKey) (entity.HookAdmissionDecision, error) {
	wait := entity.HookAdmissionDecision{Gate: entity.HookGateWaiting, Reason: "HOOK_STATE_UNAVAILABLE"}
	if r == nil || key.WorkspaceID <= 0 || key.ExperimentID <= 0 || key.RunID <= 0 || r.provider == nil {
		return wait, entity.ErrHookGateUnavailable
	}
	if r.binding != nil && r.binding.source.Key != key {
		return wait, entity.ErrHookGateUnavailable
	}
	if provider := reflect.ValueOf(r.provider); provider.Kind() == reflect.Ptr && provider.IsNil() {
		return wait, entity.ErrHookGateUnavailable
	}
	out := wait
	// Suppress driver SQL/parameters; callers observe the fixed error and reason.
	session := r.provider.NewSession(ctx, db.WithMaster()).Session(&gorm.Session{Logger: logger.Discard})
	needsSnapshot := r.binding != nil
	read := func(tx *gorm.DB, legacyOnly bool) error {
		var rows []hookGateLogRow
		logColumns := hookGateLogColumns
		if r.execution {
			logColumns += ",l.mode,e.expt_type,e.eval_set_source_type"
		}
		if err := tx.Unscoped().Table("expt_run_log AS l").Select(logColumns).
			Joins("LEFT JOIN experiment AS e ON e.id=l.expt_id").Where("l.id=?", key.RunID).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) != 1 || rows[0].ExperimentID == nil {
			return entity.ErrHookGateUnavailable
		}
		row := rows[0]
		if row.ID != key.RunID || row.LifecycleHookVersion != nil && *row.LifecycleHookVersion != 0 && *row.LifecycleHookVersion != 1 {
			return entity.ErrHookGateUnavailable
		}
		if legacyOnly && row.LifecycleHookVersion != nil && *row.LifecycleHookVersion == 1 {
			needsSnapshot = true
			return nil
		}
		if row.DeletedAt.Valid || row.ExperimentDeletedAt.Valid {
			out = entity.HookAdmissionDecision{Gate: entity.HookGateClosed, Reason: "RUN_CLOSED"}
			return nil
		}
		input := entity.HookAdmissionInput{Requested: key,
			Actual:      entity.HookRunKey{WorkspaceID: row.SpaceID, ExperimentID: row.ExptID, RunID: row.ExptRunID},
			LatestRunID: row.LatestRunID, Status: entity.ExptStatus(gptr.Indirect(row.Status)), Marker: entity.HookMarkerLegacy}
		out = entity.CheckHookAdmission(input)
		if out.Gate == entity.HookGateClosed {
			return nil
		}
		if out.Gate != entity.HookGateReady {
			return entity.ErrHookGateUnavailable
		}
		experimentInput := input
		experimentInput.Actual.WorkspaceID = row.ExperimentSpaceID
		experimentInput.Actual.ExperimentID = *row.ExperimentID
		experimentInput.Status = entity.ExptStatus(gptr.Indirect(row.ExperimentStatus))
		out = entity.CheckHookAdmission(experimentInput)
		if out.Gate == entity.HookGateClosed {
			return nil
		}
		if out.Gate != entity.HookGateReady {
			return entity.ErrHookGateUnavailable
		}
		if r.binding != nil && (row.LifecycleHookVersion == nil || *row.LifecycleHookVersion != 1) {
			return entity.ErrHookGateUnavailable
		}
		if row.LifecycleHookVersion == nil || *row.LifecycleHookVersion == 0 {
			return nil
		}
		if *row.LifecycleHookVersion != 1 || r.executionScope == nil {
			return entity.ErrHookGateUnavailable
		}
		scope, err := r.executionScope(ctx)
		if err != nil || !hookGateASCII(scope) {
			return entity.ErrHookGateUnavailable
		}
		var managed []hookGateStateRow
		stateColumns := hookGateStateColumns
		if r.execution {
			stateColumns += ",l.execution_initialized"
		}
		if err := tx.Table("expt_lifecycle_run AS l").Select(stateColumns).
			Joins("LEFT JOIN expt_lifecycle_hook_run AS o ON o.space_id=l.space_id AND o.expt_run_id=l.expt_run_id").
			Where("l.space_id=? AND l.expt_run_id=?", key.WorkspaceID, key.RunID).Limit(3).Find(&managed).Error; err != nil {
			return err
		}
		state, err := hookGateReadState(managed, key, input.Status, scope)
		if err != nil {
			return err
		}
		input.Marker, input.State = entity.HookMarkerManaged, state
		out = entity.CheckHookAdmission(input)
		if out.Reason == "HOOK_STATE_UNAVAILABLE" {
			return entity.ErrHookGateUnavailable
		}
		if err := checkBoundFinalizationRun(tx, key, scope, r.binding); err != nil {
			return err
		}
		required := r.binding != nil || entity.HookExecutionInitializationRequired(managed[0].State.BeforeEnabled || managed[0].State.AfterEnabled, entity.ExptRunMode(row.Mode), entity.ExptType(row.ExptType), entity.ExptEvalSetSourceType(row.EvalSetSourceType)) || hookMultiSetInitializationRequired(managed[0].State.BeforeEnabled || managed[0].State.AfterEnabled, entity.ExptRunMode(row.Mode), entity.ExptType(row.ExptType), entity.ExptEvalSetSourceType(row.EvalSetSourceType))
		if r.execution && out.Gate == entity.HookGateReady && required && (!managed[0].ExecutionInitialized || managed[0].State.PlanState != 1) {
			out = entity.HookAdmissionDecision{Gate: entity.HookGateWaiting, Reason: "HOOK_EXECUTION_PENDING"}
		}
		return nil
	}
	var err error
	if !needsSnapshot {
		err = read(session, true)
	}
	if err == nil && needsSnapshot {
		// A managed Run must be reread entirely in one snapshot, never joined to the probe.
		err = readHookSnapshot(ctx, r.provider, func(tx *gorm.DB) error { return read(tx, false) })
	}
	if err != nil {
		return wait, entity.ErrHookGateUnavailable
	}
	return out, nil
}

type hookGateLogRow struct {
	Mode, ExptType, EvalSetSourceType int32
	ID, SpaceID, ExptID, ExptRunID    int64
	Status                            *int64
	LifecycleHookVersion              *int32
	DeletedAt                         gorm.DeletedAt
	ExperimentID                      *int64
	ExperimentSpaceID, LatestRunID    int64
	ExperimentStatus                  *int64
	ExperimentDeletedAt               gorm.DeletedAt
}

const hookGateLogColumns = `l.id,l.space_id,l.expt_id,l.expt_run_id,l.lifecycle_hook_version,l.deleted_at,l.status,
e.id AS experiment_id,e.space_id AS experiment_space_id,e.latest_run_id,e.status AS experiment_status,e.deleted_at AS experiment_deleted_at`

type hookGateStateRow struct {
	ExecutionInitialized                 bool
	State                                hookSummaryRow `gorm:"embedded"`
	PlanCount, Version, OperationVersion int64
}

// Only state metadata is selected: no snapshots, configuration, identity or result.
const hookGateStateColumns = `l.space_id,l.expt_id,l.expt_run_id,l.before_enabled,l.after_enabled,l.execution_scope,l.gate,l.plan_state,l.finalize_state,
l.plan_count,l.version,COALESCE(OCTET_LENGTH(l.plan_hash)=64,false) AS plan_hash_valid,l.terminal_status,l.terminal_at,
(l.terminal_reason IS NOT NULL AND l.terminal_reason<>'') AS has_terminal_reason,
o.id AS operation_row_id,o.expt_id AS operation_expt_id,o.operation_id,o.execution_scope AS operation_scope,o.phase,o.status,o.attempt,o.updated_at,
(o.activated_at IS NOT NULL) AS activated,o.lease_generation,o.lease_until,o.attempt_deadline,o.operation_deadline,o.version AS operation_version`

func hookGateReadState(rows []hookGateStateRow, key entity.HookRunKey, status entity.ExptStatus, scope string) (*entity.HookRunState, error) {
	if len(rows) == 0 || len(rows) > 2 {
		return nil, entity.ErrHookGateUnavailable
	}
	first := rows[0].State
	state, err := hookSummaryRunState(first, key, status)
	if err != nil {
		return nil, entity.ErrHookGateUnavailable
	}
	seen := make(map[string]bool, 2)
	for _, stored := range rows {
		row := stored.State
		if row.SpaceID != key.WorkspaceID || row.ExptID != key.ExperimentID || row.ExptRunID != key.RunID ||
			row.ExecutionScope != scope || (!row.BeforeEnabled && !row.AfterEnabled) || stored.Version < 0 || stored.PlanCount < 0 ||
			row.OperationRowID == nil || *row.OperationRowID <= 0 || row.OperationExptID != key.ExperimentID || row.OperationScope != scope ||
			stored.OperationVersion < 0 || seen[row.Phase] || !hookGateASCII(row.OperationID) ||
			row.UpdatedAt == nil || row.UpdatedAt.IsZero() || row.Attempt < 0 || row.Attempt > 11 {
			return nil, entity.ErrHookGateUnavailable
		}
		switch entity.HookOperationStatus(row.Status) {
		case entity.HookOperationPending:
			if row.Attempt != 0 {
				return nil, entity.ErrHookGateUnavailable
			}
		case entity.HookOperationRunning, entity.HookOperationRetryWait, entity.HookOperationSucceeded:
			if row.Attempt == 0 {
				return nil, entity.ErrHookGateUnavailable
			}
		case entity.HookOperationFailed:
		default:
			return nil, entity.ErrHookGateUnavailable
		}
		switch {
		case row.Phase == "before" && row.BeforeEnabled:
			state.Before = hookSummaryOperationState(row)
		case row.Phase == "after" && row.AfterEnabled:
			state.After = hookSummaryOperationState(row)
		default:
			return nil, entity.ErrHookGateUnavailable
		}
		seen[row.Phase] = true
	}
	if seen["before"] != first.BeforeEnabled || seen["after"] != first.AfterEnabled ||
		(first.BeforeEnabled && first.Gate == 1 && first.PlanState != 1) || entity.ValidateHookStorageState(state) != nil {
		return nil, entity.ErrHookGateUnavailable
	}
	return state, nil
}

func hookGateASCII(value string) bool {
	if len(value) == 0 || len(value) > 128 {
		return false
	}
	for _, c := range value {
		if c < 33 || c > 126 {
			return false
		}
	}
	return true
}
