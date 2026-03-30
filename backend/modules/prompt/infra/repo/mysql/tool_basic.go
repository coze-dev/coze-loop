// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gen/field"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/query"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

type IToolBasicDAO interface {
	Create(ctx context.Context, basicPO *model.ToolBasic, opts ...db.Option) error
	Get(ctx context.Context, toolID int64, opts ...db.Option) (*model.ToolBasic, error)
	MGet(ctx context.Context, toolIDs []int64, opts ...db.Option) (map[int64]*model.ToolBasic, error)
	List(ctx context.Context, param ListToolBasicParam, opts ...db.Option) ([]*model.ToolBasic, int64, error)
	Update(ctx context.Context, toolID int64, updateFields map[string]interface{}, opts ...db.Option) error
	Delete(ctx context.Context, toolID int64, spaceID int64, opts ...db.Option) error
}

type ListToolBasicParam struct {
	SpaceID int64

	KeyWord       string
	CreatedBys    []string
	CommittedOnly bool

	Offset  int
	Limit   int
	OrderBy int
	Asc     bool
}

const (
	ListToolBasicOrderByID        = 1
	ListToolBasicOrderByCreatedAt = 2
)

func NewToolBasicDAO(db db.Provider) IToolBasicDAO {
	return &ToolBasicDAOImpl{db: db}
}

type ToolBasicDAOImpl struct {
	db db.Provider
}

func (d *ToolBasicDAOImpl) Create(ctx context.Context, basicPO *model.ToolBasic, opts ...db.Option) error {
	if basicPO == nil {
		return errorx.New("basicPO is empty")
	}
	q := query.Use(d.db.NewSession(ctx, opts...)).WithContext(ctx)
	basicPO.CreatedAt = time.Time{}
	basicPO.UpdatedAt = time.Time{}
	err := q.ToolBasic.Create(basicPO)
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return errorx.WrapByCode(err, prompterr.CommonResourceDuplicatedCode)
		}
		return errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	return nil
}

func (d *ToolBasicDAOImpl) Get(ctx context.Context, toolID int64, opts ...db.Option) (*model.ToolBasic, error) {
	if toolID <= 0 {
		return nil, errorx.New("toolID is invalid, toolID = %d", toolID)
	}
	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).ToolBasic
	basicPO, err := tx.Where(q.ToolBasic.ID.Eq(toolID)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	return basicPO, nil
}

func (d *ToolBasicDAOImpl) MGet(ctx context.Context, toolIDs []int64, opts ...db.Option) (map[int64]*model.ToolBasic, error) {
	if len(toolIDs) == 0 {
		return nil, nil
	}
	q := query.Use(d.db.NewSession(ctx, opts...))
	basicPOs, err := q.WithContext(ctx).ToolBasic.Where(q.ToolBasic.ID.In(toolIDs...)).Find()
	if err != nil {
		return nil, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	result := make(map[int64]*model.ToolBasic, len(basicPOs))
	for _, po := range basicPOs {
		result[po.ID] = po
	}
	return result, nil
}

func (d *ToolBasicDAOImpl) List(ctx context.Context, param ListToolBasicParam, opts ...db.Option) ([]*model.ToolBasic, int64, error) {
	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).ToolBasic

	tx = tx.Where(q.ToolBasic.SpaceID.Eq(param.SpaceID))

	if param.KeyWord != "" {
		keyword := fmt.Sprintf("%%%s%%", param.KeyWord)
		tx = tx.Where(q.ToolBasic.Name.Like(keyword))
	}
	if len(param.CreatedBys) > 0 {
		tx = tx.Where(q.ToolBasic.CreatedBy.In(param.CreatedBys...))
	}
	if param.CommittedOnly {
		tx = tx.Where(q.ToolBasic.LatestCommittedVersion.Neq(""))
	}

	orderField := d.order(q, param.OrderBy, param.Asc)

	total, err := tx.Count()
	if err != nil {
		return nil, 0, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}

	basicPOs, err := tx.Order(orderField).Offset(param.Offset).Limit(param.Limit).Find()
	if err != nil {
		return nil, 0, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	return basicPOs, total, nil
}

func (d *ToolBasicDAOImpl) Update(ctx context.Context, toolID int64, updateFields map[string]interface{}, opts ...db.Option) error {
	if toolID <= 0 {
		return errorx.New("toolID is invalid, toolID = %d", toolID)
	}
	q := query.Use(d.db.NewSession(ctx, opts...))
	_, err := q.WithContext(ctx).ToolBasic.Where(q.ToolBasic.ID.Eq(toolID)).Updates(updateFields)
	if err != nil {
		return errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	return nil
}

func (d *ToolBasicDAOImpl) Delete(ctx context.Context, toolID int64, spaceID int64, opts ...db.Option) error {
	if toolID <= 0 {
		return errorx.New("toolID is invalid, toolID = %d", toolID)
	}
	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).ToolBasic
	tx = tx.Where(q.ToolBasic.ID.Eq(toolID), q.ToolBasic.SpaceID.Eq(spaceID))
	_, err := tx.Delete()
	if err != nil {
		return errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	return nil
}

func (d *ToolBasicDAOImpl) order(q *query.Query, orderBy int, asc bool) field.Expr {
	var orderExpr field.OrderExpr
	switch orderBy {
	case ListToolBasicOrderByCreatedAt:
		orderExpr = q.ToolBasic.CreatedAt
	default:
		orderExpr = q.ToolBasic.ID
	}
	if asc {
		return orderExpr.Asc()
	}
	return orderExpr.Desc()
}
