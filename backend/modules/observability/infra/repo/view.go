// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/convertor"
)

func NewViewRepoImpl(viewDao mysql.IViewDao, idGenerator idgen.IIDGenerator) repo.IViewRepo {
	return &ViewRepoImpl{
		viewDao:     viewDao,
		idGenerator: idGenerator,
	}
}

type ViewRepoImpl struct {
	viewDao     mysql.IViewDao
	idGenerator idgen.IIDGenerator
}

func (v *ViewRepoImpl) GetView(ctx context.Context, id int64, workspaceID *int64, userID *string) (*entity.ObservabilityView, error) {
	viewPo, err := v.viewDao.GetView(ctx, id, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	return convertor.ViewPO2DO(viewPo), nil
}

func (v *ViewRepoImpl) ListViews(ctx context.Context, workspaceID int64, userID string) ([]*entity.ObservabilityView, error) {
	results, err := v.viewDao.ListViews(ctx, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	resp := make([]*entity.ObservabilityView, len(results))
	for i, result := range results {
		resp[i] = convertor.ViewPO2DO(result)
	}
	return resp, nil
}

func (v *ViewRepoImpl) CreateView(ctx context.Context, do *entity.ObservabilityView) (int64, error) {
	id, err := v.idGenerator.GenID(ctx)
	if err != nil {
		return 0, err
	}
	viewPo := convertor.ViewDO2PO(do)
	viewPo.ID = id
	return v.viewDao.CreateView(ctx, viewPo)
}

func (v *ViewRepoImpl) UpdateView(ctx context.Context, do *entity.ObservabilityView) error {
	viewPo := convertor.ViewDO2PO(do)
	return v.viewDao.UpdateView(ctx, viewPo)
}

func (v *ViewRepoImpl) DeleteView(ctx context.Context, id int64, workspaceID int64, userID string) error {
	return v.viewDao.DeleteView(ctx, id, workspaceID, userID)
}
