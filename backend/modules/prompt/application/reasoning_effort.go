// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
)

func preserveDraftModelConfig(ctx context.Context, manageRepo repo.IManageRepo, saved, incoming *entity.Prompt) error {
	previous := saved.GetPromptDetail()
	if previous == nil && saved.PromptBasic != nil {
		version := incoming.PromptDraft.DraftInfo.BaseVersion
		if version == "" {
			version = saved.PromptBasic.LatestVersion
		}
		if version != "" {
			base, err := manageRepo.GetPrompt(ctx, repo.GetPromptParam{PromptID: saved.ID, WithCommit: true, CommitVersion: version})
			if err != nil {
				return err
			}
			previous = base.GetPromptDetail()
		}
	}
	var previousModel *entity.ModelConfig
	if previous != nil {
		previousModel = previous.ModelConfig
	}
	config, err := entity.PreserveModelConfigForSave(incoming.PromptDraft.PromptDetail.ModelConfig, previousModel)
	if err != nil {
		return err
	}
	incoming.PromptDraft.PromptDetail.ModelConfig = config
	return nil
}
