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
	// GetModelByKey 按 workspace 内 (workspaceID, key) 精确匹配。key 为空返回 ResourceNotFound。
	GetModelByKey(ctx context.Context, workspaceID int64, key string) (model *entity.Model, err error)
	// ResolveByKeyOrID id 优先, key 兜底; 两者都空返回 ResourceNotFound; 两者同传以 id 为准。
	ResolveByKeyOrID(ctx context.Context, ref KeyOrIDRef) (model *entity.Model, err error)
}

// KeyOrIDRef 描述一个模型引用: ID / Key 至少填一个; 同传以 ID 为准。
type KeyOrIDRef struct {
	WorkspaceID int64
	ID          int64
	Key         string
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

func (m *ManageImpl) GetModelByKey(ctx context.Context, workspaceID int64, key string) (model *entity.Model, err error) {
	if key == "" {
		return nil, errorx.NewByCode(llm_errorx.ResourceNotFoundCode, errorx.WithExtraMsg("model_key is empty"))
	}
	model, err = m.conf.GetModelByKey(ctx, workspaceID, key)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errorx.NewByCode(llm_errorx.ResourceNotFoundCode, errorx.WithExtraMsg(fmt.Sprintf("model workspace_id:%d key:%s not exist", workspaceID, key)))
		}
		return nil, errorx.NewByCode(llm_errorx.CommonMySqlErrorCode, errorx.WithExtraMsg(err.Error()))
	}
	return model, nil
}

// ResolveByKeyOrID: 语义按 PRD "同传以 modelID 为准"。
// 1) id > 0: 走 GetModelByID (忽略 key)
// 2) id == 0 && key != "": 走 GetModelByKey
// 3) 两者都空: ResourceNotFound
func (m *ManageImpl) ResolveByKeyOrID(ctx context.Context, ref KeyOrIDRef) (model *entity.Model, err error) {
	if ref.ID > 0 {
		return m.GetModelByID(ctx, ref.ID)
	}
	if ref.Key != "" {
		return m.GetModelByKey(ctx, ref.WorkspaceID, ref.Key)
	}
	return nil, errorx.NewByCode(llm_errorx.ResourceNotFoundCode, errorx.WithExtraMsg("model_id and model_key are both empty"))
}
