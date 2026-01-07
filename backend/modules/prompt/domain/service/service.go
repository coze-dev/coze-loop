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
	MConvertBase64DataURLToFileURI(ctx context.Context, messages []*entity.Message, workspaceID int64) error
	MConvertBase64DataURLToFileURL(ctx context.Context, messages []*entity.Message, workspaceID int64) error
	// MGetPromptIDs 根据prompt key获取prompt id
	MGetPromptIDs(ctx context.Context, spaceID int64, promptKeys []string) (PromptKeyIDMap map[string]int64, err error)
	// MParseCommitVersion 统一解析提交版本，支持version和label两种方式
	MParseCommitVersion(ctx context.Context, spaceID int64, params []PromptQueryParam) (promptKeyCommitVersionMap map[PromptQueryParam]string, err error)

	// Prompt管理相关方法
	CreatePrompt(ctx context.Context, promptDO *entity.Prompt) (promptID int64, err error)
	SaveDraft(ctx context.Context, promptDO *entity.Prompt) (*entity.DraftInfo, error)
	GetPrompt(ctx context.Context, param GetPromptParam) (*entity.Prompt, error)

	// Snippet扩展相关方法
	ExpandSnippets(ctx context.Context, promptDO *entity.Prompt) error

	// Label管理相关方法
	CreateLabel(ctx context.Context, labelDO *entity.PromptLabel) error
	ListLabel(ctx context.Context, param ListLabelParam) ([]*entity.PromptLabel, *int64, error)
	UpdateCommitLabels(ctx context.Context, param UpdateCommitLabelsParam) error
	BatchGetCommitLabels(ctx context.Context, promptID int64, commitVersions []string) (map[string][]string, error)
	ValidateLabelsExist(ctx context.Context, spaceID int64, labelKeys []string) error
	BatchGetLabelMappingPromptVersion(ctx context.Context, queries []PromptLabelQuery) (map[PromptLabelQuery]string, error)
}

type PromptKeyVersionPair struct {
	PromptKey string
	Version   string
}

type PromptQueryParam struct {
	PromptID  int64
	PromptKey string
	Version   string // 可选，优先使用
	Label     string // 可选，当Version为空时使用
}

type ListLabelParam struct {
	SpaceID      int64
	LabelKeyLike string
	PageSize     int
	PageToken    *int64
}

type UpdateCommitLabelsParam struct {
	PromptID      int64
	CommitVersion string
	LabelKeys     []string
	UpdatedBy     string
}

type PromptLabelQuery struct {
	PromptID int64
	LabelKey string
}

type PromptServiceImpl struct {
	formatter            IPromptFormatter
	toolConfigProvider   IToolConfigProvider
	toolResultsCollector IToolResultsCollector
	idgen                idgen.IIDGenerator
	debugLogRepo         repo.IDebugLogRepo
	debugContextRepo     repo.IDebugContextRepo
	manageRepo           repo.IManageRepo
	labelRepo            repo.ILabelRepo
	configProvider       conf.IConfigProvider
	llm                  rpc.ILLMProvider
	file                 rpc.IFileProvider
	snippetParser        SnippetParser
}

type GetPromptParam struct {
	PromptID int64

	WithCommit    bool
	CommitVersion string

	WithDraft     bool
	UserID        string
	ExpandSnippet bool
}

func NewPromptService(
	formatter IPromptFormatter,
	toolConfigProvider IToolConfigProvider,
	toolResultsProcessor IToolResultsCollector,
	idgen idgen.IIDGenerator,
	debugLogRepo repo.IDebugLogRepo,
	debugContextRepo repo.IDebugContextRepo,
	promptManageRepo repo.IManageRepo,
	labelRepo repo.ILabelRepo,
	configProvider conf.IConfigProvider,
	llm rpc.ILLMProvider,
	file rpc.IFileProvider,
	snippetParser SnippetParser,
) IPromptService {
	return &PromptServiceImpl{
		formatter:            formatter,
		toolConfigProvider:   toolConfigProvider,
		toolResultsCollector: toolResultsProcessor,
		idgen:                idgen,
		debugLogRepo:         debugLogRepo,
		debugContextRepo:     debugContextRepo,
		manageRepo:           promptManageRepo,
		labelRepo:            labelRepo,
		configProvider:       configProvider,
		llm:                  llm,
		file:                 file,
		snippetParser:        snippetParser,
	}
}
