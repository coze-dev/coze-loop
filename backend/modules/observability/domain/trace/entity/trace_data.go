// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

type TenantInfo struct {
	TTL              loop_span.TTL  `json:"ttl"`
	WorkspaceId      string         `json:"space_id"`
	CozeAccountID    string         `json:"coze_account_id"`
	WhichIsEnough    int            `json:"which_is_enough"`
	VolcanoAccountID int64          `json:"volcano_account_id"`
	Extra            map[string]any `json:"extra,omitempty"`
}

type TraceData struct {
	Tenant     string             `json:"tenant"`
	TenantInfo TenantInfo         `json:"tenant_info"`
	SpanList   loop_span.SpanList `json:"span_list"`
}
