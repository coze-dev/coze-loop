// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	dtotrajectory "github.com/coze-dev/coze-loop/backend/kitex_gen/coze/loop/trajectory"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

func TrajectoriesDO2DTO(trajectories []*loop_span.Trajectory) []*dtotrajectory.Trajectory {
	if len(trajectories) == 0 {
		return nil
	}
	result := make([]*dtotrajectory.Trajectory, len(trajectories))
	for i, trajectory := range trajectories {
		result[i] = TrajectoryDO2DTO(trajectory)
	}
	return result
}

func TrajectoryDO2DTO(trajectory *loop_span.Trajectory) *dtotrajectory.Trajectory {
	if trajectory == nil {
		return nil
	}
	return &dtotrajectory.Trajectory{
		ID:         trajectory.ID,
		RootStep:   RootStepDO2DTO(trajectory.RootStep),
		AgentSteps: AgentStepsDO2DTO(trajectory.AgentSteps),
	}
}

func AgentStepsDO2DTO(steps []*loop_span.AgentStep) []*dtotrajectory.AgentStep {
	if len(steps) == 0 {
		return nil
	}
	result := make([]*dtotrajectory.AgentStep, len(steps))
	for i, step := range steps {
		result[i] = AgentStepDO2DTO(step)
	}
	return result
}

func AgentStepDO2DTO(step *loop_span.AgentStep) *dtotrajectory.AgentStep {
	if step == nil {
		return nil
	}
	return &dtotrajectory.AgentStep{
		ID:          step.ID,
		ParentID:    step.ParentID,
		Name:        step.Name,
		Input:       step.Input,
		Output:      step.Output,
		Steps:       StepsDO2DTO(step.Steps),
		Metadata:    step.Metadata,
		BasicInfo:   BasicInfoDO2DTO(step.BasicInfo),
		MetricsInfo: MetricsInfoDO2DTO(step.MetricsInfo),
	}
}

func StepsDO2DTO(steps []*loop_span.Step) []*dtotrajectory.Step {
	if len(steps) == 0 {
		return nil
	}
	result := make([]*dtotrajectory.Step, len(steps))
	for i, step := range steps {
		result[i] = StepDO2DTO(step)
	}
	return result
}

func StepDO2DTO(step *loop_span.Step) *dtotrajectory.Step {
	if step == nil {
		return nil
	}
	return &dtotrajectory.Step{
		ID:        step.ID,
		ParentID:  step.ParentID,
		Type:      step.Type,
		Name:      step.Name,
		Input:     step.Input,
		Output:    step.Output,
		ModelInfo: ModelInfoDO2DTO(step.ModelInfo),
		Metadata:  step.Metadata,
		BasicInfo: BasicInfoDO2DTO(step.BasicInfo),
	}
}

func ModelInfoDO2DTO(info *loop_span.ModelInfo) *dtotrajectory.ModelInfo {
	if info == nil {
		return nil
	}
	return &dtotrajectory.ModelInfo{
		InputTokens:               int64Ptr2int32Ptr(&info.InputTokens),
		OutputTokens:              int64Ptr2int32Ptr(&info.OutputTokens),
		LatencyFirstResp:          &info.LatencyFirstResp,
		ReasoningTokens:           int64Ptr2int32Ptr(&info.ReasoningTokens),
		InputReadCachedTokens:     int64Ptr2int32Ptr(&info.InputReadCachedTokens),
		InputCreationCachedTokens: int64Ptr2int32Ptr(&info.InputCreationCachedTokens),
	}
}

func int64Ptr2int32Ptr(src *int64) *int32 {
	if src == nil {
		return nil
	}
	result := int32(*src)
	return &result
}

func RootStepDO2DTO(step *loop_span.RootStep) *dtotrajectory.RootStep {
	if step == nil {
		return nil
	}
	return &dtotrajectory.RootStep{
		ID:          step.ID,
		Name:        step.Name,
		Input:       step.Input,
		Output:      step.Output,
		Metadata:    step.Metadata,
		BasicInfo:   BasicInfoDO2DTO(step.BasicInfo),
		MetricsInfo: MetricsInfoDO2DTO(step.MetricsInfo),
	}
}

func MetricsInfoDO2DTO(info *loop_span.MetricsInfo) *dtotrajectory.MetricsInfo {
	if info == nil {
		return nil
	}
	return &dtotrajectory.MetricsInfo{
		LlmDuration:        info.LlmDuration,
		ToolDuration:       info.ToolDuration,
		ToolErrors:         info.ToolErrors,
		ToolErrorRate:      info.ToolErrorRate,
		ModelErrors:        info.ModelErrors,
		ModelErrorRate:     info.ModelErrorRate,
		ToolStepProportion: info.ToolStepProportion,
	}
}

func BasicInfoDO2DTO(info *loop_span.BasicInfo) *dtotrajectory.BasicInfo {
	if info == nil {
		return nil
	}
	return &dtotrajectory.BasicInfo{
		StartedAt: &info.StartedAt,
		Duration:  &info.Duration,
		Error:     ErrorDO2DTO(info.Error),
	}
}

func ErrorDO2DTO(e *loop_span.Error) *dtotrajectory.Error {
	if e == nil {
		return nil
	}
	return &dtotrajectory.Error{
		Code: &e.Code,
		Msg:  &e.Msg,
	}
}
