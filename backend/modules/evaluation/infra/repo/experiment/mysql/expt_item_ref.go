// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"

	"gorm.io/gen/field"
	"gorm.io/gorm/clause"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/query"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

//go:generate  mockgen -destination=mocks/expt_item_ref.go  -package mocks . IExptItemRefDAO
type IExptItemRefDAO interface {
	BatchCreateNX(ctx context.Context, items []*model.ExptItemRef, opts ...db.Option) error
	ListByExptID(ctx context.Context, spaceID, exptID int64, cursor, limit int64, opts ...db.Option) ([]*model.ExptItemRef, int64, error)
	GetByExptIDAndItemID(ctx context.Context, spaceID, exptID, itemID int64, opts ...db.Option) (*model.ExptItemRef, error)
	MGetByExptIDAndItemIDs(ctx context.Context, spaceID, exptID int64, itemIDs []int64, opts ...db.Option) ([]*model.ExptItemRef, error)
	CountByEvalSetGrouped(ctx context.Context, spaceID int64, exptIDs []int64, opts ...db.Option) ([]*ExptItemRefEvalSetCount, error)
}

// ExptItemRefEvalSetCount per-set item 计数 (CountByEvalSetGrouped 结果)
type ExptItemRefEvalSetCount struct {
	ExptID           int64
	EvalSetID        int64
	EvalSetVersionID int64
	ItemCount        int64
}

type exptItemRefDAOImpl struct {
	provider db.Provider
}

func NewExptItemRefDAO(p db.Provider) IExptItemRefDAO {
	return &exptItemRefDAOImpl{provider: p}
}

func (dao *exptItemRefDAOImpl) BatchCreateNX(ctx context.Context, items []*model.ExptItemRef, opts ...db.Option) error {
	if len(items) == 0 {
		return nil
	}
	dbConn := dao.provider.NewSession(ctx, opts...)
	q := query.Use(dbConn).ExptItemRef
	err := q.WithContext(ctx).
		Clauses(clause.OnConflict{DoNothing: true}).
		CreateInBatches(items, 500)
	if err != nil {
		return errorx.Wrapf(err, "ExptItemRef BatchCreateNX fail, count=%d", len(items))
	}
	return nil
}

func (dao *exptItemRefDAOImpl) ListByExptID(ctx context.Context, spaceID, exptID int64, cursor, limit int64, opts ...db.Option) ([]*model.ExptItemRef, int64, error) {
	dbConn := dao.provider.NewSession(ctx, opts...)
	q := query.Use(dbConn).ExptItemRef
	do := q.WithContext(ctx).
		Where(q.SpaceID.Eq(spaceID), q.ExptID.Eq(exptID), q.DeletedAt.IsNull())
	if cursor > 0 {
		do = do.Where(q.ID.Gt(cursor))
	}
	items, err := do.Order(q.ID).Limit(int(limit)).Find()
	if err != nil {
		return nil, 0, errorx.Wrapf(err, "ExptItemRef ListByExptID fail, expt_id=%d", exptID)
	}
	var nextCursor int64
	if int64(len(items)) == limit && len(items) > 0 {
		nextCursor = items[len(items)-1].ID
	}
	return items, nextCursor, nil
}

func (dao *exptItemRefDAOImpl) GetByExptIDAndItemID(ctx context.Context, spaceID, exptID, itemID int64, opts ...db.Option) (*model.ExptItemRef, error) {
	dbConn := dao.provider.NewSession(ctx, opts...)
	q := query.Use(dbConn).ExptItemRef
	item, err := q.WithContext(ctx).
		Where(q.SpaceID.Eq(spaceID), q.ExptID.Eq(exptID), q.ItemID.Eq(itemID), q.DeletedAt.IsNull()).
		First()
	if err != nil {
		return nil, errorx.Wrapf(err, "ExptItemRef GetByExptIDAndItemID fail, expt_id=%d, item_id=%d", exptID, itemID)
	}
	return item, nil
}

func (dao *exptItemRefDAOImpl) MGetByExptIDAndItemIDs(ctx context.Context, spaceID, exptID int64, itemIDs []int64, opts ...db.Option) ([]*model.ExptItemRef, error) {
	if len(itemIDs) == 0 {
		return nil, nil
	}
	dbConn := dao.provider.NewSession(ctx, opts...)
	q := query.Use(dbConn).ExptItemRef
	items, err := q.WithContext(ctx).
		Where(q.SpaceID.Eq(spaceID), q.ExptID.Eq(exptID), q.ItemID.In(itemIDs...), q.DeletedAt.IsNull()).
		Find()
	if err != nil {
		return nil, errorx.Wrapf(err, "ExptItemRef MGetByExptIDAndItemIDs fail, expt_id=%d", exptID)
	}
	return items, nil
}

func (dao *exptItemRefDAOImpl) CountByEvalSetGrouped(ctx context.Context, spaceID int64, exptIDs []int64, opts ...db.Option) ([]*ExptItemRefEvalSetCount, error) {
	if len(exptIDs) == 0 {
		return nil, nil
	}
	dbConn := dao.provider.NewSession(ctx, opts...)
	q := query.Use(dbConn).ExptItemRef

	type countResult struct {
		ExptID           int64 `gorm:"column:expt_id"`
		EvalSetID        int64 `gorm:"column:eval_set_id"`
		EvalSetVersionID int64 `gorm:"column:eval_set_version_id"`
		ItemCount        int64 `gorm:"column:item_count"`
	}

	var results []countResult
	err := q.WithContext(ctx).
		Where(q.SpaceID.Eq(spaceID), q.ExptID.In(exptIDs...), q.DeletedAt.IsNull()).
		Group(q.ExptID, q.EvalSetID, q.EvalSetVersionID).
		Select(q.ExptID, q.EvalSetID, q.EvalSetVersionID, q.ItemID.Count().As("item_count")).
		Scan(&results)
	if err != nil {
		return nil, errorx.Wrapf(err, "ExptItemRef CountByEvalSetGrouped fail")
	}

	counts := make([]*ExptItemRefEvalSetCount, 0, len(results))
	for _, r := range results {
		counts = append(counts, &ExptItemRefEvalSetCount{
			ExptID:           r.ExptID,
			EvalSetID:        r.EvalSetID,
			EvalSetVersionID: r.EvalSetVersionID,
			ItemCount:        r.ItemCount,
		})
	}
	return counts, nil
}

// ensure field.Expr is used (suppress import error if needed)
var _ field.Expr
