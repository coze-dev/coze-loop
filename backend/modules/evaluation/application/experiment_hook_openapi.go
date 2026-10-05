// Copyright (c) 2026 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	domain "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/expt"
	openapi "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain_openapi/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/application/convertor/experiment"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
)

func (e *EvalOpenAPIApplication) hookApplication() *experimentApplication {
	app, ok := e.experimentApp.(*experimentApplication)
	if !ok || app == nil || app.hooks == nil {
		return &experimentApplication{}
	}
	return &experimentApplication{auth: e.auth, hooks: app.hooks}
}

func (e *EvalOpenAPIApplication) templateDTOsWithHooks(ctx context.Context, templates []*entity.ExptTemplate, spaceID int64) ([]*openapi.ExptTemplate, error) {
	dtos := experiment.OpenAPIExptTemplateDO2DTOs(templates)
	app := e.hookApplication()
	if app.hooks == nil {
		return dtos, nil
	}
	internal := experiment.ToExptTemplateDTOs(templates)
	if err := app.readTemplateHooks(ctx, templates, internal, spaceID); err != nil {
		return nil, err
	}
	for i := range dtos {
		dtos[i].LifecycleHookConf = experiment.LifecycleHookConfDomain2OpenAPI(internal[i].LifecycleHookConf)
	}
	return dtos, nil
}

func (e *EvalOpenAPIApplication) templateSubmitHooks(ctx context.Context, t *entity.ExptTemplate, override *openapi.LifecycleHookConf, spaceID int64) (*domain.LifecycleHookConf, error) {
	return e.hookApplication().templateSubmitHooks(ctx, t, experiment.OpenAPILifecycleHookConfDTO2Domain(override), spaceID)
}
