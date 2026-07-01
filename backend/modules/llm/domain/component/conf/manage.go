// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package conf

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/entity"
)

//go:generate mockgen -destination=mocks/manage.go -package=mocks . IConfigManage
type IConfigManage interface {
	ListModels(ctx context.Context, req entity.ListModelReq) (models []*entity.Model, total int64, hasMore bool, nextPageToken int64, err error)
	GetModel(ctx context.Context, id int64) (model *entity.Model, err error)
	// GetModelByKey 按 workspace 内唯一 model_key 精确匹配。未命中返回 gorm.ErrRecordNotFound。
	GetModelByKey(ctx context.Context, workspaceID int64, key string) (model *entity.Model, err error)
}
