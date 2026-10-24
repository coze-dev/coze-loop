// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"fmt"

	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/domain/repo"
	"github.com/coze-dev/coze-loop/backend/modules/prompt/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func preserveDraftModelConfig(ctx context.Context, manageRepo repo.IManageRepo, saved, incoming *entity.Prompt, models rpc.IModelReasoningEffortProvider) error {
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
	raw := incoming.PromptDraft.PromptDetail.ModelConfig
	if raw != nil && raw.Thinking != nil && raw.Thinking.ReasoningEffortValue != nil {
		if err := validateActiveReasoningEffort(ctx, models, saved.SpaceID, config, previousModel); err != nil {
			return err
		}
	}
	incoming.PromptDraft.PromptDetail.ModelConfig = config
	return nil
}

func validateActiveReasoningEffort(ctx context.Context, models rpc.IModelReasoningEffortProvider, spaceID int64, incoming, previous *entity.ModelConfig) error {
	if incoming == nil || incoming.Thinking == nil || incoming.Thinking.ReasoningEffortValue == nil {
		return nil
	}
	thinking := incoming.Thinking
	if (thinking.ThinkingOption != nil && *thinking.ThinkingOption == entity.ThinkingOptionDisabled) || *thinking.ReasoningEffortValue == "" {
		return nil
	}
	value := *thinking.ReasoningEffortValue
	if previous != nil && previous.ModelID == incoming.ModelID && previous.Thinking != nil &&
		previous.Thinking.ReasoningEffortValue != nil && *previous.Thinking.ReasoningEffortValue == value {
		return nil
	}
	path := "model_config.thinking.reasoning_effort_value"
	if models == nil || incoming.ModelID <= 0 {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg(path+": model capability query is unavailable"))
	}
	enabled, options, err := models.GetReasoningEffortOptions(ctx, spaceID, incoming.ModelID)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if !enabled {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg(path+": selected model does not support reasoning effort"))
	}
	if len(options) == 0 {
		options = []string{"low", "medium", "high"}
	}
	for _, option := range options {
		if option == value {
			return nil
		}
	}
	return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg(path+": value is not in the selected model's reasoning effort options"))
}

func validatePublishingReasoningEffort(ctx context.Context, manageRepo repo.IManageRepo, saved *entity.Prompt, models rpc.IModelReasoningEffortProvider) error {
	if saved == nil || saved.PromptDraft == nil || saved.PromptDraft.PromptDetail == nil || saved.PromptDraft.DraftInfo == nil {
		return nil
	}
	incoming := saved.PromptDraft.PromptDetail.ModelConfig
	if incoming == nil || incoming.Thinking == nil || incoming.Thinking.ReasoningEffortValue == nil ||
		*incoming.Thinking.ReasoningEffortValue == "" || (incoming.Thinking.ThinkingOption != nil && *incoming.Thinking.ThinkingOption == entity.ThinkingOptionDisabled) {
		return nil
	}
	version := saved.PromptDraft.DraftInfo.BaseVersion
	if version == "" && saved.PromptBasic != nil {
		version = saved.PromptBasic.LatestVersion
	}
	var previous *entity.ModelConfig
	if version != "" {
		base, err := manageRepo.GetPrompt(ctx, repo.GetPromptParam{PromptID: saved.ID, WithCommit: true, CommitVersion: version})
		if err != nil {
			return err
		}
		if base != nil && base.GetPromptDetail() != nil {
			previous = base.GetPromptDetail().ModelConfig
		}
	}
	return validateActiveReasoningEffort(ctx, models, saved.SpaceID, incoming, previous)
}
