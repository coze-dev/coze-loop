// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package hooks

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/infra/repo/mysql/gorm_gen/model"
)

type IPromptUserDraftHook interface {
	BeforeSave(context.Context, *model.PromptUserDraft) error
	AfterFind(context.Context, []*model.PromptUserDraft) error
}

type EmptyPromptUserDraftHook struct{}

func NewEmptyPromptUserDraftHook() IPromptUserDraftHook {
	return &EmptyPromptUserDraftHook{}
}

func (h *EmptyPromptUserDraftHook) BeforeSave(_ context.Context, _ *model.PromptUserDraft) error {
	return nil
}

func (h *EmptyPromptUserDraftHook) AfterFind(_ context.Context, _ []*model.PromptUserDraft) error {
	return nil
}
