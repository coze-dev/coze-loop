// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package convertor

import (
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/gorm_gen/model"
)

func ViewDO2PO(view *entity.ObservabilityView) *model.ObservabilityView {
	return &model.ObservabilityView{
		ID:           view.ID,
		EnterpriseID: view.EnterpriseID,
		WorkspaceID:  view.WorkspaceID,
		ViewName:     view.ViewName,
		PlatformType: view.PlatformType,
		SpanListType: view.SpanListType,
		Filters:      view.Filters,
		CreatedAt:    view.CreatedAt,
		CreatedBy:    view.CreatedBy,
		UpdatedAt:    view.UpdatedAt,
		UpdatedBy:    view.UpdatedBy,
		Scope:        view.Scope,
	}
}

func ViewPO2DO(view *model.ObservabilityView) *entity.ObservabilityView {
	return &entity.ObservabilityView{
		ID:           view.ID,
		EnterpriseID: view.EnterpriseID,
		WorkspaceID:  view.WorkspaceID,
		ViewName:     view.ViewName,
		PlatformType: view.PlatformType,
		SpanListType: view.SpanListType,
		Filters:      view.Filters,
		CreatedAt:    view.CreatedAt,
		CreatedBy:    view.CreatedBy,
		UpdatedAt:    view.UpdatedAt,
		UpdatedBy:    view.UpdatedBy,
		Scope:        view.Scope,
	}
}
