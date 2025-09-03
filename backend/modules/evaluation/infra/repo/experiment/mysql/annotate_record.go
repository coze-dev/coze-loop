// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"

	"gorm.io/plugin/dbresolver"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/query"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

//go:generate mockgen -destination=mocks/annotate_record.go -package=mocks . IAnnotateRecordDAO
type IAnnotateRecordDAO interface {
	Save(ctx context.Context, annotateRecord *model.AnnotateRecord, opts ...db.Option) error
	BatchSave(ctx context.Context, annotateRecord []*model.AnnotateRecord, opts ...db.Option) error
	Update(ctx context.Context, annotateRecord *model.AnnotateRecord, opts ...db.Option) error
	MGetByID(ctx context.Context, ids []int64) ([]*model.AnnotateRecord, error)
}

func NewAnnotateRecordDAO(db db.Provider) IAnnotateRecordDAO {
	return &annotateRecordDAO{
		db:    db,
		query: query.Use(db.NewSession(context.Background())),
	}
}

type annotateRecordDAO struct {
	db    db.Provider
	query *query.Query
}

func (a annotateRecordDAO) BatchSave(ctx context.Context, annotateRecords []*model.AnnotateRecord, opts ...db.Option) error {
	if len(annotateRecords) == 0 {
		return nil
	}
	if err := a.db.NewSession(ctx, opts...).Save(annotateRecords).Error; err != nil {
		return errorx.Wrapf(err, "create expt fail, model: %v", json.Jsonify(annotateRecords))
	}
	return nil
}

func (a annotateRecordDAO) Save(ctx context.Context, annotateRecord *model.AnnotateRecord, opts ...db.Option) error {
	if err := a.db.NewSession(ctx, opts...).Save(annotateRecord).Error; err != nil {
		return errorx.Wrapf(err, "create expt fail, model: %v", json.Jsonify(annotateRecord))
	}
	return nil
}

func (a annotateRecordDAO) Update(ctx context.Context, annotateRecord *model.AnnotateRecord, opts ...db.Option) error {
	if err := a.db.NewSession(ctx, opts...).Model(&model.AnnotateRecord{}).Where("id = ?", annotateRecord.ID).Updates(annotateRecord).Error; err != nil {
		return errorx.Wrapf(err, "create expt fail, model: %v", json.Jsonify(annotateRecord))
	}
	return nil
}

func (a annotateRecordDAO) MGetByID(ctx context.Context, ids []int64) ([]*model.AnnotateRecord, error) {
	db := a.db.NewSession(ctx)
	if contexts.CtxWriteDB(ctx) {
		db = db.Clauses(dbresolver.Write)
	}
	q := query.Use(db).AnnotateRecord

	annotateRecords, err := q.WithContext(ctx).Where(
		q.ID.In(ids...),
	).Find()
	if err != nil {
		return nil, errorx.Wrapf(err, "mysql mget expt fail, expt_ids: %v", ids)
	}

	return annotateRecords, nil
}
