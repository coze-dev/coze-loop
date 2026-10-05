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

func (dao *ExptRunLogDAOImpl) Create(ctx context.Context, exptRunLog *model.ExptRunLog, opts ...db.Option) error {
	db := dao.provider.NewSession(ctx, opts...)
	if err := db.WithContext(ctx).Create(exptRunLog).Error; err != nil {
		return errorx.Wrapf(err, "create expt_run_log fail, model: %v", json.Jsonify(exptRunLog))
	}
	return nil
}

func (dao *ExptRunLogDAOImpl) Save(ctx context.Context, exptRunLog *model.ExptRunLog, opts ...db.Option) error {
	db := dao.provider.NewSession(ctx, opts...)
	if err := db.WithContext(ctx).Omit("lifecycle_hook_version").Save(exptRunLog).Error; err != nil {
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
		Omit(q.LifecycleHookVersion).
		UpdateColumns(ufields)
	if err != nil {
		return errorx.Wrapf(err, "update expt_run_log fail, expt_id: %v, expt_run_id: %v", exptID, exptRunID)
	}
	logs.CtxInfo(ctx, "update expt_run_log success, expt_id: %v, expt_run_id: %v", exptID, exptRunID)
	return nil
}
