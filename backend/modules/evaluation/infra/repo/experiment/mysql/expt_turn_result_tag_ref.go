// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/plugin/dbresolver"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/query"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/contexts"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
)

//go:generate mockgen -destination=mocks/expt_turn_result_tag_ref.go -package=mocks . IExptTurnResultTagRefDAO
type IExptTurnResultTagRefDAO interface {
	Create(ctx context.Context, refs []*model.ExptTurnResultTagRef) error
	UpdateCompleteCount(ctx context.Context, exptID, spaceID, tagKeyID int64, opts ...db.Option) (int32, int32, error)
	Delete(ctx context.Context, exptID int64, spaceID int64, tagKeyID int64, opts ...db.Option) error

	GetByExptID(ctx context.Context, exptID int64, spaceID int64) ([]*model.ExptTurnResultTagRef, error)
	BatchGetByExptIDs(ctx context.Context, exptIDs []int64, spaceID int64) ([]*model.ExptTurnResultTagRef, error)
	GetByTagKeyID(ctx context.Context, exptID int64, spaceID int64, tagKeyID int64) (*model.ExptTurnResultTagRef, error)
}

func NewExptTurnResultTagRefDAO(db db.Provider) IExptTurnResultTagRefDAO {
	return &exptTurnResultTagRefDAO{
		db:    db,
		query: query.Use(db.NewSession(context.Background())),
	}
}

type exptTurnResultTagRefDAO struct {
	db    db.Provider
	query *query.Query
}

func (e exptTurnResultTagRefDAO) GetByTagKeyID(ctx context.Context, exptID int64, spaceID int64, tagKeyID int64) (*model.ExptTurnResultTagRef, error) {
	db := e.db.NewSession(ctx)
	if contexts.CtxWriteDB(ctx) {
		db = db.Clauses(dbresolver.Write)
	}
	q := query.Use(db).ExptTurnResultTagRef

	found, err := q.WithContext(ctx).Where(q.SpaceID.Eq(spaceID)).Where(q.ExptID.Eq(exptID)).Where(q.TagKeyID.Eq(tagKeyID)).First()
	if err != nil {
		return nil, errorx.Wrapf(err, "GetByTagKeyID ExptTurnResultTagRef fail, expt_id: %v", exptID)
	}
	return found, nil
}

func (e exptTurnResultTagRefDAO) UpdateCompleteCount(ctx context.Context, exptID, spaceID, tagKeyID int64, opts ...db.Option) (int32, int32, error) {
	po := &model.ExptTurnResultTagRef{}
	db := e.db.NewSession(ctx, opts...)
	err := db.Model(po).
		Clauses(clause.Returning{Columns: []clause.Column{{Name: "complete_cnt"}, {Name: "total_cnt"}}}).
		Where("space_id = ? AND expt_id = ?  AND tag_key_id = ?", spaceID, exptID, tagKeyID).
		Where("complete_cnt < total_cnt").
		Updates(map[string]interface{}{
			"complete_cnt": gorm.Expr("complete_cnt + ?", 1),
		}).Error
	if err != nil {
		return 0, 0, err
	}

	return po.CompleteCnt, po.TotalCnt, nil
}

func (e exptTurnResultTagRefDAO) Create(ctx context.Context, refs []*model.ExptTurnResultTagRef) error {
	if len(refs) == 0 {
		return nil
	}
	if err := e.db.NewSession(ctx).Create(refs).Error; err != nil {
		return errorx.Wrapf(err, "create ExptTurnResultTagRef fail, models: %v", json.Jsonify(refs))
	}
	return nil
}

func (e exptTurnResultTagRefDAO) Delete(ctx context.Context, exptID int64, spaceID int64, tagKeyID int64, opts ...db.Option) error {
	// 硬删除 可能删除后再关联
	po := &model.ExptTurnResultTagRef{}
	db := e.db.NewSession(ctx, opts...)
	err := db.Unscoped().Where("space_id = ? AND expt_id = ?  AND tag_key_id = ?", spaceID, exptID, tagKeyID).
		Delete(po).Error
	if err != nil {
		return err
	}

	return nil
}

func (e exptTurnResultTagRefDAO) GetByExptID(ctx context.Context, exptID int64, spaceID int64) ([]*model.ExptTurnResultTagRef, error) {
	ref := e.query.ExptTurnResultTagRef
	query := ref.WithContext(ctx)

	found, err := query.Where(ref.SpaceID.Eq(spaceID)).Where(ref.ExptID.Eq(exptID)).Order(ref.CreatedAt.Asc()).Find()
	if err != nil {
		return nil, errorx.Wrapf(err, "GetByExptID ExptTurnResultTagRef fail, expt_id: %v", exptID)
	}
	return found, nil
}

func (e exptTurnResultTagRefDAO) BatchGetByExptIDs(ctx context.Context, exptIDs []int64, spaceID int64) ([]*model.ExptTurnResultTagRef, error) {
	ref := e.query.ExptTurnResultTagRef
	query := ref.WithContext(ctx)

	found, err := query.Where(ref.SpaceID.Eq(spaceID)).Where(ref.ExptID.In(exptIDs...)).Find()
	if err != nil {
		return nil, errorx.Wrapf(err, "BatchGetByExptIDs ExptTurnResultTagRef fail, expt_id: %v", exptIDs)
	}
	return found, nil
}
