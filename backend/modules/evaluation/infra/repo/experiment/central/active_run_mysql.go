// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package central

import (
	"context"
	"strconv"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

// NewCentralActiveRunRepo reads the existing expt_run_log / experiment /
// expt_item_result_run_log tables. It never writes and never alters schema.
func NewCentralActiveRunRepo(provider db.Provider) repo.CentralActiveRunRepo {
	return &centralActiveRunRepo{provider: provider}
}

type centralActiveRunRepo struct {
	provider db.Provider
}

type activeRunRow struct {
	ExptID      int64     `gorm:"column:expt_id"`
	SpaceID     int64     `gorm:"column:space_id"`
	RunID       int64     `gorm:"column:expt_run_id"`
	Mode        int32     `gorm:"column:mode"`
	Status      int64     `gorm:"column:status"`
	CreatedBy   string    `gorm:"column:created_by"`
	CreatedAt   time.Time `gorm:"column:run_created_at"`
	ExptType    int64     `gorm:"column:expt_type"`
	SourceType  int64     `gorm:"column:source_type"`
	TriggerType string    `gorm:"column:trigger_type"`
	ExptCreated time.Time `gorm:"column:expt_created_at"`
}

func (r *centralActiveRunRepo) ListRecentRuns(ctx context.Context, since time.Time, limit int) ([]*entity.CentralActiveRun, error) {
	if r.provider == nil {
		return nil, errorx.New("central active run repo: nil db provider")
	}
	if limit <= 0 {
		limit = entity.CentralDefaultScanLimit
	}
	// A recent-run window (rather than only active statuses) is intentional:
	// finished runs must still be visible once so their reservations can be
	// released even if the terminal event was lost.
	const query = `
SELECT r.expt_id, r.space_id, r.expt_run_id, COALESCE(r.mode, 0) AS mode,
       COALESCE(r.status, 0) AS status, r.created_by,
       r.created_at AS run_created_at,
       e.expt_type, e.source_type, e.trigger_type,
       e.created_at AS expt_created_at
FROM expt_run_log r
INNER JOIN experiment e ON e.id = r.expt_id AND e.space_id = r.space_id
WHERE r.deleted_at IS NULL
  AND e.deleted_at IS NULL
  AND r.updated_at >= ?
ORDER BY r.updated_at DESC
LIMIT ?`
	var rows []activeRunRow
	if err := r.provider.NewSession(ctx).WithContext(ctx).Raw(query, since, limit).Scan(&rows).Error; err != nil {
		return nil, errorx.Wrapf(err, "central active run: list recent runs")
	}
	out := make([]*entity.CentralActiveRun, 0, len(rows))
	for _, row := range rows {
		out = append(out, &entity.CentralActiveRun{
			ExptID:        row.ExptID,
			SpaceID:       row.SpaceID,
			RunID:         row.RunID,
			RunMode:       entity.ExptRunMode(row.Mode),
			RunStatus:     entity.ExptStatus(row.Status),
			CreatedBy:     row.CreatedBy,
			RunCreatedAt:  row.CreatedAt,
			ExptCreatedAt: row.ExptCreated,
			ExptType:      entity.ExptType(row.ExptType),
			SourceType:    row.SourceType,
			TriggerType:   row.TriggerType,
		})
	}
	return out, nil
}

type snapshotRow struct {
	ExptID         int64      `gorm:"column:expt_id"`
	RunID          int64      `gorm:"column:expt_run_id"`
	InFlight       int        `gorm:"column:in_flight"`
	Queueing       int        `gorm:"column:queueing"`
	Terminal       int        `gorm:"column:terminal"`
	OldestQueuedAt *time.Time `gorm:"column:oldest_queued_at"`
}

func (r *centralActiveRunRepo) Snapshots(ctx context.Context, runs []*entity.CentralActiveRun) (map[string]*entity.CentralRunSnapshot, error) {
	out := make(map[string]*entity.CentralRunSnapshot, len(runs))
	if len(runs) == 0 {
		return out, nil
	}
	if r.provider == nil {
		return nil, errorx.New("central active run repo: nil db provider")
	}
	exptIDs := make([]int64, 0, len(runs))
	seen := map[int64]struct{}{}
	for _, run := range runs {
		if run == nil {
			continue
		}
		if _, ok := seen[run.ExptID]; ok {
			continue
		}
		seen[run.ExptID] = struct{}{}
		exptIDs = append(exptIDs, run.ExptID)
	}
	const query = `
SELECT expt_id, expt_run_id,
       SUM(CASE WHEN status = 1 THEN 1 ELSE 0 END) AS in_flight,
       SUM(CASE WHEN status = 0 THEN 1 ELSE 0 END) AS queueing,
       SUM(CASE WHEN status IN (2,3,5) THEN 1 ELSE 0 END) AS terminal,
       MIN(CASE WHEN status = 0 THEN updated_at END) AS oldest_queued_at
FROM expt_item_result_run_log
WHERE deleted_at IS NULL AND expt_id IN (?)
GROUP BY expt_id, expt_run_id`
	var rows []snapshotRow
	if err := r.provider.NewSession(ctx).WithContext(ctx).Raw(query, exptIDs).Scan(&rows).Error; err != nil {
		return nil, errorx.Wrapf(err, "central active run: snapshots")
	}
	wanted := make(map[string]struct{}, len(runs))
	for _, run := range runs {
		if run == nil {
			continue
		}
		wanted[runKey(run.ExptID, run.RunID)] = struct{}{}
	}
	for _, row := range rows {
		key := runKey(row.ExptID, row.RunID)
		if _, ok := wanted[key]; !ok {
			continue
		}
		out[key] = &entity.CentralRunSnapshot{
			InFlight:       row.InFlight,
			Queueing:       row.Queueing,
			Terminal:       row.Terminal,
			OldestQueuedAt: row.OldestQueuedAt,
		}
	}
	return out, nil
}

func runKey(exptID, runID int64) string {
	return strconv.FormatInt(exptID, 10) + ":" + strconv.FormatInt(runID, 10)
}

type experimentRow struct {
	ExptID      int64  `gorm:"column:id"`
	SpaceID     int64  `gorm:"column:space_id"`
	ExptType    int64  `gorm:"column:expt_type"`
	SourceType  int64  `gorm:"column:source_type"`
	TriggerType string `gorm:"column:trigger_type"`
	TargetID    int64  `gorm:"column:target_id"`
	TargetType  int64  `gorm:"column:target_type"`
	EvalConf    []byte `gorm:"column:eval_conf"`
}

type evalConfProjection struct {
	ItemConcurNum      *int              `json:"ItemConcurNum"`
	ItemConcurNumSnake *int              `json:"item_concur_num"`
	Ext                map[string]string `json:"Ext"`
	ExtSnake           map[string]string `json:"ext"`
}

type evaluatorRefRow struct {
	EvaluatorVersionID int64 `gorm:"column:evaluator_version_id"`
}

func (r *centralActiveRunRepo) LoadExperiment(ctx context.Context, exptID, spaceID int64) (*entity.CentralExperimentScheduling, error) {
	if r.provider == nil {
		return nil, errorx.New("central active run repo: nil db provider")
	}
	const exptQuery = `
SELECT id, space_id, expt_type, source_type, trigger_type, target_id, target_type, eval_conf
FROM experiment
WHERE id = ? AND space_id = ? AND deleted_at IS NULL
LIMIT 1`
	var expt experimentRow
	if err := r.provider.NewSession(ctx).WithContext(ctx).Raw(exptQuery, exptID, spaceID).Scan(&expt).Error; err != nil {
		return nil, errorx.Wrapf(err, "central active run: load experiment")
	}
	itemConcurNum := 0
	var ext map[string]string
	if len(expt.EvalConf) > 0 {
		var conf evalConfProjection
		if err := json.Unmarshal(expt.EvalConf, &conf); err == nil {
			if conf.ItemConcurNum != nil {
				itemConcurNum = *conf.ItemConcurNum
			} else if conf.ItemConcurNumSnake != nil {
				itemConcurNum = *conf.ItemConcurNumSnake
			}
			if len(conf.Ext) > 0 {
				ext = conf.Ext
			} else if len(conf.ExtSnake) > 0 {
				ext = conf.ExtSnake
			}
		}
	}
	const refQuery = `
SELECT evaluator_version_id
FROM expt_evaluator_ref
WHERE expt_id = ? AND space_id = ? AND deleted_at IS NULL
ORDER BY id ASC`
	var refs []evaluatorRefRow
	if err := r.provider.NewSession(ctx).WithContext(ctx).Raw(refQuery, exptID, spaceID).Scan(&refs).Error; err != nil {
		return nil, errorx.Wrapf(err, "central active run: load evaluator refs")
	}
	ids := make([]int64, 0, len(refs))
	for _, ref := range refs {
		if ref.EvaluatorVersionID > 0 {
			ids = append(ids, ref.EvaluatorVersionID)
		}
	}
	return &entity.CentralExperimentScheduling{
		ExptID:              expt.ExptID,
		SpaceID:             expt.SpaceID,
		ExptType:            entity.ExptType(expt.ExptType),
		SourceType:          expt.SourceType,
		TriggerType:         expt.TriggerType,
		TargetID:            expt.TargetID,
		TargetType:          entity.EvalTargetType(expt.TargetType),
		ItemConcurNum:       itemConcurNum,
		EvaluatorVersionIDs: ids,
		Ext:                 ext,
	}, nil
}

func (r *centralActiveRunRepo) QueueingItemIDs(ctx context.Context, run *entity.CentralActiveRun, limit int) ([]int64, error) {
	return r.itemIDs(ctx, run, int32(entity.ItemRunState_Queueing), limit)
}

func (r *centralActiveRunRepo) ProcessingItemIDs(ctx context.Context, run *entity.CentralActiveRun, limit int) ([]int64, error) {
	return r.itemIDs(ctx, run, int32(entity.ItemRunState_Processing), limit)
}

func (r *centralActiveRunRepo) itemIDs(ctx context.Context, run *entity.CentralActiveRun, status int32, limit int) ([]int64, error) {
	if r.provider == nil {
		return nil, errorx.New("central active run repo: nil db provider")
	}
	if run == nil {
		return nil, errorx.New("central active run repo: nil run")
	}
	if limit <= 0 {
		limit = entity.CentralDefaultScanLimit
	}
	const query = `
SELECT item_id
FROM expt_item_result_run_log
WHERE expt_id = ? AND expt_run_id = ? AND space_id = ? AND deleted_at IS NULL AND status = ?
ORDER BY item_id ASC
LIMIT ?`
	var rows []struct {
		ItemID int64 `gorm:"column:item_id"`
	}
	if err := r.provider.NewSession(ctx).WithContext(ctx).Raw(query, run.ExptID, run.RunID, run.SpaceID, status, limit).Scan(&rows).Error; err != nil {
		return nil, errorx.Wrapf(err, "central active run: item ids status=%d", status)
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ItemID)
	}
	return ids, nil
}

func (r *centralActiveRunRepo) ItemStates(ctx context.Context, run *entity.CentralActiveRun, itemIDs []int64) (map[int64]entity.ItemRunState, error) {
	if r.provider == nil {
		return nil, errorx.New("central active run repo: nil db provider")
	}
	if run == nil || len(itemIDs) == 0 {
		return map[int64]entity.ItemRunState{}, nil
	}
	const query = `
SELECT item_id, status
FROM expt_item_result_run_log
WHERE expt_id = ? AND expt_run_id = ? AND space_id = ? AND deleted_at IS NULL AND item_id IN (?)`
	var rows []struct {
		ItemID int64 `gorm:"column:item_id"`
		Status int32 `gorm:"column:status"`
	}
	if err := r.provider.NewSession(ctx).WithContext(ctx).Raw(query, run.ExptID, run.RunID, run.SpaceID, itemIDs).Scan(&rows).Error; err != nil {
		return nil, errorx.Wrapf(err, "central active run: item states")
	}
	out := make(map[int64]entity.ItemRunState, len(rows))
	for _, row := range rows {
		out[row.ItemID] = entity.ItemRunState(row.Status)
	}
	return out, nil
}
