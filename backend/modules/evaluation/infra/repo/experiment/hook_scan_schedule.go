// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"cmp"
	"context"
	"gorm.io/gorm"
	"slices"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"gorm.io/hints"
)

// NewHookScheduleScanRepo opts into start recovery without changing the legacy scan constructor.
func NewHookScheduleScanRepo(provider db.Provider) repo.IHookScanRepo {
	return &hookScheduleScanRepo{&hookRunRepo{provider: provider}}
}

type hookScheduleScanRepo struct{ *hookRunRepo }
type hookScheduleScanRow struct {
	SpaceID, ExptID, ExptRunID, Version int64
	PlanState, Gate, FinalizeState      int32
	ExecutionStarted                    bool
	TerminalAt                          *time.Time
	RunStatus, RunMode, Marker          *int32
}

func (r *hookScheduleScanRepo) ScanPreparingPlans(ctx context.Context, in entity.HookScanInput) (entity.HookScanPage[entity.HookRunCandidate], error) {
	var out entity.HookScanPage[entity.HookRunCandidate]
	if ctx == nil {
		return out, entity.ErrHookStoreCorrupt
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	if err := in.Validate(entity.HookScanPreparing); err != nil {
		return out, err
	}
	preparing, _, more, err := r.scanHookRows(ctx, in, entity.HookScanPreparing)
	if err != nil {
		return out, err
	}
	// Do not push sparse status predicates before LIMIT: every range must advance
	// across at most limit+1 lifecycle rows, even if every ready Run is already done.
	q := hookScheduleReadyScanQuery(r.provider.NewSession(ctx, db.WithMaster()), in)
	var ready []hookScheduleScanRow
	if err := q.Find(&ready).Error; err != nil {
		return out, err
	}
	if err := ctx.Err(); err != nil {
		return out, err
	}
	rows := make([]hookScheduleScanRow, 0, len(preparing)+len(ready))
	seen := make(map[entity.HookRunKey]bool)
	for _, row := range preparing {
		rows = append(rows, hookScheduleScanRow{SpaceID: row.SpaceID, ExptID: row.ExptID, ExptRunID: row.ExptRunID, Version: row.Version})
		seen[entity.HookRunKey{WorkspaceID: row.SpaceID, ExperimentID: row.ExptID, RunID: row.ExptRunID}] = true
	}
	for _, row := range ready {
		key := entity.HookRunKey{WorkspaceID: row.SpaceID, ExperimentID: row.ExptID, RunID: row.ExptRunID}
		if !seen[key] {
			rows = append(rows, row)
			seen[key] = true
		}
	}
	slices.SortFunc(rows, func(a, b hookScheduleScanRow) int {
		if d := cmp.Compare(a.SpaceID, b.SpaceID); d != 0 {
			return d
		}
		return cmp.Compare(a.ExptRunID, b.ExptRunID)
	})
	out.HasMore = more || len(ready) > in.Limit || len(rows) > in.Limit
	if len(rows) > in.Limit {
		rows = rows[:in.Limit]
	}
	if len(rows) == 0 {
		return out, nil
	}
	last := rows[len(rows)-1]
	out.NextCursor = &entity.HookScanCursor{Kind: entity.HookScanPreparing, ExecutionScope: in.ExecutionScope, Status: "preparing", Now: in.Now, WorkspaceID: last.SpaceID, RunID: last.ExptRunID}
	for _, row := range rows {
		if row.PlanState == 1 && row.RunMode != nil && (entity.ExptRunMode(*row.RunMode) == entity.EvaluationModeRetryItems || entity.ExptRunMode(*row.RunMode) == entity.EvaluationModeAppend) {
			online := entity.ExptRunMode(*row.RunMode) == entity.EvaluationModeAppend
			if (row.Gate == 1 || online && row.Gate == 0) && row.FinalizeState == 0 && row.TerminalAt == nil && row.Marker != nil && *row.Marker == 1 && row.RunStatus != nil && (entity.ExptStatus(*row.RunStatus) == entity.ExptStatus_Pending || entity.ExptStatus(*row.RunStatus) == entity.ExptStatus_Processing || online && entity.ExptStatus(*row.RunStatus) == entity.ExptStatus_Draining) {
				out.Candidates = append(out.Candidates, entity.HookRunCandidate{Key: entity.HookRunKey{WorkspaceID: row.SpaceID, ExperimentID: row.ExptID, RunID: row.ExptRunID}, Version: row.Version})
			}
			continue
		}
		if row.PlanState != 0 && (row.PlanState != 1 || row.Gate != 1 || row.ExecutionStarted || row.FinalizeState != 0 || row.TerminalAt != nil ||
			row.RunStatus == nil || entity.ExptStatus(*row.RunStatus) != entity.ExptStatus_Pending || row.Marker == nil || *row.Marker != 1 || row.RunMode == nil ||
			(entity.ExptRunMode(*row.RunMode) != entity.EvaluationModeSubmit && entity.ExptRunMode(*row.RunMode) != entity.EvaluationModeTrialRun && entity.ExptRunMode(*row.RunMode) != entity.EvaluationModeFailRetry && entity.ExptRunMode(*row.RunMode) != entity.EvaluationModeRetryAll)) {
			continue
		}
		out.Candidates = append(out.Candidates, entity.HookRunCandidate{Key: entity.HookRunKey{WorkspaceID: row.SpaceID, ExperimentID: row.ExptID, RunID: row.ExptRunID}, Version: row.Version})
	}
	return out, nil
}

func hookScheduleReadyScanQuery(s *gorm.DB, in entity.HookScanInput) *gorm.DB {
	q := s.Table("expt_lifecycle_run AS life").Clauses(hints.ForceIndex("idx_scope_plan_run")).
		Select("life.space_id, life.expt_id, life.expt_run_id, life.version, life.plan_state, life.gate, life.execution_started, life.finalize_state, life.terminal_at, log.status AS run_status, log.mode AS run_mode, log.lifecycle_hook_version AS marker").
		Joins("LEFT JOIN expt_run_log AS log ON log.id = life.expt_run_id AND log.space_id = life.space_id AND log.expt_id = life.expt_id AND log.expt_run_id = life.expt_run_id AND log.deleted_at IS NULL").
		Where("life.execution_scope = ? AND life.plan_state = ?", in.ExecutionScope, 1).Order("life.space_id, life.expt_run_id").Limit(in.Limit + 1)
	if c := in.Cursor; c != nil {
		q = q.Where("(life.space_id > ? OR (life.space_id = ? AND life.expt_run_id > ?))", c.WorkspaceID, c.WorkspaceID, c.RunID)
	}
	return q
}
