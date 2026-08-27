// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"time"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/view"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/trace"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

func ViewPO2DTO(v *entity.ObservabilityView) *view.View {
	if v == nil {
		return nil
	}
	return &view.View{
		ID:           v.ID,
		EnterpriseID: ptr.Of(v.EnterpriseID),
		WorkspaceID:  ptr.Of(v.WorkspaceID),
		ViewName:     v.ViewName,
		PlatformType: ptr.Of(v.PlatformType),
		SpanListType: ptr.Of(v.SpanListType),
		Filters:      v.Filters,
		IsSystem:     false,
		Scope:        ptr.Of(view.Scope(v.Scope)),
	}
}

func BatchViewPO2DTO(views []*entity.ObservabilityView) []*view.View {
	ret := make([]*view.View, len(views))
	for i, v := range views {
		ret[i] = ViewPO2DTO(v)
	}
	return ret
}

func CreateViewDTO2PO(req *trace.CreateViewRequest, userID string) *entity.ObservabilityView {
	if req == nil {
		return nil
	}
	scope := int32(req.GetScope())
	if scope == 0 {
		scope = int32(view.Scope_TraceList)
	}
	return &entity.ObservabilityView{
		EnterpriseID: req.GetEnterpriseID(),
		WorkspaceID:  req.GetWorkspaceID(),
		ViewName:     req.GetViewName(),
		PlatformType: req.GetPlatformType(),
		SpanListType: req.GetSpanListType(),
		Filters:      req.GetFilters(),
		CreatedAt:    time.Now(),
		CreatedBy:    userID,
		UpdatedAt:    time.Now(),
		UpdatedBy:    userID,
		Scope:        scope,
	}
}
