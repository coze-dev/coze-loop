// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"time"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/repo"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql"
	"github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/convertor"
	model2 "github.com/coze-dev/coze-loop/backend/modules/observability/infra/repo/mysql/gorm_gen/model"
)

func NewColumnExtractConfigRepoImpl(dao mysql.IColumnExtractConfigDao, idGenerator idgen.IIDGenerator) repo.IColumnExtractConfigRepo {
	return &ColumnExtractConfigRepoImpl{
		dao:         dao,
		idGenerator: idGenerator,
	}
}

type ColumnExtractConfigRepoImpl struct {
	dao         mysql.IColumnExtractConfigDao
	idGenerator idgen.IIDGenerator
}

func (r *ColumnExtractConfigRepoImpl) UpsertColumnExtractConfig(ctx context.Context, param *repo.UpsertColumnExtractConfigParam) error {
	existing, err := r.dao.GetColumnExtractConfig(ctx, param.WorkspaceId, param.PlatformType, param.SpanListType, param.AgentName)
	if err != nil {
		return err
	}

	if existing == nil {
		id, err := r.idGenerator.GenID(ctx)
		if err != nil {
			return err
		}
		po := &model2.ObservabilityColumnExtractConfig{
			ID:           id,
			WorkspaceID:  param.WorkspaceId,
			PlatformType: param.PlatformType,
			SpanListType: param.SpanListType,
			AgentName:    param.AgentName,
			Config:       &param.Config,
			CreatedAt:    time.Now(),
			CreatedBy:    param.UserID,
			UpdatedAt:    time.Now(),
			UpdatedBy:    param.UserID,
		}
		return r.dao.CreateColumnExtractConfig(ctx, po)
	}

	existing.Config = &param.Config
	existing.UpdatedAt = time.Now()
	existing.UpdatedBy = param.UserID
	existing.IsDeleted = false
	return r.dao.UpdateColumnExtractConfig(ctx, existing)
}

func (r *ColumnExtractConfigRepoImpl) GetColumnExtractConfig(ctx context.Context, param repo.GetColumnExtractConfigParam) (*entity.ColumnExtractConfig, error) {
	po, err := r.dao.GetColumnExtractConfig(ctx, param.WorkspaceId, param.PlatformType, param.SpanListType, param.AgentName)
	if err != nil {
		return nil, err
	}

	return convertor.ColumnExtractConfigPO2DO(po), nil
}

func (r *ColumnExtractConfigRepoImpl) ListColumnExtractConfigs(ctx context.Context, param repo.ListColumnExtractConfigParam) ([]*entity.ColumnExtractConfig, error) {
	pos, err := r.dao.ListColumnExtractConfigs(ctx, param.WorkspaceID, param.PlatformType, param.SpanListType)
	if err != nil {
		return nil, err
	}

	result := make([]*entity.ColumnExtractConfig, 0, len(pos))
	for _, po := range pos {
		if do := convertor.ColumnExtractConfigPO2DO(po); do != nil {
			result = append(result, do)
		}
	}
	return result, nil
}
