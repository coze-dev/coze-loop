// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"
	"reflect"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
)

type hookItemSourceRepo struct{ provider db.Provider }

func NewHookItemSourceRepo(provider db.Provider) repo.IHookItemSourceRepo {
	return &hookItemSourceRepo{provider: provider}
}

const hookItemSourceColumns = "l.id,l.space_id,l.expt_id,l.expt_run_id,l.status,l.lifecycle_hook_version,l.deleted_at," +
	"e.id AS experiment_id,e.space_id AS experiment_space_id,e.latest_run_id,e.deleted_at AS experiment_deleted_at"

func (r *hookItemSourceRepo) ReadItemSource(ctx context.Context, key entity.HookRunKey) (*entity.HookRunInitialization, error) {
	if r == nil || ctx == nil || ctx.Err() != nil || r.provider == nil || (entity.HookStoreGuard{Key: key}).Validate() != nil {
		return nil, entity.ErrHookGateUnavailable
	}
	if provider := reflect.ValueOf(r.provider); provider.Kind() == reflect.Ptr && provider.IsNil() {
		return nil, entity.ErrHookGateUnavailable
	}
	var rows []hookGateLogRow
	// One primary consistent read observes the tuple together without blocking writers.
	err := r.provider.NewSession(ctx, db.WithMaster()).Session(&gorm.Session{Logger: logger.Discard}).
		Unscoped().Table("expt_run_log AS l").Select(hookItemSourceColumns).
		Joins("LEFT JOIN experiment AS e ON e.id=l.expt_id").Where("l.id=?", key.RunID).Find(&rows).Error
	if err != nil || ctx.Err() != nil || len(rows) != 1 {
		return nil, entity.ErrHookGateUnavailable
	}
	row := rows[0]
	if row.ID != key.RunID || row.ExptRunID != key.RunID || row.ExptID != key.ExperimentID || row.SpaceID != key.WorkspaceID ||
		row.ExperimentID == nil || *row.ExperimentID != key.ExperimentID || row.ExperimentSpaceID != key.WorkspaceID ||
		row.DeletedAt.Valid || row.Status == nil {
		return nil, entity.ErrHookGateUnavailable
	}
	if row.LifecycleHookVersion != nil && *row.LifecycleHookVersion != 0 && *row.LifecycleHookVersion != 1 {
		return nil, entity.ErrHookGateUnavailable
	}
	managed := row.LifecycleHookVersion != nil && *row.LifecycleHookVersion == 1
	if row.ExperimentDeletedAt.Valid && (!managed || !entity.IsExptFinished(entity.ExptStatus(*row.Status))) {
		return nil, entity.ErrHookGateUnavailable
	}
	decision := entity.CheckHookAdmission(entity.HookAdmissionInput{
		Requested: key, Actual: key, LatestRunID: row.LatestRunID, Status: entity.ExptStatus(*row.Status), Marker: entity.HookMarkerLegacy,
	})
	if decision.Gate == entity.HookGateWaiting {
		return nil, entity.ErrHookGateUnavailable
	}
	return &entity.HookRunInitialization{
		LatestRunID: row.LatestRunID,
		Managed:     managed,
		RunLog:      &entity.ExptRunLog{ID: row.ID, SpaceID: row.SpaceID, ExptID: row.ExptID, ExptRunID: row.ExptRunID, Status: *row.Status},
	}, nil
}
