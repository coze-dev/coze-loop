// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/model"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/infra/repo/experiment/mysql/gorm_gen/query"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/json"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

//go:generate  mockgen -destination=mocks/expt_run_log.go  -package mocks . IExptRunLogDAO
type IExptRunLogDAO interface {
	Create(ctx context.Context, exptRunLog *model.ExptRunLog, opts ...db.Option) error
	Save(ctx context.Context, exptRunLog *model.ExptRunLog, opts ...db.Option) error
	Update(ctx context.Context, exptID, exptRunID int64, ufields map[string]any, opts ...db.Option) error
	Get(ctx context.Context, exptID, exptRunID int64, opts ...db.Option) (*model.ExptRunLog, error)
	// ListActive returns Processing runs ordered by id ascending. It is the
	// active-maintenance scan and is separate from ListPending.
	ListActive(ctx context.Context, cursor, limit int64, opts ...db.Option) ([]*model.ExptRunLog, error)
	// ListPending returns Pending runs for the separate initialization phase.
	ListPending(ctx context.Context, cursor, limit int64, opts ...db.Option) ([]*model.ExptRunLog, error)
}

type ExptRunLogDAOImpl struct {
	provider db.Provider
}

func NewExptRunLogDAO(db db.Provider) IExptRunLogDAO {
	return &ExptRunLogDAOImpl{
		provider: db,
	}
}

func (dao *ExptRunLogDAOImpl) Get(ctx context.Context, exptID, exptRunID int64, opts ...db.Option) (*model.ExptRunLog, error) {
	var exptRunLog model.ExptRunLog
	db := dao.provider.NewSession(ctx, opts...)
	if err := db.WithContext(ctx).Where("id = ?", exptRunID).First(&exptRunLog).Error; err != nil {
		return nil, errorx.Wrapf(err, "mget expt fail, expt_id: %v, expt_run_id: %v", exptID, exptRunID)
	}
	return &exptRunLog, nil
}

func (dao *ExptRunLogDAOImpl) ListActive(ctx context.Context, cursor, limit int64, opts ...db.Option) ([]*model.ExptRunLog, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	session := dao.provider.NewSession(ctx, opts...)
	q := query.Use(session).ExptRunLog
	runs, err := q.WithContext(ctx).
		Where(q.ID.Gt(cursor)).
		// entity.ExptStatus_Processing(3). Pending runs are handled by the
		// separate ListPending scan so initialization and maintenance never share
		// a cursor or a budget phase.
		Where(q.Status.Eq(int64(3))).
		Order(q.ID).
		Limit(int(limit)).
		Find()
	if err != nil {
		return nil, errorx.Wrapf(err, "list active expt run logs fail, cursor: %v", cursor)
	}
	return runs, nil
}

func (dao *ExptRunLogDAOImpl) ListPending(ctx context.Context, cursor, limit int64, opts ...db.Option) ([]*model.ExptRunLog, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	session := dao.provider.NewSession(ctx, opts...)
	q := query.Use(session).ExptRunLog
	runs, err := q.WithContext(ctx).
		Where(q.ID.Gt(cursor)).
		// entity.ExptStatus_Pending(2)
		Where(q.Status.Eq(int64(2))).
		Order(q.ID).
		Limit(int(limit)).
		Find()
	if err != nil {
		return nil, errorx.Wrapf(err, "list pending expt run logs fail, cursor: %v", cursor)
	}
	return runs, nil
}

func (dao *ExptRunLogDAOImpl) Create(ctx context.Context, exptRunLog *model.ExptRunLog, opts ...db.Option) error {
	db := dao.provider.NewSession(ctx, opts...)
	if err := db.WithContext(ctx).Create(exptRunLog).Error; err != nil {
		return errorx.Wrapf(err, "create expt_run_log fail, model: %v", json.Jsonify(exptRunLog))
	}
	return nil
}

func (dao *ExptRunLogDAOImpl) Save(ctx context.Context, exptRunLog *model.ExptRunLog, opts ...db.Option) error {
	db := dao.provider.NewSession(ctx, opts...)
	if err := db.WithContext(ctx).Save(exptRunLog).Error; err != nil {
		return errorx.Wrapf(err, "save expt_run_log fail, model: %v", json.Jsonify(exptRunLog))
	}
	logs.CtxInfo(ctx, "save expt_run_log success, model: %v", json.Jsonify(exptRunLog))
	return nil
}

func (dao *ExptRunLogDAOImpl) Update(ctx context.Context, exptID, exptRunID int64, ufields map[string]any, opts ...db.Option) error {
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptRunLog
	_, err := q.WithContext(ctx).
		Where(q.ExptID.Eq(exptID)).
		Where(q.ExptRunID.Eq(exptRunID)).
		UpdateColumns(ufields)
	if err != nil {
		return errorx.Wrapf(err, "update expt_run_log fail, expt_id: %v, expt_run_id: %v, ufields: %v", exptID, exptRunID, ufields)
	}
	logs.CtxInfo(ctx, "update expt_run_log success, expt_id: %v, expt_run_id: %v, ufields: %v", exptID, exptRunID, ufields)
	return nil
}
