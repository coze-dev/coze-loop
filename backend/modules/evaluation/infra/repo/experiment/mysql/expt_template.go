// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/query"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

//go:generate mockgen -destination=mocks/expt_template.go -package=mocks . IExptTemplateDAO
type IExptTemplateDAO interface {
	Create(ctx context.Context, template *model.ExptTemplate) error
	GetByID(ctx context.Context, id int64) (*model.ExptTemplate, error)
	GetByName(ctx context.Context, name string, spaceID int64) (*model.ExptTemplate, error)
	MGetByID(ctx context.Context, ids []int64) ([]*model.ExptTemplate, error)
	Update(ctx context.Context, template *model.ExptTemplate) error
	UpdateFields(ctx context.Context, id int64, ufields map[string]any) error
	Delete(ctx context.Context, id int64) error
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

func (d *exptTemplateDAOImpl) MGetByID(ctx context.Context, ids []int64) ([]*model.ExptTemplate, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	q := query.Use(d.db.NewSession(ctx)).ExptTemplate
	results, err := q.WithContext(ctx).Where(q.ID.In(ids...)).Find()
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
