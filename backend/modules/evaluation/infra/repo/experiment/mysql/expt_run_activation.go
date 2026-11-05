// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

// IExptRunActivationDAO is the storage port behind repo.IExptRunActivationRepo.
type IExptRunActivationDAO interface {
	MarkProcessingIfPending(ctx context.Context, exptID, runID, spaceID int64) (bool, error)
}

func (dao *ExptRunLogDAOImpl) MarkProcessingIfPending(ctx context.Context, exptID, runID, spaceID int64) (bool, error) {
	result := dao.provider.NewSession(ctx, db.WithMaster()).Model(&model.ExptRunLog{}).
		Where("space_id=? AND expt_id=? AND expt_run_id=? AND status=?", spaceID, exptID, runID, int64(entity.ExptStatus_Pending)).
		Update("status", int64(entity.ExptStatus_Processing))
	return result.RowsAffected > 0, result.Error
}
