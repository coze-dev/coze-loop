// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"context"
	"math"

	"gorm.io/gorm"

	llm_conf "github.com/coze-dev/coze-loop/backend/modules/llm/domain/component/conf"
	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/entity"
	"github.com/coze-dev/coze-loop/backend/pkg/conf"
)

type ManageImpl struct {
	loader conf.IConfigLoader
}

func NewManage(ctx context.Context, factory conf.IConfigLoaderFactory) (llm_conf.IConfigManage, error) {
	loader, err := factory.NewConfigLoader("model_config.yaml")
	if err != nil {
		return nil, err
	}
	return &ManageImpl{
		loader: loader,
	}, nil
}

// ListModel 用于获得模型的配置；后续会使用数据库管理，本期先使用yaml文件管理
func (m *ManageImpl) ListModels(ctx context.Context, req entity.ListModelReq) (models []*entity.Model, total int64, hasMore bool, nextPageToken int64, err error) {
	modelsInCfg, err := m.readConfig(ctx)
	if err != nil {
		return models, total, hasMore, nextPageToken, err
	}
	if req.Scenario != nil {
		for _, md := range modelsInCfg {
			if md.Available(req.Scenario) {
				models = append(models, md)
			}
		}
	}
	total = int64(len(models))
	if req.PageToken >= total {
		return models, total, false, req.PageToken, nil
	}
	nextPageToken = int64(math.Min(float64(req.PageToken+req.PageSize), float64(total)))
	models = models[req.PageToken:nextPageToken]
	return models, total, nextPageToken < total, nextPageToken, nil
}

func (m *ManageImpl) GetModel(ctx context.Context, id int64) (model *entity.Model, err error) {
	models, err := m.readConfig(ctx)
	if err != nil {
		return nil, err
	}
	for _, md := range models {
		if md.ID == id {
			return md, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

// GetModelByKey yaml 场景下 workspaceID 通常为 0 或与配置项一致，此处以 (workspaceID, key) 为主匹配；
// 若配置中 md.WorkspaceID 为 0 视为跨空间通用，允许命中。
func (m *ManageImpl) GetModelByKey(ctx context.Context, workspaceID int64, key string) (model *entity.Model, err error) {
	if key == "" {
		return nil, gorm.ErrRecordNotFound
	}
	models, err := m.readConfig(ctx)
	if err != nil {
		return nil, err
	}
	for _, md := range models {
		if md.ModelKey != key {
			continue
		}
		if md.WorkspaceID == 0 || md.WorkspaceID == workspaceID {
			return md, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *ManageImpl) readConfig(ctx context.Context) ([]*entity.Model, error) {
	var models []*entity.Model
	if err := m.loader.UnmarshalKey(ctx, "models", &models); err != nil {
		return nil, err
	}
	return models, nil
}
