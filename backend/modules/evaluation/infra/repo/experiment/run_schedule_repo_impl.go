// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

type RunScheduleRepoImpl struct {
	dao mysql.RunScheduleDAO
}

func NewRunScheduleRepo(dao mysql.RunScheduleDAO) repo.IRunScheduleRepo {
	return &RunScheduleRepoImpl{dao: dao}
}

func (r *RunScheduleRepoImpl) Upsert(ctx context.Context, schedule *entity.ExptRunSchedule) error {
	return r.dao.Upsert(ctx, scheduleToModel(schedule))
}

func (r *RunScheduleRepoImpl) Get(ctx context.Context, spaceID, exptID, runID int64) (*entity.ExptRunSchedule, error) {
	m, err := r.dao.Get(ctx, spaceID, exptID, runID, db.WithMaster())
	if err != nil {
		return nil, err
	}
	return scheduleFromModel(m), nil
}

func (r *RunScheduleRepoImpl) ScanActive(ctx context.Context, ownerScope string, limit int) ([]*entity.ExptRunSchedule, error) {
	ms, err := r.dao.ScanActive(ctx, ownerScope, limit, db.WithMaster())
	if err != nil {
		return nil, err
	}
	out := make([]*entity.ExptRunSchedule, 0, len(ms))
	for _, m := range ms {
		out = append(out, scheduleFromModel(m))
	}
	return out, nil
}

func (r *RunScheduleRepoImpl) UpdateStatus(ctx context.Context, spaceID, exptID, runID int64, status int32, ledgerGeneration int64) error {
	return r.dao.UpdateStatus(ctx, spaceID, exptID, runID, status, ledgerGeneration)
}

func scheduleToModel(s *entity.ExptRunSchedule) *model.ExptRunSchedule {
	if s == nil {
		return nil
	}
	return &model.ExptRunSchedule{
		ID:                s.ID,
		SpaceID:           s.SpaceID,
		ExptID:            s.ExptID,
		RunID:             s.RunID,
		ScheduleMode:      s.ScheduleMode,
		Priority:          s.Priority,
		OwnerScope:        s.OwnerScope,
		TargetConcurrency: s.TargetConcurrency,
		Status:            s.Status,
		AdmissionToken:    s.AdmissionToken,
		LedgerGeneration:  s.LedgerGeneration,
		CreatedAt:         s.CreatedAt,
		UpdatedAt:         s.UpdatedAt,
	}
}

func scheduleFromModel(s *model.ExptRunSchedule) *entity.ExptRunSchedule {
	if s == nil {
		return nil
	}
	return &entity.ExptRunSchedule{
		ID:                s.ID,
		SpaceID:           s.SpaceID,
		ExptID:            s.ExptID,
		RunID:             s.RunID,
		ScheduleMode:      s.ScheduleMode,
		Priority:          s.Priority,
		OwnerScope:        s.OwnerScope,
		TargetConcurrency: s.TargetConcurrency,
		Status:            s.Status,
		AdmissionToken:    s.AdmissionToken,
		LedgerGeneration:  s.LedgerGeneration,
		CreatedAt:         s.CreatedAt,
		UpdatedAt:         s.UpdatedAt,
	}
}
