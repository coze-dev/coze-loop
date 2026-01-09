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
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/slices"
)

//go:generate mockgen -destination=mocks/expt.go -package=mocks . IExptDAO
type IExptDAO interface {
	Create(ctx context.Context, expt *model.Experiment) error

	Update(ctx context.Context, expt *model.Experiment) error

	UpdateFields(ctx context.Context, id int64, ufields map[string]any) error

	Delete(ctx context.Context, id int64) error

	MDelete(ctx context.Context, ids []int64) error

	List(ctx context.Context, page, size int32, filter *entity.ExptListFilter, orders []*entity.OrderBy, spaceID int64) ([]*model.Experiment, int64, error)

	GetByName(ctx context.Context, name string, spaceID int64) (*model.Experiment, error)

	GetByID(ctx context.Context, id int64) (*model.Experiment, error)

	MGetByID(ctx context.Context, ids []int64) ([]*model.Experiment, error)
}

func NewExptDAO(db db.Provider) IExptDAO {
	return &exptDAOImpl{
		db:    db,
		query: query.Use(db.NewSession(context.Background())),
	}
}

const defaultLimit = 20

type exptDAOImpl struct {
	db    db.Provider
	query *query.Query
}

func (d *exptDAOImpl) UpdateFields(ctx context.Context, id int64, ufields map[string]any) error {
	q := query.Use(d.db.NewSession(ctx)).Experiment
	_, err := q.WithContext(ctx).
		Where(q.ID.Eq(id)).
		UpdateColumns(ufields)
	if err != nil {
		return errorx.Wrapf(err, "update expt fail, expt_id: %v, ufields: %v", id, ufields)
	}
	return nil
}

func (d *exptDAOImpl) Create(ctx context.Context, expt *model.Experiment) error {
	if err := d.db.NewSession(ctx).Create(expt).Error; err != nil {
		return errorx.Wrapf(err, "create expt fail, model: %v", json.Jsonify(expt))
	}
	return nil
}

func (d *exptDAOImpl) Update(ctx context.Context, expt *model.Experiment) error {
	if err := d.db.NewSession(ctx).Model(&model.Experiment{}).Where("id = ?", expt.ID).Updates(expt).Error; err != nil {
		return errorx.Wrapf(err, "update expt fail, expt_id: %v, updated: %v", expt.ID, json.Jsonify(expt))
	}
	return nil
}

func (d *exptDAOImpl) Delete(ctx context.Context, id int64) error {
	if err := d.db.NewSession(ctx).Delete(&model.Experiment{}, id).Error; err != nil {
		return errorx.Wrapf(err, "delete expt fail, expt_id: %v", id)
	}
	return nil
}

func (d *exptDAOImpl) MDelete(ctx context.Context, ids []int64) error {
	if err := d.db.NewSession(ctx).Delete(slices.Transform(ids, func(e int64, _ int) *model.Experiment {
		return &model.Experiment{ID: e}
	})).Error; err != nil {
		return errorx.Wrapf(err, "delete expts fail, expt_ids: %v", ids)
	}
	return nil
}

func (d *exptDAOImpl) List(ctx context.Context, page, size int32, filter *entity.ExptListFilter, orders []*entity.OrderBy, spaceID int64) ([]*model.Experiment, int64, error) {
	var (
		experiments []*model.Experiment
		db          = d.db.NewSession(ctx)
		count       int64
	)

	if d.filterNeedJoin(filter) {
		db = db.Model(&model.Experiment{}).
			Joins("INNER JOIN expt_evaluator_ref ON experiment.id = expt_evaluator_ref.expt_id").
			Where("experiment.space_id = ?", spaceID)
	} else {
		db = db.Model(&model.Experiment{}).Where("space_id = ?", spaceID)
	}

	conds, ok := d.toConditions(filter, orders)
	if !ok {
		return experiments, 0, nil
	}

	for _, cond := range conds {
		db = cond(db)
	}

	db.Group("experiment.id")
	db.Count(&count)

	if page > 0 && size > 0 {
		db = db.Limit(int(size)).Offset(int((page - 1) * size))
	} else {
		db = db.Limit(defaultLimit)
	}

	if err := db.Find(&experiments).Error; err != nil {
		return nil, 0, errorx.Wrapf(err, "pull expt fail, space_id: %v, page: %v, size: %v, filter: %v", spaceID, page, size, json.Jsonify(filter))
	}

	return experiments, count, nil
}

func (d *exptDAOImpl) filterNeedJoin(f *entity.ExptListFilter) bool {
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

func (d *exptDAOImpl) toConditions(f *entity.ExptListFilter, orders []*entity.OrderBy) ([]func(tx *gorm.DB) *gorm.DB, bool) {
	if f == nil && len(orders) == 0 {
		return nil, true
	}

	if f != nil && !f.Includes.IsValid() {
		return nil, false
	}

	var (
		exptPrefix = ""
		eefPrefix  = model.TableNameExptEvaluatorRef + "."
		conditions []func(tx *gorm.DB) *gorm.DB
	)

	if d.filterNeedJoin(f) {
		exptPrefix = model.TableNameExperiment + "."
	}

	condFn := func(comparator, scopeComparator string, ffields *entity.ExptFilterFields) []func(tx *gorm.DB) *gorm.DB {
		var conds []func(tx *gorm.DB) *gorm.DB

		if ffields == nil {
			return conds
		}

		if ffields != nil && len(ffields.CreatedBy) > 0 {
			conds = append(conds, func(db *gorm.DB) *gorm.DB {
				return db.Where(fmt.Sprintf("%screated_by %s (?)", exptPrefix, scopeComparator), ffields.CreatedBy)
			})
		}
		if ffields != nil && len(ffields.UpdatedBy) > 0 {
			conds = append(conds, func(db *gorm.DB) *gorm.DB {
				return db.Where(fmt.Sprintf("%supdated_by %s (?)", exptPrefix, scopeComparator), ffields.UpdatedBy)
			})
		}
		if ffields != nil && len(ffields.TargetIDs) > 0 {
			conds = append(conds, func(db *gorm.DB) *gorm.DB {
				return db.Where(fmt.Sprintf("%starget_id %s (?)", exptPrefix, scopeComparator), ffields.TargetIDs)
			})
		}
		if ffields != nil && len(ffields.EvalSetIDs) > 0 {
			conds = append(conds, func(db *gorm.DB) *gorm.DB {
				return db.Where(fmt.Sprintf("%seval_set_id %s (?)", exptPrefix, scopeComparator), ffields.EvalSetIDs)
			})
		}
		if ffields != nil && len(ffields.Status) > 0 {
			conds = append(conds, func(db *gorm.DB) *gorm.DB {
				return db.Where(fmt.Sprintf("%sstatus %s (?)", exptPrefix, scopeComparator), ffields.Status)
			})
		}
		if ffields != nil && len(ffields.TargetType) > 0 {
			conds = append(conds, func(db *gorm.DB) *gorm.DB {
				return db.Where(fmt.Sprintf("%starget_type %s (?)", exptPrefix, scopeComparator), ffields.TargetType)
			})
		}
		if ffields != nil && len(ffields.EvaluatorIDs) > 0 {
			conditions = append(conditions, func(db *gorm.DB) *gorm.DB {
				return db.Where(fmt.Sprintf("%sevaluator_id %s (?)", eefPrefix, scopeComparator), ffields.EvaluatorIDs)
			})
		}
		if ffields != nil && len(ffields.SourceID) > 0 {
			conds = append(conds, func(db *gorm.DB) *gorm.DB {
				return db.Where(fmt.Sprintf("%ssource_id %s (?)", exptPrefix, scopeComparator), ffields.SourceID)
			})
		}
		if ffields != nil && len(ffields.ExptType) > 0 {
			conds = append(conds, func(db *gorm.DB) *gorm.DB {
				return db.Where(fmt.Sprintf("%sexpt_type %s (?)", exptPrefix, scopeComparator), ffields.ExptType)
			})
		}
		if ffields != nil && len(ffields.ExptTemplateIDs) > 0 {
			conds = append(conds, func(db *gorm.DB) *gorm.DB {
				return db.Where(fmt.Sprintf("%sexpt_template_id %s (?)", exptPrefix, scopeComparator), ffields.ExptTemplateIDs)
			})
		}
		if ffields != nil && len(ffields.SourceType) > 0 {
			conds = append(conds, func(db *gorm.DB) *gorm.DB {
				return db.Where(fmt.Sprintf("%ssource_type %s (?)", exptPrefix, scopeComparator), ffields.SourceType)
			})
		}

		return conds
	}

	if f != nil && len(f.FuzzyName) > 0 {
		conditions = append(conditions, func(db *gorm.DB) *gorm.DB {
			return db.Where(exptPrefix+"name like ?", "%"+f.FuzzyName+"%")
		})
	}

	conditions = append(conditions, condFn("=", "IN", f.Includes)...)
	conditions = append(conditions, condFn("!=", "NOT IN", f.Excludes)...)

	ordered := false
	for _, orderBy := range orders {
		column := sortFieldToColumn[gptr.Indirect(orderBy.Field)]
		if len(column) == 0 {
			continue
		}

		ordered = true
		conditions = append(conditions, func(db *gorm.DB) *gorm.DB {
			sort := consts.SortDesc
			if gptr.Indirect(orderBy.IsAsc) {
				sort = consts.SortAsc
			}
			return db.Order(exptPrefix + column + " " + sort)
		})
	}

	if !ordered {
		conditions = append(conditions, func(db *gorm.DB) *gorm.DB {
			return db.Order(exptPrefix + "start_at desc")
		})
	}

	return conditions, true
}

var sortFieldToColumn = map[string]string{
	"start_time": "start_at",
	"end_time":   "end_at",
}

func (d *exptDAOImpl) GetByName(ctx context.Context, name string, spaceID int64) (*model.Experiment, error) {
	expt := d.query.Experiment
	found, err := expt.WithContext(ctx).
		Where(expt.SpaceID.Eq(spaceID)).
		Where(expt.Name.Eq(name)).
		First()
	if err != nil {
		return nil, errorx.Wrapf(err, "get expt with name %s fail", name)
	}
	return found, nil
}

func (d *exptDAOImpl) GetByID(ctx context.Context, id int64) (*model.Experiment, error) {
	expt := d.query.Experiment
	q := expt.WithContext(ctx)
	if contexts.CtxWriteDB(ctx) {
		q = q.WriteDB()
	}

	experiment, err := q.Where(
		expt.ID.Eq(id),
	).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.WrapByCode(err, errno.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("experiment %d not found", id)))
		}
		return nil, errorx.Wrapf(err, "mysql get expt fail, expt_ids: %v", id)
	}
	return experiment, nil
}

func (d *exptDAOImpl) MGetByID(ctx context.Context, ids []int64) ([]*model.Experiment, error) {
	expt := d.query.Experiment
	q := expt.WithContext(ctx)
	if contexts.CtxWriteDB(ctx) {
		q = q.WriteDB()
	}

	experiments, err := q.Where(
		expt.ID.In(ids...),
	).Find()
	if err != nil {
		return nil, errorx.Wrapf(err, "mysql mget expt fail, expt_ids: %v", ids)
	}

	experimentMap := make(map[int64]*model.Experiment)
	for _, experiment := range experiments {
		experimentMap[experiment.ID] = experiment
	}

	sortedExperiments := make([]*model.Experiment, 0, len(ids))
	for _, id := range ids {
		if experiment, exists := experimentMap[id]; exists {
			sortedExperiments = append(sortedExperiments, experiment)
		}
	}

	return sortedExperiments, nil
}
