// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/bytedance/gg/gptr"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
)

// EvaluatorDAO 定义 Evaluator 的 Dao 接口
//
//go:generate mockgen -destination mocks/evaluator_mock.go -package=mocks . EvaluatorDAO
type EvaluatorDAO interface {
	CreateEvaluator(ctx context.Context, evaluator *model.Evaluator, opts ...db.Option) error
	GetEvaluatorByID(ctx context.Context, id int64, includeDeleted bool, opts ...db.Option) (*model.Evaluator, error)
	BatchGetEvaluatorByID(ctx context.Context, ids []int64, includeDeleted bool, opts ...db.Option) ([]*model.Evaluator, error)
	UpdateEvaluatorMeta(ctx context.Context, do *model.Evaluator, opts ...db.Option) error
	UpdateEvaluatorDraftSubmitted(ctx context.Context, evaluatorID int64, draftSubmitted bool, userID string, opts ...db.Option) error
	BatchDeleteEvaluator(ctx context.Context, ids []int64, userID string, opts ...db.Option) error
	ListEvaluator(ctx context.Context, req *ListEvaluatorRequest, opts ...db.Option) (*ListEvaluatorResponse, error)
	CheckNameExist(ctx context.Context, spaceID, evaluatorID int64, name string, opts ...db.Option) (bool, error)
	UpdateEvaluatorLatestVersion(ctx context.Context, evaluatorID int64, version, userID string, opts ...db.Option) error
}

var (
	evaluatorDaoOnce      = sync.Once{}
	singletonEvaluatorDao EvaluatorDAO
)

// EvaluatorDAOImpl 实现 EvaluatorDAO 接口
type EvaluatorDAOImpl struct {
	provider db.Provider
}

func NewEvaluatorDAO(p db.Provider) EvaluatorDAO {
	evaluatorDaoOnce.Do(func() {
		singletonEvaluatorDao = &EvaluatorDAOImpl{
			provider: p,
		}
	})
	return singletonEvaluatorDao
}

func (dao *EvaluatorDAOImpl) CreateEvaluator(ctx context.Context, po *model.Evaluator, opts ...db.Option) error {
	// 通过opts获取当前的db session实例
	dbsession := dao.provider.NewSession(ctx, opts...)
	po.DraftSubmitted = gptr.Of(true) // 初始化创建时草稿统一已提交
	return dbsession.WithContext(ctx).Create(po).Error
}

// GetEvaluatorByID 根据 ID 获取 Evaluator
func (dao *EvaluatorDAOImpl) GetEvaluatorByID(ctx context.Context, id int64, includeDeleted bool, opts ...db.Option) (*model.Evaluator, error) {
	// 通过opts获取当前的db session实例
	dbsession := dao.provider.NewSession(ctx, opts...)

	po := &model.Evaluator{}
	query := dbsession.WithContext(ctx).Where("id = ?", id)
	if includeDeleted {
		query = query.Unscoped() // 解除软删除过滤
	}
	err := query.First(po).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return po, nil
}

// BatchGetEvaluatorByID 批量根据ID 获取 Evaluator
func (dao *EvaluatorDAOImpl) BatchGetEvaluatorByID(ctx context.Context, ids []int64, includeDeleted bool, opts ...db.Option) ([]*model.Evaluator, error) {
	// 通过opts获取当前的db session实例
	if contexts.CtxWriteDB(ctx) {
		opts = append(opts, db.WithMaster())
	}
	dbsession := dao.provider.NewSession(ctx, opts...)

	poList := make([]*model.Evaluator, 0)
	query := dbsession.WithContext(ctx).Where("id in (?)", ids)
	if includeDeleted {
		query = query.Unscoped() // 解除软删除过滤
	}
	err := query.Find(&poList).Error
	if err != nil {
		return nil, err
	}
	return poList, nil
}

// UpdateEvaluatorDraftSubmitted 更新 Evaluator
func (dao *EvaluatorDAOImpl) UpdateEvaluatorDraftSubmitted(ctx context.Context, evaluatorID int64, draftSubmitted bool, userID string, opts ...db.Option) error {
	// 通过opts获取当前的db session实例
	dbsession := dao.provider.NewSession(ctx, opts...)
	return dbsession.WithContext(ctx).Model(&model.Evaluator{}).
		Where("id = ?", evaluatorID).
		Where("deleted_at IS NULL").
		Updates(map[string]interface{}{
			"draft_submitted": draftSubmitted,
			"updated_by":      userID,
		}).Error
}

// UpdateEvaluatorMeta 更新 Evaluator
func (dao *EvaluatorDAOImpl) UpdateEvaluatorMeta(ctx context.Context, po *model.Evaluator, opts ...db.Option) error {
	// 通过opts获取当前的db session实例
	dbsession := dao.provider.NewSession(ctx, opts...)
	updateMap := make(map[string]interface{})
	// 基础字段转换
	updateMap["name"] = po.Name
	updateMap["description"] = po.Description
	updateMap["updated_by"] = po.UpdatedBy
	return dbsession.WithContext(ctx).Model(&model.Evaluator{}).
		Where("id = ?", po.ID).      // 添加ID筛选条件
		Where("deleted_at IS NULL"). // 添加软删除筛选条件
		Updates(updateMap).          // 使用Updates代替Save，避免全字段覆盖
		Error
}

// DeleteEvaluator 根据 ID 删除 Evaluator
func (dao *EvaluatorDAOImpl) DeleteEvaluator(ctx context.Context, id int64, userID string, opts ...db.Option) error {
	// 通过opts获取当前的db session实例
	dbsession := dao.provider.NewSession(ctx, opts...)
	return dbsession.WithContext(ctx).Model(&model.Evaluator{}).
		Where("id = ?", id). // 添加ID筛选条件
		Updates(map[string]interface{}{
			"deleted_at": gorm.DeletedAt{Time: time.Now(), Valid: true},
			"updated_by": userID,
		}).Error
}

func (dao *EvaluatorDAOImpl) BatchDeleteEvaluator(ctx context.Context, ids []int64, userID string, opts ...db.Option) error {
	if len(ids) == 0 {
		return nil
	}

	// 通过opts获取当前的db session实例
	dbsession := dao.provider.NewSession(ctx, opts...)

	// 开启事务确保操作的原子性
	return dbsession.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Model(&model.Evaluator{}).
			Where("id IN (?)", ids).
			Updates(map[string]interface{}{
				"deleted_at": gorm.DeletedAt{Time: time.Now(), Valid: true},
				"updated_by": userID,
			}).Error
	})
}

// CheckNameExist 校验当前名称是否存在
func (dao *EvaluatorDAOImpl) CheckNameExist(ctx context.Context, spaceID, evaluatorID int64, name string, opts ...db.Option) (bool, error) {
	var count int64

	// 通过opts获取当前的db session实例
	dbsession := dao.provider.NewSession(ctx, opts...)

	query := dbsession.WithContext(ctx).Model(&model.Evaluator{})
	if evaluatorID != consts.EvaluatorEmptyID {
		query = query.Where("id != ?", evaluatorID)
	}
	err := query.Where("space_id =?", spaceID).
		Where("name =?", name).
		Where("deleted_at IS NULL").
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

type ListEvaluatorRequest struct {
	SpaceID       int64
	SearchName    string
	CreatorIDs    []int64
	EvaluatorType []int32
	PageSize      int32
	PageNum       int32
	OrderBy       []*OrderBy
}

type ListEvaluatorResponse struct {
	TotalCount int64
	Evaluators []*model.Evaluator
}

func (dao *EvaluatorDAOImpl) ListEvaluator(ctx context.Context, req *ListEvaluatorRequest, opts ...db.Option) (*ListEvaluatorResponse, error) {
	// 通过opts获取当前的db session实例
	dbsession := dao.provider.NewSession(ctx, opts...)

	query := dbsession.WithContext(ctx).Model(&model.Evaluator{}).Where("space_id = ?", req.SpaceID)

	// 添加名称模糊搜索
	if len(req.SearchName) > 0 {
		query = query.Where("name LIKE ?", "%"+req.SearchName+"%")
	}

	// 添加创建者过滤
	if len(req.CreatorIDs) > 0 {
		query = query.Where("created_by IN (?)", req.CreatorIDs)
	}

	// 添加评测器类型过滤
	if len(req.EvaluatorType) > 0 {
		query = query.Where("evaluator_type IN (?)", req.EvaluatorType)
	}
	query = query.Where("deleted_at IS NULL")

	// 添加排序条件
	if len(req.OrderBy) > 0 {
		for _, orderBy := range req.OrderBy {
			if getOrderBy(orderBy) != "" {
				query = query.Order(getOrderBy(orderBy))
			}
		}
	}

	// 先查询总数
	var total int64
	countQuery := query.Session(&gorm.Session{})
	if err := countQuery.Count(&total).Error; err != nil {
		return nil, err
	}

	// 分页处理
	if req.PageSize != 0 && req.PageNum != 0 {
		offset := (req.PageNum - 1) * req.PageSize
		query = query.Limit(int(req.PageSize)).Offset(int(offset))
	}

	// 执行查询
	poList := make([]*model.Evaluator, 0)
	if err := query.Find(&poList).Error; err != nil {
		return nil, err
	}

	// 去掉 do 层转换，直接返回 poList
	return &ListEvaluatorResponse{
		Evaluators: poList,
		TotalCount: total,
	}, nil
}

func (dao *EvaluatorDAOImpl) UpdateEvaluatorLatestVersion(ctx context.Context, evaluatorID int64, version, userID string, opts ...db.Option) error {
	// 通过opts获取当前的db session实例
	dbsession := dao.provider.NewSession(ctx, opts...)

	return dbsession.WithContext(ctx).Model(&model.Evaluator{}).
		Where("id =?", evaluatorID). // 添加ID筛选条件
		Where("deleted_at IS NULL"). // 添加软删除筛选条件
		Updates(map[string]interface{}{
			"draft_submitted": true, // 提交后草稿已提交，更新为 true
			"latest_version":  version,
			"updated_by":      userID,
		}).Error
}
