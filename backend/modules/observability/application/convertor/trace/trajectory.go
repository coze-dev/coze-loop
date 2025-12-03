package trace

import (
	"strconv"

	"github.com/coze-dev/coze-loop/backend/kitex_gen/common"
	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
)

func TrajectoriesDO2DTO(trajectories []*loop_span.Trajectory) []*common.Trajectory {
	if len(trajectories) == 0 {
		return nil
	}
	result := make([]*common.Trajectory, len(trajectories))
	for i, trajectory := range trajectories {
		result[i] = TrajectoryDO2DTO(trajectory)
	}
	return result
}

func TrajectoryDO2DTO(trajectory *loop_span.Trajectory) *common.Trajectory {
	if trajectory == nil {
		return nil
	}
	return &common.Trajectory{
		ID:         trajectory.ID,
		RootStep:   RootStepDO2DTO(trajectory.RootStep),
		AgentSteps: AgentStepsDO2DTO(trajectory.AgentSteps),
	}
}

func AgentStepsDO2DTO(steps []*loop_span.AgentStep) []*common.AgentStep {
	if len(steps) == 0 {
		return nil
	}
	result := make([]*common.AgentStep, len(steps))
	for i, step := range steps {
		result[i] = AgentStepDO2DTO(step)
	}
	return result
}

func AgentStepDO2DTO(step *loop_span.AgentStep) *common.AgentStep {
	if step == nil {
		return nil
	}
	return &common.AgentStep{
		ID:        step.ID,
		ParentID:  step.ParentID,
		Name:      step.Name,
		Input:     step.Input,
		Output:    step.Output,
		Steps:     StepsDO2DTO(step.Steps),
		Metadata:  step.Metadata,
		BasicInfo: BasicInfoDO2DTO(step.BasicInfo),
	}
}

func StepsDO2DTO(steps []*loop_span.Step) []*common.Step {
	if len(steps) == 0 {
		return nil
	}
	result := make([]*common.Step, len(steps))
	for i, step := range steps {
		result[i] = StepDO2DTO(step)
	}
	return result
}

func StepDO2DTO(step *loop_span.Step) *common.Step {
	if step == nil {
		return nil
	}
	return &common.Step{
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

func ModelInfoDO2DTO(info *loop_span.ModelInfo) *common.ModelInfo {
	if info == nil {
		return nil
	}
	return &common.ModelInfo{
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

func int64Ptr2StrPtr(src *int64) *string {
	if src == nil {
		return nil
	}
	result := strconv.FormatInt(*src, 10)
	return &result
}

func RootStepDO2DTO(step *loop_span.RootStep) *common.RootStep {
	if step == nil {
		return nil
	}
	return &common.RootStep{
		ID:        step.ID,
		Name:      step.Name,
		Input:     step.Input,
		Output:    step.Output,
		Metadata:  step.Metadata,
		BasicInfo: BasicInfoDO2DTO(step.BasicInfo),
	}
}

func BasicInfoDO2DTO(info *loop_span.BasicInfo) *common.BasicInfo {
	if info == nil {
		return nil
	}
	return &common.BasicInfo{
		StartedAt: &info.StartedAt,
		Duration:  &info.Duration,
		Error:     ErrorDO2DTO(info.Error),
	}
}

func ErrorDO2DTO(e *loop_span.Error) *common.Error {
	if e == nil {
		return nil
	}
	return &common.Error{
		Code: &e.Code,
		Msg:  &e.Msg,
	}
}
