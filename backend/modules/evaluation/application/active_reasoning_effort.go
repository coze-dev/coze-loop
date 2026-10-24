// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package application

import (
	"context"
	"fmt"
	"strconv"

	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/component/rpc"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/entity"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/domain/service"
	"github.com/coze-dev/coze-loop/backend/modules/evaluation/pkg/errno"
	"github.com/coze-dev/coze-loop/backend/pkg/errorx"
)

func evaluatorModelConfig(evaluator *entity.Evaluator) *entity.ModelConfig {
	if evaluator == nil {
		return nil
	}
	switch evaluator.EvaluatorType {
	case entity.EvaluatorTypePrompt:
		if evaluator.PromptEvaluatorVersion != nil {
			return evaluator.PromptEvaluatorVersion.ModelConfig
		}
	case entity.EvaluatorTypeAgent:
		if evaluator.AgentEvaluatorVersion != nil {
			return evaluator.AgentEvaluatorVersion.ModelConfig
		}
	}
	return nil
}

func sameEvaluatorModelIdentity(next, old *entity.ModelConfig) bool {
	if next == nil || old == nil {
		return false
	}
	sameProvider := next.ProviderModelID == nil && old.ProviderModelID == nil ||
		next.ProviderModelID != nil && old.ProviderModelID != nil && *next.ProviderModelID == *old.ProviderModelID
	projected := old.GetModelID() == 0 && next.GetModelID() > 0 && old.ProviderModelID != nil &&
		strconv.FormatInt(next.GetModelID(), 10) == *old.ProviderModelID && (next.ProviderModelID == nil || sameProvider)
	knownProviderOmitted := next.GetModelID() != 0 && next.ProviderModelID == nil
	return (next.GetModelID() == old.GetModelID() || projected) && (knownProviderOmitted || sameProvider)
}

func validateActiveEvaluatorReasoningEffort(ctx context.Context, models rpc.IModelReasoningEffortProvider, spaceID int64, incoming, previous *entity.Evaluator) error {
	next, old := evaluatorModelConfig(incoming), evaluatorModelConfig(previous)
	if next == nil || next.Thinking == nil || next.Thinking.ReasoningEffort == nil || *next.Thinking.ReasoningEffort == "" {
		return nil
	}
	if err := next.ValidateThinking(); err != nil {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg(err.Error()))
	}
	sameModel := previous != nil && incoming.EvaluatorType == previous.EvaluatorType && sameEvaluatorModelIdentity(next, old)
	if next.Thinking.ThinkingOption != nil && *next.Thinking.ThinkingOption == "disabled" ||
		sameModel && next.Thinking.ThinkingOption == nil && old.Thinking != nil && old.Thinking.ThinkingOption != nil && *old.Thinking.ThinkingOption == "disabled" {
		return nil
	}
	if sameModel && old.Thinking != nil && old.Thinking.ReasoningEffort != nil && *old.Thinking.ReasoningEffort == *next.Thinking.ReasoningEffort {
		return nil
	}
	modelID := next.GetModelID()
	if modelID == 0 && next.ProviderModelID != nil {
		modelID, _ = strconv.ParseInt(*next.ProviderModelID, 10, 64)
	}
	path := "model_config.thinking.reasoning_effort"
	if models == nil || modelID <= 0 {
		return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg(path+": model capability query is unavailable"))
	}
	enabled, options, err := models.GetReasoningEffortOptions(ctx, spaceID, modelID)
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
		if option == *next.Thinking.ReasoningEffort {
			return nil
		}
	}
	return errorx.NewByCode(errno.CommonInvalidParamCode, errorx.WithExtraMsg(path+": value is not in the selected model's reasoning effort options"))
}

func validateSubmittingEvaluatorReasoningEffort(ctx context.Context, models rpc.IModelReasoningEffortProvider, evaluators service.EvaluatorService, incoming *entity.Evaluator) error {
	model := evaluatorModelConfig(incoming)
	if model == nil || model.Thinking == nil || model.Thinking.ReasoningEffort == nil || *model.Thinking.ReasoningEffort == "" ||
		model.Thinking.ThinkingOption != nil && *model.Thinking.ThinkingOption == "disabled" {
		return nil
	}
	var previous *entity.Evaluator
	if incoming.LatestVersion != "" {
		found, err := evaluators.BatchGetEvaluatorByIDAndVersion(ctx, [][2]interface{}{{incoming.ID, incoming.LatestVersion}})
		if err != nil {
			return err
		}
		if len(found) != 1 || found[0] == nil {
			return errorx.NewByCode(errno.EvaluatorNotExistCode, errorx.WithExtraMsg("published evaluator baseline not found"))
		}
		previous = found[0]
	}
	return validateActiveEvaluatorReasoningEffort(ctx, models, incoming.SpaceID, incoming, previous)
}
