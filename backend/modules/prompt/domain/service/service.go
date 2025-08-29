// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package service

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/infra/idgen"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/conf"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
)

//go:generate mockgen -destination=mocks/prompt_service.go -package=mocks . IPromptService
type IPromptService interface {
	FormatPrompt(ctx context.Context, prompt *entity.Prompt, messages []*entity.Message, variableVals []*entity.VariableVal) (formattedMessages []*entity.Message, err error)
	ExecuteStreaming(ctx context.Context, param ExecuteStreamingParam) (*entity.Reply, error)
	Execute(ctx context.Context, param ExecuteParam) (*entity.Reply, error)

	MCompleteMultiModalFileURL(ctx context.Context, messages []*entity.Message, variableVals []*entity.VariableVal) error

	// MGetPromptIDs 根据prompt key获取prompt id
	MGetPromptIDs(ctx context.Context, spaceID int64, promptKeys []string) (PromptKeyIDMap map[string]int64, err error)
	// MParseCommitVersionByPromptKey 根据prompt key解析提交版本，如果提交版本为空，则使用最新版本，否则返回指定版本
	MParseCommitVersionByPromptKey(ctx context.Context, spaceID int64, pairs []PromptKeyVersionPair) (promptKeyCommitVersionMap map[PromptKeyVersionPair]string, err error)
}

type GetBasicParam struct {
	PromptID int64

	SpaceID   int64
	PromptKey string
}

type GetPromptCommitParam struct {
	CommitVersion string
}

type GetPromptDraftParam struct {
	UserID string
}

type PromptKeyVersionPair struct {
	PromptKey string
	Version   string
}

type PromptIDVersionPair struct {
	PromptID int64
	Version  string
}

type PromptServiceImpl struct {
	idgen            idgen.IIDGenerator
	debugLogRepo     repo.IDebugLogRepo
	debugContextRepo repo.IDebugContextRepo
	manageRepo       repo.IManageRepo
	configProvider   conf.IConfigProvider
	llm              rpc.ILLMProvider
	file             rpc.IFileProvider
}

func NewPromptService(
	idgen idgen.IIDGenerator,
	debugLogRepo repo.IDebugLogRepo,
	debugContextRepo repo.IDebugContextRepo,
	promptManageRepo repo.IManageRepo,
	configProvider conf.IConfigProvider,
	llm rpc.ILLMProvider,
	file rpc.IFileProvider,
) IPromptService {
	return &PromptServiceImpl{
		idgen:            idgen,
		debugLogRepo:     debugLogRepo,
		debugContextRepo: debugContextRepo,
		manageRepo:       promptManageRepo,
		configProvider:   configProvider,
		llm:              llm,
		file:             file,
	}
}
