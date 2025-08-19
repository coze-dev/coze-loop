// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package llmfactory

import (
	"context"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/service/llmimpl/eino"
	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/service/llminterface"
	llm_errorx "github.com/coze-dev/coze-loop/backend/modules/llm/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

//go:generate mockgen -destination=mocks/factory.go -package=mocks . IFactory
type IFactory interface {
	CreateLLM(ctx context.Context, model *entity.Model, opts ...entity.Option) (llminterface.ILLM, error)
}

type FactoryImpl struct{}

var _ IFactory = (*FactoryImpl)(nil)

func (f *FactoryImpl) CreateLLM(ctx context.Context, model *entity.Model, opts ...entity.Option) (llminterface.ILLM, error) {
	// 根据frame和protocol导航到不同的frame factory
	frame, err := f.getFrameByModel(model)
	if err != nil {
		return nil, err
	}
	// 用该frame factory创建llm接口的实现
	switch frame {
	case entity.FrameEino:
		return eino.NewLLM(ctx, model, opts...)
	default:
		return nil, errorx.NewByCode(llm_errorx.ModelInvalidCode, errorx.WithExtraMsg(fmt.Sprintf("[CreateLLM] frame:%s is not supported", frame)))
	}
}

func (f *FactoryImpl) getFrameByModel(model *entity.Model) (entity.Frame, error) {
	if model == nil {
		return "", errorx.NewByCode(llm_errorx.ModelInvalidCode, errorx.WithExtraMsg("[getFrameByModel] model is nil"))
	}
	if model.Frame != entity.FrameDefault {
		return model.Frame, nil
	}
	// 目前只支持eino，所以取eino，否则要根据protocol取不同frame
	return entity.FrameEino, nil
}
