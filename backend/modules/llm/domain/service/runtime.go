// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/component/conf"
	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/service/llmfactory"
	"github.com/coze-dev/coze-loop/backend/modules/llm/domain/service/llminterface"
	llm_errorx "github.com/coze-dev/coze-loop/backend/modules/llm/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/modules/llm/pkg/httputil"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
	"github.com/coze-dev/coze-loop/backend/pkg/localos"
)

//go:generate mockgen -destination=mocks/runtime.go -package=mocks . IRuntime
type IRuntime interface {
	// Generate 非流式
	Generate(ctx context.Context, model *entity.Model, input []*entity.Message, opts ...entity.Option) (*entity.Message, error)
	// Stream 流式
	Stream(ctx context.Context, model *entity.Model, input []*entity.Message, opts ...entity.Option) (
		entity.IStreamReader, error)
	// CreateModelRequestRecord 记录模型请求
	CreateModelRequestRecord(ctx context.Context, record *entity.ModelRequestRecord) (err error)
	// HandleMsgsPreCallModel 在请求模型前处理消息，如把非公网URL转为base64
	HandleMsgsPreCallModel(ctx context.Context, model *entity.Model, msgs []*entity.Message) ([]*entity.Message, error)
	// ValidModelAndRequest 校验模型和请求是否兼容
	ValidModelAndRequest(ctx context.Context, model *entity.Model, input []*entity.Message, opts ...entity.Option) error
}

type RuntimeImpl struct {
	llmFact     llmfactory.IFactory
	idGen       idgen.IIDGenerator
	runtimeRepo repo.IRuntimeRepo
	runtimeCfg  conf.IConfigRuntime
}

var _ IRuntime = (*RuntimeImpl)(nil)

func (r *RuntimeImpl) Generate(ctx context.Context, model *entity.Model, input []*entity.Message, opts ...entity.Option) (*entity.Message, error) {
	if err := r.ValidModelAndRequest(ctx, model, input, opts...); err != nil {
		return nil, err
	}
	llm, err := r.buildLLM(ctx, model, opts...)
	if err != nil {
		return nil, err
	}
	return llm.Generate(ctx, input, opts...)
}

func (r *RuntimeImpl) Stream(ctx context.Context, model *entity.Model, input []*entity.Message, opts ...entity.Option) (
	entity.IStreamReader, error,
) {
	if err := r.ValidModelAndRequest(ctx, model, input, opts...); err != nil {
		return nil, err
	}
	llm, err := r.buildLLM(ctx, model, opts...)
	if err != nil {
		return nil, err
	}
	return llm.Stream(ctx, input, opts...)
}

func (r *RuntimeImpl) buildLLM(ctx context.Context, model *entity.Model, opts ...entity.Option) (llminterface.ILLM, error) {
	llm, err := r.llmFact.CreateLLM(ctx, model, opts...)
	if err != nil {
		return nil, errorx.WrapByCode(err, llm_errorx.BuildLLMFailedCode)
	}
	return llm, nil
}

func (r *RuntimeImpl) CreateModelRequestRecord(ctx context.Context, record *entity.ModelRequestRecord) (err error) {
	return r.runtimeRepo.CreateModelRequestRecord(ctx, record)
}

func (r *RuntimeImpl) HandleMsgsPreCallModel(ctx context.Context, model *entity.Model, msgs []*entity.Message) ([]*entity.Message, error) {
	if model == nil {
		return msgs, nil
	}
	for _, msg := range msgs {
		for _, part := range msg.MultiModalContent {
			if part.IsURL() && model.SupportMultiModalInput() && r.runtimeCfg.NeedCvtURLToBase64() {
				url := part.ImageURL.URL
				// 如果不是完整url，就认为这个url是系统内部minio签发的，需要拼接为完整url
				if !httputil.IsFullUrl(url) {
					url = fmt.Sprintf("http://%s%s", localos.GetLocalOSHost(), url)
				}
				base64Str, mimeType, err := httputil.ImageURLToBase64(url)
				if err != nil {
					return msgs, err
				}
				part.ImageURL.URL = base64Str
				part.ImageURL.MIMEType = mimeType
			}
		}
	}
	return msgs, nil
}

func (r *RuntimeImpl) ValidModelAndRequest(ctx context.Context, model *entity.Model, input []*entity.Message, opts ...entity.Option) error {
	// 如果msg中有多模态输入，看模型是否支持多模态
	var hasMultiModal, hasImageURL, hasImageBinary bool
	var maxImageCnt, maxImageSizeInByte int64
	for _, msg := range input {
		if msg.HasMultiModalContent() {
			hasMultiModal = true
			tmpHasImageURL, tmpHasImageBinary, tmpMaxImageCnt, tmpMaxImageSizeInByte := msg.GetImageCountAndMaxSize()
			if tmpHasImageURL {
				hasImageURL = true
			}
			if tmpHasImageBinary {
				hasImageBinary = true
			}
			if maxImageCnt < tmpMaxImageCnt {
				maxImageCnt = tmpMaxImageCnt
			}
			if maxImageSizeInByte < tmpMaxImageSizeInByte {
				maxImageSizeInByte = tmpMaxImageSizeInByte
			}
		}
	}
	if hasMultiModal && !model.SupportMultiModalInput() {
		return errorx.NewByCode(llm_errorx.RequestNotCompatibleWithModelAbilityCode, errorx.WithExtraMsg("messages have multi modal content, but this model does not support multi modal"))
	}
	if hasImageURL {
		s, cnt := model.SupportImageURL()
		if !s {
			return errorx.NewByCode(llm_errorx.RequestNotCompatibleWithModelAbilityCode, errorx.WithExtraMsg("messages have image url, but this model does not support image url"))
		}
		if cnt > 0 && cnt < maxImageCnt {
			return errorx.NewByCode(llm_errorx.RequestNotCompatibleWithModelAbilityCode, errorx.WithExtraMsg("one message of messages has too much images for this model"))
		}
	}
	if hasImageBinary {
		s, cnt, size := model.SupportImageBinary()
		if !s {
			return errorx.NewByCode(llm_errorx.RequestNotCompatibleWithModelAbilityCode, errorx.WithExtraMsg("messages have image binary, but this model does not support image binary"))
		}
		if cnt > 0 && cnt < maxImageCnt {
			return errorx.NewByCode(llm_errorx.RequestNotCompatibleWithModelAbilityCode, errorx.WithExtraMsg("one message of messages has too much images for this model"))
		}
		if size > 0 && size*1024*1024 < maxImageSizeInByte {
			return errorx.NewByCode(llm_errorx.RequestNotCompatibleWithModelAbilityCode, errorx.WithExtraMsg("one message of messages has too big images for this model"))
		}
	}
	// 如果option中有tool call，看模型是否支持function call
	options := entity.ApplyOptions(nil, opts...)
	if len(options.Tools) > 0 && !model.SupportFunctionCall() {
		return errorx.NewByCode(llm_errorx.RequestNotCompatibleWithModelAbilityCode, errorx.WithExtraMsg("input has tool calls, but this model does not support tool call"))
	}
	return nil
}
