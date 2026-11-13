// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"

	"gorm.io/gorm/clause"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

//go:generate mockgen -destination=mocks/central_schedule.go -package=mocks . RunScheduleDAO
type RunScheduleDAO interface {
	Upsert(ctx context.Context, schedule *model.ExptRunSchedule, opts ...db.Option) error
	Get(ctx context.Context, spaceID, exptID, runID int64, opts ...db.Option) (*model.ExptRunSchedule, error)
	ScanActive(ctx context.Context, ownerScope string, limit int, opts ...db.Option) ([]*model.ExptRunSchedule, error)
	UpdateStatus(ctx context.Context, spaceID, exptID, runID int64, status int32, ledgerGeneration int64, opts ...db.Option) error
}

func NewRunScheduleDAO(db db.Provider) RunScheduleDAO {
	return &runScheduleDAO{db: db}
}

type runScheduleDAO struct {
	db db.Provider
}

func (r *runScheduleDAO) Upsert(ctx context.Context, schedule *model.ExptRunSchedule, opts ...db.Option) error {
	err := r.db.NewSession(ctx, opts...).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{
				{Name: "space_id"}, {Name: "expt_id"}, {Name: "run_id"},
			},
			DoUpdates: clause.AssignmentColumns([]string{
				"schedule_mode", "priority", "owner_scope", "target_concurrency",
				"status", "admission_token", "ledger_generation", "updated_at",
			}),
		}).
		Create(schedule).Error
	if err != nil {
		return errorx.Wrapf(err, "upsert expt_run_schedule fail, model: %+v", schedule)
	}
	return nil
}

func (r *runScheduleDAO) Get(ctx context.Context, spaceID, exptID, runID int64, opts ...db.Option) (*model.ExptRunSchedule, error) {
	var schedule model.ExptRunSchedule
	err := r.db.NewSession(ctx, opts...).
		Where("space_id = ? AND expt_id = ? AND run_id = ?", spaceID, exptID, runID).
		First(&schedule).Error
	if err != nil {
		return nil, errorx.Wrapf(err, "get expt_run_schedule fail, spaceID=%d exptID=%d runID=%d", spaceID, exptID, runID)
	}
	return &schedule, nil
}

// ScanActive 全局扫描：owner_scope 必须精确匹配，绝不退化成公共池。
// 排序与调度侧一致：priority 降序，同优先级按 created_at 等待时长升序。
func (r *runScheduleDAO) ScanActive(ctx context.Context, ownerScope string, limit int, opts ...db.Option) ([]*model.ExptRunSchedule, error) {
	if ownerScope == "" {
		return nil, errorx.New("scan active expt_run_schedule rejected: empty owner_scope")
	}
	var schedules []*model.ExptRunSchedule
	err := r.db.NewSession(ctx, opts...).
		Where("owner_scope = ?", ownerScope).
		Where("status IN ?", entity.ActiveStatuses).
		Order("priority DESC, created_at ASC").
		Limit(limit).
		Find(&schedules).Error
	if err != nil {
		return nil, errorx.Wrapf(err, "scan active expt_run_schedule fail, ownerScope=%s", ownerScope)
	}
	return schedules, nil
}

func (r *runScheduleDAO) UpdateStatus(ctx context.Context, spaceID, exptID, runID int64, status int32, ledgerGeneration int64, opts ...db.Option) error {
	err := r.db.NewSession(ctx, opts...).
		Model(&model.ExptRunSchedule{}).
		Where("space_id = ? AND expt_id = ? AND run_id = ?", spaceID, exptID, runID).
		Updates(map[string]any{
			"status":            status,
			"ledger_generation": ledgerGeneration,
		}).Error
	if err != nil {
		return errorx.Wrapf(err, "update expt_run_schedule status fail, spaceID=%d exptID=%d runID=%d", spaceID, exptID, runID)
	}
	return nil
}

