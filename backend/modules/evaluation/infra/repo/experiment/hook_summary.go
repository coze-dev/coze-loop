// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"database/sql"
	"time"

	"github.com/bytedance/gg/gptr"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

func NewHookSummaryRepo(provider db.Provider) repo.IHookSummaryRepo {
	return &hookRunRepo{provider: provider}
}

func (r *hookRunRepo) MGetSummaries(ctx context.Context, keys []entity.HookRunKey) (map[entity.HookRunKey]*entity.LifecycleHookRunSummary, error) {
	keys, err := entity.NormalizeHookSummaryKeys(keys)
	if err != nil {
		return nil, err
	}
	out := make(map[entity.HookRunKey]*entity.LifecycleHookRunSummary, len(keys))
	if len(keys) == 0 {
		return out, nil
	}
	ids := make([]int64, 0, len(keys))
	wanted := make(map[int64]entity.HookRunKey, len(keys))
	for _, key := range keys {
		ids = append(ids, key.RunID)
		wanted[key.RunID] = key
	}
	// Both reads share an MVCC snapshot, without locking or loading encrypted data.
	err = r.provider.NewSession(ctx, db.WithMaster()).Transaction(func(tx *gorm.DB) error {
		var logs []hookSummaryLogRow
		if err := tx.Table("expt_run_log AS l").
			Select("l.id, l.space_id, l.expt_id, l.expt_run_id, l.lifecycle_hook_version, l.deleted_at, l.status").
			Joins("JOIN experiment AS e ON e.id=l.expt_id AND e.space_id=l.space_id").
			Where("l.id IN ?", ids).Find(&logs).Error; err != nil {
			return err
		}
		if len(logs) != len(keys) {
			return entity.ErrHookSummaryUnavailable
		}
		var managed [][]any
		runStatuses := make(map[int64]entity.ExptStatus, len(logs))
		for _, row := range logs {
			key, ok := wanted[row.ID]
			if !ok || row.SpaceID != key.WorkspaceID || row.ExptID != key.ExperimentID || row.ExptRunID != key.RunID || row.DeletedAt.Valid {
				return entity.ErrHookSummaryUnavailable
			}
			out[key] = nil
			if row.LifecycleHookVersion == nil || *row.LifecycleHookVersion == 0 {
				continue
			}
			if *row.LifecycleHookVersion != 1 {
				return entity.ErrHookSummaryUnavailable
			}
			managed = append(managed, []any{key.WorkspaceID, key.RunID})
			runStatuses[key.RunID] = entity.ExptStatus(gptr.Indirect(row.Status))
		}
		if len(managed) == 0 {
			return nil
		}
		var rows []hookSummaryRow
		if err := tx.Table("expt_lifecycle_run AS l").Select(hookSummaryColumns).
			Joins("LEFT JOIN expt_lifecycle_hook_run AS o ON o.space_id=l.space_id AND o.expt_run_id=l.expt_run_id").
			Where("(l.space_id,l.expt_run_id) IN ?", managed).Limit(2*len(managed) + 1).Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) > 2*len(managed) {
			return entity.ErrHookSummaryUnavailable
		}
		seen := make(map[entity.HookRunKey]map[string]bool, len(managed))
		states := make(map[entity.HookRunKey]*entity.HookRunState, len(managed))
		for _, row := range rows {
			key, ok := wanted[row.ExptRunID]
			if !ok || row.SpaceID != key.WorkspaceID || row.ExptID != key.ExperimentID || (!row.BeforeEnabled && !row.AfterEnabled) {
				return entity.ErrHookSummaryUnavailable
			}
			if out[key] == nil {
				state, err := hookSummaryRunState(row, key, runStatuses[key.RunID])
				if err != nil {
					return err
				}
				states[key] = state
				out[key] = &entity.LifecycleHookRunSummary{RunID: key.RunID, Before: entity.HookRunSummary{Status: entity.HookOperationDisabled}, After: entity.HookRunSummary{Status: entity.HookOperationDisabled}}
				seen[key] = make(map[string]bool, 2)
			}
			if row.OperationRowID == nil || *row.OperationRowID <= 0 || row.OperationExptID != key.ExperimentID || row.ExecutionScope == "" || row.OperationScope != row.ExecutionScope || seen[key][row.Phase] {
				return entity.ErrHookSummaryUnavailable
			}
			summary, err := hookOperationSummary(row)
			if err != nil {
				return err
			}
			switch {
			case row.Phase == "before" && row.BeforeEnabled:
				out[key].Before = summary
				states[key].Before = hookSummaryOperationState(row)
			case row.Phase == "after" && row.AfterEnabled:
				out[key].After = summary
				states[key].After = hookSummaryOperationState(row)
			default:
				return entity.ErrHookSummaryUnavailable
			}
			seen[key][row.Phase] = true
		}
		for _, row := range rows {
			key := wanted[row.ExptRunID]
			if seen[key]["before"] != row.BeforeEnabled || seen[key]["after"] != row.AfterEnabled {
				return entity.ErrHookSummaryUnavailable
			}
		}
		if len(seen) != len(managed) {
			return entity.ErrHookSummaryUnavailable
		}
		for _, state := range states {
			if entity.ValidateHookStorageState(state) != nil {
				return entity.ErrHookSummaryUnavailable
			}
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, entity.ErrHookSummaryUnavailable
	}
	return out, nil
}

type hookSummaryLogRow struct {
	Status                         *int64
	ID, SpaceID, ExptID, ExptRunID int64
	LifecycleHookVersion           *int32
	DeletedAt                      gorm.DeletedAt
}

type hookSummaryRow struct {
	PlanHashValid, HasTerminalReason               bool
	TerminalStatus                                 *int32
	TerminalAt                                     *time.Time
	Activated                                      bool
	LeaseGeneration                                int64
	LeaseUntil, AttemptDeadline, OperationDeadline *time.Time
	SpaceID, ExptID, ExptRunID                     int64
	BeforeEnabled, AfterEnabled                    bool
	Gate, PlanState, FinalizeState                 int32
	ExecutionScope                                 string
	OperationRowID                                 *int64
	OperationExptID                                int64
	OperationID, OperationScope, Phase, Status     string
	Attempt                                        int32
	UpdatedAt                                      *time.Time
	ResultRedacted                                 []byte
	ErrorCode, ErrorMessage                        *string
}

// In-flight and irrelevant terminal fields never leave the database. Bound corrupt
// payloads before allocation; the extra byte makes overflow distinguishable.
const hookSummaryColumns = `l.space_id,l.expt_id,l.expt_run_id,l.before_enabled,l.after_enabled,l.execution_scope,l.gate,l.plan_state,l.finalize_state,
COALESCE(OCTET_LENGTH(l.plan_hash)=64,false) AS plan_hash_valid,l.terminal_status,l.terminal_at,
(l.terminal_reason IS NOT NULL AND l.terminal_reason<>'') AS has_terminal_reason,
o.id AS operation_row_id,o.expt_id AS operation_expt_id,o.operation_id,o.execution_scope AS operation_scope,o.phase,o.status,o.attempt,o.updated_at,
(o.activated_at IS NOT NULL) AS activated,o.lease_generation,o.lease_until,o.attempt_deadline,o.operation_deadline,
CASE WHEN o.status='succeeded' THEN SUBSTRING(o.result_redacted,1,32769) END AS result_redacted,
CASE WHEN o.status='failed' THEN SUBSTRING(o.error_code,1,129) END AS error_code,
CASE WHEN o.status='failed' THEN SUBSTRING(CAST(o.error_message AS BINARY),1,2049) END AS error_message`
