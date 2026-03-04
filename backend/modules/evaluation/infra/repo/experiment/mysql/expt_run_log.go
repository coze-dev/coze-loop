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
	// ListCompletedRunIDsByExptID 列出实验下已完成的 run id 列表
	ListCompletedRunIDsByExptID(ctx context.Context, spaceID, exptID int64, opts ...db.Option) ([]int64, error)
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

// 已完成状态：Success=11, Failed=12, Terminated=13, SystemTerminated=14
var completedExptStatuses = []int64{11, 12, 13, 14}

func (dao *ExptRunLogDAOImpl) ListCompletedRunIDsByExptID(ctx context.Context, spaceID, exptID int64, opts ...db.Option) ([]int64, error) {
	db := dao.provider.NewSession(ctx, opts...)
	q := query.Use(db).ExptRunLog
	pos, err := q.WithContext(ctx).
		Select(q.ExptRunID).
		Where(q.SpaceID.Eq(spaceID)).
		Where(q.ExptID.Eq(exptID)).
		Where(q.Status.In(completedExptStatuses...)).
		Find()
	if err != nil {
		return nil, errorx.Wrapf(err, "list completed run ids fail, space_id: %v, expt_id: %v", spaceID, exptID)
	}
	ids := make([]int64, 0, len(pos))
	for _, p := range pos {
		ids = append(ids, p.ExptRunID)
	}
	return ids, nil
}
