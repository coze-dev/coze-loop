// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"database/sql"
	"fmt"

	"gorm.io/gen"
	"gorm.io/gorm/clause"
	"gorm.io/plugin/dbresolver"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/query"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

//go:generate  mockgen -destination=mocks/expt_item_result.go  -package mocks . IExptItemResultDAO
type IExptItemResultDAO interface {
	BatchGet(ctx context.Context, spaceID, exptID int64, itemIDs []int64, opts ...db.Option) ([]*model.ExptItemResult, error)
	BatchCreateNX(ctx context.Context, itemResults []*model.ExptItemResult, opts ...db.Option) error
	ScanItemResults(ctx context.Context, exptID, cursor, limit int64, status []int32, spaceID int64, opts ...db.Option) (results []*model.ExptItemResult, ncursor int64, err error)
	GetItemIDListByExptID(ctx context.Context, exptID, spaceID int64) (itemIDList []int64, err error)
	ListItemResultsByExptID(ctx context.Context, exptID, spaceID int64, page entity.Page, desc bool) ([]*model.ExptItemResult, int64, error)
	SaveItemResults(ctx context.Context, itemResults []*model.ExptItemResult, opts ...db.Option) error
	GetItemTurnResults(ctx context.Context, spaceID, exptID, itemID int64, opts ...db.Option) ([]*model.ExptTurnResult, error)
	UpdateItemsResult(ctx context.Context, spaceID, exptID int64, itemID []int64, ufields map[string]any, opts ...db.Option) error
	GetMaxItemIdxByExptID(ctx context.Context, exptID, spaceID int64, opts ...db.Option) (int32, error)

	BatchCreateNXRunLogs(ctx context.Context, itemRunLogs []*model.ExptItemResultRunLog, opts ...db.Option) error
	ScanItemRunLogs(ctx context.Context, exptID, exptRunID int64, filter *entity.ExptItemRunLogFilter, cursor, limit, spaceID int64, opts ...db.Option) ([]*model.ExptItemResultRunLog, int64, error)
	UpdateItemRunLog(ctx context.Context, exptID, exptRunID int64, itemID []int64, ufields map[string]any, spaceID int64, opts ...db.Option) error
	GetItemRunLog(ctx context.Context, exptID, exptRunID, itemID, spaceID int64, opts ...db.Option) (*model.ExptItemResultRunLog, error)
	MGetItemRunLog(ctx context.Context, exptID, exptRunID int64, itemIDs []int64, spaceID int64, opts ...db.Option) ([]*model.ExptItemResultRunLog, error)
}

type exptItemResultDAOImpl struct {
	provider db.Provider
}

func NewExptItemResultDAO(db db.Provider) IExptItemResultDAO {
	return &exptItemResultDAOImpl{
		provider: db,
	}
}

func (dao *exptItemResultDAOImpl) BatchGet(ctx context.Context, spaceID, exptID int64, itemIDs []int64, opts ...db.Option) ([]*model.ExptItemResult, error) {
	db := dao.provider.NewSession(ctx, opts...)
	if contexts.CtxWriteDB(ctx) {
		db = db.Clauses(dbresolver.Write)
	}
	q := query.Use(db).ExptItemResult
	finds, err := q.WithContext(ctx).Where(q.SpaceID.Eq(spaceID), q.ExptID.Eq(exptID), q.ItemID.In(itemIDs...)).Find()
	if err != nil {
		return nil, err
	}
	return finds, nil
}

func (dao *exptItemResultDAOImpl) UpdateItemsResult(ctx context.Context, spaceID, exptID int64, itemIDs []int64, ufields map[string]any, opts ...db.Option) error {
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptItemResult
	_, err := q.WithContext(ctx).
		Where(q.SpaceID.Eq(spaceID),
			q.ExptID.Eq(exptID),
			q.ItemID.In(itemIDs...)).
		Updates(ufields)
	if err != nil {
		return errorx.Wrapf(err, "UpdateItemsResult fail, expt_id: %v, item_id: %v, ufields: %v", exptID, itemIDs, ufields)
	}
	return nil
}

func (dao *exptItemResultDAOImpl) GetItemTurnResults(ctx context.Context, spaceID, exptID, itemID int64, opts ...db.Option) ([]*model.ExptTurnResult, error) {
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptTurnResult
	finds, err := q.WithContext(ctx).Where(q.SpaceID.Eq(spaceID), q.ExptID.Eq(exptID), q.ItemID.Eq(itemID)).Find()
	if err != nil {
		return nil, err
	}
	return finds, nil
}

func (dao *exptItemResultDAOImpl) SaveItemResults(ctx context.Context, itemResults []*model.ExptItemResult, opts ...db.Option) error {
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptItemResult
	if err := q.WithContext(ctx).Save(itemResults...); err != nil {
		return errorx.Wrapf(err, "SaveItemResults fail, model: %v", json.Jsonify(itemResults))
	}
	return nil
}

func (dao *exptItemResultDAOImpl) SaveItemRunLogs(ctx context.Context, itemRunLogs []*model.ExptItemResultRunLog, opts ...db.Option) error {
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptItemResultRunLog
	if err := q.WithContext(ctx).Save(itemRunLogs...); err != nil {
		return errorx.Wrapf(err, "SaveItemRunLogs fail, model: %v", json.Jsonify(itemRunLogs))
	}
	return nil
}

func (dao *exptItemResultDAOImpl) GetItemRunLog(ctx context.Context, exptID, exptRunID, itemID, spaceID int64, opts ...db.Option) (*model.ExptItemResultRunLog, error) {
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptItemResultRunLog
	found, err := q.WithContext(ctx).
		Where(q.SpaceID.Eq(spaceID),
			q.ExptID.Eq(exptID),
			q.ExptRunID.Eq(exptRunID),
			q.ItemID.Eq(itemID)).
		First()
	if err != nil {
		return nil, errorx.Wrapf(err, "GetItemRunLog fail, expt_id: %v, expt_run_id: %v, item_id: %v", exptID, exptRunID, itemID)
	}
	return found, nil
}

func (dao *exptItemResultDAOImpl) MGetItemRunLog(ctx context.Context, exptID, exptRunID int64, itemIDs []int64, spaceID int64, opts ...db.Option) ([]*model.ExptItemResultRunLog, error) {
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptItemResultRunLog
	found, err := q.WithContext(ctx).
		Where(q.SpaceID.Eq(spaceID),
			q.ExptID.Eq(exptID),
			q.ExptRunID.Eq(exptRunID),
			q.ItemID.In(itemIDs...)).
		Find()
	if err != nil {
		return nil, errorx.Wrapf(err, "GetItemRunLog fail, expt_id: %v, expt_run_id: %v, item_ids: %v", exptID, exptRunID, itemIDs)
	}
	return found, nil
}

func (dao *exptItemResultDAOImpl) UpdateItemRunLog(ctx context.Context, exptID, exptRunID int64, itemID []int64, ufields map[string]any, spaceID int64, opts ...db.Option) error {
	logs.CtxInfo(ctx, "UpdateItemRunLog, expt_id: %v, expt_run_id: %v, item_ids: %v, ufields: %v", exptID, exptRunID, itemID, ufields)
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptItemResultRunLog
	_, err := q.WithContext(ctx).
		Where(
			q.SpaceID.Eq(spaceID),
			q.ExptID.Eq(exptID),
			q.ExptRunID.Eq(exptRunID),
			q.ItemID.In(itemID...),
		).
		UpdateColumns(ufields)
	if err != nil {
		return errorx.Wrapf(err, "ExptItemResultRepo.UpdateItemRunLog failed, expt_id: %v, run_id: %v, item_id: %v, ufields: %v", exptID, exptRunID, itemID, ufields)
	}
	return nil
}

func (dao *exptItemResultDAOImpl) ScanItemResults(ctx context.Context, exptID, cursor, limit int64, status []int32, spaceID int64, opts ...db.Option) (results []*model.ExptItemResult, ncursor int64, err error) {
	if len(status) == 0 {
		return nil, 0, fmt.Errorf("ExptItemResultRepo.ScanItemResults with null status")
	}
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptItemResult
	conds := []gen.Condition{
		q.SpaceID.Eq(spaceID),
		q.ExptID.Eq(exptID),
		q.Status.In(status...),
	}

	if cursor > 0 {
		conds = append(conds, q.ID.Gt(cursor))
	}

	query := q.WithContext(ctx).
		Where(conds...).
		Order(q.ID.Asc())
	if limit > 0 {
		query = query.Limit(int(limit))
	}

	res, err := query.Find()
	if err != nil {
		return nil, 0, errorx.Wrapf(err, "ScanItemResults fail, exptID=%d, cursor=%d", exptID, cursor)
	}

	if len(res) == 0 {
		return nil, 0, nil
	}

	return res, res[len(res)-1].ID, nil
}

func (dao *exptItemResultDAOImpl) ListItemResultsByExptID(ctx context.Context, exptID, spaceID int64, page entity.Page, desc bool) ([]*model.ExptItemResult, int64, error) {
	db := dao.provider.NewSession(ctx)
	q := query.Use(db).ExptItemResult
	conds := []gen.Condition{
		q.SpaceID.Eq(spaceID),
		q.ExptID.Eq(exptID),
	}

	query := q.WithContext(ctx).
		Where(conds...).
		Order(q.ID.Asc())
	total, err := query.Count()
	if err != nil {
		return nil, 0, errorx.Wrapf(err, "ListItemResultsByExptID fail, exptID=%d, spaceID=%d, page=%v, desc=%v", exptID, spaceID, page, desc)
	}
	if desc {
		query = query.Order(q.ID.Desc())
	}
	if page.Limit() > 0 {
		query = query.Limit(page.Limit())
	}
	if page.Offset() > 0 {
		query = query.Offset(page.Offset())
	}
	res, err := query.Find()
	if err != nil {
		return nil, 0, errorx.Wrapf(err, "ListItemResultsByExptID fail, exptID=%d, spaceID=%d, page=%v, desc=%v", exptID, spaceID, page, desc)
	}
	return res, total, nil
}

func (dao *exptItemResultDAOImpl) GetItemIDListByExptID(ctx context.Context, exptID, spaceID int64) (itemIDList []int64, err error) {
	db := dao.provider.NewSession(ctx)
	q := query.Use(db).ExptItemResult

	conds := []gen.Condition{
		q.SpaceID.Eq(spaceID),
		q.ExptID.Eq(exptID),
	}

	query := q.WithContext(ctx).
		Select(q.ItemID).
		Where(conds...).
		Group(q.ItemID)
	query = query.Limit(consts.MaxEvalSetItemLimit)

	res, err := query.Find()
	if err != nil {
		return nil, errorx.Wrapf(err, "ScanItemResults fail, exptID=%d", exptID)
	}

	if len(res) == 0 {
		return nil, nil
	}

	for _, itemResult := range res {
		itemIDList = append(itemIDList, itemResult.ItemID)
	}
	return itemIDList, nil
}

func (dao *exptItemResultDAOImpl) ScanItemRunLogs(ctx context.Context, exptID, exptRunID int64, filter *entity.ExptItemRunLogFilter, cursor, limit, spaceID int64, opts ...db.Option) ([]*model.ExptItemResultRunLog, int64, error) {
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptItemResultRunLog
	conds := []gen.Condition{
		q.SpaceID.Eq(spaceID),
		q.ExptID.Eq(exptID),
		q.ExptRunID.Eq(exptRunID),
	}

	if filter.ResultState != nil {
		conds = append(conds, q.ResultState.In(int32(filter.GetResultState())))
	}
	if len(filter.Status) > 0 {
		conds = append(conds, q.Status.In(filter.GetStatus()...))
	}

	if cursor > 0 {
		conds = append(conds, q.ID.Gt(cursor))
	}

	query := q.WithContext(ctx).
		Where(conds...).
		Order(q.ID.Asc())
	if limit > 0 {
		query = query.Limit(int(limit))
	}

	res, err := query.Find()
	if err != nil {
		return nil, 0, errorx.Wrapf(err, "ScanItemRunLogs fail, exptID=%d, exptRunID=%d, cursor=%d", exptID, exptRunID, cursor)
	}

	if len(res) == 0 {
		return nil, 0, nil
	}

	return res, res[len(res)-1].ID, nil
}

func (dao *exptItemResultDAOImpl) BatchCreateNX(ctx context.Context, itemResults []*model.ExptItemResult, opts ...db.Option) error {
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptItemResult
	if err := q.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(itemResults, 50); err != nil {
		return errorx.Wrapf(err, "ExptItemResult.BatchCreateNX fail, cnt: %v", len(itemResults))
	}
	return nil
}

func (dao *exptItemResultDAOImpl) BatchCreateNXRunLogs(ctx context.Context, itemResults []*model.ExptItemResultRunLog, opts ...db.Option) error {
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptItemResultRunLog
	if err := q.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).CreateInBatches(itemResults, 50); err != nil {
		return errorx.Wrapf(err, "ExptItemResult.BatchCreateNXRunLogs fail, cnt: %v", len(itemResults))
	}
	return nil
}

func (dao *exptItemResultDAOImpl) GetMaxItemIdxByExptID(ctx context.Context, exptID, spaceID int64, opts ...db.Option) (int32, error) {
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptItemResult

	// 使用结构体接收聚合结果
	var result struct {
		MaxItemIdx sql.NullInt32 `gorm:"column:max_item_idx"`
	}

	err := q.WithContext(ctx).
		Select(q.ItemIdx.Max().As("max_item_idx")). // 明确指定别名
		Where(q.SpaceID.Eq(spaceID), q.ExptID.Eq(exptID)).
		Scan(&result) // 使用 Scan 代替 Row().Scan()
	if err != nil {
		return 0, errorx.Wrapf(err, "GetMaxItemIdxByExptID fail, expt_id: %v", exptID)
	}
	if !result.MaxItemIdx.Valid {
		return 0, nil // 无记录
	}
	return result.MaxItemIdx.Int32, nil
}
