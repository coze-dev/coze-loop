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
)

//go:generate  mockgen -destination=mocks/expt_turn_evaluator_result_ref.go  -package mocks . IExptTurnEvaluatorResultRefDAO
type IExptTurnEvaluatorResultRefDAO interface {
	BatchGet(ctx context.Context, spaceID int64, exptTurnResultIDs []int64, opts ...db.Option) ([]*model.ExptTurnEvaluatorResultRef, error)
	GetByExptID(ctx context.Context, spaceID, exptID int64, opts ...db.Option) ([]*model.ExptTurnEvaluatorResultRef, error)
	GetByExptEvaluatorVersionID(ctx context.Context, spaceID, exptID, evaluatorVersionID int64, opts ...db.Option) ([]*model.ExptTurnEvaluatorResultRef, error)
}

type ExptTurnEvaluatorResultRefDAOImpl struct {
	provider db.Provider
}

func NewExptTurnEvaluatorResultRefDAO(db db.Provider) IExptTurnEvaluatorResultRefDAO {
	return &ExptTurnEvaluatorResultRefDAOImpl{
		provider: db,
	}
}

func (dao *ExptTurnEvaluatorResultRefDAOImpl) GetByExptEvaluatorVersionID(ctx context.Context, spaceID, exptID, evaluatorVersionID int64, opts ...db.Option) ([]*model.ExptTurnEvaluatorResultRef, error) {
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptTurnEvaluatorResultRef
	ret, err := q.WithContext(ctx).Where(q.SpaceID.Eq(spaceID),
		q.ExptID.Eq(exptID),
		q.EvaluatorVersionID.Eq(evaluatorVersionID)).Find()
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (dao *ExptTurnEvaluatorResultRefDAOImpl) GetByExptID(ctx context.Context, spaceID, exptID int64, opts ...db.Option) ([]*model.ExptTurnEvaluatorResultRef, error) {
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptTurnEvaluatorResultRef
	ret, err := q.WithContext(ctx).Where(q.SpaceID.Eq(spaceID),
		q.ExptID.Eq(exptID)).Find()
	if err != nil {
		return nil, err
	}

	return ret, nil
}

func (dao *ExptTurnEvaluatorResultRefDAOImpl) BatchGet(ctx context.Context, spaceID int64, exptTurnResultIDs []int64, opts ...db.Option) ([]*model.ExptTurnEvaluatorResultRef, error) {
	db := dao.provider.NewSession(ctx, opts...)
	if contexts.CtxWriteDB(ctx) {
		db = db.Clauses(dbresolver.Write)
	}
	q := query.Use(db).ExptTurnEvaluatorResultRef
	ret, err := q.WithContext(ctx).Where(q.SpaceID.Eq(spaceID),
		q.ExptTurnResultID.In(exptTurnResultIDs...)).Find()
	if err != nil {
		return nil, err
	}

	return ret, nil
}
