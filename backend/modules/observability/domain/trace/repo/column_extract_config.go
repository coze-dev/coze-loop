// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
)

type UpsertColumnExtractConfigParam struct {
	WorkspaceId  int64
	PlatformType string
	SpanListType string
	AgentName    string
	Config       string
	UserID       string
}

type GetColumnExtractConfigParam struct {
	WorkspaceId  int64
	PlatformType string
	SpanListType string
	AgentName    string
}

type ListColumnExtractConfigParam struct {
	WorkspaceID  int64
	PlatformType string
	SpanListType string
}

//go:generate mockgen -destination=mocks/column_extract_config.go -package=mocks . IColumnExtractConfigRepo
type IColumnExtractConfigRepo interface {
	UpsertColumnExtractConfig(ctx context.Context, param *UpsertColumnExtractConfigParam) error
	GetColumnExtractConfig(ctx context.Context, param GetColumnExtractConfigParam) (*entity.ColumnExtractConfig, error)
	ListColumnExtractConfigs(ctx context.Context, param ListColumnExtractConfigParam) ([]*entity.ColumnExtractConfig, error)
}
