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
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/evaluator/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
)

// EvaluatorTemplateDAO 定义 EvaluatorTemplate 的 Dao 接口
//
//go:generate mockgen -destination mocks/evaluator_template_mock.go -package=mocks . EvaluatorTemplateDAO
type EvaluatorTemplateDAO interface {
	// CreateEvaluatorTemplate 创建评估器模板
	CreateEvaluatorTemplate(ctx context.Context, template *model.EvaluatorTemplate, opts ...db.Option) (*model.EvaluatorTemplate, error)

	// UpdateEvaluatorTemplate 更新评估器模板
	UpdateEvaluatorTemplate(ctx context.Context, template *model.EvaluatorTemplate, opts ...db.Option) (*model.EvaluatorTemplate, error)

	// DeleteEvaluatorTemplate 删除评估器模板（软删除）
	DeleteEvaluatorTemplate(ctx context.Context, id int64, userID string, opts ...db.Option) error

	// GetEvaluatorTemplate 根据ID获取评估器模板
	GetEvaluatorTemplate(ctx context.Context, id int64, includeDeleted bool, opts ...db.Option) (*model.EvaluatorTemplate, error)

	// ListEvaluatorTemplate 根据筛选条件查询evaluator_template列表，支持tag筛选和分页
	ListEvaluatorTemplate(ctx context.Context, req *ListEvaluatorTemplateRequest, opts ...db.Option) (*ListEvaluatorTemplateResponse, error)

	// IncrPopularityByID 基于ID将 popularity + 1
	IncrPopularityByID(ctx context.Context, id int64, opts ...db.Option) error
}

var (
	evaluatorTemplateDaoOnce      = sync.Once{}
	singletonEvaluatorTemplateDao EvaluatorTemplateDAO
)

// EvaluatorTemplateDAOImpl 实现 EvaluatorTemplateDAO 接口
type EvaluatorTemplateDAOImpl struct {
	provider db.Provider
}

func NewEvaluatorTemplateDAO(p db.Provider) EvaluatorTemplateDAO {
	evaluatorTemplateDaoOnce.Do(func() {
		singletonEvaluatorTemplateDao = &EvaluatorTemplateDAOImpl{
			provider: p,
		}
	})
	return singletonEvaluatorTemplateDao
}

type ListEvaluatorTemplateRequest struct {
	IDs            []int64
	PageSize       int32
	PageNum        int32
	IncludeDeleted bool
}

type ListEvaluatorTemplateResponse struct {
	TotalCount int64
	Templates  []*model.EvaluatorTemplate
}

func (dao *EvaluatorTemplateDAOImpl) ListEvaluatorTemplate(ctx context.Context, req *ListEvaluatorTemplateRequest, opts ...db.Option) (*ListEvaluatorTemplateResponse, error) {
	// 通过opts获取当前的db session实例
	if contexts.CtxWriteDB(ctx) {
		opts = append(opts, db.WithMaster())
	}
	dbsession := dao.provider.NewSession(ctx, opts...)

	query := dbsession.WithContext(ctx).Model(&model.EvaluatorTemplate{})

	// 添加ID过滤（支持按ID查询）
	if len(req.IDs) > 0 {
		query = query.Where("id IN (?)", req.IDs)
	}

	// 软删除过滤
	if !req.IncludeDeleted {
		query = query.Where("deleted_at IS NULL")
	} else {
		query = query.Unscoped() // 解除软删除过滤
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
	poList := make([]*model.EvaluatorTemplate, 0)
	if err := query.Find(&poList).Error; err != nil {
		return nil, err
	}

	return &ListEvaluatorTemplateResponse{
		Templates:  poList,
		TotalCount: total,
	}, nil
}

// CreateEvaluatorTemplate 创建评估器模板
func (dao *EvaluatorTemplateDAOImpl) CreateEvaluatorTemplate(ctx context.Context, template *model.EvaluatorTemplate, opts ...db.Option) (*model.EvaluatorTemplate, error) {
	if template == nil {
		return nil, errors.New("template cannot be nil")
	}

	// 通过opts获取当前的db session实例
	if contexts.CtxWriteDB(ctx) {
		opts = append(opts, db.WithMaster())
	}
	dbsession := dao.provider.NewSession(ctx, opts...)

	// 设置创建时间
	now := time.Now()
	template.CreatedAt = now
	template.UpdatedAt = now

	// 执行创建
	if err := dbsession.WithContext(ctx).Create(template).Error; err != nil {
		return nil, err
	}

	return template, nil
}

// UpdateEvaluatorTemplate 更新评估器模板
func (dao *EvaluatorTemplateDAOImpl) UpdateEvaluatorTemplate(ctx context.Context, template *model.EvaluatorTemplate, opts ...db.Option) (*model.EvaluatorTemplate, error) {
	if template == nil {
		return nil, errors.New("template cannot be nil")
	}

	// 通过opts获取当前的db session实例
	if contexts.CtxWriteDB(ctx) {
		opts = append(opts, db.WithMaster())
	}
	dbsession := dao.provider.NewSession(ctx, opts...)

	// 设置更新时间
	template.UpdatedAt = time.Now()

	// 执行更新
	if err := dbsession.WithContext(ctx).Save(template).Error; err != nil {
		return nil, err
	}

	return template, nil
}

// DeleteEvaluatorTemplate 删除评估器模板（软删除）
func (dao *EvaluatorTemplateDAOImpl) DeleteEvaluatorTemplate(ctx context.Context, id int64, userID string, opts ...db.Option) error {
	// 通过opts获取当前的db session实例
	if contexts.CtxWriteDB(ctx) {
		opts = append(opts, db.WithMaster())
	}
	dbsession := dao.provider.NewSession(ctx, opts...)

	// 执行软删除
	if err := dbsession.WithContext(ctx).Model(&model.EvaluatorTemplate{}).
		Where("id = ?", id).
		Update("deleted_at", time.Now()).
		Update("updated_by", userID).
		Update("updated_at", time.Now()).Error; err != nil {
		return err
	}

	return nil
}

// GetEvaluatorTemplate 根据ID获取评估器模板
func (dao *EvaluatorTemplateDAOImpl) GetEvaluatorTemplate(ctx context.Context, id int64, includeDeleted bool, opts ...db.Option) (*model.EvaluatorTemplate, error) {
	// 通过opts获取当前的db session实例
	if contexts.CtxWriteDB(ctx) {
		opts = append(opts, db.WithMaster())
	}
	dbsession := dao.provider.NewSession(ctx, opts...)

	query := dbsession.WithContext(ctx).Model(&model.EvaluatorTemplate{}).Where("id = ?", id)

	// 软删除过滤
	if !includeDeleted {
		query = query.Where("deleted_at IS NULL")
	} else {
		query = query.Unscoped() // 解除软删除过滤
	}

	var template model.EvaluatorTemplate
	if err := query.First(&template).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &template, nil
}

// IncrPopularityByID 基于ID将 popularity + 1
func (dao *EvaluatorTemplateDAOImpl) IncrPopularityByID(ctx context.Context, id int64, opts ...db.Option) error {
	if contexts.CtxWriteDB(ctx) {
		opts = append(opts, db.WithMaster())
	}
	dbsession := dao.provider.NewSession(ctx, opts...)
	return dbsession.WithContext(ctx).
		Model(&model.EvaluatorTemplate{}).
		Where("id = ?", id).
		Where("deleted_at IS NULL").
		UpdateColumn("popularity", gorm.Expr("popularity + 1")).
		Error
}
