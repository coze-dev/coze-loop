// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package target

import (
	"context"
	"errors"
	"github.com/bytedance/gg/gptr"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	exptmodel "github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/convertor"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/target/mysql/gorm_gen/model"
	"gorm.io/gorm"
	"gorm.io/plugin/dbresolver"
)

func (e *EvalTargetRepoImpl) LatestSandboxInvocation(ctx context.Context, spaceID, runID, itemID, turnID int64) (*entity.EvalTargetRecord, error) {
	var row model.TargetRecord
	err := e.dbProvider.NewSession(ctx).Clauses(dbresolver.Write).Where("space_id = ? AND experiment_run_id = ? AND item_id = ? AND turn_id = ? AND deleted_at IS NULL", spaceID, runID, itemID, turnID).Order("id DESC").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return convertor.EvalTargetRecordPO2DO(&row)
}

func (e *EvalTargetRepoImpl) TransitionSandboxInvocation(ctx context.Context, record *entity.EvalTargetRecord, fromPhase string) (bool, error) {
	row, err := convertor.EvalTargetRecordDO2PO(record)
	if err != nil {
		return false, err
	}
	updates := map[string]any{"ext": row.Ext, "status": row.Status, "trace_id": row.TraceID}
	if record.Status != nil && *record.Status == entity.EvalTargetRunStatusFail {
		updates["output_data"] = row.OutputData
	}
	result := e.dbProvider.NewSession(ctx).Model(&model.TargetRecord{}).Where("id = ? AND space_id = ? AND status = ? AND JSON_UNQUOTE(JSON_EXTRACT(ext, '$.sandbox_lifecycle_phase')) = ?", record.ID, record.SpaceID, entity.EvalTargetRunStatusAsyncInvoking, fromPhase).Updates(updates)
	return result.RowsAffected == 1, result.Error
}

func (e *EvalTargetRepoImpl) SandboxInvocationOwnerActive(ctx context.Context, spaceID, exptID, runID, itemID int64) (bool, error) {
	var run exptmodel.ExptRunLog
	db := e.dbProvider.NewSession(ctx).Clauses(dbresolver.Write)
	if err := db.Session(&gorm.Session{}).Where("expt_run_id = ? AND expt_id = ? AND space_id = ?", runID, exptID, spaceID).First(&run).Error; err != nil {
		return false, err
	}
	if run.Status == nil {
		return false, nil
	}
	if entity.IsExptFinished(entity.ExptStatus(gptr.Indirect(run.Status))) || entity.IsExptFinishing(entity.ExptStatus(gptr.Indirect(run.Status))) {
		return false, nil
	}
	var item exptmodel.ExptItemResultRunLog
	if err := db.Session(&gorm.Session{}).Where("space_id = ? AND expt_id = ? AND expt_run_id = ? AND item_id = ?", spaceID, exptID, runID, itemID).First(&item).Error; err != nil {
		return false, err
	}
	return !entity.IsItemRunFinished(entity.ItemRunState(item.Status)), nil
}
