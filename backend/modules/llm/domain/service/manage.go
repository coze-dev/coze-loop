// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/component/conf"
	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/entity"
	llm_errorx "github.com/coze-dev/coze-loop/backend/modules/llm/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

//go:generate mockgen -destination=mocks/manage.go -package=mocks . IManage
type IManage interface {
	ListModels(ctx context.Context, req entity.ListModelReq) (models []*entity.Model, total int64, hasMore bool, nextPageToken int64, err error)
	GetModelByID(ctx context.Context, id int64) (model *entity.Model, err error)
	// ResolveModel 统一解析入口:ID 优先,再按 model_key 在该空间可调用集合内解析。
	// 语义:req.ModelID != 0 → GetModelByID;否则 req.ModelKey != "" → 按 key 解析;都为空 → ResourceNotFound。
	// 同时传 ID+Key → 以 ID 为准(与 PRD 一致)。
	ResolveModel(ctx context.Context, req entity.GetModelReq) (model *entity.Model, err error)
}

type ManageImpl struct {
	conf conf.IConfigManage
}

var _ IManage = (*ManageImpl)(nil)

func (m *ManageImpl) ListModels(ctx context.Context, req entity.ListModelReq) (models []*entity.Model, total int64, hasMore bool, nextPageToken int64, err error) {
	return m.conf.ListModels(ctx, req)
}

func (m *ManageImpl) GetModelByID(ctx context.Context, id int64) (model *entity.Model, err error) {
	model, err = m.conf.GetModel(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewByCode(llm_errorx.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("model id:%d not exist in db", id)))
		}
		return nil, errorx.NewByCode(llm_errorx.CommonMySqlErrorCode, errorx.WithExtraMsg(err.Error()))
	}
	return model, nil
}

// ResolveModel 见 IManage.ResolveModel 说明。
// 骨架实现:ID 优先分支已可用(复用 GetModelByID);Key 分支目前仅本地 yaml 配置源支持,DB 实现进入后需在
// infra/config/manage.go 上补 GetModelByKey(spaceID, key) 并在此调用。
func (m *ManageImpl) ResolveModel(ctx context.Context, req entity.GetModelReq) (*entity.Model, error) {
	if req.ModelID != 0 {
		return m.GetModelByID(ctx, req.ModelID)
	}
	if req.ModelKey != "" {
		if err := entity.ValidateModelKey(req.ModelKey); err != nil {
			return nil, errorx.NewByCode(llm_errorx.CommonInvalidParamCode, errorx.WithExtraMsg(err.Error()))
		}
		// TODO(model_key): DB 实现进入后接 conf.GetModelByKey(spaceID, key) + 公共集合 visibility 覆盖。
		// 骨架期只支持 ID 分支,Key 分支统一返回 NotFound,与「未命中」语义对齐。
		return nil, errorx.NewByCode(llm_errorx.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("model_key:%q resolve not implemented in config source", req.ModelKey)))
	}
	return nil, errorx.NewByCode(llm_errorx.CommonInvalidParamCode, errorx.WithExtraMsg("neither model_id nor model_key provided"))
}
