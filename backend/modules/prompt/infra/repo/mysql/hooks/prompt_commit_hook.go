package hooks

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/model"
)

type IPromptCommitHook interface {
	BeforeSave(context.Context, *model.PromptCommit) error
	AfterFind(context.Context, []*model.PromptCommit) error
}

type EmptyPromptCommitHook struct{}

func NewEmptyPromptCommitHook() IPromptCommitHook {
	return &EmptyPromptCommitHook{}
}

func (h *EmptyPromptCommitHook) BeforeSave(_ context.Context, _ *model.PromptCommit) error {
	return nil
}

func (h *EmptyPromptCommitHook) AfterFind(_ context.Context, _ []*model.PromptCommit) error {
	return nil
}
