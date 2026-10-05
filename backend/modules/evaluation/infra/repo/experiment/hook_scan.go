// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"time"

	"gorm.io/gorm"
	"gorm.io/hints"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

func NewHookScanRepo(provider db.Provider) repo.IHookScanRepo {
	return &hookRunRepo{provider: provider}
}

func (r *hookRunRepo) ScanDueOperations(ctx context.Context, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
	return r.scanHookOperations(ctx, in, entity.HookScanDue)
}

func (r *hookRunRepo) ScanExpiredOperations(ctx context.Context, in entity.HookScanInput) (entity.HookScanPage[entity.HookOperationCandidate], error) {
	return r.scanHookOperations(ctx, in, entity.HookScanExpired)
}

func (r *hookRunRepo) ScanPreparingPlans(ctx context.Context, in entity.HookScanInput) (entity.HookScanPage[entity.HookRunCandidate], error) {
	return r.scanHookRuns(ctx, in, entity.HookScanPreparing)
}

func (r *hookRunRepo) ScanPendingFinalizations(ctx context.Context, in entity.HookScanInput) (entity.HookScanPage[entity.HookRunCandidate], error) {
	return r.scanHookRuns(ctx, in, entity.HookScanFinalize)
}

type hookScanRow struct {
	ID, SpaceID, ExptID, ExptRunID, Version int64
	OperationID, Phase                      string
	ScanAt                                  time.Time
}

func (r *hookRunRepo) scanHookOperations(ctx context.Context, in entity.HookScanInput, kind entity.HookScanKind) (entity.HookScanPage[entity.HookOperationCandidate], error) {
	rows, cursor, more, err := r.scanHookRows(ctx, in, kind)
	if err != nil {
		return entity.HookScanPage[entity.HookOperationCandidate]{}, err
	}
	out := entity.HookScanPage[entity.HookOperationCandidate]{NextCursor: cursor, HasMore: more}
	for _, row := range rows {
		out.Candidates = append(out.Candidates, entity.HookOperationCandidate{
			Key: entity.HookRunKey{WorkspaceID: row.SpaceID, ExperimentID: row.ExptID, RunID: row.ExptRunID},
			ID:  row.ID, OperationID: row.OperationID, Phase: entity.HookPhase(row.Phase), Version: row.Version, DueAt: row.ScanAt,
		})
	}
	return out, nil
}

func (r *hookRunRepo) scanHookRuns(ctx context.Context, in entity.HookScanInput, kind entity.HookScanKind) (entity.HookScanPage[entity.HookRunCandidate], error) {
	rows, cursor, more, err := r.scanHookRows(ctx, in, kind)
	if err != nil {
		return entity.HookScanPage[entity.HookRunCandidate]{}, err
	}
	out := entity.HookScanPage[entity.HookRunCandidate]{NextCursor: cursor, HasMore: more}
	for _, row := range rows {
		out.Candidates = append(out.Candidates, entity.HookRunCandidate{
			Key:     entity.HookRunKey{WorkspaceID: row.SpaceID, ExperimentID: row.ExptID, RunID: row.ExptRunID},
			Version: row.Version, ReconcileAt: row.ScanAt,
		})
	}
	return out, nil
}

func (r *hookRunRepo) scanHookRows(ctx context.Context, in entity.HookScanInput, kind entity.HookScanKind) ([]hookScanRow, *entity.HookScanCursor, bool, error) {
	if err := in.Validate(kind); err != nil {
		return nil, nil, false, err
	}
	var rows []hookScanRow
	if err := hookScanQuery(r.provider.NewSession(ctx, db.WithMaster()), in, kind).Find(&rows).Error; err != nil {
		return nil, nil, false, err
	}
	more := len(rows) > in.Limit
	if more {
		rows = rows[:in.Limit]
	}
	if len(rows) == 0 {
		return rows, nil, false, nil
	}
	last := rows[len(rows)-1]
	cursor := &entity.HookScanCursor{Kind: kind, ExecutionScope: in.ExecutionScope, Now: in.Now, At: last.ScanAt}
	switch kind {
	case entity.HookScanDue:
		cursor.Status, cursor.ID = string(in.Status), last.ID
	case entity.HookScanExpired:
		cursor.Status, cursor.ID = "running", last.ID
	case entity.HookScanPreparing:
		cursor.Status, cursor.WorkspaceID, cursor.RunID = "preparing", last.SpaceID, last.ExptRunID
	case entity.HookScanFinalize:
		cursor.Status, cursor.WorkspaceID, cursor.RunID = "pending", last.SpaceID, last.ExptRunID
	}
	return rows, cursor, more, nil
}

// Only equal-prefix, single-category index ranges are scanned. The ORs below
// expand lexicographic keys; row-constructor inequalities can miss MySQL range optimization.
func hookScanQuery(s *gorm.DB, in entity.HookScanInput, kind entity.HookScanKind) *gorm.DB {
	q := s.Where("execution_scope = ?", in.ExecutionScope).Limit(in.Limit + 1)
	c := in.Cursor
	switch kind {
	case entity.HookScanDue, entity.HookScanExpired:
		column, status := "next_attempt_at", string(in.Status)
		if kind == entity.HookScanExpired {
			column, status = "lease_until", "running"
		}
		q = q.Table("expt_lifecycle_hook_run").Select("id, space_id, expt_id, expt_run_id, operation_id, phase, version, "+column+" AS scan_at").
			Where("status = ? AND "+column+" <= ?", status, hookScanSQLTime(in.Now)).Order(column + ", id")
		if kind == entity.HookScanDue {
			q = q.Where("activated_at IS NOT NULL")
		}
		if c != nil {
			q = q.Where("("+column+" > ? OR ("+column+" = ? AND id > ?))", hookScanSQLTime(c.At), hookScanSQLTime(c.At), c.ID)
		}
	case entity.HookScanPreparing:
		// Without this hint MySQL can choose PRIMARY and scan other scopes on deep pages.
		q = q.Clauses(hints.ForceIndex("idx_scope_plan_run"))
		q = q.Table("expt_lifecycle_run").Select("space_id, expt_id, expt_run_id, version").
			Where("plan_state = ?", 0).Order("space_id, expt_run_id")
		if c != nil {
			q = q.Where("(space_id > ? OR (space_id = ? AND expt_run_id > ?))", c.WorkspaceID, c.WorkspaceID, c.RunID)
		}
	case entity.HookScanFinalize:
		// InnoDB extends this secondary index with (space_id, expt_run_id), the primary key.
		q = q.Table("expt_lifecycle_run").Select("space_id, expt_id, expt_run_id, version, next_reconcile_at AS scan_at").
			Where("finalize_state = ? AND next_reconcile_at <= ?", 1, hookScanSQLTime(in.Now)).Order("next_reconcile_at, space_id, expt_run_id")
		if c != nil {
			q = q.Where("(next_reconcile_at > ? OR (next_reconcile_at = ? AND (space_id > ? OR (space_id = ? AND expt_run_id > ?))))",
				hookScanSQLTime(c.At), hookScanSQLTime(c.At), c.WorkspaceID, c.WorkspaceID, c.RunID)
		}
	}
	return q
}

// DATETIME has no zone; preserve the caller's storage-clock wall time without driver conversion.
func hookScanSQLTime(at time.Time) string { return at.Format("2006-01-02 15:04:05.000") }
