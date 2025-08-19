// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package experiment

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/convert"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

func NewExptItemResultRepo(exptItemResultDAO mysql.IExptItemResultDAO) repo.IExptItemResultRepo {
	return &ExptItemResultRepoImpl{
		exptItemResultDAO: exptItemResultDAO,
	}
}

type ExptItemResultRepoImpl struct {
	exptItemResultDAO mysql.IExptItemResultDAO
}

func (e ExptItemResultRepoImpl) BatchGet(ctx context.Context, spaceID, exptID int64, itemIDs []int64) ([]*entity.ExptItemResult, error) {
	pos, err := e.exptItemResultDAO.BatchGet(ctx, spaceID, exptID, itemIDs)
	if err != nil {
		return nil, err
	}

	results := make([]*entity.ExptItemResult, 0)
	for _, po := range pos {
		results = append(results, convert.NewExptItemResultConvertor().PO2DO(po))
	}
	return results, nil
}

func (e ExptItemResultRepoImpl) UpdateItemsResult(ctx context.Context, spaceID, exptID int64, itemIDs []int64, ufields map[string]any) error {
	err := e.exptItemResultDAO.UpdateItemsResult(ctx, spaceID, exptID, itemIDs, ufields)
	if err != nil {
		return errorx.Wrapf(err, "UpdateItemsResult fail, expt_id: %v, item_id: %v, ufields: %v", exptID, itemIDs, ufields)
	}
	return nil
}

func (e ExptItemResultRepoImpl) GetItemTurnResults(ctx context.Context, spaceID, exptID, itemID int64) ([]*entity.ExptTurnResult, error) {
	pos, err := e.exptItemResultDAO.GetItemTurnResults(ctx, spaceID, exptID, itemID)
	if err != nil {
		return nil, err
	}
	results := make([]*entity.ExptTurnResult, 0)
	for _, po := range pos {
		results = append(results, convert.NewExptTurnResultConvertor().PO2DO(po, nil))
	}
	return results, nil
}

func (e ExptItemResultRepoImpl) SaveItemResults(ctx context.Context, itemResults []*entity.ExptItemResult) error {
	pos := make([]*model.ExptItemResult, 0)
	for _, itemResult := range itemResults {
		pos = append(pos, convert.NewExptItemResultConvertor().DO2PO(itemResult))
	}
	err := e.exptItemResultDAO.SaveItemResults(ctx, pos)
	if err != nil {
		return errorx.Wrapf(err, "SaveItemResults fail, model: %v", json.Jsonify(itemResults))
	}
	return nil
}

func (e ExptItemResultRepoImpl) GetItemRunLog(ctx context.Context, exptID, exptRunID, itemID, spaceID int64) (*entity.ExptItemResultRunLog, error) {
	po, err := e.exptItemResultDAO.GetItemRunLog(ctx, exptID, exptRunID, itemID, spaceID)
	if err != nil {
		return nil, errorx.Wrapf(err, "GetItemRunLog fail, expt_id: %v, expt_run_id: %v, item_id: %v", exptID, exptRunID, itemID)
	}
	return convert.NewExptItemResultRunLogConverter().PO2DO(po), nil
}

func (e ExptItemResultRepoImpl) MGetItemRunLog(ctx context.Context, exptID, exptRunID int64, itemIDs []int64, spaceID int64) ([]*entity.ExptItemResultRunLog, error) {
	pos, err := e.exptItemResultDAO.MGetItemRunLog(ctx, exptID, exptRunID, itemIDs, spaceID)
	if err != nil {
		return nil, errorx.Wrapf(err, "GetItemRunLog fail, expt_id: %v, expt_run_id: %v, item_ids: %v", exptID, exptRunID, itemIDs)
	}
	dos := make([]*entity.ExptItemResultRunLog, 0)
	for _, exptTurnResultRunLogPO := range pos {
		exptTurnResultRunLog := convert.NewExptItemResultRunLogConverter().PO2DO(exptTurnResultRunLogPO)
		dos = append(dos, exptTurnResultRunLog)
	}
	return dos, nil
}

func (e ExptItemResultRepoImpl) UpdateItemRunLog(ctx context.Context, exptID, exptRunID int64, itemID []int64, ufields map[string]any, spaceID int64) error {
	logs.CtxInfo(ctx, "UpdateItemRunLog, expt_id: %v, expt_run_id: %v, item_ids: %v, ufields: %v", exptID, exptRunID, itemID, ufields)
	err := e.exptItemResultDAO.UpdateItemRunLog(ctx, exptID, exptRunID, itemID, ufields, spaceID)
	if err != nil {
		return errorx.Wrapf(err, "UpdateItemRunLog fail, expt_id: %v, expt_run_id: %v, item_ids: %v, ufields: %v", exptID, exptRunID, itemID, ufields)
	}
	return nil
}

func (e ExptItemResultRepoImpl) ScanItemResults(ctx context.Context, exptID, cursor, limit int64, status []int32, spaceID int64) (results []*entity.ExptItemResult, ncursor int64, err error) {
	pos, ncursor, err := e.exptItemResultDAO.ScanItemResults(ctx, exptID, cursor, limit, status, spaceID)
	if err != nil {
		return nil, 0, errorx.Wrapf(err, "ScanItemResults fail, exptID=%d, cursor=%d", exptID, cursor)
	}
	for _, po := range pos {
		results = append(results, convert.NewExptItemResultConvertor().PO2DO(po))
	}
	return results, ncursor, nil
}

func (e ExptItemResultRepoImpl) GetItemIDListByExptID(ctx context.Context, exptID, spaceID int64) (itemIDList []int64, err error) {
	return e.exptItemResultDAO.GetItemIDListByExptID(ctx, exptID, spaceID)
}

func (e ExptItemResultRepoImpl) ListItemResultsByExptID(ctx context.Context, exptID, spaceID int64, page entity.Page, desc bool) ([]*entity.ExptItemResult, int64, error) {
	pos, total, err := e.exptItemResultDAO.ListItemResultsByExptID(ctx, exptID, spaceID, page, desc)
	if err != nil {
		return nil, 0, errorx.Wrapf(err, "ListItemResultsByExptID fail, exptID=%d, spaceID=%d, page=%v, desc=%v", exptID, spaceID, page, desc)
	}
	results := make([]*entity.ExptItemResult, 0)
	for _, po := range pos {
		results = append(results, convert.NewExptItemResultConvertor().PO2DO(po))
	}
	return results, total, nil
}

func (e ExptItemResultRepoImpl) ScanItemRunLogs(ctx context.Context, exptID, exptRunID int64, filter *entity.ExptItemRunLogFilter, cursor, limit, spaceID int64) ([]*entity.ExptItemResultRunLog, int64, error) {
	pos, ncursor, err := e.exptItemResultDAO.ScanItemRunLogs(ctx, exptID, exptRunID, filter, cursor, limit, spaceID)
	if err != nil {
		return nil, 0, errorx.Wrapf(err, "ScanItemRunLogs fail, exptID=%d, exptRunID=%d, cursor=%d", exptID, exptRunID, cursor)
	}
	dos := make([]*entity.ExptItemResultRunLog, 0)
	for _, exptTurnResultRunLogPO := range pos {
		exptTurnResultRunLog := convert.NewExptItemResultRunLogConverter().PO2DO(exptTurnResultRunLogPO)
		dos = append(dos, exptTurnResultRunLog)
	}
	return dos, ncursor, nil
}

func (e ExptItemResultRepoImpl) BatchCreateNX(ctx context.Context, itemResults []*entity.ExptItemResult) error {
	pos := make([]*model.ExptItemResult, 0)
	for _, itemResult := range itemResults {
		pos = append(pos, convert.NewExptItemResultConvertor().DO2PO(itemResult))
	}
	err := e.exptItemResultDAO.BatchCreateNX(ctx, pos)
	if err != nil {
		return errorx.Wrapf(err, "BatchCreateNX fail, cnt: %v", len(itemResults))
	}
	return nil
}

func (e ExptItemResultRepoImpl) BatchCreateNXRunLogs(ctx context.Context, itemResults []*entity.ExptItemResultRunLog) error {
	pos := make([]*model.ExptItemResultRunLog, 0)
	for _, itemResult := range itemResults {
		pos = append(pos, convert.NewExptItemResultRunLogConverter().DO2PO(itemResult))
	}
	err := e.exptItemResultDAO.BatchCreateNXRunLogs(ctx, pos)
	if err != nil {
		return errorx.Wrapf(err, "BatchCreateNXRunLogs fail, cnt: %v", len(itemResults))
	}
	return nil
}

func (e ExptItemResultRepoImpl) GetMaxItemIdxByExptID(ctx context.Context, exptID, spaceID int64) (int32, error) {
	return e.exptItemResultDAO.GetMaxItemIdxByExptID(ctx, exptID, spaceID)
}
