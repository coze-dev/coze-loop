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

type modelConfigAliases struct {
	ParamConfig *paramConfigAliases `mapstructure:"param_config"`
}

type paramConfigAliases struct {
	ParamSchemas []*paramSchemaAliases `mapstructure:"param_schemas"`
}

type paramSchemaAliases struct {
	DefaultVal *string               `mapstructure:"default_val"`
	Properties []*paramSchemaAliases `mapstructure:"properties"`
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

func (m *ManageImpl) readConfig(ctx context.Context) ([]*entity.Model, error) {
	var models []*entity.Model
	if err := m.loader.UnmarshalKey(ctx, "models", &models); err != nil {
		return nil, err
	}

	// default_value was the original decoder key, while every shipped model
	// config uses default_val. Read the shipped spelling separately so both
	// existing custom configs and the bundled configs remain supported.
	var aliases []*modelConfigAliases
	if err := m.loader.UnmarshalKey(ctx, "models", &aliases); err != nil {
		return nil, err
	}
	for i, alias := range aliases {
		if i >= len(models) || alias == nil || alias.ParamConfig == nil || models[i] == nil || models[i].ParamConfig == nil {
			continue
		}
		applyParamSchemaAliases(models[i].ParamConfig.ParamSchemas, alias.ParamConfig.ParamSchemas)
	}
	return models, nil
}

func applyParamSchemaAliases(schemas []*entity.ParamSchema, aliases []*paramSchemaAliases) {
	for i, alias := range aliases {
		if i >= len(schemas) || alias == nil || schemas[i] == nil {
			continue
		}
		if alias.DefaultVal != nil {
			schemas[i].DefaultValue = *alias.DefaultVal
		}
		applyParamSchemaAliases(schemas[i].Properties, alias.Properties)
	}
}
