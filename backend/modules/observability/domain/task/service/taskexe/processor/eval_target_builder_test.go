// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package processor

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	eval_target_d "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/evaluation/domain/eval_target"
	"github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/observability/domain/common"
	task_entity "github.com/coze-dev/coze-loop/backend/modules/observability/domain/task/entity"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

func TestEvalTargetBuilderImpl_Build(t *testing.T) {
	builder := &EvalTargetBuilderImpl{}
	ctx := context.Background()

	t.Run("nil SpanFilter returns Trace type", func(t *testing.T) {
		task := &task_entity.ObservabilityTask{
			SpanFilter: nil,
			TaskConfig: &task_entity.TaskConfig{},
		}
		result := builder.Build(ctx, task)
		require.NotNil(t, result)
		assert.Equal(t, eval_target_d.EvalTargetType_Trace, *result.EvalTargetType)
		assert.Nil(t, result.SourceTargetID)
	})

	t.Run("nil SpanFilter with EvaluationExperimentConfig returns sourceTargetID", func(t *testing.T) {
		srcID := "src-123"
		task := &task_entity.ObservabilityTask{
			SpanFilter: nil,
			TaskConfig: &task_entity.TaskConfig{
				EvaluationExperimentConfig: &task_entity.EvaluationExperimentConfig{
					SourceTargetID: &srcID,
				},
			},
		}
		result := builder.Build(ctx, task)
		require.NotNil(t, result)
		assert.Equal(t, eval_target_d.EvalTargetType_Trace, *result.EvalTargetType)
		require.NotNil(t, result.SourceTargetID)
		assert.Equal(t, "src-123", *result.SourceTargetID)
	})

	t.Run("PlatformTypeCozeBot returns CozeBotOnline", func(t *testing.T) {
		task := &task_entity.ObservabilityTask{
			SpanFilter: &task_entity.SpanFilterFields{
				PlatformType: loop_span.PlatformType(common.PlatformTypeCozeBot),
			},
			TaskConfig: &task_entity.TaskConfig{},
		}
		result := builder.Build(ctx, task)
		require.NotNil(t, result)
		assert.Equal(t, eval_target_d.EvalTargetType_CozeBotOnline, *result.EvalTargetType)
	})

	t.Run("PlatformTypeInnerCozeBot returns CozeBotOnline", func(t *testing.T) {
		task := &task_entity.ObservabilityTask{
			SpanFilter: &task_entity.SpanFilterFields{
				PlatformType: loop_span.PlatformType(common.PlatformTypeInnerCozeBot),
			},
			TaskConfig: &task_entity.TaskConfig{},
		}
		result := builder.Build(ctx, task)
		require.NotNil(t, result)
		assert.Equal(t, eval_target_d.EvalTargetType_CozeBotOnline, *result.EvalTargetType)
	})

	t.Run("PlatformTypePrompt returns CozeLoopPromptOnline", func(t *testing.T) {
		task := &task_entity.ObservabilityTask{
			SpanFilter: &task_entity.SpanFilterFields{
				PlatformType: loop_span.PlatformType(common.PlatformTypePrompt),
			},
			TaskConfig: &task_entity.TaskConfig{},
		}
		result := builder.Build(ctx, task)
		require.NotNil(t, result)
		assert.Equal(t, eval_target_d.EvalTargetType_CozeLoopPromptOnline, *result.EvalTargetType)
	})

	t.Run("PlatformTypeInnerPrompt returns CozeLoopPromptOnline", func(t *testing.T) {
		task := &task_entity.ObservabilityTask{
			SpanFilter: &task_entity.SpanFilterFields{
				PlatformType: loop_span.PlatformType(common.PlatformTypeInnerPrompt),
			},
			TaskConfig: &task_entity.TaskConfig{},
		}
		result := builder.Build(ctx, task)
		require.NotNil(t, result)
		assert.Equal(t, eval_target_d.EvalTargetType_CozeLoopPromptOnline, *result.EvalTargetType)
	})

	t.Run("PlatformTypeCozeloop returns CustomRPCServerOnline", func(t *testing.T) {
		task := &task_entity.ObservabilityTask{
			SpanFilter: &task_entity.SpanFilterFields{
				PlatformType: loop_span.PlatformType(common.PlatformTypeCozeloop),
			},
			TaskConfig: &task_entity.TaskConfig{},
		}
		result := builder.Build(ctx, task)
		require.NotNil(t, result)
		assert.Equal(t, eval_target_d.EvalTargetType_CustomRPCServerOnline, *result.EvalTargetType)
	})

	t.Run("PlatformTypeInnerCozeloop returns CustomRPCServerOnline", func(t *testing.T) {
		task := &task_entity.ObservabilityTask{
			SpanFilter: &task_entity.SpanFilterFields{
				PlatformType: loop_span.PlatformType(common.PlatformTypeInnerCozeloop),
			},
			TaskConfig: &task_entity.TaskConfig{},
		}
		result := builder.Build(ctx, task)
		require.NotNil(t, result)
		assert.Equal(t, eval_target_d.EvalTargetType_CustomRPCServerOnline, *result.EvalTargetType)
	})

	t.Run("PlatformTypeWorkflow returns CozeWorkflowOnline", func(t *testing.T) {
		task := &task_entity.ObservabilityTask{
			SpanFilter: &task_entity.SpanFilterFields{
				PlatformType: loop_span.PlatformType(common.PlatformTypeWorkflow),
			},
			TaskConfig: &task_entity.TaskConfig{},
		}
		result := builder.Build(ctx, task)
		require.NotNil(t, result)
		assert.Equal(t, eval_target_d.EvalTargetType_CozeWorkflowOnline, *result.EvalTargetType)
	})

	t.Run("PlatformTypeVeAgentkit returns VolcengineAgentAgentkitOnline", func(t *testing.T) {
		task := &task_entity.ObservabilityTask{
			SpanFilter: &task_entity.SpanFilterFields{
				PlatformType: loop_span.PlatformType(common.PlatformTypeVeAgentkit),
			},
			TaskConfig: &task_entity.TaskConfig{},
		}
		result := builder.Build(ctx, task)
		require.NotNil(t, result)
		assert.Equal(t, eval_target_d.EvalTargetType_VolcengineAgentAgentkitOnline, *result.EvalTargetType)
	})

	t.Run("PlatformTypeVeADK returns VolcengineAgentOnline", func(t *testing.T) {
		task := &task_entity.ObservabilityTask{
			SpanFilter: &task_entity.SpanFilterFields{
				PlatformType: loop_span.PlatformType(common.PlatformTypeVeADK),
			},
			TaskConfig: &task_entity.TaskConfig{},
		}
		result := builder.Build(ctx, task)
		require.NotNil(t, result)
		assert.Equal(t, eval_target_d.EvalTargetType_VolcengineAgentOnline, *result.EvalTargetType)
	})

	t.Run("unknown PlatformType returns Trace type", func(t *testing.T) {
		task := &task_entity.ObservabilityTask{
			SpanFilter: &task_entity.SpanFilterFields{
				PlatformType: loop_span.PlatformType("unknown_platform"),
			},
			TaskConfig: &task_entity.TaskConfig{},
		}
		result := builder.Build(ctx, task)
		require.NotNil(t, result)
		assert.Equal(t, eval_target_d.EvalTargetType_Trace, *result.EvalTargetType)
	})

	t.Run("with sourceTargetID and valid PlatformType", func(t *testing.T) {
		srcID := "target-456"
		task := &task_entity.ObservabilityTask{
			SpanFilter: &task_entity.SpanFilterFields{
				PlatformType: loop_span.PlatformType(common.PlatformTypeCozeBot),
			},
			TaskConfig: &task_entity.TaskConfig{
				EvaluationExperimentConfig: &task_entity.EvaluationExperimentConfig{
					SourceTargetID: &srcID,
				},
			},
		}
		result := builder.Build(ctx, task)
		require.NotNil(t, result)
		assert.Equal(t, eval_target_d.EvalTargetType_CozeBotOnline, *result.EvalTargetType)
		require.NotNil(t, result.SourceTargetID)
		assert.Equal(t, "target-456", *result.SourceTargetID)
	})

	t.Run("nil EvaluationExperimentConfig sourceTargetID is nil", func(t *testing.T) {
		task := &task_entity.ObservabilityTask{
			SpanFilter: &task_entity.SpanFilterFields{
				PlatformType: loop_span.PlatformType(common.PlatformTypeWorkflow),
			},
			TaskConfig: &task_entity.TaskConfig{
				EvaluationExperimentConfig: nil,
			},
		}
		result := builder.Build(ctx, task)
		require.NotNil(t, result)
		assert.Nil(t, result.SourceTargetID)
		assert.Equal(t, eval_target_d.EvalTargetType_CozeWorkflowOnline, *result.EvalTargetType)
	})

	t.Run("EvaluationExperimentConfig with nil SourceTargetID", func(t *testing.T) {
		task := &task_entity.ObservabilityTask{
			SpanFilter: &task_entity.SpanFilterFields{
				PlatformType: loop_span.PlatformType(common.PlatformTypePrompt),
			},
			TaskConfig: &task_entity.TaskConfig{
				EvaluationExperimentConfig: &task_entity.EvaluationExperimentConfig{
					SourceTargetID: nil,
					ExptTemplateID: lo.ToPtr(int64(999)),
				},
			},
		}
		result := builder.Build(ctx, task)
		require.NotNil(t, result)
		assert.Nil(t, result.SourceTargetID)
		assert.Equal(t, eval_target_d.EvalTargetType_CozeLoopPromptOnline, *result.EvalTargetType)
	})
}
