// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package mysql

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/infra/db"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/gorm_gen/model"
	genquery "github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/gorm_gen/query"
	obErrorx "github.com/coze-dev/coze-loop/backend/modules/observability/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/logs"
)

//go:generate mockgen -destination=mocks/view.go -package=mocks . IViewDao
type IViewDao interface {
	GetView(ctx context.Context, id int64, workspaceID *int64, userID *string) (*model.ObservabilityView, error)
	ListViews(ctx context.Context, workspaceID int64, userID string) ([]*model.ObservabilityView, error)
	CreateView(ctx context.Context, po *model.ObservabilityView) (int64, error)
	UpdateView(ctx context.Context, po *model.ObservabilityView) error
	DeleteView(ctx context.Context, id int64, workspaceID int64, userID string) error
}

func NewViewDaoImpl(db db.Provider) IViewDao {
	return &ViewDaoImpl{
		dbMgr: db,
	}
}

type ViewDaoImpl struct {
	dbMgr db.Provider
}

func (v *ViewDaoImpl) GetView(ctx context.Context, id int64, workspaceID *int64, userID *string) (*model.ObservabilityView, error) {
	q := genquery.Use(v.dbMgr.NewSession(ctx)).ObservabilityView
	qd := q.WithContext(ctx).Where(q.ID.Eq(id))
	if workspaceID != nil {
		qd = qd.Where(q.WorkspaceID.Eq(*workspaceID))
	}
	if userID != nil {
		qd = qd.Where(q.CreatedBy.Eq(*userID))
	}
	viewPo, err := qd.First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("view not found"))
		} else {
			return nil, errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
		}
	}
	return viewPo, nil
}

func (v *ViewDaoImpl) ListViews(ctx context.Context, workspaceID int64, userID string) ([]*model.ObservabilityView, error) {
	q := genquery.Use(v.dbMgr.NewSession(ctx)).ObservabilityView
	qd := q.WithContext(ctx)
	if workspaceID != 0 {
		qd = qd.Where(q.WorkspaceID.Eq(workspaceID))
	}
	if userID != "" {
		qd = qd.Where(q.CreatedBy.Eq(userID))
	}
	results, err := qd.Limit(100).Find()
	if err != nil {
		return nil, errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
	}
	return results, nil
}

func (v *ViewDaoImpl) CreateView(ctx context.Context, po *model.ObservabilityView) (int64, error) {
	q := genquery.Use(v.dbMgr.NewSession(ctx)).ObservabilityView
	if err := q.WithContext(ctx).Create(po); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return 0, errorx.NewByCode(obErrorx.CommercialCommonInvalidParamCodeCode, errorx.WithExtraMsg("view duplicate key"))
		} else {
			return 0, errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
		}
	} else {
		return po.ID, nil
	}
}

func (v *ViewDaoImpl) UpdateView(ctx context.Context, po *model.ObservabilityView) error {
	q := genquery.Use(v.dbMgr.NewSession(ctx)).ObservabilityView
	if err := q.WithContext(ctx).Save(po); err != nil {
		return errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
	} else {
		return nil
	}
}

func (v *ViewDaoImpl) DeleteView(ctx context.Context, id int64, workspaceID int64, userID string) error {
	q := genquery.Use(v.dbMgr.NewSession(ctx)).ObservabilityView
	qd := q.WithContext(ctx).Where(q.ID.Eq(id)).Where(q.WorkspaceID.Eq(workspaceID)).Where(q.CreatedBy.Eq(userID))
	info, err := qd.Delete()
	if err != nil {
		return errorx.WrapByCode(err, obErrorx.CommonMySqlErrorCode)
	}
	logs.CtxInfo(ctx, "%d rows deleted", info.RowsAffected)
	return nil
}
