// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"time"

	"gorm.io/gen/field"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/query"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

//go:generate mockgen -destination=mocks/tool_basic_dao.go -package=mocks . IToolBasicDAO
type IToolBasicDAO interface {
	Create(ctx context.Context, basicPO *model.ToolBasic, opts ...db.Option) (err error)
	Delete(ctx context.Context, toolID int64, opts ...db.Option) (err error)
	Get(ctx context.Context, toolID int64, opts ...db.Option) (basicPO *model.ToolBasic, err error)
	MGet(ctx context.Context, toolIDs []int64, opts ...db.Option) (idToolPOMap map[int64]*model.ToolBasic, err error)
	List(ctx context.Context, param ListToolBasicParam, opts ...db.Option) (basicPOs []*model.ToolBasic, total int64, err error)
	Update(ctx context.Context, toolID int64, updateFields map[string]interface{}, opts ...db.Option) (err error)
}

type ListToolBasicParam struct {
	SpaceID       int64
	KeyWord       string
	CreatedBys    []string
	CommittedOnly bool
	Offset        int
	Limit         int
	OrderBy       int
	Asc           bool
}

const (
	ListToolBasicOrderByCreatedAt         = 1
	ListToolBasicOrderByLatestCommittedAt = 2
)

func NewToolBasicDAO(db db.Provider) IToolBasicDAO {
	return &ToolBasicDAOImpl{
		db: db,
	}
}

type ToolBasicDAOImpl struct {
	db db.Provider
}

func (d *ToolBasicDAOImpl) Create(ctx context.Context, basicPO *model.ToolBasic, opts ...db.Option) (err error) {
	if basicPO == nil {
		return errorx.New("basicPO is empty")
	}

	q := query.Use(d.db.NewSession(ctx, opts...)).WithContext(ctx)
	basicPO.CreatedAt = time.Time{}
	basicPO.UpdatedAt = time.Time{}
	err = q.ToolBasic.Create(basicPO)
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
		}
		return errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	return nil
}

func (d *ToolBasicDAOImpl) Delete(ctx context.Context, toolID int64, opts ...db.Option) (err error) {
	if toolID <= 0 {
		return errorx.New("toolID is invalid, toolID = %d", toolID)
	}

	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).ToolBasic
	tx = tx.Where(q.ToolBasic.ID.Eq(toolID))
	_, err = tx.Delete()
	if err != nil {
		return errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	return nil
}

func (d *ToolBasicDAOImpl) Get(ctx context.Context, toolID int64, opts ...db.Option) (basicPO *model.ToolBasic, err error) {
	if toolID <= 0 {
		return nil, errorx.New("toolID is invalid, toolID = %d", toolID)
	}

	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).ToolBasic
	tx = tx.Where(q.ToolBasic.ID.Eq(toolID))
	basicPOs, err := tx.Find()
	if err != nil {
		return nil, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	if len(basicPOs) <= 0 {
		return nil, nil
	}
	return basicPOs[0], nil
}

func (d *ToolBasicDAOImpl) MGet(ctx context.Context, toolIDs []int64, opts ...db.Option) (idToolPOMap map[int64]*model.ToolBasic, err error) {
	if len(toolIDs) <= 0 {
		return nil, nil
	}

	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).ToolBasic
	tx = tx.Where(q.ToolBasic.ID.In(toolIDs...))
	basicPOs, err := tx.Find()
	if err != nil {
		return nil, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	if len(basicPOs) == 0 {
		return nil, nil
	}

	result := make(map[int64]*model.ToolBasic, len(basicPOs))
	for _, po := range basicPOs {
		result[po.ID] = po
	}
	return result, nil
}

func (d *ToolBasicDAOImpl) List(ctx context.Context, param ListToolBasicParam, opts ...db.Option) (basicPOs []*model.ToolBasic, total int64, err error) {
	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).ToolBasic
	tx = tx.Where(q.ToolBasic.SpaceID.Eq(param.SpaceID))

	if param.KeyWord != "" {
		tx = tx.Where(q.ToolBasic.Name.Like("%" + param.KeyWord + "%"))
	}

	if len(param.CreatedBys) > 0 {
		tx = tx.Where(q.ToolBasic.CreatedBy.In(param.CreatedBys...))
	}

	if param.CommittedOnly {
		tx = tx.Where(q.ToolBasic.LatestCommittedVersion.IsNotNull())
		tx = tx.Where(q.ToolBasic.LatestCommittedVersion.Neq(""))
	}

	// 排序
	var orderExpr field.OrderExpr
	switch param.OrderBy {
	case ListToolBasicOrderByLatestCommittedAt:
		orderExpr = q.ToolBasic.UpdatedAt
	default:
		orderExpr = q.ToolBasic.CreatedAt
	}

	if param.Asc {
		tx = tx.Order(orderExpr.Asc())
	} else {
		tx = tx.Order(orderExpr.Desc())
	}

	count, err := tx.Count()
	if err != nil {
		return nil, 0, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}

	tx = tx.Offset(param.Offset).Limit(param.Limit)
	basicPOs, err = tx.Find()
	if err != nil {
		return nil, 0, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}

	return basicPOs, count, nil
}

func (d *ToolBasicDAOImpl) Update(ctx context.Context, toolID int64, updateFields map[string]interface{}, opts ...db.Option) (err error) {
	if toolID <= 0 {
		return errorx.New("toolID is invalid, toolID = %d", toolID)
	}

	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).ToolBasic
	tx = tx.Where(q.ToolBasic.ID.Eq(toolID))
	_, err = tx.Updates(updateFields)
	if err != nil {
		return errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	return nil
}
