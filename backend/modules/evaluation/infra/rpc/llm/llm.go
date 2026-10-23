// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package llm

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/llm/runtime/llmruntimeservice"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	llmentity "github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

type LLMRPCAdapter struct {
	client llmruntimeservice.Client
}

func NewLLMRPCProvider(client llmruntimeservice.Client) rpc.ILLMProvider {
	return &LLMRPCAdapter{
		client: client,
	}
}

func (l *LLMRPCAdapter) Call(ctx context.Context, param *llmentity.LLMCallParam) (*llmentity.ReplyItem, error) {
	if param != nil && param.ModelConfig != nil && param.ModelConfig.Thinking != nil {
		return nil, errorx.NewByCode(errno.InvalidModelConfigCode, errorx.WithExtraMsg("thinking configuration is not supported by this LLM runtime adapter"))
	}
	req := LLMCallParamConvert(param)
	resp, err := l.client.Chat(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, nil
	}
	return ReplyItemDTO2DO(resp.Message), nil
}
