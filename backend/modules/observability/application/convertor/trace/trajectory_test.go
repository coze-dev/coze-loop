// Copyright (c) 2025 coze-dev Authors
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"testing"

	"github.com/coze-dev/coze-loop/backend/modules/observability/domain/trace/entity/loop_span"
	"github.com/stretchr/testify/assert"
)

func TestTrajectoriesDO2DTO(t *testing.T) {
	// empty
	assert.Nil(t, TrajectoriesDO2DTO(nil))
	assert.Nil(t, TrajectoriesDO2DTO([]*loop_span.Trajectory{}))
	// single
	traj := &loop_span.Trajectory{ID: strPtr("tid")}
	res := TrajectoriesDO2DTO([]*loop_span.Trajectory{traj})
	assert.Len(t, res, 1)
	assert.Equal(t, "tid", *res[0].ID)
}

func TestTrajectoryDO2DTO(t *testing.T) {
	assert.Nil(t, TrajectoryDO2DTO(nil))
	root := &loop_span.RootStep{ID: strPtr("rid"), Name: strPtr("root"), BasicInfo: &loop_span.BasicInfo{StartedAt: "1", Duration: "2"}}
	agent := &loop_span.AgentStep{ID: strPtr("aid"), Name: strPtr("agent"), Steps: []*loop_span.Step{}}
	traj := &loop_span.Trajectory{ID: strPtr("tid"), RootStep: root, AgentSteps: []*loop_span.AgentStep{agent}}
	dto := TrajectoryDO2DTO(traj)
	assert.NotNil(t, dto)
	assert.Equal(t, "tid", *dto.ID)
	assert.Equal(t, "root", *dto.RootStep.Name)
	assert.Equal(t, "agent", *dto.AgentSteps[0].Name)
}

func TestAgentStepsDO2DTO_and_AgentStepDO2DTO(t *testing.T) {
	assert.Nil(t, AgentStepsDO2DTO(nil))
	assert.Nil(t, AgentStepsDO2DTO([]*loop_span.AgentStep{}))
	st := &loop_span.Step{ID: strPtr("sid"), Name: strPtr("s"), BasicInfo: &loop_span.BasicInfo{StartedAt: "10", Duration: "20"}}
	as := &loop_span.AgentStep{ID: strPtr("aid"), ParentID: strPtr("pid"), Name: strPtr("agent"), Input: strPtr("in"), Output: strPtr("out"), Steps: []*loop_span.Step{st}, Metadata: map[string]string{"k": "v"}, BasicInfo: &loop_span.BasicInfo{StartedAt: "1", Duration: "2"}, MetricsInfo: &loop_span.MetricsInfo{InputTokens: int32Ptr(1)}}
	dto := AgentStepDO2DTO(as)
	assert.NotNil(t, dto)
	assert.Equal(t, "agent", *dto.Name)
	list := AgentStepsDO2DTO([]*loop_span.AgentStep{as})
	assert.Len(t, list, 1)
	assert.Equal(t, "agent", *list[0].Name)
}

func TestStepsDO2DTO_and_StepDO2DTO(t *testing.T) {
	assert.Nil(t, StepsDO2DTO(nil))
	assert.Nil(t, StepsDO2DTO([]*loop_span.Step{}))
	m := &loop_span.ModelInfo{InputTokens: 100, OutputTokens: 200, LatencyFirstResp: "50", ReasoningTokens: 1, InputReadCachedTokens: 2, InputCreationCachedTokens: 3}
	st := &loop_span.Step{ID: strPtr("sid"), ParentID: strPtr("pid"), Type: stepTypePtr("model"), Name: strPtr("s"), Input: strPtr("in"), Output: strPtr("out"), ModelInfo: m, Metadata: map[string]string{"mk": "mv"}, BasicInfo: &loop_span.BasicInfo{StartedAt: "10", Duration: "20", Error: &loop_span.Error{Code: 1, Msg: "err"}}}
	d := StepDO2DTO(st)
	assert.NotNil(t, d)
	assert.Equal(t, "s", *d.Name)
	assert.Equal(t, int32(100), *d.ModelInfo.InputTokens)
	list := StepsDO2DTO([]*loop_span.Step{st})
	assert.Len(t, list, 1)
}

func TestModelInfoDO2DTO(t *testing.T) {
	assert.Nil(t, ModelInfoDO2DTO(nil))
	m := &loop_span.ModelInfo{InputTokens: 5, OutputTokens: 6, LatencyFirstResp: "7", ReasoningTokens: 8, InputReadCachedTokens: 9, InputCreationCachedTokens: 10}
	d := ModelInfoDO2DTO(m)
	assert.Equal(t, int32(5), *d.InputTokens)
	assert.Equal(t, "7", *d.LatencyFirstResp)
}

func TestInt64Ptr2Int32Ptr(t *testing.T) {
	assert.Nil(t, int64Ptr2int32Ptr(nil))
	v := int64(123)
	p := int64Ptr2int32Ptr(&v)
	assert.Equal(t, int32(123), *p)
}

func TestRootStepDO2DTO(t *testing.T) {
	assert.Nil(t, RootStepDO2DTO(nil))
	rs := &loop_span.RootStep{ID: strPtr("id"), Name: strPtr("root"), Input: strPtr("in"), Output: strPtr("out"), Metadata: map[string]string{"rk": "rv"}, BasicInfo: &loop_span.BasicInfo{StartedAt: "1", Duration: "2"}, MetricsInfo: &loop_span.MetricsInfo{ModelErrorRate: floatPtr(0.1)}}
	d := RootStepDO2DTO(rs)
	assert.Equal(t, "root", *d.Name)
	assert.NotNil(t, d.MetricsInfo)
}

func TestMetricsInfoDO2DTO(t *testing.T) {
	assert.Nil(t, MetricsInfoDO2DTO(nil))
	m := &loop_span.MetricsInfo{LlmDuration: strPtr("1"), ToolDuration: strPtr("2"), ToolErrors: map[int32][]string{1: {"s1"}}, ToolErrorRate: floatPtr(0.1), ModelErrors: map[int32][]string{2: {"m1"}}, ModelErrorRate: floatPtr(0.2), ToolStepProportion: floatPtr(0.3), InputTokens: int32Ptr(10), OutputTokens: int32Ptr(20)}
	d := MetricsInfoDO2DTO(m)
	assert.Equal(t, int32(10), *d.InputTokens)
	assert.Equal(t, int32(20), *d.OutputTokens)
}

func TestBasicInfoDO2DTO_and_ErrorDO2DTO(t *testing.T) {
	assert.Nil(t, BasicInfoDO2DTO(nil))
	bi := &loop_span.BasicInfo{StartedAt: "100", Duration: "200", Error: &loop_span.Error{Code: 500, Msg: "oops"}}
	d := BasicInfoDO2DTO(bi)
	assert.Equal(t, "100", *d.StartedAt)
	assert.Equal(t, "200", *d.Duration)
	assert.Equal(t, int32(500), *d.Error.Code)
	assert.Equal(t, "oops", *d.Error.Msg)
	// Error nil
	bi2 := &loop_span.BasicInfo{StartedAt: "1", Duration: "2"}
	d2 := BasicInfoDO2DTO(bi2)
	assert.Nil(t, d2.Error)
	assert.Nil(t, ErrorDO2DTO(nil))
}

// helpers
func strPtr(s string) *string                  { return &s }
func stepTypePtr(s string) *loop_span.StepType { st := s; return &st }
func int32Ptr(v int32) *int32                  { return &v }
func floatPtr(f float64) *float64              { return &f }
