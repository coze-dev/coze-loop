// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/view"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/trace"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
)

func TestCreateViewDTO2PO_Scope(t *testing.T) {
	tests := []struct {
		name      string
		req       *trace.CreateViewRequest
		wantScope int32
	}{
		{
			name:      "scope unset defaults to trace_list",
			req:       &trace.CreateViewRequest{WorkspaceID: 1, ViewName: "v"},
			wantScope: int32(view.Scope_TraceList),
		},
		{
			name:      "scope explicit trace_detail_chat is kept",
			req:       &trace.CreateViewRequest{WorkspaceID: 1, ViewName: "v", Scope: view.ScopePtr(view.Scope_TraceDetailChat)},
			wantScope: int32(view.Scope_TraceDetailChat),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			po := CreateViewDTO2PO(tt.req, "u")
			assert.Equal(t, tt.wantScope, po.Scope)
		})
	}
}

func TestViewPO2DTO_Scope(t *testing.T) {
	dto := ViewPO2DTO(&entity.ObservabilityView{Scope: int32(view.Scope_TraceDetailTree)})
	assert.Equal(t, view.Scope_TraceDetailTree, dto.GetScope())
}
