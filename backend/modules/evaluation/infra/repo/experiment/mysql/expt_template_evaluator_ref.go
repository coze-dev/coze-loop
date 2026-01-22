// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/query"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

//go:generate mockgen -destination=mocks/expt_template_evaluator_ref.go -package=mocks . IExptTemplateEvaluatorRefDAO
type IExptTemplateEvaluatorRefDAO interface {
	Create(ctx context.Context, refs []*model.ExptTemplateEvaluatorRef) error
	GetByTemplateIDs(ctx context.Context, templateIDs []int64) ([]*model.ExptTemplateEvaluatorRef, error)
	GetByTemplateIDsIncludeDeleted(ctx context.Context, templateIDs []int64) ([]*model.ExptTemplateEvaluatorRef, error)
	DeleteByTemplateID(ctx context.Context, templateID int64) error
	SoftDeleteByIDs(ctx context.Context, ids []int64) error
	RestoreByIDs(ctx context.Context, ids []int64) error
}

func NewExptTemplateEvaluatorRefDAO(db db.Provider) IExptTemplateEvaluatorRefDAO {
	return &exptTemplateEvaluatorRefDAOImpl{
		db:    db,
		query: query.Use(db.NewSession(context.Background())),
	}
}

type exptTemplateEvaluatorRefDAOImpl struct {
	db    db.Provider
	query *query.Query
}

func (d *exptTemplateEvaluatorRefDAOImpl) Create(ctx context.Context, refs []*model.ExptTemplateEvaluatorRef) error {
	if len(refs) == 0 {
		return nil
	}
	if err := d.db.NewSession(ctx).Create(refs).Error; err != nil {
		return errorx.Wrapf(err, "create expt_template_evaluator_ref fail, refs: %v", json.Jsonify(refs))
	}
	return nil
}

func (d *exptTemplateEvaluatorRefDAOImpl) GetByTemplateIDs(ctx context.Context, templateIDs []int64) ([]*model.ExptTemplateEvaluatorRef, error) {
	if len(templateIDs) == 0 {
		return nil, nil
	}
	ref := d.query.ExptTemplateEvaluatorRef
	q := ref.WithContext(ctx)
	// 如果 context 中有写标志，使用主库
	if contexts.CtxWriteDB(ctx) {
		q = q.WriteDB()
	}
	results, err := q.Where(ref.ExptTemplateID.In(templateIDs...)).Find()
	if err != nil {
		return nil, errorx.Wrapf(err, "get expt_template_evaluator_ref by template_ids fail, template_ids: %v", templateIDs)
	}
	return results, nil
}

func (d *exptTemplateEvaluatorRefDAOImpl) DeleteByTemplateID(ctx context.Context, templateID int64) error {
	q := query.Use(d.db.NewSession(ctx)).ExptTemplateEvaluatorRef
	_, err := q.WithContext(ctx).Where(q.ExptTemplateID.Eq(templateID)).Delete()
	if err != nil {
		return errorx.Wrapf(err, "delete expt_template_evaluator_ref by template_id fail, template_id: %v", templateID)
	}
	return nil
}

func (d *exptTemplateEvaluatorRefDAOImpl) GetByTemplateIDsIncludeDeleted(ctx context.Context, templateIDs []int64) ([]*model.ExptTemplateEvaluatorRef, error) {
	if len(templateIDs) == 0 {
		return nil, nil
	}
	q := query.Use(d.db.NewSession(ctx)).ExptTemplateEvaluatorRef
	results, err := q.WithContext(ctx).Unscoped().Where(q.ExptTemplateID.In(templateIDs...)).Find()
	if err != nil {
		return nil, errorx.Wrapf(err, "get expt_template_evaluator_ref by template_ids (include deleted) fail, template_ids: %v", templateIDs)
	}
	return results, nil
}

func (d *exptTemplateEvaluatorRefDAOImpl) SoftDeleteByIDs(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	q := query.Use(d.db.NewSession(ctx)).ExptTemplateEvaluatorRef
	_, err := q.WithContext(ctx).Where(q.ID.In(ids...)).Delete()
	if err != nil {
		return errorx.Wrapf(err, "soft delete expt_template_evaluator_ref by ids fail, ids: %v", ids)
	}
	return nil
}

func (d *exptTemplateEvaluatorRefDAOImpl) RestoreByIDs(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	q := query.Use(d.db.NewSession(ctx)).ExptTemplateEvaluatorRef
	_, err := q.WithContext(ctx).Unscoped().Where(q.ID.In(ids...)).Update(q.DeletedAt, nil)
	if err != nil {
		return errorx.Wrapf(err, "restore expt_template_evaluator_ref by ids fail, ids: %v", ids)
	}
	return nil
}
