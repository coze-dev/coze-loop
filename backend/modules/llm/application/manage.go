// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"strconv"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/manage"
	"github.com/coze-dev/coze-loop/backend/modules/llm/application/convertor"
	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/service"
	"github.com/coze-dev/coze-loop/backend/pkg/lang/ptr"
)

type manageApp struct {
	manageSrv service.IManage
	auth      rpc.IAuthProvider
}

func NewManageApplication(
	manageSrv service.IManage,
	auth rpc.IAuthProvider,
) manage.LLMManageService {
	return &manageApp{
		manageSrv: manageSrv,
		auth:      auth,
	}
}

func (m *manageApp) ListModels(ctx context.Context, req *manage.ListModelsRequest) (r *manage.ListModelsResponse, err error) {
	r = manage.NewListModelsResponse()
	if err := m.auth.CheckSpacePermission(ctx, req.GetWorkspaceID(), "listModels"); err != nil {
		return r, err
	}
	var scenario *entity.Scenario
	if req.Scenario != nil {
		scenario = ptr.Of(entity.Scenario(req.GetScenario()))
	} else {
		scenario = ptr.Of(entity.ScenarioDefault)
	}
	pageToken, _ := strconv.ParseInt(req.GetPageToken(), 10, 64)
	var modelKeyList []string
	if req.Filter != nil {
		modelKeyList = req.Filter.GetModelKeyList()
	}
	models, total, hasMore, nextPageToken, err := m.manageSrv.ListModels(ctx, entity.ListModelReq{
		WorkspaceID:  req.WorkspaceID,
		Scenario:     scenario,
		PageSize:     int64(req.GetPageSize()),
		PageToken:    pageToken,
		ModelKeyList: modelKeyList,
	})
	if err != nil {
		return r, err
	}
	r.SetModels(convertor.ModelsDO2DTO(models, true))
	r.SetTotal(ptr.Of(int32(total)))
	r.SetNextPageToken(ptr.Of(strconv.FormatInt(nextPageToken, 10)))
	r.SetHasMore(ptr.Of(hasMore))
	return r, nil
}

func (m *manageApp) GetModel(ctx context.Context, req *manage.GetModelRequest) (r *manage.GetModelResponse, err error) {
	r = manage.NewGetModelResponse()
	if err := m.auth.CheckSpacePermission(ctx, req.GetWorkspaceID(), "getModel"); err != nil {
		return r, err
	}
	// 统一解析:ID 优先,Key 备用(见 IManage.ResolveModel)。同时传以 ID 为准。
	model, err := m.manageSrv.ResolveModel(ctx, entity.GetModelReq{
		WorkspaceID: req.WorkspaceID,
		ModelID:     req.GetModelID(),
		ModelKey:    req.GetModelKey(),
	})
	if err != nil {
		return r, err
	}
	r.SetModel(convertor.ModelDO2DTO(model, true))
	return r, nil
}
