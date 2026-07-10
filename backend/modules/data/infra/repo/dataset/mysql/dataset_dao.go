// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"strconv"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/infra/platestwrite"
	"github.com/coze-dev/coze-loop/backend/infra/redis"
	"github.com/coze-dev/coze-loop/backend/modules/data/domain/dataset/entity"
	"github.com/coze-dev/coze-loop/backend/modules/data/infra/repo/dataset/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/modules/data/pkg/pagination"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
	"github.com/coze-dev/coze-loop/backend/pkg/vdutil"
)

//go:generate mockgen -destination=mocks/dataset_dao.go -package=mocks . IDatasetDAO
type IDatasetDAO interface {
	CreateDataset(ctx context.Context, dataset *model.Dataset, opt ...db.Option) error
	PatchDataset(ctx context.Context, patch, where *model.Dataset, opt ...db.Option) error
	DeleteDataset(ctx context.Context, spaceID, datasetID int64, opt ...db.Option) error
	GetDataset(ctx context.Context, spaceID, datasetID int64, opt ...db.Option) (*model.Dataset, error)
	MGetDatasets(ctx context.Context, spaceID int64, ids []int64, opt ...db.Option) ([]*model.Dataset, error)
	ListDatasets(ctx context.Context, params *ListDatasetsParams, opt ...db.Option) ([]*model.Dataset, *pagination.PageResult, error)
	ListDatasetIDs(ctx context.Context, params *ListDatasetsParams, opt ...db.Option) ([]int64, *pagination.PageResult, error)
	CountDatasets(ctx context.Context, params *ListDatasetsParams, opt ...db.Option) (int64, error)
}

func NewDatasetDAO(p db.Provider, redisCli redis.Cmdable) IDatasetDAO {
	return &DatasetDAOImpl{
		db:           p,
		writeTracker: platestwrite.NewLatestWriteTracker(redisCli),
	}
}

type DatasetDAOImpl struct {
	db           db.Provider
	writeTracker platestwrite.ILatestWriteTracker
}

type ListDatasetsParams struct {
	Paginator    *pagination.Paginator
	SpaceID      int64 `validate:"required,gt=0"` // 分片键
	IDs          []int64
	Category     string
	CreatedBys   []string
	NameLike     string // 按名称模糊搜索，
	BizCategorys []string
}

func (p *ListDatasetsParams) toWhere() (*clause.Where, error) {
	if p == nil {
		return nil, errno.DAOParamIsNilError
	}
	if err := vdutil.Validate(p); err != nil {
		return nil, err
	}
	b := db.NewWhereBuilder()
	db.MaybeAddEqToWhere(b, p.SpaceID, "space_id", db.WhereWithIndex)
	db.MaybeAddEqToWhere(b, p.Category, "category")
	db.MaybeAddInToWhere(b, p.IDs, "id")
	db.MaybeAddInToWhere(b, p.CreatedBys, "created_by")
	db.MaybeAddLikeToWhere(b, p.NameLike, "name")
	db.MaybeAddInToWhere(b, p.BizCategorys, "biz_category")
	return b.Build()
}

func (d *DatasetDAOImpl) CreateDataset(ctx context.Context, dataset *model.Dataset, opt ...db.Option) error {
	session := d.db.NewSession(ctx, opt...)
	if err := session.Create(dataset).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return errno.Errorf(errno.DatasetNameDuplicatedCode, "name=%s", dataset.Name)
		}
		return errno.DBErr(err, "create dataset")
	}

	logs.CtxInfo(ctx, "create dataset %d succeeded", dataset.ID)
	d.writeTracker.SetWriteFlag(ctx, platestwrite.ResourceTypeDataset, dataset.ID, platestwrite.SetWithSearchParam(strconv.FormatInt(dataset.SpaceID, 10)))
	return nil
}

func (d *DatasetDAOImpl) PatchDataset(ctx context.Context, patch, where *model.Dataset, opt ...db.Option) error {
	if where.ID <= 0 || where.SpaceID <= 0 {
		return errno.BadReqErrorf("both dataset_id and space_id are required")
	}

	session := d.db.NewSession(ctx, opt...)
	result := session.Where(where).Updates(patch) // clause.Where
	if err := result.Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return errno.Errorf(errno.DatasetNameDuplicatedCode, "name=%s", patch.Name)
		}
		return errno.DBErr(err, "update dataset, id=%d, space_id=%d", where.ID, where.SpaceID)
	}
	if result.RowsAffected == 0 {
		// todo: add where to error message
		return errno.ConcurrentDatasetOperationsErrorf("update dataset no rows affected, id=%d, space_id=%d", where.ID, where.SpaceID)
	}

	logs.CtxInfo(ctx, "patch dataset %d succeeded", where.ID)
	d.writeTracker.SetWriteFlag(ctx, platestwrite.ResourceTypeDataset, where.ID, platestwrite.SetWithSearchParam(strconv.FormatInt(where.SpaceID, 10)))
	return nil
}

func (r *DatasetDAOImpl) GetDataset(ctx context.Context, spaceID, datasetID int64, opt ...db.Option) (*model.Dataset, error) {
	p := &model.Dataset{}
	if !db.ContainWithMasterOpt(opt) && r.writeTracker.CheckWriteFlagByID(ctx, platestwrite.ResourceTypeDataset, datasetID) {
		opt = append(opt, db.WithMaster())
	}
	err := db.RetryOnNotFound(func(opt ...db.Option) error {
		session := r.db.NewSession(ctx, opt...)
		return session.Where("space_id = ? and id = ?", spaceID, datasetID).First(p).Error
	}, opt)
	if err != nil {
		return nil, wrapDBErr(err, "get dataset %d", datasetID)
	}

	return p, nil
}

func (r *DatasetDAOImpl) MGetDatasets(ctx context.Context, spaceID int64, ids []int64, opt ...db.Option) ([]*model.Dataset, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	if !db.ContainWithMasterOpt(opt) && r.writeTracker.CheckWriteFlagBySearchParam(ctx, platestwrite.ResourceTypeDataset, strconv.FormatInt(spaceID, 10)) {
		opt = append(opt, db.WithMaster())
	}
	session := r.db.NewSession(ctx, opt...)
	pos := make([]*model.Dataset, 0, len(ids))
	if err := session.Where("space_id = ? and id in (?)", spaceID, ids).Find(&pos).Error; err != nil {
		return nil, errno.DBErr(err, "mget datasets")
	}

	return pos, nil
}

func (r *DatasetDAOImpl) DeleteDataset(ctx context.Context, spaceID, datasetID int64, opt ...db.Option) error {
	session := r.db.NewSession(ctx, opt...)
	if err := session.Model(&model.Dataset{}).
		Where("id = ? and space_id = ?", datasetID, spaceID).
		Updates(map[string]interface{}{"deleted_at": time.Now().Unix(), "status": entity.DatasetStatusDeleted}).Error; err != nil {
		return errno.DBErr(err, "delete dataset, id=%d, space_id=%d", datasetID, spaceID)
	}

	logs.CtxInfo(ctx, "delete dataset %d succeeded", datasetID)
	r.writeTracker.SetWriteFlag(ctx, platestwrite.ResourceTypeDataset, datasetID, platestwrite.SetWithSearchParam(strconv.FormatInt(spaceID, 10)))
	return nil
}

func (r *DatasetDAOImpl) ListDatasets(ctx context.Context, params *ListDatasetsParams, opt ...db.Option) ([]*model.Dataset, *pagination.PageResult, error) {
	where, err := params.toWhere()
	if err != nil {
		return nil, nil, err
	}
	if !db.ContainWithMasterOpt(opt) && r.writeTracker.CheckWriteFlagBySearchParam(ctx, platestwrite.ResourceTypeDataset, strconv.FormatInt(params.SpaceID, 10)) {
		opt = append(opt, db.WithMaster())
	}
	var (
		tx  = r.db.NewSession(ctx, opt...).Where(where)
		pos []*model.Dataset
	)

	result := params.Paginator.Find(ctx, tx, &pos)
	if result.Error != nil {
		return nil, nil, errno.MaybeDBErr(result.Error, "list datasets")
	}

	return pos, params.Paginator.Result(), nil
}

// ListDatasetIDs 仅 SELECT id 的游标分页枚举，用于 item_count 阈值统计等无需完整 Dataset 行的场景。
// 只读取 id 列，避免拉取/物化完整 Dataset PO；软删除由 model.Dataset 的 soft_delete scope 自动过滤。
func (r *DatasetDAOImpl) ListDatasetIDs(ctx context.Context, params *ListDatasetsParams, opt ...db.Option) ([]int64, *pagination.PageResult, error) {
	where, err := params.toWhere()
	if err != nil {
		return nil, nil, err
	}
	if !db.ContainWithMasterOpt(opt) && r.writeTracker.CheckWriteFlagBySearchParam(ctx, platestwrite.ResourceTypeDataset, strconv.FormatInt(params.SpaceID, 10)) {
		opt = append(opt, db.WithMaster())
	}
	var (
		// 仅 Select id：DB 只回传 id 列，但仍以 model.Dataset 承载以便 Paginator 反射出游标。
		tx  = r.db.NewSession(ctx, opt...).Model(&model.Dataset{}).Select("id").Where(where)
		pos []*model.Dataset
	)

	result := params.Paginator.Find(ctx, tx, &pos)
	if result.Error != nil {
		return nil, nil, errno.MaybeDBErr(result.Error, "list dataset ids")
	}

	ids := make([]int64, 0, len(pos))
	for _, po := range pos {
		ids = append(ids, po.ID)
	}
	return ids, params.Paginator.Result(), nil
}

func (r *DatasetDAOImpl) CountDatasets(ctx context.Context, params *ListDatasetsParams, opt ...db.Option) (int64, error) {
	where, err := params.toWhere()
	if err != nil {
		return 0, err
	}
	if !db.ContainWithMasterOpt(opt) && r.writeTracker.CheckWriteFlagBySearchParam(ctx, platestwrite.ResourceTypeDataset, strconv.FormatInt(params.SpaceID, 10)) {
		opt = append(opt, db.WithMaster())
	}
	// notice: 如果查询条件中包含不在索引中的字段，可能会影响 count 的效率
	session := r.db.NewSession(ctx, opt...)
	var count int64
	if err := session.Model(&model.Dataset{}).
		Where(where).
		Count(&count).Error; err != nil {
		return 0, errno.DBErr(err, "count datasets")
	}

	return count, nil
}
