// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package processor

import (
	"context"
	eval_target_d "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_target"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_target"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	task_entity "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/samber/lo"
)

type EvalTargetBuilder interface {
	Build(ctx context.Context, task *task_entity.ObservabilityTask) *eval_target.CreateEvalTargetParam
}

type EvalTargetBuilderImpl struct {
	EvalTargetBuilder
}

func (b *EvalTargetBuilderImpl) Build(ctx context.Context, task *task_entity.ObservabilityTask) *eval_target.CreateEvalTargetParam {
	var sourceTargetID *string = nil
	if task.TaskConfig.EvaluationExperimentConfig != nil {
		sourceTargetID = task.TaskConfig.EvaluationExperimentConfig.SourceTargetID
	}
	ret := &eval_target.CreateEvalTargetParam{
		EvalTargetType: lo.ToPtr(eval_target_d.EvalTargetType_Trace),
		SourceTargetID: sourceTargetID,
	}
	evalTargetType := eval_target_d.EvalTargetType_Trace
	switch string(task.SpanFilter.PlatformType) {
	case common.PlatformTypeInnerCozeBot:
	case common.PlatformTypeCozeBot:
		evalTargetType = eval_target_d.EvalTargetType_CozeBotOnline

	case common.PlatformTypeInnerPrompt:
	case common.PlatformTypePrompt:
		evalTargetType = eval_target_d.EvalTargetType_CozeLoopPromptOnline

	case common.PlatformTypeInnerCozeloop:
	case common.PlatformTypeCozeloop:
		evalTargetType = eval_target_d.EvalTargetType_CustomRPCServerOnline

	case common.PlatformTypeWorkflow:
		evalTargetType = eval_target_d.EvalTargetType_CozeWorkflowOnline
	case common.PlatformTypeVeAgentkit:
		evalTargetType = eval_target_d.EvalTargetType_VolcengineAgentAgentkitOnline
	case common.PlatformTypeVeADK:
		evalTargetType = eval_target_d.EvalTargetType_VolcengineAgentOnline

	}
	ret.EvalTargetType = lo.ToPtr(evalTargetType)
	return ret
}
