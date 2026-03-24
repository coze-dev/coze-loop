package mysql

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/samber/lo"
	"gorm.io/gen/field"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/query"
	prompterr "github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

type IToolBasicDAO interface {
	Create(ctx context.Context, basicPO *model.ToolBasic, opts ...db.Option) (err error)
	Delete(ctx context.Context, toolID int64, spaceID int64, opts ...db.Option) (err error)
	Get(ctx context.Context, toolID int64, opts ...db.Option) (basicPO *model.ToolBasic, err error)
	List(ctx context.Context, param ListToolBasicParam, opts ...db.Option) (basicPOs []*model.ToolBasic, total int64, err error)
	Update(ctx context.Context, toolID int64, updateFields map[string]interface{}, opts ...db.Option) (err error)
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
	ListToolBasicOrderByID          = 1
	ListToolBasicOrderByCreatedAt   = 2
	ListToolBasicOrderByUpdatedAt   = 3
	ListToolBasicOrderByCommittedAt = 4
)

func NewToolBasicDAO(db db.Provider, redisCli redis.Cmdable) IToolBasicDAO {
	return &ToolBasicDAOImpl{
		db:           db,
		writeTracker: platestwrite.NewLatestWriteTracker(redisCli),
	}
}

type ToolBasicDAOImpl struct {
	db           db.Provider
	writeTracker platestwrite.ILatestWriteTracker
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
		return errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	d.writeTracker.SetWriteFlag(ctx, platestwrite.ResourceTypeToolBasic, basicPO.ID, platestwrite.SetWithSearchParam(strconv.FormatInt(basicPO.SpaceID, 10)))
	return nil
}

func (d *ToolBasicDAOImpl) Delete(ctx context.Context, toolID int64, spaceID int64, opts ...db.Option) (err error) {
	if toolID <= 0 {
		return errorx.New("toolID is invalid, toolID = %d", toolID)
	}
	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).ToolBasic
	tx = tx.Where(q.ToolBasic.ID.Eq(toolID))
	_, err = tx.Delete(&model.ToolBasic{})
	if err != nil {
		return errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	d.writeTracker.SetWriteFlag(ctx, platestwrite.ResourceTypeToolBasic, toolID, platestwrite.SetWithSearchParam(strconv.FormatInt(spaceID, 10)))
	return nil
}

func (d *ToolBasicDAOImpl) Get(ctx context.Context, toolID int64, opts ...db.Option) (basicPO *model.ToolBasic, err error) {
	if toolID <= 0 {
		return nil, errorx.New("toolID is invalid, toolID = %d", toolID)
	}
	if d.writeTracker.CheckWriteFlagByID(ctx, platestwrite.ResourceTypeToolBasic, toolID) {
		opts = append(opts, db.WithMaster())
	}
	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).ToolBasic
	basicPOs, err := tx.Where(q.ToolBasic.ID.Eq(toolID)).Find()
	if err != nil {
		return nil, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	if len(basicPOs) == 0 {
		return nil, nil
	}
	return basicPOs[0], nil
}

func (d *ToolBasicDAOImpl) List(ctx context.Context, param ListToolBasicParam, opts ...db.Option) (basicPOs []*model.ToolBasic, total int64, err error) {
	if param.SpaceID <= 0 || param.Offset < 0 || param.Limit <= 0 {
		return nil, 0, errorx.New("param(SpaceID or Offset or Limit) is invalid, param = %s", json.Jsonify(param))
	}
	if d.writeTracker.CheckWriteFlagBySearchParam(ctx, platestwrite.ResourceTypeToolBasic, strconv.FormatInt(param.SpaceID, 10)) {
		opts = append(opts, db.WithMaster())
	}

	q := query.Use(d.db.NewSession(ctx, opts...))
	tx := q.WithContext(ctx).ToolBasic
	tx = tx.Where(q.ToolBasic.SpaceID.Eq(param.SpaceID))
	if len(param.CreatedBys) > 0 {
		tx = tx.Where(q.ToolBasic.CreatedBy.In(param.CreatedBys...))
	}
	if !lo.IsEmpty(param.KeyWord) {
		likeExpr := field.Or(
			q.ToolBasic.Name.Like(fmt.Sprintf("%%%s%%", param.KeyWord)),
			q.ToolBasic.Description.Like(fmt.Sprintf("%%%s%%", param.KeyWord)),
		)
		tx = tx.Where(likeExpr)
	}
	if param.CommittedOnly {
		tx = tx.Where(q.ToolBasic.LatestCommittedVersion.Neq(""))
	}
	total, err = tx.Count()
	if err != nil {
		return nil, 0, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	tx = tx.Order(d.order(q, param.OrderBy, param.Asc)).Offset(param.Offset).Limit(param.Limit)
	basicPOs, err = tx.Find()
	if err != nil {
		return nil, 0, errorx.WrapByCode(err, prompterr.CommonMySqlErrorCode)
	}
	return basicPOs, total, nil
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
	d.writeTracker.SetWriteFlag(ctx, platestwrite.ResourceTypeToolBasic, toolID)
	return nil
}

func (d *ToolBasicDAOImpl) order(q *query.Query, orderBy int, asc bool) field.Expr {
	var orderExpr field.OrderExpr
	switch orderBy {
	case ListToolBasicOrderByID:
		orderExpr = q.ToolBasic.ID
	case ListToolBasicOrderByCreatedAt:
		orderExpr = q.ToolBasic.CreatedAt
	case ListToolBasicOrderByCommittedAt:
		orderExpr = q.ToolBasic.LatestCommittedAt
	case ListToolBasicOrderByUpdatedAt:
		orderExpr = q.ToolBasic.UpdatedAt
	default:
		orderExpr = q.ToolBasic.ID
	}
	if asc {
		return orderExpr.Asc()
	}
	return orderExpr.Desc()
}
