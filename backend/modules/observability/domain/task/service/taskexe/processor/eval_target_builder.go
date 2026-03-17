// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package processor

import (
	"context"

	eval_target_d "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_target"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/eval_target"
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
	// todo fby
	var sourceTargetID *string = nil
	if task.TaskConfig.EvaluationExperimentConfig != nil {
		sourceTargetID = task.TaskConfig.EvaluationExperimentConfig.SourceTargetID
	}
	return &eval_target.CreateEvalTargetParam{
		EvalTargetType: lo.ToPtr(eval_target_d.EvalTargetType_Trace),
		SourceTargetID: sourceTargetID,
	}
}
