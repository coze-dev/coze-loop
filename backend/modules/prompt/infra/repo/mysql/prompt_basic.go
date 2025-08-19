// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/samber/lo"
	"gorm.io/gen/field"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/query"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

//go:generate mockgen -destination=mocks/prompt_basic_dao.go -package=mocks . IPromptBasicDAO
type IPromptBasicDAO interface {
	Create(ctx context.Context, basicPO *model.PromptBasic, opts ...db.Option) (err error)

	Delete(ctx context.Context, promptID int64, opts ...db.Option) (err error)

	Get(ctx context.Context, promptID int64, lock bool, opts ...db.Option) (basicPO *model.PromptBasic, err error)
	MGet(ctx context.Context, promptIDs []int64, opts ...db.Option) (idPromptPOMap map[int64]*model.PromptBasic, err error)
	MGetByPromptKey(ctx context.Context, spaceID int64, promptKeys []string, opts ...db.Option) (promptPOs []*model.PromptBasic, err error)
	List(ctx context.Context, param ListPromptBasicParam, opts ...db.Option) (basicPOs []*model.PromptBasic, total int64, err error)

	Update(ctx context.Context, promptID int64, updateFields map[string]interface{}, opts ...db.Option) (err error)
}

type ListPromptBasicParam struct {
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
	ListPromptBasicOrderByID                = 1
	ListPromptBasicOrderByCreatedAt         = 2
	ListPromptBasicOrderByLatestCommittedAt = 3
)

func NewPromptBasicDAO(db db.Provider, redisCli redis.Cmdable) IPromptBasicDAO {
	return &PromptBasicDAOImpl{
		db:           db,
		writeTracker: platestwrite.NewLatestWriteTracker(redisCli),
	}
}

type PromptBasicDAOImpl struct {
	db           db.Provider
	writeTracker platestwrite.ILatestWriteTracker
}

func (d *PromptBasicDAOImpl) Create(ctx context.Context, basicPO *model.PromptBasic, opts ...db.Option) (err error) {
	if basicPO == nil {
		return errorx.New("basicPO is empty")
	}

	q := query.Use(d.db.NewSession(ctx, opts...)).WithContext(ctx)
	basicPO.CreatedAt = time.Time{}
	basicPO.UpdatedAt = time.Time{}
	err = q.PromptBasic.Create(basicPO)
	if err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return errorx.WrapByCode(err, prompterr.CommonResourceDuplicatedCode)
		}
		return errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	d.writeTracker.SetWriteFlag(ctx, platestwrite.ResourceTypePromptBasic, basicPO.ID, platestwrite.SetWithSearchParam(strconv.FormatInt(basicPO.SpaceID, 10)))
	return nil
}

func (d *PromptBasicDAOImpl) Delete(ctx context.Context, promptID int64, opts ...db.Option) (err error) {
	if promptID <= 0 {
		return errorx.New("promptID is invalid, promptID = %d", promptID)
	}

	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).PromptBasic
	tx = tx.Where(q.PromptBasic.ID.Eq(promptID))
	_, err = tx.Delete(&model.PromptBasic{})
	if err != nil {
		return errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	d.writeTracker.SetWriteFlag(ctx, platestwrite.ResourceTypePromptBasic, promptID)
	return nil
}

func (d *PromptBasicDAOImpl) Get(ctx context.Context, promptID int64, lock bool, opts ...db.Option) (basicPO *model.PromptBasic, err error) {
	if promptID <= 0 {
		return nil, errorx.New("promptID is invalid, promptID = %d", promptID)
	}
	if d.writeTracker.CheckWriteFlagByID(ctx, platestwrite.ResourceTypePromptBasic, promptID) {
		opts = append(opts, db.WithMaster())
	}
	if lock {
		opts = append(opts, db.WithSelectForUpdate())
	}
	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).PromptBasic
	promptPOs, err := tx.Where(q.PromptBasic.ID.Eq(promptID)).Find()
	if err != nil {
		return nil, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	if len(promptPOs) <= 0 {
		return nil, nil
	}
	return promptPOs[0], nil
}

func (d *PromptBasicDAOImpl) MGet(ctx context.Context, promptIDs []int64, opts ...db.Option) (idPromptPOMap map[int64]*model.PromptBasic, err error) {
	if len(promptIDs) <= 0 {
		return nil, errorx.WrapByCode(err, prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("PromptBasicDAOImpl.MGet invalid param"))
	}
	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).PromptBasic
	tx = tx.Where(q.PromptBasic.ID.In(promptIDs...))
	promptPOs, err := tx.Find()
	if err != nil {
		return nil, err
	}
	if len(promptPOs) <= 0 {
		return nil, nil
	}
	idPromptPOMap = make(map[int64]*model.PromptBasic, len(promptPOs))
	for _, promptPO := range promptPOs {
		idPromptPOMap[promptPO.ID] = promptPO
	}
	if len(idPromptPOMap) <= 0 {
		return nil, nil
	}
	return idPromptPOMap, nil
}

func (d *PromptBasicDAOImpl) MGetByPromptKey(ctx context.Context, spaceID int64, promptKeys []string, opts ...db.Option) (promptPOs []*model.PromptBasic, err error) {
	if len(promptKeys) <= 0 {
		return nil, errorx.WrapByCode(err, prompterr.CommonInvalidParamCode, errorx.WithExtraMsg("PromptBasicDAOImpl.MGetByPromptKey invalid param"))
	}
	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).PromptBasic
	tx = tx.Where(q.PromptBasic.SpaceID.Eq(spaceID), q.PromptBasic.PromptKey.In(promptKeys...))
	promptPOs, err = tx.Find()
	if err != nil {
		return nil, err
	}
	return promptPOs, nil
}

func (d *PromptBasicDAOImpl) List(ctx context.Context, param ListPromptBasicParam, opts ...db.Option) (basicPOs []*model.PromptBasic, total int64, err error) {
	if param.SpaceID <= 0 || param.Offset < 0 || param.Limit <= 0 {
		return nil, 0, errorx.New("param(SpaceID or Offset or Limit) is invalid, param = %s", json.Jsonify(param))
	}
	if d.writeTracker.CheckWriteFlagBySearchParam(ctx, platestwrite.ResourceTypePromptBasic, strconv.FormatInt(param.SpaceID, 10)) {
		opts = append(opts, db.WithMaster())
	}

	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).PromptBasic
	tx = tx.Where(q.PromptBasic.SpaceID.Eq(param.SpaceID))
	if len(param.CreatedBys) > 0 {
		tx = tx.Where(q.PromptBasic.CreatedBy.In(param.CreatedBys...))
	}
	if !lo.IsEmpty(param.KeyWord) {
		likeExpr := field.Or(
			q.PromptBasic.PromptKey.Like(fmt.Sprintf("%%%s%%", param.KeyWord)),
			q.PromptBasic.Name.Like(fmt.Sprintf("%%%s%%", param.KeyWord)),
		)
		tx = tx.Where(likeExpr)
	}
	if param.CommittedOnly {
		tx = tx.Where(q.PromptBasic.LatestVersion.Neq(""))
	}
	total, err = tx.Count()
	if err != nil {
		return nil, 0, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	tx = tx.Order(d.order(q, param.OrderBy, param.Asc)).Offset(param.Offset)
	tx = tx.Limit(param.Limit)
	basicPOs, err = tx.Find()
	if err != nil {
		return nil, 0, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	if len(basicPOs) <= 0 {
		return nil, total, nil
	}
	return basicPOs, total, nil
}

func (d *PromptBasicDAOImpl) Update(ctx context.Context, promptID int64, updateFields map[string]interface{}, opts ...db.Option) (err error) {
	if promptID <= 0 {
		return errorx.New("promptID is invalid, promptID = %d", promptID)
	}
	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).PromptBasic
	tx = tx.Where(q.PromptBasic.ID.Eq(promptID))
	_, err = tx.Updates(updateFields)
	if err != nil {
		return errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	d.writeTracker.SetWriteFlag(ctx, platestwrite.ResourceTypePromptBasic, promptID)
	return nil
}

func (d *PromptBasicDAOImpl) order(q *query.Query, orderBy int, asc bool) field.Expr {
	var orderExpr field.OrderExpr
	switch orderBy {
	case ListPromptBasicOrderByID:
		orderExpr = q.PromptBasic.ID
	case ListPromptBasicOrderByCreatedAt:
		orderExpr = q.PromptBasic.CreatedAt
	case ListPromptBasicOrderByLatestCommittedAt:
		orderExpr = q.PromptBasic.LatestCommitTime
	default:
		orderExpr = q.PromptBasic.ID
	}
	if asc {
		return orderExpr.Asc()
	}
	return orderExpr.Desc()
}
