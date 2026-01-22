// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"
	"fmt"

	"github.com/bytedance/gg/gptr"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/consts"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/query"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

//go:generate mockgen -destination=mocks/expt_template.go -package=mocks . IExptTemplateDAO
type IExptTemplateDAO interface {
	Create(ctx context.Context, template *model.ExptTemplate) error
	GetByID(ctx context.Context, id int64) (*model.ExptTemplate, error)
	GetByName(ctx context.Context, name string, spaceID int64) (*model.ExptTemplate, error)
	MGetByID(ctx context.Context, ids []int64, opts ...db.Option) ([]*model.ExptTemplate, error)
	Update(ctx context.Context, template *model.ExptTemplate) error
	UpdateFields(ctx context.Context, id int64, ufields map[string]any) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context, page, size int32, filter *entity.ExptTemplateListFilter, orders []*entity.OrderBy, spaceID int64) ([]*model.ExptTemplate, int64, error)
}

func NewExptTemplateDAO(db db.Provider) IExptTemplateDAO {
	return &exptTemplateDAOImpl{
		db:    db,
		query: query.Use(db.NewSession(context.Background())),
	}
}

type exptTemplateDAOImpl struct {
	db    db.Provider
	query *query.Query
}

func (d *exptTemplateDAOImpl) Create(ctx context.Context, template *model.ExptTemplate) error {
	if err := d.db.NewSession(ctx).Create(template).Error; err != nil {
		return errorx.Wrapf(err, "create expt_template fail, model: %v", json.Jsonify(template))
	}
	return nil
}

func (d *exptTemplateDAOImpl) GetByID(ctx context.Context, id int64) (*model.ExptTemplate, error) {
	q := query.Use(d.db.NewSession(ctx)).ExptTemplate
	result, err := q.WithContext(ctx).Where(q.ID.Eq(id)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, errorx.Wrapf(err, "get expt_template fail, id: %v", id)
	}
	return result, nil
}

func (d *exptTemplateDAOImpl) GetByName(ctx context.Context, name string, spaceID int64) (*model.ExptTemplate, error) {
	q := query.Use(d.db.NewSession(ctx)).ExptTemplate
	result, err := q.WithContext(ctx).
		Where(q.SpaceID.Eq(spaceID), q.Name.Eq(name)).
		First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, errorx.Wrapf(err, "get expt_template by name fail, name: %v, space_id: %v", name, spaceID)
	}
	return result, nil
}

func (d *exptTemplateDAOImpl) MGetByID(ctx context.Context, ids []int64, opts ...db.Option) ([]*model.ExptTemplate, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	template := d.query.ExptTemplate
	q := template.WithContext(ctx)
	// 如果 context 中有写标志，使用主库
	if contexts.CtxWriteDB(ctx) {
		q = q.WriteDB()
	}
	results, err := q.Where(template.ID.In(ids...)).Find()
	if err != nil {
		return nil, errorx.Wrapf(err, "mget expt_template fail, ids: %v", ids)
	}
	return results, nil
}

func (d *exptTemplateDAOImpl) Update(ctx context.Context, template *model.ExptTemplate) error {
	if err := d.db.NewSession(ctx).Model(&model.ExptTemplate{}).Where("id = ?", template.ID).Updates(template).Error; err != nil {
		return errorx.Wrapf(err, "update expt_template fail, template_id: %v", template.ID)
	}
	return nil
}

func (d *exptTemplateDAOImpl) UpdateFields(ctx context.Context, id int64, ufields map[string]any) error {
	q := query.Use(d.db.NewSession(ctx)).ExptTemplate
	_, err := q.WithContext(ctx).
		Where(q.ID.Eq(id)).
		UpdateColumns(ufields)
	if err != nil {
		return errorx.Wrapf(err, "update expt_template fields fail, template_id: %v, ufields: %v", id, ufields)
	}
	return nil
}

func (d *exptTemplateDAOImpl) Delete(ctx context.Context, id int64) error {
	if err := d.db.NewSession(ctx).Delete(&model.ExptTemplate{}, id).Error; err != nil {
		return errorx.Wrapf(err, "delete expt_template fail, template_id: %v", id)
	}
	return nil
}

func (d *exptTemplateDAOImpl) List(ctx context.Context, page, size int32, filter *entity.ExptTemplateListFilter, orders []*entity.OrderBy, spaceID int64) ([]*model.ExptTemplate, int64, error) {
	var (
		templates []*model.ExptTemplate
		db        = d.db.NewSession(ctx)
		count     int64
		needJoin  = d.filterNeedJoin(filter)
	)

	if needJoin {
		db = db.Model(&model.ExptTemplate{}).
			Joins("INNER JOIN expt_template_evaluator_ref ON expt_template.id = expt_template_evaluator_ref.expt_template_id").
			Where("expt_template.space_id = ?", spaceID).
			Where("expt_template.deleted_at IS NULL")
	} else {
		db = db.Model(&model.ExptTemplate{}).
			Where("space_id = ?", spaceID).
			Where("deleted_at IS NULL")
	}

	conds, ok := d.toConditions(filter, orders)
	if !ok {
		return templates, 0, nil
	}

	for _, cond := range conds {
		db = cond(db)
	}

	// 只有在需要 join 时才使用 Group，避免重复数据
	if needJoin {
		db = db.Group("expt_template.id")
	}
	db.Count(&count)

	if page > 0 && size > 0 {
		db = db.Limit(int(size)).Offset(int((page - 1) * size))
	} else {
		db = db.Limit(defaultLimit)
	}

	if err := db.Find(&templates).Error; err != nil {
		return nil, 0, errorx.Wrapf(err, "list expt_template fail, space_id: %v, page: %v, size: %v, filter: %v", spaceID, page, size, json.Jsonify(filter))
	}

	return templates, count, nil
}

func (d *exptTemplateDAOImpl) filterNeedJoin(f *entity.ExptTemplateListFilter) bool {
	if f != nil {
		if f.Includes != nil && len(f.Includes.EvaluatorIDs) > 0 {
			return true
		}
		if f.Excludes != nil && len(f.Excludes.EvaluatorIDs) > 0 {
			return true
		}
	}
	return false
}

func (d *exptTemplateDAOImpl) toConditions(f *entity.ExptTemplateListFilter, orders []*entity.OrderBy) ([]func(tx *gorm.DB) *gorm.DB, bool) {
	if f == nil && len(orders) == 0 {
		return nil, true
	}

	if f != nil && !f.Includes.IsValid() {
		return nil, false
	}

	var (
		templatePrefix = ""
		refPrefix      = model.TableNameExptTemplateEvaluatorRef + "."
		conditions     []func(tx *gorm.DB) *gorm.DB
	)

	if d.filterNeedJoin(f) {
		templatePrefix = model.TableNameExptTemplate + "."
	}

	condFn := func(comparator, scopeComparator string, ffields *entity.ExptTemplateFilterFields) []func(tx *gorm.DB) *gorm.DB {
		var conds []func(tx *gorm.DB) *gorm.DB

		if ffields == nil {
			return conds
		}

		if len(ffields.CreatedBy) > 0 {
			conds = append(conds, func(db *gorm.DB) *gorm.DB {
				return db.Where(fmt.Sprintf("%screated_by %s (?)", templatePrefix, scopeComparator), ffields.CreatedBy)
			})
		}
		if len(ffields.UpdatedBy) > 0 {
			conds = append(conds, func(db *gorm.DB) *gorm.DB {
				return db.Where(fmt.Sprintf("%supdated_by %s (?)", templatePrefix, scopeComparator), ffields.UpdatedBy)
			})
		}
		if len(ffields.TargetIDs) > 0 {
			conds = append(conds, func(db *gorm.DB) *gorm.DB {
				return db.Where(fmt.Sprintf("%starget_id %s (?)", templatePrefix, scopeComparator), ffields.TargetIDs)
			})
		}
		if len(ffields.EvalSetIDs) > 0 {
			conds = append(conds, func(db *gorm.DB) *gorm.DB {
				return db.Where(fmt.Sprintf("%seval_set_id %s (?)", templatePrefix, scopeComparator), ffields.EvalSetIDs)
			})
		}
		if len(ffields.TargetType) > 0 {
			conds = append(conds, func(db *gorm.DB) *gorm.DB {
				return db.Where(fmt.Sprintf("%starget_type %s (?)", templatePrefix, scopeComparator), ffields.TargetType)
			})
		}
		if len(ffields.EvaluatorIDs) > 0 {
			conds = append(conds, func(db *gorm.DB) *gorm.DB {
				return db.Where(fmt.Sprintf("%sevaluator_id %s (?)", refPrefix, scopeComparator), ffields.EvaluatorIDs)
			})
		}
		if len(ffields.ExptType) > 0 {
			conds = append(conds, func(db *gorm.DB) *gorm.DB {
				return db.Where(fmt.Sprintf("%sexpt_type %s (?)", templatePrefix, scopeComparator), ffields.ExptType)
			})
		}

		return conds
	}

	if f != nil && len(f.FuzzyName) > 0 {
		conditions = append(conditions, func(db *gorm.DB) *gorm.DB {
			return db.Where(templatePrefix+"name like ?", "%"+f.FuzzyName+"%")
		})
	}

	if f != nil {
		conditions = append(conditions, condFn("=", "IN", f.Includes)...)
		conditions = append(conditions, condFn("!=", "NOT IN", f.Excludes)...)
	}

	ordered := false
	for _, orderBy := range orders {
		column := gptr.Indirect(orderBy.Field)
		if len(column) == 0 {
			continue
		}

		ordered = true
		// 在闭包内部使用局部变量，避免闭包捕获问题
		col := column
		prefix := templatePrefix
		conditions = append(conditions, func(db *gorm.DB) *gorm.DB {
			sort := consts.SortDesc
			if gptr.Indirect(orderBy.IsAsc) {
				sort = consts.SortAsc
			}
			return db.Order(prefix + col + " " + sort)
		})
	}

	if !ordered {
		conditions = append(conditions, func(db *gorm.DB) *gorm.DB {
			return db.Order(templatePrefix + "created_at desc")
		})
	}

	return conditions, true
}
