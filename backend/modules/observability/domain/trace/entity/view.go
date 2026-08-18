// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import (
	"time"
)

type ObservabilityView struct {
	ID           int64
	EnterpriseID string
	WorkspaceID  int64
	ViewName     string
	PlatformType string
	SpanListType string
	Filters      string
	CreatedAt    time.Time
	CreatedBy    string
	UpdatedAt    time.Time
	UpdatedBy    string
	Scope        int32
}
