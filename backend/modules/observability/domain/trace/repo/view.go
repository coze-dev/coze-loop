// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
)

//go:generate mockgen -destination=mocks/view.go -package=mocks . IViewRepo
type IViewRepo interface {
	GetView(ctx context.Context, id int64, workspaceID *int64, userID *string) (*entity.ObservabilityView, error)
	ListViews(ctx context.Context, workspaceID int64, userID string, scope int32) ([]*entity.ObservabilityView, error)
	UpdateView(ctx context.Context, do *entity.ObservabilityView) error
	CreateView(ctx context.Context, do *entity.ObservabilityView) (int64, error)
	DeleteView(ctx context.Context, id int64, workspaceID int64, userID string) error
}
