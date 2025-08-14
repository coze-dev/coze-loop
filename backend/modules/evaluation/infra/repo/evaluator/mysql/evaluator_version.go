// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"sync"
	"time"

	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
)

// EvaluatorVersionDAO 定义 EvaluatorVersion 的 Dao 接口
//
//go:generate mockgen -destination mocks/evaluator_version_mock.go -package=mocks . EvaluatorVersionDAO
type EvaluatorVersionDAO interface {
	CreateEvaluatorVersion(ctx context.Context, version *model.EvaluatorVersion, opts ...db.Option) error
	UpdateEvaluatorDraft(ctx context.Context, version *model.EvaluatorVersion, opts ...db.Option) error

	DeleteEvaluatorVersion(ctx context.Context, id int64, userID string, opts ...db.Option) error
	BatchDeleteEvaluatorVersionByEvaluatorIDs(ctx context.Context, evaluatorIDs []int64, userID string, opts ...db.Option) error

	ListEvaluatorVersion(ctx context.Context, req *ListEvaluatorVersionRequest, opts ...db.Option) (*ListEvaluatorVersionResponse, error)
	BatchGetEvaluatorVersionByID(ctx context.Context, spaceID *int64, ids []int64, includeDeleted bool, opts ...db.Option) ([]*model.EvaluatorVersion, error)
	BatchGetEvaluatorDraftByEvaluatorID(ctx context.Context, evaluatorIDs []int64, includeDeleted bool, opts ...db.Option) ([]*model.EvaluatorVersion, error)
	BatchGetEvaluatorVersionsByEvaluatorIDs(ctx context.Context, evaluatorIDs []int64, includeDeleted bool, opts ...db.Option) ([]*model.EvaluatorVersion, error)
	CheckVersionExist(ctx context.Context, evaluatorID int64, version string, opts ...db.Option) (bool, error)
}

// ListEvaluatorVersionRequest 定义查询 EvaluatorVersion 的请求结构体
type ListEvaluatorVersionRequest struct {
	EvaluatorID   int64
	QueryVersions []string
	OrderBy       []*OrderBy
	PageSize      int32
	PageNum       int32
}

// ListEvaluatorVersionResponse 定义查询 EvaluatorVersion 的响应结构体
type ListEvaluatorVersionResponse struct {
	TotalCount int64
	Versions   []*model.EvaluatorVersion
}

// OrderBy 定义排序规则结构体
type OrderBy struct {
	Field  string
	ByDesc bool
}

var (
	evaluatorVersionDaoOnce      = sync.Once{}
	singletonEvaluatorVersionDao EvaluatorVersionDAO
)

// EvaluatorVersionDAOImpl 实现 EvaluatorVersionDAO 接口
type EvaluatorVersionDAOImpl struct {
	provider db.Provider
}

// NewEvaluatorVersionDAO 创建 EvaluatorVersionDAOImpl 实例
func NewEvaluatorVersionDAO(p db.Provider) EvaluatorVersionDAO {
	evaluatorVersionDaoOnce.Do(func() {
		singletonEvaluatorVersionDao = &EvaluatorVersionDAOImpl{
			provider: p,
		}
	})
	return singletonEvaluatorVersionDao
}

var SupportedOrderBys = map[string]string{
	"updated_at": "updated_at",
	"created_at": "created_at",
}

var (
	defaultPageSize = int64(10)
	defaultPageNum  = int64(1)
)

func getOrderBy(orderBy *OrderBy) string {
	if orderBy == nil {
		return ""
	}
	if field, ok := SupportedOrderBys[orderBy.Field]; ok {
		if orderBy.ByDesc {
			return field + " desc"
		}
		return field + " asc"
	}
	return ""
}

// CreateEvaluatorVersion 创建 EvaluatorVersion 记录
func (dao *EvaluatorVersionDAOImpl) CreateEvaluatorVersion(ctx context.Context, version *model.EvaluatorVersion, opts ...db.Option) error {
	// 通过opts获取当前的db session实例
	dbsession := dao.provider.NewSession(ctx, opts...)
	return dbsession.WithContext(ctx).Create(version).Error
}

// CheckVersionExist 校验当前Evaluator版本是否存在
func (dao *EvaluatorVersionDAOImpl) CheckVersionExist(ctx context.Context, evaluatorID int64, version string, opts ...db.Option) (bool, error) {
	var count int64

	// 通过opts获取当前的db session实例
	dbsession := dao.provider.NewSession(ctx, opts...)

	err := dbsession.WithContext(ctx).Model(&model.EvaluatorVersion{}).
		Where("evaluator_id =?", evaluatorID).
		Where("version =?", version).
		Where("deleted_at IS NULL").
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ListEvaluatorVersion 获取 Evaluator 记录
func (dao *EvaluatorVersionDAOImpl) ListEvaluatorVersion(ctx context.Context, req *ListEvaluatorVersionRequest, opts ...db.Option) (*ListEvaluatorVersionResponse, error) {
	var pos []*model.EvaluatorVersion

	// 通过opts获取当前的db session实例
	dbsession := dao.provider.NewSession(ctx, opts...)

	db := dbsession.WithContext(ctx).Where("evaluator_id =?", req.EvaluatorID)
	if len(req.QueryVersions) != 0 {
		db = db.Where("version in ?", req.QueryVersions)
	}
	db.Where("version != ?", consts.EvaluatorVersionDraftKey)
	db = db.Where("deleted_at IS NULL")
	if len(req.OrderBy) > 0 {
		for _, orderBy := range req.OrderBy {
			if getOrderBy(orderBy) != "" {
				db = db.Order(getOrderBy(orderBy))
			}
		}
	}

	var total int64
	err := db.Model(&model.EvaluatorVersion{}).Count(&total).Error
	if err != nil {
		return nil, err
	}
	if req.PageSize != 0 && req.PageNum != 0 {
		db = db.Limit(int(req.PageSize)).Offset(int((req.PageNum - 1) * req.PageSize))
	} else {
		db = db.Limit(int(defaultPageSize)).Offset(int(defaultPageNum - 1))
	}
	err = db.Find(&pos).Error
	if err != nil {
		return nil, err
	}

	return &ListEvaluatorVersionResponse{
		TotalCount: total,
		Versions:   pos,
	}, nil
}

// GetEvaluatorVersionByID 根据 ID 获取 Evaluator 记录
func (dao *EvaluatorVersionDAOImpl) GetEvaluatorVersionByID(ctx context.Context, id int64, includeDeleted bool, opts ...db.Option) (*model.EvaluatorVersion, error) {
	var po model.EvaluatorVersion

	// 通过opts获取当前的db session实例
	dbsession := dao.provider.NewSession(ctx, opts...)

	query := dbsession.WithContext(ctx).Where("id = ?", id)
	if includeDeleted {
		query = query.Unscoped() // 解除软删除过滤
	}
	err := query.First(&po).Error
	if err != nil {
		return nil, err
	}
	return &po, nil
}

// BatchGetEvaluatorVersionByID 根据 ID 获取 Evaluator 记录
func (dao *EvaluatorVersionDAOImpl) BatchGetEvaluatorVersionByID(ctx context.Context, spaceID *int64, ids []int64, includeDeleted bool, opts ...db.Option) ([]*model.EvaluatorVersion, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var pos []*model.EvaluatorVersion

	// 通过opts获取当前的db session实例
	dbsession := dao.provider.NewSession(ctx, opts...)

	query := dbsession.WithContext(ctx).Where("id in (?)", ids)
	if spaceID != nil {
		query = query.Where("space_id =?", *spaceID)
	}
	if includeDeleted {
		query = query.Unscoped() // 解除软删除过滤
	}
	err := query.Find(&pos).Error
	if err != nil {
		return nil, err
	}
	return pos, nil
}

// GetEvaluatorDraftByEvaluatorID 根据评估器ID获取单个草稿版本
func (dao *EvaluatorVersionDAOImpl) GetEvaluatorDraftByEvaluatorID(ctx context.Context, evaluatorID int64, includeDeleted bool, opts ...db.Option) (*model.EvaluatorVersion, error) {
	var po model.EvaluatorVersion

	// 通过opts获取当前的db session实例
	dbsession := dao.provider.NewSession(ctx, opts...)

	query := dbsession.WithContext(ctx).Where("evaluator_id = ? AND version = ?", evaluatorID, consts.EvaluatorVersionDraftKey)
	if includeDeleted {
		query = query.Unscoped() // 解除软删除过滤
	}
	err := query.First(&po).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &po, nil
}

// BatchGetEvaluatorDraftByEvaluatorID 根据评估器ID获取草稿版本
func (dao *EvaluatorVersionDAOImpl) BatchGetEvaluatorDraftByEvaluatorID(ctx context.Context, evaluatorIDs []int64, includeDeleted bool, opts ...db.Option) ([]*model.EvaluatorVersion, error) {
	if len(evaluatorIDs) == 0 {
		return nil, nil
	}

	var pos []*model.EvaluatorVersion

	// 通过opts获取当前的db session实例
	dbsession := dao.provider.NewSession(ctx, opts...)

	query := dbsession.WithContext(ctx).Where("evaluator_id IN ? AND version = ?", evaluatorIDs, consts.EvaluatorVersionDraftKey)
	if includeDeleted {
		query = query.Unscoped() // 解除软删除过滤
	}
	err := query.Find(&pos).Error
	if err != nil {
		return nil, err
	}

	return pos, nil
}

// BatchGetEvaluatorVersionsByEvaluatorIDs 根据评估器ID获取版本
func (dao *EvaluatorVersionDAOImpl) BatchGetEvaluatorVersionsByEvaluatorIDs(ctx context.Context, evaluatorIDs []int64, includeDeleted bool, opts ...db.Option) ([]*model.EvaluatorVersion, error) {
	if len(evaluatorIDs) == 0 {
		return nil, nil
	}

	var pos []*model.EvaluatorVersion

	// 通过opts获取当前的db session实例
	dbsession := dao.provider.NewSession(ctx, opts...)

	query := dbsession.WithContext(ctx).Where("evaluator_id IN ? ", evaluatorIDs)
	if includeDeleted {
		query = query.Unscoped() // 解除软删除过滤
	}
	err := query.Find(&pos).Error
	if err != nil {
		return nil, err
	}

	return pos, nil
}

// DeleteEvaluatorVersion 根据 ID 删除 Evaluator 记录
func (dao *EvaluatorVersionDAOImpl) DeleteEvaluatorVersion(ctx context.Context, id int64, userID string, opts ...db.Option) error {
	// 通过opts获取当前的db session实例
	dbsession := dao.provider.NewSession(ctx, opts...)

	return dbsession.WithContext(ctx).Model(&model.EvaluatorVersion{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"deleted_at": gorm.DeletedAt{Time: time.Now(), Valid: true},
			"updated_by": userID,
		}).Error
}

// BatchDeleteEvaluatorVersionByEvaluatorIDs 根据评估器ID批量删除 Evaluator 记录
func (dao *EvaluatorVersionDAOImpl) BatchDeleteEvaluatorVersionByEvaluatorIDs(ctx context.Context, evaluatorIDs []int64, userID string, opts ...db.Option) error {
	// 通过opts获取当前的db session实例
	dbsession := dao.provider.NewSession(ctx, opts...)

	return dbsession.WithContext(ctx).Model(&model.EvaluatorVersion{}).
		Where("evaluator_id in ?", evaluatorIDs).
		Updates(map[string]interface{}{
			"deleted_at": gorm.DeletedAt{Time: time.Now(), Valid: true},
			"updated_by": userID,
		}).Error
}

// UpdateEvaluatorDraft 更新版本信息
func (dao *EvaluatorVersionDAOImpl) UpdateEvaluatorDraft(ctx context.Context, version *model.EvaluatorVersion, opts ...db.Option) error {
	// 通过opts获取当前的db session实例
	dbsession := dao.provider.NewSession(ctx, opts...)

	return dbsession.WithContext(ctx).Model(&model.EvaluatorVersion{}).
		Where("evaluator_id = ?", version.EvaluatorID).
		Where("version =?", consts.EvaluatorVersionDraftKey).
		Where("deleted_at IS NULL").
		Updates(version).
		Error
}
