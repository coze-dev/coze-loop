// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
)

//go:generate mockgen -destination=mocks/tool_service.go -package=mocks . IToolService

type IToolService interface {
	CreateTool(ctx context.Context, toolDO *entity.CommonTool) (toolID int64, err error)
}

type ToolServiceImpl struct {
	idgen    idgen.IIDGenerator
	toolRepo repo.IToolRepo
}

func NewToolService(
	idgen idgen.IIDGenerator,
	toolRepo repo.IToolRepo,
) IToolService {
	return &ToolServiceImpl{
		idgen:    idgen,
		toolRepo: toolRepo,
	}
}

func (s *ToolServiceImpl) CreateTool(ctx context.Context, toolDO *entity.CommonTool) (toolID int64, err error) {
	id, err := s.idgen.GenID(ctx)
	if err != nil {
		return 0, err
	}
	toolDO.ID = id
	return s.toolRepo.CreateTool(ctx, toolDO)
}
