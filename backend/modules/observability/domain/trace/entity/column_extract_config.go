// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package entity

import "time"

type ColumnExtractRule struct {
	Column   string
	JSONPath string
}

type ColumnExtractConfig struct {
	ID           int64
	WorkspaceID  int64
	PlatformType string
	SpanListType string
	AgentName    string
	Columns      []ColumnExtractRule
	CreatedAt    time.Time
	CreatedBy    string
	UpdatedAt    time.Time
	UpdatedBy    string
}
