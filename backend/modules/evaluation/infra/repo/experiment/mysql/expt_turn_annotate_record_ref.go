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

//go:generate mockgen -destination=mocks/expt_turn_annotate_record_ref.go -package=mocks . IExptTurnAnnotateRecordRefDAO
type IExptTurnAnnotateRecordRefDAO interface {
	Save(ctx context.Context, refs *model.ExptTurnAnnotateRecordRef, opts ...db.Option) error

	BatchGet(ctx context.Context, spaceID int64, exptTurnResultIDs []int64, opts ...db.Option) ([]*model.ExptTurnAnnotateRecordRef, error)
	GetByExptID(ctx context.Context, spaceID, exptID int64, opts ...db.Option) ([]*model.ExptTurnAnnotateRecordRef, error)
	BatchGetByExptIDs(ctx context.Context, spaceID int64, exptIDs []int64, opts ...db.Option) ([]*model.ExptTurnAnnotateRecordRef, error)
	GetByTagKeyID(ctx context.Context, spaceID, exptID, tagKeyID int64, opts ...db.Option) ([]*model.ExptTurnAnnotateRecordRef, error)
	DeleteByTagKeyID(ctx context.Context, spaceID, exptID, tagKeyID int64, opts ...db.Option) error
}

func NewExptTurnAnnotateRecordRefDAO(db db.Provider) IExptTurnAnnotateRecordRefDAO {
	return &exptTurnAnnotateRecordRefDAO{
		db:    db,
		query: query.Use(db.NewSession(context.Background())),
	}
}

type exptTurnAnnotateRecordRefDAO struct {
	db    db.Provider
	query *query.Query
}

func (e exptTurnAnnotateRecordRefDAO) BatchGetByExptIDs(ctx context.Context, spaceID int64, exptIDs []int64, opts ...db.Option) ([]*model.ExptTurnAnnotateRecordRef, error) {
	db := e.db.NewSession(ctx, opts...)
	q := query.Use(db).ExptTurnAnnotateRecordRef
	ret, err := q.WithContext(ctx).Where(q.SpaceID.Eq(spaceID),
		q.ExptID.In(exptIDs...),
		q.DeletedAt.IsNull()).Find()
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (e exptTurnAnnotateRecordRefDAO) BatchGet(ctx context.Context, spaceID int64, exptTurnResultIDs []int64, opts ...db.Option) ([]*model.ExptTurnAnnotateRecordRef, error) {
	db := e.db.NewSession(ctx, opts...)
	if contexts.CtxWriteDB(ctx) {
		db = db.Clauses(dbresolver.Write)
	}
	q := query.Use(db).ExptTurnAnnotateRecordRef
	ret, err := q.WithContext(ctx).Where(q.SpaceID.Eq(spaceID),
		q.ExptTurnResultID.In(exptTurnResultIDs...),
		q.DeletedAt.IsNull()).Find()
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (e exptTurnAnnotateRecordRefDAO) GetByExptID(ctx context.Context, spaceID, exptID int64, opts ...db.Option) ([]*model.ExptTurnAnnotateRecordRef, error) {
	db := e.db.NewSession(ctx, opts...)
	q := query.Use(db).ExptTurnAnnotateRecordRef
	ret, err := q.WithContext(ctx).Where(q.SpaceID.Eq(spaceID),
		q.ExptID.Eq(exptID),
		q.DeletedAt.IsNull()).Find()
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (e exptTurnAnnotateRecordRefDAO) GetByTagKeyID(ctx context.Context, spaceID, exptID, tagKeyID int64, opts ...db.Option) ([]*model.ExptTurnAnnotateRecordRef, error) {
	db := e.db.NewSession(ctx, opts...)
	q := query.Use(db).ExptTurnAnnotateRecordRef
	ret, err := q.WithContext(ctx).Where(q.SpaceID.Eq(spaceID),
		q.ExptID.Eq(exptID),
		q.TagKeyID.Eq(tagKeyID),
		q.DeletedAt.IsNull()).Find()
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (e exptTurnAnnotateRecordRefDAO) Save(ctx context.Context, ref *model.ExptTurnAnnotateRecordRef, opts ...db.Option) error {
	if err := e.db.NewSession(ctx, opts...).Save(ref).Error; err != nil {
		return errorx.Wrapf(err, "Save ExptTurnAnnotateRecordRef fail, model: %v", json.Jsonify(ref))
	}
	return nil
}

func (e exptTurnAnnotateRecordRefDAO) DeleteByTagKeyID(ctx context.Context, spaceID, exptID, tagKeyID int64, opts ...db.Option) error {
	po := &model.ExptTurnAnnotateRecordRef{}
	db := e.db.NewSession(ctx, opts...)
	err := db.Where("space_id = ? AND expt_id = ?  AND tag_key_id = ?", spaceID, exptID, tagKeyID).
		Delete(po).Error
	if err != nil {
		return err
	}

	return nil
}
