// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
)

// IExptRetryRunStatusDAO is the storage port behind repo.IExptRetryRunStatusRepo.
type IExptRetryRunStatusDAO interface {
	MarkRetryProcessing(ctx context.Context, spaceID, exptID, runID int64) (bool, error)
}

func (dao *exptDAOImpl) MarkRetryProcessing(ctx context.Context, spaceID, exptID, runID int64) (processing bool, err error) {
	err = dao.db.Transaction(ctx, func(tx *gorm.DB) error {
		var expt model.Experiment
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("space_id=? AND id=?", spaceID, exptID).
			Take(&expt).Error; err != nil {
			return fmt.Errorf("retry processing experiment: %w", err)
		}
		if expt.LatestRunID != runID {
			return fmt.Errorf("retry processing experiment run replaced")
		}
		switch {
		case expt.Status == int32(entity.ExptStatus_Terminating):
			return nil
		case expt.Status == int32(entity.ExptStatus_Processing):
			processing = true
			return nil
		case expt.Status == int32(entity.ExptStatus_Pending) || entity.IsExptFinished(entity.ExptStatus(expt.Status)):
			if err := tx.Model(&model.Experiment{}).
				Where("id=? AND space_id=? AND latest_run_id=?", expt.ID, spaceID, runID).
				Update("status", int32(entity.ExptStatus_Processing)).Error; err != nil {
				return err
			}
			processing = true
		}
		return nil
	}, db.WithMaster())
	if err != nil {
		return false, err
	}
	return processing, nil
}
